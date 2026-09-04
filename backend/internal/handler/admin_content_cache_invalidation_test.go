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
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// T10（FIX-38）：审核写点（admin ban/restore、申诉批准）绕过 ContentService
// 直接 Updates，封禁内容在 300s TTL 窗口内仍以旧缓存返回 200。这些写点必须
// 立即失效公共读路径的详情缓存 cache:content:{id}；restore 另需在审计事务内
// 补发 TopicContentPublished 喂索引（FIX-34）。

func setupT10ContentCacheRouters(t *testing.T) (*gin.Engine, *gin.Engine, *gorm.DB, *miniredis.Miniredis) {
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
		&model.AdminAuditLog{}, &model.Appeal{}, &model.OutboxEvent{},
	))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	previous := redisclient.Client
	redisclient.Client = rdb
	t.Cleanup(func() { redisclient.Client = previous })

	cfg := &config.Config{}
	cfg.JWT.Secret = "t10-cache-secret"
	cfg.Cache.ContentDetailTTL = 300
	cfg.Cache.ContentListTTL = 300

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	adminHandler := NewAdminHandler(db, cfg, rdb, auditSvc)
	adminHandler.SetContentOutbox(repository.NewOutboxRepository(db))

	// 公共读路径按 routes.go 同款装配（NewContentHandler 携带 redis 与
	// cache 配置，详情缓存行为与线上一致）。
	readRouter := gin.New()
	contentHandler := NewContentHandler(db, cfg, rdb)
	optAuth := middleware.OptionalAuth(cfg, nil, db)
	readRouter.GET("/api/v1/contents/:id", optAuth, contentHandler.GetContent)

	adminRouter := gin.New()
	register := func(method, path string, h gin.HandlerFunc) {
		adminRouter.Handle(method, path, func(c *gin.Context) {
			c.Set(middleware.UserIDKey, int64(1))
			c.Set("trace_id", "trace-t10-cache-test")
			h(c)
		})
	}
	register(http.MethodPost, "/admin/contents/:id/ban", adminHandler.BanContent)
	register(http.MethodPatch, "/admin/contents/:id/restore", adminHandler.RestoreContent)
	register(http.MethodPost, "/admin/appeals/:id", adminHandler.ResolveAppeal)

	return readRouter, adminRouter, db, mr
}

func seedT10User(t *testing.T, db *gorm.DB, id int64, username, role string) model.User {
	t.Helper()
	user := model.User{
		ID:           id,
		Email:        username + "@example.test",
		Username:     username,
		PasswordHash: "hash",
		Role:         role,
		Reputation:   10,
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func seedT10Content(t *testing.T, db *gorm.DB, id, authorID int64, status string) model.ContentItem {
	t.Helper()
	content := model.ContentItem{
		ID:          id,
		Title:       "T10 cache content",
		AuthorID:    authorID,
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		Status:      status,
		IsPublic:    true,
		AllowCopy:   true,
		ViewCount:   100,
	}
	require.NoError(t, db.Create(&content).Error)
	return content
}

func primeT10DetailCache(t *testing.T, readRouter *gin.Engine, mr *miniredis.Miniredis, id int64) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contents/"+itoa64(id), nil)
	readRouter.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, mr.Exists("cache:content:"+itoa64(id)), "前置条件：详情缓存已预热")
}

