package repository

import (
	"time"

	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

// ipPublicStatus is the only status under which an IP entity is publicly
// accessible; visits to pending/banned IPs must not be recorded or listed.
const ipPublicStatus = "approved"

// RecentIPVisitLimit caps every visible recent list to the established six.
const RecentIPVisitLimit = 6

// IPVisitHistoryItem is one recent visit joined with the public IP summary
// fields needed to render the list without exposing non-public entities.
type IPVisitHistoryItem struct {
	IPID          int64
	IPName        string
	IPSlug        string
	IPDescription string
	IPCoverURL    string
	IPCategory    string
	VisitedAt     time.Time
}

type IPVisitHistoryRepository struct {
	db *gorm.DB
}

func NewIPVisitHistoryRepository(db *gorm.DB) *IPVisitHistoryRepository {
	return &IPVisitHistoryRepository{db: db}
}

// RecordVisit checks that the IP is publicly accessible and then records or
// refreshes one visit with the given server-side timestamp. It returns false
// when the IP does not exist or is not public; no history row is written in
// that case. Repeated calls are idempotent and never lower recency.
func (r *IPVisitHistoryRepository) RecordVisit(userID, ipID int64, visitedAt time.Time) (bool, error) {
	found := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.IP{}).
			Where("id = ? AND status = ?", ipID, ipPublicStatus).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return nil
		}
		found = true
		return upsertVisit(tx, userID, ipID, visitedAt)
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

// MergeVisits upserts the accepted visits of one account inside a single
// transaction. Visits whose IP is missing or no longer publicly accessible do
// not block the merge: they are reported in discarded and never written.
// The resulting ordered recent list is read inside the same transaction so a
// success response always reflects the merged state.
func (r *IPVisitHistoryRepository) MergeVisits(userID int64, visits []model.IPVisitHistory) (accepted, discarded []int64, items []IPVisitHistoryItem, err error) {
	err = r.db.Transaction(func(tx *gorm.DB) error {
		if len(visits) > 0 {
			ids := make([]int64, 0, len(visits))
			for _, v := range visits {
				ids = append(ids, v.IPID)
			}
			var validIDs []int64
			if err := tx.Model(&model.IP{}).
				Where("id IN ? AND status = ?", ids, ipPublicStatus).
				Pluck("id", &validIDs).Error; err != nil {
				return err
			}
			valid := make(map[int64]bool, len(validIDs))
			for _, id := range validIDs {
				valid[id] = true
			}
			for _, v := range visits {
				if !valid[v.IPID] {
					discarded = append(discarded, v.IPID)
					continue
				}
				accepted = append(accepted, v.IPID)
				if err := upsertVisit(tx, userID, v.IPID, v.VisitedAt); err != nil {
					return err
				}
			}
		}
		items, err = listRecentVisits(tx, userID, RecentIPVisitLimit)
		return err
	})
	return accepted, discarded, items, err
}

// ListRecent returns the current user's most recent publicly accessible
// visits ordered by visited_at DESC with a stable ip_id DESC tie-break,
// capped at RecentIPVisitLimit.
func (r *IPVisitHistoryRepository) ListRecent(userID int64, limit int) ([]IPVisitHistoryItem, error) {
	return listRecentVisits(r.db, userID, limit)
}

func upsertVisit(tx *gorm.DB, userID, ipID int64, visitedAt time.Time) error {
	return tx.Exec(`
		INSERT INTO ip_visit_history (user_id, ip_id, visited_at)
		VALUES (?, ?, ?)
		ON CONFLICT (user_id, ip_id)
		DO UPDATE SET visited_at = GREATEST(ip_visit_history.visited_at, EXCLUDED.visited_at)
	`, userID, ipID, visitedAt).Error
}

func listRecentVisits(db *gorm.DB, userID int64, limit int) ([]IPVisitHistoryItem, error) {
	var items []IPVisitHistoryItem
	err := db.Table("ip_visit_history").
		Select(`
			ip_visit_history.ip_id AS ip_id,
			ips.name AS ip_name,
			ips.slug AS ip_slug,
			ips.description AS ip_description,
			ips.cover_url AS ip_cover_url,
			ips.category AS ip_category,
			ip_visit_history.visited_at AS visited_at
		`).
		Joins("JOIN ips ON ips.id = ip_visit_history.ip_id AND ips.status = ?", ipPublicStatus).
		Where("ip_visit_history.user_id = ?", userID).
		Order("ip_visit_history.visited_at DESC, ip_visit_history.ip_id DESC").
		Limit(limit).
		Scan(&items).Error
	return items, err
}
