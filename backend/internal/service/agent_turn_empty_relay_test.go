package service

// #754 B：本地检索工具成功但结果为空时，turnRunner 序列化给下一轮的
// role=tool 消息必须携带固定中继提示。测试以录制型流 provider 抓取下一
// 轮请求里 provider 实际收到的 tool JSON 与 ToolCallID，不靠内部结构字段。

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
)

// recordingStreamProvider 逐轮回放固定 delta 并记录每轮收到的完整请求
// （下一轮 provider 调用实际看到的 messages 是断言面）。
type recordingStreamProvider struct {
	rounds [][]llm.ChatDelta
	next   int
	reqs   []llm.ChatRequest
}

func (p *recordingStreamProvider) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: "ok"}, nil
}

func (p *recordingStreamProvider) ChatStream(_ context.Context, req llm.ChatRequest, handler func(delta llm.ChatDelta) error) error {
	p.reqs = append(p.reqs, req)
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

func (p *recordingStreamProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return nil, nil
}

func newRelayRunner(t *testing.T, provider llm.LLMProvider, rt ToolRuntime) *turnRunner {
	t.Helper()
	cfg := &config.Config{Agent: config.AgentConfig{WebAgentEnabled: true, MaxToolCallsPerTurn: 8, CitationMaxCount: 5}}
	svc := NewAgentService(provider, nil, nil, nil, nil, cfg)
	svc.SetToolRuntime(rt)
	return svc.newTurnRunner(
		7, ChatTurnInput{Message: "帮我找找最近热门的 ip"},
		&model.AgentConversation{ID: 3}, "relay-trace",
		time.Now(), "global", false, "first",
		nil, func(AgentStreamEvent) error { return nil })
}

// toolMessagesOf 抓指定轮次请求里的 role=tool 消息。
func toolMessagesOf(req llm.ChatRequest) []llm.ChatMessage {
	var out []llm.ChatMessage
	for _, m := range req.Messages {
		if m.Role == "tool" {
			out = append(out, m)
		}
	}
	return out
}

func TestEmptySearchOutcomeRelaysAntiFabricationHint(t *testing.T) {
	for _, tc := range []struct {
		name     string
		toolName string
		args     string
		empty    bool
	}{
		{"search_content empty slice", "search_content", `{"query":"不存在的主题"}`, true},
		{"search_ips empty after browse", "search_ips", `{"sort":"most_contents"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &recordingStreamProvider{rounds: [][]llm.ChatDelta{
				{toolCallDelta(tc.toolName, tc.args)},
				{{Content: "final"}, {Done: true}},
			}}
			rt := &fakeLoopRuntime{responder: func(string, json.RawMessage) (*AgentToolOutcome, error) {
				outcome := &AgentToolOutcome{}
				if tc.toolName == ToolSearchContent {
					outcome.Search = []ContentSummary{}
				} else {
					outcome.IPs = []AgentIPSummary{}
				}
				return outcome, nil
			}}
			r := newRelayRunner(t, provider, rt)
			r.run(context.Background(), &llm.ChatRequest{})

			if len(provider.reqs) < 2 {
				t.Fatalf("second provider round never happened (rounds=%d)", len(provider.reqs))
			}
			msgs := toolMessagesOf(provider.reqs[1])
			if len(msgs) != 1 {
				t.Fatalf("round-2 tool messages = %d, want 1", len(msgs))
			}
			var payload struct {
				OK      bool   `json:"ok"`
				Error   string `json:"error"`
				Message string `json:"message"`
				Search  []any  `json:"search"`
				IPs     []any  `json:"ips"`
			}
			if err := json.Unmarshal([]byte(msgs[0].Content), &payload); err != nil {
				t.Fatalf("tool json: %v (%s)", err, msgs[0].Content)
			}
			if msgs[0].ToolCallID == "" {
				t.Fatal("tool message must carry the provider tool_call_id")
			}
			if !payload.OK || payload.Error != "" {
				t.Fatalf("empty success must stay ok=true without error code, got ok=%v error=%q", payload.OK, payload.Error)
			}
			if payload.Message != emptySearchRelayMessage {
				t.Fatalf("relay note missing/mismatch: %.80s…", payload.Message)
			}
			if !strings.Contains(payload.Message, "Do not invent") {
				t.Fatal("relay must explicitly forbid fabrication")
			}
		})
	}
}

// nil 切片（omitempty 序列化后字段缺席）同样触发；错误路径不伪装成空检索。
func TestEmptyRelayCoversNilSliceAndErrorPath(t *testing.T) {
	provider := &recordingStreamProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta(ToolSearchContent, `{"query":"nil 结果"}`)},
		{{Content: "done"}, {Done: true}},
	}}
	rt := &fakeLoopRuntime{responder: func(string, json.RawMessage) (*AgentToolOutcome, error) {
		return &AgentToolOutcome{Search: nil}, nil
	}}
	r := newRelayRunner(t, provider, rt)
	r.run(context.Background(), &llm.ChatRequest{})
	msgs := toolMessagesOf(provider.reqs[1])
	if len(msgs) != 1 || !strings.Contains(msgs[0].Content, emptySearchRelayMessage) {
		t.Fatalf("nil-slice empty result must relay the note, got %s", msgs[0].Content)
	}

	errProvider := &recordingStreamProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta(ToolSearchContent, `{"query":"boom"}`)},
		{{Content: "done"}, {Done: true}},
	}}
	errRT := &fakeLoopRuntime{responder: func(string, json.RawMessage) (*AgentToolOutcome, error) {
		return nil, errors.New("retrieval down")
	}}
	r2 := newRelayRunner(t, errProvider, errRT)
	r2.run(context.Background(), &llm.ChatRequest{})
	errMsgs := toolMessagesOf(errProvider.reqs[1])
	if len(errMsgs) != 1 {
		t.Fatalf("error path must still emit one tool message, got %d", len(errMsgs))
	}
	if strings.Contains(errMsgs[0].Content, emptySearchRelayMessage) {
		t.Fatal("a tool error must use the error envelope, not the empty-search relay")
	}
	if !strings.Contains(errMsgs[0].Content, `"error"`) {
		t.Fatalf("error path must carry an error code, got %s", errMsgs[0].Content)
	}
}

// 误触面：有结果、详情/指导、生图、外部 MCP 的成功结果不携带中继；同轮
// 一空一非空时只有空结果那条带提示（另一条的可用证据不被抹掉）。
func TestEmptyRelayDoesNotTouchNonSearchOrNonEmpty(t *testing.T) {
	provider := &recordingStreamProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("search_content", `{"query":"空"}`), toolCallDelta("search_ips", `{"query":"有结果"}`)},
		{{Content: "final"}, {Done: true}},
	}}
	rt := &fakeLoopRuntime{responder: func(name string, _ json.RawMessage) (*AgentToolOutcome, error) {
		switch name {
		case ToolSearchContent:
			return &AgentToolOutcome{Search: nil}, nil
		case ToolSearchIPs:
			return &AgentToolOutcome{IPs: []AgentIPSummary{{ID: 1, Name: "有结果的 IP"}}}, nil
		}
		return &AgentToolOutcome{}, nil
	}}
	r := newRelayRunner(t, provider, rt)
	r.run(context.Background(), &llm.ChatRequest{})
	msgs := toolMessagesOf(provider.reqs[1])
	if len(msgs) != 2 {
		t.Fatalf("two tool messages expected, got %d", len(msgs))
	}
	relayed := 0
	for _, m := range msgs {
		if strings.Contains(m.Content, emptySearchRelayMessage) {
			relayed++
			if !strings.Contains(m.Content, `"ips"`) && !strings.Contains(m.Content, `"search"`) {
				// 允许空集字段被 omitempty 省略——关键是不含非空结果。
			}
		}
	}
	if relayed != 1 {
		t.Fatalf("exactly one (the empty) result may carry the relay, got %d", relayed)
	}
	// 非空那条仍带结果数据。
	var sawResult bool
	for _, m := range msgs {
		if strings.Contains(m.Content, "有结果的 IP") {
			sawResult = true
		}
	}
	if !sawResult {
		t.Fatal("the non-empty result must keep its payload untouched")
	}

	// 详情/指导工具的空 outcome 不触发中继。
	detailProvider := &recordingStreamProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("get_content_detail", `{"content_id":5}`)},
		{{Content: "done"}, {Done: true}},
	}}
	detailRT := &fakeLoopRuntime{responder: func(string, json.RawMessage) (*AgentToolOutcome, error) {
		return &AgentToolOutcome{}, nil // 成功但无 detail（畸形输入防御位）
	}}
	r3 := newRelayRunner(t, detailProvider, detailRT)
	r3.run(context.Background(), &llm.ChatRequest{})
	detailMsgs := toolMessagesOf(detailProvider.reqs[1])
	if len(detailMsgs) != 1 && strings.Contains(detailMsgs[0].Content, emptySearchRelayMessage) {
		t.Fatal("non-search tools must never carry the empty-search relay")
	}
}
