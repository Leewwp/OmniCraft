package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CategoryHandler struct {
	catSvc *service.CategoryService
}

func NewCategoryHandler(db *gorm.DB) *CategoryHandler {
	return &CategoryHandler{
		catSvc: service.NewCategoryService(repository.NewCategoryRepository(db)),
	}
}

func (h *CategoryHandler) ListCategories(c *gin.Context) {
	zone := c.Query("zone")
	level := c.Query("level")
	var parentID *int64
	if p := c.Query("parent_id"); p != "" {
		if v, err := strconv.ParseInt(p, 10, 64); err == nil {
			parentID = &v
		}
	}
	cats, err := h.catSvc.GetCategories(zone, level, parentID)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"categories": cats})
}

func (h *CategoryHandler) AdminCreateCategory(c *gin.Context) {
	var cat model.Category
	if err := c.ShouldBindJSON(&cat); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	if err := h.catSvc.AdminCreateCategory(&cat); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"category": cat})
}

func (h *CategoryHandler) AdminUpdateCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid category id"})
		return
	}
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	if err := h.catSvc.AdminUpdateCategory(id, updates); err != nil {
		response.SafeErrorResponse(c, http.StatusBadRequest, "ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *CategoryHandler) AdminDeleteCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid category id"})
		return
	}
	if err := h.catSvc.AdminDeleteCategory(id); err != nil {
		if err == service.ErrCategoryHasChildren || err == service.ErrCategoryHasContent {
			response.SafeErrorResponse(c, http.StatusConflict, "CONFLICT", err)
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *CategoryHandler) AdminReorderCategories(c *gin.Context) {
	var updates []struct {
		ID        int64 `json:"id"`
		SortOrder int   `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	if err := h.catSvc.AdminReorderCategories(updates); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "reordered"})
}
