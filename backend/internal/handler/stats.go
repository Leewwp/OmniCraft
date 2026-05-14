package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/internal/service"
)

type StatsHandler struct {
	svc *service.StatsService
}

func NewStatsHandler(svc *service.StatsService) *StatsHandler {
	return &StatsHandler{svc: svc}
}

func (h *StatsHandler) GetSummary(c *gin.Context) {
	summary, err := h.svc.GetSummary(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "INTERNAL_ERROR",
			"message": "Failed to load stats",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"summary": summary,
	})
}
