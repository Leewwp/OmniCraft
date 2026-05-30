package handler

import (
	"errors"
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
	"gorm.io/gorm"
)

type AgentHandler struct {
	agentSvc *service.AgentService
	cfg      *config.Config
	db       *gorm.DB
}

func NewAgentHandler(db *gorm.DB, cfg *config.Config) *AgentHandler {
	provider := llm.NewProvider(cfg)
	greenClient := aliyun.NewGreenClient(
		cfg.Green.AccessKeyID,
		cfg.Green.AccessKeySecret,
		cfg.Green.Region,
	)
	return &AgentHandler{
		agentSvc: service.NewAgentService(
			provider,
			repository.NewEmbeddingRepository(db),
			repository.NewContentRepository(db),
			greenClient,
			db,
			cfg,
		),
		cfg: cfg,
		db:  db,
	}
}

func (h *AgentHandler) SetQueueProducer(p queue.Producer) {
	h.agentSvc.SetQueueProducer(p)
}

func (h *AgentHandler) UploadAssist(c *gin.Context) {
	if !h.cfg.Agent.WebAgentEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FEATURE_DISABLED", "message": "agent not enabled"})
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
	callerID := middleware.GetUserID(c)
	result, err := h.agentSvc.UploadAssist(c.Request.Context(), callerID, body.Title, body.Description, body.Filename, body.ContentType)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AgentHandler) ComplianceCheck(c *gin.Context) {
	if !h.cfg.Agent.WebAgentEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FEATURE_DISABLED", "message": "agent not enabled"})
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
	result, err := h.agentSvc.ComplianceCheck(c.Request.Context(), body.Title, body.Description, body.ContentType)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AgentHandler) NLSearch(c *gin.Context) {
	if !h.cfg.Agent.WebAgentEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FEATURE_DISABLED", "message": "agent not enabled"})
		return
	}
	var body struct {
		Query string `json:"query" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request parameters")
		return
	}
	results, err := h.agentSvc.NLSearch(c.Request.Context(), body.Query)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

func (h *AgentHandler) UsageGuide(c *gin.Context) {
	if !h.cfg.Agent.WebAgentEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FEATURE_DISABLED", "message": "agent not enabled"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}

	if c.Query("stream") == "true" {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Accel-Buffering", "no")

		err := h.agentSvc.UsageGuideStream(c.Request.Context(), id, func(delta string, done bool) error {
			if done {
				c.SSEvent("message", gin.H{"done": true})
			} else if delta != "" {
				c.SSEvent("message", gin.H{"delta": delta, "done": false})
			}
			c.Writer.Flush()
			return nil
		})
		if err != nil {
			c.SSEvent("error", gin.H{"message": "service error"})
		}
		return
	}

	result, err := h.agentSvc.UsageGuide(c.Request.Context(), id)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AgentHandler) Moderate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}
	result, err := h.agentSvc.Moderate(c.Request.Context(), id)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AgentHandler) GenerateDeployScript(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}
	result, err := h.agentSvc.GenerateDeployScript(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrAgentDisabled) {
			response.SafeErrorResponse(c, http.StatusServiceUnavailable, "FEATURE_DISABLED", err)
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AgentHandler) ChatStream(c *gin.Context) {
	if !h.cfg.Agent.WebAgentEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FEATURE_DISABLED", "message": "agent not enabled"})
		return
	}
	userID := middleware.GetUserID(c)
	var body struct {
		Messages []llm.ChatMessage `json:"messages" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")

	err := h.agentSvc.ChatStream(c.Request.Context(), userID, body.Messages, func(delta string, done bool, convID int64) error {
		if done {
			c.SSEvent("message", gin.H{"done": true, "conversation_id": convID})
		} else if delta != "" {
			c.SSEvent("message", gin.H{"delta": delta, "done": false})
		}
		c.Writer.Flush()
		return nil
	})
	if err != nil {
		c.SSEvent("error", gin.H{"message": "service error"})
	}
}

func (h *AgentHandler) ListConversations(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var conversations []model.AgentConversation
	h.db.Where("user_id = ?", userID).Order("updated_at DESC").Limit(50).Find(&conversations)
	c.JSON(http.StatusOK, gin.H{"conversations": conversations})
}

func (h *AgentHandler) GetConversationMessages(c *gin.Context) {
	userID := middleware.GetUserID(c)
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid conversation id"})
		return
	}
	var conv model.AgentConversation
	if err := h.db.Where("id = ? AND user_id = ?", convID, userID).First(&conv).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "conversation not found"})
		return
	}
	var messages []model.AgentMessage
	h.db.Where("conversation_id = ?", convID).Order("created_at ASC").Find(&messages)
	c.JSON(http.StatusOK, gin.H{"conversation": conv, "messages": messages})
}
