package service

import (
	"context"
	"log"
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

func (s *HotRankService) Run() {
	if s.cfg == nil || !s.cfg.Enabled {
		log.Println("[hot_rank] recommendation engine disabled, skipping")
		return
	}

	rankInterval := 10 * time.Minute
	log.Printf("[hot_rank] starting hot rank update every %v", rankInterval)

	s.updateRank()

	rankTicker := time.NewTicker(rankInterval)
	defer rankTicker.Stop()

	var embedTicker *time.Ticker
	var embedCh <-chan time.Time
	if s.recSvc != nil && s.embedProv != nil {
		embedTicker = time.NewTicker(1 * time.Minute)
		embedCh = embedTicker.C
		defer embedTicker.Stop()
		log.Println("[hot_rank] starting embedding gap fill every 1m")
	}

	for {
		select {
		case <-rankTicker.C:
			s.updateRank()
		case <-embedCh:
			s.fillEmbeddings()
		}
	}
}

func (s *HotRankService) updateRank() {
	if !s.mu.TryLock() {
		log.Println("[hot_rank] previous update still running, skipping")
		return
	}
	defer s.mu.Unlock()

	log.Println("[hot_rank] updating hot content rank...")
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
		log.Printf("[hot_rank] update failed: %v", err)
		return
	}
	log.Println("[hot_rank] hot content rank updated successfully")
}

func (s *HotRankService) fillEmbeddings() {
	if !s.embedMu.TryLock() {
		return
	}
	defer s.embedMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := s.recSvc.FillMissingEmbeddings(ctx, s.embedProv); err != nil {
		log.Printf("[hot_rank] embedding gap fill failed: %v", err)
	}
}
