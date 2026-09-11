package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

const UserIDKey = "userID"
const UserRoleKey = "userRole"

// Auth channels: "" (anonymous / legacy), "jwt" (web session access token)
// and "pat" (external-agent machine identity, SP-16 #450). PAT and JWT share
// the resolved user identity but never the session: the PAT branch issues no
// cookies and bypasses the refresh-token machinery entirely.
const AuthChannelKey = "authChannel"
const patScopesKey = "patScopes"
const PATTokenIDKey = "patTokenID"

const (
	AuthChannelJWT = "jwt"
	AuthChannelPAT = "pat"
)

// bannedWhitelistAllowed reports whether the request may proceed for a banned
// user (FIX-15 / T29): the self-service appeal path plus /auth/me (so the
// frontend ban screen can render capabilities) are the only sanctioned escape
// hatches. The whitelist is exact per method+path to avoid loosening any
// other authReq route; handlers still scope access by callerID, so a banned
// user can only read/write their own appeals.
func bannedWhitelistAllowed(method, path string) bool {
	switch {
	case method == http.MethodGet && path == "/api/v1/auth/me":
		return true
	case method == http.MethodGet && path == "/api/v1/appeals/me":
		return true
	case method == http.MethodPost && path == "/api/v1/appeals":
		return true
	default:
		return false
	}
}

func AuthRequired(cfg *config.Config, rdb *redis.Client, db ...*gorm.DB) gin.HandlerFunc {
	var dbInstance *gorm.DB
	if len(db) > 0 {
		dbInstance = db[0]
	}
	var patSvc *service.AgentAccessTokenService
	if dbInstance != nil {
		patSvc = service.NewAgentAccessTokenService(
			repository.NewAgentAccessTokenRepository(dbInstance), cfg)
	}

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"code": "UNAUTHORIZED", "message": "authorization header required"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(401, gin.H{"code": "UNAUTHORIZED", "message": "invalid authorization header format"})
			c.Abort()
			return
		}

		tokenStr := parts[1]

		// Machine channel: PAT bearer tokens resolve to the same user
		// identity as JWT but never touch the session machinery. Every
		// guard downstream (ban resolution, interaction policies, upload
		// quotas) operates on the context user id and thus applies to PAT
		// unchanged.
		if strings.HasPrefix(tokenStr, service.AgentTokenPrefix) {
			identity, status, denial := resolvePATIdentity(c.Request.Context(), cfg, rdb, dbInstance, patSvc, tokenStr)
			switch denial {
			case "":
				if !enforcePATRateLimit(c, rdb, cfg, identity.TokenID) {
					c.JSON(429, gin.H{"code": "RATE_LIMIT_EXCEEDED", "message": "too many requests, please try again later"})
					c.Abort()
					return
				}
				c.Set(UserIDKey, status.ID)
				c.Set(UserRoleKey, status.Role)
				c.Set(AuthChannelKey, AuthChannelPAT)
				c.Set(patScopesKey, identity.Scopes)
				c.Set(PATTokenIDKey, identity.TokenID)
				c.Next()
				return
			case service.DenialReasonAuthStatusUnavailable:
				c.JSON(503, gin.H{"code": denial, "message": "auth service is temporarily unavailable, please try again later"})
			case service.DenialReasonUserBanned:
				c.JSON(401, gin.H{"code": denial, "message": "account has been banned"})
			default:
				c.JSON(401, gin.H{"code": "UNAUTHORIZED", "message": "invalid or revoked token"})
			}
			c.Abort()
			return
		}

		claims, err := jwtutil.ParseToken(tokenStr, cfg.JWT.Secret)
		if err != nil {
			c.JSON(401, gin.H{"code": "UNAUTHORIZED", "message": "invalid or expired token"})
			c.Abort()
			return
		}

		if claims.Subject != "access" {
			c.JSON(401, gin.H{"code": "UNAUTHORIZED", "message": "invalid token type"})
			c.Abort()
			return
		}

		redisAvailable := rdb != nil
		dbAvailable := dbInstance != nil

		if !redisAvailable && !dbAvailable {
			c.JSON(503, gin.H{
				"code":    service.DenialReasonAuthStatusUnavailable,
				"message": "auth service is temporarily unavailable, please try again later",
			})
			c.Abort()
			return
		}

		if rdb != nil {
			blacklistKey := fmt.Sprintf("blacklist:token:%s", tokenStr)
			val, redisErr := rdb.Get(c.Request.Context(), blacklistKey).Result()
			if redisErr == nil && val == "1" {
				c.JSON(401, gin.H{"code": "UNAUTHORIZED", "message": "token has been revoked"})
				c.Abort()
				return
			}
			if redisErr != nil && redisErr != redis.Nil {
				c.JSON(503, gin.H{"code": service.DenialReasonAuthStatusUnavailable, "message": "account status is temporarily unavailable"})
				c.Abort()
				return
			}
		}

		cache := service.NewRuntimeStatusCache(rdb, cfg)
		status, resolveErr := service.ResolveRuntimeUserStatus(c.Request.Context(), dbInstance, cache, claims.UserID)
		if resolveErr != nil {
			if errors.Is(resolveErr, service.ErrUserStatusNotFound) || errors.Is(resolveErr, service.ErrUserStatusDeleted) {
				c.JSON(401, gin.H{"code": "UNAUTHORIZED", "message": "user not found or deleted"})
				c.Abort()
				return
			}
			if status != nil && status.IsBanned {
				c.JSON(401, gin.H{"code": service.DenialReasonUserBanned, "message": "account has been banned"})
				c.Abort()
				return
			}
			c.JSON(503, gin.H{"code": service.DenialReasonAuthStatusUnavailable, "message": "account status is temporarily unavailable"})
			c.Abort()
			return
		}

		if status.IsBanned && !bannedWhitelistAllowed(c.Request.Method, c.Request.URL.Path) {
			c.JSON(401, gin.H{"code": service.DenialReasonUserBanned, "message": "account has been banned"})
			c.Abort()
			return
		}

		c.Set(UserIDKey, status.ID)
		c.Set(UserRoleKey, status.Role)
		c.Set(AuthChannelKey, AuthChannelJWT)
		c.Next()
	}
}

