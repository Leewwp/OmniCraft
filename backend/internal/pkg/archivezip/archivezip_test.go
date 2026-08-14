package archivezip

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"strings"
	"testing"

	"omnicraft/backend/config"
)

// eicarString is the standard EICAR test string. The validator is a
// structure/quota check only and must never reject content based on payload
// bytes; the string is used to prove that plain file content is not scanned.
const eicarString = `X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`

// defaultQuota mirrors the archive_scan.* production defaults (design §4).
var defaultQuota = Quota{
	MaxZipEntries:          5000,
	MaxEntryUncompressedMB: 200,
	MaxTotalUncompressedMB: 2048,
	MaxRecursionDepth:      10,
}

// testEntry is one entry to be written into an in-memory zip fixture.
type testEntry struct {
	name    string
	content []byte
	method  uint16
	flags   uint16
	mode    fs.FileMode
}

func buildTestZip(t *testing.T, entries ...testEntry) []byte {
	t.Helper()
	data, err := buildZip(entries...)
	if err != nil {
		t.Fatalf("build zip fixture: %v", err)
	}
	return data
}

// buildZip writes entries with archive/zip.Writer, so all fixtures are
// structurally valid zips (no hand-crafted bytes).
func buildZip(entries ...testEntry) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: e.method, Flags: e.flags}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(e.content); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func validateData(t *testing.T, data []byte, q Quota) (*Stats, error) {
	t.Helper()
	return Validate(context.Background(), bytes.NewReader(data), int64(len(data)), q)
}

func mustValidate(t *testing.T, data []byte, q Quota) *Stats {
	t.Helper()
	st, err := validateData(t, data, q)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return st
}

func wantSentinel(t *testing.T, err error, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}

// trackingReaderAt records how many bytes were actually requested from the
// underlying source, to prove the validator aborts early instead of reading
// the whole archive.
type trackingReaderAt struct {
	r    io.ReaderAt
	read int64
}

func (tr *trackingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	n, err := tr.r.ReadAt(p, off)
	tr.read += int64(n)
	return n, err
}

func TestValidateValidArchive(t *testing.T) {
	data := buildTestZip(t,
		testEntry{name: "mod/"},
		testEntry{name: "mod/mod.json", content: []byte(`{"name":"demo"}`)},
		testEntry{name: "mod/textures/"},
		testEntry{name: "mod/textures/a.png", content: []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}},
		testEntry{name: "mod/install.sh", content: []byte("#!/bin/sh\necho hi\n")},
	)
	st := mustValidate(t, data, defaultQuota)
	if st.EntryCount != 5 {
		t.Fatalf("EntryCount = %d, want 5", st.EntryCount)
	}
	if st.TotalUncompressed != int64(len(`{"name":"demo"}`)+8+len("#!/bin/sh\necho hi\n")) {
		t.Fatalf("TotalUncompressed = %d, want %d", st.TotalUncompressed, int64(len(`{"name":"demo"}`)+8+len("#!/bin/sh\necho hi\n")))
	}
	if st.MaxNestedDepth != 0 {
		t.Fatalf("MaxNestedDepth = %d, want 0", st.MaxNestedDepth)
	}
}

func TestValidateEmptyArchive(t *testing.T) {
	data := buildTestZip(t)
	st := mustValidate(t, data, defaultQuota)
	if st.EntryCount != 0 || st.TotalUncompressed != 0 {
		t.Fatalf("empty archive stats = %+v, want zero", st)
	}
}

func TestValidateDoesNotInspectContent(t *testing.T) {
	data := buildTestZip(t,
		testEntry{name: "payload.txt", content: []byte(eicarString)},
		testEntry{name: "run.sh", content: []byte("#!/bin/sh\nrm -rf /\n")},
	)
	st := mustValidate(t, data, defaultQuota)
	if st.EntryCount != 2 {
		t.Fatalf("EntryCount = %d, want 2", st.EntryCount)
	}
}

func TestValidateEncryptedEntries(t *testing.T) {
	for _, method := range []uint16{zip.Store, zip.Deflate} {
		data := buildTestZip(t, testEntry{name: "secret.txt", content: []byte("s3cr3t"), method: method, flags: 0x1})
		_, err := validateData(t, data, defaultQuota)
		wantSentinel(t, err, ErrEncrypted)
	}
}

