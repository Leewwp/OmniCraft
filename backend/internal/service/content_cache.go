package service

import (
	"context"
	"fmt"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/events"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
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

// EmitContentStatusEventTx writes one content outbox event inside the
// caller's transaction. Shared by the review channel, admin restore and the
// judge close-case write-back so every producer emits the same envelope
// shape; a nil outbox (unwired seam) is a no-op for backwards compatibility.
func EmitContentStatusEventTx(ctx context.Context, tx *gorm.DB, outbox repository.OutboxWriter, topic string, content *model.ContentItem, status string) error {
	if outbox == nil || content == nil {
		return nil
	}
	traceparent, tracestate := events.FromContext(ctx)
	env, err := events.NewContentEnvelope(topic, content.ID, traceparent, tracestate,
		events.ContentEventPayload{
			ContentID:   content.ID,
			AuthorID:    content.AuthorID,
			ContentType: content.ContentType,
			Status:      status,
		})
	if err != nil {
		return err
	}
	row := events.ToOutboxEvent(env)
	return outbox.CreateTx(ctx, tx, &row)
}
