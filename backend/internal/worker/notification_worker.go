package worker

import (
	"context"
	"encoding/json"
	"log/slog"

	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
)

// NotificationWorker persists notifications from the notification.create
// topic. The notification row and the inbox completion record commit in one
// database transaction, so a duplicate delivery (same stream message id after
// an ACK-loss redelivery) is skipped by the UNIQUE(consumer_group, event_id)
// guard and never creates a second notification.
type NotificationWorker struct {
	db        *gorm.DB
	notifRepo *repository.NotificationRepository
}

func NewNotificationWorker(notifRepo *repository.NotificationRepository, db *gorm.DB) *NotificationWorker {
	return &NotificationWorker{
		db:        db,
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

	// System callers pass sender_id=0 ("no sender"); persisting 0 violates
	// notifications_sender_id_fkey on Postgres, so map it to NULL here. A
	// real sender id is kept untouched.
	var senderID *int64
	if payload.SenderID != 0 {
		senderID = &payload.SenderID
	}
	n := &model.Notification{
		UserID:     payload.UserID,
		Channel:    payload.Channel,
		Type:       payload.Type,
		Title:      &payload.Title,
		Body:       &payload.Body,
		TargetType: &payload.TargetType,
		TargetID:   &payload.TargetID,
		SenderID:   senderID,
	}

	_, err := ConsumeInboxTx(ctx, w.db, msg.Group, InboxEventID(msg.Group, msg), func(ctx context.Context, tx *gorm.DB) error {
		return w.notifRepo.CreateTx(ctx, tx, n)
	})
	if err != nil {
		slog.Error("notification_worker: failed to create notification",
			"user_id", payload.UserID, "channel", payload.Channel, "error", err)
		return err
	}

	slog.Info("notification_worker: notification created",
		"user_id", payload.UserID, "channel", payload.Channel, "type", payload.Type)
	return nil
}
