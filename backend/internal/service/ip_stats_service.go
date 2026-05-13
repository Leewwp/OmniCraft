package service

import (
	"context"
	"log/slog"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const ipCategoryCountsKey = "ip_category_counts"

type IPStatsService struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewIPStatsService(db *gorm.DB, rdb *redis.Client) *IPStatsService {
	return &IPStatsService{db: db, rdb: rdb}
}

func (s *IPStatsService) UpdateCategoryCounts(ctx context.Context) error {
	if s.rdb == nil || s.db == nil {
		return nil
	}

	var rows []struct {
		Category string `gorm:"column:category"`
		Count    int64  `gorm:"column:count"`
	}
	s.db.Raw(`
		SELECT COALESCE(i.category, 'uncategorized') AS category, COUNT(*) AS count
		FROM content_items ci
		JOIN ips i ON i.id = ci.ip_id
		WHERE ci.zone = 'fanwork' AND ci.status = 'published'
		GROUP BY i.category
	`).Scan(&rows)

	pipe := s.rdb.Pipeline()
	pipe.Del(ctx, ipCategoryCountsKey)
	for _, r := range rows {
		pipe.HSet(ctx, ipCategoryCountsKey, r.Category, r.Count)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}
	slog.Info("[ip_stats] updated category counts", "count", len(rows))
	return nil
}

func (s *IPStatsService) GetCategoryCounts(ctx context.Context) (map[string]string, error) {
	if s.rdb == nil {
		return nil, nil
	}
	result, err := s.rdb.HGetAll(ctx, ipCategoryCountsKey).Result()
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return map[string]string{}, nil
	}
	return result, nil
}

func (s *IPStatsService) IncrCategoryCount(ctx context.Context, ipCategory string) {
	if s.rdb == nil {
		return
	}
	cat := ipCategory
	if cat == "" {
		cat = "uncategorized"
	}
	s.rdb.HIncrBy(ctx, ipCategoryCountsKey, cat, 1)
}

func (s *IPStatsService) DecrCategoryCount(ctx context.Context, ipCategory string) {
	if s.rdb == nil {
		return
	}
	cat := ipCategory
	if cat == "" {
		cat = "uncategorized"
	}
	s.rdb.HIncrBy(ctx, ipCategoryCountsKey, cat, -1)
}
