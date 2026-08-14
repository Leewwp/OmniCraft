package repository

import (
	"context"

	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Create(n *model.Notification) error {
	return r.db.Create(n).Error
}

// CreateTx persists a notification inside the caller's transaction. The
// notification worker uses it so the row and the inbox completion record
// commit (or roll back) together.
func (r *NotificationRepository) CreateTx(ctx context.Context, tx *gorm.DB, n *model.Notification) error {
	return tx.WithContext(ctx).Create(n).Error
}

func (r *NotificationRepository) ListActiveRecipientIDs() ([]int64, error) {
	return r.ListActiveRecipientIDsWithContext(context.Background())
}

func (r *NotificationRepository) ListActiveRecipientIDsWithContext(ctx context.Context) ([]int64, error) {
	var ids []int64
	err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("is_banned = ? AND deleted_at IS NULL", false).
		Order("id ASC").
		Pluck("id", &ids).Error
	return ids, err
}

func (r *NotificationRepository) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}

func (r *NotificationRepository) CreateBroadcastBatch(rows []model.Notification) error {
	return r.CreateBroadcastBatchTx(context.Background(), r.db, rows)
}

func (r *NotificationRepository) CreateBroadcastBatchTx(ctx context.Context, tx *gorm.DB, rows []model.Notification) error {
	if len(rows) == 0 {
		return nil
	}
	return tx.WithContext(ctx).CreateInBatches(rows, 500).Error
}

func (r *NotificationRepository) CreateBroadcastRequestTx(ctx context.Context, tx *gorm.DB, row *model.NotificationBroadcastRequest) error {
	return tx.WithContext(ctx).Create(row).Error
}

func (r *NotificationRepository) GetBroadcastRequestByKeyHash(ctx context.Context, actorID int64, keyHash string) (*model.NotificationBroadcastRequest, error) {
	var row model.NotificationBroadcastRequest
	if err := r.db.WithContext(ctx).
		Where("actor_id = ? AND key_hash = ?", actorID, keyHash).
		First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *NotificationRepository) List(userID int64, channel string, page, pageSize int) ([]model.Notification, int64, error) {
	var total int64
	q := r.db.Model(&model.Notification{}).Where("user_id = ?", userID)
	if channel != "" {
		q = q.Where("channel = ?", channel)
	}
	q.Count(&total)

	var notifications []model.Notification
	err := q.Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&notifications).Error
	return notifications, total, err
}

func (r *NotificationRepository) MarkRead(id int64, userID int64) error {
	return r.db.Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", true).Error
}

func (r *NotificationRepository) MarkAllRead(userID int64, channel string) error {
	q := r.db.Model(&model.Notification{}).Where("user_id = ? AND is_read = FALSE", userID)
	if channel != "" {
		q = q.Where("channel = ?", channel)
	}
	return q.Update("is_read", true).Error
}

func (r *NotificationRepository) UnreadCount(userID int64) (map[string]int64, error) {
	type result struct {
		Channel string
		Count   int64
	}
	var rows []result
	err := r.db.Model(&model.Notification{}).
		Select("channel, COUNT(*) as count").
		Where("user_id = ? AND is_read = FALSE", userID).
		Group("channel").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	counts := map[string]int64{}
	for _, row := range rows {
		counts[row.Channel] = row.Count
	}
	return counts, nil
}
