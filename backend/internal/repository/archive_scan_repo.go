package repository

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/events"
)

var (
	// ErrArchiveScanNotFound is returned when a job or attachment does not exist.
	ErrArchiveScanNotFound = errors.New("archive scan job not found")
	// ErrArchiveScanIllegalState is returned when a transition does not match
	// the fixed state machine (design §3):
	// pending -> scanning -> clean | blocked | failed; failed -> pending
	// (bounded retry); blocked -> manual_review -> clean | blocked.
	ErrArchiveScanIllegalState = errors.New("illegal archive scan state transition")
	// ErrArchiveScanJobExists is returned when a current job already exists for
	// the (attachment_id, scan_version) pair.
	ErrArchiveScanJobExists = errors.New("archive scan job already exists for attachment version")
	// ErrArchiveScanRetryExhausted is returned by Retry when the retry budget
	// (archive_scan.retry_backoff_sec) is spent: the job stays failed and is
	// never auto-released nor auto-promoted to manual_review.
	ErrArchiveScanRetryExhausted = errors.New("archive scan retry budget exhausted")
	// ErrArchiveScanNotScannable is returned when CreateJob targets an
	// attachment that must never be scanned (scan_status = not_required).
	ErrArchiveScanNotScannable = errors.New("attachment is not scannable")
)

// ArchiveScanRetryPolicy is the bounded retry schedule from
// archive_scan.retry_backoff_sec. Each entry schedules the next attempt after
// a scan failure; once the schedule is spent the job stays failed with a
// warning (no auto-release). An empty schedule means "no retries".
type ArchiveScanRetryPolicy struct {
	Backoff []time.Duration
}

// ArchiveScanRepository owns the archive malware scanning schema: the scan
// columns on content_attachments, archive_scan_jobs and the append-only
// archive_scan_attempts audit log (design §3). The worker (S03) and the
// publish/download gates (S04) are the consumers; this ticket (S01) only
// implements schema, state machine, storage and audit.
//
// Every transition updates the job row and mirrors scan_status on the
// attachment inside one transaction; the UPDATE filters on the current status
// so a stale or concurrent caller affects zero rows and the transition is
// rejected instead of silently accepted. CreateJob serializes on the
// attachment row (FOR UPDATE) so check-then-insert of the current job is
// race-free; uq_archive_scan_jobs_current is the database-level backstop.
type ArchiveScanRepository struct {
	db     *gorm.DB
	policy ArchiveScanRetryPolicy
	outbox OutboxWriter
}

func NewArchiveScanRepository(db *gorm.DB, policy ArchiveScanRetryPolicy) *ArchiveScanRepository {
	return NewArchiveScanRepositoryWithOutbox(db, policy, nil)
}

// NewArchiveScanRepositoryWithOutbox wires the transactional outbox used by
// production upload flows. The legacy constructor remains available for
// schema/state-machine callers that do not create jobs from an upload path.
func NewArchiveScanRepositoryWithOutbox(db *gorm.DB, policy ArchiveScanRetryPolicy, outbox OutboxWriter) *ArchiveScanRepository {
	return &ArchiveScanRepository{db: db, policy: policy, outbox: outbox}
}

