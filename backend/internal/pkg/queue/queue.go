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

type QueueConfig struct {
	Enabled         bool  `mapstructure:"enabled"`
	MaxAttempts     int   `mapstructure:"max_attempts"`
	RetryBackoffSec []int `mapstructure:"retry_backoff_sec"`
	DLQTTLHours     int   `mapstructure:"dlq_ttl_hours"`
	MaxLen          int64 `mapstructure:"maxlen"`
	WorkerReview    int   `mapstructure:"worker_review"`
	WorkerNotif     int   `mapstructure:"worker_notification"`
	WorkerEmbedding int   `mapstructure:"worker_embedding"`
	WorkerCount     int   `mapstructure:"worker_count"`
}
