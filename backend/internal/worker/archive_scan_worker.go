package worker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/clamav"
	"omnicraft/backend/internal/pkg/events"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
)

const (
	ArchiveScanTopic         = events.TopicArchiveScanRequested
	ArchiveScanConsumerGroup = "omnicraft-archive-scan"
)

var (
	ErrArchiveScanMalformedMessage = errors.New("malformed archive scan message")
	ErrArchiveScanNotReady         = errors.New("archive scan is not ready")
)

// ArchiveScanRepository is the worker seam over the S01 state machine. The
// concrete repository also owns the append-only attempt audit and attachment
// scan_status mirror.
type ArchiveScanRepository interface {
	GetJob(ctx context.Context, jobID int64) (*model.ArchiveScanJob, error)
	GetAttachmentScanState(ctx context.Context, attachmentID int64) (*model.ContentAttachment, error)
	StartScan(ctx context.Context, jobID int64) error
	FinishClean(ctx context.Context, jobID int64, engineVersion, signatureVersion, objectSHA256 string) error
	Block(ctx context.Context, jobID int64, detectionName, quarantineKey, objectSHA256 string) error
	Fail(ctx context.Context, jobID int64, errorCode string) error
	Retry(ctx context.Context, jobID int64) error
	AppendAttempt(ctx context.Context, attempt *model.ArchiveScanAttempt) error
}

type archiveScanAtomicOutcomeRepository interface {
	FinishCleanWithAttempt(ctx context.Context, jobID int64, attempt *model.ArchiveScanAttempt, engineVersion, signatureVersion, objectSHA256 string) error
	BlockWithAttempt(ctx context.Context, jobID int64, attempt *model.ArchiveScanAttempt, detectionName, quarantineKey, objectSHA256 string) error
	FailWithAttempt(ctx context.Context, jobID int64, attempt *model.ArchiveScanAttempt, errorCode string) error
}

type archiveScanRecoveryRepository interface {
	ResetScanning(ctx context.Context, jobID int64) error
}

type ArchiveScanCompletionNotifier interface {
	ArchiveScanClean(ctx context.Context, attachmentID int64) error
}

// ArchiveScanObjectStore keeps OSS operations behind a small seam so the
// worker can be tested without credentials or an external bucket.
type ArchiveScanObjectStore interface {
	Open(objectKey string) (io.ReadCloser, error)
	Copy(sourceKey, targetKey string) error
	Delete(objectKey string) error
	Exists(objectKey string) (bool, error)
}

type ArchiveScanner interface {
	Version(ctx context.Context) (clamav.Version, error)
	Scan(ctx context.Context, source io.Reader) (clamav.Result, error)
}

type ArchiveScanWorker struct {
	repository ArchiveScanRepository
	objects    ArchiveScanObjectStore
	scanner    ArchiveScanner
	timeout    time.Duration
	db         *gorm.DB
	notifier   ArchiveScanCompletionNotifier
}

func NewArchiveScanWorker(repository ArchiveScanRepository, objects ArchiveScanObjectStore, scanner ArchiveScanner, timeout time.Duration) *ArchiveScanWorker {
	return newArchiveScanWorker(repository, objects, scanner, timeout, nil, nil)
}

func NewArchiveScanWorkerWithDB(repository ArchiveScanRepository, objects ArchiveScanObjectStore, scanner ArchiveScanner, timeout time.Duration, db *gorm.DB) *ArchiveScanWorker {
	return newArchiveScanWorker(repository, objects, scanner, timeout, db, nil)
}

func NewArchiveScanWorkerWithDBAndNotifier(repository ArchiveScanRepository, objects ArchiveScanObjectStore, scanner ArchiveScanner, timeout time.Duration, db *gorm.DB, notifier ArchiveScanCompletionNotifier) *ArchiveScanWorker {
	return newArchiveScanWorker(repository, objects, scanner, timeout, db, notifier)
}

func newArchiveScanWorker(repository ArchiveScanRepository, objects ArchiveScanObjectStore, scanner ArchiveScanner, timeout time.Duration, db *gorm.DB, notifier ArchiveScanCompletionNotifier) *ArchiveScanWorker {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &ArchiveScanWorker{
		repository: repository,
		objects:    objects,
		scanner:    scanner,
		timeout:    timeout,
		db:         db,
		notifier:   notifier,
	}
}

