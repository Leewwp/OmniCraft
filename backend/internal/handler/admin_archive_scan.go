package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

type archiveScanAdminRepository interface {
	GetJob(ctx context.Context, jobID int64) (*model.ArchiveScanJob, error)
	GetAttachmentScanState(ctx context.Context, attachmentID int64) (*model.ContentAttachment, error)
	ListAttemptsByJob(ctx context.Context, jobID int64) ([]model.ArchiveScanAttempt, error)
	StartManualReviewTx(ctx context.Context, tx *gorm.DB, jobID int64) error
	ResolveManualReviewTx(ctx context.Context, tx *gorm.DB, jobID int64, outcome string) error
	RetryFailedJobTx(ctx context.Context, tx *gorm.DB, jobID int64) error
}

type archiveScanAdminAudit interface {
	Record(ctx context.Context, entry service.RecordAdminAuditInput) error
	RecordTx(ctx context.Context, tx *gorm.DB, entry service.RecordAdminAuditInput) error
}

type AdminArchiveScanHandler struct {
	db       *gorm.DB
	repo     archiveScanAdminRepository
	audit    archiveScanAdminAudit
	objects  archiveScanReviewObjectStore
	notifier archiveScanCompletionNotifier
}

type archiveScanReviewObjectStore interface {
	Open(objectKey string) (io.ReadCloser, error)
	Copy(sourceKey, targetKey string) error
	Delete(objectKey string) error
	Exists(objectKey string) (bool, error)
}

type archiveScanCompletionNotifier interface {
	ArchiveScanClean(ctx context.Context, attachmentID int64) error
}

func NewAdminArchiveScanHandler(db *gorm.DB, repo *repository.ArchiveScanRepository, audit archiveScanAdminAudit, objects ...archiveScanReviewObjectStore) *AdminArchiveScanHandler {
	var objectStore archiveScanReviewObjectStore
	if len(objects) > 0 {
		objectStore = objects[0]
	}
	return &AdminArchiveScanHandler{db: db, repo: repo, audit: audit, objects: objectStore}
}

func (h *AdminArchiveScanHandler) SetArchiveScanCompletionNotifier(notifier archiveScanCompletionNotifier) {
	h.notifier = notifier
}

func (h *AdminArchiveScanHandler) GetJob(c *gin.Context) {
	jobID, ok := parseArchiveScanJobID(c)
	if !ok {
		return
	}
	job, err := h.repo.GetJob(c.Request.Context(), jobID)
	if err != nil {
		h.respondArchiveScanError(c, err)
		return
	}
	attempts, err := h.repo.ListAttemptsByJob(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to load scan attempts"})
		return
	}
	entry := archiveScanAuditEntry(c, "archive_scan_view", jobID, map[string]any{"job_id": jobID})
	if err := h.recordArchiveScanAudit(c, entry); err != nil {
		h.respondArchiveScanError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"job": job, "attempts": attempts})
}

func (h *AdminArchiveScanHandler) Retry(c *gin.Context) {
	jobID, ok := parseArchiveScanJobID(c)
	if !ok {
		return
	}
	entry := archiveScanAuditEntry(c, "archive_scan_retry", jobID, map[string]any{"job_id": jobID})
	if err := h.withArchiveScanAuditTx(c, entry, func(tx *gorm.DB) error {
		return h.repo.RetryFailedJobTx(c.Request.Context(), tx, jobID)
	}); err != nil {
		h.respondArchiveScanError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": model.ScanStatusPending})
}

func (h *AdminArchiveScanHandler) StartManualReview(c *gin.Context) {
	jobID, ok := parseArchiveScanJobID(c)
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Reason) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "REVIEW_REASON_REQUIRED", "message": "review reason is required"})
		return
	}
	reason := strings.TrimSpace(body.Reason)
	if h.objects == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "ARCHIVE_OBJECT_STORE_UNAVAILABLE", "message": "archive object store is unavailable"})
		return
	}
	job, err := h.repo.GetJob(c.Request.Context(), jobID)
	if err != nil {
		h.respondArchiveScanError(c, err)
		return
	}
	attachment, err := h.repo.GetAttachmentScanState(c.Request.Context(), job.AttachmentID)
	if err != nil {
		h.respondArchiveScanError(c, err)
		return
	}
	present, err := h.objects.Exists(strings.TrimSpace(attachment.OSSKey))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "ARCHIVE_QUARANTINE_CLEANUP_UNKNOWN", "message": "archive cleanup state is unavailable"})
		return
	}
	if present {
		c.JSON(http.StatusConflict, gin.H{"code": "ARCHIVE_QUARANTINE_CLEANUP_PENDING", "message": "archive quarantine cleanup is pending"})
		return
	}
	if strings.TrimSpace(job.QuarantineKey) == "" {
		c.JSON(http.StatusConflict, gin.H{"code": "ARCHIVE_QUARANTINE_MISSING", "message": "archive quarantine object is unavailable"})
		return
	}
	quarantined, err := h.objects.Exists(strings.TrimSpace(job.QuarantineKey))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "ARCHIVE_QUARANTINE_CLEANUP_UNKNOWN", "message": "archive cleanup state is unavailable"})
		return
	}
	if !quarantined {
		c.JSON(http.StatusConflict, gin.H{"code": "ARCHIVE_QUARANTINE_MISSING", "message": "archive quarantine object is unavailable"})
		return
	}
	entry := archiveScanAuditEntry(c, "archive_scan_manual_review", jobID, map[string]any{
		"job_id": jobID,
		"reason": reason,
	})
	if err := h.withArchiveScanAuditTx(c, entry, func(tx *gorm.DB) error {
		return h.repo.StartManualReviewTx(c.Request.Context(), tx, jobID)
	}); err != nil {
		h.respondArchiveScanError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": model.ScanStatusManualReview})
}

