package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/service"
)

// #321: Green 未配置是永久配置态而非瞬时故障——worker 对 submit_ai_review
// 应与同步路径同样 A4 本地 fail-open（警告并 ACK），而不是无意义重试进 DLQ。
// 修复前本用例红：Handle 把 ErrGreenNotConfigured 原样上抛触发重试。
func TestReviewWorkerAcknowledgesWhenGreenNotConfigured(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AIReviewRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	reviewSvc := service.NewReviewService(db, nil, nil, service.NewReputationService(db))
	worker := NewReviewWorker(reviewSvc, db)

	payload, _ := json.Marshal(map[string]any{
		"action": "submit_ai_review", "target_type": "content", "target_id": 42,
		"content_type": "article", "title": "t", "author_id": 1,
	})
	if err := worker.Handle(context.Background(), queue.Message{
		ID: "m-321", Topic: "content.review", Payload: payload, Metadata: map[string]string{},
	}); err != nil {
		t.Fatalf("Green not configured must be acknowledged (A4 local fail-open), got error: %v", err)
	}
}
