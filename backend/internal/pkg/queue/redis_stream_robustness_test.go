package queue

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// SP-25 FR-09（heavy，TDD 先行）：毒丸 panic 不杀消费 goroutine（进 DLQ+ACK）；
// 退避表空配置不 panic、末轮不多余 sleep；滞留 pending 回收；观测节流；
// 统计字段不再把消费者个数当 consumed。

func robustnessBroker(t *testing.T) (*RedisStreamBroker, *redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	broker := NewRedisStreamBroker(rdb, &QueueConfig{
		Enabled:         true,
		MaxAttempts:     2,
		RetryBackoffSec: []int{0, 0},
		MaxLen:          1000,
	})
	t.Cleanup(broker.Stop)
	return broker, rdb, mr
}

// 中-8 核心：handler panic 必须被 recover——毒丸消息进 DLQ 并 ACK，
// handleMessage 返回（消费 goroutine 存活），后续消息继续消费。
func TestHandleMessageRecoversPanicToDLQAndAcks(t *testing.T) {
	broker, rdb, _ := robustnessBroker(t)
	ctx := context.Background()

	xadd := rdb.XAdd(ctx, &redis.XAddArgs{Stream: streamKey("poison.topic"), Values: map[string]any{"payload": `{"x":1}`}})
	id, err := xadd.Result()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := broker.ensureGroup(ctx, streamKey("poison.topic"), "poison-group"); err != nil {
		t.Fatalf("group: %v", err)
	}
	msg := broker.decodeMessage("poison.topic", redis.XMessage{ID: id, Values: map[string]any{"payload": `{"x":1}`}})
	msg.Group = "poison-group"

	done := make(chan struct{})
	go func() {
		defer close(done)
		broker.handleMessage(ctx, "poison.topic", "poison-group", msg, func(_ context.Context, _ Message) error {
			panic("poison pill")
		})
	}()
	select {
	case <-done:
		// recover 生效：handleMessage 正常返回
	case <-time.After(3 * time.Second):
		t.Fatal("handleMessage did not return — the panic killed the consume path")
	}

	if n := rdb.XLen(ctx, "omnicraft:dead-letter").Val(); n != 1 {
		t.Fatalf("DLQ entries = %d, want 1 (poison message must be dead-lettered)", n)
	}
	p := rdb.XPending(ctx, streamKey("poison.topic"), "poison-group").Val()
	if p.Count != 0 {
		t.Fatalf("pending after poison = %d, want 0 (must ACK so it is not redelivered forever)", p.Count)
	}
}

// 低-31 + 验证轮顺带修：退避决策收口为纯函数——空表走默认兜底（不除零），
// 末轮（attempt == maxAttempts-1）不再退避，表短于尝试数取模回绕。
func TestBackoffDelayGuards(t *testing.T) {
	if d := backoffDelay(nil, 0, 3); d != time.Duration(DefaultRetryBackoffSec[0])*time.Second {
		t.Fatalf("empty table must fall back to defaults, got %v", d)
	}
	if d := backoffDelay([]int{5, 15}, 0, 3); d != 5*time.Second {
		t.Fatalf("table[0], got %v", d)
	}
	if d := backoffDelay([]int{5, 15}, 1, 3); d != 15*time.Second {
		t.Fatalf("table[1], got %v", d)
	}
	if d := backoffDelay([]int{5, 15}, 2, 3); d != 0 {
		t.Fatalf("final attempt must not back off (the retry loop is over), got %v", d)
	}
	if d := backoffDelay([]int{5, 15}, 2, 4); d != 5*time.Second {
		t.Fatalf("wrap-around indexing, got %v", d)
	}
	if d := backoffDelay([]int{0, 0}, 0, 2); d != 0 {
		t.Fatalf("explicit zero backoff honored, got %v", d)
	}
}

// 空 RetryBackoffSec 走兜底不 panic（修复前 len==0 取模直接 panic）。
// 兜底表首档 10s——用可取消 ctx 在退避等待中打断，只验证「不 panic」焦点。
func TestHandleMessageEmptyBackoffScheduleNoPanic(t *testing.T) {
	broker, rdb, _ := robustnessBroker(t)
	broker.cfg.RetryBackoffSec = nil
	ctx := context.Background()

	id, err := rdb.XAdd(ctx, &redis.XAddArgs{Stream: streamKey("empty.backoff"), Values: map[string]any{"payload": "p"}}).Result()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := broker.ensureGroup(ctx, streamKey("empty.backoff"), "eb-group"); err != nil {
		t.Fatalf("group: %v", err)
	}
	msg := broker.decodeMessage("empty.backoff", redis.XMessage{ID: id, Values: map[string]any{"payload": "p"}})
	msg.Group = "eb-group"

	calls := 0
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		broker.handleMessage(runCtx, "empty.backoff", "eb-group", msg, func(_ context.Context, _ Message) error {
			calls++
			return errRetryPlease
		})
	}()
	time.Sleep(300 * time.Millisecond) // 首次失败已进入兜底退避等待
	cancel()

	select {
	case <-done:
		if calls == 0 {
			t.Fatal("handler was never invoked")
		}
		t.Log("no panic with an empty backoff schedule; backoff interrupted by ctx cancel")
	case <-time.After(2 * time.Second):
		t.Fatal("handleMessage hung")
	}
}

