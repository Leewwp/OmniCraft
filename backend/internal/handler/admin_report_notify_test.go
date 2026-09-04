package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
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

// T53（FIX-28a）：admin ResolveReport 成功后举报者收到通知（body=处置说明）；
// 不存在的举报（0 行更新）不产生通知。

type t53CaptureProducer struct {
	mu        sync.Mutex
	publishes []map[string]interface{}
}

func (p *t53CaptureProducer) Publish(ctx context.Context, topic string, payload []byte) error {
	var decoded map[string]interface{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publishes = append(p.publishes, decoded)
	return nil
}

func (p *t53CaptureProducer) reportResultNotifies() []map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]map[string]interface{}, 0)
	for _, pub := range p.publishes {
		if pub["type"] == "report_result" {
			out = append(out, pub)
		}
	}
	return out
}

func newT53ResolveRouter(db *gorm.DB, notifSvc *service.NotificationService) *gin.Engine {
	handler := NewAdminHandler(db, &config.Config{}, nil, service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db))
	handler.SetNotificationService(notifSvc)
	router := gin.New()
	router.PATCH("/admin/reports/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(99))
		c.Set("trace_id", "trace-t53")
		handler.ResolveReport(c)
	})
	return router
}

func TestAdminResolveReportNotifiesReporter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Report{}, &model.AdminAuditLog{}))

	report := model.Report{ReporterID: 5, TargetType: "content", TargetID: 42, Reason: "spam", Status: "pending"}
	require.NoError(t, db.Create(&report).Error)

	producer := &t53CaptureProducer{}
	notifSvc := service.NewNotificationService(repository.NewNotificationRepository(db))
	notifSvc.SetQueueProducer(producer)
	router := newT53ResolveRouter(db, notifSvc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/admin/reports/"+strconv.FormatInt(report.ID, 10), bytes.NewReader([]byte(`{"status":"resolved","action_taken":"已下架该内容"}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	notifies := producer.reportResultNotifies()
	require.Len(t, notifies, 1, "resolve 成功后举报者收到一条通知")
	require.Equal(t, float64(5), notifies[0]["user_id"])
	require.Contains(t, notifies[0]["body"], "已下架该内容")
	require.Equal(t, "report", notifies[0]["target_type"])
}

func TestAdminResolveReportMissingRowNoNotification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Report{}, &model.AdminAuditLog{}))

	producer := &t53CaptureProducer{}
	notifSvc := service.NewNotificationService(repository.NewNotificationRepository(db))
	notifSvc.SetQueueProducer(producer)
	router := newT53ResolveRouter(db, notifSvc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/admin/reports/404", bytes.NewReader([]byte(`{"status":"dismissed","action_taken":"不存在"}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Empty(t, producer.reportResultNotifies(), "不存在的举报不应产生通知")
}
