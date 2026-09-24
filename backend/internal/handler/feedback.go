package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

type FeedbackHandler struct {
	feedbackService *service.FeedbackService
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
		switch {
		case errors.Is(err, service.ErrFeedbackInvalidCategory):
			response.Error(c, http.StatusBadRequest, "INVALID_CATEGORY", "Invalid feedback category")
		case errors.Is(err, service.ErrFeedbackTitleAndDescriptionReq):
			response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "Title and description are required")
		case errors.Is(err, service.ErrFeedbackTitleTooLong):
			response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "Title must not exceed 160 characters")
		case errors.Is(err, service.ErrFeedbackContactEmailRequired):
			response.Error(c, http.StatusBadRequest, "CONTACT_EMAIL_REQUIRED", "Contact email is required for anonymous submissions")
		case errors.Is(err, service.ErrFeedbackCaptchaRequired):
			response.Error(c, http.StatusBadRequest, "CAPTCHA_REQUIRED", "Captcha verification is required for anonymous submissions")
		case errors.Is(err, service.ErrFeedbackCaptchaFailed):
			response.Error(c, http.StatusBadRequest, "CAPTCHA_FAILED", "Captcha verification failed")
		case errors.Is(err, service.ErrFeedbackUploadGrantInvalid):
			response.Error(c, http.StatusBadRequest, "UPLOAD_GRANT_INVALID", "Screenshot upload grant is invalid or has already been used")
		case errors.Is(err, service.ErrFeedbackAttachmentBlocked):
			response.Error(c, http.StatusBadRequest, "ATTACHMENT_BLOCKED", "Attachment image was rejected by content moderation")
		case errors.Is(err, service.ErrFeedbackAttachmentModerationUnavailable):
			response.Error(c, http.StatusServiceUnavailable, "ATTACHMENT_MODERATION_UNAVAILABLE", "Attachment image moderation is temporarily unavailable, please retry without attachments")
		case errors.Is(err, service.ErrUploadGrantUnavailable):
			response.Error(c, http.StatusServiceUnavailable, "UPLOAD_GRANT_UNAVAILABLE", "Screenshot upload grants are temporarily unavailable")
		default:
			slog.Error("failed to submit feedback", "error", err)
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to submit feedback")
		}
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
		switch {
		case errors.Is(err, service.ErrFeedbackCaptchaRequired):
			response.Error(c, http.StatusBadRequest, "CAPTCHA_REQUIRED", "Captcha verification is required for anonymous uploads")
		case errors.Is(err, service.ErrFeedbackCaptchaFailed):
			response.Error(c, http.StatusBadRequest, "CAPTCHA_FAILED", "Captcha verification failed")
		case errors.Is(err, service.ErrFeedbackInvalidMimeType):
			response.Error(c, http.StatusBadRequest, "INVALID_MIME_TYPE", "Only image uploads are supported for feedback screenshots")
		case errors.Is(err, service.ErrFeedbackFileTooLarge):
			response.Error(c, http.StatusBadRequest, "FILE_TOO_LARGE", "Screenshot must be smaller than 20MB")
		case errors.Is(err, service.ErrUploadGrantUnavailable):
			response.Error(c, http.StatusServiceUnavailable, "UPLOAD_GRANT_UNAVAILABLE", "Screenshot upload grants are temporarily unavailable")
		default:
			if errors.Is(err, service.ErrOSSNotConfigured) {
				response.Error(c, http.StatusServiceUnavailable, "OSS_NOT_CONFIGURED", "OSS upload is not configured")
			} else {
				slog.Error("failed to generate upload grant", "error", err)
				response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate upload grant")
			}
		}
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

	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid ticket ID")
		return
	}

	ticket, err := h.feedbackService.GetTicketForUser(c.Request.Context(), ticketID, userID.(int64))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrFeedbackTicketNotFound):
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "Feedback ticket not found")
		case errors.Is(err, service.ErrFeedbackForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "You can only view your own tickets")
		default:
			slog.Error("failed to get feedback ticket", "error", err)
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get feedback ticket")
		}
		return
	}

	c.JSON(http.StatusOK, ticket)
}
