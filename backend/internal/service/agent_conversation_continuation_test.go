package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
)

// titleChatProvider additionally fakes the non-streaming Chat() call the
// auto-title generator uses: chatText/chatErr shape its outcome.
type titleChatProvider struct {
	streamToolProvider
	chatText    string
	chatErr     error
	chatCalls   int
	lastChatReq llm.ChatRequest
}

func (p *titleChatProvider) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.chatCalls++
	p.lastChatReq = req
	if p.chatErr != nil {
		return nil, p.chatErr
	}
	return &llm.ChatResponse{Content: p.chatText}, nil
}

func continuationTestConfig(budget int) *config.Config {
	return &config.Config{Agent: config.AgentConfig{
		WebAgentEnabled:        true,
		MaxToolCallsPerTurn:    8,
		CitationMaxCount:       5,
		MaxUserMessageChars:    4000,
		ChatMaxContextMsgs:     10,
		MaxOutputTokens:        1200,
		ChatContextTokenBudget: budget,
	}}
}

func runChatTurn(t *testing.T, svc *AgentService, userID int64, turn ChatTurnInput) []AgentStreamEvent {
	t.Helper()
	var events []AgentStreamEvent
	err := svc.ChatStream(context.Background(), userID, turn, resolveGlobalChatContext(t, svc, userID), func(ev AgentStreamEvent) error {
		events = append(events, ev)
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, events)
	return events
}

func conversationByID(t *testing.T, svc *AgentService, id int64) model.AgentConversation {
	t.Helper()
	var conv model.AgentConversation
	require.NoError(t, svc.db.First(&conv, id).Error)
	return conv
}

// A-01: the first turn omits conversation_id; the server creates the
// conversation and returns its id in the start event. The second turn
// continues the same conversation and the provider request is assembled
// server-side from the stored history — the client uploads only the new
// message.
func TestChatStreamContinuesConversationWithServerAssembledHistory(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("get_content_detail", `{"content_id":88}`)},
		{{Content: "answer one"}, {Done: true}},
		{toolCallDelta("get_content_detail", `{"content_id":88}`)},
		{{Content: "answer two"}, {Done: true}},
	}}
	svc, db := newStreamTestService(t, provider, continuationTestConfig(100000))

	events1 := runChatTurn(t, svc, 7, ChatTurnInput{Message: "first question"})
	require.Equal(t, AgentEventStart, events1[0].Type)
	convID := events1[0].ConversationID
	require.NotZero(t, convID, "start event must carry the server-created conversation id")

	provider.lastReq = llm.ChatRequest{}
	events2 := runChatTurn(t, svc, 7, ChatTurnInput{ConversationID: convID, Message: "second question"})
	require.Equal(t, convID, events2[0].ConversationID, "continuation must reuse the same conversation")

	msgs := provider.lastReq.Messages
	// lastReq captures the final tool-loop round; the leading four messages are
	// the turn's server-assembled context.
	require.GreaterOrEqual(t, len(msgs), 4, "provider request = %#v, want system + both turns assembled", msgs)
	require.Equal(t, "system", msgs[0].Role, "server-owned system prompt must lead the assembled context")
	roles := make([]string, 0, 3)
	contents := make([]string, 0, 3)
	for _, m := range msgs[1:4] {
		roles = append(roles, m.Role)
		contents = append(contents, m.Content)
	}
	require.Equal(t, []string{"user", "assistant", "user"}, roles, "assembled history = %#v", roles)
	require.Equal(t, []string{"first question", "answer one", "second question"}, contents)

	var convCount int64
	require.NoError(t, db.Model(&model.AgentConversation{}).Count(&convCount).Error)
	require.EqualValues(t, 1, convCount, "both turns must append to one conversation")

	var stored []model.AgentMessage
	require.NoError(t, db.Order("id ASC").Find(&stored).Error)
	require.Len(t, stored, 4, "stored messages = %#v, want user/assistant/user/assistant", stored)
}

