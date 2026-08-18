package repository

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/testutil"
)

func TestRagChunkRepositoryGenerationSwitch(t *testing.T) {
	db := prepareRagRepositoryDatabase(t)
	repo := NewRagChunkRepository(db)
	ctx := context.Background()

	v1 := RagGeneration{ContentID: 10, IndexVersion: 1, ChunkingVersion: 1, EmbeddingModel: "embed-v1"}
	require.NoError(t, repo.StageGeneration(ctx, v1, []model.RagChunk{ragRepoChunk("a", 0)}))
	require.NoError(t, repo.PromoteGeneration(ctx, v1))
	// A worker can lose the acknowledgement after the transaction commits.
	// Replaying promotion of that exact ready+current generation is success.
	require.NoError(t, repo.PromoteGeneration(ctx, v1))
	current, err := repo.ListCurrent(ctx, 10, 1, "embed-v1")
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, 1, current[0].IndexVersion)
	originalChunkID := current[0].ID

	// At-least-once replay of an already-promoted event must not make the
	// current generation unreadable or replace its chunks.
	require.NoError(t, repo.StageGeneration(ctx, v1, []model.RagChunk{ragRepoChunk("z", 0)}))
	require.NoError(t, repo.MarkFailed(ctx, v1, "late duplicate failure"))
	current, err = repo.ListCurrent(ctx, 10, 1, "embed-v1")
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, originalChunkID, current[0].ID)
	require.Equal(t, strings.Repeat("a", 64), current[0].ChunkKey)

	v2 := RagGeneration{ContentID: 10, IndexVersion: 2, ChunkingVersion: 1, EmbeddingModel: "embed-v1"}
	require.NoError(t, repo.StageGeneration(ctx, v2, []model.RagChunk{ragRepoChunk("b", 0)}))
	current, err = repo.ListCurrent(ctx, 10, 1, "embed-v1")
	require.NoError(t, err)
	require.Len(t, current, 1, "staging must leave the old generation readable")
	require.Equal(t, 1, current[0].IndexVersion)
	require.NoError(t, repo.PromoteGeneration(ctx, v2))
	current, err = repo.ListCurrent(ctx, 10, 1, "embed-v1")
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, 2, current[0].IndexVersion)

	v3 := RagGeneration{ContentID: 10, IndexVersion: 3, ChunkingVersion: 1, EmbeddingModel: "embed-v1"}
	require.NoError(t, repo.StageGeneration(ctx, v3, []model.RagChunk{ragRepoChunk("c", 0)}))
	require.NoError(t, repo.MarkFailed(ctx, v3, "provider timeout"))
	current, err = repo.ListCurrent(ctx, 10, 1, "embed-v1")
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, 2, current[0].IndexVersion)
	// A failed non-current generation is retryable and can later promote.
	require.NoError(t, repo.StageGeneration(ctx, v3, []model.RagChunk{ragRepoChunk("d", 0)}))
	require.NoError(t, repo.PromoteGeneration(ctx, v3))
	current, err = repo.ListCurrent(ctx, 10, 1, "embed-v1")
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, 3, current[0].IndexVersion)

	wrongModel, err := repo.ListCurrent(ctx, 10, 1, "embed-v2")
	require.NoError(t, err)
	require.Empty(t, wrongModel)
}

func TestRagChunkRepositoryPublishedVersionGate(t *testing.T) {
	db := prepareRagRepositoryDatabase(t)
	repo := NewRagChunkRepository(db)
	version, err := repo.LatestPublishedVersion(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, version.VersionNumber)

	require.NoError(t, db.Exec(`UPDATE content_items SET status = 'pending' WHERE id = 10`).Error)
	_, err = repo.LatestPublishedVersion(context.Background(), 10)
	require.ErrorIs(t, err, ErrRagGenerationNotFound)
}

func TestLatestPublishedVersionDistinguishesPublishedTruthGap(t *testing.T) {
	db := prepareRagRepositoryDatabase(t)
	require.NoError(t, db.Exec(`UPDATE content_versions SET is_latest = FALSE WHERE content_item_id = 10`).Error)

	_, err := NewRagChunkRepository(db).LatestPublishedVersion(context.Background(), 10)
	require.ErrorIs(t, err, ErrRagPublishedVersionUnavailable)

	require.NoError(t, db.Exec(`UPDATE content_items SET status = 'banned' WHERE id = 10`).Error)
	_, err = NewRagChunkRepository(db).LatestPublishedVersion(context.Background(), 10)
	require.ErrorIs(t, err, ErrRagGenerationNotFound)
}

func TestLatestPublishedVersionChoosesHighestVersionWhenLatestFlagsAreCorrupt(t *testing.T) {
	db := prepareRagRepositoryDatabase(t)
	require.NoError(t, db.Exec(`INSERT INTO content_versions
		(content_item_id, author_id, version_number, storage_type, storage_key, status, is_latest)
		VALUES (10, 1, 2, 'full', 'new body', 'active', TRUE)`).Error)

	version, err := NewRagChunkRepository(db).LatestPublishedVersion(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 2, version.VersionNumber)
}

func ragRepoChunk(key string, index int) model.RagChunk {
	return model.RagChunk{ContentVersion: 1, ChunkIndex: index, ChunkKey: strings.Repeat(key, 64), Heading: "", Text: key, SourceStart: 0, SourceEnd: 1, Zone: "original", ContentType: "guide", Tags: pq.StringArray{}}
}

func prepareRagRepositoryDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	for _, statement := range []string{
		`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`,
		`CREATE TABLE ips (id BIGSERIAL PRIMARY KEY)`,
		`CREATE TABLE content_items (id BIGSERIAL PRIMARY KEY, author_id BIGINT NOT NULL REFERENCES users(id), description TEXT NOT NULL DEFAULT '', status VARCHAR(20) NOT NULL, deleted_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE content_versions (id BIGSERIAL PRIMARY KEY, content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE, parent_version_id BIGINT, author_id BIGINT NOT NULL REFERENCES users(id), version_number INT NOT NULL, storage_type VARCHAR(10) NOT NULL, storage_key TEXT, diff_summary TEXT, status VARCHAR(20) NOT NULL DEFAULT 'active', is_latest BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(content_item_id, version_number))`,
		`INSERT INTO users (id) VALUES (1)`,
		`INSERT INTO content_items (id, author_id, description, status) VALUES (10, 1, 'body', 'published')`,
	} {
		require.NoError(t, db.Exec(statement).Error)
	}
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "071_rag_chunks.sql"))
	return db
}