// CreateJob starts a scan for attachmentID at scanVersion: it creates a
// pending job and moves the attachment to pending (scan_required=true,
// scan_version=scanVersion) in the same transaction. Preconditions: the
// attachment exists and is scannable (scan_status != not_required), and no
// current job exists for the (attachment_id, scan_version) pair.
func (r *ArchiveScanRepository) CreateJob(ctx context.Context, attachmentID int64, scanVersion int) (*model.ArchiveScanJob, error) {
	var job model.ArchiveScanJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var attachment model.ContentAttachment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", attachmentID).
			First(&attachment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrArchiveScanNotFound
			}
			return err
		}
		if attachment.ScanStatus == model.ScanStatusNotRequired {
			return ErrArchiveScanNotScannable
		}
		var current int64
		if err := tx.Model(&model.ArchiveScanJob{}).
			Where("attachment_id = ? AND scan_version = ? AND status NOT IN ?",
				attachmentID, scanVersion, []string{model.ScanStatusClean, model.ScanStatusFailed}).
			Count(&current).Error; err != nil {
			return err
		}
		if current > 0 {
			return ErrArchiveScanJobExists
		}
		job = model.ArchiveScanJob{
			AttachmentID: attachmentID,
			ScanVersion:  scanVersion,
			Status:       model.ScanStatusPending,
		}
		if err := tx.Create(&job).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ContentAttachment{}).
			Where("id = ?", attachmentID).
			Updates(map[string]interface{}{
				"scan_status":   model.ScanStatusPending,
				"scan_required": true,
				"scan_version":  scanVersion,
			}).Error; err != nil {
			return err
		}
		if r.outbox != nil {
			traceparent, tracestate := events.FromContext(ctx)
			envelope, err := events.NewArchiveScanEnvelope(attachmentID, job.ID, traceparent, tracestate)
			if err != nil {
				return err
			}
			row := events.ToOutboxEvent(envelope)
			if err := r.outbox.CreateTx(ctx, tx, &row); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// StartScan moves a pending job (and its attachment) to scanning.
func (r *ArchiveScanRepository) StartScan(ctx context.Context, jobID int64) error {
	now := time.Now()
	return r.transition(ctx, jobID, model.ScanStatusPending,
		func(job *model.ArchiveScanJob) (string, map[string]interface{}, map[string]interface{}, error) {
			return model.ScanStatusScanning,
				map[string]interface{}{"started_at": now},
				map[string]interface{}{}, nil
		})
}

// FinishClean records a clean scan outcome: job and attachment become clean,
// with engine/signature versions and the scanned object hash as evidence, and
// the attachment records scanned_at + last_scan_job_id.
func (r *ArchiveScanRepository) FinishClean(ctx context.Context, jobID int64, engineVersion, signatureVersion, objectSHA256 string) error {
	now := time.Now()
	return r.transition(ctx, jobID, model.ScanStatusScanning,
		func(job *model.ArchiveScanJob) (string, map[string]interface{}, map[string]interface{}, error) {
			return model.ScanStatusClean,
				map[string]interface{}{
					"finished_at":       now,
					"engine_version":    engineVersion,
					"signature_version": signatureVersion,
					"object_sha256":     objectSHA256,
				},
				map[string]interface{}{
					"scanned_at":       now,
					"last_scan_job_id": job.ID,
				}, nil
		})
}

// Block records a malware hit: job and attachment become blocked with the
// detection name and quarantine key (the object was copied out and the
// original key removed by the worker; this ticket only persists the state).
func (r *ArchiveScanRepository) Block(ctx context.Context, jobID int64, detectionName, quarantineKey, objectSHA256 string) error {
	now := time.Now()
	return r.transition(ctx, jobID, model.ScanStatusScanning,
		func(job *model.ArchiveScanJob) (string, map[string]interface{}, map[string]interface{}, error) {
			return model.ScanStatusBlocked,
				map[string]interface{}{
					"finished_at":    now,
					"detection_name": detectionName,
					"quarantine_key": quarantineKey,
					"object_sha256":  objectSHA256,
				},
				map[string]interface{}{}, nil
		})
}

// Fail records a scan engine/network error: the job and attachment become
// failed. While the retry budget lasts, next_attempt_at is scheduled from
// the backoff schedule; when the budget is spent the job stays failed and a
// warning is logged (never auto-released, never auto-promoted to review).
func (r *ArchiveScanRepository) Fail(ctx context.Context, jobID int64, errorCode string) error {
	return r.transition(ctx, jobID, model.ScanStatusScanning,
		func(job *model.ArchiveScanJob) (string, map[string]interface{}, map[string]interface{}, error) {
			jobUpdates := map[string]interface{}{"error_code": errorCode}
			if job.Attempts < len(r.policy.Backoff) {
				jobUpdates["next_attempt_at"] = time.Now().Add(r.policy.Backoff[job.Attempts])
			} else {
				slog.Warn("archive scan retry budget exhausted, job stays failed",
					"job_id", job.ID, "attachment_id", job.AttachmentID, "attempts", job.Attempts)
			}
			return model.ScanStatusFailed, jobUpdates, map[string]interface{}{}, nil
		})
}

// Retry moves a failed job back to pending for one more attempt, incrementing
// attempts and consuming the scheduled next_attempt_at. It is only legal while
// the retry budget lasts; otherwise it returns ErrArchiveScanRetryExhausted
// and the job stays failed.
func (r *ArchiveScanRepository) Retry(ctx context.Context, jobID int64) error {
	return r.transition(ctx, jobID, model.ScanStatusFailed,
		func(job *model.ArchiveScanJob) (string, map[string]interface{}, map[string]interface{}, error) {
			if job.NextAttemptAt == nil {
				return "", nil, nil, ErrArchiveScanRetryExhausted
			}
			return model.ScanStatusPending,
				map[string]interface{}{
					"attempts":        job.Attempts + 1,
					"next_attempt_at": nil,
					"error_code":      "",
				},
				map[string]interface{}{}, nil
		})
}

// StartManualReview promotes a blocked job to manual_review. Per design §3 it
// is only reachable from blocked and must be initiated by an admin (the
// admin guard is S04; the repository enforces the transition source).
func (r *ArchiveScanRepository) StartManualReview(ctx context.Context, jobID int64) error {
	return r.transition(ctx, jobID, model.ScanStatusBlocked,
		func(job *model.ArchiveScanJob) (string, map[string]interface{}, map[string]interface{}, error) {
			return model.ScanStatusManualReview, map[string]interface{}{}, map[string]interface{}{}, nil
		})
}

// ResolveManualReview closes an admin review with either outcome:
// ScanStatusClean (false positive; attachment gets scanned_at +
// last_scan_job_id) or ScanStatusBlocked (confirmed malware). Any other
// outcome is rejected as an illegal transition.
func (r *ArchiveScanRepository) ResolveManualReview(ctx context.Context, jobID int64, outcome string) error {
	if outcome != model.ScanStatusClean && outcome != model.ScanStatusBlocked {
		return ErrArchiveScanIllegalState
	}
	now := time.Now()
	return r.transition(ctx, jobID, model.ScanStatusManualReview,
		func(job *model.ArchiveScanJob) (string, map[string]interface{}, map[string]interface{}, error) {
			jobUpdates := map[string]interface{}{"finished_at": now}
			attachmentUpdates := map[string]interface{}{}
			if outcome == model.ScanStatusClean {
				attachmentUpdates["scanned_at"] = now
				attachmentUpdates["last_scan_job_id"] = job.ID
			}
			return outcome, jobUpdates, attachmentUpdates, nil
		})
}

// AppendAttempt appends one immutable audit record to a job. attempt_no is
// assigned monotonically per job (1, 2, 3, ...) under a job-row lock so the
// audit log can never silently renumber; uq_archive_scan_attempts_job_no is
// the database backstop. There is no update or delete path.
func (r *ArchiveScanRepository) AppendAttempt(ctx context.Context, attempt *model.ArchiveScanAttempt) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job model.ArchiveScanJob
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", attempt.ScanJobID).
			First(&job).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrArchiveScanNotFound
			}
			return err
		}
		var maxNo int
		if err := tx.Model(&model.ArchiveScanAttempt{}).
			Where("scan_job_id = ?", attempt.ScanJobID).
			Select("COALESCE(MAX(attempt_no), 0)").
			Scan(&maxNo).Error; err != nil {
			return err
		}
		attempt.AttemptNo = maxNo + 1
		return tx.Create(attempt).Error
	})
}

