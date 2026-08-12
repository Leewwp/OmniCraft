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

func TestReconcileReadOnlyReportsDriftWithoutWriting(t *testing.T) {
	db := setupReconcileDB(t, true)
	seedReconcileDrift(t, db)
	before := loadReconcileCounts(t, db)
	var stdout, stderr bytes.Buffer

	code := executeCollectionReconcile(nil, db, &stdout, &stderr)

	if code != reconcileDriftExitCode {
		t.Fatalf("exit code = %d, want drift code %d; stderr=%s", code, reconcileDriftExitCode, stderr.String())
	}
	report := decodeReconcileReport(t, stdout.Bytes())
	if report.Apply {
		t.Fatal("read-only report unexpectedly marked apply=true")
	}
	if report.Totals.DuplicateLogicalItems != 0 || report.Totals.MissingDefaultCollections != 1 {
		t.Fatalf("unexpected totals: %#v", report.Totals)
	}
	if len(report.Users) != 2 {
		t.Fatalf("per-user-zone rows = %d, want 2", len(report.Users))
	}
	after := loadReconcileCounts(t, db)
	if before != after {
		t.Fatalf("read-only mode changed row counts: before=%#v after=%#v", before, after)
	}
	if strings.Contains(stdout.String(), "private reconciliation note") || strings.Contains(stdout.String(), "reconcile@example.test") {
		t.Fatalf("output leaked private data: %s", stdout.String())
	}
}

