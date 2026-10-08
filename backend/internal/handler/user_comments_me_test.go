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
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
)

// comments 表用手写建表（AutoMigrate 与 sqlite 不兼容，先例
// newT55DeleteCommentDB）；discussions 同款手写先例（discussion_moderation_
// gate_test）；content_items 沿 AutoMigrate 先例（message_invite_navigation_test）。
func newMyCommentsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ContentItem{}))
	require.NoError(t, db.Exec(`CREATE TABLE comments (
		id integer PRIMARY KEY AUTOINCREMENT,
		content_item_id integer,
		discussion_id integer,
		parent_id integer,
		author_id integer NOT NULL,
		target_type text,
		target_id integer,
		content text,
		body text NOT NULL,
		status text NOT NULL DEFAULT 'published',
		like_count integer NOT NULL DEFAULT 0,
		created_at datetime,
		updated_at datetime
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE discussions (
		id integer PRIMARY KEY AUTOINCREMENT,
		ip_id integer,
		content_item_id integer,
		author_id integer NOT NULL,
		title text NOT NULL,
		body text,
		status text NOT NULL DEFAULT 'published',
		is_pinned numeric NOT NULL DEFAULT 0,
		view_count integer NOT NULL DEFAULT 0,
		reply_count integer NOT NULL DEFAULT 0,
		last_active_at datetime NOT NULL DEFAULT (datetime('now')),
		created_at datetime,
		updated_at datetime
	)`).Error)
	return db
}

// #845/A3：GET /users/me/comments?status=hidden 只返回本人被隐藏评论
// （申诉「近期事件」选择框数据源），并携带关联父对象标题便于辨认。
func TestListMyCommentsIsolatesOtherUsersAndFiltersStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newMyCommentsTestDB(t)

	require.NoError(t, db.Create(&model.ContentItem{
		ID: 100, Title: "被隐藏评论的父内容", AuthorID: 9, Zone: "original",
		ContentType: "article", Status: "published",
	}).Error)
	require.NoError(t, db.Create(&model.Discussion{
		ID: 200, Title: "被隐藏评论的父讨论", AuthorID: 9, Status: "published",
	}).Error)

	for _, cm := range []model.Comment{
		// 本人隐藏评论（内容父）——应返回且带父标题。
		{AuthorID: 1, ContentItemID: ptrID(100), Body: "被隐藏的内容评论", Status: "hidden"},
		// 本人已发布评论——status 过滤应排除。
		{AuthorID: 1, ContentItemID: ptrID(100), Body: "正常评论", Status: "published"},
		// 本人隐藏评论（讨论父）——应返回且带讨论标题。
		{AuthorID: 1, DiscussionID: ptrID(200), Body: "被隐藏的讨论回复", Status: "hidden"},
		// 他人隐藏评论——不得泄露。
		{AuthorID: 2, ContentItemID: ptrID(100), Body: "别人的隐藏评论", Status: "hidden"},
	} {
		require.NoError(t, db.Create(&cm).Error)
	}

	handler := NewSocialHandlerWithService(nil, db)
	router := gin.New()
	router.GET("/users/me/comments", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.ListMyComments(c)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/me/comments?status=hidden&page=1&page_size=20", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload struct {
		Comments []struct {
			ID           int64  `json:"id"`
			Body         string `json:"body"`
			ContentTitle string `json:"content_title"`
			Status       string `json:"status"`
			CreatedAt    string `json:"created_at"`
		} `json:"comments"`
		Total int64 `json:"total"`
		Page  int   `json:"page"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.EqualValues(t, 2, payload.Total)
	require.Len(t, payload.Comments, 2)

	byBody := map[string]string{}
	for _, cm := range payload.Comments {
		require.Equal(t, "hidden", cm.Status, "只应返回 hidden 状态")
		require.NotEmpty(t, cm.CreatedAt, "日期字段必填")
		byBody[cm.Body] = cm.ContentTitle
	}
	require.Contains(t, byBody, "被隐藏的内容评论")
	require.Equal(t, "被隐藏评论的父内容", byBody["被隐藏的内容评论"])
	require.Contains(t, byBody, "被隐藏的讨论回复")
	require.Equal(t, "被隐藏评论的父讨论", byBody["被隐藏的讨论回复"])
	require.NotContains(t, byBody, "别人的隐藏评论", "不得泄露他人评论")
	require.NotContains(t, byBody, "正常评论", "published 状态必须被过滤")
}

// 父对象已软删时标题仍可回填（受众是评论作者本人，id 来自本人评论行）。
func TestListMyCommentsDecoratesSoftDeletedParentTitle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newMyCommentsTestDB(t)

	require.NoError(t, db.Create(&model.ContentItem{
		ID: 300, Title: "已软删的父内容", AuthorID: 9, Zone: "fanwork",
		ContentType: "article", Status: "banned",
	}).Error)
	require.NoError(t, db.Exec("UPDATE content_items SET deleted_at = '2026-01-01 00:00:00' WHERE id = 300").Error)
	require.NoError(t, db.Create(&model.Comment{
		AuthorID: 1, ContentItemID: ptrID(300), Body: "父内容已删", Status: "hidden",
	}).Error)

	handler := NewSocialHandlerWithService(nil, db)
	router := gin.New()
	router.GET("/users/me/comments", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.ListMyComments(c)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/me/comments", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload struct {
		Comments []struct {
			ContentTitle string `json:"content_title"`
		} `json:"comments"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Comments, 1)
	require.Equal(t, "已软删的父内容", payload.Comments[0].ContentTitle)
}

func TestListMyCommentsRequiresLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newMyCommentsTestDB(t)

	handler := NewSocialHandlerWithService(nil, db)
	router := gin.New()
	router.GET("/users/me/comments", handler.ListMyComments)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/me/comments", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

// status 白名单只认 hidden（缺省视同 hidden），其余值 400 快败。
func TestListMyCommentsRejectsNonHiddenStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newMyCommentsTestDB(t)

	handler := NewSocialHandlerWithService(nil, db)
	router := gin.New()
	router.GET("/users/me/comments", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.ListMyComments(c)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/me/comments?status=published", nil))
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Contains(t, body, "code")
	require.Contains(t, body, "message")
}
