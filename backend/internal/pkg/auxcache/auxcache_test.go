package auxcache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestCache(t *testing.T, ttl time.Duration) (*Cache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return New("title", "omnicraft:auxcache:title:", ttl, client), mr
}

func TestCacheHitMissRoundTrip(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCache(t, time.Minute)

	if _, ok := c.Get(ctx, "k1"); ok {
		t.Fatal("fresh cache must miss")
	}
	c.Set(ctx, "k1", "站点推荐指南")
	got, ok := c.Get(ctx, "k1")
	if !ok || got != "站点推荐指南" {
		t.Fatalf("expected hit with stored value, got ok=%v value=%q", ok, got)
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	ctx := context.Background()
	c, mr := newTestCache(t, 2*time.Second)

	c.Set(ctx, "k1", "v")
	mr.FastForward(3 * time.Second)
	if _, ok := c.Get(ctx, "k1"); ok {
		t.Fatal("entry past its TTL must miss")
	}
}

func TestDisabledCacheBypassesWithoutRedis(t *testing.T) {
	ctx := context.Background()

	// ttl <= 0 disables even with a live client.
	c := New("title", "p:", 0, redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"}))
	if c.Enabled() {
		t.Fatal("ttl<=0 must disable the cache")
	}
	c.Set(ctx, "k", "v")
	if _, ok := c.Get(ctx, "k"); ok {
		t.Fatal("disabled cache must never return a hit")
	}

	// nil client disables.
	c2 := New("title", "p:", time.Minute, nil)
	if c2.Enabled() {
		t.Fatal("nil client must disable the cache")
	}

	// nil receiver is a valid disabled cache (call sites stay unconditional).
	var c3 *Cache
	if c3.Enabled() {
		t.Fatal("nil cache must report disabled")
	}
	c3.Set(ctx, "k", "v")
	if _, ok := c3.Get(ctx, "k"); ok {
		t.Fatal("nil cache must never return a hit")
	}
}

func TestRedisErrorDegradesToMiss(t *testing.T) {
	ctx := context.Background()
	c, mr := newTestCache(t, time.Minute)
	c.Set(ctx, "k1", "v")
	mr.Close() // every command now fails
	if _, ok := c.Get(ctx, "k1"); ok {
		t.Fatal("redis failure must degrade to miss, never panic or hang")
	}
	// Stores on a dead redis are swallowed.
	c.Set(ctx, "k2", "v")
}

func TestHashKeyStableAndSeparated(t *testing.T) {
	a := HashKey("推荐一些站点")
	b := HashKey("推荐一些站点")
	if a != b {
		t.Fatal("identical input must hash identically")
	}
	if HashKey("a", "b") == HashKey("ab") {
		t.Fatal("part boundaries must be unambiguous")
	}
	if len(a) != 64 {
		t.Fatalf("expected sha-256 hex (64 chars), got %d", len(a))
	}
}
