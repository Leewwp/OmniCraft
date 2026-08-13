package repository

import (
	"context"
	"time"

	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OutboxWriter is the transactional-outbox seam services depend on: one event
// written inside the caller's transaction. *OutboxRepository implements it;
// tests inject failing stubs to prove atomicity.
type OutboxWriter interface {
	CreateTx(ctx context.Context, tx *gorm.DB, event *model.OutboxEvent) error
}

// OutboxRepository owns the transactional outbox and inbox idempotency rows
// (migration 070). Producers write events through CreateTx inside their own
// business transaction; the relay claims pending rows with FOR UPDATE SKIP
// LOCKED, marks them sent on success and records backoff on failure. Retried
// deliveries reuse the same row, so the event id never changes.
type OutboxRepository struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// CreateTx persists an event inside the caller's transaction, so the business
// status transition and the outbox insert commit atomically (or roll back
// together). The row id is assigned by the database and becomes the stable
// event_id.
func (r *OutboxRepository) CreateTx(ctx context.Context, tx *gorm.DB, event *model.OutboxEvent) error {
	return tx.WithContext(ctx).Create(event).Error
}

// ClaimPending claims up to limit ready (status=pending, next_attempt_at<=now)
// events with row-level locking. FOR UPDATE SKIP LOCKED lets concurrent
// claimants skip rows another claim is processing instead of blocking on them;
// an uncommitted claim is never double-assigned.
func (r *OutboxRepository) ClaimPending(ctx context.Context, limit int, now time.Time) ([]model.OutboxEvent, error) {
	var events []model.OutboxEvent
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("status = ? AND next_attempt_at <= ?", model.OutboxStatusPending, now).
		Order("next_attempt_at ASC, id ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

// ClaimPendingTx is ClaimPending bound to the caller's transaction (used by
// tests and, in T03, by a relay claim that must stay inside one unit of work).
func (r *OutboxRepository) ClaimPendingTx(ctx context.Context, tx *gorm.DB, limit int, now time.Time) ([]model.OutboxEvent, error) {
	var events []model.OutboxEvent
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("status = ? AND next_attempt_at <= ?", model.OutboxStatusPending, now).
		Order("next_attempt_at ASC, id ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

// MarkSent flips delivered events to sent and stamps sent_at on the same rows,
// keeping the original ids. Only pending rows are touched, so a concurrent
// failure record cannot be overwritten.
func (r *OutboxRepository) MarkSent(ctx context.Context, ids []int64, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.OutboxEvent{}).
		Where("id IN ? AND status = ?", ids, model.OutboxStatusPending).
		Updates(map[string]interface{}{
			"status":  model.OutboxStatusSent,
			"sent_at": now,
		}).Error
}

// RecordFailure records one failed delivery: attempts increments and
// next_attempt_at moves to the caller-computed backoff deadline. The row (and
// therefore the event id) is never replaced, which is what keeps retry
// semantics at-least-once with a stable identity.
func (r *OutboxRepository) RecordFailure(ctx context.Context, id int64, nextAttemptAt time.Time) error {
	return r.db.WithContext(ctx).Model(&model.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"attempts":        gorm.Expr("attempts + 1"),
			"next_attempt_at": nextAttemptAt,
		}).Error
}
