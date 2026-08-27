package service

import (
	"context"
	"testing"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/llm"
)

// streamingOnlyProvider intentionally implements no Chat or embedding method.
// The Agent workspace stream must depend only on the capability it consumes.
type streamingOnlyProvider struct{}

func (streamingOnlyProvider) ChatStream(_ context.Context, _ llm.ChatRequest, handler func(llm.ChatDelta) error) error {
	return handler(llm.ChatDelta{Content: "streamed", Done: true})
}

func TestAgentStreamProviderSeamAcceptsStreamingOnlyAdapter(t *testing.T) {
	svc := newAgentServiceWithChatStreamer(
		nil,
		streamingOnlyProvider{},
		nil,
		nil,
		nil,
		nil,
		&config.Config{Agent: config.AgentConfig{
			WebAgentEnabled:     true,
			MaxToolCallsPerTurn: 1,
			CitationMaxCount:    1,
			MaxOutputTokens:     100,
		}},
	)

	var events []AgentStreamEvent
	err := svc.ChatStream(
		context.Background(),
		1,
		[]llm.ChatMessage{{Role: "user", Content: "hello"}},
		&ResolvedChatContext{},
		func(event AgentStreamEvent) error {
			events = append(events, event)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("ChatStream with streaming-only adapter: %v", err)
	}
	if len(events) == 0 || events[len(events)-1].Type != AgentEventDone {
		t.Fatalf("events = %#v, want terminal done event", events)
	}
}