// Handle consumes both the direct test payload and the transactional-outbox
// envelope delivered by RelayWorker. Archive retries stay inside this one
// delivery so the generic Redis retry budget cannot exhaust before the
// archive_scan.retry_backoff_sec schedule does.
func (w *ArchiveScanWorker) Handle(ctx context.Context, msg queue.Message) error {
	request, err := archiveScanRequestFromMessage(msg)
	if err != nil {
		return err
	}

	for {
		retryNow, retryAt, attemptErr := w.handleAttempt(ctx, request.JobID)
		if attemptErr != nil {
			return attemptErr
		}
		if retryNow {
			continue
		}
		if retryAt == nil {
			return w.markConsumed(ctx, msg, request.EventID)
		}
		if err := waitForArchiveRetry(ctx, *retryAt); err != nil {
			return err
		}
	}
}

func (w *ArchiveScanWorker) markConsumed(ctx context.Context, msg queue.Message, eventID int64) error {
	if w.db == nil {
		return nil
	}
	if eventID <= 0 {
		eventID = InboxEventID(msg.Group, msg)
	}
	if err := MarkConsumedInbox(ctx, w.db, msg.Group, eventID); err != nil {
		return sanitizedWorkerError("archive_scan_inbox_failed")
	}
	return nil
}

func (w *ArchiveScanWorker) handleAttempt(ctx context.Context, jobID int64) (retryNow bool, retryAt *time.Time, err error) {
	job, err := w.repository.GetJob(ctx, jobID)
	if err != nil {
		return false, nil, sanitizedWorkerError("archive_scan_job_lookup_failed")
	}
	if job == nil || job.ID <= 0 || job.AttachmentID <= 0 {
		return false, nil, ErrArchiveScanMalformedMessage
	}
	if job.Status == model.ScanStatusClean {
		if err := w.notifyArchiveScanClean(ctx, job.AttachmentID); err != nil {
			return false, nil, sanitizedWorkerError("archive_scan_completion_failed")
		}
		return false, nil, nil
	}
	if job.Status == model.ScanStatusManualReview {
		return false, nil, nil
	}
	if job.Status == model.ScanStatusBlocked {
		if err := w.cleanupBlockedObject(ctx, job); err != nil {
			return false, nil, sanitizedWorkerError("quarantine_delete_failed")
		}
		return false, nil, nil
	}
	if job.Status == model.ScanStatusFailed {
		if job.NextAttemptAt == nil {
			return false, nil, nil
		}
		if job.NextAttemptAt.After(time.Now()) {
			next := *job.NextAttemptAt
			return false, &next, nil
		}
		if err := w.repository.Retry(ctx, job.ID); err != nil {
			if errors.Is(err, repository.ErrArchiveScanRetryExhausted) {
				return false, nil, nil
			}
			return false, nil, sanitizedWorkerError("archive_scan_retry_failed")
		}
		return true, nil, nil
	}
	if job.Status != model.ScanStatusPending {
		if job.Status == model.ScanStatusScanning && job.StartedAt != nil && time.Since(*job.StartedAt) >= w.timeout {
			if recovery, ok := w.repository.(archiveScanRecoveryRepository); ok {
				if err := recovery.ResetScanning(ctx, job.ID); err != nil {
					return false, nil, sanitizedWorkerError("archive_scan_recovery_failed")
				}
				return true, nil, nil
			}
		}
		return false, nil, ErrArchiveScanNotReady
	}

	if err := w.repository.StartScan(ctx, job.ID); err != nil {
		return false, nil, sanitizedWorkerError("archive_scan_start_failed")
	}
	scanStarted := time.Now()
	attachment, err := w.repository.GetAttachmentScanState(ctx, job.AttachmentID)
	if err != nil || attachment == nil || strings.TrimSpace(attachment.OSSKey) == "" {
		return w.fail(ctx, job.ID, scanStarted, model.ScanAttemptResultError, "archive_object_key_unavailable", "", "", "")
	}

	object, err := w.objects.Open(attachment.OSSKey)
	if err != nil {
		return w.fail(ctx, job.ID, scanStarted, model.ScanAttemptResultError, "archive_object_open_failed", "", "", "")
	}

	scanCtx, cancel := context.WithTimeout(ctx, w.timeout)
	started := time.Now()
	version, versionErr := w.scanner.Version(scanCtx)
	if versionErr != nil {
		if closeErr := object.Close(); closeErr != nil {
			slog.Warn("archive scan object close failed", "error_code", "archive_object_close_failed")
		}
		cancel()
		return w.fail(ctx, job.ID, scanStarted, model.ScanAttemptResultError, scannerErrorCode(versionErr), "", "", "")
	}

	digest := sha256.New()
	result, scanErr := w.scanner.Scan(scanCtx, io.TeeReader(object, digest))
	closeErr := object.Close()
	cancel()
	if scanErr != nil {
		return w.fail(ctx, job.ID, scanStarted, model.ScanAttemptResultError, scannerErrorCode(scanErr), version.Engine, version.Signatures, "")
	}
	if closeErr != nil {
		return w.fail(ctx, job.ID, scanStarted, model.ScanAttemptResultError, "archive_object_read_failed", version.Engine, version.Signatures, "")
	}

	durationMs := int(time.Since(started).Milliseconds())
	switch result.Status {
	case clamav.StatusClean:
		attempt := &model.ArchiveScanAttempt{
			ScanJobID: job.ID, Result: model.ScanAttemptResultClean, DurationMs: durationMs,
			EngineVersion: version.Engine, SignatureVersion: version.Signatures,
		}
		objectSHA256 := fmt.Sprintf("%x", digest.Sum(nil))
		if atomic, ok := w.repository.(archiveScanAtomicOutcomeRepository); ok {
			if err := atomic.FinishCleanWithAttempt(ctx, job.ID, attempt, version.Engine, version.Signatures, objectSHA256); err != nil {
				w.resetScanningAfterFailure(ctx, job.ID)
				return false, nil, sanitizedWorkerError("archive_scan_finish_failed")
			}
		} else {
			if err := w.repository.AppendAttempt(ctx, attempt); err != nil {
				return false, nil, sanitizedWorkerError("archive_scan_audit_failed")
			}
			if err := w.repository.FinishClean(ctx, job.ID, version.Engine, version.Signatures, objectSHA256); err != nil {
				w.resetScanningAfterFailure(ctx, job.ID)
				return false, nil, sanitizedWorkerError("archive_scan_finish_failed")
			}
		}
		if err := w.notifyArchiveScanClean(ctx, job.AttachmentID); err != nil {
			return false, nil, sanitizedWorkerError("archive_scan_completion_failed")
		}
		return false, nil, nil
	case clamav.StatusBlocked:
		quarantineKey := fmt.Sprintf("quarantine/archive-scan/%d/%d/%d", job.AttachmentID, job.ScanVersion, job.ID)
		if err := w.objects.Copy(attachment.OSSKey, quarantineKey); err != nil {
			return w.fail(ctx, job.ID, scanStarted, model.ScanAttemptResultError, "quarantine_copy_failed", version.Engine, version.Signatures, result.DetectionName)
		}
		attempt := &model.ArchiveScanAttempt{
			ScanJobID: job.ID, Result: model.ScanAttemptResultBlocked, DurationMs: durationMs,
			EngineVersion: version.Engine, SignatureVersion: version.Signatures, DetectionName: result.DetectionName,
		}
		objectSHA256 := fmt.Sprintf("%x", digest.Sum(nil))
		if atomic, ok := w.repository.(archiveScanAtomicOutcomeRepository); ok {
			if err := atomic.BlockWithAttempt(ctx, job.ID, attempt, result.DetectionName, quarantineKey, objectSHA256); err != nil {
				w.resetScanningAfterFailure(ctx, job.ID)
				return false, nil, sanitizedWorkerError("archive_scan_block_failed")
			}
		} else {
			if err := w.repository.AppendAttempt(ctx, attempt); err != nil {
				return false, nil, sanitizedWorkerError("archive_scan_audit_failed")
			}
			if err := w.repository.Block(ctx, job.ID, result.DetectionName, quarantineKey, objectSHA256); err != nil {
				w.resetScanningAfterFailure(ctx, job.ID)
				return false, nil, sanitizedWorkerError("archive_scan_block_failed")
			}
		}
		blockedJob := *job
		blockedJob.QuarantineKey = quarantineKey
		if err := w.cleanupBlockedObject(ctx, &blockedJob); err != nil {
			return false, nil, sanitizedWorkerError("quarantine_delete_failed")
		}
		return false, nil, nil
	default:
		return w.fail(ctx, job.ID, scanStarted, model.ScanAttemptResultError, "clamd_invalid_result", version.Engine, version.Signatures, "")
	}
}

