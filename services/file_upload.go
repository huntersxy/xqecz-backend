package services

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"xiaoquan-backend/config"
)

type FileUploadConfig struct {
	AllowedExtensions []string
	MaxSize           int64
	UploadDir         string
	UserID            uint
}

type FileUploadResult struct {
	Filename string
	FilePath string
	FileSize int64
	URL      string
}

type FileUploadService struct {
	config *FileUploadConfig
}

func NewFileUploadService(cfg *FileUploadConfig) *FileUploadService {
	if cfg.UploadDir == "" {
		cfg.UploadDir = config.AppConfig.Server.UploadDir
	}
	return &FileUploadService{config: cfg}
}

func (s *FileUploadService) Upload(file *multipart.FileHeader) (*FileUploadResult, error) {
	if err := s.validateExtension(file.Filename); err != nil {
		return nil, err
	}

	if err := s.validateSize(file.Size); err != nil {
		return nil, err
	}

	if err := s.validateMIME(file); err != nil {
		return nil, err
	}

	ext := filepath.Ext(file.Filename)
	filename := s.generateFilename(ext)
	filePath := filepath.Join(s.config.UploadDir, filename)

	if err := s.saveFile(file, filePath); err != nil {
		return nil, err
	}

	return &FileUploadResult{
		Filename: filename,
		FilePath: filePath,
		FileSize: file.Size,
		URL:      fmt.Sprintf("/uploads/%s", filename),
	}, nil
}

func (s *FileUploadService) validateExtension(filename string) error {
	ext := filepath.Ext(filename)
	for _, allowedExt := range s.config.AllowedExtensions {
		if ext == allowedExt {
			return nil
		}
	}
	return fmt.Errorf("不支持的文件格式: %s，支持的格式: %v", ext, s.config.AllowedExtensions)
}

func (s *FileUploadService) validateSize(size int64) error {
	if size > s.config.MaxSize {
		return fmt.Errorf("文件大小超过限制，最大允许: %d MB", s.config.MaxSize/(1024*1024))
	}
	return nil
}

func (s *FileUploadService) generateFilename(ext string) string {
	return fmt.Sprintf("%d_%d%s", s.config.UserID, time.Now().UnixNano(), ext)
}

func (s *FileUploadService) saveFile(file *multipart.FileHeader, filePath string) error {
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}

	return nil
}

func (s *FileUploadService) DeleteFile(filename string) error {
	filePath := filepath.Join(s.config.UploadDir, filename)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除文件失败: %w", err)
	}
	return nil
}

func (s *FileUploadService) validateMIME(file *multipart.FileHeader) error {
	f, err := file.Open()
	if err != nil {
		return fmt.Errorf("无法读取文件: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 512)
	if _, err := io.ReadFull(f, buf); err != nil && err != io.ErrUnexpectedEOF {
		return fmt.Errorf("无法读取文件头: %w", err)
	}

	mimeType := http.DetectContentType(buf)
	allowed := map[string]bool{
		"image/jpeg":       true,
		"image/png":        true,
		"image/gif":        true,
		"image/webp":       true,
		"video/mp4":        true,
		"video/x-msvideo":  true,
		"video/quicktime":  true,
		"video/x-matroska": true,
	}
	if !allowed[mimeType] {
		return fmt.Errorf("不支持的文件类型: %s", mimeType)
	}
	return nil
}
