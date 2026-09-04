package service

import (
	"context"
	"testing"

	"omnicraft/backend/internal/model"
)

// T12（FIX-18）前提②定夺：对讨论的评论统一由 SocialService.PostComment 的
// discussion 分支维护 reply_count 与 last_active_at（驱动讨论列表
// latest_reply 排序）。任何入口（/discussions/:id/comments 或
// /social/comments 带 discussion_id）创建一条评论只递增一次——防双计也防漏计。
func TestPostCommentOnDiscussionIncrementsReplyCount(t *testing.T) {
	db := setupSocialServiceTestDB(t)
	authorID := seedTestSocialUser(t, db)

	svc := newTestSocialService(db, "debug", &fakeTextReviewer{result: "pass"})
	disc, err := svc.PostDiscussion(context.Background(), PostDiscussionInput{Title: "讨论头", Body: "正文"}, authorID)
	if err != nil {
		t.Fatalf("seed discussion: %v", err)
	}

	replier := model.User{Email: "replier@example.com", Username: "replier", Reputation: 10}
	if err := db.Create(&replier).Error; err != nil {
		t.Fatalf("seed replier: %v", err)
	}

	if _, err := svc.PostComment(context.Background(), PostCommentInput{DiscussionID: &disc.ID, Body: "顶一帖"}, replier.ID); err != nil {
		t.Fatalf("PostComment(discussion): %v", err)
	}

	var stored model.Discussion
	if err := db.First(&stored, disc.ID).Error; err != nil {
		t.Fatalf("reload discussion: %v", err)
	}
	if stored.ReplyCount != 1 {
		t.Fatalf("reply_count = %d, want 1 (PostComment must maintain the counter for the discussion path)", stored.ReplyCount)
	}
}
