package rag

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/diffengine"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/testutil"
)

type deterministicChunkEmbedder struct {
	calls int
}

type failingChunkEmbedder struct{ err error }

type panickingChunkEmbedder struct{ value any }

func (e panickingChunkEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	panic(e.value)
}

type switchableChunkEmbedder struct {
	deterministicChunkEmbedder
	err error
}

func (e *switchableChunkEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if e.err != nil {
		return nil, e.err
	}
	return e.deterministicChunkEmbedder.Embed(ctx, texts)
}

func (f failingChunkEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	return nil, f.err
}

func (e *deterministicChunkEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	e.calls++
	vectors := make([][]float32, len(texts))
	for i := range texts {
		vectors[i] = make([]float32, 1536)
		vectors[i][0] = float32(len([]rune(texts[i])))
	}
	return vectors, nil
}

type recordingSearchProjection struct {
	documents         map[string][]SearchDocument
	calls             int
	deletes           int
	replaceErr        error
	pruneErr          error
	deleteErr         error
	created           []string
	deleted           []string
	aliasTarget       string
	aliasErr          error
	onSwap            func()
	validateErr       error
	validatedExpected int64
	swapErr           error
	swapApplies       bool
	blockIndex        string
	blockEntered      chan struct{}
	blockRelease      chan struct{}
	blockOnce         sync.Once
}

func (f *recordingSearchProjection) DeleteContent(_ context.Context, index string, contentID int64) error {
	f.deletes++
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return f.deleteContent(index, contentID)
}

func (f *recordingSearchProjection) deleteContent(index string, contentID int64) error {
	if f.documents == nil {
		return nil
	}
	documents := f.documents[index]
	kept := documents[:0]
	for _, document := range documents {
		if document.ContentID != contentID {
			kept = append(kept, document)
		}
	}
	f.documents[index] = kept
	return nil
}

func (f *recordingSearchProjection) CreateIndex(_ context.Context, index string) error {
	f.created = append(f.created, index)
	return nil
}

func (f *recordingSearchProjection) ValidateIndex(_ context.Context, index string, expected int64) error {
	f.validatedExpected = expected
	if int64(len(f.documents[index])) != expected {
		return errors.New("unexpected document count")
	}
	return f.validateErr
}

func (f *recordingSearchProjection) SwapAlias(_ context.Context, _ string, index string) error {
	if f.onSwap != nil {
		f.onSwap()
	}
	if f.swapErr == nil || f.swapApplies {
		f.aliasTarget = index
	}
	return f.swapErr
}

func (f *recordingSearchProjection) RemoveAlias(_ context.Context, _ string) error {
	f.aliasTarget = ""
	return nil
}

func (f *recordingSearchProjection) AliasTarget(context.Context, string) (string, error) {
	return f.aliasTarget, f.aliasErr
}

func (f *recordingSearchProjection) ListIndexes(_ context.Context, prefix string) ([]string, error) {
	seen := make(map[string]struct{})
	for index := range f.documents {
		if strings.HasPrefix(index, prefix) {
			seen[index] = struct{}{}
		}
	}
	for _, index := range f.created {
		if strings.HasPrefix(index, prefix) {
			seen[index] = struct{}{}
		}
	}
	if strings.HasPrefix(f.aliasTarget, prefix) {
		seen[f.aliasTarget] = struct{}{}
	}
	indexes := make([]string, 0, len(seen))
	for index := range seen {
		indexes = append(indexes, index)
	}
	return indexes, nil
}

func (f *recordingSearchProjection) DeleteIndex(_ context.Context, index string) error {
	f.deleted = append(f.deleted, index)
	delete(f.documents, index)
	return nil
}

func (f *recordingSearchProjection) UpsertContent(_ context.Context, index string, documents []SearchDocument) error {
	f.calls++
	if index == f.blockIndex && f.blockRelease != nil {
		f.blockOnce.Do(func() {
			if f.blockEntered != nil {
				close(f.blockEntered)
			}
			<-f.blockRelease
		})
	}
	if f.replaceErr != nil {
		return f.replaceErr
	}
	if f.documents == nil {
		f.documents = make(map[string][]SearchDocument)
	}
	ids := make(map[string]struct{}, len(documents))
	for _, document := range documents {
		ids[document.ID] = struct{}{}
	}
	kept := f.documents[index][:0]
	for _, document := range f.documents[index] {
		if _, replaced := ids[document.ID]; !replaced {
			kept = append(kept, document)
		}
	}
	f.documents[index] = kept
	f.documents[index] = append(f.documents[index], documents...)
	return nil
}

func (f *recordingSearchProjection) PruneStaleContent(_ context.Context, index string, contentID int64, keepIDs []string) error {
	if f.pruneErr != nil {
		return f.pruneErr
	}
	if f.documents == nil {
		f.documents = make(map[string][]SearchDocument)
	}
	keep := make(map[string]struct{}, len(keepIDs))
	for _, id := range keepIDs {
		keep[id] = struct{}{}
	}
	documents := f.documents[index]
	retained := documents[:0]
	for _, document := range documents {
		_, current := keep[document.ID]
		if document.ContentID != contentID || current {
			retained = append(retained, document)
		}
	}
	f.documents[index] = retained
	return nil
}

func TestProjectionSyncsPublishedContentToCurrentGeneration(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{}
	embedder := &deterministicChunkEmbedder{}
	projection := NewProjection(
		db,
		NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		embedder,
		search,
		ProjectionConfig{IndexVersion: 1, EmbeddingModel: "deterministic-1536"},
	)

	require.NoError(t, projection.SyncContent(context.Background(), 10))

	current, err := repository.NewRagChunkRepository(db).ListCurrent(context.Background(), 10, 1, "deterministic-1536")
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, "Guide\npublished body", current[0].Text)

	var embeddingCount int64
	require.NoError(t, db.Table("chunk_embeddings").Where("chunk_id = ? AND embedding_model = ?", current[0].ID, "deterministic-1536").Count(&embeddingCount).Error)
	require.Equal(t, int64(1), embeddingCount)

	documents := search.documents["omnicraft-rag-v1"]
	require.Len(t, documents, 1)
	require.Equal(t, current[0].ChunkKey, documents[0].ID)
	require.Equal(t, int64(10), documents[0].ContentID)
	require.Equal(t, 1, documents[0].ContentVersion)
	require.Equal(t, "published", documents[0].Status)
	require.Contains(t, search.created, "omnicraft-rag-v1", "incremental path must lazily seed the fixed generation mapping")
}

