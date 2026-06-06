package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestContentVisibilitySQL_ContainsRequiredClausesAndArgs(t *testing.T) {
	clause, args := ContentVisibilitySQL(42)

	required := []string{
		"content_items.status = ?",
		"content_items.deleted_at IS NULL",
		"is_banned = true",
		"deleted_at IS NOT NULL",
		"content_items.ip_id IS NULL",
		"ips WHERE status = ?",
		"content_items.is_public = ?",
		"content_items.author_id = ?",
	}
	for _, req := range required {
		if !strings.Contains(clause, req) {
			t.Errorf("visibility clause missing required: %q\nGot: %s", req, clause)
		}
	}
	if strings.Contains(clause, "42") {
		t.Fatalf("visibility clause must not interpolate viewer id directly: %s", clause)
	}
	if len(args) != 4 {
		t.Fatalf("args length = %d, want 4: %#v", len(args), args)
	}
	if args[0] != "published" || args[1] != "banned" || args[2] != true || args[3] != int64(42) {
		t.Fatalf("unexpected visibility args: %#v", args)
	}
}

func TestContentVisibilitySQL_AnonymousViewerUsesArgs(t *testing.T) {
	clause, args := ContentVisibilitySQL(0)
	if strings.Contains(clause, "content_items.author_id = 0") {
		t.Fatal("anonymous viewer id must be passed as an argument, not interpolated")
	}
	if len(args) != 4 || args[3] != int64(0) {
		t.Fatalf("anonymous viewer arg mismatch: %#v", args)
	}
}

func TestContentVisibilityScope_ExcludesHiddenContent(t *testing.T) {
	db := setupContentVisibilityTestDB(t)
	viewer := createVisibilityUser(t, db, "viewer@example.com", "viewer", false)
	other := createVisibilityUser(t, db, "other@example.com", "other", false)
	bannedAuthor := createVisibilityUser(t, db, "banned@example.com", "banned", true)
	deletedAuthor := createVisibilityUser(t, db, "deleted@example.com", "deleted", false)
	if err := db.Exec("UPDATE users SET deleted_at = ? WHERE id = ?", time.Now(), deletedAuthor.ID).Error; err != nil {
		t.Fatalf("mark deleted author: %v", err)
	}
	approvedIP := createVisibilityIP(t, db, "approved-ip", "approved")
	bannedIP := createVisibilityIP(t, db, "banned-ip", "banned")

	createVisibilityContent(t, db, "visible public", other.ID, nil, true, nil)
	createVisibilityContent(t, db, "private other", other.ID, nil, false, nil)
	createVisibilityContent(t, db, "private viewer", viewer.ID, nil, false, nil)
	createVisibilityContent(t, db, "banned author", bannedAuthor.ID, nil, true, nil)
	createVisibilityContent(t, db, "deleted author", deletedAuthor.ID, nil, true, nil)
	createVisibilityContent(t, db, "approved ip", other.ID, &approvedIP.ID, true, nil)
	createVisibilityContent(t, db, "banned ip", other.ID, &bannedIP.ID, true, nil)
	createVisibilityContent(t, db, "soft deleted", other.ID, nil, true, ptrTime(time.Now()))

	var results []model.ContentItem
	if err := ApplyContentVisibilityScope(db.Model(&model.ContentItem{}), viewer.ID).
		Order("content_items.title ASC").
		Find(&results).Error; err != nil {
		t.Fatalf("query visible content: %v", err)
	}

	titles := make(map[string]bool, len(results))
	for _, item := range results {
		titles[item.Title] = true
	}
	for _, want := range []string{"approved ip", "private viewer", "visible public"} {
		if !titles[want] {
			t.Fatalf("expected %q in visible results, got %#v", want, titles)
		}
	}
	for _, hidden := range []string{"banned author", "banned ip", "deleted author", "private other", "soft deleted"} {
		if titles[hidden] {
			t.Fatalf("expected %q to be hidden, got %#v", hidden, titles)
		}
	}
}

