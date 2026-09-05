package worker

// #379 follow-up: system-side notifications carry sender_id=0 (judge verdicts,
// AI moderation, report thresholds — NotifyContentStatus's system caller
// convention). sender_id=0 violates the users FK on Postgres (sender_id is
// nullable, but 0 is not a user id), so the worker must persist NULL for the
// "no sender" case. The sqlite-based T11 e2e cannot catch this: sqlite does
// not enforce the FK, which is exactly why this regression reached production
// paths unnoticed until the judge closed-case smoke exercised it live.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
)

func newSenderFKTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Notification{}, &model.InboxConsumer{}))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_inbox_consumers_group_event ON inbox_consumers (consumer_group, event_id)").Error)
	return db
}

func notificationPayload(t *testing.T, senderID int64) queue.Message {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"user_id": 42, "channel": "system", "type": "content_status",
		"title": "内容已通过审核", "body": "系统通知正文",
		"target_type": "content", "target_id": 100, "sender_id": senderID,
	})
	require.NoError(t, err)
	return queue.Message{ID: "sender-fk-msg", Topic: "notification.create", Group: "notification-workers", Payload: payload}
}

func TestNotificationWorkerPersistsZeroSenderAsNull(t *testing.T) {
	db := newSenderFKTestDB(t)
	w := NewNotificationWorker(repository.NewNotificationRepository(db), db)

	require.NoError(t, w.Handle(context.Background(), notificationPayload(t, 0)))

	var saved model.Notification
	require.NoError(t, db.First(&saved).Error)
	require.Nil(t, saved.SenderID, "system notification (sender_id=0) must persist NULL, not 0 — 0 violates notifications_sender_id_fkey on Postgres")
}

func TestNotificationWorkerKeepsRealSender(t *testing.T) {
	db := newSenderFKTestDB(t)
	w := NewNotificationWorker(repository.NewNotificationRepository(db), db)

	require.NoError(t, w.Handle(context.Background(), notificationPayload(t, 313)))

	var saved model.Notification
	require.NoError(t, db.First(&saved).Error)
	require.NotNil(t, saved.SenderID)
	require.Equal(t, int64(313), *saved.SenderID, "real sender ids must be preserved untouched")
}
