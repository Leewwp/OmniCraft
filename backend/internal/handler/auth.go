package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

type AuthHandler struct {
	authService         *service.AuthService
	verificationService *service.VerificationService
	userRepo            *repository.UserRepository
	rdb                 *redis.Client
	cfg                 *config.Config
}

func NewAuthHandler(authService *service.AuthService, verificationService *service.VerificationService, userRepo *repository.UserRepository, rdb *redis.Client, cfg *config.Config) *AuthHandler {
	return &AuthHandler{authService: authService, verificationService: verificationService, userRepo: userRepo, rdb: rdb, cfg: cfg}
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

func (h *AuthHandler) Register(c *gin.Context) {
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "invalid request parameters")
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

	user, err := h.authService.Register(input)
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

	_ = h.verificationService.SendVerification(c.Request.Context(), user)

	c.JSON(http.StatusAccepted, gin.H{
		"user": gin.H{
			"id":       user.ID,
			"email":    user.Email,
			"username": user.Username,
		},
		"verification_required": true,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	user, tokens, err := h.authService.Login(input)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "INVALID_CREDENTIALS", "message": "invalid email or password"})
			return
		}
		if errors.Is(err, service.ErrUserBanned) {
			c.JSON(http.StatusForbidden, gin.H{"code": "USER_BANNED", "message": "account has been banned"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to login"})
		return
	}

	setRefreshCookie(c, h.cfg, tokens.RefreshToken)

	c.JSON(http.StatusOK, gin.H{
		"user": user,
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

	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&body); err == nil && body.RefreshToken != "" {
		h.authService.Logout(body.RefreshToken)
	}

	clearRefreshCookie(c, h.cfg)
	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	refreshToken, _ := c.Cookie(refreshCookieName(h.cfg))

	if refreshToken == "" {
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.RefreshToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "INVALID_TOKEN", "message": "missing refresh token"})
			return
		}
		refreshToken = body.RefreshToken
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
	c.Header("X-CSRF-Token", csrfToken)
	c.JSON(http.StatusOK, gin.H{"user": user, "csrf_token": csrfToken})
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

	if err := h.verificationService.ResetPassword(c.Request.Context(), body.Token, body.NewPassword); err != nil {
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

	c.JSON(http.StatusOK, gin.H{"message": "password reset successfully"})
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

	normalized := strings.ToLower(strings.TrimSpace(body.Email))
	user, err := h.userRepo.FindByEmail(normalized)
	if err != nil || user == nil {
		c.JSON(http.StatusOK, gin.H{"message": "if the email exists and is unverified, a verification link has been sent"})
		return
	}

	if user.EmailVerifiedAt != nil {
		c.JSON(http.StatusOK, gin.H{"message": "if the email exists and is unverified, a verification link has been sent"})
		return
	}

	_ = h.verificationService.SendVerification(c.Request.Context(), user)

	c.JSON(http.StatusOK, gin.H{"message": "if the email exists and is unverified, a verification link has been sent"})
}
