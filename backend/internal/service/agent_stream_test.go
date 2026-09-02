package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
)

// streamToolProvider simulates a provider that may emit content deltas and/or
// assistant tool calls across multiple ChatStream rounds. Cancellation and raw
// provider errors are recorded so tests can assert safe-outcome behavior.
type streamToolProvider struct {
	rounds    [][]llm.ChatDelta
	streamErr error
	calls     int
	lastReq   llm.ChatRequest
	ctxSeen   context.Context
}

func (p *streamToolProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{}, nil
}

func (p *streamToolProvider) ChatStream(ctx context.Context, req llm.ChatRequest, handler func(delta llm.ChatDelta) error) error {
	p.calls++
	p.lastReq = req
	p.ctxSeen = ctx
	if p.streamErr != nil {
		return p.streamErr
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if p.calls <= len(p.rounds) {
		for _, d := range p.rounds[p.calls-1] {
			if err := handler(d); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *streamToolProvider) GetEmbedding(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1}, nil
}

func toolCallDelta(name, argsJSON string) llm.ChatDelta {
	return llm.ChatDelta{
		ToolCalls: []llm.ToolCall{{
			ID:   "call_1",
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: name, Arguments: argsJSON},
		}},
	}
}

func newStreamTestService(t *testing.T, provider llm.LLMProvider, cfg *config.Config) (*AgentService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	// :memory: databases are per-connection; pinning one connection keeps the
	// schema visible to the async auto-title goroutine and follow-up turns.
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&model.AgentConversation{}, &model.AgentMessage{}, &model.User{}, &model.ContentItem{}, &model.ContentVersion{}, &model.RagChunk{}, &model.IndexProjectionStatus{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&model.User{ID: 1, Username: "author", Email: "author@example.com"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	now := time.Now()
	if err := db.Create(&model.ContentItem{ID: 88, Title: "Published Test Content", AuthorID: 1, Zone: "fanwork", ContentType: "mod", Status: "published", IsPublic: true, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed content: %v", err)
	}
	if err := db.Create(&model.ContentVersion{ID: 880, ContentItemID: 88, AuthorID: 1, VersionNumber: 1, StorageType: "full", StorageKey: "stream-test", Status: "active", IsLatest: true}).Error; err != nil {
		t.Fatalf("seed content version: %v", err)
	}
	if err := db.Create(&model.RagChunk{ContentID: 88, ContentVersion: 1, ChunkIndex: 0, ChunkKey: fmt.Sprintf("%064x", 88), ChunkingVersion: 1, Heading: "", Text: "Published Test Content", SourceStart: 0, SourceEnd: 23, Zone: "fanwork", ContentType: "mod", IndexVersion: 1}).Error; err != nil {
		t.Fatalf("seed rag chunk: %v", err)
	}
	if err := db.Create(&model.IndexProjectionStatus{ContentID: 88, IndexVersion: 1, ChunkingVersion: 1, EmbeddingModel: "test", State: "ready", IsCurrent: true, ErrorSummary: ""}).Error; err != nil {
		t.Fatalf("seed projection status: %v", err)
	}
	if cfg == nil {
		cfg = &config.Config{Agent: config.AgentConfig{WebAgentEnabled: true, MaxToolCallsPerTurn: 8, CitationMaxCount: 5, MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, MaxOutputTokens: 1200, ChatContextTokenBudget: 100000}}
	}
	return NewAgentService(provider, nil, nil, nil, db, cfg), db
}

func resolveGlobalChatContext(t *testing.T, svc *AgentService, viewerID int64) *ResolvedChatContext {
	t.Helper()
	resolved, err := svc.ResolveChatContext(context.Background(), viewerID, &model.AgentChatContext{Surface: model.AgentChatSurfaceGlobal})
	if err != nil {
		t.Fatalf("ResolveChatContext: %v", err)
	}
	return resolved
}

func collectStreamEvents(t *testing.T, err error, events *[]AgentStreamEvent) {
	t.Helper()
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if len(*events) == 0 {
		t.Fatal("ChatStream produced no events")
	}
}

func TestAgentStreamEmitsTypedEventsAndDoneContract(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("get_content_detail", `{"content_id":88}`)},
		{{Content: "The answer"}, {Done: true}},
	}}
	svc, _ := newStreamTestService(t, provider, nil)

	var events []AgentStreamEvent
	err := svc.ChatStream(
		context.Background(),
		7,
		ChatTurnInput{Message: "hi"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error { events = append(events, ev); return nil },
	)
	collectStreamEvents(t, err, &events)

	if events[0].Type != AgentEventStart {
		t.Fatalf("first event type = %s, want start; all = %#v", events[0].Type, events)
	}
	if events[0].TraceID == "" || len(events[0].TraceID) != 32 {
		t.Fatalf("start trace_id = %q, want 32-hex trace id", events[0].TraceID)
	}

	gotDeltas := ""
	gotUsage, gotDone := false, false
	for _, ev := range events {
		switch ev.Type {
		case AgentEventDelta:
			gotDeltas += ev.Delta
		case AgentEventUsage:
			gotUsage = true
			if ev.Usage == nil {
				t.Fatal("usage event must carry a typed usage payload")
			}
		case AgentEventDone:
			gotDone = true
			if ev.TraceID == "" || len(ev.TraceID) != 32 {
				t.Fatalf("done trace_id = %q, want trace id", ev.TraceID)
			}
			if ev.Degraded {
				t.Fatal("done degraded = true, want false on healthy stream")
			}
			if ev.Answer != "The answer" {
				t.Fatalf("done answer = %q, want accumulated deltas", ev.Answer)
			}
		}
	}
	if gotDeltas != "The answer" {
		t.Fatalf("delta text = %q, want provider content", gotDeltas)
	}
	if !gotUsage || !gotDone {
		t.Fatalf("missing usage(%v)/done(%v) events in %#v", gotUsage, gotDone, events)
	}
	if events[len(events)-1].Type != AgentEventDone {
		t.Fatalf("last event = %s, want done", events[len(events)-1].Type)
	}
}

func TestAgentStreamModelToolIDsVisibilityCheckedAfterProviderCall(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("get_content_detail", `{"content_id": 101}`)},
		{{Content: "done now"}, {Done: true}},
	}}
	svc, _ := newStreamTestService(t, provider, nil)

	var events []AgentStreamEvent
	err := svc.ChatStream(
		context.Background(),
		7,
		ChatTurnInput{Message: "find it"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error { events = append(events, ev); return nil },
	)
	collectStreamEvents(t, err, &events)

	if provider.calls != 2 {
		t.Fatalf("provider ChatStream calls = %d, want 2 (first round for tool call, second for answer)", provider.calls)
	}
	var toolSeen, citationSeen, deltaSeen bool
	doneKind := ""
	doneAnswer := ""
	for _, ev := range events {
		switch ev.Type {
		case AgentEventDelta:
			deltaSeen = true
		case AgentEventToolStatus:
			toolSeen = true
			if ev.Tool == nil {
				t.Fatal("tool_status event must carry typed tool execution")
			}
			if ev.Tool.Status != AgentToolStatusError {
				t.Fatalf("tool status = %s, want error for forbidden content id", ev.Tool.Status)
			}
		case AgentEventCitation:
			citationSeen = true
			t.Fatalf("forbidden tool id must not produce a citation event")
		case AgentEventDone:
			doneKind = string(ev.AnswerKind)
			doneAnswer = ev.Answer
		}
	}
	if !toolSeen {
		t.Fatal("tool_status event missing for model tool call")
	}
	if citationSeen {
		t.Fatal("citations must not be emitted for forbidden content")
	}
	if deltaSeen {
		t.Fatal("no-evidence answer must not emit model deltas")
	}
	if doneKind != "no_evidence" {
		t.Fatalf("done answer_kind = %q, want no_evidence after forbidden tool result", doneKind)
	}
	if doneAnswer != "" {
		t.Fatalf("done answer = %q, want no-evidence answer to stay empty", doneAnswer)
	}

	var messages []model.AgentMessage
	if err := svc.db.Where("role = ?", "assistant").Find(&messages).Error; err != nil {
		t.Fatalf("load persisted assistant messages: %v", err)
	}
	if len(messages) != 1 || messages[0].Content == nil || *messages[0].Content != "" {
		t.Fatalf("persisted assistant messages = %#v, want one empty no-evidence message", messages)
	}
}

