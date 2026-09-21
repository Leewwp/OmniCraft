package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"omnicraft/backend/internal/pkg/recovery"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// stuckPendingMinIdle is the XAUTOCLAIM idle threshold for reclaiming
// messages stranded in the PEL by a crashed consumer (at-least-once delivery
// needs a redelivery path for the crash window between XREADGROUP and XACK).
const stuckPendingMinIdle = 5 * time.Minute

// reclaimInterval is how often each subscription sweeps the PEL for stuck
// messages (the startup sweep runs immediately).
const reclaimInterval = time.Minute

// backlogObserveInterval throttles the XINFO GROUPS backlog observation that
// used to fire on every read-loop iteration (2s block × per-goroutine).
const backlogObserveInterval = 30 * time.Second

type RedisStreamBroker struct {
	rdb     *redis.Client
	cfg     *QueueConfig
	stopped chan struct{}
	backlog backlogThrottle
}

func NewRedisStreamBroker(rdb *redis.Client, cfg *QueueConfig) *RedisStreamBroker {
	return &RedisStreamBroker{
		rdb:     rdb,
		cfg:     cfg,
		stopped: make(chan struct{}),
		backlog: newBacklogThrottle(backlogObserveInterval),
	}
}

func (b *RedisStreamBroker) Publish(ctx context.Context, topic string, payload []byte) error {
	streamKey := streamKey(topic)
	spanCtx, span := otel.Tracer("omnicraft/queue").Start(ctx, "queue.publish", oteltrace.WithSpanKind(oteltrace.SpanKindProducer))
	defer span.End()
	span.SetAttributes(attribute.String("messaging.system", "redis"), attribute.String("messaging.destination.name", topic))
	msg := map[string]interface{}{
		"payload":   string(payload),
		"timestamp": time.Now().UnixMilli(),
	}
	metadata := make(map[string]string, 2)
	InjectTraceContext(spanCtx, metadata)
	for key, value := range metadata {
		msg[key] = value
	}
	id, err := b.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		MaxLen: int64(b.cfg.MaxLen),
		Approx: true,
		Values: msg,
	}).Result()
	if err != nil {
		span.RecordError(err)
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

	// SP-25 中-8：滞留 PEL 回收（崩溃窗口 at-least-once 补齐）——启动即扫
	// 一轮，此后按 reclaimInterval 周期扫描。同一批消息可能与读循环并发
	// 投递，worker 侧经 inbox 幂等记录保证 at-most-once 应用效果。
	recovery.GoSafe(func() {
		b.reclaimStuckPending(ctx, streamKey, group, consumerName, stuckPendingMinIdle, handler)
		ticker := time.NewTicker(reclaimInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-b.stopped:
				return
			case <-ticker.C:
				b.reclaimStuckPending(ctx, streamKey, group, consumerName, stuckPendingMinIdle, handler)
			}
		}
	})

	recovery.GoSafe(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-b.stopped:
				return
			default:
			}

			b.observeGroupBacklog(ctx, streamKey, group)
			readCtx, readSpan := otel.Tracer("omnicraft/queue").Start(ctx, "queue.consume", oteltrace.WithSpanKind(oteltrace.SpanKindConsumer))
			readSpan.SetAttributes(attribute.String("messaging.system", "redis"), attribute.String("messaging.destination.name", topic))
			results, err := b.rdb.XReadGroup(readCtx, &redis.XReadGroupArgs{
				Group:    group,
				Consumer: consumerName,
				Streams:  []string{streamKey, ">"},
				Count:    10,
				Block:    2 * time.Second,
			}).Result()
			readSpan.End()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				// XREADGROUP returns redis.Nil when its BLOCK timeout
				// expires without a message. An idle consumer is healthy;
				// continue the loop without emitting an error log.
				if errors.Is(err, redis.Nil) {
					continue
				}
				// The group can disappear under a running consumer (operator
				// FLUSHDB, manual stream cleanup, TTL expiry). Re-create it
				// from the head instead of error-spinning on NOGROUP forever;
				// the group resumes at the oldest message, so nothing written
				// while it was missing is lost (at-least-once).
				if strings.HasPrefix(err.Error(), noGroupPrefix) {
					if ensureErr := b.ensureGroup(ctx, streamKey, group); ensureErr != nil {
						slog.Error("failed to re-create consumer group", "topic", topic, "group", group, "error", ensureErr)
					}
				}
				slog.Error("xreadgroup error", "topic", topic, "group", group, "error", err)
				time.Sleep(time.Second)
				continue
			}
			for _, stream := range results {
				for _, xmsg := range stream.Messages {
					// Re-check shutdown after the blocking read returned: a
					// message that arrived during Stop() must not reach the
					// handler (it stays pending and is redelivered on the
					// next start, at-least-once).
					select {
					case <-ctx.Done():
						return
					case <-b.stopped:
						return
					default:
					}
					msg := b.decodeMessage(topic, xmsg)
					msg.Group = group
					messageCtx, span := otel.Tracer("omnicraft/queue").Start(
						ExtractTraceContext(ctx, msg.Metadata), "queue.consume",
						oteltrace.WithSpanKind(oteltrace.SpanKindConsumer),
					)
					span.SetName("queue.process")
					span.SetAttributes(attribute.String("messaging.system", "redis"), attribute.String("messaging.destination.name", topic), attribute.String("messaging.message.id", msg.ID))
					b.handleMessage(messageCtx, topic, group, msg, handler)
					span.End()
				}
			}
		}
	})

	return nil
}

