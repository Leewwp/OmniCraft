package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/guestid"
	"omnicraft/backend/internal/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// #854 anonymous agent surface. Three cooperating pieces live here:
//
//   - GuestAgentAccess: the total gate (features.guest_agent_enabled AND
//     agent.web_agent_enabled) plus the signed device-cookie identity. Reads
//     bootstrap an absent identity (mint + initialize the budget); the
//     generation route demands an existing valid cookie so a direct API call
//     can never silently become a fresh budget. Invalid or tampered cookies
//     are rejected outright — they never downgrade to a new identity, and a
//     request carrying an Authorization credential never degrades to guest.
//   - GuestQuotaReserver: the per-device cumulative turn budget (no TTL,
//     never replenished) plus the per-device in-flight cap, reserved in ONE
//     Redis Lua operation. Every Redis fault fails closed: a missing counter
//     for a valid cookie (allkeys-lru eviction, flush) locks the device out
//     instead of re-creating a fresh budget.
//   - GuestAgentCostLimit: the per-IP agent_guest token bucket. It reuses the
//     #729 atomic bucket algorithm but — unlike the browsing buckets — runs
//     independent of rate_limit.enabled / features.guest_rate_limit_enabled
//     and fails closed on every Redis fault: a cost-bearing surface never
//     inherits the availability-first fail-open posture of public reads.

const (
	GuestDeviceKeyContextKey = "guestDeviceKey"

	// quota redis keys. The turns counter deliberately has NO TTL: the
	// budget is cumulative for the device identity's lifetime. Only the
	// in-flight marker carries a safety TTL so a crashed request cannot pin
	// a concurrency slot forever.
	guestTurnsKeyPrefix    = "agent:guest:turns:"
	guestInflightKeyPrefix = "agent:guest:inflight:"
	guestInflightTTL       = 15 * time.Minute

	// guestQuotaRedisTimeout bounds the wait on a degraded Redis before the
	// fail-closed path fires (admission refuses, nothing is consumed).
	guestQuotaRedisTimeout = 2 * time.Second
)

// Quota reserve outcomes returned by guestQuotaReserveLua.
//
//	0 admitted (turn + in-flight slot consumed)
//	1 budget exhausted (nothing consumed)
//	2 counter state missing for a presented identity (nothing consumed)
//	3 concurrency cap reached (in-flight slot rolled back, nothing consumed)
const guestQuotaReserveLua = `
local used = tonumber(redis.call('GET', KEYS[1]) or '-1')
if used < 0 then return 2 end
if used >= tonumber(ARGV[1]) then return 1 end
local inflight = redis.call('INCR', KEYS[2])
if inflight > tonumber(ARGV[2]) then
  redis.call('DECR', KEYS[2])
  return 3
end
redis.call('EXPIRE', KEYS[2], ARGV[3])
redis.call('INCR', KEYS[1])
return 0
`

// guestQuotaReleaseLua decrements the in-flight marker, clamped at zero.
const guestQuotaReleaseLua = `
local n = redis.call('DECR', KEYS[1])
if n < 0 then redis.call('SET', KEYS[1], 0) end
return n
`

var (
	guestQuotaReserveScript = redis.NewScript(guestQuotaReserveLua)
	guestQuotaReleaseScript = redis.NewScript(guestQuotaReleaseLua)

	// ErrGuestQuotaExceeded: the device spent its cumulative budget. The 4th
	// request is refused before any Provider work, UI or not.
	ErrGuestQuotaExceeded = errors.New("guest agent turn budget exhausted")
	// ErrGuestQuotaStateLost: a valid signed cookie whose counter key is gone
	// (eviction or flush). Fail closed; recovery only from a reliable
	// retention record, never an automatic re-create.
	ErrGuestQuotaStateLost = errors.New("guest agent quota state lost")
	// ErrGuestConcurrencyLimit: the device already runs the configured number
	// of in-flight turns (multi-tab / parallel direct calls).
	ErrGuestConcurrencyLimit = errors.New("guest agent concurrency limit reached")
)

