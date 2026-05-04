package repository

import (
	"time"

	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type ContentRepository struct {
	db *gorm.DB
}

func NewContentRepository(db *gorm.DB) *ContentRepository {
	return &ContentRepository{db: db}
}

type ListContentsFilter struct {
	Zone         string
	IPID         *int64
	Category     string
	ContentType  string
	ContentTypes []string
	AuthorID     *int64
	Status       string
	Tags         []string
	Sort         string
	TimeRange    string
	Page         int
	PageSize     int
}

func (r *ContentRepository) CreateContent(content *model.ContentItem) error {
	return r.db.Create(content).Error
}

func (r *ContentRepository) FindByID(id int64) (*model.ContentItem, error) {
	var content model.ContentItem
	err := r.db.Preload("Author").First(&content, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &content, nil
}

func (r *ContentRepository) ListContents(f ListContentsFilter) ([]model.ContentItem, int64, error) {
	var items []model.ContentItem
	var total int64

	q := r.db.Model(&model.ContentItem{}).Preload("Author")

	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	} else {
		q = q.Where("status = ?", "published")
	}

	if f.Zone != "" {
		q = q.Where("zone = ?", f.Zone)
	}
	if f.IPID != nil {
		q = q.Where("ip_id = ?", *f.IPID)
	}
	if f.Category != "" {
		q = q.Where("category = ?", f.Category)
	}
	if len(f.ContentTypes) > 0 {
		q = q.Where("content_type IN ?", f.ContentTypes)
	} else if f.ContentType != "" {
		q = q.Where("content_type = ?", f.ContentType)
	}
	if f.AuthorID != nil {
		q = q.Where("author_id = ?", *f.AuthorID)
	}

	if f.TimeRange != "" && f.TimeRange != "all" {
		var since time.Time
		now := time.Now()
		switch f.TimeRange {
		case "week":
			since = now.AddDate(0, 0, -7)
		case "month":
			since = now.AddDate(0, -1, 0)
		case "year":
			since = now.AddDate(-1, 0, 0)
		}
		q = q.Where("created_at >= ?", since)
	}

	if len(f.Tags) > 0 {
		for _, tag := range f.Tags {
			q = q.Where("id IN (SELECT content_item_id FROM content_tags WHERE tag = ?)", tag)
		}
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	switch f.Sort {
	case "hot":
		q = q.Order("(view_count + like_count * 3) DESC")
	case "most_views":
		q = q.Order("view_count DESC")
	case "best_rated":
		q = q.Where("(like_count + dislike_count) >= 5").
			Order("(like_count::float / NULLIF(like_count + dislike_count, 0)) DESC NULLS LAST")
	default:
		q = q.Order("created_at DESC")
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	if err := q.Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *ContentRepository) UpdateContent(id int64, updates map[string]interface{}) error {
	return r.db.Model(&model.ContentItem{}).Where("id = ?", id).Updates(updates).Error
}

func (r *ContentRepository) DeleteContent(id int64) error {
	return r.db.Delete(&model.ContentItem{}, id).Error
}

func (r *ContentRepository) IncrViewCount(id int64) error {
	return r.db.Model(&model.ContentItem{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

func (r *ContentRepository) CreateAttachments(attachments []model.ContentAttachment) error {
	if len(attachments) == 0 {
		return nil
	}
	return r.db.Create(&attachments).Error
}

func (r *ContentRepository) CreateTags(tags []model.ContentTag) error {
	if len(tags) == 0 {
		return nil
	}
	return r.db.Create(&tags).Error
}

func (r *ContentRepository) GetAttachments(contentID int64) ([]model.ContentAttachment, error) {
	var attachments []model.ContentAttachment
	err := r.db.Where("content_item_id = ?", contentID).Find(&attachments).Error
	return attachments, err
}

func (r *ContentRepository) GetTags(contentID int64) ([]model.ContentTag, error) {
	var tags []model.ContentTag
	err := r.db.Where("content_item_id = ?", contentID).Find(&tags).Error
	return tags, err
}

func (r *ContentRepository) AddTag(contentID int64, tag string) error {
	ct := model.ContentTag{ContentItemID: contentID, Tag: tag}
	return r.db.FirstOrCreate(&ct, model.ContentTag{ContentItemID: contentID, Tag: tag}).Error
}

func (r *ContentRepository) RemoveTag(contentID int64, tag string) error {
	return r.db.Where("content_item_id = ? AND tag = ?", contentID, tag).Delete(&model.ContentTag{}).Error
}
