package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/service"
)

type CollabInviteHandler struct {
	svc *service.CollabInviteService
}

func NewCollabInviteHandler(svc *service.CollabInviteService) *CollabInviteHandler {
	return &CollabInviteHandler{svc: svc}
}

// SendInvite handles POST /api/v1/contents/:id/collab-invites. The :id path
// parameter is always the content_item_id, never a content_series_id.
func (h *CollabInviteHandler) SendInvite(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		response.Unauthorized(c, "login required")
		return
	}

	contentID, ok := pathID(c, "id", "invalid content id")
	if !ok {
		return
	}

	var body struct {
		InviteeID int64 `json:"invitee_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	invite, err := h.svc.SendInvite(c.Request.Context(), contentID, callerID, body.InviteeID)
	if err != nil {
		collabInviteErrMap.write(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"invite": invite})
}

// AcceptInvite handles POST /api/v1/collab-invites/:id/accept. The :id path
// parameter is the invite id.
func (h *CollabInviteHandler) AcceptInvite(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		response.Unauthorized(c, "login required")
		return
	}

	inviteID, ok := pathID(c, "id", "invalid invite id")
	if !ok {
		return
	}

	invite, err := h.svc.AcceptInvite(c.Request.Context(), inviteID, callerID)
	if err != nil {
		collabInviteErrMap.write(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"invite": invite})
}

// DeclineInvite handles POST /api/v1/collab-invites/:id/decline. The :id path
// parameter is the invite id.
func (h *CollabInviteHandler) DeclineInvite(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		response.Unauthorized(c, "login required")
		return
	}

	inviteID, ok := pathID(c, "id", "invalid invite id")
	if !ok {
		return
	}

	invite, err := h.svc.DeclineInvite(c.Request.Context(), inviteID, callerID)
	if err != nil {
		collabInviteErrMap.write(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"invite": invite})
}

// collabInviteErrMap 是联合创作邀请三个路由（SendInvite / AcceptInvite /
// DeclineInvite，共享 mapCollabInviteError 合同）的 sentinel→HTTP 映
// 射，顺序与迁移前共享 switch 函数的 case 顺序逐条一致，status 与
// error code 来自社区功能设计 spec；未知错误兜底保持历史 500
// INTERNAL_ERROR Safe 语义。
var collabInviteErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrInviteSelfNotAllowed, status: http.StatusBadRequest, code: "INVITE_SELF_NOT_ALLOWED", message: "不能邀请自己"},
		{sentinel: service.ErrInviteAuthorNotAllowed, status: http.StatusBadRequest, code: "INVITE_AUTHOR_NOT_ALLOWED", message: "不能邀请内容作者"},
		{sentinel: service.ErrInviteAlreadyContributor, status: http.StatusConflict, code: "INVITE_ALREADY_CONTRIBUTOR", message: "对方已是联合创作者"},
		{sentinel: service.ErrInviteeUnavailable, status: http.StatusNotFound, code: "INVITEE_UNAVAILABLE", message: "邀请对象不可用"},
		{sentinel: service.ErrContentUnavailable, status: http.StatusNotFound, code: "CONTENT_UNAVAILABLE", message: "内容不可用"},
		{sentinel: service.ErrContributorLimitReached, status: http.StatusConflict, code: "CONTRIBUTOR_LIMIT_REACHED", message: "联合创作者数量已达上限"},
		{sentinel: service.ErrNotContentOwner, status: http.StatusForbidden, code: "NOT_CONTENT_OWNER", message: "仅作者或已确认的联合创作者可以邀请"},
		{sentinel: service.ErrReputationTooLow, status: http.StatusForbidden, code: "REPUTATION_TOO_LOW", message: "信誉分不足，无法邀请"},
		{sentinel: service.ErrInviteDailyLimit, status: http.StatusTooManyRequests, code: "INVITE_DAILY_LIMIT", message: "今日邀请次数已达上限"},
		{sentinel: service.ErrInviteDuplicateUser, status: http.StatusConflict, code: "INVITE_DUPLICATE_USER", message: "今日已向该用户发送过邀请"},
		{sentinel: service.ErrInviteBlocked, status: http.StatusForbidden, code: "INVITE_BLOCKED", message: "双方存在拉黑关系"},
		{sentinel: service.ErrInviteNotAccepting, status: http.StatusForbidden, code: "INVITE_NOT_ACCEPTING", message: "该用户已关闭联合创作邀请接收"},
		{sentinel: service.ErrInviteAlreadyExists, status: http.StatusConflict, code: "INVITE_ALREADY_EXISTS", message: "已向该用户发送过邀请"},
		{sentinel: service.ErrInviteRateLimitUnavailable, status: http.StatusServiceUnavailable, code: "INVITE_SERVICE_UNAVAILABLE", message: "邀请服务暂时不可用"},
		{sentinel: service.ErrInviteNotFound, status: http.StatusNotFound, code: "INVITE_NOT_FOUND", message: "邀请不存在"},
		{sentinel: service.ErrInviteExpired, status: http.StatusBadRequest, code: "INVITE_EXPIRED", message: "邀请已过期"},
		{sentinel: service.ErrInviteNotInvitee, status: http.StatusForbidden, code: "INVITE_NOT_INVITEE", message: "仅受邀用户可以操作该邀请"},
		{sentinel: service.ErrInviteNotPending, status: http.StatusConflict, code: "INVITE_NOT_PENDING", message: "邀请已处理"},
	},
	fallback: fallbackInternal,
}
