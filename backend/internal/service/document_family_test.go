package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/archivezip"
)

// #688 upload-family admission matrices, publish rules and the document
// package chain, at the service seam (external behavior only).

func newT1TestOSSService() *OSSService {
	return &OSSService{cfg: &config.Config{
		Limits: config.LimitsConfig{
			ImageMaxMB: 20, VideoMaxMB: 300, TextMaxMB: 10,
			ModMaxMB: 500, SheetMusicMaxMB: 50,
			DocumentMaxMB: 20, AudioMaxMB: 50,
		},
	}}
}

func TestDocumentFamilyAdmissionMatrix(t *testing.T) {
	svc := newT1TestOSSService()

	accept := []struct{ ext, mime string }{
		{".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{".docx", "application/octet-stream"}, // File.type empty fallback (v2.2)
		{".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{".xlsx", "application/octet-stream"},
		{".csv", "text/csv"},
		{".csv", "text/plain"},
		{".csv", "application/octet-stream"},
	}
	for _, c := range accept {
		err := svc.validateUploadByType("document", c.mime, 1024, nil, c.ext)
		require.NoError(t, err, "document %s/%s must be admitted", c.ext, c.mime)
	}

	reject := []struct {
		name    string
		ext     string
		mime    string
		size    int64
		wantMsg string
	}{
		{"macro extension docm", ".docm", "application/vnd.ms-word.document.macroEnabled.12", 1024, "macro-enabled"},
		{"macro extension xlsm", ".xlsm", "application/vnd.ms-excel.sheet.macroEnabled.12", 1024, "macro-enabled"},
		{"macro extension pptm", ".pptm", "application/vnd.ms-powerpoint.presentation.macroEnabled.12", 1024, "macro-enabled"},
		{"unsupported extension", ".doc", "application/msword", 1024, "unsupported document extension"},
		{"docx wrong mime", ".docx", "video/mp4", 1024, "unsupported document mime_type"},
		{"csv wrong mime", ".csv", "application/vnd...", 1024, "unsupported document mime_type"},
		{"over budget", ".docx", "application/octet-stream", 21 << 20, "exceeds"},
	}
	for _, c := range reject {
		err := svc.validateUploadByType("document", c.mime, c.size, nil, c.ext)
		require.Error(t, err, c.name)
		require.Contains(t, err.Error(), c.wantMsg, c.name)
	}
}

func TestAudioFamilyAdmissionMatrix(t *testing.T) {
	svc := newT1TestOSSService()

	for _, ext := range []string{".mp3", ".wav", ".flac", ".m4a", ".aac", ".ogg", ".opus"} {
		require.NoError(t, svc.validateUploadByType("audio", "audio/mpeg", 1024, nil, ext), "audio %s", ext)
		require.NoError(t, svc.validateUploadByType("audio", "application/octet-stream", 1024, nil, ext), "audio %s octet-stream", ext)
	}
	require.Error(t, svc.validateUploadByType("audio", "audio/mpeg", 1024, nil, ".txt"), "unsupported extension")
	require.Error(t, svc.validateUploadByType("audio", "video/mp4", 1024, nil, ".mp3"), "non-audio mime")
	require.Error(t, svc.validateUploadByType("audio", "audio/mpeg", 51<<20, nil, ".mp3"), "over audio budget")
}

func TestAudioMagicConsistency(t *testing.T) {
	require.True(t, MatchesAudioMagic(".mp3", []byte("ID3\x04\x00\x00\x00\x00\x00")))
	require.True(t, MatchesAudioMagic(".mp3", []byte{0xFF, 0xFB, 0x90, 0x00}))
	require.True(t, MatchesAudioMagic(".wav", []byte("RIFFxxxxWAVE")))
	require.True(t, MatchesAudioMagic(".flac", []byte("fLaC")))
	require.True(t, MatchesAudioMagic(".ogg", []byte("OggSxxxx")))
	require.True(t, MatchesAudioMagic(".opus", []byte("OggSxxxxOpusHead")))
	require.True(t, MatchesAudioMagic(".m4a", []byte{0x00, 0x00, 0x00, 0x20, 'f', 't', 'y', 'p'}))
	// Renamed content does not pass.
	require.False(t, MatchesAudioMagic(".mp3", []byte("<html>really not audio")))
	require.False(t, MatchesAudioMagic(".wav", []byte("ID3\x04")))
	require.False(t, MatchesAudioMagic(".unknown", []byte("ID3\x04")))
}

func TestNormalizeUploadFileName(t *testing.T) {
	require.Equal(t, "report.docx", NormalizeUploadFileName("../../etc/passwd/report.docx"))
	require.Equal(t, "a b.png", NormalizeUploadFileName(`C:\Users\x\a b.png`))
	require.Equal(t, "clean.txt", NormalizeUploadFileName("  clean.txt  "))
	require.Equal(t, "nonul.txt", NormalizeUploadFileName("no\x00nul.txt"), "NUL is dropped")
	require.Equal(t, "", NormalizeUploadFileName(""))
	require.Equal(t, "", NormalizeUploadFileName("/"))
	require.Equal(t, "", NormalizeUploadFileName("."))
	// Path traversal attempts collapse to the basename.
	require.Equal(t, "x.pdf", NormalizeUploadFileName("../../../x.pdf"))
}

// buildDocZip builds an in-memory OPC-ish zip for the package inspector.
func buildDocZip(t *testing.T, entries map[string]string) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range entries {
		fw, err := w.Create(name)
		require.NoError(t, err)
		_, err = fw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	return zr
}

func TestDocumentPackageIdentityAndMacroRejection(t *testing.T) {
	contentTypes := `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`

	t.Run("valid docx passes identity", func(t *testing.T) {
		zr := buildDocZip(t, map[string]string{
			"[Content_Types].xml": contentTypes,
			"word/document.xml":   "<w:document/>",
		})
		require.NoError(t, inspectDocumentZip(zr, ".docx"))
	})

	t.Run("valid xlsx passes identity", func(t *testing.T) {
		zr := buildDocZip(t, map[string]string{
			"[Content_Types].xml": contentTypes,
			"xl/workbook.xml":     "<workbook/>",
		})
		require.NoError(t, inspectDocumentZip(zr, ".xlsx"))
	})

	t.Run("plain zip renamed docx fails identity", func(t *testing.T) {
		zr := buildDocZip(t, map[string]string{
			"readme.txt":  "hello",
			"payload.bin": "xyz",
		})
		err := inspectDocumentZip(zr, ".docx")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrDocumentPackage)
		require.Contains(t, err.Error(), "missing required package part")
	})

	t.Run("vbaProject.bin entry rejected even with valid identity", func(t *testing.T) {
		zr := buildDocZip(t, map[string]string{
			"[Content_Types].xml": contentTypes,
			"word/document.xml":   "<w:document/>",
			"word/vbaProject.bin": "\x00\x01\x02",
		})
		err := inspectDocumentZip(zr, ".docx")
		require.ErrorIs(t, err, ErrDocumentPackage)
		require.Contains(t, err.Error(), "vbaProject.bin")
	})

	t.Run("macroEnabled content type rejected (renamed docm)", func(t *testing.T) {
		zr := buildDocZip(t, map[string]string{
			"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Override PartName="/word/document.xml" ContentType="application/vnd.ms-word.document.macroEnabled.12"/></Types>`,
			"word/document.xml":   "<w:document/>",
		})
		err := inspectDocumentZip(zr, ".docx")
		require.ErrorIs(t, err, ErrDocumentPackage)
		require.Contains(t, err.Error(), "macro content")
	})
}

func TestRegistryBindingAndAttachmentPolicy(t *testing.T) {
	// Shipped-baseline registry (registryCfg nil → config defaults).
	var svc ContentService

	require.True(t, svc.allowedAttachmentFamily("template", "text"))
	require.True(t, svc.allowedAttachmentFamily("template", "document"), "#688: template allows the document family")
	require.True(t, svc.allowedAttachmentFamily("audio", "audio"), "#688: audio maps to its own family (form mapping fixed)")
	require.False(t, svc.allowedAttachmentFamily("article", "image"), "article takes no attachments")
	require.False(t, svc.allowedAttachmentFamily("template", "mod"), "cross-family combo rejected")
	require.False(t, svc.allowedAttachmentFamily("video", "text"))

	// attachment_policy: no policy → pass; required_any_of enforced.
	require.NoError(t, evaluateAttachmentPolicy(nil, "template", []model.ContentAttachment{{FileType: "text"}}))
	cfg := &config.Config{}
	policy := &config.ContentTypeEntry{
		Key: "widget", Zones: []string{"original"}, Form: config.ContentFormFile,
		UploadFileTypes: []string{"model3d", "text"}, JudgeEligible: boolPtrT1(true),
		AttachmentPolicy: &config.AttachmentPolicyConfig{RequiredAnyOf: []string{"model3d"}},
	}
	cfg.ContentRegistry = config.ContentRegistryConfig{
		ContentTypes:    []config.ContentTypeEntry{*policy},
		UploadFileTypes: config.DefaultContentRegistry().UploadFileTypes,
	}
	require.Error(t, evaluateAttachmentPolicy(cfg, "widget", []model.ContentAttachment{{FileType: "text"}}),
		"pure-document post misses the required family")
	require.NoError(t, evaluateAttachmentPolicy(cfg, "widget", []model.ContentAttachment{{FileType: "text"}, {FileType: "model3d"}}))

	// #690 pilot: the shipped registry row enforces by configuration alone
	// (registryCfg nil → DefaultContentRegistry, which mirrors config.yaml).
	require.True(t, svc.allowedAttachmentFamily("3d_print", "model3d"))
	require.True(t, svc.allowedAttachmentFamily("3d_print", "text"))
	require.False(t, svc.allowedAttachmentFamily("3d_print", "document"), "3d_print allows only model3d+text")
	require.Error(t, evaluateAttachmentPolicy(nil, "3d_print", []model.ContentAttachment{{FileType: "text"}}),
		"pure-document post is not 3D printing content")
	require.NoError(t, evaluateAttachmentPolicy(nil, "3d_print", []model.ContentAttachment{{FileType: "text"}, {FileType: "model3d"}}))
}

func TestScannableFamiliesGeneralized(t *testing.T) {
	cfg := &config.Config{}
	families := cfg.ScannableUploadFamilies()
	require.Contains(t, families, "mod")
	require.Contains(t, families, "document", "#688 document joins ClamAV")
	require.Contains(t, families, "audio", "v2.2 audio joins ClamAV")
	require.NotContains(t, families, "image")
	require.True(t, isScannableAttachmentFamily(nil, "document"))
	require.False(t, isScannableAttachmentFamily(nil, "text"))
}

func TestGrantTTLConfiguration(t *testing.T) {
	require.Equal(t, 1800, config.UploadConfig{}.EffectiveContentGrantTTLSec(), "default 1800 = PUT window + publish buffer")
	require.Equal(t, 1801, config.UploadConfig{ContentGrantTTLSec: 1801}.EffectiveContentGrantTTLSec())

	// Inversion refusal: a grant shorter than the 900s PUT window must not
	// boot (#688 v2.2 invariant).
	cfg := &config.Config{Upload: config.UploadConfig{ContentGrantTTLSec: 600}}
	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "upload.content_grant_ttl_sec must exceed")
}

