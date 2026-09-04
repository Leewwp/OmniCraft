package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AppealHandler struct {
	appealRepo  *repository.AppealRepository
	contentRepo *repository.ContentRepository
	socialRepo  *repository.SocialRepository
}

func NewAppealHandler(db *gorm.DB) *AppealHandler {
	return &AppealHandler{
		appealRepo:  repository.NewAppealRepository(db),
		contentRepo: repository.NewContentRepository(db),
		socialRepo:  repository.NewSocialRepository(db),
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
				c.JSON(http.StatusNotFound, gin.H{"code": "TARGET_NOT_FOUND", "message": "appeal target not found"})
				return
			}
		case "comment":
			comment, err := h.socialRepo.FindComment(body.TargetID)
			if err != nil || comment == nil {
				c.JSON(http.StatusNotFound, gin.H{"code": "TARGET_NOT_FOUND", "message": "appeal target not found"})
				return
			}
		}
	}

	// T31（FIX-27 / F-099）：查重失败 fail-closed——DB 错误静默放行会让同一
	// 目标产生双 pending（appeals 表无 UNIQUE 兜底）。
	hasPending, err := h.appealRepo.HasPendingAppeal(callerID, body.TargetType, body.TargetID)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusServiceUnavailable, "APPEAL_CHECK_UNAVAILABLE", err)
		return
	}
	if hasPending {
		c.JSON(http.StatusConflict, gin.H{"code": "APPEAL_EXISTS", "message": "pending appeal already exists"})
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
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	appeals, total, err := h.appealRepo.ListByUser(callerID, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"appeals": appeals, "total": total})
}