// GuestTurnsKey returns the cumulative counter key of a device identity.
func GuestTurnsKey(deviceKey string) string { return guestTurnsKeyPrefix + deviceKey }

// GuestInflightKey returns the in-flight marker key of a device identity.
func GuestInflightKey(deviceKey string) string { return guestInflightKeyPrefix + deviceKey }

// GuestQuotaReserver atomically reserves one generation turn against a
// device's cumulative budget and in-flight cap.
type GuestQuotaReserver struct {
	rdb *redis.Client
	cfg *config.Config
}

// NewGuestQuotaReserver builds the reserver. A nil rdb fails closed on every
// operation; enforcement is never silently skipped.
func NewGuestQuotaReserver(rdb *redis.Client, cfg *config.Config) *GuestQuotaReserver {
	return &GuestQuotaReserver{rdb: rdb, cfg: cfg}
}

// Initialize registers a freshly minted device identity with a zeroed
// counter. SET NX semantics: a repeated bootstrap (parallel first requests,
// replayed reads) can never replenish an existing counter.
func (r *GuestQuotaReserver) Initialize(ctx context.Context, deviceKey string) error {
	if r.rdb == nil {
		return fmt.Errorf("guest quota reserver: redis unavailable")
	}
	callCtx, cancel := context.WithTimeout(ctx, guestQuotaRedisTimeout)
	defer cancel()
	// SET NX: an existing counter is kept verbatim either way (bootstrap
	// never replenishes), so the ok branch collapses to the same outcome.
	_, err := r.rdb.SetNX(callCtx, GuestTurnsKey(deviceKey), 0, 0).Result()
	if err != nil {
		return fmt.Errorf("guest quota reserver: %w", err)
	}
	return nil
}

// Reserve atomically consumes one turn. nil = admitted (the caller owes the
// Provider run and must Release the in-flight slot); the typed errors mark
// refusals that consumed nothing; any other error is a Redis fault and the
// caller MUST fail closed.
func (r *GuestQuotaReserver) Reserve(ctx context.Context, deviceKey string) error {
	if r.rdb == nil {
		return fmt.Errorf("guest quota reserver: redis unavailable")
	}
	if r.cfg == nil || r.cfg.Agent.Guest.MaxTotalTurns <= 0 || r.cfg.Agent.Guest.MaxConcurrentTurns <= 0 {
		return fmt.Errorf("guest quota reserver: invalid runtime quota configuration")
	}
	callCtx, cancel := context.WithTimeout(ctx, guestQuotaRedisTimeout)
	defer cancel()
	res, err := guestQuotaReserveScript.Run(
		callCtx,
		r.rdb,
		[]string{GuestTurnsKey(deviceKey), GuestInflightKey(deviceKey)},
		r.cfg.Agent.Guest.MaxTotalTurns,
		r.cfg.Agent.Guest.MaxConcurrentTurns,
		int(guestInflightTTL.Seconds()),
	).Int()
	if err != nil {
		return fmt.Errorf("guest quota reserver: %w", err)
	}
	switch res {
	case 0:
		return nil
	case 1:
		return ErrGuestQuotaExceeded
	case 2:
		return ErrGuestQuotaStateLost
	case 3:
		return ErrGuestConcurrencyLimit
	default:
		return fmt.Errorf("guest quota reserver: unexpected lua result %d", res)
	}
}

// Release returns one in-flight slot after the turn reached any terminal
// outcome (success, provider failure, timeout, stop, disconnect). The budget
// itself stays consumed.
func (r *GuestQuotaReserver) Release(ctx context.Context, deviceKey string) error {
	if r.rdb == nil {
		return fmt.Errorf("guest quota reserver: redis unavailable")
	}
	callCtx, cancel := context.WithTimeout(ctx, guestQuotaRedisTimeout)
	defer cancel()
	if _, err := guestQuotaReleaseScript.Run(callCtx, r.rdb, []string{GuestInflightKey(deviceKey)}).Int(); err != nil {
		return fmt.Errorf("guest quota reserver release: %w", err)
	}
	return nil
}

