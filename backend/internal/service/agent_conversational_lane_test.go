package service

import (
	"context"
	"strings"
	"testing"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
)

// SP-15 A4 conversational-lane behavior suite. These tests are deliberately
// independent of the frozen evaluation corpus (which stays untouched); they
// lock the externally observable turn outcomes of the intent layering.

// conversationalLaneProvider scripts stream rounds and counts every LLM
// entrypoint (streaming and the non-streaming auto-title/title path), so a
// zero-call assertion covers the whole shortcut contract.
type conversationalLaneProvider struct {
	rounds      [][]llm.ChatDelta
	streamCalls int
	chatCalls   int
}

func (p *conversationalLaneProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	p.chatCalls++
	return &llm.ChatResponse{}, nil
}

func (p *conversationalLaneProvider) ChatStream(_ context.Context, _ llm.ChatRequest, handler func(delta llm.ChatDelta) error) error {
	p.streamCalls++
	if p.streamCalls <= len(p.rounds) {
		for _, d := range p.rounds[p.streamCalls-1] {
			if err := handler(d); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *conversationalLaneProvider) GetEmbedding(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1}, nil
}

func conversationalLaneTestConfig() *config.Config {
	return &config.Config{
		Agent: config.AgentConfig{
			WebAgentEnabled: true, MaxToolCallsPerTurn: 8, CitationMaxCount: 5,
			MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, MaxOutputTokens: 1200,
			ChatContextTokenBudget: 100000,
			ChitchatShortcutEnabled: true,
			ChitchatPatterns: []string{"你好", "您好", "嗨", "哈喽", "hello", "hi", "hey", "在吗",
				"谢谢", "多谢", "感谢", "thanks", "thank you", "再见", "拜拜", "晚安"},
			ConversationalMaxRunes: 160,
		},
	}
}

type laneTurnResult struct {
	events    []AgentStreamEvent
	convID    int64
	doneEvent *AgentStreamEvent
}

func runLaneTurn(t *testing.T, svc *AgentService, userID int64, message string) laneTurnResult {
	t.Helper()
	var result laneTurnResult
	err := svc.ChatStream(context.Background(), userID, ChatTurnInput{Message: message},
		resolveGlobalChatContext(t, svc, userID),
		func(ev AgentStreamEvent) error {
			result.events = append(result.events, ev)
			if ev.Type == AgentEventStart {
				result.convID = ev.ConversationID
			}
			if ev.Type == AgentEventDone {
				result.doneEvent = &ev
			}
			return nil
		})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if result.doneEvent == nil {
		t.Fatal("done event missing")
	}
	return result
}

// TestChitchatKeywordShortCircuitsWithoutProvider locks SP-15 A1: an
// exact-match greeting replays the server template with zero LLM calls, both
// messages persist, and no auto-title is scheduled.
func TestChitchatKeywordShortCircuitsWithoutProvider(t *testing.T) {
	provider := &conversationalLaneProvider{}
	svc, db := newStreamTestService(t, provider, conversationalLaneTestConfig())

	result := runLaneTurn(t, svc, 7, "你好")

	if provider.streamCalls != 0 || provider.chatCalls != 0 {
		t.Fatalf("provider calls = (stream %d, chat %d), want (0, 0): the shortcut must not reach the LLM", provider.streamCalls, provider.chatCalls)
	}
	if result.doneEvent.AnswerKind != AgentAnswerConversational {
		t.Fatalf("done answer_kind = %q, want conversational", result.doneEvent.AnswerKind)
	}
	template := chitchatTemplates["greet"].zh
	if result.doneEvent.Answer != template {
		t.Fatalf("done answer = %q, want the zh greet template", result.doneEvent.Answer)
	}
	var deltas []string
	for _, ev := range result.events {
		if ev.Type == AgentEventDelta {
			deltas = append(deltas, ev.Delta)
		}
	}
	if len(deltas) != 1 || deltas[0] != template {
		t.Fatalf("deltas = %v, want exactly one carrying the whole template", deltas)
	}
	if len(result.doneEvent.Citations) != 0 || len(result.doneEvent.Tools) != 0 {
		t.Fatalf("shortcut done must carry no citations/tools: %v / %v", result.doneEvent.Citations, result.doneEvent.Tools)
	}
	if result.doneEvent.Usage == nil || result.doneEvent.Usage.PromptTokens != 0 || result.doneEvent.Usage.CompletionTokens != 0 {
		t.Fatalf("shortcut done usage = %+v, want zero values", result.doneEvent.Usage)
	}

	var messages []agentMessageRow
	if err := db.Raw("SELECT role, content FROM agent_messages WHERE conversation_id = ? ORDER BY id ASC", result.convID).Scan(&messages).Error; err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("stored messages = %d, want 2 (user + assistant template)", len(messages))
	}
	if messages[0].Role != "user" || messages[0].Content != "你好" {
		t.Fatalf("user row = %+v", messages[0])
	}
	if messages[1].Role != "assistant" || messages[1].Content != template {
		t.Fatalf("assistant row = %+v, want the template", messages[1])
	}

	var convRow struct{ Title *string }
	if err := db.Raw("SELECT title FROM agent_conversations WHERE id = ?", result.convID).Scan(&convRow).Error; err != nil {
		t.Fatalf("load conversation: %v", err)
	}
	if convRow.Title != nil && *convRow.Title != "" {
		t.Fatalf("conversation title = %q, want none scheduled for a chitchat turn", *convRow.Title)
	}
}

