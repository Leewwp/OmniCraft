package service

import (
	"omnicraft/backend/internal/agentmcp"

	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/observability"
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
	// ToolSearchIPs finds approved public IPs by keyword and optional
	// category (SP-19 G2-1).
	ToolSearchIPs = "search_ips"
	// ToolGetContentDetail returns a compact server-owned summary for one
	// viewer-visible content item.
	ToolGetContentDetail = "get_content_detail"
	// ToolGetUsageGuide returns the usage guide for one viewer-visible item.
	ToolGetUsageGuide = "get_usage_guide"
	// ToolSuggestPublishMetadata reads ONLY the typed publish-form snapshot
	// bound to the current request; it accepts no model-authored resource
	// arguments (no content IDs, no draft IDs, no route names).
	ToolSuggestPublishMetadata = "suggest_publish_metadata"

	// defaultMaxToolQueryLength bounds search_content and search_ips query
	// length.
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
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Zone        string `json:"zone"`
	ContentType string `json:"content_type"`
	Excerpt     string `json:"excerpt,omitempty"`
}

// AgentIPSummary is the compact, server-owned IP summary returned by
// search_ips. Rebuilt from the database, never from model output.
type AgentIPSummary struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug,omitempty"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
}

// AgentToolOutcome carries one tool execution result. Only the matching field
// is populated; raw arguments and internal reasoning are never exposed.
type AgentToolOutcome struct {
	Execution AgentToolExecution   `json:"execution"`
	Detail    *AgentContentSummary `json:"detail,omitempty"`
	Guide     *UsageGuideResult    `json:"guide,omitempty"`
	Search    []ContentSummary     `json:"search,omitempty"`
	IPs       []AgentIPSummary     `json:"ips,omitempty"`
	Suggest   *UploadAssistResult  `json:"suggest,omitempty"`
	// Image carries the generate_image outcome (SP-23 M1): platform-owned
	// OSS coordinates only, never the provider URL.
	Image            *AgentImageResult `json:"image,omitempty"`
	MCP              *AgentMCPResult   `json:"mcp,omitempty"`
	Degraded         bool              `json:"-"`
	RetrievalSources map[string]string `json:"-"`
	ExpandedQueries  []string          `json:"-"`
	// QueryTruncated marks that the tool-layer entry normalization cut the
	// query to defaultMaxToolQueryLength runes (#619); the stream loop
	// surfaces it in the tool step summary and trace extra.
	QueryTruncated bool `json:"-"`
}

// AgentToolPolicy exposes the config-driven budget used to stop the tool loop
// with a stable result.
type AgentToolPolicy struct {
	MaxCallsPerTurn  int
	MaxOutputTokens  int
	CitationMaxCount int
}

// Valid reports whether every provider-facing budget is explicitly configured.
// A zero value is intentionally invalid: release and enabled development
// environments must not silently inherit policy from code.
func (p AgentToolPolicy) Valid() bool {
	return p.MaxCallsPerTurn > 0 && p.MaxOutputTokens > 0 && p.CitationMaxCount > 0
}

// AllowToolCall reports whether the tool loop may run another call.
func (p AgentToolPolicy) AllowToolCall(already int) bool {
	return already < p.MaxCallsPerTurn
}

// ToolPolicy returns the budget limits read from the active configuration.
func (s *AgentService) ToolPolicy() AgentToolPolicy {
	if s == nil || s.cfg == nil {
		return AgentToolPolicy{}
	}
	cfg := s.cfg.Agent
	return AgentToolPolicy{
		MaxCallsPerTurn:  cfg.MaxToolCallsPerTurn,
		MaxOutputTokens:  cfg.MaxOutputTokens,
		CitationMaxCount: cfg.CitationMaxCount,
	}
}

// RegisteredToolNames lists the immutable server-owned tool names.
func (s *AgentService) RegisteredToolNames() []string {
	return []string{ToolSearchContent, ToolSearchIPs, ToolGetContentDetail, ToolGetUsageGuide, ToolSuggestPublishMetadata}
}

