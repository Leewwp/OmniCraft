package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/observability"
	"omnicraft/backend/internal/observability/agenttrace"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/service/promptregistry"
)

// ResolvedChatContext is the viewer-preloaded, server-owned chat context. The
// client only submits a surface enum and an optional content ID; the service
// reloads title/type/visibility with the current viewer BEFORE any quota
// reservation, so hidden content is rejected without consuming a request.
type ResolvedChatContext struct {
	Surface   model.AgentChatSurface
	ContentID *int64
	Content   *model.ContentItem
}

// ResolveChatContext performs the viewer-aware preload of any client-supplied
// context ID. Hidden/unpublished content returns ErrContentNotFound so it can
// never be probed, and callers must not reserve quota or reach the Provider
// after this fails.
func (s *AgentService) ResolveChatContext(ctx context.Context, viewerID int64, chatCtx *model.AgentChatContext) (*ResolvedChatContext, error) {
	if chatCtx == nil {
		return &ResolvedChatContext{Surface: model.AgentChatSurfaceGlobal}, nil
	}
	resolved := &ResolvedChatContext{Surface: chatCtx.Surface, ContentID: chatCtx.ContentID}
	if chatCtx.Surface == model.AgentChatSurfaceContent && chatCtx.ContentID != nil {
		content, err := s.resolveVisibleContent(ctx, viewerID, *chatCtx.ContentID)
		if err != nil {
			return nil, err
		}
		resolved.Content = content
	}
	return resolved, nil
}

// agentToolResult is the typed, server-owned tool result handed back to the
// model. Raw tool arguments and internal reasoning are never included; a
// forbidden content ID produces the uniform content_not_found result.
type agentToolResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	// Message is a model-facing relay hint for stable degradation codes
	// (image quota): what the model should tell the user instead of
	// retrying the call.
	Message string               `json:"message,omitempty"`
	Detail  *AgentContentSummary `json:"detail,omitempty"`
	Guide   *UsageGuideResult    `json:"guide,omitempty"`
	Search  []ContentSummary     `json:"search,omitempty"`
	IPs     []AgentIPSummary     `json:"ips,omitempty"`
	Suggest *UploadAssistResult  `json:"suggest,omitempty"`
	Image   *AgentImageResult    `json:"image,omitempty"`
	MCP     *AgentMCPResult      `json:"mcp,omitempty"`
}

// streamedToolCallAccumulator assembles OpenAI-style streamed tool calls,
// which arrive split into indexed fragments (id+name on the first chunk,
// arguments across later chunks). Calls are only considered complete once the
// provider round finishes, so partial JSON fragments are never executed as
// tool arguments.
type streamedToolCallAccumulator struct {
	byIndex  map[int]*llm.ToolCall
	order    []int
	complete []llm.ToolCall
}

func newStreamedToolCallAccumulator() *streamedToolCallAccumulator {
	return &streamedToolCallAccumulator{byIndex: make(map[int]*llm.ToolCall)}
}

// add merges one delta's tool-call fragments. Chunks without an index are
// treated as complete calls (non-fragmenting providers).
func (a *streamedToolCallAccumulator) add(chunks []llm.ToolCall) {
	for i := range chunks {
		chunk := chunks[i]
		if chunk.Index == nil {
			a.complete = append(a.complete, chunk)
			continue
		}
		idx := *chunk.Index
		cur, ok := a.byIndex[idx]
		if !ok {
			cur = &llm.ToolCall{ID: chunk.ID, Type: chunk.Type}
			cur.Function.Name = chunk.Function.Name
			a.byIndex[idx] = cur
			a.order = append(a.order, idx)
		}
		cur.Function.Arguments += chunk.Function.Arguments
	}
}

