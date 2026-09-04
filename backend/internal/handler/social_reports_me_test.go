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
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// T53（FIX-28a）：GET /social/reports/me 只返回本人举报，隔离他人数据。
func TestListMyReportsIsolatesOtherUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Report{}))

	for _, r := range []model.Report{
		{ReporterID: 1, TargetType: "content", TargetID: 10, Reason: "spam", Status: "resolved", ActionTaken: "已下架"},
		{ReporterID: 1, TargetType: "comment", TargetID: 20, Reason: "abuse", Status: "pending"},
		{ReporterID: 2, TargetType: "content", TargetID: 30, Reason: "other", Status: "pending"},
	} {
		require.NoError(t, db.Create(&r).Error)
	}

	handler := NewSocialHandlerWithService(nil, db)
	router := gin.New()
	router.GET("/social/reports/me", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.ListMyReports(c)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/social/reports/me?page=1&page_size=20", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload struct {
		Reports  []model.Report `json:"reports"`
		Total    int64          `json:"total"`
		Page     int            `json:"page"`
		PageSize int            `json:"page_size"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.EqualValues(t, 2, payload.Total)
	require.Len(t, payload.Reports, 2)
	for _, r := range payload.Reports {
		require.EqualValues(t, 1, r.ReporterID, "不得泄露他人举报")
	}
	// 处置说明随行返回（resolved 项带 action_taken）
	found := false
	for _, r := range payload.Reports {
		if r.Status == "resolved" {
			require.Equal(t, "已下架", r.ActionTaken)
			found = true
		}
	}
	require.True(t, found, "resolved 举报应带处置说明")
}

func TestListMyReportsRequiresLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Report{}))

	handler := NewSocialHandlerWithService(nil, db)
	router := gin.New()
	router.GET("/social/reports/me", handler.ListMyReports)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/social/reports/me", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

var _ = repository.NewNotificationRepository
var _ = service.NewNotificationService
