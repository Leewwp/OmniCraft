package model

import (
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"omnicraft/backend/internal/testutil"
)

// TestContentMediaMetadataMigration covers the empty-database upgrade path for
// 063: cover dimensions on content_items, a nullable per-content sort_order on
// content_attachments, unique ordering for new media rows, legacy NULL
// sort_order rows staying readable, and the deterministic read order used by
// the repository (sort_order ASC NULLS LAST, id ASC).
func TestContentMediaMetadataMigration(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	requireMediaMetadataBaseTables(t, db)

	migration := filepath.Join("..", "..", "migrations", "063_content_media_metadata.sql")
	testutil.ApplyMigrationFile(t, db, migration)
	testutil.ApplyMigrationFile(t, db, migration)

	for _, column := range []string{"cover_width", "cover_height"} {
		dataType, nullable := testutil.ColumnMetadata(t, db, "content_items", column)
		if dataType != "integer" || !nullable {
			t.Fatalf("%s column = (%s, nullable=%v), want integer nullable", column, dataType, nullable)
		}
	}
	dataType, nullable := testutil.ColumnMetadata(t, db, "content_attachments", "sort_order")
	if dataType != "integer" || !nullable {
		t.Fatalf("sort_order column = (%s, nullable=%v), want integer nullable", dataType, nullable)
	}
	if !testutil.IndexExists(t, db, "content_attachments", "uq_content_attachments_item_sort_order") {
		t.Fatal("expected uq_content_attachments_item_sort_order index")
	}

	if err := db.Exec(`
		INSERT INTO content_items (id, title, author_id, zone, content_type)
		VALUES (1, 'gallery', 1, 'original', 'image')
	`).Error; err != nil {
		t.Fatalf("insert gallery content: %v", err)
	}
	attachmentIDs := []int64{}
	for i := 0; i < 3; i++ {
		key := "uploads/1/image/g" + string(rune('a'+i)) + ".png"
		var id int64
		if err := db.Raw(`INSERT INTO content_attachments (content_item_id, file_type, oss_key, sort_order)
			VALUES (1, 'image', ?, ?) RETURNING id`, key, i).Scan(&id).Error; err != nil {
			t.Fatalf("insert attachment %d: %v", i, err)
		}
		attachmentIDs = append(attachmentIDs, id)
	}
	if err := db.Exec(`
		INSERT INTO content_attachments (content_item_id, file_type, oss_key, sort_order)
		VALUES (1, 'image', 'uploads/1/image/dup.png', 1)
	`).Error; err == nil {
		t.Fatal("duplicate sort_order within the same content must be rejected")
	}
	if err := db.Exec(`
		INSERT INTO content_attachments (content_item_id, file_type, oss_key, sort_order)
		VALUES (1, 'image', 'uploads/1/image/dup2.png', 0)
	`).Error; err == nil {
		t.Fatal("duplicate sort_order 0 within the same content must be rejected")
	}

	// Legacy rows with NULL sort_order stay readable; multiple NULLs allowed.
	if err := db.Exec(`
		INSERT INTO content_items (id, title, author_id, zone, content_type)
		VALUES (2, 'legacy', 1, 'original', 'video')
	`).Error; err != nil {
		t.Fatalf("insert legacy content: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := db.Exec(`
			INSERT INTO content_attachments (content_item_id, file_type, oss_key)
			VALUES (2, 'video', ?)
		`, "uploads/1/video/l"+string(rune('a'+i))+".mp4").Error; err != nil {
			t.Fatalf("insert legacy attachment %d: %v", i, err)
		}
	}

	// Deterministic read order: sort_order ASC NULLS LAST, id ASC.
	var ordered []int64
	if err := db.Raw(`
		SELECT id FROM content_attachments
		WHERE content_item_id = 1
		ORDER BY sort_order ASC NULLS LAST, id ASC
	`).Scan(&ordered).Error; err != nil {
		t.Fatalf("query gallery order: %v", err)
	}
	if len(ordered) != 3 {
		t.Fatalf("gallery attachments = %d, want 3", len(ordered))
	}
	for i := range ordered {
		if ordered[i] != attachmentIDs[i] {
			t.Fatalf("gallery order[%d] = %d, want %d (insertion id order)", i, ordered[i], attachmentIDs[i])
		}
	}

	var legacy []int64
	if err := db.Raw(`
		SELECT id FROM content_attachments
		WHERE content_item_id = 2
		ORDER BY sort_order ASC NULLS LAST, id ASC
	`).Scan(&legacy).Error; err != nil {
		t.Fatalf("query legacy order: %v", err)
	}
	if len(legacy) != 2 {
		t.Fatalf("legacy attachments = %d, want 2", len(legacy))
	}
	if legacy[0] > legacy[1] {
		t.Fatalf("legacy NULL sort_order rows must fall back to id ASC, got %v", legacy)
	}
}

func requireMediaMetadataBaseTables(t *testing.T, db *gorm.DB) {
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
		VALUES (1, 'media-owner@example.test', 'hash', 'media-owner');
	`).Error; err != nil {
		t.Fatalf("create media metadata base tables: %v", err)
	}
}
