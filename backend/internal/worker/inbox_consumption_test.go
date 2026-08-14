package worker

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/testutil"
)

func setupInboxDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "070_outbox_inbox.sql"))
	require.NoError(t, db.Exec(`CREATE TABLE probe_effects (id BIGSERIAL PRIMARY KEY, note TEXT NOT NULL)`).Error)
	return db
}

func probeEffectCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&probeEffectRow{}).Count(&n).Error)
	return n
}

type probeEffectRow struct{ ID int64 }

func (probeEffectRow) TableName() string { return "probe_effects" }

// TestConsumeInboxTxConcurrentOnce proves the UNIQUE (consumer_group, event_id)
// guard under concurrency: N consumers racing on the same event produce exactly
// one business effect and one inbox row.
func TestConsumeInboxTxConcurrentOnce(t *testing.T) {
	db := setupInboxDB(t)

	const goroutines = 8
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ConsumeInboxTx(context.Background(), db, "omnicraft-indexer", 42,
				func(ctx context.Context, tx *gorm.DB) error {
					return tx.Exec(`INSERT INTO probe_effects (note) VALUES ('effect')`).Error
				})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err, "concurrent consumers must not fail on the unique guard")
	}

	require.Equal(t, int64(1), probeEffectCount(t, db), "business effect must run exactly once")
	var inbox int64
	require.NoError(t, db.Model(&model.InboxConsumer{}).Count(&inbox).Error)
	require.Equal(t, int64(1), inbox)
}

func TestConsumeInboxTxSkipsAlreadyConsumed(t *testing.T) {
	db := setupInboxDB(t)
	effect := func(ctx context.Context, tx *gorm.DB) error {
		return tx.Exec(`INSERT INTO probe_effects (note) VALUES ('effect')`).Error
	}

	first, err := ConsumeInboxTx(context.Background(), db, "omnicraft-indexer", 7, effect)
	require.NoError(t, err)
	require.False(t, first, "first delivery is not a duplicate")

	second, err := ConsumeInboxTx(context.Background(), db, "omnicraft-indexer", 7, effect)
	require.NoError(t, err)
	require.True(t, second, "second delivery of the same (group, event_id) must be skipped")

	require.Equal(t, int64(1), probeEffectCount(t, db), "duplicate delivery must not repeat the effect")
}

func TestConsumeInboxTxRollsBackEffectAndRecordOnError(t *testing.T) {
	db := setupInboxDB(t)
	_, err := ConsumeInboxTx(context.Background(), db, "omnicraft-indexer", 9,
		func(ctx context.Context, tx *gorm.DB) error {
			require.NoError(t, tx.Exec(`INSERT INTO probe_effects (note) VALUES ('effect')`).Error)
			return errors.New("injected business failure")
		})
	require.Error(t, err)

	require.Zero(t, probeEffectCount(t, db), "failed effect must roll back")
	var inbox int64
	require.NoError(t, db.Model(&model.InboxConsumer{}).Count(&inbox).Error)
	require.Zero(t, inbox, "failed effect must not leave an inbox completion row")
}

func TestMarkConsumedInboxRecordsOnce(t *testing.T) {
	db := setupInboxDB(t)
	require.NoError(t, MarkConsumedInbox(context.Background(), db, "omnicraft-count", 55))
	require.NoError(t, MarkConsumedInbox(context.Background(), db, "omnicraft-count", 55))

	var inbox int64
	require.NoError(t, db.Model(&model.InboxConsumer{}).Count(&inbox).Error)
	require.Equal(t, int64(1), inbox)
	require.NoError(t, db.Model(&model.InboxConsumer{}).
		Where("consumer_group = ? AND event_id = ?", "omnicraft-count", 55).Count(&inbox).Error)
	require.Equal(t, int64(1), inbox)
}

func TestInboxEventIDStablePerStreamMessage(t *testing.T) {
	msg := queue.Message{ID: "1723500000000-0", Topic: "notification.create", Group: "omnicraft-notification"}
	first := InboxEventID(msg.Group, msg)
	second := InboxEventID(msg.Group, msg)
	require.Equal(t, first, second, "the idempotency key must be stable across retries of the same stream message")

	different := queue.Message{ID: "1723500000000-1", Topic: "notification.create", Group: "omnicraft-notification"}
	require.NotEqual(t, first, InboxEventID(different.Group, different), "different stream messages must map to different keys")

	otherGroup := queue.Message{ID: "1723500000000-0", Topic: "count.download", Group: "omnicraft-count"}
	require.NotEqual(t, first, InboxEventID(otherGroup.Group, otherGroup), "the consumer group must be part of the key")
}
