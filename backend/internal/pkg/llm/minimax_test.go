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
		{name: "nested block never leaks", chunks: []string{"<think>outer<think>inner</think>still secret</think>visible"}, expected: "visible"},
		{name: "nested block across chunks", chunks: []string{"safe<think>outer<thi", "nk>inner</think>secret", "</think>tail"}, expected: "safetail"},
		{name: "stray closing tag dropped", chunks: []string{"a</think>b"}, expected: "ab"},
		{name: "unclosed block at end dropped on flush", chunks: []string{"pre<think>hidden"}, expected: "pre"},
		{name: "partial open tag at end dropped on flush", chunks: []string{"text<thi"}, expected: "text"},
		{name: "closing tag split across chunks", chunks: []string{"<think>a</th", "ink>b"}, expected: "b"},
		{name: "block spanning many chunks", chunks: []string{"<th", "ink>", "secret", "</", "think>", "done"}, expected: "done"},
		// #535: MiniMax also emits the <mm:think> family, observed mixed with
		// <think> within one conversation; both must route to the thinking
		// channel and never leak into content.
		{name: "mm think block stripped", chunks: []string{"before<mm:think>hidden</mm:think>after"}, expected: "beforeafter"},
		{name: "mm block at start", chunks: []string{"<mm:think>hidden</mm:think>shown"}, expected: "shown"},
		{name: "mm block split across chunks", chunks: []string{"<mm:th", "ink>hidden</mm:thi", "nk>tail"}, expected: "tail"},
		{name: "mm partial open tag buffered across chunks", chunks: []string{"text<mm", ":think>secret</mm:think>done"}, expected: "textdone"},
		{name: "mm unclosed block dropped on flush", chunks: []string{"pre<mm:think>hidden"}, expected: "pre"},
		{name: "mm stray closing tag dropped", chunks: []string{"a</mm:think>b"}, expected: "ab"},
		{name: "mm nested blocks", chunks: []string{"a<mm:think>x<mm:think>y</mm:think>z</mm:think>b"}, expected: "ab"},
		{name: "mixed families in one stream", chunks: []string{"a<think>x</think>b<mm:think>y</mm:think>c"}, expected: "abc"},
		{name: "mixed families across chunks", chunks: []string{"a<think>x</th", "ink>b<mm:thi", "nk>y</mm:think>c"}, expected: "abc"},
		{name: "mm think tag fragment vs body text", chunks: []string{"5 < 6 and 7 > 2 ok"}, expected: "5 < 6 and 7 > 2 ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newThinkSplitter()
			var got strings.Builder
			for _, c := range tt.chunks {
				_, content := s.split(c)
				got.WriteString(content)
			}
			_, flushContent := s.splitFlush()
			got.WriteString(flushContent)
			if got.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got.String())
			}
		})
	}
}