// ToolDefinitions returns server-owned tool definitions for provider
// advertisement. They are built from constants; content cannot change them.
// generate_image joins only when the image provider is configured (switch +
// key + store), so the model never sees a dead tool (SP-23 M1 fail-closed).
func (s *AgentService) ToolDefinitions() []llm.ToolDefinition {
	defs := s.baseToolDefinitions()
	if s.imageToolAvailable() {
		defs = append(defs, llm.ToolDefinition{
			Name:        ToolGenerateImage,
			Description: "Generate one illustration from a Chinese text description and return the platform-hosted image URL. Use for covers or scene art the user asks for.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt": map[string]interface{}{"type": "string", "description": "image description in Chinese"},
					"size": map[string]interface{}{
						"type":        "string",
						"description": "optional aspect; one of: " + strings.Join(s.agentImageCfg.SizeOptions, ", "),
					},
				},
				"required": []string{"prompt"},
			},
		})
	}
	return defs
}

func (s *AgentService) baseToolDefinitions() []llm.ToolDefinition {
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
			Name:        ToolSearchIPs,
			Description: "Search approved public IPs (original settings/worlds) by keyword, optionally filtered by category. Returns compact IP summaries.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "self-contained keyword query matched against IP name and description"},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "optional IP category slug; only these values are valid: " + s.ipCategorySlugList(),
					},
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

// agentToolScope carries the per-turn execution context into tool handlers:
// viewer identity, the conversation for budget-scoped tools, the live-turn
// image count (the durable #538 rows only land at end of turn), and the
// typed publish snapshot.
type agentToolScope struct {
	ViewerID       int64
	ConversationID int64
	TurnImages     int
	Snapshot       *AgentPublishSnapshot
}

type agentToolHandler func(context.Context, json.RawMessage, agentToolScope) (*AgentToolOutcome, error)

// toolRegistry is local and fixed: callers cannot register tools at runtime.
func (s *AgentService) toolRegistry() map[string]agentToolHandler {
	return map[string]agentToolHandler{
		ToolSearchContent: func(ctx context.Context, args json.RawMessage, scope agentToolScope) (*AgentToolOutcome, error) {
			return s.toolSearchContent(ctx, args, scope.ViewerID)
		},
		ToolSearchIPs: func(ctx context.Context, args json.RawMessage, scope agentToolScope) (*AgentToolOutcome, error) {
			return s.toolSearchIPs(ctx, args)
		},
		ToolGetContentDetail: func(ctx context.Context, args json.RawMessage, scope agentToolScope) (*AgentToolOutcome, error) {
			return s.toolGetContentDetail(ctx, args, scope.ViewerID)
		},
		ToolGetUsageGuide: func(ctx context.Context, args json.RawMessage, scope agentToolScope) (*AgentToolOutcome, error) {
			return s.toolGetUsageGuide(ctx, args, scope.ViewerID)
		},
		ToolSuggestPublishMetadata: func(ctx context.Context, args json.RawMessage, scope agentToolScope) (*AgentToolOutcome, error) {
			return s.toolSuggestPublishMetadata(ctx, args, scope.Snapshot)
		},
		ToolGenerateImage: func(ctx context.Context, args json.RawMessage, scope agentToolScope) (*AgentToolOutcome, error) {
			return s.toolGenerateImage(ctx, args, scope)
		},
	}
}

// ExecuteTool validates and executes one registered read-only tool. Unknown
// names and invalid arguments never reach a provider or visibility query.
// Conversation-scoped tools (the image budget) see conversationID=0 here and
// only enforce the live-turn budget; the stream loop passes the real id via
// ExecuteToolInConversation.
func (s *AgentService) ExecuteTool(ctx context.Context, name string, rawArgs json.RawMessage, viewerID int64, snapshot *AgentPublishSnapshot) (*AgentToolOutcome, error) {
	return s.ExecuteToolInConversation(ctx, name, rawArgs, viewerID, 0, 0, snapshot)
}

