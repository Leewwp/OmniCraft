package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MessageHandler struct {
	msgRepo   *repository.MessageRepository
	notifSvc  *service.NotificationService
	cfg       *config.Config
	reviewSvc service.TextReviewer
}

type MessageDTO struct {
	ID        int64         `json:"id"`
	SenderID  int64         `json:"sender_id"`
	Text      string        `json:"text"`
	Body      string        `json:"body"`
	MsgType   string        `json:"msg_type"`
	Metadata  model.JSONMap `json:"metadata,omitempty"`
	CreatedAt string        `json:"created_at"`
}

type ConversationParticipantDTO struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

type ConversationDTO struct {
	ID           int64                        `json:"id"`
	Participants []ConversationParticipantDTO `json:"participants"`
	LastMessage  *MessageDTO                  `json:"last_message"`
	UnreadCount  int                          `json:"unread_count"`
	UpdatedAt    string                       `json:"updated_at"`
}

func NewMessageHandler(db *gorm.DB) *MessageHandler {
	return &MessageHandler{msgRepo: repository.NewMessageRepository(db)}
}

func (h *MessageHandler) SetNotificationService(ns *service.NotificationService) {
	h.notifSvc = ns
}

func (h *MessageHandler) SetReviewService(cfg *config.Config, reviewSvc service.TextReviewer) {
	h.cfg = cfg
	h.reviewSvc = reviewSvc
}

func (h *MessageHandler) ListConversations(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	summaries, err := h.msgRepo.ListConversationSummaries(callerID, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"conversations": conversationDTOs(summaries),
		"page":          page,
		"page_size":     pageSize,
	})
}

func (h *MessageHandler) SendMessage(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	var body struct {
		RecipientID int64  `json:"recipient_id" binding:"required"`
		Text        string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	if err := h.moderateText(c.Request.Context(), "dm", body.Text); err != nil {
		if errors.Is(err, service.ErrTextBlocked) {
			response.Error(c, http.StatusUnprocessableEntity, "CONTENT_BLOCKED", "内容包含违规内容，无法发送")
			return
		}
		if errors.Is(err, service.ErrModerationUnavailable) {
			response.Error(c, http.StatusServiceUnavailable, "MODERATION_UNAVAILABLE", "内容审核服务暂时不可用，请稍后重试")
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	msg, err := h.msgRepo.SendWithColdStartGuard(callerID, body.RecipientID, body.Text)
	if errors.Is(err, repository.ErrDMReplyRequired) {
		response.Error(c, http.StatusForbidden, "DM_REPLY_REQUIRED", "对方尚未回复，请等待回复后再发送新消息")
		return
	}
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	if h.notifSvc != nil {
		h.notifSvc.Notify(body.RecipientID, "system", "message", "新私信", body.Text, "message", msg.ID, callerID)
	}

	c.JSON(http.StatusCreated, gin.H{"message": messageDTO(*msg)})
}

// moderateText runs the text moderation gate before a DM is persisted. Blank
// text is skipped without an external call. A "block" (or "violation") result
// rejects the message. Availability policy follows the A4 environment
// semantics: in release mode any moderation failure is fail-closed, while in
// local/test mode an unconfigured Green client is fail-open and must be
// recorded via structured logs.
func (h *MessageHandler) moderateText(ctx context.Context, action, text string) error {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}

	if h.reviewSvc == nil {
		if h.isReleaseMode() {
			slog.Error("content moderation unavailable, rejecting message",
				"action", action, "env_mode", h.environmentMode(), "policy", "fail_closed", "reason", "review_service_not_wired")
			return service.ErrModerationUnavailable
		}
		slog.Warn("content moderation skipped, message allowed",
			"action", action, "env_mode", h.environmentMode(), "policy", "fail_open", "reason", "review_service_not_wired")
		return nil
	}

	result, err := h.reviewSvc.ReviewText(ctx, trimmed)
	if err != nil {
		if !h.isReleaseMode() && errors.Is(err, aliyun.ErrGreenNotConfigured) {
			slog.Warn("content moderation skipped, message allowed",
				"action", action, "env_mode", h.environmentMode(), "policy", "fail_open", "reason", "green_not_configured")
			return nil
		}
		slog.Error("content moderation unavailable, rejecting message",
			"action", action, "env_mode", h.environmentMode(), "policy", "fail_closed", "reason", err.Error())
		return service.ErrModerationUnavailable
	}

	if result == "block" || result == "violation" {
		return service.ErrTextBlocked
	}
	return nil
}

func (h *MessageHandler) isReleaseMode() bool {
	return h.cfg != nil && h.cfg.Server.Mode == "release"
}

func (h *MessageHandler) environmentMode() string {
	if h.cfg == nil {
		return "unknown"
	}
	return h.cfg.Server.Mode
}

func (h *MessageHandler) ListMessages(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID"})
		return
	}

	ok, _ := h.msgRepo.IsParticipant(callerID, convID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	messages, total, err := h.msgRepo.ListMessages(convID, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	h.msgRepo.UpdateLastRead(callerID, convID)
	c.JSON(http.StatusOK, gin.H{"messages": messageDTOs(messages), "total": total})
}

func (h *MessageHandler) DeleteMessage(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	msgID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID"})
		return
	}
	if err := h.msgRepo.DeleteMessage(msgID, callerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *MessageHandler) LeaveConversation(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID"})
		return
	}
	if err := h.msgRepo.LeaveConversation(convID, callerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "left conversation"})
}

func conversationDTOs(summaries []repository.ConversationSummary) []ConversationDTO {
	dtos := make([]ConversationDTO, 0, len(summaries))
	for _, summary := range summaries {
		dto := ConversationDTO{
			ID:           summary.ID,
			Participants: participantDTOs(summary.Participants),
			UnreadCount:  summary.UnreadCount,
			UpdatedAt:    formatMessageTime(summary.UpdatedAt),
		}
		if summary.LastMessage != nil {
			message := messageDTO(*summary.LastMessage)
			dto.LastMessage = &message
		}
		dtos = append(dtos, dto)
	}
	return dtos
}

func participantDTOs(participants []repository.ConversationParticipantSummary) []ConversationParticipantDTO {
	dtos := make([]ConversationParticipantDTO, 0, len(participants))
	for _, participant := range participants {
		dtos = append(dtos, ConversationParticipantDTO{
			ID:        participant.ID,
			Username:  participant.Username,
			AvatarURL: participant.AvatarURL,
		})
	}
	return dtos
}

func messageDTOs(messages []model.Message) []MessageDTO {
	dtos := make([]MessageDTO, 0, len(messages))
	for _, message := range messages {
		dtos = append(dtos, messageDTO(message))
	}
	return dtos
}

func messageDTO(message model.Message) MessageDTO {
	return MessageDTO{
		ID:        message.ID,
		SenderID:  message.SenderID,
		Text:      message.Body,
		Body:      message.Body,
		MsgType:   message.MsgType,
		Metadata:  message.Metadata,
		CreatedAt: formatMessageTime(message.CreatedAt),
	}
}

func formatMessageTime(value time.Time) string {
	return value.Format(time.RFC3339Nano)
}
