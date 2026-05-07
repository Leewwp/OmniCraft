package redisclient

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"time"
)

func SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal cache value: %w", err)
	}
	return Client.Set(ctx, key, data, ttl).Err()
}

func GetJSON(ctx context.Context, key string, dest interface{}) (bool, error) {
	data, err := Client.Get(ctx, key).Bytes()
	if err != nil {
		return false, nil
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return false, fmt.Errorf("unmarshal cache value: %w", err)
	}
	return true, nil
}

func DeleteByPattern(ctx context.Context, pattern string) error {
	var cursor uint64
	for {
		keys, nextCursor, err := Client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			Client.Del(ctx, keys...)
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
