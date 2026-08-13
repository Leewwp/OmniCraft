package model

import "time"

// Outbox event delivery states.
const (
	OutboxStatusPending = "pending"
	OutboxStatusSent    = "sent"
)

// OutboxEvent is one transactional-outbox row (migration 070). ID doubles as
// the stable event_id: retried deliveries and inbox dedup keys reuse it.
type OutboxEvent struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	AggregateID   int64      `gorm:"not null" json:"aggregate_id"`
	EventType     string     `gorm:"size:128;not null" json:"event_type"`
	SchemaVersion int        `gorm:"not null;default:1" json:"schema_version"`
	Payload       []byte     `gorm:"type:jsonb;not null" json:"payload"`
	Traceparent   string     `gorm:"size:55" json:"traceparent,omitempty"`
	Tracestate    string     `gorm:"size:512" json:"tracestate,omitempty"`
	Status        string     `gorm:"size:16;not null;default:pending" json:"status"`
	Attempts      int        `gorm:"not null;default:0" json:"attempts"`
	NextAttemptAt time.Time  `json:"next_attempt_at"`
	SentAt        *time.Time `json:"sent_at,omitempty"`
}

func (OutboxEvent) TableName() string { return "outbox_events" }

// InboxConsumer records one already-applied delivery per consumer group. The
// UNIQUE (consumer_group, event_id) constraint is the database-level
// at-least-once idempotency guard.
type InboxConsumer struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	ConsumerGroup string    `gorm:"size:64;not null" json:"consumer_group"`
	EventID       int64     `gorm:"not null" json:"event_id"`
	ConsumedAt    time.Time `json:"consumed_at"`
}

func (InboxConsumer) TableName() string { return "inbox_consumers" }
