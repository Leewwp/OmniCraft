package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// T28（FIX-35 / F-113）：DLQ 条目 JSON payload 里的 id 在写入 stream 前必然
// 为空（写入方尚不知道 msg id）——Consume 读出后必须以 stream msg id 为准，
// 否则 GET /admin/queue/dlq 返回 id=""，replay UI 拿空 id 永远失败。
func TestDLQConsumeKeepsStreamMsgIDOverStalePayloadID(t *testing.T) {
	mr := miniredis.RunT(t)
	w := NewDLQWorker(redis.NewClient(&redis.Options{Addr: mr.Addr()}))

	payload := map[string]any{"user_id": 2}
	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)
	entryJSON, err := json.Marshal(DLQEntry{
		OriginalTopic: "notification.create",
		OriginalID:    "orig-1",
		ConsumerGroup: "worker-main",
		Payload:       string(payloadJSON),
		Attempts:      3,
		Error:         "simulated failure",
		FailedAt:      "2026-09-05T00:00:00Z",
		// 注意：不设 ID——与真实写入方一致（入 stream 前无 msg id），
		// json.Unmarshal 会得到 "id":""。
	})
	require.NoError(t, err)
	_, err = mr.XAdd("omnicraft:dead-letter", "1-1", []string{"data", string(entryJSON)})
	require.NoError(t, err)

	entries, err := w.Consume(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.NotEmpty(t, entries[0].ID, "consumed entry must carry the stream msg id")
	require.Regexp(t, `^\d+-\d+$`, entries[0].ID)
	require.Equal(t, "notification.create", entries[0].OriginalTopic)
	require.Equal(t, 3, entries[0].Attempts)
}

// Replay：按 msg id 找到条目 → 重投回原 topic stream（不删原条目）。
func TestDLQReplayRedeliversWithoutDeletingSource(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	w := NewDLQWorker(redis.NewClient(&redis.Options{Addr: mr.Addr()}))

	entryJSON, err := json.Marshal(DLQEntry{
		OriginalTopic: "notification.create",
		OriginalID:    "orig-1",
		Payload:       `{"user_id":2}`,
		Attempts:      2,
		Error:         "boom",
	})
	require.NoError(t, err)
	_, err = mr.XAdd("omnicraft:dead-letter", "1-1", []string{"data", string(entryJSON)})
	require.NoError(t, err)

	entries, err := w.Consume(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	msgID := entries[0].ID

	replayed, err := w.Replay(context.Background(), msgID)
	require.NoError(t, err)
	require.Equal(t, "notification.create", replayed.OriginalTopic)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { require.NoError(t, rdb.Close()) }()

	// 重投目标 stream 收到新消息。
	target, err := rdb.XRange(ctx, "omnicraft:notification.create", "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, target, 1, "replay must deliver one message to the original topic")

	// 原条目仍在 DLQ（重投不删）。
	source, err := rdb.XRange(ctx, "omnicraft:dead-letter", "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, source, 1, "replay must not delete the source DLQ entry")

	// 不存在的 id → ErrDLQEntryNotFound。
	_, err = w.Replay(context.Background(), "9999999999999-0")
	require.ErrorIs(t, err, ErrDLQEntryNotFound)
}