func TestProjectionFirstIncrementalSeedsFixedReadAlias(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{}
	projection := newTestProjection(db, search)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.Equal(t, "omnicraft-rag-v1", search.aliasTarget)
}

func TestProjectionRuntimeCannotOverrideFixedIndexIdentity(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{}
	projection := NewProjection(
		db,
		NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		&deterministicChunkEmbedder{},
		search,
		ProjectionConfig{IndexVersion: 1, EmbeddingModel: "deterministic-1536"},
	)

	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.Contains(t, search.created, "omnicraft-rag-v1")
	require.NotContains(t, search.created, "unsafe-v1")
	require.Equal(t, "omnicraft-rag-v1", search.aliasTarget)
}

func TestProjectionAliasLookupFailureNeverFallsBackToStalePostgresGeneration(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{}
	projection := newTestProjection(db, search)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	baselineCalls := search.calls
	search.aliasTarget = "omnicraft-rag-v2"
	search.aliasErr = errors.New("transient alias lookup failure")
	require.NoError(t, db.Exec(`UPDATE content_versions SET is_latest = FALSE WHERE content_item_id = 10`).Error)
	require.NoError(t, db.Exec(`INSERT INTO content_versions
		(content_item_id, author_id, version_number, storage_type, storage_key, status, is_latest)
		VALUES (10, 1, 2, 'full', 'must write generation two', 'active', TRUE)`).Error)

	require.ErrorIs(t, projection.SyncContent(context.Background(), 10), ErrProjectionUnavailable)
	require.Equal(t, baselineCalls, search.calls, "alias uncertainty must fail before writing the stale PG generation")
	search.aliasErr = nil
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.Equal(t, 2, search.documents["omnicraft-rag-v2"][0].ContentVersion)
}

func TestProjectionReplayOfCurrentContentIsNoOp(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{}
	embedder := &deterministicChunkEmbedder{}
	projection := NewProjection(
		db,
		NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		embedder,
		search,
		ProjectionConfig{IndexVersion: 1, EmbeddingModel: "deterministic-1536"},
	)

	require.NoError(t, projection.SyncContent(context.Background(), 10))
	current, err := repository.NewRagChunkRepository(db).ListCurrent(context.Background(), 10, 1, "deterministic-1536")
	require.NoError(t, err)
	require.Len(t, current, 1)
	chunkID := current[0].ID

	require.NoError(t, projection.SyncContent(context.Background(), 10))
	replayed, err := repository.NewRagChunkRepository(db).ListCurrent(context.Background(), 10, 1, "deterministic-1536")
	require.NoError(t, err)
	require.Len(t, replayed, 1)
	require.Equal(t, chunkID, replayed[0].ID)
	require.Equal(t, 1, embedder.calls, "current truth replay must not call the embedding provider again")
	require.Equal(t, 1, search.calls, "current truth replay must not rewrite OpenSearch")
}

func TestProjectionTitleOnlyUpdateRefreshesSearchDocument(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{}
	embedder := &deterministicChunkEmbedder{}
	projection := NewProjection(
		db,
		NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		embedder,
		search,
		ProjectionConfig{IndexVersion: 1, EmbeddingModel: "deterministic-1536"},
	)
	require.NoError(t, db.Exec(`UPDATE content_versions SET storage_key = '# Existing heading

body' WHERE content_item_id = 10`).Error)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.NoError(t, db.Exec(`UPDATE content_items SET title = 'Renamed guide', updated_at = NOW() + INTERVAL '1 second' WHERE id = 10`).Error)

	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.Equal(t, "Renamed guide", search.documents["omnicraft-rag-v1"][0].Title)
	require.Equal(t, 2, search.calls, "title truth changes must not be mistaken for an event replay")
	require.Equal(t, 2, embedder.calls)
}

func TestProjectionUpdatedTruthReplacesStaleChunksAndEmbeddings(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{}
	embedder := &deterministicChunkEmbedder{}
	projection := NewProjection(
		db,
		NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		embedder,
		search,
		ProjectionConfig{IndexVersion: 1, EmbeddingModel: "deterministic-1536"},
	)
	require.NoError(t, projection.SyncContent(context.Background(), 10))

	require.NoError(t, db.Exec(`UPDATE content_versions SET is_latest = FALSE WHERE content_item_id = 10`).Error)
	require.NoError(t, db.Exec(`INSERT INTO content_versions
		(content_item_id, author_id, version_number, storage_type, storage_key, status, is_latest)
		VALUES (10, 1, 2, 'full', 'replacement body', 'active', TRUE)`).Error)
	require.NoError(t, projection.SyncContent(context.Background(), 10))

	current, err := repository.NewRagChunkRepository(db).ListCurrent(context.Background(), 10, 1, "deterministic-1536")
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, 2, current[0].ContentVersion)
	require.Equal(t, "Guide\nreplacement body", current[0].Text)
	var totalChunks, totalEmbeddings int64
	require.NoError(t, db.Table("rag_chunks").Where("content_id = ? AND index_version = ?", 10, 1).Count(&totalChunks).Error)
	require.NoError(t, db.Table("chunk_embeddings").Count(&totalEmbeddings).Error)
	require.Equal(t, int64(1), totalChunks, "stale chunks must be removed")
	require.Equal(t, int64(1), totalEmbeddings, "stale embeddings must cascade with stale chunks")
	require.Equal(t, 2, search.documents["omnicraft-rag-v1"][0].ContentVersion)
	require.Equal(t, int64(10), search.documents["omnicraft-rag-v1"][0].ContentID)
	require.Equal(t, 1, search.documents["omnicraft-rag-v1"][0].IndexVersion)
	require.Equal(t, 1, search.documents["omnicraft-rag-v1"][0].ChunkingVersion)
}

