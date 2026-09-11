package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/repository"
)

var ErrAgentDisabled = errors.New("web agent is disabled")
var ErrAgentFileTooLarge = errors.New("file too large for upload assist")

type AgentService struct {
	llmProvider     llm.LLMProvider
	chatStreamer    agentChatStreamer
	embeddingRepo   *repository.EmbeddingRepository
	contentRepo     *repository.ContentRepository
	searchRepo      *repository.SearchRepository
	usageGuideSvc   *UsageGuideService
	ragChunkRepo    *repository.RagChunkRepository
	hybridRetriever AgentContentRetriever
	greenClient     agentGreenScanner
	db              *gorm.DB
	cfg             *config.Config
	queueProducer   queue.Producer
	vectorSearch    func(embedding []float32, topK int) ([]repository.EmbeddingSearchResult, error)
}

// agentChatStreamer is the narrow Provider capability consumed by the Agent
// workspace streaming loop. The broader llm.LLMProvider remains available to
// legacy non-streaming and embedding callers behind the compatibility shell.
type agentChatStreamer interface {
	ChatStream(ctx context.Context, req llm.ChatRequest, handler func(delta llm.ChatDelta) error) error
}

// agentGreenScanner is the Green text scan seam consumed by the Agent chat
// guardrails (A-05): the input admission gate and the post-turn output audit.
// *aliyun.GreenClient is the production implementation; tests inject a fake.
type agentGreenScanner interface {
	TextModeration(ctx context.Context, text string) (*aliyun.GreenScanResult, error)
}

// AgentRetrievalCandidate is the service-owned boundary for RAG results. The
// concrete HybridRetriever lives in service/rag and is adapted by the
// container, keeping the AgentService contract independent of that package's
// projection dependencies.
type AgentRetrievalCandidate struct {
	ChunkKey        string
	ContentID       int64
	ContentVersion  int
	ChunkIndex      int
	ChunkingVersion int
	IndexVersion    int
	Title           string
	Heading         string
	Text            string
	Zone            string
	ContentType     string
	Source          string
}

type AgentRetrievalResult struct {
	Candidates []AgentRetrievalCandidate
	Degraded   string
	// ExpandedQueries mirrors the query-expansion terms used by the hybrid
	// pipeline (A-03), for display in the tool step panel.
	ExpandedQueries []string
}

type AgentContentRetriever interface {
	Retrieve(ctx context.Context, query string, viewerID int64) (AgentRetrievalResult, error)
}

func NewAgentService(provider llm.LLMProvider, embeddingRepo *repository.EmbeddingRepository, contentRepo *repository.ContentRepository, greenClient *aliyun.GreenClient, db *gorm.DB, cfg *config.Config) *AgentService {
	return newAgentServiceWithChatStreamer(provider, provider, embeddingRepo, contentRepo, greenClient, db, cfg)
}

// newAgentServiceWithChatStreamer keeps the public compatibility constructor
// while allowing the workspace stream to depend on a narrower capability.
// Legacy helpers still receive the broader provider through provider.
func newAgentServiceWithChatStreamer(provider llm.LLMProvider, chatStreamer agentChatStreamer, embeddingRepo *repository.EmbeddingRepository, contentRepo *repository.ContentRepository, greenClient *aliyun.GreenClient, db *gorm.DB, cfg *config.Config) *AgentService {
	svc := &AgentService{
		llmProvider:   provider,
		chatStreamer:  chatStreamer,
		embeddingRepo: embeddingRepo,
		contentRepo:   contentRepo,
		db:            db,
		cfg:           cfg,
		queueProducer: queue.NewNoopProducer(),
	}
	// A nil *aliyun.GreenClient must leave the interface nil (not a typed-nil
	// pointer) so unconfigured environments take the explicit skip paths.
	if greenClient != nil {
		svc.greenClient = greenClient
	}
	if db != nil {
		svc.ragChunkRepo = repository.NewRagChunkRepository(db)
	}
	if embeddingRepo != nil {
		svc.vectorSearch = embeddingRepo.VectorSearch
	}
	return svc
}

// SetGreenScanner injects the Green text scan seam. Tests substitute a fake
// to assert chat guardrail semantics without real credentials.
func (s *AgentService) SetGreenScanner(sc agentGreenScanner) {
	s.greenClient = sc
}