type agentMessageRow struct {
	Role    string
	Content string
}

// TestGarbledInputKeepsConversationalClarification locks SP-15 case ②: an
// unclear/garbled message whose model reply uses no tools stays visible as a
// conversational answer instead of the old no_evidence card.
func TestGarbledInputKeepsConversationalClarification(t *testing.T) {
	clarification := "抱歉，我不太确定你想找什么。你是想找站内的作品、IP，还是想了解用法？"
	provider := &conversationalLaneProvider{rounds: [][]llm.ChatDelta{
		{{Content: clarification}, {Done: true}},
	}}
	svc, db := newStreamTestService(t, provider, conversationalLaneTestConfig())

	result := runLaneTurn(t, svc, 7, "@#￥%……&*sdflk")

	if provider.streamCalls != 1 {
		t.Fatalf("provider stream calls = %d, want 1 (model layer reached)", provider.streamCalls)
	}
	if result.doneEvent.AnswerKind != AgentAnswerConversational {
		t.Fatalf("done answer_kind = %q, want conversational", result.doneEvent.AnswerKind)
	}
	if result.doneEvent.Answer != clarification {
		t.Fatalf("done answer = %q, want the clarification preserved", result.doneEvent.Answer)
	}
	var stored string
	if err := db.Raw("SELECT content FROM agent_messages WHERE conversation_id = ? AND role = 'assistant' ORDER BY id DESC LIMIT 1", result.convID).Scan(&stored).Error; err != nil {
		t.Fatalf("load stored answer: %v", err)
	}
	if stored != clarification {
		t.Fatalf("stored assistant answer = %q, want the clarification", stored)
	}
}

// TestLazyZeroToolLongAnswerCleared locks the length guardrail: a content
// question answered without any tool call beyond conversational_max_runes is
// still cleared as no_evidence (anti-hallucination semantics preserved).
func TestLazyZeroToolLongAnswerCleared(t *testing.T) {
	longAnswer := strings.Repeat("这是一段没有依据的回答。", 20) // 240 runes > 160
	provider := &conversationalLaneProvider{rounds: [][]llm.ChatDelta{
		{{Content: longAnswer}, {Done: true}},
	}}
	svc, db := newStreamTestService(t, provider, conversationalLaneTestConfig())

	result := runLaneTurn(t, svc, 7, "推荐几个站内作品")

	if result.doneEvent.AnswerKind != AgentAnswerNoEvidence {
		t.Fatalf("done answer_kind = %q, want no_evidence for an over-guardrail lazy answer", result.doneEvent.AnswerKind)
	}
	if result.doneEvent.Answer != "" {
		t.Fatalf("done answer = %.30q…, want cleared", result.doneEvent.Answer)
	}
	var stored string
	if err := db.Raw("SELECT content FROM agent_messages WHERE conversation_id = ? AND role = 'assistant' ORDER BY id DESC LIMIT 1", result.convID).Scan(&stored).Error; err != nil {
		t.Fatalf("load stored answer: %v", err)
	}
	if stored != "" {
		t.Fatalf("stored assistant answer = %.30q…, want cleared", stored)
	}
}

