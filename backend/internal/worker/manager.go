package worker

import (
	"context"
	"log/slog"
	"sync"

	"omnicraft/backend/internal/pkg/queue"
)

type WorkerManager struct {
	broker  queue.Broker
	topics  []topicSubscription
	wg      sync.WaitGroup
	cancel  context.CancelFunc
	stopped bool
	mu      sync.Mutex
}

type topicSubscription struct {
	topic   string
	group   string
	handler queue.Handler
}

func NewWorkerManager(broker queue.Broker) *WorkerManager {
	return &WorkerManager{
		broker: broker,
	}
}

func (m *WorkerManager) Register(topic, group string, handler queue.Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.topics = append(m.topics, topicSubscription{
		topic:   topic,
		group:   group,
		handler: handler,
	})
}

func (m *WorkerManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	childCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	for _, sub := range m.topics {
		if m.broker == nil {
			slog.Warn("worker_manager: broker is nil, skipping subscription", "topic", sub.topic)
			continue
		}
		if err := m.broker.Subscribe(childCtx, sub.topic, sub.group, sub.handler); err != nil {
			slog.Error("worker_manager: failed to subscribe", "topic", sub.topic, "error", err)
			cancel()
			return err
		}
		slog.Info("worker_manager: subscribed to topic", "topic", sub.topic, "group", sub.group)
	}

	slog.Info("worker_manager: all workers started", "topics", len(m.topics))
	return nil
}

func (m *WorkerManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopped {
		return
	}
	m.stopped = true

	if m.cancel != nil {
		m.cancel()
	}

	if m.broker != nil {
		m.broker.Stop()
	}

	slog.Info("worker_manager: all workers stopped")
}