package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"omnicraft/backend/internal/pkg/captcha"
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

type failingMailSender struct{}

func (failingMailSender) SendVerification(context.Context, string, string) error {
	return errors.New("smtp send failed")
}

func (failingMailSender) SendPasswordReset(context.Context, string, string) error {
	return errors.New("smtp send failed")
}

var _ mail.MailSender = failingMailSender{}

func setupAuthHandlerTest(t *testing.T, verifier *fakeCaptchaVerifier) (*gin.Engine, *gorm.DB, *redis.Client, *config.Config, *miniredis.Miniredis) {
	return setupAuthHandlerTestWithMail(t, verifier, noopMailSender{})
}

func setupAuthHandlerTestWithMail(t *testing.T, verifier *fakeCaptchaVerifier, mailSender mail.MailSender) (*gin.Engine, *gorm.DB, *redis.Client, *config.Config, *miniredis.Miniredis) {
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
			RegisterPendingTTLSec: 86400,
		},
		Reputation: config.ReputationConfig{MinScoreForInteraction: 3},
		Cache:      config.CacheConfig{UserStatusTTL: 300},
		Captcha:    config.CaptchaConfig{Provider: "aliyun_v2", TicketTTLSec: 120},
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	userRepo := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepo, rdb, cfg)
	verificationService := service.NewVerificationService(userRepo, rdb, mailSender, cfg)
	ticketStore := captcha.NewTicketStore(rdb, cfg.Captcha.TicketTTLSec)
	submissionVerifier := captcha.NewTicketAwareVerifier(cfg.Captcha.Provider, verifier, ticketStore)
	authHandler := NewAuthHandler(authService, verificationService, userRepo, submissionVerifier, rdb, cfg)

	r := gin.New()
	auth := r.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.POST("/forgot-password", authHandler.ForgotPassword)
	auth.POST("/resend-verification", authHandler.ResendVerification)
	captchaHandler := NewCaptchaHandler(verifier, ticketStore)
	r.POST("/captcha/verify", captchaHandler.Verify)
	return r, db, rdb, cfg, mr
}

func issueCaptchaTicket(t *testing.T, r *gin.Engine, captchaVerifyParam string) string {
	t.Helper()
	verifyReq := httptest.NewRequest(http.MethodPost, "/captcha/verify", strings.NewReader(`{"captcha_verify_param":"`+captchaVerifyParam+`"}`))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyRec := httptest.NewRecorder()
	r.ServeHTTP(verifyRec, verifyReq)
	require.Equal(t, http.StatusOK, verifyRec.Code)

	var verifyBody struct {
		CaptchaToken string `json:"captcha_token"`
	}
	require.NoError(t, json.Unmarshal(verifyRec.Body.Bytes(), &verifyBody))
	require.NotEmpty(t, verifyBody.CaptchaToken)
	return verifyBody.CaptchaToken
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

	captchaToken := issueCaptchaTicket(t, r, "aliyun-callback-param")

	okReq := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"new@test.com","username":"newuser","password":"password123","captcha_token":"`+captchaToken+`"}`))
	okReq.Header.Set("Content-Type", "application/json")
	okRec := httptest.NewRecorder()
	r.ServeHTTP(okRec, okReq)
	require.Equal(t, http.StatusAccepted, okRec.Code)
	require.Equal(t, []string{"aliyun-callback-param"}, verifier.calls)
}

func TestRegisterReturnsServiceUnavailableWhenVerificationEmailFails(t *testing.T) {
	verifier := &fakeCaptchaVerifier{}
	r, db, _, _, mr := setupAuthHandlerTestWithMail(t, verifier, failingMailSender{})
	defer mr.Close()

	captchaToken := issueCaptchaTicket(t, r, "mailfail-param")
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"mailfail@test.com","username":"mailfail","password":"password123","captcha_token":"`+captchaToken+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "EMAIL_SEND_FAILED")
	require.Equal(t, []string{"mailfail-param"}, verifier.calls)

	// Verify no user was created in DB when email send fails
	var count int64
	db.Model(&model.User{}).Where("email = ?", "mailfail@test.com").Count(&count)
	require.Equal(t, int64(0), count, "no user should exist in DB when email send fails")
}

