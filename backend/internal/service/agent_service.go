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
	llmProvider   llm.LLMProvider
	embeddingRepo *repository.EmbeddingRepository
	contentRepo   *repository.ContentRepository
	greenClient   *aliyun.GreenClient
	db            *gorm.DB
	cfg           *config.Config
	queueProducer queue.Producer
	vectorSearch  func(embedding []float32, topK int) ([]repository.EmbeddingSearchResult, error)
}

func NewAgentService(provider llm.LLMProvider, embeddingRepo *repository.EmbeddingRepository, contentRepo *repository.ContentRepository, greenClient *aliyun.GreenClient, db *gorm.DB, cfg *config.Config) *AgentService {
	svc := &AgentService{
		llmProvider:   provider,
		embeddingRepo: embeddingRepo,
		contentRepo:   contentRepo,
		greenClient:   greenClient,
		db:            db,
		cfg:           cfg,
		queueProducer: queue.NewNoopProducer(),
	}
	if embeddingRepo != nil {
		svc.vectorSearch = embeddingRepo.VectorSearch
	}
	return svc
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
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	ContentType string   `json:"content_type"`
	Score       float64  `json:"score"`
	Tags        []string `json:"tags"`
}

func (s *AgentService) NLSearch(ctx context.Context, query string, viewerID int64) ([]ContentSummary, error) {
	if !s.cfg.Agent.WebAgentEnabled {
		return nil, ErrAgentDisabled
	}

	embedding, err := s.llmProvider.GetEmbedding(ctx, query)
	if err != nil {
		return nil, err
	}

	if s.vectorSearch == nil {
		return nil, errors.New("vector search unavailable")
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
	return summaries, nil
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
}

func (s *AgentService) UsageGuide(ctx context.Context, viewerID, contentItemID int64) (*UsageGuideResult, error) {
	if !s.cfg.Agent.WebAgentEnabled {
		return nil, ErrAgentDisabled
	}

	content, err := s.resolveVisibleContent(ctx, viewerID, contentItemID)
	if err != nil {
		return nil, err
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

func (s *AgentService) UsageGuideStream(ctx context.Context, viewerID, contentItemID int64, handler func(delta string, done bool) error) error {
	if !s.cfg.Agent.WebAgentEnabled {
		return ErrAgentDisabled
	}

	content, err := s.resolveVisibleContent(ctx, viewerID, contentItemID)
	if err != nil {
		return err
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

// ChatStream runs one chat turn with a server-owned context. Only the surface
// enum and an optional content ID are accepted from the client; the service
// reloads title/type/visibility with the current viewer before the Provider is
// called, so hidden content is rejected without consuming quota or reaching
// the Provider. Client-authored summaries and raw routes are never trusted.
func (s *AgentService) ChatStream(ctx context.Context, userID int64, messages []llm.ChatMessage, chatCtx *model.AgentChatContext, handler func(delta string, done bool, conversationID int64, traceID string) error) error {
	if !s.cfg.Agent.WebAgentEnabled {
		return ErrAgentDisabled
	}

	traceID := newTraceID()
	traceAgentEvent(traceID, "chat_start", "user_id", userID)

	surface := model.AgentChatSurfaceGlobal
	var contentID *int64
	if chatCtx != nil {
		surface = chatCtx.Surface
		contentID = chatCtx.ContentID
	}

	contextType := "general"
	var resolvedContent *model.ContentItem
	if surface == model.AgentChatSurfaceContent && contentID != nil {
		content, err := s.resolveVisibleContent(ctx, userID, *contentID)
		if err != nil {
			traceAgentEvent(traceID, "chat_context_rejected", "surface", surface, "content_id", *contentID, "safe_error", "content_not_found")
			return err
		}
		resolvedContent = content
		contextType = "content"
	}

	conv := &model.AgentConversation{
		UserID:      userID,
		ContextType: contextType,
		ContextID:   contentID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if len(messages) > 0 && messages[0].Role == "user" {
		if err := s.db.Create(conv).Error; err != nil {
			slog.Error("failed to create agent conversation", "error", err)
			conv = nil
		}
	}

	if conv != nil {
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

	systemMsg := s.serverOwnedSystemPrompt(surface, resolvedContent)

	req := llm.ChatRequest{
		Messages:  append([]llm.ChatMessage{systemMsg}, messages...),
		MaxTokens: s.ToolPolicy().MaxOutputTokens,
		Stream:    true,
	}

	var buf strings.Builder
	convID := int64(0)
	if conv != nil {
		convID = conv.ID
	}

	err := s.llmProvider.ChatStream(ctx, req, func(delta llm.ChatDelta) error {
		buf.WriteString(delta.Content)
		if delta.Done && conv != nil {
			assistantContent := buf.String()
			s.db.Create(&model.AgentMessage{
				ConversationID: conv.ID,
				Role:           "assistant",
				Content:        &assistantContent,
				CreatedAt:      time.Now(),
			})
			s.db.Model(conv).Update("updated_at", time.Now())
		}
		return handler(delta.Content, delta.Done, convID, traceID)
	})

	if err != nil && conv != nil {
		slog.Error("[agent] ChatStream failed, cleaning up orphaned conversation", "conversation_id", conv.ID, "error", err)
		s.db.Where("conversation_id = ?", conv.ID).Delete(&model.AgentMessage{})
		s.db.Delete(conv)
	}

	if err != nil {
		traceAgentEvent(traceID, "chat_failed", "safe_error", "provider_error")
	} else {
		traceAgentEvent(traceID, "chat_done", "conversation_id", convID, "surface", surface)
	}
	return err
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
