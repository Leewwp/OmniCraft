package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

func TestBroadcastSystemNotificationCreatesOneNotificationPerActiveUser(t *testing.T) {
	svc, db := setupBroadcastNotificationServiceTest(t)
	activeOne := createBroadcastUser(t, db, "active-one", false, nil)
	activeTwo := createBroadcastUser(t, db, "active-two", false, nil)

	recipientCount, broadcastAt, err := svc.BroadcastSystemNotification(
		context.Background(),
		"  Maintenance notice  ",
		"  We will be offline briefly.  ",
		"",
		9001,
	)

	require.NoError(t, err)
	require.Equal(t, 2, recipientCount)
	require.False(t, broadcastAt.IsZero())

	var notifications []model.Notification
	require.NoError(t, db.Order("user_id ASC").Find(&notifications).Error)
	require.Len(t, notifications, 2)
	require.Equal(t, []int64{activeOne.ID, activeTwo.ID}, []int64{notifications[0].UserID, notifications[1].UserID})
	for _, notification := range notifications {
		require.Equal(t, "system", notification.Type)
		require.Equal(t, "broadcast", notification.Channel)
		require.NotNil(t, notification.Title)
		require.NotNil(t, notification.Body)
		require.Equal(t, "Maintenance notice", *notification.Title)
		require.Equal(t, "We will be offline briefly.", *notification.Body)
	}

	log := latestBroadcastAuditLog(t, db)
	require.Equal(t, "success", log.Result)
	require.Equal(t, "users", log.TargetType)
	require.Equal(t, float64(2), log.Metadata["recipient_count"])
	require.Equal(t, "active_users", log.Metadata["filter"])
}

func TestBroadcastSystemNotificationSkipsBannedAndDeletedUsers(t *testing.T) {
	svc, db := setupBroadcastNotificationServiceTest(t)
	active := createBroadcastUser(t, db, "active", false, nil)
	createBroadcastUser(t, db, "banned", true, nil)
	deletedAt := time.Now().UTC()
	createBroadcastUser(t, db, "deleted", false, &deletedAt)

	recipientCount, _, err := svc.BroadcastSystemNotification(
		context.Background(),
		"Status update",
		"Only active users should receive this.",
		"broadcast",
		9001,
	)

	require.NoError(t, err)
	require.Equal(t, 1, recipientCount)

	var notifications []model.Notification
	require.NoError(t, db.Find(&notifications).Error)
	require.Len(t, notifications, 1)
	require.Equal(t, active.ID, notifications[0].UserID)
}

func TestBroadcastSystemNotificationUsesBatchSize500(t *testing.T) {
	svc, db := setupBroadcastNotificationServiceTest(t)
	for i := 0; i < 501; i++ {
		createBroadcastUser(t, db, fmt.Sprintf("recipient-%03d", i), false, nil)
	}

	var batchSizes []int
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("capture_notification_batch_size", func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "Notification" {
			return
		}
		batchSizes = append(batchSizes, tx.Statement.ReflectValue.Len())
	}))

	recipientCount, _, err := svc.BroadcastSystemNotification(
		context.Background(),
		"Batch notice",
		"Batch size should be fixed at 500.",
		"broadcast",
		9001,
	)

	require.NoError(t, err)
	require.Equal(t, 501, recipientCount)
	require.Equal(t, []int{500, 1}, batchSizes)
}