func TestProjectionReconstructsDiffVersionBeforeChunking(t *testing.T) {
	db := prepareProjectionDatabase(t)
	patch := diffengine.ComputePatch("published body", "replacement body")
	require.NoError(t, db.Exec(`UPDATE content_versions SET is_latest = FALSE WHERE content_item_id = 10`).Error)
	require.NoError(t, db.Exec(`INSERT INTO content_versions
		(content_item_id, parent_version_id, author_id, version_number, storage_type, storage_key, status, is_latest)
		VALUES (10, 1, 1, 2, 'diff', ?, 'active', TRUE)`, patch).Error)

	search := &recordingSearchProjection{}
	projection := newTestProjection(db, search)
	require.NoError(t, projection.SyncContent(context.Background(), 10))

	current, err := repository.NewRagChunkRepository(db).ListCurrent(context.Background(), 10, 1, "deterministic-1536")
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, 2, current[0].ContentVersion)
	require.Equal(t, "Guide\nreplacement body", current[0].Text)
	require.Equal(t, "Guide\nreplacement body", search.documents["omnicraft-rag-v1"][0].Text)
}

func TestProjectionZeroChunkRefreshPrunesAllOldDocumentsWithoutBulk(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{}
	projection := newTestProjection(db, search)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	baselineCalls := search.calls
	require.NoError(t, db.Exec(`UPDATE content_items SET title = '', updated_at = NOW() + INTERVAL '1 second' WHERE id = 10`).Error)
	require.NoError(t, db.Exec(`UPDATE content_versions SET storage_key = '' WHERE content_item_id = 10`).Error)

	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.Equal(t, baselineCalls, search.calls, "zero chunks must skip bulk upsert")
	require.Empty(t, search.documents["omnicraft-rag-v1"])
	current, err := repository.NewRagChunkRepository(db).ListCurrent(context.Background(), 10, 1, "deterministic-1536")
	require.NoError(t, err)
	require.Empty(t, current)
}

func TestProjectionZeroChunkFullRebuildCreatesValidEmptyGeneration(t *testing.T) {
	db := prepareProjectionDatabase(t)
	require.NoError(t, db.Exec(`UPDATE content_items SET title = '' WHERE id = 10`).Error)
	require.NoError(t, db.Exec(`UPDATE content_versions SET storage_key = '' WHERE content_item_id = 10`).Error)
	search := &recordingSearchProjection{aliasTarget: "omnicraft-rag-v1"}
	projection := newTestProjection(db, search)

	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.NoError(t, projection.Rebuild(context.Background()))
	require.Equal(t, "omnicraft-rag-v2", search.aliasTarget)
	require.Empty(t, search.documents["omnicraft-rag-v2"])
	require.Zero(t, search.calls, "empty rebuild must not send an empty bulk request")
}

