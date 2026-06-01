package handler

import (
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
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_BODY", "message": "Invalid request body"})
		return
	}

	userID, exists := c.Get(middleware.UserIDKey)
	var uid *int64
	if exists {
		id := userID.(int64)
		uid = &id
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
	}

	ticket, err := h.feedbackService.SubmitTicket(c.Request.Context(), input)
	if err != nil {
		switch err.Error() {
		case "INVALID_CATEGORY":
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_CATEGORY", "message": "Invalid feedback category"})
		case "TITLE_AND_DESCRIPTION_REQUIRED":
			c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "Title and description are required"})
		case "TITLE_TOO_LONG":
			c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "Title must not exceed 160 characters"})
		case "CONTACT_EMAIL_REQUIRED_FOR_ANONYMOUS":
			c.JSON(http.StatusBadRequest, gin.H{"code": "CONTACT_EMAIL_REQUIRED", "message": "Contact email is required for anonymous submissions"})
		case "CAPTCHA_REQUIRED_FOR_ANONYMOUS":
			c.JSON(http.StatusBadRequest, gin.H{"code": "CAPTCHA_REQUIRED", "message": "Captcha verification is required for anonymous submissions"})
		case "CAPTCHA_VERIFICATION_FAILED":
			c.JSON(http.StatusBadRequest, gin.H{"code": "CAPTCHA_FAILED", "message": "Captcha verification failed"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to submit feedback"})
		}
		return
	}

	c.JSON(http.StatusCreated, ticket)
}

func (h *FeedbackHandler) PresignUpload(c *gin.Context) {
	var req struct {
		FileName    string `json:"file_name"`
		MimeType    string `json:"mime_type"`
		SizeBytes   int64  `json:"size_bytes"`
		CaptchaToken string `json:"captcha_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_BODY", "message": "Invalid request body"})
		return
	}

	userID, exists := c.Get(middleware.UserIDKey)
	var uid *int64
	if exists {
		id := userID.(int64)
		uid = &id
	}

	input := service.PresignUploadInput{
		UserID:       uid,
		FileName:     req.FileName,
		MimeType:     req.MimeType,
		SizeBytes:    req.SizeBytes,
		CaptchaToken: req.CaptchaToken,
	}

	grantID, ossKey, err := h.feedbackService.PresignUpload(c.Request.Context(), input)
	if err != nil {
		switch err.Error() {
		case "CAPTCHA_REQUIRED_FOR_ANONYMOUS":
			c.JSON(http.StatusBadRequest, gin.H{"code": "CAPTCHA_REQUIRED", "message": "Captcha verification is required for anonymous uploads"})
		case "CAPTCHA_VERIFICATION_FAILED":
			c.JSON(http.StatusBadRequest, gin.H{"code": "CAPTCHA_FAILED", "message": "Captcha verification failed"})
		case "INVALID_MIME_TYPE":
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_MIME_TYPE", "message": "Only image uploads are supported for feedback screenshots"})
		case "FILE_TOO_LARGE":
			c.JSON(http.StatusBadRequest, gin.H{"code": "FILE_TOO_LARGE", "message": "Screenshot must be smaller than 20MB"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to generate upload grant"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"grant_id": grantID,
		"oss_key":  ossKey,
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
		"items": tickets,
		"total": total,
		"page":  page,
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
		switch err.Error() {
		case "TICKET_NOT_FOUND":
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "Feedback ticket not found"})
		case "FORBIDDEN":
			c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "You can only view your own tickets"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Failed to get feedback ticket"})
		}
		return
	}

	c.JSON(http.StatusOK, ticket)
}
