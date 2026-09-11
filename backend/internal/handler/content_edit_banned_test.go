package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
)

// T43（FIX-13）handler 层：banned 内容 PATCH → 403 CONTENT_BANNED（终态禁改，
// 删除仍允许）；外链封面 → 400。

func TestUpdateBannedContentReturns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}))

	cfg := &config.Config{}
	cfg.JWT.Secret = "t43-banned-secret"
	contentHandler := NewContentHandler(db, cfg, nil)

	router := gin.New()
	router.PATCH("/api/v1/contents/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(2))
		contentHandler.UpdateContent(c)
	})

	author := model.User{ID: 2, Email: "t43-author@example.test", Username: "t43-author", PasswordHash: "hash", Reputation: 10}
	require.NoError(t, db.Create(&author).Error)
	banned := model.ContentItem{ID: 100, Title: "T43 banned", AuthorID: 2, Zone: "original", Category: "game", ContentType: "article", Status: "banned", BanReason: "spam"}
	require.NoError(t, db.Create(&banned).Error)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/contents/100", strings.NewReader(`{"title":"edited"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "CONTENT_BANNED")

	var still model.ContentItem
	require.NoError(t, db.First(&still, 100).Error)
	require.Equal(t, "T43 banned", still.Title, "banned 内容的编辑不得生效")

	// 删除仍允许（终态禁改不禁删）。
	router.DELETE("/api/v1/contents/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(2))
		contentHandler.DeleteContent(c)
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/contents/100", nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}