// ExecuteToolInConversation additionally binds the conversation id and the
// live-turn image count for per-conversation budgets (SP-23 M1 image quota).
func (s *AgentService) ExecuteToolInConversation(ctx context.Context, name string, rawArgs json.RawMessage, viewerID, conversationID int64, turnImages int, snapshot *AgentPublishSnapshot) (*AgentToolOutcome, error) {
	start := time.Now()
	// SP-23 M3: bridged external tools route through the MCP seam before
	// the local registry (the mcp_ namespace cannot collide with it).
	if strings.HasPrefix(name, agentmcp.ToolNamePrefix) {
		if s.mcpBridge == nil {
			return nil, withToolError(nil, name, ErrAgentToolUnknown, start)
		}
		outcome, err := s.mcpToolOutcome(ctx, name, rawArgs)
		return outcome, withToolError(outcome, name, err, start)
	}
	handler, ok := s.toolRegistry()[name]
	if !ok {
		return nil, ErrAgentToolUnknown
	}
	outcome, err := handler(ctx, rawArgs, agentToolScope{
		ViewerID: viewerID, ConversationID: conversationID,
		TurnImages: turnImages, Snapshot: snapshot,
	})
	return outcome, withToolError(outcome, name, err, start)
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
	observability.IncDefaultAgentToolCall(name, err != nil)
	return err
}

type searchToolArgs struct {
	Query string `json:"query"`
}

// normalizeSearchQuery is the single tool-layer entry normalization for
// search_content (#619, user ruling 2026-09-19 direction 2): trim, then
// truncate an over-length query to defaultMaxToolQueryLength runes and keep
// searching instead of rejecting it. The retrieval side itself has no such
// limit; the 200-rune line stays put (no widening, no refusal). The truncated
// flag feeds the execution summary and trace so "why did it find nothing"
// investigations can see the cut.
func normalizeSearchQuery(raw string) (query string, truncated bool) {
	query = strings.TrimSpace(raw)
	if runes := []rune(query); len(runes) > defaultMaxToolQueryLength {
		return string(runes[:defaultMaxToolQueryLength]), true
	}
	return query, false
}

