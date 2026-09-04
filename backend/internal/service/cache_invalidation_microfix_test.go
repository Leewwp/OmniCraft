package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// micro-fix（#347，T10 范围外缺口）：AI 审核路径（applyContentReviewResult
// 的两个调用方）与 IP 级联下架批量内容，状态写入后补缓存失效
// （T10 helper，T41 judge_service 先例=事务提交后失效）。

func TestProcessAICallbackContentBlockInvalidatesContentCache(t *testing.T) {
	svc, db, mr := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)
	cacheKey := fmt.Sprintf("cache:content:%d", contentID)
	require.NoError(t, mr.Set(cacheKey, "cached"))

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType:     "content",
		TargetID:       contentID,
		Result:         "block",
		ProviderTaskID: "microfix-content-block-1",
	}))

	require.Equal(t, "banned", reviewContentStatus(t, db, contentID))
	require.False(t, mr.Exists(cacheKey), "AI 审核封禁后应立即失效内容缓存（applyContentReviewResult 写点）")
}

func TestIPCascadeBatchBanInvalidatesAllContentCaches(t *testing.T) {
	svc, db, mr := setupReviewServiceTest(t)
	ctx := context.Background()

	author := seedReviewUser(t, db)
	ipID := seedReviewIP(t, db)
	ipPtr := &ipID

	contentA := seedReviewContent(t, db, author, ipPtr)
	contentB := seedReviewContent(t, db, author, ipPtr)
	keys := []string{
		fmt.Sprintf("cache:content:%d", contentA),
		fmt.Sprintf("cache:content:%d", contentB),
	}
	for _, key := range keys {
		require.NoError(t, mr.Set(key, "cached"))
	}

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType: "ip", TargetID: ipID, Result: "block", ProviderTaskID: "microfix-ip-block-1",
	}))

	require.Equal(t, "banned", reviewContentStatus(t, db, contentA))
	require.Equal(t, "banned", reviewContentStatus(t, db, contentB))
	for _, key := range keys {
		require.False(t, mr.Exists(key), "IP 级联下架批量内容后应失效全部受影响内容缓存：%s", key)
	}
}
