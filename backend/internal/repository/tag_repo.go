package repository

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TagRepository struct {
	db *gorm.DB
}

func NewTagRepository(db *gorm.DB) *TagRepository {
	return &TagRepository{db: db}
}

func (r *TagRepository) GetTagsByCategory(category string, limit int) ([]model.Tag, error) {
	var tags []model.Tag
	q := r.db.Model(&model.Tag{})
	if category != "" {
		q = q.Where("category = ?", category)
	}
	err := q.Order("usage_count DESC").Limit(limit).Find(&tags).Error
	return tags, err
}

func (r *TagRepository) GetCooccurringTags(selectedTags []string, category string, limit int) ([]model.Tag, error) {
	var tags []model.Tag
	q := r.db.Table("tags t").
		Select("t.*").
		Joins("JOIN content_tags ct ON ct.tag = t.name").
		Joins("JOIN content_items ci ON ci.id = ct.content_item_id").
		// #446/SP-16 P0：共现计数只统计匿名可见内容——私密/软删/封禁作者
		// 内容的标签不得经由计数透出（与 ApplyContentVisibilityScope(0) 同口径）。
		Where("ci.status = ? AND ci.deleted_at IS NULL AND ci.is_public = ?", "published", true).
		Where("ci.author_id NOT IN (SELECT id FROM users WHERE is_banned = true OR deleted_at IS NOT NULL)").
		Where("ci.ip_id IS NULL OR ci.ip_id NOT IN (SELECT id FROM ips WHERE status = ?)", "banned").
		Where("ct.tag NOT IN ?", selectedTags)

	for _, tag := range selectedTags {
		q = q.Where("ci.id IN (SELECT content_item_id FROM content_tags WHERE tag = ?)", tag)
	}

	if category != "" {
		q = q.Where("t.category = ?", category)
	}

	err := q.Group("t.id").Order("t.usage_count DESC").Limit(limit).Find(&tags).Error
	return tags, err
}

func (r *TagRepository) SearchTagsByName(query string, limit int) ([]model.Tag, error) {
	var tags []model.Tag
	err := r.db.Where("name ILIKE ?", "%"+query+"%").Order("usage_count DESC").Limit(limit).Find(&tags).Error
	return tags, err
}

func (r *TagRepository) IncrementUsage(name string) error {
	return r.db.Model(&model.Tag{}).Where("name = ?", name).Update("usage_count", gorm.Expr("usage_count + 1")).Error
}

func (r *TagRepository) CreateTagSuggestion(s *model.TagSuggestion) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(s).Error
}

func (r *TagRepository) ListTagSuggestions(contentItemID int64, status string) ([]model.TagSuggestion, error) {
	var suggestions []model.TagSuggestion
	q := r.db.Where("content_item_id = ?", contentItemID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("created_at DESC").Find(&suggestions).Error
	return suggestions, err
}

func (r *TagRepository) UpdateTagSuggestionStatus(id int64, status string) error {
	return r.db.Model(&model.TagSuggestion{}).Where("id = ?", id).Update("status", status).Error
}

func (r *TagRepository) FindTagSuggestion(id int64) (*model.TagSuggestion, error) {
	var s model.TagSuggestion
	err := r.db.First(&s, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *TagRepository) CreateTagGroup(g *model.TagGroup) error {
	return r.db.Create(g).Error
}

func (r *TagRepository) ListTagGroups(userID int64) ([]model.TagGroup, error) {
	var groups []model.TagGroup
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&groups).Error
	return groups, err
}

func (r *TagRepository) FindTagGroup(id int64) (*model.TagGroup, error) {
	var g model.TagGroup
	err := r.db.First(&g, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &g, nil
}

func (r *TagRepository) UpdateTagGroup(id int64, updates map[string]interface{}) error {
	return r.db.Model(&model.TagGroup{}).Where("id = ?", id).Updates(updates).Error
}

func (r *TagRepository) DeleteTagGroup(id int64) error {
	return r.db.Delete(&model.TagGroup{}, id).Error
}

func (r *TagRepository) CreateSavedSearch(s *model.SavedSearch) error {
	return r.db.Create(s).Error
}

func (r *TagRepository) ListSavedSearches(userID int64) ([]model.SavedSearch, error) {
	var searches []model.SavedSearch
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&searches).Error
	return searches, err
}

func (r *TagRepository) DeleteSavedSearch(id int64, userID int64) error {
	return r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.SavedSearch{}).Error
}