func TestValidateUnsafePaths(t *testing.T) {
	unsafe := []string{
		"",            // empty name
		"/etc/passwd", // absolute
		"//evil",      // absolute double slash
		"C:/evil.exe", // Windows drive letter
		"c:\\evil.exe",
		"\\evil", // Windows absolute
		"..",     // traversal
		"../evil.sh",
		"a/../../evil",
		"a/..",
		"..\\evil.sh",
		"a/\x00b", // NUL byte
		".",       // no-op path
	}
	for _, name := range unsafe {
		t.Run(fmt.Sprintf("unsafe_%q", name), func(t *testing.T) {
			data := buildTestZip(t, testEntry{name: name, content: []byte("x")})
			_, err := validateData(t, data, defaultQuota)
			wantSentinel(t, err, ErrPathInvalid)
		})
	}

	// "./" is a no-op path that the zip writer treats as a directory entry,
	// so it must be written without content.
	t.Run(`unsafe_"./"`, func(t *testing.T) {
		data := buildTestZip(t, testEntry{name: "./"})
		_, err := validateData(t, data, defaultQuota)
		wantSentinel(t, err, ErrPathInvalid)
	})

	safe := []string{
		"mod/file.txt",
		"./mod/file.txt", // leading ./ is harmless
		"a/./b.txt",
		"a//b.txt", // doubled separator
		"mod/textures/x.png",
		"资料/说明.txt",
		strings.Repeat("x", 3000), // very long name
	}
	for _, name := range safe {
		t.Run(fmt.Sprintf("safe_%q", name), func(t *testing.T) {
			data := buildTestZip(t, testEntry{name: name, content: []byte("x")})
			st := mustValidate(t, data, defaultQuota)
			if st.EntryCount != 1 {
				t.Fatalf("EntryCount = %d, want 1", st.EntryCount)
			}
		})
	}
}

func TestValidateRejectsSpecialFileModes(t *testing.T) {
	rejected := []fs.FileMode{
		fs.ModeSymlink | 0o777,
		fs.ModeNamedPipe | 0o644,
		fs.ModeDevice | 0o600,
		fs.ModeSocket | 0o600,
	}
	for _, mode := range rejected {
		t.Run(fmt.Sprintf("reject_%v", mode), func(t *testing.T) {
			data := buildTestZip(t, testEntry{name: "special", content: []byte("x"), mode: mode})
			_, err := validateData(t, data, defaultQuota)
			wantSentinel(t, err, ErrLinkForbidden)
		})
	}

	allowed := []fs.FileMode{0o644, fs.ModeDir | 0o755}
	for _, mode := range allowed {
		t.Run(fmt.Sprintf("allow_%v", mode), func(t *testing.T) {
			data := buildTestZip(t, testEntry{name: "normal", content: []byte("x"), mode: mode})
			mustValidate(t, data, defaultQuota)
		})
	}
}

func TestValidateDuplicateNames(t *testing.T) {
	cases := []struct {
		name    string
		entries []testEntry
	}{
		{name: "exact_duplicate", entries: []testEntry{
			{name: "a.txt", content: []byte("1")},
			{name: "a.txt", content: []byte("2")},
		}},
		{name: "case_insensitive", entries: []testEntry{
			{name: "README.md", content: []byte("1")},
			{name: "readme.md", content: []byte("2")},
		}},
		{name: "unicode_case", entries: []testEntry{
			{name: "Ä.txt", content: []byte("1")},
			{name: "ä.txt", content: []byte("2")},
		}},
		{name: "dir_vs_file", entries: []testEntry{
			{name: "dir/"},
			{name: "dir", content: []byte("2")},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := buildTestZip(t, tc.entries...)
			_, err := validateData(t, data, defaultQuota)
			wantSentinel(t, err, ErrPathInvalid)
		})
	}

	// Negative: distinct names pass.
	data := buildTestZip(t,
		testEntry{name: "a.txt", content: []byte("1")},
		testEntry{name: "b.txt", content: []byte("2")},
	)
	st := mustValidate(t, data, defaultQuota)
	if st.EntryCount != 2 {
		t.Fatalf("EntryCount = %d, want 2", st.EntryCount)
	}
}