// calls returns the fully assembled round calls in first-seen order. Index is
// a stream-only fragment marker and must never be serialized back into a
// non-streaming assistant message (strict OpenAI-compatible providers reject
// it), so assembled calls always carry a nil Index.
func (a *streamedToolCallAccumulator) calls() []llm.ToolCall {
	if len(a.complete) == 0 && len(a.order) == 0 {
		return nil
	}
	out := make([]llm.ToolCall, 0, len(a.complete)+len(a.order))
	out = append(out, a.complete...)
	for _, idx := range a.order {
		call := *a.byIndex[idx]
		call.Index = nil
		out = append(out, call)
	}
	return out
}

// ChatStream runs one chat turn against the Provider with a server-owned tool
// loop. The viewer-aware context must already be resolved (ResolveChatContext)
// and the request quota reserved by the caller before this method is invoked;
// every outcome after that — success, timeout, Provider error, client
// cancellation — consumes that reservation and emits a typed stream event.
// Conversation history is assembled server-side from the stored conversation
// (A-01): the client only submits the new message.
func (s *AgentService) ChatStream(ctx context.Context, userID int64, turn ChatTurnInput, resolved *ResolvedChatContext, handler func(ev AgentStreamEvent) error) error {
	if !s.cfg.Agent.WebAgentEnabled {
		return ErrAgentDisabled
	}
	if resolved == nil {
		resolved = &ResolvedChatContext{Surface: model.AgentChatSurfaceGlobal}
	}

	traceID := observability.TraceID(ctx)
	if traceID == "" {
		traceID = untracedTraceID
	}
	traceAgentEvent(traceID, "chat_start", "user_id", userID, "surface", resolved.Surface, "conversation_id", turn.ConversationID)
	if err := ctx.Err(); err != nil {
		return emitAgentStreamError(handler, agentContextErrorCode(err), err)
	}

	contextType := "general"
	if resolved.Surface == model.AgentChatSurfaceContent && resolved.Content != nil {
		contextType = "content"
	}

	conv, history, hadAssistantBefore, err := s.resolveChatConversation(ctx, userID, turn, contextType, resolved.ContentID)
	if err != nil {
		if errors.Is(err, ErrAgentConversationNotFound) {
			return emitAgentStreamError(handler, AgentErrorCodeConversationNotFound, err)
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return emitAgentStreamError(handler, agentContextErrorCode(err), err)
		}
		slog.Error("failed to resolve agent conversation", "error", err)
		return emitAgentStreamError(handler, AgentErrorCodeStorage, err)
	}

	convID := conv.ID
	// SP-21 T2: bind the turn to the async trace writer. The recorder rides
	// the context from here on, so the routing provider and the retriever
	// channels record without parameter changes. Sampling is decided once
	// per trace id (NewTurnRecorder) and gates every record below.
	turnRecorder := agenttrace.NewTurnRecorder(s.traceWriter, traceID)
	ctx = agenttrace.WithTurnRecorder(ctx, turnRecorder)
	turnStarted := time.Now()
	_, promptVersion := s.prompts.Resolve(ctx, promptregistry.SlotAgentSystem)
	promptVer := promptVersion
	convIDPtr := convID
	userIDPtr := userID
	turnRecorder.RecordRunStart(model.AgentTraceRun{
		StartedAt:      turnStarted,
		ConversationID: &convIDPtr,
		UserID:         &userIDPtr,
		Surface:        string(resolved.Surface),
		PromptName:     promptregistry.SlotAgentSystem.Name,
		PromptVersion:  &promptVer,
	})
	if err := handler(AgentStreamEvent{
		Type:           AgentEventStart,
		TraceID:        traceID,
		ConversationID: convID,
		AnswerKind:     AgentAnswerGroundedContent,
	}); err != nil {
		return err
	}

	// #661：回合编排器一次构造（纯装配无 IO），chitchat 短路与完整回合
	// 共用同一终局收口。
	runner := s.newTurnRunner(userID, turn, conv, traceID, turnStarted, resolved.Surface, hadAssistantBefore, firstUserMessage(history), turnRecorder, handler)

	// SP-15 A1 rule-layer shortcut: an exact-match chitchat message replays a
	// server-owned template without any Provider call. The turn already
	// passed the moderation input gate and consumed its reserved quota (both
	// happen in the handler before ChatStream), and the user row is already
	// stored by resolveChatConversation — the shortcut only skips the LLM.
	if template, ok := s.chitchatShortcutReply(turn.Message); ok {
		recordShortcutRun(turnRecorder, turnStarted, conv)
		if err := handler(AgentStreamEvent{Type: AgentEventDelta, Delta: template}); err != nil {
			runner.streamErr = err
			return runner.finalizeTurn(ctx, turnFinalizeInput{outcome: outcomeStreamError})
		}
		return runner.finalizeTurn(ctx, turnFinalizeInput{outcome: outcomeShortcut, template: template})
	}

	policy := s.ToolPolicy()
	systemMsg := s.serverOwnedSystemPrompt(ctx, resolved.Surface, resolved.Content)
	// #661: tool surface and execution ride the ToolRuntime seam (local
	// registry + MCP bridge dispatch in production, fake in unit tests).
	tools := s.toolRuntimeOrFallback().ToolDefinitions(ctx)
	req := llm.ChatRequest{
		Messages:  assembleChatContext(systemMsg, history, s.cfg.Agent.ChatContextTokenBudget, s.cfg.Agent.ChatMaxContextMsgs),
		Tools:     tools,
		MaxTokens: policy.MaxOutputTokens,
		Stream:    true,
	}
	// #539/#545: 深度思考开关——默认关（快、省 token：thinking.type=
	// disabled，首字更快）；开启时显式 Adaptive（MiniMax 语义等同默认，
	// DeepSeek 映射 enabled）。不支持该参数的 provider 按各自约定忽略。
	// #545: ModelPref 钉选注册表模型（路由层置首），空 = 配置主供给。
	if !turn.DeepThink {
		req.Thinking = llm.ThinkingDisabled
	} else {
		req.Thinking = llm.ThinkingAdaptive
	}
	req.ModelPref = turn.Model

	// #661：回合编排下沉 turnRunner——ChatStream 只做装配（上下文解析 →
	// chitchat 短路 → 循环 → 终局收口），回合状态与流循环住 runner。
	runner.run(ctx, &req)

	// #661：错误终局（provider 失败/超时/取消/连接中断/内部中止）统一走
	// finalizeTurn 收口——终局四件套（部分落库+trace 终行+metrics+终局事件）
	// 只在此一处发生。
	if runner.streamErr != nil {
		return runner.finalizeTurn(ctx, turnFinalizeInput{outcome: outcomeStreamError})
	}

	// SP-21 T2: citation revalidation and exit classification close the
	// grounded path before the answer is finalized.
	citeSpan := turnRecorder.StartNode(agenttrace.NodeTypeCitations, "citation_revalidation", nil, "")
	citations := s.revalidateCitations(ctx, userID, runner.citationCandidates, traceID)
	citeSpan.End(agenttrace.NodeEndOptions{NodeName: "citation_revalidation", Extra: model.JSONB(agentCitationExtraJSON(citations))})
	answer := runner.answerBuf.String()
	// SP-23 M4 piece 2: image URLs in the model answer survive only on the
	// platform's own hosts (or a host this conversation's tools issued).
	answer = SanitizeImageURLs(answer, s.cfg.Agent.Guardrails.ImageURLAllowHosts, runner.ownImagePrefixes, runner.answerLang)
	classifySpan := turnRecorder.StartNode(agenttrace.NodeTypeClassify, "classify", nil, "")
	kind := ClassifyStreamAnswerWithExternal(citations, runner.executedTools, answer, runner.degraded, s.conversationalMaxRunes(), s.cfg.Agent.MCP.ExternalAnswerMaxRunes, s.cfg.RAG.Refusal.MinSurvivingCitations)
	classifySpan.End(agenttrace.NodeEndOptions{
		NodeName:         "classify",
		CompletionDigest: answer,
	})
	// SP-15 B join (#435): only a grounded, non-degraded turn waits for the
	// speculative follow-up call. A result already sitting in the buffered
	// channel is taken non-blockingly even when the budget has elapsed; only a
	// still-missing result waits, bounded by what remains of followUpBudget
	// since the first delta. A miss or timeout attaches nothing.
	followUps := []string(nil)
	if runner.followUpStarted && kind == AgentAnswerGroundedContent && !runner.degraded {
		received := false
		select {
		case items := <-runner.followUpCh:
			followUps = items
			received = true
		default:
		}
		if !received {
			remaining := followUpBudget - time.Since(runner.followUpStartedAt)
			if remaining > 0 {
				timer := time.NewTimer(remaining)
				select {
				case items := <-runner.followUpCh:
					followUps = items
				case <-timer.C:
					traceAgentEvent(traceID, "follow_ups_missed", "reason", "timeout")
				}
				timer.Stop()
			} else {
				traceAgentEvent(traceID, "follow_ups_missed", "reason", "budget_elapsed")
			}
		}
	}
	if len(followUps) > 0 {
		traceAgentEvent(traceID, "follow_ups_attached", "count", len(followUps))
	}
	// 引用上限之外的 [n] 标注是死引用（前端渲染为不可点角标）：终稿与落库前
	// 统一剥离，SSE delta 阶段已流出的角标由 done 终稿替换回收。
	answer = stripOrphanCitationMarkers(answer, len(citations))
	for i := range citations {
		if err := handler(AgentStreamEvent{Type: AgentEventCitation, Citation: &citations[i]}); err != nil {
			runner.streamErr = err
			return runner.finalizeTurn(ctx, turnFinalizeInput{outcome: outcomeStreamError})
		}
	}

	usage := &AgentUsage{}
	if runner.lastUsage != nil {
		usage = &AgentUsage{PromptTokens: runner.lastUsage.PromptTokens, CompletionTokens: runner.lastUsage.CompletionTokens}
	}
	if err := handler(AgentStreamEvent{Type: AgentEventUsage, Usage: usage}); err != nil {
		runner.streamErr = err
		return runner.finalizeTurn(ctx, turnFinalizeInput{outcome: outcomeStreamError})
	}

	if kind == AgentAnswerNoEvidence || runner.degraded {
		answer = ""
	}
	// #661：成功终局统一走 finalizeTurn——终局四件套（think/tools/答案行
	// 落库+trace 终行+metrics+done 事件）只在此一处发生；done 装配（终稿、
	// 引用、follow-ups、usage、kind）留本成功路径。
	return runner.finalizeTurn(ctx, turnFinalizeInput{
		outcome:   outcomeSuccess,
		answer:    answer,
		think:     runner.thinkingBuf.String(),
		kind:      kind,
		citations: citations,
		followUps: followUps,
		usage:     usage,
	})
}

