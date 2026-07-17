package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

type CollectionHandler struct {
	collectionRepo *repository.CollectionRepository
	collectionSvc  *service.CollectionService
}

var (
	createCollectionFields = map[string]struct{}{
		"title":       {},
		"description": {},
		"zone":        {},
		"is_public":   {},
	}
	updateCollectionFields = map[string]struct{}{
		"title":       {},
		"description": {},
		"is_public":   {},
		"sort_order":  {},
	}
)

func NewCollectionHandler(db *gorm.DB) *CollectionHandler {
	collectionRepo := repository.NewCollectionRepository(db)
	return &CollectionHandler{
		collectionRepo: collectionRepo,
		collectionSvc:  service.NewCollectionService(collectionRepo, repository.NewContentRepository(db)),
	}
}

func (h *CollectionHandler) ListCollections(c *gin.Context) {
	viewerID := middleware.GetUserID(c)
	ownerID := viewerID
	rawOwnerID := c.Query("owner_id")
	explicitOwnerID := rawOwnerID != ""
	if explicitOwnerID {
		parsed, ok := parseCollectionInt64(c, rawOwnerID, "owner_id")
		if !ok {
			return
		}
		ownerID = parsed
	} else if viewerID == 0 {
		response.Error(c, http.StatusUnauthorized, "AUTH_REQUIRED", "authentication required")
		return
	}

	zone := strings.TrimSpace(c.Query("zone"))
	var containsContentItemID *int64
	if rawContentID := c.Query("content_item_id"); rawContentID != "" {
		parsed, ok := parseCollectionInt64(c, rawContentID, "content_item_id")
		if !ok {
			return
		}
		containsContentItemID = &parsed
	}

	var (
		items []repository.CollectionSummary
		err   error
	)
	if explicitOwnerID {
		items, err = h.collectionRepo.ListCollectionsForViewer(c.Request.Context(), ownerID, viewerID, zone, containsContentItemID)
	} else {
		items, err = h.collectionSvc.ListOwnCollections(c.Request.Context(), ownerID, zone, containsContentItemID)
	}
	if err != nil {
		h.writeCollectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

func (h *CollectionHandler) GetCollection(c *gin.Context) {
	collectionID, ok := parseCollectionParam(c, "id")
	if !ok {
		return
	}

	viewerID := middleware.GetUserID(c)
	var viewer *int64
	if viewerID != 0 {
		viewer = &viewerID
	}

	collection, err := h.collectionRepo.GetCollectionForViewer(c.Request.Context(), collectionID, viewer)
	if err != nil {
		h.writeCollectionError(c, err)
		return
	}

	page, pageSize := parseCollectionPagination(c)
	items, total, err := h.collectionRepo.ListItemsForViewer(c.Request.Context(), collectionID, viewerID, page, pageSize, strings.TrimSpace(c.Query("content_type")))
	if err != nil {
		h.writeCollectionError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"collection": collection,
		"items":      items,
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
	})
}

func (h *CollectionHandler) CreateCollection(c *gin.Context) {
	fields, raw, ok := readCollectionJSON(c)
	if !ok {
		return
	}
	if !validateCollectionAllowedFields(c, fields, createCollectionFields) {
		return
	}

	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Zone        string `json:"zone"`
		IsPublic    bool   `json:"is_public"`
	}
	if !decodeCollectionRequestJSON(c, raw, &input) {
		return
	}

	title, ok := validateCollectionTitle(c, input.Title)
	if !ok {
		return
	}
	zone := strings.TrimSpace(input.Zone)
	if !isCollectionHandlerZone(zone) {
		response.ValidationError(c, "invalid zone")
		return
	}

	collection, err := h.collectionRepo.CreateCollection(c.Request.Context(), &model.Collection{
		UserID:      middleware.GetUserID(c),
		Title:       title,
		Description: input.Description,
		Zone:        zone,
		IsPublic:    input.IsPublic,
	})
	if err != nil {
		h.writeCollectionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"collection": collection})
}

func (h *CollectionHandler) UpdateCollection(c *gin.Context) {
	collectionID, ok := parseCollectionParam(c, "id")
	if !ok {
		return
	}

	fields, raw, ok := readCollectionJSON(c)
	if !ok {
		return
	}
	if _, exists := fields["zone"]; exists {
		h.writeCollectionError(c, repository.ErrZoneImmutable)
		return
	}
	if !validateCollectionAllowedFields(c, fields, updateCollectionFields) {
		return
	}

	var input struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		IsPublic    *bool   `json:"is_public"`
		SortOrder   *int    `json:"sort_order"`
	}
	if !decodeCollectionRequestJSON(c, raw, &input) {
		return
	}

	patch := repository.CollectionPatch{
		Description: input.Description,
		IsPublic:    input.IsPublic,
		SortOrder:   input.SortOrder,
	}
	if _, exists := fields["title"]; exists {
		if input.Title == nil {
			response.ValidationError(c, "invalid title")
			return
		}
		title, ok := validateCollectionTitle(c, *input.Title)
		if !ok {
			return
		}
		patch.Title = &title
	}

	collection, err := h.collectionRepo.UpdateCollection(c.Request.Context(), collectionID, middleware.GetUserID(c), patch)
	if err != nil {
		h.writeCollectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"collection": collection})
}

