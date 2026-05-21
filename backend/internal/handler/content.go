package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/redis/go-redis/v9"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ContentHandler struct {
	contentSvc      *service.ContentService
	contentRepo     *repository.ContentRepository
	browseHistoryRepo *repository.BrowseHistoryRepository
	ossSvc          *service.OSSService
	ossInitErr      error
	rdb             *redis.Client
	cfg             *config.Config
}

func NewContentHandler(db *gorm.DB, cfg *config.Config, rdb *redis.Client) *ContentHandler {
	repo := repository.NewContentRepository(db)
	ossSvc, ossErr := service.NewOSSService(cfg)
	reputSvc := service.NewReputationService(db)
	reviewSvc := service.NewReviewService(db, rdb, cfg, reputSvc)

	contentSvc := service.NewContentServiceWithOSS(repo, reviewSvc, rdb, &cfg.Cache, ossSvc)

	embeddingRepo := repository.NewEmbeddingRepository(db)
	recSvc := service.NewRecommendationService(db, embeddingRepo, repo, contentSvc, rdb, &cfg.Recommendation)
	contentSvc.SetRecommendationService(recSvc)

	return &ContentHandler{
		contentSvc:        contentSvc,
		contentRepo:       repo,
		browseHistoryRepo: repository.NewBrowseHistoryRepository(db),
		ossSvc:            ossSvc,
		ossInitErr:        ossErr,
		rdb:               rdb,
		cfg:               cfg,
	}
}

func (h *ContentHandler) GenerateOSSToken(c *gin.Context) {
	if h.ossSvc == nil {
		msg := "oss service is not configured"
		if h.ossInitErr != nil {
			msg = h.ossInitErr.Error()
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "OSS_NOT_CONFIGURED", "message": msg})
		return
	}

	var req service.PresignUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)
	resp, err := h.ossSvc.GeneratePresignUploadURL(c.Request.Context(), req, userID)
	if err != nil {
		var validationErr *service.UploadValidationError
		if errors.As(err, &validationErr) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": validationErr.Error()})
			return
		}
		if errors.Is(err, service.ErrOSSNotConfigured) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": "OSS_NOT_CONFIGURED", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *ContentHandler) ListContents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	var ipID *int64
	if ipIDStr := c.Query("ip_id"); ipIDStr != "" {
		if v, err := strconv.ParseInt(ipIDStr, 10, 64); err == nil {
			ipID = &v
		}
	}

	var authorID *int64
	if authorStr := c.Query("author_id"); authorStr != "" {
		if v, err := strconv.ParseInt(authorStr, 10, 64); err == nil {
			authorID = &v
		}
	}

	var sourceOriginalID *int64
	if sourceStr := c.Query("source_original_id"); sourceStr != "" {
		if v, err := strconv.ParseInt(sourceStr, 10, 64); err == nil {
			sourceOriginalID = &v
		}
	}

	tags := parseCSVQuery(c.Query("tags"))
	contentTypes := parseCSVQuery(c.Query("content_type"))
	contentType := ""
	if len(contentTypes) == 1 {
		contentType = contentTypes[0]
	}
	if len(contentTypes) > 1 {
		contentType = ""
	}

	filter := repository.ListContentsFilter{
		Zone:             c.Query("zone"),
		IPID:             ipID,
		Category:         c.Query("category"),
		ContentType:      contentType,
		ContentTypes:     contentTypes,
		AuthorID:         authorID,
		SourceOriginalID: sourceOriginalID,
		Tags:             tags,
		Sort:             c.DefaultQuery("sort", "newest"),
		TimeRange:        c.DefaultQuery("time_range", "all"),
		Page:             page,
		PageSize:         pageSize,
	}

	contents, total, err := h.contentSvc.ListContents(filter, middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"contents":  contents,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *ContentHandler) CreateContent(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}

	var input service.PublishContentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}

	content, err := h.contentSvc.PublishContent(input, callerID)
	if err != nil {
		if errors.Is(err, service.ErrPublishFrozen) {
			c.JSON(http.StatusForbidden, gin.H{"code": "PUBLISH_FROZEN", "message": err.Error()})
			return
		}
		if errors.Is(err, service.ErrInvalidSourceOriginal) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_SOURCE_ORIGINAL", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"content": content})
}