func doT10Request(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
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

func TestAdminBanContentInvalidatesDetailCacheImmediately(t *testing.T) {
	readRouter, adminRouter, db, mr := setupT10ContentCacheRouters(t)
	seedT10User(t, db, 1, "t10-admin", "admin")
	author := seedT10User(t, db, 2, "t10-author", "user")
	seedT10Content(t, db, 100, author.ID, "published")
	primeT10DetailCache(t, readRouter, mr, 100)

	w := doT10Request(adminRouter, http.MethodPost, "/admin/contents/100/ban", `{"reason":"spam"}`)
	require.Equal(t, http.StatusOK, w.Code)

	require.False(t, mr.Exists("cache:content:100"), "admin ban 后详情缓存必须立即删除（FIX-38），否则封禁内容在 TTL 窗口内仍可读")

	w = doT10Request(readRouter, http.MethodGet, "/api/v1/contents/100", "")
	require.Equal(t, http.StatusNotFound, w.Code, "缓存失效后匿名读必须立即看到 banned 口径（404），而非 300s 旧缓存")
}

func TestAdminRestoreContentInvalidatesCacheAndEmitsPublishedEvent(t *testing.T) {
	readRouter, adminRouter, db, mr := setupT10ContentCacheRouters(t)
	seedT10User(t, db, 1, "t10-admin", "admin")
	author := seedT10User(t, db, 2, "t10-author", "user")
	seedT10Content(t, db, 100, author.ID, "published")
	primeT10DetailCache(t, readRouter, mr, 100)
	// 模拟此前封禁写点未失效缓存：DB 已 banned 而缓存仍是旧 published 行。
	require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ?", 100).
		Updates(map[string]interface{}{"status": "banned", "ban_reason": "prior"}).Error)

	w := doT10Request(adminRouter, http.MethodPatch, "/admin/contents/100/restore", "")
	require.Equal(t, http.StatusOK, w.Code)

	require.False(t, mr.Exists("cache:content:100"), "admin restore 后残留的陈旧缓存键必须立即删除（FIX-38）")

	var content model.ContentItem
	require.NoError(t, db.First(&content, 100).Error)
	require.Equal(t, "published", content.Status)

	var publishedEvents int64
	require.NoError(t, db.Model(&model.OutboxEvent{}).
		Where("event_type = ?", "content.published").Count(&publishedEvents).Error)
	require.Equal(t, int64(1), publishedEvents, "restore 必须在审计事务内补发 TopicContentPublished 喂索引（FIX-34）")

	w = doT10Request(readRouter, http.MethodGet, "/api/v1/contents/100", "")
	require.Equal(t, http.StatusOK, w.Code, "restore 后匿名读必须立即恢复可见")
}

func TestAdminRestoreAlreadyPublishedContentEmitsNoEvent(t *testing.T) {
	_, adminRouter, db, _ := setupT10ContentCacheRouters(t)
	seedT10User(t, db, 1, "t10-admin", "admin")
	author := seedT10User(t, db, 2, "t10-author", "user")
	seedT10Content(t, db, 100, author.ID, "published")

	w := doT10Request(adminRouter, http.MethodPatch, "/admin/contents/100/restore", "")
	require.Equal(t, http.StatusOK, w.Code)

	var publishedEvents int64
	require.NoError(t, db.Model(&model.OutboxEvent{}).
		Where("event_type = ?", "content.published").Count(&publishedEvents).Error)
	require.Equal(t, int64(0), publishedEvents, "无状态转换的重复 restore 不应重复发索引事件（幂等）")
}

func TestResolveAppealContentApprovalInvalidatesCache(t *testing.T) {
	readRouter, adminRouter, db, mr := setupT10ContentCacheRouters(t)
	seedT10User(t, db, 1, "t10-admin", "admin")
	author := seedT10User(t, db, 2, "t10-author", "user")
	seedT10Content(t, db, 100, author.ID, "published")
	primeT10DetailCache(t, readRouter, mr, 100)
	// 复现陈旧窗口：内容此前被封禁但缓存未失效。
	require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ?", 100).
		Update("status", "banned").Error)
	appeal := model.Appeal{UserID: author.ID, TargetType: "content", TargetID: 100, Reason: "unjust", Status: "pending"}
	require.NoError(t, db.Create(&appeal).Error)

	w := doT10Request(adminRouter, http.MethodPost, "/admin/appeals/1", `{"status":"approved","admin_response":"restored after review"}`)
	require.Equal(t, http.StatusOK, w.Code)

	require.False(t, mr.Exists("cache:content:100"), "申诉批准（content 目标）后必须立即失效详情缓存（FIX-38）")

	var content model.ContentItem
	require.NoError(t, db.First(&content, 100).Error)
	require.Equal(t, "published", content.Status)

	w = doT10Request(readRouter, http.MethodGet, "/api/v1/contents/100", "")
	require.Equal(t, http.StatusOK, w.Code)
}
