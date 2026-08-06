package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/repository"
)

// Task 2: grounded read-only tool orchestration. The registry is server-owned
// and immutable; content text can never add, rename, or alter tool
// definitions. All tools resolve content through the same viewer-aware
// visibility resolver used by chat context and the usage-guide endpoints.

var (
	// ErrAgentToolUnknown rejects unregistered or injection-shaped tool names.
	ErrAgentToolUnknown = errors.New("unknown agent tool")
	// ErrAgentToolInvalidArgs rejects unknown fields, wrong types, and
	// out-of-bounds tool arguments.
	ErrAgentToolInvalidArgs = errors.New("invalid agent tool arguments")
	// ErrAgentToolSnapshotRequired is returned when suggest_publish_metadata is
	// invoked without the typed publish-form snapshot bound to the request.
	ErrAgentToolSnapshotRequired = errors.New("publish snapshot required for suggest_publish_metadata")
)

const (
	// ToolSearchContent finds viewer-visible content by semantic similarity.
	ToolSearchContent = "search_content"
	// ToolGetContentDetail returns a compact server-owned summary for one
	// viewer-visible content item.
	ToolGetContentDetail = "get_content_detail"
	// ToolGetUsageGuide returns the usage guide for one viewer-visible item.
	ToolGetUsageGuide = "get_usage_guide"
	// ToolSuggestPublishMetadata reads ONLY the typed publish-form snapshot
	// bound to the current request; it accepts no model-authored resource
	// arguments (no content IDs, no draft IDs, no route names).
	ToolSuggestPublishMetadata = "suggest_publish_metadata"

	// defaultMaxToolQueryLength bounds search_content query length.
	defaultMaxToolQueryLength = 200
	// defaultMaxToolResultCount bounds the number of search results returned.
	defaultMaxToolResultCount = 10
)

// AgentPublishSnapshot is the typed, length-bounded publish-form snapshot that
// the client explicitly attached to the current request. The model cannot
// supply arbitrary draft/content IDs; suggest_publish_metadata only reads this
// snapshot.
type AgentPublishSnapshot struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	ContentType string   `json:"content_type"`
	Tags        []string `json:"tags"`
}

// AgentContentSummary is the compact, server-owned content summary returned by
// get_content_detail. It is rebuilt from the database, never from model output.
type AgentContentSummary struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Zone        string   `json:"zone"`
	ContentType string   `json:"content_type"`
	Excerpt     string   `json:"excerpt,omitempty"`
}

// AgentToolOutcome carries one tool execution result. Only the matching field
// is populated; raw arguments and internal reasoning are never exposed.
type AgentToolOutcome struct {
	Execution AgentToolExecution  `json:"execution"`
	Detail    *AgentContentSummary `json:"detail,omitempty"`
	Guide     *UsageGuideResult    `json:"guide,omitempty"`
	Search    []ContentSummary     `json:"search,omitempty"`
	Suggest   *UploadAssistResult  `json:"suggest,omitempty"`
}

// AgentToolPolicy exposes the config-driven budget used to stop the tool loop
// with a stable result.
type AgentToolPolicy struct {
	MaxCallsPerTurn int
	MaxOutputTokens int
	CitationMaxCount int
}

// AllowToolCall reports whether the tool loop may run another call.
func (p AgentToolPolicy) AllowToolCall(already int) bool {
	return already < p.MaxCallsPerTurn
}

// ToolPolicy returns the budget limits read from the active configuration.
func (s *AgentService) ToolPolicy() AgentToolPolicy {
	cfg := s.cfg.Agent
	maxCalls := cfg.MaxToolCallsPerTurn
	if maxCalls <= 0 {
		maxCalls = 8
	}
	maxOut := cfg.MaxOutputTokens
	if maxOut <= 0 {
		maxOut = 1200
	}
	maxCites := cfg.CitationMaxCount
	if maxCites <= 0 {
		maxCites = 5
	}
	return AgentToolPolicy{MaxCallsPerTurn: maxCalls, MaxOutputTokens: maxOut, CitationMaxCount: maxCites}
}

// RegisteredToolNames lists the immutable server-owned tool names.
func (s *AgentService) RegisteredToolNames() []string {
	return []string{ToolSearchContent, ToolGetContentDetail, ToolGetUsageGuide, ToolSuggestPublishMetadata}
}

