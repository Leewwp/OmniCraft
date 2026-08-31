package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/archivezip"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/redis/go-redis/v9"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ContentHandler struct {
	contentSvc        *service.ContentService
	contentRepo       *repository.ContentRepository
	seriesSvc         *service.SeriesService
	browseHistoryRepo *repository.BrowseHistoryRepository
	collectionRepo    *repository.CollectionRepository
	ossSvc            *service.OSSService
	ossInitErr        error
	uploadGrants      *service.UploadGrantService
	rdb               *redis.Client
	cfg               *config.Config
	queueProducer     queue.Producer
	archiveGate       *service.ArchiveScanGate
	displaySigner     *service.DisplayURLSigner
}

func NewContentHandler(db *gorm.DB, cfg *config.Config, rdb *redis.Client) *ContentHandler {
	repo := repository.NewContentRepository(db)
	ossSvc, ossErr := service.NewOSSService(cfg)
	reputSvc := service.NewReputationService(db)
	reviewSvc := service.NewReviewService(db, rdb, cfg, reputSvc)

	grantTTL := time.Duration(cfg.Feedback.UploadGrantTTLSec) * time.Second
	if grantTTL <= 0 {
		grantTTL = 5 * time.Minute
	}
	uploadGrants := service.NewUploadGrantService(rdb, grantTTL)
	contentSvc := service.NewContentServiceWithOSS(repo, reviewSvc, rdb, &cfg.Cache, ossSvc).
		WithUploadGrantService(uploadGrants).
		WithUploadedObjectVerifier(ossSvc).
		WithArchiveScanConfig(&cfg.ArchiveScan).
		WithImageDimensionsResolver(ossSvc).
		WithUploadConfig(&cfg.Upload)

	embeddingRepo := repository.NewEmbeddingRepository(db)
	recSvc := service.NewRecommendationService(db, embeddingRepo, repo, contentSvc, rdb, &cfg.Recommendation)
	contentSvc.SetRecommendationService(recSvc)

	return &ContentHandler{
		contentSvc:        contentSvc,
		contentRepo:       repo,
		seriesSvc:         service.NewSeriesService(repository.NewSeriesRepository(db)),
		browseHistoryRepo: repository.NewBrowseHistoryRepository(db),
		collectionRepo:    repository.NewCollectionRepository(db),
		ossSvc:            ossSvc,
		ossInitErr:        ossErr,
		uploadGrants:      uploadGrants,
		rdb:               rdb,
		cfg:               cfg,
		queueProducer:     queue.NewNoopProducer(),
		archiveGate:       service.NewArchiveScanGate(db, cfg.Features.ArchiveMalwareScanEnabled),
		displaySigner:     service.NewDisplayURLSigner(cfg),
	}
}

func (h *ContentHandler) SetQueueProducer(p queue.Producer) {
	h.queueProducer = p
}

// SetOutboxRepository wires the transactional outbox into the content
// service used by this handler. The route builder owns the shared repository
// instance so HTTP edits and the standalone relay observe the same rows.
func (h *ContentHandler) SetOutboxRepository(outbox repository.OutboxWriter) {
	h.contentSvc.SetOutboxRepository(outbox)
}

func (h *ContentHandler) SetArchiveScanRepository(repo *repository.ArchiveScanRepository) {
	h.contentSvc.SetArchiveScanRepository(repo, h.cfg.Features.ArchiveMalwareScanEnabled)
	h.archiveGate = service.NewArchiveScanGate(h.contentRepo.DB(), h.cfg.Features.ArchiveMalwareScanEnabled)
}