type UploadAssistResult struct {
	SuggestedTags        []string `json:"suggested_tags"`
	SuggestedCategory    string   `json:"suggested_category"`
	SuggestedTitle       string   `json:"suggested_title"`
	SuggestedDescription string   `json:"suggested_description"`
}

func (s *AgentService) UploadAssist(ctx context.Context, userID int64, title, description, filename, contentType string) (*UploadAssistResult, error) {
	if !s.cfg.Agent.WebAgentEnabled {
		return nil, ErrAgentDisabled
	}

	systemPrompt := fmt.Sprintf(`You are a content tagging assistant for a fan content platform.
Given a file named "%s" of type "%s" with title "%s" and description "%s",
suggest appropriate tags, category, title improvements, and description.
Respond ONLY with valid JSON: {"suggested_tags":[],"suggested_category":"","suggested_title":"","suggested_description":""}`,
		filename, contentType, title, description)

	req := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: "Please analyze and suggest metadata for this content."},
		},
		MaxTokens:   500,
		Temperature: 0.3,
	}

	resp, err := s.llmProvider.Chat(ctx, req)
	if err != nil {
		return nil, err
	}

	var result UploadAssistResult
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		result = UploadAssistResult{
			SuggestedTags:        []string{},
			SuggestedCategory:    contentType,
			SuggestedTitle:       title,
			SuggestedDescription: description,
		}
	}
	result = sanitizeUploadAssistResult(result)
	return &result, nil
}

const (
	uploadAssistMaxTags        = 10
	uploadAssistMaxTagLength   = 32
	uploadAssistMaxTitleLength = 500
	uploadAssistMaxDescLength  = 2000
)

var allowedUploadAssistCategories = map[string]bool{
	"film_tv":        true,
	"gaming":         true,
	"literature":     true,
	"pet":            true,
	"food":           true,
	"beauty_fashion": true,
	"home":           true,
	"tech_digital":   true,
	"travel":         true,
	"sports":         true,
	"productivity":   true,
}

func sanitizeUploadAssistResult(result UploadAssistResult) UploadAssistResult {
	result.SuggestedTitle = truncateRunes(strings.TrimSpace(result.SuggestedTitle), uploadAssistMaxTitleLength)
	result.SuggestedDescription = truncateRunes(strings.TrimSpace(result.SuggestedDescription), uploadAssistMaxDescLength)

	if !allowedUploadAssistCategories[strings.TrimSpace(result.SuggestedCategory)] {
		result.SuggestedCategory = ""
	}

	tags := make([]string, 0, minInt(len(result.SuggestedTags), uploadAssistMaxTags))
	seen := make(map[string]bool, len(result.SuggestedTags))
	for _, tag := range result.SuggestedTags {
		cleaned := truncateRunes(strings.TrimSpace(tag), uploadAssistMaxTagLength)
		if cleaned == "" || seen[cleaned] {
			continue
		}
		tags = append(tags, cleaned)
		seen[cleaned] = true
		if len(tags) == uploadAssistMaxTags {
			break
		}
	}
	result.SuggestedTags = tags

	return result
}

func truncateRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type ComplianceResult struct {
	RiskLevel   string   `json:"risk_level"`
	Reason      string   `json:"reason"`
	Suggestions []string `json:"suggestions"`
}

