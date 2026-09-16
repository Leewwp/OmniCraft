package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"omnicraft/backend/internal/service/promptregistry"
)

// AdminPromptHandler serves the prompt registry admin surface (SP-21 T5):
// slot overview, version history, immutable version creation with
// placeholder-contract validation, and production/staging label moves.
// Every write is audited; writes invalidate the shared resolver cache.
type AdminPromptHandler struct {
	repo     *repository.PromptRegistryRepository
	resolver *promptregistry.PromptResolver
	auditSvc AdminRAGAuditRecorder
}

func NewAdminPromptHandler(
	repo *repository.PromptRegistryRepository,
	resolver *promptregistry.PromptResolver,
	auditSvc AdminRAGAuditRecorder,
) *AdminPromptHandler {
	return &AdminPromptHandler{repo: repo, resolver: resolver, auditSvc: auditSvc}
}

type adminPromptSlotView struct {
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	RequiredPlaceholders []string `json:"required_placeholders"`
	ProductionVersion    int      `json:"production_version"`
	StagingVersion       int      `json:"staging_version"`
	LatestVersion        int       `json:"latest_version"`
}

// ListSlots returns every registry slot with its current label pointers.
func (h *AdminPromptHandler) ListSlots(c *gin.Context) {
	labels, err := h.repo.ListAllLabels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to load prompt labels"})
		return
	}
	pointer := map[string]map[string]int{}
	for _, l := range labels {
		if pointer[l.Name] == nil {
			pointer[l.Name] = map[string]int{}
		}
		pointer[l.Name][l.Label] = l.Version
	}
	slots := make([]adminPromptSlotView, 0, len(promptregistry.Slots))
	for _, slot := range promptregistry.Slots {
		view := adminPromptSlotView{
			Name:                 slot.Name,
			Description:          slot.Description,
			RequiredPlaceholders: slot.RequiredPlaceholders,
		}
		if p := pointer[slot.Name]; p != nil {
			view.ProductionVersion = p[promptregistry.ProductionLabel]
			view.StagingVersion = p["staging"]
		}
		if latest, err := h.repo.LatestVersion(c.Request.Context(), slot.Name); err == nil && latest != nil {
			view.LatestVersion = latest.Version
		}
		slots = append(slots, view)
	}
	c.JSON(http.StatusOK, gin.H{"slots": slots})
}

// ListVersions returns the immutable version history of one slot plus its
// label pointers (newest first) for the diff/rollback view.
func (h *AdminPromptHandler) ListVersions(c *gin.Context) {
	name := c.Param("name")
	if _, ok := promptregistry.SlotByName(name); !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": "PROMPT_SLOT_NOT_FOUND", "message": "unknown prompt slot"})
		return
	}
	versions, err := h.repo.ListVersions(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to load prompt versions"})
		return
	}
	labels, err := h.repo.ListLabels(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to load prompt labels"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"versions": versions, "labels": labels})
}

type createPromptVersionInput struct {
	Content string `json:"content" binding:"required"`
}

// CreateVersion appends an immutable version. The content must satisfy the
// slot's placeholder contract exactly (required present, no unknown tokens)
// so a hot-swapped template can never break rendering at runtime.
func (h *AdminPromptHandler) CreateVersion(c *gin.Context) {
	name := c.Param("name")
	slot, ok := promptregistry.SlotByName(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": "PROMPT_SLOT_NOT_FOUND", "message": "unknown prompt slot"})
		return
	}
	var in createPromptVersionInput
	if err := c.ShouldBindJSON(&in); err != nil || in.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGS", "message": "content is required"})
		return
	}
	if err := promptregistry.ValidateTemplate(slot, in.Content); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": "PROMPT_PLACEHOLDER_CONTRACT", "message": err.Error()})
		return
	}
	latest, err := h.repo.LatestVersion(c.Request.Context(), name)
	if err != nil && !errors.Is(err, repository.ErrPromptNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to resolve next version"})
		return
	}
	next := 1
	if latest != nil {
		next = latest.Version + 1
	}
	placeholders, err := json.Marshal(slot.RequiredPlaceholders)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to snapshot placeholders"})
		return
	}
	adminID := middleware.GetUserID(c)
	row := &model.PromptRegistry{
		Name:                 name,
		Version:              next,
		Content:              in.Content,
		RequiredPlaceholders: model.JSONB(placeholders),
		CreatedBy:            &adminID,
	}
	if err := h.repo.CreateVersion(c.Request.Context(), row); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to create prompt version"})
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.Record(c.Request.Context(), service.RecordAdminAuditInput{
			AdminUserID: adminID,
			Action:      "prompt.version.create",
			TargetType:  "prompt",
			TargetID:    name,
			Metadata:    map[string]interface{}{"version": next, "content_bytes": len(in.Content)},
			Result:      "success",
		})
	}
	c.JSON(http.StatusCreated, gin.H{"version": row.Version})
}

type setPromptLabelInput struct {
	Label   string `json:"label" binding:"required"`
	Version int    `json:"version" binding:"required"`
}

// SetLabel moves a production/staging pointer to an existing version.
// Rolling back = pointing production at an older version; the resolver
// cache is invalidated so the move is live within seconds.
func (h *AdminPromptHandler) SetLabel(c *gin.Context) {
	name := c.Param("name")
	if _, ok := promptregistry.SlotByName(name); !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": "PROMPT_SLOT_NOT_FOUND", "message": "unknown prompt slot"})
		return
	}
	var in setPromptLabelInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGS", "message": "label and version are required"})
		return
	}
	if in.Label != promptregistry.ProductionLabel && in.Label != "staging" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": "PROMPT_LABEL_INVALID", "message": "label must be production or staging"})
		return
	}
	if err := h.repo.SetLabel(c.Request.Context(), name, in.Label, in.Version); err != nil {
		if errors.Is(err, repository.ErrPromptNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "PROMPT_VERSION_NOT_FOUND", "message": "prompt version not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to move prompt label"})
		return
	}
	if h.resolver != nil {
		h.resolver.Invalidate()
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.Record(c.Request.Context(), service.RecordAdminAuditInput{
			AdminUserID: middleware.GetUserID(c),
			Action:      "prompt.label.set",
			TargetType:  "prompt",
			TargetID:    name,
			Metadata:    map[string]interface{}{"label": in.Label, "version": in.Version},
			Result:      "success",
		})
	}
	c.JSON(http.StatusOK, gin.H{"status": "moved"})
}
