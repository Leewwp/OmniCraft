package service

import (
	"context"
	"strconv"

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

func (s *SearchService) GetSuggestions(prefix string, limit int, viewerID int64) ([]repository.SearchSuggestion, error) {
	return s.searchRepo.SearchSuggestions(prefix, limit, viewerID)
}

type TrendingItem struct {
	Text      string `json:"text"`
	Score     int64  `json:"score"`
	ContentID int64  `json:"content_id"`
}

// hotRankContentsKey mirrors the hot rank ZSet maintained by likes/views and
// the UpdateHotRank rebuild (members are content IDs).
const hotRankContentsKey = "rank:hot:contents"

func (s *SearchService) GetTrending(limit int) ([]TrendingItem, error) {
	if limit <= 0 {
		limit = 20
	}
	results := []TrendingItem{}
	if s.rdb == nil {
		return results, nil
	}
	ctx := context.Background()
	// Over-fetch so invisible members (banned/under review) filtered out by
	// the visibility join can still leave a full window.
	fetch := int64(limit * 3)
	if fetch > 300 {
		fetch = 300
	}
	items, err := s.rdb.ZRevRangeWithScores(ctx, hotRankContentsKey, 0, fetch-1).Result()
	if err != nil {
		return results, err
	}
	ids := make([]int64, 0, len(items))
	for _, z := range items {
		if id, ok := parseHotRankMember(z.Member); ok {
			ids = append(ids, id)
		}
	}
	titles, err := s.searchRepo.ResolveTrendingContents(ctx, ids)
	if err != nil {
		return results, err
	}
	for _, z := range items {
		if len(results) >= limit {
			break
		}
		id, ok := parseHotRankMember(z.Member)
		if !ok {
			continue
		}
		title, visible := titles[id]
		if !visible || title == "" {
			continue
		}
		results = append(results, TrendingItem{
			Text:      title,
			Score:     int64(z.Score),
			ContentID: id,
		})
	}
	return results, nil
}

func parseHotRankMember(member interface{}) (int64, bool) {
	switch v := member.(type) {
	case string:
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, false
		}
		return id, true
	case int64:
		return v, true
	default:
		return 0, false
	}
}

func (s *SearchService) SearchContents(query, zone, category, contentType string, tagFilters []string, sort, timeRange string, page, pageSize int, viewerID int64) ([]repository.ContentSearchResult, int64, error) {
	return s.searchRepo.SearchContents(query, zone, category, contentType, tagFilters, sort, timeRange, page, pageSize, viewerID)
}

func (s *SearchService) SearchIPs(query, category string, page, pageSize int) ([]interface{}, int64, error) {
	ips, total, err := s.searchRepo.SearchIPs(query, category, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	results := make([]interface{}, len(ips))
	for i, ip := range ips {
		results[i] = ip
	}
	return results, total, nil
}

func (s *SearchService) SearchUsers(query string, page, pageSize int) ([]repository.UserSearchResult, int64, error) {
	return s.searchRepo.SearchUsers(query, page, pageSize)
}
