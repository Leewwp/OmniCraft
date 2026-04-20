package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/config"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
)

const UserIDKey = "userID"
const UserRoleKey = "userRole"

func AuthRequired(cfg *config.Config) gin.HandlerFunc {
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

		c.Set(UserIDKey, claims.UserID)
		c.Set(UserRoleKey, claims.Role)
		c.Next()
	}
}

func OptionalAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			claims, err := jwtutil.ParseToken(parts[1], cfg.JWT.Secret)
			if err == nil && claims.Subject == "access" {
				c.Set(UserIDKey, claims.UserID)
				c.Set(UserRoleKey, claims.Role)
			}
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
