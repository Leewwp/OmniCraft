package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
)

// T43（FIX-13）：审核体系不被编辑绕过——banned 内容 PATCH 拒绝（终态禁改，
// 删除仍允许）；published 内容的 title/cover 变更走增量复审（block → banned
// +扣分链路与 AI 通道一致；pass → 保持 published 但落 ai_review_records 编辑
// 审计；Green 未配置本地 fail-open 仍落记录）；封面 URL 必须是平台 OSS 域/键。

func seedT43Content(t *testing.T, svc *ContentService, authorID int64, status string) int64 {
	t.Helper()
	content := model.ContentItem{
		Title:       "T43 original title",
		AuthorID:    authorID,
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		Status:      status,
		IsPublic:    true,
	}
	require.NoError(t, svc.contentRepo.DB().Create(&content).Error)
	return content.ID
}

func t43AIReviewCount(t *testing.T, svc *ContentService, contentID int64) int64 {
	t.Helper()
	var count int64
	require.NoError(t, svc.contentRepo.DB().Model(&model.AIReviewRecord{}).
		Where("target_type = ? AND target_id = ?", "content", contentID).Count(&count).Error)
	return count
}

func TestUpdateBannedContentIsForbidden(t *testing.T) {
	svc, _, _, cleanup := newContentCoverReviewService(t)
	defer cleanup()
	contentID := seedT43Content(t, svc, 42, "banned")

	err := svc.UpdateContentWithContext(context.Background(), contentID, 42, map[string]interface{}{"title": "new title"})
	require.ErrorIs(t, err, ErrContentBanned, "banned 终态禁改（FIX-13）——编辑被拒")
}

func TestPublishedTitleEditUnderGoesReReviewPass(t *testing.T) {
	svc, _, green, cleanup := newContentCoverReviewService(t)
	defer cleanup()
	green.textResult = &aliyun.GreenScanResult{Result: "pass"}
	contentID := seedT43Content(t, svc, 42, "published")

	require.NoError(t, svc.UpdateContentWithContext(context.Background(), contentID, 42, map[string]interface{}{"title": "T43 edited title"}))

	var content model.ContentItem
	require.NoError(t, svc.contentRepo.DB().First(&content, contentID).Error)
	require.Equal(t, "T43 edited title", content.Title, "pass 后新标题生效")
	require.Equal(t, "published", content.Status, "pass 保持 published")
	require.Equal(t, int64(1), t43AIReviewCount(t, svc, contentID), "编辑复审必须落 ai_review_records（编辑审计）")
}

func TestPublishedTitleEditUnderGoesReReviewBlock(t *testing.T) {
	svc, _, green, cleanup := newContentCoverReviewService(t)
	defer cleanup()
	green.textResult = &aliyun.GreenScanResult{Result: "block"}
	contentID := seedT43Content(t, svc, 42, "published")

	require.NoError(t, svc.UpdateContentWithContext(context.Background(), contentID, 42, map[string]interface{}{"title": "T43 violating title"}))

	var content model.ContentItem
	require.NoError(t, svc.contentRepo.DB().First(&content, contentID).Error)
	require.Equal(t, "banned", content.Status, "block → banned（与 AI 通道一致）")
	require.Equal(t, "T43 original title", content.Title, "被拒的新标题不得生效")
	require.Equal(t, int64(1), t43AIReviewCount(t, svc, contentID))

	var aiViolationPenalties int64
	require.NoError(t, svc.contentRepo.DB().Model(&model.ReputationLog{}).
		Where("user_id = ? AND reason = ?", 42, "ai_violation").Count(&aiViolationPenalties).Error)
	require.Equal(t, int64(1), aiViolationPenalties, "block 扣分链路与 AI 通道一致（ai_violation -3，repeat penalty 另计）")
}

func TestPublishedCoverEditScannedAndNonPlatformRejected(t *testing.T) {
	svc, _, _, cleanup := newContentCoverReviewService(t)
	defer cleanup()
	contentID := seedT43Content(t, svc, 42, "published")

	// 外链封面拒绝（防外链投毒）。
	err := svc.UpdateContentWithContext(context.Background(), contentID, 42, map[string]interface{}{"cover_image_url": "https://evil.example.com/x.png"})
	require.ErrorIs(t, err, ErrCoverNotPlatformOSSObject)

	// 平台域封面通过校验进入图片复审。
	require.NoError(t, svc.UpdateContentWithContext(context.Background(), contentID, 42, map[string]interface{}{"cover_image_url": "https://cdn.example.test/uploads/42/image/cover.png"}))

	var content model.ContentItem
	require.NoError(t, svc.contentRepo.DB().First(&content, contentID).Error)
	require.Equal(t, "published", content.Status)
	require.Equal(t, int64(1), t43AIReviewCount(t, svc, contentID))
}

func TestPublishedEditFailOpenRecordsAndSucceeds(t *testing.T) {
	svc, _, _, cleanup := newContentCoverReviewService(t)
	defer cleanup()
	svc.reviewSvc.green = nil // 本地无 Green 配置（A4 fail-open）
	contentID := seedT43Content(t, svc, 42, "published")

	require.NoError(t, svc.UpdateContentWithContext(context.Background(), contentID, 42, map[string]interface{}{"title": "T43 fail-open title"}))

	var content model.ContentItem
	require.NoError(t, svc.contentRepo.DB().First(&content, contentID).Error)
	require.Equal(t, "T43 fail-open title", content.Title, "fail-open：本地编辑成功")
	require.Equal(t, "published", content.Status)
	require.Equal(t, int64(1), t43AIReviewCount(t, svc, contentID), "fail-open 仍落编辑审计记录")
}

func TestDraftTitleEditSkipsReReview(t *testing.T) {
	svc, _, green, cleanup := newContentCoverReviewService(t)
	defer cleanup()
	green.textResult = &aliyun.GreenScanResult{Result: "block"} // 即使 Green 会 block
	contentID := seedT43Content(t, svc, 42, "draft")

	require.NoError(t, svc.UpdateContentWithContext(context.Background(), contentID, 42, map[string]interface{}{"title": "T43 draft title"}))

	var content model.ContentItem
	require.NoError(t, svc.contentRepo.DB().First(&content, contentID).Error)
	require.Equal(t, "T43 draft title", content.Title)
	require.Equal(t, "draft", content.Status, "非 published 编辑不触发增量复审（发布时再审）")
	require.Equal(t, int64(0), t43AIReviewCount(t, svc, contentID))
}
