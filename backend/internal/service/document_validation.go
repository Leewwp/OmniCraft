package service

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"omnicraft/backend/internal/pkg/archivezip"
)

// Document package validation chain (#688): after VerifyUploadedObject, a
// document-family object must prove it is the OPC package its extension
// claims — structure (archivezip quotas, nesting, encryption, traversal),
// package identity ([Content_Types].xml + the format's root part) and macro
// content (vbaProject.bin entry or macroEnabled content types). This moves
// "renamed zip passes as docx" failures from the viewer's parse stage to the
// publish boundary.

var (
	// ErrDocumentPackage rejects an object that is structurally fine but not
	// the claimed document kind, or that carries macro content.
	ErrDocumentPackage = errors.New("document package failed validation")
	// ErrDocumentTextSanity rejects a claimed text/csv object whose bytes
	// are not plausibly text.
	ErrDocumentTextSanity = errors.New("csv content failed text sanity check")
)

// DocumentValidator is the publish-time seam for the document chain
// (ContentService consumes; OSSService implements; tests substitute).
type DocumentValidator interface {
	ValidateDocumentPackage(ctx context.Context, ossKey string, size int64, ext string, quota archivezip.Quota) error
	ValidateCSVTextSanity(ctx context.Context, ossKey string) error
}

// packageIdentityParts lists the entries an extension's OPC identity needs.
var packageIdentityParts = map[string][]string{
	".docx": {"[Content_Types].xml", "word/document.xml"},
	".xlsx": {"[Content_Types].xml", "xl/workbook.xml"},
}

// macroContentTypeMarkers are the [Content_Types].xml markers for macro
// content (vbaProject relationship / macroEnabled override parts).
var macroContentTypeMarkers = []string{
	"vbaProject",
	"macroEnabled",
}

func (s *OSSService) ValidateDocumentPackage(ctx context.Context, ossKey string, size int64, ext string, quota archivezip.Quota) error {
	if s == nil || s.client == nil {
		return ErrOSSNotConfigured
	}
	body, err := s.streamObjectToTemp(ctx, ossKey, size)
	if err != nil {
		return err
	}
	defer os.Remove(body.Name())
	defer func() { _ = body.Close() }()

	// 1. Structural validation: quotas, nesting, encryption, path traversal
	// (the same guard the mod pipeline runs).
	if _, err := body.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := archivezip.Validate(ctx, body, size, quota); err != nil {
		return err
	}

	// 2. Package identity + macro content over the entry table.
	if _, err := body.Seek(0, io.SeekStart); err != nil {
		return err
	}
	zr, err := zip.NewReader(body, size)
	if err != nil {
		return fmt.Errorf("%w: not a readable zip package", ErrDocumentPackage)
	}
	return inspectDocumentZip(zr, ext)
}

// inspectDocumentZip is the pure package-identity + macro pass over an
// already-structure-validated OPC package (unit-tested with in-memory zips).
func inspectDocumentZip(zr *zip.Reader, ext string) error {
	needed, ok := packageIdentityParts[strings.ToLower(strings.TrimSpace(ext))]
	if !ok {
		return fmt.Errorf("%w: unsupported document extension %q", ErrDocumentPackage, ext)
	}
	found := map[string]bool{}
	var contentTypes []byte
	for _, f := range zr.File {
		for _, part := range needed {
			if strings.EqualFold(f.Name, part) {
				found[part] = true
			}
		}
		// Macro defense-in-depth line 2 (line 1 = extension hard reject):
		// a renamed .docm carries vbaProject.bin somewhere in the package.
		if strings.EqualFold(path.Base(f.Name), "vbaProject.bin") {
			return fmt.Errorf("%w: package contains vbaProject.bin", ErrDocumentPackage)
		}
		if strings.EqualFold(f.Name, "[Content_Types].xml") && f.UncompressedSize64 <= 1<<20 {
			if rc, err := f.Open(); err == nil {
				raw, _ := io.ReadAll(io.LimitReader(rc, 1<<20))
				_ = rc.Close()
				contentTypes = raw
			}
		}
	}
	for _, part := range needed {
		if !found[part] {
			return fmt.Errorf("%w: missing required package part %s", ErrDocumentPackage, part)
		}
	}
	if len(contentTypes) > 0 {
		lower := strings.ToLower(string(contentTypes))
		for _, marker := range macroContentTypeMarkers {
			if strings.Contains(lower, strings.ToLower(marker)) {
				return fmt.Errorf("%w: [Content_Types].xml declares macro content (%s)", ErrDocumentPackage, marker)
			}
		}
	} else {
		// [Content_Types].xml was found as an entry (identity check above)
		// but could not be read — treat as malformed rather than pass.
		return fmt.Errorf("%w: [Content_Types].xml unreadable", ErrDocumentPackage)
	}
	return nil
}

// ValidateCSVTextSanity reads the object head and rejects implausible text
// (NUL bytes or a low printable ratio) so a renamed binary cannot ride the
// csv/text-plain MIME hint.
func (s *OSSService) ValidateCSVTextSanity(ctx context.Context, ossKey string) error {
	if s == nil || s.client == nil {
		return ErrOSSNotConfigured
	}
	object, err := s.client.Open(strings.TrimSpace(ossKey))
	if err != nil {
		return err
	}
	defer object.Close()
	head := make([]byte, 4096)
	n, err := io.ReadFull(io.LimitReader(object, int64(len(head))), head)
	if err != nil && err != io.ErrUnexpectedEOF {
		return err
	}
	head = head[:n]
	if n == 0 {
		return ErrDocumentTextSanity
	}
	printable := 0
	for _, b := range head {
		if b == 0 {
			return fmt.Errorf("%w: NUL byte in head", ErrDocumentTextSanity)
		}
		if b == '\n' || b == '\r' || b == '\t' || b >= 0x20 {
			printable++
		}
	}
	if float64(printable)/float64(n) < 0.9 {
		return fmt.Errorf("%w: low printable ratio", ErrDocumentTextSanity)
	}
	return nil
}

// ReadAudioHeader returns the first bytes of an object for the audio magic
// consistency check at publish time.
func (s *OSSService) ReadAudioHeader(ctx context.Context, ossKey string) ([]byte, error) {
	_ = ctx
	if s == nil || s.client == nil {
		return nil, ErrOSSNotConfigured
	}
	object, err := s.client.Open(strings.TrimSpace(ossKey))
	if err != nil {
		return nil, err
	}
	defer object.Close()
	head := make([]byte, 16)
	n, err := io.ReadFull(io.LimitReader(object, int64(len(head))), head)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	return head[:n], nil
}

// streamObjectToTemp materializes the object into a seekable temp file for
// the two-pass validation (structure walk + entry table).
func (s *OSSService) streamObjectToTemp(ctx context.Context, ossKey string, size int64) (*os.File, error) {
	object, err := s.client.Open(strings.TrimSpace(ossKey))
	if err != nil {
		return nil, err
	}
	defer object.Close()
	tmp, err := os.CreateTemp("", "omnicraft-document-validate-*")
	if err != nil {
		return nil, err
	}
	written, err := copyWithContext(ctx, tmp, io.LimitReader(object, size+1))
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, err
	}
	if written != size {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, &UploadValidationError{Message: "uploaded document size does not match grant"}
	}
	return tmp, nil
}
