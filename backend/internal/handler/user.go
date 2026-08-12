package handler

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userRepo    *repository.UserRepository
	reputSvc    *service.ReputationService
	contentRepo *repository.ContentRepository
	followRepo  *repository.FollowRepository
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
		followRepo:  repository.NewFollowRepository(db),
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
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	resp := sanitizeUser(user)
	stats, followersCount := h.getUserStatsAndFollowers(id)
	resp["stats"] = stats
	resp["followers_count"] = followersCount
	resp["is_following"] = h.checkIsFollowing(c, id)

	c.JSON(http.StatusOK, gin.H{"user": resp})
}

func (h *UserHandler) getUserStatsAndFollowers(userID int64) (gin.H, int64) {
	var contentsCount int64
	var likesReceived int64
	var followersCount int64

	h.contentRepo.DB().Raw(
		`SELECT
			(SELECT COUNT(*) FROM content_items WHERE author_id = ? AND status = 'published') AS contents_count,
			COALESCE((SELECT SUM(like_count) FROM content_items WHERE author_id = ? AND status = 'published'), 0) AS likes_received,
			(SELECT COUNT(*) FROM follows WHERE target_type = 'user' AND target_id = ?) AS followers_count`,
		userID, userID, userID,
	).Scan(&struct {
		Count     *int64
		Likes     *int64
		Followers *int64
	}{Count: &contentsCount, Likes: &likesReceived, Followers: &followersCount})

	return gin.H{
		"contents_count": contentsCount,
		"likes_received": likesReceived,
	}, followersCount
}

func (h *UserHandler) checkIsFollowing(c *gin.Context, targetID int64) bool {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		return false
	}
	following, err := h.followRepo.IsFollowing(callerID, "user", targetID)
	if err != nil {
		return false
	}
	return following
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
		Username            *string `json:"username"`
		AvatarURL           *string `json:"avatar_url"`
		Bio                 *string `json:"bio"`
		PreferredLocale     *string `json:"preferred_locale"`
		AcceptCollabInvites *bool   `json:"accept_collab_invites"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.SafeErrorResponse(c, http.StatusBadRequest, "INVALID_BODY", err)
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
	if req.AcceptCollabInvites != nil {
		updates["accept_collab_invites"] = *req.AcceptCollabInvites
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "NO_FIELDS", "message": "no fields to update"})
		return
	}

	if err := h.userRepo.UpdateFields(id, updates); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
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
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
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
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"contents": items, "total": total})
}

func (h *UserHandler) GetMyContents(c *gin.Context) {
	callerID := middleware.GetUserID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	contentType := c.Query("content_type")

	items, total, err := h.contentRepo.ListContents(repository.ListContentsFilter{
		AuthorID:    &callerID,
		ContentType: contentType,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
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
		response.ValidationError(c, "invalid request parameters")
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
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
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
		response.ValidationError(c, "invalid request parameters")
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
	randomHash, _ := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(make([]byte, 32))), bcrypt.DefaultCost)
	updates := map[string]interface{}{
		"username":      anonName,
		"email":         anonEmail,
		"avatar_url":    "",
		"bio":           "",
		"is_banned":     true,
		"ban_reason":    "self_deleted",
		"password_hash": string(randomHash),
	}

	if err := h.userRepo.UpdateFields(callerID, updates); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	invalidateUserTokens(h.rdb, callerID)
	middleware.InvalidateUserStatusCache(h.rdb, callerID)

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
		response.ValidationError(c, "invalid request parameters")
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
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "support info updated", "support_info": info})
}

func invalidateUserTokens(rdb *redis.Client, userID int64) {
	if rdb == nil {
		return
	}
	ctx := context.Background()
	tokenSetKey := fmt.Sprintf("user:tokens:%d", userID)
	members, err := rdb.SMembers(ctx, tokenSetKey).Result()
	if err != nil {
		return
	}
	if len(members) > 0 {
		rdb.Del(ctx, members...)
	}
	rdb.Del(ctx, tokenSetKey)
}

func sanitizeUser(u *model.User) gin.H {
	return gin.H{
		"id":                    u.ID,
		"username":              u.Username,
		"email":                 u.Email,
		"avatar_url":            u.AvatarURL,
		"bio":                   u.Bio,
		"reputation":            u.Reputation,
		"preferred_locale":      u.PreferredLocale,
		"role":                  u.Role,
		"is_banned":             u.IsBanned,
		"accept_collab_invites": u.AcceptCollabInvites,
		"created_at":            u.CreatedAt,
	}
}
