package model

import (
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"

	"omnicraft/backend/internal/testutil"
)

func TestContentSourceMigrationAddsColumnsIndexAndForeignKey(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	requireContentItemsBaseTable(t, db)

	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "036_content_source_original.sql"))

	dataType, nullable := testutil.ColumnMetadata(t, db, "content_items", "description")
	if dataType != "text" || !nullable {
		t.Fatalf("description column = (%s, nullable=%v), want text nullable", dataType, nullable)
	}

	dataType, nullable = testutil.ColumnMetadata(t, db, "content_items", "source_original_id")
	if dataType != "bigint" || !nullable {
		t.Fatalf("source_original_id column = (%s, nullable=%v), want bigint nullable", dataType, nullable)
	}
	if !testutil.IndexExists(t, db, "content_items", "idx_content_items_source_original") {
		t.Fatal("expected idx_content_items_source_original to exist")
	}
	if !testutil.ForeignKeyExists(t, db, "content_items", "source_original_id", "content_items") {
		t.Fatal("expected source_original_id foreign key to reference content_items")
	}
}

// TestContentSourceMigration covers the empty-database upgrade path for
// 064: the nullable source_fanwork_id column on content_items, its
// self-referencing foreign key with ON DELETE SET NULL, and the partial
// index used by fanwork-source lookups.
func TestContentSourceMigration(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	requireContentItemsBaseTable(t, db)

	migration := filepath.Join("..", "..", "migrations", "064_add_source_fanwork_id.sql")
	testutil.ApplyMigrationFile(t, db, migration)
	testutil.ApplyMigrationFile(t, db, migration)

	dataType, nullable := testutil.ColumnMetadata(t, db, "content_items", "source_fanwork_id")
	if dataType != "bigint" || !nullable {
		t.Fatalf("source_fanwork_id column = (%s, nullable=%v), want bigint nullable", dataType, nullable)
	}

	assertSeriesForeignKey(t, db, "content_items", "source_fanwork_id", "content_items", "SET NULL")

	if !testutil.IndexExists(t, db, "content_items", "idx_content_items_source_fanwork") {
		t.Fatal("expected idx_content_items_source_fanwork to exist")
	}
	definition := indexDefinition(t, db, "idx_content_items_source_fanwork")
	if !strings.Contains(definition, "source_fanwork_id IS NOT NULL") {
		t.Fatalf("idx_content_items_source_fanwork definition %q is not partial on source_fanwork_id", definition)
	}
}

func requireContentItemsBaseTable(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec(`
		CREATE TABLE content_items (
			id BIGSERIAL PRIMARY KEY,
			title TEXT NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error; err != nil {
		t.Fatalf("create content_items base table: %v", err)
	}
}
