package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
)

func TestCORSReleaseModeDoesNotAutoAllowLocalhost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Server: config.ServerConfig{Mode: "release"},
		Security: config.SecurityConfig{
			AllowedOrigins: []string{"https://app.leeppp.online"},
		},
	}

	r := gin.New()
	r.Use(CORS(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	allowedReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	allowedReq.Header.Set("Origin", "https://app.leeppp.online")
	allowedRec := httptest.NewRecorder()
	r.ServeHTTP(allowedRec, allowedReq)
	require.Equal(t, "https://app.leeppp.online", allowedRec.Header().Get("Access-Control-Allow-Origin"))

	localReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	localReq.Header.Set("Origin", "http://localhost:3000")
	localRec := httptest.NewRecorder()
	r.ServeHTTP(localRec, localReq)
	require.Empty(t, localRec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSPreflightAllowsIdempotencyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{Server: config.ServerConfig{Mode: "debug"}}

	r := gin.New()
	r.Use(CORS(cfg))
	r.POST("/api/v1/admin/notifications/broadcast", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/admin/notifications/broadcast", nil)
	req.Header.Set("Origin", "http://localhost:3001")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "content-type,x-csrf-token,idempotency-key")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "http://localhost:3001", rec.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Idempotency-Key")
}
