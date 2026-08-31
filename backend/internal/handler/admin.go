package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"omnicraft/backend/internal/worker"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var errAdminAuditWriteFailed = errors.New("admin audit write failed")

type AdminHandler struct {
	ipSvc         *service.IPService
	contentRepo   *repository.ContentRepository
	userRepo      *repository.UserRepository
	socialRepo    *repository.SocialRepository
	llmConfigSvc  *service.LLMConfigService
	auditSvc      *service.AdminAuditService
	cfg           *config.Config
	rdb           *redis.Client
	notifSvc      *service.NotificationService
	dlqWorker     *worker.DLQWorker
	displaySigner *service.DisplayURLSigner
	mu            sync.Mutex
}

var sensitiveConfigFields = map[string]bool{
	"secret": true, "access_key_id": true, "access_key_secret": true,
	"api_key": true, "dsn": true, "hmac_secret": true, "password": true,
}

func isSensitivePatchKey(key string) bool {
	lower := strings.ToLower(key)
	for field := range sensitiveConfigFields {
		if strings.Contains(lower, field) {
			return true
		}
	}
	return false
}

func NewAdminHandler(db *gorm.DB, cfg *config.Config, rdb *redis.Client, auditSvc *service.AdminAuditService) *AdminHandler {
	var dlqWorker *worker.DLQWorker
	if rdb != nil {
		dlqWorker = worker.NewDLQWorker(rdb)
	}
	return &AdminHandler{
		ipSvc:         service.NewIPService(repository.NewIPRepository(db)),
		contentRepo:   repository.NewContentRepository(db),
		userRepo:      repository.NewUserRepository(db),
		socialRepo:    repository.NewSocialRepository(db),
		llmConfigSvc:  service.NewLLMConfigService(repository.NewLLMConfigRepository(db), cfg),
		auditSvc:      auditSvc,
		cfg:           cfg,
		rdb:           rdb,
		dlqWorker:     dlqWorker,
		displaySigner: service.NewDisplayURLSigner(cfg),
	}
}

func (h *AdminHandler) SetNotificationService(ns *service.NotificationService) {
	h.notifSvc = ns
}

type broadcastRequest struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	Channel string `json:"channel"`
}

func (h *AdminHandler) BroadcastNotification(c *gin.Context) {
	if h.notifSvc == nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", errors.New("notification service unavailable"))
		return
	}

	var req broadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, "invalid request body")
		return
	}

	recipientCount, broadcastAt, replayed, err := h.notifSvc.BroadcastSystemNotification(
		c.Request.Context(),
		req.Title,
		req.Body,
		req.Channel,
		middleware.GetUserID(c),
		c.GetHeader("Idempotency-Key"),
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrIdempotencyKeyRequired):
			c.JSON(http.StatusBadRequest, gin.H{"code": "IDEMPOTENCY_KEY_REQUIRED", "message": "Idempotency-Key header is required"})
		case errors.Is(err, service.ErrIdempotencyKeyReused):
			c.JSON(http.StatusConflict, gin.H{"code": "IDEMPOTENCY_KEY_REUSED", "message": "idempotency key was already used with a different payload"})
		case errors.Is(err, service.ErrBroadcastValidation):
			response.ValidationError(c, "invalid request parameters")
		default:
			response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"recipient_count": recipientCount,
			"broadcast_at":    broadcastAt,
			"replayed":        replayed,
		},
	})
}

func (h *AdminHandler) ListPendingIPs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	ips, total, err := h.ipSvc.ListIPs(repository.ListIPsFilter{
		Status:   "pending",
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	h.displaySigner.DecorateIPs(ips)
	c.JSON(http.StatusOK, gin.H{"ips": ips, "total": total})
}

func (h *AdminHandler) ApproveIP(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid ip id"})
		return
	}
	entry := h.auditEntry(c, "ip_approve", "ip", strconv.FormatInt(id, 10), map[string]any{"ip_id": id, "decision": "approved"})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		var ip model.IP
		if err := tx.First(&ip, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return service.ErrIPNotFound
			}
			return err
		}
		return tx.Model(&model.IP{}).Where("id = ?", id).Update("status", "approved").Error
	}); err != nil {
		if h.respondAuditTxError(c, err, http.StatusBadRequest, "ERROR", "failed to approve ip") {
			return
		}
		response.SafeErrorResponse(c, http.StatusBadRequest, "ERROR", err)
		return
	}
	h.ipSvc.InvalidateIPCacheForAdmin(id)
	c.JSON(http.StatusOK, gin.H{"message": "ip approved"})
}