// ---------------------------------------------------------------------------
// SP-21 T2 trace instrumentation helpers
// ---------------------------------------------------------------------------

// recordShortcutRun shapes the trace run row for the rule-layer shortcut
// (chitchat): a simplified RunStart (the prelude already recorded one with
// the full turn shape; this re-shapes it as conversational) plus the
// chitchat node. The terminal quartet fires in finalizeTurn.
func recordShortcutRun(rec *agenttrace.TurnRecorder, started time.Time, conv *model.AgentConversation) {
	if rec == nil {
		return
	}
	convID := conv.ID
	rec.RecordRunStart(model.AgentTraceRun{
		StartedAt:      started,
		ConversationID: &convID,
		Status:         model.AgentTraceStatusRunning,
	})
	if span := rec.StartNode(agenttrace.NodeTypeChitchat, "chitchat_shortcut", nil, ""); span != nil {
		span.End(agenttrace.NodeEndOptions{NodeName: "chitchat_shortcut"})
	}
}

// recordAgentRunMetrics mirrors the run-end trace row into the label-free
// SLA metric set (SP-24 R7): TTFT to the first forwarded display delta,
// wall-clock duration, terminal status and classified answer kind. A zero
// firstDisplayDelta means nothing was displayed and records duration only.
func recordAgentRunMetrics(status, kind string, firstDisplayDelta, startedAt time.Time) {
	ttftSet := !firstDisplayDelta.IsZero()
	ttft := 0.0
	if ttftSet {
		ttft = firstDisplayDelta.Sub(startedAt).Seconds()
		if ttft < 0 {
			ttft = 0
		}
	}
	duration := time.Since(startedAt).Seconds()
	if duration < 0 {
		duration = 0
	}
	observability.ObserveDefaultAgentRun(status, kind, ttftSet, ttft, duration)
}

