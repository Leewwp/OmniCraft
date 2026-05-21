package worker

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

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
				json.Unmarshal([]byte(dataStr), &entry)
			}
		}
		entries = append(entries, entry)
		count++
	}
	return entries, nil
}

func (w *DLQWorker) Replay(ctx context.Context, dlqMsgID string) error {
	if w.rdb == nil {
		return nil
	}

	streamKey := "omnicraft:dead-letter"
	messages, err := w.rdb.XRange(ctx, streamKey, dlqMsgID, dlqMsgID).Result()
	if err != nil {
		return err
	}
	if len(messages) == 0 {
		return nil
	}

	msg := messages[0]
	data, ok := msg.Values["data"]
	if !ok {
		return nil
	}

	var entry DLQEntry
	dataStr, _ := data.(string)
	if err := json.Unmarshal([]byte(dataStr), &entry); err != nil {
		return err
	}

	if entry.OriginalTopic == "" {
		return nil
	}

	slog.Info("dlq_worker: replaying message to original topic",
		"dlq_msg_id", dlqMsgID, "original_topic", entry.OriginalTopic)

	targetStream := "omnicraft:" + entry.OriginalTopic
	return w.rdb.XAdd(ctx, &redis.XAddArgs{
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
	Payload       string `json:"payload"`
	Attempts      int    `json:"attempts"`
	Error         string `json:"error"`
	FailedAt      string `json:"failed_at"`
}