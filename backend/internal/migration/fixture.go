package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// FixtureSourceMigration is one real migration file covered by the fixture.
type FixtureSourceMigration struct {
	Version  int    `json:"version"`
	Filename string `json:"filename"`
	Checksum string `json:"checksum"`
}

// FixtureLedgerRow is one schema_migrations row captured at generation time.
type FixtureLedgerRow struct {
	Version  int    `json:"version"`
	Filename string `json:"filename"`
	Checksum string `json:"checksum"`
}

// FixtureManifest is the machine-readable provenance of a historical fixture.
type FixtureManifest struct {
	SchemaVersion     int                      `json:"schema_version"`
	GeneratorVersion  string                   `json:"generator_version"`
	Baseline          string                   `json:"baseline"`
	Image             string                   `json:"image"`
	ImageDigest       string                   `json:"image_digest"`
	DumpChecksum      string                   `json:"dump_checksum"`
	GeneratedAt       string                   `json:"generated_at"`
	GenerationCommand string                   `json:"generation_command"`
	Migrations        []FixtureSourceMigration `json:"migrations"`
	Ledger            []FixtureLedgerRow       `json:"ledger"`
}

const (
	pinnedFixtureImage       = "pgvector/pgvector:pg16"
	pinnedFixtureImageDigest = "pgvector/pgvector@sha256:a36250871de0833b8757561c72f2477ef1ddd1101afa4e617fb552e0de514c6b"
)

// ValidateFixture proves a historical fixture is genuine: the dump checksum
// matches its sidecar, the manifest is complete, the source migration
// checksums match the repository files, and the captured ledger exactly
// represents the baseline migration set. Hand-edited dumps, mismatched
// manifests and missing source checksums are rejected.
func ValidateFixture(sqlPath, shaPath, manifestPath, migrationsDir string) (FixtureManifest, error) {
	checksum, err := ChecksumFile(sqlPath)
	if err != nil {
		return FixtureManifest{}, fmt.Errorf("checksum fixture dump: %w", err)
	}
	shaContents, err := os.ReadFile(filepath.Clean(shaPath))
	if err != nil {
		return FixtureManifest{}, fmt.Errorf("read fixture sha256 sidecar: %w", err)
	}
	expected := strings.TrimSpace(string(shaContents))
	if !strings.EqualFold(expected, checksum) {
		return FixtureManifest{}, fmt.Errorf("fixture dump checksum mismatch: file %s, sidecar %s", checksum, expected)
	}

	manifest, err := loadFixtureManifest(manifestPath)
	if err != nil {
		return FixtureManifest{}, err
	}
	if manifest.DumpChecksum == "" {
		return FixtureManifest{}, fmt.Errorf("fixture manifest: missing dump_checksum")
	}
	if !strings.EqualFold(manifest.DumpChecksum, checksum) {
		return FixtureManifest{}, fmt.Errorf("fixture manifest dump_checksum mismatch: file %s, manifest %s", checksum, manifest.DumpChecksum)
	}
	if manifest.SchemaVersion < 1 {
		return FixtureManifest{}, fmt.Errorf("fixture manifest: schema_version must be >= 1")
	}
	if manifest.GeneratorVersion == "" {
		return FixtureManifest{}, fmt.Errorf("fixture manifest: missing generator_version")
	}
	if manifest.Image != pinnedFixtureImage {
		return FixtureManifest{}, fmt.Errorf("fixture manifest: image %q, want pinned %q", manifest.Image, pinnedFixtureImage)
	}
	if manifest.ImageDigest == "" {
		return FixtureManifest{}, fmt.Errorf("fixture manifest: missing image digest")
	}
	if manifest.ImageDigest != pinnedFixtureImageDigest {
		return FixtureManifest{}, fmt.Errorf("fixture manifest: image digest %q, want pinned %q", manifest.ImageDigest, pinnedFixtureImageDigest)
	}
	if manifest.GenerationCommand == "" {
		return FixtureManifest{}, fmt.Errorf("fixture manifest: missing generation command")
	}
	baseline, err := strconv.Atoi(manifest.Baseline)
	if err != nil {
		return FixtureManifest{}, fmt.Errorf("fixture manifest: invalid baseline %q", manifest.Baseline)
	}

	files, err := ScanFiles(migrationsDir)
	if err != nil {
		return FixtureManifest{}, fmt.Errorf("scan repository migrations: %w", err)
	}
	expectedSources := make([]FixtureSourceMigration, 0, baseline)
	for _, file := range files {
		if file.Version > baseline {
			continue
		}
		expectedSources = append(expectedSources, FixtureSourceMigration{
			Version:  file.Version,
			Filename: file.Filename,
			Checksum: file.Checksum,
		})
	}
	if len(expectedSources) != baseline {
		return FixtureManifest{}, fmt.Errorf("fixture manifest: repository has %d migrations <= baseline %s, want %d", len(expectedSources), manifest.Baseline, baseline)
	}
	if !equalSourceMigrations(manifest.Migrations, expectedSources) {
		return FixtureManifest{}, fmt.Errorf("fixture manifest: source migration/checksum set does not match repository files 001..%s", manifest.Baseline)
	}
	if !equalLedgerRows(manifest.Ledger, expectedSources) {
		return FixtureManifest{}, fmt.Errorf("fixture manifest: ledger does not exactly represent migrations 001..%s", manifest.Baseline)
	}
	dumpLedger, err := fixtureLedgerRows(sqlPath)
	if err != nil {
		return FixtureManifest{}, err
	}
	if !equalLedgerRows(dumpLedger, expectedSources) {
		return FixtureManifest{}, fmt.Errorf("fixture SQL: embedded ledger does not exactly represent migrations 001..%s", manifest.Baseline)
	}
	return manifest, nil
}

