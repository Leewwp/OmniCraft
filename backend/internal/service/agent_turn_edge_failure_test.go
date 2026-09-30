package service

// #673 回答回合边缘失败语义入档（特征测试族）：SSE 写端回调在四种形态
// 下出错——tool_status / citation / usage / chitchat delta——的当前语义是
// PR #666 的显性边缘行为修正（非「纯重构零行为」的例外面）：
//  1. 部分答案经 persistPartialTurn 仅落一次（A-01 保留语义）；
//  2. 角标以 0 引用数清理——keptCitationNumbers=nil 剥离全部 [n] 角标，
//     部分行不保存 citation payload（保留无目标角标没有可用引用）；
//  3. 错误原样返回（ErrorIs 写端错误）；
//  4. 终局 trace 终行 + SLA metrics 照发（metrics 与 RecordRunEnd 同址，
//     由 agent_turn_guard_test.go 钉住），终局 error 事件照发（safe code）。
// pre-#661 对照：citation/usage 写端失败旧路不写终局 trace/metrics 且以
// keptCitations=len(citations) 落部分行（角标保留）——旧/新差异由本族
// 测试钉死为新契约；若回归另开实现票（票面默认裁决）。
// 测试不依赖真实 provider：streamToolProvider 驱动，sqlite 内存断言。

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/observability/agenttrace"
	"omnicraft/backend/internal/pkg/llm"
)

// errEdgeWrite 是写端回调失败的哨兵错误（ErrorIs 断言用）。
var errEdgeWrite = errors.New("sse write endpoint failed")

// failOnceHandler 收集全部事件，但仅对首个目标类型事件返回写端错误
// ——终局 error 事件（不同类型）得以发出，可被断言「终局事件尝试」。
type failOnceHandler struct {
	target AgentStreamEventType
	failed bool
	events []AgentStreamEvent
}

func (h *failOnceHandler) handle(ev AgentStreamEvent) error {
	h.events = append(h.events, ev)
	if !h.failed && ev.Type == h.target {
		h.failed = true
		return errEdgeWrite
	}
	return nil
}

// edgeCitationCandidate 与 newStreamTestService 的种子（content 88 +
// version 1 + chunk %064x 88 / index 0）精确对齐，保证复验通过、回合
// 走进 citation/usage 事件。
func edgeCitationCandidate() AgentRetrievalCandidate {
	return AgentRetrievalCandidate{
		ChunkKey:        fmt.Sprintf("%064x", 88),
		ContentID:       88,
		ContentVersion:  1,
		ChunkIndex:      0,
		ChunkingVersion: 1,
		IndexVersion:    1,
		Title:           "Published Test Content",
		Text:            "Published Test Content",
		Source:          "hybrid_rrf",
		Zone:            "fanwork",
		ContentType:     "mod",
	}
}

// newEdgeFailureStack 在流测试栈上挂真 traceWriter（fake store 捕获终行）
// 并启用 hybrid 检索面（与 FT-5 编号测试同构）。
func newEdgeFailureStack(t *testing.T, provider llm.LLMProvider, retriever AgentContentRetriever) (*AgentService, *fakeTraceStore) {
	t.Helper()
	cfg := &config.Config{
		Features: config.FeaturesConfig{RAGHybridEnabled: true},
		Agent: config.AgentConfig{
			WebAgentEnabled: true, MaxToolCallsPerTurn: 6, CitationMaxCount: 5,
			MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, MaxOutputTokens: 1200,
			ChatContextTokenBudget: 100000,
		},
	}
	svc, _ := newStreamTestService(t, provider, cfg)
	store := &fakeTraceStore{}
	writer := agenttrace.NewWriter(store, agenttrace.Options{
		Enabled: true, SampleRatio: 1, ChannelSize: 64,
		FlushInterval: time.Millisecond, FlushBatchSize: 16,
	})
	writer.Start(context.Background())
	t.Cleanup(func() { writer.Stop(context.Background()) })
	svc.traceWriter = writer
	svc.hybridRetriever = retriever
	return svc, store
}

// edgeFailureAssertions 是四形态共用的终局语义断言（#673 入档契约）：
// 错误返回 + 终局 error 事件尝试（safe code + provider 降级标记）+
// 终局 trace 终行（error 状态 + safe 错误码）+ 无 done 事件。
func edgeFailureAssertions(t *testing.T, err error, h *failOnceHandler, store *fakeTraceStore) {
	t.Helper()
	require.ErrorIs(t, err, errEdgeWrite)
	require.True(t, h.failed, "handler must have failed exactly on the target event")

	terminal := h.events[len(h.events)-1]
	require.Equal(t, AgentEventError, terminal.Type, "terminal event must be attempted")
	require.Equal(t, AgentErrorCodeProvider, terminal.ErrorCode)
	require.True(t, terminal.Degraded)
	require.Equal(t, "provider_error", terminal.DegradedReason)
	for _, ev := range h.events {
		require.NotEqual(t, AgentEventDone, ev.Type, "failed write path must never emit done")
	}

	run := store.awaitRunEnd(t)
	require.Equal(t, model.AgentTraceStatusError, run.Status)
	require.Equal(t, safeAgentStreamCode(errEdgeWrite), run.ErrorCode)
}

