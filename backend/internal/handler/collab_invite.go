package handler

import (
	"errors"
	"net/http"
	"strconv"

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

	contentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
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
		mapCollabInviteError(c, err)
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

	inviteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid invite id"})
		return
	}

	invite, err := h.svc.AcceptInvite(c.Request.Context(), inviteID, callerID)
	if err != nil {
		mapCollabInviteError(c, err)
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

	inviteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid invite id"})
		return
	}

	invite, err := h.svc.DeclineInvite(c.Request.Context(), inviteID, callerID)
	if err != nil {
		mapCollabInviteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"invite": invite})
}

// mapCollabInviteError maps the CollabInviteService sentinels to the exact
// HTTP status and error codes from the community features design spec.
func mapCollabInviteError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInviteSelfNotAllowed):
		response.Error(c, http.StatusBadRequest, "INVITE_SELF_NOT_ALLOWED", "不能邀请自己")
	case errors.Is(err, service.ErrInviteAuthorNotAllowed):
		response.Error(c, http.StatusBadRequest, "INVITE_AUTHOR_NOT_ALLOWED", "不能邀请内容作者")
	case errors.Is(err, service.ErrInviteAlreadyContributor):
		response.Error(c, http.StatusConflict, "INVITE_ALREADY_CONTRIBUTOR", "对方已是联合创作者")
	case errors.Is(err, service.ErrInviteeUnavailable):
		response.Error(c, http.StatusNotFound, "INVITEE_UNAVAILABLE", "邀请对象不可用")
	case errors.Is(err, service.ErrContentUnavailable):
		response.Error(c, http.StatusNotFound, "CONTENT_UNAVAILABLE", "内容不可用")
	case errors.Is(err, service.ErrContributorLimitReached):
		response.Error(c, http.StatusConflict, "CONTRIBUTOR_LIMIT_REACHED", "联合创作者数量已达上限")
	case errors.Is(err, service.ErrNotContentOwner):
		response.Error(c, http.StatusForbidden, "NOT_CONTENT_OWNER", "仅作者或已确认的联合创作者可以邀请")
	case errors.Is(err, service.ErrReputationTooLow):
		response.Error(c, http.StatusForbidden, "REPUTATION_TOO_LOW", "信誉分不足，无法邀请")
	case errors.Is(err, service.ErrInviteDailyLimit):
		response.Error(c, http.StatusTooManyRequests, "INVITE_DAILY_LIMIT", "今日邀请次数已达上限")
	case errors.Is(err, service.ErrInviteDuplicateUser):
		response.Error(c, http.StatusConflict, "INVITE_DUPLICATE_USER", "今日已向该用户发送过邀请")
	case errors.Is(err, service.ErrInviteBlocked):
		response.Error(c, http.StatusForbidden, "INVITE_BLOCKED", "双方存在拉黑关系")
	case errors.Is(err, service.ErrInviteNotAccepting):
		response.Error(c, http.StatusForbidden, "INVITE_NOT_ACCEPTING", "该用户已关闭联合创作邀请接收")
	case errors.Is(err, service.ErrInviteAlreadyExists):
		response.Error(c, http.StatusConflict, "INVITE_ALREADY_EXISTS", "已向该用户发送过邀请")
	case errors.Is(err, service.ErrInviteRateLimitUnavailable):
		response.Error(c, http.StatusServiceUnavailable, "INVITE_SERVICE_UNAVAILABLE", "邀请服务暂时不可用")
	case errors.Is(err, service.ErrInviteNotFound):
		response.Error(c, http.StatusNotFound, "INVITE_NOT_FOUND", "邀请不存在")
	case errors.Is(err, service.ErrInviteExpired):
		response.Error(c, http.StatusBadRequest, "INVITE_EXPIRED", "邀请已过期")
	case errors.Is(err, service.ErrInviteNotInvitee):
		response.Error(c, http.StatusForbidden, "INVITE_NOT_INVITEE", "仅受邀用户可以操作该邀请")
	case errors.Is(err, service.ErrInviteNotPending):
		response.Error(c, http.StatusConflict, "INVITE_NOT_PENDING", "邀请已处理")
	default:
		response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
	}
}