func (w *ArchiveScanWorker) cleanupBlockedObject(ctx context.Context, job *model.ArchiveScanJob) error {
	if strings.TrimSpace(job.QuarantineKey) == "" {
		return nil
	}
	attachment, err := w.repository.GetAttachmentScanState(ctx, job.AttachmentID)
	if err != nil || attachment == nil || strings.TrimSpace(attachment.OSSKey) == "" {
		return errors.New("blocked archive source unavailable")
	}
	return w.objects.Delete(attachment.OSSKey)
}

func (w *ArchiveScanWorker) notifyArchiveScanClean(ctx context.Context, attachmentID int64) error {
	if w.notifier == nil {
		return nil
	}
	return w.notifier.ArchiveScanClean(ctx, attachmentID)
}

func (w *ArchiveScanWorker) resetScanningAfterFailure(ctx context.Context, jobID int64) {
	recovery, ok := w.repository.(archiveScanRecoveryRepository)
	if !ok {
		return
	}
	if err := recovery.ResetScanning(ctx, jobID); err != nil {
		slog.Warn("archive scan recovery reset failed", "job_id", jobID, "error_code", "archive_scan_recovery_failed")
	}
}
func (w *ArchiveScanWorker) fail(ctx context.Context, jobID int64, started time.Time, result, errorCode, engine, signatures, detection string) (retryNow bool, retryAt *time.Time, err error) {
	attempt := &model.ArchiveScanAttempt{
		ScanJobID: jobID, Result: result, DurationMs: int(time.Since(started).Milliseconds()),
		EngineVersion: engine, SignatureVersion: signatures, DetectionName: detection, ErrorCode: errorCode,
	}
	if atomic, ok := w.repository.(archiveScanAtomicOutcomeRepository); ok {
		if err := atomic.FailWithAttempt(ctx, jobID, attempt, errorCode); err != nil {
			w.resetScanningAfterFailure(ctx, jobID)
			return false, nil, sanitizedWorkerError("archive_scan_fail_transition_failed")
		}
	} else {
		if err := w.repository.AppendAttempt(ctx, attempt); err != nil {
			return false, nil, sanitizedWorkerError("archive_scan_audit_failed")
		}
		if err := w.repository.Fail(ctx, jobID, errorCode); err != nil {
			w.resetScanningAfterFailure(ctx, jobID)
			return false, nil, sanitizedWorkerError("archive_scan_fail_transition_failed")
		}
	}
	job, err := w.repository.GetJob(ctx, jobID)
	if err != nil || job == nil {
		return false, nil, sanitizedWorkerError("archive_scan_retry_state_lookup_failed")
	}
	if job.NextAttemptAt == nil {
		return false, nil, nil
	}
	if job.NextAttemptAt.After(time.Now()) {
		next := *job.NextAttemptAt
		return false, &next, nil
	}
	if err := w.repository.Retry(ctx, jobID); err != nil {
		if errors.Is(err, repository.ErrArchiveScanRetryExhausted) {
			return false, nil, nil
		}
		return false, nil, sanitizedWorkerError("archive_scan_retry_failed")
	}
	return true, nil, nil
}

