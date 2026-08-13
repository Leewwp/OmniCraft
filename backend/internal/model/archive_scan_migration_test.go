package model

import (
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"omnicraft/backend/internal/testutil"
)

// TestArchiveScanMigration covers the empty-database upgrade path for 072
// (archive malware scanning S01, issue #146): the five new scan columns on
// content_attachments, the legacy_unscanned backfill for existing mod
// attachments, the archive_scan_jobs / archive_scan_attempts tables with
// their FK chains, the partial unique "one current job per (attachment_id,
// scan_version)" index and the immutable attempt audit log contract
// (monotonic attempt_no uniqueness, append-only semantics).
func TestArchiveScanMigration(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	requireArchiveScanBaseTables(t, db)

	// Seed rows BEFORE the migration so the legacy_unscanned backfill has
	// real pre-migration data to classify: a mod attachment and a non-mod
	// (image) attachment.
	modID := seedPreMigrationAttachment(t, db, "uploads/1/mod/legacy.zip", "mod")
	imageID := seedPreMigrationAttachment(t, db, "uploads/1/image/legacy.png", "image")

	migration := filepath.Join("..", "..", "migrations", "072_archive_malware_scanning.sql")
	testutil.ApplyMigrationFile(t, db, migration)
	testutil.ApplyMigrationFile(t, db, migration)

	// --- content_attachments column contract ---
	assertArchiveScanColumn(t, db, "content_attachments", "scan_status", "character varying", false)
	assertArchiveScanColumn(t, db, "content_attachments", "scan_required", "boolean", false)
	assertArchiveScanColumn(t, db, "content_attachments", "scan_version", "integer", false)
	assertArchiveScanColumn(t, db, "content_attachments", "last_scan_job_id", "bigint", true)
	assertArchiveScanColumn(t, db, "content_attachments", "scanned_at", "timestamp with time zone", true)

	// --- legacy_unscanned backfill: pre-migration mod rows are marked, non-mod rows are not ---
	assertScanStatus(t, db, modID, ScanStatusLegacyUnscanned)
	assertScanStatus(t, db, imageID, ScanStatusNotRequired)

	// Rows inserted after the migration default to not_required.
	postID := seedPostMigrationAttachment(t, db, "uploads/1/text/legacy.txt", "text")
	assertScanStatus(t, db, postID, ScanStatusNotRequired)

	// --- archive_scan_jobs table + column contract ---
	assertArchiveScanTableExists(t, db, "archive_scan_jobs")
	assertArchiveScanColumn(t, db, "archive_scan_jobs", "id", "bigint", false)
	assertArchiveScanColumn(t, db, "archive_scan_jobs", "created_at", "timestamp with time zone", false)
	assertArchiveScanColumn(t, db, "archive_scan_jobs", "attachment_id", "bigint", false)
	assertArchiveScanColumn(t, db, "archive_scan_jobs", "scan_version", "integer", false)
	assertArchiveScanColumn(t, db, "archive_scan_jobs", "status", "character varying", false)
	assertArchiveScanColumn(t, db, "archive_scan_jobs", "attempts", "integer", false)
	assertArchiveScanColumn(t, db, "archive_scan_jobs", "next_attempt_at", "timestamp with time zone", true)
	assertArchiveScanColumn(t, db, "archive_scan_jobs", "object_sha256", "character varying", true)
	assertArchiveScanColumn(t, db, "archive_scan_jobs", "engine_version", "character varying", true)
	assertArchiveScanColumn(t, db, "archive_scan_jobs", "signature_version", "character varying", true)
	assertArchiveScanColumn(t, db, "archive_scan_jobs", "detection_name", "character varying", true)
	assertArchiveScanColumn(t, db, "archive_scan_jobs", "quarantine_key", "text", true)
	assertArchiveScanColumn(t, db, "archive_scan_jobs", "error_code", "character varying", true)
	assertArchiveScanColumn(t, db, "archive_scan_jobs", "started_at", "timestamp with time zone", true)
	assertArchiveScanColumn(t, db, "archive_scan_jobs", "finished_at", "timestamp with time zone", true)

	if !testutil.IndexExists(t, db, "archive_scan_jobs", "uq_archive_scan_jobs_current") {
		t.Fatal("expected partial unique index uq_archive_scan_jobs_current")
	}
	if !testutil.ForeignKeyExists(t, db, "archive_scan_jobs", "attachment_id", "content_attachments") {
		t.Fatal("expected FK archive_scan_jobs.attachment_id -> content_attachments")
	}

	// --- archive_scan_attempts table + column contract ---
	assertArchiveScanTableExists(t, db, "archive_scan_attempts")
	assertArchiveScanColumn(t, db, "archive_scan_attempts", "id", "bigint", false)
	assertArchiveScanColumn(t, db, "archive_scan_attempts", "created_at", "timestamp with time zone", false)
	assertArchiveScanColumn(t, db, "archive_scan_attempts", "scan_job_id", "bigint", false)
	assertArchiveScanColumn(t, db, "archive_scan_attempts", "attempt_no", "integer", false)
	assertArchiveScanColumn(t, db, "archive_scan_attempts", "result", "character varying", false)
	assertArchiveScanColumn(t, db, "archive_scan_attempts", "duration_ms", "integer", false)
	assertArchiveScanColumn(t, db, "archive_scan_attempts", "engine_version", "character varying", true)
	assertArchiveScanColumn(t, db, "archive_scan_attempts", "signature_version", "character varying", true)
	assertArchiveScanColumn(t, db, "archive_scan_attempts", "detection_name", "character varying", true)
	assertArchiveScanColumn(t, db, "archive_scan_attempts", "error_code", "character varying", true)

	if !testutil.IndexExists(t, db, "archive_scan_attempts", "uq_archive_scan_attempts_job_no") {
		t.Fatal("expected unique index uq_archive_scan_attempts_job_no")
	}
	if !testutil.ForeignKeyExists(t, db, "archive_scan_attempts", "scan_job_id", "archive_scan_jobs") {
		t.Fatal("expected FK archive_scan_attempts.scan_job_id -> archive_scan_jobs")
	}

	// --- one current job per (attachment_id, scan_version) ---
	jobID := seedArchiveScanJob(t, db, modID, 1, ScanStatusPending)
	if err := db.Exec(`
		INSERT INTO archive_scan_jobs (attachment_id, scan_version, status)
		VALUES (?, 1, 'pending')
	`, modID).Error; err == nil {
		t.Fatal("a second current job for the same (attachment_id, scan_version) must be rejected")
	}
	if err := db.Exec(`
		UPDATE archive_scan_jobs SET status = 'clean' WHERE id = ?
	`, jobID).Error; err != nil {
		t.Fatalf("finish first job: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO archive_scan_jobs (attachment_id, scan_version, status)
		VALUES (?, 1, 'pending')
	`, modID).Error; err != nil {
		t.Fatalf("a new job must be allowed after the previous job reached a terminal state: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO archive_scan_jobs (attachment_id, scan_version, status)
		VALUES (?, 2, 'pending')
	`, modID).Error; err != nil {
		t.Fatalf("a job for a different scan_version must be allowed: %v", err)
	}

	// --- immutable attempt audit log: monotonic attempt_no per job ---
	scanningID := seedArchiveScanJob(t, db, imageID, 1, ScanStatusPending)
	if err := db.Exec(`
		INSERT INTO archive_scan_attempts (scan_job_id, attempt_no, result, duration_ms)
		VALUES (?, 1, 'clean', 42)
	`, scanningID).Error; err != nil {
		t.Fatalf("insert first attempt: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO archive_scan_attempts (scan_job_id, attempt_no, result, duration_ms)
		VALUES (?, 1, 'clean', 42)
	`, scanningID).Error; err == nil {
		t.Fatal("duplicate attempt_no on the same job must be rejected by uq_archive_scan_attempts_job_no")
	}
	if err := db.Exec(`
		INSERT INTO archive_scan_attempts (scan_job_id, attempt_no, result, duration_ms)
		VALUES (?, 2, 'error', 12)
	`, scanningID).Error; err != nil {
		t.Fatalf("second attempt with the next attempt_no must be allowed: %v", err)
	}
}

func requireArchiveScanBaseTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			username VARCHAR(64) UNIQUE NOT NULL
		);
		CREATE TABLE content_items (
			id BIGSERIAL PRIMARY KEY,
			title VARCHAR(500) NOT NULL,
			author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			zone VARCHAR(10) NOT NULL,
			content_type VARCHAR(20) NOT NULL,
			cover_image_url TEXT,
			status VARCHAR(20) NOT NULL DEFAULT 'pending'
		);
		CREATE TABLE content_attachments (
			id BIGSERIAL PRIMARY KEY,
			content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
			file_type VARCHAR(30) NOT NULL,
			oss_key TEXT NOT NULL,
			file_size BIGINT,
			mime_type VARCHAR(100),
			duration_sec INT,
			width INT,
			height INT,
			is_primary BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		INSERT INTO users (id, email, password_hash, username)
		VALUES (1, 'archive-owner@example.test', 'hash', 'archive-owner');
		INSERT INTO content_items (id, title, author_id, zone, content_type)
		VALUES (1, 'archive item', 1, 'original', 'mod');
	`).Error; err != nil {
		t.Fatalf("create archive scan base tables: %v", err)
	}
}

func seedPreMigrationAttachment(t *testing.T, db *gorm.DB, ossKey, fileType string) int64 {
	t.Helper()
	var id int64
	if err := db.Raw(`
		INSERT INTO content_attachments (content_item_id, file_type, oss_key)
		VALUES (1, ?, ?) RETURNING id
	`, fileType, ossKey).Scan(&id).Error; err != nil {
		t.Fatalf("seed pre-migration attachment %s: %v", ossKey, err)
	}
	return id
}

func seedPostMigrationAttachment(t *testing.T, db *gorm.DB, ossKey, fileType string) int64 {
	return seedPreMigrationAttachment(t, db, ossKey, fileType)
}

func seedArchiveScanJob(t *testing.T, db *gorm.DB, attachmentID int64, scanVersion int, status string) int64 {
	t.Helper()
	var id int64
	if err := db.Raw(`
		INSERT INTO archive_scan_jobs (attachment_id, scan_version, status)
		VALUES (?, ?, ?) RETURNING id
	`, attachmentID, scanVersion, status).Scan(&id).Error; err != nil {
		t.Fatalf("seed archive scan job: %v", err)
	}
	return id
}

func assertScanStatus(t *testing.T, db *gorm.DB, attachmentID int64, want string) {
	t.Helper()
	var status string
	if err := db.Raw(`
		SELECT scan_status FROM content_attachments WHERE id = ?
	`, attachmentID).Scan(&status).Error; err != nil {
		t.Fatalf("read scan_status for attachment %d: %v", attachmentID, err)
	}
	if status != want {
		t.Fatalf("attachment %d scan_status = %q, want %q", attachmentID, status, want)
	}
}

func assertArchiveScanTableExists(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = current_schema() AND table_name = ?
		)
	`, table).Scan(&exists).Error; err != nil {
		t.Fatalf("lookup table %s: %v", table, err)
	}
	if !exists {
		t.Fatalf("expected table %s to exist after migration 072", table)
	}
}

func assertArchiveScanColumn(t *testing.T, db *gorm.DB, table, column, wantType string, wantNullable bool) {
	t.Helper()
	dataType, nullable := testutil.ColumnMetadata(t, db, table, column)
	if dataType != wantType || nullable != wantNullable {
		t.Fatalf("%s.%s = (%s, nullable=%v), want (%s, nullable=%v)", table, column, dataType, nullable, wantType, wantNullable)
	}
}