func (s *AgentService) toolSearchContent(ctx context.Context, rawArgs json.RawMessage, viewerID int64) (*AgentToolOutcome, error) {
	var args searchToolArgs
	if err := decodeToolArgs(rawArgs, &args); err != nil {
		return nil, err
	}
	query, truncated := normalizeSearchQuery(args.Query)
	if query == "" {
		return nil, ErrAgentToolInvalidArgs
	}
	if s.ragHybridEnabled() {
		if s.hybridRetriever == nil {
			return nil, errors.New("hybrid retrieval unavailable")
		}
		result, err := s.hybridRetriever.Retrieve(ctx, query, viewerID)
		if err != nil {
			return nil, err
		}
		summaries := retrievalSummaries(result.Candidates)
		return &AgentToolOutcome{
			Search:           summaries,
			Degraded:         result.Degraded != "",
			RetrievalSources: retrievalSourceMap(summaries),
			ExpandedQueries:  result.ExpandedQueries,
			QueryTruncated:   truncated,
		}, nil
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
	return &AgentToolOutcome{Search: summaries, QueryTruncated: truncated}, nil
}

type searchIPsToolArgs struct {
	Query    string `json:"query"`
	Category string `json:"category"`
}

// ipCategorySlugList renders the config allowlist for the tool schema so the
// model can only ever pass legal slugs (SP-19 G2-1). Unknown config states
// degrade to an explicit note instead of an empty hint.
func (s *AgentService) ipCategorySlugList() string {
	if s == nil || s.cfg == nil || len(s.cfg.IPCategories) == 0 {
		return "(none configured)"
	}
	return strings.Join(s.cfg.IPCategories, ", ")
}

// toolSearchIPs resolves approved public IPs through the keyword search seam
// (tsvector + ILIKE fallback). Category filters are validated against the
// config allowlist before any query runs; results are rebuilt as server-owned
// summaries, never from model output.
func (s *AgentService) toolSearchIPs(ctx context.Context, rawArgs json.RawMessage) (*AgentToolOutcome, error) {
	var args searchIPsToolArgs
	if err := decodeToolArgs(rawArgs, &args); err != nil {
		return nil, err
	}
	query := strings.TrimSpace(args.Query)
	if query == "" || len([]rune(query)) > defaultMaxToolQueryLength {
		return nil, ErrAgentToolInvalidArgs
	}
	category := strings.TrimSpace(args.Category)
	if category != "" && !s.ipCategoryAllowed(category) {
		return nil, ErrAgentToolInvalidArgs
	}
	if s.ipSearch == nil {
		return nil, errors.New("ip search unavailable")
	}
	limit := s.ipSearchLimit()
	ips, err := s.ipSearch(ctx, query, category, limit)
	if err != nil {
		return nil, err
	}
	// 分类推荐场景（「推荐几个游戏类的 IP」）模型自然给 query=「游戏」这类
	// 泛词，而全文检索只匹配具体 IP 名/简介，会空手而归。命中 0 且带分类
	// 过滤时回退为纯分类浏览（与 REST GET /ips 的空 q + category 形态一致），
	// 只重试一次、不吞错误。
	if len(ips) == 0 && category != "" {
		ips, err = s.ipSearch(ctx, "", category, limit)
		if err != nil {
			return nil, err
		}
	}
	summaries := make([]AgentIPSummary, 0, len(ips))
	for _, ip := range ips {
		summaries = append(summaries, AgentIPSummary{
			ID:          ip.ID,
			Name:        ip.Name,
			Slug:        ip.Slug,
			Category:    ip.Category,
			Description: truncateRunes(strings.TrimSpace(ip.Description), 120),
		})
	}
	return &AgentToolOutcome{IPs: summaries}, nil
}

// ipCategoryAllowed checks a slug against the config allowlist; an
// unconfigured allowlist accepts no category filter at all.
func (s *AgentService) ipCategoryAllowed(slug string) bool {
	if s == nil || s.cfg == nil {
		return false
	}
	for _, allowed := range s.cfg.IPCategories {
		if allowed == slug {
			return true
		}
	}
	return false
}

// ipSearchLimit keeps the tool result size on the configured retrieval budget
// (rag.hybrid.final_topk) with the shared tool-result bound as the floor.
func (s *AgentService) ipSearchLimit() int {
	if s != nil && s.cfg != nil && s.cfg.RAG.Hybrid.FinalTopK > 0 {
		return s.cfg.RAG.Hybrid.FinalTopK
	}
	return defaultMaxToolResultCount
}

func retrievalSourceMap(summaries []ContentSummary) map[string]string {
	sources := make(map[string]string, len(summaries))
	for _, summary := range summaries {
		if strings.TrimSpace(summary.ChunkKey) != "" && validCitationSource(summary.Source) {
			sources[summary.ChunkKey] = summary.Source
		}
	}
	return sources
}

func retrievalSummaries(candidates []AgentRetrievalCandidate) []ContentSummary {
	summaries := make([]ContentSummary, 0, len(candidates))
	for _, candidate := range candidates {
		citation, ok := citationFromRetrievalCandidate(candidate)
		if !ok {
			continue
		}
		summaries = append(summaries, ContentSummary{
			ID:             citation.ContentID,
			Title:          citation.Title,
			Zone:           citation.Zone,
			ContentType:    candidate.ContentType,
			ContentVersion: citation.ContentVersion,
			ChunkKey:       citation.ChunkKey,
			ChunkIndex:     citation.ChunkIndex,
			Excerpt:        citation.Excerpt,
			Source:         citation.Source,
		})
	}
	return summaries
}

func citationFromSearchSummary(summary ContentSummary) (AgentCitation, bool) {
	if summary.ID <= 0 || summary.ContentVersion <= 0 || summary.ChunkIndex < 0 ||
		strings.TrimSpace(summary.ChunkKey) == "" || strings.TrimSpace(summary.Title) == "" ||
		strings.TrimSpace(summary.Zone) == "" || strings.TrimSpace(summary.Excerpt) == "" ||
		!validCitationSource(summary.Source) {
		return AgentCitation{}, false
	}
	return AgentCitation{
		ContentID:      summary.ID,
		ContentVersion: summary.ContentVersion,
		ChunkKey:       summary.ChunkKey,
		ChunkIndex:     summary.ChunkIndex,
		Title:          summary.Title,
		Zone:           summary.Zone,
		Route:          contentRoute(summary.Zone, summary.ID),
		Excerpt:        summary.Excerpt,
		Source:         summary.Source,
	}, true
}

func citationFromRetrievalCandidate(candidate AgentRetrievalCandidate) (AgentCitation, bool) {
	if candidate.ContentID <= 0 || candidate.ContentVersion <= 0 || candidate.ChunkIndex < 0 ||
		candidate.ChunkKey == "" || strings.TrimSpace(candidate.Title) == "" || strings.TrimSpace(candidate.Text) == "" {
		return AgentCitation{}, false
	}
	if candidate.Zone != "original" && candidate.Zone != "fanwork" {
		return AgentCitation{}, false
	}
	if !validCitationSource(candidate.Source) {
		return AgentCitation{}, false
	}
	return AgentCitation{
		ContentID:      candidate.ContentID,
		ContentVersion: candidate.ContentVersion,
		ChunkKey:       candidate.ChunkKey,
		ChunkIndex:     candidate.ChunkIndex,
		Title:          strings.TrimSpace(candidate.Title),
		Zone:           candidate.Zone,
		Route:          contentRoute(candidate.Zone, candidate.ContentID),
		Excerpt:        truncateRunes(strings.TrimSpace(candidate.Text), 240),
		Source:         candidate.Source,
	}, true
}

func contentRoute(zone string, contentID int64) string {
	if zone == "original" {
		return fmt.Sprintf("/original/%d", contentID)
	}
	return fmt.Sprintf("/content/%d", contentID)
}

// ipRoute is the canonical hub route for IP citations (SP-19 G2-1, Q5).
func ipRoute(ipID int64) string {
	return fmt.Sprintf("/ip/%d", ipID)
}

// citationFromIPSummary shapes an approved-IP search result as a zone="ip"
// citation: no chunk provenance, route=/ip/{id}, optional category slug.
func citationFromIPSummary(summary AgentIPSummary) (AgentCitation, bool) {
	if summary.ID <= 0 || strings.TrimSpace(summary.Name) == "" {
		return AgentCitation{}, false
	}
	return AgentCitation{
		ContentID: summary.ID,
		Title:     strings.TrimSpace(summary.Name),
		Zone:      "ip",
		Route:     ipRoute(summary.ID),
		Excerpt:   truncateRunes(strings.TrimSpace(summary.Description), 160),
		Category:  summary.Category,
	}, true
}

func validCitationSource(source string) bool {
	return source == "bm25" || source == "vector" || source == "hybrid_rrf"
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
	// Structured-first applies to the in-chat tool too (SP-16 #447):
	// persisted specifics render without an LLM round-trip.
	guide, err := s.UsageGuide(ctx, viewerID, args.ContentID, false)
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

type citationTruth = repository.CitationTruth

// RevalidateCitations reloads every citation through the viewer-aware current
// chunk truth after model output. Only server-shaped citations that still
// match the latest published chunk can reach SSE.
func (s *AgentService) RevalidateCitations(ctx context.Context, viewerID int64, citations []AgentCitation) []AgentCitation {
	traceID := observability.TraceID(ctx)
	if traceID == "" {
		traceID = untracedTraceID
	}
	return s.revalidateCitations(ctx, viewerID, citations, traceID)
}

func (s *AgentService) revalidateCitations(ctx context.Context, viewerID int64, citations []AgentCitation, traceID string) []AgentCitation {
	policy := s.ToolPolicy()
	valid := make([]AgentCitation, 0, len(citations))
	rejectedCount := 0
	for _, citation := range citations {
		if len(valid) >= policy.CitationMaxCount {
			rejectedCount++
			traceAgentEvent(traceID, "citation_revalidation", "accepted", false, "reason", "citation_limit", "rejected_count", rejectedCount)
			continue
		}
		reason := s.citationRejectionReason(ctx, viewerID, citation)
		if reason != "" {
			rejectedCount++
			traceAgentEvent(traceID, "citation_revalidation", "accepted", false, "reason", reason, "rejected_count", rejectedCount)
			continue
		}
		valid = append(valid, citation)
		traceAgentEvent(traceID, "citation_revalidation", "accepted", true, "reason", "accepted")
	}
	return valid
}

func (s *AgentService) citationRejectionReason(ctx context.Context, viewerID int64, citation AgentCitation) string {
	// SP-19 G2-1: zone="ip" citations revalidate against the live IP row —
	// only approved hubs stay citable, mirroring the public IP visibility
	// gate (handler/ip.go: creators keep studio access to pending/rejected
	// hubs, but agent citations are viewer-facing and cite approved only).
	if citation.Zone == "ip" {
		if citation.ContentID <= 0 || strings.TrimSpace(citation.Title) == "" || strings.TrimSpace(citation.Route) == "" {
			return "missing_fields"
		}
		if citation.Route != ipRoute(citation.ContentID) {
			return "invalid_route"
		}
		if s.db == nil {
			return "ip_lookup_failed"
		}
		var ip model.IP
		err := s.db.WithContext(ctx).Model(&model.IP{}).Where("id = ?", citation.ContentID).First(&ip).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "not_visible"
			}
			return "ip_lookup_failed"
		}
		if ip.Status != "approved" {
			return "not_visible"
		}
		if ip.Name != citation.Title {
			return "ip_metadata_mismatch"
		}
		return ""
	}
	if !s.ragHybridEnabled() {
		if citation.ContentID <= 0 || strings.TrimSpace(citation.Title) == "" || strings.TrimSpace(citation.Zone) == "" {
			return "missing_fields"
		}
		if _, err := s.resolveVisibleContent(ctx, viewerID, citation.ContentID); err != nil {
			if errors.Is(err, ErrContentNotFound) {
				return "not_visible"
			}
			return "visibility_lookup_failed"
		}
		return ""
	}
	if citation.ContentID <= 0 || citation.ContentVersion <= 0 || citation.ChunkIndex < 0 ||
		strings.TrimSpace(citation.ChunkKey) == "" || strings.TrimSpace(citation.Title) == "" ||
		strings.TrimSpace(citation.Zone) == "" || strings.TrimSpace(citation.Route) == "" ||
		strings.TrimSpace(citation.Excerpt) == "" || strings.TrimSpace(citation.Source) == "" {
		return "missing_fields"
	}
	if citation.Zone != "original" && citation.Zone != "fanwork" {
		return "invalid_zone"
	}
	if !validCitationSource(citation.Source) {
		return "invalid_source"
	}
	if citation.Route != contentRoute(citation.Zone, citation.ContentID) {
		return "invalid_route"
	}
	truth, err := s.loadCitationTruth(ctx, viewerID, citation)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "not_visible_or_current"
		}
		return "truth_lookup_failed"
	}
	if truth.ContentID != citation.ContentID || truth.ContentVersion != citation.ContentVersion ||
		truth.ChunkIndex != citation.ChunkIndex || truth.ChunkKey != citation.ChunkKey {
		return "chunk_mismatch"
	}
	if truth.Title != citation.Title || truth.Zone != citation.Zone {
		return "content_metadata_mismatch"
	}
	if truncateRunes(strings.TrimSpace(truth.Text), 240) != strings.TrimSpace(citation.Excerpt) {
		return "excerpt_mismatch"
	}
	return ""
}

