package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"omnicraft/backend/config"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
)

const UserIDKey = "userID"
const UserRoleKey = "userRole"

type userStatus struct {
	IsBanned bool   `json:"is_banned"`
	Role     string `json:"role"`
}

func AuthRequired(cfg *config.Config, rdb *redis.Client) gin.HandlerFunc {
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
			val, redisErr := rdb.Get(context.Background(), blacklistKey).Result()
			if redisErr == nil && val == "1" {
				c.JSON(401, gin.H{"code": "UNAUTHORIZED", "message": "token has been revoked"})
				c.Abort()
				return
			}

			statusKey := fmt.Sprintf("user:status:%d", claims.UserID)
			statusStr, err := rdb.Get(context.Background(), statusKey).Result()
			if err == nil {
				var us userStatus
				if json.Unmarshal([]byte(statusStr), &us) == nil {
					if us.IsBanned {
						c.JSON(401, gin.H{"code": "USER_BANNED", "message": "account has been banned"})
						c.Abort()
						return
					}
					c.Set(UserIDKey, claims.UserID)
					c.Set(UserRoleKey, us.Role)
					c.Next()
					return
				}
			}
		}

		c.Set(UserIDKey, claims.UserID)
		c.Set(UserRoleKey, claims.Role)
		c.Next()
	}
}

func OptionalAuth(cfg *config.Config, rdb *redis.Client) gin.HandlerFunc {
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
			val, redisErr := rdb.Get(context.Background(), blacklistKey).Result()
			if redisErr == nil && val == "1" {
				c.Set(UserIDKey, int64(0))
				c.Next()
				return
			}

			statusKey := fmt.Sprintf("user:status:%d", claims.UserID)
			statusStr, err := rdb.Get(context.Background(), statusKey).Result()
			if err == nil {
				var us userStatus
				if json.Unmarshal([]byte(statusStr), &us) == nil {
					if us.IsBanned {
						c.Set(UserIDKey, int64(0))
						c.Next()
						return
					}
					c.Set(UserIDKey, claims.UserID)
					c.Set(UserRoleKey, us.Role)
					c.Next()
					return
				}
			}
		}

		c.Set(UserIDKey, claims.UserID)
		c.Set(UserRoleKey, claims.Role)
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
	status := userStatus{IsBanned: isBanned, Role: role}
	data, err := json.Marshal(status)
	if err != nil {
		return
	}
	statusKey := fmt.Sprintf("user:status:%d", userID)
	rdb.Set(context.Background(), statusKey, string(data), 5*time.Minute)
}

func InvalidateUserStatusCache(rdb *redis.Client, userID int64) {
	if rdb == nil {
		return
	}
	statusKey := fmt.Sprintf("user:status:%d", userID)
	rdb.Del(context.Background(), statusKey)
}