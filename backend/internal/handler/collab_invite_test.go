package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"omnicraft/backend/internal/testutil"
)

func setupCollabInviteHandlerTest(t *testing.T) (*gin.Engine, *gorm.DB, *redis.Client, *miniredis.Miniredis) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	createCollabInviteHandlerBaseSchema(t, db)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "065_collaboration_invites.sql"))

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })

	cfg := &config.Config{
		Collaboration: config.CollaborationConfig{
			InviteDailyLimit:       20,
			InviteExpireDays:       7,
			MaxInviteesPerPublish:  5,
			MaxContributorsPerItem: 10,
		},
		Reputation: config.ReputationConfig{MinScoreForInteraction: 3},
		Cache:      config.CacheConfig{UserStatusTTL: 300},
		JWT:        config.JWTConfig{Secret: "test-secret", AccessTokenTTL: 120, RefreshTokenTTL: 7},
	}

	userRepo := repository.NewUserRepository(db)
	svc := service.NewCollabInviteService(
		repository.NewContentRepository(db),
		repository.NewCollabInviteRepository(db),
		repository.NewMessageRepository(db),
		userRepo,
		rdb,
		cfg,
	)
	collabHandler := NewCollabInviteHandler(svc)
	userHandler := NewUserHandler(db, service.NewAuthService(userRepo, rdb, cfg), rdb, cfg)
	authHandler := NewAuthHandler(
		service.NewAuthService(userRepo, rdb, cfg),
		service.NewVerificationService(userRepo, rdb, nil, cfg),
		userRepo,
		nil,
		rdb,
		cfg,
	)

	guard := middleware.InteractionRequired(cfg, db, rdb, middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	})

	testAuth := func(c *gin.Context) {
		if raw := c.GetHeader("X-Test-User-ID"); raw != "" {
			if userID, err := strconv.ParseInt(raw, 10, 64); err == nil {
				c.Set(middleware.UserIDKey, userID)
			}
		}
	}

	router := gin.New()
	router.POST("/api/v1/contents/:id/collab-invites", testAuth, guard, collabHandler.SendInvite)
	router.POST("/api/v1/collab-invites/:id/accept", testAuth, collabHandler.AcceptInvite)
	router.POST("/api/v1/collab-invites/:id/decline", testAuth, collabHandler.DeclineInvite)
	router.PATCH("/api/v1/users/:id", testAuth, userHandler.UpdateUser)
	router.GET("/api/v1/auth/me", testAuth, authHandler.Me)

	return router, db, rdb, mr
}

func createCollabInviteHandlerBaseSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			username VARCHAR(64) UNIQUE NOT NULL,
			reputation INT NOT NULL DEFAULT 10,
			role VARCHAR(20) NOT NULL DEFAULT 'user',
			is_banned BOOLEAN NOT NULL DEFAULT FALSE,
			email_verified_at TIMESTAMPTZ,
			deleted_at TIMESTAMPTZ,
			accept_collab_invites BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE content_items (
			id BIGSERIAL PRIMARY KEY,
			title VARCHAR(500) NOT NULL,
			author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			zone VARCHAR(10) NOT NULL,
			content_type VARCHAR(20) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			is_public BOOLEAN NOT NULL DEFAULT TRUE,
			deleted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE content_contributors (
			content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			pr_count INT NOT NULL DEFAULT 1,
			first_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (content_item_id, user_id)
		);
		CREATE TABLE author_blocklist (
			author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			blocked_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (author_id, blocked_id)
		);
		CREATE TABLE conversations (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE conversation_participants (
			conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			last_read_at TIMESTAMPTZ,
			unread_count INTEGER NOT NULL DEFAULT 0,
			left_at TIMESTAMPTZ,
			PRIMARY KEY (conversation_id, user_id)
		);
		CREATE TABLE messages (
			id BIGSERIAL PRIMARY KEY,
			conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			sender_id BIGINT NOT NULL REFERENCES users(id),
			body TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`).Error; err != nil {
		t.Fatalf("create base schema: %v", err)
	}
}

func seedCollabHandlerUser(t *testing.T, db *gorm.DB, id int64, username string, reputation int, verified, banned bool, deletedAt *time.Time, accepts *bool) {
	t.Helper()
	acceptsValue := true
	if accepts != nil {
		acceptsValue = *accepts
	}
	var verifiedAt *time.Time
	if verified {
		now := time.Now()
		verifiedAt = &now
	}
	if err := db.Exec(`
		INSERT INTO users (id, email, password_hash, username, reputation, role, is_banned, email_verified_at, deleted_at, accept_collab_invites)
		VALUES (?, ?, 'hash', ?, ?, 'user', ?, ?, ?, ?)
	`, id, fmt.Sprintf("%s@example.test", username), username, reputation, banned, verifiedAt, deletedAt, acceptsValue).Error; err != nil {
		t.Fatalf("seed user %d: %v", id, err)
	}
}

func seedCollabHandlerContent(t *testing.T, db *gorm.DB, id, authorID int64, status string, deletedAt *time.Time) {
	t.Helper()
	if err := db.Exec(`
		INSERT INTO content_items (id, title, author_id, zone, content_type, status, is_public, deleted_at)
		VALUES (?, ?, ?, 'original', 'article', ?, TRUE, ?)
	`, id, fmt.Sprintf("content-%d", id), authorID, status, deletedAt).Error; err != nil {
		t.Fatalf("seed content %d: %v", id, err)
	}
}

func seedCollabHandlerInvite(t *testing.T, db *gorm.DB, id, contentID, inviterID, inviteeID int64, status string, expiresIn time.Duration) {
	t.Helper()
	if err := db.Exec(`
		INSERT INTO collaboration_invites (id, content_id, inviter_id, invitee_id, status, expires_at)
		VALUES (?, ?, ?, ?, ?, NOW() + (? * INTERVAL '1 second'))
	`, id, contentID, inviterID, inviteeID, status, int64(expiresIn.Seconds())).Error; err != nil {
		t.Fatalf("seed invite %d: %v", id, err)
	}
}

func collabInviteRequest(t *testing.T, router *gin.Engine, method, path, body string, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if userID != 0 {
		req.Header.Set("X-Test-User-ID", strconv.FormatInt(userID, 10))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeCollabInviteError(t *testing.T, rec *httptest.ResponseRecorder) (code, message string) {
	t.Helper()
	var payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v; body = %s", err, rec.Body.String())
	}
	return payload.Code, payload.Message
}

func seedCollabHandlerContributor(t *testing.T, db *gorm.DB, contentID, userID int64) {
	t.Helper()
	if err := db.Exec(`
		INSERT INTO content_contributors (content_item_id, user_id, pr_count, first_at)
		VALUES (?, ?, 1, NOW())
	`, contentID, userID).Error; err != nil {
		t.Fatalf("seed contributor %d/%d: %v", contentID, userID, err)
	}
}

func collabInviteCounts(t *testing.T, db *gorm.DB) (invites, conversations, messages int64) {
	t.Helper()
	if err := db.Model(&model.CollabInvite{}).Count(&invites).Error; err != nil {
		t.Fatalf("count invites: %v", err)
	}
	if err := db.Model(&model.Conversation{}).Count(&conversations).Error; err != nil {
		t.Fatalf("count conversations: %v", err)
	}
	if err := db.Model(&model.Message{}).Count(&messages).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	return
}

func collabInviteContributorExists(t *testing.T, db *gorm.DB, contentID, userID int64) bool {
	t.Helper()
	var count int64
	if err := db.Model(&model.ContentContributor{}).
		Where("content_item_id = ? AND user_id = ?", contentID, userID).
		Count(&count).Error; err != nil {
		t.Fatalf("count contributors: %v", err)
	}
	return count > 0
}

func TestCollabInviteSendHappyPath(t *testing.T) {
	router, db, _, _ := setupCollabInviteHandlerTest(t)
	seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)
	seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, nil)
	seedCollabHandlerContent(t, db, 100, 1, "published", nil)

	rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/contents/100/collab-invites", `{"invitee_id":2}`, 1)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Invite struct {
			ID        int64  `json:"id"`
			Status    string `json:"status"`
			MessageID *int64 `json:"message_id"`
			ContentID int64  `json:"content_id"`
			InviterID int64  `json:"inviter_id"`
			InviteeID int64  `json:"invitee_id"`
		} `json:"invite"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
	}
	if payload.Invite.ID != 1 {
		t.Fatalf("invite id = %d, want 1", payload.Invite.ID)
	}
	if payload.Invite.Status != "pending" {
		t.Fatalf("invite status = %q, want pending", payload.Invite.Status)
	}
	if payload.Invite.MessageID == nil || *payload.Invite.MessageID == 0 {
		t.Fatalf("invite message_id = %v, want non-null; body = %s", payload.Invite.MessageID, rec.Body.String())
	}
	if payload.Invite.ContentID != 100 || payload.Invite.InviterID != 1 || payload.Invite.InviteeID != 2 {
		t.Fatalf("invite ids = %+v, want content 100 inviter 1 invitee 2", payload.Invite)
	}

	invites, conversations, messages := collabInviteCounts(t, db)
	if invites != 1 || conversations != 1 || messages != 1 {
		t.Fatalf("counts = %d/%d/%d, want 1/1/1", invites, conversations, messages)
	}
}

func TestCollabInviteSendNonexistentContentFailsBeforeInvite(t *testing.T) {
	router, db, _, _ := setupCollabInviteHandlerTest(t)
	seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)
	seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, nil)

	rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/contents/999/collab-invites", `{"invitee_id":2}`, 1)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if code, _ := decodeCollabInviteError(t, rec); code != "CONTENT_UNAVAILABLE" {
		t.Fatalf("code = %q, want CONTENT_UNAVAILABLE; body = %s", code, rec.Body.String())
	}

	invites, conversations, messages := collabInviteCounts(t, db)
	if invites != 0 || conversations != 0 || messages != 0 {
		t.Fatalf("counts = %d/%d/%d, want 0/0/0 (no invite may be created)", invites, conversations, messages)
	}
}

