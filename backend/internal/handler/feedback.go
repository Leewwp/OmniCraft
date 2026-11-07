package handler

import (
	"net/http"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type FeedbackHandler struct {
	feedbackService *service.FeedbackService
}

// submitTicketErrMap 是 POST /feedback（SubmitTicket）的 sentinel→HTTP
// 合同，顺序与迁移前分支顺序逐条一致；兜底保留历史固定文案
// "Failed to submit feedback" 与兜底日志（fallbackMessage + fallbackLog
// 承接原 slog.Error 分支，日志先于响应）。
var submitTicketErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrFeedbackInvalidCategory, status: http.StatusBadRequest, code: "INVALID_CATEGORY", message: "Invalid feedback category"},
		{sentinel: service.ErrFeedbackTitleAndDescriptionReq, status: http.StatusBadRequest, code: "VALIDATION_ERROR", message: "Title and description are required"},
		{sentinel: service.ErrFeedbackTitleTooLong, status: http.StatusBadRequest, code: "VALIDATION_ERROR", message: "Title must not exceed 160 characters"},
		{sentinel: service.ErrFeedbackContactEmailRequired, status: http.StatusBadRequest, code: "CONTACT_EMAIL_REQUIRED", message: "Contact email is required for anonymous submissions"},
		{sentinel: service.ErrFeedbackCaptchaRequired, status: http.StatusBadRequest, code: "CAPTCHA_REQUIRED", message: "Captcha verification is required for anonymous submissions"},
		{sentinel: service.ErrFeedbackCaptchaFailed, status: http.StatusBadRequest, code: "CAPTCHA_FAILED", message: "Captcha verification failed"},
		{sentinel: service.ErrFeedbackUploadGrantInvalid, status: http.StatusBadRequest, code: "UPLOAD_GRANT_INVALID", message: "Screenshot upload grant is invalid or has already been used"},
		{sentinel: service.ErrFeedbackAttachmentBlocked, status: http.StatusBadRequest, code: "ATTACHMENT_BLOCKED", message: "Attachment image was rejected by content moderation"},
		{sentinel: service.ErrFeedbackAttachmentModerationUnavailable, status: http.StatusServiceUnavailable, code: "ATTACHMENT_MODERATION_UNAVAILABLE", message: "Attachment image moderation is temporarily unavailable, please retry without attachments"},
		{sentinel: service.ErrUploadGrantUnavailable, status: http.StatusServiceUnavailable, code: "UPLOAD_GRANT_UNAVAILABLE", message: "Screenshot upload grants are temporarily unavailable"},
	},
	fallback: errFallback{
		status:  fallbackInternal.status,
		code:    fallbackInternal.code,
		message: "Failed to submit feedback",
		log:     "failed to submit feedback",
	},
}

// presignUploadErrMap 是 POST /feedback/attachments/presign
// （PresignUpload）的 sentinel→HTTP 合同，顺序与迁移前分支顺序逐条一致
// （ErrOSSNotConfigured 原为 default 内嵌 errors.Is 分支，规则化后语义
// 等价：规则全不命中才落兜底）；兜底保留历史固定文案与兜底日志。
var presignUploadErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrFeedbackCaptchaRequired, status: http.StatusBadRequest, code: "CAPTCHA_REQUIRED", message: "Captcha verification is required for anonymous uploads"},
		{sentinel: service.ErrFeedbackCaptchaFailed, status: http.StatusBadRequest, code: "CAPTCHA_FAILED", message: "Captcha verification failed"},
		{sentinel: service.ErrFeedbackInvalidMimeType, status: http.StatusBadRequest, code: "INVALID_MIME_TYPE", message: "Only image uploads are supported for feedback screenshots"},
		{sentinel: service.ErrFeedbackFileTooLarge, status: http.StatusBadRequest, code: "FILE_TOO_LARGE", message: "Screenshot must be smaller than 20MB"},
		{sentinel: service.ErrUploadGrantUnavailable, status: http.StatusServiceUnavailable, code: "UPLOAD_GRANT_UNAVAILABLE", message: "Screenshot upload grants are temporarily unavailable"},
		{sentinel: service.ErrOSSNotConfigured, status: http.StatusServiceUnavailable, code: "OSS_NOT_CONFIGURED", message: "OSS upload is not configured"},
	},
	fallback: errFallback{
		status:  fallbackInternal.status,
		code:    fallbackInternal.code,
		message: "Failed to generate upload grant",
		log:     "failed to generate upload grant",
	},
}