// Remaining reports the device's unused turn budget. A missing counter for a
// presented identity is a fail-closed state, not a fresh budget.
func (r *GuestQuotaReserver) Remaining(ctx context.Context, deviceKey string) (int, error) {
	if r.rdb == nil {
		return 0, fmt.Errorf("guest quota reserver: redis unavailable")
	}
	// Same runtime-config guard shape as Reserve: without a positive budget
	// the read cannot produce a meaningful remainder, so refuse closed.
	if r.cfg == nil || r.cfg.Agent.Guest.MaxTotalTurns <= 0 {
		return 0, fmt.Errorf("guest quota reserver: invalid runtime quota configuration")
	}
	callCtx, cancel := context.WithTimeout(ctx, guestQuotaRedisTimeout)
	defer cancel()
	raw, err := r.rdb.Get(callCtx, GuestTurnsKey(deviceKey)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, ErrGuestQuotaStateLost
	}
	if err != nil {
		return 0, fmt.Errorf("guest quota reserver: %w", err)
	}
	used, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("guest quota reserver: corrupt counter %q", raw)
	}
	remaining := r.cfg.Agent.Guest.MaxTotalTurns - used
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

// guestCookieName mirrors the CSRF cookie strategy: release pins the
// __Host- prefix (Secure, Path=/, no Domain), debug keeps the plain name a
// local http stack can actually set.
func guestCookieName(cfg *config.Config) string {
	if cfg.Server.Mode == "release" {
		return "__Host-omnicraft_guest"
	}
	return "omnicraft_guest"
}

// GuestAgentAccess is the #854 total gate + device identity middleware.
func GuestAgentAccess(cfg *config.Config, rdb *redis.Client) gin.HandlerFunc {
	quota := NewGuestQuotaReserver(rdb, cfg)
	return func(c *gin.Context) {
		// Total gate: one switch for every guest surface (API, Header entry
		// and landing page read the same flags via public config). Closing it
		// never touches the authenticated agent.
		if cfg == nil || !cfg.Features.GuestAgentEnabled || !cfg.Agent.WebAgentEnabled {
			response.Error(c, http.StatusForbidden, "GUEST_AGENT_DISABLED",
				"guest agent is not available")
			return
		}
		// The guest surface is anonymous by construction. Any Authorization
		// credential — valid or not — is refused here instead of degrading to
		// guest: a bad bearer must never earn an anonymous budget, and logged-
		// in users have their own guarded surface.
		if c.GetHeader("Authorization") != "" {
			response.Error(c, http.StatusForbidden, "AUTH_CHANNEL_CONFLICT",
				"this endpoint only serves the anonymous device identity")
			return
		}

		cookieName := guestCookieName(cfg)
		raw, _ := c.Cookie(cookieName)
		if raw == "" {
			if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
				// Generation demands an existing identity: a direct POST can
				// never mint a fresh budget by skipping the read bootstrap.
				response.Error(c, http.StatusUnauthorized, "GUEST_DEVICE_REQUIRED",
					"guest device identity is required")
				return
			}
			cookie, key, err := guestid.Issue(cfg.Agent.Guest.CookieSecret)
			if err != nil {
				slog.Error("guest device issue failed", "error", err)
				response.Error(c, http.StatusServiceUnavailable, "GUEST_DEVICE_UNAVAILABLE",
					"guest identity service is temporarily unavailable")
				return
			}
			if err := quota.Initialize(c.Request.Context(), key); err != nil {
				// Without a tracked counter the budget cannot be enforced:
				// fail closed instead of handing out an untracked identity.
				slog.Error("guest device bootstrap failed", "error", err)
				response.Error(c, http.StatusServiceUnavailable, "GUEST_QUOTA_UNAVAILABLE",
					"guest quota service is temporarily unavailable")
				return
			}
			secure := cfg.Server.Mode == "release"
			// The cookie rides the CSRF posture (SameSite=Lax): legitimate
			// calls are same-site; cross-site writes stop at the browser and
			// the global double-submit check stays the second lock.
			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie(cookieName, cookie, cfg.Agent.Guest.CookieMaxAgeHours*3600, "/", "", secure, true)
			c.Set(GuestDeviceKeyContextKey, key)
			c.Next()
			return
		}

		key, err := guestid.Verify(cfg.Agent.Guest.CookieSecret, raw)
		if err != nil {
			// Tampered/garbled identity: refused outright, never re-minted —
			// an attacker must not be able to trade a broken cookie for a
			// fresh budget inside the same request.
			response.Error(c, http.StatusUnauthorized, "GUEST_DEVICE_INVALID",
				"guest device identity is invalid")
			return
		}
		c.Set(GuestDeviceKeyContextKey, key)
		c.Next()
	}
}

