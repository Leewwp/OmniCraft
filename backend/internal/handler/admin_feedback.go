package handler

import (
	"net/http"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// adminFeedbackGetErrMap 是 GET /admin/feedback/:id 的 sentinel→HTTP 合同：
// 工单缺失 → 404；其余错误先落兜底日志再回 500 固定文案。
var adminFeedbackGetErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrFeedbackTicketNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "Feedback ticket not found"},
	},
	fallback: errFallback{
		status:  fallbackInternal.status,
		code:    fallbackInternal.code,
		message: "Failed to get feedback ticket",
		log:     "failed to get feedback ticket for admin",
	},
}

// adminFeedbackPatchErrMap 是 PATCH /admin/feedback/:id 审计事务错误的
// sentinel→HTTP 合同：审计写失败 → 500 AUDIT_WRITE_FAILED（首条规则，与
// 迁移前 if 前置分支同位）；事务错误按迁移前 switch 分支序映射；兜底固定
// 文案，无兜底日志。
var adminFeedbackPatchErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: errAdminAuditWriteFailed, status: http.StatusInternalServerError, code: "AUDIT_WRITE_FAILED", message: "audit write failed"},
		{sentinel: service.ErrFeedbackTicketNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "Feedback ticket not found"},
		{sentinel: service.ErrFeedbackInvalidStatus, status: http.StatusBadRequest, code: "INVALID_STATUS", message: "Invalid status value"},
		{sentinel: service.ErrFeedbackInvalidPriority, status: http.StatusBadRequest, code: "INVALID_PRIORITY", message: "Invalid priority value"},
		{sentinel: service.ErrFeedbackDeliveryFailed, status: http.StatusBadGateway, code: "FEEDBACK_DELIVERY_FAILED", message: "Feedback update delivery failed; please retry"},
	},
	fallback: errFallback{
		status:  fallbackInternal.status,
		code:    fallbackInternal.code,
		message: "Failed to update feedback ticket",
	},
}

// adminFeedbackPatchNotifyErrMap 是 PATCH /admin/feedback/:id 通知分支的
// sentinel→HTTP 合同：投递失败 → 502；其余错误先落兜底日志再回 500 固定
// 文案。
var adminFeedbackPatchNotifyErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrFeedbackDeliveryFailed, status: http.StatusBadGateway, code: "FEEDBACK_DELIVERY_FAILED", message: "Feedback update delivery failed; please retry"},
	},
	fallback: errFallback{
		status:  fallbackInternal.status,
		code:    fallbackInternal.code,
		message: "Failed to update feedback ticket",
		log:     "failed to notify patch ticket",
	},
}

// adminFeedbackReplyErrMap 是 POST /admin/feedback/:id/replies 审计事务错
// 误的 sentinel→HTTP 合同：审计写失败 → 500 AUDIT_WRITE_FAILED（首条规
// 则）；事务错误按迁移前 switch 分支序映射；兜底固定文案，无兜底日志。
var adminFeedbackReplyErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: errAdminAuditWriteFailed, status: http.StatusInternalServerError, code: "AUDIT_WRITE_FAILED", message: "audit write failed"},
		{sentinel: service.ErrFeedbackTicketNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "Feedback ticket not found"},
		{sentinel: service.ErrFeedbackBodyRequired, status: http.StatusBadRequest, code: "VALIDATION_ERROR", message: "Reply body is required"},
		{sentinel: service.ErrFeedbackBodyTooLong, status: http.StatusBadRequest, code: "VALIDATION_ERROR", message: "Reply body must not exceed 5000 characters"},
		{sentinel: service.ErrFeedbackDeliveryFailed, status: http.StatusBadGateway, code: "FEEDBACK_DELIVERY_FAILED", message: "Feedback reply delivery failed; please retry"},
	},
	fallback: errFallback{
		status:  fallbackInternal.status,
		code:    fallbackInternal.code,
		message: "Failed to create reply",
	},
}

// adminFeedbackReplyNotifyErrMap 是 POST /admin/feedback/:id/replies 通知
// 分支的 sentinel→HTTP 合同：投递失败 → 502；其余错误先落兜底日志再回
// 500 固定文案。
var adminFeedbackReplyNotifyErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrFeedbackDeliveryFailed, status: http.StatusBadGateway, code: "FEEDBACK_DELIVERY_FAILED", message: "Feedback reply delivery failed; please retry"},
	},
	fallback: errFallback{
		status:  fallbackInternal.status,
		code:    fallbackInternal.code,
		message: "Failed to create reply",
		log:     "failed to notify admin reply",
	},
}

type AdminFeedbackHandler struct {
	feedbackSvc *service.FeedbackService
	auditSvc    *service.AdminAuditService
	db          *gorm.DB
}

func NewAdminFeedbackHandler(db *gorm.DB, feedbackSvc *service.FeedbackService, auditSvc *service.AdminAuditService) *AdminFeedbackHandler {
	return &AdminFeedbackHandler{db: db, feedbackSvc: feedbackSvc, auditSvc: auditSvc}
}

func (h *AdminFeedbackHandler) ListFeedback(c *gin.Context) {
	page, pageSize := pageQuery(c, 20)

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
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list feedback tickets")
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
	ticketID, ok := pathID(c, "id", "Invalid ticket ID")
	if !ok {
		return
	}

	ticket, err := h.feedbackSvc.GetTicketForAdmin(c.Request.Context(), ticketID)
	if err != nil {
		adminFeedbackGetErrMap.write(c, err)
		return
	}

	c.JSON(http.StatusOK, ticket)
}

func (h *AdminFeedbackHandler) PatchFeedback(c *gin.Context) {
	ticketID, ok := pathID(c, "id", "Invalid ticket ID")
	if !ok {
		return
	}

	var req struct {
		Status          string `json:"status"`
		Priority        string `json:"priority"`
		AssigneeAdminID *int64 `json:"assignee_admin_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}

	input := service.AdminPatchFeedbackInput{
		Status:          req.Status,
		Priority:        req.Priority,
		AssigneeAdminID: req.AssigneeAdminID,
	}

	var ticket *model.FeedbackTicket
	err := h.withAuditTx(c, service.RecordAdminAuditInput{
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
		adminFeedbackPatchErrMap.write(c, err)
		return
	}

	if err := h.feedbackSvc.NotifyPatchTicket(c.Request.Context(), ticket, input); err != nil {
		adminFeedbackPatchNotifyErrMap.write(c, err)
		return
	}

	c.JSON(http.StatusOK, ticket)
}

func (h *AdminFeedbackHandler) ReplyFeedback(c *gin.Context) {
	ticketID, ok := pathID(c, "id", "Invalid ticket ID")
	if !ok {
		return
	}

	var req struct {
		Body           string `json:"body"`
		IsInternalNote bool   `json:"is_internal_note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
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
	err := h.withAuditTx(c, service.RecordAdminAuditInput{
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
		adminFeedbackReplyErrMap.write(c, err)
		return
	}

	if err := h.feedbackSvc.NotifyAdminReply(c.Request.Context(), ticket, input); err != nil {
		adminFeedbackReplyNotifyErrMap.write(c, err)
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
