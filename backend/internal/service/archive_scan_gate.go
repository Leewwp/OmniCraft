package service

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"omnicraft/backend/internal/model"
)

var (
	// ErrArchiveNotClean is the stable business error used by publish and
	// download gates. It deliberately carries no scanner or detection detail.
	ErrArchiveNotClean    = errors.New("archive attachment is not clean")
	ErrArchiveScanPending = errors.New("archive scan is pending")
	ErrArchiveScanFailed  = errors.New("archive scan failed")
)

// ArchiveScanGate centralizes the feature flag and the clean-only policy for
// both publish and download paths. Non-archive attachments stay not_required;
// every attachment that requires scanning must be clean before release.
type ArchiveScanGate struct {
	db      *gorm.DB
	enabled bool
}

func NewArchiveScanGate(db *gorm.DB, enabled bool) *ArchiveScanGate {
	return &ArchiveScanGate{db: db, enabled: enabled}
}

// RequireAttachmentClean checks the persisted attachment state immediately
// before an OSS URL can be signed.
func (g *ArchiveScanGate) RequireAttachmentClean(ctx context.Context, attachmentID int64) error {
	if g == nil {
		return nil
	}
	var attachment model.ContentAttachment
	if err := g.db.WithContext(ctx).Where("id = ?", attachmentID).First(&attachment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrArchiveNotClean
		}
		return err
	}
	// Quarantine objects are never valid delivery targets, even while the
	// feature flag is disabled. Disabling scanning only disables scan-status
	// enforcement for ordinary attachments.
	if strings.HasPrefix(strings.TrimSpace(attachment.OSSKey), "quarantine/") {
		return ErrArchiveNotClean
	}
	if !g.enabled {
		return nil
	}
	return requireCleanAttachment(attachment)
}

// RequireContentCleanTx is used inside the review transaction so a content
// status transition and its published event cannot outrun archive scanning.
func (g *ArchiveScanGate) RequireContentCleanTx(ctx context.Context, tx *gorm.DB, contentID int64) error {
	if g == nil {
		return nil
	}
	query := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("content_item_id = ?", contentID)
	if g.enabled {
		query = query.Where("scan_required = ? OR file_type = ?", true, "mod")
	}
	var attachments []model.ContentAttachment
	if err := query.Find(&attachments).Error; err != nil {
		return err
	}
	for _, attachment := range attachments {
		if strings.HasPrefix(strings.TrimSpace(attachment.OSSKey), "quarantine/") {
			return ErrArchiveNotClean
		}
	}
	if !g.enabled {
		return nil
	}
	for _, attachment := range attachments {
		if err := requireCleanAttachment(attachment); err != nil {
			return err
		}
	}
	return nil
}

func requireCleanAttachment(attachment model.ContentAttachment) error {
	// A quarantine object is never a valid delivery target, even if a stale or
	// manually-corrupted row claims clean.
	if strings.HasPrefix(strings.TrimSpace(attachment.OSSKey), "quarantine/") {
		return ErrArchiveNotClean
	}
	if !strings.HasPrefix(strings.TrimSpace(attachment.OSSKey), "uploads/") {
		return ErrArchiveNotClean
	}
	// A mod archive always requires a clean result when the feature is on.
	// This also prevents legacy mod rows that still say not_required from
	// bypassing the scan-then-publish/download policy.
	if attachment.FileType == "mod" || attachment.ScanRequired {
		if attachment.ScanStatus != model.ScanStatusClean {
			return archiveScanGateError(attachment.ScanStatus)
		}
		return nil
	}
	if attachment.ScanStatus == model.ScanStatusNotRequired {
		return nil
	}
	if attachment.ScanStatus != model.ScanStatusClean {
		return archiveScanGateError(attachment.ScanStatus)
	}
	return nil
}

func archiveScanGateError(status string) error {
	switch status {
	case model.ScanStatusFailed:
		return errors.Join(ErrArchiveNotClean, ErrArchiveScanFailed)
	case model.ScanStatusPending, model.ScanStatusScanning, model.ScanStatusManualReview, model.ScanStatusLegacyUnscanned:
		return errors.Join(ErrArchiveNotClean, ErrArchiveScanPending)
	default:
		return ErrArchiveNotClean
	}
}
