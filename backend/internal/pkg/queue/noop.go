package queue

import (
	"context"
	"log/slog"
)

type NoopProducer struct{}

func NewNoopProducer() *NoopProducer {
	return &NoopProducer{}
}

func (n *NoopProducer) Publish(ctx context.Context, topic string, payload []byte) error {
	slog.Warn("queue disabled: message dropped", "topic", topic)
	return nil
}