func (h *ContentHandler) GetContent(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}

	content, err := h.contentSvc.GetContent(id)
	if err != nil {
		if err == service.ErrContentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "content not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	if err := h.contentRepo.IncrViewCount(id); err != nil {
		slog.Error("failed to incr view count", "content_id", id, "error", err)
	}
	h.contentSvc.IncrViewCount(id)

	userID := middleware.GetUserID(c)
	if userID > 0 {
		if err := h.browseHistoryRepo.Upsert(userID, id); err != nil {
			slog.Error("failed to record browse history", "user_id", userID, "content_id", id, "error", err)
		}
	}

	attachments, err := h.contentRepo.GetAttachments(id)
	if err != nil {
		slog.Error("failed to get attachments", "content_id", id, "error", err)
		attachments = nil
	}
	tags, err2 := h.contentRepo.GetTags(id)
	if err2 != nil {
		slog.Error("failed to get tags", "content_id", id, "error", err2)
		tags = nil
	}

	resp := gin.H{
		"content":     content,
		"attachments": attachments,
		"tags":        tags,
	}

	currentUserID := middleware.GetUserID(c)
	if currentUserID > 0 {
		var count int64
		h.contentRepo.DB().Model(&model.Favorite{}).
			Where("user_id = ? AND content_item_id = ?", currentUserID, id).
			Count(&count)
		resp["is_favorited"] = count > 0
	}

	if content.SourceOriginalID != nil && *content.SourceOriginalID > 0 {
		source, srcErr := h.contentSvc.GetContent(*content.SourceOriginalID)
		if srcErr == nil && source != nil {
			resp["source_original"] = gin.H{"id": source.ID, "title": source.Title}
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *ContentHandler) ListRelatedFanworks(c *gin.Context) {
	sourceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}

	source, err := h.contentSvc.GetContent(sourceID)
	if err != nil {
		if errors.Is(err, service.ErrContentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "content not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	if source.Zone != "original" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "NOT_ORIGINAL", "message": "related fanworks require original content"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	contentTypes := parseCSVQuery(c.Query("content_type"))
	contentType := ""
	if len(contentTypes) == 1 {
		contentType = contentTypes[0]
	}
	if len(contentTypes) > 1 {
		contentType = ""
	}

	contents, total, err := h.contentSvc.ListContents(repository.ListContentsFilter{
		Zone:             "fanwork",
		SourceOriginalID: &sourceID,
		ContentType:      contentType,
		ContentTypes:     contentTypes,
		Sort:             c.DefaultQuery("sort", "newest"),
		TimeRange:        c.DefaultQuery("time_range", "all"),
		Page:             page,
		PageSize:         pageSize,
	}, middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"source_original_id": sourceID,
		"contents":           contents,
		"total":              total,
		"page":               page,
		"page_size":          pageSize,
	})
}

func parseCSVQuery(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func (h *ContentHandler) UpdateContent(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}

	var req struct {
		Title         *string `json:"title"`
		CoverImageURL *string `json:"cover_image_url"`
		IsPublic      *bool   `json:"is_public"`
		AllowCopy     *bool   `json:"allow_copy"`
		AgentEnabled  *bool   `json:"agent_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_BODY", "message": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.CoverImageURL != nil {
		updates["cover_image_url"] = *req.CoverImageURL
	}
	if req.IsPublic != nil {
		updates["is_public"] = *req.IsPublic
	}
	if req.AllowCopy != nil {
		updates["allow_copy"] = *req.AllowCopy
	}
	if req.AgentEnabled != nil {
		updates["agent_enabled"] = *req.AgentEnabled
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "NO_FIELDS", "message": "no fields to update"})
		return
	}

	if err := h.contentSvc.UpdateContent(id, callerID, updates); err != nil {
		if err == service.ErrContentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "content not found"})
			return
		}
		if err == service.ErrContentForbidden {
			c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "not content author"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *ContentHandler) DeleteContent(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}

	if err := h.contentSvc.DeleteContent(id, callerID); err != nil {
		if err == service.ErrContentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "content not found"})
			return
		}
		if err == service.ErrContentForbidden {
			c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "not content author"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *ContentHandler) DownloadContent(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}

	content, err := h.contentRepo.FindByID(id)
	if err != nil || content == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "content not found"})
		return
	}

	if content.Status != "published" {
		c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "content not available for download"})
		return
	}

	if !content.AllowCopy {
		c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "download not allowed"})
		return
	}

	if h.ossSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "OSS_NOT_CONFIGURED", "message": "oss service not configured"})
		return
	}

	attachments, _ := h.contentRepo.GetAttachments(id)
	if len(attachments) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": "NO_ATTACHMENTS", "message": "no downloadable files"})
		return
	}

	// Use first primary attachment or first attachment
	ossKey := attachments[0].OSSKey
	for _, a := range attachments {
		if a.IsPrimary {
			ossKey = a.OSSKey
			break
		}
	}

	url, err := h.ossSvc.GeneratePresignDownloadURL(context.Background(), ossKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "OSS_ERROR", "message": "failed to generate download url"})
		return
	}

	// Async increment download count in Redis
	if h.rdb != nil {
		go func() {
			ctx := context.Background()
			h.rdb.ZIncrBy(ctx, "rank:download_counts", 1, fmt.Sprintf("%d", id))
		}()
	}

	c.Redirect(http.StatusFound, url)
}
