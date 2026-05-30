package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"omnicraft/backend/config"
)

type EmbeddingProvider interface {
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
}

type HotRankService struct {
	contentSvc *ContentService
	recSvc     *RecommendationService
	embedProv  EmbeddingProvider
	ipStatsSvc *IPStatsService
	cfg        *config.RecommendationConfig
	mu         sync.Mutex
	embedMu    sync.Mutex
}

func NewHotRankService(contentSvc *ContentService, cfg *config.RecommendationConfig) *HotRankService {
	return &HotRankService{contentSvc: contentSvc, cfg: cfg}
}

func (s *HotRankService) WithRecommendationService(recSvc *RecommendationService) *HotRankService {
	s.recSvc = recSvc
	return s
}

func (s *HotRankService) WithEmbeddingProvider(prov EmbeddingProvider) *HotRankService {
	s.embedProv = prov
	return s
}

func (s *HotRankService) WithIPStatsService(ipStatsSvc *IPStatsService) *HotRankService {
	s.ipStatsSvc = ipStatsSvc
	return s
}

func (s *HotRankService) Run(stop ...<-chan struct{}) {
	if s.cfg == nil || !s.cfg.Enabled {
		slog.Info("[hot_rank] recommendation engine disabled, skipping")
		return
	}

	rankInterval := 10 * time.Minute
	if s.cfg != nil && s.cfg.RankIntervalMin > 0 {
		rankInterval = time.Duration(s.cfg.RankIntervalMin) * time.Minute
	}
	slog.Info("[hot_rank] starting hot rank update", "interval", rankInterval)

	s.runAll()

	rankTicker := time.NewTicker(rankInterval)
	defer rankTicker.Stop()

	var embedTicker *time.Ticker
	var embedCh <-chan time.Time
	if s.recSvc != nil && s.embedProv != nil {
		embedTicker = time.NewTicker(1 * time.Minute)
		embedCh = embedTicker.C
		defer embedTicker.Stop()
		slog.Info("[hot_rank] starting embedding gap fill every 1m")
	}

	var stopCh <-chan struct{}
	if len(stop) > 0 {
		stopCh = stop[0]
	}

	for {
		select {
		case <-rankTicker.C:
			s.runAll()
		case <-embedCh:
			s.fillEmbeddings()
		case <-stopCh:
			slog.Info("[hot_rank] received stop signal, shutting down")
			return
		}
	}
}

func (s *HotRankService) runAll() {
	s.updateRank()
	s.updateIPCounts()
}

func (s *HotRankService) updateRank() {
	if !s.mu.TryLock() {
		slog.Info("[hot_rank] previous update still running, skipping")
		return
	}
	defer s.mu.Unlock()

	slog.Info("[hot_rank] updating hot content rank")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	windowDays := s.cfg.TrendingWindowDays
	if windowDays <= 0 {
		windowDays = 7
	}
	decayHours := s.cfg.HotDecayHours
	if decayHours <= 0 {
		decayHours = 48
	}

	if err := s.contentSvc.UpdateHotRank(ctx, windowDays, decayHours); err != nil {
		slog.Error("[hot_rank] update failed", "error", err)
		return
	}
	slog.Info("[hot_rank] hot content rank updated successfully")
}

func (s *HotRankService) updateIPCounts() {
	if s.ipStatsSvc == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := s.ipStatsSvc.UpdateCategoryCounts(ctx); err != nil {
		slog.Error("[hot_rank] ip category count update failed", "error", err)
	}
}

func (s *HotRankService) fillEmbeddings() {
	if !s.embedMu.TryLock() {
		return
	}
	defer s.embedMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := s.recSvc.FillMissingEmbeddings(ctx, s.embedProv); err != nil {
		slog.Error("[hot_rank] embedding gap fill failed", "error", err)
	}
}
