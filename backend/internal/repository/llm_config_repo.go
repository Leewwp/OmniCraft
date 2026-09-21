package repository

import (
	"context"
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

func (r *LLMConfigRepository) WithTx(tx *gorm.DB) *LLMConfigRepository {
	return &LLMConfigRepository{db: tx}
}

func (r *LLMConfigRepository) List(ctx context.Context) ([]model.LLMConfig, error) {
	var configs []model.LLMConfig
	err := r.db.WithContext(ctx).Order("id ASC").Find(&configs).Error
	return configs, err
}

func (r *LLMConfigRepository) GetByID(ctx context.Context, id int64) (*model.LLMConfig, error) {
	var c model.LLMConfig
	err := r.db.WithContext(ctx).First(&c, id).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *LLMConfigRepository) GetActive(ctx context.Context) (*model.LLMConfig, error) {
	var c model.LLMConfig
	err := r.db.WithContext(ctx).Where("is_active = ?", true).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *LLMConfigRepository) Create(ctx context.Context, c *model.LLMConfig) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *LLMConfigRepository) Update(ctx context.Context, id int64, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.LLMConfig{}).Where("id = ?", id).Updates(updates).Error
}

func (r *LLMConfigRepository) Delete(ctx context.Context, id int64) error {
	c, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if c.IsActive {
		return ErrActiveConfigCannotDelete
	}
	return r.db.WithContext(ctx).Delete(&model.LLMConfig{}, id).Error
}

func (r *LLMConfigRepository) DeleteTx(ctx context.Context, id int64) error {
	c, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if c.IsActive {
		return ErrActiveConfigCannotDelete
	}
	return r.db.WithContext(ctx).Delete(&model.LLMConfig{}, id).Error
}

func (r *LLMConfigRepository) Activate(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.LLMConfig{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.LLMConfig{}).Where("id = ?", id).Update("is_active", true).Error
	})
}

func (r *LLMConfigRepository) ActivateTx(ctx context.Context, id int64) error {
	if err := r.db.WithContext(ctx).Model(&model.LLMConfig{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&model.LLMConfig{}).Where("id = ?", id).Update("is_active", true).Error
}
