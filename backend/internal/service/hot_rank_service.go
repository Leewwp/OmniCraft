package service

import (
	"context"
	"log"
	"sync"
	"time"

	"omnicraft/backend/config"
)

type HotRankService struct {
	contentSvc *ContentService
	cfg        *config.RecommendationConfig
	mu         sync.Mutex
}

func NewHotRankService(contentSvc *ContentService, cfg *config.RecommendationConfig) *HotRankService {
	return &HotRankService{contentSvc: contentSvc, cfg: cfg}
}

func (s *HotRankService) Run() {
	if s.cfg == nil || !s.cfg.Enabled {
		log.Println("[hot_rank] recommendation engine disabled, skipping hot rank updates")
		return
	}

	interval := 10 * time.Minute
	log.Printf("[hot_rank] starting scheduled hot rank update every %v", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.update()

	for range ticker.C {
		s.update()
	}
}

func (s *HotRankService) update() {
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
