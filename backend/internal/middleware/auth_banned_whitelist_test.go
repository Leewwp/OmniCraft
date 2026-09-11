package middleware

// T29（FIX-15）：封禁用户申诉出路——auth 中间件 banned 白名单矩阵。
// banned token × {GET /api/v1/auth/me, GET /api/v1/appeals/me, POST /api/v1/appeals}
// 放行（身份照常注入，封禁用户只能读写自己的申诉，callerID 天然隔离）；
// 其余 authReq 路由仍 401 USER_BANNED——白名单精确到 method+path，勿误放开。

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthRequiredBannedWhitelistMatrix(t *testing.T) {
	cfg := makeTestConfig()
	db := setupTestDB(t)
	insertTestUser(db, 1, "user", true, true, 10, nil)
	token := makeTestToken(cfg, 1, "user")

	r := setupTestRouter()
	r.Use(AuthRequired(cfg, nil, db))
	r.GET("/api/v1/auth/me", func(c *gin.Context) {
		c.JSON(200, gin.H{"user_id": GetUserID(c)})
	})
	r.GET("/api/v1/appeals/me", func(c *gin.Context) {
		c.JSON(200, gin.H{"user_id": GetUserID(c)})
	})
	r.POST("/api/v1/appeals", func(c *gin.Context) {
		c.JSON(201, gin.H{"user_id": GetUserID(c)})
	})
	// 同路径不同 method 不在白名单（如 DELETE /appeals/:id）。
	r.DELETE("/api/v1/appeals/1", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	// 非 appeals/auth-me 的常规受保护路由仍拦。
	r.GET("/api/v1/users/me", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	r.POST("/api/v1/contents", func(c *gin.Context) {
		c.JSON(201, gin.H{"ok": true})
	})
	// appeals 下其他子路径不在白名单（防前缀误放开）。
	r.GET("/api/v1/appeals/other", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	cases := []struct {
		name       string
		method     string
		path       string
		wantCode   int
		wantUserID float64
	}{
		{"auth/me 放行", "GET", "/api/v1/auth/me", 200, 1},
		{"appeals/me 读放行", "GET", "/api/v1/appeals/me", 200, 1},
		{"appeals 写放行", "POST", "/api/v1/appeals", 201, 1},
		{"users/me 仍拦", "GET", "/api/v1/users/me", 401, 0},
		{"contents 写仍拦", "POST", "/api/v1/contents", 401, 0},
		{"appeals 非白名单 method 仍拦", "DELETE", "/api/v1/appeals/1", 401, 0},
		{"appeals 非白名单子路径仍拦", "GET", "/api/v1/appeals/other", 401, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, tc.wantCode, w.Code, w.Body.String())
			if tc.wantCode == 401 {
				var body map[string]interface{}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.Equal(t, "USER_BANNED", body["code"])
			} else {
				var body map[string]interface{}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.Equal(t, tc.wantUserID, body["user_id"], "白名单路由必须照常注入身份")
			}
		})
	}
}
