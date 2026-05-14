package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func init() {
	validate.RegisterValidation("content_type", func(fl validator.FieldLevel) bool {
		allowed := map[string]bool{
			"image": true, "article": true, "video": true, "audio": true,
			"template": true, "sheet_music": true, "mod": true, "prompt": true, "other": true,
		}
		return allowed[fl.Field().String()]
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
