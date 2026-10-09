package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

/* #729 匿名公开端点细档限流（heavy，TDD）：在全局 300/min 固定窗口之上，
 * 为八条匿名读端点叠加 per-IP 原子令牌桶。算法契约——Redis Lua 单次 EVAL
 * 完成回补计算、判定扣量与 TTL 设置（无 CHECK-THEN-ACT 竞态）；时钟由调用
 * 方注入（now_ms 进 ARGV），测试可时间旅行；拒绝不扣令牌（重试风暴不推迟
 * 恢复）；低速持续流量按回补速率消费永不累积封禁（非伪滑窗）。 */

// GuestBucketSpec re-exports the config tier type for middleware consumers.
type GuestBucketSpec = config.GuestBucketConfig

// guestBucketLua is the atomic token bucket: KEYS[1]=bucket, ARGV=[capacity,
// refill_per_ms, now_ms]. Returns {allowed(0/1), retry_after_sec}.
// Clock regress (multi-instance skew) clamps elapsed to zero; TTL = time to
// refill a full bucket + slack so idle buckets recycle.
const guestBucketLua = `
local capacity = tonumber(ARGV[1])
local refill_per_ms = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])
local tokens = tonumber(redis.call('HGET', KEYS[1], 't')) or capacity
local ts = tonumber(redis.call('HGET', KEYS[1], 'ts')) or now_ms
if ts > now_ms then ts = now_ms end
tokens = math.min(capacity, tokens + (now_ms - ts) * refill_per_ms)
local allowed = 0
local retry_after_sec = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
else
  retry_after_sec = math.ceil((1 - tokens) / (refill_per_ms * 1000))
  if retry_after_sec < 1 then retry_after_sec = 1 end
end
redis.call('HSET', KEYS[1], 't', tokens, 'ts', now_ms)
redis.call('EXPIRE', KEYS[1], math.ceil(capacity / (refill_per_ms * 1000)) + 60)
return {allowed, retry_after_sec}
`

const (
	guestBucketKeyPrefix = "ratelimit:guest"
	// guestWarnThrottle keeps Redis-fault fail-open logs rate-limited (one
	// line per window limiter-wide, not per request).
	guestWarnThrottle = 10 * time.Second
)

// guestRedisCallTimeout bounds the wait on a degraded Redis before the
// fail-open path fires (availability-first protective layer). Var (not
// const) so tests can widen it under -race scheduling pressure.
var guestRedisCallTimeout = 100 * time.Millisecond

// GuestRateLimiter serves per-tier gin handlers for the anonymous guest
// layer. Features-gated (guest_rate_limit_enabled) AND subordinate to
// rate_limit.enabled; both off ⇒ handlers are no-ops with zero Redis calls.
type GuestRateLimiter struct {
	rdb         *redis.Client
	cfg         *config.RateLimitConfig
	features    *config.FeaturesConfig
	now         func() time.Time
	exactExempt map[string]bool
	cidrExempt  []*net.IPNet
	lastWarn    atomic.Int64 // unix nano of last fail-open warn
}

// guestBucketScript is the Lua bucket run via EVALSHA with fallback (same
// wiring as agent_ratelimit) instead of shipping the script body per request.
var guestBucketScript = redis.NewScript(guestBucketLua)

func NewGuestRateLimiter(rdb *redis.Client, cfg *config.RateLimitConfig, features *config.FeaturesConfig, now func() time.Time) *GuestRateLimiter {
	g := &GuestRateLimiter{rdb: rdb, cfg: cfg, features: features, now: now, exactExempt: map[string]bool{}}
	for _, entry := range cfg.GuestExemptIPs {
		if _, network, err := net.ParseCIDR(entry); err == nil {
			g.cidrExempt = append(g.cidrExempt, network)
		} else if ip := net.ParseIP(entry); ip != nil {
			g.exactExempt[ip.String()] = true
		} else {
			slog.Warn("guest rate limiter: unparseable exempt entry ignored", "entry", entry)
		}
	}
	return g
}

func (g *GuestRateLimiter) enabled() bool {
	return g.cfg != nil && g.cfg.Enabled &&
		g.features != nil && g.features.GuestRateLimitEnabled
}

func (g *GuestRateLimiter) exempt(ip string) bool {
	if len(g.exactExempt) == 0 && len(g.cidrExempt) == 0 {
		return false
	}
	if g.exactExempt[ip] {
		return true
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, network := range g.cidrExempt {
		if network.Contains(parsed) {
			return true
		}
	}
	return false
}

// Tier returns the per-route handler for a stable route-template tier name
// (contents_detail etc.). Unknown tiers get a pass-through handler: never
// invent a bucket the census/config did not define.
func (g *GuestRateLimiter) Tier(tier string) gin.HandlerFunc {
	if g.cfg == nil {
		return func(c *gin.Context) { c.Next() }
	}
	spec, ok := g.cfg.GuestBucketFor(tier)
	if !ok {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		if !g.enabled() || g.rdb == nil {
			c.Next()
			return
		}
		// ClientIP follows the engine's SetTrustedProxies (main.go wires
		// cfg.Security.TrustedProxies, factory = loopback): an untrusted
		// peer cannot swap buckets by forging X-Forwarded-For.
		ip := c.ClientIP()
		if g.exempt(ip) {
			c.Next()
			return
		}
		key := guestBucketKeyPrefix + ":" + tier + ":" + ip
		allowed, retryAfterSec, err := g.take(c.Request.Context(), key, spec)
		if err != nil {
			g.warnFailOpen(err, tier)
			c.Next()
			return
		}
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(retryAfterSec))
			response.Error(c, http.StatusTooManyRequests, "GUEST_RATE_LIMIT_EXCEEDED",
				"guest rate limit exceeded for this endpoint, please retry later")
			return
		}
		c.Next()
	}
}

func (g *GuestRateLimiter) take(ctx context.Context, key string, spec GuestBucketSpec) (bool, int, error) {
	callCtx, cancel := context.WithTimeout(ctx, guestRedisCallTimeout)
	defer cancel()
	return takeGuestBucket(callCtx, g.rdb, key, spec, g.now)
}

// takeGuestBucket is the shared atomic #729 bucket evaluation. Both layers
// run under the same guestRedisCallTimeout bound — they differ only in the
// failure posture on a bucket error: the browsing layer fails open
// (availability-first), the #854 cost bucket fails closed.
func takeGuestBucket(ctx context.Context, rdb *redis.Client, key string, spec GuestBucketSpec, now func() time.Time) (bool, int, error) {
	refillPerMs := spec.RefillPerMinute / 60_000.0
	res, err := guestBucketScript.Run(ctx, rdb, []string{key},
		strconv.Itoa(spec.Capacity),
		strconv.FormatFloat(refillPerMs, 'g', -1, 64),
		strconv.FormatInt(now().UnixMilli(), 10),
	).Result()
	if err != nil {
		return false, 0, err
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) < 2 {
		return false, 0, fmt.Errorf("guest bucket: unexpected lua result %#v", res)
	}
	allowedFlag, _ := arr[0].(int64)
	retryAfter, _ := arr[1].(int64)
	return allowedFlag == 1, int(retryAfter), nil
}

func (g *GuestRateLimiter) warnFailOpen(err error, tier string) {
	nowNano := g.now().UnixNano()
	last := g.lastWarn.Load()
	if nowNano-last < int64(guestWarnThrottle) {
		return
	}
	if g.lastWarn.CompareAndSwap(last, nowNano) {
		slog.Warn("guest rate limiter fail-open (redis unavailable)", "tier", tier, "error", err)
	}
}
