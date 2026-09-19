// Package auxcache implements the SP-24 R6 best-effort Redis TTL cache for
// auxiliary LLM calls (auto title, query expansion). The main Q&A chain is
// deliberately never cached — semantic correctness outranks token savings.
// A disabled or unwired cache short-circuits to a bypass outcome without
// touching Redis, so call sites stay unconditional.
package auxcache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"omnicraft/backend/internal/observability"
)

// Outcomes recorded on omnicraft_aux_cache_events_total. "bypass" counts
// lookups against a disabled cache, which measures the would-be traffic
// before an operator flips the switch.
const (
	OutcomeHit         = "hit"
	OutcomeMiss        = "miss"
	OutcomeBypass      = "bypass"
	OutcomeStoreFailed = "store_failed"
)

// redisKV is the narrow Redis surface the cache needs; *redis.Client and the
// miniredis-backed test client both satisfy it.
type redisKV interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
}

// Cache is safe for concurrent use. The nil receiver is valid and behaves as
// a permanently disabled cache.
type Cache struct {
	name   string // metric label, from the allowlist in observability
	prefix string // redis key prefix, e.g. "omnicraft:auxcache:title"
	ttl    time.Duration
	client redisKV
}

// New builds one named cache. ttl <= 0 or a nil client keeps the cache
// disabled (Get bypass / Set no-op) — the R6 default-off contract.
func New(name, prefix string, ttl time.Duration, client redisKV) *Cache {
	if client == nil || ttl <= 0 {
		return &Cache{name: name, prefix: prefix, ttl: ttl}
	}
	return &Cache{name: name, prefix: prefix, ttl: ttl, client: client}
}

// Enabled reports whether lookups reach Redis.
func (c *Cache) Enabled() bool {
	return c != nil && c.ttl > 0 && c.client != nil
}

// Get returns the cached value for key. Redis errors degrade to a miss: the
// cache must never turn a working LLM call path into a failure.
func (c *Cache) Get(ctx context.Context, key string) (string, bool) {
	if !c.Enabled() {
		c.record(OutcomeBypass)
		return "", false
	}
	value, err := c.client.Get(ctx, c.prefix+key).Result()
	if err != nil {
		c.record(OutcomeMiss)
		return "", false
	}
	c.record(OutcomeHit)
	return value, true
}

// Set stores value under key. Store failures are logged and counted, never
// propagated.
func (c *Cache) Set(ctx context.Context, key, value string) {
	if !c.Enabled() {
		return
	}
	if err := c.client.Set(ctx, c.prefix+key, value, c.ttl).Err(); err != nil {
		c.record(OutcomeStoreFailed)
		slog.Warn("aux cache store failed", "cache", c.name, "error", err)
	}
}

func (c *Cache) record(outcome string) {
	if c == nil {
		return
	}
	observability.IncDefaultAuxCacheEvent(c.name, outcome)
}

// HashKey derives the cache key from the normalized call inputs: SHA-256
// over the concatenated parts, hex-encoded. Inputs are caller-normalized
// (trimmed) raw text — never user IDs — so identical content shares entries
// across conversations and users.
func HashKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
