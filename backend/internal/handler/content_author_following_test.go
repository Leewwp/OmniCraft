package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

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

// SP-17/T1 (#490)：内容详情 author.is_following 为登录视角字段——
// 关注作者=true、未关注=false（字段在位）、匿名不含该字段（不破坏
// CacheableAnonymousGET 的匿名缓存语义）；avatar_url 恒在位（签名管道已覆盖）。

type contentAuthorFollowingPayload struct {
	Content struct {
		ID int64 `json:"id"`
		Author struct {
			ID          int64  `json:"id"`
			Username    string `json:"username"`
			AvatarURL   string `json:"avatar_url"`
			IsFollowing *bool  `json:"is_following"`
		} `json:"author"`
	} `json:"content"`
}

func TestContentDetailAuthorFollowing(t *testing.T) {
	t.Run("anonymous response omits is_following and keeps avatar_url", func(t *testing.T) {
		router, db, _ := setupAuthorFollowingRouter(t)
		owner := seedAuthorFollowingUser(t, db, 10, "anon-owner")
		content := seedAuthorFollowingContent(t, db, 100, owner.ID)

		rec := getAuthorFollowingContent(t, router, content.ID, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		author := decodeAuthorFollowingBody(t, rec)
		require.Equal(t, owner.ID, author.ID)
		require.Equal(t, owner.Username, author.Username)
		require.Equal(t, owner.AvatarURL, author.AvatarURL, "author avatar_url must stay serialized")
		require.Nil(t, author.IsFollowing, "anonymous response must not expose is_following")
	})

	t.Run("logged-in follower sees is_following true", func(t *testing.T) {
		router, db, cfg := setupAuthorFollowingRouter(t)
		owner := seedAuthorFollowingUser(t, db, 10, "followed-owner")
		viewer := seedAuthorFollowingUser(t, db, 20, "follower-viewer")
		content := seedAuthorFollowingContent(t, db, 100, owner.ID)
		seedAuthorFollowingEdge(t, db, viewer.ID, owner.ID)

		rec := getAuthorFollowingContent(t, router, content.ID, authorFollowingToken(t, cfg, viewer.ID, viewer.Role))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.True(t, *decodeAuthorFollowingBody(t, rec).IsFollowing)
	})

	t.Run("logged-in non-follower sees is_following false", func(t *testing.T) {
		router, db, cfg := setupAuthorFollowingRouter(t)
		owner := seedAuthorFollowingUser(t, db, 10, "stranger-owner")
		viewer := seedAuthorFollowingUser(t, db, 20, "stranger-viewer")
		content := seedAuthorFollowingContent(t, db, 100, owner.ID)

		rec := getAuthorFollowingContent(t, router, content.ID, authorFollowingToken(t, cfg, viewer.ID, viewer.Role))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		isFollowing := decodeAuthorFollowingBody(t, rec).IsFollowing
		require.NotNil(t, isFollowing, "logged-in response must carry the field explicitly")
		require.False(t, *isFollowing)
	})

	t.Run("viewer-specific state never leaks into another viewer", func(t *testing.T) {
		router, db, cfg := setupAuthorFollowingRouter(t)
		owner := seedAuthorFollowingUser(t, db, 10, "leak-owner")
		viewer := seedAuthorFollowingUser(t, db, 20, "leak-viewer")
		other := seedAuthorFollowingUser(t, db, 30, "leak-other")
		content := seedAuthorFollowingContent(t, db, 100, owner.ID)
		seedAuthorFollowingEdge(t, db, other.ID, owner.ID)

		rec := getAuthorFollowingContent(t, router, content.ID, authorFollowingToken(t, cfg, viewer.ID, viewer.Role))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.False(t, *decodeAuthorFollowingBody(t, rec).IsFollowing, "another user's follow edge must not set the viewer state")
	})
}

func setupAuthorFollowingRouter(t *testing.T) (*gin.Engine, *gorm.DB, *config.Config) {
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
		&model.Collection{}, &model.CollectionItem{}, &model.Follow{},
	))

	cfg := &config.Config{}
	cfg.JWT.Secret = "content-author-following-secret"

	handler := NewContentHandler(db, cfg, nil)
	optAuth := middleware.OptionalAuth(cfg, nil, db)

	router := gin.New()
	router.GET("/api/v1/contents/:id", optAuth, handler.GetContent)

	return router, db, cfg
}

func seedAuthorFollowingUser(t *testing.T, db *gorm.DB, id int64, username string) model.User {
	t.Helper()
	user := model.User{
		ID:           id,
		Email:        username + "@example.com",
		Username:     username,
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
		AvatarURL:    "https://cdn.example.com/" + username + ".png",
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func seedAuthorFollowingContent(t *testing.T, db *gorm.DB, id, authorID int64) model.ContentItem {
	t.Helper()
	content := model.ContentItem{
		ID:          id,
		Title:       "Author following content",
		AuthorID:    authorID,
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		Status:      "published",
		IsPublic:    true,
		AllowCopy:   true,
	}
	require.NoError(t, db.Create(&content).Error)
	return content
}

func seedAuthorFollowingEdge(t *testing.T, db *gorm.DB, followerID, targetUserID int64) model.Follow {
	t.Helper()
	edge := model.Follow{
		FollowerID: followerID,
		TargetType: "user",
		TargetID:   targetUserID,
	}
	require.NoError(t, db.Create(&edge).Error)
	return edge
}

func getAuthorFollowingContent(t *testing.T, router *gin.Engine, contentID int64, token string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contents/"+strconv.FormatInt(contentID, 10), nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(rec, req)
	return rec
}

func decodeAuthorFollowingBody(t *testing.T, rec *httptest.ResponseRecorder) (author struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	AvatarURL   string `json:"avatar_url"`
	IsFollowing *bool  `json:"is_following"`
}) {
	t.Helper()
	var body contentAuthorFollowingPayload
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return body.Content.Author
}

func authorFollowingToken(t *testing.T, cfg *config.Config, userID int64, role string) string {
	t.Helper()
	pair, err := jwtutil.GenerateTokenPair(userID, role, cfg.JWT.Secret, 120, 7)
	require.NoError(t, err)
	return pair.AccessToken
}
