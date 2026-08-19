package service

import "encoding/json"

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
// URLs, so it always carries a valid content_id/title/zone.
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
}

// MarshalJSON keeps the pre-RAG citation contract stable while preserving the
// complete RAG provenance contract, including a valid zero-based chunk index.
func (c AgentCitation) MarshalJSON() ([]byte, error) {
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
	Name       string `json:"name"`
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
