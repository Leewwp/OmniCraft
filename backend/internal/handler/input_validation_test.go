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

	"omnicraft/backend/config"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
)

// SP-25 FR-11：输入校验与 mass-assignment 收口的契约测试。

func validationDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:fr11val?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Follow{}, &model.IP{}, &model.TagGroup{},
		&model.Category{}, &model.ContentItem{},
	))
	return db
}

// 低-24：self-follow / 不存在目标 / 封禁目标。
func TestFollowGuards(t *testing.T) {
	db := validationDB(t)
	me := &model.User{Email: "me@t.local", Username: "me", PasswordHash: "x", Reputation: 10}
	banned := &model.User{Email: "b@t.local", Username: "banned", PasswordHash: "x", Reputation: 10, IsBanned: true}
	require.NoError(t, db.Create(me).Error)
	require.NoError(t, db.Create(banned).Error)

	h := NewFollowHandler(repository.NewFollowRepository(db))
	r := gin.New()
	r.POST("/users/:id/follow", func(c *gin.Context) { c.Set(middleware.UserIDKey, me.ID); c.Next() }, h.FollowUser)

	do := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	require.Equal(t, http.StatusBadRequest, do("/users/"+itoa64(me.ID)+"/follow").Code, "self-follow must be rejected")
	require.Equal(t, http.StatusNotFound, do("/users/999999/follow").Code, "nonexistent target must 404")
	require.Equal(t, http.StatusBadRequest, do("/users/"+itoa64(banned.ID)+"/follow").Code, "banned target must be rejected")

	var follows int64
	require.NoError(t, db.Model(&model.Follow{}).Count(&follows).Error)
	require.Zero(t, follows, "no follow row may land for guarded targets")
}

// 低-23：TagGroup 更新未知字段 400，白名单字段正常通过。
func TestUpdateTagGroupRejectsUnknownFields(t *testing.T) {
	db := validationDB(t)
	owner := &model.User{Email: "tg@t.local", Username: "tgowner", PasswordHash: "x", Reputation: 10}
	require.NoError(t, db.Create(owner).Error)
	group := &model.TagGroup{UserID: owner.ID, Name: "g", Tags: []string{"a"}}
	require.NoError(t, db.Create(group).Error)

	h := NewTagHandler(service.NewTagService(repository.NewTagRepository(db), repository.NewContentRepository(db), nil, nil), 200)
	r := gin.New()
	r.PATCH("/me/tag-groups/:id", func(c *gin.Context) { c.Set(middleware.UserIDKey, owner.ID); c.Next() }, h.UpdateTagGroup)

	patch := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPatch, "/me/tag-groups/"+itoa64(group.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	require.Equal(t, http.StatusBadRequest, patch(`{"user_id": 42}`).Code,
		"mass-assignment via unknown field must 400 (user_id is protected)")
	require.Equal(t, http.StatusOK, patch(`{"name":"renamed"}`).Code)
	var reloaded model.TagGroup
	require.NoError(t, db.First(&reloaded, group.ID).Error)
	require.Equal(t, owner.ID, reloaded.UserID, "user_id must not be writable through the update endpoint")
	require.Equal(t, "renamed", reloaded.Name)
}

// 低-23（admin 面）：Category 更新未知字段 400。
func TestAdminUpdateCategoryRejectsUnknownFields(t *testing.T) {
	db := validationDB(t)
	cat := &model.Category{Zone: "original", Level: "primary", Slug: "fr11cat", NameI18n: model.JSONMap{"zh": "分类"}, IsActive: true}
	require.NoError(t, db.Create(cat).Error)

	h := NewCategoryHandler(service.NewCategoryService(repository.NewCategoryRepository(db)), nil, db)
	r := gin.New()
	r.PATCH("/admin/categories/:id", h.AdminUpdateCategory)

	patch := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPatch, "/admin/categories/"+itoa64(cat.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	require.Equal(t, http.StatusBadRequest, patch(`{"id": 999}`).Code, "unknown field must 400")
	require.Equal(t, http.StatusOK, patch(`{"sort_order": 7}`).Code)
}

// 低-25：username/bio/support_info/ChangePassword 校验。
func TestUpdateMeValidation(t *testing.T) {
	db := validationDB(t)
	user := &model.User{Email: "v@t.local", Username: "validuser", PasswordHash: "x", Reputation: 10}
	require.NoError(t, db.Create(user).Error)

	cfg := &config.Config{}
	cfg.JWT.Secret = "fr11-test-secret"
	h := NewUserHandler(db, nil, nil, cfg)
	r := gin.New()
	r.PATCH("/users/:id", func(c *gin.Context) { c.Set(middleware.UserIDKey, user.ID); c.Next() }, h.UpdateUser)

	patch := func(body string) int {
		req := httptest.NewRequest(http.MethodPatch, "/users/"+itoa64(user.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	require.Equal(t, http.StatusBadRequest, patch(`{"username":"x"}`), "username too short")
	require.Equal(t, http.StatusBadRequest, patch(`{"username":"`+strings.Repeat("长", 65)+`"}`), "username too long")
	require.Equal(t, http.StatusBadRequest, patch(`{"username":"bad name!"}`), "username charset")
	require.Equal(t, http.StatusBadRequest, patch(`{"bio":"`+strings.Repeat("字", 501)+`"}`), "bio too long")
	require.Equal(t, http.StatusOK, patch(`{"bio":"ok"}`))
}

