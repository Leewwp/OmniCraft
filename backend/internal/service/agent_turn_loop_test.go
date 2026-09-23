package service

// #661 ToolRuntime 循环单测族：以 fake runtime + 脚本化流 provider 驱动
// turnRunner.run，不再需要 sqlite 种子与真实检索——循环策略（scope 透传、
// external 归属跟工具名、未知工具安全码、轮次终止）达到秒级单测。

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"omnicraft/backend/config"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
)

// scriptedStreamProvider replays fixed delta rounds regardless of the
// request shape; the loop terminates when a round emits no tool calls.
type scriptedStreamProvider struct {
	rounds [][]llm.ChatDelta
	next   int
}

func (p *scriptedStreamProvider) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: "ok"}, nil
}

func (p *scriptedStreamProvider) ChatStream(_ context.Context, _ llm.ChatRequest, handler func(delta llm.ChatDelta) error) error {
	i := p.next
	if i >= len(p.rounds) {
		i = len(p.rounds) - 1
	}
	p.next++
	for _, d := range p.rounds[i] {
		if err := handler(d); err != nil {
			return err
		}
	}
	return nil
}

func (p *scriptedStreamProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return nil, nil
}

// fakeLoopRuntime records executions and answers from a table.
type fakeLoopRuntime struct {
	calls     []ToolScope
	byName    map[string]struct{}
	responder func(name string, raw json.RawMessage) (*AgentToolOutcome, error)
}

func (f *fakeLoopRuntime) ToolDefinitions(context.Context) []llm.ToolDefinition { return nil }

func (f *fakeLoopRuntime) ExecuteTool(_ context.Context, name string, raw json.RawMessage, scope ToolScope) (*AgentToolOutcome, error) {
	f.calls = append(f.calls, scope)
	if f.byName != nil {
		f.byName[name] = struct{}{}
	}
	if f.responder == nil {
		return &AgentToolOutcome{}, nil
	}
	return f.responder(name, raw)
}

func newLoopRunner(t *testing.T, provider llm.LLMProvider, rt ToolRuntime, maxTools int) *turnRunner {
	t.Helper()
	cfg := &config.Config{Agent: config.AgentConfig{WebAgentEnabled: true, MaxToolCallsPerTurn: maxTools, CitationMaxCount: 5}}
	svc := NewAgentService(provider, nil, nil, nil, nil, cfg)
	svc.SetToolRuntime(rt)
	return svc.newTurnRunner(
		7, ChatTurnInput{Message: "loop"},
		&model.AgentConversation{ID: 9}, "loop-trace",
		time.Now(), "global", false, "first",
		nil, func(AgentStreamEvent) error { return nil })
}

func TestTurnRunnerLoopPassesScopeThroughRuntimeSeam(t *testing.T) {
	provider := &scriptedStreamProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("search_content", `{"query":"钢琴"}`)},
		{{Content: "done"}, {Done: true}},
	}}
	rt := &fakeLoopRuntime{}
	r := newLoopRunner(t, provider, rt, 8)
	r.run(context.Background(), &llm.ChatRequest{})

	require_len := len(rt.calls)
	if require_len != 1 {
		t.Fatalf("runtime executed %d calls, want 1", require_len)
	}
	scope := rt.calls[0]
	if scope.ViewerID != 7 || scope.ConversationID != 9 {
		t.Fatalf("scope = viewer %d conv %d, want 7/9 (turn-scoped passthrough)", scope.ViewerID, scope.ConversationID)
	}
	if len(r.executedTools) != 1 || r.executedTools[0].Name != "search_content" || r.executedTools[0].Status != AgentToolStatusSuccess {
		t.Fatalf("executed tools = %+v", r.executedTools)
	}
	if r.streamErr != nil {
		t.Fatalf("unexpected stream error: %v", r.streamErr)
	}
}

// #628 语义钉子：external 归属跟工具名而非 outcome——generate_image 首调
// 失败（outcome=nil）仍是外部步。
func TestTurnRunnerLoopAttributesExternalByToolName(t *testing.T) {
	provider := &scriptedStreamProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta(ToolGenerateImage, `{}`)},
		{{Content: "img failed"}, {Done: true}},
	}}
	rt := &fakeLoopRuntime{responder: func(string, json.RawMessage) (*AgentToolOutcome, error) {
		return nil, errors.New("boom")
	}}
	r := newLoopRunner(t, provider, rt, 8)
	r.run(context.Background(), &llm.ChatRequest{})

	if len(r.executedTools) != 1 {
		t.Fatalf("executed = %+v", r.executedTools)
	}
	if !r.executedTools[0].External {
		t.Fatal("generate_image failure must still be attributed external (name-based, #628)")
	}
	if r.executedTools[0].Status != AgentToolStatusError {
		t.Fatalf("status = %s, want error", r.executedTools[0].Status)
	}
}

func TestTurnRunnerLoopMapsUnknownToolToSafePlaceholder(t *testing.T) {
	provider := &scriptedStreamProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("no_such_tool", `{}`)},
		{{Content: "fallback"}, {Done: true}},
	}}
	rt := &fakeLoopRuntime{responder: func(string, json.RawMessage) (*AgentToolOutcome, error) {
		return nil, ErrAgentToolUnknown
	}}
	r := newLoopRunner(t, provider, rt, 8)
	r.run(context.Background(), &llm.ChatRequest{})

	if len(r.executedTools) != 1 || r.executedTools[0].Name != "(unknown)" {
		t.Fatalf("unknown tool must surface as (unknown), got %+v", r.executedTools)
	}
	if r.streamErr != nil {
		t.Fatalf("unknown tool is a model-contract degradation, not a stream error: %v", r.streamErr)
	}
}

// 轮次终止：AllowToolCall 在轮间生效——MaxToolCallsPerTurn=1 时第一轮的
// 一个调用执行后，第二轮的工具请求整轮不再触达 runtime，循环以已执行
// 步骤收束（answer 仍从后续内容正常终局）。
func TestTurnRunnerLoopStopsAtToolPolicyLimitBetweenRounds(t *testing.T) {
	provider := &scriptedStreamProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("search_content", `{}`)},
		{toolCallDelta("search_ips", `{}`)},
		{{Content: "final"}, {Done: true}},
	}}
	rt := &fakeLoopRuntime{}
	r := newLoopRunner(t, provider, rt, 1) // MaxToolCallsPerTurn=1：第二轮不再执行
	r.run(context.Background(), &llm.ChatRequest{})

	if len(rt.calls) != 1 {
		t.Fatalf("runtime calls = %d, want 1 (tool policy stops further rounds)", len(rt.calls))
	}
	if len(r.executedTools) != 1 {
		t.Fatalf("executed steps = %d, want 1", len(r.executedTools))
	}
	if r.streamErr != nil {
		t.Fatalf("streamErr=%v：policy 停止不是流错误，回合交 finalizeTurn 正常收口", r.streamErr)
	}
}
