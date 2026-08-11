package response

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var ErrNotFound = errors.New("not found")
var ErrUnauthorized = errors.New("unauthorized")
var ErrForbidden = errors.New("forbidden")
var ErrConflict = errors.New("conflict")
var ErrBadRequest = errors.New("bad request")

var businessErrors = map[error]struct {
	Status  int
	Code    string
	Message string
}{
	ErrNotFound:     {http.StatusNotFound, "NOT_FOUND", "resource not found"},
	ErrUnauthorized: {http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"},
	ErrForbidden:    {http.StatusForbidden, "FORBIDDEN", "access denied"},
	ErrConflict:     {http.StatusConflict, "CONFLICT", "resource conflict"},
	ErrBadRequest:   {http.StatusBadRequest, "BAD_REQUEST", "invalid request"},
}

func init() {
}

func SafeErrorMsg(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	for knownErr, known := range businessErrors {
		if errors.Is(err, knownErr) {
			return known.Message
		}
	}
	if strings.Contains(err.Error(), "sql:") || strings.Contains(err.Error(), "pq:") ||
		strings.Contains(err.Error(), "dsn") || strings.Contains(err.Error(), "record not found") {
		return fallback
	}
	return fallback
}

func SafeErrorResponse(c *gin.Context, status int, code string, err error) {
	msg := SafeErrorMsg(err, safeMessages[code])
	if msg == "" {
		msg = "an unexpected error occurred, please try again later"
	}
	c.AbortWithStatusJSON(status, gin.H{
		"code":    code,
		"message": msg,
	})
}

var safeMessages = map[string]string{
	"DB_ERROR":                        "database operation failed, please try again later",
	"INTERNAL_ERROR":                  "an unexpected error occurred, please try again later",
	"OSS_NOT_CONFIGURED":              "file storage is not configured",
	"TEST_FAILED":                     "connection test failed",
	"AGENT_ERROR":                     "agent service unavailable",
	"VALIDATION_ERROR":                "invalid request parameters",
	"INVALID_BODY":                    "invalid request body",
	"SOURCE_NOT_ALLOWED_FOR_ORIGINAL": "original content cannot carry a source attribution",
	"FANWORK_SOURCE_REQUIRED":         "fanwork content must specify an IP or an inspiration source",
	"MULTIPLE_SOURCE_CONFLICT":        "only one of source_original_id and source_fanwork_id may be set",
	"SOURCE_ORIGINAL_UNAVAILABLE":     "source original content does not exist or is unavailable",
	"SOURCE_FANWORK_UNAVAILABLE":      "source fanwork content does not exist or is unavailable",
	"SOURCE_IMMUTABLE":                "source attribution is immutable after creation",
	"MEDIA_SET_INVALID":               "media set violates the gallery contract",
	"PUBLISH_FROZEN":                  "publishing is temporarily frozen",
	"LOW_REPUTATION":                  "reputation score too low to perform this action",
	"BLOCKED":                         "you have been blocked from this action",
	"CONFLICT":                        "resource conflict",
	"ERROR":                           "an error occurred, please try again later",
	"CONTENT_NOT_FOUND":               "content not found",
}
