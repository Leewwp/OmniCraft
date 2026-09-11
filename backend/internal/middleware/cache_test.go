package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
)

// SP-16 #448: the public cache contract applies to anonymous JSON GETs only.
func TestCacheableAnonymousGETContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.JWT.Secret = "cache-middleware-test-secret"
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&model.User{ID: 42, Email: "cache-test@example.com", Username: "cache_test", PasswordHash: "x", Role: "user", Reputation: 10}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	optAuth := OptionalAuth(cfg, nil, db)

	newRouter := func() *gin.Engine {
		r := gin.New()
		r.GET("/api/v1/things", optAuth, CacheableAnonymousGET(300), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		return r
	}

	t.Run("anonymous GET gains s-maxage and etag, revalidates as 304", func(t *testing.T) {
		r := newRouter()
		first := httptest.NewRecorder()
		r.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/v1/things", nil))
		if first.Code != http.StatusOK || first.Body.String() == "" {
			t.Fatalf("first response broken: %d %q", first.Code, first.Body.String())
		}
		cc := first.Header().Get("Cache-Control")
		etag := first.Header().Get("ETag")
		if cc != "public, max-age=60, s-maxage=300" {
			t.Fatalf("Cache-Control = %q", cc)
		}
		if etag == "" {
			t.Fatal("ETag missing")
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/things", nil)
		req.Header.Set("If-None-Match", etag)
		second := httptest.NewRecorder()
		r.ServeHTTP(second, req)
		if second.Code != http.StatusNotModified {
			t.Fatalf("revalidation status = %d, want 304", second.Code)
		}
		if second.Body.Len() != 0 {
			t.Fatalf("304 body = %q, want empty", second.Body.String())
		}
	})

	t.Run("authenticated GET is not publicly cacheable", func(t *testing.T) {
		r := newRouter()
		pair, err := jwtutil.GenerateTokenPair(42, "user", cfg.JWT.Secret, 120, 7)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/things", nil)
		req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		if rec.Header().Get("Cache-Control") == "public, max-age=60, s-maxage=300" {
			t.Fatal("viewer-dependent response must not carry the public cache directive")
		}
		if rec.Header().Get("ETag") != "" {
			t.Fatal("viewer-dependent response must not gain a content-hash ETag")
		}
	})
}
