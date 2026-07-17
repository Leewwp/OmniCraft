package repository

import (
	"time"

	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BrowseHistoryRepository struct {
	db *gorm.DB
}

// browseHistoryCleanupAdvisoryLockID is the stable application lock used by
// every API replica before deleting expired browse-history rows. The lock is
// transaction-scoped, so PostgreSQL releases it on both commit and rollback.
const browseHistoryCleanupAdvisoryLockID int64 = 0x4f4d4e4948495354 // "OMNIHIST"

type BrowseHistoryListOptions struct {
	UserID        int64
	ContentType   string
	StartDate     *time.Time
	EndDate       *time.Time
	RetentionDays int
	Now           time.Time
	Page          int
	PageSize      int
}

type BrowseHistoryItemDTO struct {
	ID          int64              `json:"id"`
	Content     *model.ContentItem `json:"content"`
	ContentItem *model.ContentItem `json:"content_item"`
	ViewedAt    time.Time          `json:"viewed_at"`
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
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&items).Error
	return items, total, err
}

func (r *BrowseHistoryRepository) ListByUserFiltered(opts BrowseHistoryListOptions) ([]BrowseHistoryItemDTO, int64, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 {
		opts.PageSize = 20
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}

	query := r.filteredBrowseHistoryQuery(opts)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []model.BrowseHistory
	err := r.filteredBrowseHistoryQuery(opts).
		Preload("ContentItem").
		Order("browse_history.viewed_at DESC").
		Offset((opts.Page - 1) * opts.PageSize).
		Limit(opts.PageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	items := make([]BrowseHistoryItemDTO, 0, len(rows))
	for _, row := range rows {
		item := BrowseHistoryItemDTO{
			ID:       row.ID,
			ViewedAt: row.ViewedAt,
		}
		if row.ContentItem.ID != 0 && row.ContentItem.Status == "published" && row.ContentItem.DeletedAt == nil {
			content := row.ContentItem
			item.Content = &content
			item.ContentItem = &content
		}
		items = append(items, item)
	}
	return items, total, nil
}

func (r *BrowseHistoryRepository) filteredBrowseHistoryQuery(opts BrowseHistoryListOptions) *gorm.DB {
	cutoff := opts.Now.AddDate(0, 0, -opts.RetentionDays)
	query := r.db.Model(&model.BrowseHistory{}).
		Where("browse_history.user_id = ?", opts.UserID).
		Where("browse_history.viewed_at >= ?", cutoff)

	if opts.ContentType != "" {
		query = query.Joins("JOIN content_items ON content_items.id = browse_history.content_item_id").
			Where("content_items.content_type = ?", opts.ContentType)
	}
	if opts.StartDate != nil {
		query = query.Where("browse_history.viewed_at >= ?", *opts.StartDate)
	}
	if opts.EndDate != nil {
		query = query.Where("browse_history.viewed_at < ?", *opts.EndDate)
	}
	return query
}

func (r *BrowseHistoryRepository) DeleteByUser(userID int64) error {
	return r.db.Where("user_id = ?", userID).Delete(&model.BrowseHistory{}).Error
}

func (r *BrowseHistoryRepository) DeleteByUserAndIDs(userID int64, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Where("user_id = ? AND id IN ?", userID, ids).Delete(&model.BrowseHistory{}).Error
}

func (r *BrowseHistoryRepository) DeleteExpired(retentionDays int, now time.Time) (int64, error) {
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.AddDate(0, 0, -retentionDays)
	result := r.db.Where("viewed_at < ?", cutoff).Delete(&model.BrowseHistory{})
	return result.RowsAffected, result.Error
}

func (r *BrowseHistoryRepository) DeleteExpiredIfLeader(retentionDays int, now time.Time) (deleted int64, acquired bool, err error) {
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.AddDate(0, 0, -retentionDays)
	acquired = true

	err = r.db.Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if lockErr := tx.Raw("SELECT pg_try_advisory_xact_lock(?)", browseHistoryCleanupAdvisoryLockID).Scan(&acquired).Error; lockErr != nil {
				return lockErr
			}
			if !acquired {
				return nil
			}
		}

		result := tx.Where("viewed_at < ?", cutoff).Delete(&model.BrowseHistory{})
		deleted = result.RowsAffected
		return result.Error
	})
	return deleted, acquired, err
}
