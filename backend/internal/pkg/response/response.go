package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Success sends a unified success response.
func Success(c *gin.Context, status int, data interface{}) {
	c.JSON(status, data)
}

// Error sends a unified error response with optional details.
// Format: { "code": "ERROR_CODE", "message": "..." }
// When details is non-nil: { "code": "ERROR_CODE", "message": "...", "details": ... }
func Error(c *gin.Context, status int, code string, message string) {
	body := gin.H{
		"code":    code,
		"message": message,
	}
	c.AbortWithStatusJSON(status, body)
}

// ErrorWithDetails sends a unified error response with details field.
func ErrorWithDetails(c *gin.Context, status int, code string, message string, details interface{}) {
	body := gin.H{
		"code":    code,
		"message": message,
		"details": details,
	}
	c.AbortWithStatusJSON(status, body)
}

// ValidationError is a shorthand for 400 VALIDATION_ERROR.
func ValidationError(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, "VALIDATION_ERROR", message)
}

// NotFound is a shorthand for 404 NOT_FOUND.
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, "NOT_FOUND", message)
}

// Unauthorized is a shorthand for 401 UNAUTHORIZED.
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

// Forbidden is a shorthand for 403 FORBIDDEN.
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, "FORBIDDEN", message)
}

// Conflict is a shorthand for 409 CONFLICT.
func Conflict(c *gin.Context, message string) {
	Error(c, http.StatusConflict, "CONFLICT", message)
}

// InternalError is a shorthand for 500 INTERNAL_ERROR.
func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", message)
}
