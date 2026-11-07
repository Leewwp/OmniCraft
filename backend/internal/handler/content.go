package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/archivezip"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
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
	cfg               *config.Config
	archiveGate       *service.ArchiveScanGate
	displaySigner     *service.DisplayURLSigner
	judgeRepo         *repository.JudgeRepository
	followRepo        *repository.FollowRepository
}

// ContentHandlerDeps carries the container-resolved modules the content
// surface needs (#658 PR-3：构造全部收口组合根，handler 只消费).
type ContentHandlerDeps struct {
	ContentService *service.ContentService
	OSS            *service.OSSService
	OSSErr         error
	UploadGrants   *service.UploadGrantService
	ContentRepo    *repository.ContentRepository
	JudgeRepo      *repository.JudgeRepository
	FollowRepo     *repository.FollowRepository
	SeriesSvc      *service.SeriesService
	BrowseHistory  *repository.BrowseHistoryRepository
	CollectionRepo *repository.CollectionRepository
	ArchiveGate    *service.ArchiveScanGate
	DisplaySigner  *service.DisplayURLSigner
}

// NewContentHandler consumes the container-owned studio stack (the
// full-featured ContentService shared with the MCP write channel, the shared
// OSS presign service — nil + OSSErr = unconfigured, per-surface 503 — and
// the shared upload-grant store) plus the repos it reads directly. Tests
// compose the same wiring through the handler test seam / testutil studio
// stack.
func NewContentHandler(cfg *config.Config, deps ContentHandlerDeps) *ContentHandler {
	return &ContentHandler{
		contentSvc:        deps.ContentService,
		contentRepo:       deps.ContentRepo,
		judgeRepo:         deps.JudgeRepo,
		followRepo:        deps.FollowRepo,
		seriesSvc:         deps.SeriesSvc,
		browseHistoryRepo: deps.BrowseHistory,
		collectionRepo:    deps.CollectionRepo,
		ossSvc:            deps.OSS,
		ossInitErr:        deps.OSSErr,
		uploadGrants:      deps.UploadGrants,
		cfg:               cfg,
		archiveGate:       deps.ArchiveGate,
		displaySigner:     deps.DisplaySigner,
	}
}

// contentPresignErrMap 是 POST /contents/oss-token（GenerateOSSToken）经
// 动态校验分流（errors.As UploadValidationError 分支留在 handler——错误
// 码随错误体变化，非 sentinel 映射）后的 sentinel→HTTP 合同；未知错误兜
// 底保持历史 Safe 500 INTERNAL_ERROR。
var contentPresignErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrOSSNotConfigured, status: http.StatusServiceUnavailable, code: "OSS_NOT_CONFIGURED"},
	},
	fallback: fallbackInternal,
}

// contentDetailErrMap 是内容详情读侧（GET /contents/:id GetContent 与
// GET /contents/:id/related-fanworks ListRelatedFanworks）的 sentinel→HTTP
// 合同：两路由迁移前一个用 `==` 直比、一个用 errors.Is，映射面一致
// （service 裸返 sentinel），共用一份声明；兜底保持历史 Safe 500
// DB_ERROR。
var contentDetailErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrContentNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "content not found"},
	},
	fallback: fallbackDB,
}

// updateContentErrMap 是 PATCH /contents/:id（UpdateContent）的
// sentinel→HTTP 合同，规则序与迁移前分支序逐条一致（ErrCoverNotPlatform
// OSSObject 迁移前走 ValidationError 形态 = 固定 400 VALIDATION_ERROR 文
// 案，规则化后逐字节相同）；兜底保持历史 Safe 500 DB_ERROR。
var updateContentErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrContentNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "content not found"},
		{sentinel: service.ErrContentForbidden, status: http.StatusForbidden, code: "FORBIDDEN", message: "not content author"},
		// banned 终态禁改（FIX-13）：删除仍允许，编辑引导走申诉。
		{sentinel: service.ErrContentBanned, status: http.StatusForbidden, code: "CONTENT_BANNED", message: "content is banned and cannot be edited"},
		{sentinel: service.ErrCoverNotPlatformOSSObject, status: http.StatusBadRequest, code: "VALIDATION_ERROR", message: "cover image must be a platform OSS object"},
	},
	fallback: fallbackDB,
}

