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
		TargetType string `json:"target_type" binding:"required,oneof=content comment"`
		TargetID   int64  `json:"target_id" binding:"required"`
		Reason     string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
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
