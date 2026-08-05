package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/internal/observability"
)

// Logger emits one structured JSON log line per request with stable fields:
// trace_id/request_id (same value, fixed semantics), route (Gin full-path
// template or the bounded "unmatched" marker), method, status, duration_ms,
// client_ip (keyed HMAC hash, never raw), client_ip_key_id and error_class.
// Query strings and bodies are never logged.
func Logger(logger *slog.Logger, hasher *observability.IPHasher) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		route := observability.NormalizeRoute(c.FullPath())

		requestID, _ := c.Get("request_id")
		id := ""
		if requestID != nil {
			id, _ = requestID.(string)
		}

		status := c.Writer.Status()
		logger.Info("request",
			"trace_id", id,
			"request_id", id,
			// path is deliberately the same bounded Gin route template as route;
			// the raw URL path and query are never logged.
			"path", route,
			"route", route,
			"method", c.Request.Method,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", hasher.Hash(c.ClientIP()),
			"client_ip_key_id", hasher.KeyID(),
			"error_class", observability.ErrorClass(status),
		)
	}
}
