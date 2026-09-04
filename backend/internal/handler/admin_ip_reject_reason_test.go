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

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// T16（FIX-24）：admin 驳回 IP 时填写的 reason 必须落库 ip_review_logs
// （actor/reason/created_at）并通知创建者（type=ip_status、body 带原因）。
// 原缺陷：RejectIP 不解析 body，_reason 在前端被丢弃，ip_review_logs 恒空，
// 创建者只见 rejected 不知为何。

func setupT16RejectRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// 内存 sqlite 的每个连接都是独立库；Notify 落库走后台 goroutine，
	// 限单连接保证建表/写入/断言共享同一份 schema。
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.IP{}, &model.IPTag{}, &model.AdminAuditLog{}, &model.AIReviewRecord{}, &model.IPReviewLog{}, &model.Notification{}))

	cfg := &config.Config{}
	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	adminHandler := NewAdminHandler(db, cfg, nil, auditSvc)
	adminHandler.SetNotificationService(service.NewNotificationService(repository.NewNotificationRepository(db)))

	router := gin.New()
	register := func(method, path string, h gin.HandlerFunc) {
		router.Handle(method, path, func(c *gin.Context) {
			c.Set(middleware.UserIDKey, int64(1))
			c.Set("trace_id", "trace-t16-reject-test")
			h(c)
		})
	}
	register(http.MethodPost, "/admin/ips/:id/reject", adminHandler.RejectIP)
	register(http.MethodPost, "/admin/ips/:id/approve", adminHandler.ApproveIP)
	return router, db
}

func seedT16IPWithCreator(t *testing.T, db *gorm.DB) model.IP {
	t.Helper()
	admin := model.User{ID: 1, Email: "t16-admin@example.test", Username: "t16-admin", PasswordHash: "hash", Role: "admin", Reputation: 10}
	require.NoError(t, db.Create(&admin).Error)
	creator := model.User{ID: 2, Email: "t16-creator@example.test", Username: "t16-creator", PasswordHash: "hash", Role: "user", Reputation: 10}
	require.NoError(t, db.Create(&creator).Error)
	ip := model.IP{ID: 10, Name: "T16 IP", Slug: "t16-ip", Category: "game", CreatorID: &creator.ID, Status: "pending"}
	require.NoError(t, db.Create(&ip).Error)
	return ip
}

func TestRejectIPPersistsReasonAndNotifiesCreator(t *testing.T) {
	router, db := setupT16RejectRouter(t)
	ip := seedT16IPWithCreator(t, db)

	req := httptest.NewRequest(http.MethodPost, "/admin/ips/10/reject", strings.NewReader(`{"reason":"介绍含站外引流信息"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// ip_review_logs 恰好一行：action=reject、reason 落库、actor=admin
	var logs []model.IPReviewLog
	require.NoError(t, db.Where("ip_id = ?", ip.ID).Find(&logs).Error)
	require.Len(t, logs, 1, "reject 带 reason 必须写一行 ip_review_logs")
	require.Equal(t, "reject", logs[0].Action)
	require.Equal(t, "介绍含站外引流信息", logs[0].Reason)
	require.NotNil(t, logs[0].ReviewerID)
	require.EqualValues(t, 1, *logs[0].ReviewerID)
	require.False(t, logs[0].CreatedAt.IsZero(), "created_at 由 autoCreateTime 落库")

	// 创建者收到 IP 决策通知，body 携带原因（Notify 落库走异步 goroutine，
	// 断言等待副作用生效而非固定 sleep 后单查）
	var notifs []model.Notification
	require.Eventually(t, func() bool {
		require.NoError(t, db.Where("user_id = ? AND type = ?", 2, "ip_status").Find(&notifs).Error)
		return len(notifs) == 1
	}, 2*time.Second, 20*time.Millisecond, "reject 必须给创建者发一条 ip_status 通知")
	require.Equal(t, "system", notifs[0].Channel)
	require.Equal(t, "ip", *notifs[0].TargetType)
	require.EqualValues(t, ip.ID, *notifs[0].TargetID)
	require.Contains(t, *notifs[0].Body, "介绍含站外引流信息")
	require.Contains(t, *notifs[0].Body, ip.Name)

	// IP 状态更新为 rejected
	var status string
	require.NoError(t, db.Raw("SELECT status FROM ips WHERE id = ?", ip.ID).Scan(&status).Error)
	require.Equal(t, "rejected", status)
}

func TestRejectIPWithoutReasonIsRejectedWith400(t *testing.T) {
	router, db := setupT16RejectRouter(t)
	seedT16IPWithCreator(t, db)

	// reason binding required：缺失 body / 空 reason 均 400
	for _, payload := range []string{`{}`, `{"reason":"  "}`} {
		req := httptest.NewRequest(http.MethodPost, "/admin/ips/10/reject", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, "payload %s must be rejected", payload)
	}

	var count int64
	require.NoError(t, db.Model(&model.IPReviewLog{}).Count(&count).Error)
	require.Zero(t, count, "无效请求不得写 ip_review_logs")
	var notifCount int64
	require.NoError(t, db.Model(&model.Notification{}).Count(&notifCount).Error)
	require.Zero(t, notifCount, "无效请求不得发通知")
}

func TestApproveIPNotifiesCreator(t *testing.T) {
	router, db := setupT16RejectRouter(t)
	ip := seedT16IPWithCreator(t, db)

	req := httptest.NewRequest(http.MethodPost, "/admin/ips/10/approve", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var notifs []model.Notification
	require.Eventually(t, func() bool {
		require.NoError(t, db.Where("user_id = ? AND type = ?", 2, "ip_status").Find(&notifs).Error)
		return len(notifs) == 1
	}, 2*time.Second, 20*time.Millisecond, "approve 必须给创建者发一条 ip_status 通知（T52 状态可知性依赖）")
	require.Contains(t, *notifs[0].Body, ip.Name)

	var status string
	require.NoError(t, db.Raw("SELECT status FROM ips WHERE id = ?", ip.ID).Scan(&status).Error)
	require.Equal(t, "approved", status)
}
