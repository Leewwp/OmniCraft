package service

import (
	"context"
	"encoding/json"
	"path/filepath"
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
	relayTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	relayTracestate  = "congo=t61rcWkgMzE"
)

// setupRelayTestDB wires an ephemeral Postgres (070 only) with miniredis and a
// real RedisStreamBroker so relay tests exercise the full XAdd path.
func setupRelayTestDB(t *testing.T) (*gorm.DB, *redis.Client) {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "070_outbox_inbox.sql"))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return db, rdb
}

func relayOutboxEvent(t *testing.T, aggregateID int64, eventType string) *model.OutboxEvent {
	t.Helper()
	env, err := events.NewContentEnvelope(eventType, aggregateID, relayTraceparent, relayTracestate,
		events.ContentEventPayload{ContentID: aggregateID, AuthorID: 7, ContentType: "article"})
	require.NoError(t, err)
	row := events.ToOutboxEvent(env)
	row.NextAttemptAt = time.Now().UTC().Add(-time.Minute)
	return &row
}

func newRelayService(db *gorm.DB, producer queue.Producer, outbox OutboxClaimer) *RelayService {
	if outbox == nil {
		outbox = repository.NewOutboxRepository(db)
	}
	return NewRelayService(outbox, producer, &queue.QueueConfig{RetryBackoffSec: []int{1, 2}})
}

func countStreamMessages(t *testing.T, rdb *redis.Client, topic string) int64 {
	t.Helper()
	n, err := rdb.XLen(context.Background(), "omnicraft:"+topic).Result()
	require.NoError(t, err)
	return n
}

func streamEnvelopes(t *testing.T, rdb *redis.Client, topic string) []events.Envelope {
	t.Helper()
	msgs, err := rdb.XRange(context.Background(), "omnicraft:"+topic, "-", "+").Result()
	require.NoError(t, err)
	var out []events.Envelope
	for _, m := range msgs {
		payloadStr, ok := m.Values["payload"].(string)
		require.True(t, ok, "stream message must carry a payload field")
		var env events.Envelope
		require.NoError(t, json.Unmarshal([]byte(payloadStr), &env))
		out = append(out, env)
	}
	return out
}

func TestRelayDeliversPendingEventsToStreams(t *testing.T) {
	db, rdb := setupRelayTestDB(t)
	broker := queue.NewRedisStreamBroker(rdb, &queue.QueueConfig{MaxLen: 100000, MaxAttempts: 3, RetryBackoffSec: []int{1}})
	relay := newRelayService(db, broker, nil)

	require.NoError(t, db.Create(relayOutboxEvent(t, 1001, events.TopicContentPublished)).Error)
	require.NoError(t, db.Create(relayOutboxEvent(t, 1002, events.TopicContentDeleted)).Error)

	delivered, err := relay.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, delivered)

	require.Equal(t, int64(1), countStreamMessages(t, rdb, events.TopicContentPublished))
	require.Equal(t, int64(1), countStreamMessages(t, rdb, events.TopicContentDeleted))

	env := streamEnvelopes(t, rdb, events.TopicContentPublished)[0]
	require.Equal(t, events.TopicContentPublished, env.EventType)
	require.Equal(t, int64(1001), env.AggregateID)
	require.Equal(t, relayTraceparent, env.Traceparent, "W3C trace context must survive the relay round-trip")
	require.Equal(t, relayTracestate, env.Tracestate)

	var row model.OutboxEvent
	require.NoError(t, db.First(&row, env.EventID).Error)
	require.Equal(t, model.OutboxStatusSent, row.Status)
	require.NotNil(t, row.SentAt)
}

func TestRelaySkipsAlreadySentEvents(t *testing.T) {
	db, rdb := setupRelayTestDB(t)
	broker := queue.NewRedisStreamBroker(rdb, &queue.QueueConfig{MaxLen: 100000, MaxAttempts: 3, RetryBackoffSec: []int{1}})
	relay := newRelayService(db, broker, nil)

	require.NoError(t, db.Create(relayOutboxEvent(t, 1, events.TopicContentPublished)).Error)
	require.NoError(t, db.Create(relayOutboxEvent(t, 2, events.TopicContentPublished)).Error)

	first, err := relay.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, first)
	require.Equal(t, int64(2), countStreamMessages(t, rdb, events.TopicContentPublished))

	second, err := relay.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, second, "sent events must never be delivered twice by the relay")
	require.Equal(t, int64(2), countStreamMessages(t, rdb, events.TopicContentPublished))
}

