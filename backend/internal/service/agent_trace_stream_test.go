package service

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/observability/agenttrace"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/repository"
)

// traceTestDB provisions the trace tables on the stream test's sqlite
// database and exposes read helpers for contract assertions.
type traceTestDB struct {
	repo *repository.AgentTraceRepository
	db   *gorm.DB
}

func newTraceTestDB(t *testing.T, db *gorm.DB) *traceTestDB {
	t.Helper()
	// A separate sqlite handle would get a separate :memory: database; the
	// tables must live on the SAME connection the service writes through.
	if err := db.AutoMigrate(&model.AgentTraceRun{}, &model.AgentTraceNode{}); err != nil {
		t.Fatalf("trace tables: %v", err)
	}
	return &traceTestDB{repo: repository.NewAgentTraceRepository(db), db: db}
}

func (x *traceTestDB) nodesByType(t *testing.T, traceID string) map[string][]model.AgentTraceNode {
	t.Helper()
	nodes, err := x.repo.ListNodesByTrace(context.Background(), traceID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	byType := map[string][]model.AgentTraceNode{}
	for _, n := range nodes {
		byType[n.NodeType] = append(byType[n.NodeType], n)
	}
	return byType
}

func nodeTypeKeys(byType map[string][]model.AgentTraceNode) []string {
	keys := make([]string, 0, len(byType))
	for k := range byType {
		keys = append(keys, k)
	}
	return keys
}

// guard: sqlite driver stays referenced if the harness evolves.
var _ = sqlite.Open

// TestAgentStreamTraceRunAndNodesMatchSSETraceID pins the SP-21 T2 contract:
// the SSE events' trace_id is exactly the trace id persisted to
// agent_trace_runs, and one grounded tool turn produces the full node chain
// (llm rounds, the tool, citation revalidation, classify) plus a terminal
// SUCCESS run row with answer_kind and prompt attribution.
func TestAgentStreamTraceRunAndNodesMatchSSETraceID(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("get_content_detail", `{"content_id": 88}`)},
		{{Content: "About Published Test Content", Thinking: "hmm"}, {Content: " body"}, {Done: true, Usage: &llm.TokenUsage{PromptTokens: 120, CompletionTokens: 45}}},
	}}
	svc, db := newStreamTestService(t, provider, nil)

	// Real writer over the test DB: the async buffer is flushed by Stop.
	traceDB := newTraceTestDB(t, db)
	writer := agenttrace.NewWriter(traceDB.repo, agenttrace.Options{
		Enabled: true, SampleRatio: 1, ChannelSize: 128,
		FlushInterval: 50 * 1000 * 1000, FlushBatchSize: 50, DigestMaxRunes: 500, RetentionDays: 30,
	})
	writer.Start(context.Background())
	defer writer.Stop(context.Background())
	svc.SetTraceWriter(writer)

	var events []AgentStreamEvent
	err := svc.ChatStream(
		context.Background(),
		7,
		ChatTurnInput{Message: "what is this"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error { events = append(events, ev); return nil },
	)
	collectStreamEvents(t, err, &events)
	// Flush the async writer before asserting on persisted rows.
	writer.Stop(context.Background())

	// SSE side: start + done carry the same trace id.
	var sseTraceID string
	answerKind := ""
	for _, ev := range events {
		switch ev.Type {
		case AgentEventStart:
			sseTraceID = ev.TraceID
		case AgentEventDone:
			if ev.TraceID != sseTraceID {
				t.Fatalf("done trace_id %q != start trace_id %q", ev.TraceID, sseTraceID)
			}
			answerKind = string(ev.AnswerKind)
		}
	}
	if sseTraceID == "" {
		t.Fatal("SSE events carry no trace id")
	}

	// DB side: exactly that trace id, terminal SUCCESS, answer kind, TTFT.
	run, err := traceDB.repo.GetRunByTraceID(context.Background(), sseTraceID)
	if err != nil {
		t.Fatalf("no run row for the SSE trace id %q: %v", sseTraceID, err)
	}
	if run.Status != model.AgentTraceStatusSuccess {
		t.Fatalf("run status = %s, want SUCCESS", run.Status)
	}
	if run.AnswerKind != answerKind || run.AnswerKind != "grounded_content" {
		t.Fatalf("run answer_kind = %q (SSE said %q)", run.AnswerKind, answerKind)
	}
	if run.TTFTMs == nil {
		t.Fatal("run ttft_ms missing although a display delta was forwarded")
	}
	if run.PromptName != "agent_system" || run.PromptVersion == nil {
		t.Fatalf("run prompt attribution missing: %q %v", run.PromptName, run.PromptVersion)
	}

	// Node chain: llm rounds, the executed tool, revalidation, classify.
	nodes := traceDB.nodesByType(t, sseTraceID)
	for _, want := range []string{"llm_round", "tool", "citation_revalidation", "classify"} {
		if len(nodes[want]) == 0 {
			t.Fatalf("node chain missing %q nodes (have %v)", want, nodeTypeKeys(nodes))
		}
	}
	toolNode := nodes["tool"][0]
	if toolNode.ParentNodeKey == nil || !strings.HasPrefix(*toolNode.ParentNodeKey, "llm_round") {
		t.Fatalf("tool node not nested under its llm round: %+v", toolNode)
	}
	if toolNode.TokensIn != nil {
		t.Fatal("tool node should carry no token counts (they belong to llm rounds)")
	}
	if len(nodes["llm_round"]) >= 2 {
		last := nodes["llm_round"][len(nodes["llm_round"])-1]
		if last.TokensIn == nil || *last.TokensIn != 120 {
			t.Fatalf("final llm round tokens_in = %v, want 120", last.TokensIn)
		}
	}
}

// TestAgentStreamChitchatTurnTraced: the rule-layer shortcut still yields a
// run row (conversational) with its chitchat node — no LLM, no rounds.
func TestAgentStreamChitchatTurnTraced(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{{{Content: "unused", Done: true}}}}
	svc, db := newStreamTestService(t, provider, conversationalLaneTestConfig())
	traceDB := newTraceTestDB(t, db)
	writer := agenttrace.NewWriter(traceDB.repo, agenttrace.Options{Enabled: true, SampleRatio: 1, ChannelSize: 128, FlushBatchSize: 50, DigestMaxRunes: 500, RetentionDays: 30})
	writer.Start(context.Background())
	defer writer.Stop(context.Background())
	svc.SetTraceWriter(writer)

	var events []AgentStreamEvent
	err := svc.ChatStream(
		context.Background(),
		7,
		ChatTurnInput{Message: "你好"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error { events = append(events, ev); return nil },
	)
	collectStreamEvents(t, err, &events)
	writer.Stop(context.Background())

	sseTraceID := ""
	for _, ev := range events {
		if ev.Type == AgentEventDone {
			sseTraceID = ev.TraceID
		}
	}
	run, err := traceDB.repo.GetRunByTraceID(context.Background(), sseTraceID)
	if err != nil {
		t.Fatalf("chitchat turn left no run row: %v", err)
	}
	if run.AnswerKind != "conversational" {
		t.Fatalf("chitchat run answer_kind = %q", run.AnswerKind)
	}
	if nodes := traceDB.nodesByType(t, sseTraceID); len(nodes["chitchat"]) != 1 {
		t.Fatalf("chitchat node missing: %v", nodeTypeKeys(nodes))
	}
}
