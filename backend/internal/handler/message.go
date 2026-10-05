package handler

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/contentroute"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type MessageHandler struct {
	msgRepo       *repository.MessageRepository
	contentNav    *repository.ContentNavigationRepository
	notifSvc      *service.NotificationService
	cfg           *config.Config
	reviewSvc     service.TextReviewer
	displaySigner *service.DisplayURLSigner
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

// NewMessageHandler receives the container-owned repository (#658 PR-3).
// msgRepo 允许为 nil（部分装配的路由安全测试）：contentNav 相应留 nil，
// 装饰取数 fail-open 返回空表。
func NewMessageHandler(msgRepo *repository.MessageRepository) *MessageHandler {
	h := &MessageHandler{msgRepo: msgRepo}
	if msgRepo != nil {
		h.contentNav = repository.NewContentNavigationRepository(msgRepo.DB())
	}
	return h
}

func (h *MessageHandler) SetNotificationService(ns *service.NotificationService) {
	h.notifSvc = ns
}

func (h *MessageHandler) SetReviewService(cfg *config.Config, reviewSvc service.TextReviewer) {
	h.cfg = cfg
	h.reviewSvc = reviewSvc
	h.displaySigner = service.NewDisplayURLSigner(cfg)
}

func (h *MessageHandler) ListConversations(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	page, pageSize := pageQuery(c, 20)

	summaries, err := h.msgRepo.ListConversationSummaries(callerID, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"conversations": conversationDTOs(h.displaySigner, summaries),
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

	// B4（SP-19 G1-5）：私信长度上限与前端 MAX_DM_LENGTH 对齐，API 直发超长文本被拒。
	if h.cfg != nil && h.cfg.Limits.DMMaxLength > 0 && len([]rune(body.Text)) > h.cfg.Limits.DMMaxLength {
		response.ValidationError(c, "dm text exceeds maximum length")
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
		// T36（FIX-30b）：通知 body 只带摘要——私信全文仅存在于会话内，
		// 不得经通知列表/下拉泄露。
		h.notifSvc.Notify(body.RecipientID, "system", "message", "新私信", "你有一条新私信", "message", msg.ID, callerID)
	}

	c.JSON(http.StatusCreated, gin.H{"message": messageDTO(*msg)})
}

// moderateText runs the text moderation gate before a DM is persisted. Blank
// text is skipped without an external call. A "block" (or "violation") result
// rejects the message. Availability policy follows the A4 environment
// semantics via RunModerationGate: in release mode any moderation failure is
// fail-closed, while in local/test mode an unconfigured Green client is
// fail-open and must be recorded via structured logs.
func (h *MessageHandler) moderateText(ctx context.Context, action, text string) error {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	var review func(context.Context) (string, error)
	if h.reviewSvc != nil {
		review = func(ctx context.Context) (string, error) {
			return h.reviewSvc.ReviewText(ctx, trimmed)
		}
	}
	return service.RunModerationGate(ctx, h.cfg, action, "content moderation", "message",
		review, true, service.ErrTextBlocked, service.ErrModerationUnavailable)
}

func (h *MessageHandler) ListMessages(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	convID, ok := pathID(c, "id", "")
	if !ok {
		return
	}

	ok, _ = h.msgRepo.IsParticipant(callerID, convID)
	if !ok {
		response.CodeOnly(c, http.StatusForbidden, "FORBIDDEN")
		return
	}

	page, pageSize := pageQuery(c, 50)

	messages, total, err := h.msgRepo.ListMessages(convID, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	// #793：邀请卡深链读侧解析——DB 当前 zone 是导航唯一真源（写时不再落
	// content_zone 快照），只装饰响应，不回写历史消息。
	decorateCollabInviteNavigation(h.contentNav, messages)
	h.msgRepo.UpdateLastRead(callerID, convID)
	c.JSON(http.StatusOK, gin.H{"messages": messageDTOs(messages), "total": total})
}

func (h *MessageHandler) DeleteMessage(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	msgID, ok := pathID(c, "id", "")
	if !ok {
		return
	}
	if err := h.msgRepo.DeleteMessage(msgID, callerID); err != nil {
		response.Error(c, http.StatusInternalServerError, "DB_ERROR", "database error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *MessageHandler) LeaveConversation(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	convID, ok := pathID(c, "id", "")
	if !ok {
		return
	}
	if err := h.msgRepo.LeaveConversation(convID, callerID); err != nil {
		response.Error(c, http.StatusInternalServerError, "DB_ERROR", "database error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "left conversation"})
}

// decorateCollabInviteNavigation（#793）在已完成会话参与者校验的消息读取路径上，
// 为本页 collab_invite 消息装饰 metadata.content_zone / metadata.content_url：
//   - 按页去重收集 content_id、一次批量查询（ContentNavigationRepository），
//     不逐条查库；
//   - 旧行缺键、旧键错误（写时快照过期）、与当前 zone 不一致——一律以数据库
//     当前 zone 为准；路由计算走 contentroute 纯函数；
//   - 只改内存中的响应视图，不回写历史消息、不做数据迁移；content_id /
//     content_title / inviter_* 等事件快照字段原样保留；
//   - 目标不存在或装饰查询失败（SummariesByIDs fail-open 返回空表）：整条
//     metadata 原样保留（旧回退语义，前端 getContentHref 兜底），不用空值
//     覆盖已有键，也不制造 /content/0；内容访问仍由详情读取规则决定，
//     本装饰不扩大消息/内容授权范围。
func decorateCollabInviteNavigation(nav *repository.ContentNavigationRepository, messages []model.Message) {
	ids := make([]int64, 0, len(messages))
	seen := make(map[int64]bool, len(messages))
	for i := range messages {
		if messages[i].MsgType != "collab_invite" || messages[i].Metadata == nil {
			continue
		}
		if id, ok := collabInviteContentID(messages[i].Metadata); ok && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return
	}
	summaries := nav.SummariesByIDs(ids)
	if len(summaries) == 0 {
		return
	}
	for i := range messages {
		m := &messages[i]
		if m.MsgType != "collab_invite" || m.Metadata == nil {
			continue
		}
		contentID, ok := collabInviteContentID(m.Metadata)
		if !ok {
			continue
		}
		summary, resolved := summaries[contentID]
		if !resolved {
			continue
		}
		m.Metadata["content_zone"] = summary.Zone
		m.Metadata["content_url"] = contentroute.ContentDetailRoute(summary.Zone, contentID)
	}
}

// collabInviteContentID 提取邀请 metadata 的 content_id（JSONMap 经 JSON
// round-trip 后数字为 float64；新写入路径为 int64）。非正数视为无效——
// 恒发 URL+zone 只适用于可解析的有效内容目标。
func collabInviteContentID(metadata model.JSONMap) (int64, bool) {
	switch v := metadata["content_id"].(type) {
	case float64:
		if v > 0 && v == math.Trunc(v) {
			return int64(v), true
		}
	case int64:
		if v > 0 {
			return v, true
		}
	case int:
		if v > 0 {
			return int64(v), true
		}
	}
	return 0, false
}

func conversationDTOs(signer *service.DisplayURLSigner, summaries []repository.ConversationSummary) []ConversationDTO {
	dtos := make([]ConversationDTO, 0, len(summaries))
	for _, summary := range summaries {
		dto := ConversationDTO{
			ID:           summary.ID,
			Participants: participantDTOs(signer, summary.Participants),
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

func participantDTOs(signer *service.DisplayURLSigner, participants []repository.ConversationParticipantSummary) []ConversationParticipantDTO {
	dtos := make([]ConversationParticipantDTO, 0, len(participants))
	for _, participant := range participants {
		dtos = append(dtos, ConversationParticipantDTO{
			ID:        participant.ID,
			Username:  participant.Username,
			AvatarURL: signer.SignURL(participant.AvatarURL),
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