// TestGroundedRetrievalTurnUnchanged locks case ④: a normal retrieval turn
// keeps the grounded_content contract untouched by the new lane.
func TestGroundedRetrievalTurnUnchanged(t *testing.T) {
	provider := &conversationalLaneProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("get_content_detail", `{"content_id": 88}`)},
		{{Content: "基于检索的回答 [1]。"}, {Done: true}},
	}}
	svc, _ := newStreamTestService(t, provider, conversationalLaneTestConfig())

	result := runLaneTurn(t, svc, 7, "介绍一下测试内容")

	if result.doneEvent.AnswerKind != AgentAnswerGroundedContent {
		t.Fatalf("done answer_kind = %q, want grounded_content", result.doneEvent.AnswerKind)
	}
	if result.doneEvent.Answer == "" {
		t.Fatal("grounded answer must stay visible")
	}
	if len(result.doneEvent.Citations) != 1 || result.doneEvent.Citations[0].ContentID != 88 {
		t.Fatalf("done citations = %+v, want the revalidated content 88", result.doneEvent.Citations)
	}
}

// laneDegradedRetriever reports a degraded retrieval with no candidates, so
// the turn degrades with zero citations.
type laneDegradedRetriever struct{}

func (laneDegradedRetriever) Retrieve(_ context.Context, _ string, _ int64) (AgentRetrievalResult, error) {
	return AgentRetrievalResult{Degraded: "keyword_fallback"}, nil
}

// TestDegradedTurnStillCleared locks case ⑤: a short, zero-citation,
// degraded reply can never be rescued by the conversational lane.
func TestDegradedTurnStillCleared(t *testing.T) {
	provider := &conversationalLaneProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("search_content", `{"query":"anything"}`)},
		{{Content: "降级轮的短回答"}, {Done: true}},
	}}
	cfg := conversationalLaneTestConfig()
	cfg.Features.RAGHybridEnabled = true
	svc, _ := newStreamTestService(t, provider, cfg)
	svc.hybridRetriever = laneDegradedRetriever{}

	result := runLaneTurn(t, svc, 7, "找点东西")

	if !result.doneEvent.Degraded {
		t.Fatal("done degraded = false, want true")
	}
	if result.doneEvent.AnswerKind == AgentAnswerConversational || result.doneEvent.Answer != "" {
		t.Fatalf("degraded turn = (kind %q, answer %.20q…), want cleared and not conversational", result.doneEvent.AnswerKind, result.doneEvent.Answer)
	}
}

// TestNonExactGreetingReachesModelLayer locks case ⑥: a real request that
// merely starts with a greeting keyword is never short-circuited.
func TestNonExactGreetingReachesModelLayer(t *testing.T) {
	reply := "好的，你想找哪类作品？可以告诉我题材或作者。"
	provider := &conversationalLaneProvider{rounds: [][]llm.ChatDelta{
		{{Content: reply}, {Done: true}},
	}}
	svc, _ := newStreamTestService(t, provider, conversationalLaneTestConfig())

	result := runLaneTurn(t, svc, 7, "你好，帮我找点东西")

	if provider.streamCalls != 1 {
		t.Fatalf("provider stream calls = %d, want 1 (non-exact greeting must reach the model)", provider.streamCalls)
	}
	if result.doneEvent.Answer != reply {
		t.Fatalf("done answer = %q, want the provider reply, not a template", result.doneEvent.Answer)
	}
	if result.doneEvent.AnswerKind != AgentAnswerConversational {
		t.Fatalf("done answer_kind = %q, want conversational for a short zero-tool reply", result.doneEvent.AnswerKind)
	}
}

