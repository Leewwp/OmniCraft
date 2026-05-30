package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/service"
)

type InternalHandler struct {
	reviewSvc     *service.ReviewService
	cfg           *config.Config
	queueProducer queue.Producer
}

func NewInternalHandler(db *gorm.DB, rdb *redis.Client, cfg *config.Config) *InternalHandler {
	reputSvc := service.NewReputationService(db)
	reviewSvc := service.NewReviewService(db, rdb, cfg, reputSvc)
	return &InternalHandler{reviewSvc: reviewSvc, cfg: cfg, queueProducer: queue.NewNoopProducer()}
}

func (h *InternalHandler) SetQueueProducer(p queue.Producer) {
	h.queueProducer = p
}

func (h *InternalHandler) AICallback(c *gin.Context) {
	if !h.isAllowedSourceIP(c.ClientIP()) {
		c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "callback source ip is not allowed"})
		return
	}

	var input service.AICallbackInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	if _, ok := h.queueProducer.(*queue.NoopProducer); !ok && h.queueProducer != nil {
		recovery.GoSafe(func() {
			payload, _ := json.Marshal(map[string]interface{}{
				"action":       "process_ai_callback",
				"target_type":  input.TargetType,
				"target_id":    input.TargetID,
				"result":       input.Result,
				"raw_response": input.RawResponse,
			})
			topic := "content.review"
			if strings.EqualFold(input.TargetType, "ip") {
				topic = "ip.review"
			}
			if err := h.queueProducer.Publish(context.Background(), topic, payload); err != nil {
				slog.Error("failed to publish ai callback to queue", "topic", topic, "error", err)
			}
		})
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
		return
	}

	if err := h.reviewSvc.ProcessAICallback(c.Request.Context(), input); err != nil {
		if err == service.ErrReviewTargetNotFound {
			response.NotFound(c, "resource not found")
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
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