func TestProjectionUsesOneConnectionForLocksAndBusinessWork(t *testing.T) {
	db := prepareProjectionDatabase(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	search := &recordingSearchProjection{}
	projection := newTestProjection(db, search)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	require.NoError(t, projection.SyncContent(ctx, 10))
	require.NoError(t, projection.Rebuild(ctx))
}

func TestProjectionDiscardsPhysicalConnectionWhenUnlockIsAmbiguous(t *testing.T) {
	db := prepareProjectionDatabase(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	projection := newTestProjection(db, &recordingSearchProjection{})
	originalUnlock := projectionUnlockAll
	originalDiscard := projectionDiscardPhysicalConnection
	discards := 0
	projectionUnlockAll = func(context.Context, *gorm.DB) error { return errors.New("ambiguous unlock") }
	projectionDiscardPhysicalConnection = func(conn *sql.Conn) error {
		discards++
		return originalDiscard(conn)
	}
	t.Cleanup(func() {
		projectionUnlockAll = originalUnlock
		projectionDiscardPhysicalConnection = originalDiscard
	})

	require.ErrorIs(t, projection.SyncContent(context.Background(), 10), ErrProjectionUnavailable)
	require.Equal(t, 1, discards)
	projectionUnlockAll = originalUnlock
	require.NoError(t, projection.SyncContent(context.Background(), 10), "the pool must replace the discarded locked session")
}

func TestProjectionPanicCleansOrDiscardsSessionAndReturnsUnavailable(t *testing.T) {
	db := prepareProjectionDatabase(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	search := &recordingSearchProjection{}
	panicValue := &struct{ name string }{name: "provider panic"}
	projection := NewProjection(db, NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		panickingChunkEmbedder{value: panicValue}, search, ProjectionConfig{IndexVersion: 1, EmbeddingModel: "deterministic-1536"})
	originalUnlock := projectionUnlockAll
	originalDiscard := projectionDiscardPhysicalConnection
	discards := 0
	projectionUnlockAll = func(context.Context, *gorm.DB) error { return errors.New("ambiguous unlock during panic") }
	projectionDiscardPhysicalConnection = func(conn *sql.Conn) error {
		discards++
		return originalDiscard(conn)
	}
	t.Cleanup(func() {
		projectionUnlockAll = originalUnlock
		projectionDiscardPhysicalConnection = originalDiscard
	})

	require.ErrorIs(t, projection.SyncContent(context.Background(), 10), ErrProjectionUnavailable)
	require.Equal(t, 1, discards)
	projectionUnlockAll = originalUnlock
	require.NoError(t, newTestProjection(db, search).SyncContent(context.Background(), 10),
		"a panic must not return a locked or ambiguous physical session to the pool")
}

func TestProjectionCurrentRefreshFailuresKeepLastReadyProjection(t *testing.T) {
	for _, test := range []struct {
		name            string
		expectedVersion int
		configure       func(*switchableChunkEmbedder, *recordingSearchProjection, *gorm.DB)
	}{
		{name: "embedding", expectedVersion: 1, configure: func(embedder *switchableChunkEmbedder, _ *recordingSearchProjection, _ *gorm.DB) {
			embedder.err = errors.New("embedding unavailable")
		}},
		{name: "bulk", expectedVersion: 1, configure: func(_ *switchableChunkEmbedder, search *recordingSearchProjection, _ *gorm.DB) {
			search.replaceErr = errors.New("bulk unavailable")
		}},
		{name: "prune", expectedVersion: 2, configure: func(_ *switchableChunkEmbedder, search *recordingSearchProjection, _ *gorm.DB) {
			search.pruneErr = errors.New("prune unavailable")
		}},
		{name: "postgres", expectedVersion: 1, configure: func(_ *switchableChunkEmbedder, _ *recordingSearchProjection, db *gorm.DB) {
			require.NoError(t, db.Exec(`CREATE FUNCTION reject_rag_chunk_delete() RETURNS trigger LANGUAGE plpgsql AS $$
				BEGIN RAISE EXCEPTION 'injected projection write failure'; END $$`).Error)
			require.NoError(t, db.Exec(`CREATE TRIGGER reject_rag_chunk_delete BEFORE DELETE ON rag_chunks
				FOR EACH STATEMENT EXECUTE FUNCTION reject_rag_chunk_delete()`).Error)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := prepareProjectionDatabase(t)
			search := &recordingSearchProjection{}
			embedder := &switchableChunkEmbedder{}
			projection := NewProjection(db, NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}), embedder, search,
				ProjectionConfig{IndexVersion: 1, EmbeddingModel: "deterministic-1536"})
			require.NoError(t, projection.SyncContent(context.Background(), 10))
			require.NoError(t, db.Exec(`UPDATE content_versions SET is_latest = FALSE WHERE content_item_id = 10`).Error)
			require.NoError(t, db.Exec(`INSERT INTO content_versions
				(content_item_id, author_id, version_number, storage_type, storage_key, status, is_latest)
				VALUES (10, 1, 2, 'full', 'replacement must retry', 'active', TRUE)`).Error)
			test.configure(embedder, search, db)

			require.Error(t, projection.SyncContent(context.Background(), 10))
			current, err := repository.NewRagChunkRepository(db).ListCurrent(context.Background(), 10, 1, "deterministic-1536")
			require.NoError(t, err)
			require.Len(t, current, 1)
			require.Equal(t, test.expectedVersion, current[0].ContentVersion)
			var status model.IndexProjectionStatus
			require.NoError(t, db.Where("content_id = ? AND index_version = ?", 10, 1).Take(&status).Error)
			require.True(t, status.IsCurrent)
			require.Equal(t, "ready", status.State)
			oldSearchable := false
			for _, document := range search.documents["omnicraft-rag-v1"] {
				oldSearchable = oldSearchable || document.ContentVersion == 1
			}
			require.True(t, oldSearchable, "the last valid search projection must remain available")
			if test.name == "prune" {
				search.pruneErr = nil
				require.NoError(t, projection.SyncContent(context.Background(), 10), "retry must not no-op before stale ids are pruned")
				require.Len(t, search.documents["omnicraft-rag-v1"], 1)
				require.Equal(t, 2, search.documents["omnicraft-rag-v1"][0].ContentVersion)
			}
		})
	}
}

func TestProjectionSerializesSameContentAndConvergesToNewestTruth(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{blockIndex: "omnicraft-rag-v1", blockEntered: make(chan struct{}), blockRelease: make(chan struct{})}
	projection := newTestProjection(db, search)
	firstDone := make(chan error, 1)
	go func() { firstDone <- projection.SyncContent(context.Background(), 10) }()
	select {
	case <-search.blockEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("first projection did not reach search")
	}
	require.NoError(t, db.Exec(`UPDATE content_versions SET is_latest = FALSE WHERE content_item_id = 10`).Error)
	require.NoError(t, db.Exec(`INSERT INTO content_versions
		(content_item_id, author_id, version_number, storage_type, storage_key, status, is_latest)
		VALUES (10, 1, 2, 'full', 'newest serialized truth', 'active', TRUE)`).Error)
	secondDone := make(chan error, 1)
	go func() { secondDone <- projection.SyncContent(context.Background(), 10) }()
	close(search.blockRelease)
	require.NoError(t, <-firstDone)
	require.NoError(t, <-secondDone)
	require.Equal(t, 2, search.documents["omnicraft-rag-v1"][0].ContentVersion)
	current, err := repository.NewRagChunkRepository(db).ListCurrent(context.Background(), 10, 1, "deterministic-1536")
	require.NoError(t, err)
	require.Equal(t, 2, current[0].ContentVersion)
}

func TestProjectionBannedTruthRemovesAllSearchableProjections(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{}
	projection := NewProjection(
		db,
		NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		&deterministicChunkEmbedder{},
		search,
		ProjectionConfig{IndexVersion: 1, EmbeddingModel: "deterministic-1536"},
	)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.NoError(t, db.Exec(`UPDATE content_items SET status = 'banned' WHERE id = 10`).Error)

	require.NoError(t, projection.SyncContent(context.Background(), 10))

	current, err := repository.NewRagChunkRepository(db).ListCurrent(context.Background(), 10, 1, "deterministic-1536")
	require.NoError(t, err)
	require.Empty(t, current)
	var chunks, embeddings, statuses int64
	require.NoError(t, db.Table("rag_chunks").Where("content_id = ?", 10).Count(&chunks).Error)
	require.NoError(t, db.Table("chunk_embeddings").Count(&embeddings).Error)
	require.NoError(t, db.Table("index_projection_status").Where("content_id = ?", 10).Count(&statuses).Error)
	require.Zero(t, chunks)
	require.Zero(t, embeddings)
	require.Zero(t, statuses)
	require.Equal(t, 1, search.deletes)
}

func TestProjectionPartialDeleteFailureKeepsPostgresUntilRetryConverges(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{}
	projection := newTestProjection(db, search)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.NoError(t, db.Exec(`UPDATE content_items SET status = 'banned' WHERE id = 10`).Error)
	search.deleteErr = repository.ErrOpenSearchUnavailable

	require.Error(t, projection.SyncContent(context.Background(), 10))
	var statuses int64
	require.NoError(t, db.Table("index_projection_status").Where("content_id = ?", 10).Count(&statuses).Error)
	require.Equal(t, int64(1), statuses)
	search.deleteErr = nil
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.NoError(t, db.Table("index_projection_status").Where("content_id = ?", 10).Count(&statuses).Error)
	require.Zero(t, statuses)
}

func TestProjectionDeletedAndMissingTruthConvergeToNoSearchDocuments(t *testing.T) {
	t.Run("soft deleted", func(t *testing.T) {
		db := prepareProjectionDatabase(t)
		search := &recordingSearchProjection{}
		projection := newTestProjection(db, search)
		require.NoError(t, projection.SyncContent(context.Background(), 10))
		require.NoError(t, db.Exec(`UPDATE content_items SET deleted_at = NOW() WHERE id = 10`).Error)
		require.NoError(t, projection.SyncContent(context.Background(), 10))
		require.Equal(t, 1, search.deletes)
	})
	t.Run("missing", func(t *testing.T) {
		db := prepareProjectionDatabase(t)
		search := &recordingSearchProjection{}
		projection := newTestProjection(db, search)
		require.NoError(t, projection.SyncContent(context.Background(), 999999))
		require.Equal(t, 1, search.deletes)
	})
}

func TestProjectionPublishedWithoutLatestActiveVersionIsRetryableFailure(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{}
	projection := newTestProjection(db, search)
	require.NoError(t, db.Exec(`UPDATE content_versions SET is_latest = FALSE WHERE content_item_id = 10`).Error)

	err := projection.SyncContent(context.Background(), 10)
	require.ErrorIs(t, err, ErrProjectionUnavailable)
	require.Zero(t, search.deletes, "a published truth gap must retry instead of being acknowledged as a purge")
}

func TestProjectionNonIndexableContentPurgesCurrentAndRollbackGenerations(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{aliasTarget: "omnicraft-rag-v1"}
	projection := newTestProjection(db, search)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.NoError(t, projection.Rebuild(context.Background()))
	require.NotEmpty(t, search.documents["omnicraft-rag-v1"])
	require.NotEmpty(t, search.documents["omnicraft-rag-v2"])
	// Simulate a retained rollback index whose per-content PG registration was
	// already cleaned. Purge must still cover the immediately previous index.
	require.NoError(t, db.Exec(`DELETE FROM rag_chunks WHERE content_id = 10 AND index_version = 1`).Error)
	require.NoError(t, db.Exec(`DELETE FROM index_projection_status WHERE content_id = 10 AND index_version = 1`).Error)
	require.NoError(t, db.Exec(`UPDATE content_items SET status = 'banned', updated_at = NOW() WHERE id = 10`).Error)

	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.Empty(t, search.documents["omnicraft-rag-v1"], "rollback generation must not resurrect banned content")
	require.Empty(t, search.documents["omnicraft-rag-v2"], "current generation must purge banned content")
	var statuses int64
	require.NoError(t, db.Table("index_projection_status").Where("content_id = ?", 10).Count(&statuses).Error)
	require.Zero(t, statuses)
}

func newTestProjection(db *gorm.DB, search *recordingSearchProjection) *Projection {
	return NewProjection(
		db,
		NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		&deterministicChunkEmbedder{},
		search,
		ProjectionConfig{IndexVersion: 1, EmbeddingModel: "deterministic-1536"},
	)
}

func TestProjectionEmbeddingFailureIsRetryableAndPersistedWithoutSecrets(t *testing.T) {
	db := prepareProjectionDatabase(t)
	projection := NewProjection(
		db,
		NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		failingChunkEmbedder{err: errors.New("provider secret sk-live-must-not-leak")},
		&recordingSearchProjection{},
		ProjectionConfig{IndexVersion: 1, EmbeddingModel: "deterministic-1536"},
	)

	err := projection.SyncContent(context.Background(), 10)
	require.Error(t, err, "provider failure must reach the worker so existing retry/DLQ remains active")
	require.NotContains(t, err.Error(), "sk-live-must-not-leak")

	var status struct {
		State        string
		ErrorSummary string
		IsCurrent    bool
	}
	require.NoError(t, db.Table("index_projection_status").Where("content_id = ? AND index_version = ?", 10, 1).Take(&status).Error)
	require.Equal(t, "failed", status.State)
	require.False(t, status.IsCurrent)
	require.NotEmpty(t, strings.TrimSpace(status.ErrorSummary))
	require.NotContains(t, status.ErrorSummary, "sk-live-must-not-leak")
}

func TestProjectionSearchFailureLeavesNoCurrentGenerationAndHidesDetails(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{replaceErr: errors.New("opensearch basic-auth secret")}
	projection := NewProjection(
		db,
		NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		&deterministicChunkEmbedder{},
		search,
		ProjectionConfig{IndexVersion: 1, EmbeddingModel: "deterministic-1536"},
	)

	err := projection.SyncContent(context.Background(), 10)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "basic-auth secret")
	var status struct {
		State        string
		ErrorSummary string
		IsCurrent    bool
	}
	require.NoError(t, db.Table("index_projection_status").Where("content_id = ? AND index_version = ?", 10, 1).Take(&status).Error)
	require.Equal(t, "failed", status.State)
	require.False(t, status.IsCurrent)
	require.Equal(t, "search projection unavailable", status.ErrorSummary)
}

