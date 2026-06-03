package handler

import (
	"errors"
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CategoryHandler struct {
	catSvc   *service.CategoryService
	auditSvc *service.AdminAuditService
	db       *gorm.DB
}

func NewCategoryHandler(db *gorm.DB, auditSvc *service.AdminAuditService) *CategoryHandler {
	return &CategoryHandler{
		catSvc:   service.NewCategoryService(repository.NewCategoryRepository(db)),
		auditSvc: auditSvc,
		db:       db,
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
	entry := h.auditEntry(c, "category_create", "category", "", map[string]any{"name": cat.Slug, "slug": cat.Slug, "display_order": cat.SortOrder})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		if err := tx.Create(&cat).Error; err != nil {
			return err
		}
		entry.TargetID = strconv.FormatInt(cat.ID, 10)
		return nil
	}); err != nil {
		if respondCategoryAuditError(c, err) {
			return
		}
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
	entry := h.auditEntry(c, "category_update", "category", strconv.FormatInt(id, 10), map[string]any{"category_id": id})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		var cat model.Category
		if err := tx.First(&cat, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return service.ErrCategoryNotFound
			}
			return err
		}
		return tx.Model(&model.Category{}).Where("id = ?", id).Updates(updates).Error
	}); err != nil {
		if respondCategoryAuditError(c, err) {
			return
		}
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
	entry := h.auditEntry(c, "category_delete", "category", strconv.FormatInt(id, 10), map[string]any{"category_id": id})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		txRepo := repository.NewCategoryRepository(tx)
		txSvc := service.NewCategoryService(txRepo)
		return txSvc.AdminDeleteCategory(id)
	}); err != nil {
		if respondCategoryAuditError(c, err) {
			return
		}
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
	entry := h.auditEntry(c, "category_reorder", "category", "", map[string]any{"order": len(updates)})
	if err := h.withAuditTx(c, &entry, func(tx *gorm.DB) error {
		for _, u := range updates {
			if err := tx.Model(&model.Category{}).Where("id = ?", u.ID).Update("sort_order", u.SortOrder).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if respondCategoryAuditError(c, err) {
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "reordered"})
}

func (h *CategoryHandler) auditEntry(c *gin.Context, action, targetType, targetID string, metadata map[string]any) service.RecordAdminAuditInput {
	return service.RecordAdminAuditInput{
		AdminUserID: middleware.GetUserID(c),
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		TraceID:     c.GetString("trace_id"),
		Metadata:    metadata,
		Result:      "success",
	}
}

func (h *CategoryHandler) withAuditTx(c *gin.Context, entry *service.RecordAdminAuditInput, mutate func(tx *gorm.DB) error) error {
	return h.db.Transaction(func(tx *gorm.DB) error {
		if err := mutate(tx); err != nil {
			return err
		}
		if h.auditSvc == nil {
			return nil
		}
		if err := h.auditSvc.RecordTx(c.Request.Context(), tx, *entry); err != nil {
			return errAdminAuditWriteFailed
		}
		return nil
	})
}

func respondCategoryAuditError(c *gin.Context, err error) bool {
	if errors.Is(err, errAdminAuditWriteFailed) {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "AUDIT_WRITE_FAILED", "message": "audit write failed"})
		return true
	}
	return false
}
