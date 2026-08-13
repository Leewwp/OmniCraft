package handler

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
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

// aliyunCallbackContent is the JSON payload of the Aliyun content-safety
// scan-result callback. It travels inside the form field "content" of an
// application/x-www-form-urlencoded POST; the exact bytes of this JSON are
// what the checksum signs.
type aliyunCallbackContent struct {
	DataID  string `json:"dataId"`
	TaskID  string `json:"taskId"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Results []struct {
		Scene      string `json:"scene"`
		Label      string `json:"label"`
		Suggestion string `json:"suggestion"`
	} `json:"results"`
}

// AICallback is the inbound Aliyun scan-result callback endpoint. It parses
// application/x-www-form-urlencoded (checksum + content), verifies the
// checksum SHA256(uid + seed + content) with the main-account UID and the
// per-deployment seed, then reuses the existing review processing flow (sync
// or via the content.review / ip.review queue topics). The checksum is the
// only inbound authentication: the legacy source-IP allowlist was retired in
// #104/#106 because Aliyun publishes no callback source ranges.
func (h *InternalHandler) AICallback(c *gin.Context) {
	content := c.PostForm("content")
	checksum := strings.TrimSpace(c.PostForm("checksum"))
	if content == "" || checksum == "" {
		response.Forbidden(c, "invalid checksum")
		return
	}

	seed := strings.TrimSpace(h.cfg.Green.Seed)
	uid := strings.TrimSpace(h.cfg.Green.UID)
	if seed == "" || uid == "" {
		// Fail closed: without the configured contract inputs no callback
		// can be authenticated.
		response.Forbidden(c, "invalid checksum")
		return
	}
	expected := sha256Hex(uid + seed + content)
	if subtle.ConstantTimeCompare([]byte(checksum), []byte(expected)) != 1 {
		response.Forbidden(c, "invalid checksum")
		return
	}

	var callback aliyunCallbackContent
	if err := json.Unmarshal([]byte(content), &callback); err != nil {
		response.ValidationError(c, "invalid callback content")
		return
	}

	targetType, targetID, err := parseCallbackDataID(callback.DataID)
	if err != nil {
		response.ValidationError(c, "invalid callback target")
		return
	}

	result := "pass"
	for _, r := range callback.Results {
		result = mergeCallbackSuggestion(result, normalizeCallbackSuggestion(r.Suggestion))
	}

	var raw map[string]interface{}
	_ = json.Unmarshal([]byte(content), &raw)
	input := service.AICallbackInput{
		TargetType:     targetType,
		TargetID:       targetID,
		Result:         result,
		RawResponse:    raw,
		ProviderTaskID: strings.TrimSpace(callback.TaskID),
	}

	if _, ok := h.queueProducer.(*queue.NoopProducer); !ok && h.queueProducer != nil {
		recovery.GoSafe(func() {
			payload, _ := json.Marshal(map[string]interface{}{
				"action":           "process_ai_callback",
				"target_type":      input.TargetType,
				"target_id":        input.TargetID,
				"result":           input.Result,
				"raw_response":     input.RawResponse,
				"provider_task_id": input.ProviderTaskID,
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

// parseCallbackDataID restores a {target_type}:<id> business identifier into
// its parts. Only currently supported target types are accepted: content.
// The ip namespace is reserved for a future IP-media extension and, like any
// unknown or malformed identifier, must be rejected without side effects (IP
// has no media attachment today, so Aliyun never delivers an ip: callback).
func parseCallbackDataID(dataID string) (string, int64, error) {
	parts := strings.SplitN(dataID, ":", 2)
	if len(parts) != 2 {
		return "", 0, errors.New("invalid callback dataId")
	}
	targetType := strings.TrimSpace(parts[0])
	if !strings.EqualFold(targetType, "content") {
		return "", 0, errors.New("unsupported callback target type")
	}
	id, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil || id <= 0 {
		return "", 0, errors.New("invalid callback target id")
	}
	return targetType, id, nil
}

// normalizeCallbackSuggestion maps a raw Aliyun suggestion value onto the
// application review vocabulary used by ReviewService.ProcessAICallback.
func normalizeCallbackSuggestion(suggestion string) string {
	switch strings.ToLower(strings.TrimSpace(suggestion)) {
	case "block", "violation":
		return "block"
	case "review", "pending":
		return "review"
	default:
		return "pass"
	}
}

// mergeCallbackSuggestion keeps the strictest of two normalized suggestions
// (pass < review < block), mirroring the merge semantics of the review
// service so the inbound contract and the internal flow agree.
func mergeCallbackSuggestion(current, incoming string) string {
	priority := map[string]int{"pass": 1, "review": 2, "block": 3}
	if priority[incoming] > priority[current] {
		return incoming
	}
	return current
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