// GetJob returns one job by id.
func (r *ArchiveScanRepository) GetJob(ctx context.Context, jobID int64) (*model.ArchiveScanJob, error) {
	var job model.ArchiveScanJob
	if err := r.db.WithContext(ctx).Where("id = ?", jobID).First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrArchiveScanNotFound
		}
		return nil, err
	}
	return &job, nil
}

// GetCurrentJob returns the current job for an (attachment_id, scan_version)
// pair — a job whose status is neither clean nor failed.
func (r *ArchiveScanRepository) GetCurrentJob(ctx context.Context, attachmentID int64, scanVersion int) (*model.ArchiveScanJob, error) {
	var job model.ArchiveScanJob
	if err := r.db.WithContext(ctx).
		Where("attachment_id = ? AND scan_version = ? AND status NOT IN ?",
			attachmentID, scanVersion, []string{model.ScanStatusClean, model.ScanStatusFailed}).
		First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrArchiveScanNotFound
		}
		return nil, err
	}
	return &job, nil
}

// ListJobsByAttachment returns all jobs for an attachment, newest first.
func (r *ArchiveScanRepository) ListJobsByAttachment(ctx context.Context, attachmentID int64) ([]model.ArchiveScanJob, error) {
	var jobs []model.ArchiveScanJob
	err := r.db.WithContext(ctx).
		Where("attachment_id = ?", attachmentID).
		Order("created_at DESC, id DESC").
		Find(&jobs).Error
	return jobs, err
}

