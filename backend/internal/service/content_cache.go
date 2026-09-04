package service

import (
	"context"
	"fmt"

	redisclient "omnicraft/backend/internal/pkg/redis"

	"github.com/redis/go-redis/v9"
)

// InvalidateContentCaches drops the cached public detail row and the list
// cache keys for the given contents. Moderation write paths that bypass
// ContentService (admin ban/restore, appeal approval, report auto-hide) call
// this so decisions take effect immediately instead of after the TTL. A nil
// rdb (local stack without redis) is tolerated as a no-op.
func InvalidateContentCaches(rdb *redis.Client, ids ...int64) {
	if rdb == nil || len(ids) == 0 {
		return
	}
	ctx := context.Background()
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, fmt.Sprintf("cache:content:%d", id))
	}
	rdb.Del(ctx, keys...)
	redisclient.DeleteByPattern(ctx, "cache:content:list:*")
}
