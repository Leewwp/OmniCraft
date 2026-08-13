package model

import "time"

// Scan status vocabulary shared by content_attachments.scan_status and
// archive_scan_jobs.status (archive malware scanning design §3). The
// attachment-level status mirrors the current job's status; not_required and
// legacy_unscanned only ever appear on attachments (non-mod objects and
// pre-migration mod archives respectively).
const (
	ScanStatusNotRequired     = "not_required"
	ScanStatusLegacyUnscanned = "legacy_unscanned"
	ScanStatusPending         = "pending"
	ScanStatusScanning        = "scanning"
	ScanStatusClean           = "clean"
	ScanStatusBlocked         = "blocked"
	ScanStatusFailed          = "failed"
	ScanStatusManualReview    = "manual_review"
)

// Scan attempt results recorded in the immutable archive_scan_attempts audit
// log. error_code values on error attempts are sanitized codes, never raw
// clamd output.
const (
	ScanAttemptResultClean   = "clean"
	ScanAttemptResultBlocked = "blocked"
	ScanAttemptResultError   = "error"
)

// ArchiveScanJob is one scan workflow for a single (attachment, scan_version)
// pair. The partial unique index uq_archive_scan_jobs_current guarantees at
// most one current job per pair while terminal statuses (clean, failed)
// release the slot for an admin-initiated re-scan.
type ArchiveScanJob struct {
	ID               int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
	AttachmentID     int64      `gorm:"not null;index" json:"attachment_id"`
	ScanVersion      int        `gorm:"not null;default:0" json:"scan_version"`
	Status           string     `gorm:"size:32;not null;default:pending" json:"status"`
	Attempts         int        `gorm:"not null;default:0" json:"attempts"`
	NextAttemptAt    *time.Time `json:"next_attempt_at,omitempty"`
	ObjectSHA256     string     `gorm:"size:64" json:"object_sha256,omitempty"`
	EngineVersion    string     `gorm:"size:64" json:"engine_version,omitempty"`
	SignatureVersion string     `gorm:"size:64" json:"signature_version,omitempty"`
	DetectionName    string     `gorm:"size:255" json:"detection_name,omitempty"`
	QuarantineKey    string     `gorm:"type:text" json:"quarantine_key,omitempty"`
	ErrorCode        string     `gorm:"size:64" json:"error_code,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
}

func (ArchiveScanJob) TableName() string { return "archive_scan_jobs" }

// ArchiveScanAttempt is one immutable audit record inside a scan job. The
// repository only appends: attempt_no is assigned monotonically per job and
// there is no update or delete path (design §3: "不可变审计记录").
type ArchiveScanAttempt struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
	ScanJobID        int64     `gorm:"not null;index" json:"scan_job_id"`
	AttemptNo        int       `gorm:"not null" json:"attempt_no"`
	Result           string    `gorm:"size:16;not null" json:"result"`
	DurationMs       int       `gorm:"not null" json:"duration_ms"`
	EngineVersion    string    `gorm:"size:64" json:"engine_version,omitempty"`
	SignatureVersion string    `gorm:"size:64" json:"signature_version,omitempty"`
	DetectionName    string    `gorm:"size:255" json:"detection_name,omitempty"`
	ErrorCode        string    `gorm:"size:64" json:"error_code,omitempty"`
}

func (ArchiveScanAttempt) TableName() string { return "archive_scan_attempts" }
