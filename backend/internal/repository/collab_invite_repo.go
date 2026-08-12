package repository

import (
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

// CollabInviteRepository persists collaboration invites and the author
// blocklist checks used by the invite anti-abuse chain.
type CollabInviteRepository struct {
	db *gorm.DB
}

func NewCollabInviteRepository(db *gorm.DB) *CollabInviteRepository {
	return &CollabInviteRepository{db: db}
}

// DB exposes the underlying handle so callers can build tx-scoped
// repositories for a shared transaction.
func (r *CollabInviteRepository) DB() *gorm.DB {
	return r.db
}

// Transaction runs fn on a tx-scoped repository.
func (r *CollabInviteRepository) Transaction(fn func(tx *CollabInviteRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		txRepo := &CollabInviteRepository{db: tx}
		return fn(txRepo)
	})
}

// CreateInvite inserts a new invite row.
func (r *CollabInviteRepository) CreateInvite(invite *model.CollabInvite) error {
	return r.db.Create(invite).Error
}

// UpdateMessageID backfills the invite card message id after the typed
// message has been created.
func (r *CollabInviteRepository) UpdateMessageID(inviteID, messageID int64) error {
	return r.db.Model(&model.CollabInvite{}).
		Where("id = ?", inviteID).
		Update("message_id", messageID).Error
}

// IsBlockedByEither reports whether userA blocks userB or userB blocks
// userA (bidirectional author_blocklist check).
func (r *CollabInviteRepository) IsBlockedByEither(userA, userB int64) (bool, error) {
	var count int64
	err := r.db.Model(&model.AuthorBlocklist{}).
		Where("(author_id = ? AND blocked_id = ?) OR (author_id = ? AND blocked_id = ?)", userA, userB, userB, userA).
		Count(&count).Error
	return count > 0, err
}

// HasActiveInvite reports whether a pending or accepted invite already
// exists for the content/invitee pair; expired and declined invites do not
// block a re-invite.
func (r *CollabInviteRepository) HasActiveInvite(contentID, inviteeID int64) (bool, error) {
	var count int64
	err := r.db.Model(&model.CollabInvite{}).
		Where("content_id = ? AND invitee_id = ? AND status IN ?", contentID, inviteeID,
			[]string{model.CollabInviteStatusPending, model.CollabInviteStatusAccepted}).
		Count(&count).Error
	return count > 0, err
}