func TestSearchContentsKeywordAppliesSharedVisibility(t *testing.T) {
	db := setupContentVisibilityTestDB(t)
	viewer := createVisibilityUser(t, db, "search-viewer@example.com", "search-viewer", false)
	other := createVisibilityUser(t, db, "search-other@example.com", "search-other", false)
	bannedAuthor := createVisibilityUser(t, db, "search-banned@example.com", "search-banned", true)
	deletedAuthor := createVisibilityUser(t, db, "search-deleted@example.com", "search-deleted", false)
	if err := db.Exec("UPDATE users SET deleted_at = ? WHERE id = ?", time.Now(), deletedAuthor.ID).Error; err != nil {
		t.Fatalf("mark deleted author: %v", err)
	}
	bannedIP := createVisibilityIP(t, db, "search-banned-ip", "banned")

	createVisibilityContent(t, db, "Needle visible public", other.ID, nil, true, nil)
	createVisibilityContent(t, db, "Needle private viewer", viewer.ID, nil, false, nil)
	createVisibilityContent(t, db, "Needle private other", other.ID, nil, false, nil)
	createVisibilityContent(t, db, "Needle banned author", bannedAuthor.ID, nil, true, nil)
	createVisibilityContent(t, db, "Needle deleted author", deletedAuthor.ID, nil, true, nil)
	createVisibilityContent(t, db, "Needle banned ip", other.ID, &bannedIP.ID, true, nil)
	createVisibilityContent(t, db, "Needle soft deleted", other.ID, nil, true, ptrTime(time.Now()))

	results, total, err := NewSearchRepository(db).SearchContents("Needle", "", "", "", nil, "relevance", 1, 20, viewer.ID)
	if err != nil {
		t.Fatalf("SearchContents: %v", err)
	}

	titles := contentSearchResultTitles(results)
	for _, want := range []string{"Needle private viewer", "Needle visible public"} {
		if !titles[want] {
			t.Fatalf("expected %q in keyword search results, got %#v", want, titles)
		}
	}
	for _, hidden := range []string{"Needle banned author", "Needle banned ip", "Needle deleted author", "Needle private other", "Needle soft deleted"} {
		if titles[hidden] {
			t.Fatalf("expected %q to be hidden from keyword search, got %#v", hidden, titles)
		}
	}
	if total != int64(len(results)) || total != 2 {
		t.Fatalf("total = %d, results = %d, want 2 visible keyword matches", total, len(results))
	}
}

func TestSearchSuggestionsApplySharedVisibility(t *testing.T) {
	db := setupContentVisibilityTestDB(t)
	viewer := createVisibilityUser(t, db, "suggest-viewer@example.com", "suggest-viewer", false)
	other := createVisibilityUser(t, db, "suggest-other@example.com", "suggest-other", false)
	bannedAuthor := createVisibilityUser(t, db, "suggest-banned@example.com", "suggest-banned", true)
	bannedIP := createVisibilityIP(t, db, "suggest-banned-ip", "banned")

	createVisibilityContent(t, db, "Needle visible public", other.ID, nil, true, nil)
	createVisibilityContent(t, db, "Needle private viewer", viewer.ID, nil, false, nil)
	createVisibilityContent(t, db, "Needle private other", other.ID, nil, false, nil)
	createVisibilityContent(t, db, "Needle banned author", bannedAuthor.ID, nil, true, nil)
	createVisibilityContent(t, db, "Needle banned ip", other.ID, &bannedIP.ID, true, nil)
	createVisibilityContent(t, db, "Needle soft deleted", other.ID, nil, true, ptrTime(time.Now()))

	suggestions, err := NewSearchRepository(db).SearchSuggestions("Needle", 20, viewer.ID)
	if err != nil {
		t.Fatalf("SearchSuggestions: %v", err)
	}

	texts := searchSuggestionTexts(suggestions)
	for _, want := range []string{"Needle private viewer", "Needle visible public"} {
		if !texts[want] {
			t.Fatalf("expected %q in suggestions, got %#v", want, texts)
		}
	}
	for _, hidden := range []string{"Needle banned author", "Needle banned ip", "Needle private other", "Needle soft deleted"} {
		if texts[hidden] {
			t.Fatalf("expected %q to be hidden from suggestions, got %#v", hidden, texts)
		}
	}

	anonymousSuggestions, err := NewSearchRepository(db).SearchSuggestions("Needle", 20, 0)
	if err != nil {
		t.Fatalf("anonymous SearchSuggestions: %v", err)
	}
	anonymousTexts := searchSuggestionTexts(anonymousSuggestions)
	if anonymousTexts["Needle private viewer"] {
		t.Fatalf("anonymous suggestions must not include viewer-private content: %#v", anonymousTexts)
	}
	if !anonymousTexts["Needle visible public"] {
		t.Fatalf("anonymous suggestions should still include public content: %#v", anonymousTexts)
	}
}

