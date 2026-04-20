package repository

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type CategoryRepository struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) ListByZoneAndLevel(zone, level string, parentID *int64) ([]model.Category, error) {
	var categories []model.Category
	q := r.db.Model(&model.Category{}).Where("is_active = TRUE")
	if zone != "" {
		q = q.Where("zone = ?", zone)
	}
	if level != "" {
		q = q.Where("level = ?", level)
	}
	if parentID != nil {
		q = q.Where("parent_id = ?", *parentID)
	} else if level != "" && level != "primary" {
		q = q.Where("parent_id IS NOT NULL")
	}
	err := q.Order("sort_order ASC").Find(&categories).Error
	return categories, err
}

func (r *CategoryRepository) GetBySlug(slug string) (*model.Category, error) {
	var cat model.Category
	err := r.db.Where("slug = ?", slug).First(&cat).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &cat, nil
}

func (r *CategoryRepository) FindByID(id int64) (*model.Category, error) {
	var cat model.Category
	err := r.db.First(&cat, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &cat, nil
}

func (r *CategoryRepository) Create(cat *model.Category) error {
	return r.db.Create(cat).Error
}

func (r *CategoryRepository) Update(id int64, updates map[string]interface{}) error {
	return r.db.Model(&model.Category{}).Where("id = ?", id).Updates(updates).Error
}

func (r *CategoryRepository) Delete(id int64) error {
	return r.db.Delete(&model.Category{}, id).Error
}

func (r *CategoryRepository) HasChildren(id int64) (bool, error) {
	var count int64
	err := r.db.Model(&model.Category{}).Where("parent_id = ?", id).Count(&count).Error
	return count > 0, err
}

func (r *CategoryRepository) HasLinkedContent(id int64) (bool, error) {
	var cat model.Category
	if err := r.db.First(&cat, id).Error; err != nil {
		return false, err
	}
	var count int64
	r.db.Raw("SELECT COUNT(*) FROM content_items WHERE category = (SELECT slug FROM categories WHERE id = ?)", id).Scan(&count)
	return count > 0, nil
}

func (r *CategoryRepository) UpdateSortOrder(updates []struct {
	ID        int64 `json:"id"`
	SortOrder int   `json:"sort_order"`
}) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, u := range updates {
			if err := tx.Model(&model.Category{}).Where("id = ?", u.ID).Update("sort_order", u.SortOrder).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