// deleteContentErrMap 是 DELETE /contents/:id（DeleteContent）的
// sentinel→HTTP 合同，规则序与迁移前分支序逐条一致；兜底保持历史 Safe
// 500 DB_ERROR。
var deleteContentErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrContentNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "content not found"},
		{sentinel: service.ErrContentForbidden, status: http.StatusForbidden, code: "FORBIDDEN", message: "not content author"},
	},
	fallback: fallbackDB,
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
			// T25（FIX-41）：超限走专用码（前端出专属文案），其余参数错误保持通用码。
			// 动态码随错误体变化（非 sentinel 映射），留在 handler。
			if validationErr.Code != "" {
				response.Error(c, http.StatusBadRequest, validationErr.Code, validationErr.Message)
				return
			}
			response.ValidationError(c, "invalid request parameters")
			return
		}
		// C3.4：OSS 未配置与其余未知错误收口为 contentPresignErrMap。
		contentPresignErrMap.write(c, err)
		return
	}

	grant, err := h.uploadGrants.Issue(c.Request.Context(), service.UploadGrant{
		UserID:   userID,
		Purpose:  "content",
		OSSKey:   resp.OSSKey,
		FileType: req.FileType,
		MimeType: req.MimeType,
		FileSize: req.FileSize,
		// #688: the server locks the normalized name into the grant at
		// presign time; the later publish payload's file_name is never
		// trusted for this.
		OriginalFileName: service.NormalizeUploadFileName(req.FileName),
	})
	if err != nil {
		response.SafeErrorResponse(c, http.StatusServiceUnavailable, "UPLOAD_GRANT_UNAVAILABLE", err)
		return
	}
	resp.GrantID = grant.ID

	c.JSON(http.StatusOK, resp)
}

func (h *ContentHandler) ListContents(c *gin.Context) {
	page, pageSize := pageQuery(c, 20)
	// #668：标准列表分页迁 pageQuery（缺省/无效/越界语义见 pagination.go）。

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

// publishErrMap 是 POST /contents（CreateContent）的 sentinel→HTTP 合同，
// 顺序与迁移前 content.go 分支顺序逐条一致；未知错误兜底保持历史 500
// INTERNAL_ERROR（与下载端点的 DB_ERROR 兜底不同）。
var publishErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrPublishFrozen, status: http.StatusForbidden, code: "PUBLISH_FROZEN"},
		{sentinel: service.ErrSourceNotAllowedForOriginal, status: http.StatusBadRequest, code: "SOURCE_NOT_ALLOWED_FOR_ORIGINAL"},
		{sentinel: service.ErrFanworkSourceRequired, status: http.StatusBadRequest, code: "FANWORK_SOURCE_REQUIRED"},
		{sentinel: service.ErrMultipleSourceConflict, status: http.StatusBadRequest, code: "MULTIPLE_SOURCE_CONFLICT"},
		{sentinel: service.ErrSourceOriginalUnavailable, status: http.StatusBadRequest, code: "SOURCE_ORIGINAL_UNAVAILABLE"},
		{sentinel: service.ErrSourceFanworkUnavailable, status: http.StatusBadRequest, code: "SOURCE_FANWORK_UNAVAILABLE"},
		{sentinel: service.ErrUploadGrantInvalid, status: http.StatusBadRequest, code: "UPLOAD_GRANT_INVALID"},
		{sentinel: service.ErrMediaSetInvalid, status: http.StatusBadRequest, code: "MEDIA_SET_INVALID"},
		{sentinel: service.ErrArchiveAttachmentRequired, status: http.StatusBadRequest, code: "ARCHIVE_ATTACHMENT_REQUIRED", message: "mod content requires a zip archive attachment"},
		// #690：注册表 attachment_policy（required_any_of）违规面——任何
		// 配置了 policy 的类型共用，非 3d_print 专属分支。
		{sentinel: service.ErrAttachmentPolicyRequired, status: http.StatusBadRequest, code: "ATTACHMENT_POLICY_REQUIRED", message: "content type requires at least one attachment of the required family"},
		{sentinel: archivezip.ErrEncrypted, status: http.StatusBadRequest, code: "ARCHIVE_ENCRYPTED", message: "archive is encrypted"},
		{sentinel: archivezip.ErrPathInvalid, status: http.StatusBadRequest, code: "ARCHIVE_PATH_INVALID", message: "archive path is invalid"},
		{sentinel: archivezip.ErrLinkForbidden, status: http.StatusBadRequest, code: "ARCHIVE_LINK_FORBIDDEN", message: "archive link is forbidden"},
		{sentinel: archivezip.ErrLimitExceeded, status: http.StatusBadRequest, code: "ARCHIVE_LIMIT_EXCEEDED", message: "archive limits exceeded"},
		{sentinel: archivezip.ErrInvalid, status: http.StatusBadRequest, code: "ARCHIVE_INVALID", message: "archive is invalid"},
		{sentinel: service.ErrArchiveScanUnavailable, status: http.StatusServiceUnavailable, code: "ARCHIVE_SCAN_UNAVAILABLE", message: "archive scanning is unavailable"},
		{sentinel: service.ErrArchiveScanFailed, status: http.StatusConflict, code: "ARCHIVE_SCAN_FAILED", message: "archive scan failed"},
		{sentinel: service.ErrArchiveScanPending, status: http.StatusConflict, code: "ARCHIVE_SCAN_PENDING", message: "archive scan is pending"},
		{sentinel: service.ErrUploadGrantUnavailable, status: http.StatusServiceUnavailable, code: "UPLOAD_GRANT_UNAVAILABLE"},
	},
	fallback: fallbackInternal,
}