func TestThinkSplitterRoutesThinking(t *testing.T) {
	// #535: reasoning must land in the thinking channel (the SSE think
	// delta + persisted think row), for both tag families.
	tests := []struct {
		name             string
		chunks           []string
		expectedThinking string
	}{
		{name: "think family", chunks: []string{"<think>why</think>body"}, expectedThinking: "why"},
		{name: "mm think family", chunks: []string{"<mm:think>reasoning here</mm:think>body"}, expectedThinking: "reasoning here"},
		{name: "mm family across chunks", chunks: []string{"<mm:th", "ink>deep thought</mm", ":think>answer"}, expectedThinking: "deep thought"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newThinkSplitter()
			var thinking strings.Builder
			for _, c := range tt.chunks {
				th, _ := s.split(c)
				thinking.WriteString(th)
			}
			th, _ := s.splitFlush()
			thinking.WriteString(th)
			got := thinking.String()
			if got != tt.expectedThinking {
				t.Errorf("expected thinking %q, got %q", tt.expectedThinking, got)
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

func TestMiniMaxProvider_GetEmbedding_UsesLegacyBaseAndGroupID(t *testing.T) {
	var receivedReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedReq = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"vectors":[[0.1]],"base_resp":{"status_code":0}}`)
	}))
	defer server.Close()

	p := NewMiniMaxProvider("test-key", "https://chat.example.test", "MiniMax-M3", "embo-01",
		WithEmbeddingAPIBase(server.URL), WithEmbeddingGroupID("group/a"))
	if _, err := p.GetEmbedding(context.Background(), "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedReq.URL.Path != "/v1/embeddings" {
		t.Fatalf("path = %q, want /v1/embeddings", receivedReq.URL.Path)
	}
	if got := receivedReq.URL.Query().Get("GroupId"); got != "group/a" {
		t.Fatalf("GroupId = %q, want group/a", got)
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
			Choices: []openAIChoice{{
				Message: openAIMessage{Content: "<think>internal reasoning</think>visible answer"},
			}},
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

// #539: the MiniMax provider maps ChatRequest.Thinking onto the wire —
// disabled/adaptive become thinking.type, the zero value omits the field; the
// plain openai_compat provider never emits it.
func TestMiniMaxProvider_ThinkingWire(t *testing.T) {
	type captured struct {
		body   map[string]any
		stream bool
	}
	bodies := make(chan captured, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		json.Unmarshal(raw, &parsed)
		_, stream := parsed["stream"]
		bodies <- captured{body: parsed, stream: stream}
		if stream {
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\n"+"data: [DONE]\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResponse{Choices: []openAIChoice{{Message: openAIMessage{Content: "ok"}}}})
	}))
	defer server.Close()

	p := NewMiniMaxProvider("test-key", server.URL, "MiniMax-M3", "embo-01")

	if err := p.ChatStream(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}}, func(delta ChatDelta) error { return nil }); err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if err := p.ChatStream(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}, Thinking: ThinkingDisabled}, func(delta ChatDelta) error { return nil }); err != nil {
		t.Fatalf("ChatStream disabled: %v", err)
	}
	if _, err := p.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}, Thinking: ThinkingAdaptive}); err != nil {
		t.Fatalf("Chat adaptive: %v", err)
	}

	first := <-bodies
	if _, present := first.body["thinking"]; present {
		t.Errorf("default request must omit the thinking field, got %v", first.body["thinking"])
	}
	second := <-bodies
	thinking, _ := second.body["thinking"].(map[string]any)
	if thinking["type"] != "disabled" {
		t.Errorf("ThinkingDisabled must map to thinking.type=disabled, got %v", second.body["thinking"])
	}
	third := <-bodies
	if _, isStream := third.body["stream"]; isStream {
		t.Errorf("Chat must not send the stream field")
	}
	thinking, _ = third.body["thinking"].(map[string]any)
	if thinking["type"] != "adaptive" {
		t.Errorf("ThinkingAdaptive must map to thinking.type=adaptive, got %v", third.body["thinking"])
	}
}

// #539: providers without a thinking style never emit the thinking field, so
// their request bodies stay byte-compatible with plain OpenAI servers.
func TestOpenAICompatProvider_ThinkingFieldOmittedWithoutWire(t *testing.T) {
	bodies := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		json.Unmarshal(raw, &parsed)
		bodies <- parsed
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResponse{Choices: []openAIChoice{{Message: openAIMessage{Content: "ok"}}}})
	}))
	defer server.Close()

	p := NewOpenAICompatProvider("test-key", server.URL, "m", "e")
	if _, err := p.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}, Thinking: ThinkingDisabled}); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	body := <-bodies
	if _, present := body["thinking"]; present {
		t.Errorf("openai_compat must not emit thinking, got %v", body["thinking"])
	}
}

// #545: the deepseek style maps Disabled→disabled and Adaptive→enabled on the
// openai_compat wire; a request without a mode keeps the field omitted.
func TestDeepSeekThinkingStyleWire(t *testing.T) {
	bodies := make(chan map[string]any, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		json.Unmarshal(raw, &parsed)
		bodies <- parsed
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResponse{Choices: []openAIChoice{{Message: openAIMessage{Content: "ok"}}}})
	}))
	defer server.Close()

	p := NewOpenAICompatProvider("k", server.URL, "deepseek-chat", "", WithThinkingStyle("deepseek"))
	if _, err := p.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}}); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if _, err := p.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}, Thinking: ThinkingDisabled}); err != nil {
		t.Fatalf("Chat disabled: %v", err)
	}
	if _, err := p.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}, Thinking: ThinkingAdaptive}); err != nil {
		t.Fatalf("Chat adaptive: %v", err)
	}

	first := <-bodies
	if _, present := first["thinking"]; present {
		t.Errorf("no mode must omit thinking, got %v", first["thinking"])
	}
	second := <-bodies
	thinking, _ := second["thinking"].(map[string]any)
	if thinking["type"] != "disabled" {
		t.Errorf("ThinkingDisabled must map to disabled, got %v", second["thinking"])
	}
	third := <-bodies
	thinking, _ = third["thinking"].(map[string]any)
	if thinking["type"] != "enabled" {
		t.Errorf("ThinkingAdaptive must map to enabled (DeepSeek naming), got %v", third["thinking"])
	}
}
