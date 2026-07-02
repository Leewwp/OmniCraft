package model

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lib/pq"
	"omnicraft/backend/internal/testutil"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCollectionMigration(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	requireCollectionsMigrationBaseTables(t, db)
	seedCollectionsMigrationData(t, db)

	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "058_create_collections.sql"))
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "058_create_collections.sql"))

	if !tableExists(t, db, "collections") {
		t.Fatal("expected collections table to exist")
	}
	if !tableExists(t, db, "collection_items") {
		t.Fatal("expected collection_items table to exist")
	}
	if !testutil.IndexExists(t, db, "collections", "idx_collections_one_default_per_zone") {
		t.Fatal("expected idx_collections_one_default_per_zone to exist")
	}

	indexDef := indexDefinition(t, db, "idx_collections_one_default_per_zone")
	if !strings.Contains(strings.ToLower(indexDef), "where (is_default = true)") {
		t.Fatalf("idx_collections_one_default_per_zone definition = %q, want partial WHERE is_default = TRUE", indexDef)
	}

	if !uniqueConstraintExists(t, db, "collection_items", []string{"collection_id", "content_item_id"}) {
		t.Fatal("expected UNIQUE(collection_id, content_item_id) on collection_items")
	}
	assertDefaultCollectionsBackfilled(t, db)
	assertFavoritesBackfilledAndRetained(t, db)

	if err := silentDB(db).Exec(`
		INSERT INTO collections (user_id, title, zone)
		VALUES (1, 'invalid zone', 'music')
	`).Error; err == nil {
		t.Fatal("expected invalid collection zone to be rejected")
	}

	for _, zone := range []string{"original", "fanwork"} {
		if err := db.Exec(`
			INSERT INTO collections (user_id, title, zone)
			VALUES (1, ?, ?)
		`, zone+" collection", zone).Error; err != nil {
			t.Fatalf("expected zone %q to be accepted: %v", zone, err)
		}
	}
}

func seedCollectionsMigrationData(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec(`
		INSERT INTO users (id, email, password_hash, username)
		VALUES
			(1, 'alpha@example.com', 'hash', 'alpha'),
			(2, 'beta@example.com', 'hash', 'beta')
	`).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO content_items (id, title, author_id, zone, content_type, status)
		VALUES
			(10, 'original content', 1, 'original', 'article', 'published'),
			(11, 'fanwork content', 2, 'fanwork', 'image', 'published')
	`).Error; err != nil {
		t.Fatalf("seed content_items: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO favorites (user_id, content_item_id)
		VALUES
			(1, 10),
			(1, 11),
			(2, 11)
	`).Error; err != nil {
		t.Fatalf("seed favorites: %v", err)
	}
}

func requireCollectionsMigrationBaseTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			username VARCHAR(64) UNIQUE NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error; err != nil {
		t.Fatalf("create users base table: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE content_items (
			id BIGSERIAL PRIMARY KEY,
			title VARCHAR(500) NOT NULL,
			author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			zone VARCHAR(10) NOT NULL,
			content_type VARCHAR(20) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error; err != nil {
		t.Fatalf("create content_items base table: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE favorites (
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY(user_id, content_item_id)
		)
	`).Error; err != nil {
		t.Fatalf("create favorites base table: %v", err)
	}
}

func assertDefaultCollectionsBackfilled(t *testing.T, db *gorm.DB) {
	t.Helper()

	type row struct {
		UserID    int64
		Zone      string
		Title     string
		SortOrder int
		Count     int
	}
	var rows []row
	if err := db.Raw(`
		SELECT user_id, zone, title, sort_order, COUNT(*) AS count
		FROM collections
		WHERE is_default = TRUE
		GROUP BY user_id, zone, title, sort_order
		ORDER BY user_id, zone
	`).Scan(&rows).Error; err != nil {
		t.Fatalf("query default collections: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("default collections count = %d, want 4; rows=%#v", len(rows), rows)
	}

	expectedTitles := map[string]string{
		"original": "默认原创收藏",
		"fanwork":  "默认二创收藏",
	}
	for _, row := range rows {
		if row.Count != 1 {
			t.Fatalf("default collection duplicate for user=%d zone=%s count=%d, want 1", row.UserID, row.Zone, row.Count)
		}
		if row.Title != expectedTitles[row.Zone] {
			t.Fatalf("default title for zone %s = %q, want %q", row.Zone, row.Title, expectedTitles[row.Zone])
		}
		if row.SortOrder != 0 {
			t.Fatalf("default sort_order for user=%d zone=%s = %d, want 0", row.UserID, row.Zone, row.SortOrder)
		}
	}
}

func assertFavoritesBackfilledAndRetained(t *testing.T, db *gorm.DB) {
	t.Helper()

	var favoriteCount int64
	if err := db.Table("favorites").Count(&favoriteCount).Error; err != nil {
		t.Fatalf("count favorites: %v", err)
	}
	if favoriteCount != 3 {
		t.Fatalf("favorites count = %d, want 3", favoriteCount)
	}

	type row struct {
		UserID        int64
		ContentItemID int64
		Zone          string
	}
	var rows []row
	if err := db.Raw(`
		SELECT c.user_id, ci.content_item_id, c.zone
		FROM collection_items ci
		JOIN collections c ON c.id = ci.collection_id
		ORDER BY c.user_id, ci.content_item_id
	`).Scan(&rows).Error; err != nil {
		t.Fatalf("query migrated collection items: %v", err)
	}
	want := []row{
		{UserID: 1, ContentItemID: 10, Zone: "original"},
		{UserID: 1, ContentItemID: 11, Zone: "fanwork"},
		{UserID: 2, ContentItemID: 11, Zone: "fanwork"},
	}
	if len(rows) != len(want) {
		t.Fatalf("migrated collection items = %#v, want %#v", rows, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("migrated collection item[%d] = %#v, want %#v", i, rows[i], want[i])
		}
	}
}

func tableExists(t *testing.T, db *gorm.DB, table string) bool {
	t.Helper()

	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = current_schema()
			  AND table_name = ?
		)
	`, table).Scan(&exists).Error; err != nil {
		t.Fatalf("lookup table %s: %v", table, err)
	}
	return exists
}

func indexDefinition(t *testing.T, db *gorm.DB, index string) string {
	t.Helper()

	var definition string
	if err := db.Raw(`
		SELECT indexdef
		FROM pg_indexes
		WHERE schemaname = current_schema()
		  AND indexname = ?
	`, index).Scan(&definition).Error; err != nil {
		t.Fatalf("lookup index definition %s: %v", index, err)
	}
	if definition == "" {
		t.Fatalf("index %s not found", index)
	}
	return definition
}

func uniqueConstraintExists(t *testing.T, db *gorm.DB, table string, columns []string) bool {
	t.Helper()

	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu
			  ON tc.constraint_name = kcu.constraint_name
			 AND tc.table_schema = kcu.table_schema
			WHERE tc.table_schema = current_schema()
			  AND tc.table_name = ?
			  AND tc.constraint_type = 'UNIQUE'
			GROUP BY tc.constraint_name
			HAVING array_agg(kcu.column_name ORDER BY kcu.ordinal_position) = ?
		)
	`, table, pq.Array(columns)).Scan(&exists).Error; err != nil {
		t.Fatalf("lookup unique constraint on %s: %v", table, err)
	}
	return exists
}

func silentDB(db *gorm.DB) *gorm.DB {
	return db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
}
