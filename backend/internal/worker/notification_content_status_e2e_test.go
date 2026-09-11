package worker

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// T11（FIX-17a）worker 端到端：NotifyContentStatus 发出的 notification.create
// 消息经 NotificationWorker 消费后落 notifications 表（内容被 ban → 作者
// 通知落库）。

type t11WorkerCapture struct {
	mu    sync.Mutex
	raw   []byte
	topic string
}

func (p *t11WorkerCapture) Publish(ctx context.Context, topic string, payload []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.topic = topic
	p.raw = payload
	return nil
}

func TestNotificationWorkerPersistsContentStatusBan(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Notification{}, &model.InboxConsumer{}))
	// AutoMigrate does not create the composite unique index the inbox
	// idempotency guard depends on; mirror migration 070 so replays dedupe.
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_inbox_consumers_group_event ON inbox_consumers (consumer_group, event_id)").Error)

	producer := &t11WorkerCapture{}
	notifSvc := service.NewNotificationService(repository.NewNotificationRepository(db))
	notifSvc.SetQueueProducer(producer)
	notifSvc.NotifyContentStatus(42, 100, "worker 端到端内容", "banned", "AI 审核判定违规", 0)

	require.Equal(t, "notification.create", producer.topic)

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
	require.NoError(t, json.Unmarshal(producer.raw, &payload))

	w := NewNotificationWorker(repository.NewNotificationRepository(db), db)
	msg := queue.Message{
		ID:      "t11-e2e-msg-1",
		Topic:   "notification.create",
		Group:   "notification-workers",
		Payload: producer.raw,
	}
	require.NoError(t, w.Handle(context.Background(), msg))

	var notifications []model.Notification
	require.NoError(t, db.Find(&notifications).Error)
	require.Len(t, notifications, 1, "内容被 ban 的作者通知必须落库（票面验收：worker 运行下端到端）")
	saved := notifications[0]
	require.Equal(t, int64(42), saved.UserID)
	require.Equal(t, "system", saved.Channel)
	require.Equal(t, "content_status", saved.Type)
	require.NotNil(t, saved.TargetType)
	require.Equal(t, "content", *saved.TargetType)
	require.NotNil(t, saved.TargetID)
	require.Equal(t, int64(100), *saved.TargetID)
	require.NotNil(t, saved.Body)
	require.Contains(t, *saved.Body, "AI 审核判定违规")

	// 重放（同消息 id）被 inbox 幂等守卫拦下，不产生第二条。
	require.NoError(t, w.Handle(context.Background(), msg))
	require.NoError(t, db.Find(&notifications).Error)
	require.Len(t, notifications, 1, "重复投递不得重复落库")
}
