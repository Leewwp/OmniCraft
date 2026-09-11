package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"omnicraft/backend/internal/model"
)

type UsageGuideRepository struct {
	db *gorm.DB
}

func NewUsageGuideRepository(db *gorm.DB) *UsageGuideRepository {
	return &UsageGuideRepository{db: db}
}

func (r *UsageGuideRepository) DB() *gorm.DB { return r.db }

// FindByID returns the content-scoped guide row for one locale, nil when absent.
func (r *UsageGuideRepository) Find(ctx context.Context, contentID int64, locale string) (*model.ContentUsageGuide, error) {
	var row model.ContentUsageGuide
	err := r.db.WithContext(ctx).
		Where("content_id = ? AND locale = ?", contentID, locale).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// Upsert saves the specifics row for (content_id, locale).
func (r *UsageGuideRepository) Upsert(ctx context.Context, row *model.ContentUsageGuide) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "content_id"}, {Name: "locale"}},
		DoUpdates: clause.AssignmentColumns([]string{"requirements", "steps", "notes", "source", "updated_at"}),
	}).Create(row).Error
}