func (s *AgentService) ComplianceCheck(ctx context.Context, title, description, contentType string) (*ComplianceResult, error) {
	if !s.cfg.Agent.WebAgentEnabled {
		return nil, ErrAgentDisabled
	}

	// Step 1/3: Aliyun Green text moderation
	var greenResult string
	var greenReason string
	text := strings.TrimSpace(title + "\n" + description)
	if s.greenClient != nil && text != "" {
		scanRes, err := s.greenClient.TextModeration(ctx, text)
		if err == nil {
			greenResult = scanRes.Result
			greenReason = scanRes.Reason
		}
	}

	// Green block → immediate violation, skip LLM
	if greenResult == "block" {
		return &ComplianceResult{
			RiskLevel:   "violation",
			Reason:      "Content flagged by automated safety check: " + greenReason,
			Suggestions: []string{"Content violates platform safety policy"},
		}, nil
	}

	// Step 2/3: LLM copyright / compliance analysis
	prompt := fmt.Sprintf(`Analyze the following content for compliance issues (copyright infringement, inappropriate content):
Title: %s
Description: %s
Type: %s

Respond ONLY with valid JSON: {"risk_level":"safe|warning|violation","reason":"","suggestions":[]}`,
		title, description, contentType)

	req := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: "You are a content compliance checker. Be conservative but fair."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   300,
		Temperature: 0.1,
	}

	resp, err := s.llmProvider.Chat(ctx, req)
	if err != nil {
		// LLM unavailable but Green returned review → return warning
		if greenResult == "review" {
			return &ComplianceResult{
				RiskLevel:   "warning",
				Reason:      "Content requires manual review (safety check: " + greenReason + ")",
				Suggestions: []string{"LLM analysis unavailable, review manually"},
			}, nil
		}
		return nil, err
	}

	var llmResult ComplianceResult
	if err := json.Unmarshal([]byte(resp.Content), &llmResult); err != nil {
		llmResult = ComplianceResult{RiskLevel: "safe", Reason: resp.Content}
	}

	// Step 3/3: Aggregate Green + LLM results
	return aggregateComplianceResults(greenResult, greenReason, &llmResult), nil
}

func aggregateComplianceResults(greenResult, greenReason string, llmResult *ComplianceResult) *ComplianceResult {
	if greenResult == "block" {
		return &ComplianceResult{
			RiskLevel:   "violation",
			Reason:      "Content flagged by automated safety check: " + greenReason,
			Suggestions: []string{"Content violates platform safety policy"},
		}
	}
	if greenResult == "review" {
		if llmResult.RiskLevel == "safe" {
			llmResult.RiskLevel = "warning"
		}
		if greenReason != "" {
			llmResult.Reason = "[SafetyCheck:review] " + llmResult.Reason
		}
	}
	return llmResult
}

type ContentSummary struct {
	ID             int64    `json:"id"`
	Title          string   `json:"title"`
	Zone           string   `json:"zone,omitempty"`
	ContentType    string   `json:"content_type"`
	ContentVersion int      `json:"content_version,omitempty"`
	ChunkKey       string   `json:"chunk_key,omitempty"`
	ChunkIndex     int      `json:"chunk_index,omitempty"`
	Excerpt        string   `json:"excerpt,omitempty"`
	Source         string   `json:"source,omitempty"`
	Score          float64  `json:"-"`
	Tags           []string `json:"tags,omitempty"`
}

// CheckContentVisible is the handler-side viewer-aware precheck for
// client-supplied resource IDs (usage-guide path). Hidden content returns
// ErrContentNotFound before any quota reservation or Provider call.
func (s *AgentService) CheckContentVisible(ctx context.Context, viewerID, contentID int64) error {
	_, err := s.resolveVisibleContent(ctx, viewerID, contentID)
	return err
}

func (s *AgentService) listVisibleNLSearchContents(contentIDs []int64, viewerID int64) ([]model.ContentItem, error) {
	if len(contentIDs) == 0 {
		return nil, nil
	}
	if s.contentRepo == nil {
		return nil, errors.New("content repository unavailable")
	}
	var contents []model.ContentItem
	// Shared visibility enforces published status, IsBanned author exclusion, IsPublic/viewerID access, and banned-IP exclusion.
	err := repository.ApplyContentVisibilityScope(s.contentRepo.DB().Model(&model.ContentItem{}), viewerID).
		Where("content_items.id IN ?", contentIDs).
		Find(&contents).Error
	if err != nil {
		return nil, err
	}

	order := make(map[int64]int, len(contentIDs))
	for i, id := range contentIDs {
		order[id] = i
	}
	for i := 0; i < len(contents); i++ {
		for j := i + 1; j < len(contents); j++ {
			if order[contents[j].ID] < order[contents[i].ID] {
				contents[i], contents[j] = contents[j], contents[i]
			}
		}
	}
	return contents, nil
}

type UsageGuideResult struct {
	Guide string `json:"guide"`
	// Structured marks that the guide came from persisted structured data
	// (system template + author specifics) instead of a live LLM call
	// (SP-16 #447).
	Structured bool   `json:"structured,omitempty"`
	Source     string `json:"source,omitempty"`
}

