package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	jwtutil "omnicraft/backend/internal/pkg/jwt"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

func TestAdminNotificationBroadcastRejectsNonAdmin(t *testing.T) {
	router, db, _, userToken, _ := setupAdminNotificationBroadcastRouter(t)

	rec := postAdminNotificationBroadcast(t, router, userToken, `{
		"title": "Maintenance notice",
		"body": "The site will be down briefly.",
		"channel": "broadcast"
	}`)

	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "FORBIDDEN")
	require.Equal(t, int64(0), countAdminBroadcastNotifications(t, db))
	require.Empty(t, listAdminBroadcastAuditLogs(t, db))
}

func TestAdminNotificationBroadcastSuccessResponseAndAudit(t *testing.T) {
	router, db, adminToken, _, seeded := setupAdminNotificationBroadcastRouter(t)
	body := "The site will be down briefly. Keep this full body out of audit metadata."

	rec := postAdminNotificationBroadcast(t, router, adminToken, fmt.Sprintf(`{
		"title": "  Maintenance notice  ",
		"body": %q,
		"channel": ""
	}`, "  "+body+"  "))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload struct {
		Data struct {
			RecipientCount int    `json:"recipient_count"`
			BroadcastAt    string `json:"broadcast_at"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, len(seeded.activeUsers), payload.Data.RecipientCount)
	_, err := time.Parse(time.RFC3339Nano, payload.Data.BroadcastAt)
	require.NoError(t, err)
	require.Equal(t, int64(len(seeded.activeUsers)), countAdminBroadcastNotifications(t, db))

	logs := listAdminBroadcastAuditLogs(t, db)
	require.Len(t, logs, 1)
	require.Equal(t, "success", logs[0].Result)
	require.Equal(t, "users", logs[0].TargetType)
	require.Equal(t, float64(len(seeded.activeUsers)), logs[0].Metadata["recipient_count"])
	assertAdminBroadcastAuditHasNoFullText(t, logs[0].Metadata, "Maintenance notice", body)
}

func TestAdminNotificationBroadcastValidationErrorsWriteRejectedAudit(t *testing.T) {
	longTitle := strings.Repeat("t", 121)
	longBody := strings.Repeat("b", 5001)

	cases := []struct {
		name  string
		title string
		body  string
		ch    string
		field string
	}{
		{name: "blank title", title: "   ", body: "Valid body", ch: "broadcast", field: "title"},
		{name: "blank body", title: "Valid title", body: "   ", ch: "broadcast", field: "body"},
		{name: "title too long", title: longTitle, body: "Valid body", ch: "broadcast", field: "title"},
		{name: "body too long", title: "Valid title", body: longBody, ch: "broadcast", field: "body"},
		{name: "invalid channel", title: "Valid title", body: "Valid body", ch: "email", field: "channel"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, db, adminToken, _, _ := setupAdminNotificationBroadcastRouter(t)

			rec := postAdminNotificationBroadcast(t, router, adminToken, fmt.Sprintf(`{
				"title": %q,
				"body": %q,
				"channel": %q
			}`, tc.title, tc.body, tc.ch))

			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var errPayload struct {
				Code string `json:"code"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errPayload))
			require.Equal(t, "VALIDATION_ERROR", errPayload.Code)
			require.Equal(t, int64(0), countAdminBroadcastNotifications(t, db))

			logs := listAdminBroadcastAuditLogs(t, db)
			require.Len(t, logs, 1)
			require.Equal(t, "rejected", logs[0].Result)
			require.Equal(t, "VALIDATION_ERROR", logs[0].Metadata["validation_error_code"])
			requireValidationField(t, logs[0].Metadata, tc.field)
			assertAdminBroadcastAuditHasNoFullText(t, logs[0].Metadata, tc.title, tc.body)
		})
	}
}

