package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"omnicraft/backend/internal/model"
)

var (
	ErrSeriesNotFound               = errors.New("SERIES_NOT_FOUND")
	ErrNotSeriesOwner               = errors.New("NOT_SERIES_OWNER")
	ErrContentNotOwnedOrContributed = errors.New("CONTENT_NOT_OWNED_OR_CONTRIBUTED")
	ErrSeriesZoneMismatch           = errors.New("ZONE_MISMATCH")
	ErrSeriesContentUnavailable     = errors.New("SERIES_CONTENT_UNAVAILABLE")
	ErrDuplicateSeriesItem          = errors.New("DUPLICATE_SERIES_ITEM")
	ErrCoverNotInSeries             = errors.New("COVER_NOT_IN_SERIES")
	ErrSeriesItemSetMismatch        = errors.New("SERIES_ITEM_SET_MISMATCH")
	ErrSeriesItemNotFound           = errors.New("SERIES_ITEM_NOT_FOUND")
	ErrInvalidSeries                = errors.New("INVALID_SERIES")
)

type SeriesRepository struct {
	db *gorm.DB
}

type SeriesPatch struct {
	Title          *string
	Description    *string
	CoverContentID *int64
	ClearCover     bool
}

type SeriesDetail struct {
	Series model.ContentSeries
	Items  []model.ContentSeriesItem
}

type SeriesContentSummary struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type SeriesMembership struct {
	SeriesID     int64                 `json:"series_id"`
	SeriesTitle  string                `json:"series_title"`
	SeriesZone   string                `json:"series_zone"`
	CurrentIndex int                   `json:"current_index"`
	Total        int                   `json:"total"`
	Previous     *SeriesContentSummary `json:"previous,omitempty"`
	Next         *SeriesContentSummary `json:"next,omitempty"`
}

func NewSeriesRepository(db *gorm.DB) *SeriesRepository {
	return &SeriesRepository{db: db}
}

func (r *SeriesRepository) DB() *gorm.DB {
	return r.db
}

func (r *SeriesRepository) CreateSeries(ctx context.Context, series *model.ContentSeries) (*model.ContentSeries, error) {
	if series == nil {
		return nil, ErrInvalidSeries
	}
	if err := r.db.WithContext(ctx).Create(series).Error; err != nil {
		return nil, err
	}
	return series, nil
}

func (r *SeriesRepository) ListSeriesByOwner(ctx context.Context, ownerID int64, zone string) ([]model.ContentSeries, error) {
	query := r.db.WithContext(ctx).Where("owner_id = ?", ownerID)
	if zone != "" {
		query = query.Where("zone = ?", zone)
	}
	var series []model.ContentSeries
	if err := query.Order("updated_at DESC").Order("id DESC").Find(&series).Error; err != nil {
		return nil, err
	}
	return series, nil
}

func (r *SeriesRepository) ListCandidateContents(ctx context.Context, ownerID int64, zone, search string, limit int) ([]model.ContentItem, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	query := r.db.WithContext(ctx).Model(&model.ContentItem{}).
		Preload("Author").
		Where("content_items.deleted_at IS NULL").
		Where("content_items.status IN ?", []string{"pending", "published"}).
		Where("content_items.zone = ?", zone).
		Where(`(
			content_items.author_id = ?
			OR EXISTS (
				SELECT 1 FROM content_contributors
				WHERE content_contributors.content_item_id = content_items.id
				  AND content_contributors.user_id = ?
			)
		)`, ownerID, ownerID)
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		query = query.Where("LOWER(content_items.title) LIKE ?", "%"+strings.ToLower(trimmed)+"%")
	}
	var items []model.ContentItem
	if err := query.Order("content_items.updated_at DESC").Order("content_items.id DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SeriesRepository) GetSeriesDetail(ctx context.Context, seriesID int64) (*SeriesDetail, error) {
	series, err := r.GetSeries(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	items, err := r.ListSeriesItems(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	return &SeriesDetail{Series: *series, Items: items}, nil
}

func (r *SeriesRepository) GetSeries(ctx context.Context, seriesID int64) (*model.ContentSeries, error) {
	var series model.ContentSeries
	if err := r.db.WithContext(ctx).Preload("Owner").First(&series, seriesID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSeriesNotFound
		}
		return nil, err
	}
	return &series, nil
}

func (r *SeriesRepository) ListSeriesItems(ctx context.Context, seriesID int64) ([]model.ContentSeriesItem, error) {
	var items []model.ContentSeriesItem
	if err := r.db.WithContext(ctx).
		Preload("ContentItem", func(db *gorm.DB) *gorm.DB { return db.Unscoped() }).
		Preload("ContentItem.Author").
		Preload("ContentItem.IP").
		Where("series_id = ?", seriesID).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SeriesRepository) ListVisibleSeriesItems(ctx context.Context, seriesID, viewerID int64) ([]model.ContentSeriesItem, error) {
	visibilitySQL, visibilityArgs := ContentVisibilitySQL(viewerID)
	var items []model.ContentSeriesItem
	if err := r.db.WithContext(ctx).Model(&model.ContentSeriesItem{}).
		Joins("JOIN content_items ON content_items.id = content_series_items.content_item_id").
		Where("content_series_items.series_id = ?", seriesID).
		Where(visibilitySQL, visibilityArgs...).
		Preload("ContentItem.Author").
		Preload("ContentItem.IP").
		Order("content_series_items.sort_order ASC").
		Order("content_series_items.id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SeriesRepository) UpdateSeries(ctx context.Context, seriesID, ownerID int64, patch SeriesPatch) (*model.ContentSeries, error) {
	var updated model.ContentSeries
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		series, err := lockSeries(tx, seriesID)
		if err != nil {
			return err
		}
		if series.OwnerID != ownerID {
			return ErrNotSeriesOwner
		}
		if patch.CoverContentID != nil {
			var count int64
			if err := tx.Model(&model.ContentSeriesItem{}).
				Where("series_id = ? AND content_item_id = ?", seriesID, *patch.CoverContentID).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrCoverNotInSeries
			}
		}
		updates := map[string]interface{}{}
		if patch.ClearCover {
			updates["cover_content_id"] = nil
		}
		if patch.Title != nil {
			updates["title"] = *patch.Title
		}
		if patch.Description != nil {
			updates["description"] = *patch.Description
		}
		if patch.CoverContentID != nil {
			updates["cover_content_id"] = *patch.CoverContentID
		}
		if len(updates) > 0 {
			if err := tx.Model(&model.ContentSeries{}).Where("id = ?", seriesID).Updates(updates).Error; err != nil {
				return err
			}
		}
		return tx.First(&updated, seriesID).Error
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *SeriesRepository) DeleteSeries(ctx context.Context, seriesID, ownerID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		series, err := lockSeries(tx, seriesID)
		if err != nil {
			return err
		}
		if series.OwnerID != ownerID {
			return ErrNotSeriesOwner
		}
		return tx.Delete(&model.ContentSeries{}, seriesID).Error
	})
}

