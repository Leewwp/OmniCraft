package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
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
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/mail"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

type fakeCaptchaVerifier struct {
	calls []string
	err   error
}

func (f *fakeCaptchaVerifier) Verify(_ context.Context, token, _ string) error {
	f.calls = append(f.calls, token)
	return f.err
}

type noopMailSender struct{}

func (noopMailSender) SendVerification(context.Context, string, string) error {
	return nil
}

func (noopMailSender) SendPasswordReset(context.Context, string, string) error {
	return nil
}

var _ mail.MailSender = noopMailSender{}

func setupAuthHandlerTest(t *testing.T, verifier *fakeCaptchaVerifier) (*gin.Engine, *gorm.DB, *redis.Client, *config.Config, *miniredis.Miniredis) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr, err := miniredis.Run()
	require.NoError(t, err)

	cfg := &config.Config{
		Server: config.ServerConfig{Mode: "debug"},
		JWT: config.JWTConfig{
			Secret:          "auth-handler-test-secret",
			AccessTokenTTL:  120,
			RefreshTokenTTL: 7,
		},
		Verification: config.VerificationConfig{
			EmailTTLSec:           3600,
			ResetTTLSec:           3600,
			ResendCooldownSec:     60,
			LoginCaptchaThreshold: 3,
			PasswordMinLength:     8,
		},
		Reputation: config.ReputationConfig{MinScoreForInteraction: 3},
		Cache:      config.CacheConfig{UserStatusTTL: 300},
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	userRepo := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepo, rdb, cfg)
	verificationService := service.NewVerificationService(userRepo, rdb, noopMailSender{}, cfg)
	authHandler := NewAuthHandler(authService, verificationService, userRepo, verifier, rdb, cfg)

	r := gin.New()
	auth := r.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.POST("/forgot-password", authHandler.ForgotPassword)
	auth.POST("/resend-verification", authHandler.ResendVerification)
	return r, db, rdb, cfg, mr
}

func createAuthHandlerTestUser(t *testing.T, db *gorm.DB, email, password string, verified bool) *model.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 4)
	require.NoError(t, err)
	user := &model.User{
		Email:        strings.ToLower(strings.TrimSpace(email)),
		Username:     strings.Split(email, "@")[0],
		PasswordHash: string(hash),
		Reputation:   10,
		Role:         "user",
	}
	if verified {
		now := time.Now()
		user.EmailVerifiedAt = &now
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func TestRegisterRequiresAndVerifiesCaptcha(t *testing.T) {
	verifier := &fakeCaptchaVerifier{}
	r, _, _, _, mr := setupAuthHandlerTest(t, verifier)
	defer mr.Close()

	missingReq := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"new@test.com","username":"newuser","password":"password123"}`))
	missingReq.Header.Set("Content-Type", "application/json")
	missingRec := httptest.NewRecorder()
	r.ServeHTTP(missingRec, missingReq)
	require.Equal(t, http.StatusBadRequest, missingRec.Code)
	require.Contains(t, missingRec.Body.String(), "CAPTCHA_REQUIRED")

	okReq := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"new@test.com","username":"newuser","password":"password123","captcha_token":"captcha-ok"}`))
	okReq.Header.Set("Content-Type", "application/json")
	okRec := httptest.NewRecorder()
	r.ServeHTTP(okRec, okReq)
	require.Equal(t, http.StatusAccepted, okRec.Code)
	require.Equal(t, []string{"captcha-ok"}, verifier.calls)
}

func TestForgotPasswordRequiresAndVerifiesCaptcha(t *testing.T) {
	verifier := &fakeCaptchaVerifier{err: errors.New("bad captcha")}
	r, _, _, _, mr := setupAuthHandlerTest(t, verifier)
	defer mr.Close()

	missingReq := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", strings.NewReader(`{"email":"user@test.com"}`))
	missingReq.Header.Set("Content-Type", "application/json")
	missingRec := httptest.NewRecorder()
	r.ServeHTTP(missingRec, missingReq)
	require.Equal(t, http.StatusBadRequest, missingRec.Code)
	require.Contains(t, missingRec.Body.String(), "CAPTCHA_REQUIRED")

	badReq := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", strings.NewReader(`{"email":"user@test.com","captcha_token":"captcha-bad"}`))
	badReq.Header.Set("Content-Type", "application/json")
	badRec := httptest.NewRecorder()
	r.ServeHTTP(badRec, badReq)
	require.Equal(t, http.StatusBadRequest, badRec.Code)
	require.Contains(t, badRec.Body.String(), "CAPTCHA_FAILED")
	require.Equal(t, []string{"captcha-bad"}, verifier.calls)
}

func TestResendVerificationRequiresAndVerifiesCaptcha(t *testing.T) {
	verifier := &fakeCaptchaVerifier{}
	r, _, _, _, mr := setupAuthHandlerTest(t, verifier)
	defer mr.Close()

	missingReq := httptest.NewRequest(http.MethodPost, "/auth/resend-verification", strings.NewReader(`{"email":"user@test.com"}`))
	missingReq.Header.Set("Content-Type", "application/json")
	missingRec := httptest.NewRecorder()
	r.ServeHTTP(missingRec, missingReq)
	require.Equal(t, http.StatusBadRequest, missingRec.Code)
	require.Contains(t, missingRec.Body.String(), "CAPTCHA_REQUIRED")

	okReq := httptest.NewRequest(http.MethodPost, "/auth/resend-verification", strings.NewReader(`{"email":"missing@test.com","captcha_token":"captcha-ok"}`))
	okReq.Header.Set("Content-Type", "application/json")
	okRec := httptest.NewRecorder()
	r.ServeHTTP(okRec, okReq)
	require.Equal(t, http.StatusOK, okRec.Code)
	require.Equal(t, []string{"captcha-ok"}, verifier.calls)
}

func TestLoginRequiresCaptchaAfterFailureThreshold(t *testing.T) {
	verifier := &fakeCaptchaVerifier{}
	r, db, rdb, _, mr := setupAuthHandlerTest(t, verifier)
	defer mr.Close()
	createAuthHandlerTestUser(t, db, "login@test.com", "password123", true)

	sum := sha256.Sum256([]byte("login@test.com"))
	key := "captcha:login-failures:" + hex.EncodeToString(sum[:])
	require.NoError(t, rdb.Set(context.Background(), key, "3", 0).Err())

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"login@test.com","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "CAPTCHA_REQUIRED")
	require.Empty(t, verifier.calls)
}
