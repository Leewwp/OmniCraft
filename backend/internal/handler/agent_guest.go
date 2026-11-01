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

// guestConversationErrMap 是游客会话门（GetConversationMessages 与
// ChatStream 所有权门，迁移前同一 switch 合同）的 sentinel→HTTP 映射：
// 过期 → 410（客户端可安全丢弃死条目）、外属/缺失 → 404 防探测；未知错
// 误保持 Safe 500 AGENT_ERROR。
var guestConversationErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrAgentConversationExpired, status: http.StatusGone, code: "AGENT_CONVERSATION_EXPIRED", message: "conversation has expired"},
		{sentinel: service.ErrAgentConversationNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "conversation not found"},
	},
	fallback: errFallback{
		status: http.StatusInternalServerError,
		code:   "AGENT_ERROR",
	},
}

// guestQuotaReserveErrMap 是游客回合预留的 sentinel→HTTP 合同：超限与并
// 发上限 → 429 各自 code、状态丢失 → 403；其余 503 fail-closed 固定文案
// 兜底。
var guestQuotaReserveErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: middleware.ErrGuestQuotaExceeded, status: http.StatusTooManyRequests, code: "GUEST_QUOTA_EXHAUSTED", message: "guest turn budget is used up"},
		{sentinel: middleware.ErrGuestConcurrencyLimit, status: http.StatusTooManyRequests, code: "GUEST_CONCURRENCY_LIMIT", message: "too many concurrent guest requests"},
		{sentinel: middleware.ErrGuestQuotaStateLost, status: http.StatusForbidden, code: "GUEST_QUOTA_STATE_LOST", message: "guest quota state is unavailable"},
	},
	fallback: errFallback{
		status:  http.StatusServiceUnavailable,
		code:    "GUEST_QUOTA_UNAVAILABLE",
		message: "guest quota service is temporarily unavailable",
	},
}

// guestQuotaStatusErrMap 是配额余量查询的 503 fail-closed 兜底：无规则，
// 任何送入的错误都落该固定响应。ErrGuestQuotaStateLost 的放行（fail
// closed 读作余量 0，继续 200）是调用点的控制流谓词，不在 map 面。
var guestQuotaStatusErrMap = endpointErrMap{
	fallback: errFallback{
		status:  http.StatusServiceUnavailable,
		code:    "GUEST_QUOTA_UNAVAILABLE",
		message: "guest quota service is temporarily unavailable",
	},
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
		guestQuotaStatusErrMap.write(c, err)
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
		guestConversationErrMap.write(c, err)
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
			guestConversationErrMap.write(c, err)
			return
		}
	}

	// Anonymous viewer preload of client-supplied context: guest surface
	// accepts no content context, so the resolved context is always global.
	resolved := &service.ResolvedChatContext{Surface: model.AgentChatSurfaceGlobal}

	// A-05 input admission gate before reservation (same semantics as the
	// authenticated surface: block → 422, unavailable-in-release → 503).
	if err := h.agentSvc.ModerateChatInput(c.Request.Context(), message); err != nil {
		agentModerationErrMap.write(c, err)
		return
	}

	// The turn reservation: first Redis operation that consumes anything.
	// Failures refuse closed and consume nothing.
	if err := h.quota.Reserve(c.Request.Context(), deviceKey); err != nil {
		guestQuotaReserveErrMap.write(c, err)
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
