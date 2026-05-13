package scheduler

import (
	"context"
	"log/slog"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/service"
)

type ViewCountSync struct {
	contentSvc *service.ContentService
	interval   time.Duration
	stopCh     chan struct{}
}

func NewViewCountSync(svc *service.ContentService, cfg *config.CacheConfig) *ViewCountSync {
	interval := time.Duration(cfg.ViewCountFlushInterval) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	return &ViewCountSync{
		contentSvc: svc,
		interval:   interval,
		stopCh:     make(chan struct{}),
	}
}

func (s *ViewCountSync) Start() {
	recovery.GoSafe(func() {
		slog.Info("[ViewCountSync] Starting", "interval", s.interval)
		s.flush()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.flush()
			case <-s.stopCh:
				slog.Info("[ViewCountSync] Stopped")
				return
			}
		}
	})
}

func (s *ViewCountSync) Stop() {
	close(s.stopCh)
}

func (s *ViewCountSync) flush() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.contentSvc.FlushViewCounts(ctx); err != nil {
		slog.Error("[ViewCountSync] Flush error", "error", err)
	}
}
