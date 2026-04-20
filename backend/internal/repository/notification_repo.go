package repository

import (
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

func (r *NotificationRepository) List(userID int64, channel string, page, pageSize int) ([]model.Notification, int64, error) {
	var total int64
	q := r.db.Model(&model.Notification{}).Where("user_id = ?", userID)
	if channel != "" {
		q = q.Where("channel = ?", channel)
	}
	q.Count(&total)

	var notifications []model.Notification
	err := q.Order("created_at DESC").
		Offset((page-1)*pageSize).Limit(pageSize).
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
