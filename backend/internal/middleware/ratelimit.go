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

func RateLimit(rdb *redis.Client, cfg *config.RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rdb == nil || cfg == nil || !cfg.Enabled {
			c.Next()
			return
		}
		limit := cfg.NormalPerMinute
		if limit <= 0 {
			limit = 100
		}

		ip := c.ClientIP()
		window := time.Now().Unix() / 60
		key := fmt.Sprintf("ratelimit:ip:%s:%d", ip, window)

		ctx := context.Background()
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if count == 1 {
			windowTTL := 2 * time.Minute
			if cfg.NormalWindowSec > 0 {
				windowTTL = time.Duration(cfg.NormalWindowSec) * time.Second
			}
			rdb.Expire(ctx, key, windowTTL)
		}
		if int(count) > limit {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    "RATE_LIMIT_EXCEEDED",
				"message": "too many requests, please try again later",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func UploadRateLimit(rdb *redis.Client, cfg *config.RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rdb == nil || cfg == nil || !cfg.Enabled {
			c.Next()
			return
		}
		limit := cfg.UploadPerHour
		if limit <= 0 {
			limit = 10
		}

		userID := GetUserID(c)
		if userID == 0 {
			c.Next()
			return
		}
		window := time.Now().Unix() / 3600
		key := fmt.Sprintf("ratelimit:upload:%d:%d", userID, window)

		ctx := context.Background()
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if count == 1 {
			uploadWindowTTL := 2 * time.Hour
			if cfg.UploadWindowSec > 0 {
				uploadWindowTTL = time.Duration(cfg.UploadWindowSec) * time.Second
			}
			rdb.Expire(ctx, key, uploadWindowTTL)
		}
		if int(count) > limit {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    "UPLOAD_RATE_LIMIT_EXCEEDED",
				"message": "upload limit exceeded, please try again later",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func CredentialRateLimit(rdb *redis.Client, cfg *config.RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rdb == nil || cfg == nil || !cfg.Enabled {
			c.Next()
			return
		}

		limit := 5
		ip := c.ClientIP()
		window := time.Now().Unix() / 60
		key := fmt.Sprintf("ratelimit:credential:%s:%d", ip, window)

		ctx := context.Background()
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if count == 1 {
			rdb.Expire(ctx, key, time.Duration(cfg.NormalWindowSec)*time.Second)
		}
		if int(count) > limit {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    "CREDENTIAL_RATE_LIMIT_EXCEEDED",
				"message": "too many credential attempts, please try again later",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func CommentEditRateLimit(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rdb == nil {
			c.Next()
			return
		}

		userID := GetUserID(c)
		if userID == 0 {
			c.Next()
			return
		}

		limit := 5
		window := time.Now().Unix() / 30
		key := fmt.Sprintf("ratelimit:comment_edit:%d:%d", userID, window)

		ctx := context.Background()
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if count == 1 {
			rdb.Expire(ctx, key, 60*time.Second)
		}
		if int(count) > limit {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    "COMMENT_EDIT_RATE_LIMIT",
				"message": "comment edit rate limit exceeded, please try again later",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
