package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/internal/observability"
)

// Metrics records low-cardinality request metrics: only route templates
// (never raw paths or IDs), methods and status classes become labels.
func Metrics(m *observability.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := observability.NormalizeRoute(c.FullPath())
		m.ObserveHTTPRequest(c.Request.Method, route, statusClass(c.Writer.Status()), time.Since(start).Seconds())
	}
}

func typeName(v any) string {
	return fmt.Sprintf("%T", v)
}

func statusClass(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	case status >= 200:
		return "2xx"
	default:
		return "1xx"
	}
}
