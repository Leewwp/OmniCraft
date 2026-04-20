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

	var userIDs []int64
	r.db.Model(&model.Follow{}).Select("follower_id").
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Order("created_at DESC").
		Offset((page-1)*pageSize).Limit(pageSize).
		Pluck("follower_id", &userIDs)

	var users []model.User
	if len(userIDs) > 0 {
		r.db.Where("id IN ?", userIDs).Find(&users)
	}
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
