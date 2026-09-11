package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
)

// SP-15 #434 D query-understanding suite. The model-side behavior (does M3
// actually emit self-contained, decomposed queries?) is verified by the
// real-stack probe rig frontend/sp15-d-verify.mjs; these tests lock the
// contract-level preconditions the directives rely on: the prompt carries the
// D1/D2 directives after the untouched must-search/A2 instructions, a
// follow-up turn actually receives the prior exchange, and the tool loop can
// run several distinct search_content queries in one turn.

// queryUnderstandingRetriever backs search_content in hybrid mode with one
// non-degraded candidate so scripted search rounds execute successfully.
type queryUnderstandingRetriever struct {
	queries []string
}

func (r *queryUnderstandingRetriever) Retrieve(_ context.Context, query string, _ int64) (AgentRetrievalResult, error) {
	r.queries = append(r.queries, query)
	return AgentRetrievalResult{Candidates: []AgentRetrievalCandidate{{
		// ChunkKey 必须匹配 newStreamTestService 种入的 RagChunk
		// （fmt.Sprintf("%064x", 88)），否则引用复验拒绝 → 答案被清空。
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
	}}}, nil
}

// TestQueryUnderstandingDirectivesInPrompt pins that the D1 self-contained
// rewrite and D2 decomposition directives are appended after the 2026-09-06
// must-search directive and the SP-15 A2 conversation-lane directive, whose
// wording must stay byte-identical (spec: 既有指令原文不动).
func TestQueryUnderstandingDirectivesInPrompt(t *testing.T) {
	svc, _ := newStreamTestService(t, &conversationalLaneProvider{}, conversationalLaneTestConfig())
	prompt := svc.serverOwnedSystemPrompt(model.AgentChatSurfaceGlobal, nil).Content

	mustSearch := "for any request to find, search, recommend, compare or summarize site content, you must call the cited_search tool first and ground the answer only in its results; never recommend or describe site content from your own knowledge"
	mustSearchIdx := strings.Index(prompt, mustSearch)
	if mustSearchIdx < 0 {
		t.Fatalf("must-search directive missing or reworded: %q", prompt)
	}
	a2Marker := "only for pure greetings, thanks, farewells"
	a2Idx := strings.Index(prompt, a2Marker)
	d1Marker := "every search_content query must be fully self-contained"
	d1Idx := strings.Index(prompt, d1Marker)
	d2Marker := "decompose it into multiple search_content calls"
	d2Idx := strings.Index(prompt, d2Marker)
	if a2Idx < 0 || d1Idx < 0 || d2Idx < 0 {
		t.Fatalf("directives missing (a2=%d d1=%d d2=%d): %q", a2Idx, d1Idx, d2Idx, prompt)
	}
	if !(mustSearchIdx < a2Idx && a2Idx < d1Idx && d1Idx < d2Idx) {
		t.Fatalf("directive order must be must-search < A2 < D1 < D2, got %d < %d < %d < %d", mustSearchIdx, a2Idx, d1Idx, d2Idx)
	}
	if !strings.Contains(prompt, "「《迟到的邮差》的作者的其他作品」") || !strings.Contains(prompt, "never a bare reference such as 「第二个」 or 「它的作者」") {
		t.Fatalf("D1 few-shot example incomplete: %q", prompt)
	}
	if !strings.Contains(prompt, "one call per sub-question, each with its own self-contained query") {
		t.Fatalf("D2 per-sub-question clause missing: %q", prompt)
	}
}

