package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/repository"
)

// SP-16 #451: download sentinel errors. The REST handler maps these onto the
// exact HTTP codes it always produced; the MCP tool omnicraft_request_download
// surfaces them as tool errors so both channels share one implementation.
var (
	ErrDownloadUnauthorized    = errors.New("login required")
	ErrDownloadNotPublished    = errors.New("content not available for download")
	ErrDownloadUnavailable     = errors.New("content is unavailable")
	ErrDownloadNotAllowed      = errors.New("download not allowed")
	ErrNoAttachments           = errors.New("no downloadable files")
	ErrInvalidAttachmentID     = errors.New("invalid attachment_id")
	ErrAttachmentMismatch      = errors.New("attachment does not belong to this content")
	ErrAmbiguousAttachment     = errors.New("specify attachment_id; cannot determine a unique primary attachment")
	ErrDownloadPresignFailed   = errors.New("failed to generate download url")
	ErrDownloadStatusMalformed = errors.New("account status unavailable")
)

// DownloadURLSigner abstracts the OSS presign step so RequestDownload stays
// hermetically testable (production wiring uses the real *OSSService).
type DownloadURLSigner interface {
	GeneratePresignDownloadURL(ctx context.Context, ossKey string, ttl time.Duration) (string, error)
}

// WithDownloadSigner overrides the presign implementation (tests only; the
// default resolves to the configured OSS service).
func (s *ContentService) WithDownloadSigner(signer DownloadURLSigner) *ContentService {
	s.downloadSigner = signer
	return s
}

// DownloadResult carries everything the REST response and the MCP tool shape
// from one authorized download grant.
type DownloadResult struct {
	URL         string
	ExpiresIn   int64
	ContentType string
	Attachment  *model.ContentAttachment
}

// RequestDownload is the single download orchestration behind both
// GET /contents/:id/download and the MCP tool omnicraft_request_download:
// visibility scope, AllowCopy, archive malware gate, short-TTL presign URL and
// the metered download count. Semantics moved verbatim from the handler in
// SP-16 #451 — do not fork them per channel.
func (s *ContentService) RequestDownload(ctx context.Context, callerID int64, contentID int64, attachmentIDStr string) (*DownloadResult, error) {
	if callerID == 0 {
		return nil, ErrDownloadUnauthorized
	}

	content, err := s.contentRepo.FindByID(contentID)
	if err != nil || content == nil {
		return nil, ErrContentNotFound
	}

	if content.Status != "published" {
		return nil, ErrDownloadNotPublished
	}

	var visibleCount int64
	if err := repository.ApplyContentVisibilityScope(s.contentRepo.DB().WithContext(ctx).Model(&model.ContentItem{}), callerID).
		Where("content_items.id = ?", contentID).
		Count(&visibleCount).Error; err != nil {
		return nil, err
	}
	if visibleCount == 0 {
		return nil, ErrDownloadUnavailable
	}

	if !content.AllowCopy {
		return nil, ErrDownloadNotAllowed
	}

	if s.ossSvc == nil && s.downloadSigner == nil {
		return nil, ErrOSSNotConfigured
	}

	attachments, _ := s.contentRepo.GetAttachments(contentID)
	if len(attachments) == 0 {
		return nil, ErrNoAttachments
	}

	var target *model.ContentAttachment
	if attachmentIDStr != "" {
		attachmentID, parseErr := strconv.ParseInt(attachmentIDStr, 10, 64)
		if parseErr != nil {
			return nil, ErrInvalidAttachmentID
		}
		for i := range attachments {
			if attachments[i].ID == attachmentID {
				target = &attachments[i]
				break
			}
		}
		if target == nil {
			return nil, ErrAttachmentMismatch
		}
	} else {
		var primaries []int
		for i := range attachments {
			if attachments[i].IsPrimary != nil && *attachments[i].IsPrimary {
				primaries = append(primaries, i)
			}
		}
		if len(primaries) == 1 {
			target = &attachments[primaries[0]]
		} else {
			return nil, ErrAmbiguousAttachment
		}
	}

	gate := NewArchiveScanGate(s.contentRepo.DB(), s.archiveScanEnabled)
	if err := gate.RequireAttachmentClean(ctx, target.ID); err != nil {
		return nil, err
	}

	ttl := downloadURLTTL(s.ossDownloadTTLSec(), s.archiveScanEnabled, s.archiveScanTTLSec(), *target)

	signer := s.downloadSigner
	if signer == nil {
		signer = s.ossSvc
	}
	url, err := signer.GeneratePresignDownloadURL(ctx, target.OSSKey, ttl)
	if err != nil {
		return nil, ErrDownloadPresignFailed
	}

	s.recordDownloadCount(contentID)

	return &DownloadResult{URL: url, ExpiresIn: int64(ttl.Seconds()), ContentType: content.ContentType, Attachment: target}, nil
}

// recordDownloadCount mirrors the historical metering: the async queue when a
// real producer is wired, the redis sorted-set fallback otherwise.
func (s *ContentService) recordDownloadCount(contentID int64) {
	if _, ok := s.queueProducer.(*queue.NoopProducer); !ok && s.queueProducer != nil {
		recovery.GoSafe(func() {
			payload, _ := json.Marshal(map[string]interface{}{
				"content_id": contentID,
				"action":     "download",
			})
			if err := s.queueProducer.Publish(context.Background(), "count.download", payload); err != nil {
				slog.Error("failed to publish download count message", "content_id", contentID, "error", err)
			}
		})
	} else if s.rdb != nil {
		recovery.GoSafe(func() {
			ctx := context.Background()
			s.rdb.ZIncrBy(ctx, "rank:download:counts", 1, fmt.Sprintf("%d", contentID))
		})
	}
}

func (s *ContentService) ossDownloadTTLSec() int {
	if s.ossSvc != nil && s.ossSvc.cfg != nil && s.ossSvc.cfg.OSS.DownloadURLTTL > 0 {
		return s.ossSvc.cfg.OSS.DownloadURLTTL
	}
	return 0
}

func (s *ContentService) archiveScanTTLSec() int {
	if s.archiveScanCfg != nil {
		return s.archiveScanCfg.URLTTLSec
	}
	return 0
}

// downloadURLTTL is the shared presign TTL policy (moved from the handler in
// SP-16 #451): the configured OSS default, tightened and capped at 300s for
// scanned archive targets, always positive.
func downloadURLTTL(ossTTLSec int, archiveScanEnabled bool, archiveScanTTLSec int, attachment model.ContentAttachment) time.Duration {
	ttlSec := 300
	if ossTTLSec > 0 {
		ttlSec = ossTTLSec
	}
	if archiveScanEnabled && (attachment.FileType == "mod" || attachment.ScanRequired) {
		scanTTL := archiveScanTTLSec
		if scanTTL <= 0 || scanTTL > 300 {
			scanTTL = 300
		}
		ttlSec = scanTTL
	}
	if ttlSec <= 0 {
		ttlSec = 300
	}
	return time.Duration(ttlSec) * time.Second
}
