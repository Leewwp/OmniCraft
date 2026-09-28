package service

import (
	"path"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Upload admission tables for the #688 document and audio families. The
// registry (config content_registry.upload_file_types) carries the same
// extension sets; these tables add the per-extension MIME hint matrices that
// stay code-side this ticket (validateUploadByType parity with the video
// duration special case, recorded in the #687 census exception ledger).

var (
	documentExtensions = map[string]bool{
		".docx": true,
		".xlsx": true,
		".csv":  true,
	}
	documentMacroExtensions = map[string]bool{
		".docm": true,
		".xlsm": true,
		".pptm": true,
	}
	documentMIMEByExt = map[string]map[string]bool{
		// Registered OpenXML MIME plus the octet-stream fallback (browsers
		// report an empty File.type which the client normalizes to
		// application/octet-stream; the server-side package-identity check
		// after upload is the authority).
		".docx": {
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
			"application/octet-stream": true,
		},
		".xlsx": {
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
			"application/octet-stream": true,
		},
		// CSV additionally accepts text/plain with a text sanity check at
		// publish time.
		".csv": {
			"text/csv":                true,
			"text/plain":              true,
			"application/octet-stream": true,
		},
	}
	audioExtensions = map[string]bool{
		".mp3": true, ".wav": true, ".flac": true,
		".m4a": true, ".aac": true, ".ogg": true, ".opus": true,
	}
)

func isAllowedDocumentExt(ext string) bool {
	return documentExtensions[strings.ToLower(strings.TrimSpace(ext))]
}

func isDocumentMacroExt(ext string) bool {
	return documentMacroExtensions[strings.ToLower(strings.TrimSpace(ext))]
}

func isAllowedDocumentMIME(ext, mimeType string) bool {
	allowed, ok := documentMIMEByExt[strings.ToLower(strings.TrimSpace(ext))]
	if !ok {
		return false
	}
	return allowed[strings.ToLower(strings.TrimSpace(mimeType))]
}

func isAllowedAudioExt(ext string) bool {
	return audioExtensions[strings.ToLower(strings.TrimSpace(ext))]
}

// DocumentExtension returns the normalized extension (with dot) when ext
// belongs to the document family (used by the publish-time package checks).
func DocumentExtension(ext string) (string, bool) {
	e := strings.ToLower(strings.TrimSpace(ext))
	if documentExtensions[e] {
		return e, true
	}
	return "", false
}

// NormalizeUploadFileName locks a client-supplied name into a safe,
// storable form (#688): strip any path component, drop NUL and control
// characters, Unicode-normalize (NFC), trim, cap at 255 runes. The result is
// the single trusted source persisted on the attachment row — the publish
// payload's file_name is never used.
func NormalizeUploadFileName(name string) string {
	// filepath.Base on the raw value drops both separators and any
	// platform-specific drive weirdness; on a name that is pure separators
	// it returns "." which the empty-check below rejects.
	base := path.Base(strings.ReplaceAll(name, "\\", "/"))
	if base == "." || base == "/" || base == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range norm.NFC.String(base) {
		if r == 0 || unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	normalized := strings.TrimSpace(b.String())
	const maxRunes = 255
	if len([]rune(normalized)) > maxRunes {
		normalized = string([]rune(normalized)[:maxRunes])
	}
	return normalized
}

// audioMagicSignatures maps extension → accepted header magic prefixes
// (#688): the publish-time sniffer asserts the uploaded object's header
// matches its claimed extension so a renamed executable cannot ride the
// audio family.
var audioMagicSignatures = map[string][][]byte{
	".mp3":  {{0x49, 0x44, 0x33}, {0xFF, 0xFB}, {0xFF, 0xF3}, {0xFF, 0xF2}, {0xFF, 0xF7}, {0xFF, 0xF1}}, // ID3 / MPEG frame sync
	".wav":  {{0x52, 0x49, 0x46, 0x46}},                                                               // RIFF
	".flac": {{0x66, 0x4C, 0x61, 0x43}},                                                               // fLaC
	".ogg":  {{0x4F, 0x67, 0x67, 0x53}},                                                               // OggS (vorbis/opus containers)
	".opus": {{0x4F, 0x67, 0x67, 0x53}},                                                               // Opus rides an Ogg container
	".m4a":  {{0x00, 0x00, 0x00}, {0x33, 0x67, 0x70, 0x35}},                                           // MP4 box length / '5gp' variant
	".aac":  {{0xFF, 0xF1}, {0xFF, 0xF9}, {0x4F, 0x67, 0x67, 0x53}},                                   // ADTS / Ogg fallback
}

// MatchesAudioMagic reports whether the uploaded object's header is
// consistent with the claimed audio extension.
func MatchesAudioMagic(ext string, header []byte) bool {
	prefixes, ok := audioMagicSignatures[strings.ToLower(strings.TrimSpace(ext))]
	if !ok {
		return false
	}
	for _, prefix := range prefixes {
		if len(header) >= len(prefix) && string(header[:len(prefix)]) == string(prefix) {
			return true
		}
	}
	return false
}
