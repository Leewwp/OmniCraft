package repository

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type IPRepository struct {
	db *gorm.DB
}

func NewIPRepository(db *gorm.DB) *IPRepository {
	return &IPRepository{db: db}
}

type ListIPsFilter struct {
	Search   string
	Category string
	Status   string
	Page     int
	PageSize int
}

func (r *IPRepository) CreateIP(ip *model.IP) error {
	return r.db.Create(ip).Error
}

func (r *IPRepository) FindByID(id int64) (*model.IP, error) {
	var ip model.IP
	err := r.db.First(&ip, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &ip, nil
}

func (r *IPRepository) FindBySlug(slug string) (*model.IP, error) {
	var ip model.IP
	err := r.db.Where("slug = ?", slug).First(&ip).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &ip, nil
}

func (r *IPRepository) ListIPs(f ListIPsFilter) ([]model.IP, int64, error) {
	var ips []model.IP
	var total int64

	q := r.db.Model(&model.IP{})

	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	} else {
		q = q.Where("status = ?", "approved")
	}

	if f.Category != "" {
		q = q.Where("category = ?", f.Category)
	}

	if f.Search != "" {
		q = q.Where("to_tsvector('simple', name) @@ plainto_tsquery('simple', ?)", f.Search)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
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
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&ips).Error; err != nil {
		return nil, 0, err
	}

	return ips, total, nil
}

func (r *IPRepository) UpdateStatus(id int64, status string) error {
	return r.db.Model(&model.IP{}).Where("id = ?", id).Update("status", status).Error
}

func (r *IPRepository) BanIPAndContents(id int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.IP{}).Where("id = ?", id).Update("status", "banned").Error; err != nil {
			return err
		}
		return tx.Model(&model.ContentItem{}).Where("ip_id = ?", id).Update("status", "banned").Error
	})
}
