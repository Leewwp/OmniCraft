package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
)

const testAuditDomain = "https://cdn.example.test"

func setupAvatarAuditDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	return db
}

func seedAvatarAuditUsers(t *testing.T, db *gorm.DB) {
	t.Helper()
	users := []model.User{
		{Email: "external@example.test", Username: "external", AvatarURL: "https://gravatar.example.com/face.png"},
		{Email: "external-2@example.test", Username: "external2", AvatarURL: "https://evil.example.net/tracker.gif"},
		{Email: "platform@example.test", Username: "platform", AvatarURL: testAuditDomain + "/uploads/7/avatar/2026/08/13/ok.png"},
		{Email: "empty@example.test", Username: "empty", AvatarURL: ""},
		{Email: "null@example.test", Username: "null"},
		{Email: "soft-deleted@example.test", Username: "soft-deleted", AvatarURL: "https://gravatar.example.com/old.png"},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatalf("create audit user %d: %v", i, err)
		}
	}
	if err := db.Model(&model.User{}).Where("username = ?", "soft-deleted").Update("deleted_at", "2026-08-13T00:00:00Z").Error; err != nil {
		t.Fatalf("soft-delete user: %v", err)
	}
}

func loadAvatarAuditValues(t *testing.T, db *gorm.DB) map[string]string {
	t.Helper()
	rows := []struct {
		Username  string
		AvatarURL string
	}{}
	if err := db.Table("users").Select("username, avatar_url").Scan(&rows).Error; err != nil {
		t.Fatalf("load avatar values: %v", err)
	}
	values := make(map[string]string, len(rows))
	for _, row := range rows {
		values[row.Username] = row.AvatarURL
	}
	return values
}

func decodeAvatarAuditReport(t *testing.T, data []byte) avatarAuditReport {
	t.Helper()
	var report avatarAuditReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode report: %v; output=%s", err, data)
	}
	return report
}

func TestAvatarAuditReadOnlyReportsExternalAvatarsWithoutWriting(t *testing.T) {
	db := setupAvatarAuditDB(t)
	seedAvatarAuditUsers(t, db)
	before := loadAvatarAuditValues(t, db)
	var stdout, stderr bytes.Buffer

	code := executeAvatarAudit(nil, db, testAuditDomain, &stdout, &stderr)

	if code != avatarAuditDriftExitCode {
		t.Fatalf("exit code = %d, want drift code %d; stderr=%s", code, avatarAuditDriftExitCode, stderr.String())
	}
	report := decodeAvatarAuditReport(t, stdout.Bytes())
	if report.Apply {
		t.Fatal("read-only report unexpectedly marked apply=true")
	}
	if report.Totals.ExternalAvatarUsers != 2 {
		t.Fatalf("external avatar total = %d, want 2 (platform/empty/null/soft-deleted excluded)", report.Totals.ExternalAvatarUsers)
	}
	if len(report.Users) != 2 {
		t.Fatalf("reported users = %d, want 2", len(report.Users))
	}
	after := loadAvatarAuditValues(t, db)
	if !equalStringMaps(before, after) {
		t.Fatalf("read-only mode changed avatar values: before=%v after=%v", before, after)
	}
	if !strings.Contains(stdout.String(), "gravatar.example.com") {
		t.Fatalf("audit output must include the offending URL for review: %s", stdout.String())
	}
}

