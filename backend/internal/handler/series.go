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

type SeriesHandler struct {
	seriesSvc *service.SeriesService
}

type seriesOwnerResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type seriesSummaryResponse struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Zone        string `json:"zone"`
}

type seriesDetailResponse struct {
	ID             int64               `json:"id"`
	Title          string              `json:"title"`
	Description    string              `json:"description"`
	Zone           string              `json:"zone"`
	Owner          seriesOwnerResponse `json:"owner"`
	Cover          *string             `json:"cover"`
	CoverContentID *int64              `json:"cover_content_id,omitempty"`
	ItemCount      int64               `json:"item_count"`
}

type seriesItemResponse struct {
	ID            int64                 `json:"id"`
	SortOrder     int                   `json:"sort_order"`
	ContentItemID int64                 `json:"content_item_id"`
	Content       seriesContentResponse `json:"content"`
}

type seriesContentResponse struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	Zone          string `json:"zone"`
	ContentType   string `json:"content_type"`
	CoverImageURL string `json:"cover_image_url,omitempty"`
	Status        string `json:"status"`
}

func NewSeriesHandler(db *gorm.DB) *SeriesHandler {
	repo := repository.NewSeriesRepository(db)
	return &SeriesHandler{seriesSvc: service.NewSeriesService(repo)}
}

func (h *SeriesHandler) CreateSeries(c *gin.Context) {
	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Zone        string `json:"zone"`
	}
	if !decodeSeriesJSON(c, map[string]struct{}{"title": {}, "description": {}, "zone": {}}, &input) {
		return
	}
	if !validSeriesTitle(input.Title) || !validSeriesZone(strings.TrimSpace(input.Zone)) {
		response.ValidationError(c, "invalid series title or zone")
		return
	}
	series, err := h.seriesSvc.CreateSeries(c.Request.Context(), middleware.GetUserID(c), input.Title, input.Description, strings.TrimSpace(input.Zone))
	if err != nil {
		h.writeSeriesError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"series": mapSeriesSummary(*series)})
}