func TestCollabInviteSendAuthorAndOwnerChecks(t *testing.T) {
	t.Run("invitee is the content author", func(t *testing.T) {
		router, db, _, _ := setupCollabInviteHandlerTest(t)
		seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)
		seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, nil)
		seedCollabHandlerContent(t, db, 100, 2, "published", nil)
		seedCollabHandlerContributor(t, db, 100, 1)

		rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/contents/100/collab-invites", `{"invitee_id":2}`, 1)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
		}
		if code, _ := decodeCollabInviteError(t, rec); code != "INVITE_AUTHOR_NOT_ALLOWED" {
			t.Fatalf("code = %q, want INVITE_AUTHOR_NOT_ALLOWED; body = %s", code, rec.Body.String())
		}
	})

	t.Run("inviter is neither author nor contributor", func(t *testing.T) {
		router, db, _, _ := setupCollabInviteHandlerTest(t)
		seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)
		seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, nil)
		seedCollabHandlerUser(t, db, 3, "carol", 10, true, false, nil, nil)
		seedCollabHandlerContent(t, db, 100, 3, "published", nil)

		rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/contents/100/collab-invites", `{"invitee_id":2}`, 1)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
		}
		if code, _ := decodeCollabInviteError(t, rec); code != "NOT_CONTENT_OWNER" {
			t.Fatalf("code = %q, want NOT_CONTENT_OWNER; body = %s", code, rec.Body.String())
		}
	})
}

func TestCollabInviteSendRejectsInviteeWhoClosedInvites(t *testing.T) {
	router, db, _, _ := setupCollabInviteHandlerTest(t)
	accepts := false
	seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)
	seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, &accepts)
	seedCollabHandlerContent(t, db, 100, 1, "published", nil)

	rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/contents/100/collab-invites", `{"invitee_id":2}`, 1)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
	if code, _ := decodeCollabInviteError(t, rec); code != "INVITE_NOT_ACCEPTING" {
		t.Fatalf("code = %q, want INVITE_NOT_ACCEPTING; body = %s", code, rec.Body.String())
	}
}

