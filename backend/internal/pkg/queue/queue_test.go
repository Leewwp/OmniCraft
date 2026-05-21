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