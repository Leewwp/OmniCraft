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
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
)

// T11（FIX-17a）：NotifyContentStatus helper 契约（channel=system、
// type=content_status、target=content、body 带 reason）+ AI 审核结果挂接 +
// IP 级联下架按受影响作者去重各一条（防通知风暴）。

type t11CapturedPublish struct {
	Topic   string
	Payload map[string]interface{}
}

type t11CaptureProducer struct {
	mu        sync.Mutex
	publishes []t11CapturedPublish
}

func (p *t11CaptureProducer) Publish(ctx context.Context, topic string, payload []byte) error {
	var decoded map[string]interface{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publishes = append(p.publishes, t11CapturedPublish{Topic: topic, Payload: decoded})
	return nil
}

func (p *t11CaptureProducer) contentStatusNotifies() []map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]map[string]interface{}, 0)
	for _, pub := range p.publishes {
		if pub.Topic == "notification.create" && pub.Payload["type"] == "content_status" {
			out = append(out, pub.Payload)
		}
	}
	return out
}

func newT11NotificationService(db *gorm.DB, producer queue.Producer) *NotificationService {
	svc := NewNotificationService(repository.NewNotificationRepository(db))
	svc.SetQueueProducer(producer)
	return svc
}

func TestNotifyContentStatusContract(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Notification{}))

	producer := &t11CaptureProducer{}
	svc := newT11NotificationService(db, producer)

	svc.NotifyContentStatus(42, 100, "测试内容", "banned", "违规内容", 7)
	svc.NotifyContentStatus(42, 100, "测试内容", "under_review", "举报达到阈值", 0)
	svc.NotifyContentStatus(42, 100, "测试内容", "published", "", 0)
	svc.NotifyContentStatus(42, 100, "测试内容", "draft", "", 0) // 未知状态：不通知

	notifies := producer.contentStatusNotifies()
	require.Len(t, notifies, 3, "banned/under_review/published 三态各一条，draft 等未知状态不产生通知")

	for _, n := range notifies {
		require.Equal(t, "system", n["channel"])
		require.Equal(t, "content_status", n["type"])
		require.Equal(t, "content", n["target_type"])
		require.EqualValues(t, 100, n["target_id"])
		require.EqualValues(t, 42, n["user_id"])
	}

	require.Contains(t, notifies[0]["body"], "原因", "banned 通知 body 必须携带 reason（FIX-17 契约）")
	require.Contains(t, notifies[0]["body"], "违规内容")
	require.Contains(t, notifies[0]["body"], "测试内容", "通知 body 应指明是哪篇内容")
	require.Contains(t, notifies[1]["body"], "举报达到阈值")
	require.Contains(t, notifies[2]["body"], "测试内容")
	require.EqualValues(t, 7, notifies[0]["sender_id"], "admin 动作的通知应携带操作者 senderID")
}

func TestApplyContentReviewResultNotifiesAuthor(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	producer := &t11CaptureProducer{}
	svc.SetNotificationService(newT11NotificationService(db, producer))

	author := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, author)

	// pass：pending → published
	require.NoError(t, svc.ProcessAICallback(context.Background(), AICallbackInput{
		TargetType: "content", TargetID: contentID, Result: "pass", ProviderTaskID: "t11-pass-1",
	}))
	notifies := producer.contentStatusNotifies()
	require.Len(t, notifies, 1, "AI 审核通过（发布）应通知作者")
	require.Contains(t, notifies[0]["body"], "已通过审核")
	require.EqualValues(t, author, notifies[0]["user_id"])

	// review：published → under_review
	require.NoError(t, svc.ProcessAICallback(context.Background(), AICallbackInput{
		TargetType: "content", TargetID: contentID, Result: "review", ProviderTaskID: "t11-review-1",
	}))
	notifies = producer.contentStatusNotifies()
	require.Len(t, notifies, 2, "AI 转人工复核应通知作者")
	require.Contains(t, notifies[1]["body"], "人工复核")

	// block：under_review → banned
	require.NoError(t, svc.ProcessAICallback(context.Background(), AICallbackInput{
		TargetType: "content", TargetID: contentID, Result: "block", ProviderTaskID: "t11-block-1",
	}))
	notifies = producer.contentStatusNotifies()
	require.Len(t, notifies, 3, "AI 封禁应通知作者")
	require.Contains(t, notifies[2]["body"], "封禁")

	// 幂等：已 banned 再收 block（新 taskID）——不重复处罚也不重复通知
	require.NoError(t, svc.ProcessAICallback(context.Background(), AICallbackInput{
		TargetType: "content", TargetID: contentID, Result: "block", ProviderTaskID: "t11-block-2",
	}))
	require.Len(t, producer.contentStatusNotifies(), 3, "已终态 banned 的重复 block 不应再发通知")
}

func TestProcessIPReviewResultCascadeNotifiesOncePerAuthor(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	producer := &t11CaptureProducer{}
	svc.SetNotificationService(newT11NotificationService(db, producer))

	authorA := seedReviewUser(t, db)
	authorB := seedReviewUser(t, db)
	ipID := seedReviewIP(t, db)
	ipPtr := &ipID

	contentA1 := seedReviewContent(t, db, authorA, ipPtr)
	contentA2 := seedReviewContent(t, db, authorA, ipPtr)
	contentB1 := seedReviewContent(t, db, authorB, ipPtr)
	_ = contentA2

	require.NoError(t, svc.ProcessAICallback(context.Background(), AICallbackInput{
		TargetType: "ip", TargetID: ipID, Result: "block", ProviderTaskID: "t11-ip-block-1",
	}))

	var status string
	require.NoError(t, db.Raw("SELECT status FROM ips WHERE id = ?", ipID).Scan(&status).Error)
	require.Equal(t, "banned", status)
	for _, id := range []int64{contentA1, contentA2, contentB1} {
		require.NoError(t, db.Raw("SELECT status FROM content_items WHERE id = ?", id).Scan(&status).Error)
		require.Equal(t, "banned", status)
	}

	notifies := producer.contentStatusNotifies()
	require.Len(t, notifies, 2, "IP 级联下架按受影响作者去重各一条（防通知风暴）：2 位作者 = 2 条通知")

	byAuthor := map[int64]map[string]interface{}{}
	for _, n := range notifies {
		byAuthor[int64(n["user_id"].(float64))] = n
	}
	require.Len(t, byAuthor, 2)

	notifyA := byAuthor[authorA]
	require.Contains(t, notifyA["body"], "2 篇", "多内容作者的通知应说明下架篇数")
	require.Contains(t, notifyA["body"], "IP")
	require.Equal(t, "content", notifyA["target_type"], "级联通知 target 仍为 content（代表该作者第一条受影响内容）")
}

func TestNotifyContentStatusWithoutNotificationServiceIsNoop(t *testing.T) {
	// ReviewService 未注入 notifSvc（如未装配的测试/本地路径）时静默跳过，不 panic。
	svc, db, _ := setupReviewServiceTest(t)
	author := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, author)
	require.NotPanics(t, func() {
		require.NoError(t, svc.ProcessAICallback(context.Background(), AICallbackInput{
			TargetType: "content", TargetID: contentID, Result: "pass", ProviderTaskID: "t11-noop-1",
		}))
	})
}