func TestAdminNotificationBroadcastFailureWritesFailedAudit(t *testing.T) {
	router, db, adminToken, _, seeded := setupAdminNotificationBroadcastRouter(t)
	installAdminBroadcastNotificationFailureTrigger(t, db, seeded.activeUsers[0].ID)
	body := "A forced insert failure must not store this full Markdown body in audit metadata."

	rec := postAdminNotificationBroadcast(t, router, adminToken, fmt.Sprintf(`{
		"title": "Failure path",
		"body": %q,
		"channel": "broadcast"
	}`, body))

	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	require.Equal(t, int64(0), countAdminBroadcastNotifications(t, db))

	logs := listAdminBroadcastAuditLogs(t, db)
	require.Len(t, logs, 1)
	require.Equal(t, "failed", logs[0].Result)
	require.Contains(t, logs[0].Metadata, "error_code")
	assertAdminBroadcastAuditHasNoFullText(t, logs[0].Metadata, "Failure path", body)
}

type adminBroadcastSeededUsers struct {
	activeUsers []model.User
}

func setupAdminNotificationBroadcastRouter(t *testing.T) (*gin.Engine, *gorm.DB, string, string, adminBroadcastSeededUsers) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Notification{}, &model.AdminAuditLog{}, &model.NotificationBroadcastRequest{}))

	cfg := &config.Config{}
	cfg.JWT.Secret = "admin-broadcast-secret"

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	notifSvc := service.NewNotificationService(repository.NewNotificationRepository(db))
	notifSvc.SetAdminAuditService(auditSvc)

	adminHandler := NewAdminHandler(db, cfg, nil, auditSvc)
	adminHandler.SetNotificationService(notifSvc)

	adminUser := createAdminBroadcastRouteUser(t, db, "admin-broadcast", "admin", false, nil)
	normalUser := createAdminBroadcastRouteUser(t, db, "user-broadcast", "user", false, nil)
	recipientUser := createAdminBroadcastRouteUser(t, db, "recipient-broadcast", "user", false, nil)
	deletedAt := time.Now().UTC()
	createAdminBroadcastRouteUser(t, db, "deleted-broadcast", "user", false, &deletedAt)
	createAdminBroadcastRouteUser(t, db, "banned-broadcast", "user", true, nil)

	router := gin.New()
	admin := router.Group("/api/v1/admin", middleware.AuthRequired(cfg, nil, db), middleware.AdminRequired())
	admin.POST("/notifications/broadcast", func(c *gin.Context) {
		c.Set("trace_id", "trace-admin-broadcast")
		adminHandler.BroadcastNotification(c)
	})

	return router,
		db,
		adminBroadcastToken(t, cfg, adminUser.ID, adminUser.Role),
		adminBroadcastToken(t, cfg, normalUser.ID, normalUser.Role),
		adminBroadcastSeededUsers{activeUsers: []model.User{adminUser, normalUser, recipientUser}}
}

func adminBroadcastToken(t *testing.T, cfg *config.Config, userID int64, role string) string {
	t.Helper()

	pair, err := jwtutil.GenerateTokenPair(userID, role, cfg.JWT.Secret, 120, 7)
	require.NoError(t, err)
	return pair.AccessToken
}

