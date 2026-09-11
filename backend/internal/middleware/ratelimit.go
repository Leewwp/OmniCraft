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
		/* #400：无条件续期——原先仅 count==1 时设置 TTL，INCR 与 EXPIRE 之间失败
		   （或既有泄漏 key）会永生；固定窗口 TTL ≥ 窗口长度，重复刷新无害。 */
		windowTTL := 2 * time.Minute
		if cfg.NormalWindowSec > 0 {
			windowTTL = time.Duration(cfg.NormalWindowSec) * time.Second
		}
		rdb.Expire(ctx, key, windowTTL)
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

// ConsumeUploadQuota is the per-user hourly upload window shared by
// middleware.UploadRateLimit (web/JWT channel) and the MCP upload tools
// (SP-16 #451): external agents burn the same quota as studio uploads, never
// a parallel one. It increments first and rejects after, matching the
// middleware's historical counter semantics; nil redis fails open.
func ConsumeUploadQuota(ctx context.Context, rdb *redis.Client, cfg *config.RateLimitConfig, userID int64) error {
	if rdb == nil || cfg == nil || !cfg.Enabled || userID == 0 {
		return nil
	}
	limit := cfg.UploadPerHour
	if limit <= 0 {
		limit = 10
	}

	window := time.Now().Unix() / 3600
	key := fmt.Sprintf("ratelimit:upload:%d:%d", userID, window)

	count, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return nil
	}
	uploadWindowTTL := 2 * time.Hour
	if cfg.UploadWindowSec > 0 {
		uploadWindowTTL = time.Duration(cfg.UploadWindowSec) * time.Second
	}
	rdb.Expire(ctx, key, uploadWindowTTL)
	if int(count) > limit {
		return fmt.Errorf("upload limit exceeded (limit %d per window)", limit)
	}
	return nil
}

func UploadRateLimit(rdb *redis.Client, cfg *config.RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID == 0 {
			c.Next()
			return
		}
		if err := ConsumeUploadQuota(c.Request.Context(), rdb, cfg, userID); err != nil {
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
			rdb.Expire(ctx, key, credWindowTTL)
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
		rdb.Expire(ctx, key, 2*window)
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
		rdb.Expire(ctx, key, 60*time.Second)
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
