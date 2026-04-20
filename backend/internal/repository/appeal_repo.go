package repository

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type AppealRepository struct {
	db *gorm.DB
}

func NewAppealRepository(db *gorm.DB) *AppealRepository {
	return &AppealRepository{db: db}
}

func (r *AppealRepository) Create(a *model.Appeal) error {
	return r.db.Create(a).Error
}

func (r *AppealRepository) FindByID(id int64) (*model.Appeal, error) {
	var a model.Appeal
	err := r.db.First(&a, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

func (r *AppealRepository) ListByUser(userID int64, page, pageSize int) ([]model.Appeal, int64, error) {
	var total int64
	r.db.Model(&model.Appeal{}).Where("user_id = ?", userID).Count(&total)
	var appeals []model.Appeal
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset((page-1)*pageSize).Limit(pageSize).
		Find(&appeals).Error
	return appeals, total, err
}

func (r *AppealRepository) ListPending(page, pageSize int) ([]model.Appeal, int64, error) {
	var total int64
	r.db.Model(&model.Appeal{}).Where("status = 'pending'").Count(&total)
	var appeals []model.Appeal
	err := r.db.Where("status = 'pending'").
		Order("created_at ASC").
		Offset((page-1)*pageSize).Limit(pageSize).
		Find(&appeals).Error
	return appeals, total, err
}

func (r *AppealRepository) UpdateStatus(id int64, status, adminResponse string, resolvedBy int64) error {
	updates := map[string]interface{}{
		"status":      status,
		"resolved_by": resolvedBy,
	}
	if adminResponse != "" {
		updates["admin_response"] = adminResponse
	}
	return r.db.Model(&model.Appeal{}).Where("id = ?", id).Updates(updates).Error
}

func (r *AppealRepository) HasPendingAppeal(userID int64, targetType string, targetID int64) (bool, error) {
	var count int64
	err := r.db.Model(&model.Appeal{}).
		Where("user_id = ? AND target_type = ? AND target_id = ? AND status = 'pending'", userID, targetType, targetID).
		Count(&count).Error
	return count > 0, err
}
