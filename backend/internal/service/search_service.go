package service

import (
	"context"

	"omnicraft/backend/internal/repository"

	"github.com/redis/go-redis/v9"
)

type SearchService struct {
	searchRepo *repository.SearchRepository
	rdb        *redis.Client
}

func NewSearchService(searchRepo *repository.SearchRepository, rdb *redis.Client) *SearchService {
	return &SearchService{searchRepo: searchRepo, rdb: rdb}
}

func (s *SearchService) GetSuggestions(prefix string, limit int) ([]repository.SearchSuggestion, error) {
	return s.searchRepo.SearchSuggestions(prefix, limit)
}

type TrendingItem struct {
	Text  string `json:"text"`
	Score int64  `json:"score"`
}

func (s *SearchService) GetTrending(limit int) ([]TrendingItem, error) {
	if limit <= 0 {
		limit = 20
	}
	var results []TrendingItem
	if s.rdb == nil {
		return results, nil
	}
	ctx := context.Background()
	items, err := s.rdb.ZRevRangeWithScores(ctx, "rank:hot:contents", 0, int64(limit-1)).Result()
	if err != nil {
		return results, err
	}
	for _, z := range items {
		results = append(results, TrendingItem{
			Text:  z.Member.(string),
			Score: int64(z.Score),
		})
	}
	return results, nil
}