// TestScanAwarePreviewGateMatrix is the #688 preview-gate seam: REST detail
// and MCP read both call ScanAwareDecorateAttachments, so the matrix is
// proven once at the signer with a stub gate backed by attachment rows.
func TestScanAwarePreviewGateMatrix(t *testing.T) {
	// Behavior classes proven without a DB: gate nil passthrough and the
	// short-TTL signing shape; the DB-backed fail-closed matrix lives in
	// archive_scan_gate_test.go (requireCleanAttachment semantics) and the
	// requireCleanAttachmentScannable unit below.
	signer := &DisplayURLSigner{}
	attachments := []model.ContentAttachment{{ID: 1, FileType: "image", OSSKey: "uploads/1/image/a.png"}}
	// nil signer + nil gate: no panic, no URL (fail-closed by absence).
	signer.ScanAwareDecorateAttachments(context.Background(), attachments, nil, 0)
	require.Equal(t, "", attachments[0].OSSURL)

	gate := NewArchiveScanGate(nil, true, nil)
	require.True(t, gate.IsScannableFamily("mod"))
	require.True(t, gate.IsScannableFamily("document"))
	require.False(t, gate.IsScannableFamily("image"))
	require.Equal(t, 300, gate.ScannablePreviewTTLSec(0))
	require.Equal(t, 300, gate.ScannablePreviewTTLSec(5000), "scan preview TTL caps at 300s")
	require.Equal(t, 120, gate.ScannablePreviewTTLSec(120))

	// not_required on a scannable family = wiring anomaly → fail-closed
	// (mod precedent); not_required on a non-scannable family passes.
	err := requireCleanAttachmentScannable(model.ContentAttachment{
		FileType: "document", ScanStatus: model.ScanStatusNotRequired, OSSKey: "uploads/1/doc/a.docx",
	}, map[string]bool{"document": true})
	require.Error(t, err, "scannable not_required is fail-closed")
	err = requireCleanAttachmentScannable(model.ContentAttachment{
		FileType: "image", ScanStatus: model.ScanStatusNotRequired, OSSKey: "uploads/1/image/a.png",
	}, map[string]bool{"document": true})
	require.NoError(t, err, "non-scannable not_required passes")
	err = requireCleanAttachmentScannable(model.ContentAttachment{
		FileType: "audio", ScanStatus: model.ScanStatusPending, OSSKey: "uploads/1/audio/a.mp3",
	}, map[string]bool{"audio": true})
	require.Error(t, err, "pending audio is not previewable")
	err = requireCleanAttachmentScannable(model.ContentAttachment{
		FileType: "mod", ScanStatus: model.ScanStatusClean, OSSKey: "uploads/1/mod/a.zip",
	}, map[string]bool{"mod": true})
	require.NoError(t, err)
	// Quarantine stays unreachable regardless of family/status.
	err = requireCleanAttachmentScannable(model.ContentAttachment{
		FileType: "image", ScanStatus: model.ScanStatusClean, OSSKey: "quarantine/1/evil.png",
	}, map[string]bool{})
	require.Error(t, err)
}

