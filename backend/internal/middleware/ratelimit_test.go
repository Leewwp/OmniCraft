package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"omnicraft/backend/config"
)

func TestCredentialRateLimitUsesAccountKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	cfg := &config.RateLimitConfig{
		Enabled:             true,
		CredentialPerMinute: 2,
		NormalWindowSec:     60,
	}
	r := gin.New()
	r.POST("/login", CredentialRateLimit(rdb, cfg), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"User@Example.com","password":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = fmt.Sprintf("198.51.100.%d:12345", i+10)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if i < 2 && w.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 body=%s", i+1, w.Code, w.Body.String())
		}
		if i == 2 && w.Code != http.StatusTooManyRequests {
			t.Fatalf("third request status = %d, want 429 body=%s", w.Code, w.Body.String())
		}
		if i == 2 && !strings.Contains(w.Body.String(), "CREDENTIAL_RATE_LIMIT_EXCEEDED") {
			t.Fatalf("third response body = %s, want CREDENTIAL_RATE_LIMIT_EXCEEDED", w.Body.String())
		}
	}
}

func TestRedisFixedWindowLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	r := gin.New()
	r.GET("/search", RedisFixedWindowLimit(rdb, "ratelimit:test", 2, time.Minute, false), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/search", nil)
		req.RemoteAddr = "198.51.100.10:12345"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if i < 2 && w.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 body=%s", i+1, w.Code, w.Body.String())
		}
		if i == 2 && w.Code != http.StatusTooManyRequests {
			t.Fatalf("third request status = %d, want 429 body=%s", w.Code, w.Body.String())
		}
	}
}
