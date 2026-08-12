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
	ErrCollectionNotFound         = errors.New("COLLECTION_NOT_FOUND")
	ErrZoneMismatch               = errors.New("ZONE_MISMATCH")
	ErrDuplicateCollectionItem    = errors.New("DUPLICATE_COLLECTION_ITEM")
	ErrDefaultCollectionProtected = errors.New("DEFAULT_COLLECTION_PROTECTED")
	ErrZoneImmutable              = errors.New("ZONE_IMMUTABLE")
	ErrInvalidContent             = errors.New("INVALID_CONTENT")
)

type CollectionRepository struct {
	db *gorm.DB
}

type CollectionSummary struct {
	ID           int64      `json:"id"`
	UserID       int64      `json:"user_id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Zone         string     `json:"zone"`
	IsDefault    bool       `json:"is_default"`
	IsPublic     bool       `json:"is_public"`
	SortOrder    int        `json:"sort_order"`
	ItemCount    int64      `json:"item_count"`
	ContainsItem bool       `json:"contains_item"`
	ItemID       *int64     `json:"item_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

type CollectionPatch struct {
	Title       *string
	Description *string
	IsPublic    *bool
	SortOrder   *int
	Zone        *string
}

func NewCollectionRepository(db *gorm.DB) *CollectionRepository {
	return &CollectionRepository{db: db}
}

func (r *CollectionRepository) DB() *gorm.DB {
	return r.db
}

func (r *CollectionRepository) CreateCollection(ctx context.Context, collection *model.Collection) (*model.Collection, error) {
	if collection == nil || !isCollectionZone(collection.Zone) {
		return nil, ErrZoneMismatch
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if !collection.IsDefault && collection.SortOrder == 0 {
			var maxSort int
			if err := tx.Model(&model.Collection{}).
				Where("user_id = ? AND zone = ? AND deleted_at IS NULL", collection.UserID, collection.Zone).
				Select("COALESCE(MAX(sort_order), 0)").
				Scan(&maxSort).Error; err != nil {
				return err
			}
			collection.SortOrder = maxSort + 1
		}
		return tx.Create(collection).Error
	})
	if err != nil {
		return nil, err
	}
	return collection, nil
}

func (r *CollectionRepository) ListCollections(ctx context.Context, ownerID int64, zone string, containsContentItemID *int64) ([]CollectionSummary, error) {
	return r.ListCollectionsForViewer(ctx, ownerID, ownerID, zone, containsContentItemID)
}

func (r *CollectionRepository) ListCollectionsForViewer(ctx context.Context, ownerID, viewerID int64, zone string, containsContentItemID *int64) ([]CollectionSummary, error) {
	if zone != "" && !isCollectionZone(zone) {
		return nil, ErrZoneMismatch
	}

	if containsContentItemID != nil {
		content, err := r.findVisibleContent(ctx, *containsContentItemID, viewerID)
		if err != nil {
			return nil, err
		}
		if zone != "" && zone != content.Zone {
			return nil, ErrZoneMismatch
		}
		zone = content.Zone
	}

	visibilitySQL, visibilityArgs := ContentVisibilitySQL(viewerID)
	selectSQL := `
		collections.id,
		collections.user_id,
		collections.title,
		collections.description,
		collections.zone,
		collections.is_default,
		collections.is_public,
		collections.sort_order,
		collections.created_at,
		collections.updated_at,
		collections.deleted_at,
		(
			SELECT COUNT(*)
			FROM collection_items item_count_rows
			JOIN content_items
			  ON content_items.id = item_count_rows.content_item_id
			WHERE item_count_rows.collection_id = collections.id
			  AND ` + visibilitySQL + `
		) AS item_count`
	args := append([]interface{}{}, visibilityArgs...)
	if containsContentItemID != nil {
		selectSQL += `,
		(
			SELECT contains_rows.id
			FROM collection_items contains_rows
			WHERE contains_rows.collection_id = collections.id
			  AND contains_rows.content_item_id = ?
			LIMIT 1
		) AS item_id`
		args = append(args, *containsContentItemID)
	}

	query := r.db.WithContext(ctx).Table("collections").
		Select(selectSQL, args...).
		Where("collections.user_id = ? AND collections.deleted_at IS NULL", ownerID)
	if viewerID != ownerID {
		query = query.Where("collections.is_public = ?", true)
	}
	if zone != "" {
		query = query.Where("collections.zone = ?", zone)
	}

	var rows []CollectionSummary
	if err := query.
		Order("collections.is_default DESC").
		Order("collections.sort_order ASC").
		Order("collections.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	for i := range rows {
		if containsContentItemID == nil {
			rows[i].ContainsItem = false
			rows[i].ItemID = nil
			continue
		}
		rows[i].ContainsItem = rows[i].ItemID != nil
	}
	return rows, nil
}

