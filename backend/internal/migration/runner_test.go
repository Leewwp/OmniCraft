package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const repoMigrationsDir = "../../migrations"

func TestLoadMetadataDefaultTransactional(t *testing.T) {
	meta, err := LoadMetadata(filepath.Join(repoMigrationsDir, "metadata.json"))
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	if !meta.DefaultTransactional {
		t.Fatal("metadata.json must declare transactional-by-default migrations")
	}
	if len(meta.NonTransactional) == 0 {
		t.Fatal("metadata.json must list at least the CONCURRENTLY migrations as non-transactional")
	}
	if len(meta.SelfTransactional) == 0 {
		t.Fatal("metadata.json must declare the self-transactional migration (059_create_content_series.sql)")
	}
}

func TestSelfTransactionalEntryComplete(t *testing.T) {
	meta, err := LoadMetadata(filepath.Join(repoMigrationsDir, "metadata.json"))
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	if len(meta.SelfTransactional) != 4 {
		t.Fatalf("self_transactional has %d entries, want 4 (057..059, 062)", len(meta.SelfTransactional))
	}
	declared := make(map[string]bool)
	for _, entry := range meta.SelfTransactional {
		declared[entry.Filename] = true
	}
	for _, want := range []string{"057_add_broadcast_channel.sql", "058_create_collections.sql", "059_create_content_series.sql", "062_notification_broadcast_idempotency.sql"} {
		if !declared[want] {
			t.Errorf("self_transactional must declare %s", want)
		}
	}
	for _, entry := range meta.SelfTransactional {
		if entry.Reason == "" || entry.Reviewer == "" || entry.ReviewedAt == "" {
			t.Errorf("%s: self-transactional entry must declare reason, reviewer and reviewed_at", entry.Filename)
		}
		if len(entry.Preconditions) == 0 || len(entry.Postconditions) == 0 {
			t.Errorf("%s: self-transactional entry must declare pre/postconditions", entry.Filename)
		}
		if entry.IdempotentResume == "" || entry.Reconciliation == "" {
			t.Errorf("%s: self-transactional entry must declare idempotent/resume and reconciliation", entry.Filename)
		}
	}
}

func TestNonTransactionalEntryFieldsComplete(t *testing.T) {
	meta, err := LoadMetadata(filepath.Join(repoMigrationsDir, "metadata.json"))
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	for _, entry := range meta.NonTransactional {
		if entry.Filename == "" {
			t.Error("non-transactional entry missing filename")
		}
		if entry.Reason == "" {
			t.Errorf("%s: missing reason", entry.Filename)
		}
		if entry.Reviewer == "" {
			t.Errorf("%s: missing reviewer", entry.Filename)
		}
		if entry.ReviewedAt == "" {
			t.Errorf("%s: missing reviewed_at", entry.Filename)
		}
		if len(entry.Preconditions) == 0 {
			t.Errorf("%s: missing machine-checkable preconditions", entry.Filename)
		}
		if len(entry.Postconditions) == 0 {
			t.Errorf("%s: missing machine-checkable postconditions", entry.Filename)
		}
		if entry.IdempotentResume == "" {
			t.Errorf("%s: missing idempotent/resume strategy", entry.Filename)
		}
		if entry.Reconciliation == "" {
			t.Errorf("%s: missing reconciliation instructions", entry.Filename)
		}
	}
}

func TestMetadataFilenamesExistInMigrationsDir(t *testing.T) {
	meta, err := LoadMetadata(filepath.Join(repoMigrationsDir, "metadata.json"))
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	for _, entry := range meta.NonTransactional {
		path := filepath.Join(repoMigrationsDir, entry.Filename)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("non-transactional filename %q does not exist: %v", entry.Filename, err)
		}
	}
	for _, entry := range meta.SelfTransactional {
		path := filepath.Join(repoMigrationsDir, entry.Filename)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("self-transactional filename %q does not exist: %v", entry.Filename, err)
		}
	}
}

func TestSelfManagedTransactionsDeclared(t *testing.T) {
	// Every migration whose first top-level statement opens BEGIN must be
	// declared in metadata.json self_transactional.
	meta, err := LoadMetadata(filepath.Join(repoMigrationsDir, "metadata.json"))
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	declared := make(map[string]bool)
	for _, entry := range meta.SelfTransactional {
		declared[entry.Filename] = true
	}
	files, err := ScanFiles(repoMigrationsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		statements, err := readStatements(file.Path)
		if err != nil {
			t.Fatal(err)
		}
		if len(statements) > 0 && isSelfManagedTransaction(statements[0]) && !declared[file.Filename] {
			t.Errorf("migration %s manages its own transaction but is not declared in metadata.json", file.Filename)
		}
	}
}

func TestLoadMetadataRejectsMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metadata.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMetadata(path); err == nil {
		t.Fatal("LoadMetadata must reject malformed JSON")
	}
}

func TestNonTransactionalReferencesOnlyConcurrentlyMigrations(t *testing.T) {
	meta, err := LoadMetadata(filepath.Join(repoMigrationsDir, "metadata.json"))
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	for _, entry := range meta.NonTransactional {
		content, err := os.ReadFile(filepath.Join(repoMigrationsDir, entry.Filename))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.ToUpper(string(content)), "CONCURRENTLY") {
			t.Errorf("%s is declared non-transactional but does not use CONCURRENTLY", entry.Filename)
		}
		if strings.Contains(strings.ToUpper(string(content)), "CREATE INDEX CONCURRENTLY") == false {
			t.Errorf("%s lacks CREATE INDEX CONCURRENTLY", entry.Filename)
		}
	}
}

func TestMetadataSchemaFileExists(t *testing.T) {
	if _, err := os.Stat(filepath.Join(repoMigrationsDir, "metadata.schema.json")); err != nil {
		t.Fatalf("metadata.schema.json missing: %v", err)
	}
}
