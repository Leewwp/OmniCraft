package repository

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"omnicraft/backend/internal/model"
)

// collabInviteExpiryAdvisoryLockID is the stable transaction-scoped advisory
// lock used by the invite expiry sweep so only one replica marks expired
// invites in a batch.
const collabInviteExpiryAdvisoryLockID int64 = 0x4f4d4e494e564954 // "OMNINVIT"

// CollabInviteRepository persists collaboration invites and the author
// blocklist checks used by the invite anti-abuse chain.
type CollabInviteRepository struct {
	db *gorm.DB
}

func NewCollabInviteRepository(db *gorm.DB) *CollabInviteRepository {
	return &CollabInviteRepository{db: db}
}

// FindByIDForUpdate locks the invite row for UPDATE inside a transaction and
// returns it, or nil when it does not exist.
func (r *CollabInviteRepository) FindByIDForUpdate(id int64) (*model.CollabInvite, error) {
	var invite model.CollabInvite
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&invite, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &invite, nil
}

// UpdateStatus transitions the invite to a terminal status and records the
// response time.
func (r *CollabInviteRepository) UpdateStatus(id int64, status string, respondedAt *time.Time) error {
	return r.db.Model(&model.CollabInvite{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"status": status, "responded_at": respondedAt}).Error
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

// ExpirePendingIfLeader marks pending invites older than expireDays as
// expired. The update runs inside a transaction that first takes a
// transaction-scoped PostgreSQL advisory leader lock, so a replica that loses
// the race commits without touching any rows (acquired=false). Non-postgres
// dialects skip the lock and always sweep.
func (r *CollabInviteRepository) ExpirePendingIfLeader(expireDays int, now time.Time) (expired int64, acquired bool, err error) {
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.AddDate(0, 0, -expireDays)
	acquired = true

	err = r.db.Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if lockErr := tx.Raw("SELECT pg_try_advisory_xact_lock(?)", collabInviteExpiryAdvisoryLockID).Scan(&acquired).Error; lockErr != nil {
				return lockErr
			}
			if !acquired {
				return nil
			}
		}

		result := tx.Model(&model.CollabInvite{}).
			Where("status = ? AND created_at < ?", model.CollabInviteStatusPending, cutoff).
			Update("status", model.CollabInviteStatusExpired)
		expired = result.RowsAffected
		return result.Error
	})
	return expired, acquired, err
}
