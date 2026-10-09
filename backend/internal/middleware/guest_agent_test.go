package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/guestid"
)

func guestTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Features.GuestAgentEnabled = true
	cfg.Agent.WebAgentEnabled = true
	cfg.Agent.Guest = config.AgentGuestConfig{
		MaxTotalTurns:       3,
		ConversationTTLDays: 7,
		MaxConcurrentTurns:  3,
		CookieSecret:        "unit-test-guest-cookie-secret-0123456789abcdef",
		CookieMaxAgeHours:   8760,
	}
	return cfg
}

func newGuestQuotaFixture(t *testing.T) (*GuestQuotaReserver, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewGuestQuotaReserver(rdb, guestTestConfig()), mr
}

func TestGuestQuotaReservesThreeTurnsThenRejects(t *testing.T) {
	res, _ := newGuestQuotaFixture(t)
	ctx := context.Background()
	require.NoError(t, res.Initialize(ctx, "device-a"))

	for i := 0; i < 3; i++ {
		require.NoError(t, res.Reserve(ctx, "device-a"), "turn %d must be admitted", i+1)
	}
	require.True(t, errors.Is(res.Reserve(ctx, "device-a"), ErrGuestQuotaExceeded),
		"turn 4 must be rejected as exhausted")
	require.True(t, errors.Is(res.Reserve(ctx, "device-a"), ErrGuestQuotaExceeded),
		"rejection must not consume: still exhausted")
	remaining, err := res.Remaining(ctx, "device-a")
	require.NoError(t, err)
	require.Equal(t, 0, remaining, "three reservations must consume the whole budget")
}

func TestGuestQuotaRejectionDoesNotConsumeBudget(t *testing.T) {
	res, _ := newGuestQuotaFixture(t)
	ctx := context.Background()
	require.NoError(t, res.Initialize(ctx, "device-a"))
	require.NoError(t, res.Reserve(ctx, "device-a"))
	// A concurrent over-admission is rejected and must not eat budget.
	_ = res.Reserve(ctx, "device-a") // 2nd
	_ = res.Reserve(ctx, "device-a") // 3rd
	require.True(t, errors.Is(res.Reserve(ctx, "device-a"), ErrGuestQuotaExceeded))
	remaining, _ := res.Remaining(ctx, "device-a")
	require.Equal(t, 0, remaining)
}

func TestGuestQuotaMissingStateFailsClosed(t *testing.T) {
	res, _ := newGuestQuotaFixture(t)
	ctx := context.Background()
	// A signed device cookie whose counter key was evicted (allkeys-lru) or
	// flushed: fail closed, never re-create as a fresh budget.
	require.True(t, errors.Is(res.Reserve(ctx, "device-lost"), ErrGuestQuotaStateLost),
		"missing counter state must fail closed")
	_, err := res.Remaining(ctx, "device-lost")
	require.True(t, errors.Is(err, ErrGuestQuotaStateLost))
}

func TestGuestQuotaRedisUnavailableFailsClosed(t *testing.T) {
	res, mr := newGuestQuotaFixture(t)
	ctx := context.Background()
	require.NoError(t, res.Initialize(ctx, "device-a"))
	mr.Close()
	err := res.Reserve(ctx, "device-a")
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrGuestQuotaExceeded), "outage must not read as exhaustion")
	require.NotErrorIs(t, err, ErrGuestQuotaStateLost)
}

func TestGuestQuotaInitializeNeverOverwrites(t *testing.T) {
	res, _ := newGuestQuotaFixture(t)
	ctx := context.Background()
	require.NoError(t, res.Initialize(ctx, "device-a"))
	// Simulate two consumed turns, then a repeated bootstrap.
	require.NoError(t, res.Reserve(ctx, "device-a"))
	require.NoError(t, res.Reserve(ctx, "device-a"))
	require.NoError(t, res.Initialize(ctx, "device-a"))
	remaining, err := res.Remaining(ctx, "device-a")
	require.NoError(t, err)
	require.Equal(t, 1, remaining, "bootstrap must never replenish")
}

func TestGuestQuotaConcurrencyCap(t *testing.T) {
	res, _ := newGuestQuotaFixture(t)
	// Ample budget so this test isolates the in-flight cap: exactly
	// max_concurrent_turns reservations may run at once, the rest bounce off
	// the concurrency limit without consuming budget.
	res.cfg.Agent.Guest.MaxTotalTurns = 100
	ctx := context.Background()
	require.NoError(t, res.Initialize(ctx, "device-a"))

	const workers = 8
	results := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- res.Reserve(ctx, "device-a")
		}()
	}
	wg.Wait()
	close(results)

	admitted, exceeded := 0, 0
	for err := range results {
		switch {
		case err == nil:
			admitted++
		case errors.Is(err, ErrGuestConcurrencyLimit):
			exceeded++
		default:
			t.Fatalf("unexpected reserve error: %v", err)
		}
	}
	require.Equal(t, 3, admitted, "at most max_concurrent_turns may run at once")
	require.Equal(t, workers-3, exceeded)
	// Release frees slots again.
	for i := 0; i < admitted; i++ {
		require.NoError(t, res.Release(ctx, "device-a"))
	}
	require.NoError(t, res.Reserve(ctx, "device-a"))
}