func (s *AgentService) UsageGuide(ctx context.Context, viewerID, contentItemID int64, forceLLM bool) (*UsageGuideResult, error) {
	if !s.cfg.Agent.WebAgentEnabled {
		return nil, ErrAgentDisabled
	}

	content, err := s.resolveVisibleContent(ctx, viewerID, contentItemID)
	if err != nil {
		return nil, err
	}

	// SP-16 #447 structured-first: persisted specifics render without any
	// LLM call; forceLLM (studio draft button) regenerates instead.
	if !forceLLM {
		if view, ok := s.structuredUsageGuide(ctx, contentItemID); ok {
			return &UsageGuideResult{
				Guide:      RenderUsageGuideMarkdown(view),
				Structured: true,
				Source:     view.Source,
			}, nil
		}
	}

	var guideType string
	switch content.ContentType {
	case "mod":
		guideType = "installation steps, compatibility requirements, and conflict resolution"
	case "sheet_music":
		guideType = "recommended software, playback instructions, and printing tips"
	default:
		guideType = "usage instructions and best practices"
	}

	prompt := fmt.Sprintf(`Generate a concise usage guide for this content:
Title: %s
Type: %s
Description: %s

Focus on: %s
Format as Markdown.`, content.Title, content.ContentType, content.Description, guideType)

	req := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: "You are a helpful content guide writer."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   800,
		Temperature: 0.5,
	}

	resp, err := s.llmProvider.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	return &UsageGuideResult{Guide: resp.Content}, nil
}

func (s *AgentService) UsageGuideStream(ctx context.Context, viewerID, contentItemID int64, forceLLM bool, handler func(delta string, done bool) error) error {
	if !s.cfg.Agent.WebAgentEnabled {
		return ErrAgentDisabled
	}

	content, err := s.resolveVisibleContent(ctx, viewerID, contentItemID)
	if err != nil {
		return err
	}

	// SP-16 #447 structured-first on the stream contract: one delta carries
	// the rendered markdown, then done.
	if !forceLLM {
		if view, ok := s.structuredUsageGuide(ctx, contentItemID); ok {
			if err := handler(RenderUsageGuideMarkdown(view), false); err != nil {
				return err
			}
			return handler("", true)
		}
	}

	prompt := fmt.Sprintf("Generate a concise usage guide for: %s (type: %s)", content.Title, content.ContentType)
	req := llm.ChatRequest{
		Messages:    []llm.ChatMessage{{Role: "user", Content: prompt}},
		MaxTokens:   800,
		Temperature: 0.5,
		Stream:      true,
	}

	return s.llmProvider.ChatStream(ctx, req, func(delta llm.ChatDelta) error {
		return handler(delta.Content, delta.Done)
	})
}

type ModerationResult struct {
	RiskLevel   string   `json:"risk_level"`
	Violations  []string `json:"violations"`
	Suggestions []string `json:"suggestions"`
}

