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
	// #411 F3：可选 zone 参数（original|fanwork）——分区统计（内容数按区
	// 过滤、创作者 = 区内去重作者数）；缺省维持全局语义（向后兼容）。
	zone := c.Query("zone")
	if !service.ValidStatsZone(zone) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_PARAM",
			"message": "zone must be original or fanwork",
		})
		return
	}
	summary, err := h.svc.GetSummaryForZone(c.Request.Context(), zone)
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