// servingModel attributes the turn to the model that actually served it:
// the last routing event's target when a failover happened, else the
// client-pinned preference, else the configured display name.
func (s *AgentService) servingModel(rec *agenttrace.TurnRecorder, pref string) string {
	if serving := rec.ServingModel(); serving != "" {
		return serving
	}
	if pref != "" {
		return pref
	}
	if s.cfg != nil {
		return s.cfg.Agent.LLMModel
	}
	return ""
}

// runTerminalStatus maps a stream failure onto the trace run state machine.
func runTerminalStatus(err error) string {
	switch {
	case err == nil:
		return model.AgentTraceStatusSuccess
	case errors.Is(err, context.Canceled):
		return model.AgentTraceStatusCanceled
	default:
		return model.AgentTraceStatusError
	}
}

// usagePtr lifts a token counter off a usage snapshot, nil-safe.
func usagePtr(u *llm.TokenUsage, pick func(*llm.TokenUsage) int64) *int64 {
	if u == nil {
		return nil
	}
	v := pick(u)
	return &v
}

func toolNodeStatus(err error) string {
	if err != nil {
		return model.AgentTraceStatusError
	}
	return model.AgentTraceStatusSuccess
}

// agentToolNodeKey sanitizes a tool name into a stable node-key fragment.
func agentToolNodeKey(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "tool"
	}
	return b.String()
}

