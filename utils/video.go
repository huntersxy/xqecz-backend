package utils

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"xiaoquan-backend/config"
)

func CheckFFmpeg() error {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg未安装或未配置到PATH环境变量")
	}
	return exec.Command(ffmpeg, "-version").Run()
}

func GetFFmpegVersion() (string, error) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", fmt.Errorf("获取ffmpeg版本失败: %v", err)
	}
	output, err := exec.Command(ffmpeg, "-version").Output()
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
	thumbExt := ".webp"
	thumbFilename := filename[:len(filename)-len(filepath.Ext(filename))] + "_thumb" + thumbExt
	thumbPath := filepath.Join(config.AppConfig.Server.ThumbnailDir, thumbFilename)

	ffmpeg, _ := exec.LookPath("ffmpeg")
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	// 宽图：居中，完整高度，横向4:3
	// 高图：从距离顶部8%高度开始，裁剪完整宽度的4:3
	vf := `select=eq(n\,9),crop=w='if(gte(in_w/in_h,4/3),in_h*4/3,in_w)':h='if(gte(in_w/in_h,4/3),in_h,in_w*3/4)':x='if(gte(in_w/in_h,4/3),(in_w-in_h*4/3)/2,0)':y='if(gte(in_w/in_h,4/3),0,min(in_h*0.08,in_h-in_w*3/4))'`
	cmd := exec.Command(ffmpeg, "-i", videoPath, "-vf", vf, "-vframes", "1", "-c:v", "libwebp", "-quality", "60", "-y", thumbPath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to generate thumbnail: %v", err)
	}

	return thumbFilename, nil
}

func DeleteVideoThumbnail(filename string) error {
	thumbFilename := filename[:len(filename)-len(filepath.Ext(filename))] + "_thumb.webp"
	thumbPath := filepath.Join(config.AppConfig.Server.ThumbnailDir, thumbFilename)

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

	ffmpeg, _ := exec.LookPath("ffmpeg")
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	// 宽图：居中，完整高度，横向4:3
	// 高图：从距离顶部8%高度开始，裁剪完整宽度的4:3
	vf := `crop=w='if(gte(in_w/in_h,4/3),in_h*4/3,in_w)':h='if(gte(in_w/in_h,4/3),in_h,in_w*3/4)':x='if(gte(in_w/in_h,4/3),(in_w-in_h*4/3)/2,0)':y='if(gte(in_w/in_h,4/3),0,min(in_h*0.08,in_h-in_w*3/4))',scale=800:-1`
	cmd := exec.Command(ffmpeg, "-i", originalPath, "-vf", vf, "-q:v", "8", "-y", thumbPath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to generate image thumbnail: %v", err)
	}

	return thumbFilename, nil
}

func GenerateVideoThumbnailAsync(videoPath, filename string, callback func(string, error)) {
	go func() {
		thumbFilename, err := GenerateVideoThumbnail(videoPath, filename)
		if err != nil {
			log.Printf("[缩略图] 异步生成视频缩略图失败: %v", err)
		} else {
			log.Printf("[缩略图] 异步生成视频缩略图成功: %s", thumbFilename)
		}
		if callback != nil {
			callback(thumbFilename, err)
		}
	}()
}

func GenerateImageThumbnailAsync(originalPath, filename string, callback func(string, error)) {
	go func() {
		thumbFilename, err := GenerateImageThumbnail(originalPath, filename)
		if err != nil {
			log.Printf("[缩略图] 异步生成图片缩略图失败: %v", err)
		} else {
			log.Printf("[缩略图] 异步生成图片缩略图成功: %s", thumbFilename)
		}
		if callback != nil {
			callback(thumbFilename, err)
		}
	}()
}
