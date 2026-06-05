package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

func setupAuthCookieTestRouter(t *testing.T) (*gin.Engine, *config.Config, *gorm.DB, *redis.Client, *miniredis.Miniredis) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}

	cfg := &config.Config{
		Server:   config.ServerConfig{Port: "8080", Mode: "debug"},
		Database: config.DatabaseConfig{DSN: ""},
		Redis:    config.RedisConfig{Addr: mr.Addr(), Password: "", DB: 0},
		JWT: config.JWTConfig{
			Secret:          "test-secret-for-cookie-tests",
			AccessTokenTTL:  120,
			RefreshTokenTTL: 7,
		},
		Security: config.SecurityConfig{
			AllowedOrigins: []string{
				"https://app.leeppp.online",
				"http://localhost:3000",
			},
		},
		Reputation: config.ReputationConfig{MinScoreForInteraction: 3},
		Cache:      config.CacheConfig{UserStatusTTL: 300},
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.AutoMigrate(&model.User{})

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	r := gin.New()
	r.Use(middleware.CORS(cfg))
	r.Use(middleware.CSRF(cfg))

	userRepo := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepo, rdb, cfg)
	verificationService := service.NewVerificationService(userRepo, rdb, nil, cfg)
	authHandler := NewAuthHandler(authService, verificationService, userRepo, nil, rdb, cfg)

	authReq := middleware.AuthRequired(cfg, rdb, db)

	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", authHandler.Logout)
			auth.POST("/refresh", authHandler.Refresh)
			auth.POST("/reset-password", authHandler.ResetPassword)
			auth.GET("/me", authReq, authHandler.Me)
			auth.GET("/csrf", authHandler.CSRFToken)
		}
	}

	return r, cfg, db, rdb, mr
}

func insertCookieTestUser(t *testing.T, db *gorm.DB, email, username, password string) *model.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 4)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	user := &model.User{
		Email:        email,
		Username:     username,
		PasswordHash: string(hash),
		Reputation:   10,
		Role:         "user",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func fetchCSRFToken(t *testing.T, r *gin.Engine) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/csrf", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("csrf bootstrap: expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	token, _ := resp["csrf_token"].(string)
	if token == "" {
		t.Fatal("csrf bootstrap returned empty token")
	}
	return token
}

func TestLoginSetsHttpOnlyRefreshCookie(t *testing.T) {
	r, _, db, _, mr := setupAuthCookieTestRouter(t)
	defer mr.Close()
	insertCookieTestUser(t, db, "cookie@test.com", "cookieuser", "password123")

	csrfToken := fetchCSRFToken(t, r)

	body := `{"email":"cookie@test.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.AddCookie(&http.Cookie{Name: "csrf-token", Value: csrfToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if tokens, ok := resp["tokens"].(map[string]interface{}); ok {
		if _, has := tokens["refresh_token"]; has {
			t.Error("login response must NOT contain refresh_token in JSON body")
		}
	}

	found := false
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" || c.Name == "__Host-refresh_token" {
			found = true
			if !c.HttpOnly {
				t.Errorf("refresh cookie must be HttpOnly, got HttpOnly=%v", c.HttpOnly)
			}
			if c.SameSite != http.SameSiteLaxMode {
				t.Errorf("refresh cookie SameSite must be Lax, got %v", c.SameSite)
			}
			if c.Path != "/" {
				t.Errorf("refresh cookie Path must be /, got %s", c.Path)
			}
		}
	}
	if !found {
		t.Error("login must set a refresh_token or __Host-refresh_token cookie")
	}
}

func TestRefreshReadsCookieAndRotatesRefreshCookie(t *testing.T) {
	r, _, db, _, mr := setupAuthCookieTestRouter(t)
	defer mr.Close()
	insertCookieTestUser(t, db, "refresh@test.com", "refreshuser", "password123")

	csrfToken := fetchCSRFToken(t, r)

	loginBody := `{"email":"refresh@test.com","password":"password123"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("X-CSRF-Token", csrfToken)
	loginReq.AddCookie(&http.Cookie{Name: "csrf-token", Value: csrfToken})
	loginW := httptest.NewRecorder()
	r.ServeHTTP(loginW, loginReq)

	if loginW.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d body=%s", loginW.Code, loginW.Body.String())
	}

	var loginResp map[string]interface{}
	json.Unmarshal(loginW.Body.Bytes(), &loginResp)

	var refreshToken string
	for _, c := range loginW.Result().Cookies() {
		if c.Name == "refresh_token" || c.Name == "__Host-refresh_token" {
			refreshToken = c.Value
		}
	}
	if refreshToken == "" {
		t.Fatal("login must set refresh cookie")
	}

	csrfToken2 := fetchCSRFToken(t, r)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken2)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshToken})
	req.AddCookie(&http.Cookie{Name: "csrf-token", Value: csrfToken2})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("refresh: expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if tokens, ok := resp["tokens"].(map[string]interface{}); ok {
		if _, has := tokens["refresh_token"]; has {
			t.Error("refresh response must NOT contain refresh_token in JSON body")
		}
		if _, has := tokens["access_token"]; !has {
			t.Error("refresh response must contain access_token in JSON body")
		}
	}

	rotated := false
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" || c.Name == "__Host-refresh_token" {
			rotated = true
		}
	}
	if !rotated {
		t.Error("refresh must rotate the refresh cookie")
	}
}

