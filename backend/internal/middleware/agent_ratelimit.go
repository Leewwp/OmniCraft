package middleware

import (
	"context"
	"errors"
	"fmt"
	"time"

	"omnicraft/backend/config"

	"github.com/redis/go-redis/v9"
)

// Agent generation quota: per-user per-minute and per-day request reservations.
//
// The minute and day counters are reserved together in ONE Redis Lua operation
// immediately before the first Provider call, so concurrent requests can never
// over-reserve. Feature/auth/schema and client-supplied context visibility
// rejections all happen before Reserve is invoked and therefore never touch
// Redis. Once a reservation is made it is never released: success, timeout,
// Provider error and client cancellation all consume the same request.
//
// Redis unavailability is fail-closed for Provider-consuming routes: Reserve
// returns an error that is NOT ErrAgentQuotaExceeded so callers can respond
// with a service-unavailable error instead of admitting the request.

// ErrAgentQuotaExceeded marks an admitted-but-rejected request: the user has
// consumed their per-minute or per-day Agent generation budget.
var ErrAgentQuotaExceeded = errors.New("agent request quota exceeded")

// agentQuotaReserveScript atomically increments the per-minute and per-day
// counters and returns 1 when either limit is exceeded, 0 when the request is
// admitted. KEYS[1]=minute key, KEYS[2]=day key; ARGV[1]=minute limit,
// ARGV[2]=day limit, ARGV[3]=minute TTL seconds, ARGV[4]=day TTL seconds.
var agentQuotaReserveScript = redis.NewScript(`
local mc = redis.call('INCR', KEYS[1])
if mc == 1 then redis.call('EXPIRE', KEYS[1], ARGV[3]) end
local dc = redis.call('INCR', KEYS[2])
if dc == 1 then redis.call('EXPIRE', KEYS[2], ARGV[4]) end
if mc > tonumber(ARGV[1]) or dc > tonumber(ARGV[2]) then
  return 1
end
return 0
`)

// AgentQuotaReserver atomically reserves one request against the user's
// per-minute and per-day Agent generation budget.
type AgentQuotaReserver struct {
	rdb *redis.Client
	cfg *config.Config
}

// NewAgentQuotaReserver builds a reserver. A nil rdb still fails closed on
// Reserve; the reserver itself never silently skips enforcement.
func NewAgentQuotaReserver(rdb *redis.Client, cfg *config.Config) *AgentQuotaReserver {
	return &AgentQuotaReserver{rdb: rdb, cfg: cfg}
}

// Reserve atomically reserves one request. A nil error means the request is
// admitted; ErrAgentQuotaExceeded means the budget is exhausted; any other
// error means Redis is unavailable and the caller MUST fail closed.
func (r *AgentQuotaReserver) Reserve(ctx context.Context, userID int64) error {
	if r.rdb == nil {
		return fmt.Errorf("agent quota reserver: redis unavailable")
	}
	if userID <= 0 {
		return fmt.Errorf("agent quota reserver: missing user id")
	}
	now := time.Now()

	if r.cfg == nil || r.cfg.Agent.RateLimitPerMinute <= 0 || r.cfg.Agent.RateLimitPerDay <= 0 ||
		r.cfg.RateLimit.AgentMinuteWindowSec <= 0 || r.cfg.RateLimit.AgentWindowSec <= 0 {
		return fmt.Errorf("agent quota reserver: invalid runtime quota configuration")
	}
	perMinute := r.cfg.Agent.RateLimitPerMinute
	perDay := r.cfg.Agent.RateLimitPerDay
	minuteTTL := time.Duration(r.cfg.RateLimit.AgentMinuteWindowSec) * time.Second
	dayTTL := time.Duration(r.cfg.RateLimit.AgentWindowSec) * time.Second

	exceeded, err := agentQuotaReserveScript.Run(
		ctx,
		r.rdb,
		[]string{r.MinuteKey(userID, now), r.DayKey(userID, now)},
		perMinute,
		perDay,
		int(minuteTTL.Seconds()),
		int(dayTTL.Seconds()),
	).Int()
	if err != nil {
		return fmt.Errorf("agent quota reserver: %w", err)
	}
	if exceeded == 1 {
		return ErrAgentQuotaExceeded
	}
	return nil
}

// DayKey returns the daily counter key for the user on the given day.
func (r *AgentQuotaReserver) DayKey(userID int64, at ...time.Time) string {
	t := time.Now()
	if len(at) > 0 {
		t = at[0]
	}
	return fmt.Sprintf("agent:quota:%d:%s", userID, t.Format("2006-01-02"))
}

// MinuteKey returns the per-minute counter key for the user at the given time.
func (r *AgentQuotaReserver) MinuteKey(userID int64, at ...time.Time) string {
	t := time.Now()
	if len(at) > 0 {
		t = at[0]
	}
	return fmt.Sprintf("agent:quota:%d:%s", userID, t.Format("2006-01-02-15:04"))
}
