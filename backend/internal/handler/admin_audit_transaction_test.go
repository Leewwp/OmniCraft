package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

func TestAdminBanContentRollsBackWhenAuditWriteFails(t *testing.T) {
	router, db := setupAdminAuditRouter(t)
	author := createAdminAuditUser(t, db, "author@example.com", "author")
	content := model.ContentItem{
		Title:       "published content",
		AuthorID:    author.ID,
		Zone:        "original",
		ContentType: "article",
		Status:      "published",
		IsPublic:    true,
		AllowCopy:   true,
	}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("create content: %v", err)
	}
	installFailingAdminAuditTrigger(t, db)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/contents/"+strconv.FormatInt(content.ID, 10)+"/ban", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AUDIT_WRITE_FAILED") {
		t.Fatalf("response missing AUDIT_WRITE_FAILED: %s", rec.Body.String())
	}

	var reloaded model.ContentItem
	if err := db.First(&reloaded, content.ID).Error; err != nil {
		t.Fatalf("reload content: %v", err)
	}
	if reloaded.Status != "published" {
		t.Fatalf("content status = %q, want published", reloaded.Status)
	}
}

func TestAdminPatchFeedbackRollsBackWhenAuditWriteFails(t *testing.T) {
	router, db := setupAdminFeedbackAuditRouter(t)
	ticket := model.FeedbackTicket{
		Category:    "web_bug",
		Title:       "Bug",
		Description: "Bug report",
		Status:      "open",
		Priority:    "normal",
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	installFailingAdminAuditTrigger(t, db)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/admin/feedback/"+strconv.FormatInt(ticket.ID, 10), bytes.NewReader([]byte(`{"status":"closed"}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AUDIT_WRITE_FAILED") {
		t.Fatalf("response missing AUDIT_WRITE_FAILED: %s", rec.Body.String())
	}

	var reloaded model.FeedbackTicket
	if err := db.First(&reloaded, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if reloaded.Status != "open" {
		t.Fatalf("feedback status = %q, want open", reloaded.Status)
	}
}

func TestAdminFeedbackPatchReturnsDeliveryFailedWhenAnonymousEmailFails(t *testing.T) {
	router, db := setupAdminFeedbackDeliveryRouter(t)
	ticket := model.FeedbackTicket{
		ContactEmail: "anon@example.com",
		Category:     "web_bug",
		Title:        "Bug",
		Description:  "Bug report",
		Status:       "open",
		Priority:     "normal",
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/admin/feedback/"+strconv.FormatInt(ticket.ID, 10), bytes.NewReader([]byte(`{"status":"closed"}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "FEEDBACK_DELIVERY_FAILED") {
		t.Fatalf("response missing FEEDBACK_DELIVERY_FAILED: %s", rec.Body.String())
	}
}

func TestAdminCreateCategoryRollsBackWhenAuditWriteFails(t *testing.T) {
	router, db := setupCategoryAuditRouter(t)
	installFailingAdminAuditTrigger(t, db)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/categories", bytes.NewReader([]byte(`{
		"zone":"original",
		"level":"primary",
		"name_i18n":{"zh":"影视"},
		"slug":"film",
		"sort_order":1,
		"is_active":true
	}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AUDIT_WRITE_FAILED") {
		t.Fatalf("response missing AUDIT_WRITE_FAILED: %s", rec.Body.String())
	}

	var count int64
	if err := db.Model(&model.Category{}).Where("slug = ?", "film").Count(&count).Error; err != nil {
		t.Fatalf("count categories: %v", err)
	}
	if count != 0 {
		t.Fatalf("category count = %d, want 0", count)
	}
}

func TestAdminCreateJudgeQuestionRollsBackWhenAuditWriteFails(t *testing.T) {
	router, db := setupJudgeAuditRouter(t)
	installFailingAdminAuditTrigger(t, db)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/judge/questions", bytes.NewReader([]byte(`[
		{"content_type":"article","question_data":"e30=","is_active":true,"created_by":"admin"}
	]`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AUDIT_WRITE_FAILED") {
		t.Fatalf("response missing AUDIT_WRITE_FAILED: %s", rec.Body.String())
	}

	var count int64
	if err := db.Model(&model.JudgeQuestion{}).Count(&count).Error; err != nil {
		t.Fatalf("count judge questions: %v", err)
	}
	if count != 0 {
		t.Fatalf("judge question count = %d, want 0", count)
	}
}

func TestAdminCreateLLMConfigRollsBackWhenAuditWriteFails(t *testing.T) {
	router, db := setupAdminLLMAuditRouter(t)
	installFailingAdminAuditTrigger(t, db)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/llm-configs", bytes.NewReader([]byte(`{
		"config_name":"test-model",
		"provider_type":"openai",
		"api_base":"https://example.test",
		"model":"gpt-test",
		"api_key":"secret"
	}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AUDIT_WRITE_FAILED") {
		t.Fatalf("response missing AUDIT_WRITE_FAILED: %s", rec.Body.String())
	}

	var count int64
	if err := db.Model(&model.LLMConfig{}).Where("config_name = ?", "test-model").Count(&count).Error; err != nil {
		t.Fatalf("count llm configs: %v", err)
	}
	if count != 0 {
		t.Fatalf("llm config count = %d, want 0", count)
	}
}