func TestGuestQuotaNilRedisFailsClosed(t *testing.T) {
	res := NewGuestQuotaReserver(nil, guestTestConfig())
	require.Error(t, res.Reserve(context.Background(), "device-a"))
}

func TestGuestQuotaInvalidConfigFailsClosed(t *testing.T) {
	res, _ := newGuestQuotaFixture(t)
	cfg := guestTestConfig()
	cfg.Agent.Guest.MaxTotalTurns = 0
	res.cfg = cfg
	require.Error(t, res.Reserve(context.Background(), "device-a"))
}

// --- access middleware ---

func runGuestAccess(t *testing.T, cfg *config.Config, rdb *redis.Client, method, path string, cookie string, headers map[string]string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var gotKey string
	router.Handle(method, path, GuestAgentAccess(cfg, rdb), func(c *gin.Context) {
		gotKey = GetGuestDeviceKey(c)
		c.String(http.StatusOK, "ok")
	})
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: guestCookieName(cfg), Value: cookie})
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec, gotKey
}

func TestGuestAccessGateClosed(t *testing.T) {
	cfg := guestTestConfig()
	cfg.Features.GuestAgentEnabled = false
	rec, _ := runGuestAccess(t, cfg, nil, http.MethodGet, "/api/v1/agent/guest/quota", "", nil)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "GUEST_AGENT_DISABLED")

	cfg = guestTestConfig()
	cfg.Agent.WebAgentEnabled = false
	rec, _ = runGuestAccess(t, cfg, nil, http.MethodGet, "/api/v1/agent/guest/quota", "", nil)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "GUEST_AGENT_DISABLED")
}

