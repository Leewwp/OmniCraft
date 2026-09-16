package llm

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// scriptedProvider is a fake chat model: it plays a fixed delta script per
// call and can fail with a configured error, recording the requests it saw.
type scriptedProvider struct {
	calls    int
	requests []ChatRequest
	scripts  [][]ChatDelta
	chatResp *ChatResponse
	chatErr  error
}

func (p *scriptedProvider) Chat(_ context.Context, req ChatRequest) (*ChatResponse, error) {
	p.calls++
	p.requests = append(p.requests, req)
	if p.chatErr != nil {
		return nil, p.chatErr
	}
	if p.chatResp != nil {
		return p.chatResp, nil
	}
	return &ChatResponse{Content: "ok"}, nil
}

func (p *scriptedProvider) ChatStream(_ context.Context, req ChatRequest, handler func(delta ChatDelta) error) error {
	p.calls++
	p.requests = append(p.requests, req)
	script := []ChatDelta{{Content: "default", Done: true}}
	if p.calls <= len(p.scripts) {
		script = p.scripts[p.calls-1]
	}
	for _, d := range script {
		if err := handler(d); err != nil {
			return err
		}
	}
	return nil
}

func (p *scriptedProvider) GetEmbedding(_ context.Context, _ string) ([]float32, error) {
	return []float32{1}, nil
}

func newTestRouter(primary, fallback *scriptedProvider, retryOn []string) *RoutingProvider {
	return NewRoutingProvider(
		map[string]LLMProvider{"a": primary, "b": fallback},
		[]string{"a", "b"},
		retryOn,
		[]AgentModelOption{{ID: "a", DisplayName: "A"}, {ID: "b", DisplayName: "B"}},
	)
}

