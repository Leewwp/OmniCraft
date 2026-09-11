package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"omnicraft/backend/internal/testutil"
)

type patTestStack struct {
	engine *gin.Engine
	db     *gorm.DB
	rdb    *redis.Client
	cfg    *config.Config
	svc    *service.AgentAccessTokenService
	userID int64
}

// setupPATMiddlewareTest mounts the PAT-aware auth middleware on probe routes
// that echo the resolved identity, mirroring how routes.go wires AuthRequired
// (cfg, rdb, db) plus the CSRF middleware for the exemption checks.
func setupPATMiddlewareTest(t *testing.T, rateLimitPerMinute int) *patTestStack {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			username VARCHAR(64) UNIQUE NOT NULL,
			role VARCHAR(20) NOT NULL DEFAULT 'user',
			is_banned BOOLEAN NOT NULL DEFAULT FALSE,
			email_verified_at TIMESTAMPTZ,
			reputation INT NOT NULL DEFAULT 10,
			deleted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "079_agent_access_tokens.sql"))

	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })

	cfg := &config.Config{
		JWT:         config.JWTConfig{Secret: "pat-middleware-test-secret"},
		RateLimit:   config.RateLimitConfig{Enabled: true, PATPerMinute: rateLimitPerMinute, PATWindowSec: 60},
		AgentAccess: config.AgentAccessConfig{MaxTokensPerUser: 10},
		Server:      config.ServerConfig{Mode: "debug"},
	}

	svc := service.NewAgentAccessTokenService(repository.NewAgentAccessTokenRepository(db), cfg)

	var userID int64
	require.NoError(t, db.Raw(`
		INSERT INTO users (email, username, email_verified_at, reputation)
		VALUES ('patmw@example.com', 'patmw', NOW(), 10) RETURNING id
	`).Scan(&userID).Error)

	engine := gin.New()
	engine.Use(func(c *gin.Context) { c.Next() })
	echoIdentity := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id":  GetUserID(c),
			"role":     c.GetString(UserRoleKey),
			"channel":  GetAuthChannel(c),
			"scopes":   GetPATScopes(c),
			"token_id": c.GetInt64("patTokenID"),
			"has_dl":   HasPATScope(c, "download"),
			"has_ul":   HasPATScope(c, "upload"),
		})
	}
	authReq := AuthRequired(cfg, rdb, db)
	optAuth := OptionalAuth(cfg, rdb, db)

	engine.GET("/api/v1/auth/me", authReq, echoIdentity)
	engine.GET("/opt", optAuth, echoIdentity)
	engine.GET("/scoped-upload", authReq, RequireScopeForPAT("upload"), echoIdentity)
	engine.GET("/scoped-download", authReq, RequireScopeForPAT("download"), echoIdentity)
	engine.GET("/jwt-only", authReq, RequireJWTChannel(), echoIdentity)
	engine.POST("/write", CSRF(cfg), authReq, echoIdentity)
	engine.GET("/write-get", CSRF(cfg), authReq, echoIdentity)

	return &patTestStack{engine: engine, db: db, rdb: rdb, cfg: cfg, svc: svc, userID: userID}
}

func (s *patTestStack) issueToken(t *testing.T, scopes []string) string {
	t.Helper()
	issued, err := s.svc.Issue(t.Context(), s.userID, "middleware test token", scopes)
	require.NoError(t, err)
	return issued.Token
}

func (s *patTestStack) bearer(token string) string {
	return "Bearer " + token
}

func doJSON(t *testing.T, engine *gin.Engine, method, path, authHeader string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	body := map[string]any{}
	if strings.HasPrefix(w.Body.String(), "{") {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	}
	return w, body
}