func (h *SeriesHandler) ListSeries(c *gin.Context) {
	zone := strings.TrimSpace(c.Query("zone"))
	if zone != "" && !validSeriesZone(zone) {
		response.ValidationError(c, "invalid zone")
		return
	}
	series, err := h.seriesSvc.ListOwnedSeries(c.Request.Context(), middleware.GetUserID(c), zone)
	if err != nil {
		h.writeSeriesError(c, err)
		return
	}
	items := make([]seriesSummaryResponse, 0, len(series))
	for _, item := range series {
		items = append(items, mapSeriesSummary(item))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

func (h *SeriesHandler) ListCandidates(c *gin.Context) {
	zone := strings.TrimSpace(c.Query("zone"))
	if !validSeriesZone(zone) {
		response.ValidationError(c, "valid zone is required")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, err := h.seriesSvc.ListCandidateContents(c.Request.Context(), middleware.GetUserID(c), zone, c.Query("q"), limit)
	if err != nil {
		h.writeSeriesError(c, err)
		return
	}
	result := make([]seriesContentResponse, 0, len(items))
	for _, item := range items {
		result = append(result, mapSeriesContent(item))
	}
	c.JSON(http.StatusOK, gin.H{"items": result, "total": len(result)})
}

func (h *SeriesHandler) GetSeries(c *gin.Context) {
	seriesID, ok := parseSeriesParam(c, "id")
	if !ok {
		return
	}
	viewerID := middleware.GetUserID(c)
	management := c.Query("manage") == "true"
	var detail *service.SeriesDetail
	var err error
	if management {
		detail, err = h.seriesSvc.GetSeriesManagementDetail(c.Request.Context(), seriesID, viewerID)
	} else {
		detail, err = h.seriesSvc.GetSeriesDetail(c.Request.Context(), seriesID, viewerID)
	}
	if err != nil {
		h.writeSeriesError(c, err)
		return
	}
	series := mapSeriesDetail(detail.Series, detail.Cover, detail.ItemCount)
	if management {
		series.CoverContentID = detail.Series.CoverContentID
	}
	items := make([]seriesItemResponse, 0, len(detail.Items))
	for _, item := range detail.Items {
		items = append(items, mapSeriesItem(item))
	}
	c.JSON(http.StatusOK, gin.H{"series": series, "items": items})
}

func (h *SeriesHandler) UpdateSeries(c *gin.Context) {
	seriesID, ok := parseSeriesParam(c, "id")
	if !ok {
		return
	}
	fields, raw, ok := readSeriesJSON(c)
	if !ok || !validateSeriesFields(c, fields, map[string]struct{}{"title": {}, "description": {}, "cover_content_id": {}}) {
		return
	}
	if len(fields) == 0 {
		response.ValidationError(c, "at least one field is required")
		return
	}
	var input struct {
		Title          *string `json:"title"`
		Description    *string `json:"description"`
		CoverContentID *int64  `json:"cover_content_id"`
	}
	if !unmarshalSeriesJSON(c, raw, &input) {
		return
	}
	patch := repository.SeriesPatch{}
	if _, exists := fields["title"]; exists {
		if input.Title == nil || !validSeriesTitle(*input.Title) {
			response.ValidationError(c, "invalid title")
			return
		}
		patch.Title = input.Title
	}
	if _, exists := fields["description"]; exists {
		if input.Description == nil {
			response.ValidationError(c, "invalid description")
			return
		}
		patch.Description = input.Description
	}
	if _, exists := fields["cover_content_id"]; exists {
		if input.CoverContentID == nil {
			patch.ClearCover = true
		} else if *input.CoverContentID <= 0 {
			response.ValidationError(c, "invalid cover_content_id")
			return
		} else {
			patch.CoverContentID = input.CoverContentID
		}
	}
	series, err := h.seriesSvc.UpdateSeries(c.Request.Context(), seriesID, middleware.GetUserID(c), patch)
	if err != nil {
		h.writeSeriesError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"series": mapSeriesSummary(*series)})
}

func (h *SeriesHandler) DeleteSeries(c *gin.Context) {
	seriesID, ok := parseSeriesParam(c, "id")
	if !ok {
		return
	}
	if err := h.seriesSvc.DeleteSeries(c.Request.Context(), seriesID, middleware.GetUserID(c)); err != nil {
		h.writeSeriesError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *SeriesHandler) AddItem(c *gin.Context) {
	seriesID, ok := parseSeriesParam(c, "id")
	if !ok {
		return
	}
	var input struct {
		ContentItemID int64 `json:"content_item_id"`
	}
	if !decodeSeriesJSON(c, map[string]struct{}{"content_item_id": {}}, &input) {
		return
	}
	if input.ContentItemID <= 0 {
		response.ValidationError(c, "content_item_id is required")
		return
	}
	item, err := h.seriesSvc.AddItem(c.Request.Context(), seriesID, middleware.GetUserID(c), input.ContentItemID)
	if err != nil {
		h.writeSeriesError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"item": gin.H{
		"id": item.ID, "series_id": item.SeriesID, "content_item_id": item.ContentItemID,
		"sort_order": item.SortOrder, "added_at": item.AddedAt,
	}})
}

func (h *SeriesHandler) RemoveItem(c *gin.Context) {
	seriesID, ok := parseSeriesParam(c, "id")
	if !ok {
		return
	}
	itemID, ok := parseSeriesParam(c, "itemId")
	if !ok {
		return
	}
	if err := h.seriesSvc.RemoveItem(c.Request.Context(), seriesID, middleware.GetUserID(c), itemID); err != nil {
		h.writeSeriesError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "removed"})
}

func (h *SeriesHandler) ReorderItems(c *gin.Context) {
	seriesID, ok := parseSeriesParam(c, "id")
	if !ok {
		return
	}
	var input struct {
		ItemIDs []int64 `json:"item_ids"`
	}
	if !decodeSeriesJSON(c, map[string]struct{}{"item_ids": {}}, &input) {
		return
	}
	if input.ItemIDs == nil {
		response.ValidationError(c, "item_ids is required")
		return
	}
	for _, itemID := range input.ItemIDs {
		if itemID <= 0 {
			response.ValidationError(c, "item_ids must contain positive IDs")
			return
		}
	}
	if err := h.seriesSvc.ReorderItems(c.Request.Context(), seriesID, middleware.GetUserID(c), input.ItemIDs); err != nil {
		h.writeSeriesError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "reordered"})
}

