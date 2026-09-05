package queue

import (
	"context"
	"time"
)

type Message struct {
	ID        string            `json:"id"`
	Topic     string            `json:"topic"`
	Group     string            `json:"group,omitempty"`
	Payload   []byte            `json:"payload"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Attempts  int               `json:"attempts"`
	CreatedAt time.Time         `json:"created_at"`
}

type Handler func(ctx context.Context, msg Message) error

type Producer interface {
	Publish(ctx context.Context, topic string, payload []byte) error
}

type Consumer interface {
	Subscribe(ctx context.Context, topic string, group string, handler Handler) error
}

type Broker interface {
	Producer
	Consumer
	Close() error
	Stop()
}

// QueueConfig backs the queue/relay nodes of config.yaml (#138). Consumed
// only by the standalone worker process (ADR 0005): the API server never
// starts asynchronous consumers. Runtime truth lives in backend/config.yaml;
// docs/reference/config.md §7.1 mirrors the human-readable summary.
type QueueConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// MaxAttempts caps delivery attempts before an entry lands in the DLQ.
	MaxAttempts int `mapstructure:"max_attempts"`
	// RetryBackoffSec is the per-attempt backoff schedule in seconds.
	RetryBackoffSec []int `mapstructure:"retry_backoff_sec"`
	// DLQTTLHours bounds how long dead-letter entries are retained.
	DLQTTLHours int `mapstructure:"dlq_ttl_hours"`
	// MaxLen caps the Redis Stream length per topic (entries trimmed from the head).
	MaxLen int64 `mapstructure:"maxlen"`
	// WorkerReview/WorkerNotif/WorkerEmbedding size the per-topic consumer
	// counts for review/notification/embedding; WorkerCount is the default
	// for topics without a dedicated override (see container.StartWorkers).
	WorkerReview    int `mapstructure:"worker_review"`
	WorkerNotif     int `mapstructure:"worker_notification"`
	WorkerEmbedding int `mapstructure:"worker_embedding"`
	WorkerCount     int `mapstructure:"worker_count"`
}