func TestAvatarAuditApplyDowngradesExternalAvatarsIdempotently(t *testing.T) {
	db := setupAvatarAuditDB(t)
	seedAvatarAuditUsers(t, db)
	var stdout, stderr bytes.Buffer

	code := executeAvatarAudit([]string{"--apply", "--maintenance-window-confirmed"}, db, testAuditDomain, &stdout, &stderr)

	if code != avatarAuditCleanExitCode {
		t.Fatalf("apply exit code = %d, want clean; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	report := decodeAvatarAuditReport(t, stdout.Bytes())
	if !report.Apply || report.Totals.HasDrift() {
		t.Fatalf("apply report = %#v, want clean applied report", report)
	}
	if report.Repairs == nil || report.Repairs.DowngradedToDefault != 2 {
		t.Fatalf("apply repairs = %#v, want two downgrades", report.Repairs)
	}
	values := loadAvatarAuditValues(t, db)
	if values["external"] != "" || values["external-2"] != "" {
		t.Fatalf("external avatars must be downgraded to the default: %v", values)
	}
	if values["platform"] != testAuditDomain+"/uploads/7/avatar/2026/08/13/ok.png" {
		t.Fatalf("platform OSS avatar must be preserved: %v", values)
	}
	if values["soft-deleted"] == "" {
		t.Fatalf("soft-deleted user avatar must not be touched: %v", values)
	}

	stdout.Reset()
	stderr.Reset()
	code = executeAvatarAudit([]string{"--apply", "--maintenance-window-confirmed"}, db, testAuditDomain, &stdout, &stderr)
	if code != avatarAuditCleanExitCode {
		t.Fatalf("second apply exit code = %d; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	after := loadAvatarAuditValues(t, db)
	if !equalStringMaps(values, after) {
		t.Fatalf("second apply was not idempotent: first=%v second=%v", values, after)
	}
}

func TestAvatarAuditApplyRequiresMaintenanceWindowConfirmationWithoutWriting(t *testing.T) {
	db := setupAvatarAuditDB(t)
	seedAvatarAuditUsers(t, db)
	before := loadAvatarAuditValues(t, db)
	var stdout, stderr bytes.Buffer

	code := executeAvatarAudit([]string{"--apply"}, db, testAuditDomain, &stdout, &stderr)

	if code != avatarAuditUsageExitCode {
		t.Fatalf("exit code = %d, want usage code %d", code, avatarAuditUsageExitCode)
	}
	if !strings.Contains(stderr.String(), "maintenance window") {
		t.Fatalf("stderr = %q, want maintenance-window requirement", stderr.String())
	}
	after := loadAvatarAuditValues(t, db)
	if !equalStringMaps(before, after) {
		t.Fatalf("unconfirmed apply changed data: before=%v after=%v", before, after)
	}
}

func TestAvatarAuditRejectsUnknownArguments(t *testing.T) {
	db := setupAvatarAuditDB(t)
	var stdout, stderr bytes.Buffer

	code := executeAvatarAudit([]string{"--nuke"}, db, testAuditDomain, &stdout, &stderr)

	if code != avatarAuditUsageExitCode {
		t.Fatalf("exit code = %d, want usage code %d", code, avatarAuditUsageExitCode)
	}
	if !strings.Contains(stderr.String(), "unsupported argument") {
		t.Fatalf("stderr = %q, want unsupported argument", stderr.String())
	}
}

func TestAvatarAuditTreatsEmptyDomainAsNoVerifiablePlatform(t *testing.T) {
	db := setupAvatarAuditDB(t)
	seedAvatarAuditUsers(t, db)
	var stdout, stderr bytes.Buffer

	code := executeAvatarAudit(nil, db, "", &stdout, &stderr)

	if code != avatarAuditDriftExitCode {
		t.Fatalf("exit code = %d, want drift code %d; stderr=%s", code, avatarAuditDriftExitCode, stderr.String())
	}
	report := decodeAvatarAuditReport(t, stdout.Bytes())
	if report.Totals.ExternalAvatarUsers != 3 {
		t.Fatalf("empty-domain external total = %d, want 3 (platform URL is unverifiable without a domain)", report.Totals.ExternalAvatarUsers)
	}
}

func TestLoadAvatarAuditDSNExplicitEnvWinsOverDotEnv(t *testing.T) {
	t.Setenv("DB_DSN", "host=explicit-db dbname=explicit")
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte("DB_DSN=host=dotenv-db dbname=dotenv\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if restoreErr := os.Chdir(previousDir); restoreErr != nil {
			t.Errorf("restore working directory: %v", restoreErr)
		}
	})

	if got := loadAvatarAuditDSN(); got != "host=explicit-db dbname=explicit" {
		t.Fatalf("DSN = %q, want explicit process environment", got)
	}
}

func equalStringMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
