package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/testutil"
)

func TestRAGRetrievalAppliesVisibilityAndCurrentGenerationPredicates(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	for _, statement := range []string{
		`CREATE TABLE users (id BIGSERIAL PRIMARY KEY, is_banned BOOLEAN NOT NULL DEFAULT FALSE, deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE ips (id BIGSERIAL PRIMARY KEY, status VARCHAR(20) NOT NULL DEFAULT 'active')`,
		`CREATE TABLE content_items (id BIGSERIAL PRIMARY KEY, title TEXT NOT NULL, author_id BIGINT NOT NULL REFERENCES users(id), description TEXT NOT NULL DEFAULT '', status VARCHAR(20) NOT NULL, deleted_at TIMESTAMPTZ, ip_id BIGINT REFERENCES ips(id), is_public BOOLEAN NOT NULL DEFAULT TRUE, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), search_vector TSVECTOR)`,
		`CREATE TABLE content_versions (id BIGSERIAL PRIMARY KEY, content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE, parent_version_id BIGINT, author_id BIGINT NOT NULL REFERENCES users(id), version_number INT NOT NULL, storage_type VARCHAR(10) NOT NULL, storage_key TEXT, diff_summary TEXT, status VARCHAR(20) NOT NULL DEFAULT 'active', is_latest BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(content_item_id, version_number))`,
		`INSERT INTO users (id) VALUES (1), (2), (3)`,
		`UPDATE users SET is_banned = TRUE WHERE id = 3`,
		`INSERT INTO content_items (id, title, author_id, description, status, is_public, search_vector) VALUES (10, 'Visible guide', 2, 'visible keyword', 'published', TRUE, to_tsvector('simple', 'Visible guide visible keyword')), (11, 'Private guide', 2, 'private keyword', 'published', FALSE, to_tsvector('simple', 'Private guide private keyword')), (12, 'Banned guide', 3, 'banned keyword', 'published', TRUE, to_tsvector('simple', 'Banned guide banned keyword'))`,
		`INSERT INTO content_versions (content_item_id, author_id, version_number, storage_type, storage_key, status, is_latest) VALUES (10, 2, 1, 'full', 'visible', 'active', TRUE), (11, 2, 1, 'full', 'private', 'active', TRUE), (12, 3, 1, 'full', 'banned', 'active', TRUE)`,
	} {
		require.NoError(t, db.Exec(statement).Error)
	}
	testutil.ApplyMigrationFile(t, db, "../../migrations/071_rag_chunks.sql")
	chunkRepo := NewRagChunkRepository(db)
	for _, item := range []struct {
		id   int64
		key  string
		text string
	}{
		{10, "visible", "visible keyword"},
		{11, "private", "private keyword"},
		{12, "banned", "banned keyword"},
	} {
		chunk := model.RagChunk{ContentID: item.id, ContentVersion: 1, ChunkIndex: 0, ChunkKey: strings.Repeat(item.key, 64/len(item.key)), ChunkingVersion: 1, Text: item.text, SourceStart: 0, SourceEnd: len(item.text), Zone: "original", ContentType: "article", Tags: pq.StringArray{}, IndexVersion: 1}
		require.NoError(t, chunkRepo.StageGeneration(context.Background(), RagGeneration{ContentID: item.id, IndexVersion: 1, ChunkingVersion: 1, EmbeddingModel: "test-model"}, []model.RagChunk{chunk}))
		require.NoError(t, chunkRepo.PromoteGeneration(context.Background(), RagGeneration{ContentID: item.id, IndexVersion: 1, ChunkingVersion: 1, EmbeddingModel: "test-model"}))
		var chunkID int64
		require.NoError(t, db.Raw("SELECT id FROM rag_chunks WHERE content_id = ?", item.id).Scan(&chunkID).Error)
		require.NoError(t, db.Exec("INSERT INTO chunk_embeddings (chunk_id, embedding, embedding_model) VALUES (?, ?, ?)", chunkID, testVectorLiteral(), "test-model").Error)
	}

	ctx := context.Background()
	keyword, err := NewSearchRepository(db).SearchRAGChunks(ctx, "keyword", 20, 1)
	require.NoError(t, err)
	require.Len(t, keyword, 1)
	require.Equal(t, int64(10), keyword[0].ContentID)

	vector, err := NewEmbeddingRepository(db).VectorSearchChunks(ctx, testVector(), 20, 1, "test-model")
	require.NoError(t, err)
	require.Len(t, vector, 1)
	require.Equal(t, int64(10), vector[0].ContentID)
}

