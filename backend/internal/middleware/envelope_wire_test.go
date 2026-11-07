package middleware

// #927 wire lock（零 wire 等价证明的前半）：本套特征测试在信封迁移
// **之前**录制并锁死 middleware 每个 inline 错误信封调用点的字节级
// wire 形态（status + 完整 body + Content-Type）。迁移到 response 助手
// 后同一套测试必须原样全绿 —— 这是「headers/status/code/message 逐字
// 节不变」的机械等价证据。
//
// 覆盖口径 = 票面 census 的 42 处信封调用点，其中 40 处直接驱动断言；
// 2 处防御性分支（auth.go:167、interaction.go:58）在当前
// ResolveRuntimeUserStatus / EvaluateInteractionAccess 契约下不可达
// （错误返回时 status 恒为 nil；banned 在上游分支已拦截），且其信封
// 与已覆盖的 auth.go:177 / interaction.go:45 字节相同，此处不重复驱动。

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/observability"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
	"omnicraft/backend/internal/pkg/rediskeys"
)

// assertEnvelope asserts the byte-exact wire form of one envelope call site.
func assertEnvelope(t *testing.T, w *httptest.ResponseRecorder, wantStatus int, wantBody string) {
	t.Helper()
	require.Equal(t, wantStatus, w.Code, "body=%s", w.Body.String())
	require.Equal(t, wantBody, w.Body.String())
	require.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
}

func okHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func serveReq(r *gin.Engine, method, path, authHeader string, body io.Reader) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ---------------------------------------------------------------------------
// auth.go（18 处；:167 见文件头注释）
// ---------------------------------------------------------------------------