func boolPtrT1(v bool) *bool { return &v }

// --- #689 model3d family ---

func TestModel3DFamilyAdmissionMatrix(t *testing.T) {
	svc := newT1TestOSSService()
	svc.cfg.Limits.Model3DMaxMB = 50

	for _, ext := range []string{".stl", ".obj", ".3mf", ".gcode", ".ply", ".mtl"} {
		require.NoError(t, svc.validateUploadByType("model3d", "application/octet-stream", 1024, nil, ext), "model3d %s", ext)
		// MIME is a hint: model/*, text/plain (gcode) and common shapes pass.
		require.NoError(t, svc.validateUploadByType("model3d", "model/stl", 1024, nil, ext))
	}
	require.Error(t, svc.validateUploadByType("model3d", "application/octet-stream", 1024, nil, ".blend"), "unsupported extension")
	require.Error(t, svc.validateUploadByType("model3d", "video/mp4", 1024, nil, ".stl"), "implausible mime")
	require.Error(t, svc.validateUploadByType("model3d", "application/octet-stream", 51<<20, nil, ".stl"), "over model3d budget")
}

func TestThreeMFPackageIdentity(t *testing.T) {
	contentTypes := `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`

	t.Run("valid 3mf passes identity", func(t *testing.T) {
		zr := buildDocZip(t, map[string]string{
			"[Content_Types].xml": contentTypes,
			"3D/3dmodel.model":    `<?xml version="1.0"?><model/>`,
		})
		require.NoError(t, inspectDocumentZip(zr, ".3mf"))
	})

	t.Run("plain zip renamed 3mf fails identity", func(t *testing.T) {
		zr := buildDocZip(t, map[string]string{
			"[Content_Types].xml": contentTypes,
			"readme.txt":          "hello",
		})
		err := inspectDocumentZip(zr, ".3mf")
		require.ErrorIs(t, err, ErrDocumentPackage)
		require.Contains(t, err.Error(), "3D/*.model")
	})
}

