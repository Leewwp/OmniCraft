package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/config"
)

const csrfHeaderName = "X-CSRF-Token"
const csrfTokenLength = 32

func CSRF(cfg *config.Config) gin.HandlerFunc {
	isSecure := cfg.Server.Mode == "release"

	return func(c *gin.Context) {
		if isInternalPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Always read from "csrf-token" cookie.
		// In cross-domain setups (app.xxx → api.xxx), the browser will only
		// send back a cookie with SameSite=None; the old __Host-csrf cookie
		// (SameSite=Lax) is never returned by the browser on cross-origin
		// fetch requests, so we stop reading it.
		token, _ := c.Cookie("csrf-token")
		if token == "" {
			token = generateCSRFToken()
		}

		// Use a single cookie name "csrf-token" for all modes.
		// The __Host- prefix is incompatible with SameSite=None (RFC 6265bis
		// requires __Host- cookies to have no Domain attribute and the
		// browser semantics favour SameSite=Strict/Lax).  Since production
		// needs SameSite=None for cross-domain credential flows, we drop
		// the __Host- prefix entirely.
		cookieName := "csrf-token"

		if isSecure {
			c.SetSameSite(http.SameSiteNoneMode)
		} else {
			c.SetSameSite(http.SameSiteLaxMode)
		}
		c.SetCookie(cookieName, token, 0, "/", "", isSecure, false)
		c.Set("csrfToken", token)

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

func isInternalPath(path string) bool {
	internalPrefixes := []string{"/api/v1/internal/"}
	for _, prefix := range internalPrefixes {
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			return true
		}
	}
	if path == "/api/v1/deploy-grants" {
		return true
	}
	if len(path) >= len("/api/v1/payments/") && path[:len("/api/v1/payments/")] == "/api/v1/payments/" {
		return true
	}
	if path == "/api/v1/payments" {
		return true
	}
	return false
}

func GetCSRFToken(c *gin.Context) string {
	if val, exists := c.Get("csrfToken"); exists {
		if s, ok := val.(string); ok && s != "" {
			return s
		}
	}
	token, _ := c.Cookie("csrf-token")
	if token == "" {
		token = generateCSRFToken()
	}
	return token
}

func generateCSRFToken() string {
	b := make([]byte, csrfTokenLength)
	n, err := rand.Read(b)
	if err != nil || n != csrfTokenLength {
		panic("csrf: failed to generate random bytes")
	}
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