func TestAgentStreamVisibleToolProducesCitationAndGroundedAnswer(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("get_content_detail", `{"content_id": 88}`)},
		{{Content: "About Published Test Content"}, {Done: true}},
	}}
	svc, _ := newStreamTestService(t, provider, nil)

	var events []AgentStreamEvent
	err := svc.ChatStream(
		context.Background(),
		7,
		ChatTurnInput{Message: "what is this"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error { events = append(events, ev); return nil },
	)
	collectStreamEvents(t, err, &events)

	citations, kind := 0, ""
	for _, ev := range events {
		switch ev.Type {
		case AgentEventCitation:
			citations++
			if ev.Citation == nil || ev.Citation.ContentID != 88 || ev.Citation.Title == "" || ev.Citation.Zone == "" {
				t.Fatalf("citation event payload = %#v, want server-owned id/title/zone", ev.Citation)
			}
		case AgentEventDone:
			kind = string(ev.AnswerKind)
			if len(ev.Citations) != 1 || ev.Citations[0].ContentID != 88 {
				t.Fatalf("done citations = %#v, want revalidated citation for content 88", ev.Citations)
			}
		}
	}
	if citations != 1 {
		t.Fatalf("citation events = %d, want 1", citations)
	}
	if kind != "grounded_content" {
		t.Fatalf("done answer_kind = %q, want grounded_content", kind)
	}
}

