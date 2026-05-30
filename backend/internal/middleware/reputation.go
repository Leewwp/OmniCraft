package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/internal/pkg/response"
)

func CheckReputation(db *gorm.DB, rdb *redis.Client, minScore int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get(UserIDKey)
		if !exists {
			response.Unauthorized(c, "authentication required")
			c.Abort()
			return
		}

		var reputation int64
		if rdb != nil {
			val, err := rdb.Get(c.Request.Context(), fmt.Sprintf("user:reputation:%v", userID)).Int64()
			if err == nil {
				reputation = val
			}
		}

		if reputation == 0 {
			var user struct{ Reputation int64 }
			if err := db.Table("users").Select("reputation").Where("id = ? AND deleted_at IS NULL", userID).Scan(&user).Error; err != nil {
				response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
				c.Abort()
				return
			}
			reputation = user.Reputation
			if rdb != nil {
				rdb.Set(c.Request.Context(), fmt.Sprintf("user:reputation:%v", userID), reputation, 5*time.Minute)
			}
		}

		if reputation < minScore {
			response.Error(c, http.StatusForbidden, "REPUTATION_TOO_LOW", "reputation score too low to perform this action")
			c.Abort()
			return
		}
		c.Next()
	}
}
