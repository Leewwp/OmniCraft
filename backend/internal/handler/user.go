package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserHandler struct {
	userRepo    *repository.UserRepository
	reputSvc    *service.ReputationService
	contentRepo *repository.ContentRepository
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{
		userRepo:    repository.NewUserRepository(db),
		reputSvc:    service.NewReputationService(db),
		contentRepo: repository.NewContentRepository(db),
	}
}

func (h *UserHandler) GetUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}

	user, err := h.userRepo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "USER_NOT_FOUND", "message": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": sanitizeUser(user)})
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}

	callerID := middleware.GetUserID(c)
	if callerID != id {
		c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "can only update your own profile"})
		return
	}

	var req struct {
		Username        *string `json:"username"`
		AvatarURL       *string `json:"avatar_url"`
		Bio             *string `json:"bio"`
		PreferredLocale *string `json:"preferred_locale"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_BODY", "message": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Username != nil {
		updates["username"] = *req.Username
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = *req.AvatarURL
	}
	if req.Bio != nil {
		updates["bio"] = *req.Bio
	}
	if req.PreferredLocale != nil {
		updates["preferred_locale"] = *req.PreferredLocale
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "NO_FIELDS", "message": "no fields to update"})
		return
	}

	if err := h.userRepo.UpdateFields(id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	user, _ := h.userRepo.FindByID(id)
	c.JSON(http.StatusOK, gin.H{"user": sanitizeUser(user)})
}

func (h *UserHandler) GetReputation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	logs, total, err := h.reputSvc.GetLogs(id, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *UserHandler) GetUserContents(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	contentType := c.Query("content_type")

	items, total, err := h.contentRepo.ListContents(repository.ListContentsFilter{
		AuthorID:    &id,
		ContentType: contentType,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contents": items, "total": total})
}

func sanitizeUser(u *model.User) gin.H {
	return gin.H{
		"id":               u.ID,
		"username":         u.Username,
		"email":            u.Email,
		"avatar_url":       u.AvatarURL,
		"bio":              u.Bio,
		"reputation":       u.Reputation,
		"preferred_locale": u.PreferredLocale,
		"role":             u.Role,
		"is_banned":        u.IsBanned,
		"created_at":       u.CreatedAt,
	}
}
