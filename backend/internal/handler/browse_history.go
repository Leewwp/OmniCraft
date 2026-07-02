package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BrowseHistoryHandler struct {
	histRepo *repository.BrowseHistoryRepository
	cfg      *config.Config
}

func NewBrowseHistoryHandler(db *gorm.DB, cfg *config.Config) *BrowseHistoryHandler {
	return &BrowseHistoryHandler{histRepo: repository.NewBrowseHistoryRepository(db), cfg: cfg}
}

func (h *BrowseHistoryHandler) RecordView(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	var body struct {
		ContentItemID int64 `json:"content_item_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	if err := h.histRepo.Upsert(callerID, body.ContentItemID); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	redisclient.ClearRecCache(context.Background(), callerID)
	c.JSON(http.StatusOK, gin.H{"message": "recorded"})
}

func (h *BrowseHistoryHandler) GetHistory(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	opts, ok := h.parseListOptions(c, callerID)
	if !ok {
		return
	}

	items, total, err := h.histRepo.ListByUserFiltered(opts)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":          items,
		"history":        items,
		"total":          total,
		"page":           opts.Page,
		"page_size":      opts.PageSize,
		"retention_days": h.retentionDays(),
	})
}

func (h *BrowseHistoryHandler) ClearHistory(c *gin.Context) {
	callerID := middleware.GetUserID(c)

	var body struct {
		IDs []int64 `json:"ids"`
	}
	err := c.ShouldBindJSON(&body)
	if err != nil && !errors.Is(err, io.EOF) {
		response.ValidationError(c, "invalid request body")
		return
	}
	if len(body.IDs) > 100 {
		response.Error(c, http.StatusBadRequest, "TOO_MANY_IDS", "too many browse history ids")
		return
	}

	if len(body.IDs) == 0 {
		err = h.histRepo.DeleteByUser(callerID)
	} else {
		err = h.histRepo.DeleteByUserAndIDs(callerID, body.IDs)
	}
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	redisclient.ClearRecCache(context.Background(), callerID)
	c.JSON(http.StatusOK, gin.H{"message": "cleared"})
}

func (h *BrowseHistoryHandler) parseListOptions(c *gin.Context, userID int64) (repository.BrowseHistoryListOptions, bool) {
	contentType := c.Query("content_type")
	if contentType != "" && !validBrowseHistoryContentType(contentType) {
		response.Error(c, http.StatusBadRequest, "INVALID_CONTENT_TYPE", "invalid content type")
		return repository.BrowseHistoryListOptions{}, false
	}

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
		return repository.BrowseHistoryListOptions{}, false
	}
	startDate, ok := parseBrowseHistoryDate(c, "start_date", loc)
	if !ok {
		return repository.BrowseHistoryListOptions{}, false
	}
	endDate, ok := parseBrowseHistoryDate(c, "end_date", loc)
	if !ok {
		return repository.BrowseHistoryListOptions{}, false
	}
	if startDate != nil && endDate != nil && startDate.After(*endDate) {
		response.Error(c, http.StatusBadRequest, "INVALID_DATE", "invalid date range")
		return repository.BrowseHistoryListOptions{}, false
	}
	if endDate != nil {
		exclusive := endDate.AddDate(0, 0, 1)
		endDate = &exclusive
	}

	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("page_size"), 0)
	if pageSize == 0 {
		pageSize = parsePositiveInt(c.Query("limit"), 20)
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return repository.BrowseHistoryListOptions{
		UserID:        userID,
		ContentType:   contentType,
		StartDate:     startDate,
		EndDate:       endDate,
		RetentionDays: h.retentionDays(),
		Now:           time.Now(),
		Page:          page,
		PageSize:      pageSize,
	}, true
}

func (h *BrowseHistoryHandler) retentionDays() int {
	if h.cfg != nil && h.cfg.BrowseHistory.RetentionDays > 0 {
		return h.cfg.BrowseHistory.RetentionDays
	}
	return 7
}

func parseBrowseHistoryDate(c *gin.Context, key string, loc *time.Location) (*time.Time, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, true
	}
	parsed, err := time.ParseInLocation("2006-01-02", raw, loc)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_DATE", "invalid date")
		return nil, false
	}
	return &parsed, true
}

func parsePositiveInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func validBrowseHistoryContentType(contentType string) bool {
	switch contentType {
	case "image", "article", "video", "audio", "template", "sheet_music", "mod", "prompt", "other":
		return true
	default:
		return false
	}
}