func TestAdminResolveReportInvalidStatusRecordsFailedAudit(t *testing.T) {
	router, db := setupAdminReportAuditRouter(t)
	report := model.Report{
		ReporterID: 1,
		TargetType: "content",
		TargetID:   42,
		Reason:     "spam",
		Status:     "pending",
	}
	if err := db.Create(&report).Error; err != nil {
		t.Fatalf("create report: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/reports/"+strconv.FormatInt(report.ID, 10)+"/resolve", bytes.NewReader([]byte(`{"status":"invalid","action_taken":"secret raw db text"}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}

	var logs []model.AdminAuditLog
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("audit log count = %d, want 1", len(logs))
	}
	log := logs[0]
	if log.Result != "failed" {
		t.Fatalf("audit result = %q, want failed", log.Result)
	}
	if log.Action != "report_resolve" {
		t.Fatalf("audit action = %q, want report_resolve", log.Action)
	}
	if log.TargetID != strconv.FormatInt(report.ID, 10) {
		t.Fatalf("audit target id = %q, want report id", log.TargetID)
	}
	if log.TraceID != "trace-admin-audit-test" {
		t.Fatalf("trace id = %q, want trace-admin-audit-test", log.TraceID)
	}
	metadata := log.Metadata
	if metadata["reason"] != "VALIDATION_ERROR" {
		t.Fatalf("metadata reason = %v, want VALIDATION_ERROR", metadata["reason"])
	}
	if _, ok := metadata["action_taken"]; ok {
		t.Fatalf("metadata must not contain raw action_taken: %v", metadata)
	}
}

func setupAdminAuditRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.AdminAuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	handler := NewAdminHandler(db, &config.Config{}, nil, auditSvc)
	router := gin.New()
	router.POST("/admin/contents/:id/ban", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(99))
		c.Set("trace_id", "trace-admin-audit-test")
		handler.BanContent(c)
	})
	return router, db
}

func setupAdminReportAuditRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Report{}, &model.AdminAuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	handler := NewAdminHandler(db, &config.Config{}, nil, auditSvc)
	router := gin.New()
	router.POST("/admin/reports/:id/resolve", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(99))
		c.Set("trace_id", "trace-admin-audit-test")
		handler.ResolveReport(c)
	})
	return router, db
}

func setupAdminLLMAuditRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.LLMConfig{}, &model.AdminAuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	handler := NewAdminHandler(db, &config.Config{}, nil, auditSvc)
	router := gin.New()
	router.POST("/admin/llm-configs", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(99))
		c.Set("trace_id", "trace-admin-audit-test")
		handler.CreateLLMConfig(c)
	})
	return router, db
}

func setupAdminFeedbackAuditRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.FeedbackTicket{}, &model.FeedbackReply{}, &model.FeedbackAttachment{}, &model.AdminAuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	feedbackSvc := service.NewFeedbackService(repository.NewFeedbackRepository(db), repository.NewUserRepository(db), nil, nil, 300)
	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	handler := NewAdminFeedbackHandler(db, feedbackSvc, auditSvc)
	router := gin.New()
	router.PATCH("/admin/feedback/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(99))
		c.Set("trace_id", "trace-admin-audit-test")
		handler.PatchFeedback(c)
	})
	return router, db
}

func setupAdminFeedbackDeliveryRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	router, db := setupAdminFeedbackAuditRouter(t)
	feedbackSvc := service.NewFeedbackService(repository.NewFeedbackRepository(db), repository.NewUserRepository(db), nil, nil, 300)
	feedbackSvc.SetFeedbackMailSender(failingFeedbackMailSender{})
	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	handler := NewAdminFeedbackHandler(db, feedbackSvc, auditSvc)
	router = gin.New()
	router.PATCH("/admin/feedback/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(99))
		c.Set("trace_id", "trace-admin-feedback-delivery-test")
		handler.PatchFeedback(c)
	})
	return router, db
}

type failingFeedbackMailSender struct{}

func (failingFeedbackMailSender) SendFeedbackUpdate(context.Context, string, string, string) error {
	return errors.New("mail failed")
}

func setupCategoryAuditRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Category{}, &model.AdminAuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	handler := NewCategoryHandler(db, auditSvc)
	router := gin.New()
	router.POST("/admin/categories", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(99))
		c.Set("trace_id", "trace-admin-audit-test")
		handler.AdminCreateCategory(c)
	})
	return router, db
}

func setupJudgeAuditRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.JudgeQuestion{}, &model.AdminAuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	handler := NewJudgeHandler(db, &config.Config{}, auditSvc)
	router := gin.New()
	router.POST("/admin/judge/questions", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(99))
		c.Set("trace_id", "trace-admin-audit-test")
		handler.CreateQuestions(c)
	})
	return router, db
}

func installFailingAdminAuditTrigger(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`
		CREATE TRIGGER fail_admin_audit_insert
		BEFORE INSERT ON admin_audit_logs
		BEGIN
			SELECT RAISE(FAIL, 'audit blocked by test');
		END;
	`).Error; err != nil {
		t.Fatalf("create audit failure trigger: %v", err)
	}
}

func createAdminAuditUser(t *testing.T, db *gorm.DB, email, username string) model.User {
	t.Helper()
	user := model.User{
		Email:        email,
		Username:     username,
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return user
}
