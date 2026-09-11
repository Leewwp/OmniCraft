package service

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

// T55（FIX-31b）：通知细节修复包——点踩不再通知作者；反馈 resolved 也通知。

func TestReactionDislikeDoesNotNotifyAuthor(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.Reaction{}))

	svc := NewSocialServiceWithRedis(
		repository.NewSocialRepository(db),
		repository.NewContentRepository(db),
		repository.NewUserRepository(db),
		&config.Config{Server: config.ServerConfig{Mode: "dev"}},
		nil,
		nil,
	)
	producer := &t11CaptureProducer{}
	notifSvc := NewNotificationService(repository.NewNotificationRepository(db))
	notifSvc.SetQueueProducer(producer)
	svc.SetNotificationService(notifSvc)

	author := model.User{Email: "author@example.test", Username: "author", Reputation: 10}
	disliker := model.User{Email: "disliker@example.test", Username: "disliker", Reputation: 10}
	liker := model.User{Email: "liker@example.test", Username: "liker", Reputation: 10}
	require.NoError(t, db.Create(&author).Error)
	require.NoError(t, db.Create(&disliker).Error)
	require.NoError(t, db.Create(&liker).Error)
	content := model.ContentItem{AuthorID: author.ID, Title: "T55 点踩目标", Zone: "original", ContentType: "image", Status: "published"}
	require.NoError(t, db.Create(&content).Error)

	// 他人点踩：不得通知作者（减少负面轰炸）
	_, err = svc.React(ReactInput{TargetType: "content", TargetID: content.ID, Reaction: "dislike"}, disliker.ID)
	require.NoError(t, err)
	for _, p := range producer.publishes {
		if p.Topic == "notification.create" {
			t.Fatalf("点踩不应产生任何通知: %v", p.Payload)
		}
	}

	// 他人点赞：仍通知作者（回归）
	_, err = svc.React(ReactInput{TargetType: "content", TargetID: content.ID, Reaction: "like"}, liker.ID)
	require.NoError(t, err)
	liked := false
	for _, p := range producer.publishes {
		if p.Topic == "notification.create" && p.Payload["type"] == "like" {
			liked = true
		}
	}
	require.True(t, liked, "点赞仍应通知作者")
}

func TestFeedbackPatchTicketResolvedNotifiesUser(t *testing.T) {
	svc, db, _ := setupFeedbackServiceTest(t)

	owner := model.User{Email: "resolved-owner@example.test", Username: "resolvedowner", Reputation: 10}
	require.NoError(t, db.Create(&owner).Error)
	ticket := model.FeedbackTicket{
		UserID:      ptrInt64(owner.ID),
		Category:    "web_bug",
		Title:       "T55 resolved 通知",
		Description: "desc",
		Status:      "in_progress",
		Priority:    "normal",
	}
	require.NoError(t, db.Create(&ticket).Error)

	_, err := svc.PatchTicket(context.Background(), ticket.ID, AdminPatchFeedbackInput{Status: "resolved"})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		var count int64
		err := db.Model(&model.Notification{}).
			Where("user_id = ? AND type = ? AND target_type = ? AND target_id = ?", owner.ID, "system", "feedback_ticket", ticket.ID).
			Count(&count).Error
		return err == nil && count == 1
	}, 500*time.Millisecond, 10*time.Millisecond, "反馈 resolved 也应通知提交者")
}
