package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/redis/go-redis/v9"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type IPHandler struct {
	ipSvc          *service.IPService
	contentRepo    *repository.ContentRepository
	discussionRepo *repository.DiscussionRepository
	displaySigner  *service.DisplayURLSigner
}

func NewIPHandler(db *gorm.DB) *IPHandler {
	return &IPHandler{
		ipSvc:          service.NewIPService(repository.NewIPRepository(db)),
		contentRepo:    repository.NewContentRepository(db),
		discussionRepo: repository.NewDiscussionRepository(db),
	}
}

func NewIPHandlerWithCache(db *gorm.DB, rdb *redis.Client, cfg *config.Config) *IPHandler {
	reputSvc := service.NewReputationService(db)
	reviewSvc := service.NewReviewService(db, rdb, cfg, reputSvc)
	return &IPHandler{
		ipSvc:          service.NewIPServiceWithReview(repository.NewIPRepository(db), rdb, &cfg.Cache, reviewSvc),
		contentRepo:    repository.NewContentRepository(db),
		discussionRepo: repository.NewDiscussionRepository(db),
		displaySigner:  service.NewDisplayURLSigner(cfg),
	}
}

func (h *IPHandler) ListIPs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	filter := repository.ListIPsFilter{
		Search:   c.Query("q"),
		Category: c.Query("category"),
		Sort:     c.DefaultQuery("sort", "newest"),
		Page:     page,
		PageSize: pageSize,
	}

	ips, total, err := h.ipSvc.ListIPs(filter)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	h.displaySigner.DecorateIPs(ips)
	c.JSON(http.StatusOK, gin.H{
		"ips":       ips,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetMyIPs serves GET /users/me/ips: the caller's own IPs across every
// status, with the latest reject reason on rejected rows (T52/FIX-23b).
// The me route group already enforces auth.
func (h *IPHandler) GetMyIPs(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	ips, total, err := h.ipSvc.ListMyIPs(c.Request.Context(), callerID, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	h.displaySigner.DecorateIPs(ips)
	c.JSON(http.StatusOK, gin.H{
		"ips":       ips,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *IPHandler) CreateIP(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}

	var input service.CreateIPInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	ip, err := h.ipSvc.CreateIP(c.Request.Context(), input, callerID)
	if err != nil {
		if errors.Is(err, service.ErrIPSlugTaken) {
			c.JSON(http.StatusConflict, gin.H{"code": "IP_SLUG_TAKEN", "message": "could not derive a unique slug for this name, try a different name"})
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
		return
	}

	h.displaySigner.DecorateIP(ip)
	c.JSON(http.StatusCreated, gin.H{"ip": ip})
}

func (h *IPHandler) GetIP(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid ip id"})
		return
	}

	ip, err := h.ipSvc.GetIP(id)
	if err != nil {
		if err == service.ErrIPNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "IP_NOT_FOUND", "message": "ip not found"})
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	// GetIP may be served from the Redis detail cache; the signature is
	// re-issued on every response so it is never frozen into cached rows.
	h.displaySigner.DecorateIP(ip)
	// Hub stats are computed live (never cached inside the IP detail cache)
	// so follow/discussion counts stay fresh (#290).
	c.JSON(http.StatusOK, gin.H{"ip": ip, "stats": h.hubStats(id)})
}

// ipHubStats powers the IP detail header: follower / discussion / work counts.
// Discussions count only published rows (FIX-25 absorption); works count the
// public fanwork zone under the unified visibility scope.
type ipHubStats struct {
	FollowerCount   int64 `json:"follower_count"`
	DiscussionCount int64 `json:"discussion_count"`
	WorkCount       int64 `json:"work_count"`
}

func (h *IPHandler) hubStats(ipID int64) ipHubStats {
	var stats ipHubStats
	db := h.contentRepo.DB()
	if err := db.Model(&model.Follow{}).
		Where("target_type = ? AND target_id = ?", "ip", ipID).
		Count(&stats.FollowerCount).Error; err != nil {
		slog.Warn("ip hub stats follower count failed", "ip_id", ipID, "error", err)
	}
	if err := db.Model(&model.Discussion{}).
		Where("ip_id = ? AND status = ?", ipID, "published").
		Count(&stats.DiscussionCount).Error; err != nil {
		slog.Warn("ip hub stats discussion count failed", "ip_id", ipID, "error", err)
	}
	if err := repository.ApplyContentVisibilityScope(db.Model(&model.ContentItem{}), 0).
		Where("content_items.ip_id = ? AND content_items.zone = ?", ipID, "fanwork").
		Count(&stats.WorkCount).Error; err != nil {
		slog.Warn("ip hub stats work count failed", "ip_id", ipID, "error", err)
	}
	return stats
}

func (h *IPHandler) GetIPContents(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid ip id"})
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

	filter := repository.ListContentsFilter{
		IPID:     &id,
		ViewerID: middleware.GetUserID(c),
		Page:     page,
		PageSize: pageSize,
		// 分享 tab 只收该 IP 的二创（#290 §1.2，沿用 zone=fanwork）；
		// type 走白名单类目，q 做标题包含匹配。
		Zone:        "fanwork",
		ContentType: c.Query("type"),
		Sort:        c.DefaultQuery("sort", "hot"),
		Search:      c.Query("q"),
	}
	items, total, err := h.contentRepo.ListContents(filter)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	// 媒体类型 chips 计数分面（#290）：与列表同语义但不受当前 type 影响；
	// 分面查询仅服务展示，失败时降级为空计数不阻塞列表。
	typeCounts, err := h.contentRepo.CountByTypeWithinIP(id, filter.Search)
	if err != nil {
		slog.Warn("ip content type facet failed", "ip_id", id, "error", err)
		typeCounts = map[string]int64{}
	}
	h.displaySigner.DecorateContents(items)
	c.JSON(http.StatusOK, gin.H{
		"contents":    items,
		"total":       total,
		"type_counts": typeCounts,
		"page":        page,
		"page_size":   pageSize,
	})
}
