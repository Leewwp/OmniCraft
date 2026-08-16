package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
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
		response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
		return
	}

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

	c.JSON(http.StatusOK, gin.H{"ip": ip})
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

	items, total, err := h.contentRepo.ListContents(repository.ListContentsFilter{
		IPID:     &id,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"contents": items, "total": total, "page": page, "page_size": pageSize})
}


