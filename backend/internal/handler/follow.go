package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type FollowHandler struct {
	followRepo    *repository.FollowRepository
	notifSvc      *service.NotificationService
	displaySigner *service.DisplayURLSigner
}

// NewFollowHandler receives the container-owned FollowRepository (#658 PR-3).
func NewFollowHandler(followRepo *repository.FollowRepository) *FollowHandler {
	return &FollowHandler{followRepo: followRepo}
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
	// SP-25 低-24：self-follow 拒绝；目标必须存在且未封禁（此前
	// FirstOrCreate 无条件落行，可关注不存在/封禁用户留下悬挂关系）。
	if callerID == targetID {
		c.JSON(http.StatusBadRequest, gin.H{"code": "SELF_FOLLOW_NOT_ALLOWED", "message": "cannot follow yourself"})
		return
	}
	exists, banned, err := h.followRepo.FollowTargetStatus("user", targetID)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"code": "USER_NOT_FOUND", "message": "target user not found"})
		return
	}
	if banned {
		c.JSON(http.StatusBadRequest, gin.H{"code": "USER_BANNED", "message": "target user is banned"})
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
	// SP-25 低-24：IP 目标存在性校验（防悬挂关注关系）。
	ipExists, _, err := h.followRepo.FollowTargetStatus("ip", ipID)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	if !ipExists {
		c.JSON(http.StatusNotFound, gin.H{"code": "IP_NOT_FOUND", "message": "target ip not found"})
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
	page, pageSize := pageQuery(c, 20)
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
	page, pageSize := pageQuery(c, 20)
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
