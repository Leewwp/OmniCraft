package middleware

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"omnicraft/backend/config"
)

// SP-16 #451: ConsumeUploadQuota is the shared hourly upload window behind
// middleware.UploadRateLimit and the MCP upload tools — external agents must
// consume the same per-user quota as web uploads, never a parallel one.
func TestConsumeUploadQuota(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cfg := &config.RateLimitConfig{Enabled: true, UploadPerHour: 2, UploadWindowSec: 7200}

	for i := 0; i < 2; i++ {
		if err := ConsumeUploadQuota(context.Background(), rdb, cfg, 77); err != nil {
			t.Fatalf("call %d within limit: %v", i, err)
		}
	}
	if err := ConsumeUploadQuota(context.Background(), rdb, cfg, 77); err == nil {
		t.Fatal("third call in the same window must be rejected")
	}

	// A different user has an independent window.
	if err := ConsumeUploadQuota(context.Background(), rdb, cfg, 88); err != nil {
		t.Fatalf("other user rejected: %v", err)
	}

	// nil redis disables the quota (local dev / degraded mode), matching
	// UploadRateLimit's fail-open posture.
	if err := ConsumeUploadQuota(context.Background(), nil, cfg, 77); err != nil {
		t.Fatalf("nil rdb must fail open: %v", err)
	}
}
