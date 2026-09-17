package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

// AdminTraceHandler serves the agent observability surface (SP-21 T3):
// the /admin/traces request list with multi-dimensional filters and the
// GLOBAL window statistics card (aggregated over the window, not the
// current page — the polyu shortcoming this batch fixes).
type AdminTraceHandler struct {
	repo *repository.AgentTraceRepository
}

func NewAdminTraceHandler(repo *repository.AgentTraceRepository) *AdminTraceHandler {
	return &AdminTraceHandler{repo: repo}
}

// parseRunFilter maps the query string onto the repository filter; invalid
// numeric values are rejected with 400 rather than silently ignored.
func (h *AdminTraceHandler) parseRunFilter(c *gin.Context) (repository.AgentTraceRunFilter, bool) {
	f := repository.AgentTraceRunFilter{
		TraceID:    trimNonEmpty(c.Query("trace_id")),
		Model:      trimNonEmpty(c.Query("model")),
		Status:     trimNonEmpty(c.Query("status")),
		AnswerKind: trimNonEmpty(c.Query("answer_kind")),
		Surface:    trimNonEmpty(c.Query("surface")),
	}
	if raw := trimNonEmpty(c.Query("conversation_id")); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGS", "message": "conversation_id must be a positive integer"})
			return f, false
		}
		f.ConversationID = &v
	}
	if raw := trimNonEmpty(c.Query("user_id")); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGS", "message": "user_id must be a positive integer"})
			return f, false
		}
		f.UserID = &v
	}
	if raw := trimNonEmpty(c.Query("page")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGS", "message": "page must be a positive integer"})
			return f, false
		}
		f.Page = v
	}
	if raw := trimNonEmpty(c.Query("page_size")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGS", "message": "page_size must be between 1 and 100"})
			return f, false
		}
		f.PageSize = v
	}
	if raw := trimNonEmpty(c.Query("from")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGS", "message": "from must be RFC3339"})
			return f, false
		}
		f.From = &t
	}
	if raw := trimNonEmpty(c.Query("to")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGS", "message": "to must be RFC3339"})
			return f, false
		}
		f.To = &t
	}
	return f, true
}

func trimNonEmpty(s string) string {
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

// ListTraces returns one page of agent trace runs, newest first.
func (h *AdminTraceHandler) ListTraces(c *gin.Context) {
	filter, ok := h.parseRunFilter(c)
	if !ok {
		return
	}
	items, total, err := h.repo.ListRuns(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to load trace runs"})
		return
	}
	if items == nil {
		items = []model.AgentTraceRun{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

// GetTraceDetail returns one run plus its full node list (start order) for
// the waterfall view. Unknown trace ids are a 404.
func (h *AdminTraceHandler) GetTraceDetail(c *gin.Context) {
	traceID := c.Param("trace_id")
	if len(traceID) < 8 || len(traceID) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGS", "message": "invalid trace id"})
		return
	}
	run, err := h.repo.GetRunByTraceID(c.Request.Context(), traceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "TRACE_NOT_FOUND", "message": "trace run not found"})
		return
	}
	nodes, err := h.repo.ListNodesByTrace(c.Request.Context(), traceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to load trace nodes"})
		return
	}
	if nodes == nil {
		nodes = []model.AgentTraceNode{}
	}
	c.JSON(http.StatusOK, gin.H{"run": run, "nodes": nodes})
}

// Stats returns global window aggregates for the statistics card. The
// window defaults to the last 24h; from/to accept RFC3339 bounds.
func (h *AdminTraceHandler) Stats(c *gin.Context) {
	from := time.Time{}
	to := time.Time{}
	if raw := c.Query("from"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGS", "message": "from must be RFC3339"})
			return
		}
		from = t
	}
	if raw := c.Query("to"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGS", "message": "to must be RFC3339"})
			return
		}
		to = t
	}
	var fromPtr, toPtr *time.Time
	if !from.IsZero() {
		fromPtr = &from
	}
	if !to.IsZero() {
		toPtr = &to
	}
	stats, err := h.repo.Stats(c.Request.Context(), fromPtr, toPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to aggregate trace stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}
