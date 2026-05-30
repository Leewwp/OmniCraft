package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"

	"omnicraft/backend/internal/pkg/queue"
)

type CountWorker struct {
	rdb *redis.Client
}

func NewCountWorker(rdb *redis.Client) *CountWorker {
	return &CountWorker{rdb: rdb}
}

func (w *CountWorker) Handle(ctx context.Context, msg queue.Message) error {
	var payload struct {
		ContentID int64  `json:"content_id"`
		Action    string `json:"action"`
	}

	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		slog.Error("count_worker: failed to unmarshal payload", "msg_id", msg.ID, "error", err)
		return err
	}

	slog.Info("count_worker: processing message",
		"msg_id", msg.ID, "content_id", payload.ContentID, "action", payload.Action)

	if w.rdb == nil {
		slog.Warn("count_worker: redis client is nil, skipping")
		return nil
	}

	switch payload.Action {
	case "download":
		key := "rank:download:counts"
		member := fmt.Sprintf("%d", payload.ContentID)
		if err := w.rdb.ZIncrBy(ctx, key, 1, member).Err(); err != nil {
			slog.Error("count_worker: failed to increment download count",
				"content_id", payload.ContentID, "error", err)
			return err
		}
	case "view":
		key := "rank:view:counts"
		member := fmt.Sprintf("%d", payload.ContentID)
		if err := w.rdb.ZIncrBy(ctx, key, 1, member).Err(); err != nil {
			slog.Error("count_worker: failed to increment view count",
				"content_id", payload.ContentID, "error", err)
			return err
		}
	default:
		slog.Warn("count_worker: unknown action", "action", payload.Action, "msg_id", msg.ID)
	}

	return nil
}