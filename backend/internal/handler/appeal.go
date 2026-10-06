package handler

import (
	"net/http"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"

	"github.com/gin-gonic/gin"
)

type AppealHandler struct {
	appealRepo  *repository.AppealRepository
	contentRepo *repository.ContentRepository
	socialRepo  *repository.SocialRepository
}

// NewAppealHandler receives the container-owned repositories (#658 PR-3).
func NewAppealHandler(appealRepo *repository.AppealRepository, contentRepo *repository.ContentRepository, socialRepo *repository.SocialRepository) *AppealHandler {
	return &AppealHandler{
		appealRepo:  appealRepo,
		contentRepo: contentRepo,
		socialRepo:  socialRepo,
	}
}

func (h *AppealHandler) SubmitAppeal(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	var body struct {
		TargetType string `json:"target_type" binding:"required,oneof=content comment account"`
		TargetID   int64  `json:"target_id"`
		Reason     string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	// T29（FIX-15）：account 申诉固定指向申诉者本人（免填 target_id，
	// 忽略请求体值防止替他人提交账号申诉；本人存在性由 token 保证）。
	// T31（FIX-27）：content/comment 目标必须真实存在，防假 id 污染 admin 队列。
	// run-1 审计 #13：content/comment 目标还须归属申诉者本人——任何人不得
	// 对他人的内容/评论提申诉（此前仅校验存在性）。
	if body.TargetType == "account" {
		body.TargetID = callerID
	} else {
		if body.TargetID <= 0 {
			response.ValidationError(c, "invalid request parameters")
			return
		}
		switch body.TargetType {
		case "content":
			item, err := h.contentRepo.FindByID(body.TargetID)
			if err != nil || item == nil {
				response.Error(c, http.StatusNotFound, "TARGET_NOT_FOUND", "appeal target not found")
				return
			}
			if item.AuthorID != callerID {
				response.Error(c, http.StatusForbidden, "FORBIDDEN", "only the target owner may appeal")
				return
			}
		case "comment":
			comment, err := h.socialRepo.FindComment(body.TargetID)
			if err != nil || comment == nil {
				response.Error(c, http.StatusNotFound, "TARGET_NOT_FOUND", "appeal target not found")
				return
			}
			if comment.AuthorID != callerID {
				response.Error(c, http.StatusForbidden, "FORBIDDEN", "only the target owner may appeal")
				return
			}
		}
	}

	// T31（FIX-27 / F-099）：查重失败 fail-closed——DB 错误静默放行会让同一
	// 目标产生双 pending（appeals 表无 UNIQUE 兜底）。run-1 审计 #13：单一
	// pending 不变量按 target 维度执行（同目标最多一个 pending，不论提交者）。
	hasPending, err := h.appealRepo.HasPendingAppealForTarget(body.TargetType, body.TargetID)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusServiceUnavailable, "APPEAL_CHECK_UNAVAILABLE", err)
		return
	}
	if hasPending {
		response.Error(c, http.StatusConflict, "APPEAL_EXISTS", "pending appeal already exists")
		return
	}

	appeal := &model.Appeal{
		UserID:     callerID,
		TargetType: body.TargetType,
		TargetID:   body.TargetID,
		Reason:     body.Reason,
		Status:     "pending",
	}
	if err := h.appealRepo.Create(appeal); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"appeal": appeal})
}

func (h *AppealHandler) GetMyAppeals(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	page, pageSize := pageQuery(c, 20)

	appeals, total, err := h.appealRepo.ListByUser(callerID, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"appeals": appeals, "total": total})
}