var fixtureLedgerInsertPattern = regexp.MustCompile(`(?m)^INSERT INTO public\.schema_migrations VALUES \(([0-9]+), '([^']+)', '([0-9a-f]{64})',`)

func fixtureLedgerRows(path string) ([]FixtureLedgerRow, error) {
	contents, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read fixture ledger from SQL: %w", err)
	}
	matches := fixtureLedgerInsertPattern.FindAllSubmatch(contents, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("fixture SQL: no schema_migrations ledger rows found")
	}
	rows := make([]FixtureLedgerRow, 0, len(matches))
	for _, match := range matches {
		version, err := strconv.Atoi(string(match[1]))
		if err != nil {
			return nil, fmt.Errorf("fixture SQL: invalid ledger version %q: %w", match[1], err)
		}
		rows = append(rows, FixtureLedgerRow{
			Version: version, Filename: string(match[2]), Checksum: string(match[3]),
		})
	}
	return rows, nil
}

func loadFixtureManifest(path string) (FixtureManifest, error) {
	contents, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return FixtureManifest{}, fmt.Errorf("read fixture manifest: %w", err)
	}
	var manifest FixtureManifest
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return FixtureManifest{}, fmt.Errorf("parse fixture manifest: %w", err)
	}
	return manifest, nil
}

func equalSourceMigrations(a, b []FixtureSourceMigration) bool {
	if len(a) != len(b) {
		return false
	}
	normalize := func(in []FixtureSourceMigration) []FixtureSourceMigration {
		out := append([]FixtureSourceMigration(nil), in...)
		sort.Slice(out, func(i, j int) bool {
			if out[i].Version != out[j].Version {
				return out[i].Version < out[j].Version
			}
			return out[i].Filename < out[j].Filename
		})
		return out
	}
	a, b = normalize(a), normalize(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalLedgerRows(rows []FixtureLedgerRow, sources []FixtureSourceMigration) bool {
	if len(rows) != len(sources) {
		return false
	}
	normalized := append([]FixtureLedgerRow(nil), rows...)
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Version != normalized[j].Version {
			return normalized[i].Version < normalized[j].Version
		}
		return normalized[i].Filename < normalized[j].Filename
	})
	for i := range sources {
		if normalized[i].Version != sources[i].Version ||
			normalized[i].Filename != sources[i].Filename ||
			normalized[i].Checksum != sources[i].Checksum {
			return false
		}
	}
	return true
}

// fixtureChecksum returns the hex SHA-256 of a string, used by tests that
// mutate fixture copies.
func fixtureChecksum(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}

// LoadFixtureSQL loads a pg_dump plain fixture into an empty database on the
// dedicated connection. psql meta-commands (\restrict/\unrestrict) emitted by
// modern pg_dump are stripped; the search_path reset emitted by pg_dump is
// undone at the end so the session does not leak an empty search_path back
// into the connection pool.
func LoadFixtureSQL(ctx context.Context, conn *sql.Conn, path string) error {
	contents, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("read fixture %s: %w", path, err)
	}
	filtered := make([]byte, 0, len(contents))
	for _, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, `\restrict `) || strings.HasPrefix(trimmed, `\unrestrict `) {
			continue
		}
		filtered = append(filtered, []byte(line+"\n")...)
	}
	for _, statement := range splitSQLStatements(string(filtered)) {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply fixture statement from %s: %w", path, err)
		}
	}
	if _, err := conn.ExecContext(ctx, "SET search_path = public, pg_catalog"); err != nil {
		return fmt.Errorf("reset search_path after fixture load: %w", err)
	}
	return nil
}