func testVector() []float32 {
	vector := make([]float32, 1536)
	vector[0] = 1
	return vector
}

func testVectorLiteral() string {
	values := make([]string, 1536)
	values[0] = "1"
	for i := 1; i < len(values); i++ {
		values[i] = "0"
	}
	return fmt.Sprintf("[%s]", strings.Join(values, ","))
}

// TestRAGRetrievalMatchesThroughStoredSearchVector locks the A-03 lexical
// upgrade: the match runs against the 041-maintained content_items
// search_vector, not a runtime-recomputed chunk vector — the query term
// exists only in the description weight of the stored vector here.
func TestRAGRetrievalMatchesThroughStoredSearchVector(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	for _, statement := range []string{
		`CREATE TABLE users (id BIGSERIAL PRIMARY KEY, is_banned BOOLEAN NOT NULL DEFAULT FALSE, deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE ips (id BIGSERIAL PRIMARY KEY, status VARCHAR(20) NOT NULL DEFAULT 'active')`,
		`CREATE TABLE content_items (id BIGSERIAL PRIMARY KEY, title TEXT NOT NULL, author_id BIGINT NOT NULL REFERENCES users(id), description TEXT NOT NULL DEFAULT '', status VARCHAR(20) NOT NULL, deleted_at TIMESTAMPTZ, ip_id BIGINT REFERENCES ips(id), is_public BOOLEAN NOT NULL DEFAULT TRUE, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), search_vector TSVECTOR)`,
		`CREATE TABLE content_versions (id BIGSERIAL PRIMARY KEY, content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE, parent_version_id BIGINT, author_id BIGINT NOT NULL REFERENCES users(id), version_number INT NOT NULL, storage_type VARCHAR(10) NOT NULL, storage_key TEXT, diff_summary TEXT, status VARCHAR(20) NOT NULL DEFAULT 'active', is_latest BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(content_item_id, version_number))`,
		`INSERT INTO users (id) VALUES (1)`,
		`INSERT INTO content_items (id, title, author_id, description, status, is_public, search_vector) VALUES (20, 'Vector guide', 1, 'uniquemarker', 'published', TRUE, to_tsvector('simple', 'Vector guide uniquemarker'))`,
		`INSERT INTO content_versions (content_item_id, author_id, version_number, storage_type, storage_key, status, is_latest) VALUES (20, 1, 1, 'full', 'vec', 'active', TRUE)`,
	} {
		require.NoError(t, db.Exec(statement).Error)
	}
	testutil.ApplyMigrationFile(t, db, "../../migrations/071_rag_chunks.sql")
	chunk := model.RagChunk{ContentID: 20, ContentVersion: 1, ChunkIndex: 0, ChunkKey: strings.Repeat("vec", 21), ChunkingVersion: 1, Text: "chunk body without the marker", SourceStart: 0, SourceEnd: 10, Zone: "original", ContentType: "article", Tags: pq.StringArray{}, IndexVersion: 1}
	require.NoError(t, NewRagChunkRepository(db).StageGeneration(context.Background(), RagGeneration{ContentID: 20, IndexVersion: 1, ChunkingVersion: 1, EmbeddingModel: "test-model"}, []model.RagChunk{chunk}))
	require.NoError(t, NewRagChunkRepository(db).PromoteGeneration(context.Background(), RagGeneration{ContentID: 20, IndexVersion: 1, ChunkingVersion: 1, EmbeddingModel: "test-model"}))

	results, err := NewSearchRepository(db).SearchRAGChunks(context.Background(), "uniquemarker", 20, 1)
	require.NoError(t, err)
	require.Len(t, results, 1, "stored search_vector must drive the lexical match")
	require.Equal(t, int64(20), results[0].ContentID)
}
