package handler

import (
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// agentSSEWriter centralizes SSE serialization and flushing for Agent
// endpoints. Once headers are written, all errors are reported as SSE events;
// a second JSON response is never attempted.
type agentSSEWriter struct {
	c *gin.Context
}

// begin writes the SSE headers. It must be called only after feature,
// schema, visibility and quota checks passed, so those rejections can still
// use regular JSON error responses.
func (w *agentSSEWriter) begin() {
	w.c.Header("Content-Type", "text/event-stream")
	w.c.Header("Cache-Control", "no-cache")
	w.c.Header("X-Accel-Buffering", "no")
}

// emit serializes one typed stream event and flushes it. The event name is
// server-owned (start/tool_status/delta/citation/usage/done/error).
func (w *agentSSEWriter) emit(ev service.AgentStreamEvent) error {
	w.c.SSEvent(string(ev.Type), ev)
	w.c.Writer.Flush()
	return nil
}
