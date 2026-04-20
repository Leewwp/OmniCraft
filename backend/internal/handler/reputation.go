package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ReputationHandler struct {
	reputationSvc *service.ReputationService
}

func NewReputationHandler(db *gorm.DB) *ReputationHandler {
	return &ReputationHandler{
		reputationSvc: service.NewReputationService(db),
	}
}

func (h *ReputationHandler) GetMyReputationLogs(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	logs, total, err := h.reputationSvc.GetLogs(callerID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs, "total": total})
}
