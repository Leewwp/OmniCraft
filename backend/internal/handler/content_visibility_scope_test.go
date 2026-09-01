package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

// FIX-12+43 (T09): 内容可见性统一收口——非 published 仅作者/admin 可读；
// 公共列表排除封禁作者内容与 is_public=false 私密内容。
func TestContentDetailVisibilityScope(t *testing.T) {
	router, db, cfg := setupContentDetailFavoritedRouter(t)
	author := seedFavoritedStateUser(t, db, 10, "vis-author")
	admin := seedFavoritedStateUser(t, db, 20, "vis-admin")
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", admin.ID).Update("role", "admin").Error)

	underReview := seedFavoritedStateContent(t, db, 100, author.ID, "original")
	require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ?", underReview.ID).Update("status", "under_review").Error)

	private := seedFavoritedStateContent(t, db, 101, author.ID, "original")
	require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ?", private.ID).Update("is_public", false).Error)

	bannedAuthor := seedFavoritedStateUser(t, db, 30, "vis-banned")
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", bannedAuthor.ID).Update("is_banned", true).Error)
	bannedAuthorContent := seedFavoritedStateContent(t, db, 102, bannedAuthor.ID, "original")

	get := func(id int64, token string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/contents/"+itoa64(id), nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	anonToken := ""
	t.Run("anonymous cannot read under_review/private/banned-author content", func(t *testing.T) {
		require.Equal(t, http.StatusNotFound, get(underReview.ID, anonToken))
		require.Equal(t, http.StatusNotFound, get(private.ID, anonToken))
		require.Equal(t, http.StatusNotFound, get(bannedAuthorContent.ID, anonToken))
	})

	t.Run("author reads own under_review and private content", func(t *testing.T) {
		token := favoritedStateToken(t, cfg, author.ID, author.Role)
		require.Equal(t, http.StatusOK, get(underReview.ID, token))
		require.Equal(t, http.StatusOK, get(private.ID, token))
	})

	t.Run("admin reads everything", func(t *testing.T) {
		token := favoritedStateToken(t, cfg, admin.ID, "admin")
		require.Equal(t, http.StatusOK, get(underReview.ID, token))
		require.Equal(t, http.StatusOK, get(bannedAuthorContent.ID, token))
	})

	t.Run("other user cannot read others private or banned-author content", func(t *testing.T) {
		other := seedFavoritedStateUser(t, db, 40, "vis-other")
		token := favoritedStateToken(t, cfg, other.ID, other.Role)
		require.Equal(t, http.StatusNotFound, get(private.ID, token))
		require.Equal(t, http.StatusNotFound, get(bannedAuthorContent.ID, token))
	})
}

func TestMainListVisibilityScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:vislist?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}))

	author := seedFavoritedStateUser(t, db, 10, "list-author")
	privateAuthor := seedFavoritedStateUser(t, db, 11, "list-private-author")
	banned := seedFavoritedStateUser(t, db, 30, "list-banned")
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", banned.ID).Update("is_banned", true).Error)

	seedFavoritedStateContent(t, db, 100, author.ID, "original")
	privateItem := seedFavoritedStateContent(t, db, 101, privateAuthor.ID, "original")
	require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ?", privateItem.ID).Update("is_public", false).Error)
	bannedItem := seedFavoritedStateContent(t, db, 102, banned.ID, "original")

	repo := repository.NewContentRepository(db)

	listIDs := func(viewerID int64) []int64 {
		items, _, err := repo.ListContents(repository.ListContentsFilter{ViewerID: viewerID, Page: 1, PageSize: 50})
		require.NoError(t, err)
		ids := make([]int64, 0, len(items))
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		return ids
	}

	t.Run("anonymous list excludes private and banned-author content", func(t *testing.T) {
		ids := listIDs(0)
		require.Contains(t, ids, int64(100))
		require.NotContains(t, ids, privateItem.ID, "private content must not appear in public lists")
		require.NotContains(t, ids, bannedItem.ID, "banned author's content must not appear in public lists")
	})

	t.Run("author sees own private content", func(t *testing.T) {
		ids := listIDs(privateAuthor.ID)
		require.Contains(t, ids, privateItem.ID)
	})

	t.Run("explicit status filter (admin queue) still lists under_review", func(t *testing.T) {
		require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ?", 100).Update("status", "under_review").Error)
		items, _, err := repo.ListContents(repository.ListContentsFilter{Status: "under_review", Page: 1, PageSize: 50})
		require.NoError(t, err)
		require.NotEmpty(t, items, "admin under_review queue must keep working")
	})
}