func (h *SeriesHandler) writeSeriesError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrSeriesNotFound):
		response.Error(c, http.StatusNotFound, "SERIES_NOT_FOUND", "series not found")
	case errors.Is(err, repository.ErrNotSeriesOwner):
		response.Error(c, http.StatusForbidden, "NOT_SERIES_OWNER", "only the series owner can manage it")
	case errors.Is(err, repository.ErrContentNotOwnedOrContributed):
		response.Error(c, http.StatusBadRequest, "CONTENT_NOT_OWNED_OR_CONTRIBUTED", "content is not owned or contributed by the series owner")
	case errors.Is(err, repository.ErrSeriesZoneMismatch):
		response.Error(c, http.StatusBadRequest, "ZONE_MISMATCH", "series and content zones do not match")
	case errors.Is(err, repository.ErrDuplicateSeriesItem):
		response.Error(c, http.StatusConflict, "DUPLICATE_SERIES_ITEM", "content already exists in series")
	case errors.Is(err, repository.ErrCoverNotInSeries):
		response.Error(c, http.StatusBadRequest, "COVER_NOT_IN_SERIES", "cover content must belong to the series")
	case errors.Is(err, repository.ErrSeriesContentUnavailable):
		response.Error(c, http.StatusBadRequest, "CONTENT_UNAVAILABLE", "content is unavailable for series management")
	case errors.Is(err, repository.ErrSeriesItemSetMismatch):
		response.Error(c, http.StatusBadRequest, "SERIES_ITEM_SET_MISMATCH", "item_ids must match the complete series item set")
	case errors.Is(err, repository.ErrSeriesItemNotFound):
		response.Error(c, http.StatusNotFound, "SERIES_ITEM_NOT_FOUND", "series item not found")
	case errors.Is(err, repository.ErrInvalidSeries):
		response.Error(c, http.StatusBadRequest, "INVALID_SERIES", "invalid series")
	default:
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
	}
}

func mapSeriesSummary(series model.ContentSeries) seriesSummaryResponse {
	return seriesSummaryResponse{ID: series.ID, Title: series.Title, Description: series.Description, Zone: series.Zone}
}

func mapSeriesDetail(series model.ContentSeries, cover *string, itemCount int64) seriesDetailResponse {
	return seriesDetailResponse{
		ID: series.ID, Title: series.Title, Description: series.Description, Zone: series.Zone,
		Owner: seriesOwnerResponse{ID: series.OwnerID, Username: series.Owner.Username},
		Cover: cover, ItemCount: itemCount,
	}
}

func mapSeriesItem(item model.ContentSeriesItem) seriesItemResponse {
	content := item.ContentItem
	return seriesItemResponse{
		ID:            item.ID,
		SortOrder:     item.SortOrder,
		ContentItemID: item.ContentItemID,
		Content:       mapSeriesContent(content),
	}
}

func mapSeriesContent(content model.ContentItem) seriesContentResponse {
	return seriesContentResponse{
		ID: content.ID, Title: content.Title, Zone: content.Zone, ContentType: content.ContentType,
		CoverImageURL: content.CoverImageURL, Status: content.Status,
	}
}

func parseSeriesParam(c *gin.Context, name string) (int64, bool) {
	value, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || value <= 0 {
		response.ValidationError(c, "invalid "+name)
		return 0, false
	}
	return value, true
}

func readSeriesJSON(c *gin.Context) (map[string]json.RawMessage, []byte, bool) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil || strings.TrimSpace(string(raw)) == "" {
		response.ValidationError(c, "invalid request body")
		return nil, nil, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		response.ValidationError(c, "request body must be a JSON object")
		return nil, nil, false
	}
	return fields, raw, true
}

func decodeSeriesJSON(c *gin.Context, allowed map[string]struct{}, target interface{}) bool {
	fields, raw, ok := readSeriesJSON(c)
	if !ok || !validateSeriesFields(c, fields, allowed) {
		return false
	}
	return unmarshalSeriesJSON(c, raw, target)
}

func unmarshalSeriesJSON(c *gin.Context, raw []byte, target interface{}) bool {
	if err := json.Unmarshal(raw, target); err != nil {
		response.ValidationError(c, "invalid request body")
		return false
	}
	return true
}

func validateSeriesFields(c *gin.Context, fields map[string]json.RawMessage, allowed map[string]struct{}) bool {
	for field := range fields {
		if _, ok := allowed[field]; !ok {
			response.ValidationError(c, "unsupported field: "+field)
			return false
		}
	}
	return true
}

func validSeriesTitle(title string) bool {
	title = strings.TrimSpace(title)
	return title != "" && utf8.RuneCountInString(title) <= 200
}

func validSeriesZone(zone string) bool {
	return zone == "original" || zone == "fanwork"
}
