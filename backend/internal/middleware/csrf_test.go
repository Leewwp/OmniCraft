package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
)

func setupCSRFProbe(t *testing.T, mode string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{Server: config.ServerConfig{Mode: mode}}
	r := gin.New()
	r.Use(CSRF(cfg))
	r.GET("/probe", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/probe", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

// F-08 regression: in release mode the double-submit cookie must carry the
// __Host- prefix with its mandatory attributes (Secure, Path=/, no Domain)
// and SameSite=Lax. The regression shipped SameSite=None on a prefix-less
// cookie settable with Domain=.leeppp.online, so any sibling subdomain
// could cookie-toss a known CSRF pair.
func TestCSRFCookieAttributesReleaseMode(t *testing.T) {
	r := setupCSRFProbe(t, "release")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))
	require.Equal(t, http.StatusOK, w.Code)

	setCookie := w.Header().Get("Set-Cookie")
	require.NotEmpty(t, setCookie, "GET must bootstrap the CSRF cookie")
	require.Contains(t, setCookie, "__Host-csrf=", "release must use the __Host- prefixed name")
	require.Contains(t, setCookie, "Secure")
	require.Contains(t, setCookie, "Path=/")
	require.Contains(t, setCookie, "SameSite=Lax")
	require.NotContains(t, setCookie, "SameSite=None")
	require.NotContains(t, setCookie, "Domain=", "no Domain attribute: __Host- semantics and cookie-toss hardening")
}

// Debug mode keeps the plain name: the __Host- prefix requires Secure,
// which cannot be honored on a local http dev stack.
func TestCSRFCookieAttributesDebugMode(t *testing.T) {
	r := setupCSRFProbe(t, "debug")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))
	require.Equal(t, http.StatusOK, w.Code)

	setCookie := w.Header().Get("Set-Cookie")
	require.Contains(t, setCookie, "csrf-token=")
	require.NotContains(t, setCookie, "__Host-")
	require.NotContains(t, setCookie, "Secure")
	require.Contains(t, setCookie, "SameSite=Lax")
	require.NotContains(t, setCookie, "Domain=")
}

// The double-submit contract is unchanged: a state-changing request must
// present the header matching the cookie, and the mode-aware cookie name
// must be the one being read back.
func TestCSRFDoubleSubmitRoundTrip(t *testing.T) {
	for _, mode := range []string{"release", "debug"} {
		t.Run(mode, func(t *testing.T) {
			r := setupCSRFProbe(t, mode)

			// bootstrap
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))
			require.Equal(t, http.StatusOK, w.Code)

			cookieHeader := w.Header().Get("Set-Cookie")
			cookieName := "csrf-token"
			if mode == "release" {
				cookieName = "__Host-csrf"
			}
			require.Contains(t, cookieHeader, cookieName+"=", "cookie must use the mode-aware name")
			cookieValue := extractCookieValue(t, cookieHeader, cookieName)

			// POST without header -> rejected
			w = httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/probe", nil)
			req.Header.Set("Cookie", cookieName+"="+cookieValue)
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusForbidden, w.Code)

			// POST with matching header -> accepted
			w = httptest.NewRecorder()
			req = httptest.NewRequest(http.MethodPost, "/probe", nil)
			req.Header.Set("Cookie", cookieName+"="+cookieValue)
			req.Header.Set("X-CSRF-Token", cookieValue)
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func extractCookieValue(t *testing.T, setCookieHeader, name string) string {
	t.Helper()
	for _, part := range splitSetCookie(setCookieHeader) {
		if len(part) > len(name)+1 && part[:len(name)] == name && part[len(name)] == '=' {
			return part[len(name)+1:]
		}
	}
	t.Fatalf("cookie %s not found in %q", name, setCookieHeader)
	return ""
}

func splitSetCookie(header string) []string {
	parts := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(header); i++ {
		if header[i] == ';' {
			parts = append(parts, trimSpace(header[start:i]))
			start = i + 1
		}
	}
	parts = append(parts, trimSpace(header[start:]))
	return parts
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
