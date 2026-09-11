package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// T11（FIX-17a）：admin ban/restore 写点挂接 NotifyContentStatus——
// 封禁通知 body 带 admin 填写的 reason，恢复通知告知重新公开。

type t11AdminCapture struct {
	mu        sync.Mutex
	publishes []map[string]interface{}
}

func (p *t11AdminCapture) Publish(ctx context.Context, topic string, payload []byte) error {
	var decoded map[string]interface{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publishes = append(p.publishes, decoded)
	return nil
}

func (p *t11AdminCapture) contentStatusNotifies() []map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]map[string]interface{}, 0)
	for _, pub := range p.publishes {
		if pub["type"] == "content_status" {
			out = append(out, pub)
		}
	}
	return out
}

func setupT11AdminNotifyRouter(t *testing.T) (*gin.Engine, *gorm.DB, *t11AdminCapture) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.IP{}, &model.ContentItem{}, &model.AdminAuditLog{},
	))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	previous := redisclient.Client
	redisclient.Client = rdb
	t.Cleanup(func() { redisclient.Client = previous })

	cfg := &config.Config{}
	cfg.JWT.Secret = "t11-notify-secret"

	producer := &t11AdminCapture{}
	notifSvc := service.NewNotificationService(repository.NewNotificationRepository(db))
	notifSvc.SetQueueProducer(producer)

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	adminHandler := NewAdminHandler(db, cfg, rdb, auditSvc)
	adminHandler.SetNotificationService(notifSvc)

	router := gin.New()
	register := func(method, path string, h gin.HandlerFunc) {
		router.Handle(method, path, func(c *gin.Context) {
			c.Set(middleware.UserIDKey, int64(1))
			c.Set("trace_id", "trace-t11-notify-test")
			h(c)
		})
	}
	register(http.MethodPost, "/admin/contents/:id/ban", adminHandler.BanContent)
	register(http.MethodPatch, "/admin/contents/:id/restore", adminHandler.RestoreContent)
	return router, db, producer
}

func TestAdminBanContentNotifiesAuthorWithReason(t *testing.T) {
	router, db, producer := setupT11AdminNotifyRouter(t)
	admin := model.User{ID: 1, Email: "t11-admin@example.test", Username: "t11-admin", PasswordHash: "hash", Role: "admin", Reputation: 10}
	require.NoError(t, db.Create(&admin).Error)
	author := model.User{ID: 2, Email: "t11-author@example.test", Username: "t11-author", PasswordHash: "hash", Reputation: 10}
	require.NoError(t, db.Create(&author).Error)
	content := model.ContentItem{ID: 100, Title: "T11 notify content", AuthorID: author.ID, Zone: "original", Category: "game", ContentType: "article", Status: "published", IsPublic: true}
	require.NoError(t, db.Create(&content).Error)

	w := doT11AdminRequest(router, http.MethodPost, "/admin/contents/100/ban", `{"reason":"色情低俗"}`)
	require.Equal(t, http.StatusOK, w.Code)

	notifies := producer.contentStatusNotifies()
	require.Len(t, notifies, 1, "admin 封禁必须通知作者（FIX-17a）")
	require.EqualValues(t, author.ID, notifies[0]["user_id"])
	require.Equal(t, "system", notifies[0]["channel"])
	require.Equal(t, "content_status", notifies[0]["type"])
	require.Contains(t, notifies[0]["body"], "色情低俗", "封禁通知 body 必须带 admin 填写的 reason")
	require.EqualValues(t, admin.ID, notifies[0]["sender_id"], "admin 动作通知携带操作者")
}

func TestAdminRestoreContentNotifiesAuthor(t *testing.T) {
	router, db, producer := setupT11AdminNotifyRouter(t)
	admin := model.User{ID: 1, Email: "t11-admin@example.test", Username: "t11-admin", PasswordHash: "hash", Role: "admin", Reputation: 10}
	require.NoError(t, db.Create(&admin).Error)
	author := model.User{ID: 2, Email: "t11-author@example.test", Username: "t11-author", PasswordHash: "hash", Reputation: 10}
	require.NoError(t, db.Create(&author).Error)
	content := model.ContentItem{ID: 100, Title: "T11 notify content", AuthorID: author.ID, Zone: "original", Category: "game", ContentType: "article", Status: "banned", BanReason: "prior"}
	require.NoError(t, db.Create(&content).Error)

	w := doT11AdminRequest(router, http.MethodPatch, "/admin/contents/100/restore", "")
	require.Equal(t, http.StatusOK, w.Code)

	notifies := producer.contentStatusNotifies()
	require.Len(t, notifies, 1, "admin 恢复必须通知作者（FIX-17a）")
	require.EqualValues(t, author.ID, notifies[0]["user_id"])
	require.Contains(t, notifies[0]["body"], "T11 notify content")
	require.Contains(t, notifies[0]["body"], "公开发布")
}

func doT11AdminRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
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
