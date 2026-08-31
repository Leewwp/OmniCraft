package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/captcha"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

type AuthHandler struct {
	authService         *service.AuthService
	verificationService *service.VerificationService
	userRepo            *repository.UserRepository
	captchaVerifier     captcha.CaptchaVerifier
	rdb                 *redis.Client
	cfg                 *config.Config
	displaySigner       *service.DisplayURLSigner
}

type AuthCapabilities struct {
	CanInteract             bool   `json:"can_interact"`
	InteractionDenialReason string `json:"interaction_denial_reason,omitempty"`
}

func buildAuthCapabilities(user *model.User, cfg *config.Config) (AuthCapabilities, string) {
	status := &service.RuntimeUserStatus{
		ID:              int64(user.ID),
		Role:            user.Role,
		IsBanned:        user.IsBanned,
		EmailVerifiedAt: user.EmailVerifiedAt,
		Reputation:      user.Reputation,
	}
	decision := service.EvaluateInteractionAccess(status, cfg, true, true)
	if !decision.Allowed && !service.IsSoftDenialReason(decision.DenialReason) {
		return AuthCapabilities{}, decision.DenialReason
	}
	return AuthCapabilities{
		CanInteract:             decision.Allowed,
		InteractionDenialReason: decision.DenialReason,
	}, ""
}

func NewAuthHandler(authService *service.AuthService, verificationService *service.VerificationService, userRepo *repository.UserRepository, captchaVerifier captcha.CaptchaVerifier, rdb *redis.Client, cfg *config.Config) *AuthHandler {
	// Wire up the circular dependency: VerificationService needs AuthService for pending registration flow
	verificationService.SetAuthService(authService)
	return &AuthHandler{authService: authService, verificationService: verificationService, userRepo: userRepo, captchaVerifier: captchaVerifier, rdb: rdb, cfg: cfg, displaySigner: service.NewDisplayURLSigner(cfg)}
}

func refreshCookieName(cfg *config.Config) string {
	if cfg.Server.Mode == "release" {
		return "__Host-refresh_token"
	}
	return "refresh_token"
}

func setRefreshCookie(c *gin.Context, cfg *config.Config, token string) {
	name := refreshCookieName(cfg)
	isSecure := cfg.Server.Mode == "release"
	ttl := cfg.JWT.RefreshTokenTTL
	if ttl <= 0 {
		ttl = 7
	}
	maxAge := ttl * 24 * 60 * 60

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, token, maxAge, "/", "", isSecure, true)
}

func clearRefreshCookie(c *gin.Context, cfg *config.Config) {
	name := refreshCookieName(cfg)
	isSecure := cfg.Server.Mode == "release"

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, "", -1, "/", "", isSecure, true)
}

func (h *AuthHandler) verifyCaptcha(c *gin.Context, token string) bool {
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "CAPTCHA_REQUIRED", "message": "captcha verification required"})
		return false
	}
	if h.captchaVerifier != nil {
		if err := h.captchaVerifier.Verify(c.Request.Context(), token, c.ClientIP()); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "CAPTCHA_FAILED", "message": "captcha verification failed"})
			return false
		}
	}
	return true
}

func (h *AuthHandler) loginCaptchaKey(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	sum := sha256.Sum256([]byte(normalized))
	return "captcha:login-failures:" + hex.EncodeToString(sum[:])
}

func (h *AuthHandler) loginCaptchaTTL() time.Duration {
	ttl := time.Duration(h.cfg.Verification.ResendCooldownSec) * time.Second
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return ttl
}

func (h *AuthHandler) captchaRequiredForLogin(c *gin.Context, email string) (bool, bool) {
	threshold := h.cfg.Verification.LoginCaptchaThreshold
	if threshold <= 0 || h.rdb == nil {
		return false, true
	}
	raw, err := h.rdb.Get(c.Request.Context(), h.loginCaptchaKey(email)).Result()
	if err == redis.Nil {
		return false, true
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": service.DenialReasonAuthStatusUnavailable, "message": "account status is temporarily unavailable"})
		return false, false
	}
	failures, err := strconv.Atoi(raw)
	if err != nil {
		failures = 0
	}
	return failures >= threshold, true
}

func (h *AuthHandler) recordLoginFailure(c *gin.Context, email string) {
	if h.rdb == nil {
		return
	}
	key := h.loginCaptchaKey(email)
	count := h.rdb.Incr(c.Request.Context(), key)
	if count.Err() == nil {
		h.rdb.Expire(c.Request.Context(), key, h.loginCaptchaTTL())
	}
}