func TestCollabInviteSendDuplicateInvites(t *testing.T) {
	router, db, _, _ := setupCollabInviteHandlerTest(t)
	seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)
	seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, nil)
	seedCollabHandlerContent(t, db, 100, 1, "published", nil)

	first := collabInviteRequest(t, router, http.MethodPost, "/api/v1/contents/100/collab-invites", `{"invitee_id":2}`, 1)
	if first.Code != http.StatusCreated {
		t.Fatalf("first send status = %d, want 201; body = %s", first.Code, first.Body.String())
	}

	t.Run("active invite already exists", func(t *testing.T) {
		rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/contents/100/collab-invites", `{"invitee_id":2}`, 1)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
		}
		if code, _ := decodeCollabInviteError(t, rec); code != "INVITE_ALREADY_EXISTS" {
			t.Fatalf("code = %q, want INVITE_ALREADY_EXISTS; body = %s", code, rec.Body.String())
		}
	})

	t.Run("declined invite still consumes the daily per-user quota", func(t *testing.T) {
		declined := collabInviteRequest(t, router, http.MethodPost, "/api/v1/collab-invites/1/decline", "", 2)
		if declined.Code != http.StatusOK {
			t.Fatalf("decline status = %d, want 200; body = %s", declined.Code, declined.Body.String())
		}
		rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/contents/100/collab-invites", `{"invitee_id":2}`, 1)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
		}
		if code, _ := decodeCollabInviteError(t, rec); code != "INVITE_DUPLICATE_USER" {
			t.Fatalf("code = %q, want INVITE_DUPLICATE_USER; body = %s", code, rec.Body.String())
		}
	})
}

func TestCollabInviteSendInvalidRequest(t *testing.T) {
	t.Run("missing invitee_id", func(t *testing.T) {
		router, db, _, _ := setupCollabInviteHandlerTest(t)
		seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)
		seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, nil)
		seedCollabHandlerContent(t, db, 100, 1, "published", nil)

		rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/contents/100/collab-invites", `{}`, 1)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
		}
		if code, _ := decodeCollabInviteError(t, rec); code != "VALIDATION_ERROR" {
			t.Fatalf("code = %q, want VALIDATION_ERROR; body = %s", code, rec.Body.String())
		}
	})

	t.Run("non-numeric content id", func(t *testing.T) {
		router, db, _, _ := setupCollabInviteHandlerTest(t)
		seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)
		seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, nil)
		seedCollabHandlerContent(t, db, 100, 1, "published", nil)

		rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/contents/abc/collab-invites", `{"invitee_id":2}`, 1)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
		}
		if code, _ := decodeCollabInviteError(t, rec); code != "INVALID_ID" {
			t.Fatalf("code = %q, want INVALID_ID; body = %s", code, rec.Body.String())
		}
	})
}

func TestCollabInviteSendRouteGuard(t *testing.T) {
	t.Run("no authentication", func(t *testing.T) {
		router, db, _, _ := setupCollabInviteHandlerTest(t)
		seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)

		rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/contents/100/collab-invites", `{"invitee_id":2}`, 0)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("banned user", func(t *testing.T) {
		router, db, _, _ := setupCollabInviteHandlerTest(t)
		seedCollabHandlerUser(t, db, 11, "banned", 10, true, true, nil, nil)

		rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/contents/100/collab-invites", `{"invitee_id":2}`, 11)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body.String())
		}
		if code, _ := decodeCollabInviteError(t, rec); code != "USER_BANNED" {
			t.Fatalf("code = %q, want USER_BANNED; body = %s", code, rec.Body.String())
		}
	})

	t.Run("unverified email", func(t *testing.T) {
		router, db, _, _ := setupCollabInviteHandlerTest(t)
		seedCollabHandlerUser(t, db, 12, "unverified", 10, false, false, nil, nil)

		rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/contents/100/collab-invites", `{"invitee_id":2}`, 12)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
		}
		if code, _ := decodeCollabInviteError(t, rec); code != "EMAIL_NOT_VERIFIED" {
			t.Fatalf("code = %q, want EMAIL_NOT_VERIFIED; body = %s", code, rec.Body.String())
		}
	})

	t.Run("insufficient reputation", func(t *testing.T) {
		router, db, _, _ := setupCollabInviteHandlerTest(t)
		seedCollabHandlerUser(t, db, 13, "lowrep", 2, true, false, nil, nil)

		rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/contents/100/collab-invites", `{"invitee_id":2}`, 13)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
		}
		if code, _ := decodeCollabInviteError(t, rec); code != "INSUFFICIENT_REPUTATION" {
			t.Fatalf("code = %q, want INSUFFICIENT_REPUTATION; body = %s", code, rec.Body.String())
		}
		invites, _, _ := collabInviteCounts(t, db)
		if invites != 0 {
			t.Fatalf("invite count = %d, want 0", invites)
		}
	})
}

