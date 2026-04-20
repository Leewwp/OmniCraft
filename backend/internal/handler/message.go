package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MessageHandler struct {
	msgRepo *repository.MessageRepository
}

func NewMessageHandler(db *gorm.DB) *MessageHandler {
	return &MessageHandler{msgRepo: repository.NewMessageRepository(db)}
}

func (h *MessageHandler) ListConversations(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	convs, err := h.msgRepo.ListConversations(callerID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}

	convID, err := h.msgRepo.FindOrCreateConversation(callerID, body.RecipientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	msg, err := h.msgRepo.Send(callerID, convID, body.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	h.msgRepo.UpdateLastRead(callerID, convID)
	c.JSON(http.StatusOK, gin.H{"messages": messages, "total": total})
}
