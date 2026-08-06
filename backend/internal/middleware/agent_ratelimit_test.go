package middleware

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"omnicraft/backend/config"
)

func newQuotaTestReserver(t *testing.T, cfg *config.Config) (*AgentQuotaReserver, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewAgentQuotaReserver(rdb, cfg), mr
}

func TestAgentQuotaReserverAtomicMinuteReservationUnderConcurrency(t *testing.T) {
	cfg := &config.Config{Agent: config.AgentConfig{RateLimitPerMinute: 5, RateLimitPerDay: 1000}}
	res, _ := newQuotaTestReserver(t, cfg)

	const workers = 20
	allowed := make([]bool, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			allowed[i] = res.Reserve(context.Background(), 42) == nil
		}(i)
	}
	wg.Wait()

	ok := 0
	for _, a := range allowed {
		if a {
			ok++
		}
	}
	if ok != 5 {
		t.Fatalf("allowed = %d, want exactly 5 (minute limit); no over-reservation allowed under concurrency", ok)
	}
}

func TestAgentQuotaReserverAtomicDailyReservationUnderConcurrency(t *testing.T) {
	cfg := &config.Config{Agent: config.AgentConfig{RateLimitPerMinute: 1000, RateLimitPerDay: 3}}
	res, _ := newQuotaTestReserver(t, cfg)

	const workers = 20
	allowed := make([]bool, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			allowed[i] = res.Reserve(context.Background(), 7) == nil
		}(i)
	}
	wg.Wait()

	ok := 0
	for _, a := range allowed {
		if a {
			ok++
		}
	}
	if ok != 3 {
		t.Fatalf("allowed = %d, want exactly 3 (daily limit); no over-reservation allowed under concurrency", ok)
	}
}

func TestAgentQuotaReserverMinuteWindowRollsOver(t *testing.T) {
	cfg := &config.Config{Agent: config.AgentConfig{RateLimitPerMinute: 1, RateLimitPerDay: 100}}
	res, mr := newQuotaTestReserver(t, cfg)
	ctx := context.Background()

	if err := res.Reserve(ctx, 9); err != nil {
		t.Fatalf("first reserve: %v", err)
	}
	if err := res.Reserve(ctx, 9); !errors.Is(err, ErrAgentQuotaExceeded) {
		t.Fatalf("second reserve err = %v, want ErrAgentQuotaExceeded", err)
	}

	mr.FastForward(61 * time.Second)

	if err := res.Reserve(ctx, 9); err != nil {
		t.Fatalf("reserve after minute rollover: %v", err)
	}
}

func TestAgentQuotaReserverFailClosedWhenRedisUnavailable(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	addr := mr.Addr()
	mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer rdb.Close()

	res := NewAgentQuotaReserver(rdb, &config.Config{Agent: config.AgentConfig{RateLimitPerMinute: 5, RateLimitPerDay: 50}})
	err = res.Reserve(context.Background(), 7)
	if err == nil {
		t.Fatal("Reserve must fail closed when Redis is unavailable")
	}
	if errors.Is(err, ErrAgentQuotaExceeded) {
		t.Fatalf("fail-closed error must not masquerade as quota-exceeded: %v", err)
	}
}

func TestAgentQuotaReserverLimitsAreConfigDriven(t *testing.T) {
	res, mr := newQuotaTestReserver(t, &config.Config{Agent: config.AgentConfig{}})
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := res.Reserve(ctx, 3); err != nil {
			t.Fatalf("reserve %d: %v", i+1, err)
		}
	}
	if err := res.Reserve(ctx, 3); !errors.Is(err, ErrAgentQuotaExceeded) {
		t.Fatalf("6th reserve err = %v, want ErrAgentQuotaExceeded (default minute limit 5)", err)
	}
	if _, err := mr.Get(res.DayKey(3)); err != nil {
		t.Fatalf("day key must exist after 6 reservations: %v", err)
	}
}
