package middleware

import (
	"net/http"
	"strings"

	"omnicraft/backend/config"

	"github.com/gin-gonic/gin"
)

func CORS(cfg *config.Config) gin.HandlerFunc {
	allowed := make(map[string]bool)
	for _, o := range cfg.Security.AllowedOrigins {
		allowed[strings.TrimSpace(o)] = true
	}

	localhostVariants := []string{
		"http://localhost:3000",
		"http://localhost:3001",
		"http://127.0.0.1:3000",
		"http://127.0.0.1:3001",
	}
	if cfg.Server.Mode != "release" {
		for _, v := range localhostVariants {
			if !allowed[v] {
				allowed[v] = true
			}
		}
	}

	allowedMethods := "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	allowedHeaders := "Origin, Content-Type, Authorization, X-Requested-With, X-CSRF-Token"
	exposeHeaders := "Content-Length"
	maxAge := "86400"

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		c.Header("Access-Control-Allow-Methods", allowedMethods)
		c.Header("Access-Control-Allow-Headers", allowedHeaders)
		c.Header("Access-Control-Expose-Headers", exposeHeaders)
		c.Header("Access-Control-Max-Age", maxAge)

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
