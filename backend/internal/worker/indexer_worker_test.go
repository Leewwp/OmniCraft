package worker

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/events"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/testutil"
)

const (
	indexerGroup       = "omnicraft-indexer"
	indexerTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	indexerTracestate  = "congo=t61rcWkgMzE"
)

// recordingEmbedder is the indexer's async-projection seam: it counts how many
// times a content re-embedding was triggered (the T03 side effect).
type recordingEmbedder struct {
	calls atomic.Int64
}

type failingContentProjection struct{ err error }

func (f failingContentProjection) SyncContent(context.Context, int64) error { return f.err }

type recoveringContentProjection struct {
	calls atomic.Int64
	fail  atomic.Bool
}

func (p *recoveringContentProjection) SyncContent(context.Context, int64) error {
	p.calls.Add(1)
	if p.fail.Load() {
		return errors.New("transient alias lookup failure")
	}
	return nil
}

func (r *recordingEmbedder) EmbedContentAsync(contentItemID int64, text string) {
	r.calls.Add(1)
}

func setupIndexerDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "070_outbox_inbox.sql"))
	createIndexerContentTable(t, db)
	return db
}

// createIndexerContentTable creates the minimal content_items shape the
// indexer loads (id/title/description). Migration 022's content_embeddings
// references content_items, so the banned-event test needs it too.
func createIndexerContentTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`CREATE TABLE content_items (
		id BIGSERIAL PRIMARY KEY,
		title TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`).Error)
}

func seedIndexerContent(t *testing.T, db *gorm.DB, id int64, title string) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO content_items (id, title, description) VALUES (?, ?, 'desc')`, id, title).Error)
}

// indexerEnvelope builds the wire envelope exactly as the relay delivers it:
// the event_id is the outbox row id, filled by the relay before XAdd.
func indexerEnvelope(t *testing.T, eventType string, contentID, eventID int64) string {
	t.Helper()
	env, err := events.NewContentEnvelope(eventType, contentID, indexerTraceparent, indexerTracestate,
		events.ContentEventPayload{ContentID: contentID, AuthorID: 7, ContentType: "article"})
	require.NoError(t, err)
	env.EventID = eventID
	raw, err := json.Marshal(env)
	require.NoError(t, err)
	return string(raw)
}

func indexerInboxRows(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&model.InboxConsumer{}).Where("consumer_group = ?", indexerGroup).Count(&n).Error)
	return n
}

// TestIndexerConsumesPublishedEventIdempotently proves the full at-least-once
// path: the relay may duplicate an envelope (same event_id, new stream id) but
// the indexer applies the side effect exactly once thanks to the inbox guard.
func TestIndexerConsumesPublishedEventIdempotently(t *testing.T) {
	db := setupIndexerDB(t)
	seedIndexerContent(t, db, 5001, "idempotent title")
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	embedder := &recordingEmbedder{}
	broker := queue.NewRedisStreamBroker(rdb, &queue.QueueConfig{Enabled: true, MaxAttempts: 3, RetryBackoffSec: []int{0, 0}, MaxLen: 100000})
	defer broker.Stop()

	mgr := NewWorkerManager(broker)
	mgr.Register(events.TopicContentPublished, indexerGroup,
		NewIndexerWorker(db, embedder, repository.NewEmbeddingRepository(db), nil).Handle)
	require.NoError(t, mgr.Start(context.Background()))

	payload := indexerEnvelope(t, events.TopicContentPublished, 5001, 5001)
	require.NoError(t, broker.Publish(context.Background(), events.TopicContentPublished, []byte(payload)))
	require.NoError(t, broker.Publish(context.Background(), events.TopicContentPublished, []byte(payload)))

	deadline := time.Now().Add(8 * time.Second)
	for embedder.calls.Load() < 1 {
		if time.Now().After(deadline) {
			t.Fatal("indexer never consumed the published event")
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Give the duplicate delivery a chance to (incorrectly) fire too.
	time.Sleep(400 * time.Millisecond)

	require.Equal(t, int64(1), embedder.calls.Load(), "duplicate delivery must not repeat the projection side effect")
	require.Equal(t, int64(1), indexerInboxRows(t, db), "exactly one inbox completion row per (group, event_id)")
}

// TestIndexerBannedEventRemovesEmbeddingInSameTransaction proves the
// DB-bound side effect runs inside the inbox transaction: the embedding row
// disappears with the completion record, and redelivery stays a no-op.
func TestIndexerBannedEventRemovesEmbeddingInSameTransaction(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "019_pgvector.sql"))
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "070_outbox_inbox.sql"))
	createIndexerContentTable(t, db)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "022_content_embeddings.sql"))

	const contentID = 6001
	seedIndexerContent(t, db, contentID, "banned title")
	vector := "[" + strings.Repeat("0.5,", 1535) + "0.5]"
	require.NoError(t, db.Exec(`INSERT INTO content_embeddings (content_item_id, embedding, embedded_at) VALUES (?, ?, NOW())`, contentID, vector).Error)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	embedder := &recordingEmbedder{}
	broker := queue.NewRedisStreamBroker(rdb, &queue.QueueConfig{Enabled: true, MaxAttempts: 3, RetryBackoffSec: []int{0, 0}, MaxLen: 100000})
	defer broker.Stop()

	mgr := NewWorkerManager(broker)
	mgr.Register(events.TopicContentBanned, indexerGroup,
		NewIndexerWorker(db, embedder, repository.NewEmbeddingRepository(db), nil).Handle)
	require.NoError(t, mgr.Start(context.Background()))

	payload := indexerEnvelope(t, events.TopicContentBanned, contentID, contentID)
	require.NoError(t, broker.Publish(context.Background(), events.TopicContentBanned, []byte(payload)))
	require.NoError(t, broker.Publish(context.Background(), events.TopicContentBanned, []byte(payload)))

	deadline := time.Now().Add(8 * time.Second)
	for indexerInboxRows(t, db) < 1 {
		if time.Now().After(deadline) {
			t.Fatal("indexer never consumed the banned event")
		}
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(400 * time.Millisecond)

	var remaining int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM content_embeddings WHERE content_item_id = ?`, contentID).Scan(&remaining).Error)
	require.Zero(t, remaining, "banned event must remove the content embedding")
	require.Equal(t, int64(1), indexerInboxRows(t, db))
}

