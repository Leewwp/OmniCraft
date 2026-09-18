package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

// ErrAgentDraftNotFound is returned when a draft id does not exist or does
// not belong to the requesting user — the two cases are deliberately
// indistinguishable to callers (no ownership probing).
var ErrAgentDraftNotFound = errors.New("agent draft not found")

// AgentDraftRepository owns the agent workspace draft table (SP-23 M2).
// Every method is owner-scoped: the userID argument is the server-bound
// identity, never a caller-supplied one.
type AgentDraftRepository struct {
	db *gorm.DB
}

func NewAgentDraftRepository(db *gorm.DB) *AgentDraftRepository {
	return &AgentDraftRepository{db: db}
}

func (r *AgentDraftRepository) ListForUser(ctx context.Context, userID int64) ([]model.AgentDraft, error) {
	var drafts []model.AgentDraft
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, "active").
		Order("updated_at DESC").
		Limit(50).
		Find(&drafts).Error
	return drafts, err
}

// GetForUser loads one draft; a missing or foreign draft is the same error.
func (r *AgentDraftRepository) GetForUser(ctx context.Context, userID, draftID int64) (*model.AgentDraft, error) {
	var draft model.AgentDraft
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", draftID, userID).
		First(&draft).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAgentDraftNotFound
	}
	if err != nil {
		return nil, err
	}
	return &draft, nil
}

func (r *AgentDraftRepository) Create(ctx context.Context, draft *model.AgentDraft) error {
	return r.db.WithContext(ctx).Create(draft).Error
}

// Update rewrites mutable fields and bumps updated_at — the timestamp that
// invalidates outstanding confirmation tokens (optimistic concurrency).
func (r *AgentDraftRepository) Update(ctx context.Context, draft *model.AgentDraft) error {
	return r.db.WithContext(ctx).
		Model(&model.AgentDraft{}).
		Where("id = ? AND user_id = ?", draft.ID, draft.UserID).
		Updates(map[string]interface{}{
			"title": draft.Title, "tags": draft.Tags, "body": draft.Body,
			"updated_at": draft.UpdatedAt,
		}).Error
}
