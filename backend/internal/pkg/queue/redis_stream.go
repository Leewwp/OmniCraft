package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"omnicraft/backend/internal/pkg/recovery"

	"github.com/redis/go-redis/v9"
)

type RedisStreamBroker struct {
	rdb      *redis.Client
	cfg      *QueueConfig
	stopped  chan struct{}
}

func NewRedisStreamBroker(rdb *redis.Client, cfg *QueueConfig) *RedisStreamBroker {
	return &RedisStreamBroker{
		rdb:     rdb,
		cfg:     cfg,
		stopped: make(chan struct{}),
	}
}

func (b *RedisStreamBroker) Publish(ctx context.Context, topic string, payload []byte) error {
	streamKey := streamKey(topic)
	msg := map[string]interface{}{
		"payload":   string(payload),
		"timestamp": time.Now().UnixMilli(),
	}
	id, err := b.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		MaxLen: int64(b.cfg.MaxLen),
		Approx: true,
		Values: msg,
	}).Result()
	if err != nil {
		return fmt.Errorf("xadd to %s: %w", streamKey, err)
	}
	logQueueEvent("publish", topic, id, 0)
	return nil
}

func (b *RedisStreamBroker) Subscribe(ctx context.Context, topic string, group string, handler Handler) error {
	streamKey := streamKey(topic)
	consumerName := fmt.Sprintf("%s-worker", group)

	if err := b.ensureGroup(ctx, streamKey, group); err != nil {
		return fmt.Errorf("ensure group %s for %s: %w", group, streamKey, err)
	}

	recovery.GoSafe(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-b.stopped:
				return
			default:
			}

			results, err := b.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    group,
				Consumer: consumerName,
				Streams:  []string{streamKey, ">"},
				Count:    10,
				Block:    2 * time.Second,
			}).Result()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Error("xreadgroup error", "topic", topic, "group", group, "error", err)
				time.Sleep(time.Second)
				continue
			}

			for _, stream := range results {
				for _, xmsg := range stream.Messages {
					msg := b.decodeMessage(topic, xmsg)
					b.handleMessage(ctx, topic, group, msg, handler)
				}
			}
		}
	})

	return nil
}

func (b *RedisStreamBroker) handleMessage(ctx context.Context, topic, group string, msg Message, handler Handler) {
	retryBackoff := b.cfg.RetryBackoffSec
	maxAttempts := b.cfg.MaxAttempts

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		msg.Attempts = attempt + 1
		logQueueEvent("consume_start", topic, msg.ID, attempt)
		if err := handler(ctx, msg); err != nil {
			lastErr = err
			slog.Warn("handler failed, will retry",
				"topic", topic, "msg_id", msg.ID, "attempt", attempt, "error", err)
			if attempt < maxAttempts {
				wait := time.Duration(retryBackoff[attempt%len(retryBackoff)]) * time.Second
				select {
				case <-time.After(wait):
				case <-ctx.Done():
					return
				}
			}
			continue
		}

		b.rdb.XAck(ctx, streamKey(topic), group, msg.ID)
		logQueueEvent("consume_success", topic, msg.ID, attempt)
		return
	}

	slog.Error("handler exhausted retries, sending to DLQ",
		"topic", topic, "msg_id", msg.ID, "attempts", msg.Attempts, "last_error", lastErr)
	b.sendToDLQ(ctx, msg, lastErr)
	b.rdb.XAck(ctx, streamKey(topic), group, msg.ID)
}

func (b *RedisStreamBroker) sendToDLQ(ctx context.Context, msg Message, err error) {
	dlqKey := "omnicraft:dead-letter"
	payload, marshalErr := json.Marshal(map[string]interface{}{
		"original_topic": msg.Topic,
		"original_id":    msg.ID,
		"payload":        string(msg.Payload),
		"metadata":       msg.Metadata,
		"attempts":       msg.Attempts,
		"error":          err.Error(),
		"failed_at":      time.Now().Format(time.RFC3339),
	})
	if marshalErr != nil {
		slog.Error("failed to marshal DLQ payload", "topic", msg.Topic, "error", marshalErr)
		return
	}
	if _, xaddErr := b.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: dlqKey,
		MaxLen: 10000,
		Approx: true,
		Values: map[string]interface{}{
			"data": string(payload),
		},
	}).Result(); xaddErr != nil {
		slog.Error("failed to write message to DLQ",
			"topic", msg.Topic,
			"msg_id", msg.ID,
			"error", xaddErr)
	}
}

func (b *RedisStreamBroker) ensureGroup(ctx context.Context, streamKey, group string) error {
	err := b.rdb.XGroupCreateMkStream(ctx, streamKey, group, "0").Err()
	if err != nil {
		errStr := err.Error()
		if len(errStr) >= 8 && errStr[:8] == "BUSYGROUP" {
			return nil
		}
	}
	return err
}

func (b *RedisStreamBroker) decodeMessage(topic string, xmsg redis.XMessage) Message {
	payloadStr, _ := xmsg.Values["payload"].(string)
	return Message{
		ID:        xmsg.ID,
		Topic:     topic,
		Payload:   []byte(payloadStr),
		Metadata:  map[string]string{},
		Attempts:  0,
		CreatedAt: time.Now(),
	}
}

func (b *RedisStreamBroker) Close() error {
	return nil
}

func (b *RedisStreamBroker) Stop() {
	close(b.stopped)
}

func streamKey(topic string) string {
	return fmt.Sprintf("omnicraft:%s", topic)
}