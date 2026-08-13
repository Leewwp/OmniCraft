package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/testutil"
)

const testPlatformAvatarURL = "https://cdn.example.test/uploads/7/avatar/2026/08/13/avatar.png"

// fakeAvatarReviewer records every image passed to the moderation seam and
// returns a scripted result or error, so handler tests assert both the gate
// semantics (blocked / unavailable / unconfigured) and that external URLs
// never reach the scanner.
type fakeAvatarReviewer struct {
	result string
	err    error
	calls  []string
}

func (f *fakeAvatarReviewer) ReviewImageURL(_ context.Context, imageURL string) (string, error) {
	f.calls = append(f.calls, imageURL)
	if f.err != nil {
		return "", f.err
	}
	return f.result, nil
}

// setupUserUpdateTest wires the PATCH /api/v1/users/:id route backed by an
// ephemeral Postgres users table and an optional avatar reviewer, mirroring
// the production wiring in routes.go.
func setupUserUpdateTest(t *testing.T, mode, ossDomain string, reviewer avatarReviewer) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	createUserUpdateBaseSchema(t, db)

	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })

	cfg := &config.Config{
		Server: config.ServerConfig{Mode: mode},
		OSS:    config.OSSConfig{Domain: ossDomain},
		JWT:    config.JWTConfig{Secret: "user-update-test-secret"},
	}
	userHandler := NewUserHandler(db, nil, rdb, cfg, reviewer)

	router := gin.New()
	router.PATCH("/api/v1/users/:id", func(c *gin.Context) {
		if raw := c.GetHeader("X-Test-User-ID"); raw != "" {
			if userID, err := strconv.ParseInt(raw, 10, 64); err == nil {
				c.Set(middleware.UserIDKey, userID)
			}
		}
	}, userHandler.UpdateUser)
	return router, db
}

func createUserUpdateBaseSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			username VARCHAR(64) UNIQUE NOT NULL,
			avatar_url TEXT,
			bio TEXT,
			reputation INT NOT NULL DEFAULT 10,
			preferred_locale VARCHAR(10) NOT NULL DEFAULT 'zh-CN',
			support_info JSONB NOT NULL DEFAULT '{}',
			role VARCHAR(20) NOT NULL DEFAULT 'user',
			is_banned BOOLEAN NOT NULL DEFAULT FALSE,
			ban_reason TEXT,
			email_verified_at TIMESTAMPTZ,
			accept_collab_invites BOOLEAN NOT NULL DEFAULT TRUE,
			accepted_terms_version VARCHAR(32),
			accepted_terms_at TIMESTAMPTZ,
			accepted_privacy_version VARCHAR(32),
			accepted_privacy_at TIMESTAMPTZ,
			deleted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`).Error)
}

func createUserUpdateTestUser(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	user := model.User{Email: "avatar@example.test", Username: "avatar-user", PasswordHash: "hash", Reputation: 10, Role: "user"}
	require.NoError(t, db.Create(&user).Error)
	return user.ID
}

func patchUserAvatar(t *testing.T, router *gin.Engine, userID int64, avatarURL string) *httptest.ResponseRecorder {
	t.Helper()
	return patchUserJSON(t, router, userID, fmt.Sprintf(`{"avatar_url":%q}`, avatarURL))
}

func patchUserJSON(t *testing.T, router *gin.Engine, userID int64, payload string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/users/%d", userID), strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", strconv.FormatInt(userID, 10))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func requireAvatarStored(t *testing.T, db *gorm.DB, userID int64, want string) {
	t.Helper()
	var stored struct {
		AvatarURL string
	}
	require.NoError(t, db.Table("users").Select("avatar_url").Where("id = ?", userID).Scan(&stored).Error)
	require.Equal(t, want, stored.AvatarURL)
}

func captureSlog(t *testing.T, fn func()) []string {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(previous)
	fn()
	return strings.Split(buf.String(), "\n")
}

func TestUpdateUserAvatarRejectsExternalURL(t *testing.T) {
	reviewer := &fakeAvatarReviewer{result: "pass"}
	router, db := setupUserUpdateTest(t, "debug", "https://cdn.example.test", reviewer)
	userID := createUserUpdateTestUser(t, db)

	rec := patchUserAvatar(t, router, userID, "https://evil.example.com/tracker.png")

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "AVATAR_NOT_PLATFORM_OSS_OBJECT", body["code"])
	require.NotContains(t, rec.Body.String(), "evil.example.com", "external URL must not leak into the client response")
	require.Empty(t, reviewer.calls, "external URL must never reach the image scanner")
	requireAvatarStored(t, db, userID, "")
}

func TestUpdateUserAvatarRejectsEveryURLWhenDomainUnset(t *testing.T) {
	reviewer := &fakeAvatarReviewer{result: "pass"}
	router, db := setupUserUpdateTest(t, "debug", "", reviewer)
	userID := createUserUpdateTestUser(t, db)

	rec := patchUserAvatar(t, router, userID, testPlatformAvatarURL)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "AVATAR_NOT_PLATFORM_OSS_OBJECT", body["code"])
	require.Empty(t, reviewer.calls, "without a configured delivery domain no URL is verifiable")
	requireAvatarStored(t, db, userID, "")
}

func TestUpdateUserAvatarRejectsBlockedImage(t *testing.T) {
	reviewer := &fakeAvatarReviewer{result: "block"}
	router, db := setupUserUpdateTest(t, "debug", "https://cdn.example.test", reviewer)
	userID := createUserUpdateTestUser(t, db)

	rec := patchUserAvatar(t, router, userID, testPlatformAvatarURL)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "AVATAR_BLOCKED", body["code"])
	require.Equal(t, []string{testPlatformAvatarURL}, reviewer.calls)
	requireAvatarStored(t, db, userID, "")
}

func TestUpdateUserAvatarAllowsPassAndReview(t *testing.T) {
	for _, result := range []string{"pass", "review"} {
		t.Run(result, func(t *testing.T) {
			reviewer := &fakeAvatarReviewer{result: result}
			router, db := setupUserUpdateTest(t, "debug", "https://cdn.example.test", reviewer)
			userID := createUserUpdateTestUser(t, db)

			rec := patchUserAvatar(t, router, userID, testPlatformAvatarURL)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, []string{testPlatformAvatarURL}, reviewer.calls)
			requireAvatarStored(t, db, userID, testPlatformAvatarURL)
		})
	}
}

func TestUpdateUserAvatarFailOpenInLocalModeWhenGreenNotConfigured(t *testing.T) {
	reviewer := &fakeAvatarReviewer{err: aliyun.ErrGreenNotConfigured}
	router, db := setupUserUpdateTest(t, "debug", "https://cdn.example.test", reviewer)
	userID := createUserUpdateTestUser(t, db)

	var rec *httptest.ResponseRecorder
	logs := captureSlog(t, func() {
		rec = patchUserAvatar(t, router, userID, testPlatformAvatarURL)
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	requireAvatarStored(t, db, userID, testPlatformAvatarURL)
	require.Contains(t, strings.Join(logs, "\n"), "policy=fail_open")
	require.Contains(t, strings.Join(logs, "\n"), "reason=green_not_configured")
}

func TestUpdateUserAvatarFailOpenInLocalModeWhenReviewerNotWired(t *testing.T) {
	router, db := setupUserUpdateTest(t, "debug", "https://cdn.example.test", nil)
	userID := createUserUpdateTestUser(t, db)

	var rec *httptest.ResponseRecorder
	logs := captureSlog(t, func() {
		rec = patchUserAvatar(t, router, userID, testPlatformAvatarURL)
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	requireAvatarStored(t, db, userID, testPlatformAvatarURL)
	require.Contains(t, strings.Join(logs, "\n"), "policy=fail_open")
	require.Contains(t, strings.Join(logs, "\n"), "reason=review_service_not_wired")
}

func TestUpdateUserAvatarFailClosedInReleaseModeWhenModerationFails(t *testing.T) {
	reviewer := &fakeAvatarReviewer{err: errors.New("scan backend down")}
	router, db := setupUserUpdateTest(t, "release", "https://cdn.example.test", reviewer)
	userID := createUserUpdateTestUser(t, db)

	var rec *httptest.ResponseRecorder
	logs := captureSlog(t, func() {
		rec = patchUserAvatar(t, router, userID, testPlatformAvatarURL)
	})

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "MODERATION_UNAVAILABLE", body["code"])
	require.NotContains(t, rec.Body.String(), "scan backend down", "raw error must never reach the client")
	requireAvatarStored(t, db, userID, "")
	require.Contains(t, strings.Join(logs, "\n"), "policy=fail_closed")
}

func TestUpdateUserAvatarFailClosedInReleaseModeWhenReviewerNotWired(t *testing.T) {
	router, db := setupUserUpdateTest(t, "release", "https://cdn.example.test", nil)
	userID := createUserUpdateTestUser(t, db)

	var rec *httptest.ResponseRecorder
	logs := captureSlog(t, func() {
		rec = patchUserAvatar(t, router, userID, testPlatformAvatarURL)
	})

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "MODERATION_UNAVAILABLE", body["code"])
	requireAvatarStored(t, db, userID, "")
	require.Contains(t, strings.Join(logs, "\n"), "policy=fail_closed")
	require.Contains(t, strings.Join(logs, "\n"), "reason=review_service_not_wired")
}

func TestUpdateUserAvatarAllowsClearingToDefaultWithoutModeration(t *testing.T) {
	reviewer := &fakeAvatarReviewer{result: "block"}
	router, db := setupUserUpdateTest(t, "debug", "https://cdn.example.test", reviewer)
	userID := createUserUpdateTestUser(t, db)
	require.NoError(t, db.Table("users").Where("id = ?", userID).Update("avatar_url", testPlatformAvatarURL).Error)

	rec := patchUserAvatar(t, router, userID, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Empty(t, reviewer.calls, "clearing the avatar must not trigger a scan")
	requireAvatarStored(t, db, userID, "")
}

func TestUpdateUserAvatarSkipsModerationWhenFieldAbsent(t *testing.T) {
	reviewer := &fakeAvatarReviewer{result: "block"}
	router, db := setupUserUpdateTest(t, "debug", "https://cdn.example.test", reviewer)
	userID := createUserUpdateTestUser(t, db)

	rec := patchUserJSON(t, router, userID, `{"username":"renamed"}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Empty(t, reviewer.calls)
	var stored struct {
		Username string
	}
	require.NoError(t, db.Table("users").Select("username").Where("id = ?", userID).Scan(&stored).Error)
	require.Equal(t, "renamed", stored.Username)
}
