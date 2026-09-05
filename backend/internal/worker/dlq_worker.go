package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

// ErrDLQEntryNotFound marks a replay request for a stream id that has no
// dead-letter entry (or whose entry is not replayable).
var ErrDLQEntryNotFound = errors.New("dlq entry not found")

type DLQWorker struct {
	rdb *redis.Client
}

func NewDLQWorker(rdb *redis.Client) *DLQWorker {
	return &DLQWorker{rdb: rdb}
}

func (w *DLQWorker) Consume(ctx context.Context, limit int64) ([]DLQEntry, error) {
	if w.rdb == nil {
		return nil, nil
	}

	streamKey := "omnicraft:dead-letter"
	messages, err := w.rdb.XRevRange(ctx, streamKey, "+", "-").Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var entries []DLQEntry
	count := int64(0)
	for _, msg := range messages {
		if count >= limit {
			break
		}
		entry := DLQEntry{ID: msg.ID}
		if data, ok := msg.Values["data"]; ok {
			if dataStr, ok := data.(string); ok {
				if json.Unmarshal([]byte(dataStr), &entry) == nil {
					// T28（FIX-35 / F-113）：payload 里的 id 在入 stream 前必然
					// 为空（写入方尚不知道 msg id），unmarshal 不得覆盖 stream id，
					// 否则 replay UI 拿到空 id 永远失败。
					entry.ID = msg.ID
				}
			}
		}
		entries = append(entries, entry)
		count++
	}
	return entries, nil
}

func (w *DLQWorker) Replay(ctx context.Context, dlqMsgID string) (DLQEntry, error) {
	if w.rdb == nil {
		return DLQEntry{}, ErrDLQEntryNotFound
	}

	streamKey := "omnicraft:dead-letter"
	messages, err := w.rdb.XRange(ctx, streamKey, dlqMsgID, dlqMsgID).Result()
	if err != nil {
		return DLQEntry{}, err
	}
	if len(messages) == 0 {
		return DLQEntry{}, ErrDLQEntryNotFound
	}

	msg := messages[0]
	data, ok := msg.Values["data"]
	if !ok {
		return DLQEntry{}, ErrDLQEntryNotFound
	}

	var entry DLQEntry
	dataStr, _ := data.(string)
	if err := json.Unmarshal([]byte(dataStr), &entry); err != nil {
		return DLQEntry{}, err
	}

	if entry.OriginalTopic == "" {
		return DLQEntry{}, ErrDLQEntryNotFound
	}
	entry.ID = dlqMsgID

	slog.Info("dlq_worker: replaying message to original topic",
		"dlq_msg_id", dlqMsgID, "original_topic", entry.OriginalTopic, "consumer_group", entry.ConsumerGroup)

	targetStream := "omnicraft:" + entry.OriginalTopic
	return entry, w.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: targetStream,
		MaxLen: 100000,
		Approx: true,
		Values: map[string]interface{}{
			"payload":   entry.Payload,
			"timestamp": msg.Values["timestamp"],
		},
	}).Err()
}

type DLQEntry struct {
	ID            string `json:"id"`
	OriginalTopic string `json:"original_topic"`
	OriginalID    string `json:"original_id"`
	ConsumerGroup string `json:"consumer_group,omitempty"`
	Payload       string `json:"payload"`
	Attempts      int    `json:"attempts"`
	Error         string `json:"error"`
	FailedAt      string `json:"failed_at"`
}
