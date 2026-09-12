package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/service"
)

const csrfHeaderName = "X-CSRF-Token"
const csrfTokenLength = 32

// csrfCookieName returns the double-submit cookie name for the mode.
// Release uses the __Host- prefix (Secure, Path=/, no Domain) so no sibling
// subdomain can set or toss this cookie with Domain=.leeppp.online (audit
// F-08). Debug keeps the plain name: __Host- requires Secure, which a local
// http dev stack cannot honor.
func csrfCookieName(cfg *config.Config) string {
	if cfg.Server.Mode == "release" {
		return "__Host-csrf"
	}
	return "csrf-token"
}

func CSRF(cfg *config.Config) gin.HandlerFunc {
	isSecure := cfg.Server.Mode == "release"
	cookieName := csrfCookieName(cfg)

	return func(c *gin.Context) {
		if isInternalPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Machine channels carry their own explicit credentials (or none at
		// all) and never ambient cookies, so CSRF does not apply — neither
		// the double-submit check nor cookie issuance.
		// - PAT requests (#450): the web frontend always authenticates with
		//   JWTs, never PATs, so this cannot become a browser-side bypass.
		// - The MCP endpoint (#449): anonymous Streamable-HTTP JSON-RPC,
		//   per-IP rate-limited; authenticated MCP tools (#451) use the
		//   Authorization header, which a cross-site form cannot set.
		if requestCarriesPAT(c) || isMCPProtocolPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		token, _ := c.Cookie(cookieName)
		if token == "" {
			token = generateCSRFToken()
		}

		// All legitimate requests are same-site (app.leeppp.online ->
		// api.leeppp.online share the registrable domain), so SameSite=Lax
		// rides along on every real flow while cross-site requests stop at
		// the browser (audit F-08: SameSite=None only widened the
		// cookie-tossing surface without any legitimate cross-site flow
		// needing it).
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(cookieName, token, 0, "/", "", isSecure, false)
		c.Set("csrfToken", token)
		c.Set("csrfCookieName", cookieName)

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

// requestCarriesPAT reports whether the Authorization header authenticates
// via the PAT machine channel ("Bearer oc_pat_...").
func requestCarriesPAT(c *gin.Context) bool {
	header := c.GetHeader("Authorization")
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return false
	}
	return strings.HasPrefix(parts[1], service.AgentTokenPrefix)
}

func GetCSRFToken(c *gin.Context) string {
	if val, exists := c.Get("csrfToken"); exists {
		if s, ok := val.(string); ok && s != "" {
			return s
		}
	}
	// Outside the CSRF middleware (or when it skipped cookie issuance for a
	// machine channel), fall back to reading the cookie by the mode-aware
	// name the middleware stashed, else the debug-mode default.
	cookieName := "csrf-token"
	if val, exists := c.Get("csrfCookieName"); exists {
		if s, ok := val.(string); ok && s != "" {
			cookieName = s
		}
	}
	token, _ := c.Cookie(cookieName)
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

func isMCPProtocolPath(path string) bool {
	return path == "/api/v1/mcp"
}
