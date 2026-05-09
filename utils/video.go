package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"xiaoquan-backend/config"
)

func CheckFFmpeg() error {
	cmd := exec.Command("ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg未安装或未配置到PATH环境变量")
	}
	return nil
}

func GetFFmpegVersion() (string, error) {
	cmd := exec.Command("ffmpeg", "-version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取ffmpeg版本失败: %v", err)
	}
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		return lines[0], nil
	}
	return "", nil
}

func GenerateVideoThumbnail(videoPath, filename string) (string, error) {
	thumbExt := ".jpg"
	thumbFilename := filename[:len(filename)-len(filepath.Ext(filename))] + "_thumb" + thumbExt
	thumbPath := filepath.Join(config.AppConfig.Server.UploadDir, thumbFilename)

	cmd := exec.Command("ffmpeg", "-i", videoPath, "-vf", "select=eq(n\\,9)", "-vframes", "1", "-q:v", "2", thumbPath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to generate thumbnail: %v", err)
	}

	return thumbFilename, nil
}

func DeleteVideoThumbnail(filename string) error {
	thumbFilename := filename[:len(filename)-len(filepath.Ext(filename))] + "_thumb.jpg"
	thumbPath := filepath.Join(config.AppConfig.Server.UploadDir, thumbFilename)

	if err := os.Remove(thumbPath); err != nil {
		return fmt.Errorf("failed to delete thumbnail: %v", err)
	}

	return nil
}

func GenerateImageThumbnail(originalPath, filename string) (string, error) {
	thumbFilename := filename[:len(filename)-len(filepath.Ext(filename))] + "_thumb.webp"
	thumbPath := filepath.Join(config.AppConfig.Server.ThumbnailDir, thumbFilename)

	if _, err := os.Stat(thumbPath); err == nil {
		return thumbFilename, nil
	}

	cmd := exec.Command("ffmpeg", "-i", originalPath, "-vf", "scale=800:-1", "-q:v", "8", "-y", thumbPath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to generate image thumbnail: %v", err)
	}

	return thumbFilename, nil
}
