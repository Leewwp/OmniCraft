package service

import (
	"os"
	"strings"
	"testing"
	"time"
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
	want := "tag_suggest:10:20:2026-05-07"
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
	if updates := appealTargetUpdates("comment", "approved"); len(updates) != 0 {
		t.Fatalf("comment appeal should not use content updates, got %#v", updates)
	}
	if updates := appealTargetUpdates("content", "rejected"); len(updates) != 0 {
		t.Fatalf("rejected content appeal should not restore target, got %#v", updates)
	}
}

func TestRehabStartedAtMigrationExists(t *testing.T) {
	bytes, err := os.ReadFile("../../migrations/037_rehab_started_at.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := strings.ToLower(string(bytes))
	for _, want := range []string{"started_at", "drop not null"} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration 037 must include %q, got:\n%s", want, sql)
		}
	}
}