func (r *CollectionRepository) GetCollectionForViewer(ctx context.Context, collectionID int64, viewerID *int64) (*model.Collection, error) {
	query := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", collectionID)
	if viewerID == nil {
		query = query.Where("is_public = ?", true)
	} else {
		query = query.Where("(is_public = ? OR user_id = ?)", true, *viewerID)
	}

	var collection model.Collection
	if err := query.First(&collection).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCollectionNotFound
		}
		return nil, err
	}
	return &collection, nil
}

func (r *CollectionRepository) UpdateCollection(ctx context.Context, collectionID, ownerID int64, patch CollectionPatch) (*model.Collection, error) {
	if patch.Zone != nil {
		return nil, ErrZoneImmutable
	}

	collection, err := r.findOwnedCollection(ctx, collectionID, ownerID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if patch.Title != nil {
		updates["title"] = *patch.Title
	}
	if patch.Description != nil {
		updates["description"] = *patch.Description
	}
	if patch.IsPublic != nil {
		updates["is_public"] = *patch.IsPublic
	}
	if patch.SortOrder != nil {
		updates["sort_order"] = *patch.SortOrder
	}

	if len(updates) > 0 {
		if err := r.db.WithContext(ctx).Model(&model.Collection{}).
			Where("id = ? AND user_id = ? AND deleted_at IS NULL", collectionID, ownerID).
			Updates(updates).Error; err != nil {
			return nil, err
		}
		if err := r.db.WithContext(ctx).First(collection, collectionID).Error; err != nil {
			return nil, err
		}
	}
	return collection, nil
}

func (r *CollectionRepository) DeleteCollection(ctx context.Context, collectionID, ownerID int64) error {
	collection, err := r.findOwnedCollection(ctx, collectionID, ownerID)
	if err != nil {
		return err
	}
	if collection.IsDefault {
		return ErrDefaultCollectionProtected
	}
	return r.db.WithContext(ctx).Model(&model.Collection{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", collectionID, ownerID).
		Update("deleted_at", time.Now()).Error
}

func (r *CollectionRepository) AddItem(ctx context.Context, collectionID, ownerID, contentID int64, note string) (*model.CollectionItem, error) {
	collection, err := r.findOwnedCollection(ctx, collectionID, ownerID)
	if err != nil {
		return nil, err
	}
	content, err := r.findVisibleContent(ctx, contentID, ownerID)
	if err != nil {
		return nil, err
	}
	if collection.Zone != content.Zone {
		return nil, ErrZoneMismatch
	}

	item := &model.CollectionItem{
		CollectionID:  collectionID,
		ContentItemID: contentID,
		Note:          note,
	}
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		if isCollectionItemDuplicateError(err) {
			return nil, ErrDuplicateCollectionItem
		}
		return nil, err
	}
	return item, nil
}

func (r *CollectionRepository) RemoveItem(ctx context.Context, collectionID, ownerID, itemID int64) error {
	if _, err := r.findOwnedCollection(ctx, collectionID, ownerID); err != nil {
		return err
	}
	result := r.db.WithContext(ctx).
		Where("id = ? AND collection_id = ?", itemID, collectionID).
		Delete(&model.CollectionItem{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrCollectionNotFound
	}
	return nil
}

func (r *CollectionRepository) RemoveDefaultItemByContentID(ctx context.Context, ownerID, contentID int64) error {
	defaultCollections := r.db.WithContext(ctx).Model(&model.Collection{}).
		Select("id").
		Where("user_id = ? AND is_default = ? AND deleted_at IS NULL", ownerID, true)
	return r.db.WithContext(ctx).
		Where("content_item_id = ? AND collection_id IN (?)", contentID, defaultCollections).
		Delete(&model.CollectionItem{}).Error
}

func (r *CollectionRepository) UpdateItemNote(ctx context.Context, collectionID, ownerID, itemID int64, note string) (*model.CollectionItem, error) {
	if _, err := r.findOwnedCollection(ctx, collectionID, ownerID); err != nil {
		return nil, err
	}
	result := r.db.WithContext(ctx).Model(&model.CollectionItem{}).
		Where("id = ? AND collection_id = ?", itemID, collectionID).
		Update("note", note)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrCollectionNotFound
	}

	var item model.CollectionItem
	if err := r.db.WithContext(ctx).Preload("ContentItem").
		Where("id = ? AND collection_id = ?", itemID, collectionID).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCollectionNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *CollectionRepository) ListItems(ctx context.Context, collectionID int64, page, pageSize int, contentType string) ([]model.CollectionItem, int64, error) {
	return r.ListItemsForViewer(ctx, collectionID, 0, page, pageSize, contentType)
}

func (r *CollectionRepository) ListItemsForViewer(ctx context.Context, collectionID, viewerID int64, page, pageSize int, contentType string) ([]model.CollectionItem, int64, error) {
	if err := r.ensureCollectionVisible(ctx, collectionID, viewerID); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := r.availableCollectionItemsQuery(ctx, collectionID, viewerID, contentType)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.CollectionItem
	if err := r.availableCollectionItemsQuery(ctx, collectionID, viewerID, contentType).
		Preload("ContentItem").
		Order("collection_items.added_at DESC").
		Order("collection_items.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *CollectionRepository) EnsureDefaultCollection(ctx context.Context, userID int64, zone string) (*model.Collection, error) {
	if !isCollectionZone(zone) {
		return nil, ErrZoneMismatch
	}

	collection := model.Collection{
		UserID:      userID,
		Title:       defaultCollectionTitle(zone),
		Description: "",
		Zone:        zone,
		IsDefault:   true,
		IsPublic:    false,
		SortOrder:   0,
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&collection).Error; err != nil {
		return nil, err
	}

	var existing model.Collection
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND zone = ? AND is_default = ? AND deleted_at IS NULL", userID, zone, true).
		Order("id ASC").
		First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCollectionNotFound
		}
		return nil, err
	}
	return &existing, nil
}

// CountActiveMembershipsForContent counts how many of the user's active
// (non-deleted) collections contain the content item. #74: this is the single
// source of truth for the "favorited" state.
func (r *CollectionRepository) CountActiveMembershipsForContent(ctx context.Context, userID, contentID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.CollectionItem{}).
		Joins("JOIN collections ON collections.id = collection_items.collection_id").
		Where("collection_items.content_item_id = ? AND collections.user_id = ? AND collections.deleted_at IS NULL", contentID, userID).
		Count(&count).Error
	return count, err
}