// A-01: a foreign or missing conversation id is rejected before any provider
// call; ownership is not probeable.
func TestChatStreamRejectsForeignAndMissingConversation(t *testing.T) {
	provider := &streamToolProvider{}
	svc, db := newStreamTestService(t, provider, continuationTestConfig(100000))

	events := runChatTurn(t, svc, 7, ChatTurnInput{Message: "mine"})
	convID := events[0].ConversationID
	require.NotZero(t, convID)
	provider.calls = 0 // only the rejected turns count from here on

	require.NoError(t, db.Create(&model.User{ID: 8, Username: "other", Email: "other@example.com"}).Error)

	err := svc.ChatStream(context.Background(), 8, ChatTurnInput{ConversationID: convID, Message: "hijack"}, resolveGlobalChatContext(t, svc, 8), func(ev AgentStreamEvent) error { return nil })
	require.ErrorIs(t, err, ErrAgentConversationNotFound)

	err = svc.ChatStream(context.Background(), 7, ChatTurnInput{ConversationID: 999999, Message: "ghost"}, resolveGlobalChatContext(t, svc, 7), func(ev AgentStreamEvent) error { return nil })
	require.ErrorIs(t, err, ErrAgentConversationNotFound)

	require.Zero(t, provider.calls, "foreign/missing conversation must be rejected before any provider call")
}

// A-01: server-side context assembly keeps the most recent exchanges within
// the token budget; a single over-long message is tail-truncated instead of
// dropping the whole exchange.
func TestChatStreamAssemblesHistoryWithinTokenBudget(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{{{Content: "ok"}, {Done: true}}}}
	svc, db := newStreamTestService(t, provider, continuationTestConfig(120))

	conv := &model.AgentConversation{UserID: 7, ContextType: "general"}
	require.NoError(t, db.Create(conv).Error)
	seed := []model.AgentMessage{
		{ConversationID: conv.ID, Role: "user", Content: strPtr(strings.Repeat("古", 200))}, // 200 tokens — overlong, dropped
		{ConversationID: conv.ID, Role: "assistant", Content: strPtr(strings.Repeat("答", 60))},
		{ConversationID: conv.ID, Role: "user", Content: strPtr(strings.Repeat("今", 40))},
		{ConversationID: conv.ID, Role: "assistant", Content: strPtr(strings.Repeat("复", 40))},
	}
	for i := range seed {
		require.NoError(t, db.Create(&seed[i]).Error)
	}

	events := runChatTurn(t, svc, 7, ChatTurnInput{ConversationID: conv.ID, Message: "tail question"})
	require.Equal(t, conv.ID, events[0].ConversationID)

	msgs := provider.lastReq.Messages
	require.Equal(t, "system", msgs[0].Role)
	var userContents []string
	for _, m := range msgs[1:] {
		if m.Role == "user" {
			userContents = append(userContents, m.Content)
		}
	}
	require.NotContains(t, userContents, strings.Repeat("古", 200), "the over-budget oldest exchange must be dropped")
	require.Contains(t, userContents, strings.Repeat("今", 40), "the recent exchange must survive the budget")
	require.Contains(t, userContents, "tail question", "the newest user message must always be included")
	for _, m := range msgs[1:] {
		require.LessOrEqual(t, len([]rune(m.Content)), 120, "no single message may exceed the whole budget")
	}
}

// strPtr is shared with ip_proposal_service_test.go in this package.

// A-01: after the first turn completes, an async short call stores a summary
// title; the stored title wins over later regeneration.
func TestChatStreamAutoTitleStoredAfterFirstTurn(t *testing.T) {
	provider := &titleChatProvider{chatText: "站点推荐指南"}
	provider.rounds = [][]llm.ChatDelta{{{Content: "answer"}, {Done: true}}}
	svc, _ := newStreamTestService(t, provider, continuationTestConfig(100000))

	events := runChatTurn(t, svc, 7, ChatTurnInput{Message: "推荐一些站点"})
	convID := events[0].ConversationID

	require.Eventually(t, func() bool {
		conv := conversationByID(t, svc, convID)
		return conv.Title != nil && *conv.Title == "站点推荐指南"
	}, 2*time.Second, 10*time.Millisecond, "auto title must be stored after the first turn")
}

