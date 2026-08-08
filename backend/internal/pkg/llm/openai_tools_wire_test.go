package llm_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"omnicraft/backend/internal/pkg/llm"
)

// TestOpenAICompatProvider_ToolsWireFormat guards the OpenAI-compatible wire
// format: tools must be sent as {"type":"function","function":{...}}, which
// DeepSeek and other openai_compat providers require. The legacy flat shape
// ({"name":...}) is rejected with HTTP 400 by those providers.
func TestOpenAICompatProvider_ToolsWireFormat(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		gotBody = buf.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	p := llm.NewOpenAICompatProvider("test-key", server.URL, "deepseek-chat", "text-embedding-3-small")
	_, err := p.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.ChatMessage{{Role: "user", Content: "hi"}},
		Tools: []llm.ToolDefinition{
			{Name: "search_content", Description: "read-only retrieval tool", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}, "required": []string{}}},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	var payload struct {
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tools"`
	}
	if err := json.Unmarshal([]byte(gotBody), &payload); err != nil {
		t.Fatalf("decode captured body: %v", err)
	}
	if len(payload.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(payload.Tools))
	}
	tool := payload.Tools[0]
	if tool.Type != "function" {
		t.Errorf("expected tool.type=function, got %q", tool.Type)
	}
	if tool.Function.Name != "search_content" {
		t.Errorf("expected tool.function.name=search_content, got %q", tool.Function.Name)
	}
}