// agentToolExtraJSON snapshots the #538 step shape into the node extra.
func agentToolExtraJSON(execution AgentToolExecution, outcome *AgentToolOutcome) []byte {
	payload := map[string]any{
		"hits":        execution.Hits,
		"status":      string(execution.Status),
		"external":    execution.External,
		"duration_ms": execution.DurationMs,
	}
	// SP-23 M5: external-tool specifics — image cost estimate (T7 rate
	// linkage) and MCP server/tool/truncation — ride the trace node extra.
	if outcome != nil && outcome.Image != nil {
		payload["image_cost_cny"] = outcome.Image.CostCNY
		payload["image_size"] = outcome.Image.Size
	}
	if outcome != nil && outcome.MCP != nil {
		payload["mcp_server"] = outcome.MCP.Server
		payload["mcp_tool"] = outcome.MCP.Tool
		payload["mcp_truncated"] = outcome.MCP.Truncated
	}
	// #619: tool-layer query truncation rides the trace node extra next to
	// the MCP truncation flag.
	if outcome != nil && outcome.QueryTruncated {
		payload["query_truncated"] = true
	}
	return promptregistryMustJSON(payload)
}

func agentCitationExtraJSON(citations []AgentCitation) []byte {
	return promptregistryMustJSON(map[string]any{"kept": len(citations)})
}

func promptregistryMustJSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

// firstLine returns the first line of s (bounded digests for side calls).
func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

// emitChitchatTemplateTurn finishes a rule-layer shortcut turn (SP-15 A1):
// one delta carrying the whole template, then a conversational done event.
// The assistant template row persists like any answer (history replay stays
// complete), but no LLM auto-title is scheduled — a chitchat excerpt is not
// worth a title call and the title-IS-NUL semantics let the first real
// question name the conversation later. Output moderation is skipped too: the
// text is a server-owned constant, not model- or user-generated content.
// conversationalMaxRunes guards against a nil cfg (DB-less service seams).
func (s *AgentService) conversationalMaxRunes() int {
	if s.cfg == nil {
		return 0
	}
	return s.cfg.Agent.ConversationalMaxRunes
}

