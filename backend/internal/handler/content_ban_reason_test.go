package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"omnicraft/backend/internal/model"
)

// FIX-16 (T08): admin 封禁填写的 reason 必须落库并让作者可见；
// ban_reason 仅作者/admin 视图暴露，匿名/他人响应不含。
func TestContentBanReasonVisibility(t *testing.T) {
	router, db, cfg := setupContentDetailFavoritedRouter(t)
	author := seedFavoritedStateUser(t, db, 10, "ban-author")
	admin := seedFavoritedStateUser(t, db, 20, "ban-admin")
	admin.Role = "admin"
	require.NoError(t, db.Save(&admin).Error)
	content := seedFavoritedStateContent(t, db, 100, author.ID, "original")
	require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ?", content.ID).
		Updates(map[string]interface{}{"status": "banned", "ban_reason": "违反社区准则：搬运未署名"}).Error)

	getBody := func(token string) map[string]any {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/contents/"+itoa64(content.ID), nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		return body
	}

	t.Run("anonymous view has no ban_reason", func(t *testing.T) {
		body := getBody("")
		c := body["content"].(map[string]any)
		require.NotContains(t, c, "ban_reason")
	})

	t.Run("author view carries ban_reason", func(t *testing.T) {
		body := getBody(favoritedStateToken(t, cfg, author.ID, author.Role))
		c := body["content"].(map[string]any)
		require.Equal(t, "违反社区准则：搬运未署名", c["ban_reason"])
	})

	t.Run("admin view carries ban_reason", func(t *testing.T) {
		body := getBody(favoritedStateToken(t, cfg, admin.ID, admin.Role))
		c := body["content"].(map[string]any)
		require.Equal(t, "违反社区准则：搬运未署名", c["ban_reason"])
	})

	t.Run("other user view has no ban_reason", func(t *testing.T) {
		other := seedFavoritedStateUser(t, db, 30, "ban-other")
		body := getBody(favoritedStateToken(t, cfg, other.ID, other.Role))
		c := body["content"].(map[string]any)
		require.NotContains(t, c, "ban_reason")
	})
}

func TestMyContentsCarryBanReasonForAuthor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, db, cfg := setupContentDetailFavoritedRouter(t)
	author := seedFavoritedStateUser(t, db, 10, "myc-author")
	content := seedFavoritedStateContent(t, db, 100, author.ID, "original")
	require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ?", content.ID).
		Updates(map[string]interface{}{"status": "banned", "ban_reason": "T08 作者侧可见性"}).Error)

	// register the self-path route with a caller-ID stub middleware
	router.GET("/api/v1/users/me/contents", func(c *gin.Context) {
		c.Set("userID", author.ID)
		c.Next()
	}, func(c *gin.Context) {
		handler := NewUserHandler(db, nil, nil, cfg)
		handler.GetMyContents(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/contents?page=1&page_size=10", nil)
	req.Header.Set("Authorization", "Bearer "+favoritedStateToken(t, cfg, author.ID, author.Role))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body struct {
		Contents []map[string]any `json:"contents"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotEmpty(t, body.Contents)
	found := false
	for _, item := range body.Contents {
		if item["id"].(float64) == float64(content.ID) {
			found = true
			require.Equal(t, "T08 作者侧可见性", item["ban_reason"])
		}
	}
	require.True(t, found, "banned content must appear in the author's own list")
}