func TestBroadcastSystemNotificationAuditMetadataDoesNotStoreFullBody(t *testing.T) {
	svc, db := setupBroadcastNotificationServiceTest(t)
	createBroadcastUser(t, db, "active", false, nil)

	fullBody := "Do not store this **Markdown** body or its https://example.com/private link."
	recipientCount, broadcastAt, err := svc.BroadcastSystemNotification(
		context.Background(),
		"   ",
		fullBody,
		"broadcast",
		9001,
	)

	require.Error(t, err)
	var validationErr *BroadcastValidationError
	require.True(t, errors.As(err, &validationErr), "error = %v", err)
	require.Equal(t, 0, recipientCount)
	require.True(t, broadcastAt.IsZero())

	var notificationCount int64
	require.NoError(t, db.Model(&model.Notification{}).Count(&notificationCount).Error)
	require.Equal(t, int64(0), notificationCount)

	log := latestBroadcastAuditLog(t, db)
	require.Equal(t, "rejected", log.Result)
	assertBroadcastAuditMetadataDoesNotContainUnsafeFields(t, log.Metadata, fullBody)
	require.Equal(t, "VALIDATION_ERROR", log.Metadata["validation_error_code"])
	require.Equal(t, float64(0), log.Metadata["title_length"])
	require.Equal(t, float64(len([]rune(fullBody))), log.Metadata["body_length"])
}

func TestBroadcastSystemNotificationSuccessAuditFailureRollsBackNotifications(t *testing.T) {
	svc, db := setupBroadcastNotificationServiceTest(t)
	createBroadcastUser(t, db, "active-one", false, nil)
	createBroadcastUser(t, db, "active-two", false, nil)
	installBroadcastAuditFailureTrigger(t, db, "success")

	recipientCount, broadcastAt, err := svc.BroadcastSystemNotification(
		context.Background(),
		"Audit failure",
		"Success audit failure must not leave recipient notifications behind.",
		"broadcast",
		9001,
	)

	require.Error(t, err)
	require.Equal(t, 0, recipientCount)
	require.True(t, broadcastAt.IsZero())
	require.Equal(t, int64(0), countBroadcastNotifications(t, db))
}

func TestBroadcastSystemNotificationValidationErrorSurvivesRejectedAuditFailure(t *testing.T) {
	svc, db := setupBroadcastNotificationServiceTest(t)
	createBroadcastUser(t, db, "active", false, nil)
	installBroadcastAuditFailureTrigger(t, db, "rejected")

	recipientCount, broadcastAt, err := svc.BroadcastSystemNotification(
		context.Background(),
		" ",
		"Body is valid but title is not.",
		"broadcast",
		9001,
	)

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrBroadcastValidation), "error = %v", err)
	require.Equal(t, 0, recipientCount)
	require.True(t, broadcastAt.IsZero())
	require.Equal(t, int64(0), countBroadcastNotifications(t, db))
}

func TestBroadcastSystemNotificationInsertFailureRollsBackFirstBatchAndWritesFailedAudit(t *testing.T) {
	svc, db := setupBroadcastNotificationServiceTestWithGormConfig(t, &gorm.Config{SkipDefaultTransaction: true})
	var failingRecipient model.User
	for i := 0; i < 501; i++ {
		user := createBroadcastUser(t, db, fmt.Sprintf("recipient-%03d", i), false, nil)
		if i == 500 {
			failingRecipient = user
		}
	}
	installBroadcastNotificationFailureTrigger(t, db, failingRecipient.ID)

	recipientCount, broadcastAt, err := svc.BroadcastSystemNotification(
		context.Background(),
		"Second batch failure",
		"The failing row is in the second batch.",
		"broadcast",
		9001,
	)

	require.Error(t, err)
	require.Equal(t, 0, recipientCount)
	require.True(t, broadcastAt.IsZero())
	require.Equal(t, int64(0), countBroadcastNotifications(t, db))

	log := latestBroadcastAuditLog(t, db)
	require.Equal(t, "failed", log.Result)
	require.Equal(t, "BROADCAST_INSERT_FAILED", log.Metadata["error_code"])
	require.Equal(t, float64(501), log.Metadata["recipient_count"])
}

