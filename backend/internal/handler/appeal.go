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
}

func NewAppealHandler(db *gorm.DB) *AppealHandler {
	return &AppealHandler{
		appealRepo:  repository.NewAppealRepository(db),
		contentRepo: repository.NewContentRepository(db),
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
	// 忽略请求体值防止替他人提交账号申诉）；其余目标仍要求有效 id。
	if body.TargetType == "account" {
		body.TargetID = callerID
	} else if body.TargetID <= 0 {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	hasPending, _ := h.appealRepo.HasPendingAppeal(callerID, body.TargetType, body.TargetID)
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
