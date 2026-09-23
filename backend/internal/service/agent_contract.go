package service

import (
	"encoding/json"
	"errors"

	"omnicraft/backend/internal/model"
)

// ---------------------------------------------------------------------------
// Stream event vocabulary (chat turn SSE contract)
// ---------------------------------------------------------------------------

// AgentStreamEventType is the server-owned SSE event name set for the chat
// stream contract: start, tool_status, delta, citation, usage, done, error.
//
// done 顺序语义（契约固化）：done 事件携带的终稿 Answer 替换此前 delta 流
// 累积的正文——delta 阶段已流出、终稿阶段被剥离或清空的字符（超出引用上限
// 的 [n] 死角标、no_evidence/degraded 的清空）由 done 终稿回收；FollowUps
// 只挂 grounded 且非 degraded 的 done；done 之后的 error 事件不存在——
// 终局之后流即关闭。
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
	MessageID  int64                `json:"message_id,omitempty"`
	AnswerKind AgentAnswerKind      `json:"answer_kind,omitempty"`
	Delta      string               `json:"delta,omitempty"`
	Tool       *AgentToolExecution  `json:"tool,omitempty"`
	Citation   *AgentCitation       `json:"citation,omitempty"`
	Usage      *AgentUsage          `json:"usage,omitempty"`
	Answer     string               `json:"answer,omitempty"`
	Citations  []AgentCitation      `json:"citations,omitempty"`
	Tools      []AgentToolExecution `json:"tools,omitempty"`
	// FollowUps carries 2-3 suggested next questions (SP-15 B #435). Only the
	// done event of a grounded_content turn may carry them; generation is a
	// speculative non-streaming call started at the first answer delta and
	// joined before done assembly with a bounded budget — a miss, timeout or
	// parse failure leaves the field empty (progressive enhancement, never a
	// stream failure). v1 does not persist follow-ups.
	FollowUps      []string `json:"follow_ups,omitempty"`
	Degraded       bool     `json:"degraded"`
	DegradedReason string   `json:"degraded_reason,omitempty"`
	ErrorCode      string   `json:"error_code,omitempty"`
	ErrorMessage   string   `json:"error_message,omitempty"`
}

// AgentAnswerKind is a server-owned enum that determines whether an answer
// must carry citations. The model never chooses citation requirements; the
// value is fixed by the request surface and the service execution path.
type AgentAnswerKind string

const (
	// AgentAnswerGroundedContent marks natural-language answers produced for
	// chat/search/detail/usage-guide surfaces; they must retain at least one
	// valid, visibility-rechecked citation.
	AgentAnswerGroundedContent AgentAnswerKind = "grounded_content"
	// AgentAnswerNoEvidence marks answers without valid evidence; the caller
	// falls back to keyword search instead of a fabricated citation.
	AgentAnswerNoEvidence AgentAnswerKind = "no_evidence"
	// AgentAnswerConversational marks turns that never needed evidence in the
	// first place (SP-15 A2): a deterministic server-side lane admitting only
	// zero-tool, zero-citation, non-degraded replies within the configured
	// rune guardrail. The model never opts in itself; every condition is
	// checked server-side after the stream finishes.
	AgentAnswerConversational AgentAnswerKind = "conversational"
	// AgentAnswerPublishSuggestion is the typed contract for publish-metadata
	// suggestions; it carries no site-content citation requirement.
	AgentAnswerPublishSuggestion AgentAnswerKind = "publish_suggestion"
)

// AgentToolStatus values reported in AgentToolExecution.
const (
	AgentToolStatusSuccess = "success"
	AgentToolStatusError   = "error"
	AgentToolStatusSkipped = "skipped"
)

// AgentCitation is a server-normalized reference to viewer-visible content.
// It is rebuilt from backend-owned content summaries, never from model-authored
// URLs, so it always carries a valid content_id/title/zone. zone="ip" (SP-19
// G2-1) references approved IP hubs instead of content chunks: no version/
// chunk/source provenance, route=/ip/{id}, and an optional category slug.
type AgentCitation struct {
	ContentID      int64  `json:"content_id"`
	ContentVersion int    `json:"content_version"`
	ChunkKey       string `json:"chunk_key"`
	ChunkIndex     int    `json:"chunk_index"`
	Title          string `json:"title"`
	Zone           string `json:"zone"`
	Route          string `json:"route"`
	Excerpt        string `json:"excerpt"`
	Source         string `json:"source"`
	Category       string `json:"category,omitempty"`
}