func TestWireLockAuthRequired(t *testing.T) {
	t.Run("auth.go:71 missing authorization header", func(t *testing.T) {
		cfg := makeTestConfig()
		r := setupTestRouter()
		r.Use(AuthRequired(cfg, nil))
		r.GET("/test", okHandler)
		w := serveReq(r, http.MethodGet, "/test", "", nil)
		assertEnvelope(t, w, 401, `{"code":"UNAUTHORIZED","message":"authorization header required"}`)
	})

	t.Run("auth.go:78 malformed authorization header", func(t *testing.T) {
		cfg := makeTestConfig()
		r := setupTestRouter()
		r.Use(AuthRequired(cfg, nil))
		r.GET("/test", okHandler)
		w := serveReq(r, http.MethodGet, "/test", "Basic some-token", nil)
		assertEnvelope(t, w, 401, `{"code":"UNAUTHORIZED","message":"invalid authorization header format"}`)
	})

	t.Run("auth.go:119 unparsable jwt", func(t *testing.T) {
		cfg := makeTestConfig()
		r := setupTestRouter()
		r.Use(AuthRequired(cfg, nil))
		r.GET("/test", okHandler)
		w := serveReq(r, http.MethodGet, "/test", "Bearer not-a-jwt", nil)
		assertEnvelope(t, w, 401, `{"code":"UNAUTHORIZED","message":"invalid or expired token"}`)
	})

	t.Run("auth.go:125 refresh token presented as access", func(t *testing.T) {
		cfg := makeTestConfig()
		refresh, err := jwtutil.GenerateRefreshToken(1, "user", cfg.JWT.Secret, 7)
		require.NoError(t, err)
		r := setupTestRouter()
		r.Use(AuthRequired(cfg, nil))
		r.GET("/test", okHandler)
		w := serveReq(r, http.MethodGet, "/test", "Bearer "+refresh, nil)
		assertEnvelope(t, w, 401, `{"code":"UNAUTHORIZED","message":"invalid token type"}`)
	})

	t.Run("auth.go:134 neither redis nor db wired", func(t *testing.T) {
		cfg := makeTestConfig()
		token := makeTestToken(cfg, 1, "user")
		r := setupTestRouter()
		r.Use(AuthRequired(cfg, nil))
		r.GET("/test", okHandler)
		w := serveReq(r, http.MethodGet, "/test", "Bearer "+token, nil)
		assertEnvelope(t, w, 503, `{"code":"AUTH_STATUS_UNAVAILABLE","message":"auth service is temporarily unavailable, please try again later"}`)
	})

	t.Run("auth.go:147 token blacklisted via digest key", func(t *testing.T) {
		cfg := makeTestConfig()
		db := setupTestDB(t)
		insertTestUser(db, 1, "user", false, true, 10, nil)
		token := makeTestToken(cfg, 1, "user")
		mr, err := miniredis.Run()
		require.NoError(t, err)
		defer mr.Close()
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		defer rdb.Close()
		require.NoError(t, rdb.Set(t.Context(), rediskeys.TokenBlacklistKey(token), "1", 0).Err())

		r := setupTestRouter()
		r.Use(AuthRequired(cfg, rdb, db))
		r.GET("/test", okHandler)
		w := serveReq(r, http.MethodGet, "/test", "Bearer "+token, nil)
		assertEnvelope(t, w, 401, `{"code":"UNAUTHORIZED","message":"token has been revoked"}`)
	})

	t.Run("auth.go:152 blacklist redis check fails", func(t *testing.T) {
		cfg := makeTestConfig()
		db := setupTestDB(t)
		insertTestUser(db, 1, "user", false, true, 10, nil)
		token := makeTestToken(cfg, 1, "user")
		rdb := redis.NewClient(&redis.Options{
			Addr:        "127.0.0.1:1",
			DialTimeout: 10 * time.Millisecond,
			ReadTimeout: 10 * time.Millisecond,
		})
		defer rdb.Close()

		r := setupTestRouter()
		r.Use(AuthRequired(cfg, rdb, db))
		r.GET("/test", okHandler)
		w := serveReq(r, http.MethodGet, "/test", "Bearer "+token, nil)
		assertEnvelope(t, w, 503, `{"code":"AUTH_STATUS_UNAVAILABLE","message":"account status is temporarily unavailable"}`)
	})

	t.Run("auth.go:162 user not found", func(t *testing.T) {
		cfg := makeTestConfig()
		db := setupTestDB(t)
		token := makeTestToken(cfg, 99, "user")
		r := setupTestRouter()
		r.Use(AuthRequired(cfg, nil, db))
		r.GET("/test", okHandler)
		w := serveReq(r, http.MethodGet, "/test", "Bearer "+token, nil)
		assertEnvelope(t, w, 401, `{"code":"UNAUTHORIZED","message":"user not found or deleted"}`)
	})

	t.Run("auth.go:171 status resolution fails on db error", func(t *testing.T) {
		cfg := makeTestConfig()
		db := setupTestDB(t)
		require.NoError(t, db.Exec("DROP TABLE users").Error)
		token := makeTestToken(cfg, 1, "user")
		r := setupTestRouter()
		r.Use(AuthRequired(cfg, nil, db))
		r.GET("/test", okHandler)
		w := serveReq(r, http.MethodGet, "/test", "Bearer "+token, nil)
		assertEnvelope(t, w, 503, `{"code":"AUTH_STATUS_UNAVAILABLE","message":"account status is temporarily unavailable"}`)
	})

	t.Run("auth.go:177 banned user on non-whitelisted path", func(t *testing.T) {
		cfg := makeTestConfig()
		db := setupTestDB(t)
		insertTestUser(db, 1, "user", true, true, 10, nil)
		token := makeTestToken(cfg, 1, "user")
		r := setupTestRouter()
		r.Use(AuthRequired(cfg, nil, db))
		r.GET("/test", okHandler)
		w := serveReq(r, http.MethodGet, "/test", "Bearer "+token, nil)
		assertEnvelope(t, w, 401, `{"code":"USER_BANNED","message":"account has been banned"}`)
	})
}