func (h *AdminHandler) RejectIP(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid ip id"})
		return
	}
	entry := h.auditEntry(c, "ip_reject", "ip", strconv.FormatInt(id, 10), map[string]any{"ip_id": id, "decision": "rejected"})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		var ip model.IP
		if err := tx.First(&ip, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return service.ErrIPNotFound
			}
			return err
		}
		return tx.Model(&model.IP{}).Where("id = ?", id).Update("status", "rejected").Error
	}); err != nil {
		if h.respondAuditTxError(c, err, http.StatusBadRequest, "ERROR", "failed to reject ip") {
			return
		}
		response.SafeErrorResponse(c, http.StatusBadRequest, "ERROR", err)
		return
	}
	h.ipSvc.InvalidateIPCacheForAdmin(id)
	c.JSON(http.StatusOK, gin.H{"message": "ip rejected"})
}

func (h *AdminHandler) ListUnderReviewContents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	contentSvc := service.NewContentService(h.contentRepo)
	contents, total, err := contentSvc.ListContents(repository.ListContentsFilter{
		Status:   "under_review",
		Page:     page,
		PageSize: pageSize,
	}, 0)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	h.displaySigner.DecorateContents(contents)
	c.JSON(http.StatusOK, gin.H{"contents": contents, "total": total})
}

func (h *AdminHandler) ListTrashedContents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var contents []model.ContentItem
	var total int64
	h.contentRepo.DB().Model(&model.ContentItem{}).Where("deleted_at IS NOT NULL").Count(&total)
	h.contentRepo.DB().Preload("Author").Where("deleted_at IS NOT NULL").
		Order("deleted_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&contents)

	h.displaySigner.DecorateContents(contents)
	c.JSON(http.StatusOK, gin.H{"contents": contents, "total": total, "page": page, "page_size": pageSize})
}

func (h *AdminHandler) BanContent(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		slog.Warn("ban content: failed to bind json", "error", err)
	}
	updates := map[string]interface{}{"status": "banned"}
	if body.Reason != "" {
		updates["ban_reason"] = body.Reason
	}
	entry := h.auditEntry(c, "content_ban", "content", strconv.FormatInt(id, 10), map[string]any{"content_id": id, "reason": body.Reason})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		return tx.Model(&model.ContentItem{}).Where("id = ?", id).Updates(updates).Error
	}); err != nil {
		if h.respondAuditTxError(c, err, http.StatusInternalServerError, "DB_ERROR", "failed to ban content") {
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "content banned"})
}

func (h *AdminHandler) RestoreContent(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}
	entry := h.auditEntry(c, "content_restore", "content", strconv.FormatInt(id, 10), map[string]any{"content_id": id})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		return tx.Model(&model.ContentItem{}).Where("id = ?", id).Updates(map[string]interface{}{"status": "published", "ban_reason": ""}).Error
	}); err != nil {
		if h.respondAuditTxError(c, err, http.StatusInternalServerError, "DB_ERROR", "failed to restore content") {
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "content restored"})
}

func (h *AdminHandler) BanUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		slog.Warn("ban user: failed to bind json", "error", err)
	}
	entry := h.auditEntry(c, "user_ban", "user", strconv.FormatInt(id, 10), map[string]any{"target_user_id": id, "reason": body.Reason})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		return tx.Model(&model.User{}).Where("id = ?", id).Updates(map[string]interface{}{"is_banned": true, "ban_reason": body.Reason}).Error
	}); err != nil {
		if h.respondAuditTxError(c, err, http.StatusInternalServerError, "DB_ERROR", "failed to ban user") {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to ban user"})
		return
	}
	middleware.InvalidateUserStatusCache(h.rdb, id)
	c.JSON(http.StatusOK, gin.H{"message": "user banned"})
}

