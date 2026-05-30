package handler

import (
	"context"
	"net/http"

	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type IPStatsHandler struct {
	statsSvc *service.IPStatsService
}

func NewIPStatsHandler(statsSvc *service.IPStatsService) *IPStatsHandler {
	return &IPStatsHandler{statsSvc: statsSvc}
}

func (h *IPStatsHandler) GetCategoryCounts(c *gin.Context) {
	counts, err := h.statsSvc.GetCategoryCounts(context.Background())
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"category_counts": counts})
}
