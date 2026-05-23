package scheduler

import (
	"context"
	"log"
	"time"
	"xiaoquan-backend/models"
	"xiaoquan-backend/utils"

	"github.com/go-redis/redis/v8"
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
