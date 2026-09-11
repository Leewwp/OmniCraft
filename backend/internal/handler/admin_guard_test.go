package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
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
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// T27（FIX-34）：admin 写操作必须可信——ban/restore/unban/resolve report 对
// 不存在的 ID 返回 404 而不是 200 假成功（污染审计日志）；BanUser 拒绝 admin
// 目标与自己；restore 必须清 deleted_at（否则恢复的内容仍留在回收站且前台
// 不可见）。

func setupAdminGuardRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.ContentItem{}, &model.Report{}, &model.AdminAuditLog{}, &model.OutboxEvent{},
	))

	cfg := &config.Config{}
	cfg.JWT.Secret = "admin-guard-secret"

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	adminHandler := NewAdminHandler(db, cfg, nil, auditSvc)
	adminHandler.SetContentOutbox(repository.NewOutboxRepository(db))

	router := gin.New()
	register := func(method, path string, h gin.HandlerFunc) {
		router.Handle(method, path, func(c *gin.Context) {
			c.Set(middleware.UserIDKey, int64(1))
			c.Set("trace_id", "trace-admin-guard")
			h(c)
		})
	}
	register(http.MethodPost, "/admin/contents/:id/ban", adminHandler.BanContent)
	register(http.MethodPatch, "/admin/contents/:id/restore", adminHandler.RestoreContent)
	register(http.MethodPost, "/admin/users/:id/ban", adminHandler.BanUser)
	register(http.MethodPost, "/admin/users/:id/unban", adminHandler.UnbanUser)
	register(http.MethodPatch, "/admin/reports/:id", adminHandler.ResolveReport)
	return router, db
}

func doAdminGuardRequest(t *testing.T, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(w, req)
	return w
}

func seedAdminGuardUser(t *testing.T, db *gorm.DB, id int64, role string) model.User {
	t.Helper()
	user := model.User{
		ID:           id,
		Email:        "guard-user-" + strings.ToLower(role) + "-" + itoa64(id) + "@example.test",
		Username:     "guard-" + role + "-" + itoa64(id),
		PasswordHash: "hash",
		Role:         role,
		Reputation:   10,
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func seedAdminGuardReport(t *testing.T, db *gorm.DB, id, reporterID, targetID int64) model.Report {
	t.Helper()
	report := model.Report{
		ID:         id,
		ReporterID: reporterID,
		TargetType: "content",
		TargetID:   targetID,
		Reason:     "spam",
		Status:     "pending",
	}
	require.NoError(t, db.Create(&report).Error)
	return report
}

func countAdminGuardSuccessAudits(t *testing.T, db *gorm.DB, action string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&model.AdminAuditLog{}).Where("action = ? AND result = ?", action, "success").Count(&n).Error)
	return n
}

// ① BanContent：不存在的 ID 必须 404，且不写成功审计行（防审计污染）。
func TestAdminBanContentMissingIDReturns404(t *testing.T) {
	router, db := setupAdminGuardRouter(t)

	w := doAdminGuardRequest(t, router, http.MethodPost, "/admin/contents/999999/ban", `{}`)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "CONTENT_NOT_FOUND")
	require.Equal(t, int64(0), countAdminGuardSuccessAudits(t, db, "content_ban"), "404 must not write a success audit row")
}

// ① RestoreContent：不存在的 ID 必须 404（现状 return nil → 200 假成功）。
func TestAdminRestoreContentMissingIDReturns404(t *testing.T) {
	router, db := setupAdminGuardRouter(t)

	w := doAdminGuardRequest(t, router, http.MethodPatch, "/admin/contents/999999/restore", "")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "CONTENT_NOT_FOUND")
	require.Equal(t, int64(0), countAdminGuardSuccessAudits(t, db, "content_restore"), "404 must not write a success audit row")
}

