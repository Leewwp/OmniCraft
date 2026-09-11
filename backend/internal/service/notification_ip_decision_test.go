package service

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
	"omnicraft/backend/internal/repository"
)

// T16（FIX-24）：IP 决策通知走 T11 的 Notify 契约模式——channel=system、
// type=ip_status、target=ip、reject body 必须携带 admin 填写的原因；approve
// 通知供 T52（我的 IP 状态可知性）消费校验。未知 decision 不产生通知。

type t16CaptureProducer struct {
	mu        sync.Mutex
	publishes []map[string]interface{}
}

func (p *t16CaptureProducer) Publish(ctx context.Context, topic string, payload []byte) error {
	var decoded map[string]interface{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publishes = append(p.publishes, decoded)
	return nil
}

func (p *t16CaptureProducer) ipDecisionNotifies() []map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]map[string]interface{}, 0)
	for _, pub := range p.publishes {
		if pub["type"] == "ip_status" {
			out = append(out, pub)
		}
	}
	return out
}

func TestNotifyIPDecisionContract(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Notification{}))

	producer := &t16CaptureProducer{}
	svc := NewNotificationService(repository.NewNotificationRepository(db))
	svc.SetQueueProducer(producer)

	svc.NotifyIPDecision(42, 100, "示例 IP", "rejected", "名称与介绍不符", 7)
	svc.NotifyIPDecision(42, 100, "示例 IP", "approved", "", 7)
	svc.NotifyIPDecision(42, 100, "示例 IP", "deleted", "", 7) // 未知 decision：不通知

	notifies := producer.ipDecisionNotifies()
	require.Len(t, notifies, 2, "rejected/approved 各一条，未知 decision 不产生通知")

	for _, n := range notifies {
		require.Equal(t, "system", n["channel"])
		require.Equal(t, "ip_status", n["type"])
		require.Equal(t, "ip", n["target_type"])
		require.EqualValues(t, 100, n["target_id"])
		require.EqualValues(t, 42, n["user_id"])
	}

	require.Contains(t, notifies[0]["body"], "原因", "reject 通知 body 必须携带原因（FIX-24）")
	require.Contains(t, notifies[0]["body"], "名称与介绍不符")
	require.Contains(t, notifies[0]["body"], "示例 IP", "通知 body 应指明是哪个 IP")
	require.EqualValues(t, 7, notifies[0]["sender_id"], "admin 动作的通知应携带操作者 senderID")
	require.Contains(t, notifies[1]["body"], "示例 IP")
}
