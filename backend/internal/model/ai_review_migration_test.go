package model

import (
	"path/filepath"
	"testing"

	"omnicraft/backend/internal/testutil"
)

// TestAIReviewRecordsTaskIDMigration covers the upgrade path for 068: the
// nullable provider_task_id column, the (provider, provider_task_id) unique
// index, PostgreSQL's NULL-ignoring uniqueness (multiple task-less records
// coexist) and the idempotent forward-only re-application contract.
func TestAIReviewRecordsTaskIDMigration(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "013_ai_review.sql"))

	migration := filepath.Join("..", "..", "migrations", "068_add_ai_review_records_task_id.sql")
	testutil.ApplyMigrationFile(t, db, migration)
	testutil.ApplyMigrationFile(t, db, migration)

	dataType, nullable := testutil.ColumnMetadata(t, db, "ai_review_records", "provider_task_id")
	if dataType != "character varying" || !nullable {
		t.Fatalf("provider_task_id column = (%s, nullable=%v), want varchar nullable", dataType, nullable)
	}

	if !testutil.IndexExists(t, db, "ai_review_records", "uq_ai_review_records_provider_task") {
		t.Fatal("expected unique index uq_ai_review_records_provider_task")
	}

	if err := db.Exec(`
		INSERT INTO ai_review_records (target_type, target_id, provider, result, provider_task_id)
		VALUES ('content', 1, 'aliyun', 'pass', 'task-abc')
	`).Error; err != nil {
		t.Fatalf("seed first async record: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO ai_review_records (target_type, target_id, provider, result, provider_task_id)
		VALUES ('content', 1, 'aliyun', 'pass', 'task-abc')
	`).Error; err == nil {
		t.Fatal("duplicate (provider, provider_task_id) insert must be rejected by the unique index")
	}

	// The same task id under a different provider is a distinct key.
	if err := db.Exec(`
		INSERT INTO ai_review_records (target_type, target_id, provider, result, provider_task_id)
		VALUES ('content', 1, 'other', 'pass', 'task-abc')
	`).Error; err != nil {
		t.Fatalf("same task id under another provider must be allowed: %v", err)
	}

	// Multiple NULL provider_task_id rows coexist: synchronous records
	// without a task id never collide with each other.
	for i := 1; i <= 3; i++ {
		if err := db.Exec(`
			INSERT INTO ai_review_records (target_type, target_id, provider, result)
			VALUES ('content', ?, 'aliyun', 'pass')
		`, i).Error; err != nil {
			t.Fatalf("seed task-less sync record %d: %v", i, err)
		}
	}
}
