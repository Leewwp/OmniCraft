package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/observability"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/pkg/recovery"
)

// AgentStreamEventType is the server-owned SSE event name set for the chat
// stream contract: start, tool_status, delta, citation, usage, done, error.
type AgentStreamEventType string

const (
	AgentEventStart      AgentStreamEventType = "start"
	AgentEventThinkDelta AgentStreamEventType = "think_delta"
	AgentEventToolStatus AgentStreamEventType = "tool_status"
	AgentEventDelta      AgentStreamEventType = "delta"
	AgentEventCitation   AgentStreamEventType = "citation"
	AgentEventUsage      AgentStreamEventType = "usage"
	AgentEventDone       AgentStreamEventType = "done"
	AgentEventError      AgentStreamEventType = "error"

	// AgentErrorCodeProvider is the safe code for Provider-side failures; raw
	// Provider errors are never serialized into the stream.
	AgentErrorCodeProvider = "AGENT_PROVIDER_ERROR"
	// AgentErrorCodeProviderTimeout marks a Provider-side deadline exceeded.
	// It is distinct from client cancellation so the UI can degrade to keyword
	// search instead of treating it as an aborted stream.
	AgentErrorCodeProviderTimeout = "AGENT_PROVIDER_TIMEOUT"
	// AgentErrorCodeCancelled marks a client-cancelled stream. The request
	// still consumed its reserved quota and emitted an outcome.
	AgentErrorCodeCancelled = "STREAM_CANCELLED"
	// AgentErrorCodeStorage marks a persistence failure without exposing the
	// underlying database error to the client.
	AgentErrorCodeStorage = "AGENT_STORAGE_ERROR"
)

var (
	// ErrAgentInputBlocked is returned by ModerateChatInput when Green flags
	// the chat input; the handler maps it to a 422 CONTENT_BLOCKED rejection.
	ErrAgentInputBlocked = errors.New("agent chat input rejected by content moderation")
	// ErrAgentModerationUnavailable is returned when the input gate cannot run
	// and the A4 environment semantics require fail-closed (release mode).
	ErrAgentModerationUnavailable = errors.New("agent content moderation unavailable")
)

// AgentStreamEvent is the typed stream event. Only the fields relevant to the
// event Type are populated; there are no raw prompts, raw tool arguments
// (tool steps carry a server-derived summary only), internal reasoning
// verbatim into non-think channels, or Provider errors in any event. The
// think_delta event is display-only reasoning forwarded per A-02.
type AgentStreamEvent struct {
	Type           AgentStreamEventType `json:"type"`
	TraceID        string               `json:"trace_id,omitempty"`
	ConversationID int64                `json:"conversation_id,omitempty"`
	// MessageID identifies the persisted assistant answer row; it is set on
	// the done event so clients can reference the stored message.
	MessageID     int64                `json:"message_id,omitempty"`
	AnswerKind     AgentAnswerKind      `json:"answer_kind,omitempty"`
	Delta          string               `json:"delta,omitempty"`
	Tool           *AgentToolExecution  `json:"tool,omitempty"`
	Citation       *AgentCitation       `json:"citation,omitempty"`
	Usage          *AgentUsage          `json:"usage,omitempty"`
	Answer         string               `json:"answer,omitempty"`
	Citations      []AgentCitation      `json:"citations,omitempty"`
	Tools          []AgentToolExecution `json:"tools,omitempty"`
	Degraded       bool                 `json:"degraded"`
	DegradedReason string               `json:"degraded_reason,omitempty"`
	ErrorCode      string               `json:"error_code,omitempty"`
	ErrorMessage   string               `json:"error_message,omitempty"`
}

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
	OK      bool                 `json:"ok"`
	Error   string               `json:"error,omitempty"`
	Detail  *AgentContentSummary `json:"detail,omitempty"`
	Guide   *UsageGuideResult    `json:"guide,omitempty"`
	Search  []ContentSummary     `json:"search,omitempty"`
	Suggest *UploadAssistResult  `json:"suggest,omitempty"`
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
	if err := handler(AgentStreamEvent{
		Type:           AgentEventStart,
		TraceID:        traceID,
		ConversationID: convID,
		AnswerKind:     AgentAnswerGroundedContent,
	}); err != nil {
		return err
	}

	policy := s.ToolPolicy()
	systemMsg := s.serverOwnedSystemPrompt(resolved.Surface, resolved.Content)
	req := llm.ChatRequest{
		Messages:  assembleChatContext(systemMsg, history, s.cfg.Agent.ChatContextTokenBudget, s.cfg.Agent.ChatMaxContextMsgs),
		Tools:     s.ToolDefinitions(),
		MaxTokens: policy.MaxOutputTokens,
		Stream:    true,
	}

	var answerBuf strings.Builder
	var thinkingBuf strings.Builder
	var executedTools []AgentToolExecution
	citationCandidates := make([]AgentCitation, 0, policy.CitationMaxCount)
	seenCitationKeys := make(map[string]bool, policy.CitationMaxCount)
	retrievalSources := make(map[string]string)
	degraded := false
	streamErr := error(nil)
	var lastUsage *llm.TokenUsage

