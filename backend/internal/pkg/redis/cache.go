package redisclient

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

func SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal cache value: %w", err)
	}
	return Client.Set(ctx, key, data, ttl).Err()
}

func GetJSON(ctx context.Context, key string, dest interface{}) (bool, error) {
	// SP-25 低-33：nil 守卫对称（SetJSON 路径也依赖 Client，DeleteByPattern
	// 已守卫）；Redis 故障吞错当 miss 保留降级语义，但必须留 WARN——
	// 否则缓存层整体宕机与「冷缓存」不可区分。
	if Client == nil {
		return false, nil
	}
	data, err := Client.Get(ctx, key).Bytes()
	if err != nil {
		if err != redis.Nil {
			slog.WarnContext(ctx, "cache get failed, degrading to miss",
				"key", key, "error", err)
		}
		return false, nil
	}
	if err := json.Unmarshal(data, dest); err != nil {
		slog.WarnContext(ctx, "cache value unmarshal failed, degrading to miss",
			"key", key, "error", err)
		return false, fmt.Errorf("unmarshal cache value: %w", err)
	}
	return true, nil
}

func DeleteByPattern(ctx context.Context, pattern string) error {
	if Client == nil {
		return nil
	}
	var cursor uint64
	for {
		keys, nextCursor, err := Client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			pipe := Client.Pipeline()
			for _, key := range keys {
				pipe.Unlink(ctx, key)
			}
			_, err := pipe.Exec(ctx)
			if err != nil {
				return err
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

func ListCacheKey(prefix string, filter interface{}) string {
	data, _ := json.Marshal(filter)
	hash := md5.Sum(data)
	return fmt.Sprintf("cache:%s:list:%x", prefix, hash)
}

func ClearRecCache(ctx context.Context, userID int64) {
	if Client == nil {
		return
	}
	DeleteByPattern(ctx, fmt.Sprintf("rec:original:%d:*", userID))
}