func TestCollabInviteAcceptHappyPath(t *testing.T) {
	router, db, _, _ := setupCollabInviteHandlerTest(t)
	seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)
	seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, nil)
	seedCollabHandlerContent(t, db, 100, 1, "published", nil)
	seedCollabHandlerInvite(t, db, 1, 100, 1, 2, "pending", 7*24*time.Hour)

	rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/collab-invites/1/accept", "", 2)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Invite struct {
			ID          int64   `json:"id"`
			Status      string  `json:"status"`
			RespondedAt *string `json:"responded_at"`
		} `json:"invite"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
	}
	if payload.Invite.Status != "accepted" {
		t.Fatalf("invite status = %q, want accepted; body = %s", payload.Invite.Status, rec.Body.String())
	}
	if payload.Invite.RespondedAt == nil || *payload.Invite.RespondedAt == "" {
		t.Fatalf("invite responded_at = %v, want non-empty; body = %s", payload.Invite.RespondedAt, rec.Body.String())
	}
	if !collabInviteContributorExists(t, db, 100, 2) {
		t.Fatal("contributor row (100, 2) must exist after accept")
	}
}

func TestCollabInviteAcceptRejectsNonInvitee(t *testing.T) {
	router, db, _, _ := setupCollabInviteHandlerTest(t)
	seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)
	seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, nil)
	seedCollabHandlerUser(t, db, 3, "carol", 10, true, false, nil, nil)
	seedCollabHandlerContent(t, db, 100, 1, "published", nil)
	seedCollabHandlerInvite(t, db, 1, 100, 1, 2, "pending", 7*24*time.Hour)

	rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/collab-invites/1/accept", "", 3)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
	if code, _ := decodeCollabInviteError(t, rec); code != "INVITE_NOT_INVITEE" {
		t.Fatalf("code = %q, want INVITE_NOT_INVITEE; body = %s", code, rec.Body.String())
	}
}

func TestCollabInviteAcceptRejectsNonPending(t *testing.T) {
	router, db, _, _ := setupCollabInviteHandlerTest(t)
	seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)
	seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, nil)
	seedCollabHandlerContent(t, db, 100, 1, "published", nil)
	seedCollabHandlerInvite(t, db, 1, 100, 1, 2, "declined", 7*24*time.Hour)

	rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/collab-invites/1/accept", "", 2)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}
	if code, _ := decodeCollabInviteError(t, rec); code != "INVITE_NOT_PENDING" {
		t.Fatalf("code = %q, want INVITE_NOT_PENDING; body = %s", code, rec.Body.String())
	}
}

func TestCollabInviteAcceptRejectsExpired(t *testing.T) {
	router, db, _, _ := setupCollabInviteHandlerTest(t)
	seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)
	seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, nil)
	seedCollabHandlerContent(t, db, 100, 1, "published", nil)
	seedCollabHandlerInvite(t, db, 1, 100, 1, 2, "pending", -time.Hour)

	rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/collab-invites/1/accept", "", 2)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	if code, _ := decodeCollabInviteError(t, rec); code != "INVITE_EXPIRED" {
		t.Fatalf("code = %q, want INVITE_EXPIRED; body = %s", code, rec.Body.String())
	}
}

func TestCollabInviteAcceptNotFound(t *testing.T) {
	router, db, _, _ := setupCollabInviteHandlerTest(t)
	seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, nil)

	rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/collab-invites/999/accept", "", 2)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if code, _ := decodeCollabInviteError(t, rec); code != "INVITE_NOT_FOUND" {
		t.Fatalf("code = %q, want INVITE_NOT_FOUND; body = %s", code, rec.Body.String())
	}
}

func TestCollabInviteDeclineHappyPath(t *testing.T) {
	router, db, _, _ := setupCollabInviteHandlerTest(t)
	seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)
	seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, nil)
	seedCollabHandlerContent(t, db, 100, 1, "published", nil)
	seedCollabHandlerInvite(t, db, 1, 100, 1, 2, "pending", 7*24*time.Hour)

	rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/collab-invites/1/decline", "", 2)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Invite struct {
			Status string `json:"status"`
		} `json:"invite"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
	}
	if payload.Invite.Status != "declined" {
		t.Fatalf("invite status = %q, want declined; body = %s", payload.Invite.Status, rec.Body.String())
	}
	if collabInviteContributorExists(t, db, 100, 2) {
		t.Fatal("no contributor row may exist after decline")
	}
}

