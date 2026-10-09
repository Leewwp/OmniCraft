package handler

// #854 anonymous agent surface — HTTP endpoints. Every route is mounted
// behind middleware.GuestAgentAccess (total gate + signed device identity)
// and the generation route additionally behind GuestAgentCostLimit (per-IP
// fail-closed cost bucket) plus the global CSRF/Origin posture. Quota is
// reserved here AFTER config/schema/visibility/ownership/input checks and
// immediately before the first Provider work; every outcome after the
// reservation consumes the turn (middleware.GuestQuotaReserver semantics).

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AgentGuestHandler struct {
	agentSvc *service.AgentService
	cfg      *config.Config
	db       *gorm.DB
	quota    *middleware.GuestQuotaReserver
}

func NewAgentGuestHandler(db *gorm.DB, cfg *config.Config, agentSvc *service.AgentService, quota *middleware.GuestQuotaReserver) *AgentGuestHandler {
	return &AgentGuestHandler{agentSvc: agentSvc, cfg: cfg, db: db, quota: quota}
}

// deviceKeyOf extracts the verified device identity; a missing key on a
// route that passed GuestAgentAccess is a wiring bug and refuses closed.
func (h *AgentGuestHandler) deviceKeyOf(c *gin.Context) (string, bool) {
	key := middleware.GetGuestDeviceKey(c)
	if key == "" {
		response.Error(c, http.StatusUnauthorized, "GUEST_DEVICE_REQUIRED", "guest device identity is required")
		return "", false
	}
	return key, true
}

// Quota reports the device's remaining turn budget. A lost counter state
// reads as zero remaining: the workspace shows the conversion card and the
// server refuses generation (fail closed), never re-creating a budget.
func (h *AgentGuestHandler) Quota(c *gin.Context) {
	deviceKey, ok := h.deviceKeyOf(c)
	if !ok {
		return
	}
	remaining, err := h.quota.Remaining(c.Request.Context(), deviceKey)
	if err != nil && !errors.Is(err, middleware.ErrGuestQuotaStateLost) {
		response.Error(c, http.StatusServiceUnavailable, "GUEST_QUOTA_UNAVAILABLE",
			"guest quota service is temporarily unavailable")
		return
	}
	exhausted := remaining <= 0
	c.JSON(http.StatusOK, gin.H{
		"remaining":             remaining,
		"max_turns":             h.cfg.Agent.Guest.MaxTotalTurns,
		"exhausted":             exhausted,
		"conversation_ttl_days": h.cfg.Agent.Guest.ConversationTTLDays,
	})
}

// ListModels serves the same registered chat models as the authenticated
// surface; model display metadata is public.
func (h *AgentGuestHandler) ListModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"models": h.agentSvc.AgentModels()})
}

// ListConversations returns the device's live guest conversations.
func (h *AgentGuestHandler) ListConversations(c *gin.Context) {
	deviceKey, ok := h.deviceKeyOf(c)
	if !ok {
		return
	}
	listLimit := h.cfg.Agent.ConversationListLimit
	if listLimit <= 0 {
		response.Error(c, http.StatusServiceUnavailable, "AGENT_CONFIG_INVALID", "agent conversation limits are not configured")
		return
	}
	conversations, err := h.agentSvc.ListGuestConversations(c.Request.Context(), deviceKey, listLimit)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"conversations": conversations})
}

// GetConversationMessages replays one own, live guest conversation. Foreign
// ids are the owner-scoped 404; expired ones are a distinct 410 so the client
// can drop the dead entry without reading it as an existence probe.
func (h *AgentGuestHandler) GetConversationMessages(c *gin.Context) {
	deviceKey, ok := h.deviceKeyOf(c)
	if !ok {
		return
	}
	convID, ok := pathID(c, "id", "invalid conversation id")
	if !ok {
		return
	}
	conv, err := h.agentSvc.GetGuestConversation(c.Request.Context(), deviceKey, convID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAgentConversationExpired):
			response.Error(c, http.StatusGone, "AGENT_CONVERSATION_EXPIRED", "conversation has expired")
		case errors.Is(err, service.ErrAgentConversationNotFound):
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "conversation not found")
		default:
			response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		}
		return
	}
	pageSize := h.cfg.Agent.ConversationPageSize
	if pageSize <= 0 {
		response.Error(c, http.StatusServiceUnavailable, "AGENT_CONFIG_INVALID", "agent conversation limits are not configured")
		return
	}
	var messages []model.AgentMessage
	if err := h.db.WithContext(c.Request.Context()).
		Where("conversation_id = ?", convID).
		Order("created_at ASC").
		Limit(pageSize).
		Find(&messages).Error; err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}
	dtos := make([]agentConversationMessageDTO, len(messages))
	for i := range messages {
		dtos[i] = agentMessageHistoryDTO(messages[i])
	}
	c.JSON(http.StatusOK, gin.H{"conversation": conv, "messages": dtos})
}

