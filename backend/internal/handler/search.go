package handler

import (
	"net/http"
	"strconv"
	"strings"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type SearchHandler struct {
	searchSvc *service.SearchService
	cfg       *config.Config
}

func NewSearchHandler(searchSvc *service.SearchService, cfg *config.Config) *SearchHandler {
	return &SearchHandler{searchSvc: searchSvc, cfg: cfg}
}

func (h *SearchHandler) Suggestions(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusOK, gin.H{"suggestions": []repository.SearchSuggestion{}})
		return
	}
	if rejectLongQuery(c, q, h.maxQueryChars()) {
		return
	}
	limit := clampLimit(c.DefaultQuery("limit", "10"), 10, h.maxSearchLimit())
	suggestions, err := h.searchSvc.GetSuggestions(q, limit, middleware.GetUserID(c))
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
	limit := clampLimit(c.DefaultQuery("limit", "20"), 20, h.maxSearchLimit())
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
	if rejectLongQuery(c, query, h.maxQueryChars()) {
		return
	}
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

	page := clampPage(c.DefaultQuery("page", "1"), h.maxSearchPage())
	pageSize := clampLimit(c.DefaultQuery("limit", "20"), 20, h.maxSearchLimit())

	// Get viewerID from context (0 if anonymous)
	viewerID, _ := c.Get(middleware.UserIDKey)
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
	if rejectLongQuery(c, q, h.maxQueryChars()) {
		return
	}
	page := clampPage(c.DefaultQuery("page", "1"), h.maxSearchPage())
	pageSize := clampLimit(c.DefaultQuery("limit", "20"), 20, h.maxSearchLimit())

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

const defaultMaxQueryChars = 120
const defaultMaxSearchLimit = 50
const defaultMaxSearchPage = 100

func clampLimit(raw string, def, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if max <= 0 {
		max = defaultMaxSearchLimit
	}
	if n > max {
		return max
	}
	return n
}

func rejectLongQuery(c *gin.Context, q string, max int) bool {
	if max <= 0 {
		max = defaultMaxQueryChars
	}
	if len([]rune(q)) > max {
		c.JSON(http.StatusBadRequest, gin.H{"code": "QUERY_TOO_LONG", "message": "search query is too long"})
		return true
	}
	return false
}

func clampPage(raw string, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 1
	}
	if max <= 0 {
		max = defaultMaxSearchPage
	}
	if n > max {
		return max
	}
	return n
}

func (h *SearchHandler) maxQueryChars() int {
	if h == nil || h.cfg == nil {
		return defaultMaxQueryChars
	}
	return h.cfg.RateLimit.MaxQueryChars
}

func (h *SearchHandler) maxSearchLimit() int {
	if h == nil || h.cfg == nil {
		return defaultMaxSearchLimit
	}
	return h.cfg.RateLimit.MaxSearchLimit
}

func (h *SearchHandler) maxSearchPage() int {
	if h == nil || h.cfg == nil {
		return defaultMaxSearchPage
	}
	return h.cfg.RateLimit.MaxSearchPage
}
