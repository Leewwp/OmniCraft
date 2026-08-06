package llm_test

import (
	"context"
	"errors"
	"testing"

	"omnicraft/backend/internal/pkg/llm"
)

type mockProvider struct {
	chatResp     *llm.ChatResponse
	chatErr      error
	streamChunks []string
	streamErr    error
	embedding    []float32
	embedErr     error
}

func (m *mockProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	return m.chatResp, m.chatErr
}

func (m *mockProvider) ChatStream(_ context.Context, _ llm.ChatRequest, handler func(llm.ChatDelta) error) error {
	if m.streamErr != nil {
		return m.streamErr
	}
	for i, chunk := range m.streamChunks {
		done := i == len(m.streamChunks)-1
		if err := handler(llm.ChatDelta{Content: chunk, Done: done}); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockProvider) GetEmbedding(_ context.Context, _ string) ([]float32, error) {
	return m.embedding, m.embedErr
}

func TestMockProvider_Chat(t *testing.T) {
	p := &mockProvider{chatResp: &llm.ChatResponse{Content: "hello"}}
	resp, err := p.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello" {
		t.Errorf("expected 'hello', got %q", resp.Content)
	}
}

func TestMockProvider_Chat_Error(t *testing.T) {
	p := &mockProvider{chatErr: errors.New("api unavailable")}
	_, err := p.Chat(context.Background(), llm.ChatRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "api unavailable" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestMockProvider_ChatStream(t *testing.T) {
	p := &mockProvider{streamChunks: []string{"foo", " ", "bar"}}
	var collected string
	err := p.ChatStream(context.Background(), llm.ChatRequest{}, func(d llm.ChatDelta) error {
		collected += d.Content
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if collected != "foo bar" {
		t.Errorf("expected 'foo bar', got %q", collected)
	}
}

func TestMockProvider_ChatStream_LastDeltaDone(t *testing.T) {
	chunks := []string{"a", "b", "c"}
	p := &mockProvider{streamChunks: chunks}
	var lastDone bool
	_ = p.ChatStream(context.Background(), llm.ChatRequest{}, func(d llm.ChatDelta) error {
		lastDone = d.Done
		return nil
	})
	if !lastDone {
		t.Error("expected last delta to have Done=true")
	}
}

func TestMockProvider_ChatStream_Error(t *testing.T) {
	p := &mockProvider{streamErr: errors.New("stream error")}
	err := p.ChatStream(context.Background(), llm.ChatRequest{}, func(_ llm.ChatDelta) error { return nil })
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMockProvider_GetEmbedding(t *testing.T) {
	expected := []float32{0.1, 0.2, 0.3}
	p := &mockProvider{embedding: expected}
	result, err := p.GetEmbedding(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != len(expected) {
		t.Fatalf("expected %d dims, got %d", len(expected), len(result))
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("dim[%d]: expected %v, got %v", i, v, result[i])
		}
	}
}

func TestMockProvider_GetEmbedding_Error(t *testing.T) {
	p := &mockProvider{embedErr: errors.New("embedding failed")}
	_, err := p.GetEmbedding(context.Background(), "text")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestProviderInterface_Compile(t *testing.T) {
	var _ llm.LLMProvider = (*mockProvider)(nil)
}