// resolvePATIdentity authenticates a bearer PAT and resolves the owning
// user's runtime status. It returns ("", identity, status) on success or a
// denial code with nil results. Banned users get no whitelist on the PAT
// channel: the appeals escape hatch is a web self-service concept.
func resolvePATIdentity(ctx context.Context, cfg *config.Config, rdb *redis.Client, db *gorm.DB, patSvc *service.AgentAccessTokenService, tokenStr string) (*service.AgentAccessTokenIdentity, *service.RuntimeUserStatus, string) {
	if patSvc == nil {
		return nil, nil, service.DenialReasonAuthStatusUnavailable
	}
	identity, err := patSvc.Authenticate(ctx, tokenStr)
	if err != nil {
		return nil, nil, "UNAUTHORIZED"
	}

	cache := service.NewRuntimeStatusCache(rdb, cfg)
	status, resolveErr := service.ResolveRuntimeUserStatus(ctx, db, cache, identity.UserID)
	if resolveErr != nil {
		if errors.Is(resolveErr, service.ErrUserStatusNotFound) || errors.Is(resolveErr, service.ErrUserStatusDeleted) {
			return nil, nil, "UNAUTHORIZED"
		}
		return nil, nil, service.DenialReasonAuthStatusUnavailable
	}
	if status.IsBanned {
		return nil, nil, service.DenialReasonUserBanned
	}
	return identity, status, ""
}

// enforcePATRateLimit applies the per-token fixed window on top of the
// anonymous IP window; Redis failures stay fail-open exactly like RateLimit.
func enforcePATRateLimit(c *gin.Context, rdb *redis.Client, cfg *config.Config, tokenID int64) bool {
	if rdb == nil || cfg == nil || !cfg.RateLimit.Enabled {
		return true
	}
	limit := cfg.RateLimit.PATPerMinute
	if limit <= 0 {
		limit = 60
	}
	windowSec := cfg.RateLimit.PATWindowSec
	if windowSec <= 0 {
		windowSec = 60
	}
	window := time.Now().Unix() / int64(windowSec)
	key := fmt.Sprintf("ratelimit:pat:%d:%d", tokenID, window)

	count, err := rdb.Incr(context.Background(), key).Result()
	if err != nil {
		return true
	}
	rdb.Expire(context.Background(), key, 2*time.Duration(windowSec)*time.Second)
	return int(count) <= limit
}