func (h *AuthHandler) clearLoginFailures(c *gin.Context, email string) {
	if h.rdb == nil {
		return
	}
	h.rdb.Del(c.Request.Context(), h.loginCaptchaKey(email))
}

func (h *AuthHandler) Register(c *gin.Context) {
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	if !h.verifyCaptcha(c, input.CaptchaToken) {
		return
	}

	if h.cfg.Legal.CurrentTermsVersion != "" && input.AcceptedTermsVersion != h.cfg.Legal.CurrentTermsVersion {
		c.JSON(http.StatusBadRequest, gin.H{"code": "TERMS_VERSION_MISMATCH", "message": "accepted terms version does not match current version"})
		return
	}
	if h.cfg.Legal.CurrentPrivacyVersion != "" && input.AcceptedPrivacyVersion != h.cfg.Legal.CurrentPrivacyVersion {
		c.JSON(http.StatusBadRequest, gin.H{"code": "PRIVACY_VERSION_MISMATCH", "message": "accepted privacy version does not match current version"})
		return
	}

	// Store registration data in Redis (NOT in DB yet)
	regID, err := h.authService.RegisterPending(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, service.ErrUserAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"code": "USER_EXISTS", "message": "email already registered"})
			return
		}
		if errors.Is(err, service.ErrUsernameTaken) {
			c.JSON(http.StatusConflict, gin.H{"code": "USERNAME_TAKEN", "message": "username already taken"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to register user"})
		return
	}

	// Send verification email for the pending registration
	normalizedEmail := strings.ToLower(strings.TrimSpace(input.Email))
	if err := h.verificationService.SendVerificationForPending(c.Request.Context(), regID, normalizedEmail); err != nil {
		slog.Error("failed to send verification email", "reg_id", regID, "email", normalizedEmail, "error", err)
		// Clean up pending registration since email failed
		pending, _ := h.authService.GetPendingRegistration(c.Request.Context(), regID)
		if pending != nil {
			_ = h.authService.DeletePendingRegistration(c.Request.Context(), regID, pending)
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "EMAIL_SEND_FAILED", "message": "verification email could not be sent"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"email":                 normalizedEmail,
		"username":              input.Username,
		"verification_required": true,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	captchaRequired, ok := h.captchaRequiredForLogin(c, input.Email)
	if !ok {
		return
	}
	if captchaRequired && !h.verifyCaptcha(c, input.CaptchaToken) {
		return
	}

	user, tokens, err := h.authService.Login(input)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			h.recordLoginFailure(c, input.Email)
			c.JSON(http.StatusUnauthorized, gin.H{"code": "INVALID_CREDENTIALS", "message": "invalid email or password"})
			return
		}
		if errors.Is(err, service.ErrUserBanned) {
			c.JSON(http.StatusForbidden, gin.H{"code": service.DenialReasonUserBanned, "message": "account has been banned"})
			return
		}
		if errors.Is(err, service.ErrEmailNotVerified) {
			c.JSON(http.StatusForbidden, gin.H{"code": service.DenialReasonEmailNotVerified, "message": "email verification required before login"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to login"})
		return
	}

	capabilities, capabilityErrCode := buildAuthCapabilities(user, h.cfg)
	if capabilityErrCode != "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": capabilityErrCode, "message": "interaction status is temporarily unavailable"})
		return
	}

	h.clearLoginFailures(c, input.Email)
	setRefreshCookie(c, h.cfg, tokens.RefreshToken)

	h.displaySigner.DecorateUser(user)
	c.JSON(http.StatusOK, gin.H{
		"user":         user,
		"capabilities": capabilities,
		"tokens": gin.H{
			"access_token": tokens.AccessToken,
		},
	})
	middleware.InvalidateUserStatusCache(h.rdb, int64(user.ID))
}

func (h *AuthHandler) Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 {
		if err := h.authService.Logout(parts[1]); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to logout"})
			return
		}
	}

	cookieToken, err := c.Cookie(refreshCookieName(h.cfg))
	if err == nil && cookieToken != "" {
		h.authService.Logout(cookieToken)
	}

	clearRefreshCookie(c, h.cfg)
	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	refreshToken, _ := c.Cookie(refreshCookieName(h.cfg))

	if refreshToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "INVALID_TOKEN", "message": "missing refresh token"})
		return
	}

	tokens, err := h.authService.RefreshToken(refreshToken)
	if err != nil {
		clearRefreshCookie(c, h.cfg)
		c.JSON(http.StatusUnauthorized, gin.H{"code": "INVALID_TOKEN", "message": "invalid or expired refresh token"})
		return
	}

	setRefreshCookie(c, h.cfg, tokens.RefreshToken)

	c.JSON(http.StatusOK, gin.H{
		"tokens": gin.H{
			"access_token": tokens.AccessToken,
		},
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "not authenticated"})
		return
	}

	user, err := h.userRepo.FindByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "USER_NOT_FOUND", "message": "user not found"})
		return
	}

	csrfToken := middleware.GetCSRFToken(c)
	capabilities, capabilityErrCode := buildAuthCapabilities(user, h.cfg)
	if capabilityErrCode != "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": capabilityErrCode, "message": "interaction status is temporarily unavailable"})
		return
	}
	c.Header("X-CSRF-Token", csrfToken)
	h.displaySigner.DecorateUser(user)
	c.JSON(http.StatusOK, gin.H{
		"user":         user,
		"csrf_token":   csrfToken,
		"capabilities": capabilities,
	})
}

