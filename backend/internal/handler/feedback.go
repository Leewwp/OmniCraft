package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"omnicraft/backend/internal/middleware"
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
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_BODY", "message": "Invalid request body"})
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
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_CATEGORY", "message": "Invalid feedback category"})
		case errors.Is(err, service.ErrFeedbackTitleAndDescriptionReq):
			c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "Title and description are required"})
		case errors.Is(err, service.ErrFeedbackTitleTooLong):
			c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "Title must not exceed 160 characters"})
		case errors.Is(err, service.ErrFeedbackContactEmailRequired):
			c.JSON(http.StatusBadRequest, gin.H{"code": "CONTACT_EMAIL_REQUIRED", "message": "Contact email is required for anonymous submissions"})
		case errors.Is(err, service.ErrFeedbackCaptchaRequired):
			c.JSON(http.StatusBadRequest, gin.H{"code": "CAPTCHA_REQUIRED", "message": "Captcha verification is required for anonymous submissions"})
		case errors.Is(err, service.ErrFeedbackCaptchaFailed):
			c.JSON(http.StatusBadRequest, gin.H{"code": "CAPTCHA_FAILED", "message": "Captcha verification failed"})
		case errors.Is(err, service.ErrFeedbackUploadGrantInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"code": "UPLOAD_GRANT_INVALID", "message": "Screenshot upload grant is invalid or has already been used"})
		case errors.Is(err, service.ErrUploadGrantUnavailable):
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": "UPLOAD_GRANT_UNAVAILABLE", "message": "Screenshot upload grants are temporarily unavailable"})
		default:
			slog.Error("failed to submit feedback", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to submit feedback"})
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
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_BODY", "message": "Invalid request body"})
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
			c.JSON(http.StatusBadRequest, gin.H{"code": "CAPTCHA_REQUIRED", "message": "Captcha verification is required for anonymous uploads"})
		case errors.Is(err, service.ErrFeedbackCaptchaFailed):
			c.JSON(http.StatusBadRequest, gin.H{"code": "CAPTCHA_FAILED", "message": "Captcha verification failed"})
		case errors.Is(err, service.ErrFeedbackInvalidMimeType):
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_MIME_TYPE", "message": "Only image uploads are supported for feedback screenshots"})
		case errors.Is(err, service.ErrFeedbackFileTooLarge):
			c.JSON(http.StatusBadRequest, gin.H{"code": "FILE_TOO_LARGE", "message": "Screenshot must be smaller than 20MB"})
		case errors.Is(err, service.ErrUploadGrantUnavailable):
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": "UPLOAD_GRANT_UNAVAILABLE", "message": "Screenshot upload grants are temporarily unavailable"})
		default:
			if errors.Is(err, service.ErrOSSNotConfigured) {
				c.JSON(http.StatusServiceUnavailable, gin.H{"code": "OSS_NOT_CONFIGURED", "message": "OSS upload is not configured"})
			} else {
				slog.Error("failed to generate upload grant", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to generate upload grant"})
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
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "Authentication required"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	tickets, total, err := h.feedbackService.ListUserTickets(c.Request.Context(), userID.(int64), page, pageSize)
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

func (h *FeedbackHandler) GetTicket(c *gin.Context) {
	userID, exists := c.Get(middleware.UserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "Authentication required"})
		return
	}

	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "Invalid ticket ID"})
		return
	}

	ticket, err := h.feedbackService.GetTicketForUser(c.Request.Context(), ticketID, userID.(int64))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrFeedbackTicketNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "Feedback ticket not found"})
		case errors.Is(err, service.ErrFeedbackForbidden):
			c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "You can only view your own tickets"})
		default:
			slog.Error("failed to get feedback ticket", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to get feedback ticket"})
		}
		return
	}

	c.JSON(http.StatusOK, ticket)
}
