package middleware

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"omnicraft/backend/config"
)

var validate = validator.New()

// registeredContentTypes is the content_type validator's allowed set. It
// boots from the registry baseline and is refreshed from the loaded config
// at wiring (SetRegisteredContentTypes) so a registry-added category needs
// no validator code change (#687).
var (
	registeredContentTypesMu sync.RWMutex
	registeredContentTypes   = defaultContentTypeSet()
)

func defaultContentTypeSet() map[string]bool {
	set := make(map[string]bool)
	for _, entry := range config.DefaultContentRegistry().ContentTypes {
		set[entry.Key] = true
	}
	return set
}

// SetRegisteredContentTypes refreshes the content_type validation set from
// the loaded configuration. Called once during wiring; the boot default
// equals the shipped baseline so pre-wiring tests behave identically.
func SetRegisteredContentTypes(cfg *config.Config) {
	if cfg == nil {
		return
	}
	set := make(map[string]bool)
	for _, entry := range cfg.EffectiveContentTypes() {
		set[entry.Key] = true
	}
	registeredContentTypesMu.Lock()
	registeredContentTypes = set
	registeredContentTypesMu.Unlock()
}

func isRegisteredContentType(value string) bool {
	registeredContentTypesMu.RLock()
	set := registeredContentTypes
	registeredContentTypesMu.RUnlock()
	return set[value]
}

func init() {
	validate.RegisterValidation("content_type", func(fl validator.FieldLevel) bool {
		return isRegisteredContentType(fl.Field().String())
	})

	validate.RegisterValidation("safe_username", func(fl validator.FieldLevel) bool {
		for _, r := range fl.Field().String() {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
				return false
			}
		}
		return true
	})
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func BindAndValidate(obj interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := c.ShouldBindJSON(obj); err != nil {
			errs, ok := err.(validator.ValidationErrors)
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "invalid request body",
				})
				c.Abort()
				return
			}

			var details []ValidationError
			for _, e := range errs {
				details = append(details, ValidationError{
					Field:   jsonFieldName(e.Namespace()),
					Message: fieldErrorMessage(e),
				})
			}

			c.JSON(http.StatusBadRequest, gin.H{
				"code":    "VALIDATION_ERROR",
				"message": "validation failed",
				"details": details,
			})
			c.Abort()
			return
		}

		c.Set("validated", obj)
		c.Next()
	}
}

func jsonFieldName(ns string) string {
	parts := strings.Split(ns, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ns
}

func fieldErrorMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "this field is required"
	case "min":
		return "must be at least " + e.Param() + " characters"
	case "max":
		return "must be at most " + e.Param() + " characters"
	case "email":
		return "must be a valid email address"
	case "oneof":
		return "must be one of: " + e.Param()
	case "content_type":
		return "invalid content type"
	case "safe_username":
		return "must contain only letters, numbers, and underscores"
	}
	return e.Tag()
}