func (h *AdminHandler) UnbanUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}
	entry := h.auditEntry(c, "user_unban", "user", strconv.FormatInt(id, 10), map[string]any{"target_user_id": id})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		return tx.Model(&model.User{}).Where("id = ?", id).Updates(map[string]interface{}{"is_banned": false, "ban_reason": ""}).Error
	}); err != nil {
		if h.respondAuditTxError(c, err, http.StatusInternalServerError, "DB_ERROR", "failed to unban user") {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to unban user"})
		return
	}
	middleware.InvalidateUserStatusCache(h.rdb, id)
	c.JSON(http.StatusOK, gin.H{"message": "user unbanned"})
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var users []map[string]interface{}
	var total int64
	h.userRepo.DB().Model(&struct{ ID uint }{}).Table("users").Count(&total)
	h.userRepo.DB().Table("users").Select("id, username, email, role, is_banned, reputation, created_at").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&users)
	c.JSON(http.StatusOK, gin.H{"users": users, "total": total})
}

func (h *AdminHandler) ListAppeals(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var appeals []map[string]interface{}
	var total int64
	h.userRepo.DB().Model(&struct{ ID uint }{}).Table("appeals").Where("status = ?", "pending").Count(&total)
	h.userRepo.DB().Table("appeals").Where("status = ?", "pending").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&appeals)
	c.JSON(http.StatusOK, gin.H{"appeals": appeals, "total": total})
}