func (h *AdminArchiveScanHandler) ResolveManualReview(c *gin.Context) {
	jobID, ok := parseArchiveScanJobID(c)
	if !ok {
		return
	}
	var body struct {
		Outcome string `json:"outcome"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_BODY", "message": "invalid request body"})
		return
	}
	body.Outcome = strings.TrimSpace(body.Outcome)
	if body.Outcome != model.ScanStatusClean && body.Outcome != model.ScanStatusBlocked {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_OUTCOME", "message": "outcome must be clean or blocked"})
		return
	}
	if strings.TrimSpace(body.Reason) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "REVIEW_REASON_REQUIRED", "message": "review reason is required"})
		return
	}
	if h.audit == nil {
		h.respondArchiveScanError(c, errAdminAuditUnavailable)
		return
	}
	if body.Outcome == model.ScanStatusClean && h.objects == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "ARCHIVE_OBJECT_STORE_UNAVAILABLE", "message": "archive object store is unavailable"})
		return
	}
	reason := strings.TrimSpace(body.Reason)
	entry := archiveScanAuditEntry(c, "archive_scan_review_resolve", jobID, map[string]any{
		"job_id":  jobID,
		"outcome": body.Outcome,
		"reason":  reason,
	})
	var err error
	var attachmentID int64
	if body.Outcome == model.ScanStatusClean {
		attachmentID, err = h.resolveCleanManualReview(c, jobID, entry)
	} else {
		err = h.withArchiveScanAuditTx(c, entry, func(tx *gorm.DB) error {
			return h.repo.ResolveManualReviewTx(c.Request.Context(), tx, jobID, body.Outcome)
		})
	}
	if err != nil {
		h.respondArchiveScanError(c, err)
		return
	}
	if h.notifier != nil && body.Outcome == model.ScanStatusClean {
		if err := h.notifier.ArchiveScanClean(c.Request.Context(), attachmentID); err != nil {
			h.respondArchiveScanError(c, errors.Join(errArchiveScanCompletionFailed, err))
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": body.Outcome})
}

func (h *AdminArchiveScanHandler) resolveCleanManualReview(c *gin.Context, jobID int64, entry service.RecordAdminAuditInput) (int64, error) {
	var sourceKey, targetKey string
	var expectedSHA256 string
	var attachmentID int64
	restored := false
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var job model.ArchiveScanJob
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", jobID).First(&job).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return repository.ErrArchiveScanNotFound
			}
			return err
		}
		if job.Status == model.ScanStatusClean {
			attachmentID = job.AttachmentID
			return nil
		}
		if job.Status != model.ScanStatusManualReview {
			return repository.ErrArchiveScanIllegalState
		}
		var attachment model.ContentAttachment
		if err := tx.Where("id = ?", job.AttachmentID).First(&attachment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return repository.ErrArchiveScanNotFound
			}
			return err
		}
		attachmentID = attachment.ID
		sourceKey = strings.TrimSpace(job.QuarantineKey)
		targetKey = strings.TrimSpace(attachment.OSSKey)
		expectedSHA256 = strings.TrimSpace(job.ObjectSHA256)
		if sourceKey == "" || targetKey == "" {
			return repository.ErrArchiveScanIllegalState
		}
		targetPresent, err := h.objects.Exists(targetKey)
		if err != nil {
			return errors.Join(errArchiveRestoreFailed, err)
		}
		if targetPresent {
			return errArchiveRestoreTargetExists
		}
		if err := h.objects.Copy(sourceKey, targetKey); err != nil {
			return errors.Join(errArchiveRestoreFailed, err)
		}
		restored = true
		if err := h.repo.ResolveManualReviewTx(c.Request.Context(), tx, jobID, model.ScanStatusClean); err != nil {
			return err
		}
		if h.audit == nil {
			return errAdminAuditUnavailable
		}
		if err := h.audit.RecordTx(c.Request.Context(), tx, entry); err != nil {
			return errors.Join(errAdminAuditWriteFailed, err)
		}
		return nil
	})
	if err == nil || !restored {
		return attachmentID, err
	}
	if rollbackErr := h.rollbackArchiveRestore(targetKey, sourceKey, expectedSHA256); rollbackErr != nil {
		slog.Error("archive review restore compensation failed", "error_code", "archive_restore_rollback_failed")
		return attachmentID, errors.Join(err, errArchiveRestoreRollbackFailed)
	}
	return attachmentID, err
}

func (h *AdminArchiveScanHandler) rollbackArchiveRestore(targetKey, sourceKey, expectedSHA256 string) error {
	if strings.TrimSpace(expectedSHA256) == "" {
		return errors.New("archive restore source hash is unavailable")
	}
	actualSHA256, err := archiveObjectSHA256(h.objects, targetKey)
	if err != nil || !strings.EqualFold(actualSHA256, expectedSHA256) {
		return errors.New("archive restore target changed during compensation")
	}
	if err := h.objects.Copy(targetKey, sourceKey); err != nil {
		return err
	}
	return h.objects.Delete(targetKey)
}

func archiveObjectSHA256(objects archiveScanReviewObjectStore, objectKey string) (string, error) {
	object, err := objects.Open(objectKey)
	if err != nil {
		return "", err
	}
	digest := sha256.New()
	_, copyErr := io.Copy(digest, object)
	closeErr := object.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func (h *AdminArchiveScanHandler) recordArchiveScanAudit(c *gin.Context, entry service.RecordAdminAuditInput) error {
	if h.audit == nil {
		return errAdminAuditUnavailable
	}
	return h.audit.Record(c.Request.Context(), entry)
}

func (h *AdminArchiveScanHandler) withArchiveScanAuditTx(c *gin.Context, entry service.RecordAdminAuditInput, mutate func(tx *gorm.DB) error) error {
	return h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := mutate(tx); err != nil {
			return err
		}
		if h.audit == nil {
			return errAdminAuditUnavailable
		}
		if err := h.audit.RecordTx(c.Request.Context(), tx, entry); err != nil {
			return errors.Join(errAdminAuditWriteFailed, err)
		}
		return nil
	})
}

func (h *AdminArchiveScanHandler) respondArchiveScanError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errArchiveRestoreRollbackFailed):
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ARCHIVE_RESTORE_ROLLBACK_FAILED", "message": "archive restore rollback failed"})
	case errors.Is(err, errAdminAuditWriteFailed):
		c.JSON(http.StatusInternalServerError, gin.H{"code": "AUDIT_WRITE_FAILED", "message": "audit write failed"})
	case errors.Is(err, errAdminAuditUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "AUDIT_UNAVAILABLE", "message": "audit service is unavailable"})
	case errors.Is(err, repository.ErrArchiveScanNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "archive scan job not found"})
	case errors.Is(err, repository.ErrArchiveScanIllegalState):
		c.JSON(http.StatusConflict, gin.H{"code": "ARCHIVE_SCAN_ILLEGAL_STATE", "message": "archive scan state transition is not allowed"})
	case errors.Is(err, repository.ErrArchiveScanRetryExhausted):
		c.JSON(http.StatusConflict, gin.H{"code": "ARCHIVE_SCAN_RETRY_EXHAUSTED", "message": "archive scan retry budget is exhausted"})
	case errors.Is(err, errArchiveRestoreFailed):
		c.JSON(http.StatusBadGateway, gin.H{"code": "ARCHIVE_RESTORE_FAILED", "message": "archive restore failed"})
	case errors.Is(err, errArchiveRestoreTargetExists):
		c.JSON(http.StatusConflict, gin.H{"code": "ARCHIVE_RESTORE_TARGET_EXISTS", "message": "archive restore target already exists"})
	case errors.Is(err, errArchiveScanCompletionFailed):
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "ARCHIVE_SCAN_COMPLETION_FAILED", "message": "archive scan completion could not be applied"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "archive scan operation failed"})
	}
}

var errAdminAuditUnavailable = errors.New("archive scan audit service unavailable")
var errArchiveRestoreFailed = errors.New("archive restore failed")
var errArchiveRestoreRollbackFailed = errors.New("archive restore rollback failed")
var errArchiveRestoreTargetExists = errors.New("archive restore target already exists")
var errArchiveScanCompletionFailed = errors.New("archive scan completion failed")

func parseArchiveScanJobID(c *gin.Context) (int64, bool) {
	jobID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || jobID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid archive scan job id"})
		return 0, false
	}
	return jobID, true
}

func archiveScanAuditEntry(c *gin.Context, action string, jobID int64, metadata map[string]any) service.RecordAdminAuditInput {
	return service.RecordAdminAuditInput{
		AdminUserID: middleware.GetUserID(c),
		Action:      action,
		TargetType:  "archive_scan_job",
		TargetID:    strconv.FormatInt(jobID, 10),
		TraceID:     c.GetString("trace_id"),
		Metadata:    metadata,
		Result:      "success",
	}
}
