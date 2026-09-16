package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"omnicraft/backend/internal/model"
)

// ErrPromptVersionExists is returned when a (name, version) row already
// exists; version rows are immutable, callers must create the next version.
var ErrPromptVersionExists = errors.New("prompt version already exists")

// ErrPromptNotFound is returned when a name/version/label lookup misses.
var ErrPromptNotFound = errors.New("prompt not found")

// PromptRegistryRepository owns prompt_registry + prompt_labels. Content
// rows are insert-only (immutability); labels are upserted pointers.
type PromptRegistryRepository struct {
	db *gorm.DB
}

func NewPromptRegistryRepository(db *gorm.DB) *PromptRegistryRepository {
	return &PromptRegistryRepository{db: db}
}

// GetByVersion returns one immutable version row.
func (r *PromptRegistryRepository) GetByVersion(ctx context.Context, name string, version int) (*model.PromptRegistry, error) {
	var row model.PromptRegistry
	err := r.db.WithContext(ctx).
		Where("name = ? AND version = ?", name, version).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPromptNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// LatestVersion returns the highest version row of one prompt name.
func (r *PromptRegistryRepository) LatestVersion(ctx context.Context, name string) (*model.PromptRegistry, error) {
	var row model.PromptRegistry
	err := r.db.WithContext(ctx).
		Where("name = ?", name).
		Order("version DESC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPromptNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ListVersions returns all versions of one prompt name, newest first.
func (r *PromptRegistryRepository) ListVersions(ctx context.Context, name string) ([]model.PromptRegistry, error) {
	var rows []model.PromptRegistry
	err := r.db.WithContext(ctx).
		Where("name = ?", name).
		Order("version DESC").
		Find(&rows).Error
	return rows, err
}

// DistinctNames returns every prompt name that has at least one version row.
func (r *PromptRegistryRepository) DistinctNames(ctx context.Context) ([]string, error) {
	var names []string
	err := r.db.WithContext(ctx).
		Model(&model.PromptRegistry{}).
		Distinct("name").
		Order("name").
		Pluck("name", &names).Error
	return names, err
}

// CreateVersion inserts a new immutable version row. A duplicate
// (name, version) is rejected with ErrPromptVersionExists.
func (r *PromptRegistryRepository) CreateVersion(ctx context.Context, row *model.PromptRegistry) error {
	if len(row.RequiredPlaceholders) == 0 {
		row.RequiredPlaceholders = model.JSONB("[]")
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}, {Name: "version"}},
		DoNothing: true,
	}).Create(row).Error
	if err != nil {
		return err
	}
	if row.ID == 0 {
		return ErrPromptVersionExists
	}
	return nil
}

// GetByLabel resolves the version row a label points at.
func (r *PromptRegistryRepository) GetByLabel(ctx context.Context, name, label string) (*model.PromptRegistry, error) {
	var pointer model.PromptLabel
	err := r.db.WithContext(ctx).
		Where("name = ? AND label = ?", name, label).
		First(&pointer).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPromptNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.GetByVersion(ctx, name, pointer.Version)
}

// ListLabels returns every label pointer of one prompt name.
func (r *PromptRegistryRepository) ListLabels(ctx context.Context, name string) ([]model.PromptLabel, error) {
	var rows []model.PromptLabel
	err := r.db.WithContext(ctx).
		Where("name = ?", name).
		Order("label").
		Find(&rows).Error
	return rows, err
}

// SetLabel moves a label pointer to a version (upsert; the version must
// already exist). Rolling back = pointing at an older version.
func (r *PromptRegistryRepository) SetLabel(ctx context.Context, name, label string, version int) error {
	if _, err := r.GetByVersion(ctx, name, version); err != nil {
		return err
	}
	now := time.Now()
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}, {Name: "label"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"version":    version,
			"updated_at": now,
		}),
	}).Create(&model.PromptLabel{Name: name, Label: label, Version: version, UpdatedAt: now}).Error
}

// EnsureLabel seeds a label pointer only when none exists (v1 bootstrap);
// it never moves an admin-managed pointer.
func (r *PromptRegistryRepository) EnsureLabel(ctx context.Context, name, label string, version int) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}, {Name: "label"}},
		DoNothing: true,
	}).Create(&model.PromptLabel{Name: name, Label: label, Version: version}).Error
}

// ListAllLabels returns every label pointer (admin slot overview).
func (r *PromptRegistryRepository) ListAllLabels(ctx context.Context) ([]model.PromptLabel, error) {
	var rows []model.PromptLabel
	err := r.db.WithContext(ctx).Order("name, label").Find(&rows).Error
	return rows, err
}