func (r *SeriesRepository) AddItem(ctx context.Context, seriesID, ownerID, contentID int64) (*model.ContentSeriesItem, error) {
	var item model.ContentSeriesItem
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		series, err := lockSeries(tx, seriesID)
		if err != nil {
			return err
		}
		if series.OwnerID != ownerID {
			return ErrNotSeriesOwner
		}

		var content model.ContentItem
		if err := tx.Where("id = ? AND deleted_at IS NULL", contentID).First(&content).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSeriesContentUnavailable
			}
			return err
		}
		if content.Status != "pending" && content.Status != "published" {
			return ErrSeriesContentUnavailable
		}
		if content.Zone != series.Zone {
			return ErrSeriesZoneMismatch
		}
		if content.AuthorID != series.OwnerID {
			var contributorCount int64
			if err := tx.Model(&model.ContentContributor{}).
				Where("content_item_id = ? AND user_id = ?", content.ID, series.OwnerID).
				Count(&contributorCount).Error; err != nil {
				return err
			}
			if contributorCount == 0 {
				return ErrContentNotOwnedOrContributed
			}
		}

		var duplicateCount int64
		if err := tx.Model(&model.ContentSeriesItem{}).
			Where("series_id = ? AND content_item_id = ?", seriesID, contentID).
			Count(&duplicateCount).Error; err != nil {
			return err
		}
		if duplicateCount != 0 {
			return ErrDuplicateSeriesItem
		}

		var maxOrder int
		if err := tx.Model(&model.ContentSeriesItem{}).
			Where("series_id = ?", seriesID).
			Select("COALESCE(MAX(sort_order), -1)").
			Scan(&maxOrder).Error; err != nil {
			return err
		}
		item = model.ContentSeriesItem{SeriesID: seriesID, ContentItemID: contentID, SortOrder: maxOrder + 1}
		if err := tx.Create(&item).Error; err != nil {
			if isSeriesItemDuplicateError(err) {
				return ErrDuplicateSeriesItem
			}
			return err
		}
		return touchSeries(tx, seriesID)
	})
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *SeriesRepository) RemoveItem(ctx context.Context, seriesID, ownerID, itemID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		series, err := lockSeries(tx, seriesID)
		if err != nil {
			return err
		}
		if series.OwnerID != ownerID {
			return ErrNotSeriesOwner
		}

		var item model.ContentSeriesItem
		if err := tx.Where("id = ? AND series_id = ?", itemID, seriesID).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSeriesItemNotFound
			}
			return err
		}
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		if series.CoverContentID != nil && *series.CoverContentID == item.ContentItemID {
			if err := tx.Model(&model.ContentSeries{}).Where("id = ?", seriesID).Update("cover_content_id", nil).Error; err != nil {
				return err
			}
		}
		return touchSeries(tx, seriesID)
	})
}

