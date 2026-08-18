package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"

	"omnicraft/backend/internal/testutil"
)

func TestRagChunksMigrationAndPublishedBackfill(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	db.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`)
	db.Exec(`CREATE TABLE ips (id BIGSERIAL PRIMARY KEY)`)
	db.Exec(`CREATE TABLE content_items (
		id BIGSERIAL PRIMARY KEY, author_id BIGINT NOT NULL REFERENCES users(id),
		description TEXT NOT NULL DEFAULT '', status VARCHAR(20) NOT NULL,
		deleted_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`)
	db.Exec(`CREATE TABLE content_versions (
		id BIGSERIAL PRIMARY KEY, content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
		parent_version_id BIGINT, author_id BIGINT NOT NULL REFERENCES users(id), version_number INT NOT NULL,
		storage_type VARCHAR(10) NOT NULL, storage_key TEXT, diff_summary TEXT,
		status VARCHAR(20) NOT NULL DEFAULT 'active', is_latest BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(content_item_id, version_number))`)
	requireExec(t, db, `INSERT INTO users (id) VALUES (1)`)
	requireExec(t, db, `INSERT INTO content_items (id, author_id, description, status) VALUES (10, 1, 'published body', 'published'), (11, 1, 'draft', 'pending')`)

	migration := filepath.Join("..", "..", "migrations", "071_rag_chunks.sql")
	migrationSQL, err := os.ReadFile(migration)
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		"-- Deduplicates deterministic chunk identity within one index generation.\n    CONSTRAINT uq_rag_chunks_generation_key",
		"-- Prevents duplicate ordering slots during generation staging/replay.\n    CONSTRAINT uq_rag_chunks_generation_order",
		"-- Supports content-version projection replacement and cascade cleanup.\nCREATE INDEX IF NOT EXISTS idx_rag_chunks_content",
		"-- Supports generation-scoped chunk reads and rebuild bookkeeping.\nCREATE INDEX IF NOT EXISTS idx_rag_chunks_generation",
		"-- Supports IP visibility filtering before retrieval ranking.\nCREATE INDEX IF NOT EXISTS idx_rag_chunks_ip",
		"-- Prevents duplicate provider-model embeddings for the same chunk.\n    CONSTRAINT uq_chunk_embeddings_model",
		"-- Supports cosine nearest-neighbor retrieval over chunk embeddings.\nCREATE INDEX IF NOT EXISTS idx_chunk_embeddings_vector",
		"-- Coordinates one lifecycle row for each content index generation.\n    CONSTRAINT uq_index_projection_generation",
		"-- Enforces one query-visible generation per content during atomic promotion.\nCREATE UNIQUE INDEX IF NOT EXISTS uq_index_projection_current",
		"-- Supports current ready-generation lookup by chunker and embedding contract.\nCREATE INDEX IF NOT EXISTS idx_index_projection_ready",
	} {
		if !strings.Contains(string(migrationSQL), contract) {
			t.Fatalf("migration index rationale missing contract %q", contract)
		}
	}
	testutil.ApplyMigrationFile(t, db, migration)
	testutil.ApplyMigrationFile(t, db, migration)

	for _, table := range []string{"rag_chunks", "chunk_embeddings", "index_projection_status"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing table %s", table)
		}
	}
	if !db.Migrator().HasIndex("rag_chunks", "idx_rag_chunks_ip") {
		t.Fatal("missing rag_chunks(ip) visibility-filter index")
	}
	dataType, nullable := testutil.ColumnMetadata(t, db, "rag_chunks", "category")
	if dataType != "character varying" || !nullable {
		t.Fatalf("rag_chunks.category = (%s, nullable=%v)", dataType, nullable)
	}
	dataType, _ = testutil.ColumnMetadata(t, db, "chunk_embeddings", "embedding")
	if dataType != "USER-DEFINED" {
		t.Fatalf("chunk_embeddings.embedding type = %s, want USER-DEFINED vector", dataType)
	}

	var count int64
	requireExecScan(t, db, `SELECT count(*) FROM content_versions WHERE content_item_id = 10 AND version_number = 1 AND storage_key = 'published body' AND is_latest`, &count)
	if count != 1 {
		t.Fatalf("published backfill count = %d, want 1", count)
	}
	requireExecScan(t, db, `SELECT count(*) FROM content_versions WHERE content_item_id = 11`, &count)
	if count != 0 {
		t.Fatalf("unpublished content was backfilled")
	}

	requireExec(t, db, `INSERT INTO rag_chunks (content_id, content_version, chunk_index, chunk_key, chunking_version, text, source_start, source_end, zone, content_type, index_version)
		VALUES (10, 1, 0, repeat('a', 64), 1, 'body', 0, 4, 'original', 'guide', 1)`)
	if err := db.Exec(`INSERT INTO rag_chunks (content_id, content_version, chunk_index, chunk_key, chunking_version, text, source_start, source_end, zone, content_type, index_version)
		VALUES (10, 1, 1, repeat('a', 64), 1, 'body', 0, 4, 'original', 'guide', 1)`).Error; err == nil {
		t.Fatal("same index_version/chunk_key must be unique")
	}
	requireExec(t, db, `INSERT INTO rag_chunks (content_id, content_version, chunk_index, chunk_key, chunking_version, text, source_start, source_end, zone, content_type, index_version)
		VALUES (10, 1, 0, repeat('a', 64), 1, 'body', 0, 4, 'original', 'guide', 2)`)
	if err := db.Exec(`INSERT INTO rag_chunks (content_id, content_version, chunk_index, chunk_key, chunking_version, text, source_start, source_end, zone, content_type, index_version)
		VALUES (10, 1, 0, repeat('b', 64), 1, 'other', 0, 5, 'original', 'guide', 2)`).Error; err == nil {
		t.Fatal("same content/version/chunking/index/chunk_index must be unique")
	}

	var chunkID int64
	requireExecScan(t, db, `SELECT id FROM rag_chunks WHERE index_version = 1`, &chunkID)
	zeroVector := "[" + strings.Repeat("0,", 1535) + "0]"
	requireExec(t, db, `INSERT INTO chunk_embeddings (chunk_id, embedding, embedding_model) VALUES (?, ?::vector, 'embed-v1')`, chunkID, zeroVector)
	if err := db.Exec(`INSERT INTO chunk_embeddings (chunk_id, embedding, embedding_model) VALUES (?, ?::vector, 'embed-v1')`, chunkID, zeroVector).Error; err == nil {
		t.Fatal("same chunk_id/embedding_model must be unique")
	}
	requireExec(t, db, `INSERT INTO chunk_embeddings (chunk_id, embedding, embedding_model) VALUES (?, ?::vector, 'embed-v2')`, chunkID, zeroVector)

	requireExec(t, db, `INSERT INTO index_projection_status (content_id, index_version, chunking_version, embedding_model, state, is_current) VALUES (10, 1, 1, 'embed-v1', 'ready', TRUE)`)
	if err := db.Exec(`INSERT INTO index_projection_status (content_id, index_version, chunking_version, embedding_model, state, is_current) VALUES (10, 2, 1, 'embed-v1', 'ready', TRUE)`).Error; err == nil {
		t.Fatal("at most one current projection per content must be allowed")
	}
}

func requireExec(t *testing.T, db interface{ Exec(string, ...any) *gorm.DB }, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatal(err)
	}
}

func requireExecScan(t *testing.T, db *gorm.DB, query string, dest any) {
	t.Helper()
	if err := db.Raw(query).Scan(dest).Error; err != nil {
		t.Fatal(err)
	}
}