// #544 form A: a stream that completes without error but emits only reasoning
// (whitespace-or-empty body, no tool calls) retries on the next model in the
// chain; the fallback's content becomes the turn's content.
func TestRoutingProviderChatStreamRetriesBlankAnswer(t *testing.T) {
	primary := &scriptedProvider{scripts: [][]ChatDelta{
		{{Thinking: "reasoning only"}, {Done: true}},
	}}
	fallback := &scriptedProvider{scripts: [][]ChatDelta{
		{{Content: "   "}, {Done: true}}, // whitespace-only counts as blank too
	}}
	router := newTestRouter(primary, fallback, []string{RetryOnBlankAnswer})

	var contents []string
	err := router.ChatStream(context.Background(), ChatRequest{}, func(delta ChatDelta) error {
		if delta.Content != "" {
			contents = append(contents, delta.Content)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if primary.calls != 1 || fallback.calls != 1 {
		t.Fatalf("calls = primary %d fallback %d, want 1/1", primary.calls, fallback.calls)
	}
	if strings.Join(contents, "") != "   " {
		t.Fatalf("contents = %q, want the fallback whitespace delta forwarded", contents)
	}
}

// A round with tool calls is a normal intermediate round: it never counts as
// blank and must not trigger a retry.
func TestRoutingProviderChatStreamToolRoundIsNotBlank(t *testing.T) {
	primary := &scriptedProvider{scripts: [][]ChatDelta{
		{routerToolCallDelta()},
	}}
	fallback := &scriptedProvider{}
	router := newTestRouter(primary, fallback, []string{RetryOnBlankAnswer})

	if err := router.ChatStream(context.Background(), ChatRequest{}, func(delta ChatDelta) error { return nil }); err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if fallback.calls != 0 {
		t.Fatalf("tool round must not retry, fallback calls = %d", fallback.calls)
	}
}

// provider_error failover only while nothing observable streamed: an error
// after content started flowing is surfaced (no half-answer concatenation).
func TestRoutingProviderChatStreamErrorFailoverRules(t *testing.T) {
	// Error before any content → silent failover to the next model.
	fallback := &scriptedProvider{scripts: [][]ChatDelta{{{Content: "fallback answer"}, {Done: true}}}}
	failingProvider := &failingScriptedProvider{err: errors.New("provider down")}
	router2 := NewRoutingProvider(
		map[string]LLMProvider{"a": failingProvider, "b": fallback},
		[]string{"a", "b"},
		[]string{RetryOnProviderError},
		nil,
	)
	var got string
	if err := router2.ChatStream(context.Background(), ChatRequest{}, func(delta ChatDelta) error {
		got += delta.Content
		return nil
	}); err != nil {
		t.Fatalf("ChatStream should fail over, got err %v", err)
	}
	if got != "fallback answer" {
		t.Fatalf("got = %q, want fallback answer", got)
	}

	// Error after content started → surfaced, no second attempt.
	partial := &partialStreamProvider{failAfter: "partial answer"}
	router3 := NewRoutingProvider(
		map[string]LLMProvider{"a": partial, "b": fallback},
		[]string{"a", "b"},
		[]string{RetryOnProviderError},
		nil,
	)
	got = ""
	err := router3.ChatStream(context.Background(), ChatRequest{}, func(delta ChatDelta) error {
		got += delta.Content
		return nil
	})
	if err == nil {
		t.Fatalf("error after content started must surface, got success with %q", got)
	}
	if fallback.calls != 1 {
		t.Fatalf("fallback calls = %d after router2, want no additional attempt", fallback.calls-1)
	}
}

// failingScriptedProvider fails the stream immediately (nothing streamed).
type failingScriptedProvider struct{ err error }

func (p *failingScriptedProvider) Chat(context.Context, ChatRequest) (*ChatResponse, error) {
	return nil, p.err
}
func (p *failingScriptedProvider) ChatStream(_ context.Context, _ ChatRequest, _ func(delta ChatDelta) error) error {
	return p.err
}
func (p *failingScriptedProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return nil, p.err
}

// partialStreamProvider streams content then fails mid-answer.
type partialStreamProvider struct{ failAfter string }

func (p *partialStreamProvider) Chat(context.Context, ChatRequest) (*ChatResponse, error) {
	return nil, errors.New("unused")
}
func (p *partialStreamProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return nil, errors.New("unused")
}
func (p *partialStreamProvider) ChatStream(_ context.Context, _ ChatRequest, handler func(delta ChatDelta) error) error {
	if err := handler(ChatDelta{Content: p.failAfter}); err != nil {
		return err
	}
	return errors.New("died mid answer")
}

// A registered ModelPref goes first and the configured chain follows; an
// unknown preference keeps the configured order.
func TestRoutingProviderModelPrefOrdering(t *testing.T) {
	primary := &scriptedProvider{scripts: [][]ChatDelta{{{Content: "a"}, {Done: true}}}}
	fallback := &scriptedProvider{scripts: [][]ChatDelta{{{Content: "b"}, {Done: true}}}}
	router := newTestRouter(primary, fallback, nil)

	_ = router.ChatStream(context.Background(), ChatRequest{ModelPref: "b"}, func(delta ChatDelta) error { return nil })
	if len(fallback.requests) != 1 || fallback.requests[0].ModelPref != "b" {
		t.Fatalf("pref request not routed to fallback provider first")
	}

	// Unknown pref → primary stays first, no error at this layer.
	_ = router.ChatStream(context.Background(), ChatRequest{ModelPref: "ghost"}, func(delta ChatDelta) error { return nil })
	if primary.calls != 1 || fallback.calls != 1 {
		t.Fatalf("unknown pref must keep chain order: primary %d fallback %d", primary.calls, fallback.calls)
	}
}

// Client cancellation never retries — the cancel propagates immediately.
func TestRoutingProviderCancelledContextNoRetry(t *testing.T) {
	cancelled := &cancelErrProvider{}
	fallback := &scriptedProvider{}
	router := NewRoutingProvider(
		map[string]LLMProvider{"a": cancelled, "b": fallback},
		[]string{"a", "b"},
		[]string{RetryOnProviderError, RetryOnBlankAnswer},
		nil,
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := router.ChatStream(ctx, ChatRequest{}, func(delta ChatDelta) error { return nil }); err == nil {
		t.Fatal("cancelled stream must return the cancellation error")
	}
	if fallback.calls != 0 {
		t.Fatalf("cancelled stream must not retry, fallback calls = %d", fallback.calls)
	}
}

type cancelErrProvider struct{}

func (p *cancelErrProvider) Chat(context.Context, ChatRequest) (*ChatResponse, error) {
	return nil, context.Canceled
}
func (p *cancelErrProvider) ChatStream(_ context.Context, _ ChatRequest, _ func(delta ChatDelta) error) error {
	return context.Canceled
}
func (p *cancelErrProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return nil, context.Canceled
}

// Non-streaming Chat follows the same rules: blank content retries, errors
// retry, and the last outcome wins when the chain is exhausted.
func TestRoutingProviderChatFailover(t *testing.T) {
	primary := &scriptedProvider{chatResp: &ChatResponse{Content: "   "}}
	fallback := &scriptedProvider{chatResp: &ChatResponse{Content: "real answer"}}
	router := newTestRouter(primary, fallback, []string{RetryOnBlankAnswer})

	resp, err := router.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "real answer" {
		t.Fatalf("content = %q, want fallback answer", resp.Content)
	}

	failing := &failingScriptedProvider{err: errors.New("down")}
	router2 := NewRoutingProvider(
		map[string]LLMProvider{"a": failing, "b": fallback},
		[]string{"a", "b"},
		[]string{RetryOnProviderError},
		nil,
	)
	if _, err := router2.Chat(context.Background(), ChatRequest{}); err != nil {
		t.Fatalf("Chat should fail over: %v", err)
	}
}

// routerToolCallDelta builds one streamed tool-call fragment (the anonymous
// Function struct cannot be written inline in a composite literal).
func routerToolCallDelta() ChatDelta {
	var fn struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}
	fn.Name = "search"
	fn.Arguments = `{"query":"q"}`
	return ChatDelta{ToolCalls: []ToolCall{{ID: "call_1", Type: "function", Function: fn}}}
}
