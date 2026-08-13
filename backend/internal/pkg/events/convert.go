package events

import (
	"time"

	"omnicraft/backend/internal/model"
)

// ToOutboxEvent maps an envelope onto the outbox storage row. EventID is
// filled by the database (BIGSERIAL) on insert and doubles as the stable
// event_id for retries and inbox dedup.
func ToOutboxEvent(env Envelope) model.OutboxEvent {
	return model.OutboxEvent{
		AggregateID:   env.AggregateID,
		EventType:     env.EventType,
		SchemaVersion: env.SchemaVersion,
		Payload:       env.Payload,
		Traceparent:   env.Traceparent,
		Tracestate:    env.Tracestate,
		Status:        model.OutboxStatusPending,
		NextAttemptAt: time.Now().UTC(),
	}
}

// FromOutboxEvent rebuilds the fixed envelope from a stored row: the row id is
// the envelope event_id, so a delivered event always carries the same id the
// producer committed.
func FromOutboxEvent(row model.OutboxEvent) Envelope {
	return Envelope{
		EventID:       row.ID,
		EventType:     row.EventType,
		SchemaVersion: row.SchemaVersion,
		AggregateID:   row.AggregateID,
		OccurredAt:    row.CreatedAt,
		Traceparent:   row.Traceparent,
		Tracestate:    row.Tracestate,
		Payload:       row.Payload,
	}
}
