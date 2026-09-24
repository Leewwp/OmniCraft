package handler

import (
	"math"
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type VersionHandler struct {
	versionSvc *service.VersionService
}

// NewVersionHandler receives the container-owned VersionService (#658 PR-3：
// handler 不再自建服务图).
func NewVersionHandler(versionSvc *service.VersionService) *VersionHandler {
	return &VersionHandler{
		versionSvc: versionSvc,
	}
}

func (h *VersionHandler) ListVersions(c *gin.Context) {
	contentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}

	page, pageSize := pageQuery(c, 20)
	// #668：标准列表分页迁 pageQuery（缺省/无效/越界语义见 pagination.go）。

	// proposed versions are author-only in the list (FIX-21①); the whole
	// listing is content-visibility gated (#446).
	versions, total, err := h.versionSvc.ListVersionsPagedForViewer(contentID, page, pageSize, middleware.GetUserID(c), middleware.IsAdmin(c))
	if err != nil {
		if err == service.ErrContentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "content not found"})
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	c.JSON(http.StatusOK, gin.H{
		"versions":    versions,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": totalPages,
	})
}

func (h *VersionHandler) GetVersion(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid version id"})
		return
	}

	// participant gate: content author / proposed submitter / admin — the
	// version text of banned content must not leak to arbitrary readers
	// (F-056, FIX-21④).
	content, err := h.versionSvc.GetVersionForViewer(id, middleware.GetUserID(c), middleware.IsAdmin(c))
	if err != nil {
		switch err {
		case service.ErrVersionNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "version not found"})
		case service.ErrVersionForbidden:
			response.SafeErrorResponse(c, http.StatusForbidden, "FORBIDDEN", err)
		default:
			response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"content": content})
}
