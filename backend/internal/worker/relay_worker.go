package worker

import (
	"context"
	"log/slog"
	"time"

	"omnicraft/backend/internal/service"
)

// RelayWorker runs the outbox relay in a loop until its context is
// cancelled. It is started only by cmd/worker (ADR 0005: the API server never
// runs asynchronous consumers). The poll interval is read from config.relay
// (issue #200); the outbox backoff gate (next_attempt_at) is the real delivery
// schedule, while the poll interval only bounds delivery latency.
type RelayWorker struct {
	svc      *service.RelayService
	interval time.Duration
}

func NewRelayWorker(svc *service.RelayService, interval time.Duration) *RelayWorker {
	return &RelayWorker{svc: svc, interval: interval}
}

// Start polls the relay until ctx is cancelled. Per-run failures are logged
// and the loop continues; a failed delivery never blocks the next batch
// because the outbox row carries its own backoff deadline.
func (w *RelayWorker) Start(ctx context.Context) error {
	w.runOnce(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *RelayWorker) runOnce(ctx context.Context) {
	delivered, err := w.svc.RunOnce(ctx)
	if err != nil {
		slog.Error("relay_worker: relay run failed", "error", err)
		return
	}
	if delivered > 0 {
		slog.Info("relay_worker: delivered outbox events", "count", delivered)
	}
}
