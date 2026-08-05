package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/internal/observability"
)

// PanicRecovery converts handler panics into a bounded 500 response. The
// panic value is never logged verbatim (it may carry untrusted data); only
// its type, the error_class and the stack are recorded.
func PanicRecovery(logger *slog.Logger, metrics *observability.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				metrics.IncPanics()
				logger.Error("http handler panic recovered",
					"error_class", "panic",
					"panic_type", typeName(r),
					"route", c.FullPath(),
					"method", c.Request.Method,
					"stack", string(debug.Stack()),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "internal server error",
				})
			}
		}()
		c.Next()
	}
}