func TestWireLockAuthPATBranch(t *testing.T) {
	t.Run("auth.go:95 per-token PAT rate limit", func(t *testing.T) {
		s := setupPATMiddlewareTest(t, 1)
		token := s.issueToken(t, []string{"download"})
		w, _ := doJSON(t, s.engine, http.MethodGet, "/api/v1/auth/me", s.bearer(token))
		require.Equal(t, http.StatusOK, w.Code)
		w, _ = doJSON(t, s.engine, http.MethodGet, "/api/v1/auth/me", s.bearer(token))
		assertEnvelope(t, w, 429, `{"code":"RATE_LIMIT_EXCEEDED","message":"too many requests, please try again later"}`)
	})

	t.Run("auth.go:107 PAT auth status unavailable", func(t *testing.T) {
		s := setupPATMiddlewareTest(t, 100)
		token := s.issueToken(t, []string{"download"})
		gin.SetMode(gin.TestMode)
		engine := gin.New()
		engine.GET("/api/v1/auth/me", AuthRequired(s.cfg, s.rdb), okHandler)
		w := serveReq(engine, http.MethodGet, "/api/v1/auth/me", "Bearer "+token, nil)
		assertEnvelope(t, w, 503, `{"code":"AUTH_STATUS_UNAVAILABLE","message":"auth service is temporarily unavailable, please try again later"}`)
	})

	t.Run("auth.go:109 banned user via PAT", func(t *testing.T) {
		s := setupPATMiddlewareTest(t, 100)
		token := s.issueToken(t, []string{"download"})
		require.NoError(t, s.db.Exec(`UPDATE users SET is_banned = TRUE WHERE id = ?`, s.userID).Error)
		w, _ := doJSON(t, s.engine, http.MethodGet, "/api/v1/auth/me", s.bearer(token))
		assertEnvelope(t, w, 401, `{"code":"USER_BANNED","message":"account has been banned"}`)
	})

	t.Run("auth.go:111 invalid or revoked PAT", func(t *testing.T) {
		s := setupPATMiddlewareTest(t, 100)
		w, _ := doJSON(t, s.engine, http.MethodGet, "/api/v1/auth/me", "Bearer oc_pat_"+strings.Repeat("q", 43))
		assertEnvelope(t, w, 401, `{"code":"UNAUTHORIZED","message":"invalid or revoked token"}`)
	})

	t.Run("auth.go:361 PAT missing required scope", func(t *testing.T) {
		s := setupPATMiddlewareTest(t, 100)
		downloadOnly := s.issueToken(t, []string{"download"})
		w, _ := doJSON(t, s.engine, http.MethodGet, "/scoped-upload", s.bearer(downloadOnly))
		assertEnvelope(t, w, 403, `{"code":"PAT_SCOPE_REQUIRED","message":"token lacks required scope: upload"}`)
	})

	t.Run("auth.go:374 PAT on JWT-only route", func(t *testing.T) {
		s := setupPATMiddlewareTest(t, 100)
		token := s.issueToken(t, []string{"download", "upload"})
		w, _ := doJSON(t, s.engine, http.MethodGet, "/jwt-only", s.bearer(token))
		assertEnvelope(t, w, 403, `{"code":"JWT_CHANNEL_REQUIRED","message":"this endpoint requires the web session channel"}`)
	})
}

func TestWireLockAdminRequired(t *testing.T) {
	t.Run("auth.go:385 non-admin caller", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.GET("/admin", func(c *gin.Context) {
			c.Set(UserRoleKey, "user")
			c.Next()
		}, AdminRequired(), okHandler)
		w := serveReq(r, http.MethodGet, "/admin", "", nil)
		assertEnvelope(t, w, 403, `{"code":"FORBIDDEN","message":"admin access required"}`)
	})
}

// ---------------------------------------------------------------------------
// interaction.go（10 处；:58 见文件头注释）
// ---------------------------------------------------------------------------