// TestIndexerPermanentFailureLandsInDLQWithConsumerGroup proves a permanently
// failing event (unknown content id) exhausts retries into the dead-letter
// stream and that the DLQ entry now carries the consumer_group field.
func TestIndexerPermanentFailureLandsInDLQWithConsumerGroup(t *testing.T) {
	db := setupIndexerDB(t)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	embedder := &recordingEmbedder{}
	broker := queue.NewRedisStreamBroker(rdb, &queue.QueueConfig{Enabled: true, MaxAttempts: 2, RetryBackoffSec: []int{0, 0}, MaxLen: 100000})
	defer broker.Stop()

	mgr := NewWorkerManager(broker)
	mgr.Register(events.TopicContentPublished, indexerGroup,
		NewIndexerWorker(db, embedder, repository.NewEmbeddingRepository(db), nil).Handle)
	require.NoError(t, mgr.Start(context.Background()))

	payload := indexerEnvelope(t, events.TopicContentPublished, 999999, 999999)
	require.NoError(t, broker.Publish(context.Background(), events.TopicContentPublished, []byte(payload)))

	deadline := time.Now().Add(8 * time.Second)
	var dlqEntries []redis.XMessage
	for {
		entries, err := rdb.XRange(context.Background(), "omnicraft:dead-letter", "-", "+").Result()
		require.NoError(t, err)
		if len(entries) > 0 {
			dlqEntries = entries
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("permanently failing event never reached the DLQ")
		}
		time.Sleep(50 * time.Millisecond)
	}

	var entry struct {
		OriginalTopic string `json:"original_topic"`
		OriginalID    string `json:"original_id"`
		ConsumerGroup string `json:"consumer_group"`
		Error         string `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(dlqEntries[len(dlqEntries)-1].Values["data"].(string)), &entry))
	require.Equal(t, events.TopicContentPublished, entry.OriginalTopic)
	require.Equal(t, indexerGroup, entry.ConsumerGroup, "DLQ entries must record the consumer group")
	require.NotEmpty(t, entry.Error)

	require.Zero(t, indexerInboxRows(t, db), "a failed event must never leave an inbox completion row")
}

func TestIndexerRejectsInvalidEnvelope(t *testing.T) {
	db := setupIndexerDB(t)
	embedder := &recordingEmbedder{}
	idx := NewIndexerWorker(db, embedder, repository.NewEmbeddingRepository(db), nil)

	err := idx.Handle(context.Background(), queue.Message{
		ID: "1-0", Topic: events.TopicContentPublished, Group: indexerGroup, Payload: []byte("not-json"),
	})
	require.Error(t, err, "garbage payloads are permanent failures, not silent skips")

	bad := indexerEnvelope(t, events.TopicContentPublished, 5001, 5001)
	bad = strings.Replace(bad, `"schema_version":1`, `"schema_version":0`, 1)
	err = idx.Handle(context.Background(), queue.Message{
		ID: "1-1", Topic: events.TopicContentPublished, Group: indexerGroup, Payload: []byte(bad),
	})
	require.Error(t, err, "schema-invalid envelopes are permanent failures")

	require.Zero(t, embedder.calls.Load())
	require.Zero(t, indexerInboxRows(t, db))
}

func TestIndexerMissingContentIsPermanentFailure(t *testing.T) {
	db := setupIndexerDB(t)
	embedder := &recordingEmbedder{}
	idx := NewIndexerWorker(db, embedder, repository.NewEmbeddingRepository(db), nil)

	err := idx.Handle(context.Background(), queue.Message{
		ID: "1-0", Topic: events.TopicContentPublished, Group: indexerGroup,
		Payload: []byte(indexerEnvelope(t, events.TopicContentPublished, 777777, 777777)),
	})
	require.Error(t, err, "an event for a missing content row must fail so it can be DLQ'd and replayed after the row exists")
	require.Zero(t, indexerInboxRows(t, db))
}

func TestIndexerProjectionFailurePreventsInboxCompletion(t *testing.T) {
	db := setupIndexerDB(t)
	seedIndexerContent(t, db, 5002, "retry projection")
	embedder := &recordingEmbedder{}
	idx := NewIndexerWorker(
		db,
		embedder,
		repository.NewEmbeddingRepository(db),
		failingContentProjection{err: errors.New("projection unavailable")},
	)

	err := idx.Handle(context.Background(), queue.Message{
		ID: "1-0", Topic: events.TopicContentPublished, Group: indexerGroup,
		Payload: []byte(indexerEnvelope(t, events.TopicContentPublished, 5002, 5002)),
	})
	require.ErrorContains(t, err, "projection unavailable")
	require.Zero(t, indexerInboxRows(t, db), "projection failure must remain retryable and never complete inbox")
	require.Zero(t, embedder.calls.Load(), "legacy embedding remains after successful inbox completion only")
}

func TestIndexerPublishedTruthGapRemainsRetryableWithoutInboxCompletion(t *testing.T) {
	db := setupIndexerDB(t)
	seedIndexerContent(t, db, 5003, "published without latest active version")
	idx := NewIndexerWorker(
		db,
		&recordingEmbedder{},
		repository.NewEmbeddingRepository(db),
		failingContentProjection{err: errors.New("published content version unavailable")},
	)

	err := idx.Handle(context.Background(), queue.Message{
		ID: "1-0", Topic: events.TopicContentPublished, Group: indexerGroup,
		Payload: []byte(indexerEnvelope(t, events.TopicContentPublished, 5003, 5003)),
	})
	require.ErrorContains(t, err, "published content version unavailable")
	require.Zero(t, indexerInboxRows(t, db))
}

func TestIndexerRetriesTransientAliasLookupWithoutCompletingInbox(t *testing.T) {
	db := setupIndexerDB(t)
	seedIndexerContent(t, db, 5004, "alias retry")
	projection := &recoveringContentProjection{}
	projection.fail.Store(true)
	idx := NewIndexerWorker(db, &recordingEmbedder{}, repository.NewEmbeddingRepository(db), projection)
	message := queue.Message{
		ID: "1-0", Topic: events.TopicContentUpdated, Group: indexerGroup,
		Payload: []byte(indexerEnvelope(t, events.TopicContentUpdated, 5004, 5004)),
	}

	require.ErrorContains(t, idx.Handle(context.Background(), message), "transient alias lookup failure")
	require.Zero(t, indexerInboxRows(t, db))
	projection.fail.Store(false)
	require.NoError(t, idx.Handle(context.Background(), message))
	require.Equal(t, int64(1), indexerInboxRows(t, db))
	require.Equal(t, int64(2), projection.calls.Load())
}