func (s *AgentService) loadCitationTruth(ctx context.Context, viewerID int64, citation AgentCitation) (citationTruth, error) {
	if s.ragChunkRepo == nil {
		return citationTruth{}, gorm.ErrRecordNotFound
	}
	return s.ragChunkRepo.LoadVisibleCitationTruth(ctx, viewerID, repository.CitationLookup{
		ContentID: citation.ContentID, ContentVersion: citation.ContentVersion,
		ChunkIndex: citation.ChunkIndex, ChunkKey: citation.ChunkKey,
	})
}

func (s *AgentService) citationForContent(ctx context.Context, viewerID, contentID int64) (AgentCitation, error) {
	if s.ragChunkRepo == nil || contentID <= 0 {
		return AgentCitation{}, gorm.ErrRecordNotFound
	}
	truth, err := s.ragChunkRepo.FirstVisibleCitationTruth(ctx, viewerID, contentID)
	if err != nil {
		return AgentCitation{}, err
	}
	return AgentCitation{
		ContentID:      truth.ContentID,
		ContentVersion: truth.ContentVersion,
		ChunkKey:       truth.ChunkKey,
		ChunkIndex:     truth.ChunkIndex,
		Title:          truth.Title,
		Zone:           truth.Zone,
		Route:          contentRoute(truth.Zone, truth.ContentID),
		Excerpt:        truncateRunes(strings.TrimSpace(truth.Text), 240),
	}, nil
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

// ClassifyStreamAnswer extends the grounded classification with the
// conversational lane (SP-15 A2): a streamed reply that used no tools, kept no
// citations, is non-empty, did not degrade, and fits within the configured
// rune guardrail is conversational and survives the no_evidence clearing.
// Every condition is server-side and deterministic — the model cannot opt
// itself into the lane. Any miss (including conversationalMaxRunes <= 0,
// which disables the lane) falls back to the strict grounded classification,
// so a lazy zero-retrieval long answer on a content question is still cleared.
func ClassifyStreamAnswer(citations []AgentCitation, executedTools []AgentToolExecution, answer string, degraded bool, conversationalMaxRunes int) AgentAnswerKind {
	return ClassifyStreamAnswerWithExternal(citations, executedTools, answer, degraded, conversationalMaxRunes, 0, 1)
}

// ClassifyStreamAnswerWithExternal extends the lane for SP-23 M3: a turn
// whose executed tools were ALL external (MCP bridge / image generation)
// may keep a citation-free answer within externalMaxRunes — its grounding
// is workspace data the tool fetched, not RAG chunks, so the strict citation
// gate would otherwise clear every substantive external-tool answer. Any
// local retrieval tool in the mix keeps the strict shape.
//
// SP-24 R3: minSurvivingCitations moves the grounded boundary — a grounded
// answer keeps its text only with at least N revalidation-surviving
// citations (1 = the historical zero-citation no_evidence boundary; the
// conversational/external lanes above stay exempt by design). The
// revalidation gate itself (revalidateCitations) is untouched.
func ClassifyStreamAnswerWithExternal(citations []AgentCitation, executedTools []AgentToolExecution, answer string, degraded bool, conversationalMaxRunes, externalMaxRunes, minSurvivingCitations int) AgentAnswerKind {
	trimmed := strings.TrimSpace(answer)
	if len(citations) == 0 && trimmed != "" && !degraded {
		if len(executedTools) == 0 && conversationalMaxRunes > 0 && len([]rune(trimmed)) <= conversationalMaxRunes {
			return AgentAnswerConversational
		}
		if externalMaxRunes > 0 && allToolsExternal(executedTools) && len([]rune(trimmed)) <= externalMaxRunes {
			return AgentAnswerConversational
		}
	}
	return classifyGroundedWithBoundary(citations, minSurvivingCitations)
}

// classifyGroundedWithBoundary applies the SP-24 R3 configurable citation
// boundary (clamped to >= 1 so a zero/omitted config can never weaken the
// anti-hallucination gate below its shipped semantics).
func classifyGroundedWithBoundary(citations []AgentCitation, minSurvivingCitations int) AgentAnswerKind {
	if minSurvivingCitations < 1 {
		minSurvivingCitations = 1
	}
	if len(citations) < minSurvivingCitations {
		return AgentAnswerNoEvidence
	}
	return AgentAnswerGroundedContent
}

func allToolsExternal(tools []AgentToolExecution) bool {
	if len(tools) == 0 {
		return false
	}
	for _, t := range tools {
		if !t.External {
			return false
		}
	}
	return true
}

// untracedTraceID is an explicit marker for direct service callers that do
// not pass through HTTP tracing. Production requests replace it with the
// OTel trace ID created by the HTTP root span.
const untracedTraceID = "00000000000000000000000000000000"

// traceAgentEvent emits a safe structured trace line. Raw prompts, full
// private content, and raw Provider errors are never logged.
func traceAgentEvent(traceID string, event string, fields ...any) {
	args := append([]any{"trace_id", traceID, "event", event}, fields...)
	slog.Info("[agent] trace", args...)
}
