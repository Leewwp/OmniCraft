// Package archivezip validates mod archive structure and decompression
// quotas before an object is handed to the malware scan worker (archive
// malware scanning design §2/§4). It streams through the central directory
// and every entry, rejecting encrypted entries, unsafe paths (absolute,
// drive-letter, backslash, ".." traversal), symlinks and other special
// files, case-insensitive duplicate names, and any archive exceeding the
// configured entry/size/recursion quotas. Content is never executed and
// never stored beyond a restricted, cleaned-up temp file used to inspect
// nested zips; validation is purely structural.
package archivezip

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"omnicraft/backend/config"
)

// Stable business error codes (archive malware scanning design §7). The
// sentinel string IS the API code so the S04 publish/download gate can map
// errors onto handler responses with errors.Is and no translation layer.
// Errors other than these are internal (malformed archives, I/O failures,
// context cancellation) and must not be exposed to clients raw.
var (
	ErrEncrypted     = errors.New("ARCHIVE_ENCRYPTED")
	ErrPathInvalid   = errors.New("ARCHIVE_PATH_INVALID")
	ErrLinkForbidden = errors.New("ARCHIVE_LINK_FORBIDDEN")
	ErrLimitExceeded = errors.New("ARCHIVE_LIMIT_EXCEEDED")
	ErrInvalid       = errors.New("ARCHIVE_INVALID")
)

// errInvalidQuota marks a Quota with non-positive limits. It is an internal
// error (config validation should reject such values) and never a business
// sentinel.
var errInvalidQuota = errors.New("archive zip quota must be positive")

// Quota carries the zip structure/decompression limits. All values come
// from the archive_scan.* config section (design §4/§6); the 500 MiB upload
// size bound is enforced by the upload path, not by this package.
type Quota struct {
	MaxZipEntries          int
	MaxEntryUncompressedMB int
	MaxTotalUncompressedMB int
	MaxRecursionDepth      int
}

// QuotaFromConfig maps the archive_scan.* config block onto Quota.
func QuotaFromConfig(c config.ArchiveScanConfig) Quota {
	return Quota{
		MaxZipEntries:          c.MaxZipEntries,
		MaxEntryUncompressedMB: c.MaxEntryUncompressedMB,
		MaxTotalUncompressedMB: c.MaxTotalUncompressedMB,
		MaxRecursionDepth:      c.MaxRecursionDepth,
	}
}

func (q Quota) maxEntryBytes() int64 { return int64(q.MaxEntryUncompressedMB) << 20 }
func (q Quota) maxTotalBytes() int64 { return int64(q.MaxTotalUncompressedMB) << 20 }

func (q Quota) valid() error {
	if q.MaxZipEntries <= 0 || q.MaxEntryUncompressedMB <= 0 ||
		q.MaxTotalUncompressedMB <= 0 || q.MaxRecursionDepth <= 0 {
		return fmt.Errorf("%w: %+v", errInvalidQuota, q)
	}
	return nil
}

// Stats reports what the validator observed across all nesting levels. It is
// intended for audit logging by the scan pipeline, not for client output.
type Stats struct {
	EntryCount        int64
	TotalUncompressed int64
	MaxNestedDepth    int
}