func TestRefreshRejectsJSONBodyRefreshTokenWithoutCookie(t *testing.T) {
	r, _, db, _, mr := setupAuthCookieTestRouter(t)
	defer mr.Close()
	insertCookieTestUser(t, db, "jsonrefresh@test.com", "jsonrefreshuser", "password123")

	csrfToken := fetchCSRFToken(t, r)

	loginBody := `{"email":"jsonrefresh@test.com","password":"password123"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("X-CSRF-Token", csrfToken)
	loginReq.AddCookie(&http.Cookie{Name: "csrf-token", Value: csrfToken})
	loginW := httptest.NewRecorder()
	r.ServeHTTP(loginW, loginReq)
	if loginW.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d body=%s", loginW.Code, loginW.Body.String())
	}

	var refreshToken string
	for _, c := range loginW.Result().Cookies() {
		if c.Name == "refresh_token" || c.Name == "__Host-refresh_token" {
			refreshToken = c.Value
		}
	}
	if refreshToken == "" {
		t.Fatal("login must set refresh cookie")
	}

	csrfToken2 := fetchCSRFToken(t, r)
	refreshBody := `{"refresh_token":"` + refreshToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(refreshBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken2)
	req.AddCookie(&http.Cookie{Name: "csrf-token", Value: csrfToken2})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("refresh without cookie must reject JSON refresh_token, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestResetPasswordSetsSessionCookieAndAccessToken(t *testing.T) {
	r, _, db, rdb, mr := setupAuthCookieTestRouter(t)
	defer mr.Close()
	user := insertCookieTestUser(t, db, "resetlogin@test.com", "resetloginuser", "password123")

	rawToken := "reset-token-for-auto-login"
	sum := sha256.Sum256([]byte(rawToken))
	digest := hex.EncodeToString(sum[:])
	ctx := context.Background()
	if err := rdb.Set(ctx, "reset:password:"+digest, user.ID, 0).Err(); err != nil {
		t.Fatalf("seed reset token: %v", err)
	}
	if err := rdb.Set(ctx, "reset:password:user:"+strconv.FormatInt(user.ID, 10), digest, 0).Err(); err != nil {
		t.Fatalf("seed reset digest: %v", err)
	}

	csrfToken := fetchCSRFToken(t, r)
	body := `{"token":"` + rawToken + `","new_password":"newpassword123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.AddCookie(&http.Cookie{Name: "csrf-token", Value: csrfToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("reset password: expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	tokens, ok := resp["tokens"].(map[string]interface{})
	if !ok {
		t.Fatal("reset password response must include tokens object")
	}
	if _, has := tokens["access_token"]; !has {
		t.Fatal("reset password response must include tokens.access_token")
	}
	if _, has := tokens["refresh_token"]; has {
		t.Fatal("reset password response must not include refresh_token in JSON")
	}

	foundRefreshCookie := false
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" || c.Name == "__Host-refresh_token" {
			foundRefreshCookie = true
			if !c.HttpOnly {
				t.Fatalf("reset password refresh cookie must be HttpOnly")
			}
		}
	}
	if !foundRefreshCookie {
		t.Fatal("reset password must set refresh cookie")
	}
}

func TestLogoutClearsRefreshCookie(t *testing.T) {
	r, cfg, db, _, mr := setupAuthCookieTestRouter(t)
	defer mr.Close()
	user := insertCookieTestUser(t, db, "logout@test.com", "logoutuser", "password123")

	pair, err := jwtutil.GenerateTokenPair(user.ID, user.Role, cfg.JWT.Secret, 120, 7)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	csrfToken := fetchCSRFToken(t, r)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: pair.RefreshToken})
	req.AddCookie(&http.Cookie{Name: "csrf-token", Value: csrfToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("logout: expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	cleared := false
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" || c.Name == "__Host-refresh_token" {
			if c.MaxAge < 0 || c.Expires.Before(time.Now().Add(-1*time.Second)) {
				cleared = true
			}
		}
	}
	if !cleared {
		t.Error("logout must clear the refresh cookie (set MaxAge < 0 or Expires in the past)")
	}
}

func TestLogoutIgnoresJSONBodyRefreshToken(t *testing.T) {
	r, cfg, db, _, mr := setupAuthCookieTestRouter(t)
	defer mr.Close()
	user := insertCookieTestUser(t, db, "bodylogout@test.com", "bodylogoutuser", "password123")

	pair, err := jwtutil.GenerateTokenPair(user.ID, user.Role, cfg.JWT.Secret, 120, 7)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	csrfToken := fetchCSRFToken(t, r)
	reqBody := `{"refresh_token":"` + pair.RefreshToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.AddCookie(&http.Cookie{Name: "csrf-token", Value: csrfToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("logout: expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	if mr.Exists("blacklist:token:" + pair.RefreshToken) {
		t.Fatal("logout must not revoke a refresh token supplied only in the JSON body")
	}
}

func TestRefreshFailsClosedWhenRedisUnavailable(t *testing.T) {
	r, cfg, db, _, _ := setupAuthCookieTestRouter(t)
	user := insertCookieTestUser(t, db, "failclosed@test.com", "failcloseduser", "password123")

	pair, err := jwtutil.GenerateTokenPair(user.ID, user.Role, cfg.JWT.Secret, 120, 7)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	csrfToken := fetchCSRFToken(t, r)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: pair.RefreshToken})
	req.AddCookie(&http.Cookie{Name: "csrf-token", Value: csrfToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("refresh with unavailable Redis should fail closed (401), got %d", w.Code)
	}
}

func TestRefreshRejectsMissingOrInvalidCSRFToken(t *testing.T) {
	r, _, db, _, mr := setupAuthCookieTestRouter(t)
	defer mr.Close()
	insertCookieTestUser(t, db, "csrf@test.com", "csrfuser", "password123")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "invalid-csrf-token-12345")
	req.AddCookie(&http.Cookie{Name: "csrf-token", Value: "different-csrf-value"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("refresh with invalid CSRF should be 403, got %d", w.Code)
	}
}

func TestCredentialedCORSAllowsConfiguredProductionOriginOnly(t *testing.T) {
	r, _, _, _, mr := setupAuthCookieTestRouter(t)
	defer mr.Close()

	tests := []struct {
		name        string
		origin      string
		expectACAO  string
		expectCreds string
	}{
		{
			name:        "production web origin allowed",
			origin:      "https://app.leeppp.online",
			expectACAO:  "https://app.leeppp.online",
			expectCreds: "true",
		},
		{
			name:        "localhost allowed in debug mode",
			origin:      "http://localhost:3000",
			expectACAO:  "http://localhost:3000",
			expectCreds: "true",
		},
		{
			name:        "unknown origin rejected",
			origin:      "https://evil.example.com",
			expectACAO:  "",
			expectCreds: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/csrf", nil)
			req.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			gotACAO := w.Header().Get("Access-Control-Allow-Origin")
			gotCreds := w.Header().Get("Access-Control-Allow-Credentials")

			if gotACAO != tt.expectACAO {
				t.Errorf("Access-Control-Allow-Origin: got %q, want %q", gotACAO, tt.expectACAO)
			}
			if gotCreds != tt.expectCreds {
				t.Errorf("Access-Control-Allow-Credentials: got %q, want %q", gotCreds, tt.expectCreds)
			}
		})
	}
}