func TestBroadcastNotificationAuditAllowlistDropsUnexpectedBodyField(t *testing.T) {
	filtered := filterMetadata("broadcast_notification", map[string]interface{}{
		"recipient_count":       2,
		"title_length":          12,
		"body_length":           80,
		"filter":                "active_users",
		"validation_error_code": "VALIDATION_ERROR",
		"validation_fields":     []string{"title"},
		"error_code":            "BROADCAST_SEND_FAILED",
		"body":                  "full body must not be stored",
		"title":                 "full title must not be stored",
		"markdown":              "**markdown**",
		"html":                  "<strong>html</strong>",
		"recipients":            []int64{1, 2},
	})

	require.Equal(t, 7, len(filtered))
	require.Equal(t, 2, filtered["recipient_count"])
	require.Equal(t, 12, filtered["title_length"])
	require.Equal(t, 80, filtered["body_length"])
	require.Equal(t, "active_users", filtered["filter"])
	require.Equal(t, "VALIDATION_ERROR", filtered["validation_error_code"])
	require.Equal(t, []string{"title"}, filtered["validation_fields"])
	require.Equal(t, "BROADCAST_SEND_FAILED", filtered["error_code"])
	assertBroadcastAuditMetadataDoesNotContainUnsafeFields(t, filtered, "full body must not be stored")
}

func setupBroadcastNotificationServiceTest(t *testing.T) (*NotificationService, *gorm.DB) {
	t.Helper()

	return setupBroadcastNotificationServiceTestWithGormConfig(t, &gorm.Config{})
}

func setupBroadcastNotificationServiceTestWithGormConfig(t *testing.T, cfg *gorm.Config) (*NotificationService, *gorm.DB) {
	t.Helper()
	silenceBroadcastServiceTestLogs(t)

	if cfg.Logger == nil {
		cfg.Logger = logger.Default.LogMode(logger.Silent)
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), cfg)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Notification{}, &model.AdminAuditLog{}))

	notificationRepo := repository.NewNotificationRepository(db)
	auditSvc := NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	svc := NewNotificationService(notificationRepo)
	svc.SetAdminAuditService(auditSvc)

	return svc, db
}

func silenceBroadcastServiceTestLogs(t *testing.T) {
	t.Helper()

	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})
}

func createBroadcastUser(t *testing.T, db *gorm.DB, username string, banned bool, deletedAt *time.Time) model.User {
	t.Helper()

	user := model.User{
		Email:        username + "@example.com",
		Username:     username,
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
		IsBanned:     banned,
		DeletedAt:    deletedAt,
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func latestBroadcastAuditLog(t *testing.T, db *gorm.DB) model.AdminAuditLog {
	t.Helper()

	var log model.AdminAuditLog
	require.NoError(t, db.Where("action = ?", "broadcast_notification").Order("id DESC").First(&log).Error)
	return log
}

func countBroadcastNotifications(t *testing.T, db *gorm.DB) int64 {
	t.Helper()

	var count int64
	require.NoError(t, db.Model(&model.Notification{}).Where("channel = ?", "broadcast").Count(&count).Error)
	return count
}

func installBroadcastAuditFailureTrigger(t *testing.T, db *gorm.DB, result string) {
	t.Helper()

	require.NoError(t, db.Exec(fmt.Sprintf(`
		CREATE TRIGGER fail_broadcast_%s_audit
		BEFORE INSERT ON admin_audit_logs
		WHEN NEW.action = 'broadcast_notification' AND NEW.result = '%s'
		BEGIN
			SELECT RAISE(ABORT, 'forced broadcast audit failure');
		END;
	`, result, result)).Error)
}

func installBroadcastNotificationFailureTrigger(t *testing.T, db *gorm.DB, userID int64) {
	t.Helper()

	require.NoError(t, db.Exec(fmt.Sprintf(`
		CREATE TRIGGER fail_broadcast_notification_insert
		BEFORE INSERT ON notifications
		WHEN NEW.channel = 'broadcast' AND NEW.user_id = %d
		BEGIN
			SELECT RAISE(ABORT, 'forced broadcast insert failure');
		END;
	`, userID)).Error)
}

func assertBroadcastAuditMetadataDoesNotContainUnsafeFields(t *testing.T, metadata model.JSONMap, fullBody string) {
	t.Helper()

	for _, key := range []string{"body", "title", "markdown", "html", "recipients"} {
		require.NotContains(t, metadata, key)
	}
	for key, value := range metadata {
		require.NotEqualf(t, fullBody, value, "metadata key %q stores the full body", key)
	}
}