func (h *ContentHandler) GenerateOSSToken(c *gin.Context) {
	if h.ossSvc == nil {
		response.SafeErrorResponse(c, http.StatusServiceUnavailable, "OSS_NOT_CONFIGURED", h.ossInitErr)
		return
	}

	var req service.PresignUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request parameters")
		return
	}

	userID := middleware.GetUserID(c)
	resp, err := h.ossSvc.GeneratePresignUploadURL(c.Request.Context(), req, userID)
	if err != nil {
		var validationErr *service.UploadValidationError
		if errors.As(err, &validationErr) {
			response.ValidationError(c, "invalid request parameters")
			return
		}
		if errors.Is(err, service.ErrOSSNotConfigured) {
			response.SafeErrorResponse(c, http.StatusServiceUnavailable, "OSS_NOT_CONFIGURED", err)
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
		return
	}

	grant, err := h.uploadGrants.Issue(c.Request.Context(), service.UploadGrant{
		UserID:   userID,
		Purpose:  "content",
		OSSKey:   resp.OSSKey,
		FileType: req.FileType,
		MimeType: req.MimeType,
		FileSize: req.FileSize,
	})
	if err != nil {
		response.SafeErrorResponse(c, http.StatusServiceUnavailable, "UPLOAD_GRANT_UNAVAILABLE", err)
		return
	}
	resp.GrantID = grant.ID

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
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	h.displaySigner.DecorateContents(contents)
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
		response.ValidationError(c, "invalid request parameters")
		return
	}

	content, err := h.contentSvc.PublishContentWithContext(c.Request.Context(), input, callerID)
	if err != nil {
		if errors.Is(err, service.ErrPublishFrozen) {
			response.SafeErrorResponse(c, http.StatusForbidden, "PUBLISH_FROZEN", err)
			return
		}
		if errors.Is(err, service.ErrSourceNotAllowedForOriginal) {
			response.SafeErrorResponse(c, http.StatusBadRequest, "SOURCE_NOT_ALLOWED_FOR_ORIGINAL", err)
			return
		}
		if errors.Is(err, service.ErrFanworkSourceRequired) {
			response.SafeErrorResponse(c, http.StatusBadRequest, "FANWORK_SOURCE_REQUIRED", err)
			return
		}
		if errors.Is(err, service.ErrMultipleSourceConflict) {
			response.SafeErrorResponse(c, http.StatusBadRequest, "MULTIPLE_SOURCE_CONFLICT", err)
			return
		}
		if errors.Is(err, service.ErrSourceOriginalUnavailable) {
			response.SafeErrorResponse(c, http.StatusBadRequest, "SOURCE_ORIGINAL_UNAVAILABLE", err)
			return
		}
		if errors.Is(err, service.ErrSourceFanworkUnavailable) {
			response.SafeErrorResponse(c, http.StatusBadRequest, "SOURCE_FANWORK_UNAVAILABLE", err)
			return
		}
		if errors.Is(err, service.ErrUploadGrantInvalid) {
			response.SafeErrorResponse(c, http.StatusBadRequest, "UPLOAD_GRANT_INVALID", err)
			return
		}
		if errors.Is(err, service.ErrMediaSetInvalid) {
			response.SafeErrorResponse(c, http.StatusBadRequest, "MEDIA_SET_INVALID", err)
			return
		}
		if errors.Is(err, service.ErrArchiveAttachmentRequired) {
			response.Error(c, http.StatusBadRequest, "ARCHIVE_ATTACHMENT_REQUIRED", "mod content requires a zip archive attachment")
			return
		}
		if errors.Is(err, archivezip.ErrEncrypted) {
			response.Error(c, http.StatusBadRequest, "ARCHIVE_ENCRYPTED", "archive is encrypted")
			return
		}
		if errors.Is(err, archivezip.ErrPathInvalid) {
			response.Error(c, http.StatusBadRequest, "ARCHIVE_PATH_INVALID", "archive path is invalid")
			return
		}
		if errors.Is(err, archivezip.ErrLinkForbidden) {
			response.Error(c, http.StatusBadRequest, "ARCHIVE_LINK_FORBIDDEN", "archive link is forbidden")
			return
		}
		if errors.Is(err, archivezip.ErrLimitExceeded) {
			response.Error(c, http.StatusBadRequest, "ARCHIVE_LIMIT_EXCEEDED", "archive limits exceeded")
			return
		}
		if errors.Is(err, archivezip.ErrInvalid) {
			response.Error(c, http.StatusBadRequest, "ARCHIVE_INVALID", "archive is invalid")
			return
		}
		if errors.Is(err, service.ErrArchiveScanUnavailable) {
			response.Error(c, http.StatusServiceUnavailable, "ARCHIVE_SCAN_UNAVAILABLE", "archive scanning is unavailable")
			return
		}
		if errors.Is(err, service.ErrArchiveScanFailed) {
			response.Error(c, http.StatusConflict, "ARCHIVE_SCAN_FAILED", "archive scan failed")
			return
		}
		if errors.Is(err, service.ErrArchiveScanPending) {
			response.Error(c, http.StatusConflict, "ARCHIVE_SCAN_PENDING", "archive scan is pending")
			return
		}
		if errors.Is(err, service.ErrUploadGrantUnavailable) {
			response.SafeErrorResponse(c, http.StatusServiceUnavailable, "UPLOAD_GRANT_UNAVAILABLE", err)
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
		return
	}

	h.displaySigner.DecorateContent(content)
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
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
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
	h.displaySigner.DecorateContent(content)
	h.displaySigner.DecorateAttachments(attachments)
	tags, err2 := h.contentRepo.GetTags(id)
	if err2 != nil {
		slog.Error("failed to get tags", "content_id", id, "error", err2)
		tags = nil
	}
	seriesMemberships, err := h.seriesSvc.ListMembershipsForContent(c.Request.Context(), id)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	resp := gin.H{
		"content":            content,
		"attachments":        attachments,
		"tags":               tags,
		"series_memberships": seriesMemberships,
	}

	// 收藏状态以收藏成员关系为唯一事实源 —— 当前用户至少一个活动收藏集
	// 包含该内容即为已收藏；已删除收藏集不计入。
	currentUserID := middleware.GetUserID(c)
	if currentUserID > 0 {
		memberships, err := h.collectionRepo.CountActiveMembershipsForContent(c.Request.Context(), currentUserID, id)
		if err != nil {
			slog.Error("failed to count collection memberships", "user_id", currentUserID, "content_id", id, "error", err)
			memberships = 0
		}
		resp["is_favorited"] = memberships > 0
	}

	if content.SourceOriginalID != nil && *content.SourceOriginalID > 0 {
		source, srcErr := h.contentSvc.GetContent(*content.SourceOriginalID)
		if srcErr == nil && source != nil {
			resp["source_original"] = gin.H{"id": source.ID, "title": source.Title, "zone": "original"}
		}
	}
	if content.SourceFanworkID != nil && *content.SourceFanworkID > 0 {
		source, srcErr := h.contentSvc.GetContent(*content.SourceFanworkID)
		if srcErr == nil && source != nil {
			resp["source_fanwork"] = gin.H{"id": source.ID, "title": source.Title, "zone": "fanwork"}
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

	viewerID := middleware.GetUserID(c)
	source, err := h.contentSvc.GetVisibleContent(sourceID, viewerID)
	if err != nil {
		if errors.Is(err, service.ErrContentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "content not found"})
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	var sourceOriginalID, sourceFanworkID *int64
	switch source.Zone {
	case "original":
		sourceOriginalID = &sourceID
	case "fanwork":
		sourceFanworkID = &sourceID
	default:
		// Zones are constrained by the publish contract; any other zone has
		// no source-linkage children.
	}

	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("page_size"), 0)
	if pageSize == 0 {
		pageSize = parsePositiveInt(c.Query("limit"), 24)
	}
	if pageSize > 100 {
		pageSize = 100
	}

	contentTypes, ok := parseRelatedFanworkContentTypes(c.Query("content_type"))
	if !ok {
		response.Error(c, http.StatusBadRequest, "INVALID_CONTENT_TYPE", "invalid content type")
		return
	}

	sort := "hot"
	switch c.Query("sort") {
	case "", "hot":
		sort = "hot"
	case "new", "newest":
		sort = "new"
	default:
		// Legacy sorts (most_views, best_rated) pass through to the
		// repository ordering; anything unknown falls back to newest.
		sort = c.Query("sort")
	}

	contents, total, err := h.contentSvc.ListContents(repository.ListContentsFilter{
		Zone:             "fanwork",
		SourceOriginalID: sourceOriginalID,
		SourceFanworkID:  sourceFanworkID,
		ContentTypes:     contentTypes,
		Sort:             sort,
		TimeRange:        c.DefaultQuery("time_range", "all"),
		Page:             page,
		PageSize:         pageSize,
		ViewerID:         viewerID,
	}, viewerID)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	h.displaySigner.DecorateContents(contents)
	c.JSON(http.StatusOK, gin.H{
		"source_content_id": sourceID,
		"source_zone":       source.Zone,
		"contents":          contents,
		"total":             total,
		"page":              page,
		"page_size":         pageSize,
	})
}

var validRelatedFanworkContentTypes = map[string]bool{
	"image": true, "article": true, "video": true, "audio": true,
	"template": true, "sheet_music": true, "mod": true, "prompt": true, "other": true,
}

// parseRelatedFanworkContentTypes parses the comma-separated content_type
// query for related-fanworks. Every entry must be a standard allowlisted
// content type; empty or unknown entries reject the request so the caller
// can answer 400 INVALID_CONTENT_TYPE.
func parseRelatedFanworkContentTypes(raw string) ([]string, bool) {
	if raw == "" {
		return nil, true
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" || !validRelatedFanworkContentTypes[value] {
			return nil, false
		}
		values = append(values, value)
	}
	return values, true
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
		Title            *string `json:"title"`
		CoverImageURL    *string `json:"cover_image_url"`
		IsPublic         *bool   `json:"is_public"`
		AllowCopy        *bool   `json:"allow_copy"`
		AgentEnabled     *bool   `json:"agent_enabled"`
		IPID             *int64  `json:"ip_id"`
		SourceOriginalID *int64  `json:"source_original_id"`
		SourceFanworkID  *int64  `json:"source_fanwork_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.SafeErrorResponse(c, http.StatusBadRequest, "INVALID_BODY", err)
		return
	}

	if req.IPID != nil || req.SourceOriginalID != nil || req.SourceFanworkID != nil {
		response.SafeErrorResponse(c, http.StatusBadRequest, "SOURCE_IMMUTABLE", service.ErrSourceImmutable)
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

	if err := h.contentSvc.UpdateContentWithContext(c.Request.Context(), id, callerID, updates); err != nil {
		if err == service.ErrContentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "content not found"})
			return
		}
		if err == service.ErrContentForbidden {
			c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "not content author"})
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
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

	if err := h.contentSvc.DeleteContentWithContext(c.Request.Context(), id, callerID); err != nil {
		if err == service.ErrContentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "content not found"})
			return
		}
		if err == service.ErrContentForbidden {
			c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "not content author"})
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
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

	var visibleCount int64
	if err := repository.ApplyContentVisibilityScope(h.contentRepo.DB().Model(&model.ContentItem{}), callerID).
		Where("content_items.id = ?", id).
		Count(&visibleCount).Error; err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	if visibleCount == 0 {
		c.JSON(http.StatusForbidden, gin.H{"code": "CONTENT_UNAVAILABLE", "message": "content is unavailable"})
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

	var target *model.ContentAttachment
	attachmentIDStr := c.Query("attachment_id")
	if attachmentIDStr != "" {
		attachmentID, err := strconv.ParseInt(attachmentIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ATTACHMENT_ID", "message": "invalid attachment_id"})
			return
		}
		for i := range attachments {
			if attachments[i].ID == attachmentID {
				target = &attachments[i]
				break
			}
		}
		if target == nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "ATTACHMENT_MISMATCH", "message": "attachment does not belong to this content"})
			return
		}
	} else {
		var primaries []int
		for i := range attachments {
			if attachments[i].IsPrimary != nil && *attachments[i].IsPrimary {
				primaries = append(primaries, i)
			}
		}
		if len(primaries) == 1 {
			target = &attachments[primaries[0]]
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"code": "AMBIGUOUS_ATTACHMENT", "message": "specify attachment_id; cannot determine a unique primary attachment"})
			return
		}
	}

	if h.archiveGate != nil {
		if err := h.archiveGate.RequireAttachmentClean(c.Request.Context(), target.ID); err != nil {
			if errors.Is(err, service.ErrArchiveNotClean) {
				response.Error(c, http.StatusForbidden, "ARCHIVE_NOT_CLEAN", "archive is not clean")
				return
			}
			response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
			return
		}
	}

	ttl := downloadURLTTL(h.cfg, *target)

	url, err := h.ossSvc.GeneratePresignDownloadURL(context.Background(), target.OSSKey, ttl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "OSS_ERROR", "message": "failed to generate download url"})
		return
	}

	if _, ok := h.queueProducer.(*queue.NoopProducer); !ok && h.queueProducer != nil {
		recovery.GoSafe(func() {
			payload, _ := json.Marshal(map[string]interface{}{
				"content_id": id,
				"action":     "download",
			})
			if err := h.queueProducer.Publish(context.Background(), "count.download", payload); err != nil {
				slog.Error("failed to publish download count message", "content_id", id, "error", err)
			}
		})
	} else if h.rdb != nil {
		recovery.GoSafe(func() {
			ctx := context.Background()
			h.rdb.ZIncrBy(ctx, "rank:download:counts", 1, fmt.Sprintf("%d", id))
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"download_url": url,
		"expires_in":   int64(ttl.Seconds()),
	})
}

func downloadURLTTL(cfg *config.Config, attachment model.ContentAttachment) time.Duration {
	ttlSec := 300
	if cfg != nil {
		ttlSec = cfg.OSS.DownloadURLTTL
		if cfg.Features.ArchiveMalwareScanEnabled && (attachment.FileType == "mod" || attachment.ScanRequired) {
			ttlSec = cfg.ArchiveScan.URLTTLSec
			if ttlSec <= 0 || ttlSec > 300 {
				ttlSec = 300
			}
		}
	}
	if ttlSec <= 0 {
		ttlSec = 300
	}
	return time.Duration(ttlSec) * time.Second
}
