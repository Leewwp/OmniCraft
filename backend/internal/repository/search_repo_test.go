package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

func TestToTSQuery_SplitsWords(t *testing.T) {
	result := toTSQuery("hello world")
	if !strings.Contains(result, "hello:*") || !strings.Contains(result, "world:*") {
		t.Errorf("expected both words with prefix match, got %q", result)
	}
	if !strings.Contains(result, "&") {
		t.Error("expected AND operator between words")
	}
}

func TestToTSQuery_SingleWord(t *testing.T) {
	result := toTSQuery("穿搭")
	if result != "穿搭:*" {
		t.Errorf("expected single word with prefix match, got %q", result)
	}
}

func TestToTSQuery_ChineseSubstring(t *testing.T) {
	result := toTSQuery("春日穿搭")
	if result != "春日穿搭:*" {
		t.Errorf("expected full Chinese string with prefix match, got %q", result)
	}
}

func TestSplitAndNormalize_SplitsOnSpecial(t *testing.T) {
	words := splitAndNormalize("hello & world | test")
	if len(words) != 3 {
		t.Fatalf("expected 3 words, got %d: %v", len(words), words)
	}
	if words[0] != "hello" || words[1] != "world" || words[2] != "test" {
		t.Errorf("unexpected split result: %v", words)
	}
}

func TestSplitAndNormalize_ChineseNoSplit(t *testing.T) {
	words := splitAndNormalize("春日穿搭指南")
	if len(words) != 1 {
		t.Fatalf("expected 1 word for Chinese without spaces, got %d: %v", len(words), words)
	}
	if words[0] != "春日穿搭指南" {
		t.Errorf("expected full Chinese string, got %q", words[0])
	}
}

func TestSplitAndNormalize_ChineseWithSpaces(t *testing.T) {
	words := splitAndNormalize("春日 穿搭")
	if len(words) != 2 {
		t.Fatalf("expected 2 words for Chinese with spaces, got %d: %v", len(words), words)
	}
}

func TestContentVisibilityWhere_ContainsRequiredClauses(t *testing.T) {
	clause := ContentVisibilityWhere(42)

	required := []string{
		"content_items.status = 'published'",
		"content_items.deleted_at IS NULL",
		"is_banned = true",
		"deleted_at IS NOT NULL",
		"content_items.is_public = true",
		"content_items.author_id = 42",
	}
	for _, req := range required {
		if !strings.Contains(clause, req) {
			t.Errorf("visibility clause missing required: %q\nGot: %s", req, clause)
		}
	}
}

func TestContentVisibilityWhere_AnonymousViewer(t *testing.T) {
	clause := ContentVisibilityWhere(0)
	if !strings.Contains(clause, "content_items.author_id = 0") {
		t.Error("anonymous viewer should have author_id = 0, meaning only public content is visible")
	}
}

func TestApplyContentVisibilityScope_ReturnsGormQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB-dependent test in short mode")
	}
}

func TestWebBetaReviewRepairMigrationAddsFeedbackAndReportSchema(t *testing.T) {
	migrationPath := filepath.Join("..", "..", "migrations", "053_web_beta_review_repairs.sql")
	data, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read repair migration: %v", err)
	}
	sql := string(data)

	required := []string{
		"ADD COLUMN IF NOT EXISTS action_taken",
		"feedback_tickets_status_check",
		"'reopened'",
	}
	for _, req := range required {
		if !strings.Contains(sql, req) {
			t.Fatalf("repair migration missing %q:\n%s", req, sql)
		}
	}
}

func TestUpdateReportStatusPersistsActionTaken(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Report{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	reporter := model.User{
		Email:        "reporter@example.com",
		Username:     "reporter",
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
	}
	if err := db.Create(&reporter).Error; err != nil {
		t.Fatalf("create reporter: %v", err)
	}
	report := model.Report{
		ReporterID: reporter.ID,
		TargetType: "content",
		TargetID:   100,
		Reason:     "spam",
		Status:     "pending",
	}
	if err := db.Create(&report).Error; err != nil {
		t.Fatalf("create report: %v", err)
	}

	repo := NewSearchRepository(db)
	if err := repo.UpdateReportStatus(report.ID, "resolved", "content hidden"); err != nil {
		t.Fatalf("UpdateReportStatus: %v", err)
	}

	var updated model.Report
	if err := db.First(&updated, report.ID).Error; err != nil {
		t.Fatalf("load report: %v", err)
	}
	if updated.Status != "resolved" {
		t.Fatalf("status = %q, want resolved", updated.Status)
	}
	if updated.ActionTaken != "content hidden" {
		t.Fatalf("action_taken = %q, want content hidden", updated.ActionTaken)
	}
}
