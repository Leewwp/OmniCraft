package handler

import (
	"net/http"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AdminFeedbackHandler struct {
	feedbackSvc *service.FeedbackService
	auditSvc    *service.AdminAuditService
}

func NewAdminFeedbackHandler(feedbackSvc *service.FeedbackService, auditSvc *service.AdminAuditService) *AdminFeedbackHandler {
	return &AdminFeedbackHandler{feedbackSvc: feedbackSvc, auditSvc: auditSvc}
}

func (h *AdminFeedbackHandler) ListFeedback(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	filter := repository.AdminFeedbackFilter{
		Status:   c.DefaultQuery("status", ""),
		Category: c.DefaultQuery("category", ""),
		Priority: c.DefaultQuery("priority", ""),
		Page:     page,
		PageSize: pageSize,
	}

	if assigneeStr := c.Query("assignee_id"); assigneeStr != "" {
		aid, err := strconv.ParseInt(assigneeStr, 10, 64)
		if err == nil {
			filter.AssigneeID = &aid
		}
	}

	tickets, total, err := h.feedbackSvc.ListAdminFeedback(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to list feedback tickets"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":     tickets,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *AdminFeedbackHandler) GetFeedback(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "Invalid ticket ID"})
		return
	}

	ticket, err := h.feedbackSvc.GetTicketForAdmin(c.Request.Context(), ticketID)
	if err != nil {
		if err.Error() == "TICKET_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "Feedback ticket not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to get feedback ticket"})
		return
	}

	c.JSON(http.StatusOK, ticket)
}

func (h *AdminFeedbackHandler) PatchFeedback(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "Invalid ticket ID"})
		return
	}

	var req struct {
		Status          string `json:"status"`
		Priority        string `json:"priority"`
		AssigneeAdminID *int64 `json:"assignee_admin_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_BODY", "message": "Invalid request body"})
		return
	}

	input := service.AdminPatchFeedbackInput{
		Status:          req.Status,
		Priority:        req.Priority,
		AssigneeAdminID: req.AssigneeAdminID,
	}

	ticket, err := h.feedbackSvc.PatchTicket(c.Request.Context(), ticketID, input)
	if err != nil {
		switch err.Error() {
		case "TICKET_NOT_FOUND":
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "Feedback ticket not found"})
		case "INVALID_STATUS":
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_STATUS", "message": "Invalid status value"})
		case "INVALID_PRIORITY":
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_PRIORITY", "message": "Invalid priority value"})
		case "FEEDBACK_NOTIFICATION_FAILED":
			c.JSON(http.StatusBadGateway, gin.H{"code": "FEEDBACK_NOTIFICATION_FAILED", "message": "Feedback update notification failed; please retry"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to update feedback ticket"})
		}
		return
	}

	metadata := map[string]any{
		"ticket_id": ticketID,
	}
	if req.Status != "" {
		metadata["status"] = req.Status
	}
	if req.Priority != "" {
		metadata["priority"] = req.Priority
	}
	action := "feedback_priority"
	if req.Status != "" {
		action = "feedback_close"
		if req.Status == "reopened" {
			action = "feedback_reopen"
		}
	}

	if h.auditSvc != nil {
		adminID := middleware.GetUserID(c)
		entry := service.RecordAdminAuditInput{
			AdminUserID: adminID,
			Action:      action,
			TargetType:  "feedback_ticket",
			TargetID:    strconv.FormatInt(ticketID, 10),
			TraceID:     c.GetString("trace_id"),
			Metadata:    metadata,
			Result:      "success",
		}
		if err := h.auditSvc.Record(c.Request.Context(), entry); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "AUDIT_WRITE_FAILED", "message": "audit write failed"})
			return
		}
	}

	c.JSON(http.StatusOK, ticket)
}

func (h *AdminFeedbackHandler) ReplyFeedback(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "Invalid ticket ID"})
		return
	}

	var req struct {
		Body           string `json:"body"`
		IsInternalNote bool   `json:"is_internal_note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_BODY", "message": "Invalid request body"})
		return
	}

	adminID := middleware.GetUserID(c)

	input := service.AdminReplyInput{
		TicketID:       ticketID,
		AuthorAdminID:  adminID,
		Body:           req.Body,
		IsInternalNote: req.IsInternalNote,
	}

	reply, err := h.feedbackSvc.AdminReply(c.Request.Context(), input)
	if err != nil {
		switch err.Error() {
		case "TICKET_NOT_FOUND":
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "Feedback ticket not found"})
		case "BODY_REQUIRED":
			c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "Reply body is required"})
		case "BODY_TOO_LONG":
			c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "Reply body must not exceed 5000 characters"})
		case "FEEDBACK_NOTIFICATION_FAILED":
			c.JSON(http.StatusBadGateway, gin.H{"code": "FEEDBACK_NOTIFICATION_FAILED", "message": "Feedback reply notification failed; please retry"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to create reply"})
		}
		return
	}

	if h.auditSvc != nil {
		entry := service.RecordAdminAuditInput{
			AdminUserID: adminID,
			Action:      "feedback_reply",
			TargetType:  "feedback_ticket",
			TargetID:    strconv.FormatInt(ticketID, 10),
			TraceID:     c.GetString("trace_id"),
			Metadata: map[string]any{
				"ticket_id":        ticketID,
				"is_internal_note": req.IsInternalNote,
			},
			Result: "success",
		}
		if err := h.auditSvc.Record(c.Request.Context(), entry); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "AUDIT_WRITE_FAILED", "message": "audit write failed"})
			return
		}
	}

	c.JSON(http.StatusCreated, reply)
}