// ChatStream runs one guest turn: cost bucket and identity have already
// passed as route middleware; this validates the payload, gates ownership /
// visibility / input moderation, reserves the turn and only then opens the
// SSE stream.
func (h *AgentGuestHandler) ChatStream(c *gin.Context) {
	deviceKey, ok := h.deviceKeyOf(c)
	if !ok {
		return
	}
	var body struct {
		ConversationID *int64  `json:"conversation_id,omitempty"`
		Message        string  `json:"message"`
		DeepThink      bool    `json:"deep_think,omitempty"`
		Model          string  `json:"model,omitempty"`
		Locale         string  `json:"locale,omitempty"`
		Surface        *string `json:"surface,omitempty"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Message) == "" {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	conversationID := int64(0)
	if body.ConversationID != nil {
		if *body.ConversationID <= 0 {
			response.ValidationError(c, "invalid conversation id")
			return
		}
		conversationID = *body.ConversationID
	}

	if h.cfg.Agent.MaxUserMessageChars <= 0 || h.cfg.Agent.ChatContextTokenBudget <= 0 ||
		h.cfg.Agent.ChatMaxContextMsgs <= 0 || h.cfg.Agent.MaxToolCallsPerTurn <= 0 ||
		h.cfg.Agent.MaxOutputTokens <= 0 || h.cfg.Agent.CitationMaxCount <= 0 {
		response.Error(c, http.StatusServiceUnavailable, "AGENT_CONFIG_INVALID", "agent limits are not configured")
		return
	}
	message := strings.TrimSpace(body.Message)
	if len([]rune(message)) > h.cfg.Agent.MaxUserMessageChars {
		response.ValidationError(c, "message exceeds maximum length")
		return
	}
	modelPref := strings.TrimSpace(body.Model)
	if modelPref != "" && !h.agentSvc.AgentModelRegistered(modelPref) {
		response.Error(c, http.StatusBadRequest, "INVALID_MODEL", "unknown model")
		return
	}
	// The guest surface answers global public questions only: content-scoped
	// context rides the authenticated surface.
	if body.Surface != nil && *body.Surface != string(model.AgentChatSurfaceGlobal) {
		response.ValidationError(c, "invalid context surface")
		return
	}

	// Owner + expiry gate BEFORE the reservation: foreign, missing and
	// expired conversations never consume a turn.
	if conversationID > 0 {
		if err := h.agentSvc.EnsureGuestConversationUsable(c.Request.Context(), deviceKey, conversationID); err != nil {
			switch {
			case errors.Is(err, service.ErrAgentConversationExpired):
				response.Error(c, http.StatusGone, "AGENT_CONVERSATION_EXPIRED", "conversation has expired")
			case errors.Is(err, service.ErrAgentConversationNotFound):
				response.Error(c, http.StatusNotFound, "NOT_FOUND", "conversation not found")
			default:
				response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
			}
			return
		}
	}

	// Anonymous viewer preload of client-supplied context: guest surface
	// accepts no content context, so the resolved context is always global.
	resolved := &service.ResolvedChatContext{Surface: model.AgentChatSurfaceGlobal}

	// A-05 input admission gate before reservation (same semantics as the
	// authenticated surface: block → 422, unavailable-in-release → 503).
	if err := h.agentSvc.ModerateChatInput(c.Request.Context(), message); err != nil {
		if errors.Is(err, service.ErrAgentInputBlocked) {
			response.Error(c, http.StatusUnprocessableEntity, "CONTENT_BLOCKED", "content was rejected by content moderation")
			return
		}
		if errors.Is(err, service.ErrAgentModerationUnavailable) {
			response.Error(c, http.StatusServiceUnavailable, "MODERATION_UNAVAILABLE", "content moderation unavailable")
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "AGENT_ERROR", err)
		return
	}

	// The turn reservation: first Redis operation that consumes anything.
	// Failures refuse closed and consume nothing.
	if err := h.quota.Reserve(c.Request.Context(), deviceKey); err != nil {
		switch {
		case errors.Is(err, middleware.ErrGuestQuotaExceeded):
			response.Error(c, http.StatusTooManyRequests, "GUEST_QUOTA_EXHAUSTED", "guest turn budget is used up")
		case errors.Is(err, middleware.ErrGuestConcurrencyLimit):
			response.Error(c, http.StatusTooManyRequests, "GUEST_CONCURRENCY_LIMIT", "too many concurrent guest requests")
		case errors.Is(err, middleware.ErrGuestQuotaStateLost):
			response.Error(c, http.StatusForbidden, "GUEST_QUOTA_STATE_LOST", "guest quota state is unavailable")
		default:
			response.Error(c, http.StatusServiceUnavailable, "GUEST_QUOTA_UNAVAILABLE", "guest quota service is temporarily unavailable")
		}
		return
	}
	// Reserved: success, failure, timeout, stop and disconnect all consume
	// the turn; the in-flight slot comes back at any terminal outcome.
	defer func() {
		// The request context is frequently cancelled by the disconnect that
		// ended the turn — release on a detached bounded context.
		if err := h.quota.Release(context.Background(), deviceKey); err != nil {
			slog.Error("guest in-flight release failed", "error", err)
		}
	}()

	writer := &agentSSEWriter{c: c}
	writer.begin()
	if err := h.agentSvc.ChatStream(c.Request.Context(), 0, service.ChatTurnInput{
		ConversationID: conversationID,
		Message:        message,
		DeepThink:      body.DeepThink,
		Model:          modelPref,
		Locale:         body.Locale,
		GuestDeviceKey: deviceKey,
	}, resolved, func(ev service.AgentStreamEvent) error {
		return writer.emit(ev)
	}); err != nil {
		// The service already emitted a safe SSE error event.
		return
	}
}