func TestWireLockInteractionRequired(t *testing.T) {
	mount := func(cfg *config.Config, db *gorm.DB, rdb *redis.Client, policy InteractionPolicy, userID int64) *gin.Engine {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.POST("/action", func(c *gin.Context) {
			if userID != 0 {
				c.Set(UserIDKey, userID)
			}
			c.Next()
		}, InteractionRequired(cfg, db, rdb, policy), okHandler)
		return r
	}

	t.Run("interaction.go:26 no authenticated user", func(t *testing.T) {
		cfg := makeTestConfig()
		r := mount(cfg, setupTestDB(t), nil, InteractionPolicy{}, 0)
		w := serveReq(r, http.MethodPost, "/action", "", nil)
		assertEnvelope(t, w, 401, `{"code":"UNAUTHORIZED","message":"authentication required"}`)
	})

	t.Run("interaction.go:35 user not found", func(t *testing.T) {
		cfg := makeTestConfig()
		r := mount(cfg, setupTestDB(t), nil, InteractionPolicy{}, 1)
		w := serveReq(r, http.MethodPost, "/action", "", nil)
		assertEnvelope(t, w, 401, `{"code":"UNAUTHORIZED","message":"user not found or deleted"}`)
	})

	t.Run("interaction.go:39 status resolution fails", func(t *testing.T) {
		cfg := makeTestConfig()
		db := setupTestDB(t)
		require.NoError(t, db.Exec("DROP TABLE users").Error)
		r := mount(cfg, db, nil, InteractionPolicy{}, 1)
		w := serveReq(r, http.MethodPost, "/action", "", nil)
		assertEnvelope(t, w, 503, `{"code":"AUTH_STATUS_UNAVAILABLE","message":"account status is temporarily unavailable"}`)
	})

	t.Run("interaction.go:45 banned user", func(t *testing.T) {
		cfg := makeTestConfig()
		db := setupTestDB(t)
		insertTestUser(db, 1, "user", true, true, 10, nil)
		r := mount(cfg, db, nil, InteractionPolicy{}, 1)
		w := serveReq(r, http.MethodPost, "/action", "", nil)
		assertEnvelope(t, w, 401, `{"code":"USER_BANNED","message":"account has been banned"}`)
	})

	t.Run("interaction.go:54 email not verified", func(t *testing.T) {
		cfg := makeTestConfig()
		db := setupTestDB(t)
		insertTestUser(db, 1, "user", false, false, 10, nil)
		r := mount(cfg, db, nil, InteractionPolicy{RequireVerifiedEmail: true}, 1)
		w := serveReq(r, http.MethodPost, "/action", "", nil)
		assertEnvelope(t, w, 403, `{"code":"EMAIL_NOT_VERIFIED","message":"email verification required"}`)
	})

	t.Run("interaction.go:56 reputation below threshold", func(t *testing.T) {
		cfg := makeTestConfig() // MinScoreForInteraction = 3
		db := setupTestDB(t)
		insertTestUser(db, 1, "user", false, true, 1, nil)
		r := mount(cfg, db, nil, InteractionPolicy{RequireReputation: true}, 1)
		w := serveReq(r, http.MethodPost, "/action", "", nil)
		assertEnvelope(t, w, 403, `{"code":"INSUFFICIENT_REPUTATION","message":"reputation score too low to perform this action"}`)
	})

	t.Run("interaction.go:60 unresolved denial reason (config error)", func(t *testing.T) {
		cfg := makeTestConfig()
		cfg.Reputation.MinScoreForInteraction = 0
		db := setupTestDB(t)
		insertTestUser(db, 1, "user", false, true, 10, nil)
		r := mount(cfg, db, nil, InteractionPolicy{RequireReputation: true}, 1)
		w := serveReq(r, http.MethodPost, "/action", "", nil)
		assertEnvelope(t, w, 503, `{"code":"CONFIG_ERROR","message":"interaction status is temporarily unavailable"}`)
	})

	t.Run("interaction.go:69 freeze check unavailable", func(t *testing.T) {
		cfg := makeTestConfig()
		db := setupTestDB(t)
		insertTestUser(db, 1, "user", false, true, 10, nil)
		r := mount(cfg, db, nil, InteractionPolicy{RequireNoPublishFreeze: true}, 1)
		w := serveReq(r, http.MethodPost, "/action", "", nil)
		assertEnvelope(t, w, 503, `{"code":"AUTH_STATUS_UNAVAILABLE","message":"account status is temporarily unavailable"}`)
	})

	t.Run("interaction.go:74 publish frozen", func(t *testing.T) {
		cfg := makeTestConfig()
		db := setupTestDB(t)
		insertTestUser(db, 1, "user", false, true, 10, nil)
		mr, err := miniredis.Run()
		require.NoError(t, err)
		defer mr.Close()
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		defer rdb.Close()
		require.NoError(t, rdb.Set(t.Context(), rediskeys.PublishFreezeKey(1), "1", 0).Err())

		r := mount(cfg, db, rdb, InteractionPolicy{RequireNoPublishFreeze: true}, 1)
		w := serveReq(r, http.MethodPost, "/action", "", nil)
		assertEnvelope(t, w, 403, `{"code":"PUBLISH_FROZEN","message":"publishing is temporarily frozen due to recent violations"}`)
	})
}