// TestMultiTurnTurnCarriesHistoryAndRewrittenQuery locks the D1 preconditions:
// the second turn's provider request includes the first exchange (so references
// CAN be resolved), and the search query the model emits is the query that
// reaches the tool execution contract (ArgsSummary) unchanged.
func TestMultiTurnTurnCarriesHistoryAndRewrittenQuery(t *testing.T) {
	rewritten := "《Published Test Content》的作者的其他作品"
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("search_content", `{"query":"配色练习 原创内容"}`)},
		{{Content: "第一个结果介绍 [1]。"}, {Done: true}},
		{toolCallDelta("search_content", `{"query":"`+rewritten+`"}`)},
		{{Content: "同一作者的另一件作品介绍 [1]。"}, {Done: true}},
	}}
	cfg := conversationalLaneTestConfig()
	cfg.Features.RAGHybridEnabled = true
	svc, _ := newStreamTestService(t, provider, cfg)
	retriever := &queryUnderstandingRetriever{}
	svc.hybridRetriever = retriever

	var turn2Done *AgentStreamEvent
	var convID int64
	runTurn := func(message string) {
		t.Helper()
		err := svc.ChatStream(context.Background(), 7, ChatTurnInput{ConversationID: convID, Message: message},
			resolveGlobalChatContext(t, svc, 7),
			func(ev AgentStreamEvent) error {
				if ev.Type == AgentEventStart {
					convID = ev.ConversationID
				}
				if ev.Type == AgentEventDone {
					turn2Done = &ev
				}
				return nil
			})
		if err != nil {
			t.Fatalf("ChatStream: %v", err)
		}
	}
	runTurn("站内有哪些关于配色练习的原创内容？")
	runTurn("第二个的作者还有什么作品")

	// History precondition: the last provider request of turn 2 still carries
	// the turn-1 user message and assistant answer, so the model can resolve
	// 「第二个」 into a concrete title.
	lastReqJSON, err := json.Marshal(provider.lastReq.Messages)
	if err != nil {
		t.Fatalf("marshal last request: %v", err)
	}
	if !strings.Contains(string(lastReqJSON), "站内有哪些关于配色练习的原创内容") || !strings.Contains(string(lastReqJSON), "第一个结果介绍") {
		t.Fatalf("turn-2 request misses the turn-1 exchange: %s", lastReqJSON)
	}

	// Tool contract: turn 2 executed the rewritten self-contained query, and it
	// is not a bare reference.
	if turn2Done == nil {
		t.Fatal("turn-2 done event missing")
	}
	var turn2Queries []string
	for _, tool := range turn2Done.Tools {
		if tool.Name == ToolSearchContent {
			turn2Queries = append(turn2Queries, tool.ArgsSummary)
		}
	}
	if len(turn2Queries) != 1 {
		t.Fatalf("turn-2 search_content executions = %v, want exactly the rewritten query", turn2Queries)
	}
	if !strings.Contains(turn2Queries[0], "Published Test Content") {
		t.Fatalf("turn-2 query %q must carry the resolved entity, not a bare reference", turn2Queries[0])
	}
	if got := retriever.queries; len(got) != 2 || got[1] != rewritten {
		t.Fatalf("retriever queries = %v, want the turn-2 rewritten query %q executed verbatim", got, rewritten)
	}
}

// TestCompositeQuestionRunsDistinctSearchQueries locks the D2 precondition:
// one round carrying two search_content calls with distinct queries executes
// both within the per-turn budget, each reported with its own query summary.
func TestCompositeQuestionRunsDistinctSearchQueries(t *testing.T) {
	twoCalls := llm.ChatDelta{ToolCalls: []llm.ToolCall{
		{ID: "call_1", Type: "function", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: ToolSearchContent, Arguments: `{"query":"水彩入门教程"}`}},
		{ID: "call_2", Type: "function", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: ToolSearchContent, Arguments: `{"query":"摄影后期调色技巧"}`}},
	}}
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{twoCalls},
		{{Content: "两个子问题分别回答 [1][2]。"}, {Done: true}},
	}}
	cfg := conversationalLaneTestConfig()
	cfg.Features.RAGHybridEnabled = true
	svc, _ := newStreamTestService(t, provider, cfg)
	retriever := &queryUnderstandingRetriever{}
	svc.hybridRetriever = retriever

	var doneEvent *AgentStreamEvent
	var toolEvents []*AgentToolExecution
	err := svc.ChatStream(context.Background(), 7, ChatTurnInput{Message: "分别找找水彩入门教程和摄影后期调色技巧"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error {
			if ev.Type == AgentEventToolStatus && ev.Tool != nil {
				toolEvents = append(toolEvents, ev.Tool)
			}
			if ev.Type == AgentEventDone {
				doneEvent = &ev
			}
			return nil
		})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	if len(toolEvents) != 2 {
		t.Fatalf("tool executions = %d, want 2 (one per sub-question)", len(toolEvents))
	}
	seen := map[string]bool{}
	for _, tool := range toolEvents {
		if tool.Name != ToolSearchContent || tool.Status != AgentToolStatusSuccess {
			t.Fatalf("unexpected tool execution %+v", tool)
		}
		seen[tool.ArgsSummary] = true
	}
	if !seen["水彩入门教程"] || !seen["摄影后期调色技巧"] {
		t.Fatalf("distinct sub-question queries not both executed: %v", seen)
	}
	if doneEvent == nil || len(doneEvent.Tools) != 2 {
		t.Fatalf("done tools must carry both sub-question searches: %+v", doneEvent)
	}
}
