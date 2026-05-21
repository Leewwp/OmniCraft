package handler

import (
	"net/http"
	"strconv"
	"strings"

	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type SearchHandler struct {
	searchSvc *service.SearchService
}

func NewSearchHandler(searchSvc *service.SearchService) *SearchHandler {
	return &SearchHandler{searchSvc: searchSvc}
}

func (h *SearchHandler) Suggestions(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusOK, gin.H{"suggestions": []repository.SearchSuggestion{}})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	suggestions, err := h.searchSvc.GetSuggestions(q, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "search failed"})
		return
	}
	if suggestions == nil {
		suggestions = []repository.SearchSuggestion{}
	}
	c.JSON(http.StatusOK, gin.H{"suggestions": suggestions})
}

func (h *SearchHandler) Trending(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.searchSvc.GetTrending(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "search failed"})
		return
	}
	if items == nil {
		items = []service.TrendingItem{}
	}
	c.JSON(http.StatusOK, gin.H{"trending": items})
}

func (h *SearchHandler) SearchContents(c *gin.Context) {
	query := c.Query("q")
	zone := c.Query("zone")
	category := c.Query("category")
	contentType := c.Query("content_type")
	sort := c.DefaultQuery("sort", "relevance")
	timeRange := c.Query("time_range")

	if contentType != "" {
		contentTypes := strings.Split(contentType, ",")
		if len(contentTypes) > 1 {
			contentType = contentTypes[0] // use first for now; multi-type support in content repo
		}
	}

	var tagFilters []string
	if tags := c.Query("tags"); tags != "" {
		tagFilters = strings.Split(tags, ",")
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	// Get viewerID from context (0 if anonymous)
	viewerID, _ := c.Get("user_id")
	var vid int64
	if id, ok := viewerID.(int64); ok {
		vid = id
	}

	results, total, err := h.searchSvc.SearchContents(query, zone, category, contentType, tagFilters, sort, page, pageSize, vid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "search failed"})
		return
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       results,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": totalPages,
		"time_range":  timeRange,
	})
}

func (h *SearchHandler) SearchUsers(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusOK, gin.H{"users": []repository.UserSearchResult{}, "total": 0})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	users, total, err := h.searchSvc.SearchUsers(q, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "user search failed"})
		return
	}
	if users == nil {
		users = []repository.UserSearchResult{}
	}
	c.JSON(http.StatusOK, gin.H{"users": users, "total": total})
}