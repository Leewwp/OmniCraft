package middleware

import (
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

		if rdb != nil {
			blacklistKey := fmt.Sprintf("blacklist:token:%s", tokenStr)
			val, redisErr := rdb.Get(c.Request.Context(), blacklistKey).Result()
			if redisErr == nil && val == "1" {
				c.JSON(401, gin.H{"code": "UNAUTHORIZED", "message": "token has been revoked"})
				c.Abort()
				return
			}
			if redisErr != nil && redisErr != redis.Nil {
				c.JSON(503, gin.H{"code": "AUTH_STATUS_UNAVAILABLE", "message": "account status is temporarily unavailable"})
				c.Abort()
				return
			}
		}

		cache := service.NewRuntimeStatusCache(rdb, cfg)
		status, resolveErr := service.ResolveRuntimeUserStatus(c.Request.Context(), dbInstance, cache, claims.UserID)
		if resolveErr != nil {
			errCode := resolveErr.Error()
			if errCode == "USER_NOT_FOUND" || errCode == "USER_DELETED" {
				c.JSON(401, gin.H{"code": "UNAUTHORIZED", "message": "user not found or deleted"})
				c.Abort()
				return
			}
			if status != nil && status.IsBanned {
				c.JSON(401, gin.H{"code": "USER_BANNED", "message": "account has been banned"})
				c.Abort()
				return
			}
			c.JSON(503, gin.H{"code": "AUTH_STATUS_UNAVAILABLE", "message": "account status is temporarily unavailable"})
			c.Abort()
			return
		}

		if status.IsBanned {
			c.JSON(401, gin.H{"code": "USER_BANNED", "message": "account has been banned"})
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
		role, exists := c.Get(UserRoleKey)
		if !exists || role != "admin" {
			c.JSON(403, gin.H{"code": "FORBIDDEN", "message": "admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
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