// ToolDefinitions returns server-owned tool definitions for provider
// advertisement. They are built from constants; content cannot change them.
func (s *AgentService) ToolDefinitions() []llm.ToolDefinition {
	return []llm.ToolDefinition{
		{
			Name:        ToolSearchContent,
			Description: "Search viewer-visible published content. Returns compact content summaries.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "natural-language search query"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        ToolGetContentDetail,
			Description: "Get a compact summary of one viewer-visible content item.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"content_id": map[string]interface{}{"type": "integer", "description": "content id"},
				},
				"required": []string{"content_id"},
			},
		},
		{
			Name:        ToolGetUsageGuide,
			Description: "Get the usage guide for one viewer-visible content item.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"content_id": map[string]interface{}{"type": "integer", "description": "content id"},
				},
				"required": []string{"content_id"},
			},
		},
		{
			Name:        ToolSuggestPublishMetadata,
			Description: "Suggest metadata for the publish form snapshot bound to the current request. Accepts no arguments.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

// decodeToolArgs strictly decodes model-supplied tool arguments: unknown
// fields, wrong types, and malformed JSON are rejected.
func decodeToolArgs(raw json.RawMessage, out any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return ErrAgentToolInvalidArgs
	}
	return nil
}

// resolveVisibleContent is the single viewer-aware content resolver shared by
// tool calls, chat context, and both usage-guide response modes. Every failure
// mode returns the same ErrContentNotFound so hidden content cannot be probed.
func (s *AgentService) resolveVisibleContent(ctx context.Context, viewerID, contentID int64) (*model.ContentItem, error) {
	if s.db == nil {
		return nil, ErrContentNotFound
	}
	var content model.ContentItem
	err := repository.ApplyContentVisibilityScope(s.db.WithContext(ctx).Model(&model.ContentItem{}), viewerID).
		Where("content_items.id = ?", contentID).
		First(&content).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Error("agent visibility resolution failed", "content_id", contentID, "error", err)
		}
		return nil, ErrContentNotFound
	}
	return &content, nil
}

// ExecuteTool validates and executes one registered read-only tool. Unknown
// names and invalid arguments never reach a provider or the database query
// beyond the shared visibility resolver.
func (s *AgentService) ExecuteTool(ctx context.Context, name string, rawArgs json.RawMessage, viewerID int64, snapshot *AgentPublishSnapshot) (*AgentToolOutcome, error) {
	start := time.Now()

	switch name {
	case ToolSearchContent:
		outcome, err := s.toolSearchContent(ctx, rawArgs, viewerID)
		return outcome, withToolError(outcome, name, err, start)
	case ToolGetContentDetail:
		outcome, err := s.toolGetContentDetail(ctx, rawArgs, viewerID)
		return outcome, withToolError(outcome, name, err, start)
	case ToolGetUsageGuide:
		outcome, err := s.toolGetUsageGuide(ctx, rawArgs, viewerID)
		return outcome, withToolError(outcome, name, err, start)
	case ToolSuggestPublishMetadata:
		outcome, err := s.toolSuggestPublishMetadata(ctx, rawArgs, snapshot)
		return outcome, withToolError(outcome, name, err, start)
	default:
		return nil, ErrAgentToolUnknown
	}
}

func withToolError(outcome *AgentToolOutcome, name string, err error, start time.Time) error {
	if outcome == nil {
		outcome = &AgentToolOutcome{}
	}
	status := AgentToolStatusSuccess
	if err != nil {
		status = AgentToolStatusError
	}
	outcome.Execution = AgentToolExecution{Name: name, Status: status, DurationMs: time.Since(start).Milliseconds()}
	return err
}

type searchToolArgs struct {
	Query string `json:"query"`
}

func (s *AgentService) toolSearchContent(ctx context.Context, rawArgs json.RawMessage, viewerID int64) (*AgentToolOutcome, error) {
	var args searchToolArgs
	if err := decodeToolArgs(rawArgs, &args); err != nil {
		return nil, err
	}
	query := strings.TrimSpace(args.Query)
	if query == "" || len([]rune(query)) > defaultMaxToolQueryLength {
		return nil, ErrAgentToolInvalidArgs
	}
	if s.vectorSearch == nil || s.embeddingRepo == nil {
		return nil, errors.New("vector search unavailable")
	}
	embedding, err := s.llmProvider.GetEmbedding(ctx, query)
	if err != nil {
		return nil, err
	}
	results, err := s.vectorSearch(embedding, 20)
	if err != nil {
		return nil, err
	}
	contentIDs := make([]int64, 0, len(results))
	scoreMap := make(map[int64]float64, len(results))
	for _, r := range results {
		contentIDs = append(contentIDs, r.ContentItemID)
		scoreMap[r.ContentItemID] = r.Score
	}
	contents, err := s.listVisibleNLSearchContents(contentIDs, viewerID)
	if err != nil {
		return nil, err
	}
	summaries := make([]ContentSummary, 0, len(contents))
	for _, content := range contents {
		summaries = append(summaries, ContentSummary{
			ID:          content.ID,
			Title:       content.Title,
			ContentType: content.ContentType,
			Score:       scoreMap[content.ID],
		})
	}
	if len(summaries) > defaultMaxToolResultCount {
		summaries = summaries[:defaultMaxToolResultCount]
	}
	return &AgentToolOutcome{Search: summaries}, nil
}