func (h *ContentHandler) CreateContent(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
		return
	}

	var input service.PublishContentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	content, err := h.contentSvc.PublishContentWithContext(c.Request.Context(), input, callerID)
	if err != nil {
		// #794：sentinel→HTTP 合同集中在文件内 publishErrMap（按原分支
		// 顺序 errors.Is 首个命中）；未知错误兜底保持历史 500
		// INTERNAL_ERROR。
		publishErrMap.write(c, err)
		return
	}

	h.displaySigner.DecorateContent(content)
	c.JSON(http.StatusCreated, gin.H{"content": content})
}

func (h *ContentHandler) GetContent(c *gin.Context) {
	id, ok := pathID(c, "id", "invalid content id")
	if !ok {
		return
	}

	content, err := h.contentSvc.GetContent(id)
	if err != nil {
		// C3.4：迁移前 `==` 直比变体收口为 contentDetailErrMap（与
		// ListRelatedFanworks 的 errors.Is 链映射面一致，共用一份声明）。
		contentDetailErrMap.write(c, err)
		return
	}

	userID := middleware.GetUserID(c)

	// 内容可见性统一口径（FIX-12+43）：非 published / 私密 / 封禁作者的内容
	// 仅作者与 admin 可读；判官读豁免钩子留待 T40（FIX-36d）。
	if !h.contentVisibleToViewer(content, c) {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "content not found")
		return
	}

	// view_count 仅对 published 且非作者本人计数。
	if content.Status == "published" && userID != content.AuthorID {
		if err := h.contentRepo.IncrViewCount(id); err != nil {
			slog.Error("failed to incr view count", "content_id", id, "error", err)
		}
		h.contentSvc.IncrViewCount(id)
	}
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
	// #688 AttachmentScanGate 预览门：详情路径与下载门同源同判——非 clean
	// 不签发 oss_url（前端渲染扫描状态卡）；scannable 族群走 scan-aware
	// 短 TTL 签名（cap 300s、无 bucket 对齐）。
	h.displaySigner.ScanAwareDecorateAttachments(c.Request.Context(), attachments, h.archiveGate, h.cfg.ArchiveScan.URLTTLSec)

	// SP-17/T1：作者关注态按 viewer 附加（同 is_favorited 的缓存外模式）——
	// 详情内容行有 Redis 整行缓存，登录视角字段禁止进缓存。匿名（user_id=0）
	// 不设置，序列化时 omit，不破坏 CacheableAnonymousGET 的匿名缓存语义。
	if userID > 0 && content.Author.ID > 0 {
		isFollowing, err := h.followRepo.IsFollowing(userID, "user", content.Author.ID)
		if err != nil {
			slog.Error("failed to check author is_following", "viewer_id", userID, "author_id", content.Author.ID, "error", err)
		} else {
			content.Author.IsFollowing = &isFollowing
		}
	}

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

	contentPayload := any(content)
	// ban_reason 仅作者/admin 可见（FIX-16）：model 层不序列化，此处按 viewer 附加。
	if userID == content.AuthorID || isAdminRole(c) {
		m, err := contentWithBanReason(content)
		if err != nil {
			response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
			return
		}
		contentPayload = m
	}

	resp := gin.H{
		"content":            contentPayload,
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
		if id, title, ok := h.visibleSourceLite(c, *content.SourceOriginalID); ok {
			resp["source_original"] = gin.H{"id": id, "title": title, "zone": "original"}
		}
	}
	if content.SourceFanworkID != nil && *content.SourceFanworkID > 0 {
		if id, title, ok := h.visibleSourceLite(c, *content.SourceFanworkID); ok {
			resp["source_fanwork"] = gin.H{"id": id, "title": title, "zone": "fanwork"}
		}
	}
	c.JSON(http.StatusOK, resp)
}

