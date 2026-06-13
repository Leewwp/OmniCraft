package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestPublishAndSubscribe(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	cfg := &QueueConfig{
		Enabled:         true,
		MaxAttempts:     3,
		RetryBackoffSec: []int{1, 2, 3},
		MaxLen:          100000,
	}

	broker := NewRedisStreamBroker(rdb, cfg)
	defer broker.Stop()

	received := make(chan Message, 1)
	handler := func(ctx context.Context, msg Message) error {
		received <- msg
		return nil
	}

	err := broker.Subscribe(context.Background(), "test.topic", "omnicraft-test", handler)
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	payload, _ := json.Marshal(map[string]string{"key": "value"})
	err = broker.Publish(context.Background(), "test.topic", payload)
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	select {
	case msg := <-received:
		if msg.Topic != "test.topic" {
			t.Errorf("expected topic test.topic, got %s", msg.Topic)
		}
		var data map[string]string
		if err := json.Unmarshal(msg.Payload, &data); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if data["key"] != "value" {
			t.Errorf("expected key=value, got %s", data["key"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestRetryAndDLQ(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	cfg := &QueueConfig{
		Enabled:         true,
		MaxAttempts:     2,
		RetryBackoffSec: []int{0, 0},
		MaxLen:          100000,
	}

	broker := NewRedisStreamBroker(rdb, cfg)
	defer broker.Stop()

	attempts := 0
	handler := func(ctx context.Context, msg Message) error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("simulated failure")
		}
		return nil
	}

	_ = broker.Subscribe(context.Background(), "test.retry", "omnicraft-retry", handler)

	payload, _ := json.Marshal(map[string]string{"test": "retry"})
	_ = broker.Publish(context.Background(), "test.retry", payload)

	time.Sleep(2 * time.Second)

	if attempts < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attempts)
	}
}

func TestRetryAndDLQ_PayloadAndMetadataPreserved(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	cfg := &QueueConfig{
		Enabled:         true,
		MaxAttempts:     2,
		RetryBackoffSec: []int{0, 0},
		MaxLen:          100000,
	}

	broker := NewRedisStreamBroker(rdb, cfg)
	defer broker.Stop()

	// Handler that always fails so retries exhaust
	handler := func(ctx context.Context, msg Message) error {
		return fmt.Errorf("permanent failure")
	}

	topic := "test.dlq-payload"
	group := "omnicraft-dlq-test"

	err := broker.Subscribe(context.Background(), topic, group, handler)
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// Publish a message with a known payload
	originalPayload, _ := json.Marshal(map[string]string{"order": "abc-123", "action": "process"})
	err = broker.Publish(context.Background(), topic, originalPayload)
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	// Wait for retries to exhaust and DLQ write
	time.Sleep(3 * time.Second)

	// 1. Read from the DLQ stream
	dlqKey := "omnicraft:dead-letter"
	dlqEntries, err := rdb.XRange(context.Background(), dlqKey, "-", "+").Result()
	if err != nil {
		t.Fatalf("reading DLQ stream: %v", err)
	}
	if len(dlqEntries) == 0 {
		t.Fatal("expected at least one entry in DLQ, got none")
	}

	// 2. Verify DLQ entry contents
	latestEntry := dlqEntries[len(dlqEntries)-1]
	dataStr, ok := latestEntry.Values["data"].(string)
	if !ok {
		t.Fatal("DLQ entry missing 'data' field")
	}

	var dlqData map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &dlqData); err != nil {
		t.Fatalf("unmarshal DLQ data: %v", err)
	}

	// Verify original payload
	payloadStr, _ := dlqData["payload"].(string)
	if payloadStr != string(originalPayload) {
		t.Errorf("DLQ payload mismatch: expected %s, got %s", string(originalPayload), payloadStr)
	}
	var payloadParsed map[string]string
	if err := json.Unmarshal([]byte(payloadStr), &payloadParsed); err != nil {
		t.Fatalf("unmarshal DLQ payload inner: %v", err)
	}
	if payloadParsed["order"] != "abc-123" || payloadParsed["action"] != "process" {
		t.Errorf("DLQ payload content mismatch: got %v", payloadParsed)
	}

	// Verify original topic name
	originalTopic, _ := dlqData["original_topic"].(string)
	if originalTopic != topic {
		t.Errorf("DLQ original_topic mismatch: expected %s, got %s", topic, originalTopic)
	}

	// Verify error message
	errMsg, _ := dlqData["error"].(string)
	if errMsg != "permanent failure" {
		t.Errorf("DLQ error mismatch: expected 'permanent failure', got %s", errMsg)
	}

	// Verify failed_at timestamp exists and is parseable
	failedAt, _ := dlqData["failed_at"].(string)
	if failedAt == "" {
		t.Error("DLQ entry missing failed_at timestamp")
	}
	if _, err := time.Parse(time.RFC3339, failedAt); err != nil {
		t.Errorf("DLQ failed_at is not a valid RFC3339 timestamp: %s", failedAt)
	}

	// Verify attempts count (loop runs 0..maxAttempts-1, so total = maxAttempts)
	expectedAttempts := cfg.MaxAttempts
	attempts, _ := dlqData["attempts"].(float64)
	if attempts != float64(expectedAttempts) {
		t.Errorf("DLQ attempts mismatch: expected %d, got %v", expectedAttempts, attempts)
	}

	// 3. Verify original stream has no pending messages for the consumer group
	streamKey := fmt.Sprintf("omnicraft:%s", topic)
	groups, err := rdb.XInfoGroups(context.Background(), streamKey).Result()
	if err != nil {
		t.Fatalf("xinfo groups: %v", err)
	}
	for _, g := range groups {
		if g.Name == group && g.Pending > 0 {
			t.Errorf("expected 0 pending messages in consumer group %s, got %d", group, g.Pending)
		}
	}
}

func TestIdempotentCheck(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	ctx := context.Background()

	ok, err := IdempotentCheck(ctx, rdb, "test.topic", "msg-1")
	if err != nil {
		t.Fatalf("idempotent check failed: %v", err)
	}
	if !ok {
		t.Error("first check should return true")
	}

	ok, err = IdempotentCheck(ctx, rdb, "test.topic", "msg-1")
	if err != nil {
		t.Fatalf("second idempotent check failed: %v", err)
	}
	if ok {
		t.Error("second check should return false (duplicate)")
	}

	ok, err = IdempotentCheck(ctx, rdb, "test.topic", "msg-2")
	if err != nil {
		t.Fatalf("different msg id check failed: %v", err)
	}
	if !ok {
		t.Error("different msg id should return true")
	}
}

func TestNoopProducer(t *testing.T) {
	noop := NewNoopProducer()
	err := noop.Publish(context.Background(), "test.topic", []byte("payload"))
	if err != nil {
		t.Errorf("noop producer should not return error, got %v", err)
	}
}