func TestSearchSuggestionsDoesNotFilterTagsByMissingStatus(t *testing.T) {
	data, err := os.ReadFile("search_repo.go")
	if err != nil {
		t.Fatalf("read search repo: %v", err)
	}
	sql := string(data)
	if strings.Contains(sql, "FROM tags WHERE name ILIKE ? AND status") {
		t.Fatal("SearchSuggestions must not filter tags by tags.status; the tags table has no status column")
	}
}

func setupContentVisibilityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentTag{}, &model.Tag{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, stmt := range []string{
		"ALTER TABLE users ADD COLUMN deleted_at DATETIME",
		"ALTER TABLE content_items ADD COLUMN deleted_at DATETIME",
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("schema patch %q: %v", stmt, err)
		}
	}
	return db
}

func createVisibilityUser(t *testing.T, db *gorm.DB, email, username string, banned bool) model.User {
	t.Helper()
	user := model.User{
		Email:        email,
		Username:     username,
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
		IsBanned:     banned,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return user
}

func createVisibilityIP(t *testing.T, db *gorm.DB, slug, status string) model.IP {
	t.Helper()
	ip := model.IP{Name: slug, Slug: slug, Status: status}
	if err := db.Create(&ip).Error; err != nil {
		t.Fatalf("create ip %s: %v", slug, err)
	}
	return ip
}

func createVisibilityContent(t *testing.T, db *gorm.DB, title string, authorID int64, ipID *int64, public bool, deletedAt *time.Time) model.ContentItem {
	t.Helper()
	item := model.ContentItem{
		Title:       title,
		AuthorID:    authorID,
		IPID:        ipID,
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		Status:      "published",
		IsPublic:    public,
		AllowCopy:   true,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create content %s: %v", title, err)
	}
	if !public {
		if err := db.Exec("UPDATE content_items SET is_public = ? WHERE id = ?", false, item.ID).Error; err != nil {
			t.Fatalf("mark private content %s: %v", title, err)
		}
	}
	if deletedAt != nil {
		if err := db.Exec("UPDATE content_items SET deleted_at = ? WHERE id = ?", *deletedAt, item.ID).Error; err != nil {
			t.Fatalf("soft delete content %s: %v", title, err)
		}
	}
	return item
}

func ptrTime(v time.Time) *time.Time {
	return &v
}

func contentSearchResultTitles(results []ContentSearchResult) map[string]bool {
	titles := make(map[string]bool, len(results))
	for _, item := range results {
		titles[item.Title] = true
	}
	return titles
}

func searchSuggestionTexts(results []SearchSuggestion) map[string]bool {
	texts := make(map[string]bool, len(results))
	for _, item := range results {
		texts[item.Text] = true
	}
	return texts
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

func TestReportUpdateStatusPersistsActionTaken(t *testing.T) {
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
