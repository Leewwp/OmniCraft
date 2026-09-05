package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// T17（FIX-25 收窄版）+ E2E B-004 回归：admin approve/reject 后必须立即失效
// 公共读路径的详情缓存（cache:ip:{id}）与列表缓存（cache:ip:list:*）。
// 原缺陷：NewAdminHandler 自建的 IPService 不带 redis，
// InvalidateIPCacheForAdmin 静默 no-op，旧详情在 300s TTL 窗口内继续返回。

func setupAdminIPCacheRouter(t *testing.T) (*gin.Engine, *gorm.DB, *miniredis.Miniredis, *service.IPService) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.IP{}, &model.IPTag{}, &model.AdminAuditLog{}, &model.IPReviewLog{}))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	previous := redisclient.Client
	redisclient.Client = rdb
	t.Cleanup(func() { redisclient.Client = previous })

	cfg := &config.Config{Cache: config.CacheConfig{IPListTTL: 300, IPDetailTTL: 300}}
	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	adminHandler := NewAdminHandler(db, cfg, rdb, auditSvc)

	// 公共读路径按 routes.go 的 NewIPHandlerWithCache 装配缓存，用它预热。
	readSvc := service.NewIPServiceWithCache(repository.NewIPRepository(db), rdb, &cfg.Cache)

	router := gin.New()
	register := func(method, path string, h gin.HandlerFunc) {
		router.Handle(method, path, func(c *gin.Context) {
			c.Set(middleware.UserIDKey, int64(1))
			c.Set("trace_id", "trace-t17-cache-test")
			h(c)
		})
	}
	register(http.MethodPost, "/admin/ips/:id/approve", adminHandler.ApproveIP)
	register(http.MethodPost, "/admin/ips/:id/reject", adminHandler.RejectIP)
	return router, db, mr, readSvc
}

func seedT17PendingIP(t *testing.T, db *gorm.DB, id int64) model.IP {
	t.Helper()
	admin := model.User{ID: 1, Email: "t17-admin@example.test", Username: "t17-admin", PasswordHash: "hash", Role: "admin", Reputation: 10}
	require.NoError(t, db.Create(&admin).Error)
	creator := model.User{ID: 2, Email: "t17-creator@example.test", Username: "t17-creator", PasswordHash: "hash", Role: "user", Reputation: 10}
	require.NoError(t, db.Create(&creator).Error)
	ip := model.IP{ID: id, Name: "T17 IP", Slug: "t17-ip", Category: "game", CreatorID: &creator.ID, Status: "pending"}
	require.NoError(t, db.Create(&ip).Error)
	return ip
}

func primeT17IPCache(t *testing.T, readSvc *service.IPService, ipID int64) {
	t.Helper()
	_, err := readSvc.GetIP(ipID)
	require.NoError(t, err)
	_, _, err = readSvc.ListIPs(repository.ListIPsFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
}

func requireNoIPCacheLeft(t *testing.T, mr *miniredis.Miniredis, ipID int64) {
	t.Helper()
	require.False(t, mr.Exists("cache:ip:1"), "admin 状态变更后详情缓存必须立即删除（B-004）")
	for _, key := range mr.Keys() {
		require.NotContains(t, key, "cache:ip:list:", "admin 状态变更后 IP 列表缓存必须立即删除")
	}
}

func TestAdminApproveIPInvalidatesDetailAndListCache(t *testing.T) {
	router, db, mr, readSvc := setupAdminIPCacheRouter(t)
	ip := seedT17PendingIP(t, db, 1)
	primeT17IPCache(t, readSvc, ip.ID)
	require.True(t, mr.Exists("cache:ip:1"), "前置条件：详情缓存已预热")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/ips/1/approve", nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	requireNoIPCacheLeft(t, mr, ip.ID)

	got, err := readSvc.GetIP(ip.ID)
	require.NoError(t, err)
	require.Equal(t, "approved", got.Status, "approve 后下一次公共读必须命中新状态，而非 300s 旧缓存")
}

func TestAdminRejectIPInvalidatesDetailAndListCache(t *testing.T) {
	router, db, mr, readSvc := setupAdminIPCacheRouter(t)
	ip := seedT17PendingIP(t, db, 1)
	primeT17IPCache(t, readSvc, ip.ID)
	require.True(t, mr.Exists("cache:ip:1"), "前置条件：详情缓存已预热")

	w := httptest.NewRecorder()
	// T16 起 reject 契约要求 reason（binding required），与状态更新同事务落
	// ip_review_logs，因此该 setup 也迁移了该表。
	req := httptest.NewRequest(http.MethodPost, "/admin/ips/1/reject", strings.NewReader(`{"reason":"T16 缓存失效回归原因"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	requireNoIPCacheLeft(t, mr, ip.ID)

	got, err := readSvc.GetIP(ip.ID)
	require.NoError(t, err)
	require.Equal(t, "rejected", got.Status, "reject 后下一次公共读必须命中新状态，而非 300s 旧缓存")
}

func TestAdminIPStatusChangeOnMissingIPKeeps400(t *testing.T) {
	router, db, _, _ := setupAdminIPCacheRouter(t)
	seedT17PendingIP(t, db, 1)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/ips/999/approve", nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}
