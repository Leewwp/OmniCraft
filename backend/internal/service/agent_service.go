package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/observability"
	"omnicraft/backend/internal/observability/agenttrace"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service/promptregistry"
)

var ErrAgentDisabled = errors.New("web agent is disabled")
var ErrAgentFileTooLarge = errors.New("file too large for upload assist")

type AgentService struct {
	llmProvider     llm.LLMProvider
	chatStreamer    agentChatStreamer
	modelRouter     llm.ModelRouter
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
	// ipSearch is the IP keyword-search seam consumed by search_ips (SP-19
	// G2-1): production wiring closes over SearchRepository.SearchIPs; tests
	// inject a fake because the tsvector SQL is PostgreSQL-only.
	ipSearch func(ctx context.Context, query, category string, limit int) ([]model.IP, error)
	// prompts resolves versioned prompt templates (SP-21 T5): nil = the
	// compiled-in builtin of every slot; DB production rows override.
	prompts *promptregistry.PromptResolver
	// traceWriter feeds the async agent trace persistence (SP-21 T2): nil
	// disables turn recording entirely (tests, DB-less seams).
	traceWriter *agenttrace.Writer
}

// SetPromptResolver wires the shared resolver (container constructs it after
// the repository; both server and worker share one instance so admin label
// moves invalidate coherently).
func (s *AgentService) SetPromptResolver(r *promptregistry.PromptResolver) {
	s.prompts = r
}

