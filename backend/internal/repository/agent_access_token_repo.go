package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

// AgentAccessTokenRepository persists PAT rows. All reads used by the auth
// path operate on the live (non-revoked) subset only.
type AgentAccessTokenRepository struct {
	db *gorm.DB
}

func NewAgentAccessTokenRepository(db *gorm.DB) *AgentAccessTokenRepository {
	return &AgentAccessTokenRepository{db: db}
}

func (r *AgentAccessTokenRepository) Create(ctx context.Context, token *model.AgentAccessToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *AgentAccessTokenRepository) FindActiveByHash(ctx context.Context, tokenHash string) (*model.AgentAccessToken, error) {
	var row model.AgentAccessToken
	err := r.db.WithContext(ctx).
		Where("token_hash = ? AND revoked_at IS NULL", tokenHash).
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *AgentAccessTokenRepository) ListActiveByUser(ctx context.Context, userID int64) ([]model.AgentAccessToken, error) {
	var rows []model.AgentAccessToken
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Order("created_at DESC, id DESC").
		Find(&rows).Error
	return rows, err
}

func (r *AgentAccessTokenRepository) CountActiveByUser(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.AgentAccessToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Count(&count).Error
	return count, err
}

// RevokeByIDAndUser marks the caller's own live token revoked and reports
// the affected row count; 0 means the token does not exist, is already
// revoked, or belongs to another user.
func (r *AgentAccessTokenRepository) RevokeByIDAndUser(ctx context.Context, userID, tokenID int64, at time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&model.AgentAccessToken{}).
		Where("id = ? AND user_id = ? AND revoked_at IS NULL", tokenID, userID).
		Update("revoked_at", at)
	return res.RowsAffected, res.Error
}

func (r *AgentAccessTokenRepository) TouchLastUsed(ctx context.Context, tokenID int64, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.AgentAccessToken{}).
		Where("id = ?", tokenID).
		Update("last_used_at", at).Error
}