// Validate streams the zip archive in r (size bytes) and rejects it with a
// stable sentinel error if any structure or quota rule is violated. Quota
// violations are detected while the stream is being consumed and abort
// immediately; the archive is never fully decompressed into memory. Nested
// zips are validated recursively up to Quota.MaxRecursionDepth using a
// restricted temp file (0o700 directory, 0o600 file) that is always removed.
// ctx cancellation aborts validation with the context error.
func Validate(ctx context.Context, r io.ReaderAt, size int64, q Quota) (*Stats, error) {
	if err := q.valid(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	acc := &accounter{maxTotal: q.maxTotalBytes()}
	dir, err := os.MkdirTemp("", "omnicraft-archivezip-*")
	if err != nil {
		return nil, fmt.Errorf("create restricted temp dir: %w", err)
	}
	defer os.RemoveAll(dir)
	if err := validateLevel(ctx, r, size, q, 0, acc, dir); err != nil {
		return nil, err
	}
	return &Stats{
		EntryCount:        acc.entryCount,
		TotalUncompressed: acc.totalUncompressed,
		MaxNestedDepth:    acc.maxDepth,
	}, nil
}

// accounter is shared across nesting levels so the cumulative decompression
// quota aborts the moment it is crossed, wherever in the recursion it
// happens.
type accounter struct {
	maxTotal          int64
	totalUncompressed int64
	entryCount        int64
	maxDepth          int
}

func (a *accounter) add(n int64) error {
	if a.totalUncompressed+n > a.maxTotal {
		return fmt.Errorf("total uncompressed size %d exceeds limit %d: %w",
			a.totalUncompressed+n, a.maxTotal, ErrLimitExceeded)
	}
	a.totalUncompressed += n
	return nil
}

// centralDirBytesPerEntry is the per-entry byte budget granted to the zip
// central directory (46B fixed record header + generous file-name headroom,
// audit #3). An archive whose declared directory is larger than
// MaxZipEntries × this budget would materialize more per-record state than
// the entry quota allows before any check in validateLevel runs.
const centralDirBytesPerEntry = 128

// EOCD (end-of-central-directory) record layout, mirrored from the zip spec:
// signature, disk numbers, entry counts, directory size/offset and the
// trailing comment length. The 64-bit variants carry the real values when
// the classic fields hit their 0xFFFF/0xFFFFFFFF sentinels.
const (
	eocdSignature      = 0x06054b50
	eocdLen            = 22
	eocdMaxSearch      = 65557 // eocdLen + maximum comment (0xFFFF)
	zip64LocSignature  = 0x07064b50
	zip64LocLen        = 20
	zip64EOCDSignature = 0x06064b50
	zip64EOCDMinLen    = 56
	dirOffsetSentinel  = 0xFFFFFFFF
	dirSizeSentinel    = 0xFFFFFFFF
	entriesSentinel    = 0xFFFF
)

// checkCentralDir pre-screens the archive budget BEFORE zip.NewReader
// materializes one zip.File (plus map headroom) per central-directory record
// (audit #3 / #814): the EOCD at the tail already declares the directory
// offset/size and entry count, so a pathological archive (e.g. millions of
// minimal records inside the upload size cap) is rejected after reading only
// the EOCD tail window. A malformed tail is left for zip.NewReader to rule
// on — this screen must never reject what the reference parser accepts.
func (q Quota) checkCentralDir(r io.ReaderAt, size int64) error {
	dirOffset, dirSize, entries, ok := findCentralDir(r, size)
	if !ok {
		return nil
	}
	budget := int64(q.MaxZipEntries) * centralDirBytesPerEntry
	if dirSize > budget {
		return fmt.Errorf("zip central directory %d bytes exceeds limit %d: %w", dirSize, budget, ErrLimitExceeded)
	}
	if entries > int64(q.MaxZipEntries) {
		return fmt.Errorf("zip entry count %d exceeds limit %d: %w", entries, q.MaxZipEntries, ErrLimitExceeded)
	}
	if dirOffset < 0 || dirSize < 0 || dirOffset+dirSize > size {
		return fmt.Errorf("zip central directory bounds [%d,%d) outside archive of %d bytes: %w",
			dirOffset, dirOffset+dirSize, size, ErrLimitExceeded)
	}
	return nil
}

// findCentralDir locates the EOCD record (falling back to the ZIP64 EOCD when
// the classic fields carry sentinels) and reports the declared directory
// offset/size and total entry count. ok=false means the tail does not carry a
// parseable EOCD; the caller then defers to zip.NewReader's own verdict.
func findCentralDir(r io.ReaderAt, size int64) (dirOffset, dirSize, entries int64, ok bool) {
	if size < eocdLen {
		return 0, 0, 0, false
	}
	window := int64(eocdMaxSearch)
	if size < window {
		window = size
	}
	buf := make([]byte, window)
	if _, err := r.ReadAt(buf, size-window); err != nil {
		return 0, 0, 0, false
	}
	// Scan backwards like archive/zip does, so a matching record followed by
	// a well-formed comment (EOCD + comment ends exactly at the archive tail)
	// wins over embedded decoy signatures.
	for i := int(window) - eocdLen; i >= 0; i-- {
		if binary.LittleEndian.Uint32(buf[i:i+4]) != eocdSignature {
			continue
		}
		commentLen := int(binary.LittleEndian.Uint16(buf[i+20 : i+22]))
		if i+eocdLen+commentLen != int(window) {
			continue
		}
		n := int64(binary.LittleEndian.Uint16(buf[i+10 : i+12]))
		s := int64(binary.LittleEndian.Uint32(buf[i+12 : i+16]))
		o := int64(binary.LittleEndian.Uint32(buf[i+16 : i+20]))
		if n == entriesSentinel || s == dirSizeSentinel || o == dirOffsetSentinel {
			return readZip64EOCD(r, buf, i)
		}
		return o, s, n, true
	}
	return 0, 0, 0, false
}

// readZip64EOCD resolves the real directory bounds from the ZIP64 EOCD
// record pointed at by the locator that sits just before the classic EOCD.
func readZip64EOCD(r io.ReaderAt, eocdWindow []byte, eocdIdx int) (dirOffset, dirSize, entries int64, ok bool) {
	locIdx := eocdIdx - zip64LocLen
	if locIdx < 0 {
		return 0, 0, 0, false
	}
	if binary.LittleEndian.Uint32(eocdWindow[locIdx:locIdx+4]) != zip64LocSignature {
		return 0, 0, 0, false
	}
	z64Off := int64(binary.LittleEndian.Uint64(eocdWindow[locIdx+8 : locIdx+16]))
	rec := make([]byte, zip64EOCDMinLen)
	if _, err := r.ReadAt(rec, z64Off); err != nil {
		return 0, 0, 0, false
	}
	if binary.LittleEndian.Uint32(rec[:4]) != zip64EOCDSignature {
		return 0, 0, 0, false
	}
	n := int64(binary.LittleEndian.Uint64(rec[32:40]))
	s := int64(binary.LittleEndian.Uint64(rec[40:48]))
	o := int64(binary.LittleEndian.Uint64(rec[48:56]))
	return o, s, n, true
}

func validateLevel(ctx context.Context, r io.ReaderAt, size int64, q Quota, depth int, acc *accounter, dir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := q.checkCentralDir(r, size); err != nil {
		return err
	}
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if len(zr.File) > q.MaxZipEntries {
		return fmt.Errorf("zip entry count %d exceeds limit %d: %w",
			len(zr.File), q.MaxZipEntries, ErrLimitExceeded)
	}
	seen := make(map[string]struct{}, len(zr.File))
	for _, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := checkEntry(f); err != nil {
			return err
		}
		key := dedupKey(f.Name)
		if _, dup := seen[key]; dup {
			return fmt.Errorf("duplicate entry %q: %w", f.Name, ErrPathInvalid)
		}
		seen[key] = struct{}{}
		if f.UncompressedSize64 > uint64(q.maxEntryBytes()) {
			return fmt.Errorf("entry %q declared uncompressed size %d exceeds limit %d: %w",
				f.Name, f.UncompressedSize64, q.maxEntryBytes(), ErrLimitExceeded)
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open entry %q: %w", f.Name, err)
		}
		err = consumeEntry(ctx, rc, q, depth, acc, dir)
		rc.Close()
		if err != nil {
			var corrupt flate.CorruptInputError
			if errors.Is(err, zip.ErrChecksum) || errors.Is(err, io.ErrUnexpectedEOF) || errors.As(err, &corrupt) {
				return fmt.Errorf("entry %q is corrupt: %w", f.Name, ErrInvalid)
			}
			return err
		}
		acc.entryCount++
	}
	return nil
}