func TestProjectionRetryConvergesAfterSearchRecovery(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{replaceErr: errors.New("unavailable")}
	projection := newTestProjection(db, search)
	require.ErrorIs(t, projection.SyncContent(context.Background(), 10), ErrProjectionUnavailable)
	search.replaceErr = nil
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	current, err := repository.NewRagChunkRepository(db).ListCurrent(context.Background(), 10, 1, "deterministic-1536")
	require.NoError(t, err)
	require.NotEmpty(t, current)
	var state string
	require.NoError(t, db.Table("index_projection_status").Select("state").
		Where("content_id = ? AND is_current = TRUE", 10).Scan(&state).Error)
	require.Equal(t, "ready", state)
}

func TestProjectionRebuildSwapsAliasBeforePromotingPostgresGeneration(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{}
	projection := NewProjection(
		db,
		NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		&deterministicChunkEmbedder{},
		search,
		ProjectionConfig{
			IndexVersion: 1, EmbeddingModel: "deterministic-1536",
		},
	)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	search.aliasTarget = "omnicraft-rag-v1"
	search.onSwap = func() {
		var currentVersion int
		require.NoError(t, db.Table("index_projection_status").Select("index_version").
			Where("content_id = ? AND is_current = TRUE", 10).Scan(&currentVersion).Error)
		require.Equal(t, 1, currentVersion, "alias must switch before PostgreSQL current generation flips")
	}

	require.NoError(t, projection.Rebuild(context.Background()))
	require.Contains(t, search.created, "omnicraft-rag-v2")
	require.Equal(t, "omnicraft-rag-v2", search.aliasTarget)
	var currentVersion int
	require.NoError(t, db.Table("index_projection_status").Select("index_version").
		Where("content_id = ? AND is_current = TRUE", 10).Scan(&currentVersion).Error)
	require.Equal(t, 2, currentVersion)
}

