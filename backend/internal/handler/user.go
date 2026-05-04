package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userRepo    *repository.UserRepository
	reputSvc    *service.ReputationService
	contentRepo *repository.ContentRepository
	authSvc     *service.AuthService
	rdb         *redis.Client
	cfg         *config.Config
	jwtSecret   string
}

func NewUserHandler(db *gorm.DB, authSvc *service.AuthService, rdb *redis.Client, cfg *config.Config) *UserHandler {
	return &UserHandler{
		userRepo:    repository.NewUserRepository(db),
		reputSvc:    service.NewReputationService(db),
		contentRepo: repository.NewContentRepository(db),
		authSvc:     authSvc,
		rdb:         rdb,
		cfg:         cfg,
		jwtSecret:   cfg.JWT.Secret,
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

func (h *UserHandler) ChangePassword(c *gin.Context) {
	callerID := middleware.GetUserID(c)

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6,max=128"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}

	user, err := h.userRepo.FindByID(callerID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "USER_NOT_FOUND", "message": "user not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": "INVALID_PASSWORD", "message": "old password is incorrect"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to hash password"})
		return
	}

	if err := h.userRepo.UpdateFields(callerID, map[string]interface{}{"password_hash": string(hash)}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	invalidateUserTokens(h.rdb, callerID)

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}

func (h *UserHandler) DeleteAccount(c *gin.Context) {
	callerID := middleware.GetUserID(c)

	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}

	user, err := h.userRepo.FindByID(callerID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "USER_NOT_FOUND", "message": "user not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": "INVALID_PASSWORD", "message": "password is incorrect"})
		return
	}

	anonName := fmt.Sprintf("已注销用户_%d", callerID)
	anonEmail := fmt.Sprintf("deleted_%d@anon.local", callerID)
	updates := map[string]interface{}{
		"username":      anonName,
		"email":         anonEmail,
		"avatar_url":    "",
		"bio":           "",
		"is_banned":     true,
		"ban_reason":    "self_deleted",
		"password_hash": "",
	}

	if err := h.userRepo.UpdateFields(callerID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	invalidateUserTokens(h.rdb, callerID)

	c.JSON(http.StatusOK, gin.H{"message": "account deleted successfully"})
}

func (h *UserHandler) UpdateSupportInfo(c *gin.Context) {
	if !h.cfg.Features.CreatorSupportEnabled {
		c.JSON(http.StatusForbidden, gin.H{"code": "FEATURE_DISABLED", "message": "creator support is not enabled"})
		return
	}

	callerID := middleware.GetUserID(c)

	var req struct {
		DonationImageURL string   `json:"donation_image_url"`
		ExternalLinks    []string `json:"external_links"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}

	if len(req.ExternalLinks) > 3 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": "VALIDATION_ERROR", "message": "external_links maximum is 3"})
		return
	}

	info := model.JSONMap{
		"donation_image_url": req.DonationImageURL,
		"external_links":     req.ExternalLinks,
	}

	if err := h.userRepo.UpdateFields(callerID, map[string]interface{}{"support_info": info}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "support info updated", "support_info": info})
}

func invalidateUserTokens(rdb *redis.Client, userID int64) {
	if rdb == nil {
		return
	}
	ctx := context.Background()
	pattern := fmt.Sprintf("refresh_token:%d:*", userID)
	keys, err := rdb.Keys(ctx, pattern).Result()
	if err != nil {
		return
	}
	if len(keys) > 0 {
		rdb.Del(ctx, keys...)
	}
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