func (b *RedisStreamBroker) observeGroupBacklog(ctx context.Context, streamKey, group string) {
	if !b.backlog.allow(time.Now()) {
		return
	}
	groups, err := b.rdb.XInfoGroups(ctx, streamKey).Result()
	if err != nil {
		return
	}
	for _, info := range groups {
		if info.Name != group {
			continue
		}
		backlog := info.Pending
		if info.Lag > 0 {
			backlog += info.Lag
		}
		observeQueueBacklog(float64(backlog))
		return
	}
}

// reclaimStuckPending sweeps the group's PEL via XAUTOCLAIM: entries idle
// longer than minIdle (claimed but never ACKed — the crashed-consumer window)
// are re-claimed by this consumer and pushed through the same handler path.
// minIdle is a parameter so tests can reclaim immediately (miniredis does not
// advance stream idle clocks).
func (b *RedisStreamBroker) reclaimStuckPending(ctx context.Context, streamKey, group, consumerName string, minIdle time.Duration, handler Handler) {
	cursor := "0-0"
	for {
		result, next, err := b.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   streamKey,
			Group:    group,
			Consumer: consumerName,
			MinIdle:  minIdle,
			Start:    cursor,
			Count:    10,
		}).Result()
		if err != nil {
			if ctx.Err() == nil && !strings.HasPrefix(err.Error(), noGroupPrefix) {
				slog.Warn("xautoclaim failed", "stream", streamKey, "group", group, "error", err)
			}
			return
		}
		for _, xmsg := range result {
			select {
			case <-ctx.Done():
				return
			case <-b.stopped:
				return
			default:
			}
			msg := b.decodeMessage(strings.TrimPrefix(streamKey, "omnicraft:"), xmsg)
			msg.Group = group
			slog.Warn("reclaiming stuck pending message", "topic", msg.Topic, "group", group, "msg_id", msg.ID)
			b.handleMessage(ctx, msg.Topic, group, msg, handler)
		}
		if next == "0-0" || len(result) == 0 {
			return
		}
		cursor = next
	}
}

func (b *RedisStreamBroker) handleMessage(ctx context.Context, topic, group string, msg Message, handler Handler) {
	// 中-8：handler panic（毒丸消息）必须被 recover——消费 goroutine 不死，
	// 消息进 DLQ 并 ACK（与重试耗尽同一条出路），后续消息继续消费。
	defer func() {
		if r := recover(); r != nil {
			slog.Error("queue handler panicked, dead-lettering message",
				"topic", topic, "msg_id", msg.ID, "panic", r, "stack", string(debug.Stack()))
			observeWorkerFailure()
			b.sendToDLQ(ctx, msg, group, fmt.Errorf("handler panic: %v", r))
			b.ackLogged(ctx, topic, group, msg.ID)
		}
	}()

	// 低-31：经 RetryBackoff() 兜底（空表走 DefaultRetryBackoffSec），
	// 不再直接取原始配置——空切片取模曾直接 panic。
	retryBackoff := RetryBackoff(b.cfg)
	maxAttempts := b.cfg.MaxAttempts

	// A non-positive attempt budget means the message can never be processed
	// (misconfiguration, e.g. max_attempts zeroed by an override bug). Never
	// attempt the handler, never send to the DLQ with a nil error: ACK so the
	// message does not linger pending, and log loudly for the operator.
	if maxAttempts <= 0 {
		slog.Error("queue max_attempts is not positive, acknowledging without processing",
			"topic", topic, "msg_id", msg.ID, "max_attempts", maxAttempts)
		observeWorkerFailure()
		b.ackLogged(ctx, topic, group, msg.ID)
		return
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		msg.Attempts = attempt + 1
		logQueueEvent("consume_start", topic, msg.ID, attempt)
		if err := handler(ctx, msg); err != nil {
			lastErr = err
			slog.Warn("handler failed, will retry",
				"topic", topic, "msg_id", msg.ID, "attempt", attempt, "error", err)
			// 末轮（最后一Attempt 失败后即进 DLQ）不再退避——旧守卫
			// attempt < maxAttempts 在循环内恒真，白等一轮。
			if wait := backoffDelay(retryBackoff, attempt, maxAttempts); wait > 0 {
				select {
				case <-time.After(wait):
				case <-ctx.Done():
					return
				}
			}
			continue
		}

		b.ackLogged(ctx, topic, group, msg.ID)
		logQueueEvent("consume_success", topic, msg.ID, attempt)
		return
	}

	slog.Error("handler exhausted retries, sending to DLQ",
		"topic", topic, "msg_id", msg.ID, "attempts", msg.Attempts, "last_error", lastErr)
	observeWorkerFailure()
	b.sendToDLQ(ctx, msg, group, lastErr)
	b.ackLogged(ctx, topic, group, msg.ID)
}