// visibleSourceLite returns the id+title of a source reference only when the
// source itself is visible to the caller (#446 / SP-16 P0): a published
// fanwork must not leak the title of a later-banned or privated source.
func (h *ContentHandler) visibleSourceLite(c *gin.Context, sourceID int64) (int64, string, bool) {
	type sourceRow struct {
		ID    int64
		Title string
	}
	var row sourceRow
	visSQL, visArgs := repository.ContentVisibilitySQL(middleware.GetUserID(c))
	err := h.contentRepo.DB().Model(&model.ContentItem{}).
		Select("id", "title").
		Where("id = ?", sourceID).
		Where(visSQL, visArgs...).
		Take(&row).Error
	if err != nil {
		return 0, "", false
	}
	return row.ID, row.Title, true
}

func (h *ContentHandler) ListRelatedFanworks(c *gin.Context) {
	sourceID, ok := pathID(c, "id", "invalid content id")
	if !ok {
		return
	}

	viewerID := middleware.GetUserID(c)
	source, err := h.contentSvc.GetVisibleContent(sourceID, viewerID)
	if err != nil {
		// C3.4：迁移前 errors.Is 链收口为 contentDetailErrMap（与 GetContent
		// 映射面一致，共用一份声明）。
		contentDetailErrMap.write(c, err)
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

	contentTypes, ok := parseRelatedFanworkContentTypes(c.Query("content_type"), h.cfg)
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

// parseRelatedFanworkContentTypes parses the comma-separated content_type
// query for related-fanworks. Every entry must be a registry-registered
// content type (#687 taxonomy axis); empty or unknown entries reject the
// request so the caller can answer 400 INVALID_CONTENT_TYPE.
func parseRelatedFanworkContentTypes(raw string, cfg *config.Config) ([]string, bool) {
	if raw == "" {
		return nil, true
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" || !cfg.IsRegisteredContentType(value) {
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
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
		return
	}

	id, ok := pathID(c, "id", "invalid content id")
	if !ok {
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
		response.Error(c, http.StatusBadRequest, "NO_FIELDS", "no fields to update")
		return
	}

	if err := h.contentSvc.UpdateContentWithContext(c.Request.Context(), id, callerID, updates); err != nil {
		// C3.4：迁移前 `==` 直比四分支收口为 updateContentErrMap，规则序
		// 与分支序一致，wire 逐字节不变。
		updateContentErrMap.write(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *ContentHandler) DeleteContent(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
		return
	}

	id, ok := pathID(c, "id", "invalid content id")
	if !ok {
		return
	}

	if err := h.contentSvc.DeleteContentWithContext(c.Request.Context(), id, callerID); err != nil {
		// C3.4：迁移前 `==` 直比两分支收口为 deleteContentErrMap。
		deleteContentErrMap.write(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// downloadErrMap 是 GET /contents/:id/download（DownloadContent）的
// sentinel→HTTP 合同，顺序与迁移前 content.go 分支顺序逐条一致；未知错
// 误兜底保持历史 500 DB_ERROR。该合同同时是 SP-16 #451 MCP 工具
// omnicraft_request_download 的 REST 面投影，语义不得分叉。
var downloadErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrDownloadUnauthorized, status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "login required"},
		{sentinel: service.ErrContentNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "content not found"},
		{sentinel: service.ErrDownloadNotPublished, status: http.StatusForbidden, code: "FORBIDDEN", message: "content not available for download"},
		{sentinel: service.ErrDownloadUnavailable, status: http.StatusForbidden, code: "CONTENT_UNAVAILABLE", message: "content is unavailable"},
		{sentinel: service.ErrDownloadNotAllowed, status: http.StatusForbidden, code: "FORBIDDEN", message: "download not allowed"},
		{sentinel: service.ErrOSSNotConfigured, status: http.StatusServiceUnavailable, code: "OSS_NOT_CONFIGURED", message: "oss service not configured"},
		{sentinel: service.ErrNoAttachments, status: http.StatusNotFound, code: "NO_ATTACHMENTS", message: "no downloadable files"},
		{sentinel: service.ErrInvalidAttachmentID, status: http.StatusBadRequest, code: "INVALID_ATTACHMENT_ID", message: "invalid attachment_id"},
		{sentinel: service.ErrAttachmentMismatch, status: http.StatusBadRequest, code: "ATTACHMENT_MISMATCH", message: "attachment does not belong to this content"},
		{sentinel: service.ErrAmbiguousAttachment, status: http.StatusBadRequest, code: "AMBIGUOUS_ATTACHMENT", message: "specify attachment_id; cannot determine a unique primary attachment"},
		{sentinel: service.ErrArchiveNotClean, status: http.StatusForbidden, code: "ARCHIVE_NOT_CLEAN", message: "archive is not clean"},
		{sentinel: service.ErrDownloadPresignFailed, status: http.StatusInternalServerError, code: "OSS_ERROR", message: "failed to generate download url"},
	},
	fallback: fallbackDB,
}

func (h *ContentHandler) DownloadContent(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
		return
	}

	id, ok := pathID(c, "id", "invalid content id")
	if !ok {
		return
	}

	// SP-16 #451: the orchestration lives in ContentService.RequestDownload
	// so the MCP tool omnicraft_request_download shares byte-identical
	// semantics; this handler only maps sentinel errors onto the historical
	// HTTP contract.
	res, err := h.contentSvc.RequestDownload(c.Request.Context(), callerID, id, c.Query("attachment_id"))
	if err != nil {
		// #794：sentinel→HTTP 合同集中在文件内 downloadErrMap（按原分
		// 支顺序 errors.Is 首个命中）；未知错误兜底保持历史 500 DB_ERROR。
		downloadErrMap.write(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"download_url": res.URL,
		"expires_in":   res.ExpiresIn,
	})
}

// isAdminRole reports whether the caller holds the admin role.
func isAdminRole(c *gin.Context) bool {
	role, exists := c.Get(middleware.UserRoleKey)
	return exists && role == "admin"
}

// contentWithBanReason marshals a content item (ban_reason hidden at model
// level) and re-attaches the field explicitly for the author/admin view.
func contentWithBanReason(content *model.ContentItem) (map[string]any, error) {
	payload, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}
	m := map[string]any{}
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, err
	}
	if content.BanReason != "" {
		m["ban_reason"] = content.BanReason
	}
	return m, nil
}