// A-01: when the title call fails the stored title falls back to a truncation
// of the first user message.
func TestChatStreamAutoTitleFallsBackToFirstUserMessage(t *testing.T) {
	provider := &titleChatProvider{chatErr: errors.New("title llm down")}
	provider.rounds = [][]llm.ChatDelta{{{Content: "answer"}, {Done: true}}}
	svc, _ := newStreamTestService(t, provider, continuationTestConfig(100000))

	long := strings.Repeat("问", 80)
	events := runChatTurn(t, svc, 7, ChatTurnInput{Message: long})
	convID := events[0].ConversationID

	require.Eventually(t, func() bool {
		conv := conversationByID(t, svc, convID)
		if conv.Title == nil {
			return false
		}
		got := []rune(*conv.Title)
		return len(got) <= 50 && strings.HasPrefix(*conv.Title, strings.Repeat("问", 10))
	}, 2*time.Second, 10*time.Millisecond, "fallback title = first user message truncation (<=50 runes)")
}

// A-01: later turns must not overwrite an established title.
func TestChatStreamAutoTitleNotRegeneratedOnLaterTurns(t *testing.T) {
	provider := &titleChatProvider{chatText: "第一轮标题"}
	provider.rounds = [][]llm.ChatDelta{{{Content: "answer"}, {Done: true}}}
	svc, _ := newStreamTestService(t, provider, continuationTestConfig(100000))

	events := runChatTurn(t, svc, 7, ChatTurnInput{Message: "first"})
	convID := events[0].ConversationID
	require.Eventually(t, func() bool {
		conv := conversationByID(t, svc, convID)
		return conv.Title != nil
	}, 2*time.Second, 10*time.Millisecond)

	provider.chatText = "第二轮标题"
	runChatTurn(t, svc, 7, ChatTurnInput{ConversationID: convID, Message: "second"})

	conv := conversationByID(t, svc, convID)
	require.NotNil(t, conv.Title)
	require.Equal(t, "第一轮标题", *conv.Title, "later turns must not overwrite the stored title")
}

// A-01: PATCH-facing conversation mutation lives in the handler; this service
// test only locks the owner-scoped conversation loader contract used by chat
// continuation (foreign id must not leak existence).
func TestChatStreamContinuationAppendsUserMessageAndBumpsUpdatedAt(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{{{Content: "answer"}, {Done: true}}}}
	svc, db := newStreamTestService(t, provider, continuationTestConfig(100000))

	events := runChatTurn(t, svc, 7, ChatTurnInput{Message: "one"})
	convID := events[0].ConversationID
	before := conversationByID(t, svc, convID)

	time.Sleep(10 * time.Millisecond)
	runChatTurn(t, svc, 7, ChatTurnInput{ConversationID: convID, Message: "two"})

	after := conversationByID(t, svc, convID)
	require.True(t, after.UpdatedAt.After(before.UpdatedAt), "continuation must bump conversation updated_at (list ordering key)")

	var userMsgs []model.AgentMessage
	require.NoError(t, db.Where("conversation_id = ? AND role = ?", convID, "user").Order("id ASC").Find(&userMsgs).Error)
	require.Len(t, userMsgs, 2, "each turn appends exactly one user message")
}

// A-01 review fix: an emit failure on the success path (here: the usage
// event) must still retain the generated answer — the conversation keeps
// whatever content existed at the moment the stream broke.
func TestChatStreamEmitFailureRetainsGeneratedAnswer(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{{{Content: "full answer"}, {Done: true}}}}
	svc, db := newStreamTestService(t, provider, continuationTestConfig(100000))

	err := svc.ChatStream(context.Background(), 7, ChatTurnInput{Message: "hi"}, resolveGlobalChatContext(t, svc, 7), func(ev AgentStreamEvent) error {
		if ev.Type == AgentEventUsage {
			return errors.New("client write failed")
		}
		return nil
	})
	require.Error(t, err)

	var assistant []model.AgentMessage
	require.NoError(t, db.Where("role = ?", "assistant").Find(&assistant).Error)
	require.Len(t, assistant, 1, "emit failure must retain the generated answer")
	require.NotNil(t, assistant[0].Content)
	require.Equal(t, "full answer", *assistant[0].Content)
}

