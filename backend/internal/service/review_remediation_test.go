package service

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/internal/testutil"
)

func TestLLMAPIKeyEncryptionRoundTripDoesNotStorePlaintext(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")

	encrypted, err := encryptLLMAPIKey("sk-secret-value")
	if err != nil {
		t.Fatalf("encryptLLMAPIKey: %v", err)
	}
	if encrypted == "sk-secret-value" || strings.Contains(encrypted, "sk-secret-value") {
		t.Fatalf("encrypted API key must not contain plaintext, got %q", encrypted)
	}

	decrypted, err := decryptLLMAPIKey(encrypted)
	if err != nil {
		t.Fatalf("decryptLLMAPIKey: %v", err)
	}
	if decrypted != "sk-secret-value" {
		t.Fatalf("decryptLLMAPIKey() = %q, want original key", decrypted)
	}
}

func TestRefreshTokenStorageKeyDoesNotExposeRawToken(t *testing.T) {
	key := buildRefreshTokenKey(42, "raw-refresh-token")
	if !strings.HasPrefix(key, "refresh_token:42:") {
		t.Fatalf("refresh token key = %q, want user scoped refresh_token prefix", key)
	}
	if strings.Contains(key, "raw-refresh-token") {
		t.Fatalf("refresh token key must not expose raw token: %q", key)
	}
}

func TestTagSuggestionRateLimitKeyScopesUserContentAndDate(t *testing.T) {
	date := time.Date(2026, 5, 7, 1, 2, 3, 0, time.UTC)
	got := buildTagSuggestRateLimitKey(10, 20, date)
	want := "tag:suggest:10:20:2026-05-07"
	if got != want {
		t.Fatalf("buildTagSuggestRateLimitKey() = %q, want %q", got, want)
	}
}

func TestCanCompleteRehabCourseRequiresStartedAndElapsedTime(t *testing.T) {
	now := time.Date(2026, 5, 7, 1, 2, 3, 0, time.UTC)
	started := now.Add(-3 * time.Minute)

	if err := canCompleteRehabCourse(nil, 60, now); err == nil {
		t.Fatalf("canCompleteRehabCourse without start = nil, want error")
	}
	if err := canCompleteRehabCourse(&started, 240, now); err == nil {
		t.Fatalf("canCompleteRehabCourse before minimum read time = nil, want error")
	}
	if err := canCompleteRehabCourse(&started, 180, now); err != nil {
		t.Fatalf("canCompleteRehabCourse after minimum read time = %v, want nil", err)
	}
}

func TestApprovedAppealRestoresContentStatus(t *testing.T) {
	updates := appealTargetUpdates("content", "approved")
	if updates["status"] != "published" {
		t.Fatalf("approved content appeal updates = %#v, want published status", updates)
	}
	// T31（FIX-27）：comment approved 恢复 hidden → published（行为变更）。
	if updates := appealTargetUpdates("comment", "approved"); updates["status"] != "published" {
		t.Fatalf("approved comment appeal should restore published status, got %#v", updates)
	}
	if updates := appealTargetUpdates("content", "rejected"); len(updates) != 0 {
		t.Fatalf("rejected content appeal should not restore target, got %#v", updates)
	}
}

func TestRehabStartedAtMigrationBackfillsColumnAndRelaxesNullability(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	requireRehabCompletionsBaseTable(t, db)

	completedAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	if err := db.Exec(`INSERT INTO rehab_completions (completed_at) VALUES (?)`, completedAt).Error; err != nil {
		t.Fatalf("seed rehab completion: %v", err)
	}

	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "037_rehab_started_at.sql"))

	dataType, nullable := testutil.ColumnMetadata(t, db, "rehab_completions", "started_at")
	if dataType != "timestamp with time zone" || !nullable {
		t.Fatalf("started_at column = (%s, nullable=%v), want timestamptz nullable", dataType, nullable)
	}

	_, nullable = testutil.ColumnMetadata(t, db, "rehab_completions", "completed_at")
	if !nullable {
		t.Fatal("completed_at should be nullable after migration 037")
	}

	var row struct {
		StartedAt   time.Time
		CompletedAt *time.Time
	}
	if err := db.Raw(`SELECT started_at, completed_at FROM rehab_completions LIMIT 1`).Scan(&row).Error; err != nil {
		t.Fatalf("load rehab completion: %v", err)
	}
	if !row.StartedAt.Equal(completedAt) {
		t.Fatalf("started_at = %s, want %s", row.StartedAt, completedAt)
	}
	if row.CompletedAt == nil || !row.CompletedAt.Equal(completedAt) {
		t.Fatalf("completed_at = %v, want %s", row.CompletedAt, completedAt)
	}
}

func requireRehabCompletionsBaseTable(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec(`
		CREATE TABLE rehab_completions (
			id BIGSERIAL PRIMARY KEY,
			completed_at TIMESTAMPTZ NOT NULL
		)
	`).Error; err != nil {
		t.Fatalf("create rehab_completions base table: %v", err)
	}
}
