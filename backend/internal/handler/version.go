package handler

import (
	"math"
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type VersionHandler struct {
	versionSvc *service.VersionService
}

func NewVersionHandler(db *gorm.DB) *VersionHandler {
	return &VersionHandler{
		versionSvc: service.NewVersionService(
			repository.NewVersionRepository(db),
			repository.NewContentRepository(db),
		),
	}
}

func (h *VersionHandler) ListVersions(c *gin.Context) {
	contentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
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

	// proposed versions are author-only in the list (FIX-21①).
	versions, total, err := h.versionSvc.ListVersionsPagedForViewer(contentID, page, pageSize, middleware.GetUserID(c))
	if err != nil {
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