func OptionalAuth(cfg *config.Config, rdb *redis.Client, db ...*gorm.DB) gin.HandlerFunc {
	var dbInstance *gorm.DB
	if len(db) > 0 {
		dbInstance = db[0]
	}
	var patSvc *service.AgentAccessTokenService
	if dbInstance != nil {
		patSvc = service.NewAgentAccessTokenService(
			repository.NewAgentAccessTokenRepository(dbInstance), cfg)
	}

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Set(UserIDKey, int64(0))
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.Set(UserIDKey, int64(0))
			c.Next()
			return
		}

		tokenStr := parts[1]

		// Machine channel: a valid PAT personalizes the anonymous view
		// (and still consumes the per-token window); any bad token
		// degrades to anonymous exactly like a bad JWT.
		if strings.HasPrefix(tokenStr, service.AgentTokenPrefix) {
			identity, status, denial := resolvePATIdentity(c.Request.Context(), cfg, rdb, dbInstance, patSvc, tokenStr)
			if denial == "" && enforcePATRateLimit(c, rdb, cfg, identity.TokenID) {
				c.Set(UserIDKey, status.ID)
				c.Set(UserRoleKey, status.Role)
				c.Set(AuthChannelKey, AuthChannelPAT)
				c.Set(patScopesKey, identity.Scopes)
				c.Set(PATTokenIDKey, identity.TokenID)
			} else {
				c.Set(UserIDKey, int64(0))
			}
			c.Next()
			return
		}

		claims, err := jwtutil.ParseToken(tokenStr, cfg.JWT.Secret)
		if err != nil || claims.Subject != "access" {
			c.Set(UserIDKey, int64(0))
			c.Next()
			return
		}

		if rdb != nil {
			blacklistKey := fmt.Sprintf("blacklist:token:%s", tokenStr)
			val, redisErr := rdb.Get(c.Request.Context(), blacklistKey).Result()
			if redisErr == nil && val == "1" {
				c.Set(UserIDKey, int64(0))
				c.Next()
				return
			}
		}

		cache := service.NewRuntimeStatusCache(rdb, cfg)
		status, resolveErr := service.ResolveRuntimeUserStatus(c.Request.Context(), dbInstance, cache, claims.UserID)
		if resolveErr != nil {
			c.Set(UserIDKey, int64(0))
			c.Next()
			return
		}

		if status.IsBanned {
			c.Set(UserIDKey, int64(0))
			c.Next()
			return
		}

		c.Set(UserIDKey, status.ID)
		c.Set(UserRoleKey, status.Role)
		c.Set(AuthChannelKey, AuthChannelJWT)
		c.Next()
	}
}

// GetAuthChannel reports how the request authenticated: "" (anonymous or
// pre-SP-16 callers), "jwt" (web session) or "pat" (machine identity).
func GetAuthChannel(c *gin.Context) string {
	return c.GetString(AuthChannelKey)
}

// GetPATScopes returns the scopes carried by a PAT-authenticated request.
func GetPATScopes(c *gin.Context) []string {
	v, exists := c.Get(patScopesKey)
	if !exists {
		return nil
	}
	scopes, _ := v.([]string)
	return scopes
}

// HasPATScope reports whether the PAT channel grants the given scope.
func HasPATScope(c *gin.Context, scope string) bool {
	for _, s := range GetPATScopes(c) {
		if s == scope {
			return true
		}
	}
	return false
}

// RequireScopeForPAT gates a route behind a PAT scope while leaving web
// JWT sessions untouched: scopes are the machine channel's capability
// vocabulary, not a new restriction on browser users.
func RequireScopeForPAT(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if GetAuthChannel(c) != AuthChannelPAT {
			c.Next()
			return
		}
		if !HasPATScope(c, scope) {
			c.JSON(403, gin.H{"code": "PAT_SCOPE_REQUIRED", "message": "token lacks required scope: " + scope})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireJWTChannel rejects PAT-authenticated requests. Token management is
// JWT-only by design: a leaked PAT must never be able to mint more tokens.
func RequireJWTChannel() gin.HandlerFunc {
	return func(c *gin.Context) {
		if GetAuthChannel(c) == AuthChannelPAT {
			c.JSON(403, gin.H{"code": "JWT_CHANNEL_REQUIRED", "message": "this endpoint requires the web session channel"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsAdmin(c) {
			c.JSON(403, gin.H{"code": "FORBIDDEN", "message": "admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// IsAdmin reports whether the current context carries the admin role. Works
// after both AuthRequired and OptionalAuth resolved the identity.
func IsAdmin(c *gin.Context) bool {
	role, exists := c.Get(UserRoleKey)
	return exists && role == "admin"
}

func GetUserID(c *gin.Context) int64 {
	v, exists := c.Get(UserIDKey)
	if !exists {
		return 0
	}
	switch id := v.(type) {
	case int64:
		return id
	case uint:
		return int64(id)
	case uint64:
		return int64(id)
	}
	return 0
}

func SetUserStatusCache(rdb *redis.Client, userID int64, isBanned bool, role string) {
	if rdb == nil {
		return
	}
	cache := service.NewRuntimeStatusCache(rdb, nil)
	cache.Invalidate(userID)
}

func InvalidateUserStatusCache(rdb *redis.Client, userID int64) {
	if rdb == nil {
		return
	}
	cache := service.NewRuntimeStatusCache(rdb, nil)
	cache.Invalidate(userID)
}
