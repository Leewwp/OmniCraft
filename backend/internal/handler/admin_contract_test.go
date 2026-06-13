package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

func TestAdminCreateCategoryRouteRequiresAdminRole(t *testing.T) {
	router, _, _, userToken := setupAdminContractRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/categories", bytes.NewBufferString(`{
		"zone":"fanwork",
		"level":"category",
		"name_i18n":{"en":"Games"},
		"slug":"games",
		"sort_order":1,
		"is_active":true
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("FORBIDDEN")) {
		t.Fatalf("body = %s, want FORBIDDEN", rec.Body.String())
	}
}

func TestAdminCreateCategoryRoutePersistsCategoryAndAuditRow(t *testing.T) {
	router, db, adminToken, _ := setupAdminContractRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/categories", bytes.NewBufferString(`{
		"zone":"fanwork",
		"level":"category",
		"name_i18n":{"en":"Games","zh":"游戏"},
		"slug":"games",
		"sort_order":1,
		"is_active":true
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	var created struct {
		Category model.Category `json:"category"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
	}
	if created.Category.Slug != "games" || created.Category.Zone != "fanwork" {
		t.Fatalf("category = %#v, want persisted fanwork/games", created.Category)
	}

	var auditLogs []model.AdminAuditLog
	if err := db.Find(&auditLogs).Error; err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(auditLogs) != 1 {
		t.Fatalf("audit log count = %d, want 1", len(auditLogs))
	}
	if auditLogs[0].Action != "category_create" || auditLogs[0].TargetID == "" {
		t.Fatalf("audit log = %#v, want category_create with target id", auditLogs[0])
	}
}

func TestAdminCreateCategoryRouteRejectsInvalidZone(t *testing.T) {
	router, _, adminToken, _ := setupAdminContractRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/categories", bytes.NewBufferString(`{
		"zone":"invalid-zone",
		"level":"category",
		"name_i18n":{"en":"Games"},
		"slug":"games",
		"sort_order":1,
		"is_active":true
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("VALIDATION_ERROR")) {
		t.Fatalf("body = %s, want VALIDATION_ERROR", rec.Body.String())
	}
}

func TestAdminEndpointsReturnExpectedSchemas(t *testing.T) {
	router, db, adminToken, _ := setupAdminContractRouter(t)
	seedAdminContractData(t, db)

	type schemaCheck struct {
		path   string
		assert func(t *testing.T, body []byte)
	}

	checks := []schemaCheck{
		{
			path: "/api/v1/categories?zone=fanwork&level=category",
			assert: func(t *testing.T, body []byte) {
				t.Helper()
				var payload struct {
					Categories []model.Category `json:"categories"`
				}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("decode categories: %v; body = %s", err, body)
				}
				if len(payload.Categories) == 0 {
					t.Fatal("expected at least one category")
				}
				if payload.Categories[0].ID == 0 || payload.Categories[0].Slug == "" {
					t.Fatalf("category schema drift: %#v", payload.Categories[0])
				}
			},
		},
		{
			path: "/api/v1/admin/audit-logs",
			assert: func(t *testing.T, body []byte) {
				t.Helper()
				var payload struct {
					Items    []model.AdminAuditLog `json:"items"`
					Total    int64                 `json:"total"`
					Page     int                   `json:"page"`
					PageSize int                   `json:"page_size"`
				}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("decode audit logs: %v; body = %s", err, body)
				}
				if payload.Page != 1 || payload.PageSize != 20 {
					t.Fatalf("pagination schema drift: %#v", payload)
				}
				if payload.Total < 1 || len(payload.Items) < 1 {
					t.Fatalf("expected audit items, got %#v", payload)
				}
			},
		},
		{
			path: "/api/v1/admin/feedback",
			assert: func(t *testing.T, body []byte) {
				t.Helper()
				var payload struct {
					Items    []model.FeedbackTicket `json:"items"`
					Total    int64                  `json:"total"`
					Page     int                    `json:"page"`
					PageSize int                    `json:"page_size"`
				}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("decode feedback list: %v; body = %s", err, body)
				}
				if payload.Total != 1 || len(payload.Items) != 1 {
					t.Fatalf("feedback list schema drift: %#v", payload)
				}
			},
		},
		{
			path: "/api/v1/admin/feedback/77",
			assert: func(t *testing.T, body []byte) {
				t.Helper()
				var payload model.FeedbackTicket
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("decode feedback detail: %v; body = %s", err, body)
				}
				if payload.ID != 77 || payload.Title == "" {
					t.Fatalf("feedback detail schema drift: %#v", payload)
				}
			},
		},
		{
			path: "/api/v1/admin/reports",
			assert: func(t *testing.T, body []byte) {
				t.Helper()
				var payload struct {
					Reports  []map[string]any `json:"reports"`
					Total    int64            `json:"total"`
					Page     int              `json:"page"`
					PageSize int              `json:"page_size"`
				}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("decode reports: %v; body = %s", err, body)
				}
				if payload.Total != 1 || len(payload.Reports) != 1 {
					t.Fatalf("reports schema drift: %#v", payload)
				}
				for _, key := range []string{"id", "reporter_id", "target_type", "target_id", "reason", "status", "created_at"} {
					if _, ok := payload.Reports[0][key]; !ok {
						t.Fatalf("report schema missing %q: %#v", key, payload.Reports[0])
					}
				}
			},
		},
	}

	for _, check := range checks {
		t.Run(check.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, check.path, nil)
			if bytes.HasPrefix([]byte(check.path), []byte("/api/v1/admin/")) {
				req.Header.Set("Authorization", "Bearer "+adminToken)
			}
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
			}
			check.assert(t, rec.Body.Bytes())
		})
	}
}

