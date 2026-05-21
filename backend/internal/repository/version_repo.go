package repository

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type VersionRepository struct {
	db *gorm.DB
}

func NewVersionRepository(db *gorm.DB) *VersionRepository {
	return &VersionRepository{db: db}
}

func (r *VersionRepository) CreateVersion(v *model.ContentVersion) error {
	return r.db.Create(v).Error
}

func (r *VersionRepository) FindByID(id int64) (*model.ContentVersion, error) {
	var v model.ContentVersion
	err := r.db.First(&v, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}

func (r *VersionRepository) ListByContent(contentID int64) ([]model.ContentVersion, error) {
	var versions []model.ContentVersion
	err := r.db.Where("content_item_id = ? AND status != ?", contentID, "pending").
		Order("version_number ASC").Find(&versions).Error
	return versions, err
}

func (r *VersionRepository) ListByContentPaged(contentID int64, page, pageSize int) ([]model.ContentVersion, int64, error) {
	var total int64
	if err := r.db.Model(&model.ContentVersion{}).Where("content_item_id = ? AND status != ?", contentID, "pending").Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var versions []model.ContentVersion
	err := r.db.Where("content_item_id = ? AND status != ?", contentID, "pending").
		Order("version_number ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&versions).Error
	return versions, total, err
}

func (r *VersionRepository) GetLatest(contentID int64) (*model.ContentVersion, error) {
	var v model.ContentVersion
	err := r.db.Where("content_item_id = ? AND is_latest = TRUE", contentID).First(&v).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}

func (r *VersionRepository) SetLatest(contentID int64, versionID int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ContentVersion{}).
			Where("content_item_id = ?", contentID).
			Update("is_latest", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.ContentVersion{}).
			Where("id = ?", versionID).
			Update("is_latest", true).Error
	})
}

func (r *VersionRepository) GetVersionChain(contentID int64) ([]model.ContentVersion, error) {
	var versions []model.ContentVersion
	err := r.db.Where("content_item_id = ?", contentID).
		Order("version_number ASC").Find(&versions).Error
	return versions, err
}