func TestRelaySkipsNotDueEvents(t *testing.T) {
	db, rdb := setupRelayTestDB(t)
	broker := queue.NewRedisStreamBroker(rdb, &queue.QueueConfig{MaxLen: 100000, MaxAttempts: 3, RetryBackoffSec: []int{1}})
	relay := newRelayService(db, broker, nil)

	due := relayOutboxEvent(t, 1, events.TopicContentPublished)
	require.NoError(t, db.Create(due).Error)
	future := relayOutboxEvent(t, 2, events.TopicContentUpdated)
	future.NextAttemptAt = time.Now().UTC().Add(time.Hour)
	require.NoError(t, db.Create(future).Error)

	delivered, err := relay.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, delivered, "only due events (next_attempt_at <= now) may be delivered")
	require.Equal(t, int64(1), countStreamMessages(t, rdb, events.TopicContentPublished))
	require.Equal(t, int64(0), countStreamMessages(t, rdb, events.TopicContentUpdated))
}

// TestRelayRecordsFailureBackoffOnDeliveryError proves a failed XAdd records
// the backoff on the row: attempts increments and the event stays pending but
// is not claimable again until next_attempt_at.
func TestRelayRecordsFailureBackoffOnDeliveryError(t *testing.T) {
	db, _ := setupRelayTestDB(t)
	relay := newRelayService(db, failingRelayProducer{}, nil)

	require.NoError(t, db.Create(relayOutboxEvent(t, 1, events.TopicContentPublished)).Error)
	var eventID int64
	require.NoError(t, db.Model(&model.OutboxEvent{}).Pluck("id", &eventID).Error)

	delivered, err := relay.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, delivered)

	var row model.OutboxEvent
	require.NoError(t, db.First(&row, eventID).Error)
	require.Equal(t, model.OutboxStatusPending, row.Status)
	require.Equal(t, 1, row.Attempts)
	require.True(t, row.NextAttemptAt.After(time.Now().UTC()), "failed delivery must schedule the backoff retry in the future")

	again, err := relay.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, again, "event must not be retried before next_attempt_at")
}

// failingRelayProducer is the XAdd failure injection seam: every publish fails.
type failingRelayProducer struct{}

func (failingRelayProducer) Publish(ctx context.Context, topic string, payload []byte) error {
	return context.DeadlineExceeded
}

// failOnceMarkSentOutbox wraps the real repository and fails the first MarkSent
// call, simulating a relay crash between XAdd and MarkSent (at-least-once).
type failOnceMarkSentOutbox struct {
	OutboxClaimer
	failed bool
}

func (f *failOnceMarkSentOutbox) MarkSent(ctx context.Context, ids []int64, now time.Time) error {
	if !f.failed {
		f.failed = true
		return context.DeadlineExceeded
	}
	return f.OutboxClaimer.MarkSent(ctx, ids, now)
}

// TestRelayCrashBetweenXAddAndMarkSentRedeliversSameEventID proves at-least-once
// semantics: when MarkSent fails after XAdd succeeded, the next run re-publishes
// the same envelope with the same event id (consumers dedup via inbox).
func TestRelayCrashBetweenXAddAndMarkSentRedeliversSameEventID(t *testing.T) {
	db, rdb := setupRelayTestDB(t)
	broker := queue.NewRedisStreamBroker(rdb, &queue.QueueConfig{MaxLen: 100000, MaxAttempts: 3, RetryBackoffSec: []int{1}})
	real := repository.NewOutboxRepository(db)
	relay := newRelayService(db, broker, &failOnceMarkSentOutbox{OutboxClaimer: real})

	require.NoError(t, db.Create(relayOutboxEvent(t, 42, events.TopicContentPublished)).Error)

	first, err := relay.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, first)
	require.Equal(t, int64(1), countStreamMessages(t, rdb, events.TopicContentPublished))

	second, err := relay.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, second, "the event must be re-delivered after the MarkSent crash")

	envs := streamEnvelopes(t, rdb, events.TopicContentPublished)
	require.Len(t, envs, 2, "both deliveries must reach the stream (consumers dedup)")
	require.Equal(t, envs[0].EventID, envs[1].EventID, "retried delivery must reuse the same event id")

	var row model.OutboxEvent
	require.NoError(t, db.First(&row, envs[0].EventID).Error)
	require.Equal(t, model.OutboxStatusSent, row.Status)
}
