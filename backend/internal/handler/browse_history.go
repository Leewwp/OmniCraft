package handler

import (
	"context"
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BrowseHistoryHandler struct {
	histRepo *repository.BrowseHistoryRepository
}

func NewBrowseHistoryHandler(db *gorm.DB) *BrowseHistoryHandler {
	return &BrowseHistoryHandler{histRepo: repository.NewBrowseHistoryRepository(db)}
}

func (h *BrowseHistoryHandler) RecordView(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	var body struct {
		ContentItemID int64 `json:"content_item_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}
	if err := h.histRepo.Upsert(callerID, body.ContentItemID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	redisclient.ClearRecCache(context.Background(), callerID)
	c.JSON(http.StatusOK, gin.H{"message": "recorded"})
}

func (h *BrowseHistoryHandler) GetHistory(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	items, total, err := h.histRepo.ListByUser(callerID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"history": items, "total": total})
}

func (h *BrowseHistoryHandler) ClearHistory(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if err := h.histRepo.DeleteByUser(callerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	redisclient.ClearRecCache(context.Background(), callerID)
	c.JSON(http.StatusOK, gin.H{"message": "cleared"})
}