func setupAdminContractRouter(t *testing.T) (*gin.Engine, *gorm.DB, string, string) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Category{},
		&model.AdminAuditLog{},
		&model.FeedbackTicket{},
		&model.FeedbackReply{},
		&model.FeedbackAttachment{},
		&model.Report{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := &config.Config{}
	cfg.JWT.Secret = "admin-contract-secret"

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	categoryHandler := NewCategoryHandler(db, auditSvc)
	feedbackSvc := service.NewFeedbackService(repository.NewFeedbackRepository(db), repository.NewUserRepository(db), nil, nil, 300)
	adminFeedbackHandler := NewAdminFeedbackHandler(db, feedbackSvc, auditSvc)
	adminHandler := NewAdminHandler(db, cfg, nil, auditSvc)
	adminAuditHandler := NewAdminAuditHandler(auditSvc)
	authReq := middleware.AuthRequired(cfg, nil, db)

	adminUser := model.User{
		Email:        "admin-contract@example.com",
		Username:     "admin-contract",
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "admin",
	}
	if err := db.Create(&adminUser).Error; err != nil {
		t.Fatalf("create admin user: %v", err)
	}
	normalUser := model.User{
		Email:        "user-contract@example.com",
		Username:     "user-contract",
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
	}
	if err := db.Create(&normalUser).Error; err != nil {
		t.Fatalf("create normal user: %v", err)
	}

	adminToken := mustToken(t, cfg, adminUser.ID, adminUser.Role)
	userToken := mustToken(t, cfg, normalUser.ID, normalUser.Role)

	router := gin.New()
	router.GET("/api/v1/categories", categoryHandler.ListCategories)
	admin := router.Group("/api/v1/admin", authReq, middleware.AdminRequired())
	admin.POST("/categories", func(c *gin.Context) {
		c.Set("trace_id", "trace-admin-contract")
		categoryHandler.AdminCreateCategory(c)
	})
	admin.GET("/audit-logs", adminAuditHandler.ListAuditLogs)
	admin.GET("/feedback", adminFeedbackHandler.ListFeedback)
	admin.GET("/feedback/:id", adminFeedbackHandler.GetFeedback)
	admin.GET("/reports", adminHandler.ListReports)

	return router, db, adminToken, userToken
}

func seedAdminContractData(t *testing.T, db *gorm.DB) {
	t.Helper()

	cat := model.Category{
		ID:        10,
		Zone:      "fanwork",
		Level:     "primary",
		NameI18n:  model.JSONMap{"en": "Games Root"},
		Slug:      "games-root",
		SortOrder: 1,
		IsActive:  true,
	}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatalf("create root category: %v", err)
	}

	child := model.Category{
		ID:        11,
		Zone:      "fanwork",
		Level:     "category",
		ParentID:  ptrInt64(10),
		NameI18n:  model.JSONMap{"en": "Games"},
		Slug:      "games",
		SortOrder: 1,
		IsActive:  true,
	}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child category: %v", err)
	}

	adminAudit := model.AdminAuditLog{
		AdminUserID: 1,
		Action:      "category_create",
		TargetType:  "category",
		TargetID:    strconv.FormatInt(child.ID, 10),
		TraceID:     "trace-admin-contract",
		Metadata:    model.JSONMap{"slug": "games"},
		Result:      "success",
	}
	if err := db.Create(&adminAudit).Error; err != nil {
		t.Fatalf("create audit log: %v", err)
	}

	ticket := model.FeedbackTicket{
		ID:           77,
		UserID:       ptrInt64(2),
		ContactEmail: "user@example.com",
		Category:     "web_bug",
		Title:        "Broken control",
		Description:  "The select looks native.",
		Status:       "open",
		Priority:     "normal",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create feedback ticket: %v", err)
	}

	report := model.Report{
		ReporterID: 2,
		TargetType: "content",
		TargetID:   42,
		Reason:     "spam",
		Status:     "pending",
	}
	if err := db.Create(&report).Error; err != nil {
		t.Fatalf("create report: %v", err)
	}
}

func mustToken(t *testing.T, cfg *config.Config, userID int64, role string) string {
	t.Helper()
	pair, err := jwtutil.GenerateTokenPair(userID, role, cfg.JWT.Secret, 120, 7)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return pair.AccessToken
}

func ptrInt64(v int64) *int64 {
	return &v
}
