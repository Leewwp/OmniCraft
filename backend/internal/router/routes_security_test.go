package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/container"
	"omnicraft/backend/internal/model"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/pkg/mail"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

func TestCredentialRoutesAreRateLimitedWhileCSRFRouteIsNot(t *testing.T) {
	t.Run("register", func(t *testing.T) {
		assertCredentialRouteRateLimited(t, "/api/v1/auth/register", `{"email":"register-rate@example.com"}`)
	})
	t.Run("login", func(t *testing.T) {
		assertCredentialRouteRateLimited(t, "/api/v1/auth/login", `{"email":"login-rate@example.com"}`)
	})
	t.Run("forgot-password", func(t *testing.T) {
		assertCredentialRouteRateLimited(t, "/api/v1/auth/forgot-password", `{"email":"forgot-rate@example.com"}`)
	})

	router, _, cleanup := buildRoutesSecurityRouter(t)
	defer cleanup()

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/csrf", nil)
		req.RemoteAddr = "198.51.100.80:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("csrf request %d status = %d, want 200; body = %s", i+1, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "RATE_LIMIT") {
			t.Fatalf("csrf request %d body = %s, route should not be credential-rate-limited", i+1, rec.Body.String())
		}
	}
}

func TestAgentChatRouteAppliesRuntimeAgentQuota(t *testing.T) {
	router, cfg, cleanup := buildRoutesSecurityRouter(t)
	defer cleanup()

	token := makeRoutesSecurityToken(cfg, 1, "user")
	valid := `{"message":"hi"}`

	// 1) Request-schema rejection happens BEFORE reservation: malformed bodies
	// never consume quota and never reach the Provider.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/chat/stream", strings.NewReader(`{"messages":`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.RemoteAddr = "198.51.100.90:12345"

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("malformed request %d status = %d, want 400; body = %s", i+1, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "VALIDATION_ERROR") {
			t.Fatalf("malformed request %d body = %s, want VALIDATION_ERROR", i+1, rec.Body.String())
		}
	}

	// 2) A schema-valid request reserves the day quota and streams.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/chat/stream", strings.NewReader(valid))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.RemoteAddr = "198.51.100.90:12345"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid request status = %d, want 200 SSE; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("valid request Content-Type = %q, want text/event-stream", rec.Header().Get("Content-Type"))
	}

	// 3) The next admitted request exceeds the day limit (rate_limit_per_day=1)
	// and is rejected before any Provider work.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/agent/chat/stream", strings.NewReader(valid))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.RemoteAddr = "198.51.100.90:12345"
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("third agent request status = %d, want 429; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AGENT_RATE_LIMIT_EXCEEDED") {
		t.Fatalf("third agent request body = %s, want AGENT_RATE_LIMIT_EXCEEDED", rec.Body.String())
	}
}

func TestAgentRoutesRequireAuthentication(t *testing.T) {
	router, _, cleanup := buildRoutesSecurityRouter(t)
	defer cleanup()

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/agent/chat/stream", `{"messages":[{"role":"user","content":"hi"}]}`},
		{http.MethodGet, "/api/v1/agent/conversations", ""},
		{http.MethodDelete, "/api/v1/agent/conversations/1", ""},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s without token status = %d, want 401; body = %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "UNAUTHORIZED") {
			t.Fatalf("%s %s body = %s, want UNAUTHORIZED", tc.method, tc.path, rec.Body.String())
		}
	}
}

