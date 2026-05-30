package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TagHandler struct {
	tagSvc *service.TagService
}

func NewTagHandler(db *gorm.DB, rdb *redis.Client, cacheCfg *config.CacheConfig) *TagHandler {
	return &TagHandler{
		tagSvc: service.NewTagService(
			repository.NewTagRepository(db),
			repository.NewContentRepository(db),
			rdb,
			cacheCfg,
		),
	}
}

func (h *TagHandler) GetFacetedTags(c *gin.Context) {
	category := c.Query("category")
	var selectedTags []string
	if s := c.QueryArray("selected_tags[]"); len(s) > 0 {
		selectedTags = s
	} else if s := c.Query("selected_tags"); s != "" {
		selectedTags = strings.Split(s, ",")
	}

	tags, err := h.tagSvc.GetFacetedTags(category, selectedTags)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

func (h *TagHandler) SearchTags(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "MISSING_QUERY", "message": "q is required"})
		return
	}
	tags, err := h.tagSvc.SearchTags(q)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

func (h *TagHandler) SuggestTag(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	contentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}
	var body struct {
		Tag    string `json:"tag" binding:"required"`
		Action string `json:"action" binding:"required,oneof=add remove"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	if err := h.tagSvc.SuggestTag(contentID, callerID, body.Tag, body.Action); err != nil {
		if errors.Is(err, service.ErrTagSuggestRateLimited) {
			response.SafeErrorResponse(c, http.StatusTooManyRequests, "TAG_SUGGEST_RATE_LIMIT_EXCEEDED", err)
			return
		}
		response.SafeErrorResponse(c, http.StatusBadRequest, "ERROR", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "suggestion submitted"})
}

func (h *TagHandler) ListTagSuggestions(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	contentID, err := strconv.ParseInt(c.Query("content_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "content_id required"})
		return
	}
	suggestions, err := h.tagSvc.ListTagSuggestions(contentID, callerID)
	if err != nil {
		response.Forbidden(c, "access denied")
		return
	}
	c.JSON(http.StatusOK, gin.H{"suggestions": suggestions})
}

func (h *TagHandler) UpdateTagSuggestion(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid suggestion id"})
		return
	}
	var body struct {
		Status string `json:"status" binding:"required,oneof=approved rejected"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	var svcErr error
	if body.Status == "approved" {
		svcErr = h.tagSvc.ApproveTagSuggestion(id, callerID)
	} else {
		svcErr = h.tagSvc.RejectTagSuggestion(id, callerID)
	}
	if svcErr != nil {
		response.SafeErrorResponse(c, http.StatusBadRequest, "ERROR", svcErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *TagHandler) ListTagGroups(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	groups, err := h.tagSvc.ListTagGroups(callerID)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tag_groups": groups})
}

func (h *TagHandler) CreateTagGroup(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	var body struct {
		Name string   `json:"name" binding:"required"`
		Tags []string `json:"tags" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	g, err := h.tagSvc.CreateTagGroup(callerID, body.Name, body.Tags)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"tag_group": g})
}

func (h *TagHandler) UpdateTagGroup(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid group id"})
		return
	}
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	if err := h.tagSvc.UpdateTagGroup(id, callerID, updates); err != nil {
		response.SafeErrorResponse(c, http.StatusBadRequest, "ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *TagHandler) DeleteTagGroup(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid group id"})
		return
	}
	if err := h.tagSvc.DeleteTagGroup(id, callerID); err != nil {
		response.SafeErrorResponse(c, http.StatusBadRequest, "ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *TagHandler) ListSavedSearches(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	searches, err := h.tagSvc.ListSavedSearches(callerID)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"saved_searches": searches})
}

func (h *TagHandler) CreateSavedSearch(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	var body struct {
		Name   string        `json:"name" binding:"required"`
		Config model.JSONMap `json:"config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	ss, err := h.tagSvc.CreateSavedSearch(callerID, body.Name, body.Config)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"saved_search": ss})
}

func (h *TagHandler) DeleteSavedSearch(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid saved search id"})
		return
	}
	if err := h.tagSvc.DeleteSavedSearch(id, callerID); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