// checkEntry applies the per-entry structural rules in a stable order:
// encryption, special file mode, unsafe name. On first violation the
// corresponding sentinel is returned.
func checkEntry(f *zip.File) error {
	if f.Flags&0x0001 != 0 {
		return fmt.Errorf("entry %q is encrypted: %w", f.Name, ErrEncrypted)
	}
	if m := f.Mode(); m&(fs.ModeSymlink|fs.ModeDevice|fs.ModeNamedPipe|fs.ModeSocket|fs.ModeIrregular) != 0 {
		return fmt.Errorf("entry %q has forbidden file mode %v: %w", f.Name, m, ErrLinkForbidden)
	}
	if err := checkName(f.Name); err != nil {
		return err
	}
	return nil
}

// checkName rejects entry names that are empty, contain a NUL byte or a
// backslash, are absolute (leading "/"), carry a Windows drive-letter prefix,
// or contain any ".." segment. Names that collapse to "." are also rejected
// as no-op paths. A "." segment is otherwise tolerated ("a/./b") because it
// cannot escape the extraction root.
func checkName(name string) error {
	if name == "" {
		return fmt.Errorf("empty entry name: %w", ErrPathInvalid)
	}
	if strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("entry name %q contains NUL byte: %w", name, ErrPathInvalid)
	}
	if strings.Contains(name, `\`) {
		return fmt.Errorf("entry name %q contains backslash: %w", name, ErrPathInvalid)
	}
	if strings.HasPrefix(name, "/") {
		return fmt.Errorf("entry name %q is absolute: %w", name, ErrPathInvalid)
	}
	if len(name) >= 2 && isASCIIAlpha(name[0]) && name[1] == ':' {
		return fmt.Errorf("entry name %q has Windows drive-letter prefix: %w", name, ErrPathInvalid)
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == ".." {
			return fmt.Errorf("entry name %q contains path traversal: %w", name, ErrPathInvalid)
		}
	}
	if path.Clean(name) == "." {
		return fmt.Errorf("entry name %q is a no-op path: %w", name, ErrPathInvalid)
	}
	return nil
}

func isASCIIAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// dedupKey normalizes an entry name for case-insensitive duplicate detection
// (spec §2: "重名覆盖" rejected, case-insensitive per platform convention).
func dedupKey(name string) string {
	return path.Clean(strings.ToLower(name))
}

// consumeEntry reads one entry, counting decompressed bytes against the
// per-entry and cumulative quotas. If the entry is itself a zip (magic
// sniffed from the stream), it is spooled to a restricted temp file and
// validated recursively at depth+1.
func consumeEntry(ctx context.Context, rc io.Reader, q Quota, depth int, acc *accounter, dir string) error {
	cw := &countingWriter{ctx: ctx, q: q, acc: acc}

	head := make([]byte, 4)
	n, err := io.ReadFull(rc, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("read entry head: %w", err)
	}
	if n > 0 {
		if _, err := cw.Write(head[:n]); err != nil {
			return err
		}
	}
	if n != len(head) || !isZipMagic(head) {
		if _, err := io.Copy(cw, rc); err != nil {
			return err
		}
		return nil
	}

	if depth+1 > q.MaxRecursionDepth {
		return fmt.Errorf("nested zip depth %d exceeds limit %d: %w",
			depth+1, q.MaxRecursionDepth, ErrLimitExceeded)
	}
	tf, err := os.CreateTemp(dir, "nested-*.zip")
	if err != nil {
		return fmt.Errorf("create restricted temp file: %w", err)
	}
	name := tf.Name()
	defer os.Remove(name)
	// The magic bytes were already consumed by the sniff; write them back so
	// the spooled copy is byte-identical to the entry content.
	if _, err := tf.Write(head[:n]); err != nil {
		tf.Close()
		return fmt.Errorf("write temp head: %w", err)
	}
	if _, err := io.Copy(io.MultiWriter(tf, cw), rc); err != nil {
		tf.Close()
		return err
	}
	if err := tf.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	rf, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("reopen temp file: %w", err)
	}
	defer rf.Close()
	info, err := rf.Stat()
	if err != nil {
		return fmt.Errorf("stat temp file: %w", err)
	}
	if err := validateLevel(ctx, rf, info.Size(), q, depth+1, acc, dir); err != nil {
		return err
	}
	if depth+1 > acc.maxDepth {
		acc.maxDepth = depth + 1
	}
	return nil
}

// countingWriter is a discard sink that enforces the per-entry and the
// shared cumulative decompression quotas while bytes stream through it, and
// honors context cancellation. It is the zip-bomb tripwire: the moment a
// limit is crossed the caller aborts, so oversized archives are never fully
// decompressed.
type countingWriter struct {
	ctx   context.Context
	q     Quota
	acc   *accounter
	entry int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if w.entry+int64(len(p)) > w.q.maxEntryBytes() {
		return 0, fmt.Errorf("entry uncompressed size %d exceeds limit %d: %w",
			w.entry+int64(len(p)), w.q.maxEntryBytes(), ErrLimitExceeded)
	}
	if err := w.acc.add(int64(len(p))); err != nil {
		return 0, err
	}
	w.entry += int64(len(p))
	return len(p), nil
}

// isZipMagic reports whether head looks like the start of a zip archive:
// local file header, empty-archive end-of-central-directory, or the split
// marker. It is the structure-only heuristic used to decide whether an entry
// is a nested archive.
func isZipMagic(head []byte) bool {
	return bytes.Equal(head, []byte{'P', 'K', 0x03, 0x04}) ||
		bytes.Equal(head, []byte{'P', 'K', 0x05, 0x06}) ||
		bytes.Equal(head, []byte{'P', 'K', 0x07, 0x08})
}