// SetTraceWriter wires the shared async trace writer (SP-21 T2).
func (s *AgentService) SetTraceWriter(w *agenttrace.Writer) {
	s.traceWriter = w
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
	// SP-20 (#545): when the provider is a routing surface, expose model
	// preference validation and the selectable option list; single-provider
	// wirings keep a nil router and reject any client-supplied model id.
	if router, ok := provider.(llm.ModelRouter); ok {
		svc.modelRouter = router
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

// auxLLMTrace wraps one auxiliary non-streaming LLM call (moderation,
// compliance, upload assist, usage guide) with its own trace run and node
// (SP-21 T2): these calls happen outside chat turns, so each becomes an
// independent run row attributable in the admin list and the T7 cost
// ledger. Untraced contexts (nil writer, unsampled) run the call as-is.
func (s *AgentService) auxLLMTrace(ctx context.Context, surface string, slot promptregistry.PromptSlot, nodeType string, call func() (*llm.ChatResponse, error)) (*llm.ChatResponse, error) {
	rec := agenttrace.NewTurnRecorder(s.traceWriter, observability.TraceID(ctx))
	started := time.Now()
	_, promptVersion := s.prompts.Resolve(ctx, slot)
	promptVer := promptVersion
	rec.RecordRunStart(model.AgentTraceRun{
		StartedAt:     started,
		Surface:       surface,
		PromptName:    slot.Name,
		PromptVersion: &promptVer,
	})
	span := rec.StartNode(nodeType, nodeType, nil, "")
	resp, err := call()
	span.End(agenttrace.NodeEndOptions{
		NodeName:         nodeType,
		Status:           auxNodeStatus(err),
		ErrorCode:        auxErrCode(err),
		CompletionDigest: firstLineOfChat(resp),
		TokensIn:         auxUsage(resp, func(u *llm.TokenUsage) int64 { return int64(u.PromptTokens) }),
		TokensOut:        auxUsage(resp, func(u *llm.TokenUsage) int64 { return int64(u.CompletionTokens) }),
		Model:            s.servingModel(rec, ""),
	})
	rec.RecordRunEnd(agenttrace.RunEnd{
		Status:    auxNodeStatus(err),
		StartedAt: started,
		Model:     s.servingModel(rec, ""),
	})
	return resp, err
}

func auxNodeStatus(err error) string {
	if err != nil {
		return model.AgentTraceStatusError
	}
	return model.AgentTraceStatusSuccess
}

func auxErrCode(err error) string {
	if err != nil {
		return "aux_llm_call_failed"
	}
	return ""
}

func firstLineOfChat(resp *llm.ChatResponse) string {
	if resp == nil {
		return ""
	}
	if idx := strings.IndexByte(resp.Content, '\n'); idx >= 0 {
		return resp.Content[:idx]
	}
	return resp.Content
}

func auxUsage(resp *llm.ChatResponse, pick func(*llm.TokenUsage) int64) *int64 {
	if resp == nil || resp.Usage == nil {
		return nil
	}
	v := pick(resp.Usage)
	return &v
}

func (s *AgentService) UploadAssist(ctx context.Context, userID int64, title, description, filename, contentType string) (*UploadAssistResult, error) {
	if !s.cfg.Agent.WebAgentEnabled {
		return nil, ErrAgentDisabled
	}

	systemPrompt := s.prompts.RenderSlot(ctx, promptregistry.SlotUploadAssist, map[string]string{
		"filename":     filename,
		"content_type": contentType,
		"title":        title,
		"description":  description,
	})

	req := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: "Please analyze and suggest metadata for this content."},
		},
		MaxTokens:   500,
		Temperature: 0.3,
	}

	// SP-21 T2: auxiliary LLM calls each become their own trace run.
	resp, err := s.auxLLMTrace(ctx, "aux_upload_assist", promptregistry.SlotUploadAssist, agenttrace.NodeTypeUploadAssist, func() (*llm.ChatResponse, error) {
		return s.llmProvider.Chat(ctx, req)
	})
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
	prompt := s.prompts.RenderSlot(ctx, promptregistry.SlotComplianceCheck, map[string]string{
		"title":        title,
		"description":  description,
		"content_type": contentType,
	})

	req := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: "You are a content compliance checker. Be conservative but fair."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   300,
		Temperature: 0.1,
	}

	resp, err := s.auxLLMTrace(ctx, "aux_compliance", promptregistry.SlotComplianceCheck, agenttrace.NodeTypeCompliance, func() (*llm.ChatResponse, error) {
		return s.llmProvider.Chat(ctx, req)
	})
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

	prompt := s.prompts.RenderSlot(ctx, promptregistry.SlotUsageGuide, map[string]string{
		"title":        content.Title,
		"content_type": content.ContentType,
		"description":  content.Description,
		"guide_focus":  guideType,
	})

	req := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: "You are a helpful content guide writer."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   800,
		Temperature: 0.5,
	}

	resp, err := s.auxLLMTrace(ctx, "aux_guide", promptregistry.SlotUsageGuide, agenttrace.NodeTypeGuide, func() (*llm.ChatResponse, error) {
		return s.llmProvider.Chat(ctx, req)
	})
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
	prompt := s.prompts.RenderSlot(ctx, promptregistry.SlotContentModeration, map[string]string{
		"title":        content.Title,
		"description":  content.Description,
		"content_type": content.ContentType,
	})

	req := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: "You are a strict content moderation AI."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   400,
		Temperature: 0.1,
	}

	resp, err := s.auxLLMTrace(ctx, "aux_moderation", promptregistry.SlotContentModeration, agenttrace.NodeTypeModerate, func() (*llm.ChatResponse, error) {
		return s.llmProvider.Chat(ctx, req)
	})
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
// conversational search is unavailable (degraded mode), and the search_ips
// tool's IP keyword seam (SP-19 G2-1).
func (s *AgentService) SetSearchRepository(repo *repository.SearchRepository) {
	s.searchRepo = repo
	if repo != nil {
		s.ipSearch = func(ctx context.Context, query, category string, limit int) ([]model.IP, error) {
			ips, _, err := repo.SearchIPs(query, category, 1, limit)
			return ips, err
		}
	}
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
// content types, routes, or visibility claims are never interpolated. The
// instruction corpus is the versioned agent_system slot (SP-21 T5): the
// registry template carries it (DB production version overrides the builtin
// hot), while the surface prefix and the config-conditional IP clause remain
// code-built dynamic values.
func (s *AgentService) serverOwnedSystemPrompt(ctx context.Context, surface model.AgentChatSurface, content *model.ContentItem) llm.ChatMessage {
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
	prompt := s.prompts.RenderSlot(ctx, promptregistry.SlotAgentSystem, map[string]string{
		"surface_context": strings.Join(parts, "; "),
	})
	// SP-19 G2-1: the IP-category clause depends on config, not on prompt
	// management, so it stays dynamic and appends after the rendered slot.
	if s.cfg != nil && len(s.cfg.IPCategories) > 0 {
		prompt += "; " + fmt.Sprintf("when the user asks to find, recommend, or browse IPs (original settings/worlds), call the search_ips tool with a self-contained keyword query; when the user names a genre, pass category with one of these slugs only: %s; cite the IPs you used with the same [n] marks as content results", strings.Join(s.cfg.IPCategories, ", "))
	}
	return llm.ChatMessage{
		Role:    "system",
		Content: prompt,
	}
}

type AIReviewRecord struct {
	model.AIReviewRecord
}

// AgentModels lists the selectable chat models (SP-20 #545). A single-provider
// wiring exposes exactly one option synthesized from the config; a routing
// surface lists the registered chain. The shape is handler-owned and safe to
// serialize to authenticated clients (no credentials).
func (s *AgentService) AgentModels() []llm.AgentModelOption {
	if s.modelRouter != nil {
		return s.modelRouter.ModelOptions()
	}
	display := strings.TrimSpace(s.cfg.Agent.LLMModel)
	if display == "" {
		display = strings.ToLower(strings.TrimSpace(s.cfg.Agent.LLMProvider))
	}
	return []llm.AgentModelOption{{ID: strings.ToLower(strings.TrimSpace(s.cfg.Agent.LLMProvider)), DisplayName: display}}
}

// AgentModelRegistered validates a client-supplied model id against the
// registry (routing surface) or the single-provider id.
func (s *AgentService) AgentModelRegistered(id string) bool {
	if s.modelRouter != nil {
		return s.modelRouter.ModelRegistered(id)
	}
	return strings.EqualFold(strings.TrimSpace(id), strings.TrimSpace(s.cfg.Agent.LLMProvider))
}