func TestAgentStreamToolCallLimitStopsLoopWithStableDone(t *testing.T) {
	cfg := &config.Config{Agent: config.AgentConfig{WebAgentEnabled: true, MaxToolCallsPerTurn: 1, CitationMaxCount: 5, MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, MaxOutputTokens: 1200, ChatContextTokenBudget: 100000}}
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("get_content_detail", `{"content_id": 88}`)},
		{toolCallDelta("get_content_detail", `{"content_id": 88}`)},
		{toolCallDelta("get_content_detail", `{"content_id": 88}`)},
	}}
	svc, _ := newStreamTestService(t, provider, cfg)

	var events []AgentStreamEvent
	err := svc.ChatStream(
		context.Background(),
		7,
		ChatTurnInput{Message: "loop"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error { events = append(events, ev); return nil },
	)
	collectStreamEvents(t, err, &events)

	toolExecutions := 0
	last := events[len(events)-1]
	for _, ev := range events {
		if ev.Type == AgentEventToolStatus {
			toolExecutions++
		}
	}
	if toolExecutions > 1 {
		t.Fatalf("tool executions = %d, want stopped at config limit 1", toolExecutions)
	}
	if last.Type != AgentEventDone {
		t.Fatalf("last event = %s, want stable done event after tool limit", last.Type)
	}
}

func TestAgentStreamProviderErrorEmitsSafeErrorEvent(t *testing.T) {
	provider := &streamToolProvider{streamErr: errors.New("RAW PROVIDER SECRET detail")}
	svc, _ := newStreamTestService(t, provider, nil)

	var events []AgentStreamEvent
	streamErr := svc.ChatStream(
		context.Background(),
		7,
		ChatTurnInput{Message: "hi"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error { events = append(events, ev); return nil },
	)
	if streamErr == nil {
		t.Fatal("ChatStream must return the provider error")
	}
	var errorEvent *AgentStreamEvent
	for i := range events {
		if events[i].Type == AgentEventError {
			errorEvent = &events[i]
		}
	}
	if errorEvent == nil {
		t.Fatalf("no error event in %#v", events)
	}
	if errorEvent.ErrorCode != "AGENT_PROVIDER_ERROR" {
		t.Fatalf("error code = %q, want AGENT_PROVIDER_ERROR", errorEvent.ErrorCode)
	}
	if !errorEvent.Degraded || errorEvent.DegradedReason != "provider_error" {
		t.Fatalf("provider error degradation = (%v, %q), want (true, provider_error)", errorEvent.Degraded, errorEvent.DegradedReason)
	}
	if strings.Contains(errorEvent.ErrorMessage, "RAW") || strings.Contains(errorEvent.ErrorMessage, "SECRET") {
		t.Fatalf("raw provider error leaked to client: %q", errorEvent.ErrorMessage)
	}
}

func TestAgentStreamClientCancellationCancelsProviderContext(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{{toolCallDelta("get_content_detail", `{"content_id": 88}`)}}}
	svc, _ := newStreamTestService(t, provider, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var events []AgentStreamEvent
	err := svc.ChatStream(
		ctx,
		7,
		ChatTurnInput{Message: "hi"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error { events = append(events, ev); return nil },
	)
	if err == nil {
		t.Fatal("cancelled ChatStream must return an error")
	}
	var errorEvent *AgentStreamEvent
	for i := range events {
		if events[i].Type == AgentEventError {
			errorEvent = &events[i]
		}
	}
	if errorEvent == nil || errorEvent.ErrorCode != "STREAM_CANCELLED" {
		t.Fatalf("cancelled stream events = %#v, want STREAM_CANCELLED error event", events)
	}
	if errorEvent.Degraded || errorEvent.DegradedReason != "" {
		t.Fatalf("cancelled stream degradation = (%v, %q), want (false, empty)", errorEvent.Degraded, errorEvent.DegradedReason)
	}
}

func TestAgentStreamResolveChatContextRejectsHiddenContent(t *testing.T) {
	provider := &streamToolProvider{}
	svc, _ := newStreamTestService(t, provider, nil)

	hidden := int64(999)
	_, err := svc.ResolveChatContext(context.Background(), 7, &model.AgentChatContext{Surface: model.AgentChatSurfaceContent, ContentID: &hidden})
	if err == nil || !errors.Is(err, ErrContentNotFound) {
		t.Fatalf("ResolveChatContext(hidden) err = %v, want ErrContentNotFound", err)
	}
	if provider.calls != 0 {
		t.Fatal("hidden chat context must be rejected before any provider call")
	}
}

func TestAgentStreamUnknownToolNamesAreIgnoredWithStableDone(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("drop_table", `{}`)},
		{{Content: "final"}, {Done: true}},
	}}
	svc, _ := newStreamTestService(t, provider, nil)

	var events []AgentStreamEvent
	err := svc.ChatStream(
		context.Background(),
		7,
		ChatTurnInput{Message: "hi"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error { events = append(events, ev); return nil },
	)
	collectStreamEvents(t, err, &events)

	last := events[len(events)-1]
	if last.Type != AgentEventDone {
		t.Fatalf("last event = %s, want done; unknown tool must not abort the stream", last.Type)
	}
	if !json.Valid([]byte(`{}`)) {
		t.Fatal("sanity")
	}
}

