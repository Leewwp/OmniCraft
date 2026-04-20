package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type FollowHandler struct {
	followRepo *repository.FollowRepository
}

func NewFollowHandler(db *gorm.DB) *FollowHandler {
	return &FollowHandler{followRepo: repository.NewFollowRepository(db)}
}

func (h *FollowHandler) FollowUser(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	targetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID"})
		return
	}
	if err := h.followRepo.Follow(callerID, "user", targetID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users, "total": total})
}

func (h *FollowHandler) GetFollowing(c *gin.Context) {
	targetID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	follows, total, err := h.followRepo.GetFollowing(targetID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"following": follows, "total": total})
}