func (h *AdminHandler) ResolveAppeal(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid appeal id"})
		return
	}
	var body struct {
		Status        string `json:"status" binding:"required,oneof=approved rejected"`
		AdminResponse string `json:"admin_response"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request body")
		return
	}
	db := h.userRepo.DB()
	var appeal model.Appeal
	if err := db.First(&appeal, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "APPEAL_NOT_FOUND", "message": "appeal not found"})
		return
	}

	updates := map[string]interface{}{
		"status":         body.Status,
		"admin_response": body.AdminResponse,
		"resolved_by":    middleware.GetUserID(c),
		"resolved_at":    gorm.Expr("NOW()"),
	}
	entry := h.auditEntry(c, "appeal_resolve", "appeal", strconv.FormatInt(id, 10), map[string]any{"appeal_id": id, "decision": body.Status, "reason": body.AdminResponse})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		if err := tx.Model(&model.Appeal{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		targetUpdates := service.AppealTargetUpdates(appeal.TargetType, body.Status)
		if len(targetUpdates) == 0 {
			return nil
		}
		switch appeal.TargetType {
		case "content":
			return tx.Model(&model.ContentItem{}).Where("id = ?", appeal.TargetID).Updates(targetUpdates).Error
		case "comment":
			return tx.Model(&model.Comment{}).Where("id = ?", appeal.TargetID).Updates(targetUpdates).Error
		default:
			return nil
		}
	}); err != nil {
		if h.respondAuditTxError(c, err, http.StatusInternalServerError, "DB_ERROR", "failed to resolve appeal") {
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	if h.notifSvc != nil {
		adminID := middleware.GetUserID(c)
		h.notifSvc.Notify(appeal.UserID, "system", "appeal_result", "申诉处理结果", body.AdminResponse, "appeal", appeal.ID, adminID)
	}
	c.JSON(http.StatusOK, gin.H{"message": "appeal resolved"})
}

func (h *AdminHandler) GetConfig(c *gin.Context) {
	public := model.PublicConfig{
		Features:   h.cfg.Features,
		Limits:     h.cfg.Limits,
		Reputation: h.cfg.Reputation,
		Judge:      h.cfg.Judge,
		Social:     h.cfg.Social,
		Agent: model.PublicAgentConfig{
			WebAgentEnabled:       h.cfg.Agent.WebAgentEnabled,
			RateLimitPerDay:       h.cfg.Agent.RateLimitPerDay,
			UploadAssistMaxFileMB: h.cfg.Agent.UploadAssistMaxFileMB,
		},
		Upload:         h.cfg.Upload,
		Cache:          h.cfg.Cache,
		RateLimit:      h.cfg.RateLimit,
		Recommendation: h.cfg.Recommendation,
	}
	redactStatus := model.ConfigRedactStatus{
		JWTSecretConfigured:  h.cfg.JWT.Secret != "",
		OSSKeyConfigured:     h.cfg.OSS.AccessKeyID != "" && h.cfg.OSS.AccessKeySecret != "",
		GreenKeyConfigured:   h.cfg.Green.AccessKeyID != "" && h.cfg.Green.AccessKeySecret != "",
		LLMApiKeyConfigured:  h.cfg.Agent.LLMAPIKey != "",
		HMACSecretConfigured: h.cfg.Agent.HMACSecret != "",
		DatabaseConfigured:   h.cfg.Database.DSN != "",
		RedisConfigured:      h.cfg.Redis.Addr != "",
	}
	c.JSON(http.StatusOK, gin.H{"config": public, "secrets_status": redactStatus})
}

func (h *AdminHandler) PatchConfig(c *gin.Context) {
	var patches map[string]interface{}
	if err := c.ShouldBindJSON(&patches); err != nil {
		response.ValidationError(c, "invalid request body")
		return
	}

	filterSensitivePatches(patches)

	if len(patches) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "NO_ALLOWED_FIELDS", "message": "no allowed fields in request"})
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if limits, ok := patches["limits"].(map[string]interface{}); ok {
		if v, ok := limits["video_max_mb"].(float64); ok {
			h.cfg.Limits.VideoMaxMB = int(v)
		}
		if v, ok := limits["image_max_mb"].(float64); ok {
			h.cfg.Limits.ImageMaxMB = int(v)
		}
		if v, ok := limits["mod_max_mb"].(float64); ok {
			h.cfg.Limits.ModMaxMB = int(v)
		}
		if v, ok := limits["text_max_mb"].(float64); ok {
			h.cfg.Limits.TextMaxMB = int(v)
		}
		if v, ok := limits["sheet_music_max_mb"].(float64); ok {
			h.cfg.Limits.SheetMusicMaxMB = int(v)
		}
		if v, ok := limits["video_max_sec"].(float64); ok {
			h.cfg.Limits.VideoMaxSec = int(v)
		}
	}
	if features, ok := patches["features"].(map[string]interface{}); ok {
		if v, ok := features["payment_enabled"].(bool); ok {
			h.cfg.Features.PaymentEnabled = v
		}
		if v, ok := features["creator_support_enabled"].(bool); ok {
			h.cfg.Features.CreatorSupportEnabled = v
		}
	}
	if reputation, ok := patches["reputation"].(map[string]interface{}); ok {
		if v, ok := reputation["repeat_violation_threshold"].(float64); ok {
			h.cfg.Reputation.RepeatViolationThreshold = int(v)
		}
		if v, ok := reputation["repeat_violation_window_days"].(float64); ok {
			h.cfg.Reputation.RepeatViolationWindowDays = int(v)
		}
		if v, ok := reputation["repeat_violation_extra_penalty"].(float64); ok {
			h.cfg.Reputation.RepeatViolationExtraPenalty = int(v)
		}
	}
	if agent, ok := patches["agent"].(map[string]interface{}); ok {
		if v, ok := agent["web_agent_enabled"].(bool); ok {
			h.cfg.Agent.WebAgentEnabled = v
		}
		if v, ok := agent["rate_limit_per_day"].(float64); ok {
			h.cfg.Agent.RateLimitPerDay = int(v)
		}
	}
	if social, ok := patches["social"].(map[string]interface{}); ok {
		if v, ok := social["report_auto_hide_rate"].(float64); ok {
			h.cfg.Social.ReportAutoHideRate = v
		}
		if v, ok := social["comment_fold_threshold"].(float64); ok {
			h.cfg.Social.CommentFoldThreshold = v
		}
	}
	if recommendation, ok := patches["recommendation"].(map[string]interface{}); ok {
		if v, ok := recommendation["personalization_weight"].(float64); ok {
			h.cfg.Recommendation.PersonalizationWeight = v
		}
		if v, ok := recommendation["min_interaction_for_personalize"].(float64); ok {
			h.cfg.Recommendation.MinInteractionForPersonalize = int(v)
		}
	}

	overridePath := "data/config_override.yaml"
	if v := os.Getenv("CONFIG_OVERRIDE_PATH"); v != "" {
		overridePath = v
	}
	if err := h.cfg.SaveOverride(overridePath); err != nil {
		slog.Error("failed to save config override", "error", err)
	}

	if !h.auditOrFail(c, "config_patch", "config", "", map[string]any{"field": "multiple", "old_value_masked": "***", "new_value_masked": "***"}) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config": model.PublicConfig{
			Features:   h.cfg.Features,
			Limits:     h.cfg.Limits,
			Reputation: h.cfg.Reputation,
			Judge:      h.cfg.Judge,
			Social:     h.cfg.Social,
			Agent: model.PublicAgentConfig{
				WebAgentEnabled:       h.cfg.Agent.WebAgentEnabled,
				RateLimitPerDay:       h.cfg.Agent.RateLimitPerDay,
				UploadAssistMaxFileMB: h.cfg.Agent.UploadAssistMaxFileMB,
			},
			Upload:         h.cfg.Upload,
			Cache:          h.cfg.Cache,
			RateLimit:      h.cfg.RateLimit,
			Recommendation: h.cfg.Recommendation,
		},
		"message": "config updated",
	})
}

func filterSensitivePatches(patches map[string]interface{}) {
	for key := range patches {
		if isSensitivePatchKey(key) {
			delete(patches, key)
			continue
		}
		if nested, ok := patches[key].(map[string]interface{}); ok {
			for nestedKey := range nested {
				if isSensitivePatchKey(nestedKey) {
					delete(nested, nestedKey)
				}
			}
			if len(nested) == 0 {
				delete(patches, key)
			}
		}
	}
}

func (h *AdminHandler) ListLLMConfigs(c *gin.Context) {
	configs, err := h.llmConfigSvc.ListConfigs()
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"configs": configs})
}

func (h *AdminHandler) CreateLLMConfig(c *gin.Context) {
	var req struct {
		ConfigName   string `json:"config_name" binding:"required"`
		ProviderType string `json:"provider_type" binding:"required"`
		APIBase      string `json:"api_base"`
		Model        string `json:"model" binding:"required"`
		APIKey       string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, "invalid request body")
		return
	}
	var r *service.LLMConfigResponse
	entry := h.auditEntry(c, "llm_config_create", "llm_config", "", map[string]any{"provider": req.ProviderType, "model": req.Model})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		var txErr error
		r, txErr = h.llmConfigSvc.CreateConfigTx(tx, req.ConfigName, req.ProviderType, req.APIBase, req.Model, req.APIKey)
		if txErr != nil {
			return txErr
		}
		entry.TargetID = strconv.FormatInt(r.ID, 10)
		return nil
	}); err != nil {
		if h.respondAuditTxError(c, err, http.StatusInternalServerError, "DB_ERROR", "failed to create config") {
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"config": r})
}

func (h *AdminHandler) UpdateLLMConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid config id"})
		return
	}
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, "invalid request body")
		return
	}
	delete(req, "id")
	delete(req, "is_active")
	delete(req, "api_key_enc")
	entry := h.auditEntry(c, "llm_config_update", "llm_config", strconv.FormatInt(id, 10), map[string]any{"config_id": id})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		return h.llmConfigSvc.UpdateConfigTx(tx, id, req)
	}); err != nil {
		if h.respondAuditTxError(c, err, http.StatusInternalServerError, "DB_ERROR", "failed to update config") {
			return
		}
		if err == service.ErrConfigNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "CONFIG_NOT_FOUND", "message": "config not found"})
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
}

func (h *AdminHandler) DeleteLLMConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid config id"})
		return
	}
	entry := h.auditEntry(c, "llm_config_delete", "llm_config", strconv.FormatInt(id, 10), map[string]any{"config_id": id})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		return h.llmConfigSvc.DeleteConfigTx(tx, id)
	}); err != nil {
		if h.respondAuditTxError(c, err, http.StatusInternalServerError, "DB_ERROR", "failed to delete config") {
			return
		}
		if err == repository.ErrActiveConfigCannotDelete {
			c.JSON(http.StatusConflict, gin.H{"code": "ACTIVE_CONFIG", "message": "cannot delete active config"})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"code": "CONFIG_NOT_FOUND", "message": "config not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "config deleted"})
}

func (h *AdminHandler) ActivateLLMConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid config id"})
		return
	}
	entry := h.auditEntry(c, "llm_config_activate", "llm_config", strconv.FormatInt(id, 10), map[string]any{"config_id": id})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		return h.llmConfigSvc.ActivateConfigTx(tx, id)
	}); err != nil {
		if h.respondAuditTxError(c, err, http.StatusInternalServerError, "DB_ERROR", "failed to activate config") {
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"code": "CONFIG_NOT_FOUND", "message": "config not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "config activated"})
}

func (h *AdminHandler) TestLLMConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid config id"})
		return
	}
	resp, err := h.llmConfigSvc.TestConnection(id)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "TEST_FAILED", err)
		return
	}
	_ = h.auditOrFail(c, "llm_config_test", "llm_config", strconv.FormatInt(id, 10), map[string]any{"config_id": id})
	c.JSON(http.StatusOK, gin.H{"response": resp})
}

func (h *AdminHandler) ListReports(c *gin.Context) {
	status := c.Query("status")
	targetType := c.Query("target_type")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	searchRepo := repository.NewSearchRepository(h.contentRepo.DB())
	reports, total, err := searchRepo.ListReports(status, targetType, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	if reports == nil {
		reports = []model.Report{}
	}
	c.JSON(http.StatusOK, gin.H{
		"reports":   reports,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *AdminHandler) ResolveReport(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid report id"})
		return
	}
	var body struct {
		Status      string `json:"status" binding:"required"`
		ActionTaken string `json:"action_taken"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "status is required"})
		return
	}
	if body.Status != "resolved" && body.Status != "dismissed" {
		h.auditFailed(c, "report_resolve", "report", strconv.FormatInt(id, 10), map[string]any{
			"report_id": id,
			"decision":  body.Status,
			"reason":    "VALIDATION_ERROR",
		})
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "status must be resolved or dismissed"})
		return
	}
	entry := h.auditEntry(c, "report_resolve", "report", strconv.FormatInt(id, 10), map[string]any{"report_id": id, "decision": body.Status, "reason": body.ActionTaken})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		return tx.Model(&model.Report{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":       body.Status,
			"action_taken": body.ActionTaken,
		}).Error
	}); err != nil {
		if h.respondAuditTxError(c, err, http.StatusInternalServerError, "DB_ERROR", "failed to update report") {
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "report updated"})
}

