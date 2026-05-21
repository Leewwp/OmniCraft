package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AdminHandler struct {
	ipSvc        *service.IPService
	contentRepo  *repository.ContentRepository
	userRepo     *repository.UserRepository
	socialRepo   *repository.SocialRepository
	llmConfigSvc *service.LLMConfigService
	cfg          *config.Config
	rdb          *redis.Client
	notifSvc     *service.NotificationService
	mu           sync.Mutex
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

func NewAdminHandler(db *gorm.DB, cfg *config.Config, rdb *redis.Client) *AdminHandler {
	return &AdminHandler{
		ipSvc:        service.NewIPService(repository.NewIPRepository(db)),
		contentRepo:  repository.NewContentRepository(db),
		userRepo:     repository.NewUserRepository(db),
		socialRepo:   repository.NewSocialRepository(db),
		llmConfigSvc: service.NewLLMConfigService(repository.NewLLMConfigRepository(db), cfg),
		cfg:          cfg,
		rdb:          rdb,
	}
}

func (h *AdminHandler) SetNotificationService(ns *service.NotificationService) {
	h.notifSvc = ns
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
	c.JSON(http.StatusOK, gin.H{"ips": ips, "total": total})
}

func (h *AdminHandler) ApproveIP(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid ip id"})
		return
	}
	if err := h.ipSvc.ApproveIP(id); err != nil {
		response.SafeErrorResponse(c, http.StatusBadRequest, "ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ip approved"})
}

func (h *AdminHandler) RejectIP(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid ip id"})
		return
	}
	if err := h.ipSvc.RejectIP(id); err != nil {
		response.SafeErrorResponse(c, http.StatusBadRequest, "ERROR", err)
		return
	}
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
	if err := h.contentRepo.UpdateContent(id, updates); err != nil {
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
	if err := h.contentRepo.UpdateContent(id, map[string]interface{}{"status": "published", "ban_reason": ""}); err != nil {
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
	if err := h.userRepo.UpdateFields(id, map[string]interface{}{"is_banned": true, "ban_reason": body.Reason}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to ban user"})
		return
	}
	user, _ := h.userRepo.FindByID(id)
	role := ""
	if user != nil {
		role = user.Role
	}
	middleware.SetUserStatusCache(h.rdb, id, true, role)
	c.JSON(http.StatusOK, gin.H{"message": "user banned"})
}

func (h *AdminHandler) UnbanUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}
	if err := h.userRepo.UpdateFields(id, map[string]interface{}{"is_banned": false, "ban_reason": ""}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to unban user"})
		return
	}
	user, _ := h.userRepo.FindByID(id)
	role := ""
	if user != nil {
		role = user.Role
	}
	middleware.SetUserStatusCache(h.rdb, id, false, role)
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
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
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
	if err := db.Transaction(func(tx *gorm.DB) error {
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
		GreenKeyConfigured:    h.cfg.Green.AccessKeyID != "" && h.cfg.Green.AccessKeySecret != "",
		LLMApiKeyConfigured:   h.cfg.Agent.LLMAPIKey != "",
		HMACSecretConfigured:  h.cfg.Agent.HMACSecret != "",
		DatabaseConfigured:    h.cfg.Database.DSN != "",
		RedisConfigured:       h.cfg.Redis.Addr != "",
	}
	c.JSON(http.StatusOK, gin.H{"config": public, "secrets_status": redactStatus})
}

func (h *AdminHandler) PatchConfig(c *gin.Context) {
	var patches map[string]interface{}
	if err := c.ShouldBindJSON(&patches); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_BODY", "message": err.Error()})
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

	if err := h.cfg.SaveOverride("data/config_override.yaml"); err != nil {
		slog.Error("failed to save config override", "error", err)
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
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}
	r, err := h.llmConfigSvc.CreateConfig(req.ConfigName, req.ProviderType, req.APIBase, req.Model, req.APIKey)
	if err != nil {
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
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}
	delete(req, "id")
	delete(req, "is_active")
	delete(req, "api_key_enc")
	if err := h.llmConfigSvc.UpdateConfig(id, req); err != nil {
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
	if err := h.llmConfigSvc.DeleteConfig(id); err != nil {
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
	if err := h.llmConfigSvc.ActivateConfig(id); err != nil {
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
	response, err := h.llmConfigSvc.TestConnection(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "TEST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"response": response})
}
