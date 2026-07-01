package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/repository"

	"gorm.io/gorm"
)

type NotificationService struct {
	notifRepo     *repository.NotificationRepository
	queueProducer queue.Producer
	auditSvc      *AdminAuditService
}

func NewNotificationService(notifRepo *repository.NotificationRepository) *NotificationService {
	return &NotificationService{notifRepo: notifRepo, queueProducer: queue.NewNoopProducer()}
}

func (s *NotificationService) SetQueueProducer(p queue.Producer) {
	s.queueProducer = p
}

func (s *NotificationService) SetAdminAuditService(auditSvc *AdminAuditService) {
	s.auditSvc = auditSvc
}

var (
	ErrBroadcastValidation       = errors.New("VALIDATION_ERROR")
	errBroadcastAuditUnavailable = errors.New("BROADCAST_AUDIT_UNAVAILABLE")
	errBroadcastSendFailed       = errors.New("BROADCAST_SEND_FAILED")
)

type BroadcastValidationError struct {
	Code   string
	Fields []string
}

func (e *BroadcastValidationError) Error() string {
	if e == nil || e.Code == "" {
		return ErrBroadcastValidation.Error()
	}
	return e.Code
}

func (e *BroadcastValidationError) Is(target error) bool {
	return target == ErrBroadcastValidation
}

func (s *NotificationService) BroadcastSystemNotification(ctx context.Context, title, body, channel string, actorID int64) (int, time.Time, error) {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	channel = strings.TrimSpace(channel)
	if channel == "" {
		channel = "broadcast"
	}

	titleLength := len([]rune(title))
	bodyLength := len([]rune(body))
	validationFields := validateBroadcastNotification(titleLength, bodyLength, channel)
	if len(validationFields) > 0 {
		auditErr := s.recordBroadcastAudit(ctx, actorID, "rejected", map[string]interface{}{
			"title_length":          titleLength,
			"body_length":           bodyLength,
			"validation_error_code": ErrBroadcastValidation.Error(),
			"validation_fields":     validationFields,
		})
		if auditErr != nil {
			slog.Warn("broadcast validation audit write failed",
				slog.Int64("actor_id", actorID),
				slog.String("error", auditErr.Error()),
			)
		}
		return 0, time.Time{}, &BroadcastValidationError{
			Code:   ErrBroadcastValidation.Error(),
			Fields: validationFields,
		}
	}

	if s.auditSvc == nil {
		return 0, time.Time{}, errBroadcastAuditUnavailable
	}

	recipientIDs, err := s.notifRepo.ListActiveRecipientIDsWithContext(ctx)
	if err != nil {
		_ = s.recordBroadcastAudit(ctx, actorID, "failed", map[string]interface{}{
			"title_length":    titleLength,
			"body_length":     bodyLength,
			"recipient_count": 0,
			"error_code":      "RECIPIENT_QUERY_FAILED",
		})
		return 0, time.Time{}, errBroadcastSendFailed
	}

	broadcastAt := time.Now().UTC()
	rows := make([]model.Notification, 0, len(recipientIDs))
	for _, recipientID := range recipientIDs {
		titleCopy := title
		bodyCopy := body
		rows = append(rows, model.Notification{
			UserID:    recipientID,
			Type:      "system",
			Channel:   "broadcast",
			Title:     &titleCopy,
			Body:      &bodyCopy,
			CreatedAt: broadcastAt,
		})
	}

	failureCode := "BROADCAST_INSERT_FAILED"
	err = s.notifRepo.Transaction(ctx, func(tx *gorm.DB) error {
		if err := s.notifRepo.CreateBroadcastBatchTx(ctx, tx, rows); err != nil {
			failureCode = "BROADCAST_INSERT_FAILED"
			return err
		}
		if err := s.recordBroadcastAuditTx(ctx, tx, actorID, "success", map[string]interface{}{
			"recipient_count": len(recipientIDs),
			"title_length":    titleLength,
			"body_length":     bodyLength,
			"filter":          "active_users",
		}); err != nil {
			failureCode = "AUDIT_WRITE_FAILED"
			return err
		}
		return nil
	})
	if err != nil {
		_ = s.recordBroadcastAudit(ctx, actorID, "failed", map[string]interface{}{
			"title_length":    titleLength,
			"body_length":     bodyLength,
			"recipient_count": len(recipientIDs),
			"error_code":      failureCode,
		})
		return 0, time.Time{}, errBroadcastSendFailed
	}

	return len(recipientIDs), broadcastAt, nil
}

func validateBroadcastNotification(titleLength, bodyLength int, channel string) []string {
	var fields []string
	if titleLength == 0 || titleLength > 120 {
		fields = append(fields, "title")
	}
	if bodyLength == 0 || bodyLength > 5000 {
		fields = append(fields, "body")
	}
	if channel != "broadcast" {
		fields = append(fields, "channel")
	}
	return fields
}

func (s *NotificationService) recordBroadcastAuditTx(ctx context.Context, tx *gorm.DB, actorID int64, result string, metadata map[string]interface{}) error {
	if s.auditSvc == nil {
		return errBroadcastAuditUnavailable
	}
	return s.auditSvc.RecordTx(ctx, tx, RecordAdminAuditInput{
		AdminUserID: actorID,
		Action:      "broadcast_notification",
		TargetType:  "users",
		Metadata:    metadata,
		Result:      result,
	})
}

func (s *NotificationService) recordBroadcastAudit(ctx context.Context, actorID int64, result string, metadata map[string]interface{}) error {
	if s.auditSvc == nil {
		return errBroadcastAuditUnavailable
	}
	return s.auditSvc.Record(ctx, RecordAdminAuditInput{
		AdminUserID: actorID,
		Action:      "broadcast_notification",
		TargetType:  "users",
		Metadata:    metadata,
		Result:      result,
	})
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
