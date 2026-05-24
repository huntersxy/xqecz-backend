package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"xiaoquan-backend/config"
)

var tinifyHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        5,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     60 * time.Second,
	},
}

func CompressImage(originalPath string, userID uint) (string, error) {
	apiKey := config.AppConfig.TinifyAPIKey
	if apiKey == "" {
		return "", fmt.Errorf("tinify API key not configured")
	}

	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("api:"+apiKey))

	origInfo, _ := os.Stat(originalPath)
	origSize := int64(0)
	if origInfo != nil {
		origSize = origInfo.Size()
	}
	log.Printf("[Tinify] 开始压缩 file=%s size=%d bytes(%.1f KB)", originalPath, origSize, float64(origSize)/1024)

	uploadStart := time.Now()
	outputURL, err := uploadToTinify(originalPath, auth)
	uploadDuration := time.Since(uploadStart)
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}
	log.Printf("[Tinify] 上传完成 output=%s 耗时=%v", outputURL, uploadDuration)

	compressedFilename := filepath.Base(originalPath)
	ext := filepath.Ext(compressedFilename)
	compressedFilename = compressedFilename[:len(compressedFilename)-len(ext)] + "_tinified.webp"
	destDir := filepath.Join(config.AppConfig.Server.UploadDir, "..", "images")
	absDir, _ := filepath.Abs(destDir)
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return "", fmt.Errorf("create images dir failed: %w", err)
	}
	destPath := filepath.Join(absDir, compressedFilename)

	downloadStart := time.Now()
	if err := downloadFromTinify(outputURL, auth, destPath); err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	downloadDuration := time.Since(downloadStart)

	newInfo, _ := os.Stat(destPath)
	newSize := int64(0)
	if newInfo != nil {
		newSize = newInfo.Size()
	}

	if origSize > 0 {
		reduction := float64(origSize-newSize) / float64(origSize) * 100
		log.Printf("[Tinify] 压缩完成 filename=%s orig=%d → new=%d bytes 减少=%.1f%% 下载耗时=%v 总耗时=%v",
			compressedFilename, origSize, newSize, reduction, downloadDuration, uploadDuration+downloadDuration)
	} else {
		log.Printf("[Tinify] 压缩完成 filename=%s new=%d bytes 下载耗时=%v 总耗时=%v",
			compressedFilename, newSize, downloadDuration, uploadDuration+downloadDuration)
	}

	return compressedFilename, nil
}

func uploadToTinify(filePath, auth string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	req, err := http.NewRequest("POST", "https://api.tinify.com/shrink", f)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", auth)

	resp, err := tinifyHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		errBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d body=%s headers=%v", resp.StatusCode, string(errBody), resp.Header)
	}

	return resp.Header.Get("Location"), nil
}

func downloadFromTinify(outputURL, auth, destPath string) error {
	body := map[string]interface{}{
		"convert": map[string]string{"type": "image/webp"},
	}
	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", outputURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := tinifyHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d body=%s", resp.StatusCode, string(errBody))
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	log.Printf("[Tinify] 下载字节数=%d contentType=%s", written, resp.Header.Get("Content-Type"))
	return nil
}
