package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/config"
)

const csrfCookieName = "__Host-csrf"
const csrfHeaderName = "X-CSRF-Token"
const csrfTokenLength = 32

func CSRF(cfg *config.Config) gin.HandlerFunc {
	isSecure := cfg.Server.Mode == "release"

	return func(c *gin.Context) {
		token, err := c.Cookie(csrfCookieName)
		if err != nil || token == "" {
			token = generateCSRFToken()
		}

		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(csrfCookieName, token, 0, "/", "", isSecure, false)

		if c.Request.Method == http.MethodPost ||
			c.Request.Method == http.MethodPatch ||
			c.Request.Method == http.MethodPut ||
			c.Request.Method == http.MethodDelete {
			headerToken := c.GetHeader(csrfHeaderName)
			if headerToken == "" || !hmacEqual(headerToken, token) {
				c.JSON(http.StatusForbidden, gin.H{
					"code":    "CSRF_TOKEN_INVALID",
					"message": "CSRF token missing or invalid",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

func GetCSRFToken(c *gin.Context) string {
	token, _ := c.Cookie(csrfCookieName)
	if token == "" {
		token = generateCSRFToken()
	}
	return token
}

func generateCSRFToken() string {
	b := make([]byte, csrfTokenLength)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func hmacEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
