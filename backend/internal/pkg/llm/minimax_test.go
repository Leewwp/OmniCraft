package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestThinkStripper(t *testing.T) {
	tests := []struct {
		name     string
		chunks   []string
		expected string
	}{
		{name: "plain text passes through", chunks: []string{"hello world"}, expected: "hello world"},
		{name: "full think block stripped", chunks: []string{"before<think>hidden</think>after"}, expected: "beforeafter"},
		{name: "block at start", chunks: []string{"<think>hidden</think>shown"}, expected: "shown"},
		{name: "think block split across chunks", chunks: []string{"<thi", "nk>hidden</thin", "k>tail"}, expected: "tail"},
		{name: "multiple blocks", chunks: []string{"a<think>x</think>b<think>y</think>c"}, expected: "abc"},
		{name: "stray closing tag dropped", chunks: []string{"a</think>b"}, expected: "ab"},
		{name: "unclosed block at end dropped on flush", chunks: []string{"pre<think>hidden"}, expected: "pre"},
		{name: "partial open tag at end dropped on flush", chunks: []string{"text<thi"}, expected: "text"},
		{name: "closing tag split across chunks", chunks: []string{"<think>a</th", "ink>b"}, expected: "b"},
		{name: "block spanning many chunks", chunks: []string{"<th", "ink>", "secret", "</", "think>", "done"}, expected: "done"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newThinkStripper()
			var got strings.Builder
			for _, c := range tt.chunks {
				got.WriteString(s.next(c))
			}
			got.WriteString(s.flush())
			if got.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got.String())
			}
		})
	}
}

func TestMiniMaxProvider_GetEmbedding_Serialization(t *testing.T) {
	var receivedReq *http.Request
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedReq = r
		receivedBody, _ = io.ReadAll(r.Body)

		resp := minimaxEmbeddingResponse{
			Vectors: [][]float32{{0.1, 0.2, 0.3}},
		}
		resp.BaseResp.StatusCode = 0
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewMiniMaxProvider("test-key", server.URL, "MiniMax-M1", "embo-01")
	embedding, err := p.GetEmbedding(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	auth := receivedReq.Header.Get("Authorization")
	if auth != "Bearer test-key" {
		t.Errorf("expected Authorization %q, got %q", "Bearer test-key", auth)
	}
	if receivedReq.URL.Path != "/v1/embeddings" {
		t.Errorf("expected path /v1/embeddings, got %s", receivedReq.URL.Path)
	}

	var reqBody minimaxEmbeddingRequest
	if err := json.Unmarshal(receivedBody, &reqBody); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if reqBody.Model != "embo-01" {
		t.Errorf("expected model %q, got %q", "embo-01", reqBody.Model)
	}
	if len(reqBody.Texts) != 1 || reqBody.Texts[0] != "hello world" {
		t.Errorf("expected texts [hello world], got %v", reqBody.Texts)
	}
	if reqBody.Type != "query" {
		t.Errorf("expected type %q, got %q", "query", reqBody.Type)
	}

	expected := []float32{0.1, 0.2, 0.3}
	if len(embedding) != len(expected) {
		t.Fatalf("expected %d dims, got %d", len(expected), len(embedding))
	}
	for i, v := range expected {
		if embedding[i] != v {
			t.Errorf("dim[%d]: expected %v, got %v", i, v, embedding[i])
		}
	}
}

func TestMiniMaxProvider_GetEmbedding_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := minimaxEmbeddingResponse{}
		resp.BaseResp.StatusCode = 2013
		resp.BaseResp.StatusMsg = "invalid params"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewMiniMaxProvider("test-key", server.URL, "MiniMax-M1", "embo-01")
	_, err := p.GetEmbedding(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for non-zero base_resp.status_code")
	}
}

func TestMiniMaxProvider_Chat_StripsThink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Choices: []struct {
				Message struct {
					Content   string     `json:"content"`
					ToolCalls []ToolCall `json:"tool_calls,omitempty"`
				} `json:"message"`
				Delta struct {
					Content   string     `json:"content"`
					ToolCalls []ToolCall `json:"tool_calls,omitempty"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Message: struct {
						Content   string     `json:"content"`
						ToolCalls []ToolCall `json:"tool_calls,omitempty"`
					}{Content: "<think>internal reasoning</think>visible answer"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewMiniMaxProvider("test-key", server.URL, "MiniMax-M1", "embo-01")
	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "visible answer" {
		t.Errorf("expected think block stripped, got %q", resp.Content)
	}
}

func TestMiniMaxProvider_ChatStream_StripsThinkAcrossChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stream := []string{
			`data: {"choices":[{"delta":{"content":"<thi"}}]}`,
			`data: {"choices":[{"delta":{"content":"nk>secret"}}]}`,
			`data: {"choices":[{"delta":{"content":"</think>ans"}}]}`,
			`data: {"choices":[{"delta":{"content":"wer"}}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"stop"}}]`,
			"data: [DONE]",
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, strings.Join(stream, "\n")+"\n")
	}))
	defer server.Close()

	p := NewMiniMaxProvider("test-key", server.URL, "MiniMax-M1", "embo-01")
	var got strings.Builder
	err := p.ChatStream(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	}, func(delta ChatDelta) error {
		got.WriteString(delta.Content)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "answer" {
		t.Errorf("expected think block stripped across chunks, got %q", got.String())
	}
}

// MiniMax closes the stream after the finish_reason chunk ("stop" for plain
// answers, "tool_calls" for tool rounds) without a [DONE] sentinel; the
// provider must terminate cleanly on that chunk.
func TestMiniMaxProvider_ChatStream_TerminatesWithoutDONE(t *testing.T) {
	for _, tc := range []struct {
		name         string
		finishReason string
	}{
		{name: "stop_reason", finishReason: "stop"},
		{name: "tool_calls_reason", finishReason: "tool_calls"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				stream := []string{
					`data: {"choices":[{"delta":{"content":"hello"}}]}`,
					`data: {"choices":[{"delta":{"content":" world"}}]}`,
					fmt.Sprintf(`data: {"choices":[{"delta":{},"finish_reason":"%s"}]}`, tc.finishReason),
				}
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, strings.Join(stream, "\n")+"\n")
			}))
			defer server.Close()

			p := NewMiniMaxProvider("test-key", server.URL, "MiniMax-M1", "embo-01")
			var got strings.Builder
			err := p.ChatStream(context.Background(), ChatRequest{
				Messages: []ChatMessage{{Role: "user", Content: "hi"}},
			}, func(delta ChatDelta) error {
				got.WriteString(delta.Content)
				return nil
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != "hello world" {
				t.Errorf("expected %q, got %q", "hello world", got.String())
			}
		})
	}
}
