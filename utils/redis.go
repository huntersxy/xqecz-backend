package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"xiaoquan-backend/config"

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
	data, err := json.Marshal(value)
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
	return json.Unmarshal([]byte(data), value)
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
	return RedisClient.Set(ctx, getKey("session:"+sessionID), userID, 24*time.Hour).Err()
}

func GetSession(sessionID string) (uint, error) {
	val, err := RedisClient.Get(ctx, getKey("session:"+sessionID)).Uint64()
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
