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
	// AgentErrorCodeCancelled marks a client-cancelled stream. The request
	// still consumed its reserved quota and emitted an outcome.
	AgentErrorCodeCancelled = "STREAM_CANCELLED"
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
		if err := s.db.Create(conv).Error; err != nil {
			slog.Error("failed to create agent conversation", "error", err)
			conv = nil
		}
	}
	if conv != nil && s.db != nil {
		for _, msg := range messages {
			content := msg.Content
			s.db.Create(&model.AgentMessage{
				ConversationID: conv.ID,
				Role:           msg.Role,
				Content:        &content,
				CreatedAt:      time.Now(),
			})
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

loop:
	for {
		var roundCalls []llm.ToolCall
		err := s.llmProvider.ChatStream(ctx, req, func(delta llm.ChatDelta) error {
			if len(delta.ToolCalls) > 0 {
				roundCalls = append(roundCalls, delta.ToolCalls...)
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
		if len(roundCalls) == 0 {
			break loop
		}
		if !policy.AllowToolCall(len(executedTools)) {
			traceAgentEvent(traceID, "tool_limit_reached", "executed", len(executedTools))
			break loop
		}

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
			resultJSON, _ := json.Marshal(result)
			toolMessages = append(toolMessages, llm.ChatMessage{Role: "tool", Content: string(resultJSON)})
		}
		req.Messages = append(req.Messages, toolMessages...)
	}

	if streamErr != nil {
		traceAgentEvent(traceID, "chat_failed", "safe_error", safeAgentStreamCode(streamErr))
		var code AgentStreamEventType = AgentErrorCodeProvider
		if errors.Is(streamErr, context.Canceled) || errors.Is(streamErr, context.DeadlineExceeded) {
			code = AgentErrorCodeCancelled
		}
		handler(AgentStreamEvent{Type: AgentEventError, ErrorCode: string(code), ErrorMessage: safeAgentStreamMessage(code)})
		if conv != nil && s.db != nil {
			s.db.Where("conversation_id = ?", conv.ID).Delete(&model.AgentMessage{})
			s.db.Delete(conv)
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
	if err := handler(AgentStreamEvent{Type: AgentEventUsage, Usage: &AgentUsage{}}); err != nil {
		return err
	}

	answer := answerBuf.String()
	if conv != nil && s.db != nil {
		s.db.Create(&model.AgentMessage{
			ConversationID: conv.ID,
			Role:           "assistant",
			Content:        &answer,
			CreatedAt:      time.Now(),
		})
		s.db.Model(conv).Update("updated_at", time.Now())
	}

	if err := handler(AgentStreamEvent{
		Type:           AgentEventDone,
		TraceID:        traceID,
		ConversationID: convID,
		AnswerKind:     kind,
		Answer:         answer,
		Citations:      citations,
		Tools:          executedTools,
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
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "cancelled"
	default:
		return "provider_error"
	}
}

// safeAgentStreamMessage returns localized-safe copy for a stream error code.
func safeAgentStreamMessage(code AgentStreamEventType) string {
	if code == AgentErrorCodeCancelled {
		return "stream cancelled"
	}
	return fmt.Sprintf("provider unavailable")
}