func (r *CollectionRepository) findOwnedCollection(ctx context.Context, collectionID, ownerID int64) (*model.Collection, error) {
	var collection model.Collection
	if err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", collectionID, ownerID).
		First(&collection).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCollectionNotFound
		}
		return nil, err
	}
	return &collection, nil
}

func (r *CollectionRepository) findVisibleContent(ctx context.Context, contentID, viewerID int64) (*model.ContentItem, error) {
	var content model.ContentItem
	if err := ApplyContentVisibilityScope(r.db.WithContext(ctx).Model(&model.ContentItem{}), viewerID).
		Where("content_items.id = ?", contentID).
		First(&content).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidContent
		}
		return nil, err
	}
	return &content, nil
}

func (r *CollectionRepository) ensureCollectionExists(ctx context.Context, collectionID int64) error {
	var collection model.Collection
	if err := r.db.WithContext(ctx).
		Select("id").
		Where("id = ? AND deleted_at IS NULL", collectionID).
		First(&collection).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCollectionNotFound
		}
		return err
	}
	return nil
}

func (r *CollectionRepository) ensureCollectionVisible(ctx context.Context, collectionID, viewerID int64) error {
	query := r.db.WithContext(ctx).
		Select("id").
		Where("id = ? AND deleted_at IS NULL", collectionID)
	if viewerID == 0 {
		query = query.Where("is_public = ?", true)
	} else {
		query = query.Where("(is_public = ? OR user_id = ?)", true, viewerID)
	}

	var collection model.Collection
	if err := query.First(&collection).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCollectionNotFound
		}
		return err
	}
	return nil
}

func (r *CollectionRepository) availableCollectionItemsQuery(ctx context.Context, collectionID, viewerID int64, contentType string) *gorm.DB {
	visibilitySQL, visibilityArgs := ContentVisibilitySQL(viewerID)
	query := r.db.WithContext(ctx).Model(&model.CollectionItem{}).
		Joins("JOIN content_items ON content_items.id = collection_items.content_item_id").
		Where("collection_items.collection_id = ?", collectionID).
		Where(visibilitySQL, visibilityArgs...)
	if contentType != "" {
		query = query.Where("content_items.content_type = ?", contentType)
	}
	return query
}

func isCollectionZone(zone string) bool {
	return zone == "original" || zone == "fanwork"
}

func defaultCollectionTitle(zone string) string {
	if zone == "fanwork" {
		return "\u9ed8\u8ba4\u4e8c\u521b\u6536\u85cf"
	}
	return "\u9ed8\u8ba4\u539f\u521b\u6536\u85cf"
}

func isCollectionItemDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "violates unique constraint")
}
