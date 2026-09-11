package service

import (
	"context"
	"errors"
	"testing"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
)

// partialThenFailProvider streams one content delta and then fails, so the
// answer buffer holds partial content at failure time — the exact state a
// client "stop" or provider drop leaves behind mid-turn.
type partialThenFailProvider struct {
	streamToolProvider
	delta string
	err   error
}

func (p *partialThenFailProvider) ChatStream(ctx context.Context, req llm.ChatRequest, handler func(delta llm.ChatDelta) error) error {
	p.calls++
	p.lastReq = req
	p.ctxSeen = ctx
	if p.delta != "" {
		if err := handler(llm.ChatDelta{Content: p.delta}); err != nil {
			return err
		}
	}
	return p.err
}

// A-01: provider failure must keep the conversation and persist the partial
// answer that already streamed out. Deletion may only happen on the user's
// explicit DELETE call.
func TestAgentStreamProviderFailureKeepsConversationAndPartialAnswer(t *testing.T) {
	provider := &partialThenFailProvider{delta: "partial answer", err: errors.New("RAW PROVIDER SECRET")}
	svc, db := newStreamTestService(t, provider, nil)

	var events []AgentStreamEvent
	err := svc.ChatStream(
		context.Background(),
		7,
		ChatTurnInput{Message: "hi"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error { events = append(events, ev); return nil },
	)
	if err == nil {
		t.Fatal("ChatStream must return the provider error")
	}

	var convCount int64
	if err := db.Model(&model.AgentConversation{}).Count(&convCount).Error; err != nil {
		t.Fatalf("count conversations: %v", err)
	}
	if convCount != 1 {
		t.Fatalf("conversations after provider failure = %d, want 1 (conversation must be kept)", convCount)
	}

	var messages []model.AgentMessage
	if err := db.Order("id ASC").Find(&messages).Error; err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("persisted messages = %d, want 2 (user + partial assistant); got %#v", len(messages), messages)
	}
	if messages[0].Role != "user" || messages[0].Content == nil || *messages[0].Content != "hi" {
		t.Fatalf("first message = %#v, want the user turn", messages[0])
	}
	if messages[1].Role != "assistant" || messages[1].Content == nil || *messages[1].Content != "partial answer" {
		t.Fatalf("assistant message = %#v, want the partial streamed content retained", messages[1])
	}
}

// A-01: client cancellation (stop button / disconnect) keeps the conversation
// and the already-streamed partial content.
func TestAgentStreamCancellationKeepsConversationAndPartialAnswer(t *testing.T) {
	provider := &partialThenFailProvider{delta: "half said", err: context.Canceled}
	svc, db := newStreamTestService(t, provider, nil)

	var events []AgentStreamEvent
	err := svc.ChatStream(
		context.Background(),
		7,
		ChatTurnInput{Message: "hi"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error { events = append(events, ev); return nil },
	)
	if err == nil {
		t.Fatal("cancelled ChatStream must return an error")
	}
	sawCancelled := false
	for _, ev := range events {
		if ev.Type == AgentEventError && ev.ErrorCode == "STREAM_CANCELLED" {
			sawCancelled = true
		}
	}
	if !sawCancelled {
		t.Fatalf("events = %#v, want STREAM_CANCELLED error event", events)
	}

	var convCount int64
	if err := db.Model(&model.AgentConversation{}).Count(&convCount).Error; err != nil {
		t.Fatalf("count conversations: %v", err)
	}
	if convCount != 1 {
		t.Fatalf("conversations after cancellation = %d, want 1 (stop must retain the conversation)", convCount)
	}

	var assistant []model.AgentMessage
	if err := db.Where("role = ?", "assistant").Find(&assistant).Error; err != nil {
		t.Fatalf("load assistant messages: %v", err)
	}
	if len(assistant) != 1 || assistant[0].Content == nil || *assistant[0].Content != "half said" {
		t.Fatalf("assistant messages = %#v, want the partial content persisted with a detached context", assistant)
	}
}
