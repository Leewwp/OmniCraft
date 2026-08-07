package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AgentHandler struct {
	agentSvc *service.AgentService
	cfg      *config.Config
	db       *gorm.DB
	quota    *middleware.AgentQuotaReserver
}

func NewAgentHandler(db *gorm.DB, cfg *config.Config, rdb *redis.Client) *AgentHandler {
	provider := llm.NewProvider(cfg)
	greenClient := aliyun.NewGreenClient(
		cfg.Green.AccessKeyID,
		cfg.Green.AccessKeySecret,
		cfg.Green.Region,
	)
	agentSvc := service.NewAgentService(
		provider,
		repository.NewEmbeddingRepository(db),
		repository.NewContentRepository(db),
		greenClient,
		db,
		cfg,
	)
	agentSvc.SetSearchRepository(repository.NewSearchRepository(db))
	return &AgentHandler{
		agentSvc: agentSvc,
		cfg:      cfg,
		db:       db,
		quota:    middleware.NewAgentQuotaReserver(rdb, cfg),
	}
}

// NewAgentHandlerWithService builds the handler around a shared AgentService
// (e.g. the one wired in the service container), which also keeps the search
// repository fallback and queue producer consistent.
func NewAgentHandlerWithService(db *gorm.DB, cfg *config.Config, rdb *redis.Client, agentSvc *service.AgentService) *AgentHandler {
	return &AgentHandler{
		agentSvc: agentSvc,
		cfg:      cfg,
		db:       db,
		quota:    middleware.NewAgentQuotaReserver(rdb, cfg),
	}
}

func (h *AgentHandler) SetQueueProducer(p queue.Producer) {
	h.agentSvc.SetQueueProducer(p)
}