// SP-15 B (#435) follow-up generation constants. followUpBudget is a var so
// tests can shorten the join window; production reads the 4s spec value.
var followUpBudget = 4 * time.Second

const (
	followUpMaxCount = 3
	// followUpMaxTokens 需覆盖思考型模型的 reasoning 开销：M3 的思考链会先吃
	// completion 预算（SP-13 既有教训），128 时正文只剩 2 字符残根。

	followUpMaxRunes   = 20
	followUpMaxTitles  = 8
	followUpPrefixCap  = 200
	followUpMaxTokens  = 1024
	followUpMaxTitleLn = 80
)

// followUpTitles lifts the display titles of the current citation candidates
// for the generation prompt (pre-revalidation is fine: the prompt only needs
// what the model saw while answering).
func followUpTitles(candidates []AgentCitation) []string {
	titles := make([]string, 0, followUpMaxTitles)
	for _, c := range candidates {
		if len(titles) >= followUpMaxTitles {
			break
		}
		if t := strings.TrimSpace(c.Title); t != "" {
			titles = append(titles, truncateChatRunes(t, followUpMaxTitleLn))
		}
	}
	return titles
}

// followUpRequest builds the bounded non-streaming prompt: user question +
// retrieved titles + answer prefix, asking for 2-3 same-language follow-up
// questions, one per line, each within the rune cap. The instruction corpus
// is the versioned follow_ups_prompt slot (SP-21 T5); the resolver falls
// back to the compiled-in builtin.
func followUpRequest(resolver *promptregistry.PromptResolver, question string, titles []string, answerPrefix string) llm.ChatRequest {
	titlesJoined := "(none)"
	if len(titles) > 0 {
		titlesJoined = strings.Join(titles, " / ")
	}
	content := resolver.RenderSlot(context.Background(), promptregistry.SlotFollowUps, map[string]string{
		"question":      strings.TrimSpace(question),
		"titles":        titlesJoined,
		"answer_prefix": truncateChatRunes(strings.TrimSpace(answerPrefix), followUpPrefixCap),
	})
	return llm.ChatRequest{
		Messages:  []llm.ChatMessage{{Role: "user", Content: content}},
		MaxTokens: followUpMaxTokens,
	}
}

// generateFollowUps runs the small non-streaming call and parses its line
// output. Every failure mode (call error, empty or over-long lines, garbage)
// returns nil — the feature degrades to "no follow-ups" silently and never
// affects the main stream. The traceID labels the side call for diagnosis.
func generateFollowUps(ctx context.Context, turnRecorder *agenttrace.TurnRecorder, resolver *promptregistry.PromptResolver, provider llm.LLMProvider, traceID, question string, titles []string, answerPrefix string) []string {
	// SP-21 T2: the side call gets its own node under the turn's trace. The
	// terminal End is deferred BEFORE the provider call so a panic inside
	// Chat still closes the node (GoSafe recovers; without this the node
	// stays RUNNING forever — caught live on the real-trace waterfall).
	followSpan := turnRecorder.StartNode(agenttrace.NodeTypeFollowUps, "follow_ups", nil, "")
	var resp *llm.ChatResponse
	var err error
	defer func() {
		status := model.AgentTraceStatusSuccess
		errCode := ""
		digest := ""
		if err != nil || resp == nil {
			status = model.AgentTraceStatusError
			errCode = "follow_ups_call_failed"
		} else {
			digest = firstLine(resp.Content)
		}
		// resp is nil on the error path; reading resp.Content there panicked
		// inside GoSafe and left the node RUNNING forever (caught live on the
		// demo-site smoke: deadline-exceeded follow-up side call).
		followSpan.End(agenttrace.NodeEndOptions{NodeName: "follow_ups", Status: status, ErrorCode: errCode, CompletionDigest: digest})
	}()
	resp, err = provider.Chat(ctx, followUpRequest(resolver, question, titles, answerPrefix))
	if err != nil {
		reason := "provider_error"
		if ctx.Err() == context.DeadlineExceeded {
			reason = "deadline_exceeded"
		}
		traceAgentEvent(traceID, "follow_ups_call_failed", "reason", reason)
		return nil
	}
	if resp == nil {
		traceAgentEvent(traceID, "follow_ups_call_failed", "reason", "nil_response")
		return nil
	}
	items := parseFollowUpItems(resp.Content)
	if len(items) == 0 {
		traceAgentEvent(traceID, "follow_ups_parse_empty", "content_runes", len([]rune(resp.Content)))
	}
	return items
}

