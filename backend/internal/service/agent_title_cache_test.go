package service

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/internal/pkg/auxcache"
	"omnicraft/backend/internal/pkg/llm"
)

func newTitleTestCache(t *testing.T) *auxcache.Cache {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return auxcache.New("title", "omnicraft:auxcache:title:", time.Minute, client)
}

// SP-24 R6: two conversations with the same opening message generate the
// title once; the second conversation reuses the cached entry.
func TestAutoTitleCacheReusesTitleForSameFirstMessage(t *testing.T) {
	provider := &titleChatProvider{chatText: "站点推荐指南"}
	provider.rounds = [][]llm.ChatDelta{{{Content: "answer"}, {Done: true}}}
	svc, _ := newStreamTestService(t, provider, continuationTestConfig(100000))
	svc.SetTitleCache(newTitleTestCache(t))

	events1 := runChatTurn(t, svc, 7, ChatTurnInput{Message: "推荐一些站点"})
	conv1 := events1[0].ConversationID
	// The first title must be fully stored (its cache Set included) before
	// the second turn starts, or the test would race both LLM calls.
	require.Eventually(t, func() bool {
		conv := conversationByID(t, svc, conv1)
		return conv.Title != nil && *conv.Title == "站点推荐指南"
	}, 2*time.Second, 10*time.Millisecond, "first title must be stored")

	events2 := runChatTurn(t, svc, 8, ChatTurnInput{Message: "推荐一些站点"})
	conv2 := events2[0].ConversationID
	require.Eventually(t, func() bool {
		conv := conversationByID(t, svc, conv2)
		return conv.Title != nil && *conv.Title == "站点推荐指南"
	}, 2*time.Second, 10*time.Millisecond, "second conversation must get the cached title")

	if provider.chatCalls != 1 {
		t.Fatalf("identical opening message must hit the cache: expected 1 title LLM call, got %d", provider.chatCalls)
	}
}

// SP-24 R6: without a cache (the default-off wiring) every conversation
// generates its own title — zero behavior change.
func TestAutoTitleWithoutCacheCallsForEachConversation(t *testing.T) {
	provider := &titleChatProvider{chatText: "站点推荐指南"}
	provider.rounds = [][]llm.ChatDelta{{{Content: "answer"}, {Done: true}}}
	svc, _ := newStreamTestService(t, provider, continuationTestConfig(100000))

	for _, userID := range []int64{7, 8} {
		events := runChatTurn(t, svc, userID, ChatTurnInput{Message: "推荐一些站点"})
		convID := events[0].ConversationID
		require.Eventually(t, func() bool {
			conv := conversationByID(t, svc, convID)
			return conv.Title != nil && *conv.Title == "站点推荐指南"
		}, 2*time.Second, 10*time.Millisecond, "title must be stored")
	}
	if provider.chatCalls != 2 {
		t.Fatalf("no cache wired: expected 2 title LLM calls, got %d", provider.chatCalls)
	}
}

// SP-24 R6: different opening messages never collide on one cache entry.
func TestAutoTitleCacheSeparatesDifferentMessages(t *testing.T) {
	provider := &titleChatProvider{chatText: "通用标题"}
	provider.rounds = [][]llm.ChatDelta{{{Content: "answer"}, {Done: true}}}
	svc, _ := newStreamTestService(t, provider, continuationTestConfig(100000))
	svc.SetTitleCache(newTitleTestCache(t))

	for i, msg := range []string{"推荐一些站点", "帮我起个设定"} {
		events := runChatTurn(t, svc, int64(7+i), ChatTurnInput{Message: msg})
		convID := events[0].ConversationID
		require.Eventually(t, func() bool {
			conv := conversationByID(t, svc, convID)
			return conv.Title != nil && *conv.Title == "通用标题"
		}, 2*time.Second, 10*time.Millisecond, "title must be stored")
	}
	if provider.chatCalls != 2 {
		t.Fatalf("distinct messages must not share cache entries: expected 2 calls, got %d", provider.chatCalls)
	}
}

// SP-24 R6: a disabled cache (enabled=false wiring) behaves like no cache.
func TestAutoTitleDisabledCacheBypasses(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	// ttl 0 = the resilience.aux_cache.title.enabled=false wiring.
	disabled := auxcache.New("title", "omnicraft:auxcache:title:", 0, client)

	provider := &titleChatProvider{chatText: "站点推荐指南"}
	provider.rounds = [][]llm.ChatDelta{{{Content: "answer"}, {Done: true}}}
	svc, _ := newStreamTestService(t, provider, continuationTestConfig(100000))
	svc.SetTitleCache(disabled)

	for _, userID := range []int64{7, 8} {
		events := runChatTurn(t, svc, userID, ChatTurnInput{Message: "推荐一些站点"})
		convID := events[0].ConversationID
		require.Eventually(t, func() bool {
			conv := conversationByID(t, svc, convID)
			return conv.Title != nil && *conv.Title == "站点推荐指南"
		}, 2*time.Second, 10*time.Millisecond, "title must be stored")
	}
	if provider.chatCalls != 2 {
		t.Fatalf("disabled cache must bypass: expected 2 calls, got %d", provider.chatCalls)
	}
}