// ③ RestoreContent：恢复回收站内容必须清 deleted_at（同事务），否则内容
// status=published 却仍留在回收站且前台查询（deleted_at IS NULL）不可见。
func TestAdminRestoreContentClearsDeletedAt(t *testing.T) {
	router, db := setupAdminGuardRouter(t)

	author := seedAdminGuardUser(t, db, 20, "user")
	deletedAt := time.Now().Add(-24 * time.Hour)
	content := model.ContentItem{
		ID:          300,
		Title:       "trashed content",
		AuthorID:    author.ID,
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		Status:      "banned",
		IsPublic:    true,
		DeletedAt:   &deletedAt,
	}
	require.NoError(t, db.Create(&content).Error)

	w := doAdminGuardRequest(t, router, http.MethodPatch, "/admin/contents/300/restore", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var reloaded model.ContentItem
	require.NoError(t, db.First(&reloaded, 300).Error)
	require.Nil(t, reloaded.DeletedAt, "restore must clear deleted_at (TDD: deleted_at IS NULL)")
	require.Equal(t, "published", reloaded.Status)

	// 回收站视图不再包含该内容。
	var trashed int64
	require.NoError(t, db.Model(&model.ContentItem{}).Where("deleted_at IS NOT NULL").Count(&trashed).Error)
	require.Equal(t, int64(0), trashed, "restored content must leave the trash view")
}

// ② BanUser：拒绝 admin 目标（API 层守卫，前端按钮禁用不足够）。
func TestAdminBanUserRejectsAdminTarget(t *testing.T) {
	router, db := setupAdminGuardRouter(t)
	seedAdminGuardUser(t, db, 30, "admin")

	w := doAdminGuardRequest(t, router, http.MethodPost, "/admin/users/30/ban", `{"reason":"test"}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "ADMIN_TARGET_FORBIDDEN")

	var target model.User
	require.NoError(t, db.First(&target, 30).Error)
	require.False(t, target.IsBanned, "admin target must stay unbanned")
}

// ② BanUser：拒绝封禁自己。
func TestAdminBanUserRejectsSelf(t *testing.T) {
	router, db := setupAdminGuardRouter(t)
	seedAdminGuardUser(t, db, 1, "admin") // caller id == 1 per router setup

	w := doAdminGuardRequest(t, router, http.MethodPost, "/admin/users/1/ban", `{"reason":"self"}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "SELF_BAN_FORBIDDEN")

	var target model.User
	require.NoError(t, db.First(&target, 1).Error)
	require.False(t, target.IsBanned, "self ban must be rejected")
}

// ① BanUser：不存在的 ID 必须 404。
func TestAdminBanUserMissingIDReturns404(t *testing.T) {
	router, db := setupAdminGuardRouter(t)

	w := doAdminGuardRequest(t, router, http.MethodPost, "/admin/users/999999/ban", `{"reason":"x"}`)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "USER_NOT_FOUND")
	require.Equal(t, int64(0), countAdminGuardSuccessAudits(t, db, "user_ban"), "404 must not write a success audit row")
}

// ① UnbanUser：不存在的 ID 必须 404。
func TestAdminUnbanUserMissingIDReturns404(t *testing.T) {
	router, _ := setupAdminGuardRouter(t)

	w := doAdminGuardRequest(t, router, http.MethodPost, "/admin/users/999999/unban", "")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "USER_NOT_FOUND")
}

// F-114：ResolveReport 同样缺存在性校验——不存在 ID + 合法 body 之前是
// 200 假成功 + 成功审计行。
func TestAdminResolveReportMissingIDReturns404(t *testing.T) {
	router, db := setupAdminGuardRouter(t)
	seedAdminGuardReport(t, db, 400, 2, 300) // 存在一条，但请求打的是 999999

	w := doAdminGuardRequest(t, router, http.MethodPatch, "/admin/reports/999999", `{"status":"resolved","action_taken":"removed"}`)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "REPORT_NOT_FOUND")
	require.Equal(t, int64(0), countAdminGuardSuccessAudits(t, db, "report_resolve"), "404 must not write a success audit row")

	// 存在的 report 不受影响。
	var report model.Report
	require.NoError(t, db.First(&report, 400).Error)
	require.Equal(t, "pending", report.Status)
}