func waitForArchiveRetry(ctx context.Context, retryAt time.Time) error {
	delay := time.Until(retryAt)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type archiveScanRequest struct {
	JobID   int64
	EventID int64
}

func archiveScanRequestFromMessage(msg queue.Message) (archiveScanRequest, error) {
	var direct struct {
		JobID int64 `json:"job_id"`
	}
	if err := json.Unmarshal(msg.Payload, &direct); err == nil && direct.JobID > 0 {
		return archiveScanRequest{JobID: direct.JobID, EventID: InboxEventID(msg.Group, msg)}, nil
	}
	var envelope events.Envelope
	if err := json.Unmarshal(msg.Payload, &envelope); err != nil || envelope.EventType != ArchiveScanTopic || envelope.Validate() != nil || envelope.EventID <= 0 {
		return archiveScanRequest{}, ErrArchiveScanMalformedMessage
	}
	var archivePayload events.ArchiveScanEventPayload
	if err := json.Unmarshal(envelope.Payload, &archivePayload); err != nil || archivePayload.JobID <= 0 {
		return archiveScanRequest{}, ErrArchiveScanMalformedMessage
	}
	return archiveScanRequest{JobID: archivePayload.JobID, EventID: envelope.EventID}, nil
}

func scannerErrorCode(err error) string {
	switch {
	case errors.Is(err, clamav.ErrUnavailable):
		return "clamd_unavailable"
	case errors.Is(err, clamav.ErrScanTimeout):
		return "clamd_timeout"
	case errors.Is(err, clamav.ErrResponseTooLong):
		return "clamd_response_too_long"
	case errors.Is(err, clamav.ErrProtocol):
		return "clamd_protocol_error"
	case errors.Is(err, clamav.ErrScanFailed):
		return "clamd_scan_failed"
	default:
		return "clamd_scan_error"
	}
}

func sanitizedWorkerError(code string) error {
	slog.Warn("archive scan worker operation failed", "error_code", code)
	return fmt.Errorf("archive scan failed: %s", code)
}
