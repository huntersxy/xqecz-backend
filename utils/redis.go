package utils

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"xiaoquan-backend/config"
	"xiaoquan-backend/models"

	"github.com/bytedance/sonic"
	"github.com/go-redis/redis/v8"
)

var RedisClient *redis.Client
var ctx = context.Background()

func InitRedis() error {
	cfg := config.AppConfig.Redis

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("redis connection failed: %w", err)
	}

	log.Println("Redis connected successfully")
	return nil
}

func getKey(key string) string {
	return config.AppConfig.Redis.Prefix + key
}

func SetCache(key string, value interface{}, expiration time.Duration) error {
	return RedisClient.Set(ctx, getKey(key), value, expiration).Err()
}

func GetCache(key string) (string, error) {
	return RedisClient.Get(ctx, getKey(key)).Result()
}

func SetCacheJSON(key string, value interface{}, expiration time.Duration) error {
	data, err := sonic.Marshal(value)
	if err != nil {
		return err
	}
	return RedisClient.Set(ctx, getKey(key), data, expiration).Err()
}

func GetCacheJSON(key string, value interface{}) error {
	data, err := RedisClient.Get(ctx, getKey(key)).Result()
	if err != nil {
		return err
	}
	return sonic.Unmarshal([]byte(data), value)
}

func DelCache(key string) error {
	return RedisClient.Del(ctx, getKey(key)).Err()
}

func ExistsCache(key string) (bool, error) {
	exists, err := RedisClient.Exists(ctx, getKey(key)).Result()
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}

func SetSession(sessionID string, userID uint) error {
	return RedisClient.Set(ctx, getKey("session:"+sessionID), userID, 30*24*time.Hour).Err()
}

func GetSession(sessionID string) (uint, error) {
	val, err := RedisClient.Get(ctx, getKey("session:"+sessionID)).Uint64()
	if err == nil {
		RedisClient.Expire(ctx, getKey("session:"+sessionID), 30*24*time.Hour)
	}
	return uint(val), err
}

func DeleteSession(sessionID string) error {
	return RedisClient.Del(ctx, getKey("session:"+sessionID)).Err()
}

func ClearUserCache(userID uint) error {
	keys, err := RedisClient.Keys(ctx, getKey("user:"+fmt.Sprintf("%d:*", userID))).Result()
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return RedisClient.Del(ctx, keys...).Err()
	}
	return nil
}

func ClearContentCache(contentID uint) error {
	keys, err := RedisClient.Keys(ctx, getKey("content:"+fmt.Sprintf("%d*", contentID))).Result()
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return RedisClient.Del(ctx, keys...).Err()
	}
	return nil
}

func ClearCommentCache(contentID uint) error {
	keys, err := RedisClient.Keys(ctx, getKey("comments:"+fmt.Sprintf("%d*", contentID))).Result()
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return RedisClient.Del(ctx, keys...).Err()
	}
	return nil
}

func IncrementViewCountByDate(contentID uint, dateStr string) error {
	key := fmt.Sprintf("views:date:%s:%d", dateStr, contentID)
	err := RedisClient.Incr(ctx, getKey(key)).Err()
	if err != nil {
		return err
	}
	return RedisClient.Expire(ctx, getKey(key), 32*24*time.Hour).Err()
}

func GetViewCountInPeriod(contentID uint, days int) (int64, error) {
	var total int64
	now := time.Now()

	for i := 0; i < days; i++ {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		key := fmt.Sprintf("views:date:%s:%d", dateStr, contentID)
		count, err := RedisClient.Get(ctx, getKey(key)).Int64()
		if err == nil {
			total += count
		}
	}

	return total, nil
}

func GetAllPeriodViewCounts(contentID uint) (map[string]int64, error) {
	result := make(map[string]int64)

	count1Day, _ := GetViewCountInPeriod(contentID, 1)
	count3Day, _ := GetViewCountInPeriod(contentID, 3)
	count7Day, _ := GetViewCountInPeriod(contentID, 7)
	count1Month, _ := GetViewCountInPeriod(contentID, 30)

	result["1day"] = count1Day
	result["3day"] = count3Day
	result["7day"] = count7Day
	result["1month"] = count1Month

	return result, nil
}