// assistantRowsOf 提取该会话已落库的 assistant 行（部分答案仅落一次的
// 断言面；conversation start 事件携带会话 id）。
func assistantRowsOf(t *testing.T, svc *AgentService, h *failOnceHandler) []model.AgentMessage {
	t.Helper()
	require.NotEmpty(t, h.events)
	convID := h.events[0].ConversationID
	require.NotZero(t, convID)
	var rows []model.AgentMessage
	require.NoError(t, svc.db.Where("conversation_id = ? AND role = ?", convID, "assistant").Order("id").Find(&rows).Error)
	return rows
}

// TestToolStatusWriteFailureKeepsPartialOnceAndFinalizes：工具步事件写
// 端失败——已流出的部分答案（含角标）仅落一次、角标被清零剥离、终局
// 四件套齐发。
func TestToolStatusWriteFailureKeepsPartialOnceAndFinalizes(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{{Content: "部分回答 [1]"}, toolCallDelta(ToolSearchContent, `{"query":"q"}`)},
		{{Content: "正文不可达"}, {Done: true}}, // 循环在 tool_status 失败后中止
	}}
	retriever := &queuedRetriever{results: []AgentRetrievalResult{
		{Candidates: []AgentRetrievalCandidate{edgeCitationCandidate()}},
	}}
	svc, store := newEdgeFailureStack(t, provider, retriever)
	h := &failOnceHandler{target: AgentEventToolStatus}

	err := svc.ChatStream(context.Background(), 3, ChatTurnInput{Message: "find"},
		&ResolvedChatContext{Surface: model.AgentChatSurfaceSearch}, h.handle)
	edgeFailureAssertions(t, err, h, store)

	rows := assistantRowsOf(t, svc, h)
	require.Len(t, rows, 1, "partial answer must persist exactly once")
	require.Equal(t, "部分回答 ", *rows[0].Content, "citation markers are stripped with a zero kept set")
	require.Empty(t, rows[0].Citations, "partial row carries no citation payload")
}

// TestCitationWriteFailureKeepsPartialOnceAndFinalizes：引用卡事件写端
// 失败（pre-#661 旧语义对照点：旧路不写终局 trace/metrics 且保留角标）。
func TestCitationWriteFailureKeepsPartialOnceAndFinalizes(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta(ToolSearchContent, `{"query":"q"}`)},
		{{Content: "答案 [1]"}, {Done: true}},
	}}
	retriever := &queuedRetriever{results: []AgentRetrievalResult{
		{Candidates: []AgentRetrievalCandidate{edgeCitationCandidate()}},
	}}
	svc, store := newEdgeFailureStack(t, provider, retriever)
	h := &failOnceHandler{target: AgentEventCitation}

	err := svc.ChatStream(context.Background(), 3, ChatTurnInput{Message: "find"},
		&ResolvedChatContext{Surface: model.AgentChatSurfaceSearch}, h.handle)
	edgeFailureAssertions(t, err, h, store)

	rows := assistantRowsOf(t, svc, h)
	require.Len(t, rows, 1, "partial answer must persist exactly once")
	require.Equal(t, "答案 ", *rows[0].Content, "citation markers are stripped with a zero kept set")
	require.Empty(t, rows[0].Citations)
}

// TestUsageWriteFailureKeepsPartialOnceAndFinalizes：用量事件写端失败
// ——引用事件已成功发出后失败，语义与 citation 形态一致。
func TestUsageWriteFailureKeepsPartialOnceAndFinalizes(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta(ToolSearchContent, `{"query":"q"}`)},
		{{Content: "答案 [1]"}, {Done: true}},
	}}
	retriever := &queuedRetriever{results: []AgentRetrievalResult{
		{Candidates: []AgentRetrievalCandidate{edgeCitationCandidate()}},
	}}
	svc, store := newEdgeFailureStack(t, provider, retriever)
	h := &failOnceHandler{target: AgentEventUsage}

	err := svc.ChatStream(context.Background(), 3, ChatTurnInput{Message: "find"},
		&ResolvedChatContext{Surface: model.AgentChatSurfaceSearch}, h.handle)
	edgeFailureAssertions(t, err, h, store)

	rows := assistantRowsOf(t, svc, h)
	require.Len(t, rows, 1, "partial answer must persist exactly once")
	require.Equal(t, "答案 ", *rows[0].Content, "citation markers are stripped with a zero kept set")
	require.Empty(t, rows[0].Citations)
}

// TestChitchatDeltaWriteFailureFinalizesWithoutTemplateRow：规则短路模板
// 的 delta 写端失败——模板从未进入 answerBuf，失败终局不落任何 assistant
// 行（成功路径才由 finalizeShortcut 落模板行），终局四件套照发。
func TestChitchatDeltaWriteFailureFinalizesWithoutTemplateRow(t *testing.T) {
	provider := &streamToolProvider{} // 短路不触 provider
	svc, store := newEdgeFailureStack(t, provider, &queuedRetriever{})
	// 规则短路门控是配置面：显式开开关并注册「你好」pattern。
	svc.cfg.Agent.ChitchatShortcutEnabled = true
	svc.cfg.Agent.ChitchatPatterns = []string{"你好"}
	h := &failOnceHandler{target: AgentEventDelta}

	err := svc.ChatStream(context.Background(), 3, ChatTurnInput{Message: "你好"},
		&ResolvedChatContext{Surface: model.AgentChatSurfaceGlobal}, h.handle)
	edgeFailureAssertions(t, err, h, store)

	rows := assistantRowsOf(t, svc, h)
	require.Empty(t, rows, "chitchat template must not persist on the failed write path")
}
