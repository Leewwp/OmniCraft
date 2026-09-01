package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type FollowHandler struct {
	followRepo    *repository.FollowRepository
	notifSvc      *service.NotificationService
	displaySigner *service.DisplayURLSigner
}

func NewFollowHandler(db *gorm.DB) *FollowHandler {
	return &FollowHandler{followRepo: repository.NewFollowRepository(db)}
}

func (h *FollowHandler) SetNotificationService(ns *service.NotificationService) {
	h.notifSvc = ns
}

// SetDisplayURLSigner wires display URL signing for follower avatars (B-002).
func (h *FollowHandler) SetDisplayURLSigner(signer *service.DisplayURLSigner) {
	h.displaySigner = signer
}

func (h *FollowHandler) FollowUser(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	targetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID"})
		return
	}
	if err := h.followRepo.Follow(callerID, "user", targetID); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	if h.notifSvc != nil && callerID != targetID {
		h.notifSvc.Notify(targetID, "follow", "follow", "你有新粉丝", "", "user", callerID, callerID)
	}
	c.JSON(http.StatusOK, gin.H{"message": "followed"})
}

func (h *FollowHandler) UnfollowUser(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	targetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID"})
		return
	}
	if err := h.followRepo.Unfollow(callerID, "user", targetID); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "unfollowed"})
}

func (h *FollowHandler) FollowIP(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	ipID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID"})
		return
	}
	if err := h.followRepo.Follow(callerID, "ip", ipID); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	// 不发 user_id=0 幽灵广播（FIX-31 首项）：向该 IP 真实粉丝的 fan-out 属 T55 范围。
	c.JSON(http.StatusOK, gin.H{"message": "followed"})
}

func (h *FollowHandler) UnfollowIP(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	ipID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID"})
		return
	}
	if err := h.followRepo.Unfollow(callerID, "ip", ipID); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "unfollowed"})
}

func (h *FollowHandler) GetFollowers(c *gin.Context) {
	targetID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	users, total, err := h.followRepo.GetFollowers("user", targetID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "database error"})
		return
	}
	h.displaySigner.DecorateUsers(users)
	c.JSON(http.StatusOK, gin.H{"users": users, "total": total})
}

func (h *FollowHandler) GetFollowing(c *gin.Context) {
	targetID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	follows, total, err := h.followRepo.GetFollowing(targetID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"following": follows, "total": total})
}

func (h *FollowHandler) GetFollowerStats(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 || days > 365 {
		days = 30
	}
	stats, err := h.followRepo.GetFollowerStats(userID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "database error"})
		return
	}
	c.JSON(http.StatusOK, stats)
}
