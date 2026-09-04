package middleware

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
	"omnicraft/backend/internal/service"
)

const UserIDKey = "userID"
const UserRoleKey = "userRole"

func AuthRequired(cfg *config.Config, rdb *redis.Client, db ...*gorm.DB) gin.HandlerFunc {
	var dbInstance *gorm.DB
	if len(db) > 0 {
		dbInstance = db[0]
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

		if status.IsBanned {
			c.JSON(401, gin.H{"code": service.DenialReasonUserBanned, "message": "account has been banned"})
			c.Abort()
			return
		}

		c.Set(UserIDKey, status.ID)
		c.Set(UserRoleKey, status.Role)
		c.Next()
	}
}

func OptionalAuth(cfg *config.Config, rdb *redis.Client, db ...*gorm.DB) gin.HandlerFunc {
	var dbInstance *gorm.DB
	if len(db) > 0 {
		dbInstance = db[0]
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
