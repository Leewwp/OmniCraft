package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/redis/go-redis/v9"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SocialHandler struct {
	socialSvc *service.SocialService
}

func NewSocialHandler(db *gorm.DB, cfg *config.Config, rdb *redis.Client) *SocialHandler {
	return &SocialHandler{
		socialSvc: service.NewSocialServiceWithRedis(
			repository.NewSocialRepository(db),
			repository.NewContentRepository(db),
			repository.NewUserRepository(db),
			cfg,
			rdb,
		),
	}
}

func (h *SocialHandler) PostComment(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	var input service.PostCommentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}
	comment, err := h.socialSvc.PostComment(input, callerID)
	if err != nil {
		if err == service.ErrLowReputation {
			c.JSON(http.StatusForbidden, gin.H{"code": "LOW_REPUTATION", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"comment": comment})
}

func (h *SocialHandler) DeleteComment(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid comment id"})
		return
	}
	if err := h.socialSvc.DeleteComment(id, callerID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *SocialHandler) ListComments(c *gin.Context) {
	contentIDStr := c.Query("content_item_id")
	if contentIDStr == "" {
		contentIDStr = c.Query("content_id")
	}
	contentID, err := strconv.ParseInt(contentIDStr, 10, 64)
	if err != nil || contentID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "content_item_id required"})
		return
	}
	var parentID *int64
	if p := c.Query("parent_id"); p != "" {
		if v, err := strconv.ParseInt(p, 10, 64); err == nil {
			parentID = &v
		}
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	comments, total, err := h.socialSvc.ListComments(contentID, parentID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"comments": comments, "total": total, "page": page, "page_size": pageSize})
}

func (h *SocialHandler) ListDiscussions(c *gin.Context) {
	var ipID, contentID *int64
	if v, err := strconv.ParseInt(c.Query("ip_id"), 10, 64); err == nil {
		ipID = &v
	}
	if v, err := strconv.ParseInt(c.Query("content_id"), 10, 64); err == nil {
		contentID = &v
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	discussions, total, err := h.socialSvc.ListDiscussions(ipID, contentID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"discussions": discussions, "total": total, "page": page, "page_size": pageSize})
}

func (h *SocialHandler) PostDiscussion(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	var input service.PostDiscussionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}
	d, err := h.socialSvc.PostDiscussion(input, callerID)
	if err != nil {
		if err == service.ErrLowReputation {
			c.JSON(http.StatusForbidden, gin.H{"code": "LOW_REPUTATION", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"discussion": d})
}

func (h *SocialHandler) GetDiscussion(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid discussion id"})
		return
	}
	d, err := h.socialSvc.GetDiscussion(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "discussion not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"discussion": d})
}

func (h *SocialHandler) React(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	var input service.ReactInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}
	action, err := h.socialSvc.React(input, callerID)
	if err != nil {
		if err == service.ErrLowReputation {
			c.JSON(http.StatusForbidden, gin.H{"code": "LOW_REPUTATION", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"action": action})
}

func (h *SocialHandler) ReportContent(c *gin.Context) {
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
		Reason string `json:"reason" binding:"required"`
		Detail string `json:"detail"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}
	if err := h.socialSvc.Report("content", contentID, callerID, body.Reason, body.Detail); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "reported"})
}

func (h *SocialHandler) ReportComment(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	commentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid comment id"})
		return
	}
	var body struct {
		Reason string `json:"reason" binding:"required"`
		Detail string `json:"detail"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}
	if err := h.socialSvc.Report("comment", commentID, callerID, body.Reason, body.Detail); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "reported"})
}

type FavoriteHandler struct {
	socialSvc *service.SocialService
}

func NewFavoriteHandler(db *gorm.DB, cfg *config.Config) *FavoriteHandler {
	return &FavoriteHandler{
		socialSvc: service.NewSocialService(
			repository.NewSocialRepository(db),
			repository.NewContentRepository(db),
			repository.NewUserRepository(db),
			cfg,
		),
	}
}

func (h *FavoriteHandler) AddFavorite(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	var body struct {
		ContentItemID int64 `json:"content_item_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}
	if err := h.socialSvc.Favorite(callerID, body.ContentItemID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "added to favorites"})
}

func (h *FavoriteHandler) RemoveFavorite(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	contentID, err := strconv.ParseInt(c.Param("contentId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}
	if err := h.socialSvc.Unfavorite(callerID, contentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "removed from favorites"})
}

func (h *FavoriteHandler) ListUserFavorites(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	favs, total, err := h.socialSvc.ListFavorites(userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"favorites": favs, "total": total, "page": page, "page_size": pageSize})
}