func TestAuthRequiredAcceptsPAT(t *testing.T) {
	s := setupPATMiddlewareTest(t, 100)
	token := s.issueToken(t, []string{"download"})

	w, body := doJSON(t, s.engine, http.MethodGet, "/api/v1/auth/me", s.bearer(token))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, float64(s.userID), body["user_id"])
	require.Equal(t, "pat", body["channel"])
	require.Equal(t, []any{"download"}, body["scopes"])
	require.Equal(t, true, body["has_dl"])
	require.Equal(t, false, body["has_ul"])

	// PAT requests must not leave session traces: no Set-Cookie at all
	// (the CSRF middleware is not on this route, and the PAT branch never
	// issues one).
	require.Empty(t, w.Header().Values("Set-Cookie"))
}

func TestAuthRequiredRejectsBadPAT(t *testing.T) {
	s := setupPATMiddlewareTest(t, 100)
	token := s.issueToken(t, []string{"download"})

	// revoked token stops working immediately
	require.NoError(t, s.svc.Revoke(t.Context(), s.userID, 1))
	w, _ := doJSON(t, s.engine, http.MethodGet, "/api/v1/auth/me", s.bearer(token))
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// malformed / unknown tokens
	for _, bad := range []string{
		"Bearer oc_pat_short",
		"Bearer oc_pat_" + strings.Repeat("x", 43),
		"Bearer not-a-token",
	} {
		w, _ = doJSON(t, s.engine, http.MethodGet, "/api/v1/auth/me", bad)
		require.Equal(t, http.StatusUnauthorized, w.Code, "auth header %q", bad)
	}
}

func TestAuthRequiredBannedUserPATFailsImmediately(t *testing.T) {
	s := setupPATMiddlewareTest(t, 100)
	token := s.issueToken(t, []string{"download"})

	require.NoError(t, s.db.Exec(`UPDATE users SET is_banned = TRUE WHERE id = ?`, s.userID).Error)

	// The banned whitelist (appeals self-service) is a web-session concept:
	// a banned user's PAT is dead on every route, including /auth/me.
	w, _ := doJSON(t, s.engine, http.MethodGet, "/api/v1/auth/me", s.bearer(token))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthRequiredPATDeletedUserFails(t *testing.T) {
	s := setupPATMiddlewareTest(t, 100)
	token := s.issueToken(t, []string{"download"})

	require.NoError(t, s.db.Exec(`UPDATE users SET deleted_at = NOW() WHERE id = ?`, s.userID).Error)
	w, _ := doJSON(t, s.engine, http.MethodGet, "/api/v1/auth/me", s.bearer(token))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOptionalAuthResolvesPATAndDegradesGracefully(t *testing.T) {
	s := setupPATMiddlewareTest(t, 100)
	token := s.issueToken(t, []string{"download", "upload"})

	w, body := doJSON(t, s.engine, http.MethodGet, "/opt", s.bearer(token))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, float64(s.userID), body["user_id"])
	require.Equal(t, "pat", body["channel"])

	// a bad PAT degrades to anonymous exactly like a bad JWT
	w, body = doJSON(t, s.engine, http.MethodGet, "/opt", "Bearer oc_pat_"+strings.Repeat("q", 43))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, float64(0), body["user_id"])

	// no header at all stays anonymous
	w, body = doJSON(t, s.engine, http.MethodGet, "/opt", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, float64(0), body["user_id"])
}

func TestJWTPathUnchangedAlongsidePAT(t *testing.T) {
	s := setupPATMiddlewareTest(t, 100)
	jwtToken, err := jwtutil.GenerateAccessToken(s.userID, "user", s.cfg.JWT.Secret, 15)
	require.NoError(t, err)

	w, body := doJSON(t, s.engine, http.MethodGet, "/api/v1/auth/me", s.bearer(jwtToken))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, float64(s.userID), body["user_id"])
	require.Equal(t, "jwt", body["channel"])
	require.Nil(t, body["scopes"])
}