// MarshalJSON keeps the pre-RAG citation contract stable while preserving the
// complete RAG provenance contract, including a valid zero-based chunk index.
// zone="ip" citations serialize the IP shape: no chunk provenance, an optional
// category slug, route=/ip/{id}.
func (c AgentCitation) MarshalJSON() ([]byte, error) {
	if c.Zone == "ip" {
		return json.Marshal(struct {
			ContentID int64  `json:"content_id"`
			Title     string `json:"title"`
			Zone      string `json:"zone"`
			Route     string `json:"route"`
			Excerpt   string `json:"excerpt,omitempty"`
			Category  string `json:"category,omitempty"`
		}{
			ContentID: c.ContentID,
			Title:     c.Title,
			Zone:      c.Zone,
			Route:     c.Route,
			Excerpt:   c.Excerpt,
			Category:  c.Category,
		})
	}
	if c.ContentVersion == 0 && c.ChunkKey == "" && c.ChunkIndex == 0 && c.Route == "" && c.Source == "" {
		return json.Marshal(struct {
			ContentID int64  `json:"content_id"`
			Title     string `json:"title"`
			Zone      string `json:"zone"`
			Excerpt   string `json:"excerpt"`
		}{
			ContentID: c.ContentID,
			Title:     c.Title,
			Zone:      c.Zone,
			Excerpt:   c.Excerpt,
		})
	}

	return json.Marshal(struct {
		ContentID      int64  `json:"content_id"`
		ContentVersion int    `json:"content_version"`
		ChunkKey       string `json:"chunk_key"`
		ChunkIndex     int    `json:"chunk_index"`
		Title          string `json:"title"`
		Zone           string `json:"zone"`
		Route          string `json:"route"`
		Excerpt        string `json:"excerpt"`
		Source         string `json:"source"`
	}{
		ContentID:      c.ContentID,
		ContentVersion: c.ContentVersion,
		ChunkKey:       c.ChunkKey,
		ChunkIndex:     c.ChunkIndex,
		Title:          c.Title,
		Zone:           c.Zone,
		Route:          c.Route,
		Excerpt:        c.Excerpt,
		Source:         c.Source,
	})
}

// AgentToolExecution reports one registered tool invocation without exposing
// raw arguments or internal reasoning.
type AgentToolExecution struct {
	Name string `json:"name"`
	// External marks bridged/external tools (MCP, image generation) so the
	// process panel and trace waterfall can badge them (SP-23 M5).
	External bool `json:"external,omitempty"`
	// ArgsSummary is a server-derived, display-safe argument summary (for
	// example the search query or the requested content id); raw tool
	// argument JSON is never serialized into events.
	ArgsSummary string `json:"args_summary,omitempty"`
	// Hits counts how many retrievable items the tool returned (search
	// result count, 1/0 for a detail lookup).
	Hits       int    `json:"hits"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
}

// AgentUsage carries observed token counts for the request. It is an
// observability value, not a monetary ledger.
type AgentUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// AgentAnswer is the typed success contract for grounded agent responses.
type AgentAnswer struct {
	TraceID    string               `json:"trace_id"`
	AnswerKind AgentAnswerKind      `json:"answer_kind"`
	Answer     string               `json:"answer"`
	Citations  []AgentCitation      `json:"citations"`
	Tools      []AgentToolExecution `json:"tools"`
	Usage      AgentUsage           `json:"usage"`
	Degraded   bool                 `json:"degraded"`
}

// AgentErrorDTO is the safe, non-raw error contract exposed to clients. Raw
// Provider errors must never be serialized into this shape.
type AgentErrorDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// citationsToModel maps the stream citation contract onto the persistence
// shape (N4): storage keeps the complete 9-field form including RAG
// provenance; the stream-side legacy-minimal MarshalJSON stays a wire
// concern only.
func citationsToModel(citations []AgentCitation) []model.AgentCitation {
	if len(citations) == 0 {
		return nil
	}
	out := make([]model.AgentCitation, len(citations))
	for i := range citations {
		out[i] = model.AgentCitation{
			ContentID:      citations[i].ContentID,
			ContentVersion: citations[i].ContentVersion,
			ChunkKey:       citations[i].ChunkKey,
			ChunkIndex:     citations[i].ChunkIndex,
			Title:          citations[i].Title,
			Zone:           citations[i].Zone,
			Route:          citations[i].Route,
			Excerpt:        citations[i].Excerpt,
			Source:         citations[i].Source,
			Category:       citations[i].Category,
		}
	}
	return out
}
