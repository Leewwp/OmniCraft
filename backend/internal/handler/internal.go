package handler

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/service"
)

type InternalHandler struct {
	reviewSvc *service.ReviewService
	cfg       *config.Config
}

func NewInternalHandler(db *gorm.DB, rdb *redis.Client, cfg *config.Config) *InternalHandler {
	reputSvc := service.NewReputationService(db)
	reviewSvc := service.NewReviewService(db, rdb, cfg, reputSvc)
	return &InternalHandler{reviewSvc: reviewSvc, cfg: cfg}
}

func (h *InternalHandler) AICallback(c *gin.Context) {
	if !h.isAllowedSourceIP(c.ClientIP()) {
		c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "callback source ip is not allowed"})
		return
	}

	var input service.AICallbackInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}

	if err := h.reviewSvc.ProcessAICallback(c.Request.Context(), input); err != nil {
		if err == service.ErrReviewTargetNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func (h *InternalHandler) isAllowedSourceIP(rawIP string) bool {
	clientIP := net.ParseIP(strings.TrimSpace(rawIP))
	if clientIP == nil {
		return false
	}

	allowed := []string{}
	if h.cfg != nil {
		allowed = h.cfg.Green.CallbackAllowedIPs
	}
	if len(allowed) == 0 {
		allowed = []string{"127.0.0.1", "::1"}
	}

	for _, candidate := range allowed {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		if ip := net.ParseIP(trimmed); ip != nil && ip.Equal(clientIP) {
			return true
		}
		if _, network, err := net.ParseCIDR(trimmed); err == nil && network.Contains(clientIP) {
			return true
		}
	}

	return false
}