loop:
	for {
		acc := newStreamedToolCallAccumulator()
		if s.chatStreamer == nil {
			streamErr = errors.New("agent streaming provider unavailable")
			break loop
		}
		err := s.chatStreamer.ChatStream(ctx, req, func(delta llm.ChatDelta) error {
			if len(delta.ToolCalls) > 0 {
				acc.add(delta.ToolCalls)
			}
			if delta.Usage != nil {
				lastUsage = delta.Usage
			}
			// A-02 real streaming: reasoning and body increments are
			// forwarded as they arrive. Thinking is display-only; it never
			// enters the answer buffer, tool results or citation revalidation.
			if delta.Thinking != "" {
				thinkingBuf.WriteString(delta.Thinking)
				if err := handler(AgentStreamEvent{Type: AgentEventThinkDelta, Delta: delta.Thinking}); err != nil {
					return err
				}
			}
			if delta.Content != "" {
				answerBuf.WriteString(delta.Content)
				if err := handler(AgentStreamEvent{Type: AgentEventDelta, Delta: delta.Content}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			streamErr = err
			break loop
		}
		roundCalls := acc.calls()
		if len(roundCalls) == 0 {
			break loop
		}
		if !policy.AllowToolCall(len(executedTools)) {
			traceAgentEvent(traceID, "tool_limit_reached", "executed", len(executedTools))
			break loop
		}

		// OpenAI-compatible protocol order: the assistant message carrying the
		// tool_calls must precede the tool result messages, and every tool
		// message references its call by tool_call_id. Strict providers reject
		// tool messages that appear without the assistant tool_calls message.
		assistantMsg := llm.ChatMessage{Role: "assistant", ToolCalls: roundCalls}
		req.Messages = append(req.Messages, assistantMsg)

		toolMessages := make([]llm.ChatMessage, 0, len(roundCalls))
		for _, tc := range roundCalls {
			toolStartedAt := time.Now()
			outcome, toolErr := s.ExecuteTool(ctx, tc.Function.Name, json.RawMessage(tc.Function.Arguments), userID, nil)
			execution := AgentToolExecution{
				Name:        tc.Function.Name,
				ArgsSummary: agentToolArgsSummary(tc.Function.Name, json.RawMessage(tc.Function.Arguments)),
				Status:      AgentToolStatusSuccess,
				DurationMs:  time.Since(toolStartedAt).Milliseconds(),
			}
			result := agentToolResult{OK: true}
			if toolErr != nil {
				execution.Status = AgentToolStatusError
				if errors.Is(toolErr, ErrAgentToolUnknown) {
					execution.Name = "(unknown)"
					result = agentToolResult{OK: false, Error: "tool_error"}
				} else if errors.Is(toolErr, ErrAgentToolInvalidArgs) {
					result = agentToolResult{OK: false, Error: "invalid_args"}
				} else if errors.Is(toolErr, ErrContentNotFound) {
					result = agentToolResult{OK: false, Error: "content_not_found"}
				} else {
					result = agentToolResult{OK: false, Error: "tool_error"}
				}
				traceAgentEvent(traceID, "tool_error", "tool", tc.Function.Name, "safe_error", result.Error)
			} else if outcome != nil {
				execution.Hits = agentToolHitCount(outcome)
				result.Detail = outcome.Detail
				result.Guide = outcome.Guide
				result.Search = outcome.Search
				result.Suggest = outcome.Suggest
				for chunkKey, source := range outcome.RetrievalSources {
					retrievalSources[chunkKey] = source
				}
				if outcome.Degraded {
					degraded = true
					traceAgentEvent(traceID, "retrieval_degraded", "tool", tc.Function.Name)
				}
				for _, summary := range outcome.Search {
					citation, ok := citationFromSearchSummary(summary)
					if !ok || seenCitationKeys[citation.ChunkKey] {
						continue
					}
					seenCitationKeys[citation.ChunkKey] = true
					citationCandidates = append(citationCandidates, citation)
				}
				if outcome.Detail != nil {
					if !s.ragHybridEnabled() {
						legacyKey := fmt.Sprintf("content:%d", outcome.Detail.ID)
						if !seenCitationKeys[legacyKey] {
							seenCitationKeys[legacyKey] = true
							citationCandidates = append(citationCandidates, AgentCitation{
								ContentID: outcome.Detail.ID,
								Title:     outcome.Detail.Title,
								Zone:      outcome.Detail.Zone,
								Excerpt:   outcome.Detail.Excerpt,
							})
						}
					} else {
						citation, err := s.citationForContent(ctx, userID, outcome.Detail.ID)
						if err != nil {
							traceAgentEvent(traceID, "citation_revalidation", "accepted", false, "reason", "citation_truth_unavailable")
						} else if source, ok := retrievalSources[citation.ChunkKey]; !ok {
							traceAgentEvent(traceID, "citation_revalidation", "accepted", false, "reason", "citation_source_unavailable")
						} else if !seenCitationKeys[citation.ChunkKey] {
							citation.Source = source
							seenCitationKeys[citation.ChunkKey] = true
							citationCandidates = append(citationCandidates, citation)
						}
					}
				}
			}
			// A-03: expansion terms surface in the tool step summary so the
			// process panel can show what the retrieval fanned out to. The
			// outcome is nil when the tool itself failed.
			if outcome != nil && len(outcome.ExpandedQueries) > 0 {
				execution.ArgsSummary += " +expanded: " + strings.Join(outcome.ExpandedQueries, " / ")
			}
			executedTools = append(executedTools, execution)
			if err := handler(AgentStreamEvent{Type: AgentEventToolStatus, Tool: &execution}); err != nil {
				s.persistPartialTurn(conv.ID, answerBuf.String())
				return err
			}
			resultJSON, err := json.Marshal(result)
			if err != nil {
				return fmt.Errorf("marshal agent tool result: %w", err)
			}
			toolMessages = append(toolMessages, llm.ChatMessage{Role: "tool", ToolCallID: tc.ID, Content: string(resultJSON)})
		}
		req.Messages = append(req.Messages, toolMessages...)
	}

	if streamErr != nil {
		traceAgentEvent(traceID, "chat_failed", "safe_error", safeAgentStreamCode(streamErr))
		var code AgentStreamEventType = AgentErrorCodeProvider
		switch {
		case errors.Is(streamErr, context.Canceled):
			code = AgentErrorCodeCancelled
		case errors.Is(streamErr, context.DeadlineExceeded):
			code = AgentErrorCodeProviderTimeout
		}
		providerFallback := code != AgentErrorCodeCancelled
		degradedReason := ""
		if providerFallback {
			degradedReason = "provider_error"
		}
		if emitErr := handler(AgentStreamEvent{
			Type:           AgentEventError,
			Degraded:       providerFallback,
			DegradedReason: degradedReason,
			ErrorCode:      string(code),
			ErrorMessage:   safeAgentStreamMessage(code),
		}); emitErr != nil {
			streamErr = errors.Join(streamErr, emitErr)
		}
		// A-01: stop/failure keeps the conversation and whatever partial
		// content already streamed out; deletion happens only on the user's
		// explicit DELETE. The request context is commonly canceled on client
		// disconnect, so persistence runs on a detached bounded context.
		if conv != nil {
			s.persistPartialTurn(conv.ID, answerBuf.String())
		}
		return streamErr
	}

	citations := s.revalidateCitations(ctx, userID, citationCandidates, traceID)
	kind := ClassifyGroundedAnswer(citations)
	answer := answerBuf.String()
	for i := range citations {
		if err := handler(AgentStreamEvent{Type: AgentEventCitation, Citation: &citations[i]}); err != nil {
			s.persistPartialTurn(conv.ID, answerBuf.String())
			return err
		}
	}

	usage := &AgentUsage{}
	if lastUsage != nil {
		usage = &AgentUsage{PromptTokens: lastUsage.PromptTokens, CompletionTokens: lastUsage.CompletionTokens}
	}
	if err := handler(AgentStreamEvent{Type: AgentEventUsage, Usage: usage}); err != nil {
		s.persistPartialTurn(conv.ID, answerBuf.String())
		return err
	}

	if kind == AgentAnswerNoEvidence || degraded {
		answer = ""
	}
	answerMessageID := int64(0)
	if s.db != nil {
		// The provider already finished; the client may disconnect at any
		// moment, so the final answer persists on a detached bounded context.
		storeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if think := thinkingBuf.String(); think != "" {
			// A-02: the reasoning block persists as its own phase-marked row
			// (tool_calls = {"phase":"think"}) ahead of the answer row, for
			// history replay and audit; readers treat it as display-only.
			if err := s.db.WithContext(storeCtx).Create(&model.AgentMessage{
				ConversationID: conv.ID,
				Role:           "assistant",
				Content:        &think,
				ToolCalls:      model.JSONMap{"phase": "think"},
				CreatedAt:      time.Now(),
			}).Error; err != nil {
				cancel()
				slog.Error("failed to persist agent thinking message", "error", err)
				return emitAgentStreamError(handler, AgentErrorCodeStorage, err)
			}
		}
		answerRow := model.AgentMessage{
			ConversationID: conv.ID,
			Role:           "assistant",
			Content:        &answer,
			CreatedAt:      time.Now(),
		}
		if err := s.db.WithContext(storeCtx).Create(&answerRow).Error; err != nil {
			cancel()
			slog.Error("failed to persist agent assistant message", "error", err)
			return emitAgentStreamError(handler, AgentErrorCodeStorage, err)
		}
		answerMessageID = answerRow.ID
		if err := s.db.WithContext(storeCtx).Model(conv).Update("updated_at", time.Now()).Error; err != nil {
			cancel()
			slog.Error("failed to update agent conversation timestamp", "error", err)
			return emitAgentStreamError(handler, AgentErrorCodeStorage, err)
		}
		cancel()
		if !hadAssistantBefore {
			s.scheduleAutoTitle(traceID, conv.ID, firstUserMessage(history))
		}
		// A-05: the persisted answer is audited asynchronously after the
		// turn; a flagged row is redacted by the history endpoint.
		s.scheduleOutputModeration(traceID, answerMessageID, answer)
	}

	if err := handler(AgentStreamEvent{
		Type:           AgentEventDone,
		TraceID:        traceID,
		ConversationID: convID,
		MessageID:      answerMessageID,
		AnswerKind:     kind,
		Answer:         answer,
		Citations:      citations,
		Tools:          executedTools,
		Usage:          usage,
		Degraded:       degraded,
	}); err != nil {
		return err
	}

	traceAgentEvent(traceID, "chat_done", "conversation_id", convID, "surface", resolved.Surface, "answer_kind", kind, "tools", len(executedTools))
	return nil
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