func TestValidateEntryCountLimit(t *testing.T) {
	three := buildTestZip(t,
		testEntry{name: "a.txt", content: []byte("a")},
		testEntry{name: "b.txt", content: []byte("b")},
		testEntry{name: "c.txt", content: []byte("c")},
	)
	q := Quota{MaxZipEntries: 2, MaxEntryUncompressedMB: 200, MaxTotalUncompressedMB: 2048, MaxRecursionDepth: 10}
	_, err := validateData(t, three, q)
	wantSentinel(t, err, ErrLimitExceeded)

	// Boundary: exactly the limit passes.
	two := buildTestZip(t,
		testEntry{name: "a.txt", content: []byte("a")},
		testEntry{name: "b.txt", content: []byte("b")},
	)
	mustValidate(t, two, q)

	// The limit applies per nesting level: a nested zip with 3 entries is
	// rejected even though the outer archive has only 1 entry.
	inner := buildTestZip(t,
		testEntry{name: "a.txt", content: []byte("a")},
		testEntry{name: "b.txt", content: []byte("b")},
		testEntry{name: "c.txt", content: []byte("c")},
	)
	outer := buildTestZip(t, testEntry{name: "inner.zip", content: inner})
	_, err = validateData(t, outer, q)
	wantSentinel(t, err, ErrLimitExceeded)
}

func TestValidateSingleEntryLimit(t *testing.T) {
	miB := 1024 * 1024
	q := Quota{MaxZipEntries: 5000, MaxEntryUncompressedMB: 1, MaxTotalUncompressedMB: 2048, MaxRecursionDepth: 10}

	over := buildTestZip(t, testEntry{name: "big.bin", content: bytes.Repeat([]byte{0x42}, miB+1)})
	_, err := validateData(t, over, q)
	wantSentinel(t, err, ErrLimitExceeded)

	// Boundary: exactly the limit passes.
	exact := buildTestZip(t, testEntry{name: "big.bin", content: bytes.Repeat([]byte{0x42}, miB)})
	st := mustValidate(t, exact, q)
	if st.TotalUncompressed != int64(miB) {
		t.Fatalf("TotalUncompressed = %d, want %d", st.TotalUncompressed, miB)
	}

	// Several entries each under the per-entry limit pass.
	two := buildTestZip(t,
		testEntry{name: "a.bin", content: bytes.Repeat([]byte{0x11}, 600*1024)},
		testEntry{name: "b.bin", content: bytes.Repeat([]byte{0x22}, 600*1024)},
	)
	mustValidate(t, two, q)
}

func TestValidateTotalLimitBoundary(t *testing.T) {
	const halfMiB = 512 * 1024
	q := Quota{MaxZipEntries: 5000, MaxEntryUncompressedMB: 200, MaxTotalUncompressedMB: 1, MaxRecursionDepth: 10}

	exact := buildTestZip(t,
		testEntry{name: "a.bin", content: bytes.Repeat([]byte{0x11}, halfMiB)},
		testEntry{name: "b.bin", content: bytes.Repeat([]byte{0x22}, halfMiB)},
	)
	st := mustValidate(t, exact, q)
	if st.TotalUncompressed != int64(2*halfMiB) {
		t.Fatalf("TotalUncompressed = %d, want %d", st.TotalUncompressed, 2*halfMiB)
	}

	over := buildTestZip(t,
		testEntry{name: "a.bin", content: bytes.Repeat([]byte{0x11}, halfMiB)},
		testEntry{name: "b.bin", content: bytes.Repeat([]byte{0x22}, halfMiB+1)},
	)
	_, err := validateData(t, over, q)
	wantSentinel(t, err, ErrLimitExceeded)
}

// TestValidateTotalLimitInterruptsStreaming proves a zip bomb is detected
// while the stream is being consumed: the validator must stop reading the
// source long before the whole archive has been delivered.
func TestValidateTotalLimitInterruptsStreaming(t *testing.T) {
	const chunk = 600 * 1024
	data := buildTestZip(t,
		testEntry{name: "a.bin", content: bytes.Repeat([]byte{0x11}, chunk)},
		testEntry{name: "b.bin", content: bytes.Repeat([]byte{0x22}, chunk)},
		testEntry{name: "c.bin", content: bytes.Repeat([]byte{0x33}, chunk)},
	)
	q := Quota{MaxZipEntries: 5000, MaxEntryUncompressedMB: 200, MaxTotalUncompressedMB: 1, MaxRecursionDepth: 10}
	tr := &trackingReaderAt{r: bytes.NewReader(data)}
	_, err := Validate(context.Background(), tr, int64(len(data)), q)
	wantSentinel(t, err, ErrLimitExceeded)
	if tr.read >= int64(len(data)) {
		t.Fatalf("validator consumed the whole archive (%d of %d bytes) before detecting the quota violation", tr.read, len(data))
	}
}