func (h *AdminHandler) GetReportStats(c *gin.Context) {
	searchRepo := repository.NewSearchRepository(h.contentRepo.DB())
	stats, err := searchRepo.GetReportStats()
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *AdminHandler) GetQueueStats(c *gin.Context) {
	topics := []string{
		"content.review",
		"ip.review",
		"notification.create",
		"count.download",
		"content.embedding",
	}
	stats, err := queue.GetQueueStats(c.Request.Context(), h.rdb, topics)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "QUEUE_STATS_ERROR", err)
		return
	}

	dlqCount, _ := queue.GetDLQStats(c.Request.Context(), h.rdb)
	c.JSON(http.StatusOK, gin.H{
		"topics":        stats,
		"dlq_count":     dlqCount,
		"queue_enabled": h.cfg.Queue.Enabled,
	})
}

func (h *AdminHandler) GetDLQEntries(c *gin.Context) {
	limit := int64(100)
	if l, err := strconv.ParseInt(c.DefaultQuery("limit", "100"), 10, 64); err == nil && l > 0 && l <= 1000 {
		limit = l
	}

	if h.dlqWorker == nil {
		c.JSON(http.StatusOK, gin.H{"entries": []worker.DLQEntry{}, "count": 0})
		return
	}

	entries, err := h.dlqWorker.Consume(c.Request.Context(), limit)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DLQ_ERROR", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"entries": entries,
		"count":   len(entries),
	})
}

