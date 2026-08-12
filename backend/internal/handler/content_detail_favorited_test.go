package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
)

// #74: 内容详情 `is_favorited` 的唯一事实源是收藏成员关系 —— 当前用户至少
// 一个活动收藏集包含该内容即为已收藏；已删除收藏集不计入；匿名视图不携带该字段。

type contentDetailFavoritedPayload struct {
	Content struct {
		ID int64 `json:"id"`
	} `json:"content"`
	IsFavorited *bool `json:"is_favorited"`
}

func TestContentDetailFavoritedStateFromCollectionMembership(t *testing.T) {
	t.Run("anonymous response does not carry is_favorited", func(t *testing.T) {
		router, db, _ := setupContentDetailFavoritedRouter(t)
		owner := seedFavoritedStateUser(t, db, 10, "anon-owner")
		content := seedFavoritedStateContent(t, db, 100, owner.ID, "original")
		collection := seedFavoritedStateCollection(t, db, 200, owner.ID, "original")
		seedFavoritedStateItem(t, db, 300, collection.ID, content.ID)

		rec := getFavoritedStateContent(t, router, content.ID, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		body := decodeFavoritedStateBody(t, rec)
		require.False(t, body.IsFavorited != nil, "anonymous response must not expose is_favorited, got %v", body.IsFavorited)
	})

	t.Run("no membership reports not favorited", func(t *testing.T) {
		router, db, cfg := setupContentDetailFavoritedRouter(t)
		owner := seedFavoritedStateUser(t, db, 10, "empty-owner")
		viewer := seedFavoritedStateUser(t, db, 20, "empty-viewer")
		content := seedFavoritedStateContent(t, db, 100, owner.ID, "original")

		rec := getFavoritedStateContent(t, router, content.ID, favoritedStateToken(t, cfg, viewer.ID, viewer.Role))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		body := decodeFavoritedStateBody(t, rec)
		require.NotNil(t, body.IsFavorited)
		require.False(t, *body.IsFavorited)
	})

	t.Run("single collection membership reports favorited", func(t *testing.T) {
		router, db, cfg := setupContentDetailFavoritedRouter(t)
		owner := seedFavoritedStateUser(t, db, 10, "single-owner")
		viewer := seedFavoritedStateUser(t, db, 20, "single-viewer")
		content := seedFavoritedStateContent(t, db, 100, owner.ID, "original")
		collection := seedFavoritedStateCollection(t, db, 200, viewer.ID, "original")
		seedFavoritedStateItem(t, db, 300, collection.ID, content.ID)

		rec := getFavoritedStateContent(t, router, content.ID, favoritedStateToken(t, cfg, viewer.ID, viewer.Role))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		body := decodeFavoritedStateBody(t, rec)
		require.True(t, *body.IsFavorited)
	})

	t.Run("membership in several collections stays favorited", func(t *testing.T) {
		router, db, cfg := setupContentDetailFavoritedRouter(t)
		owner := seedFavoritedStateUser(t, db, 10, "multi-owner")
		viewer := seedFavoritedStateUser(t, db, 20, "multi-viewer")
		content := seedFavoritedStateContent(t, db, 100, owner.ID, "original")
		first := seedFavoritedStateCollection(t, db, 200, viewer.ID, "original")
		second := seedFavoritedStateCollection(t, db, 201, viewer.ID, "original")
		firstItem := seedFavoritedStateItem(t, db, 300, first.ID, content.ID)
		seedFavoritedStateItem(t, db, 301, second.ID, content.ID)

		require.True(t, favoritedStateRead(t, router, cfg, viewer, content.ID), "member of two collections")

		// Removing from one of several collections keeps the state favorited.
		require.NoError(t, removeFavoritedStateItem(t, db, firstItem.ID))
		require.True(t, favoritedStateRead(t, router, cfg, viewer, content.ID), "still member of second collection")

		// Removing from the last active collection cancels the state.
		var remaining []model.CollectionItem
		require.NoError(t, db.Where("collection_id = ? AND content_item_id = ?", second.ID, content.ID).Find(&remaining).Error)
		require.Len(t, remaining, 1)
		require.NoError(t, removeFavoritedStateItem(t, db, remaining[0].ID))
		require.False(t, favoritedStateRead(t, router, cfg, viewer, content.ID), "last membership removed")
	})

	t.Run("deleted collection membership does not count", func(t *testing.T) {
		router, db, cfg := setupContentDetailFavoritedRouter(t)
		owner := seedFavoritedStateUser(t, db, 10, "deleted-owner")
		viewer := seedFavoritedStateUser(t, db, 20, "deleted-viewer")
		content := seedFavoritedStateContent(t, db, 100, owner.ID, "original")
		deleted := seedFavoritedStateCollection(t, db, 200, viewer.ID, "original")
		active := seedFavoritedStateCollection(t, db, 201, viewer.ID, "original")
		seedFavoritedStateItem(t, db, 300, deleted.ID, content.ID)
		activeItem := seedFavoritedStateItem(t, db, 301, active.ID, content.ID)

		now := time.Now()
		require.NoError(t, db.Model(&model.Collection{}).Where("id = ?", deleted.ID).Update("deleted_at", &now).Error)
		require.True(t, favoritedStateRead(t, router, cfg, viewer, content.ID), "active membership still counts after sibling deletion")

		require.NoError(t, removeFavoritedStateItem(t, db, activeItem.ID))
		require.False(t, favoritedStateRead(t, router, cfg, viewer, content.ID), "only deleted-collection membership left")
	})

	t.Run("another user's membership never leaks into the viewer state", func(t *testing.T) {
		router, db, cfg := setupContentDetailFavoritedRouter(t)
		owner := seedFavoritedStateUser(t, db, 10, "leak-owner")
		viewer := seedFavoritedStateUser(t, db, 20, "leak-viewer")
		other := seedFavoritedStateUser(t, db, 30, "leak-other")
		content := seedFavoritedStateContent(t, db, 100, owner.ID, "original")
		otherCollection := seedFavoritedStateCollection(t, db, 200, other.ID, "original")
		seedFavoritedStateItem(t, db, 300, otherCollection.ID, content.ID)

		rec := getFavoritedStateContent(t, router, content.ID, favoritedStateToken(t, cfg, viewer.ID, viewer.Role))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		body := decodeFavoritedStateBody(t, rec)
		require.False(t, *body.IsFavorited, "another user's membership must not set the viewer state")
	})
}

func favoritedStateRead(t *testing.T, router *gin.Engine, cfg *config.Config, viewer model.User, contentID int64) bool {
	t.Helper()
	rec := getFavoritedStateContent(t, router, contentID, favoritedStateToken(t, cfg, viewer.ID, viewer.Role))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decodeFavoritedStateBody(t, rec)
	require.NotNil(t, body.IsFavorited)
	return *body.IsFavorited
}

func getFavoritedStateContent(t *testing.T, router *gin.Engine, contentID int64, token string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contents/"+strconv.FormatInt(contentID, 10), nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(rec, req)
	return rec
}

func decodeFavoritedStateBody(t *testing.T, rec *httptest.ResponseRecorder) contentDetailFavoritedPayload {
	t.Helper()
	var body contentDetailFavoritedPayload
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return body
}

func favoritedStateToken(t *testing.T, cfg *config.Config, userID int64, role string) string {
	t.Helper()
	pair, err := jwtutil.GenerateTokenPair(userID, role, cfg.JWT.Secret, 120, 7)
	require.NoError(t, err)
	return pair.AccessToken
}

func setupContentDetailFavoritedRouter(t *testing.T) (*gin.Engine, *gorm.DB, *config.Config) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentAttachment{}, &model.ContentTag{},
		&model.BrowseHistory{}, &model.ContentSeries{}, &model.ContentSeriesItem{},
		&model.Collection{}, &model.CollectionItem{},
	))

	cfg := &config.Config{}
	cfg.JWT.Secret = "content-detail-favorited-secret"

	handler := NewContentHandler(db, cfg, nil)
	optAuth := middleware.OptionalAuth(cfg, nil, db)

	router := gin.New()
	router.GET("/api/v1/contents/:id", optAuth, handler.GetContent)

	return router, db, cfg
}

