package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type StatsSummary struct {
	Users    int64 `json:"users"`
	IPs      int64 `json:"ips"`
	Contents int64 `json:"contents"`
}

type StatsService struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewStatsService(db *gorm.DB, rdb *redis.Client) *StatsService {
	return &StatsService{db: db, rdb: rdb}
}

const statsCacheKey = "stats:summary"
const statsCacheTTL = 10 * time.Minute

func (s *StatsService) GetSummary(ctx context.Context) (StatsSummary, error) {
	if s.rdb != nil {
		cached, err := s.rdb.Get(ctx, statsCacheKey).Bytes()
		if err == nil {
			var summary StatsSummary
			if err := json.Unmarshal(cached, &summary); err == nil {
				return summary, nil
			}
		}
	}

	var summary StatsSummary
	if err := s.db.WithContext(ctx).Table("users").Where("deleted_at IS NULL AND is_banned = false").Count(&summary.Users).Error; err != nil {
		return summary, fmt.Errorf("count users: %w", err)
	}
	if err := s.db.WithContext(ctx).Table("ips").Where("status = ?", "published").Count(&summary.IPs).Error; err != nil {
		return summary, fmt.Errorf("count ips: %w", err)
	}
	if err := s.db.WithContext(ctx).Table("content_items").Where("status = ? AND deleted_at IS NULL", "published").Count(&summary.Contents).Error; err != nil {
		return summary, fmt.Errorf("count contents: %w", err)
	}

	if s.rdb != nil {
		data, _ := json.Marshal(summary)
		s.rdb.Set(ctx, statsCacheKey, data, statsCacheTTL)
	}

	return summary, nil
}