// TestChitchatShortcutMatchingRules unit-locks the exact-match normalization:
// trim, case fold, full-width fold, whitespace collapse; everything else —
// substrings, longer messages, appended punctuation — reaches the model layer.
func TestChitchatShortcutMatchingRules(t *testing.T) {
	cfg := conversationalLaneTestConfig()
	svc, _ := newStreamTestService(t, &conversationalLaneProvider{}, cfg)

	for _, tc := range []struct {
		message string
		want    bool
	}{
		{"你好", true},
		{"  你好  ", true},
		{"HELLO", true},
		{"  Thank  You  ", true},
		{"ｈｅｙ", true},  // full-width fold
		{"你好呀", false}, // substring never matches
		{"你好，帮我找点东西", false},
		{"hello!", false}, // trailing punctuation is not part of the table
		{"早上好呀", false},
		{"", false},
	} {
		_, ok := svc.chitchatShortcutReply(tc.message)
		if ok != tc.want {
			t.Fatalf("chitchatShortcutReply(%q) = %v, want %v", tc.message, ok, tc.want)
		}
	}

	cfg.Agent.ChitchatShortcutEnabled = false
	if _, ok := svc.chitchatShortcutReply("你好"); ok {
		t.Fatal("shortcut must be disabled by config")
	}
	cfg.Agent.ChitchatShortcutEnabled = true
	cfg.Agent.ChitchatPatterns = nil
	if _, ok := svc.chitchatShortcutReply("你好"); ok {
		t.Fatal("empty pattern table disables the shortcut")
	}

	// Language selection: CJK input gets the zh template, ASCII gets en.
	cfg.Agent.ChitchatPatterns = []string{"hello", "你好"}
	svc, _ = newStreamTestService(t, &conversationalLaneProvider{}, cfg)
	if reply, ok := svc.chitchatShortcutReply("hello"); !ok || reply != chitchatTemplates["greet"].en {
		t.Fatalf("ASCII shortcut reply = (%q, %v), want en template", reply, ok)
	}
	if reply, ok := svc.chitchatShortcutReply("你好"); !ok || reply != chitchatTemplates["greet"].zh {
		t.Fatalf("CJK shortcut reply = (%q, %v), want zh template", reply, ok)
	}
}

// TestConversationalLaneSystemPromptDirective pins that the A2 instruction is
// appended after the 2026-09-06 must-search directive, whose wording must stay
// byte-identical (spec: 既有指令原文不动).
func TestConversationalLaneSystemPromptDirective(t *testing.T) {
	svc, _ := newStreamTestService(t, &conversationalLaneProvider{}, conversationalLaneTestConfig())
	prompt := svc.serverOwnedSystemPrompt(model.AgentChatSurfaceGlobal, nil).Content

	mustSearch := "for any request to find, search, recommend, compare or summarize site content, you must call the cited_search tool first and ground the answer only in its results; never recommend or describe site content from your own knowledge"
	if idx := strings.Index(prompt, mustSearch); idx < 0 {
		t.Fatalf("must-search directive missing or reworded: %q", prompt)
	} else if conversationalIdx := strings.Index(prompt, "for pure greetings, thanks, farewells"); conversationalIdx < idx {
		t.Fatal("conversational directive must come after the must-search directive")
	}
	if !strings.Contains(prompt, "no searchable text at all") || !strings.Contains(prompt, "one clarifying question") {
		t.Fatalf("conversational directive incomplete: %q", prompt)
	}
	if !strings.Contains(prompt, "always call the cited_search tool with it before replying, even if the intent seems ambiguous") {
		t.Fatalf("gate tightening missing (bare titles/quotes must search first): %q", prompt)
	}
}