func GetAllPeriodViewCountsBatch(contentIDs []uint) map[uint]map[string]int64 {
	result := make(map[uint]map[string]int64)
	if len(contentIDs) == 0 || RedisClient == nil {
		return result
	}

	now := time.Now()
	numDays := 30
	numContents := len(contentIDs)

	keys := make([]string, 0, numDays*numContents)
	positions := make(map[string][2]int)

	for day := 0; day < numDays; day++ {
		date := now.AddDate(0, 0, -day)
		dateStr := date.Format("2006-01-02")
		for contentIdx, contentID := range contentIDs {
			key := fmt.Sprintf("views:date:%s:%d", dateStr, contentID)
			realKey := getKey(key)
			keys = append(keys, realKey)
			positions[realKey] = [2]int{day, contentIdx}
		}
	}

	for _, contentID := range contentIDs {
		result[contentID] = map[string]int64{
			"1day":   0,
			"3day":   0,
			"7day":   0,
			"1month": 0,
		}
	}

	if len(keys) == 0 {
		return result
	}

	values, err := RedisClient.MGet(ctx, keys...).Result()
	if err != nil {
		log.Printf("GetAllPeriodViewCountsBatch MGet error: %v", err)
		return result
	}

	for i, value := range values {
		realKey := keys[i]
		pos, ok := positions[realKey]
		if !ok {
			continue
		}

		day := pos[0]
		contentID := contentIDs[pos[1]]

		var count int64
		if value != nil {
			switch v := value.(type) {
			case int64:
				count = v
			case int:
				count = int64(v)
			case string:
				fmt.Sscanf(v, "%d", &count)
			}
		}

		if day < 1 {
			result[contentID]["1day"] += count
		}
		if day < 3 {
			result[contentID]["3day"] += count
		}
		if day < 7 {
			result[contentID]["7day"] += count
		}
		if day < 30 {
			result[contentID]["1month"] += count
		}
	}

	return result
}

func ZAddRecommend(contentID uint, score float64) error {
	return RedisClient.ZAdd(ctx, getKey("recommend:zset"), &redis.Z{Score: score, Member: contentID}).Err()
}

func ZRevRangeRecommend(start, stop int64) ([]string, error) {
	return RedisClient.ZRevRange(ctx, getKey("recommend:zset"), start, stop).Result()
}

func ZCardRecommend() (int64, error) {
	return RedisClient.ZCard(ctx, getKey("recommend:zset")).Result()
}

func ZDelRecommend(contentID uint) error {
	return RedisClient.ZRem(ctx, getKey("recommend:zset"), contentID).Err()
}

func ZClearRecommend() error {
	return RedisClient.Del(ctx, getKey("recommend:zset")).Err()
}

func ZGetScoreRecommend(contentID uint) (float64, error) {
	return RedisClient.ZScore(ctx, getKey("recommend:zset"), fmt.Sprintf("%d", contentID)).Result()
}

func ZAddToTempRecommend(contentID uint, score float64) error {
	return RedisClient.ZAdd(ctx, getKey("recommend:zset:temp"), &redis.Z{Score: score, Member: contentID}).Err()
}

func SwapRecommendZSet() error {
	tempKey := getKey("recommend:zset:temp")
	mainKey := getKey("recommend:zset")
	return RedisClient.Rename(ctx, tempKey, mainKey).Err()
}

func DeleteTempRecommendZSet() error {
	return RedisClient.Del(ctx, getKey("recommend:zset:temp")).Err()
}

func ClearCachesOnStartup() {
	if RedisClient == nil {
		return
	}
	var cursor uint64
	deleted := 0
	for {
		keys, next, err := RedisClient.Scan(ctx, cursor, getKey("*"), 100).Result()
		if err != nil {
			log.Printf("[缓存] 扫描失败: %v", err)
			return
		}
		var toDelete []string
		for _, key := range keys {
			if strings.Contains(key, ":session:") || strings.Contains(key, ":views:") {
				continue
			}
			toDelete = append(toDelete, key)
		}
		if len(toDelete) > 0 {
			if err := RedisClient.Del(ctx, toDelete...).Err(); err != nil {
				log.Printf("[缓存] 删除失败: %v", err)
			} else {
				deleted += len(toDelete)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	log.Printf("[缓存] 启动清理完成，已清除 %d 个缓存键（保留登录态）", deleted)
}

func GetUserInfoCache(userID uint) (*models.User, error) {
	if RedisClient == nil {
		return nil, fmt.Errorf("redis not available")
	}
	var user models.User
	if err := GetCacheJSON("user_info:"+strconv.Itoa(int(userID)), &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func SetUserInfoCache(user *models.User) {
	if RedisClient == nil {
		return
	}
	SetCacheJSON("user_info:"+strconv.Itoa(int(user.ID)), user, 5*time.Minute)
}

func ClearUserInfoCache(userID uint) {
	if RedisClient == nil {
		return
	}
	DelCache("user_info:" + strconv.Itoa(int(userID)))
}

func GetKey(key string) string {
	return getKey(key)
}
