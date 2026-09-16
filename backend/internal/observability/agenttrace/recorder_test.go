package agenttrace

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"omnicraft/backend/internal/model"
)

func TestTurnRecorderNilSafety(t *testing.T) {
	var rec *TurnRecorder
	rec.RecordRouting("a", "b", "provider_error", errors.New("x"))
	if got := string(rec.RoutingEventsJSON()); got != "[]" {
		t.Fatalf("nil recorder routing json = %s", got)
	}
	rec.RecordRunStart(model.AgentTraceRun{})

	// nil recorder in context round-trips as nil.
	ctx := WithTurnRecorder(context.Background(), nil)
	if TurnRecorderFrom(ctx) != nil {
		t.Fatal("nil recorder must not attach")
	}
	if TurnRecorderFrom(context.Background()) != nil {
		t.Fatal("bare context has no recorder")
	}
}

func TestNewTurnRecorderGating(t *testing.T) {
	w := NewWriter(newFakeStore(), testOptions())
	if rec := NewTurnRecorder(w, ""); rec != nil {
		t.Fatal("empty trace id must not record")
	}
	if rec := NewTurnRecorder(nil, "abc"); rec != nil {
		t.Fatal("nil writer must not record")
	}
	disabled := NewWriter(newFakeStore(), Options{Enabled: false, SampleRatio: 1})
	if rec := NewTurnRecorder(disabled, "abc"); rec != nil {
		t.Fatal("disabled writer must not record")
	}
	if rec := NewTurnRecorder(w, "abc"); rec == nil {
		t.Fatal("sampled (ratio 1) turn must record")
	}
	never := func() Options { o := testOptions(); o.SampleRatio = 0; return o }()
	sampledOut := NewWriter(newFakeStore(), never)
	if rec := NewTurnRecorder(sampledOut, "abc"); rec != nil {
		t.Fatal("unsampled turn must not record")
	}
}

func TestTurnRecorderRunAndNodes(t *testing.T) {
	store := newFakeStore()
	w := NewWriter(store, testOptions())
	rec := NewTurnRecorder(w, "trace-rec-1")
	if rec == nil {
		t.Fatal("recorder nil")
	}
	if rec.TraceID() != "trace-rec-1" {
		t.Fatalf("trace id = %s", rec.TraceID())
	}

	started := time.Now().Add(-time.Second)
	rec.RecordRunStart(model.AgentTraceRun{
		StartedAt:  started,
		UserID:     int64Ptr(5),
		Surface:    "global",
		PromptName: "agent_system",
	})

	// A phase node with a nested child.
	tool := rec.StartNode(NodeTypeTool, "tool_cited_search_1", nil, "")
	chan1 := rec.StartNode(NodeTypeRetrieval, "retrieval_lexical_1", tool, "")
	chan1.End(NodeEndOptions{NodeName: "lexical"})
	chan2 := rec.StartNode(NodeTypeRetrieval, "retrieval_vector_1", tool, "")
	chan2.End(NodeEndOptions{Status: model.AgentTraceStatusError, ErrorCode: "TIMEOUT", ErrorMessage: "vector channel timed out"})
	tin, tout := int64(900), int64(120)
	tool.End(NodeEndOptions{TokensIn: &tin, TokensOut: &tout, CompletionDigest: "hits=12"})

	rec.RecordRouting("minimax-m3", "deepseek-chat", RetryReasonProviderError, errors.New("context deadline exceeded"))
	rec.RecordRouting("deepseek-chat", "minimax-m3", RetryReasonBlank, nil)

	msg := int64(77)
	firstDelta := started.Add(300 * time.Millisecond)
	rec.RecordRunEnd(RunEnd{
		Status:            model.AgentTraceStatusSuccess,
		StartedAt:         started,
		FirstDisplayDelta: firstDelta,
		AnswerKind:        "grounded_content",
		Model:             "minimax-m3",
		MessageID:         &msg,
	})

	w.finalFlush()

	run := store.runs["trace-rec-1"]
	if run.Status != model.AgentTraceStatusSuccess || run.AnswerKind != "grounded_content" {
		t.Fatalf("run = %+v", run)
	}
	if run.MessageID == nil || *run.MessageID != 77 {
		t.Fatal("run message_id missing")
	}
	if run.TTFTMs == nil || *run.TTFTMs < 250 {
		t.Fatalf("ttft_ms = %v, want ~300", run.TTFTMs)
	}
	if run.PromptName != "agent_system" || run.UserID == nil || *run.UserID != 5 {
		t.Fatalf("run start identity lost: %+v", run)
	}
	routing := string(run.RoutingEvents)
	if !strings.Contains(routing, `"reason":"provider_error"`) || !strings.Contains(routing, `"reason":"blank_answer"`) || !strings.Contains(routing, "context deadline exceeded") {
		t.Fatalf("routing events = %s", routing)
	}

	toolNode := store.nodes[nodeKey{trace: "trace-rec-1", key: "tool_cited_search_1"}]
	if toolNode.Status != model.AgentTraceStatusSuccess || toolNode.TokensIn == nil || *toolNode.TokensIn != 900 {
		t.Fatalf("tool node = %+v", toolNode)
	}
	if toolNode.Depth != 1 || toolNode.ParentNodeKey != nil {
		t.Fatalf("tool node tree fields wrong: %+v", toolNode)
	}
	child := store.nodes[nodeKey{trace: "trace-rec-1", key: "retrieval_lexical_1"}]
	if child.Depth != 2 || child.ParentNodeKey == nil || *child.ParentNodeKey != "tool_cited_search_1" {
		t.Fatalf("child node tree fields wrong: %+v", child)
	}
	failed := store.nodes[nodeKey{trace: "trace-rec-1", key: "retrieval_vector_1"}]
	if failed.Status != model.AgentTraceStatusError || failed.ErrorCode != "TIMEOUT" {
		t.Fatalf("failed channel = %+v", failed)
	}
}

func TestTurnRecorderContextPropagation(t *testing.T) {
	w := NewWriter(newFakeStore(), testOptions())
	rec := NewTurnRecorder(w, "trace-ctx")
	ctx := WithTurnRecorder(context.Background(), rec)

	got := TurnRecorderFrom(ctx)
	if got != rec {
		t.Fatal("recorder not propagated")
	}
	// Deep seams record via the context (this is the routing provider path).
	got.RecordRouting("a", "b", RetryReasonProviderError, nil)
	if len(got.routing) != 1 {
		t.Fatal("routing event not buffered")
	}
}
