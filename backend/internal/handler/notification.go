package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NotificationHandler struct {
	notifRepo *repository.NotificationRepository
}

func NewNotificationHandler(db *gorm.DB) *NotificationHandler {
	return &NotificationHandler{notifRepo: repository.NewNotificationRepository(db)}
}

func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	channel := c.Query("channel")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	notifications, total, err := h.notifRepo.List(callerID, channel, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"notifications": notifications, "total": total})
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid notification id"})
		return
	}
	if err := h.notifRepo.MarkRead(id, callerID); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "marked read"})
}

func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	channel := c.Query("channel")
	if err := h.notifRepo.MarkAllRead(callerID, channel); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "all marked read"})
}

func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	counts, err := h.notifRepo.UnreadCount(callerID)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	var total int64
	for _, c := range counts {
		total += c
	}
	counts["total"] = total
	c.JSON(http.StatusOK, gin.H{"unread_counts": counts})
}
