package handler

import (
	"net/http"
	"strconv"

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

	versions, err := h.versionSvc.ListVersions(contentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"versions": versions})
}

func (h *VersionHandler) GetVersion(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid version id"})
		return
	}

	content, err := h.versionSvc.GetVersionContent(id)
	if err != nil {
		if err == service.ErrVersionNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "version not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"content": content})
}
