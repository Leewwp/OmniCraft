package middleware

import (
	"fmt"
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

		fmt.Printf("[%s] %d | %s | %s | %s %s\n",
			start.Format("2006/01/02 15:04:05"),
			statusCode,
			latency,
			clientIP,
			method,
			path,
		)
	}
}