func createAdminBroadcastRouteUser(t *testing.T, db *gorm.DB, username, role string, banned bool, deletedAt *time.Time) model.User {
	t.Helper()

	user := model.User{
		Email:        username + "@example.com",
		Username:     username,
		PasswordHash: "hash",
		Reputation:   10,
		Role:         role,
		IsBanned:     banned,
		DeletedAt:    deletedAt,
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func postAdminNotificationBroadcast(t *testing.T, router *gin.Engine, token string, body string) *httptest.ResponseRecorder {
	t.Helper()

	return postAdminNotificationBroadcastWithKey(t, router, token, body, "handler-idem-key")
}

func postAdminNotificationBroadcastWithKey(t *testing.T, router *gin.Engine, token string, body string, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/notifications/broadcast", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	router.ServeHTTP(rec, req)
	return rec
}

func listAdminBroadcastAuditLogs(t *testing.T, db *gorm.DB) []model.AdminAuditLog {
	t.Helper()

	var logs []model.AdminAuditLog
	require.NoError(t, db.Where("action = ?", "broadcast_notification").Order("id ASC").Find(&logs).Error)
	return logs
}

func countAdminBroadcastNotifications(t *testing.T, db *gorm.DB) int64 {
	t.Helper()

	var count int64
	require.NoError(t, db.Model(&model.Notification{}).Where("channel = ?", "broadcast").Count(&count).Error)
	return count
}

func installAdminBroadcastNotificationFailureTrigger(t *testing.T, db *gorm.DB, userID int64) {
	t.Helper()

	require.NoError(t, db.Exec(fmt.Sprintf(`
		CREATE TRIGGER fail_admin_broadcast_notification_insert
		BEFORE INSERT ON notifications
		WHEN NEW.channel = 'broadcast' AND NEW.user_id = %d
		BEGIN
			SELECT RAISE(ABORT, 'forced admin broadcast insert failure');
		END;
	`, userID)).Error)
}

func requireValidationField(t *testing.T, metadata model.JSONMap, field string) {
	t.Helper()

	raw, ok := metadata["validation_fields"]
	require.True(t, ok, "metadata missing validation_fields: %#v", metadata)
	switch fields := raw.(type) {
	case []interface{}:
		for _, candidate := range fields {
			if candidate == field {
				return
			}
		}
	case []string:
		for _, candidate := range fields {
			if candidate == field {
				return
			}
		}
	}
	t.Fatalf("validation_fields = %#v, want %q", raw, field)
}

func assertAdminBroadcastAuditHasNoFullText(t *testing.T, metadata model.JSONMap, title, body string) {
	t.Helper()

	for _, key := range []string{"body", "title", "markdown", "html", "recipients"} {
		require.NotContains(t, metadata, key)
	}
	raw, err := json.Marshal(metadata)
	require.NoError(t, err)
	if strings.TrimSpace(title) != "" {
		require.NotContains(t, string(raw), strings.TrimSpace(title))
	}
	if strings.TrimSpace(body) != "" {
		require.NotContains(t, string(raw), strings.TrimSpace(body))
	}
}

func TestAdminNotificationBroadcastIdempotencyKeyRequired(t *testing.T) {
	router, db, adminToken, _, _ := setupAdminNotificationBroadcastRouter(t)

	rec := postAdminNotificationBroadcastWithKey(t, router, adminToken, `{
		"title": "Maintenance notice",
		"body": "The site will be down briefly.",
		"channel": "broadcast"
	}`, "")

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var errPayload struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errPayload))
	require.Equal(t, "IDEMPOTENCY_KEY_REQUIRED", errPayload.Code)
	require.Equal(t, int64(0), countAdminBroadcastNotifications(t, db))
}

func TestAdminNotificationBroadcastReplayReturnsStoredResultWithoutDuplicates(t *testing.T) {
	router, db, adminToken, _, seeded := setupAdminNotificationBroadcastRouter(t)
	body := `{"title":"Maintenance notice","body":"The site will be down briefly.","channel":"broadcast"}`

	first := postAdminNotificationBroadcastWithKey(t, router, adminToken, body, "replay-key-1")
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	second := postAdminNotificationBroadcastWithKey(t, router, adminToken, body, "replay-key-1")
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())

	var payload struct {
		Data struct {
			RecipientCount int    `json:"recipient_count"`
			BroadcastAt    string `json:"broadcast_at"`
			Replayed       bool   `json:"replayed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &payload))
	require.True(t, payload.Data.Replayed)
	require.Equal(t, len(seeded.activeUsers), payload.Data.RecipientCount)
	require.Equal(t, int64(len(seeded.activeUsers)), countAdminBroadcastNotifications(t, db))

	var successAudits int64
	require.NoError(t, db.Model(&model.AdminAuditLog{}).
		Where("action = ? AND result = ?", "broadcast_notification", "success").
		Count(&successAudits).Error)
	require.Equal(t, int64(1), successAudits)
}

func TestAdminNotificationBroadcastSameKeyDifferentPayloadConflict(t *testing.T) {
	router, _, adminToken, _, _ := setupAdminNotificationBroadcastRouter(t)

	first := postAdminNotificationBroadcastWithKey(t, router, adminToken, `{
		"title": "Maintenance notice",
		"body": "The site will be down briefly.",
		"channel": "broadcast"
	}`, "conflict-key-1")
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())

	second := postAdminNotificationBroadcastWithKey(t, router, adminToken, `{
		"title": "Maintenance notice",
		"body": "A different body reuses the key.",
		"channel": "broadcast"
	}`, "conflict-key-1")
	require.Equal(t, http.StatusConflict, second.Code, second.Body.String())
	var errPayload struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &errPayload))
	require.Equal(t, "IDEMPOTENCY_KEY_REUSED", errPayload.Code)
}
