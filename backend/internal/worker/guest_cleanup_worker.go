package worker

// #854 guest conversation retention loop. The deletion unit lives in the
// service layer (GuestConversationCleaner.RunOnce, directly testable); this
// file only owns the lifecycle: a fixed-interval ticker bounded by the
// worker process context, so stopping the worker pauses cleanup without
// touching the REST/SSE surface (ADR 0005 posture).

import (
	"context"
	"log/slog"
	"time"

	"omnicraft/backend/internal/service"
)

// GuestCleanupInterval is the sweep cadence. Seven-day retention with an
// hourly sweep keeps expired rows readable-rejectable for at most one extra
// hour — read/write paths refuse them immediately regardless.
const GuestCleanupInterval = time.Hour

// RunGuestConversationCleanup sweeps expired guest conversations until the
// context is cancelled. A sweep failure is logged and retried next tick:
// deleting history late is safe (reads/writes of expired rows are refused
// synchronously), so it never escalates to a crash loop.
func RunGuestConversationCleanup(ctx context.Context, cleaner *service.GuestConversationCleaner, interval time.Duration) {
	if interval <= 0 {
		interval = GuestCleanupInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	sweep := func() {
		deleted, err := cleaner.RunOnce(context.Background(), time.Now())
		if err != nil {
			slog.Error("guest conversation cleanup failed", "error", err)
			return
		}
		if deleted > 0 {
			slog.Info("guest conversation cleanup swept", "deleted", deleted)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweep()
		}
	}
}