func (s *AgentService) Moderate(ctx context.Context, contentItemID int64) (*ModerationResult, error) {
	if !s.cfg.Agent.WebAgentEnabled {
		return nil, ErrAgentDisabled
	}

	content, err := s.contentRepo.FindByID(contentItemID)
	if err != nil || content == nil {
		return nil, ErrContentNotFound
	}

	// Step 1: Aliyun Green text moderation
	var greenResult string
	var greenViolations []string
	text := strings.TrimSpace(content.Title + "\n" + content.Description)
	if s.greenClient != nil && text != "" {
		scanRes, err := s.greenClient.TextModeration(ctx, text)
		if err == nil {
			greenResult = scanRes.Result
			if scanRes.Result == "block" {
				greenViolations = append(greenViolations, scanRes.Reason)
			}
		}
	}

	// Step 2: LLM comprehensive analysis
	prompt := fmt.Sprintf(`Moderate this content for policy violations:
Title: %s
Description: %s
Type: %s

Check for: copyright infringement, adult content, spam, hate speech.
Respond ONLY with JSON: {"risk_level":"safe|warning|violation","violations":[],"suggestions":[]}`,
		content.Title, content.Description, content.ContentType)

	req := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: "You are a strict content moderation AI."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   400,
		Temperature: 0.1,
	}

	resp, err := s.llmProvider.Chat(ctx, req)
	if err != nil {
		// LLM unavailable, use Green result as fallback
		if greenResult == "block" {
			return &ModerationResult{
				RiskLevel:  "violation",
				Violations: greenViolations,
			}, nil
		}
		return nil, err
	}

	var llmResult ModerationResult
	if err := json.Unmarshal([]byte(resp.Content), &llmResult); err != nil {
		llmResult = ModerationResult{RiskLevel: "safe", Violations: []string{}, Suggestions: []string{}}
	}

	// Aggregate: Green violations take precedence
	if greenResult == "block" {
		llmResult.RiskLevel = "violation"
		llmResult.Violations = append(greenViolations, llmResult.Violations...)
	}

	// Step 3: Auto-create AI review record on violation
	if llmResult.RiskLevel == "violation" && s.db != nil {
		raw, _ := json.Marshal(llmResult)
		record := model.AIReviewRecord{
			TargetType:  "content",
			TargetID:    contentItemID,
			Provider:    "agent",
			Result:      "block",
			RawResponse: raw,
			ScannedAt:   time.Now(),
		}
		if err := s.db.Create(&record).Error; err != nil {
			slog.Error("failed to create ai review record", "content_id", contentItemID, "error", err)
		}
	}

	return &llmResult, nil
}

func (s *AgentService) SetQueueProducer(p queue.Producer) {
	s.queueProducer = p
}

// SetSearchRepository wires the keyword search fallback used when
// conversational search is unavailable (degraded mode).
func (s *AgentService) SetSearchRepository(repo *repository.SearchRepository) {
	s.searchRepo = repo
}

// SetUsageGuideService wires the merged guide view the in-site agent reads
// before falling back to LLM generation (SP-16 #447).
func (s *AgentService) SetUsageGuideService(svc *UsageGuideService) {
	s.usageGuideSvc = svc
}

// structuredUsageGuide returns the merged view when the content has
// persisted specifics (pure templates never short-circuit the LLM here —
// they are generic, not content-specific guidance).
func (s *AgentService) structuredUsageGuide(ctx context.Context, contentItemID int64) (*UsageGuideView, bool) {
	if s.usageGuideSvc == nil {
		return nil, false
	}
	view, err := s.usageGuideSvc.GetMergedView(ctx, contentItemID, "zh")
	if err != nil || view == nil || view.Source == "" {
		return nil, false
	}
	return view, true
}

// HasStructuredGuide reports whether the usage-guide read will be served
// from persisted specifics, letting the handler skip quota reservation for
// non-LLM answers.
func (s *AgentService) HasStructuredGuide(ctx context.Context, contentItemID int64) bool {
	_, ok := s.structuredUsageGuide(ctx, contentItemID)
	return ok
}

// RenderUsageGuideMarkdown flattens a merged guide view into the Markdown
// shape the in-site agent surface has always served.
func RenderUsageGuideMarkdown(view *UsageGuideView) string {
	var b strings.Builder
	b.WriteString("## 前置要求\n")
	for _, item := range view.Requirements {
		b.WriteString("- " + item + "\n")
	}
	b.WriteString("\n## 使用步骤\n")
	for i, step := range view.Steps {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
	}
	if strings.TrimSpace(view.Notes) != "" {
		b.WriteString("\n## 说明\n" + view.Notes + "\n")
	}
	b.WriteString("\n## 安全提示\n")
	for _, item := range view.Safety {
		b.WriteString("- " + item + "\n")
	}
	return b.String()
}

// SetContentRetriever injects the viewer-aware hybrid retrieval boundary used
// by Agent tools and NLSearch. The container supplies the concrete adapter.
func (s *AgentService) SetContentRetriever(retriever AgentContentRetriever) {
	s.hybridRetriever = retriever
}

func (s *AgentService) ragHybridEnabled() bool {
	return s != nil && s.cfg != nil && s.cfg.Features.RAGHybridEnabled
}

