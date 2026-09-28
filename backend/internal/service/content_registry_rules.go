package service

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/archivezip"
)

// Registry-driven publish rules (#688): binding (allowed attachment
// families), attachment_policy (required_any_of) and the scannable family
// set all read the dual-axis registry through the nil-safe Effective*
// accessors, so legacy constructions (registryCfg == nil) fall back to the
// shipped baseline.

// defaultDocumentQuota mirrors archive_scan.* on the shipped config for
// constructions without an ArchiveScanConfig; document packages are small
// OPC zips, these bounds are generous.
var defaultDocumentQuota = archivezip.Quota{
	MaxZipEntries:          5000,
	MaxEntryUncompressedMB: 200,
	MaxTotalUncompressedMB: 2048,
	MaxRecursionDepth:      10,
}

// allowedAttachmentFamily reports whether family is in the content type's
// registry-declared upload_file_types set.
func (s *ContentService) allowedAttachmentFamily(contentType, family string) bool {
	return allowedAttachmentFamily(s.registryCfg, contentType, family)
}

func allowedAttachmentFamily(cfg *config.Config, contentType, family string) bool {
	entry, ok := cfg.ContentTypeLookup(contentType)
	if !ok {
		return false
	}
	for _, allowed := range entry.UploadFileTypes {
		if allowed == family {
			return true
		}
	}
	return false
}

// isScannableAttachmentFamily reports whether the family joins the ClamAV
// pipeline (registry capability axis; mod + document + audio today).
func isScannableAttachmentFamily(cfg *config.Config, family string) bool {
	entry, ok := cfg.UploadFileTypeLookup(family)
	return ok && entry.Scannable
}

// evaluateAttachmentPolicy enforces the registry's attachment_policy
// required_any_of: at least one consumed attachment must belong to a
// required family. Types without a policy pass unchanged.
func evaluateAttachmentPolicy(cfg *config.Config, contentType string, attachments []model.ContentAttachment) error {
	entry, ok := cfg.ContentTypeLookup(contentType)
	if !ok || entry.AttachmentPolicy == nil || len(entry.AttachmentPolicy.RequiredAnyOf) == 0 {
		return nil
	}
	required := make(map[string]bool, len(entry.AttachmentPolicy.RequiredAnyOf))
	for _, family := range entry.AttachmentPolicy.RequiredAnyOf {
		required[family] = true
	}
	for _, attachment := range attachments {
		if required[attachment.FileType] {
			return nil
		}
	}
	return fmt.Errorf("%w: %s requires one of %v", ErrAttachmentPolicyRequired, contentType, entry.AttachmentPolicy.RequiredAnyOf)
}

// validateDocumentAttachment runs the #688 document content-level chain for
// one consumed grant: OPC structure + package identity + macro detection for
// .docx/.xlsx, text sanity for .csv. Not gated by the archive-scan feature
// flag: package identity and macro rejection are type safety, not malware
// verdicts, and must hold with the scanner disabled too.
func (s *ContentService) validateDocumentAttachment(ctx context.Context, grant UploadGrant) error {
	validator := s.documentValidator
	if validator == nil {
		if s.ossSvc == nil {
			return ErrOSSNotConfigured
		}
		validator = s.ossSvc
	}
	ext := extensionOfGrant(grant)
	switch ext {
	case ".csv":
		if err := validator.ValidateCSVTextSanity(ctx, grant.OSSKey); err != nil {
			return fmt.Errorf("%w: %v", ErrUploadGrantInvalid, err)
		}
		return nil
	case ".docx", ".xlsx":
		quota := defaultDocumentQuota
		if s.archiveScanCfg != nil {
			quota = archivezip.QuotaFromConfig(*s.archiveScanCfg)
		}
		if err := validator.ValidateDocumentPackage(ctx, grant.OSSKey, grant.FileSize, ext, quota); err != nil {
			var validationErr *UploadValidationError
			if errors.As(err, &validationErr) {
				return fmt.Errorf("%w: %v", ErrUploadGrantInvalid, err)
			}
			return err
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported document extension %q", ErrUploadGrantInvalid, ext)
	}
}

// validateAudioAttachment asserts the uploaded object's header magic
// matches the claimed extension (#688): a renamed binary cannot ride the
// audio family even with a spoofed content type on the PUT.
func (s *ContentService) validateAudioAttachment(ctx context.Context, grant UploadGrant) error {
	source := s.audioHeaderSource
	if source == nil {
		if s.ossSvc == nil {
			return ErrOSSNotConfigured
		}
		source = s.ossSvc
	}
	ext := extensionOfGrant(grant)
	if !isAllowedAudioExt(ext) {
		return fmt.Errorf("%w: unsupported audio extension %q", ErrUploadGrantInvalid, ext)
	}
	header, err := source.ReadAudioHeader(ctx, grant.OSSKey)
	if err != nil {
		return err
	}
	if !MatchesAudioMagic(ext, header) {
		return fmt.Errorf("%w: audio content does not match the %s format", ErrUploadGrantInvalid, ext)
	}
	return nil
}

// extensionOfGrant derives the attachment extension from the server-locked
// original file name, falling back to the OSS key (buildOSSKey preserves
// the extension).
func extensionOfGrant(grant UploadGrant) string {
	name := grant.OriginalFileName
	if name == "" {
		name = grant.OSSKey
	}
	return strings.ToLower(path.Ext(name))
}