func TestValidateNestedZip(t *testing.T) {
	inner := buildTestZip(t,
		testEntry{name: "inner/leaf1.txt", content: []byte("leaf1")},
		testEntry{name: "inner/leaf2.txt", content: []byte("leaf2")},
	)
	outer := buildTestZip(t,
		testEntry{name: "readme.txt", content: []byte("readme")},
		testEntry{name: "mods/inner.zip", content: inner},
	)
	st := mustValidate(t, outer, defaultQuota)
	if st.EntryCount != 4 {
		t.Fatalf("EntryCount = %d, want 4", st.EntryCount)
	}
	want := int64(len(inner) + len("readme") + len("leaf1") + len("leaf2"))
	if st.TotalUncompressed != want {
		t.Fatalf("TotalUncompressed = %d, want %d", st.TotalUncompressed, want)
	}
	if st.MaxNestedDepth != 1 {
		t.Fatalf("MaxNestedDepth = %d, want 1", st.MaxNestedDepth)
	}
}

func TestValidateNestedZipRecursionLimit(t *testing.T) {
	chain1 := nestedChain(t, 1)
	chain2 := nestedChain(t, 2)
	chain3 := nestedChain(t, 3)

	q1 := Quota{MaxZipEntries: 5000, MaxEntryUncompressedMB: 200, MaxTotalUncompressedMB: 2048, MaxRecursionDepth: 1}
	q2 := Quota{MaxZipEntries: 5000, MaxEntryUncompressedMB: 200, MaxTotalUncompressedMB: 2048, MaxRecursionDepth: 2}
	q3 := Quota{MaxZipEntries: 5000, MaxEntryUncompressedMB: 200, MaxTotalUncompressedMB: 2048, MaxRecursionDepth: 3}

	mustValidate(t, chain1, q1) // depth 1 == limit
	mustValidate(t, chain2, q2) // depth 2 == limit

	_, err := validateData(t, chain2, q1)
	wantSentinel(t, err, ErrLimitExceeded)

	_, err = validateData(t, chain3, q2)
	wantSentinel(t, err, ErrLimitExceeded)

	mustValidate(t, chain3, q3) // depth 3 == limit

	// A zero quota is invalid config, not "no nesting allowed".
	_, err = validateData(t, chain1, Quota{MaxZipEntries: 5000, MaxEntryUncompressedMB: 200, MaxTotalUncompressedMB: 2048, MaxRecursionDepth: 0})
	if err == nil || errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("zero recursion depth must be an invalid-quota error, got %v", err)
	}
}

// nestedChain builds `levels` nested zips: level 1 is a zip containing a
// plain zip, level 2 contains that zip, and so on.
func nestedChain(t *testing.T, levels int) []byte {
	t.Helper()
	inner := buildTestZip(t, testEntry{name: "leaf.txt", content: []byte("leaf")})
	for i := 0; i < levels; i++ {
		inner = buildTestZip(t, testEntry{name: fmt.Sprintf("lvl%d.zip", i), content: inner})
	}
	return inner
}

func TestQuotaFromConfig(t *testing.T) {
	q := QuotaFromConfig(config.ArchiveScanConfig{
		MaxUploadSizeMB:        500,
		MaxZipEntries:          2,
		MaxEntryUncompressedMB: 1,
		MaxTotalUncompressedMB: 2,
		MaxRecursionDepth:      1,
		ScanTimeoutSec:         120,
		RetryBackoffSec:        []int{60, 300, 1800},
		URLTTLSec:              300,
	})
	if q.MaxZipEntries != 2 || q.MaxEntryUncompressedMB != 1 || q.MaxTotalUncompressedMB != 2 || q.MaxRecursionDepth != 1 {
		t.Fatalf("QuotaFromConfig mapping mismatch: %+v", q)
	}

	// The config-derived quota must be enforced by the validator.
	data := buildTestZip(t,
		testEntry{name: "a.txt", content: []byte("a")},
		testEntry{name: "b.txt", content: []byte("b")},
		testEntry{name: "c.txt", content: []byte("c")},
	)
	_, err := validateData(t, data, q)
	wantSentinel(t, err, ErrLimitExceeded)
}