func (h *AuthHandler) CSRFToken(c *gin.Context) {
	csrfToken := middleware.GetCSRFToken(c)
	c.JSON(http.StatusOK, gin.H{"csrf_token": csrfToken})
}

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var body struct {
		Email        string `json:"email" binding:"required,email"`
		CaptchaToken string `json:"captcha_token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "email required"})
		return
	}
	if !h.verifyCaptcha(c, body.CaptchaToken) {
		return
	}

	_ = h.verificationService.SendPasswordReset(c.Request.Context(), body.Email)

	c.JSON(http.StatusOK, gin.H{"message": "if the email exists, a reset link has been sent"})
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var body struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "token and new_password required"})
		return
	}

	userID, err := h.verificationService.ResetPassword(c.Request.Context(), body.Token, body.NewPassword)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_TOKEN", "message": "invalid or expired reset token"})
			return
		}
		if errors.Is(err, service.ErrPasswordTooShort) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "PASSWORD_TOO_SHORT", "message": "password does not meet minimum length requirement"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to reset password"})
		return
	}

	user, err := h.userRepo.FindByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to establish session"})
		return
	}
	tokens, err := h.authService.IssueTokenPairForUser(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to establish session"})
		return
	}

	setRefreshCookie(c, h.cfg, tokens.RefreshToken)
	middleware.InvalidateUserStatusCache(h.rdb, int64(user.ID))
	c.JSON(http.StatusOK, gin.H{
		"message": "password reset successfully",
		"tokens": gin.H{
			"access_token": tokens.AccessToken,
		},
	})
}

func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	var body struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "token required"})
		return
	}

	if err := h.verificationService.VerifyEmail(c.Request.Context(), body.Token); err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_TOKEN", "message": "invalid or expired verification token"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to verify email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "email verified successfully"})
}

func (h *AuthHandler) ResendVerification(c *gin.Context) {
	var body struct {
		Email        string `json:"email" binding:"required,email"`
		CaptchaToken string `json:"captcha_token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "email required"})
		return
	}
	if !h.verifyCaptcha(c, body.CaptchaToken) {
		return
	}

	normalized := strings.ToLower(strings.TrimSpace(body.Email))

	// First check Redis for pending registration
	regID, pending, err := h.authService.FindPendingByEmail(c.Request.Context(), normalized)
	if err == nil && pending != nil {
		if err := h.verificationService.SendVerificationForPending(c.Request.Context(), regID, pending.Email); err != nil {
			if errors.Is(err, service.ErrResendCooldown) {
				c.JSON(http.StatusTooManyRequests, gin.H{"code": "RESEND_COOLDOWN", "message": "please wait before requesting another verification email"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to send verification email"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "if the email exists and is unverified, a verification link has been sent"})
		return
	}

	// Fall back to DB for legacy unverified users
	user, err := h.userRepo.FindByEmail(normalized)
	if err != nil || user == nil {
		c.JSON(http.StatusOK, gin.H{"message": "if the email exists and is unverified, a verification link has been sent"})
		return
	}

	if user.EmailVerifiedAt != nil {
		c.JSON(http.StatusOK, gin.H{"message": "if the email exists and is unverified, a verification link has been sent"})
		return
	}

	if err := h.verificationService.SendVerification(c.Request.Context(), user); err != nil {
		if errors.Is(err, service.ErrResendCooldown) {
			c.JSON(http.StatusTooManyRequests, gin.H{"code": "RESEND_COOLDOWN", "message": "please wait before requesting another verification email"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to send verification email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "if the email exists and is unverified, a verification link has been sent"})
}