// parseFollowUpItems normalizes model line output into 1..3 trimmed, deduped,
// rune-bounded questions; anything invalid is dropped.
func parseFollowUpItems(content string) []string {
	seen := make(map[string]bool, followUpMaxCount)
	items := make([]string, 0, followUpMaxCount)
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		line = strings.TrimPrefix(line, "-")
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)
		line = strings.TrimSuffix(line, "。")
		line = strings.TrimSpace(line)
		// M3 常见编号前缀（prompt 已禁但不可全信）：1. / 1、 / (1) / ①。
		for {
			trimmed := strings.TrimLeft(line, "0123456789")
			if trimmed != line && (strings.HasPrefix(trimmed, ".") || strings.HasPrefix(trimmed, "、") || strings.HasPrefix(trimmed, ")")) {
				line = strings.TrimSpace(strings.TrimLeft(trimmed, ".、) "))
				continue
			}
			break
		}
		if strings.HasPrefix(line, "①") || strings.HasPrefix(line, "②") || strings.HasPrefix(line, "③") {
			line = strings.TrimSpace(line[3:])
		}
		if line == "" || strings.HasPrefix(line, "(") || strings.HasPrefix(line, "（") {
			continue
		}
		runes := []rune(line)
		if len(runes) == 0 || len(runes) > followUpMaxRunes {
			continue
		}
		if seen[line] {
			continue
		}
		seen[line] = true
		items = append(items, line)
		if len(items) >= followUpMaxCount {
			break
		}
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

// ModerateChatInput applies the A-05 input admission gate over a chat message
// before the turn starts. A "block" (or normalized "violation") result
// rejects the input. Availability follows the A4 environment semantics via
// RunModerationGate: release mode fails closed on any moderation failure,
// while local/test mode fails open when Green is not configured (recorded via
// structured logs). Blank text is skipped without an external call.
func (s *AgentService) ModerateChatInput(ctx context.Context, text string) error {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	var review func(context.Context) (string, error)
	if s.greenClient != nil {
		review = func(ctx context.Context) (string, error) {
			res, err := s.greenClient.TextModeration(ctx, trimmed)
			if err != nil {
				return "", err
			}
			return NormalizeReviewResult(res.Result), nil
		}
	}
	return RunModerationGate(ctx, s.cfg, "agent_chat_input", "content moderation", "chat turn",
		review, true, ErrAgentInputBlocked, ErrAgentModerationUnavailable)
}

