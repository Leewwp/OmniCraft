package service

import (
	"log/slog"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/repository"
)

type NotificationService struct {
	notifRepo *repository.NotificationRepository
}

func NewNotificationService(notifRepo *repository.NotificationRepository) *NotificationService {
	return &NotificationService{notifRepo: notifRepo}
}

func (s *NotificationService) Notify(userID int64, channel, notifType, title, body, targetType string, targetID int64, senderID int64) {
	recovery.GoSafe(func() {
		n := &model.Notification{
			UserID:     userID,
			Channel:    channel,
			Type:       notifType,
			Title:      &title,
			Body:       &body,
			TargetType: &targetType,
			TargetID:   &targetID,
			SenderID:   &senderID,
		}
		if err := s.notifRepo.Create(n); err != nil {
			slog.Error("failed to create notification", "error", err, "user_id", userID, "channel", channel)
		}
	})
}
