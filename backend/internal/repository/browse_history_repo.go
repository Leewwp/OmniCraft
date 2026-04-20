package repository

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BrowseHistoryRepository struct {
	db *gorm.DB
}

func NewBrowseHistoryRepository(db *gorm.DB) *BrowseHistoryRepository {
	return &BrowseHistoryRepository{db: db}
}

func (r *BrowseHistoryRepository) Upsert(userID, contentItemID int64) error {
	bh := model.BrowseHistory{
		UserID:        userID,
		ContentItemID: contentItemID,
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "content_item_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"viewed_at": gorm.Expr("NOW()")}),
	}).Create(&bh).Error
}

func (r *BrowseHistoryRepository) ListByUser(userID int64, page, pageSize int) ([]model.BrowseHistory, int64, error) {
	var total int64
	r.db.Model(&model.BrowseHistory{}).Where("user_id = ?", userID).Count(&total)
	var items []model.BrowseHistory
	err := r.db.Where("user_id = ?", userID).
		Order("viewed_at DESC").
		Offset((page-1)*pageSize).Limit(pageSize).
		Find(&items).Error
	return items, total, err
}

func (r *BrowseHistoryRepository) DeleteByUser(userID int64) error {
	return r.db.Where("user_id = ?", userID).Delete(&model.BrowseHistory{}).Error
}
