package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var idempotentTTL = 24 * time.Hour

func IdempotentCheck(ctx context.Context, rdb *redis.Client, topic, msgID string) (bool, error) {
	key := fmt.Sprintf("queue:processed:%s:%s", topic, msgID)
	ok, err := rdb.SetNX(ctx, key, "1", idempotentTTL).Result()
	if err != nil {
		return false, fmt.Errorf("idempotent check failed: %w", err)
	}
	return ok, nil
}