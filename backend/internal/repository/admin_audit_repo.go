package repository

import (
	"time"

	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type AdminAuditRepository struct {
	db *gorm.DB
}

func NewAdminAuditRepository(db *gorm.DB) *AdminAuditRepository {
	return &AdminAuditRepository{db: db}
}

func (r *AdminAuditRepository) Create(log *model.AdminAuditLog) error {
	return r.db.Create(log).Error
}

func (r *AdminAuditRepository) CreateTx(tx *gorm.DB, log *model.AdminAuditLog) error {
	return tx.Create(log).Error
}

func (r *AdminAuditRepository) List(filter AdminAuditFilter) ([]model.AdminAuditLog, int64, error) {
	q := r.db.Model(&model.AdminAuditLog{})

	if filter.Action != "" {
		q = q.Where("action = ?", filter.Action)
	}
	if filter.AdminUserID != 0 {
		q = q.Where("admin_user_id = ?", filter.AdminUserID)
	}
	if filter.From != nil {
		q = q.Where("created_at >= ?", filter.From)
	}
	if filter.To != nil {
		q = q.Where("created_at <= ?", filter.To)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.AdminAuditLog
	err := q.Order("created_at DESC").
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Find(&logs).Error
	return logs, total, err
}

type AdminAuditFilter struct {
	Action      string
	AdminUserID int64
	From        *time.Time
	To          *time.Time
	Page        int
	PageSize    int
}
