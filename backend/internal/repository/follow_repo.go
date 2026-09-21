package repository

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type FollowRepository struct {
	db *gorm.DB
}

func NewFollowRepository(db *gorm.DB) *FollowRepository {
	return &FollowRepository{db: db}
}

func (r *FollowRepository) Follow(followerID int64, targetType string, targetID int64) error {
	follow := model.Follow{
		FollowerID: followerID,
		TargetType: targetType,
		TargetID:   targetID,
	}
	return r.db.Where(model.Follow{FollowerID: followerID, TargetType: targetType, TargetID: targetID}).
		FirstOrCreate(&follow).Error
}

func (r *FollowRepository) Unfollow(followerID int64, targetType string, targetID int64) error {
	return r.db.Where("follower_id = ? AND target_type = ? AND target_id = ?", followerID, targetType, targetID).
		Delete(&model.Follow{}).Error
}

func (r *FollowRepository) IsFollowing(followerID int64, targetType string, targetID int64) (bool, error) {
	var count int64
	err := r.db.Model(&model.Follow{}).
		Where("follower_id = ? AND target_type = ? AND target_id = ?", followerID, targetType, targetID).
		Count(&count).Error
	return count > 0, err
}

func (r *FollowRepository) GetFollowers(targetType string, targetID int64, page, pageSize int) ([]model.User, int64, error) {
	var total int64
	r.db.Model(&model.Follow{}).Where("target_type = ? AND target_id = ?", targetType, targetID).Count(&total)

	var users []model.User
	r.db.Table("users").
		Joins("JOIN follows ON follows.follower_id = users.id").
		Where("follows.target_type = ? AND follows.target_id = ?", targetType, targetID).
		Order("follows.created_at DESC").
		Offset((page-1)*pageSize).Limit(pageSize).
		Find(&users)

	return users, total, nil
}

func (r *FollowRepository) GetFollowing(followerID int64, page, pageSize int) ([]model.Follow, int64, error) {
	var total int64
	r.db.Model(&model.Follow{}).Where("follower_id = ?", followerID).Count(&total)

	var follows []model.Follow
	err := r.db.Where("follower_id = ?", followerID).
		Order("created_at DESC").
		Offset((page-1)*pageSize).Limit(pageSize).
		Find(&follows).Error
	return follows, total, err
}

func (r *FollowRepository) CountFollowers(targetType string, targetID int64) (int64, error) {
	var count int64
	err := r.db.Model(&model.Follow{}).
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Count(&count).Error
	return count, err
}

type FollowerDailyStats struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
	Lost  int64  `json:"lost"`
}

type FollowerSourceStats struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

type FollowerStatsResult struct {
	Total        int64                `json:"total"`
	NewThisMonth int64                `json:"new_this_month"`
	LostThisMonth int64               `json:"lost_this_month"`
	Daily        []FollowerDailyStats `json:"daily"`
	Sources      []FollowerSourceStats `json:"sources"`
}

func (r *FollowRepository) GetFollowerStats(userID int64, days int) (*FollowerStatsResult, error) {
	var total int64
	if err := r.db.Model(&model.Follow{}).Where("target_type = 'user' AND target_id = ?", userID).Count(&total).Error; err != nil {
		return nil, err
	}

	var newThisMonth int64
	if err := r.db.Model(&model.Follow{}).
		Where("target_type = 'user' AND target_id = ? AND created_at >= NOW() - INTERVAL '30 days'", userID).
		Count(&newThisMonth).Error; err != nil {
		return nil, err
	}

	// 掉粉统计在 follows 硬删模型下不可推导（无 deleted_at 列，Unfollow 即
	// 物理删除——SP-25 D1 裁决诚实缺失）：lost 恒 0，docs/reference/api.md
	// 标注「暂不支持」；恢复能力须先落 follows 软删迁移，本轮明确不做。
	var lostThisMonth int64

	// 每日曲线只承载真实的新增粉丝数（gained）；lost 子查询随 deleted_at
	// 谓词一并移除，输出列 lost 恒 0 保持响应形状稳定。
	var daily []FollowerDailyStats
	if err := r.db.Raw(`
		SELECT d.dt::date::text AS date,
			COALESCE(gained.cnt, 0) AS count,
			0 AS lost
		FROM generate_series(NOW() - (? || ' days')::interval, NOW(), '1 day') d(dt)
		LEFT JOIN (
			SELECT created_at::date AS dt, COUNT(*) AS cnt
			FROM follows
			WHERE target_type = 'user' AND target_id = ? AND created_at >= NOW() - (? || ' days')::interval
			GROUP BY created_at::date
		) gained ON gained.dt = d.dt::date
		ORDER BY d.dt
	`, days, userID, days).Scan(&daily).Error; err != nil {
		return nil, err
	}

	var sources []FollowerSourceStats
	if err := r.db.Raw(`
		SELECT COALESCE(NULLIF(ips.name, ''), 'direct') AS name, COUNT(*) AS value
		FROM follows f
		LEFT JOIN content_items ci ON f.follower_id = ci.user_id
		LEFT JOIN ips ON ci.ip_id = ips.id
		WHERE f.target_type = 'user' AND f.target_id = ?
		GROUP BY ips.name
		ORDER BY value DESC
		LIMIT 5
	`, userID).Scan(&sources).Error; err != nil {
		return nil, err
	}

	return &FollowerStatsResult{
		Total:         total,
		NewThisMonth:  newThisMonth,
		LostThisMonth: lostThisMonth,
		Daily:         daily,
		Sources:       sources,
	}, nil
}
