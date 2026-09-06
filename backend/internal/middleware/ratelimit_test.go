package middleware

import (
	"context"
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

// #400 子项 3：泄漏自愈——已存在的无 TTL key（INCR/EXPIRE 间失败或历史泄漏）
// 在下一次命中时必须补上 TTL，而不是永生累积计数。
func TestRedisFixedWindowLimitHealsLeakedKeyTTL(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	router := gin.New()
	router.GET("/x", RedisFixedWindowLimit(rdb, "ratelimit:test-heal", 10, time.Minute, false), func(c *gin.Context) { c.Status(200) })

	/* 预置一个已泄漏的 key（计数 5、无 TTL）。 */
	ip := "203.0.113.9"
	windowID := time.Now().UnixNano() / int64(time.Minute)
	leaked := fmt.Sprintf("ratelimit:test-heal:%s:%d", ip, windowID)
	if err := rdb.Set(context.Background(), leaked, 5, 0).Err(); err != nil {
		t.Fatalf("seed leaked key: %v", err)
	}
	if ttl := rdb.TTL(context.Background(), leaked).Val(); ttl != -1 {
		t.Fatalf("seed precondition: leaked key must start without TTL, got %v", ttl)
	}

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("within limit status = %d", rec.Code)
	}

	ttl := rdb.TTL(context.Background(), leaked).Val()
	if ttl <= 0 {
		t.Fatalf("leaked key must regain a TTL on the next hit, got %v", ttl)
	}
}