// getTicketErrMap 是 GET /feedback/:id（GetTicket）的 sentinel→HTTP 合
// 同，顺序与迁移前分支顺序逐条一致；兜底保留历史固定文案与兜底日志。
var getTicketErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrFeedbackTicketNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "Feedback ticket not found"},
		{sentinel: service.ErrFeedbackForbidden, status: http.StatusForbidden, code: "FORBIDDEN", message: "You can only view your own tickets"},
	},
	fallback: errFallback{
		status:  fallbackInternal.status,
		code:    fallbackInternal.code,
		message: "Failed to get feedback ticket",
		log:     "failed to get feedback ticket",
	},
}

func NewFeedbackHandler(feedbackService *service.FeedbackService) *FeedbackHandler {
	return &FeedbackHandler{feedbackService: feedbackService}
}

func (h *FeedbackHandler) SubmitTicket(c *gin.Context) {
	var req struct {
		ContactEmail      string                 `json:"contact_email"`
		Category          string                 `json:"category"`
		Title             string                 `json:"title"`
		Description       string                 `json:"description"`
		DiagnosticSummary map[string]interface{} `json:"diagnostic_summary"`
		CaptchaToken      string                 `json:"captcha_token"`
		AttachmentKeys    []string               `json:"attachment_oss_keys"`
		AttachmentGrants  []struct {
			GrantID string `json:"grant_id"`
			OSSKey  string `json:"oss_key"`
		} `json:"attachment_grants"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}

	userID, exists := c.Get(middleware.UserIDKey)
	var uid *int64
	if exists {
		if id, ok := userID.(int64); ok && id > 0 {
			uid = &id
		}
	}

	attachmentGrants := make([]service.FeedbackAttachmentGrantInput, 0, len(req.AttachmentGrants))
	for _, grant := range req.AttachmentGrants {
		attachmentGrants = append(attachmentGrants, service.FeedbackAttachmentGrantInput{
			GrantID: grant.GrantID,
			OSSKey:  grant.OSSKey,
		})
	}

	input := service.SubmitTicketInput{
		UserID:            uid,
		ContactEmail:      req.ContactEmail,
		Category:          req.Category,
		Title:             req.Title,
		Description:       req.Description,
		DiagnosticSummary: req.DiagnosticSummary,
		CaptchaToken:      req.CaptchaToken,
		AttachmentOSSKeys: req.AttachmentKeys,
		AttachmentGrants:  attachmentGrants,
	}

	ticket, err := h.feedbackService.SubmitTicket(c.Request.Context(), input)
	if err != nil {
		submitTicketErrMap.write(c, err)
		return
	}

	c.JSON(http.StatusCreated, ticket)
}

func (h *FeedbackHandler) PresignUpload(c *gin.Context) {
	var req struct {
		FileName     string `json:"file_name"`
		MimeType     string `json:"mime_type"`
		SizeBytes    int64  `json:"size_bytes"`
		CaptchaToken string `json:"captcha_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}

	userID, exists := c.Get(middleware.UserIDKey)
	var uid *int64
	if exists {
		if id, ok := userID.(int64); ok && id > 0 {
			uid = &id
		}
	}

	input := service.PresignUploadInput{
		UserID:       uid,
		FileName:     req.FileName,
		MimeType:     req.MimeType,
		SizeBytes:    req.SizeBytes,
		CaptchaToken: req.CaptchaToken,
	}

	grant, err := h.feedbackService.PresignUpload(c.Request.Context(), input)
	if err != nil {
		presignUploadErrMap.write(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"grant_id":   grant.GrantID,
		"oss_key":    grant.OSSKey,
		"upload_url": grant.UploadURL,
		"expires_in": grant.ExpiresIn,
	})
}

func (h *FeedbackHandler) ListMyTickets(c *gin.Context) {
	userID, exists := c.Get(middleware.UserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	page, pageSize := pageQuery(c, 20)
	// #668：标准列表分页迁 pageQuery（缺省/无效/越界语义见 pagination.go）。

	tickets, total, err := h.feedbackService.ListUserTickets(c.Request.Context(), userID.(int64), page, pageSize)
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

func (h *FeedbackHandler) GetTicket(c *gin.Context) {
	userID, exists := c.Get(middleware.UserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	ticketID, ok := pathID(c, "id", "Invalid ticket ID")
	if !ok {
		return
	}

	ticket, err := h.feedbackService.GetTicketForUser(c.Request.Context(), ticketID, userID.(int64))
	if err != nil {
		getTicketErrMap.write(c, err)
		return
	}

	c.JSON(http.StatusOK, ticket)
}