// ListAttemptsByJob returns the immutable audit trail for a job in
// chronological attempt order.
func (r *ArchiveScanRepository) ListAttemptsByJob(ctx context.Context, jobID int64) ([]model.ArchiveScanAttempt, error) {
	var attempts []model.ArchiveScanAttempt
	err := r.db.WithContext(ctx).
		Where("scan_job_id = ?", jobID).
		Order("attempt_no ASC").
		Find(&attempts).Error
	return attempts, err
}

// GetAttachmentScanState returns a content attachment row including its scan
// columns (scan_status, scan_required, scan_version, last_scan_job_id,
// scanned_at) for download/publish gates.
func (r *ArchiveScanRepository) GetAttachmentScanState(ctx context.Context, attachmentID int64) (*model.ContentAttachment, error) {
	var attachment model.ContentAttachment
	if err := r.db.WithContext(ctx).Where("id = ?", attachmentID).First(&attachment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrArchiveScanNotFound
		}
		return nil, err
	}
	return &attachment, nil
}

// transition applies one fixed state-machine edge to a job and mirrors the
// new scan_status on its attachment inside a single transaction. The mutate
// callback decides the edge (next status + extra job/attachment column
// updates); returning ErrArchiveScanRetryExhausted aborts without changing
// anything. Both UPDATEs filter on the expected current status, so illegal or
// stale transitions affect zero rows and are rejected.
func (r *ArchiveScanRepository) transition(ctx context.Context, jobID int64, wantStatus string,
	mutate func(job *model.ArchiveScanJob) (string, map[string]interface{}, map[string]interface{}, error)) error {

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job model.ArchiveScanJob
		if err := tx.Where("id = ?", jobID).First(&job).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrArchiveScanNotFound
			}
			return err
		}
		if job.Status != wantStatus {
			return ErrArchiveScanIllegalState
		}
		nextStatus, jobUpdates, attachmentUpdates, err := mutate(&job)
		if err != nil {
			return err
		}
		jobUpdates["status"] = nextStatus
		if err := tx.Model(&model.ArchiveScanJob{}).
			Where("id = ? AND status = ?", job.ID, wantStatus).
			Updates(jobUpdates).Error; err != nil {
			return err
		}
		attachmentUpdates["scan_status"] = nextStatus
		result := tx.Model(&model.ContentAttachment{}).
			Where("id = ? AND scan_status = ?", job.AttachmentID, wantStatus).
			Updates(attachmentUpdates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrArchiveScanIllegalState
		}
		return nil
	})
}