func (r *SeriesRepository) ReorderItems(ctx context.Context, seriesID, ownerID int64, itemIDs []int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		series, err := lockSeries(tx, seriesID)
		if err != nil {
			return err
		}
		if series.OwnerID != ownerID {
			return ErrNotSeriesOwner
		}

		var existing []model.ContentSeriesItem
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("series_id = ?", seriesID).
			Order("id ASC").
			Find(&existing).Error; err != nil {
			return err
		}
		if len(existing) != len(itemIDs) {
			return ErrSeriesItemSetMismatch
		}
		set := make(map[int64]struct{}, len(existing))
		for _, item := range existing {
			set[item.ID] = struct{}{}
		}
		for _, itemID := range itemIDs {
			if _, ok := set[itemID]; !ok {
				return ErrSeriesItemSetMismatch
			}
			delete(set, itemID)
		}
		if len(set) != 0 {
			return ErrSeriesItemSetMismatch
		}

		for sortOrder, itemID := range itemIDs {
			result := tx.Model(&model.ContentSeriesItem{}).
				Where("id = ? AND series_id = ?", itemID, seriesID).
				Update("sort_order", sortOrder)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrSeriesItemSetMismatch
			}
		}
		return touchSeries(tx, seriesID)
	})
}

func touchSeries(tx *gorm.DB, seriesID int64) error {
	return tx.Model(&model.ContentSeries{}).
		Where("id = ?", seriesID).
		Update("updated_at", time.Now().UTC()).Error
}

func (r *SeriesRepository) ListMembershipsForContent(ctx context.Context, contentID int64) ([]SeriesMembership, error) {
	var visibleCurrentCount int64
	if err := ApplyContentVisibilityScope(r.db.WithContext(ctx).Model(&model.ContentItem{}), 0).
		Where("content_items.id = ?", contentID).
		Count(&visibleCurrentCount).Error; err != nil {
		return nil, err
	}
	if visibleCurrentCount == 0 {
		return []SeriesMembership{}, nil
	}

	type seriesRow struct {
		ID    int64
		Title string
		Zone  string
	}
	var seriesRows []seriesRow
	if err := r.db.WithContext(ctx).Table("content_series_items").
		Select("content_series.id, content_series.title, content_series.zone").
		Joins("JOIN content_series ON content_series.id = content_series_items.series_id").
		Where("content_series_items.content_item_id = ?", contentID).
		Order("content_series.updated_at DESC").
		Order("content_series.id DESC").
		Scan(&seriesRows).Error; err != nil {
		return nil, err
	}

	type itemRow struct {
		SeriesID  int64
		ID        int64
		Title     string
		SortOrder int
	}
	seriesIDs := make([]int64, 0, len(seriesRows))
	for _, series := range seriesRows {
		seriesIDs = append(seriesIDs, series.ID)
	}
	var visibleItemRows []itemRow
	visibilitySQL, visibilityArgs := ContentVisibilitySQL(0)
	if len(seriesIDs) > 0 {
		if err := r.db.WithContext(ctx).Table("content_series_items").
			Select("content_series_items.series_id, content_items.id, content_items.title, content_series_items.sort_order").
			Joins("JOIN content_items ON content_items.id = content_series_items.content_item_id").
			Where("content_series_items.series_id IN ?", seriesIDs).
			Where(visibilitySQL, visibilityArgs...).
			Order("content_series_items.series_id ASC").
			Order("content_series_items.sort_order ASC").
			Order("content_series_items.id ASC").
			Scan(&visibleItemRows).Error; err != nil {
			return nil, err
		}
	}
	itemsBySeries := make(map[int64][]itemRow, len(seriesRows))
	for _, item := range visibleItemRows {
		itemsBySeries[item.SeriesID] = append(itemsBySeries[item.SeriesID], item)
	}

	memberships := make([]SeriesMembership, 0, len(seriesRows))
	for _, series := range seriesRows {
		visibleItems := itemsBySeries[series.ID]

		currentIndex := -1
		for index, item := range visibleItems {
			if item.ID == contentID {
				currentIndex = index
				break
			}
		}
		if currentIndex < 0 {
			continue
		}

		membership := SeriesMembership{
			SeriesID:     series.ID,
			SeriesTitle:  series.Title,
			SeriesZone:   series.Zone,
			CurrentIndex: currentIndex + 1,
			Total:        len(visibleItems),
		}
		if currentIndex > 0 {
			previous := visibleItems[currentIndex-1]
			membership.Previous = &SeriesContentSummary{ID: previous.ID, Title: previous.Title}
		}
		if currentIndex+1 < len(visibleItems) {
			next := visibleItems[currentIndex+1]
			membership.Next = &SeriesContentSummary{ID: next.ID, Title: next.Title}
		}
		memberships = append(memberships, membership)
	}
	return memberships, nil
}

func lockSeries(tx *gorm.DB, seriesID int64) (*model.ContentSeries, error) {
	var series model.ContentSeries
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&series, seriesID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSeriesNotFound
		}
		return nil, err
	}
	return &series, nil
}

func isSeriesItemDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate") ||
		strings.Contains(message, "unique constraint failed") ||
		strings.Contains(message, "violates unique constraint")
}
