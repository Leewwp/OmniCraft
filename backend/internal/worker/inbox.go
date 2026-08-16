package worker

import (
	"context"
	"hash/fnv"

	"gorm.io/gorm"

	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
)

// Package worker hosts the background consumers (reviews, counters, indexer,
// notifications, relays, admin DLQ). Consumption is at-least-once; every topic
// guarantees at-most-once application through an inbox idempotency record:
// inbox_consumers holds UNIQUE(consumer_group, event_id), keyed by
// InboxEventID for stream messages, and a topic consumes through EXACTLY ONE
// of two paths — never both:
//
//   - ConsumeInboxTx runs a DB-bound business effect and the completion row in
//     a single gorm transaction: either both commit or neither does. Pick it
//     when the effect is a DB write in the transaction scope (content rows,
//     embedding rows). A failing effect rolls back its own writes together
//     with the record; a crash before commit re-runs everything on redelivery,
//     so duplicates are impossible and retries are clean.
//
//   - MarkConsumedInbox records the completion row AFTER a side effect that
//     cannot join the inbox transaction (Redis-only writes such as count
//     increments, external calls such as AI review, LLM embedding). Pick it
//     when the effect lives outside the DB transaction scope. A crash between
//     effect and record re-runs the effect on redelivery, so such effects MUST
//     be idempotent by construction (provider dedup keys, conditional updates,
//     eventual counters); duplicate records are no-ops.
//
// A message the worker cannot handle (unknown action / malformed envelope) is
// a permanent failure, not a silent skip: the consumer returns an error so the
// broker retries with exponential backoff and dead-letters the message once
// attempts are exhausted, leaving no inbox completion row behind.
//
// InboxEventID derives the stable idempotency key for messages that carry no
// outbox envelope (legacy topics such as content.review / notification.create
// / count.download / content.embedding). Redis re-delivers the same stream
// message id on ACK loss, so (consumer_group, message id) is stable across
// retries; the FNV-1a hash maps the string id into the BIGINT event_id column
// without collisions for realistically-sized streams.
func InboxEventID(group string, msg queue.Message) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(group))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(msg.ID))
	return int64(h.Sum64() & (1<<63 - 1))
}

// ConsumeInboxTx guards a DB-bound business effect with the inbox idempotency
// record: the completion row (inbox_consumers, UNIQUE(consumer_group,
// event_id)) and the effect commit or roll back in one transaction.
// alreadyConsumed is true when this (group, eventID) pair was applied before;
// the effect is then skipped. A failing effect rolls back both the inbox row
// and its own writes, so a retry (or replay) re-runs cleanly.
func ConsumeInboxTx(ctx context.Context, db *gorm.DB, group string, eventID int64, effect func(ctx context.Context, tx *gorm.DB) error) (alreadyConsumed bool, err error) {
	outbox := repository.NewOutboxRepository(db)
	var consumed bool
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wasConsumed, markErr := outbox.MarkConsumedTx(ctx, tx, group, eventID)
		if markErr != nil {
			return markErr
		}
		if wasConsumed {
			consumed = true
			return nil
		}
		return effect(ctx, tx)
	})
	return consumed, err
}

// MarkConsumedInbox records the (consumer_group, event_id) completion row
// after an effect that cannot join the inbox transaction (Redis-only writes
// like count increments, external review calls, LLM embedding). The caller
// runs its effect first: a crash before this record re-runs the effect on
// redelivery (at-least-once, the effect is idempotent by construction), never
// loses work. Duplicate records are no-ops.
func MarkConsumedInbox(ctx context.Context, db *gorm.DB, group string, eventID int64) error {
	outbox := repository.NewOutboxRepository(db)
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, err := outbox.MarkConsumedTx(ctx, tx, group, eventID)
		return err
	})
}
