package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/repository"

	sqlite "github.com/glebarez/go-sqlite"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	sqlite3 "modernc.org/sqlite/lib"
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
	ErrIdempotencyKeyRequired    = errors.New("IDEMPOTENCY_KEY_REQUIRED")
	ErrIdempotencyKeyReused      = errors.New("IDEMPOTENCY_KEY_REUSED")
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

func (s *NotificationService) BroadcastSystemNotification(ctx context.Context, title, body, channel string, actorID int64, idempotencyKey string) (int, time.Time, bool, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return 0, time.Time{}, false, ErrIdempotencyKeyRequired
	}

	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	channel = strings.TrimSpace(channel)
	if channel == "" {
		channel = "broadcast"
	}

	keyHash := hashBroadcastIdempotencyKey(idempotencyKey)
	payloadHash := hashBroadcastPayload(title, body, channel)

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
		return 0, time.Time{}, false, &BroadcastValidationError{
			Code:   ErrBroadcastValidation.Error(),
			Fields: validationFields,
		}
	}

	if s.auditSvc == nil {
		return 0, time.Time{}, false, errBroadcastAuditUnavailable
	}

	recipientIDs, err := s.notifRepo.ListActiveRecipientIDsWithContext(ctx)
	if err != nil {
		_ = s.recordBroadcastAudit(ctx, actorID, "failed", map[string]interface{}{
			"title_length":    titleLength,
			"body_length":     bodyLength,
			"recipient_count": 0,
			"error_code":      "RECIPIENT_QUERY_FAILED",
		})
		return 0, time.Time{}, false, errBroadcastSendFailed
	}

	// PostgreSQL stores timestamptz at microsecond precision. Normalize before
	// both persisting and returning so a replay returns the exact first response.
	broadcastAt := time.Now().UTC().Truncate(time.Microsecond)
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

	requestRow := &model.NotificationBroadcastRequest{
		ActorID:        actorID,
		KeyHash:        keyHash,
		PayloadHash:    payloadHash,
		RecipientCount: len(recipientIDs),
		BroadcastAt:    broadcastAt,
	}

	failureCode := "BROADCAST_INSERT_FAILED"
	err = s.notifRepo.Transaction(ctx, func(tx *gorm.DB) error {
		if err := s.notifRepo.CreateBroadcastRequestTx(ctx, tx, requestRow); err != nil {
			failureCode = "BROADCAST_REQUEST_RESERVE_FAILED"
			return err
		}
		if err := s.notifRepo.CreateBroadcastBatchTx(ctx, tx, rows); err != nil {
			failureCode = "BROADCAST_INSERT_FAILED"
			return err
		}
		if err := s.recordBroadcastAuditTx(ctx, tx, actorID, "success", map[string]interface{}{
			"recipient_count": len(recipientIDs),
			"title_length":    titleLength,
			"body_length":     bodyLength,
			"filter":          "active_users",
			"key_fingerprint": keyHash[:12],
			"replayed":        false,
		}); err != nil {
			failureCode = "AUDIT_WRITE_FAILED"
			return err
		}
		return nil
	})
	if err != nil {
		if isBroadcastUniqueViolation(err) {
			return s.replayBroadcastRequest(ctx, actorID, keyHash, payloadHash)
		}
		_ = s.recordBroadcastAudit(ctx, actorID, "failed", map[string]interface{}{
			"title_length":    titleLength,
			"body_length":     bodyLength,
			"recipient_count": len(recipientIDs),
			"error_code":      failureCode,
		})
		return 0, time.Time{}, false, errBroadcastSendFailed
	}

	return len(recipientIDs), broadcastAt, false, nil
}

// replayBroadcastRequest resolves a uniqueness conflict on (actor_id, key_hash).
// On PostgreSQL the conflicting insert waits for the in-flight owner transaction,
// so by the time the violation is visible the committed row is readable. A
// matching payload replays the stored response; a different payload means the
// key was reused for another broadcast.
func (s *NotificationService) replayBroadcastRequest(ctx context.Context, actorID int64, keyHash, payloadHash string) (int, time.Time, bool, error) {
	existing, err := s.notifRepo.GetBroadcastRequestByKeyHash(ctx, actorID, keyHash)
	if err != nil {
		slog.Error("broadcast idempotency replay lookup failed",
			slog.Int64("actor_id", actorID),
			slog.String("error", err.Error()),
		)
		return 0, time.Time{}, false, errBroadcastSendFailed
	}
	if existing.PayloadHash != payloadHash {
		return 0, time.Time{}, false, ErrIdempotencyKeyReused
	}
	return existing.RecipientCount, existing.BroadcastAt, true, nil
}

func hashBroadcastIdempotencyKey(key string) string {
	sum := sha256.Sum256([]byte("omnicraft-broadcast-key:" + key))
	return hex.EncodeToString(sum[:])
}

func hashBroadcastPayload(title, body, channel string) string {
	sum := sha256.Sum256([]byte("omnicraft-broadcast-payload:" + title + "\x1f" + body + "\x1f" + channel))
	return hex.EncodeToString(sum[:])
}

const (
	postgresUniqueViolationSQLState = "23505"
	broadcastRequestKeyConstraint   = "uq_notification_broadcast_requests_actor_key"
)

func isBroadcastUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == postgresUniqueViolationSQLState &&
			pgErr.ConstraintName == broadcastRequestKeyConstraint
	}
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
	}
	return false
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
