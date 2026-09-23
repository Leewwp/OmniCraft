package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

// FR-06（中-10）：/users/me/contents 响应携带全量 totals 聚合——工作台概览卡
// 的 totalViews/totalLikes 不得再由首页 5 条 reduce（分页数据只覆盖当前页）。
func TestMyContentsReturnsAuthorTotals(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.Category{}))

	author := model.User{Email: "totals-author@example.com", Username: "totals-author", PasswordHash: "hash", Role: "user"}
	require.NoError(t, db.Create(&author).Error)

	// 6 条内容（超过一页 page_size=5），合计 views=33 / likes=6。
	for i := 0; i < 6; i++ {
		require.NoError(t, db.Create(&model.ContentItem{
			AuthorID: author.ID, Title: "t", Zone: "original", ContentType: "text",
			Status: "published", ViewCount: int64(3 + i), LikeCount: 1,
		}).Error)
	}
	// 他人内容不进作者合计。
	other := model.User{Email: "totals-other@example.com", Username: "totals-other", PasswordHash: "hash", Role: "user"}
	require.NoError(t, db.Create(&other).Error)
	require.NoError(t, db.Create(&model.ContentItem{
		AuthorID: other.ID, Title: "x", Zone: "original", ContentType: "text",
		Status: "published", ViewCount: 9999, LikeCount: 9999,
	}).Error)

	userHandler := newUserHandlerForTest(db, nil, nil, &config.Config{})
	router := gin.New()
	router.GET("/api/v1/users/me/contents", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, author.ID)
		c.Next()
	}, userHandler.GetMyContents)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/contents?page=1&page_size=5", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body struct {
		Contents []map[string]any `json:"contents"`
		Total    int64            `json:"total"`
		Totals   struct {
			Views int64 `json:"views"`
			Likes int64 `json:"likes"`
		} `json:"totals"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, int64(6), body.Total)
	require.Len(t, body.Contents, 5, "first page carries 5 items")
	require.Equal(t, int64(33), body.Totals.Views, "totals must aggregate ALL author contents, not the loaded page")
	require.Equal(t, int64(6), body.Totals.Likes)
}

// repo 层守卫：content_type 过滤与软删排除。
func TestAuthorContentTotalsFiltersByTypeAndSoftDelete(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ContentItem{}, &model.Category{}))

	require.NoError(t, db.Create(&model.ContentItem{
		AuthorID: 1, Title: "text", Zone: "original", ContentType: "text",
		Status: "published", ViewCount: 10, LikeCount: 2,
	}).Error)
	require.NoError(t, db.Create(&model.ContentItem{
		AuthorID: 1, Title: "video", Zone: "original", ContentType: "video",
		Status: "published", ViewCount: 40, LikeCount: 4,
	}).Error)
	deleted := model.ContentItem{
		AuthorID: 1, Title: "gone", Zone: "original", ContentType: "text",
		Status: "published", ViewCount: 1000, LikeCount: 1000,
	}
	require.NoError(t, db.Create(&deleted).Error)
	require.NoError(t, db.Delete(&deleted).Error) // soft delete

	repo := repository.NewContentRepository(db)
	views, likes, err := repo.AuthorContentTotals(1, "text")
	require.NoError(t, err)
	require.Equal(t, int64(10), views, "type filter + soft-delete exclusion")
	require.Equal(t, int64(2), likes)

	viewsAll, likesAll, err := repo.AuthorContentTotals(1, "")
	require.NoError(t, err)
	require.Equal(t, int64(50), viewsAll)
	require.Equal(t, int64(6), likesAll)
}