func (s *AgentService) EmbedContentAsync(contentItemID int64, text string) {
	if _, ok := s.queueProducer.(*queue.NoopProducer); !ok && s.queueProducer != nil {
		recovery.GoSafe(func() {
			payload, _ := json.Marshal(map[string]interface{}{
				"content_id": contentItemID,
				"text":       text,
			})
			if err := s.queueProducer.Publish(context.Background(), "content.embedding", payload); err != nil {
				slog.Error("failed to publish content.embedding message", "content_id", contentItemID, "error", err)
			}
		})
	} else {
		recovery.GoSafe(func() {
			if err := s.embedContent(context.Background(), contentItemID, text); err != nil {
				slog.Error("embedding error", "content_id", contentItemID, "error", err)
			}
		})
	}
}

func (s *AgentService) EmbedContent(ctx context.Context, contentItemID int64, text string) error {
	return s.embedContent(ctx, contentItemID, text)
}

func (s *AgentService) embedContent(ctx context.Context, contentItemID int64, text string) error {
	embedding, err := s.llmProvider.GetEmbedding(ctx, text)
	if err != nil {
		return err
	}
	if err := s.embeddingRepo.UpsertEmbedding(contentItemID, embedding); err != nil {
		return err
	}
	return nil
}

// serverOwnedSystemPrompt builds prompt text exclusively from the server-owned
// surface enum and database-reloaded resource context. Client-supplied titles,
// content types, routes, or visibility claims are never interpolated.
func (s *AgentService) serverOwnedSystemPrompt(surface model.AgentChatSurface, content *model.ContentItem) llm.ChatMessage {
	var parts []string
	switch surface {
	case model.AgentChatSurfaceContent:
		parts = append(parts, "surface=content")
		if content != nil {
			parts = append(parts, fmt.Sprintf("content_id=%d", content.ID))
			parts = append(parts, fmt.Sprintf("title=%s", content.Title))
			parts = append(parts, fmt.Sprintf("content_type=%s", content.ContentType))
		}
	case model.AgentChatSurfaceSearch:
		parts = append(parts, "surface=search")
	case model.AgentChatSurfacePublish:
		parts = append(parts, "surface=publish")
	default:
		parts = append(parts, "surface=global")
	}
	// A-06 行内引用锚定（SP-13 R2-Q9）：指示模型在句末标注引用序号，前端把
	// [n] 渲染为可点击角标并映射到服务端复验后的引用卡片（纯展示层，复验
	// 语义与引用候选收集逻辑零改动）。引用上限之外的标注由流式收口剥离
	// （stripOrphanCitationMarkers），此处要求模型克制标注以减少剥离量。
	parts = append(parts, "when your answer relies on retrieved results, mark the sentence end with 1-based citation indexes like [1] or [2], where n is the position of the result in the search output you used; only mark results you actually used and keep the total number of distinct marks small")
	// 2026-09-06 实测修复（浏览器验收会话）：两个高频体验缺陷的 prompt 层缓解。
	// ① 推荐/发现类请求模型会跳过工具直接凭常识作答，而 grounded 契约会把
	// 无引用回答整体替换为拒答 → 强制先检索再回答；
	// ② 工具输出含内部 id，模型原样复述暴露实现细节 → 禁止在回答中出现。
	// （"思考/回答跟随用户语言"指令经实测无法约束 M3 思考链语言，按用户裁决
	// 移除；该问题仍未解决，待换方案重试。）
	parts = append(parts, "for any request to find, search, recommend, compare or summarize site content, you must call the cited_search tool first and ground the answer only in its results; never recommend or describe site content from your own knowledge")
	parts = append(parts, "never mention internal numeric content ids in your answer")
	// SP-15 A2（2026-09-09）：会话车道指令——寒暄/闲聊/意图不明的消息免工具短答。
	// 与上一条 must-search 指令互补而非覆盖：内容相关问题永远先检索，本条只放行
	// 本来就不需要引用的会话轮。短答约束与服务端 ≤160 runes 护栏双保险。
	// 2026-09-09 评测回退门两轮收紧：首版让模型把裸标题/引文式查询当意图不明跳过
	// 检索（冻结 test vi-0003/vi-0013/ke-0051 逃逸）；第二版把「含具体标题/引文/
	// 关键词 = 内容请求必须先检索」提为句首主导子句，澄清仅限零可检索文本的消息。
	parts = append(parts, "when the user's message contains a concrete title, quote, character name, or keyword that could exist on the site, always call the cited_search tool with it before replying, even if the intent seems ambiguous; for example, a message that is just a title like 「星轨下的制琴师」or 'A Quiet Ledger of Small Storms' is a search request: search that exact text first, then answer from the results, and only say you found nothing usable if the search comes back empty; only for pure greetings, thanks, farewells, or a message with no searchable text at all (for example garbled characters), reply briefly without any tool and without citation marks — one or two sentences in the user's language, either a greeting back or one clarifying question about what site content they need")
	// SP-15 D1/D2（2026-09-10 #434）：查询理解三件套，prompt 层指令为主。
	// D1 自包含改写——search_content 的 query 必须消解指代/省略，独立可理解；
	// few-shot 示例刻意避开冻结评测集查询与站内真实标题（防背题）。D2 复合
	// 问题拆分——多个子问题多次检索，预算 max_tool_calls_per_turn=8 内充足。
	// 上方 must-search 与 A2 会话车道指令原文不动，本组指令追加其后。
	parts = append(parts, "every search_content query must be fully self-contained: resolve all pronouns, ellipsis and context references into the concrete entities they point to (exact titles, author or character names, topics), so each query is understandable with zero prior conversation context; for example, when the user asks 「第二个的作者还有什么作品」 after earlier results, the query must be rewritten like 「《迟到的邮差》的作者的其他作品」 with the resolved title, never a bare reference such as 「第二个」 or 「它的作者」; a message that is just a bare title or quote is itself the self-contained query for its first search")
	parts = append(parts, "when one message combines several independent sub-questions, decompose it into multiple search_content calls — one call per sub-question, each with its own self-contained query — instead of merging them into a single vague query; the per-turn tool budget is sized for this")
	return llm.ChatMessage{
		Role:    "system",
		Content: "[OmniCraft Agent Context] " + strings.Join(parts, "; "),
	}
}