func TestPATPerTokenRateLimit(t *testing.T) {
	s := setupPATMiddlewareTest(t, 2)
	token := s.issueToken(t, []string{"download"})

	for i := 0; i < 2; i++ {
		w, _ := doJSON(t, s.engine, http.MethodGet, "/api/v1/auth/me", s.bearer(token))
		require.Equal(t, http.StatusOK, w.Code, "request %d within limit", i+1)
	}
	w, body := doJSON(t, s.engine, http.MethodGet, "/api/v1/auth/me", s.bearer(token))
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Equal(t, "RATE_LIMIT_EXCEEDED", body["code"])

	// a different token has its own window
	other := s.issueToken(t, []string{"download"})
	w, _ = doJSON(t, s.engine, http.MethodGet, "/api/v1/auth/me", s.bearer(other))
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRequireScopeForPAT(t *testing.T) {
	s := setupPATMiddlewareTest(t, 100)
	downloadOnly := s.issueToken(t, []string{"download"})
	uploadOnly := s.issueToken(t, []string{"upload"})
	jwtToken, err := jwtutil.GenerateAccessToken(s.userID, "user", s.cfg.JWT.Secret, 15)
	require.NoError(t, err)

	// upload-scope route: download-only PAT is rejected, upload passes
	w, body := doJSON(t, s.engine, http.MethodGet, "/scoped-upload", s.bearer(downloadOnly))
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Equal(t, "PAT_SCOPE_REQUIRED", body["code"])
	w, _ = doJSON(t, s.engine, http.MethodGet, "/scoped-upload", s.bearer(uploadOnly))
	require.Equal(t, http.StatusOK, w.Code)

	// download-scope route mirrors it
	w, _ = doJSON(t, s.engine, http.MethodGet, "/scoped-download", s.bearer(uploadOnly))
	require.Equal(t, http.StatusForbidden, w.Code)
	w, _ = doJSON(t, s.engine, http.MethodGet, "/scoped-download", s.bearer(downloadOnly))
	require.Equal(t, http.StatusOK, w.Code)

	// JWT sessions are never scope-gated by the PAT guard
	w, _ = doJSON(t, s.engine, http.MethodGet, "/scoped-upload", s.bearer(jwtToken))
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRequireJWTChannel(t *testing.T) {
	s := setupPATMiddlewareTest(t, 100)
	token := s.issueToken(t, []string{"download", "upload"})
	jwtToken, err := jwtutil.GenerateAccessToken(s.userID, "user", s.cfg.JWT.Secret, 15)
	require.NoError(t, err)

	// PAT must never mint more PATs (privilege escalation vector)
	w, body := doJSON(t, s.engine, http.MethodGet, "/jwt-only", s.bearer(token))
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Equal(t, "JWT_CHANNEL_REQUIRED", body["code"])

	w, _ = doJSON(t, s.engine, http.MethodGet, "/jwt-only", s.bearer(jwtToken))
	require.Equal(t, http.StatusOK, w.Code)
}

func TestCSRFMiddlewareSkipsPATBearer(t *testing.T) {
	s := setupPATMiddlewareTest(t, 100)
	token := s.issueToken(t, []string{"upload"})

	// A machine client carries no csrf cookie; the PAT bearer prefix is the
	// explicit-credential signal that makes CSRF checks unnecessary.
	w, body := doJSON(t, s.engine, http.MethodPost, "/write", s.bearer(token))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "pat", body["channel"])
	require.Empty(t, w.Header().Values("Set-Cookie"), "PAT responses must not carry cookies")

	// GETs through CSRF with PAT likewise leave no cookie trace
	w, _ = doJSON(t, s.engine, http.MethodGet, "/write-get", s.bearer(token))
	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, w.Header().Values("Set-Cookie"))

	// non-PAT requests keep the full CSRF contract
	req := httptest.NewRequest(http.MethodPost, "/write", nil)
	w2 := httptest.NewRecorder()
	s.engine.ServeHTTP(w2, req)
	require.Equal(t, http.StatusForbidden, w2.Code)
}

func TestPATAuthFailsClosedWithoutDB(t *testing.T) {
	stack := setupPATMiddlewareTest(t, 100)
	token := stack.issueToken(t, []string{"download"})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/v1/auth/me", AuthRequired(stack.cfg, stack.rdb), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": GetUserID(c)})
	})
	w, _ := doJSON(t, engine, http.MethodGet, "/api/v1/auth/me", stack.bearer(token))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}
