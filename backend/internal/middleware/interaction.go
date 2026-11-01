package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/rediskeys"
	"omnicraft/backend/internal/pkg/response"
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
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		cache := service.NewRuntimeStatusCache(rdb, cfg)
		status, err := service.ResolveRuntimeUserStatus(c.Request.Context(), db, cache, userID)
		if err != nil {
			if errors.Is(err, service.ErrUserStatusNotFound) || errors.Is(err, service.ErrUserStatusDeleted) {
				response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "user not found or deleted")
				return
			}
			response.Error(c, http.StatusServiceUnavailable, service.DenialReasonAuthStatusUnavailable, "account status is temporarily unavailable")
			return
		}

		if status.IsBanned {
			response.Error(c, http.StatusUnauthorized, service.DenialReasonUserBanned, "account has been banned")
			return
		}

		decision := service.EvaluateInteractionAccess(status, cfg, policy.RequireVerifiedEmail, policy.RequireReputation)
		if !decision.Allowed {
			switch decision.DenialReason {
			case service.DenialReasonEmailNotVerified:
				response.Error(c, http.StatusForbidden, decision.DenialReason, "email verification required")
			case service.DenialReasonInsufficientReputation:
				response.Error(c, http.StatusForbidden, decision.DenialReason, "reputation score too low to perform this action")
			case service.DenialReasonUserBanned:
				response.Error(c, http.StatusUnauthorized, decision.DenialReason, "account has been banned")
			default:
				response.Error(c, http.StatusServiceUnavailable, decision.DenialReason, "interaction status is temporarily unavailable")
			}
			return
		}

		if policy.RequireNoPublishFreeze {
			frozen, freezeErr := isPublishFrozen(c, rdb, userID)
			if freezeErr != nil {
				response.Error(c, http.StatusServiceUnavailable, service.DenialReasonAuthStatusUnavailable, "account status is temporarily unavailable")
				return
			}
			if frozen {
				response.Error(c, http.StatusForbidden, "PUBLISH_FROZEN", "publishing is temporarily frozen due to recent violations")
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
