package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()

		if query != "" {
			path = path + "?" + query
		}

		requestID, _ := c.Get("request_id")
		slog.Info("request",
			"method", method,
			"path", path,
			"status", statusCode,
			"duration_ms", latency.Milliseconds(),
			"client_ip", clientIP,
			"request_id", requestID,
		)
	}
}
