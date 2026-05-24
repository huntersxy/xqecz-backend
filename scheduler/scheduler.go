package scheduler

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"xiaoquan-backend/config"
	"xiaoquan-backend/models"
	"xiaoquan-backend/services"
	"xiaoquan-backend/utils"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func StartRecommendScheduler() {
	log.Println("[定时任务] 推荐列表更新调度器已启动")

	go generateRecommendList()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		generateRecommendList()
	}
}

func generateRecommendList() {
	log.Println("[定时任务] 开始生成推荐列表...")
	startTime := time.Now()

	var allContents []models.Content
	if err := utils.DB.Model(&models.Content{}).Where("audit_status = ?", models.AuditStatusApproved).Find(&allContents).Error; err != nil {
		log.Printf("[定时任务] 获取内容失败: %v", err)
		return
	}

	if len(allContents) == 0 {
		log.Println("[定时任务] 没有可推荐的内容")
		return
	}

	if utils.RedisClient == nil {
		log.Println("[定时任务] Redis不可用，跳过")
		return
	}

	utils.DeleteTempRecommendZSet()

	today := time.Now().Truncate(24 * time.Hour)

	contentIDs := make([]uint, len(allContents))
	for i, c := range allContents {
		contentIDs[i] = c.ID
	}

	viewCounts := utils.GetAllPeriodViewCountsBatch(contentIDs)

	pipe := utils.RedisClient.Pipeline()
	key := utils.GetKey("recommend:zset:temp")

	for _, content := range allContents {
		score := 0.0

		if content.CreatedAt.After(today) {
			score += 100.0
		} else {
			daysAgo := time.Since(content.CreatedAt).Hours() / 24.0
			if daysAgo < 7 {
				score += 50.0 * (1 - daysAgo/7.0)
			}
		}

		if vc, ok := viewCounts[content.ID]; ok {
			score += float64(vc["1day"]) * 2.0
			score += float64(vc["3day"]) * 1.0
			score += float64(vc["7day"]) * 0.5
			score += float64(vc["1month"]) * 0.2
		}

		pipe.ZAdd(context.Background(), key, &redis.Z{Score: score, Member: content.ID})
	}

	if _, err := pipe.Exec(context.Background()); err != nil {
		log.Printf("[定时任务] Redis写入失败: %v", err)
		return
	}

	if err := utils.SwapRecommendZSet(); err != nil {
		log.Printf("[定时任务] 替换失败: %v", err)
		return
	}

	log.Printf("[定时任务] 推荐列表更新完成，共%d条内容，耗时: %v", len(allContents), time.Since(startTime))
}

func StartTinifyScheduler() {
	if !config.AppConfig.TinifyEnabled || config.AppConfig.TinifyAPIKey == "" {
		log.Println("[Tinify] 未启用或API Key未配置，跳过启动")
		return
	}

	log.Println("[Tinify] === 图片压缩调度器已启动（每分钟1张，连续2次失败暂停至下月/重启）===")

	consecutiveFails := 0
	paused := false
	compressedCount := 0
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		if paused && now.Day() != 1 {
			continue
		}
		if paused && now.Day() == 1 {
			log.Printf("[Tinify] === 新月已至(%s)，恢复压缩任务 ===", now.Format("2006-01-02"))
			paused = false
			consecutiveFails = 0
		}

		var content models.Content
		err := utils.DB.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).
			Where("type = ? AND file_path != ? AND (compressed_path = ? OR compressed_path IS NULL) AND file_path NOT LIKE ?",
				models.ContentTypeImage, "", "", "%.gif").Order("id ASC").First(&content).Error
		if err != nil {
			continue
		}

		originalPath := filepath.Join(config.AppConfig.Server.UploadDir, content.FilePath)
		if _, err := os.Stat(originalPath); os.IsNotExist(err) {
			log.Printf("[Tinify] 跳过 content=%d 文件不存在 %s", content.ID, originalPath)
			utils.DB.Model(&content).Update("compressed_path", "-") // 标记跳过
			continue
		}

		origInfo, _ := os.Stat(originalPath)
		log.Printf("[Tinify] ┌ 开始压缩 #%d", compressedCount+1)
		log.Printf("[Tinify] │ content=%d title=%s file=%s size=%d bytes(%.1f KB)",
			content.ID, content.Title, content.FilePath, origInfo.Size(), float64(origInfo.Size())/1024)

		filename, err := services.CompressImage(originalPath, content.UserID)
		if err != nil {
			errStr := err.Error()
			isNetwork := strings.Contains(errStr, "timeout") ||
				strings.Contains(errStr, "deadline") ||
				strings.Contains(errStr, "connection refused") ||
				strings.Contains(errStr, "no such host")
			if isNetwork {
				log.Printf("[Tinify] │ 网络超时(不计入): %v", err)
				log.Println("[Tinify] └ 跳过，下次继续尝试")
			} else {
				log.Printf("[Tinify] │ 压缩失败: %v", err)
				consecutiveFails++
				if consecutiveFails >= 2 {
					paused = true
					log.Printf("[Tinify] └ 连续%d次API错误，暂停调度（下月1号恢复）", consecutiveFails)
				} else {
					log.Println("[Tinify] └ 继续调度（需连败2次才暂停）")
				}
			}
			continue
		}
		consecutiveFails = 0
		compressedCount++

		utils.DB.Model(&content).Update("compressed_path", filename)
		log.Printf("[Tinify] └ 数据库已更新 content=%d compressed_path=%s 累计成功=%d",
			content.ID, filename, compressedCount)
	}
}
