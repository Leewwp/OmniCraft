package service

import (
	"context"
	"log/slog"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const ipCategoryCountsKey = "ip:category:counts"

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

	// #843：口径改为纯 IP 维度——按 ips 表 status='approved' 统计每分类
	// approved IP 个数（NULL 类目按既有 COALESCE 语义归并 uncategorized，
	// 保留原行为），不再 JOIN content_items（原口径数的是 fanwork
	// published 内容行数，标签写「IP」实为内容数）。数据源只有本周期重建
	// （hot_rank），Incr/Decr 无调用点。
	var rows []struct {
		Category string `gorm:"column:category"`
		Count    int64  `gorm:"column:count"`
	}
	if err := s.db.Raw(`
		SELECT COALESCE(category, 'uncategorized') AS category, COUNT(*) AS count
		FROM ips
		WHERE status = 'approved'
		GROUP BY category
	`).Scan(&rows).Error; err != nil {
		return err
	}

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