func (h *CollectionHandler) DeleteCollection(c *gin.Context) {
	collectionID, ok := parseCollectionParam(c, "id")
	if !ok {
		return
	}
	if err := h.collectionRepo.DeleteCollection(c.Request.Context(), collectionID, middleware.GetUserID(c)); err != nil {
		h.writeCollectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *CollectionHandler) AddItem(c *gin.Context) {
	collectionID, ok := parseCollectionParam(c, "id")
	if !ok {
		return
	}

	var input struct {
		ContentItemID int64  `json:"content_item_id"`
		Note          string `json:"note"`
	}
	if !bindCollectionJSON(c, &input) {
		return
	}
	if input.ContentItemID <= 0 {
		response.ValidationError(c, "content_item_id is required")
		return
	}

	item, err := h.collectionSvc.AddItem(c.Request.Context(), collectionID, middleware.GetUserID(c), input.ContentItemID, input.Note)
	if err != nil {
		h.writeCollectionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"item": item})
}

func (h *CollectionHandler) RemoveItem(c *gin.Context) {
	collectionID, itemID, ok := parseCollectionItemParams(c)
	if !ok {
		return
	}
	if err := h.collectionSvc.RemoveItem(c.Request.Context(), collectionID, middleware.GetUserID(c), itemID); err != nil {
		h.writeCollectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "removed"})
}

func (h *CollectionHandler) UpdateItem(c *gin.Context) {
	collectionID, itemID, ok := parseCollectionItemParams(c)
	if !ok {
		return
	}

	var input struct {
		Note string `json:"note"`
	}
	if !bindCollectionJSON(c, &input) {
		return
	}

	item, err := h.collectionRepo.UpdateItemNote(c.Request.Context(), collectionID, middleware.GetUserID(c), itemID, input.Note)
	if err != nil {
		h.writeCollectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": item})
}

func (h *CollectionHandler) writeCollectionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrCollectionNotFound):
		response.Error(c, http.StatusNotFound, "COLLECTION_NOT_FOUND", "collection not found")
	case errors.Is(err, repository.ErrZoneMismatch):
		response.Error(c, http.StatusBadRequest, "ZONE_MISMATCH", "collection and content zones do not match")
	case errors.Is(err, repository.ErrDuplicateCollectionItem):
		response.Error(c, http.StatusConflict, "DUPLICATE_COLLECTION_ITEM", "content already exists in collection")
	case errors.Is(err, repository.ErrDefaultCollectionProtected):
		response.Error(c, http.StatusBadRequest, "DEFAULT_COLLECTION_PROTECTED", "default collection cannot be deleted")
	case errors.Is(err, repository.ErrZoneImmutable):
		response.Error(c, http.StatusBadRequest, "ZONE_IMMUTABLE", "collection zone cannot be changed")
	case errors.Is(err, repository.ErrInvalidContent):
		response.Error(c, http.StatusBadRequest, "INVALID_CONTENT", "content is unavailable")
	default:
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
	}
}

func parseCollectionItemParams(c *gin.Context) (int64, int64, bool) {
	collectionID, ok := parseCollectionParam(c, "id")
	if !ok {
		return 0, 0, false
	}
	itemID, ok := parseCollectionParam(c, "itemId")
	if !ok {
		return 0, 0, false
	}
	return collectionID, itemID, true
}

func parseCollectionParam(c *gin.Context, name string) (int64, bool) {
	return parseCollectionInt64(c, c.Param(name), name)
}

func parseCollectionInt64(c *gin.Context, raw string, name string) (int64, bool) {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		response.ValidationError(c, "invalid "+name)
		return 0, false
	}
	return value, true
}

func parseCollectionPagination(c *gin.Context) (int, int) {
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("page_size"), 20)
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func readCollectionJSON(c *gin.Context) (map[string]json.RawMessage, []byte, bool) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.ValidationError(c, "invalid request body")
		return nil, nil, false
	}
	if strings.TrimSpace(string(raw)) == "" {
		response.ValidationError(c, "request body is required")
		return nil, nil, false
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		response.ValidationError(c, "invalid request body")
		return nil, nil, false
	}
	if fields == nil {
		response.ValidationError(c, "request body must be a JSON object")
		return nil, nil, false
	}
	return fields, raw, true
}

func bindCollectionJSON(c *gin.Context, target interface{}) bool {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil || strings.TrimSpace(string(raw)) == "" {
		response.ValidationError(c, "invalid request body")
		return false
	}
	return decodeCollectionRequestJSON(c, raw, target)
}

func decodeCollectionRequestJSON(c *gin.Context, raw []byte, target interface{}) bool {
	if err := json.Unmarshal(raw, target); err != nil {
		response.ValidationError(c, "invalid request body")
		return false
	}
	return true
}

func validateCollectionAllowedFields(c *gin.Context, fields map[string]json.RawMessage, allowed map[string]struct{}) bool {
	for field := range fields {
		if _, ok := allowed[field]; !ok {
			response.ValidationError(c, "unsupported field: "+field)
			return false
		}
	}
	return true
}

func validateCollectionTitle(c *gin.Context, raw string) (string, bool) {
	title := strings.TrimSpace(raw)
	if title == "" || utf8.RuneCountInString(title) > 200 {
		response.ValidationError(c, "invalid title")
		return "", false
	}
	return title, true
}

func isCollectionHandlerZone(zone string) bool {
	return zone == "original" || zone == "fanwork"
}