// dlqEntryIDPattern matches Redis stream ids ("<millis>-<seq>"), the only
// valid dead-letter entry identifiers.
var dlqEntryIDPattern = regexp.MustCompile(`^\d+-\d+$`)

// ReplayDLQEntry re-delivers one dead-letter entry back to its original topic
// (mounts DLQWorker.Replay, which previously had no route). Admin-only via the
// /api/v1/admin group middleware; every attempt is recorded in the admin
// audit log with the operator, time and entry id.
func (h *AdminHandler) ReplayDLQEntry(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if !dlqEntryIDPattern.MatchString(id) {
		h.auditFailed(c, "dlq_replay", "dlq_entry", id, map[string]any{"error_code": "INVALID_ID"})
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid dlq entry id"})
		return
	}
	if h.dlqWorker == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "DLQ_UNAVAILABLE", "message": "dead-letter queue is unavailable"})
		return
	}

	entry, err := h.dlqWorker.Replay(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, worker.ErrDLQEntryNotFound) {
			h.auditFailed(c, "dlq_replay", "dlq_entry", id, map[string]any{"error_code": "DLQ_ENTRY_NOT_FOUND"})
			c.JSON(http.StatusNotFound, gin.H{"code": "DLQ_ENTRY_NOT_FOUND", "message": "dlq entry not found"})
			return
		}
		h.auditFailed(c, "dlq_replay", "dlq_entry", id, map[string]any{"error_code": "REPLAY_FAILED"})
		response.SafeErrorResponse(c, http.StatusInternalServerError, "REPLAY_FAILED", err)
		return
	}

	if !h.auditOrFail(c, "dlq_replay", "dlq_entry", id, map[string]any{
		"original_topic": entry.OriginalTopic,
		"original_id":    entry.OriginalID,
	}) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "dlq entry replayed", "entry": entry})
}

