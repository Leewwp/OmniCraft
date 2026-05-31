package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
	userRepo    *repository.UserRepository
	rdb         *redis.Client
	cfg         *config.Config
}

func NewAuthHandler(authService *service.AuthService, userRepo *repository.UserRepository, rdb *redis.Client, cfg *config.Config) *AuthHandler {
	return &AuthHandler{authService: authService, userRepo: userRepo, rdb: rdb, cfg: cfg}
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

	user, tokens, err := h.authService.Register(input)
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

	setRefreshCookie(c, h.cfg, tokens.RefreshToken)

	c.JSON(http.StatusCreated, gin.H{
		"user": user,
		"tokens": gin.H{
			"access_token": tokens.AccessToken,
		},
	})
	middleware.InvalidateUserStatusCache(h.rdb, int64(user.ID))
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
		Email string `json:"email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "email required"})
		return
	}

	user, err := h.userRepo.FindByEmail(body.Email)
	if err != nil || user == nil {
		c.JSON(http.StatusOK, gin.H{"message": "if the email exists, a reset link has been sent"})
		return
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to generate reset token"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	ctx := context.Background()
	key := "reset:token:" + token
	ttl := time.Hour
	if h.cfg != nil && h.cfg.Cache.EmailVerifyTTL > 0 {
		ttl = time.Duration(h.cfg.Cache.EmailVerifyTTL) * time.Second
	}
	if err := h.rdb.Set(ctx, key, user.ID, ttl).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to store reset token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "if the email exists, a reset link has been sent", "token": token})
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var body struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "token and new_password (min 6 chars) required"})
		return
	}

	ctx := context.Background()
	key := "reset:token:" + body.Token
	userID, err := h.rdb.Get(ctx, key).Int64()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_TOKEN", "message": "invalid or expired reset token"})
		return
	}

	if err := h.authService.ChangePassword(userID, body.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to update password"})
		return
	}

	h.rdb.Del(ctx, key)
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

	ctx := context.Background()
	key := "verify:email:" + body.Token
	userID, err := h.rdb.Get(ctx, key).Int64()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_TOKEN", "message": "invalid or expired verification token"})
		return
	}

	now := time.Now()
	if err := h.userRepo.UpdateFields(userID, map[string]interface{}{"email_verified_at": now}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to verify email"})
		return
	}

	h.rdb.Del(ctx, key)
	middleware.InvalidateUserStatusCache(h.rdb, userID)
	c.JSON(http.StatusOK, gin.H{"message": "email verified successfully"})
}

func (h *AuthHandler) SendVerificationEmail(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}

	user, err := h.userRepo.FindByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "USER_NOT_FOUND", "message": "user not found"})
		return
	}

	if user.EmailVerifiedAt != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ALREADY_VERIFIED", "message": "email already verified"})
		return
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to generate token"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	ctx := context.Background()
	key := "verify:email:" + token
	ttl := 24 * time.Hour
	if h.cfg != nil && h.cfg.Cache.PasswordResetTTL > 0 {
		ttl = time.Duration(h.cfg.Cache.PasswordResetTTL) * time.Second
	}
	if err := h.rdb.Set(ctx, key, userID, ttl).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to store token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "verification email sent", "token": token})
}