// toolIndexPtr builds the streamed tool-call chunk index used by OpenAI-style
// providers that fragment tool calls across deltas.
func toolIndexPtr(v int) *int { return &v }

// TestAgentStreamAppendsAssistantToolCallMessageBeforeToolResults locks the
// OpenAI-compatible tool loop message order: every tool result message must be
// preceded by the assistant message that requested the calls (with the
// tool_calls payload), and each tool message carries the matching tool_call_id.
// Strict providers reject tool messages that appear without the assistant
// tool_calls message.
func TestAgentStreamAppendsAssistantToolCallMessageBeforeToolResults(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("get_content_detail", `{"content_id": 88}`)},
		{{Content: "final"}, {Done: true}},
	}}
	svc, _ := newStreamTestService(t, provider, nil)

	var events []AgentStreamEvent
	err := svc.ChatStream(
		context.Background(),
		7,
		ChatTurnInput{Message: "hi"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error { events = append(events, ev); return nil },
	)
	collectStreamEvents(t, err, &events)

	if provider.calls != 2 {
		t.Fatalf("provider calls = %d, want 2", provider.calls)
	}
	msgs := provider.lastReq.Messages
	if len(msgs) < 2 {
		t.Fatalf("second-round messages = %#v, want assistant tool_calls message + tool message", msgs)
	}
	assistant := msgs[len(msgs)-2]
	toolMsg := msgs[len(msgs)-1]
	if assistant.Role != "assistant" {
		t.Fatalf("message before tool result = role %q, want assistant", assistant.Role)
	}
	if len(assistant.ToolCalls) != 1 || assistant.ToolCalls[0].Function.Name != "get_content_detail" {
		t.Fatalf("assistant tool_calls = %#v, want the requested get_content_detail call", assistant.ToolCalls)
	}
	if toolMsg.Role != "tool" {
		t.Fatalf("tool result message role = %q, want tool", toolMsg.Role)
	}
	if toolMsg.ToolCallID == "" || toolMsg.ToolCallID != assistant.ToolCalls[0].ID {
		t.Fatalf("tool message tool_call_id = %q, want matching assistant call id %q", toolMsg.ToolCallID, assistant.ToolCalls[0].ID)
	}
}

