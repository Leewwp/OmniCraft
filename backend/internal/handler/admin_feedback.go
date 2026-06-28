package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AdminFeedbackHandler struct {
	feedbackSvc *service.FeedbackService
	auditSvc    *service.AdminAuditService
	db          *gorm.DB
}

func NewAdminFeedbackHandler(db *gorm.DB, feedbackSvc *service.FeedbackService, auditSvc *service.AdminAuditService) *AdminFeedbackHandler {
	return &AdminFeedbackHandler{db: db, feedbackSvc: feedbackSvc, auditSvc: auditSvc}
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
		if errors.Is(err, service.ErrFeedbackTicketNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "Feedback ticket not found"})
			return
		}
		slog.Error("failed to get feedback ticket for admin", "error", err)
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

	var ticket *model.FeedbackTicket
	err = h.withAuditTx(c, service.RecordAdminAuditInput{
		AdminUserID: middleware.GetUserID(c),
		Action:      feedbackPatchAuditAction(req.Status),
		TargetType:  "feedback_ticket",
		TargetID:    strconv.FormatInt(ticketID, 10),
		TraceID:     c.GetString("trace_id"),
		Metadata:    feedbackPatchAuditMetadata(ticketID, req.Status, req.Priority, req.AssigneeAdminID),
		Result:      "success",
	}, func(tx *gorm.DB) error {
		var txErr error
		ticket, txErr = h.feedbackSvc.PatchTicketTx(c.Request.Context(), tx, ticketID, input)
		return txErr
	})
	if err != nil {
		if errors.Is(err, errAdminAuditWriteFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "AUDIT_WRITE_FAILED", "message": "audit write failed"})
			return
		}
		switch {
		case errors.Is(err, service.ErrFeedbackTicketNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "Feedback ticket not found"})
		case errors.Is(err, service.ErrFeedbackInvalidStatus):
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_STATUS", "message": "Invalid status value"})
		case errors.Is(err, service.ErrFeedbackInvalidPriority):
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_PRIORITY", "message": "Invalid priority value"})
		case errors.Is(err, service.ErrFeedbackDeliveryFailed):
			c.JSON(http.StatusBadGateway, gin.H{"code": "FEEDBACK_DELIVERY_FAILED", "message": "Feedback update delivery failed; please retry"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to update feedback ticket"})
		}
		return
	}

	if err := h.feedbackSvc.NotifyPatchTicket(c.Request.Context(), ticket, input); err != nil {
		if errors.Is(err, service.ErrFeedbackDeliveryFailed) {
			c.JSON(http.StatusBadGateway, gin.H{"code": "FEEDBACK_DELIVERY_FAILED", "message": "Feedback update delivery failed; please retry"})
			return
		}
		slog.Error("failed to notify patch ticket", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to update feedback ticket"})
		return
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

	var reply *model.FeedbackReply
	var ticket *model.FeedbackTicket
	err = h.withAuditTx(c, service.RecordAdminAuditInput{
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
	}, func(tx *gorm.DB) error {
		var txErr error
		reply, ticket, txErr = h.feedbackSvc.AdminReplyTx(c.Request.Context(), tx, input)
		return txErr
	})
	if err != nil {
		if errors.Is(err, errAdminAuditWriteFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "AUDIT_WRITE_FAILED", "message": "audit write failed"})
			return
		}
		switch {
		case errors.Is(err, service.ErrFeedbackTicketNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "Feedback ticket not found"})
		case errors.Is(err, service.ErrFeedbackBodyRequired):
			c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "Reply body is required"})
		case errors.Is(err, service.ErrFeedbackBodyTooLong):
			c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "Reply body must not exceed 5000 characters"})
		case errors.Is(err, service.ErrFeedbackDeliveryFailed):
			c.JSON(http.StatusBadGateway, gin.H{"code": "FEEDBACK_DELIVERY_FAILED", "message": "Feedback reply delivery failed; please retry"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to create reply"})
		}
		return
	}

	if err := h.feedbackSvc.NotifyAdminReply(c.Request.Context(), ticket, input); err != nil {
		if errors.Is(err, service.ErrFeedbackDeliveryFailed) {
			c.JSON(http.StatusBadGateway, gin.H{"code": "FEEDBACK_DELIVERY_FAILED", "message": "Feedback reply delivery failed; please retry"})
			return
		}
		slog.Error("failed to notify admin reply", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to create reply"})
		return
	}

	c.JSON(http.StatusCreated, reply)
}

func (h *AdminFeedbackHandler) withAuditTx(c *gin.Context, entry service.RecordAdminAuditInput, mutate func(tx *gorm.DB) error) error {
	return h.db.Transaction(func(tx *gorm.DB) error {
		if err := mutate(tx); err != nil {
			return err
		}
		if h.auditSvc == nil {
			return nil
		}
		if err := h.auditSvc.RecordTx(c.Request.Context(), tx, entry); err != nil {
			return errAdminAuditWriteFailed
		}
		return nil
	})
}

func feedbackPatchAuditAction(status string) string {
	if status == "" {
		return "feedback_priority"
	}
	if status == "reopened" {
		return "feedback_reopen"
	}
	return "feedback_close"
}

func feedbackPatchAuditMetadata(ticketID int64, status, priority string, assigneeID *int64) map[string]any {
	metadata := map[string]any{"ticket_id": ticketID}
	if status != "" {
		metadata["status"] = status
	}
	if priority != "" {
		metadata["priority"] = priority
	}
	if assigneeID != nil {
		metadata["assignee_admin_id"] = *assigneeID
	}
	return metadata
}