type contentIDToolArgs struct {
	ContentID int64 `json:"content_id"`
}

func (s *AgentService) toolGetContentDetail(ctx context.Context, rawArgs json.RawMessage, viewerID int64) (*AgentToolOutcome, error) {
	var args contentIDToolArgs
	if err := decodeToolArgs(rawArgs, &args); err != nil {
		return nil, err
	}
	if args.ContentID <= 0 {
		return nil, ErrAgentToolInvalidArgs
	}
	content, err := s.resolveVisibleContent(ctx, viewerID, args.ContentID)
	if err != nil {
		return nil, err
	}
	summary := &AgentContentSummary{
		ID:          content.ID,
		Title:       content.Title,
		Zone:        content.Zone,
		ContentType: content.ContentType,
		Excerpt:     truncateRunes(strings.TrimSpace(content.Description), 240),
	}
	return &AgentToolOutcome{Detail: summary}, nil
}

func (s *AgentService) toolGetUsageGuide(ctx context.Context, rawArgs json.RawMessage, viewerID int64) (*AgentToolOutcome, error) {
	var args contentIDToolArgs
	if err := decodeToolArgs(rawArgs, &args); err != nil {
		return nil, err
	}
	if args.ContentID <= 0 {
		return nil, ErrAgentToolInvalidArgs
	}
	if _, err := s.resolveVisibleContent(ctx, viewerID, args.ContentID); err != nil {
		return nil, err
	}
	guide, err := s.UsageGuide(ctx, viewerID, args.ContentID)
	if err != nil {
		return nil, err
	}
	return &AgentToolOutcome{Guide: guide}, nil
}

// toolSuggestPublishMetadata accepts NO model-authored resource arguments. It
// only reads the typed publish-form snapshot bound to the current request.
func (s *AgentService) toolSuggestPublishMetadata(ctx context.Context, rawArgs json.RawMessage, snapshot *AgentPublishSnapshot) (*AgentToolOutcome, error) {
	var empty struct{}
	if err := decodeToolArgs(rawArgs, &empty); err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, ErrAgentToolSnapshotRequired
	}
	result, err := s.UploadAssist(ctx, 0, truncateRunes(snapshot.Title, uploadAssistMaxTitleLength), truncateRunes(snapshot.Description, uploadAssistMaxDescLength), "", snapshot.ContentType)
	if err != nil {
		return nil, err
	}
	return &AgentToolOutcome{Suggest: result}, nil
}

// RevalidateCitations reloads every citation through the viewer-aware
// visibility resolver after model output. Citations without valid
// id/title/zone or pointing at hidden content are dropped.
func (s *AgentService) RevalidateCitations(ctx context.Context, viewerID int64, citations []AgentCitation) []AgentCitation {
	policy := s.ToolPolicy()
	valid := make([]AgentCitation, 0, len(citations))
	for _, c := range citations {
		if c.ContentID <= 0 || c.Title == "" || c.Zone == "" {
			continue
		}
		if _, err := s.resolveVisibleContent(ctx, viewerID, c.ContentID); err != nil {
			continue
		}
		valid = append(valid, c)
		if len(valid) >= policy.CitationMaxCount {
			break
		}
	}
	return valid
}

// ClassifyGroundedAnswer deterministically decides whether a natural-language
// answer may be grounded_content. The model never chooses citation
// requirements: without at least one valid citation the answer is no_evidence.
func ClassifyGroundedAnswer(citations []AgentCitation) AgentAnswerKind {
	if len(citations) == 0 {
		return AgentAnswerNoEvidence
	}
	return AgentAnswerGroundedContent
}

// newTraceID returns a 32-hex trace id for one agent request.
func newTraceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%032x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// traceAgentEvent emits a safe structured trace line. Raw prompts, full
// private content, and raw Provider errors are never logged.
func traceAgentEvent(traceID string, event string, fields ...any) {
	args := append([]any{"trace_id", traceID, "event", event}, fields...)
	slog.Info("[agent] trace", args...)
}
