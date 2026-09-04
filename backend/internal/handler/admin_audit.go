package handler

import (
	"net/http"
	"omnicraft/backend/internal/service"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type AdminAuditHandler struct {
	auditSvc *service.AdminAuditService
}

func NewAdminAuditHandler(auditSvc *service.AdminAuditService) *AdminAuditHandler {
	return &AdminAuditHandler{auditSvc: auditSvc}
}

func (h *AdminAuditHandler) ListAuditActions(c *gin.Context) {
	actions, err := h.auditSvc.DistinctActions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to list audit actions"})
		return
	}
	if actions == nil {
		actions = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"actions": actions})
}

func (h *AdminAuditHandler) ListAuditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	filter := service.AdminAuditFilter{
		Action:   c.DefaultQuery("action", ""),
		Page:     page,
		PageSize: pageSize,
	}

	if adminIDStr := c.Query("admin_user_id"); adminIDStr != "" {
		aid, err := strconv.ParseInt(adminIDStr, 10, 64)
		if err == nil {
			filter.AdminUserID = aid
		}
	}

	if fromStr := c.Query("from"); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err == nil {
			filter.From = &t
		}
	}

	if toStr := c.Query("to"); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err == nil {
			filter.To = &t
		}
	}

	logs, total, err := h.auditSvc.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to list audit logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":     logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
