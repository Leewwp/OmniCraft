package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

// UsageGuideCacheRepository persists #728 auto-generation cache rows.
type UsageGuideCacheRepository struct{ db *gorm.DB }

func NewUsageGuideCacheRepository(db *gorm.DB) *UsageGuideCacheRepository {
	return &UsageGuideCacheRepository{db: db}
}

func (r *UsageGuideCacheRepository) Find(ctx context.Context, contentID int64, locale string) (*model.ContentUsageGuideCache, error) {
	var row model.ContentUsageGuideCache
	if err := r.db.WithContext(ctx).
		Where("content_id = ? AND locale = ?", contentID, locale).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// UpsertGuarded writes a completed generation result but never lets a stale
// task clobber a newer row. The guard lives in the UPDATE's WHERE clause
// (not a Go-side check-then-write) so two racing writers cannot both pass:
// the existing row only yields when it came from an older-or-equal content
// snapshot AND an older-or-equal prompt version; a losing stale writer
// matches zero rows and reports written=false. No row yet → insert.
func (r *UsageGuideCacheRepository) UpsertGuarded(ctx context.Context, row *model.ContentUsageGuideCache) (bool, error) {
	var written int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 条件 UPDATE：守卫在 SQL 层（TOCTOU 不可穿）。
		res := tx.Model(&model.ContentUsageGuideCache{}).
			Where(
				"content_id = ? AND locale = ? AND content_updated_at <= ? AND prompt_version <= ?",
				row.ContentItemID, row.Locale, row.ContentUpdatedAt, row.PromptVersion,
			).
			Updates(map[string]interface{}{
				"prompt_version":     row.PromptVersion,
				"input_fingerprint":  row.InputFingerprint,
				"content_updated_at": row.ContentUpdatedAt,
				"guide_markdown":     row.GuideMarkdown,
				"source":             row.Source,
				"updated_at":         time.Now().UTC(),
			})
		if res.Error != nil {
			return res.Error
		}
		written = res.RowsAffected
		if res.RowsAffected > 0 {
			return nil
		}
		// 未命中：要么无行（插入），要么守卫拒绝（静默放弃）。
		var exists int64
		if err := tx.Model(&model.ContentUsageGuideCache{}).
			Where("content_id = ? AND locale = ?", row.ContentItemID, row.Locale).
			Count(&exists).Error; err != nil {
			return err
		}
		if exists > 0 {
			return nil // 守卫拒绝：新版本的行已在，不覆盖。
		}
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		written = 1
		return nil
	})
	if err != nil {
		return false, err
	}
	return written > 0, nil
}