// ---------------------------------------------------------------------------
// ratelimit.go（9 处）
// ---------------------------------------------------------------------------

func brokenRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 10 * time.Millisecond,
		ReadTimeout: 10 * time.Millisecond,
	})
}

func TestWireLockRateLimit(t *testing.T) {
	t.Run("ratelimit.go:50 ip window exceeded", func(t *testing.T) {
		mr, err := miniredis.Run()
		require.NoError(t, err)
		defer mr.Close()
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		defer rdb.Close()
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.GET("/t", RateLimit(rdb, &config.RateLimitConfig{Enabled: true, NormalPerMinute: 1, NormalWindowSec: 60}), okHandler)
		w := serveReq(r, http.MethodGet, "/t", "", nil)
		require.Equal(t, http.StatusOK, w.Code)
		w = serveReq(r, http.MethodGet, "/t", "", nil)
		assertEnvelope(t, w, 429, `{"code":"RATE_LIMIT_EXCEEDED","message":"too many requests, please try again later"}`)
	})

	t.Run("ratelimit.go:101 upload quota exceeded", func(t *testing.T) {
		mr, err := miniredis.Run()
		require.NoError(t, err)
		defer mr.Close()
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		defer rdb.Close()
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.POST("/up", func(c *gin.Context) {
			c.Set(UserIDKey, int64(7))
			c.Next()
		}, UploadRateLimit(rdb, &config.RateLimitConfig{Enabled: true, UploadPerHour: 1, UploadWindowSec: 3600}), okHandler)
		w := serveReq(r, http.MethodPost, "/up", "", nil)
		require.Equal(t, http.StatusOK, w.Code)
		w = serveReq(r, http.MethodPost, "/up", "", nil)
		assertEnvelope(t, w, 429, `{"code":"UPLOAD_RATE_LIMIT_EXCEEDED","message":"upload limit exceeded, please try again later"}`)
	})

	t.Run("ratelimit.go:119 credential limiter without redis", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.POST("/login", CredentialRateLimit(nil, &config.RateLimitConfig{Enabled: true}), okHandler)
		w := serveReq(r, http.MethodPost, "/login", "", nil)
		assertEnvelope(t, w, 503, `{"code":"RATE_LIMIT_UNAVAILABLE","message":"rate limit temporarily unavailable"}`)
	})

	t.Run("ratelimit.go:145 credential limiter redis error", func(t *testing.T) {
		rdb := brokenRedisClient()
		defer rdb.Close()
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.POST("/login", CredentialRateLimit(rdb, &config.RateLimitConfig{Enabled: true, CredentialPerMinute: 5}), okHandler)
		w := serveReq(r, http.MethodPost, "/login", "", nil)
		assertEnvelope(t, w, 503, `{"code":"RATE_LIMIT_UNAVAILABLE","message":"rate limit temporarily unavailable"}`)
	})

	t.Run("ratelimit.go:151 credential attempts exceeded", func(t *testing.T) {
		mr, err := miniredis.Run()
		require.NoError(t, err)
		defer mr.Close()
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		defer rdb.Close()
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.POST("/login", CredentialRateLimit(rdb, &config.RateLimitConfig{Enabled: true, CredentialPerMinute: 1, NormalWindowSec: 60}), okHandler)
		body := strings.NewReader(`{"email":"a@example.com"}`)
		w := serveReq(r, http.MethodPost, "/login", "", body)
		require.Equal(t, http.StatusOK, w.Code)
		w = serveReq(r, http.MethodPost, "/login", "", strings.NewReader(`{"email":"a@example.com"}`))
		assertEnvelope(t, w, 429, `{"code":"CREDENTIAL_RATE_LIMIT_EXCEEDED","message":"too many credential attempts, please try again later"}`)
	})

	t.Run("ratelimit.go:174 fixed window fail-closed without redis", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.GET("/s", RedisFixedWindowLimit(nil, "rl:test", 2, time.Minute, true), okHandler)
		w := serveReq(r, http.MethodGet, "/s", "", nil)
		assertEnvelope(t, w, 503, `{"code":"RATE_LIMIT_UNAVAILABLE","message":"rate limit temporarily unavailable"}`)
	})

	t.Run("ratelimit.go:190 fixed window fail-closed on redis error", func(t *testing.T) {
		rdb := brokenRedisClient()
		defer rdb.Close()
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.GET("/s", RedisFixedWindowLimit(rdb, "rl:test", 2, time.Minute, true), okHandler)
		w := serveReq(r, http.MethodGet, "/s", "", nil)
		assertEnvelope(t, w, 503, `{"code":"RATE_LIMIT_UNAVAILABLE","message":"rate limit temporarily unavailable"}`)
	})

	t.Run("ratelimit.go:199 fixed window exceeded", func(t *testing.T) {
		mr, err := miniredis.Run()
		require.NoError(t, err)
		defer mr.Close()
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		defer rdb.Close()
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.GET("/s", RedisFixedWindowLimit(rdb, "rl:test", 1, time.Minute, false), okHandler)
		w := serveReq(r, http.MethodGet, "/s", "", nil)
		require.Equal(t, http.StatusOK, w.Code)
		w = serveReq(r, http.MethodGet, "/s", "", nil)
		assertEnvelope(t, w, 429, `{"code":"RATE_LIMIT_EXCEEDED","message":"too many requests, please try again later"}`)
	})

	t.Run("ratelimit.go:258 comment edit limit exceeded", func(t *testing.T) {
		mr, err := miniredis.Run()
		require.NoError(t, err)
		defer mr.Close()
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		defer rdb.Close()
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.POST("/c", func(c *gin.Context) {
			c.Set(UserIDKey, int64(7))
			c.Next()
		}, CommentEditRateLimit(rdb), okHandler)
		var w *httptest.ResponseRecorder
		for i := 0; i < 5; i++ {
			w = serveReq(r, http.MethodPost, "/c", "", nil)
			require.Equal(t, http.StatusOK, w.Code, "request %d within limit", i+1)
		}
		w = serveReq(r, http.MethodPost, "/c", "", nil)
		assertEnvelope(t, w, 429, `{"code":"COMMENT_EDIT_RATE_LIMIT","message":"comment edit rate limit exceeded, please try again later"}`)
	})
}

