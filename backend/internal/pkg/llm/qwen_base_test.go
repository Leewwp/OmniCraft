package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestQwenProviderDefaultBaseURL pins the built-in endpoint: without
// AGENT_LLM_API_BASE the provider targets the public DashScope
// compatible-mode host.
func TestQwenProviderDefaultBaseURL(t *testing.T) {
	t.Setenv("AGENT_LLM_API_BASE", "")
	p := NewQwenProvider("test-key", "qwen-plus", "text-embedding-v4")
	if p.baseURL != qwenBaseURL {
		t.Fatalf("default baseURL = %q, want %q", p.baseURL, qwenBaseURL)
	}
}

// TestQwenProviderAPIBaseEnvOverride routes the provider through
// AGENT_LLM_API_BASE (trailing slash trimmed) so the root .env stays the
// single endpoint entry, including dedicated DashScope instance hosts.
func TestQwenProviderAPIBaseEnvOverride(t *testing.T) {
	t.Setenv("AGENT_LLM_API_BASE", "https://ws-instance.example.com/compatible-mode/")
	p := NewQwenProvider("test-key", "qwen-plus", "text-embedding-v4")
	if p.baseURL != "https://ws-instance.example.com/compatible-mode" {
		t.Fatalf("baseURL = %q, want trimmed override", p.baseURL)
	}
}

// TestQwenProviderChatHitsOverriddenBase proves the override reaches the
// wire: the chat POST must land on {base}/v1/chat/completions.
func TestQwenProviderChatHitsOverriddenBase(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":"ok"}}]}}`))
	}))
	defer server.Close()

	t.Setenv("AGENT_LLM_API_BASE", server.URL)
	p := NewQwenProvider("test-key", "qwen-plus", "text-embedding-v4")
	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("request path = %q, want /v1/chat/completions", gotPath)
	}
	if resp.Content != "ok" {
		t.Fatalf("content = %q, want ok", resp.Content)
	}
}
