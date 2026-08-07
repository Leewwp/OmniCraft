package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/rediskeys"
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
			if errors.Is(err, service.ErrUserStatusNotFound) || errors.Is(err, service.ErrUserStatusDeleted) {
				c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "user not found or deleted"})
				c.Abort()
				return
			}
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": service.DenialReasonAuthStatusUnavailable, "message": "account status is temporarily unavailable"})
			c.Abort()
			return
		}

		if status.IsBanned {
			c.JSON(http.StatusUnauthorized, gin.H{"code": service.DenialReasonUserBanned, "message": "account has been banned"})
			c.Abort()
			return
		}

		decision := service.EvaluateInteractionAccess(status, cfg, policy.RequireVerifiedEmail, policy.RequireReputation)
		if !decision.Allowed {
			switch decision.DenialReason {
			case service.DenialReasonEmailNotVerified:
				c.JSON(http.StatusForbidden, gin.H{"code": decision.DenialReason, "message": "email verification required"})
			case service.DenialReasonInsufficientReputation:
				c.JSON(http.StatusForbidden, gin.H{"code": decision.DenialReason, "message": "reputation score too low to perform this action"})
			case service.DenialReasonUserBanned:
				c.JSON(http.StatusUnauthorized, gin.H{"code": decision.DenialReason, "message": "account has been banned"})
			default:
				c.JSON(http.StatusServiceUnavailable, gin.H{"code": decision.DenialReason, "message": "interaction status is temporarily unavailable"})
			}
			c.Abort()
			return
		}

		if policy.RequireNoPublishFreeze {
			frozen, freezeErr := isPublishFrozen(c, rdb, userID)
			if freezeErr != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"code": service.DenialReasonAuthStatusUnavailable, "message": "account status is temporarily unavailable"})
				c.Abort()
				return
			}
			if frozen {
				c.JSON(http.StatusForbidden, gin.H{"code": "PUBLISH_FROZEN", "message": "publishing is temporarily frozen due to recent violations"})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

func isPublishFrozen(c *gin.Context, rdb *redis.Client, userID int64) (bool, error) {
	if rdb == nil {
		return false, errors.New("redis unavailable")
	}
	key := rediskeys.PublishFreezeKey(userID)
	_, err := rdb.Get(c.Request.Context(), key).Result()
	if err == nil {
		return true, nil
	}
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	return false, err
}