func TestCollabInviteDeclineGuard(t *testing.T) {
	t.Run("no authentication", func(t *testing.T) {
		router, _, _, _ := setupCollabInviteHandlerTest(t)

		rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/collab-invites/1/decline", "", 0)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("non-invitee cannot decline", func(t *testing.T) {
		router, db, _, _ := setupCollabInviteHandlerTest(t)
		seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)
		seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, nil)
		seedCollabHandlerUser(t, db, 3, "carol", 10, true, false, nil, nil)
		seedCollabHandlerContent(t, db, 100, 1, "published", nil)
		seedCollabHandlerInvite(t, db, 1, 100, 1, 2, "pending", 7*24*time.Hour)

		rec := collabInviteRequest(t, router, http.MethodPost, "/api/v1/collab-invites/1/decline", "", 3)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
		}
		if code, _ := decodeCollabInviteError(t, rec); code != "INVITE_NOT_INVITEE" {
			t.Fatalf("code = %q, want INVITE_NOT_INVITEE; body = %s", code, rec.Body.String())
		}
	})
}

func TestCollabInviteUserSettingsContract(t *testing.T) {
	router, db, _, _ := setupCollabInviteHandlerTest(t)
	seedCollabHandlerUser(t, db, 1, "alice", 10, true, false, nil, nil)
	seedCollabHandlerUser(t, db, 2, "bob", 10, true, false, nil, nil)

	t.Run("current user can disable and re-enable invite reception", func(t *testing.T) {
		rec := collabInviteRequest(t, router, http.MethodPatch, "/api/v1/users/1", `{"accept_collab_invites":false}`, 1)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
		}
		var payload struct {
			User struct {
				AcceptCollabInvites *bool `json:"accept_collab_invites"`
			} `json:"user"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
		}
		if payload.User.AcceptCollabInvites == nil || *payload.User.AcceptCollabInvites {
			t.Fatalf("patch response accept_collab_invites = %v, want false; body = %s", payload.User.AcceptCollabInvites, rec.Body.String())
		}

		rec = collabInviteRequest(t, router, http.MethodPatch, "/api/v1/users/1", `{"accept_collab_invites":true}`, 1)
		if rec.Code != http.StatusOK {
			t.Fatalf("re-enable status = %d, want 200; body = %s", rec.Code, rec.Body.String())
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode re-enable response: %v; body = %s", err, rec.Body.String())
		}
		if payload.User.AcceptCollabInvites == nil || !*payload.User.AcceptCollabInvites {
			t.Fatalf("re-enable response accept_collab_invites = %v, want true; body = %s", payload.User.AcceptCollabInvites, rec.Body.String())
		}
	})

	t.Run("other users cannot update the setting", func(t *testing.T) {
		rec := collabInviteRequest(t, router, http.MethodPatch, "/api/v1/users/1", `{"accept_collab_invites":false}`, 2)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
		}
		if code, _ := decodeCollabInviteError(t, rec); code != "FORBIDDEN" {
			t.Fatalf("code = %q, want FORBIDDEN; body = %s", code, rec.Body.String())
		}
	})

	t.Run("empty body is rejected", func(t *testing.T) {
		rec := collabInviteRequest(t, router, http.MethodPatch, "/api/v1/users/1", `{}`, 1)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
		}
		if code, _ := decodeCollabInviteError(t, rec); code != "NO_FIELDS" {
			t.Fatalf("code = %q, want NO_FIELDS; body = %s", code, rec.Body.String())
		}
	})

	t.Run("me returns the setting", func(t *testing.T) {
		rec := collabInviteRequest(t, router, http.MethodPatch, "/api/v1/users/1", `{"accept_collab_invites":false}`, 1)
		if rec.Code != http.StatusOK {
			t.Fatalf("patch status = %d, want 200; body = %s", rec.Code, rec.Body.String())
		}
		rec = collabInviteRequest(t, router, http.MethodGet, "/api/v1/auth/me", "", 1)
		if rec.Code != http.StatusOK {
			t.Fatalf("me status = %d, want 200; body = %s", rec.Code, rec.Body.String())
		}
		var payload struct {
			User struct {
				AcceptCollabInvites *bool `json:"accept_collab_invites"`
			} `json:"user"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode me response: %v; body = %s", err, rec.Body.String())
		}
		if payload.User.AcceptCollabInvites == nil || *payload.User.AcceptCollabInvites {
			t.Fatalf("me accept_collab_invites = %v, want false; body = %s", payload.User.AcceptCollabInvites, rec.Body.String())
		}
	})
}