// GetGuestDeviceKey returns the verified device identity of a guest request.
func GetGuestDeviceKey(c *gin.Context) string {
	return c.GetString(GuestDeviceKeyContextKey)
}

// guestAgentCostBucket resolves the agent_guest cost tier with fail-closed
// semantics: an explicit invalid override refuses instead of silently
// falling back to the code default (the fallback exists to protect browsing
// tiers against typos; on a cost-bearing tier it would mask a disable
// attempt, so the structural config gate rejects it and this resolver refuses
// at runtime too).
func guestAgentCostBucket(cfg *config.Config) (config.GuestBucketConfig, bool) {
	if cfg.RateLimit.GuestBuckets != nil {
		if spec, ok := cfg.RateLimit.GuestBuckets["agent_guest"]; ok {
			if spec.Capacity > 0 && spec.RefillPerMinute > 0 {
				return spec, true
			}
			return config.GuestBucketConfig{}, false
		}
	}
	spec, ok := config.DefaultGuestBuckets["agent_guest"]
	return spec, ok
}

// GuestAgentCostLimit applies the per-IP agent_guest token bucket to the
// generation route. Same atomic #729 algorithm, fail-closed posture.
func GuestAgentCostLimit(rdb *redis.Client, cfg *config.Config, now func() time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg == nil || !cfg.Features.GuestAgentEnabled {
			c.Next()
			return
		}
		spec, ok := guestAgentCostBucket(cfg)
		if !ok {
			response.Error(c, http.StatusServiceUnavailable, "GUEST_AGENT_CONFIG_INVALID",
				"guest agent cost limits are not configured")
			return
		}
		if rdb == nil {
			response.Error(c, http.StatusServiceUnavailable, "GUEST_RATE_LIMIT_UNAVAILABLE",
				"guest rate limiting is temporarily unavailable")
			return
		}
		key := guestBucketKeyPrefix + ":agent_guest:" + c.ClientIP()
		callCtx, cancel := context.WithTimeout(c.Request.Context(), guestRedisCallTimeout)
		allowed, retryAfterSec, err := takeGuestBucket(callCtx, rdb, key, spec, now)
		cancel()
		if err != nil {
			// Fail closed: generation admission never inherits the public
			// read layer's fail-open.
			slog.Warn("guest agent cost bucket unavailable, refusing generation", "error", err)
			response.Error(c, http.StatusServiceUnavailable, "GUEST_RATE_LIMIT_UNAVAILABLE",
				"guest rate limiting is temporarily unavailable")
			return
		}
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(retryAfterSec))
			response.Error(c, http.StatusTooManyRequests, "GUEST_RATE_LIMIT_EXCEEDED",
				"too many guest agent requests from this address, please retry later")
			return
		}
		c.Next()
	}
}