type AIReviewRecord struct {
	model.AIReviewRecord
}

type DeployAction struct {
	Action  string      `json:"action"`
	Payload interface{} `json:"payload"`
}

type DeployScript struct {
	ContentID string         `json:"content_id"`
	Actions   []DeployAction `json:"actions"`
}

type SignedDeployScript struct {
	Script    DeployScript `json:"script"`
	Signature string       `json:"signature"`
}

func extFromURL(rawURL string) string {
	parts := strings.Split(rawURL, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return "bin"
}

func filenameFromOSSKey(key string) string {
	if idx := strings.LastIndexByte(key, '/'); idx >= 0 {
		return key[idx+1:]
	}
	return key
}

func isArchiveFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".rar") || strings.HasSuffix(lower, ".7z")
}

func signHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *AgentService) GenerateDeployScript(ctx context.Context, contentID int64) (*SignedDeployScript, error) {
	if s.cfg.Agent.HMACSecret == "" {
		return nil, errors.New("HMAC secret not configured")
	}

	content, err := s.contentRepo.FindByID(contentID)
	if err != nil {
		return nil, fmt.Errorf("content not found: %w", err)
	}

	var actions []DeployAction

	if content.CoverImageURL != "" {
		actions = append(actions, DeployAction{
			Action:  "download_file",
			Payload: map[string]string{"url": content.CoverImageURL, "dest": "cover." + extFromURL(content.CoverImageURL)},
		})
	}
	attachments, _ := s.contentRepo.GetAttachments(contentID)
	for _, att := range attachments {
		fn := filenameFromOSSKey(att.OSSKey)
		actions = append(actions, DeployAction{
			Action:  "download_file",
			Payload: map[string]string{"url": att.OSSKey, "dest": fn},
		})
		if isArchiveFile(fn) {
			actions = append(actions, DeployAction{
				Action:  "extract_archive",
				Payload: map[string]string{"path": fn, "dest": "extracted"},
			})
		}
	}

	script := DeployScript{
		ContentID: fmt.Sprintf("%d", contentID),
		Actions:   actions,
	}

	payload, err := json.Marshal(script)
	if err != nil {
		return nil, fmt.Errorf("marshal script: %w", err)
	}

	sig := signHMAC(payload, s.cfg.Agent.HMACSecret)

	return &SignedDeployScript{
		Script:    script,
		Signature: sig,
	}, nil
}
