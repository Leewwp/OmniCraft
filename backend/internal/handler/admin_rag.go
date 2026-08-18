package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/service"
)

type RAGRebuilder interface {
	Rebuild(ctx context.Context) error
}

type AdminRAGAuditRecorder interface {
	Record(ctx context.Context, entry service.RecordAdminAuditInput) error
}

type AdminRAGHandler struct {
	cfg       *config.Config
	rebuilder RAGRebuilder
	auditSvc  AdminRAGAuditRecorder
}

func NewAdminRAGHandler(cfg *config.Config, rebuilder RAGRebuilder, auditSvc AdminRAGAuditRecorder) *AdminRAGHandler {
	return &AdminRAGHandler{cfg: cfg, rebuilder: rebuilder, auditSvc: auditSvc}
}

func (h *AdminRAGHandler) Rebuild(c *gin.Context) {
	operationID, err := newRAGRebuildOperationID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "AUDIT_WRITE_FAILED", "message": "audit write failed"})
		return
	}
	if h.cfg == nil || !h.cfg.Features.RAGHybridEnabled {
		if err := h.recordAudit(c.Request.Context(), c, operationID, "failed", "FEATURE_DISABLED"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "AUDIT_WRITE_FAILED", "message": "audit write failed"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FEATURE_DISABLED", "message": "RAG hybrid search is disabled"})
		return
	}
	if h.rebuilder == nil {
		if err := h.recordAudit(c.Request.Context(), c, operationID, "failed", "RAG_REBUILD_UNAVAILABLE"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "AUDIT_WRITE_FAILED", "message": "audit write failed"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "RAG_REBUILD_UNAVAILABLE", "message": "RAG rebuild is unavailable"})
		return
	}
	if err := h.recordAudit(c.Request.Context(), c, operationID, "started", ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "AUDIT_WRITE_FAILED", "message": "audit write failed"})
		return
	}
	if err := h.rebuilder.Rebuild(c.Request.Context()); err != nil {
		auditCtx, cancel := h.terminalAuditContext(c)
		defer cancel()
		if auditErr := h.recordAudit(auditCtx, c, operationID, "failed", "RAG_REBUILD_UNAVAILABLE"); auditErr != nil {
			h.logAuditDegradation(c, operationID, "failed", auditErr)
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "RAG_REBUILD_UNAVAILABLE", "message": "RAG rebuild is unavailable"})
		return
	}
	auditCtx, cancel := h.terminalAuditContext(c)
	defer cancel()
	if err := h.recordAudit(auditCtx, c, operationID, "success", ""); err != nil {
		h.logAuditDegradation(c, operationID, "success", err)
	}
	c.JSON(http.StatusOK, gin.H{"status": "completed"})
}

func (h *AdminRAGHandler) recordAudit(ctx context.Context, c *gin.Context, operationID, result, errorCode string) error {
	if h.auditSvc == nil {
		return errors.New("audit service unavailable")
	}
	metadata := map[string]any{"operation_id": operationID}
	if errorCode != "" {
		metadata["error_code"] = errorCode
	}
	return h.auditSvc.Record(ctx, service.RecordAdminAuditInput{
		AdminUserID: middleware.GetUserID(c),
		Action:      "rag_rebuild",
		TargetType:  "rag_index",
		TargetID:    operationID,
		TraceID:     c.GetString("trace_id"),
		Metadata:    metadata,
		Result:      result,
	})
}

func (h *AdminRAGHandler) terminalAuditContext(c *gin.Context) (context.Context, context.CancelFunc) {
	base := context.WithoutCancel(c.Request.Context())
	if h.cfg == nil || h.cfg.RAG.Index.AuditTimeoutSec <= 0 {
		return context.WithCancel(base)
	}
	return context.WithTimeout(base, time.Duration(h.cfg.RAG.Index.AuditTimeoutSec)*time.Second)
}

func (h *AdminRAGHandler) logAuditDegradation(c *gin.Context, operationID, result string, err error) {
	slog.Error("rag rebuild result audit degraded",
		"operation_id", operationID,
		"result", result,
		"trace_id", c.GetString("trace_id"),
		"error", err,
	)
}

func newRAGRebuildOperationID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
