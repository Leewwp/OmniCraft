package handler

import (
	"net/http"
	"strconv"

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
