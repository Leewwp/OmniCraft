package worker

import (
	"context"
	"encoding/json"
	"log/slog"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
)

type NotificationWorker struct {
	notifRepo *repository.NotificationRepository
}

func NewNotificationWorker(notifRepo *repository.NotificationRepository) *NotificationWorker {
	return &NotificationWorker{
		notifRepo: notifRepo,
	}
}

func (w *NotificationWorker) Handle(ctx context.Context, msg queue.Message) error {
	var payload struct {
		UserID     int64  `json:"user_id"`
		Channel    string `json:"channel"`
		Type       string `json:"type"`
		Title      string `json:"title"`
		Body       string `json:"body"`
		TargetType string `json:"target_type"`
		TargetID   int64  `json:"target_id"`
		SenderID   int64  `json:"sender_id"`
	}

	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		slog.Error("notification_worker: failed to unmarshal payload", "msg_id", msg.ID, "error", err)
		return err
	}

	slog.Info("notification_worker: processing message",
		"msg_id", msg.ID, "user_id", payload.UserID, "channel", payload.Channel, "type", payload.Type)

	n := &model.Notification{
		UserID:     payload.UserID,
		Channel:    payload.Channel,
		Type:       payload.Type,
		Title:      &payload.Title,
		Body:       &payload.Body,
		TargetType: &payload.TargetType,
		TargetID:   &payload.TargetID,
		SenderID:   &payload.SenderID,
	}

	if err := w.notifRepo.Create(n); err != nil {
		slog.Error("notification_worker: failed to create notification",
			"user_id", payload.UserID, "channel", payload.Channel, "error", err)
		return err
	}

	slog.Info("notification_worker: notification created",
		"user_id", payload.UserID, "channel", payload.Channel, "type", payload.Type)
	return nil
}