func TestProjectionAliasAheadOfFailedPromoteKeepsStagingForReconcile(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{aliasTarget: "omnicraft-rag-v1"}
	projection := newTestProjection(db, search)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.NoError(t, db.Exec(`CREATE FUNCTION reject_target_promote() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.index_version = 2 AND NEW.state = 'ready' THEN
				RAISE EXCEPTION 'injected promote failure';
			END IF;
			RETURN NEW;
		END $$`).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_target_promote BEFORE UPDATE ON index_projection_status
		FOR EACH ROW EXECUTE FUNCTION reject_target_promote()`).Error)

	require.Error(t, projection.Rebuild(context.Background()))
	require.Equal(t, "omnicraft-rag-v1", search.aliasTarget, "a failed promotion must restore the old read alias")
	var state string
	require.NoError(t, db.Table("index_projection_status").Select("state").
		Where("content_id = ? AND index_version = ?", 10, 2).Scan(&state).Error)
	require.Equal(t, "staging", state, "alias-ahead target must remain durable for reconciliation")
	require.NoError(t, db.Exec(`DROP TRIGGER reject_target_promote ON index_projection_status`).Error)
	// Model a process crash after the alias switch but before PostgreSQL
	// promotion; the next rebuild must reconcile the durable staging generation.
	search.aliasTarget = "omnicraft-rag-v2"
	require.NoError(t, projection.Rebuild(context.Background()))
	var current bool
	require.NoError(t, db.Table("index_projection_status").Select("is_current").
		Where("content_id = ? AND index_version = ?", 10, 2).Scan(&current).Error)
	require.True(t, current)
}

func TestProjectionRebuildValidationFailureKeepsOldAliasAndMarksTargetFailed(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{}
	projection := NewProjection(
		db,
		NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		&deterministicChunkEmbedder{},
		search,
		ProjectionConfig{
			IndexVersion: 1, EmbeddingModel: "deterministic-1536",
		},
	)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	search.aliasTarget = "omnicraft-rag-v1"
	search.validateErr = errors.New("invalid mapping with hidden details")

	err := projection.Rebuild(context.Background())
	require.ErrorIs(t, err, ErrProjectionUnavailable)
	require.Equal(t, "omnicraft-rag-v1", search.aliasTarget)
	var currentVersion int
	require.NoError(t, db.Table("index_projection_status").Select("index_version").
		Where("content_id = ? AND is_current = TRUE", 10).Scan(&currentVersion).Error)
	require.Equal(t, 1, currentVersion)
	var targetState string
	require.NoError(t, db.Table("index_projection_status").Select("state").
		Where("content_id = ? AND index_version = ?", 10, 2).Scan(&targetState).Error)
	require.Equal(t, "failed", targetState)
}

func TestProjectionRebuildClearsOrphanDocumentsFromReusedTargetIndex(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{
		aliasTarget: "omnicraft-rag-v1",
		documents: map[string][]SearchDocument{
			"omnicraft-rag-v2": {{ID: "orphan", ChunkKey: "orphan", ContentID: 999, Status: "published"}},
		},
	}
	projection := newTestProjection(db, search)
	require.NoError(t, projection.SyncContent(context.Background(), 10))

	require.NoError(t, projection.Rebuild(context.Background()))
	require.Contains(t, search.deleted, "omnicraft-rag-v2", "a non-alias rebuild target must be recreated, not reused")
	require.Equal(t, int64(1), search.validatedExpected)
	require.Len(t, search.documents["omnicraft-rag-v2"], 1)
	require.Equal(t, int64(10), search.documents["omnicraft-rag-v2"][0].ContentID)
}

func TestProjectionRebuildReadsAliasAfterAmbiguousSwapTimeout(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{aliasTarget: "omnicraft-rag-v1", swapErr: errors.New("timeout"), swapApplies: true}
	projection := NewProjection(
		db,
		NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		&deterministicChunkEmbedder{},
		search,
		ProjectionConfig{
			IndexVersion: 1, EmbeddingModel: "deterministic-1536",
		},
	)
	require.NoError(t, projection.SyncContent(context.Background(), 10))

	require.NoError(t, projection.Rebuild(context.Background()), "alias read-after-timeout must resolve an applied swap")
	require.Equal(t, "omnicraft-rag-v2", search.aliasTarget)
	var currentVersion int
	require.NoError(t, db.Table("index_projection_status").Select("index_version").
		Where("content_id = ? AND is_current = TRUE", 10).Scan(&currentVersion).Error)
	require.Equal(t, 2, currentVersion)
}

func TestProjectionRebuildReconcilesAliasAheadOfPostgresAfterCrash(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{aliasTarget: "omnicraft-rag-v1"}
	projection := NewProjection(
		db,
		NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		&deterministicChunkEmbedder{},
		search,
		ProjectionConfig{
			IndexVersion: 1, EmbeddingModel: "deterministic-1536",
		},
	)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.NoError(t, projection.Rebuild(context.Background()))
	require.Equal(t, "omnicraft-rag-v2", search.aliasTarget)

	// Simulate a crash after alias swap but before the PostgreSQL flip was
	// durably observed: the target remains staged while the old row is current.
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("index_projection_status").Where("content_id = ?", 10).
			Update("is_current", false).Error; err != nil {
			return err
		}
		if err := tx.Table("index_projection_status").Where("content_id = ? AND index_version = ?", 10, 1).
			Updates(map[string]any{"is_current": true, "state": "ready"}).Error; err != nil {
			return err
		}
		return tx.Table("index_projection_status").Where("content_id = ? AND index_version = ?", 10, 2).
			Updates(map[string]any{"is_current": false, "state": "staging"}).Error
	}))

	require.NoError(t, projection.Rebuild(context.Background()))
	require.Equal(t, "omnicraft-rag-v2", search.aliasTarget, "reconciliation must not create a third generation")
	var currentVersion int
	require.NoError(t, db.Table("index_projection_status").Select("index_version").
		Where("content_id = ? AND is_current = TRUE", 10).Scan(&currentVersion).Error)
	require.Equal(t, 2, currentVersion)
}

func TestProjectionRebuildRestoresCompleteAliasBeforeRebuildingIncompleteGeneration(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{aliasTarget: "omnicraft-rag-v1"}
	projection := newTestProjection(db, search)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.NoError(t, projection.Rebuild(context.Background()))
	require.NoError(t, db.Table("index_projection_status").
		Where("content_id = ? AND index_version = ?", 10, 2).
		Updates(map[string]any{"state": "failed", "is_current": false}).Error)
	require.NoError(t, db.Table("index_projection_status").
		Where("content_id = ? AND index_version = ?", 10, 1).
		Updates(map[string]any{"state": "ready", "is_current": true}).Error)

	require.NoError(t, projection.Rebuild(context.Background()))
	require.Equal(t, "omnicraft-rag-v3", search.aliasTarget)
	var currentVersion int
	require.NoError(t, db.Table("index_projection_status").Select("index_version").
		Where("content_id = ? AND is_current = TRUE", 10).Scan(&currentVersion).Error)
	require.Equal(t, 3, currentVersion, "an incomplete alias target must restore the last valid generation before rebuilding")
}

func TestProjectionCrashReconciliationStillAppliesGenerationRetention(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{aliasTarget: "omnicraft-rag-v1"}
	projection := newTestProjection(db, search)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.NoError(t, projection.Rebuild(context.Background()))
	require.NoError(t, projection.Rebuild(context.Background()))
	search.deleted = nil
	require.NoError(t, db.Exec(`INSERT INTO index_projection_status
		(content_id, index_version, chunking_version, embedding_model, state, error_summary, is_current)
		VALUES (10, 1, 1, 'deterministic-1536', 'failed', '', FALSE)`).Error)
	require.NoError(t, db.Table("index_projection_status").Where("content_id = ?", 10).Update("is_current", false).Error)
	require.NoError(t, db.Table("index_projection_status").Where("content_id = ? AND index_version = ?", 10, 2).
		Updates(map[string]any{"state": "ready", "is_current": true}).Error)
	require.NoError(t, db.Table("index_projection_status").Where("content_id = ? AND index_version = ?", 10, 3).
		Updates(map[string]any{"state": "staging", "is_current": false}).Error)

	require.NoError(t, projection.Rebuild(context.Background()))
	require.Contains(t, search.deleted, "omnicraft-rag-v1")
}

func TestProjectionEmptyRebuildKeepsIndexVersionsMonotonic(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{aliasTarget: "omnicraft-rag-v1"}
	projection := newTestProjection(db, search)
	require.NoError(t, db.Exec(`UPDATE content_items SET status = 'deleted' WHERE id = 10`).Error)

	require.NoError(t, projection.Rebuild(context.Background()))
	require.Equal(t, "omnicraft-rag-v2", search.aliasTarget)
	require.NoError(t, projection.Rebuild(context.Background()))
	require.Equal(t, "omnicraft-rag-v3", search.aliasTarget)
}

func TestProjectionRebuildWaitsForInFlightIncrementalProjection(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{aliasTarget: "omnicraft-rag-v1"}
	projection := NewProjection(
		db,
		NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		&deterministicChunkEmbedder{},
		search,
		ProjectionConfig{
			IndexVersion: 1, EmbeddingModel: "deterministic-1536",
		},
	)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.NoError(t, db.Exec(`UPDATE content_versions SET is_latest = FALSE WHERE content_item_id = 10`).Error)
	require.NoError(t, db.Exec(`INSERT INTO content_versions
		(content_item_id, author_id, version_number, storage_type, storage_key, status, is_latest)
		VALUES (10, 1, 2, 'full', 'incremental in flight', 'active', TRUE)`).Error)
	search.blockIndex = "omnicraft-rag-v1"
	search.blockEntered = make(chan struct{})
	search.blockRelease = make(chan struct{})
	syncDone := make(chan error, 1)
	go func() { syncDone <- projection.SyncContent(context.Background(), 10) }()
	select {
	case <-search.blockEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("incremental projection did not reach OpenSearch")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := projection.Rebuild(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NotContains(t, search.created, "omnicraft-rag-v2", "exclusive rebuild must not start while a shared incremental lock is held")
	close(search.blockRelease)
	require.NoError(t, <-syncDone)
}

func TestProjectionRebuildRetainsOnlyOnePreviousGeneration(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{aliasTarget: "omnicraft-rag-v1"}
	projection := NewProjection(
		db,
		NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1}),
		&deterministicChunkEmbedder{},
		search,
		ProjectionConfig{
			IndexVersion: 1, EmbeddingModel: "deterministic-1536",
		},
	)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.NoError(t, projection.Rebuild(context.Background()))
	require.NoError(t, projection.Rebuild(context.Background()))

	require.Equal(t, "omnicraft-rag-v3", search.aliasTarget)
	require.Contains(t, search.deleted, "omnicraft-rag-v1")
	var versions []int
	require.NoError(t, db.Table("index_projection_status").Distinct("index_version").Order("index_version").Pluck("index_version", &versions).Error)
	require.Equal(t, []int{2, 3}, versions)
}

func TestProjectionRetentionDeletesOpenSearchOnlyOlderGeneration(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{aliasTarget: "omnicraft-rag-v1"}
	projection := newTestProjection(db, search)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.NoError(t, projection.Rebuild(context.Background()))
	require.NoError(t, projection.Rebuild(context.Background()))
	search.deleted = nil
	search.documents["omnicraft-rag-v1"] = []SearchDocument{{ID: "orphan-v1", ContentID: 999}}
	search.documents["omnicraft-rag-vjunk"] = []SearchDocument{{ID: "unrelated", ContentID: 999}}

	require.NoError(t, projection.retainRebuildGenerations(context.Background(), search, 3, "omnicraft-rag-v2"))
	require.Contains(t, search.deleted, "omnicraft-rag-v1")
	require.NotContains(t, search.deleted, "omnicraft-rag-v2")
	require.NotContains(t, search.deleted, "omnicraft-rag-v3")
	require.NotContains(t, search.deleted, "omnicraft-rag-vjunk", "malformed fixed-prefix names must not enter retention")
}

func TestProjectionIncrementalFollowsCurrentReadAliasGenerationAfterRebuild(t *testing.T) {
	db := prepareProjectionDatabase(t)
	search := &recordingSearchProjection{}
	projection := newTestProjection(db, search)
	require.NoError(t, projection.SyncContent(context.Background(), 10))
	search.aliasTarget = "omnicraft-rag-v1"
	require.NoError(t, projection.Rebuild(context.Background()))
	require.Equal(t, "omnicraft-rag-v2", search.aliasTarget)
	require.NoError(t, db.Exec(`UPDATE content_versions SET is_latest = FALSE WHERE content_item_id = 10`).Error)
	require.NoError(t, db.Exec(`INSERT INTO content_versions
		(content_item_id, author_id, version_number, storage_type, storage_key, status, is_latest)
		VALUES (10, 1, 2, 'full', 'post rebuild update', 'active', TRUE)`).Error)

	require.NoError(t, projection.SyncContent(context.Background(), 10))
	require.NotEmpty(t, search.documents["omnicraft-rag-v2"])
	require.Equal(t, 2, search.documents["omnicraft-rag-v2"][0].ContentVersion)
	var currentVersion int
	require.NoError(t, db.Table("index_projection_status").Select("index_version").
		Where("content_id = ? AND is_current = TRUE", 10).Scan(&currentVersion).Error)
	require.Equal(t, 2, currentVersion)
}

func prepareProjectionDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	for _, statement := range []string{
		`CREATE TABLE users (id BIGSERIAL PRIMARY KEY)`,
		`CREATE TABLE ips (id BIGSERIAL PRIMARY KEY)`,
		`CREATE TABLE content_items (
			id BIGSERIAL PRIMARY KEY,
			author_id BIGINT NOT NULL REFERENCES users(id),
			title TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			zone VARCHAR(10) NOT NULL DEFAULT 'original',
			content_type VARCHAR(20) NOT NULL DEFAULT 'guide',
			category VARCHAR(50),
			ip_id BIGINT REFERENCES ips(id),
			status VARCHAR(20) NOT NULL,
			deleted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE content_tags (content_item_id BIGINT NOT NULL REFERENCES content_items(id), tag VARCHAR(50) NOT NULL, PRIMARY KEY(content_item_id, tag))`,
		`CREATE TABLE content_versions (
			id BIGSERIAL PRIMARY KEY,
			content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
			parent_version_id BIGINT,
			author_id BIGINT NOT NULL REFERENCES users(id),
			version_number INT NOT NULL,
			storage_type VARCHAR(10) NOT NULL,
			storage_key TEXT,
			diff_summary TEXT,
			status VARCHAR(20) NOT NULL DEFAULT 'active',
			is_latest BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(content_item_id, version_number)
		)`,
		`INSERT INTO users (id) VALUES (1)`,
		`INSERT INTO content_items (id, author_id, title, description, zone, content_type, category, status)
		 VALUES (10, 1, 'Guide', 'published body', 'original', 'guide', 'build', 'published')`,
		`INSERT INTO content_versions (content_item_id, author_id, version_number, storage_type, storage_key, status, is_latest)
		 VALUES (10, 1, 1, 'full', 'published body', 'active', TRUE)`,
		`INSERT INTO content_tags (content_item_id, tag) VALUES (10, 'featured')`,
	} {
		require.NoError(t, db.Exec(statement).Error)
	}
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "..", "migrations", "071_rag_chunks.sql"))
	return db
}
