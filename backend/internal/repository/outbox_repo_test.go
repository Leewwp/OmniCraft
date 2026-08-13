package repository

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/events"
	"omnicraft/backend/internal/testutil"
)

const (
	validTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	validTracestate  = "congo=t61rcWkgMzE"
)

func setupOutboxRepoDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "070_outbox_inbox.sql"))
	return db
}

func outboxEvent(t *testing.T, aggregateID int64, eventType string, traceparent, tracestate string) *model.OutboxEvent {
	t.Helper()
	env, err := events.NewContentEnvelope(eventType, aggregateID, traceparent, tracestate,
		events.ContentEventPayload{ContentID: aggregateID, AuthorID: 7, ContentType: "article"})
	require.NoError(t, err)
	row := events.ToOutboxEvent(env)
	// Deterministic claim semantics: the default next_attempt_at is the insert
	// time, so tests pin it in the past and claim with an explicit now.
	row.NextAttemptAt = time.Now().UTC().Add(-time.Minute)
	return &row
}

// TestOutboxCreateInTransactionAtomicity proves the transactional-outbox
// property: an event written inside the caller's transaction commits with the
// business write, and a rolled-back transaction leaves no event behind.
func TestOutboxCreateInTransactionAtomicity(t *testing.T) {
	db := setupOutboxRepoDB(t)
	repo := NewOutboxRepository(db)

	err := db.Transaction(func(tx *gorm.DB) error {
		if err := repo.CreateTx(context.Background(), tx, outboxEvent(t, 1001, events.TopicContentPublished, "", "")); err != nil {
			return err
		}
		return tx.Exec(`CREATE TABLE atomic_probe (id BIGINT)`).Error
	})
	require.NoError(t, err)

	var committed int64
	require.NoError(t, db.Model(&model.OutboxEvent{}).Count(&committed).Error)
	require.Equal(t, int64(1), committed)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		if err := repo.CreateTx(context.Background(), tx, outboxEvent(t, 1002, events.TopicContentUpdated, "", "")); err != nil {
			return err
		}
		return nil
	}))
	_ = db.Transaction(func(tx *gorm.DB) error {
		if err := repo.CreateTx(context.Background(), tx, outboxEvent(t, 1003, events.TopicContentDeleted, "", "")); err != nil {
			return err
		}
		return context.DeadlineExceeded // force rollback
	})

	var total int64
	require.NoError(t, db.Model(&model.OutboxEvent{}).Count(&total).Error)
	require.Equal(t, int64(2), total, "rolled-back transaction must not leave outbox events")
}

// TestOutboxClaimPendingSkipsLocked proves the FOR UPDATE SKIP LOCKED row-level
// claim semantics: a concurrent claimant never blocks on rows another claim is
// processing, and claimed-but-uncommitted rows are not double-claimed.
func TestOutboxClaimPendingSkipsLocked(t *testing.T) {
	db := setupOutboxRepoDB(t)
	repo := NewOutboxRepository(db)
	now := time.Now()

	require.NoError(t, repo.CreateTx(context.Background(), db, outboxEvent(t, 1, events.TopicContentPublished, "", "")))
	require.NoError(t, repo.CreateTx(context.Background(), db, outboxEvent(t, 2, events.TopicContentUpdated, "", "")))

	lockTx := db.Begin()
	require.NoError(t, lockTx.Error)
	locked, err := repo.ClaimPendingTx(context.Background(), lockTx, 10, now)
	require.NoError(t, err)
	require.Len(t, locked, 2, "first claim takes both ready events")

	// A second claim from a different connection must not block on the rows
	// locked by lockTx: SKIP LOCKED returns nothing until the first claim
	// commits.
	second, err := repo.ClaimPending(context.Background(), 10, now)
	require.NoError(t, err)
	require.Empty(t, second, "SKIP LOCKED must skip rows locked by an uncommitted claim")

	require.NoError(t, lockTx.Commit().Error)

	after, err := repo.ClaimPending(context.Background(), 10, now)
	require.NoError(t, err)
	require.Len(t, after, 2, "events become claimable again only after the previous claim commits")
}

// TestOutboxClaimReadyOnlyDueEvents proves the next_attempt_at backoff gate:
// pending events whose next attempt is in the future stay unclaimed.
func TestOutboxClaimReadyOnlyDueEvents(t *testing.T) {
	db := setupOutboxRepoDB(t)
	repo := NewOutboxRepository(db)
	now := time.Now()

	require.NoError(t, repo.CreateTx(context.Background(), db, outboxEvent(t, 1, events.TopicContentPublished, "", "")))
	due := db.Exec(`
		INSERT INTO outbox_events (aggregate_id, event_type, schema_version, payload, status, attempts, next_attempt_at)
		VALUES (2, 'content.updated', 1, '{"content_id": 2, "author_id": 7}'::jsonb, 'pending', 3, ?)
	`, now.Add(30*time.Minute))
	require.NoError(t, due.Error)

	got, err := repo.ClaimPending(context.Background(), 10, now)
	require.NoError(t, err)
	require.Len(t, got, 1, "only due events (next_attempt_at <= now) are claimable")
	require.Equal(t, int64(1), got[0].AggregateID)
}

