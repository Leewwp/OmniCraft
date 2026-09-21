package worker

import (
	"context"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"omnicraft/backend/internal/pkg/queue"
)

// SP-25 低-15：count_worker 对未知 action 静默 ACK 违背 inbox.go 契约
// （「unknown action 是永久失败，返回错误进 DLQ，不留完成行」）。
// 对照 review_worker 同契约。
func TestCountWorkerUnknownActionReturnsError(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	w := NewCountWorker(rdb, nil)
	msg := queue.Message{
		ID:      "1-1",
		Topic:   "count.download",
		Payload: []byte(`{"content_id":42,"action":"teleport"}`),
	}
	err := w.Handle(context.Background(), msg)
	if err == nil {
		t.Fatal("unknown action must return an error (dead-letter contract), not a silent ACK")
	}
	if !strings.Contains(err.Error(), "teleport") {
		t.Fatalf("error should name the offending action, got %v", err)
	}
}