// TestAgentStreamAccumulatesStreamedToolCallFragmentsByIndex locks the
// streaming tool-call assembly: OpenAI-style providers split one tool call into
// indexed fragments (id+name in the first chunk, arguments across later
// chunks). The service must assemble the fragments and only execute the call
// once the provider round completes; executing each fragment as a standalone
// call would run broken JSON arguments and fail with invalid_args.
func TestAgentStreamAccumulatesStreamedToolCallFragmentsByIndex(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{
			{ToolCalls: []llm.ToolCall{{Index: toolIndexPtr(0), ID: "call_1", Type: "function", Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: "get_content_detail", Arguments: `{"content_id"`}}}},
			{ToolCalls: []llm.ToolCall{{Index: toolIndexPtr(0), Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Arguments: ": 88"}}}},
			{ToolCalls: []llm.ToolCall{{Index: toolIndexPtr(0), Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Arguments: "}"}}}},
		},
		{{Content: "About Published Test Content"}, {Done: true}},
	}}
	svc, _ := newStreamTestService(t, provider, nil)

	var events []AgentStreamEvent
	err := svc.ChatStream(
		context.Background(),
		7,
		ChatTurnInput{Message: "what is this"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error { events = append(events, ev); return nil },
	)
	collectStreamEvents(t, err, &events)

	if provider.calls != 2 {
		t.Fatalf("provider calls = %d, want 2 (fragments must be assembled into one call before execution)", provider.calls)
	}
	toolExecutions, citations, kind := 0, 0, ""
	for _, ev := range events {
		switch ev.Type {
		case AgentEventToolStatus:
			toolExecutions++
			if ev.Tool == nil || ev.Tool.Status != AgentToolStatusSuccess {
				t.Fatalf("streamed tool call executed as %#v, want one success", ev.Tool)
			}
		case AgentEventCitation:
			citations++
			if ev.Citation == nil || ev.Citation.ContentID != 88 {
				t.Fatalf("citation = %#v, want content 88 from assembled arguments", ev.Citation)
			}
		case AgentEventDone:
			kind = string(ev.AnswerKind)
		}
	}
	if toolExecutions != 1 {
		t.Fatalf("tool executions = %d, want exactly 1 assembled call", toolExecutions)
	}
	if citations != 1 {
		t.Fatalf("citations = %d, want 1 for valid assembled arguments", citations)
	}
	if kind != "grounded_content" {
		t.Fatalf("answer_kind = %q, want grounded_content", kind)
	}

	// The reassembled call must be replayed without the stream-only index
	// fragment marker: strict OpenAI-compatible providers reject `index` on
	// non-streaming assistant messages.
	var assistantMsg llm.ChatMessage
	for _, msg := range provider.lastReq.Messages {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			assistantMsg = msg
			break
		}
	}
	if assistantMsg.Role != "assistant" || len(assistantMsg.ToolCalls) != 1 {
		t.Fatalf("provider request messages = %#v, want an assistant message carrying the assembled tool call", provider.lastReq.Messages)
	}
	if assistantMsg.ToolCalls[0].Index != nil {
		t.Fatalf("replayed tool call carries stream-only Index = %#v, want nil", assistantMsg.ToolCalls[0].Index)
	}
}

// TestAgentStreamPassesThroughProviderUsageTokens locks usage passthrough:
// the usage event and the done event must carry the provider-observed token
// counts instead of a hardcoded zero.
func TestAgentStreamPassesThroughProviderUsageTokens(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{{Content: "first "},
			{Content: "answer", Done: true, Usage: &llm.TokenUsage{PromptTokens: 120, CompletionTokens: 300}}},
	}}
	svc, _ := newStreamTestService(t, provider, nil)

	var events []AgentStreamEvent
	err := svc.ChatStream(
		context.Background(),
		7,
		ChatTurnInput{Message: "hi"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error { events = append(events, ev); return nil },
	)
	collectStreamEvents(t, err, &events)

	var usageEvent, doneEvent *AgentStreamEvent
	for i := range events {
		switch events[i].Type {
		case AgentEventUsage:
			usageEvent = &events[i]
		case AgentEventDone:
			doneEvent = &events[i]
		}
	}
	if usageEvent == nil || usageEvent.Usage == nil {
		t.Fatalf("usage event missing in %#v", events)
	}
	if usageEvent.Usage.PromptTokens != 120 || usageEvent.Usage.CompletionTokens != 300 {
		t.Fatalf("usage event tokens = %#v, want provider {120, 300}", usageEvent.Usage)
	}
	if doneEvent == nil || doneEvent.Usage == nil {
		t.Fatalf("done event missing provider usage in %#v", events)
	}
	if doneEvent.Usage.PromptTokens != 120 || doneEvent.Usage.CompletionTokens != 300 {
		t.Fatalf("done usage tokens = %#v, want provider {120, 300}", doneEvent.Usage)
	}
}