// requireAgentFeature is the shared feature-flag guard. Provider-consuming
// routes fail with 503 FEATURE_DISABLED before any quota or Provider work.
func (h *AgentHandler) requireAgentFeature(c *gin.Context) bool {
	if h.cfg.Agent.WebAgentEnabled {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FEATURE_DISABLED", "message": "web agent is disabled"})
	return false
}

// reserveGenerationQuota reserves one per-minute/per-day request for the
// current user and maps the outcome: quota-exceeded -> 429, Redis unavailable
// -> 503 fail-closed. Callers invoke it AFTER schema/visibility checks and
// immediately BEFORE the first Provider call; every downstream outcome then
// consumes the reservation.
func (h *AgentHandler) reserveGenerationQuota(c *gin.Context) bool {
	if h.quota == nil {
		response.Error(c, http.StatusServiceUnavailable, "AGENT_QUOTA_UNAVAILABLE", "agent quota service is temporarily unavailable")
		return false
	}
	userID := middleware.GetUserID(c)
	err := h.quota.Reserve(c.Request.Context(), userID)
	if err == nil {
		return true
	}
	if errors.Is(err, middleware.ErrAgentQuotaExceeded) {
		response.Error(c, http.StatusTooManyRequests, "AGENT_RATE_LIMIT_EXCEEDED", "agent request limit exceeded")
		return false
	}
	response.Error(c, http.StatusServiceUnavailable, "AGENT_QUOTA_UNAVAILABLE", "agent quota service is temporarily unavailable")
	return false
}

func (h *AgentHandler) UploadAssist(c *gin.Context) {
	if !h.requireAgentFeature(c) {
		return
	}
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request parameters")
		return
	}
	if !h.reserveGenerationQuota(c) {
		return
	}
	callerID := middleware.GetUserID(c)
	result, err := h.agentSvc.UploadAssist(c.Request.Context(), callerID, body.Title, body.Description, body.Filename, body.ContentType)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AgentHandler) ComplianceCheck(c *gin.Context) {
	if !h.requireAgentFeature(c) {
		return
	}
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		ContentType string `json:"content_type"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request parameters")
		return
	}
	if !h.reserveGenerationQuota(c) {
		return
	}
	result, err := h.agentSvc.ComplianceCheck(c.Request.Context(), body.Title, body.Description, body.ContentType)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AgentHandler) NLSearch(c *gin.Context) {
	if !h.requireAgentFeature(c) {
		return
	}
	var body struct {
		Query string `json:"query" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request parameters")
		return
	}
	if !h.reserveGenerationQuota(c) {
		return
	}
	viewerID := middleware.GetUserID(c)
	result, err := h.agentSvc.NLSearch(c.Request.Context(), body.Query, viewerID)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AgentHandler) UsageGuide(c *gin.Context) {
	if !h.requireAgentFeature(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}
	viewerID := middleware.GetUserID(c)

	// Client-supplied resource IDs are visibility-prechecked BEFORE any quota
	// reservation, so hidden content cannot be probed and never consumes quota.
	if err := h.agentSvc.CheckContentVisible(c.Request.Context(), viewerID, id); err != nil {
		response.NotFound(c, "content not found")
		return
	}
	if !h.reserveGenerationQuota(c) {
		return
	}

	if c.Query("stream") == "true" {
		writer := &agentSSEWriter{c: c}
		writer.begin()
		err := h.agentSvc.UsageGuideStream(c.Request.Context(), viewerID, id, func(delta string, done bool) error {
			if done {
				return writer.emit(service.AgentStreamEvent{Type: service.AgentEventDone})
			}
			if delta != "" {
				return writer.emit(service.AgentStreamEvent{Type: service.AgentEventDelta, Delta: delta})
			}
			return nil
		})
		if err != nil {
			writer.emit(service.AgentStreamEvent{
				Type:         service.AgentEventError,
				ErrorCode:    service.AgentErrorCodeProvider,
				ErrorMessage: "provider unavailable",
			})
		}
		return
	}

	result, err := h.agentSvc.UsageGuide(c.Request.Context(), viewerID, id)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AgentHandler) ChatStream(c *gin.Context) {
	if !h.requireAgentFeature(c) {
		return
	}
	userID := middleware.GetUserID(c)
	var body struct {
		Messages []llm.ChatMessage `json:"messages" binding:"required"`
		Context  *ChatContextDTO   `json:"context,omitempty"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	maxMsgLen := h.cfg.Agent.MaxUserMessageChars
	if maxMsgLen <= 0 {
		response.Error(c, http.StatusServiceUnavailable, "AGENT_CONFIG_INVALID", "agent limits are not configured")
		return
	}
	maxCtxMsgs := h.cfg.Agent.ChatMaxContextMsgs
	if maxCtxMsgs <= 0 {
		response.Error(c, http.StatusServiceUnavailable, "AGENT_CONFIG_INVALID", "agent limits are not configured")
		return
	}
	if h.cfg.Agent.MaxToolCallsPerTurn <= 0 || h.cfg.Agent.MaxOutputTokens <= 0 || h.cfg.Agent.CitationMaxCount <= 0 {
		response.Error(c, http.StatusServiceUnavailable, "AGENT_CONFIG_INVALID", "agent tool limits are not configured")
		return
	}

	allowedRoles := map[string]bool{"user": true, "assistant": true}
	filtered := make([]llm.ChatMessage, 0, len(body.Messages))
	for _, msg := range body.Messages {
		if !allowedRoles[msg.Role] {
			continue
		}
		if len([]rune(msg.Content)) > maxMsgLen {
			response.ValidationError(c, "message exceeds maximum length")
			return
		}
		filtered = append(filtered, msg)
	}
	if len(filtered) == 0 {
		response.ValidationError(c, "at least one user or assistant message is required")
		return
	}
	if len(filtered) > maxCtxMsgs {
		filtered = filtered[len(filtered)-maxCtxMsgs:]
	}

	var chatCtx *model.AgentChatContext
	if body.Context != nil {
		surface := model.AgentChatSurface(body.Context.Surface)
		switch surface {
		case model.AgentChatSurfaceGlobal, model.AgentChatSurfaceContent, model.AgentChatSurfaceSearch, model.AgentChatSurfacePublish:
		default:
			response.ValidationError(c, "invalid context surface")
			return
		}
		chatCtx = &model.AgentChatContext{
			Surface:   surface,
			ContentID: body.Context.ContentID,
		}
	}

	// Viewer-aware preload of any client-supplied context ID: hidden content is
	// rejected here, BEFORE reservation, and never reaches the Provider.
	resolved, err := h.agentSvc.ResolveChatContext(c.Request.Context(), userID, chatCtx)
	if err != nil {
		response.NotFound(c, "content not found")
		return
	}

	if !h.reserveGenerationQuota(c) {
		return
	}

	writer := &agentSSEWriter{c: c}
	writer.begin()

	if err := h.agentSvc.ChatStream(c.Request.Context(), userID, filtered, resolved, func(ev service.AgentStreamEvent) error {
		return writer.emit(ev)
	}); err != nil {
		// The service already emitted a safe SSE error event; no second JSON
		// response may follow once headers are written.
		return
	}
}

type ChatContextDTO struct {
	Surface   string `json:"surface"`
	ContentID *int64 `json:"content_id,omitempty"`
}

func (h *AgentHandler) ListConversations(c *gin.Context) {
	if !h.requireAgentFeature(c) {
		return
	}
	userID := middleware.GetUserID(c)
	listLimit := h.cfg.Agent.ConversationListLimit
	if listLimit <= 0 {
		response.Error(c, http.StatusServiceUnavailable, "AGENT_CONFIG_INVALID", "agent conversation limits are not configured")
		return
	}
	var conversations []model.AgentConversation
	if err := h.db.Where("user_id = ?", userID).Order("updated_at DESC").Limit(listLimit).Find(&conversations).Error; err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"conversations": conversations})
}

func (h *AgentHandler) GetConversationMessages(c *gin.Context) {
	if !h.requireAgentFeature(c) {
		return
	}
	userID := middleware.GetUserID(c)
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid conversation id"})
		return
	}
	var conv model.AgentConversation
	if err := h.db.Where("id = ? AND user_id = ?", convID, userID).First(&conv).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "conversation not found"})
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	page := 1
	pageSize := h.cfg.Agent.ConversationPageSize
	if pageSize <= 0 {
		response.Error(c, http.StatusServiceUnavailable, "AGENT_CONFIG_INVALID", "agent conversation limits are not configured")
		return
	}
	if raw := c.Query("page"); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil || page <= 0 {
			response.ValidationError(c, "invalid page")
			return
		}
	}
	if raw := c.Query("page_size"); raw != "" {
		requested, parseErr := strconv.Atoi(raw)
		if parseErr != nil || requested <= 0 {
			response.ValidationError(c, "invalid page_size")
			return
		}
		pageSize = requested
	}
	if pageSize > h.cfg.Agent.ConversationPageSize {
		pageSize = h.cfg.Agent.ConversationPageSize
	}
	var messages []model.AgentMessage
	if err := h.db.Where("conversation_id = ?", convID).Order("created_at ASC").Offset((page - 1) * pageSize).Limit(pageSize + 1).Find(&messages).Error; err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	hasMore := len(messages) > pageSize
	if hasMore {
		messages = messages[:pageSize]
	}
	c.JSON(http.StatusOK, gin.H{"conversation": conv, "messages": messages, "page": page, "page_size": pageSize, "has_more": hasMore})
}

// DeleteConversation deletes only the current user's conversation and its
// message cascade inside one owner-scoped transaction. Missing, already
// deleted or foreign IDs all return idempotent 204 so existence cannot be
// probed; a DB failure rolls back (no partial message deletion) and returns a
// stable error. Deletion never touches the Agent generation quota.
func (h *AgentHandler) DeleteConversation(c *gin.Context) {
	if !h.requireAgentFeature(c) {
		return
	}
	userID := middleware.GetUserID(c)
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || convID <= 0 {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid conversation id")
		return
	}

	tx := h.db.Begin()
	if tx.Error != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", tx.Error)
		return
	}
	committed := false
	defer func() {
		if !committed {
			if err := tx.Rollback().Error; err != nil && !errors.Is(err, gorm.ErrInvalidTransaction) {
				// The response has already been selected on the failure path; keep
				// the rollback failure in structured logs without exposing it.
				slog.Error("agent conversation rollback failed", "error", err)
			}
		}
	}()

	var conv model.AgentConversation
	if err := tx.Where("id = ? AND user_id = ?", convID, userID).First(&conv).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if commitErr := tx.Commit().Error; commitErr != nil {
				response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", commitErr)
				return
			}
			committed = true
			c.Status(http.StatusNoContent)
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	if err := tx.Where("conversation_id = ?", conv.ID).Delete(&model.AgentMessage{}).Error; err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	if err := tx.Delete(&model.AgentConversation{}, "id = ?", conv.ID).Error; err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	if err := tx.Commit().Error; err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	committed = true
	c.Status(http.StatusNoContent)
}
