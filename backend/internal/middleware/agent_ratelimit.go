package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"omnicraft/backend/config"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func AgentRateLimit(rdb *redis.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rdb == nil || cfg == nil {
			c.Next()
			return
		}
		userID := GetUserID(c)
		if userID == 0 {
			c.Next()
			return
		}
		limit := cfg.Agent.RateLimitPerDay
		if limit == 0 {
			limit = 50
		}

		date := time.Now().Format("2006-01-02")
		key := fmt.Sprintf("agent:ratelimit:%d:%s", userID, date)

		ctx := context.Background()
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if count == 1 {
			agentWindowTTL := 25 * time.Hour
			if cfg.RateLimit.AgentWindowSec > 0 {
				agentWindowTTL = time.Duration(cfg.RateLimit.AgentWindowSec) * time.Second
			}
			rdb.Expire(ctx, key, agentWindowTTL)
		}
		if int(count) > limit {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    "AGENT_RATE_LIMIT_EXCEEDED",
				"message": "daily agent request limit exceeded",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
