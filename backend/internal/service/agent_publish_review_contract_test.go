package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
)

// agentPublishScanner records the text moderation calls so a publish can be
// proven to have entered SubmitForAIReview. It embeds the shared
// fakeGreenScanner so the image/video branches keep their recorded behavior.
type agentPublishScanner struct {
	*fakeGreenScanner
	textCalls []string
}

func (s *agentPublishScanner) TextModeration(ctx context.Context, text string) (*aliyun.GreenScanResult, error) {
	s.textCalls = append(s.textCalls, text)
	if s.textResult != nil {
		return s.textResult, nil
	}
	return &aliyun.GreenScanResult{Result: "pass"}, nil
}

// TestAgentPublishEntersContentReviewChain is the C1 contract test (grilling
// 2026-08-08 decision): the Agent publish flow has no direct persistence path
// — the publish form is submitted through ContentService.PublishContent
// (route POST /api/v1/contents). This test locks that the shared publish
// chain always enters the AI review chain (SubmitForAIReview → Green scan →
// ai_review_records), so a future Agent direct-publish path cannot bypass
// moderation silently. If a bypass is introduced, the review record and the
// scan calls disappear and this contract fails.
func TestAgentPublishEntersContentReviewChain(t *testing.T) {
	svc, _, _, cleanup := newContentCoverReviewService(t)
	defer cleanup()

	scanner := &agentPublishScanner{fakeGreenScanner: &fakeGreenScanner{}}
	svc.reviewSvc.green = scanner

	author := model.User{
		ID:           42,
		Email:        "agent-publish-author@example.com",
		PasswordHash: "hash",
		Username:     "agent-publish-author",
		Role:         "user",
		Reputation:   10,
	}
	require.NoError(t, svc.contentRepo.DB().Create(&author).Error)

	t.Run("pass result publishes and records one aliyun review", func(t *testing.T) {
		content, err := svc.PublishContent(PublishContentInput{
			Title:       "agent-assisted publish",
			Description: "draft finalized with the agent publish surface",
			Zone:        "original",
			Category:    "game",
			ContentType: "article",
			IsPublic:    true,
			AllowCopy:   true,
		}, 42)
		require.NoError(t, err)

		require.Equal(t, "published", contentCoverDBStatus(t, svc, content.ID))
		require.Equal(t, []string{"agent-assisted publish\ndraft finalized with the agent publish surface"}, scanner.textCalls,
			"the publish must enter SubmitForAIReview and scan the submitted text")
		records := reviewRecordsOf(t, svc, content.ID)
		require.Len(t, records, 1, "the publish must record exactly one ai_review_records row")
		require.Equal(t, "aliyun", records[0].Provider)
		require.Equal(t, "pass", records[0].Result)
		require.Equal(t, "content", records[0].TargetType)
		require.Equal(t, content.ID, records[0].TargetID)

		var reloaded model.User
		require.NoError(t, svc.contentRepo.DB().First(&reloaded, 42).Error)
		require.Equal(t, 10, reloaded.Reputation, "a pass result must not penalize the author")
	})

	t.Run("block result bans and penalizes through the same chain", func(t *testing.T) {
		scanner.textResult = &aliyun.GreenScanResult{Result: "block", Reason: "prohibited_content"}

		content, err := svc.PublishContent(PublishContentInput{
			Title:       "violating draft",
			Description: "draft finalized with the agent publish surface",
			Zone:        "original",
			Category:    "game",
			ContentType: "article",
			IsPublic:    true,
			AllowCopy:   true,
		}, 42)
		require.NoError(t, err, "block is applied as a status transition, not a publish error")

		require.Equal(t, "banned", contentCoverDBStatus(t, svc, content.ID))
		records := reviewRecordsOf(t, svc, content.ID)
		require.Len(t, records, 1)
		require.Equal(t, "aliyun", records[0].Provider)
		require.Equal(t, "block", records[0].Result)

		var reloaded model.User
		require.NoError(t, svc.contentRepo.DB().First(&reloaded, 42).Error)
		require.Less(t, reloaded.Reputation, 10, "an agent-assisted publish with a block result must still be penalized")
	})
}

// TestAgentToolsetHasNoDirectPublishPath guards the C1 contract against a
// future Agent direct-publish tool: the registered tool set is read-only
// (search/detail/guide/metadata suggestion). Adding a write/publish tool
// changes this set and breaks the contract, forcing the review chain
// integration to be considered explicitly.
func TestAgentToolsetHasNoDirectPublishPath(t *testing.T) {
	svc := NewAgentService(
		&fakeAgentLLMProvider{},
		nil,
		nil,
		nil,
		nil,
		&config.Config{Agent: config.AgentConfig{WebAgentEnabled: true}},
	)

	require.ElementsMatch(t,
		[]string{ToolSearchContent, ToolGetContentDetail, ToolGetUsageGuide, ToolSuggestPublishMetadata},
		svc.RegisteredToolNames(),
		"the Agent tool set must stay read-only; content may only be published through ContentService")

	names := make(map[string]bool)
	for _, def := range svc.ToolDefinitions() {
		names[def.Name] = true
	}
	for _, want := range []string{ToolSearchContent, ToolGetContentDetail, ToolGetUsageGuide, ToolSuggestPublishMetadata} {
		require.True(t, names[want], "ToolDefinitions must advertise %q", want)
	}
	require.Equal(t, 4, len(names), "no additional (write) tools may be advertised")
}

func reviewRecordsOf(t *testing.T, svc *ContentService, contentID int64) []model.AIReviewRecord {
	t.Helper()
	var records []model.AIReviewRecord
	require.NoError(t, svc.contentRepo.DB().
		Where("target_type = ? AND target_id = ?", "content", contentID).
		Find(&records).Error)
	return records
}