func TestScannableFamiliesIncludeModel3D(t *testing.T) {
	cfg := &config.Config{}
	families := cfg.ScannableUploadFamilies()
	require.Contains(t, families, "model3d", "#689 model3d joins ClamAV")
}

// TestDocumentPackageRejectionWireCode pins the #691 smoke finding: a macro
// container renamed to .docx (or a non-OPC zip) must surface as
// ErrUploadGrantInvalid (400 UPLOAD_GRANT_INVALID on the wire), not escape
// raw and become 500 INTERNAL_ERROR.
func TestDocumentPackageRejectionWireCode(t *testing.T) {
	macroErr := fmt.Errorf("%w: [Content_Types].xml declares macro content", ErrDocumentPackage)
	stub := &stubDocumentValidator{packageErr: macroErr}
	svc := (&ContentService{}).SetDocumentValidator(stub, nil)

	err := svc.validateDocumentAttachment(context.Background(),
		UploadGrant{OSSKey: "uploads/1/document/a.docx", OriginalFileName: "a.docx", FileSize: 10})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrUploadGrantInvalid, "package-identity/macro rejection must map to the 400 wire code")

	// Internal errors (OSS fetch failure) stay raw — they are not user input.
	fetchErr := errors.New("oss: connection reset")
	svc2 := (&ContentService{}).SetDocumentValidator(&stubDocumentValidator{packageErr: fetchErr}, nil)
	err2 := svc2.validateDocumentAttachment(context.Background(),
		UploadGrant{OSSKey: "uploads/1/document/a.docx", OriginalFileName: "a.docx", FileSize: 10})
	require.ErrorIs(t, err2, fetchErr)
	require.NotErrorIs(t, err2, ErrUploadGrantInvalid)
}

type stubDocumentValidator struct {
	packageErr error
}

func (s *stubDocumentValidator) ValidateDocumentPackage(ctx context.Context, ossKey string, size int64, ext string, quota archivezip.Quota) error {
	return s.packageErr
}

func (s *stubDocumentValidator) ValidateCSVTextSanity(ctx context.Context, ossKey string) error {
	return nil
}
