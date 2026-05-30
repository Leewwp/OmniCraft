package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MessageHandler struct {
	msgRepo  *repository.MessageRepository
	notifSvc *service.NotificationService
}

func NewMessageHandler(db *gorm.DB) *MessageHandler {
	return &MessageHandler{msgRepo: repository.NewMessageRepository(db)}
}

func (h *MessageHandler) SetNotificationService(ns *service.NotificationService) {
	h.notifSvc = ns
}

func (h *MessageHandler) ListConversations(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	convs, err := h.msgRepo.ListConversations(callerID, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"conversations": convs})
}

func (h *MessageHandler) SendMessage(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	var body struct {
		RecipientID int64  `json:"recipient_id" binding:"required"`
		Text        string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	convID, err := h.msgRepo.FindOrCreateConversation(callerID, body.RecipientID)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	msg, err := h.msgRepo.Send(callerID, convID, body.Text)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	if h.notifSvc != nil {
		h.notifSvc.Notify(body.RecipientID, "system", "message", "新私信", body.Text, "message", msg.ID, callerID)
	}

	c.JSON(http.StatusCreated, gin.H{"message": msg})
}

func (h *MessageHandler) ListMessages(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID"})
		return
	}

	ok, _ := h.msgRepo.IsParticipant(callerID, convID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	messages, total, err := h.msgRepo.ListMessages(convID, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	h.msgRepo.UpdateLastRead(callerID, convID)
	c.JSON(http.StatusOK, gin.H{"messages": messages, "total": total})
}

func (h *MessageHandler) DeleteMessage(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	msgID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID"})
		return
	}
	if err := h.msgRepo.DeleteMessage(msgID, callerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *MessageHandler) LeaveConversation(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID"})
		return
	}
	if err := h.msgRepo.LeaveConversation(convID, callerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "left conversation"})
}
