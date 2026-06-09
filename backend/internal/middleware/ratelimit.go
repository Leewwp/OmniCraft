package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
		if cfg == nil || !cfg.Enabled {
			c.Next()
			return
		}
		if rdb == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": "RATE_LIMIT_UNAVAILABLE", "message": "rate limit temporarily unavailable"})
			c.Abort()
			return
		}

		limit := cfg.CredentialPerMinute
		if limit <= 0 {
			limit = 5
		}
		ip := c.ClientIP()
		window := time.Now().Unix() / 60
		keys := []string{
			fmt.Sprintf("ratelimit:credential:ip:%s:%d", ip, window),
		}
		if accountKey := credentialAccountKey(c); accountKey != "" {
			keys = append(keys, fmt.Sprintf("ratelimit:credential:acct:%s:%d", accountKey, window))
		}

		ctx := context.Background()
		credWindowTTL := 2 * time.Minute
		if cfg.NormalWindowSec > 0 {
			credWindowTTL = time.Duration(cfg.NormalWindowSec) * time.Second
		}
		for _, key := range keys {
			count, err := rdb.Incr(ctx, key).Result()
			if err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"code": "RATE_LIMIT_UNAVAILABLE", "message": "rate limit temporarily unavailable"})
				c.Abort()
				return
			}
			if count == 1 {
				rdb.Expire(ctx, key, credWindowTTL)
			}
			if int(count) > limit {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"code":    "CREDENTIAL_RATE_LIMIT_EXCEEDED",
					"message": "too many credential attempts, please try again later",
				})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

func RedisFixedWindowLimit(rdb *redis.Client, keyPrefix string, limit int, window time.Duration, failClosed bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limit <= 0 {
			c.Next()
			return
		}
		if window <= 0 {
			window = time.Minute
		}
		if rdb == nil {
			if failClosed {
				c.JSON(http.StatusServiceUnavailable, gin.H{"code": "RATE_LIMIT_UNAVAILABLE", "message": "rate limit temporarily unavailable"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		ip := c.ClientIP()
		windowID := time.Now().UnixNano() / int64(window)
		key := fmt.Sprintf("%s:%s:%d", keyPrefix, ip, windowID)

		ctx := context.Background()
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			if failClosed {
				c.JSON(http.StatusServiceUnavailable, gin.H{"code": "RATE_LIMIT_UNAVAILABLE", "message": "rate limit temporarily unavailable"})
				c.Abort()
				return
			}
			c.Next()
			return
		}
		if count == 1 {
			rdb.Expire(ctx, key, 2*window)
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

func credentialAccountKey(c *gin.Context) string {
	var body struct {
		Email string `json:"email"`
	}
	if c.Request.Body == nil {
		return ""
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	normalized := strings.ToLower(strings.TrimSpace(body.Email))
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
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