// backoffDelay decides the sleep before retry `attempt+1` of maxAttempts:
// the final attempt gets none (the loop is about to dead-letter), an empty
// table falls back to DefaultRetryBackoffSec, and a short table wraps around.
func backoffDelay(retryBackoff []int, attempt, maxAttempts int) time.Duration {
	if attempt >= maxAttempts-1 {
		return 0
	}
	if len(retryBackoff) == 0 {
		retryBackoff = DefaultRetryBackoffSec
	}
	sec := retryBackoff[attempt%len(retryBackoff)]
	if sec < 0 {
		sec = 0
	}
	return time.Duration(sec) * time.Second
}

// ackLogged ACKs and surfaces failures: a lost ACK keeps the message in the
// PEL for redelivery (at-least-once), so the operator should see why.
func (b *RedisStreamBroker) ackLogged(ctx context.Context, topic, group, msgID string) {
	if err := b.rdb.XAck(ctx, streamKey(topic), group, msgID).Err(); err != nil {
		slog.Warn("xack failed; message stays pending and will be redelivered (handlers are idempotent via inbox)",
			"topic", topic, "msg_id", msgID, "error", err)
	}
}

// backlogThrottle gates the periodic XINFO GROUPS observation so it fires at
// most once per interval instead of on every read-loop wakeup.
type backlogThrottle struct {
	mu       sync.Mutex
	interval time.Duration
	last     time.Time
}

func newBacklogThrottle(interval time.Duration) backlogThrottle {
	return backlogThrottle{interval: interval}
}

func (t *backlogThrottle) allow(now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.last.IsZero() && now.Sub(t.last) < t.interval {
		return false
	}
	t.last = now
	return true
}

func (b *RedisStreamBroker) sendToDLQ(ctx context.Context, msg Message, group string, err error) {
	dlqKey := "omnicraft:dead-letter"
	errStr := "unknown error"
	if err != nil {
		errStr = err.Error()
	}
	payload, marshalErr := json.Marshal(map[string]interface{}{
		"original_topic": msg.Topic,
		"original_id":    msg.ID,
		"consumer_group": group,
		"payload":        string(msg.Payload),
		"metadata":       msg.Metadata,
		"attempts":       msg.Attempts,
		"error":          errStr,
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

const busyGroupPrefix = "BUSYGROUP"

const noGroupPrefix = "NOGROUP"

func (b *RedisStreamBroker) ensureGroup(ctx context.Context, streamKey, group string) error {
	err := b.rdb.XGroupCreateMkStream(ctx, streamKey, group, "0").Err()
	if err != nil {
		errStr := err.Error()
		// The group already exists (second consumer of the same topic, e.g.
		// worker.concurrency > 1): that is a normal no-op, not a failure.
		if len(errStr) >= len(busyGroupPrefix) && errStr[:len(busyGroupPrefix)] == busyGroupPrefix {
			return nil
		}
	}
	return err
}

func (b *RedisStreamBroker) decodeMessage(topic string, xmsg redis.XMessage) Message {
	payloadStr, _ := xmsg.Values["payload"].(string)
	metadata := make(map[string]string, 2)
	for _, key := range []string{"traceparent", "tracestate"} {
		if value, ok := xmsg.Values[key].(string); ok && value != "" {
			metadata[key] = value
		}
	}
	return Message{
		ID:        xmsg.ID,
		Topic:     topic,
		Payload:   []byte(payloadStr),
		Metadata:  metadata,
		Attempts:  0,
		CreatedAt: time.Now(),
	}
}

// InjectTraceContext serializes the W3C trace context into queue metadata.
// Only propagation fields are copied; payloads and prompts never enter the
// carrier.
func InjectTraceContext(ctx context.Context, metadata map[string]string) {
	if metadata == nil {
		return
	}
	propagation.TraceContext{}.Inject(ctx, propagation.MapCarrier(metadata))
}

// ExtractTraceContext restores the W3C parent from queue metadata. Invalid or
// absent metadata safely falls back to the supplied context.
func ExtractTraceContext(ctx context.Context, metadata map[string]string) context.Context {
	return propagation.TraceContext{}.Extract(ctx, propagation.MapCarrier(metadata))
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
