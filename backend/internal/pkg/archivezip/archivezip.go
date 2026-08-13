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
	"context"
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

func validateLevel(ctx context.Context, r io.ReaderAt, size int64, q Quota, depth int, acc *accounter, dir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return fmt.Errorf("invalid zip archive: %w", err)
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