func seedFavoritedStateUser(t *testing.T, db *gorm.DB, id int64, username string) model.User {
	t.Helper()
	now := time.Now()
	user := model.User{
		ID:              id,
		Email:           username + "@example.com",
		Username:        username,
		PasswordHash:    "hash",
		Reputation:      10,
		Role:            "user",
		EmailVerifiedAt: &now,
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func seedFavoritedStateContent(t *testing.T, db *gorm.DB, id, authorID int64, zone string) model.ContentItem {
	t.Helper()
	content := model.ContentItem{
		ID:          id,
		Title:       "Favorited state content",
		AuthorID:    authorID,
		Zone:        zone,
		Category:    "game",
		ContentType: "article",
		Status:      "published",
		IsPublic:    true,
		AllowCopy:   true,
	}
	require.NoError(t, db.Create(&content).Error)
	return content
}

func seedFavoritedStateCollection(t *testing.T, db *gorm.DB, id, userID int64, zone string) model.Collection {
	t.Helper()
	collection := model.Collection{
		ID:        id,
		UserID:    userID,
		Title:     "Favorited state collection",
		Zone:      zone,
		IsDefault: false,
		IsPublic:  false,
		SortOrder: 1,
	}
	require.NoError(t, db.Create(&collection).Error)
	return collection
}

func seedFavoritedStateItem(t *testing.T, db *gorm.DB, id, collectionID, contentItemID int64) model.CollectionItem {
	t.Helper()
	item := model.CollectionItem{
		ID:            id,
		CollectionID:  collectionID,
		ContentItemID: contentItemID,
	}
	require.NoError(t, db.Create(&item).Error)
	return item
}

func removeFavoritedStateItem(t *testing.T, db *gorm.DB, itemID int64) error {
	t.Helper()
	return db.Delete(&model.CollectionItem{}, itemID).Error
}