// A-01 review fix: assembleChatContext must tail-truncate a single over-long
// message and keep it in the assembled context instead of dropping it.
func TestAssembleChatContextTruncatesOverlongSingleMessage(t *testing.T) {
	system := llm.ChatMessage{Role: "system", Content: "sys"}
	overlong := strings.Repeat("长", 300)
	history := []model.AgentMessage{
		{Role: "user", Content: strPtr(overlong)},
	}
	msgs := assembleChatContext(system, history, 200, 10)
	require.Len(t, msgs, 2, "over-long newest message must stay included, truncated")
	require.Equal(t, "system", msgs[0].Role)
	require.LessOrEqual(t, len([]rune(msgs[1].Content)), 200)
	require.Contains(t, msgs[1].Content, strings.Repeat("长", 10), "head of the message must be kept")
	require.True(t, strings.HasSuffix(msgs[1].Content, "…"), "truncation marker must terminate the tail cut: %q", msgs[1].Content)
}

// A-01 review fix: legacy conversations (assistant history present, NULL
// title — every pre-A-01 survivor looks like this) must never be auto-titled
// on continuation.
func TestChatStreamLegacyConversationNotRetitled(t *testing.T) {
	provider := &titleChatProvider{chatText: "不该出现的标题"}
	provider.rounds = [][]llm.ChatDelta{{{Content: "second answer"}, {Done: true}}}
	svc, db := newStreamTestService(t, provider, continuationTestConfig(100000))

	conv := &model.AgentConversation{UserID: 7, ContextType: "general"}
	require.NoError(t, db.Create(conv).Error)
	require.NoError(t, db.Create(&model.AgentMessage{ConversationID: conv.ID, Role: "user", Content: strPtr("旧问题")}).
		Error)
	require.NoError(t, db.Create(&model.AgentMessage{ConversationID: conv.ID, Role: "assistant", Content: strPtr("旧回答")}).
		Error)

	runChatTurn(t, svc, 7, ChatTurnInput{ConversationID: conv.ID, Message: "继续"})
	time.Sleep(100 * time.Millisecond)

	stored := conversationByID(t, svc, conv.ID)
	require.Nil(t, stored.Title, "legacy conversation must not be auto-titled")
	require.Zero(t, provider.chatCalls, "no title LLM call may happen for legacy conversations")
}

// A-01 review fix: a conversation that already carries a title (e.g. set via
// PATCH before the first completed turn) must keep it even when the first
// successful assistant answer triggers the title path.
func TestChatStreamSeededTitleNotOverwritten(t *testing.T) {
	provider := &titleChatProvider{chatText: "LLM 新标题"}
	provider.rounds = [][]llm.ChatDelta{{{Content: "answer"}, {Done: true}}}
	svc, db := newStreamTestService(t, provider, continuationTestConfig(100000))

	conv := &model.AgentConversation{UserID: 7, ContextType: "general", Title: strPtr("用户手动命名")}
	require.NoError(t, db.Create(conv).Error)

	runChatTurn(t, svc, 7, ChatTurnInput{ConversationID: conv.ID, Message: "first completed turn"})
	// Wait until the async title call actually ran, so the assertion below
	// checks the post-goroutine state instead of racing past it.
	require.Eventually(t, func() bool { return provider.chatCalls >= 1 }, 2*time.Second, 10*time.Millisecond)

	stored := conversationByID(t, svc, conv.ID)
	require.NotNil(t, stored.Title)
	require.Equal(t, "用户手动命名", *stored.Title, "WHERE title IS NULL must protect an established title")
}

// A-01 review fix: the TOCTOU conversation-not-found path needs its own safe
// copy instead of the generic provider-unavailable text.
func TestSafeAgentStreamMessageConversationNotFound(t *testing.T) {
	require.Equal(t, "conversation no longer available", safeAgentStreamMessage(AgentStreamEventType(AgentErrorCodeConversationNotFound)))
}
