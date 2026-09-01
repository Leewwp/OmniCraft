package handler

import (
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
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// FIX-19a (T03): a registration email must never cross the API boundary for
// another user — neither via sanitizeUser projections nor via any serialized
// model.User (Author preloads on content/discussion/comment/search/series and
// the followers/following lists). Self and admin views keep the email.

func TestUserModelSerializationHidesEmail(t *testing.T) {
	user := model.User{ID: 7, Email: "secret@example.com", Username: "someone"}
	payload, err := json.Marshal(user)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "secret@example.com", "model.User JSON must not carry the email")
	require.NotContains(t, string(payload), "\"email\"", "model.User JSON must not emit an email key at all")
}

func setupUserPrivacyRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.Follow{}))

	cfg := &config.Config{
		Server:       config.ServerConfig{Mode: "debug"},
		JWT:          config.JWTConfig{Secret: "user-privacy-test-secret", AccessTokenTTL: 120, RefreshTokenTTL: 7},
		Verification: config.VerificationConfig{EmailTTLSec: 3600, ResetTTLSec: 3600, ResendCooldownSec: 60, LoginCaptchaThreshold: 3, PasswordMinLength: 8, RegisterPendingTTLSec: 86400},
		Reputation:   config.ReputationConfig{MinScoreForInteraction: 3},
		Cache:        config.CacheConfig{UserStatusTTL: 300},
	}
	userRepo := repository.NewUserRepository(db)
	authSvc := service.NewAuthService(userRepo, rdb, cfg)
	verificationSvc := service.NewVerificationService(userRepo, rdb, noopMailSender{}, cfg)
	userHandler := NewUserHandler(db, authSvc, rdb, cfg)
	authHandler := NewAuthHandler(authSvc, verificationSvc, userRepo, nil, rdb, cfg)

	r := gin.New()
	// Stub of OptionalAuth: tests drive identity with these headers.
	r.Use(func(c *gin.Context) {
		if v := c.GetHeader("X-Test-User-ID"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			require.NoError(t, err)
			c.Set(middleware.UserIDKey, id)
		} else {
			c.Set(middleware.UserIDKey, int64(0))
		}
		if role := c.GetHeader("X-Test-Role"); role != "" {
			c.Set(middleware.UserRoleKey, role)
		}
		c.Next()
	})
	r.GET("/users/:id", userHandler.GetUser)
	r.GET("/auth/me", authHandler.Me)
	return r, db
}

func createPrivacyTestUser(t *testing.T, db *gorm.DB, email, role string) *model.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), 4)
	require.NoError(t, err)
	now := time.Now()
	user := &model.User{
		Email:           strings.ToLower(email),
		Username:        strings.Split(email, "@")[0],
		PasswordHash:    string(hash),
		Role:            role,
		Reputation:      10,
		EmailVerifiedAt: &now,
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func getUserBody(t *testing.T, r *gin.Engine, targetID int64, headers map[string]string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/users/"+strconv.FormatInt(targetID, 10), nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	user, ok := body["user"].(map[string]any)
	require.True(t, ok, "response must wrap the user object")
	return user
}

func TestGetUserHidesEmailForAnonymousViewer(t *testing.T) {
	r, db := setupUserPrivacyRouter(t)
	target := createPrivacyTestUser(t, db, "target@example.com", "user")

	user := getUserBody(t, r, target.ID, nil)
	require.NotContains(t, user, "email", "anonymous GET /users/:id must not include the email key")
	require.NotContains(t, user, "preferred_locale", "preferred_locale is self-tier data")
}

func TestGetUserHidesEmailForOtherViewer(t *testing.T) {
	r, db := setupUserPrivacyRouter(t)
	target := createPrivacyTestUser(t, db, "target@example.com", "user")
	other := createPrivacyTestUser(t, db, "other@example.com", "user")

	user := getUserBody(t, r, target.ID, map[string]string{
		"X-Test-User-ID": strconv.FormatInt(other.ID, 10),
		"X-Test-Role":    "user",
	})
	require.NotContains(t, user, "email", "another user's profile must not leak the email")
}

func TestGetUserReturnsEmailForSelf(t *testing.T) {
	r, db := setupUserPrivacyRouter(t)
	target := createPrivacyTestUser(t, db, "target@example.com", "user")

	user := getUserBody(t, r, target.ID, map[string]string{
		"X-Test-User-ID": strconv.FormatInt(target.ID, 10),
		"X-Test-Role":    "user",
	})
	require.Equal(t, "target@example.com", user["email"], "self view keeps the email")
	require.Contains(t, user, "preferred_locale", "self view keeps preferred_locale")
}

func TestGetUserReturnsEmailForAdmin(t *testing.T) {
	r, db := setupUserPrivacyRouter(t)
	target := createPrivacyTestUser(t, db, "target@example.com", "user")
	admin := createPrivacyTestUser(t, db, "admin@example.com", "admin")

	user := getUserBody(t, r, target.ID, map[string]string{
		"X-Test-User-ID": strconv.FormatInt(admin.ID, 10),
		"X-Test-Role":    "admin",
	})
	require.Equal(t, "target@example.com", user["email"], "admin view keeps the email (admin semantics unchanged)")
}

func TestAuthMeKeepsEmailForSelf(t *testing.T) {
	r, db := setupUserPrivacyRouter(t)
	self := createPrivacyTestUser(t, db, "self@example.com", "user")

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("X-Test-User-ID", strconv.FormatInt(self.ID, 10))
	req.Header.Set("X-Test-Role", "user")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body struct {
		User map[string]any `json:"user"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.User)
	require.Equal(t, "self@example.com", body.User["email"], "/auth/me is the self path and must keep the email")
}

func TestLoginResponseKeepsEmailForSelf(t *testing.T) {
	verifier := &fakeCaptchaVerifier{}
	r, db, _, _, mr := setupAuthHandlerTest(t, verifier)
	defer mr.Close()
	user := createAuthHandlerTestUser(t, db, "selflogin@example.com", "password123", true)

	captchaToken := issueCaptchaTicket(t, r, "ok")
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(
		`{"email":"selflogin@example.com","password":"password123","captcha_token":"`+captchaToken+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body struct {
		User map[string]any `json:"user"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.User)
	require.Equal(t, user.Email, body.User["email"], "login is a self path and must keep the email")
}