// scheduleOutputModeration asynchronously audits a persisted assistant answer
// through Green text moderation (A-05). It is a post-turn audit, never an
// admission gate: it must not block or fail the stream. On "block" the stored
// answer row is flagged (tool_calls = {"moderation":"blocked"}) so the
// conversation history returns a redacted representation; the raw text stays
// stored for audit. Scan unavailability is fail-open with structured logs in
// every environment — hiding every answer because the scanner is down would
// break the product, unlike the input gate's release fail-closed semantics.
func (s *AgentService) scheduleOutputModeration(traceID string, messageID int64, answer string) {
	if s.db == nil || s.greenClient == nil {
		return
	}
	text := strings.TrimSpace(answer)
	if text == "" {
		return
	}
	recovery.GoSafe(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		res, err := s.greenClient.TextModeration(ctx, text)
		envMode := "unknown"
		if s.cfg != nil {
			envMode = s.cfg.Server.Mode
		}
		if err != nil {
			slog.Warn("agent output moderation skipped, answer kept visible",
				"action", "agent_chat_output", "env_mode", envMode, "policy", "fail_open", "reason", err.Error())
			traceAgentEvent(traceID, "output_moderation_skipped", "message_id", messageID)
			return
		}
		if NormalizeReviewResult(res.Result) != "block" {
			return
		}
		if err := s.db.WithContext(ctx).Model(&model.AgentMessage{}).
			Where("id = ?", messageID).
			Update("tool_calls", model.JSONMap{"moderation": "blocked"}).Error; err != nil {
			slog.Error("failed to flag moderated agent answer", "message_id", messageID, "error", err)
			traceAgentEvent(traceID, "output_moderation_flag_failed", "message_id", messageID)
			return
		}
		traceAgentEvent(traceID, "output_moderation_blocked", "message_id", messageID)
	})
}

// safeAgentStreamCode maps a stream failure to a safe event code without
// exposing raw Provider errors.
func safeAgentStreamCode(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "provider_timeout"
	default:
		return "provider_error"
	}
}

func agentContextErrorCode(err error) string {
	if errors.Is(err, context.Canceled) {
		return AgentErrorCodeCancelled
	}
	return AgentErrorCodeProviderTimeout
}

// safeAgentStreamMessage returns localized-safe copy for a stream error code.
func safeAgentStreamMessage(code AgentStreamEventType) string {
	switch code {
	case AgentErrorCodeStorage:
		return "agent history unavailable"
	case AgentStreamEventType(AgentErrorCodeConversationNotFound):
		return "conversation no longer available"
	case AgentErrorCodeCancelled:
		return "stream cancelled"
	case AgentErrorCodeProviderTimeout:
		return "provider timed out"
	default:
		return "provider unavailable"
	}
}

func emitAgentStreamError(handler func(ev AgentStreamEvent) error, code string, cause error) error {
	if err := handler(AgentStreamEvent{
		Type:         AgentEventError,
		ErrorCode:    code,
		ErrorMessage: safeAgentStreamMessage(AgentStreamEventType(code)),
	}); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

// stripOrphanCitationMarkers removes plain [n] citation markers that no longer
// resolve to a kept citation (n 超出保留引用数或非法)。模型自然产出的标注量
// 常超过 citation_max_count，残留的角标在前端渲染为不可点死引用；终稿与落库
// 前统一剥离。Markdown 链接形如 [1](url) 的数字文本不受影响。
func stripOrphanCitationMarkers(answer string, kept int) string {
	if kept < 0 {
		return answer
	}
	var b strings.Builder
	b.Grow(len(answer))
	for i := 0; i < len(answer); {
		if answer[i] == '[' {
			if end := strings.IndexByte(answer[i+1:], ']'); end > 0 {
				inner := answer[i+1 : i+1+end]
				if n, ok := parseCitationMarker(inner); ok {
					next := i + end + 2
					followedByParen := next < len(answer) && answer[next] == '('
					if !followedByParen && (n > kept || n <= 0) {
						i = next
						continue
					}
				}
			}
		}
		b.WriteByte(answer[i])
		i++
	}
	return b.String()
}

// parseCitationMarker accepts short pure-digit marker bodies only ("12", not
// "1,2" or long digit runs that are unlikely citation marks).
func parseCitationMarker(inner string) (int, bool) {
	if len(inner) == 0 || len(inner) > 3 {
		return 0, false
	}
	n := 0
	for k := 0; k < len(inner); k++ {
		if inner[k] < '0' || inner[k] > '9' {
			return 0, false
		}
		n = n*10 + int(inner[k]-'0')
	}
	return n, true
}
