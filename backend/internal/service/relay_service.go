package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/events"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
)

// OutboxClaimer is the relay's seam over the outbox rows: claim due pending
// events, mark delivered rows sent, record failed deliveries with backoff.
// Tests inject wrappers to simulate the crash window between XAdd and
// MarkSent (at-least-once redelivery).
type OutboxClaimer interface {
	ClaimPending(ctx context.Context, limit int, now time.Time) ([]model.OutboxEvent, error)
	MarkSent(ctx context.Context, ids []int64, now time.Time) error
	RecordFailure(ctx context.Context, id int64, nextAttemptAt time.Time) error
}

var _ OutboxClaimer = (*repository.OutboxRepository)(nil)

// RelayService polls the transactional outbox and delivers each pending event
// to its Redis Stream (`omnicraft:<event_type>`) with the fixed envelope as
// payload, then marks the row sent. Delivery is at-least-once: a crash
// between XAdd and MarkSent leaves the row pending, so the next run publishes
// the same envelope again with the same event id; consumers dedup via
// inbox_consumers.
type RelayService struct {
	outbox       OutboxClaimer
	producer     queue.Producer
	batchSize    int
	retryBackoff []int
}

// NewRelayService builds the relay. batchSize bounds how many due outbox
// events one run claims (issue #200: read from config.relay.batch_size,
// validated in release mode; never a hardcoded default). cfg supplies the
// retry backoff schedule (shared with the consumer retry policy); a nil or
// empty cfg falls back to the queue package defaults.
func NewRelayService(outbox OutboxClaimer, producer queue.Producer, batchSize int, cfg *queue.QueueConfig) *RelayService {
	relay := &RelayService{
		outbox:    outbox,
		producer:  producer,
		batchSize: batchSize,
	}
	if cfg != nil {
		relay.retryBackoff = queue.RetryBackoff(cfg)
	} else {
		relay.retryBackoff = queue.DefaultRetryBackoffSec
	}
	return relay
}

// RunOnce claims one batch of due pending events, publishes each envelope to
// its stream and marks the delivered rows sent. Failed deliveries are
// recorded with exponential backoff (attempts + next_attempt_at) and stay
// pending for a later run. Returns the number of events delivered this run.
func (s *RelayService) RunOnce(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	pending, err := s.outbox.ClaimPending(ctx, s.batchSize, now)
	if err != nil {
		return 0, err
	}
	if len(pending) == 0 {
		return 0, nil
	}

	var (
		delivered []int64
		count     int
	)
	for _, row := range pending {
		envelope := events.FromOutboxEvent(row)
		payload, marshalErr := json.Marshal(envelope)
		if marshalErr != nil {
			s.recordFailure(ctx, row, now, marshalErr)
			continue
		}
		if publishErr := s.producer.Publish(ctx, row.EventType, payload); publishErr != nil {
			s.recordFailure(ctx, row, now, publishErr)
			continue
		}
		delivered = append(delivered, row.ID)
		count++
	}

	if len(delivered) > 0 {
		if err := s.outbox.MarkSent(ctx, delivered, now); err != nil {
			// At-least-once: the XAdd already happened. Leave the rows
			// pending so the next run re-publishes the same event ids; the
			// consumer-side inbox guard makes the duplicate a no-op.
			slog.Error("relay: failed to mark events sent", "count", len(delivered), "error", err)
		}
	}
	return count, nil
}

func (s *RelayService) recordFailure(ctx context.Context, row model.OutboxEvent, now time.Time, cause error) {
	backoff := s.retryBackoff[row.Attempts%len(s.retryBackoff)]
	nextAttempt := now.Add(time.Duration(backoff) * time.Second)
	slog.Warn("relay: delivery failed, scheduling backoff retry",
		"event_id", row.ID, "event_type", row.EventType, "attempt", row.Attempts, "error", cause)
	if err := s.outbox.RecordFailure(ctx, row.ID, nextAttempt); err != nil {
		slog.Error("relay: failed to record delivery failure", "event_id", row.ID, "error", err)
	}
}
