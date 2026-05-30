package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/service"
)

type InteractionPolicy struct {
	RequireVerifiedEmail   bool
	RequireReputation      bool
	RequireNoPublishFreeze bool
}

func InteractionRequired(cfg *config.Config, db *gorm.DB, rdb *redis.Client, policy InteractionPolicy) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "authentication required"})
			c.Abort()
			return
		}

		cache := service.NewRuntimeStatusCache(rdb, cfg)
		status, err := service.ResolveRuntimeUserStatus(c.Request.Context(), db, cache, userID)
		if err != nil {
			errCode := err.Error()
			if errCode == "USER_NOT_FOUND" || errCode == "USER_DELETED" {
				c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "user not found or deleted"})
				c.Abort()
				return
			}
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": "AUTH_STATUS_UNAVAILABLE", "message": "account status is temporarily unavailable"})
			c.Abort()
			return
		}

		if status.IsBanned {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "USER_BANNED", "message": "account has been banned"})
			c.Abort()
			return
		}

		if policy.RequireVerifiedEmail {
			if status.EmailVerifiedAt == nil {
				c.JSON(http.StatusForbidden, gin.H{"code": "EMAIL_NOT_VERIFIED", "message": "email verification required"})
				c.Abort()
				return
			}
		}

		if policy.RequireReputation {
			if cfg.Reputation.MinScoreForInteraction <= 0 {
				c.JSON(http.StatusServiceUnavailable, gin.H{"code": "CONFIG_ERROR", "message": "reputation threshold is misconfigured"})
				c.Abort()
				return
			}
			if status.Reputation < cfg.Reputation.MinScoreForInteraction {
				c.JSON(http.StatusForbidden, gin.H{"code": "INSUFFICIENT_REPUTATION", "message": "reputation score too low to perform this action"})
				c.Abort()
				return
			}
		}

		if policy.RequireNoPublishFreeze {
			if isPublishFrozen(c, rdb, userID) {
				c.JSON(http.StatusForbidden, gin.H{"code": "PUBLISH_FROZEN", "message": "publishing is temporarily frozen due to recent violations"})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

func isPublishFrozen(c *gin.Context, rdb *redis.Client, userID int64) bool {
	if rdb == nil {
		return false
	}
	key := fmt.Sprintf("user:publish_freeze:%d", userID)
	_, err := rdb.Get(c.Request.Context(), key).Result()
	return err == nil
}