// ---------------------------------------------------------------------------
// validation.go（2 处）、body_limit.go（1 处）、csrf.go（1 处）、
// panic_recovery.go（1 处）
// ---------------------------------------------------------------------------

type wireProbePayload struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=3"`
}

func TestWireLockValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/submit", BindAndValidate(&wireProbePayload{}), okHandler)

	t.Run("validation.go:81 unparsable request body", func(t *testing.T) {
		w := serveReq(r, http.MethodPost, "/submit", "", strings.NewReader("not-json"))
		assertEnvelope(t, w, 400, `{"code":"VALIDATION_ERROR","message":"invalid request body"}`)
	})

	t.Run("validation.go:97 binding validation failed with details", func(t *testing.T) {
		w := serveReq(r, http.MethodPost, "/submit", "", strings.NewReader(`{"email":"not-an-email","username":"ab"}`))
		assertEnvelope(t, w, 400, `{"code":"VALIDATION_ERROR","details":[{"field":"Email","message":"must be a valid email address"},{"field":"Username","message":"must be at least 3 characters"}],"message":"validation failed"}`)
	})
}

func TestWireLockBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/upload", BodyLimit(16), okHandler)
	w := serveReq(r, http.MethodPost, "/upload", "", strings.NewReader("0123456789abcdefghijklmnop"))
	assertEnvelope(t, w, 413, `{"code":"REQUEST_BODY_TOO_LARGE","message":"request body is too large"}`)
}

func TestWireLockCSRF(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Mode: "debug"}}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/write", CSRF(cfg), okHandler)
	w := serveReq(r, http.MethodPost, "/write", "", nil)
	assertEnvelope(t, w, 403, `{"code":"CSRF_TOKEN_INVALID","message":"CSRF token missing or invalid"}`)
}

func TestWireLockPanicRecovery(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(PanicRecovery(logger, observability.NewMetrics()))
	r.GET("/boom", func(c *gin.Context) {
		panic("wire-lock probe")
	})
	w := serveReq(r, http.MethodGet, "/boom", "", nil)
	assertEnvelope(t, w, 500, `{"code":"INTERNAL_ERROR","message":"internal server error"}`)
}
