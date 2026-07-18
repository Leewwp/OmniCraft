package model

import (
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"omnicraft/backend/internal/testutil"
)

func TestContentSeriesMigration(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	requireContentSeriesMigrationBaseTables(t, db)

	migration := filepath.Join("..", "..", "migrations", "059_create_content_series.sql")
	testutil.ApplyMigrationFile(t, db, migration)
	testutil.ApplyMigrationFile(t, db, migration)

	for _, table := range []string{"content_series", "content_series_items"} {
		if !tableExists(t, db, table) {
			t.Fatalf("expected %s table to exist", table)
		}
	}
	for table, indexes := range map[string][]string{
		"content_series":       {"idx_content_series_owner"},
		"content_series_items": {"idx_series_items_series", "idx_series_items_content"},
	} {
		for _, index := range indexes {
			if !testutil.IndexExists(t, db, table, index) {
				t.Fatalf("expected %s on %s", index, table)
			}
		}
	}
	if !uniqueConstraintExists(t, db, "content_series_items", []string{"series_id", "content_item_id"}) {
		t.Fatal("expected UNIQUE(series_id, content_item_id) on content_series_items")
	}

	assertSeriesForeignKey(t, db, "content_series", "owner_id", "users", "CASCADE")
	assertSeriesForeignKey(t, db, "content_series", "cover_content_id", "content_items", "SET NULL")
	assertSeriesForeignKey(t, db, "content_series_items", "series_id", "content_series", "CASCADE")
	assertSeriesForeignKey(t, db, "content_series_items", "content_item_id", "content_items", "CASCADE")

	if err := silentDB(db).Exec(`
		INSERT INTO content_series (title, owner_id, zone)
		VALUES ('invalid zone', 1, 'music')
	`).Error; err == nil {
		t.Fatal("expected invalid series zone to be rejected")
	}
	for _, zone := range []string{"original", "fanwork"} {
		if err := db.Exec(`
			INSERT INTO content_series (title, owner_id, zone)
			VALUES (?, 1, ?)
		`, zone+" series", zone).Error; err != nil {
			t.Fatalf("expected zone %q to be accepted: %v", zone, err)
		}
	}
}

func requireContentSeriesMigrationBaseTables(t *testing.T, db *gorm.DB) {
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
			status VARCHAR(20) NOT NULL DEFAULT 'pending'
		);
		INSERT INTO users (id, email, password_hash, username)
		VALUES (1, 'series-owner@example.test', 'hash', 'series-owner');
		INSERT INTO content_items (id, title, author_id, zone, content_type, status)
		VALUES (10, 'series cover', 1, 'original', 'article', 'published');
	`).Error; err != nil {
		t.Fatalf("create series migration base tables: %v", err)
	}
}

func assertSeriesForeignKey(t *testing.T, db *gorm.DB, table, column, referencedTable, deleteRule string) {
	t.Helper()
	if !testutil.ForeignKeyExists(t, db, table, column, referencedTable) {
		t.Fatalf("expected %s.%s to reference %s", table, column, referencedTable)
	}
	var actual string
	if err := db.Raw(`
		SELECT rc.delete_rule
		FROM information_schema.referential_constraints rc
		JOIN information_schema.key_column_usage kcu
		  ON kcu.constraint_schema = rc.constraint_schema
		 AND kcu.constraint_name = rc.constraint_name
		WHERE kcu.table_schema = current_schema()
		  AND kcu.table_name = ?
		  AND kcu.column_name = ?
	`, table, column).Scan(&actual).Error; err != nil {
		t.Fatalf("lookup delete rule for %s.%s: %v", table, column, err)
	}
	if actual != deleteRule {
		t.Fatalf("delete rule for %s.%s = %q, want %q", table, column, actual, deleteRule)
	}
}