func TestAgentConversationDeleteRouteDoesNotConsumeQuota(t *testing.T) {
	router, cfg, cleanup := buildRoutesSecurityRouter(t)
	defer cleanup()

	token := makeRoutesSecurityToken(cfg, 1, "user")

	// Deleting a missing/foreign conversation is idempotent 204 and must not
	// consume any generation quota (rate_limit_per_day=1 stays available).
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/agent/conversations/999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE missing conversation status = %d, want idempotent 204; body = %s", rec.Code, rec.Body.String())
	}

	valid := `{"message":"hi"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/agent/chat/stream", strings.NewReader(valid))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("chat after delete status = %d, want 200 (delete must not consume quota); body = %s", rec.Code, rec.Body.String())
	}
}

func TestDisabledDesktopAndPaymentRoutesReturnExistingContract(t *testing.T) {
	router, _, cleanup := buildRoutesSecurityRouter(t)
	defer cleanup()

	tests := []struct {
		method  string
		path    string
		message string
	}{
		{method: http.MethodPost, path: "/api/v1/deploy-grants", message: "desktop deploy is not enabled"},
		{method: http.MethodPost, path: "/api/v1/payments/checkout", message: "payment is not enabled"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503; body = %s", rec.Code, rec.Body.String())
			}
			if body := rec.Body.String(); !strings.Contains(body, `"code":"FEATURE_DISABLED"`) || !strings.Contains(body, tt.message) {
				t.Fatalf("body = %s, want FEATURE_DISABLED and %q", body, tt.message)
			}
		})
	}
}

func assertCredentialRouteRateLimited(t *testing.T, path, body string) {
	t.Helper()

	router, _, cleanup := buildRoutesSecurityRouter(t)
	defer cleanup()

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "198.51.100.70:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if i == 0 {
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s first request status = %d, want 400; body = %s", path, rec.Code, rec.Body.String())
			}
			continue
		}

		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("%s second request status = %d, want 429; body = %s", path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "CREDENTIAL_RATE_LIMIT_EXCEEDED") {
			t.Fatalf("%s second request body = %s, want CREDENTIAL_RATE_LIMIT_EXCEEDED", path, rec.Body.String())
		}
	}
}

func buildRoutesSecurityRouter(t *testing.T) (*gin.Engine, *config.Config, func()) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		mr.Close()
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		_ = rdb.Close()
		mr.Close()
		t.Fatalf("migrate users: %v", err)
	}
	if err := db.AutoMigrate(&model.AgentConversation{}, &model.AgentMessage{}); err != nil {
		_ = rdb.Close()
		mr.Close()
		t.Fatalf("migrate agent tables: %v", err)
	}

	verifiedAt := time.Now()
	user := model.User{
		ID:              1,
		Email:           "agent-route@example.com",
		PasswordHash:    "hash",
		Username:        "agent-route",
		Role:            "user",
		Reputation:      10,
		EmailVerifiedAt: &verifiedAt,
	}
	if err := db.Create(&user).Error; err != nil {
		_ = rdb.Close()
		mr.Close()
		t.Fatalf("create route test user: %v", err)
	}

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:          "route-security-secret",
			AccessTokenTTL:  7200,
			RefreshTokenTTL: 604800,
		},
		Server: config.ServerConfig{
			Mode: "debug",
		},
		RateLimit: config.RateLimitConfig{
			Enabled:              true,
			CredentialPerMinute:  1,
			NormalWindowSec:      60,
			AgentWindowSec:       60,
			AgentMinuteWindowSec: 60,
		},
		Agent: config.AgentConfig{
			WebAgentEnabled:        true,
			RateLimitPerDay:        1,
			RateLimitPerMinute:     5,
			MaxUserMessageChars:    4000,
			ChatMaxContextMsgs:     10,
			ChatContextTokenBudget: 100000,
			MaxToolCallsPerTurn:    8,
			MaxOutputTokens:        1200,
			CitationMaxCount:       5,
		},
		Reputation: config.ReputationConfig{
			MinScoreForInteraction: 3,
		},
		Cache: config.CacheConfig{
			UserStatusTTL:    300,
			PublishFreezeTTL: 604800,
		},
	}

	userRepo := repository.NewUserRepository(db)
	authSvc := service.NewAuthService(userRepo, rdb, cfg)
	verificationSvc := service.NewVerificationService(userRepo, rdb, mail.NewLoggerSender(), cfg)
	ctr := &container.ServiceContainer{
		DB:                  db,
		RDB:                 rdb,
		Cfg:                 cfg,
		UserRepo:            userRepo,
		AuthService:         authSvc,
		VerificationService: verificationSvc,
		QueueProducer:       queue.NewNoopProducer(),
		AgentService: service.NewAgentService(
			&routeFakeAgentProvider{},
			nil,
			repository.NewContentRepository(db),
			nil,
			db,
			cfg,
		),
	}

	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterRoutes(v1, cfg, ctr)

	cleanup := func() {
		_ = rdb.Close()
		mr.Close()
	}
	return router, cfg, cleanup
}

func makeRoutesSecurityToken(cfg *config.Config, userID int64, role string) string {
	pair, err := jwtutil.GenerateTokenPair(userID, role, cfg.JWT.Secret, 120, 7)
	if err != nil {
		panic(err)
	}
	return pair.AccessToken
}

// routeFakeAgentProvider keeps router-level tests hermetic: the chat stream
// succeeds without any external Provider network call.
type routeFakeAgentProvider struct{}

func (p *routeFakeAgentProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{}, nil
}

func (p *routeFakeAgentProvider) ChatStream(_ context.Context, _ llm.ChatRequest, handler func(delta llm.ChatDelta) error) error {
	return handler(llm.ChatDelta{Content: "ok", Done: true})
}

func (p *routeFakeAgentProvider) GetEmbedding(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1}, nil
}

func readRoutesSource(t *testing.T) string {
	t.Helper()
	bytes, err := os.ReadFile(filepath.Join(".", "routes.go"))
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}
	return string(bytes)
}
