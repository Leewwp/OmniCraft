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

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// T28（FIX-35）：admin 检索与运维增强——用户搜索服务端化（跨页命中 + 稳定排序）、
// audit 动作下拉走后端 distinct 端点、LLM Activate 区分 404/500。

func setupAdminOpsRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.AdminAuditLog{}, &model.LLMConfig{},
	))

	cfg := &config.Config{}
	cfg.JWT.Secret = "admin-ops-secret"

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	adminHandler := NewAdminHandler(db, cfg, nil, auditSvc)
	adminAuditHandler := NewAdminAuditHandler(auditSvc)

	router := gin.New()
	admin := router.Group("/api/v1/admin", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		c.Set("trace_id", "trace-admin-ops")
	})
	admin.GET("/users", adminHandler.ListUsers)
	admin.GET("/audit-logs/actions", adminAuditHandler.ListAuditActions)
	admin.POST("/llm-configs/:id/activate", adminHandler.ActivateLLMConfig)
	return router, db
}

func doAdminOpsRequest(t *testing.T, router *gin.Engine, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

// ① search 跨页命中：page_size=1 且按 id DESC 排序时，搜索词仍能定位到
// 首页之外的匹配用户（现状：客户端只过滤当前页 20 条，跨页命中不可能）。
func TestAdminListUsersSearchAcrossPages(t *testing.T) {
	router, db := setupAdminOpsRouter(t)
	for _, u := range []model.User{
		{ID: 101, Email: "alice@example.test", Username: "alice", PasswordHash: "h", Role: "user", Reputation: 10},
		{ID: 102, Email: "bob@example.test", Username: "bob", PasswordHash: "h", Role: "user", Reputation: 10},
		{ID: 103, Email: "carol@example.test", Username: "carol", PasswordHash: "h", Role: "user", Reputation: 10},
	} {
		require.NoError(t, db.Create(&u).Error)
	}

	// 默认无 search：按 id DESC 稳定排序（现状无 ORDER BY，翻页可能重复/漏）。
	w := doAdminOpsRequest(t, router, http.MethodGet, "/api/v1/admin/users?page=1&page_size=2")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var page1 struct {
		Users []map[string]any `json:"users"`
		Total int64            `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page1))
	require.Equal(t, int64(3), page1.Total)
	require.Len(t, page1.Users, 2)
	require.EqualValues(t, 103, page1.Users[0]["id"], "default order must be id DESC")
	require.EqualValues(t, 102, page1.Users[1]["id"])

	// search=bo：只命中 bob（id=102），page_size=1 也能命中——服务端跨页过滤。
	w = doAdminOpsRequest(t, router, http.MethodGet, "/api/v1/admin/users?page=1&page_size=1&search=bo")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var found struct {
		Users []map[string]any `json:"users"`
		Total int64            `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &found))
	require.Equal(t, int64(1), found.Total, "search must filter server-side so total reflects matches")
	require.Len(t, found.Users, 1)
	require.Equal(t, "bob", found.Users[0]["username"])

	// search 大小写不敏感：BO 命中 bob。
	w = doAdminOpsRequest(t, router, http.MethodGet, "/api/v1/admin/users?page=1&page_size=5&search=BO")
	require.Equal(t, http.StatusOK, w.Code)
	var ci struct {
		Total int64 `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ci))
	require.Equal(t, int64(1), ci.Total, "search must be case-insensitive")

	// search 同时匹配 email。
	w = doAdminOpsRequest(t, router, http.MethodGet, "/api/v1/admin/users?page=1&page_size=5&search=alice@example")
	require.Equal(t, http.StatusOK, w.Code)
	var byEmail struct {
		Total int64 `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &byEmail))
	require.Equal(t, int64(1), byEmail.Total, "search must match email too")
}

// ② audit-logs/actions：distinct + 排序（现状前端硬编码 15 项，缺 llm_config_*/dlq_replay 等）。
func TestAdminAuditActionsDistinct(t *testing.T) {
	router, db := setupAdminOpsRouter(t)
	for _, entry := range []model.AdminAuditLog{
		{AdminUserID: 1, Action: "content_ban", TargetType: "content", TargetID: "1", Result: "success"},
		{AdminUserID: 1, Action: "content_ban", TargetType: "content", TargetID: "2", Result: "success"},
		{AdminUserID: 1, Action: "dlq_replay", TargetType: "dlq_entry", TargetID: "1-1", Result: "success"},
		{AdminUserID: 1, Action: "llm_config_activate", TargetType: "llm_config", TargetID: "1", Result: "success"},
		{AdminUserID: 1, Action: "config_patch", TargetType: "config", TargetID: "judge", Result: "success"},
	} {
		require.NoError(t, db.Create(&entry).Error)
	}

	w := doAdminOpsRequest(t, router, http.MethodGet, "/api/v1/admin/audit-logs/actions")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp struct {
		Actions []string `json:"actions"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, []string{"config_patch", "content_ban", "dlq_replay", "llm_config_activate"}, resp.Actions,
		"actions must be distinct and sorted; must include actions missing from the hardcoded list")
}

// ③ Activate 不存在的配置 → 404（现状：任何错误统一 404，DB 故障也报 404 误导排障）。
func TestAdminActivateLLMConfigNotFound(t *testing.T) {
	router, _ := setupAdminOpsRouter(t)

	w := doAdminOpsRequest(t, router, http.MethodPost, "/api/v1/admin/llm-configs/999999/activate")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "CONFIG_NOT_FOUND")
}
