package service

import (
	"context"
	"testing"

	"omnicraft/backend/internal/pkg/llm"
)

// streamingOnlyProvider intentionally implements no Chat or embedding method.
// The Agent workspace stream must depend only on the capability it consumes.
type streamingOnlyProvider struct{}

func (streamingOnlyProvider) ChatStream(_ context.Context, _ llm.ChatRequest, handler func(llm.ChatDelta) error) error {
	return handler(llm.ChatDelta{Content: "streamed", Done: true})
}

func TestAgentStreamProviderSeamAcceptsStreamingOnlyAdapter(t *testing.T) {
	var seam agentChatStreamer = streamingOnlyProvider{}
	if err := seam.ChatStream(context.Background(), llm.ChatRequest{}, func(llm.ChatDelta) error { return nil }); err != nil {
		t.Fatalf("streaming-only adapter returned error: %v", err)
	}
}
