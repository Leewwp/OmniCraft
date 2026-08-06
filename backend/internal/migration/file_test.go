package migration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFilenameValid(t *testing.T) {
	cases := []struct {
		name    string
		wantVer int
	}{
		{"001_users.sql", 1},
		{"050_verification_and_terms.sql", 50},
		{"062_notification_broadcast_idempotency.sql", 62},
		{"000_leading_zero.sql", 0},
		{"999_max.sql", 999},
	}
	for _, tc := range cases {
		got, err := ParseFilename(tc.name)
		if err != nil {
			t.Errorf("ParseFilename(%q) unexpected error: %v", tc.name, err)
			continue
		}
		if got != tc.wantVer {
			t.Errorf("ParseFilename(%q) = %d, want %d", tc.name, got, tc.wantVer)
		}
	}
}

func TestParseFilenameInvalid(t *testing.T) {
	invalid := []string{
		"users.sql",
		"1_users.sql",
		"001_users",
		"001_Users.sql",
		"001_user s.sql",
		"001_über.sql",
		"001_users.txt",
		"abc_users.sql",
		"0011_users.sql",
		"001_users.sql.bak",
		"",
		"001_-leading-dash.sql",
	}
	for _, name := range invalid {
		if _, err := ParseFilename(name); err == nil {
			t.Errorf("ParseFilename(%q) expected error, got none", name)
		}
	}
}

func TestScanFilesStableOrder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"010_ten.sql", "002_two.sql", "005_five.sql", "002_two_dup.sql"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ScanFiles(dir); err == nil {
		t.Fatal("ScanFiles must reject duplicate versions")
	}
}

func TestScanFilesOrderAndChecksum(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"010_ten.sql":  "SELECT 10;\n",
		"002_two.sql":  "SELECT 2;\n",
		"005_five.sql": "SELECT 5;\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ScanFiles(dir)
	if err != nil {
		t.Fatalf("ScanFiles: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ScanFiles returned %d files, want 3", len(got))
	}
	for i, want := range []string{"002_two.sql", "005_five.sql", "010_ten.sql"} {
		if got[i].Filename != want {
			t.Errorf("order[%d] = %q, want %q", i, got[i].Filename, want)
		}
	}
	if got[0].Version != 2 || got[1].Version != 5 || got[2].Version != 10 {
		t.Errorf("unexpected versions: %+v", got)
	}
}

func TestScanFilesMissingDirectory(t *testing.T) {
	if _, err := ScanFiles(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("ScanFiles must fail on a missing directory")
	}
}

func TestChecksumFileKnownValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.sql")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ChecksumFile(path)
	if err != nil {
		t.Fatalf("ChecksumFile: %v", err)
	}
	const want = "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"
	if got != want {
		t.Errorf("ChecksumFile = %q, want %q", got, want)
	}
}

func TestChecksumFileMissing(t *testing.T) {
	if _, err := ChecksumFile(filepath.Join(t.TempDir(), "missing.sql")); err == nil {
		t.Fatal("ChecksumFile must fail on a missing file")
	}
}

func TestParseFilenameRejectsDirectories(t *testing.T) {
	if _, err := ParseFilename("001_users.sql/"); err == nil {
		t.Fatal("ParseFilename must reject directory-looking names")
	}
}
