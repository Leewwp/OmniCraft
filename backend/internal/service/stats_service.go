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

// #411 F3：字段语义按 zone 分野——全局请求 users = 非删除未封禁注册用户数
// （现行语义不变）；zone 请求 users = 该分区内已发布内容的去重作者数
// （字段名保持 users 以兼容既有响应结构与前端映射）。
type StatsZone string

const (
	StatsZoneGlobal   StatsZone = ""
	StatsZoneOriginal StatsZone = "original"
	StatsZoneFanwork  StatsZone = "fanwork"
)

// ValidStatsZone 报告 zone 参数是否合法（空串 = 全局语义）。
func ValidStatsZone(zone string) bool {
	switch StatsZone(zone) {
	case StatsZoneGlobal, StatsZoneOriginal, StatsZoneFanwork:
		return true
	default:
		return false
	}
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

// statsCacheKeyForZone：缓存键按 zone 隔离——全局沿用旧键（不受分区
// 结果污染），分区各自独立（#411 F3）。
func statsCacheKeyForZone(zone StatsZone) string {
	if zone == StatsZoneGlobal {
		return statsCacheKey
	}
	return fmt.Sprintf("%s:zone:%s", statsCacheKey, zone)
}

// GetSummary 返回全局统计（现行语义：注册用户 / approved IP / 已发布内容）。
func (s *StatsService) GetSummary(ctx context.Context) (StatsSummary, error) {
	return s.getSummary(ctx, StatsZoneGlobal)
}

// GetSummaryForZone 返回分区统计（#411 F3）：内容数按 zone 过滤、
// 「创作者」= 该分区内已发布内容的去重作者数；「活跃 IP」语义不变
// （approved IP 全站口径）。
func (s *StatsService) GetSummaryForZone(ctx context.Context, zone string) (StatsSummary, error) {
	return s.getSummary(ctx, StatsZone(zone))
}

func (s *StatsService) getSummary(ctx context.Context, zone StatsZone) (StatsSummary, error) {
	cacheKey := statsCacheKeyForZone(zone)
	if s.rdb != nil {
		cached, err := s.rdb.Get(ctx, cacheKey).Bytes()
		if err == nil {
			var summary StatsSummary
			if err := json.Unmarshal(cached, &summary); err == nil {
				return summary, nil
			}
		}
	}

	var summary StatsSummary
	if zone == StatsZoneGlobal {
		if err := s.db.WithContext(ctx).Table("users").Where("deleted_at IS NULL AND is_banned = false").Count(&summary.Users).Error; err != nil {
			return summary, fmt.Errorf("count users: %w", err)
		}
		if err := s.db.WithContext(ctx).Table("content_items").Where(statusCountsAnonymousVisibleSQL()).Count(&summary.Contents).Error; err != nil {
			return summary, fmt.Errorf("count contents: %w", err)
		}
	} else {
		zoneFilter := s.db.WithContext(ctx).Table("content_items").
			Where(statusCountsAnonymousVisibleSQL()).
			Where("zone = ?", string(zone))
		if err := zoneFilter.Session(&gorm.Session{}).Count(&summary.Contents).Error; err != nil {
			return summary, fmt.Errorf("count contents by zone: %w", err)
		}
		/* 创作者 = 分区内已发布内容的去重作者数（不再复用全站注册用户数；
		   与内容数同 zone/status/软删口径，author_id 非空才计入）。 */
		if err := zoneFilter.Session(&gorm.Session{}).Where("author_id IS NOT NULL").
			Distinct("author_id").Count(&summary.Users).Error; err != nil {
			return summary, fmt.Errorf("count distinct authors by zone: %w", err)
		}
	}
	// IP 状态词表是 pending/approved/rejected/banned（"published" 是内容状态词，曾使 Active IPs 恒 0）
	if err := s.db.WithContext(ctx).Table("ips").Where("status = ?", "approved").Count(&summary.IPs).Error; err != nil {
		return summary, fmt.Errorf("count ips: %w", err)
	}

	if s.rdb != nil {
		data, _ := json.Marshal(summary)
		s.rdb.Set(ctx, cacheKey, data, statsCacheTTL)
	}

	return summary, nil
}

// statusCountsAnonymousVisibleSQL keeps public aggregates consistent with
// what an anonymous viewer can actually see (#446 / SP-16 P0): private,
// soft-deleted, banned and banned-author content must not inflate public
// counters.
func statusCountsAnonymousVisibleSQL() string {
	return "status = 'published' AND deleted_at IS NULL AND is_public = true" +
		" AND author_id NOT IN (SELECT id FROM users WHERE is_banned = true OR deleted_at IS NOT NULL)" +
		" AND (ip_id IS NULL OR ip_id NOT IN (SELECT id FROM ips WHERE status = 'banned'))"
}
