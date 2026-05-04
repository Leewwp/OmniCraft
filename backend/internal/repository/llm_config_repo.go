package repository

import (
	"errors"

	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

var ErrActiveConfigCannotDelete = errors.New("cannot delete active config")

type LLMConfigRepository struct {
	db *gorm.DB
}

func NewLLMConfigRepository(db *gorm.DB) *LLMConfigRepository {
	return &LLMConfigRepository{db: db}
}

func (r *LLMConfigRepository) List() ([]model.LLMConfig, error) {
	var configs []model.LLMConfig
	err := r.db.Order("id ASC").Find(&configs).Error
	return configs, err
}

func (r *LLMConfigRepository) GetByID(id int64) (*model.LLMConfig, error) {
	var c model.LLMConfig
	err := r.db.First(&c, id).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *LLMConfigRepository) GetActive() (*model.LLMConfig, error) {
	var c model.LLMConfig
	err := r.db.Where("is_active = ?", true).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *LLMConfigRepository) Create(c *model.LLMConfig) error {
	return r.db.Create(c).Error
}

func (r *LLMConfigRepository) Update(id int64, updates map[string]interface{}) error {
	return r.db.Model(&model.LLMConfig{}).Where("id = ?", id).Updates(updates).Error
}

func (r *LLMConfigRepository) Delete(id int64) error {
	c, err := r.GetByID(id)
	if err != nil {
		return err
	}
	if c.IsActive {
		return ErrActiveConfigCannotDelete
	}
	return r.db.Delete(&model.LLMConfig{}, id).Error
}

func (r *LLMConfigRepository) Activate(id int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.LLMConfig{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.LLMConfig{}).Where("id = ?", id).Update("is_active", true).Error
	})
}