func TestReconcileApplyEnsuresMissingDefaultCollectionsIdempotently(t *testing.T) {
	db := setupReconcileDB(t, true)
	seedReconcileDrift(t, db)
	before := loadReconcileCounts(t, db)
	var stdout, stderr bytes.Buffer

	code := executeCollectionReconcile([]string{"--apply", "--maintenance-window-confirmed"}, db, &stdout, &stderr)

	if code != reconcileCleanExitCode {
		t.Fatalf("apply exit code = %d, want clean; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	report := decodeReconcileReport(t, stdout.Bytes())
	if !report.Apply || report.Totals.HasDrift() {
		t.Fatalf("apply report = %#v, want clean applied report", report)
	}
	if report.Repairs == nil || report.Repairs.DefaultCollectionsCreated != 1 {
		t.Fatalf("apply repairs = %#v, want one default collection created", report.Repairs)
	}
	afterFirst := loadReconcileCounts(t, db)
	if afterFirst.Collections <= before.Collections || afterFirst.CollectionItems != before.CollectionItems {
		t.Fatalf("apply must only add the missing default collection: before=%#v after=%#v", before, afterFirst)
	}
	stdout.Reset()
	stderr.Reset()
	code = executeCollectionReconcile([]string{"--apply", "--maintenance-window-confirmed"}, db, &stdout, &stderr)
	if code != reconcileCleanExitCode {
		t.Fatalf("second apply exit code = %d; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	afterSecond := loadReconcileCounts(t, db)
	if afterFirst != afterSecond {
		t.Fatalf("second apply was not idempotent: first=%#v second=%#v", afterFirst, afterSecond)
	}
}

func TestReconcileApplyRequiresMaintenanceWindowConfirmationWithoutWriting(t *testing.T) {
	db := setupReconcileDB(t, true)
	seedReconcileDrift(t, db)
	before := loadReconcileCounts(t, db)
	var stdout, stderr bytes.Buffer

	code := executeCollectionReconcile([]string{"--apply"}, db, &stdout, &stderr)

	if code != reconcileUsageExitCode {
		t.Fatalf("exit code = %d, want usage code %d", code, reconcileUsageExitCode)
	}
	if !strings.Contains(stderr.String(), "maintenance window") {
		t.Fatalf("stderr = %q, want maintenance-window requirement", stderr.String())
	}
	after := loadReconcileCounts(t, db)
	if before != after {
		t.Fatalf("unconfirmed apply changed data: before=%#v after=%#v", before, after)
	}
}

func TestReconcileApplyRefusesToDeleteDuplicateLogicalItems(t *testing.T) {
	db := setupReconcileDB(t, false)
	user := seedReconcileUser(t, db, 1)
	content := seedReconcileContent(t, db, 100, user.ID, "original")
	first := seedReconcileCollection(t, db, 10, user.ID, "original", true)
	second := seedReconcileCollection(t, db, 11, user.ID, "original", true)
	seedReconcileCollectionItem(t, db, 1000, first.ID, content.ID, "first private note")
	seedReconcileCollectionItem(t, db, 1001, second.ID, content.ID, "second private note")
	var stdout, stderr bytes.Buffer

	code := executeCollectionReconcile([]string{"--apply", "--maintenance-window-confirmed"}, db, &stdout, &stderr)

	if code != reconcileDriftExitCode {
		t.Fatalf("duplicate apply exit code = %d, want drift; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	report := decodeReconcileReport(t, stdout.Bytes())
	if report.Totals.DuplicateLogicalItems != 1 {
		t.Fatalf("duplicate total = %d, want 1", report.Totals.DuplicateLogicalItems)
	}
	var preserved int64
	if err := db.Model(&model.CollectionItem{}).Where("id IN ?", []int64{1000, 1001}).Count(&preserved).Error; err != nil {
		t.Fatalf("count duplicate rows after apply: %v", err)
	}
	if preserved != 2 {
		t.Fatalf("apply deleted duplicate rows: preserved=%d, want 2", preserved)
	}
}

func TestReconcileRejectsUnknownArguments(t *testing.T) {
	db := setupReconcileDB(t, true)
	var stdout, stderr bytes.Buffer

	code := executeCollectionReconcile([]string{"--delete-legacy"}, db, &stdout, &stderr)

	if code != reconcileUsageExitCode {
		t.Fatalf("exit code = %d, want usage code %d", code, reconcileUsageExitCode)
	}
	if !strings.Contains(stderr.String(), "unsupported argument") {
		t.Fatalf("stderr = %q, want unsupported argument", stderr.String())
	}
}

func TestMaintenanceToolingHasNoLegacyFavoritesDependency(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read reconcile main source: %v", err)
	}
	if strings.Contains(strings.ToLower(string(mainSource)), "favorite") {
		t.Fatal("collection-reconcile must not reference legacy favorites after cutover")
	}

	seedSource, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "seed_local_rich_data.py"))
	if err != nil {
		t.Fatalf("read seed script: %v", err)
	}
	if strings.Contains(strings.ToLower(string(seedSource)), "favorite") {
		t.Fatal("seed_local_rich_data.py must not create or count legacy favorites after cutover")
	}
}

func TestLoadCollectionReconcileDSNExplicitEnvWinsOverDotEnv(t *testing.T) {
	t.Setenv("DB_DSN", "host=explicit-db dbname=explicit")
	tempDir := t.TempDir()
	if err := os.WriteFile(tempDir+string(os.PathSeparator)+".env", []byte("DB_DSN=host=dotenv-db dbname=dotenv\n"), 0o600); err != nil {
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

	if got := loadCollectionReconcileDSN(); got != "host=explicit-db dbname=explicit" {
		t.Fatalf("DSN = %q, want explicit process environment", got)
	}
}

type reconcileCounts struct {
	Collections     int64
	CollectionItems int64
}

func setupReconcileDB(t *testing.T, enforceDefaultUniqueness bool) *gorm.DB {
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
	if err := db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.Collection{}, &model.CollectionItem{}); err != nil {
		t.Fatalf("migrate reconcile models: %v", err)
	}
	if enforceDefaultUniqueness {
		if err := db.Exec(`CREATE UNIQUE INDEX idx_collections_one_default_per_zone ON collections (user_id, zone) WHERE is_default = TRUE`).Error; err != nil {
			t.Fatalf("create default unique index: %v", err)
		}
	}
	return db
}

func seedReconcileDrift(t *testing.T, db *gorm.DB) {
	t.Helper()
	user := seedReconcileUser(t, db, 1)
	seedReconcileContent(t, db, 100, user.ID, "original")
	seedReconcileContent(t, db, 102, user.ID, "fanwork")
	originalDefault := seedReconcileCollection(t, db, 10, user.ID, "original", true)
	seedReconcileCollectionItem(t, db, 1000, originalDefault.ID, 101, "private reconciliation note")
}

func seedReconcileUser(t *testing.T, db *gorm.DB, id int64) model.User {
	t.Helper()
	user := model.User{ID: id, Email: "reconcile@example.test", Username: "reconcile-user", PasswordHash: "hash", Role: "user", Reputation: 10}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func seedReconcileContent(t *testing.T, db *gorm.DB, id, authorID int64, zone string) model.ContentItem {
	t.Helper()
	content := model.ContentItem{ID: id, Title: "reconcile content", AuthorID: authorID, Zone: zone, Category: "game", ContentType: "article", Status: "published", IsPublic: true}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("create content: %v", err)
	}
	return content
}

func seedReconcileCollection(t *testing.T, db *gorm.DB, id, userID int64, zone string, isDefault bool) model.Collection {
	t.Helper()
	collection := model.Collection{ID: id, UserID: userID, Title: "default " + zone, Zone: zone, IsDefault: isDefault}
	if err := db.Create(&collection).Error; err != nil {
		t.Fatalf("create collection: %v", err)
	}
	return collection
}

func seedReconcileCollectionItem(t *testing.T, db *gorm.DB, id, collectionID, contentID int64, note string) {
	t.Helper()
	item := model.CollectionItem{ID: id, CollectionID: collectionID, ContentItemID: contentID, Note: note}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create collection item: %v", err)
	}
}

func loadReconcileCounts(t *testing.T, db *gorm.DB) reconcileCounts {
	t.Helper()
	counts := reconcileCounts{}
	if err := db.Model(&model.Collection{}).Count(&counts.Collections).Error; err != nil {
		t.Fatalf("count collections: %v", err)
	}
	if err := db.Model(&model.CollectionItem{}).Count(&counts.CollectionItems).Error; err != nil {
		t.Fatalf("count collection items: %v", err)
	}
	return counts
}

func decodeReconcileReport(t *testing.T, data []byte) reconcileReport {
	t.Helper()
	var report reconcileReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode report: %v; output=%s", err, data)
	}
	return report
}