var errRetryPlease = &retryError{}

type retryError struct{}

func (*retryError) Error() string { return "retry me" }

// 中-8：滞留 pending（claimed 未 ACK，进程崩溃形态）经 XAUTOCLAIM 回收重投。
// 生产阈值 5 分钟；测试注入 minIdle=0 立即回收（miniredis 不推进空闲时钟）。
func TestReclaimStuckPendingRedelivers(t *testing.T) {
	broker, rdb, mr := robustnessBroker(t)
	ctx := context.Background()

	id, err := rdb.XAdd(ctx, &redis.XAddArgs{Stream: streamKey("stuck.topic"), Values: map[string]any{"payload": "s"}}).Result()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := broker.ensureGroup(ctx, streamKey("stuck.topic"), "stuck-group"); err != nil {
		t.Fatalf("group: %v", err)
	}
	// 另一个消费者读走但不 ACK：消息滞留 PEL（崩溃窗口形态）。
	if _, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: "stuck-group", Consumer: "crashed-worker",
		Streams: []string{streamKey("stuck.topic"), ">"}, Count: 10,
	}).Result(); err != nil {
		t.Fatalf("stuck read: %v", err)
	}
	_ = mr

	received := make(chan string, 1)
	handler := func(_ context.Context, msg Message) error {
		received <- msg.ID
		return nil
	}

	broker.reclaimStuckPending(ctx, streamKey("stuck.topic"), "stuck-group", "stuck-group-worker", 0, handler)

	select {
	case got := <-received:
		if got != id {
			t.Fatalf("reclaimed id = %s, want %s", got, id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stuck pending message was not reclaimed/redelivered")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if rdb.XPending(ctx, streamKey("stuck.topic"), "stuck-group").Val().Count == 0 {
			return // reclaimed, handled, ACKed
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("reclaimed message was not ACKed after handling")
}

// 低-32：consumed 不再展示消费者个数，改为 entries_read 推导。
func TestGetQueueStatsConsumedNotConsumerCount(t *testing.T) {
	_, rdb, _ := robustnessBroker(t)
	ctx := context.Background()

	rdb.XAdd(ctx, &redis.XAddArgs{Stream: streamKey("stats.topic"), Values: map[string]any{"payload": "1"}})
	rdb.XGroupCreateMkStream(ctx, streamKey("stats.topic"), "stats-group", "0")
	rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: "stats-group", Consumer: "c1",
		Streams: []string{streamKey("stats.topic"), ">"}, Count: 10,
	})
	groups, err := rdb.XInfoGroups(ctx, streamKey("stats.topic")).Result()
	if err != nil || len(groups) == 0 {
		t.Fatalf("xinfo groups: %v", err)
	}
	consumers := groups[0].Consumers
	if consumers < 1 {
		t.Fatalf("precondition: need ≥1 consumer, got %d", consumers)
	}

	stats, err := GetQueueStats(ctx, rdb, []string{"stats.topic"})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("stats rows = %d", len(stats))
	}
	// 与 XInfoGroups 原始值自洽：必须等于 entries_read（未跟踪时为 0），
	// 不得再回填消费者个数。
	if stats[0].Consumed != groups[0].EntriesRead {
		t.Fatalf("consumed = %d, want entries_read %d (must not mirror consumer count %d)",
			stats[0].Consumed, groups[0].EntriesRead, consumers)
	}
}

// 中-8 顺带修：backlog 观测节流（2s×goroutine 的 XINFO GROUPS 轮询收敛）。
func TestBacklogThrottleAllowsOncePerInterval(t *testing.T) {
	th := newBacklogThrottle(30 * time.Second)
	now := time.Now()
	if !th.allow(now) {
		t.Fatal("first observation must pass")
	}
	if th.allow(now.Add(time.Second)) {
		t.Fatal("observation within the interval must be throttled")
	}
	if th.allow(now.Add(29 * time.Second)) {
		t.Fatal("observation still within the interval must be throttled")
	}
	if !th.allow(now.Add(31 * time.Second)) {
		t.Fatal("observation after the interval must pass")
	}
}