func TestRegisterDoesNotCreateUserInDBBeforeVerification(t *testing.T) {
	verifier := &fakeCaptchaVerifier{}
	r, db, _, _, mr := setupAuthHandlerTest(t, verifier)
	defer mr.Close()

	captchaToken := issueCaptchaTicket(t, r, "pending-param")
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"pending@test.com","username":"pendinguser","password":"password123","captcha_token":"`+captchaToken+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Contains(t, rec.Body.String(), "verification_required")

	// Verify no user was created in DB yet
	var count int64
	db.Model(&model.User{}).Where("email = ?", "pending@test.com").Count(&count)
	require.Equal(t, int64(0), count, "user should NOT exist in DB before email verification")
}

func TestRegisterDuplicateEmailAfterFailedSend(t *testing.T) {
	verifier := &fakeCaptchaVerifier{}
	r, _, _, _, mr := setupAuthHandlerTestWithMail(t, verifier, failingMailSender{})
	defer mr.Close()

	// First attempt: email send fails, pending data is cleaned up
	captchaToken1 := issueCaptchaTicket(t, r, "retry-param-1")
	req1 := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"retry@test.com","username":"retryuser","password":"password123","captcha_token":"`+captchaToken1+`"}`))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusServiceUnavailable, rec1.Code)

	// Second attempt: should succeed (pending data was cleaned up)
	// But email still fails, so we get 503 again - but NOT 409 USER_EXISTS
	captchaToken2 := issueCaptchaTicket(t, r, "retry-param-2")
	req2 := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"retry@test.com","username":"retryuser","password":"password123","captcha_token":"`+captchaToken2+`"}`))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusServiceUnavailable, rec2.Code)
	require.Contains(t, rec2.Body.String(), "EMAIL_SEND_FAILED")
	require.NotContains(t, rec2.Body.String(), "USER_EXISTS")
}

func TestLoginBlocksUnverifiedUser(t *testing.T) {
	verifier := &fakeCaptchaVerifier{}
	r, db, _, _, mr := setupAuthHandlerTest(t, verifier)
	defer mr.Close()

	// Create an unverified user directly in DB (legacy scenario)
	createAuthHandlerTestUser(t, db, "unverified@test.com", "password123", false)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"unverified@test.com","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "EMAIL_NOT_VERIFIED")
}

func TestLoginSucceedsForVerifiedUser(t *testing.T) {
	verifier := &fakeCaptchaVerifier{}
	r, db, _, _, mr := setupAuthHandlerTest(t, verifier)
	defer mr.Close()

	createAuthHandlerTestUser(t, db, "verified@test.com", "password123", true)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"verified@test.com","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
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
	require.Empty(t, verifier.calls)
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

	captchaToken := issueCaptchaTicket(t, r, "resend-param")
	okReq := httptest.NewRequest(http.MethodPost, "/auth/resend-verification", strings.NewReader(`{"email":"missing@test.com","captcha_token":"`+captchaToken+`"}`))
	okReq.Header.Set("Content-Type", "application/json")
	okRec := httptest.NewRecorder()
	r.ServeHTTP(okRec, okReq)
	require.Equal(t, http.StatusOK, okRec.Code)
	require.Equal(t, []string{"resend-param"}, verifier.calls)
}

func TestResendVerificationReturns429DuringCooldown(t *testing.T) {
	verifier := &fakeCaptchaVerifier{}
	r, db, _, _, mr := setupAuthHandlerTest(t, verifier)
	defer mr.Close()
	createAuthHandlerTestUser(t, db, "resend@test.com", "password123", false)

	captchaToken1 := issueCaptchaTicket(t, r, "resend-cooldown-param-1")
	body1 := `{"email":"resend@test.com","captcha_token":"` + captchaToken1 + `"}`
	firstReq := httptest.NewRequest(http.MethodPost, "/auth/resend-verification", strings.NewReader(body1))
	firstReq.Header.Set("Content-Type", "application/json")
	firstRec := httptest.NewRecorder()
	r.ServeHTTP(firstRec, firstReq)
	require.Equal(t, http.StatusOK, firstRec.Code)

	captchaToken2 := issueCaptchaTicket(t, r, "resend-cooldown-param-2")
	body2 := `{"email":"resend@test.com","captcha_token":"` + captchaToken2 + `"}`
	secondReq := httptest.NewRequest(http.MethodPost, "/auth/resend-verification", strings.NewReader(body2))
	secondReq.Header.Set("Content-Type", "application/json")
	secondRec := httptest.NewRecorder()
	r.ServeHTTP(secondRec, secondReq)

	require.Equal(t, http.StatusTooManyRequests, secondRec.Code)
	require.Contains(t, secondRec.Body.String(), "RESEND_COOLDOWN")
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