func TestGuestAccessIssuesCookieOnReadWithoutCookie(t *testing.T) {
	mr := newMiniredisOrFatal(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := guestTestConfig()

	rec, _ := runGuestAccess(t, cfg, rdb, http.MethodGet, "/api/v1/agent/guest/quota", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	cookies := rec.Result().Cookies()
	var device *http.Cookie
	for _, c := range cookies {
		if c.Name == guestCookieName(cfg) {
			device = c
		}
	}
	require.NotNil(t, device, "read bootstrap must set the device cookie")
	require.True(t, device.HttpOnly)
	require.Equal(t, http.SameSiteLaxMode, device.SameSite)
	require.Equal(t, "/", device.Path)
	// The minted identity's budget must be initialized exactly once.
	_, verifyErr := guestid.Verify(cfg.Agent.Guest.CookieSecret, device.Value)
	require.NoError(t, verifyErr)
	key, _ := guestid.Verify(cfg.Agent.Guest.CookieSecret, device.Value)
	stored, err := rdb.Get(context.Background(), GuestTurnsKey(key)).Result()
	require.NoError(t, err)
	require.Equal(t, "0", stored)
}

func TestGuestAccessReleaseModeCookieAttributes(t *testing.T) {
	mr := newMiniredisOrFatal(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := guestTestConfig()
	cfg.Server.Mode = "release"

	rec, _ := runGuestAccess(t, cfg, rdb, http.MethodGet, "/api/v1/agent/guest/quota", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var rawSetCookie string
	for _, v := range rec.Result().Header.Values("Set-Cookie") {
		if strings.HasPrefix(v, "__Host-omnicraft_guest") {
			rawSetCookie = v
		}
	}
	require.NotEmpty(t, rawSetCookie, "release mode must use the __Host- prefixed device cookie")
	require.Contains(t, rawSetCookie, "HttpOnly")
	require.Contains(t, rawSetCookie, "Secure")
	require.Contains(t, rawSetCookie, "SameSite=Lax")
}

func TestGuestAccessGenerationRequiresExistingDeviceCookie(t *testing.T) {
	mr := newMiniredisOrFatal(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := guestTestConfig()

	rec, _ := runGuestAccess(t, cfg, rdb, http.MethodPost, "/api/v1/agent/guest/chat/stream", "", nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "GUEST_DEVICE_REQUIRED")
}

func TestGuestAccessRejectsTamperedCookie(t *testing.T) {
	mr := newMiniredisOrFatal(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := guestTestConfig()

	cookie, _, err := guestid.Issue(cfg.Agent.Guest.CookieSecret)
	require.NoError(t, err)
	parts := strings.SplitN(cookie, ".", 2)
	bad := parts[0] + ".0000000000000000000000000000000000000000000000000000000000000000"
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec, _ := runGuestAccess(t, cfg, rdb, method, "/api/v1/agent/guest/quota", bad, nil)
		require.Equal(t, http.StatusUnauthorized, rec.Code, "%s with tampered cookie", method)
		require.Contains(t, rec.Body.String(), "GUEST_DEVICE_INVALID")
	}
	_ = mr
}

func TestGuestAccessRejectsCredentialHeaderInsteadOfDegrading(t *testing.T) {
	mr := newMiniredisOrFatal(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := guestTestConfig()

	cookie, _, err := guestid.Issue(cfg.Agent.Guest.CookieSecret)
	require.NoError(t, err)
	rec, _ := runGuestAccess(t, cfg, rdb, http.MethodGet, "/api/v1/agent/guest/quota", cookie,
		map[string]string{"Authorization": "Bearer not-a-real-token"})
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "AUTH_CHANNEL_CONFLICT")
}

func TestGuestAccessValidCookieResolvesDeviceKey(t *testing.T) {
	mr := newMiniredisOrFatal(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := guestTestConfig()

	cookie, wantKey, err := guestid.Issue(cfg.Agent.Guest.CookieSecret)
	require.NoError(t, err)
	rec, gotKey := runGuestAccess(t, cfg, rdb, http.MethodGet, "/api/v1/agent/guest/quota", cookie, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, wantKey, gotKey)
}

func newMiniredisOrFatal(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	return mr
}

// --- per-IP cost bucket (fail-closed) ---

func newGuestCostLimiter(t *testing.T, mutate func(*config.Config)) (*gin.Engine, *miniredis.Miniredis) {
	t.Helper()
	mr := newMiniredisOrFatal(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := guestTestConfig()
	if mutate != nil {
		mutate(cfg)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/gen", GuestAgentCostLimit(rdb, cfg, time.Now), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return router, mr
}

func doGuestCost(router *gin.Engine, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/gen", nil)
	req.RemoteAddr = ip + ":12345"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestGuestCostLimitBurstThenReject(t *testing.T) {
	router, _ := newGuestCostLimiter(t, nil)
	for i := 0; i < 6; i++ {
		require.Equal(t, http.StatusOK, doGuestCost(router, "198.51.100.10").Code, "request %d", i+1)
	}
	rec := doGuestCost(router, "198.51.100.10")
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Contains(t, rec.Body.String(), "GUEST_RATE_LIMIT_EXCEEDED")
	require.NotEmpty(t, rec.Header().Get("Retry-After"))
	// A different IP keeps its own bucket.
	require.Equal(t, http.StatusOK, doGuestCost(router, "198.51.100.11").Code)
}

func TestGuestCostLimitFailClosedOnRedisOutage(t *testing.T) {
	router, mr := newGuestCostLimiter(t, nil)
	mr.Close()
	rec := doGuestCost(router, "203.0.113.20")
	require.Equal(t, http.StatusServiceUnavailable, rec.Code,
		"generation admission must fail closed, not inherit the public GET fail-open")
	require.Contains(t, rec.Body.String(), "GUEST_RATE_LIMIT_UNAVAILABLE")
}

func TestGuestCostLimitIndependentOfBrowsingLimitSwitches(t *testing.T) {
	// rate_limit.enabled=false and features.guest_rate_limit_enabled=false
	// must NOT turn the agent_guest cost bucket into a no-op.
	router, _ := newGuestCostLimiter(t, func(cfg *config.Config) {
		cfg.RateLimit.Enabled = false
		cfg.Features.GuestRateLimitEnabled = false
	})
	for i := 0; i < 6; i++ {
		require.Equal(t, http.StatusOK, doGuestCost(router, "203.0.113.21").Code, "request %d", i+1)
	}
	require.Equal(t, http.StatusTooManyRequests, doGuestCost(router, "203.0.113.21").Code)
}

func TestGuestCostLimitFailClosedOnUnresolvableBucket(t *testing.T) {
	router, _ := newGuestCostLimiter(t, func(cfg *config.Config) {
		cfg.RateLimit.GuestBuckets = map[string]config.GuestBucketConfig{
			// Explicit zeroed override with the resolution fallback masked:
			// the limiter must refuse admission, not serve for free.
			"agent_guest": {Capacity: -1, RefillPerMinute: -1},
		}
	})
	rec := doGuestCost(router, "203.0.113.22")
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "GUEST_AGENT_CONFIG_INVALID")
}
