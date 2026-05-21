package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/repository"
)

type NotificationService struct {
	notifRepo     *repository.NotificationRepository
	queueProducer queue.Producer
}

func NewNotificationService(notifRepo *repository.NotificationRepository) *NotificationService {
	return &NotificationService{notifRepo: notifRepo, queueProducer: queue.NewNoopProducer()}
}

func (s *NotificationService) SetQueueProducer(p queue.Producer) {
	s.queueProducer = p
}

func (s *NotificationService) Notify(userID int64, channel, notifType, title, body, targetType string, targetID int64, senderID int64) {
	if _, ok := s.queueProducer.(*queue.NoopProducer); !ok && s.queueProducer != nil {
		payload, _ := json.Marshal(map[string]interface{}{
			"user_id":     userID,
			"channel":     channel,
			"type":        notifType,
			"title":       title,
			"body":        body,
			"target_type": targetType,
			"target_id":   targetID,
			"sender_id":   senderID,
		})
		if err := s.queueProducer.Publish(context.Background(), "notification.create", payload); err != nil {
			slog.Error("failed to publish notification message", "user_id", userID, "channel", channel, "error", err)
		}
		return
	}
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