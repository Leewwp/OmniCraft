package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// UsageGuideHandler serves the public usage-guide surface (SP-16 #447 /
// spec D4): an anonymous merged view (system template + content specifics)
// with ETag + s-maxage caching, plus the author-only studio write path.
type UsageGuideHandler struct {
	guideSvc    *service.UsageGuideService
	contentRepo *repository.ContentRepository
}

func NewUsageGuideHandler(guideSvc *service.UsageGuideService, contentRepo *repository.ContentRepository) *UsageGuideHandler {
	return &UsageGuideHandler{guideSvc: guideSvc, contentRepo: contentRepo}
}

// cacheControl is the anonymous read contract: shared caches may serve the
// merged view for 5 minutes; private caches for 1 minute; revalidation via
// ETag keeps specifics edits visible promptly.
const usageGuideCacheControl = "public, max-age=60, s-maxage=300"

func (h *UsageGuideHandler) GetGuide(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}

	// The guide is content-derived data: it follows the same visibility
	// gate as the content detail (#446 discipline) — author/admin keep
	// access to non-public content, everyone else needs the anonymous
	// visibility scope.
	content, err := h.contentRepo.FindByID(id)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	if content == nil || !h.guideVisibleToViewer(c, content) {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "content not found"})
		return
	}

	view, err := h.guideSvc.GetMergedView(c.Request.Context(), id, c.Query("locale"))
	if err != nil {
		h.mapUsageGuideError(c, err)
		return
	}

	c.Header("Cache-Control", usageGuideCacheControl)
	c.Header("ETag", view.ETag)
	if match := c.GetHeader("If-None-Match"); match != "" && etagMatches(match, view.ETag) {
		c.Status(http.StatusNotModified)
		return
	}
	c.JSON(http.StatusOK, view)
}

// SaveGuide is the studio write path (authReq + author check in service).
func (h *UsageGuideHandler) SaveGuide(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}
	var input service.UsageGuideInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	if err := h.guideSvc.SaveSpecifics(c.Request.Context(), middleware.GetUserID(c), id, input); err != nil {
		h.mapUsageGuideError(c, err)
		return
	}
	row, err := h.guideSvc.GetSpecifics(c.Request.Context(), middleware.GetUserID(c), id, input.Locale)
	if err != nil {
		h.mapUsageGuideError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"guide": row})
}

// GetAuthorGuide returns the author's saved specifics row for the studio
// editor initial state (authReq + author check).
func (h *UsageGuideHandler) GetAuthorGuide(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}
	row, err := h.guideSvc.GetSpecifics(c.Request.Context(), middleware.GetUserID(c), id, c.Query("locale"))
	if err != nil {
		h.mapUsageGuideError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"guide": row})
}

func (h *UsageGuideHandler) mapUsageGuideError(c *gin.Context, err error) {
	switch err {
	case service.ErrUsageGuideContentNotFound:
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "content not found"})
	case service.ErrUsageGuideForbidden:
		response.SafeErrorResponse(c, http.StatusForbidden, "FORBIDDEN", err)
	case service.ErrUsageGuideInvalidLocale, service.ErrUsageGuideInvalidPayload:
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
	default:
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
	}
}

// guideVisibleToViewer mirrors the content-detail gate minus the judge
// preview exemption: guidance is not case evidence, so under_review content
// serves its guide only to author/admin.
func (h *UsageGuideHandler) guideVisibleToViewer(c *gin.Context, content *model.ContentItem) bool {
	viewer := middleware.GetUserID(c)
	if viewer != 0 && viewer == content.AuthorID {
		return true
	}
	if middleware.IsAdmin(c) {
		return true
	}
	if content.Status != "published" || content.DeletedAt != nil {
		return false
	}
	return repository.ContentVisibleToViewer(h.contentRepo.DB(), content, viewer)
}

// etagMatches implements list matching for If-None-Match: an exact hit or
// any comma-separated member match wins; "*" always matches.
func etagMatches(header, etag string) bool {
	if header == "*" {
		return true
	}
	for _, part := range strings.Split(header, ",") {
		if part = strings.TrimSpace(part); part == etag || part == "W/"+etag {
			return true
		}
	}
	return false
}