func (h *AdminHandler) auditOrFail(c *gin.Context, action, targetType, targetID string, metadata map[string]any) bool {
	if h.auditSvc == nil {
		return true
	}
	entry := h.auditEntry(c, action, targetType, targetID, metadata)
	if err := h.auditSvc.Record(c.Request.Context(), entry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "AUDIT_WRITE_FAILED", "message": "audit write failed"})
		return false
	}
	return true
}

func (h *AdminHandler) auditEntry(c *gin.Context, action, targetType, targetID string, metadata map[string]any) service.RecordAdminAuditInput {
	return service.RecordAdminAuditInput{
		AdminUserID: middleware.GetUserID(c),
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		TraceID:     c.GetString("trace_id"),
		Metadata:    metadata,
		Result:      "success",
	}
}

func (h *AdminHandler) auditFailed(c *gin.Context, action, targetType, targetID string, metadata map[string]any) {
	if h.auditSvc == nil {
		return
	}
	entry := h.auditEntry(c, action, targetType, targetID, metadata)
	entry.Result = "failed"
	if err := h.auditSvc.Record(c.Request.Context(), entry); err != nil {
		slog.Warn("failed to record admin audit failure", "action", action, "target_type", targetType, "error", err)
	}
}

func (h *AdminHandler) withAuditTx(c *gin.Context, entry *service.RecordAdminAuditInput, mutate func(tx *gorm.DB) error) error {
	db := h.contentRepo.DB()
	return db.Transaction(func(tx *gorm.DB) error {
		if err := mutate(tx); err != nil {
			return err
		}
		if h.auditSvc == nil {
			return nil
		}
		if err := h.auditSvc.RecordTx(c.Request.Context(), tx, *entry); err != nil {
			return fmt.Errorf("%w: %v", errAdminAuditWriteFailed, err)
		}
		return nil
	})
}

func (h *AdminHandler) respondAuditTxError(c *gin.Context, err error, status int, code, message string) bool {
	if errors.Is(err, errAdminAuditWriteFailed) {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "AUDIT_WRITE_FAILED", "message": "audit write failed"})
		return true
	}
	if status > 0 && code != "" && message != "" {
		return false
	}
	return false
}
