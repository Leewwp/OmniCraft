package service

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type ReputationService struct {
	db *gorm.DB
}

func NewReputationService(db *gorm.DB) *ReputationService {
	return &ReputationService{db: db}
}

func (s *ReputationService) AddReputation(userID int64, delta int, reason string, relatedID *int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var relID *uint
		if relatedID != nil {
			v := uint(*relatedID)
			relID = &v
		}
		log := model.ReputationLog{
			UserID:    uint(userID),
			Delta:     delta,
			Reason:    reason,
			RelatedID: relID,
		}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		return tx.Model(&model.User{}).
			Where("id = ?", userID).
			UpdateColumn("reputation", gorm.Expr("reputation + ?", delta)).Error
	})
}

func (s *ReputationService) GetLogs(userID int64, page, pageSize int) ([]model.ReputationLog, int64, error) {
	var logs []model.ReputationLog
	var total int64

	q := s.db.Model(&model.ReputationLog{}).Where("user_id = ?", uint(userID))
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