func TestValidateMalformedArchive(t *testing.T) {
	cases := [][]byte{
		[]byte("this is definitely not a zip archive"),
		{0x50, 0x4B, 0x03, 0x04}, // truncated local file header
	}
	for _, data := range cases {
		_, err := validateData(t, data, defaultQuota)
		if err == nil {
			t.Fatalf("expected error for malformed archive %q", data)
		}
		for _, s := range []error{ErrEncrypted, ErrPathInvalid, ErrLinkForbidden, ErrLimitExceeded} {
			if errors.Is(err, s) {
				t.Fatalf("malformed archive must not map to business sentinel %v, got %v", s, err)
			}
		}
	}
}

func TestValidateRejectsInvalidQuota(t *testing.T) {
	data := buildTestZip(t, testEntry{name: "a.txt", content: []byte("a")})
	quotas := []Quota{
		{}, // all zeros
		{MaxZipEntries: -1, MaxEntryUncompressedMB: 1, MaxTotalUncompressedMB: 1, MaxRecursionDepth: 1},
	}
	for _, q := range quotas {
		_, err := validateData(t, data, q)
		if err == nil {
			t.Fatalf("expected error for invalid quota %+v", q)
		}
		for _, s := range []error{ErrEncrypted, ErrPathInvalid, ErrLinkForbidden, ErrLimitExceeded} {
			if errors.Is(err, s) {
				t.Fatalf("invalid quota must not map to business sentinel %v, got %v", s, err)
			}
		}
	}
}

func TestValidateRespectsContextCancellation(t *testing.T) {
	data := buildTestZip(t, testEntry{name: "a.txt", content: []byte("a")})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Validate(ctx, bytes.NewReader(data), int64(len(data)), defaultQuota)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// TestValidateRandomGarbageNeverPanics is the fuzz-style robustness check:
// arbitrary bytes (including PK-prefixed noise) must never panic and must
// always return either success or an error.
func TestValidateRandomGarbageNeverPanics(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	quotas := []Quota{defaultQuota, {MaxZipEntries: 3, MaxEntryUncompressedMB: 1, MaxTotalUncompressedMB: 4, MaxRecursionDepth: 2}}
	for iter := 0; iter < 300; iter++ {
		n := rng.Intn(4096)
		data := make([]byte, n)
		if _, err := rng.Read(data); err != nil {
			t.Fatal(err)
		}
		if iter%3 == 0 && len(data) >= 4 {
			copy(data[:4], []byte{0x50, 0x4B, 0x03, 0x04}) // PK magic to exercise nested-zip sniffing
		}
		q := quotas[iter%len(quotas)]
		_, err := validateData(t, data, q)
		if err != nil {
			_ = err // any error is acceptable; the point is "no panic and no execution"
		}
	}
}

func TestValidateSentinelCodes(t *testing.T) {
	// The sentinel strings ARE the stable API error codes (spec §7), so the
	// S04 gate can map them onto handler responses without translating.
	if ErrEncrypted.Error() != "ARCHIVE_ENCRYPTED" {
		t.Fatalf("ErrEncrypted = %q", ErrEncrypted.Error())
	}
	if ErrPathInvalid.Error() != "ARCHIVE_PATH_INVALID" {
		t.Fatalf("ErrPathInvalid = %q", ErrPathInvalid.Error())
	}
	if ErrLinkForbidden.Error() != "ARCHIVE_LINK_FORBIDDEN" {
		t.Fatalf("ErrLinkForbidden = %q", ErrLinkForbidden.Error())
	}
	if ErrLimitExceeded.Error() != "ARCHIVE_LIMIT_EXCEEDED" {
		t.Fatalf("ErrLimitExceeded = %q", ErrLimitExceeded.Error())
	}
}

func FuzzValidateNeverPanics(f *testing.F) {
	valid, err := buildZip(testEntry{name: "mod/file.txt", content: []byte("hello")})
	if err != nil {
		f.Fatal(err)
	}
	inner, err := buildZip(testEntry{name: "leaf.txt", content: []byte("leaf")})
	if err != nil {
		f.Fatal(err)
	}
	nested, err := buildZip(
		testEntry{name: "readme.txt", content: []byte("readme")},
		testEntry{name: "mods/inner.zip", content: inner},
	)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(valid)
	f.Add(nested)
	f.Add([]byte("garbage bytes that are not a zip"))
	f.Add([]byte{0x50, 0x4B, 0x03, 0x04, 0x00, 0x01, 0x02})

	q := Quota{MaxZipEntries: 100, MaxEntryUncompressedMB: 4, MaxTotalUncompressedMB: 8, MaxRecursionDepth: 3}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := Validate(context.Background(), bytes.NewReader(data), int64(len(data)), q)
		_ = err
	})
}
