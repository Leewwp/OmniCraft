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
	"omnicraft/backend/internal/pkg/llm"

	"gorm.io/gorm"
)

// AgentStreamEventType is the server-owned SSE event name set for the chat
// stream contract: start, tool_status, delta, citation, usage, done, error.
type AgentStreamEventType string

const (
	AgentEventStart      AgentStreamEventType = "start"
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

// AgentStreamEvent is the typed stream event. Only the fields relevant to the
// event Type are populated; there are no raw prompts, tool arguments, internal
// reasoning or Provider errors in any event.
type AgentStreamEvent struct {
	Type           AgentStreamEventType `json:"type"`
	TraceID        string               `json:"trace_id,omitempty"`
	ConversationID int64                `json:"conversation_id,omitempty"`
	AnswerKind     AgentAnswerKind      `json:"answer_kind,omitempty"`
	Delta          string               `json:"delta,omitempty"`
	Tool           *AgentToolExecution  `json:"tool,omitempty"`
	Citation       *AgentCitation       `json:"citation,omitempty"`
	Usage          *AgentUsage          `json:"usage,omitempty"`
	Answer         string               `json:"answer,omitempty"`
	Citations      []AgentCitation      `json:"citations,omitempty"`
	Tools          []AgentToolExecution `json:"tools,omitempty"`
	Degraded       bool                 `json:"degraded"`
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
func (s *AgentService) ChatStream(ctx context.Context, userID int64, messages []llm.ChatMessage, resolved *ResolvedChatContext, handler func(ev AgentStreamEvent) error) error {
	if !s.cfg.Agent.WebAgentEnabled {
		return ErrAgentDisabled
	}
	if resolved == nil {
		resolved = &ResolvedChatContext{Surface: model.AgentChatSurfaceGlobal}
	}

	traceID := newTraceID()
	traceAgentEvent(traceID, "chat_start", "user_id", userID, "surface", resolved.Surface)
	if err := ctx.Err(); err != nil {
		return emitAgentStreamError(handler, agentContextErrorCode(err), err)
	}

	contextType := "general"
	if resolved.Surface == model.AgentChatSurfaceContent && resolved.Content != nil {
		contextType = "content"
	}

	conv := &model.AgentConversation{
		UserID:      userID,
		ContextType: contextType,
		ContextID:   resolved.ContentID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if len(messages) > 0 && messages[0].Role == "user" && s.db != nil {
		if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(conv).Error; err != nil {
				return err
			}
			for _, msg := range messages {
				content := msg.Content
				if err := tx.Create(&model.AgentMessage{
					ConversationID: conv.ID,
					Role:           msg.Role,
					Content:        &content,
					CreatedAt:      time.Now(),
				}).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			slog.Error("failed to persist agent conversation", "error", err)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return emitAgentStreamError(handler, agentContextErrorCode(err), err)
			}
			return emitAgentStreamError(handler, AgentErrorCodeStorage, err)
		}
	}

	convID := int64(0)
	if conv != nil {
		convID = conv.ID
	}
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
		Messages:  append([]llm.ChatMessage{systemMsg}, messages...),
		Tools:     s.ToolDefinitions(),
		MaxTokens: policy.MaxOutputTokens,
		Stream:    true,
	}

	var answerBuf strings.Builder
	var executedTools []AgentToolExecution
	citationCandidates := make([]AgentCitation, 0, policy.CitationMaxCount)
	seenCitationIDs := make(map[int64]bool, policy.CitationMaxCount)
	streamErr := error(nil)
	var lastUsage *llm.TokenUsage

loop:
	for {
		acc := newStreamedToolCallAccumulator()
		err := s.llmProvider.ChatStream(ctx, req, func(delta llm.ChatDelta) error {
			if len(delta.ToolCalls) > 0 {
				acc.add(delta.ToolCalls)
			}
			if delta.Usage != nil {
				lastUsage = delta.Usage
			}
			if delta.Content != "" {
				answerBuf.WriteString(delta.Content)
				return handler(AgentStreamEvent{Type: AgentEventDelta, Delta: delta.Content})
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
			outcome, toolErr := s.ExecuteTool(ctx, tc.Function.Name, json.RawMessage(tc.Function.Arguments), userID, nil)
			execution := AgentToolExecution{Name: tc.Function.Name, Status: AgentToolStatusSuccess}
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
				result.Detail = outcome.Detail
				result.Guide = outcome.Guide
				result.Search = outcome.Search
				result.Suggest = outcome.Suggest
				if outcome.Detail != nil {
					if !seenCitationIDs[outcome.Detail.ID] {
						seenCitationIDs[outcome.Detail.ID] = true
						citationCandidates = append(citationCandidates, AgentCitation{
							ContentID: outcome.Detail.ID,
							Title:     outcome.Detail.Title,
							Zone:      outcome.Detail.Zone,
							Excerpt:   outcome.Detail.Excerpt,
						})
					}
				}
			}
			executedTools = append(executedTools, execution)
			if err := handler(AgentStreamEvent{Type: AgentEventToolStatus, Tool: &execution}); err != nil {
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
		if emitErr := handler(AgentStreamEvent{Type: AgentEventError, ErrorCode: string(code), ErrorMessage: safeAgentStreamMessage(code)}); emitErr != nil {
			streamErr = errors.Join(streamErr, emitErr)
		}
		if conv != nil && s.db != nil {
			// The request context is commonly canceled when the client disconnects.
			// Cleanup must still run so an interrupted turn cannot leave orphaned
			// conversation history behind.
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := s.db.WithContext(cleanupCtx).Where("conversation_id = ?", conv.ID).Delete(&model.AgentMessage{}).Error; err != nil {
				slog.Error("failed to clean up agent messages after stream failure", "error", err)
			}
			if err := s.db.WithContext(cleanupCtx).Delete(conv).Error; err != nil {
				slog.Error("failed to clean up agent conversation after stream failure", "error", err)
			}
			cancel()
		}
		return streamErr
	}

	citations := s.RevalidateCitations(ctx, userID, citationCandidates)
	for i := range citations {
		if err := handler(AgentStreamEvent{Type: AgentEventCitation, Citation: &citations[i]}); err != nil {
			return err
		}
	}

	kind := ClassifyGroundedAnswer(citations)
	usage := &AgentUsage{}
	if lastUsage != nil {
		usage = &AgentUsage{PromptTokens: lastUsage.PromptTokens, CompletionTokens: lastUsage.CompletionTokens}
	}
	if err := handler(AgentStreamEvent{Type: AgentEventUsage, Usage: usage}); err != nil {
		return err
	}

	answer := answerBuf.String()
	if conv != nil && s.db != nil {
		if err := s.db.WithContext(ctx).Create(&model.AgentMessage{
			ConversationID: conv.ID,
			Role:           "assistant",
			Content:        &answer,
			CreatedAt:      time.Now(),
		}).Error; err != nil {
			slog.Error("failed to persist agent assistant message", "error", err)
			return emitAgentStreamError(handler, AgentErrorCodeStorage, err)
		}
		if err := s.db.WithContext(ctx).Model(conv).Update("updated_at", time.Now()).Error; err != nil {
			slog.Error("failed to update agent conversation timestamp", "error", err)
			return emitAgentStreamError(handler, AgentErrorCodeStorage, err)
		}
	}

	if err := handler(AgentStreamEvent{
		Type:           AgentEventDone,
		TraceID:        traceID,
		ConversationID: convID,
		AnswerKind:     kind,
		Answer:         answer,
		Citations:      citations,
		Tools:          executedTools,
		Usage:          usage,
		Degraded:       false,
	}); err != nil {
		return err
	}

	traceAgentEvent(traceID, "chat_done", "conversation_id", convID, "surface", resolved.Surface, "answer_kind", kind, "tools", len(executedTools))
	return nil
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