// TestOutboxRetryBackoffKeepsEventID proves retries reuse the same row: a
// failed delivery bumps attempts, schedules next_attempt_at in the future and
// never changes the event id; repeated failures accumulate attempts on the
// same id.
func TestOutboxRetryBackoffKeepsEventID(t *testing.T) {
	db := setupOutboxRepoDB(t)
	repo := NewOutboxRepository(db)
	now := time.Now()

	require.NoError(t, repo.CreateTx(context.Background(), db, outboxEvent(t, 1, events.TopicContentPublished, "", "")))
	var eventID int64
	require.NoError(t, db.Model(&model.OutboxEvent{}).Pluck("id", &eventID).Error)

	claimed, err := repo.ClaimPending(context.Background(), 10, now)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, eventID, claimed[0].ID)

	require.NoError(t, repo.RecordFailure(context.Background(), eventID, now.Add(time.Hour)))

	notDue, err := repo.ClaimPending(context.Background(), 10, now)
	require.NoError(t, err)
	require.Empty(t, notDue, "failed event is not claimable before next_attempt_at")

	again, err := repo.ClaimPending(context.Background(), 10, now.Add(2*time.Hour))
	require.NoError(t, err)
	require.Len(t, again, 1, "failed event becomes claimable after backoff")
	require.Equal(t, eventID, again[0].ID, "retry must reuse the same event id")
	require.Equal(t, 1, again[0].Attempts)

	require.NoError(t, repo.RecordFailure(context.Background(), eventID, now.Add(2*time.Hour)))
	third, err := repo.ClaimPending(context.Background(), 10, now.Add(3*time.Hour))
	require.NoError(t, err)
	require.Len(t, third, 1)
	require.Equal(t, eventID, third[0].ID)
	require.Equal(t, 2, third[0].Attempts)
}

// TestOutboxMarkSentReusesEventID proves a successful delivery marks the same
// row sent: sent_at is set, status flips to sent, the id is unchanged and the
// event is no longer claimable.
func TestOutboxMarkSentReusesEventID(t *testing.T) {
	db := setupOutboxRepoDB(t)
	repo := NewOutboxRepository(db)
	now := time.Now()

	require.NoError(t, repo.CreateTx(context.Background(), db, outboxEvent(t, 1, events.TopicContentPublished, "", "")))
	var eventID int64
	require.NoError(t, db.Model(&model.OutboxEvent{}).Pluck("id", &eventID).Error)

	claimed, err := repo.ClaimPending(context.Background(), 10, now)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	require.NoError(t, repo.MarkSent(context.Background(), []int64{claimed[0].ID}, now))

	var sent model.OutboxEvent
	require.NoError(t, db.First(&sent, eventID).Error)
	require.Equal(t, eventID, sent.ID, "marking sent must keep the original event id")
	require.Equal(t, model.OutboxStatusSent, sent.Status)
	require.NotNil(t, sent.SentAt)

	after, err := repo.ClaimPending(context.Background(), 10, now.Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, after, "sent events are never claimed again")
}

// TestOutboxTraceContextRoundTrip proves W3C trace context survives the full
// write -> read cycle through the outbox row and comes back identical inside
// the envelope.
func TestOutboxTraceContextRoundTrip(t *testing.T) {
	db := setupOutboxRepoDB(t)
	repo := NewOutboxRepository(db)

	env, err := events.NewContentEnvelope(events.TopicContentBanned, 42, validTraceparent, validTracestate,
		events.ContentEventPayload{ContentID: 42, AuthorID: 7, ContentType: "video", Status: "banned"})
	require.NoError(t, err)
	row := events.ToOutboxEvent(env)
	require.NoError(t, repo.CreateTx(context.Background(), db, &row))

	var stored model.OutboxEvent
	require.NoError(t, db.First(&stored, row.ID).Error)
	got := events.FromOutboxEvent(stored)

	require.Equal(t, validTraceparent, got.Traceparent)
	require.Equal(t, validTracestate, got.Tracestate)
	require.Equal(t, row.ID, got.EventID)
	require.Equal(t, events.TopicContentBanned, got.EventType)
	require.Equal(t, int64(42), got.AggregateID)
	require.Equal(t, events.ContentSchemaVersion, got.SchemaVersion)
	require.False(t, got.OccurredAt.IsZero())

	var payload events.ContentEventPayload
	require.NoError(t, json.Unmarshal(got.Payload, &payload))
	require.Equal(t, int64(42), payload.ContentID)
	require.Equal(t, int64(7), payload.AuthorID)
	require.Equal(t, "banned", payload.Status)
}
