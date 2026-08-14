package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/events"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

func setupReplayRouter(t *testing.T) (*gin.Engine, *gorm.DB, *redis.Client, string, string) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	requireNoErr(t, err)
	requireNoErr(t, db.AutoMigrate(&model.User{}, &model.AdminAuditLog{}))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cfg := &config.Config{}
	cfg.JWT.Secret = "replay-contract-secret"

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	adminHandler := NewAdminHandler(db, cfg, rdb, auditSvc)
	authReq := middleware.AuthRequired(cfg, nil, db)

	adminUser := model.User{Email: "replay-admin@example.com", Username: "replay-admin", PasswordHash: "hash", Reputation: 10, Role: "admin"}
	normalUser := model.User{Email: "replay-user@example.com", Username: "replay-user", PasswordHash: "hash", Reputation: 10, Role: "user"}
	requireNoErr(t, db.Create(&adminUser).Error)
	requireNoErr(t, db.Create(&normalUser).Error)

	adminToken := mustToken(t, cfg, adminUser.ID, adminUser.Role)
	userToken := mustToken(t, cfg, normalUser.ID, normalUser.Role)

	router := gin.New()
	admin := router.Group("/api/v1/admin", authReq, middleware.AdminRequired())
	admin.POST("/queue/dlq/:id/replay", adminHandler.ReplayDLQEntry)

	return router, db, rdb, adminToken, userToken
}

func requireNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func seedDLQEntry(t *testing.T, rdb *redis.Client, topic, originalID string) string {
	t.Helper()
	env, err := events.NewContentEnvelope(events.TopicContentPublished, 9001, "", "",
		events.ContentEventPayload{ContentID: 9001, AuthorID: 7, ContentType: "article"})
	requireNoErr(t, err)
	env.EventID = 9001 // the relay fills the event id before XAdd
	raw, err := json.Marshal(env)
	requireNoErr(t, err)

	payload, err := json.Marshal(map[string]interface{}{
		"original_topic": topic,
		"original_id":    originalID,
		"consumer_group": "omnicraft-indexer",
		"payload":        string(raw),
		"attempts":       3,
		"error":          "permanent failure",
		"failed_at":      "2026-08-13T00:00:00Z",
	})
	requireNoErr(t, err)
	id, err := rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "omnicraft:dead-letter",
		Values: map[string]interface{}{"data": string(payload)},
	}).Result()
	requireNoErr(t, err)
	return id
}

func TestReplayDLQEntryRequiresAdminRole(t *testing.T) {
	router, _, _, adminToken, userToken := setupReplayRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/queue/dlq/1-0/replay", nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want 401; body = %s", rec.Code, rec.Body.String())
	}

	req.Header.Set("Authorization", "Bearer "+userToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("FORBIDDEN")) {
		t.Fatalf("body = %s, want FORBIDDEN", rec.Body.String())
	}

	_ = adminToken
}

func TestReplayDLQEntryPositivePath(t *testing.T) {
	router, db, rdb, adminToken, _ := setupReplayRouter(t)
	entryID := seedDLQEntry(t, rdb, "content.published", "1700000000000-0")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/queue/dlq/"+entryID+"/replay", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	// The envelope must land back on the original topic stream.
	stream, err := rdb.XRange(context.Background(), "omnicraft:content.published", "-", "+").Result()
	requireNoErr(t, err)
	if len(stream) != 1 {
		t.Fatalf("replayed stream messages = %d, want 1", len(stream))
	}
	var replayed struct {
		EventID   int64  `json:"event_id"`
		EventType string `json:"event_type"`
	}
	requireNoErr(t, json.Unmarshal([]byte(stream[0].Values["payload"].(string)), &replayed))
	if replayed.EventType != "content.published" || replayed.EventID == 0 {
		t.Fatalf("replayed envelope drift: %#v", replayed)
	}

	// The audit row must record operator / time / entry id.
	var logs []model.AdminAuditLog
	requireNoErr(t, db.Find(&logs).Error)
	if len(logs) != 1 {
		t.Fatalf("audit log count = %d, want 1", len(logs))
	}
	if logs[0].Action != "dlq_replay" || logs[0].TargetID != entryID || logs[0].Result != "success" {
		t.Fatalf("audit log = %#v, want dlq_replay success on %s", logs[0], entryID)
	}
	if logs[0].AdminUserID == 0 || logs[0].CreatedAt.IsZero() {
		t.Fatalf("audit log must carry operator id and time: %#v", logs[0])
	}
}

func TestReplayDLQEntryNotFound(t *testing.T) {
	router, db, _, adminToken, _ := setupReplayRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/queue/dlq/9999999999999-0/replay", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("DLQ_ENTRY_NOT_FOUND")) {
		t.Fatalf("body = %s, want DLQ_ENTRY_NOT_FOUND", rec.Body.String())
	}

	var logs []model.AdminAuditLog
	requireNoErr(t, db.Find(&logs).Error)
	if len(logs) != 1 || logs[0].Result != "failed" {
		t.Fatalf("failed replay must be audited with result=failed: %#v", logs)
	}
}

func TestReplayDLQEntryInvalidID(t *testing.T) {
	router, _, _, adminToken, _ := setupReplayRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/queue/dlq/not-a-stream-id/replay", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("INVALID_ID")) {
		t.Fatalf("body = %s, want INVALID_ID", rec.Body.String())
	}
}
