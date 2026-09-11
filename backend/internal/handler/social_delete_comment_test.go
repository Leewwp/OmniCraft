package handler

import (
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
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// T55（FIX-31b/F-093）：删他人评论的错误风格与编辑对齐——403 + 专用码，
// 不再落入 400 "ERROR" 通配（可能透传底层错误文案）。
// comments 表用手写建表（AutoMigrate 与 sqlite 不兼容，先例 social_service_test）。
func newT55DeleteCommentDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
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
	return db
}

func TestDeleteCommentForbiddenStyleAligned(t *testing.T) {
	db := newT55DeleteCommentDB(t)

	author := model.User{Email: "author@example.test", Username: "c-author", Reputation: 10}
	other := model.User{Email: "other@example.test", Username: "c-other", Reputation: 10}
	require.NoError(t, db.Create(&author).Error)
	require.NoError(t, db.Create(&other).Error)
	comment := model.Comment{ContentItemID: ptrID(1), AuthorID: author.ID, Body: "hello"}
	require.NoError(t, db.Create(&comment).Error)

	svc := service.NewSocialServiceWithRedis(
		repository.NewSocialRepository(db),
		repository.NewContentRepository(db),
		repository.NewUserRepository(db),
		&config.Config{Server: config.ServerConfig{Mode: "dev"}},
		nil, nil,
	)
	handler := NewSocialHandlerWithService(svc, db)
	router := gin.New()
	router.DELETE("/comments/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, other.ID)
		handler.DeleteComment(c)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/comments/"+strconv.FormatInt(comment.ID, 10), nil))

	require.Equal(t, http.StatusForbidden, rec.Code, "删他人评论应 403（实际 %d）: %s", rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "FORBIDDEN", "错误码应为 FORBIDDEN 专用码")
}

func TestDeleteCommentNotFoundStyleAligned(t *testing.T) {
	db := newT55DeleteCommentDB(t)

	caller := model.User{Email: "caller@example.test", Username: "c-caller", Reputation: 10}
	require.NoError(t, db.Create(&caller).Error)

	svc := service.NewSocialServiceWithRedis(
		repository.NewSocialRepository(db),
		repository.NewContentRepository(db),
		repository.NewUserRepository(db),
		&config.Config{Server: config.ServerConfig{Mode: "dev"}},
		nil, nil,
	)
	handler := NewSocialHandlerWithService(svc, db)
	router := gin.New()
	router.DELETE("/comments/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, caller.ID)
		handler.DeleteComment(c)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/comments/404", nil))

	require.Equal(t, http.StatusNotFound, rec.Code, "不存在的评论应 404（实际 %d）: %s", rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "NOT_FOUND")
}

func ptrID(v int64) *int64 { return &v }