// contentVisibleToViewer enforces the unified content visibility rule at the
// detail boundary: author and admin see everything; everyone else only
// published, public content from a non-banned, non-deleted author.
func (h *ContentHandler) contentVisibleToViewer(content *model.ContentItem, c *gin.Context) bool {
	viewer := middleware.GetUserID(c)
	if viewer != 0 && viewer == content.AuthorID {
		return true
	}
	if role, exists := c.Get(middleware.UserRoleKey); exists && role == "admin" {
		return true
	}
	// T40（FIX-36d）：持对应类型有效资格的判官可读 under_review 内容（审案
	// 预览）。banned/私密不在此豁免范围内，照旧 404。
	if viewer != 0 && content.Status == "under_review" {
		if qualified, err := h.judgeRepo.CheckQualification(viewer, content.ContentType); err == nil && qualified {
			return true
		}
	}
	// #446/SP-16 P0：与列表口径对齐——封禁 IP 下的内容详情不再对非参与方
	// 可读（此前仅列表过滤，直连详情仍可读）。
	if content.IPID != nil && *content.IPID > 0 && h.contentIPBanned(*content.IPID) {
		return false
	}
	return content.Status == "published" &&
		content.IsPublic &&
		!content.Author.IsBanned &&
		content.Author.DeletedAt == nil
}

// contentIPBanned reports whether the IP a content row hangs under is banned.
func (h *ContentHandler) contentIPBanned(ipID int64) bool {
	var status string
	if err := h.contentRepo.DB().Model(&model.IP{}).
		Where("id = ?", ipID).
		Pluck("status", &status).Error; err != nil {
		return false
	}
	return status == "banned"
}
