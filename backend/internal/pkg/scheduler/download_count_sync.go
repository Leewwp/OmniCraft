package scheduler

import (
	"context"
	"log/slog"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/service"
)

type DownloadCountSync struct {
	contentSvc *service.ContentService
	interval   time.Duration
	stopCh     chan struct{}
}

func NewDownloadCountSync(svc *service.ContentService, cfg *config.CacheConfig) *DownloadCountSync {
	interval := time.Duration(cfg.ViewCountFlushInterval) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	return &DownloadCountSync{
		contentSvc: svc,
		interval:   interval,
		stopCh:     make(chan struct{}),
	}
}

func (s *DownloadCountSync) Start() {
	recovery.GoSafe(func() {
		slog.Info("[DownloadCountSync] Starting", "interval", s.interval)
		s.flush()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.flush()
			case <-s.stopCh:
				slog.Info("[DownloadCountSync] Stopped")
				return
			}
		}
	})
}

func (s *DownloadCountSync) Stop() {
	close(s.stopCh)
}

func (s *DownloadCountSync) flush() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.contentSvc.FlushDownloadCounts(ctx); err != nil {
		slog.Error("[DownloadCountSync] Flush error", "error", err)
	}
}