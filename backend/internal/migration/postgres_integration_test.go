package migration

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

const (
	defaultTestAdminDSN = "host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=postgres sslmode=disable"
	testdataDir         = "testdata"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	adminDSN := os.Getenv("OMNICRAFT_TEST_POSTGRES_ADMIN_DSN")
	if strings.TrimSpace(adminDSN) == "" {
		adminDSN = os.Getenv("OMNICRAFT_TEST_POSTGRES_DSN")
	}
	if strings.TrimSpace(adminDSN) == "" {
		adminDSN = defaultTestAdminDSN
	}
	admin, err := sql.Open("postgres", adminDSN)
	if err != nil {
		t.Fatalf("open postgres admin connection: %v", err)
	}
	if err := admin.Ping(); err != nil {
		t.Fatalf("ping postgres admin connection (is postgres running?): %v", err)
	}

	name, err := newTestDatabaseName()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`CREATE DATABASE "` + name + `"`); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	db, err := sql.Open("postgres", rewriteDatabaseName(adminDSN, name))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
		_, _ = admin.Exec(`
			SELECT pg_terminate_backend(pid)
			FROM pg_stat_activity
			WHERE datname = $1 AND pid <> pg_backend_pid()
		`, name)
		_, _ = admin.Exec(`DROP DATABASE IF EXISTS "` + name + `"`)
		_ = admin.Close()
	})
	return db
}

func newTestDatabaseName() (string, error) {
	entropy, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return "omnicraft_migration_test_" + entropy, nil
}

func randomHex(n int) (string, error) {
	buffer := make([]byte, n)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func rewriteDatabaseName(dsn, name string) string {
	parts := strings.Fields(dsn)
	values := map[string]string{}
	for _, part := range parts {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			values[key] = value
		}
	}
	values["dbname"] = name
	out := make([]string, 0, len(values))
	for _, key := range []string{"host", "port", "user", "password", "dbname", "sslmode"} {
		if value, ok := values[key]; ok {
			out = append(out, fmt.Sprintf("%s=%s", key, value))
		}
	}
	return strings.Join(out, " ")
}

func transactionalMetadata() *Metadata {
	return &Metadata{SchemaVersion: 1, DefaultTransactional: true}
}

func realMetadata(t *testing.T) *Metadata {
	t.Helper()
	meta, err := LoadMetadata(filepath.Join(repoMigrationsDir, "metadata.json"))
	if err != nil {
		t.Fatalf("load real metadata.json: %v", err)
	}
	return meta
}

func newRunner(t *testing.T, db *sql.DB, dir string, meta *Metadata) *Runner {
	t.Helper()
	return &Runner{DB: db, Dir: dir, Metadata: meta, ReconcileVersions: map[int]bool{}}
}

func writeMigrationFile(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var exists bool
	err := db.QueryRow(
		"SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_class WHERE relname = $1 AND relkind IN ('r', 'p', 'i'))",
		table).Scan(&exists)
	if err != nil {
		t.Fatalf("check table %s: %v", table, err)
	}
	return exists
}

func ledgerCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	err := db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return 0
		}
		t.Fatalf("count ledger: %v", err)
	}
	return count
}

func waitForCondition(t *testing.T, timeout time.Duration, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestPostgresEmptyToLatest(t *testing.T) {
	db := openTestDB(t)
	runner := newRunner(t, db, repoMigrationsDir, realMetadata(t))

	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("run empty-to-latest migrations: %v", err)
	}
	if len(result.Applied) != len(expectedAllMigrations()) {
		t.Errorf("applied %d migrations, want %d", len(result.Applied), len(expectedAllMigrations()))
	}
	if result.FinishedAt.IsZero() || result.FinishedAt.Before(result.StartedAt) {
		t.Errorf("migration result has invalid timestamps: started=%s finished=%s", result.StartedAt, result.FinishedAt)
	}
	if !tableExists(t, db, "users") {
		t.Error("users table missing after empty-to-latest")
	}
	if ledgerCount(t, db) != len(expectedAllMigrations()) {
		t.Errorf("ledger count %d, want %d", ledgerCount(t, db), len(expectedAllMigrations()))
	}

	if err := runner.Verify(context.Background()); err != nil {
		t.Errorf("Verify after empty-to-latest: %v", err)
	}
}

func TestPostgresSelfManagedMigrationAndLedgerRollbackTogether(t *testing.T) {
	db := openTestDB(t)
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if _, err := db.Exec(`CREATE FUNCTION reject_migration_ledger() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'ledger insert rejected for test';
END;
$$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER reject_migration_ledger BEFORE INSERT ON schema_migrations FOR EACH ROW EXECUTE FUNCTION reject_migration_ledger()`); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	writeMigrationFile(t, dir, "001_self.sql", "BEGIN;\nCREATE TABLE self_managed_probe (id int);\nCOMMIT;\n")
	meta := transactionalMetadata()
	meta.SelfTransactional = []NonTransactionalMigration{{
		Filename: "001_self.sql", Reason: "test self-managed transaction", Reviewer: "test",
		ReviewedAt:    "2026-08-05T00:00:00Z",
		Preconditions: []SQLCondition{{Description: "database is available", Query: "SELECT true"}},
		Postconditions: []SQLCondition{{
			Description: "self-managed table exists", Query: "SELECT to_regclass('public.self_managed_probe') IS NOT NULL",
		}},
		IdempotentResume: "transaction rolls back", Reconciliation: "not required",
	}}
	runner := newRunner(t, db, dir, meta)
	if _, err := runner.Run(context.Background()); err == nil {
		t.Fatal("ledger trigger must fail the migration")
	}
	if tableExists(t, db, "self_managed_probe") {
		t.Fatal("self-managed schema change must roll back when its ledger insert fails")
	}
}

func TestPostgresNonTransactionalPreconditionIsExecuted(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	writeMigrationFile(t, dir, "001_nt.sql", "CREATE INDEX CONCURRENTLY idx_missing_probe ON missing_probe (id);\n")
	meta := transactionalMetadata()
	meta.NonTransactional = []NonTransactionalMigration{{
		Filename: "001_nt.sql", Reason: "test precondition", Reviewer: "test",
		ReviewedAt:       "2026-08-05T00:00:00Z",
		Preconditions:    []SQLCondition{{Description: "deliberate false precondition", Query: "SELECT false"}},
		Postconditions:   []SQLCondition{{Description: "deliberate true postcondition", Query: "SELECT true"}},
		IdempotentResume: "drop invalid index then retry",
		Reconciliation:   "inspect and approve",
	}}
	runner := newRunner(t, db, dir, meta)
	_, err := runner.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "precondition") {
		t.Fatalf("machine-checkable false precondition must block execution, got: %v", err)
	}
	var attempts int
	if err := db.QueryRow("SELECT count(*) FROM schema_migration_attempts").Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 {
		t.Fatalf("precondition failure must occur before an attempt starts, got %d attempts", attempts)
	}
}

func expectedAllMigrations() []string {
	files, err := ScanFiles(repoMigrationsDir)
	if err != nil {
		panic(err)
	}
	names := make([]string, 0, len(files))
	for _, file := range files {
		names = append(names, file.Filename)
	}
	return names
}

func TestPostgresNoRerun(t *testing.T) {
	db := openTestDB(t)
	runner := newRunner(t, db, repoMigrationsDir, realMetadata(t))

	if _, err := runner.Run(context.Background()); err != nil {
		t.Fatalf("first run: %v", err)
	}
	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(result.Applied) != 0 {
		t.Errorf("second run applied %d migrations, want 0 (no rerun)", len(result.Applied))
	}
	if result.Skipped != len(expectedAllMigrations()) {
		t.Errorf("second run skipped %d, want %d", result.Skipped, len(expectedAllMigrations()))
	}
}

func TestPostgresChecksumRejection(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	writeMigrationFile(t, dir, "001_a.sql", "CREATE TABLE t1 (id int);\n")
	writeMigrationFile(t, dir, "002_b.sql", "CREATE TABLE t2 (id int);\n")
	runner := newRunner(t, db, dir, transactionalMetadata())

	if _, err := runner.Run(context.Background()); err != nil {
		t.Fatalf("first run: %v", err)
	}
	writeMigrationFile(t, dir, "001_a.sql", "CREATE TABLE t1 (id int);\n-- tampered\n")
	_, err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("checksum drift on an applied migration must be rejected")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("error must mention checksum drift, got: %v", err)
	}
	if got := ledgerCount(t, db); got != 2 {
		t.Errorf("ledger count after rejected run = %d, want 2 (ledger unchanged)", got)
	}
}

func TestPostgresMissingLowerVersion(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	writeMigrationFile(t, dir, "001_a.sql", "CREATE TABLE t1 (id int);\n")
	writeMigrationFile(t, dir, "003_c.sql", "CREATE TABLE t3 (id int);\n")
	runner := newRunner(t, db, dir, transactionalMetadata())

	if _, err := runner.Run(context.Background()); err != nil {
		t.Fatalf("first run: %v", err)
	}
	writeMigrationFile(t, dir, "002_b.sql", "CREATE TABLE t2 (id int);\n")
	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(result.Applied) != 1 || result.Applied[0] != "002_b.sql" {
		t.Errorf("applied %v, want only the inserted missing 002_b.sql", result.Applied)
	}
	if !tableExists(t, db, "t2") {
		t.Error("t2 missing after backfill")
	}
}

func TestPostgresTransactionalRollback(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	writeMigrationFile(t, dir, "001_ok.sql", "CREATE TABLE t1 (id int);\n")
	writeMigrationFile(t, dir, "002_bad.sql", "CREATE TABLE t2 (id int);\nCREATE TABLE t1 (id int);\n")
	runner := newRunner(t, db, dir, transactionalMetadata())

	_, err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("a failing transactional migration must fail the run")
	}
	if !tableExists(t, db, "t1") {
		t.Error("t1 must exist: the earlier migration committed in its own transaction")
	}
	if tableExists(t, db, "t2") {
		t.Error("t2 must not exist: the failing migration transaction must roll back")
	}
	if got := ledgerCount(t, db); got != 1 {
		t.Errorf("ledger count after rollback = %d, want 1 (only 001 recorded)", got)
	}
	var version int
	if err := db.QueryRow("SELECT version FROM schema_migrations").Scan(&version); err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if version != 1 {
		t.Errorf("ledger version = %d, want 1 (failed migration must not be recorded)", version)
	}
}

func TestPostgresConcurrentRunnersOnlyOneHoldsLock(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	writeMigrationFile(t, dir, "001_fast.sql", "CREATE TABLE t1 (id int);\n")
	writeMigrationFile(t, dir, "002_slow.sql", "CREATE TABLE t2 (id int);\nSELECT pg_sleep(3);\n")
	runner := newRunner(t, db, dir, transactionalMetadata())

	errs := make(chan error, 2)
	go func() {
		_, err := runner.Run(context.Background())
		errs <- err
	}()

	waitForCondition(t, 15*time.Second, "first runner to apply 001", func() bool {
		return ledgerCount(t, db) >= 1
	})

	// While the first runner holds the advisory lock, a second connection
	// must not be able to take it.
	probe, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var acquired bool
	if err := probe.QueryRowContext(context.Background(),
		"SELECT pg_try_advisory_lock($1)", advisoryLockKey).Scan(&acquired); err != nil {
		t.Fatalf("probe advisory lock: %v", err)
	}
	if acquired {
		_, _ = probe.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", advisoryLockKey)
		t.Fatal("second connection acquired the advisory lock while the first runner was active")
	}
	_ = probe.Close()

	go func() {
		_, err := runner.Run(context.Background())
		errs <- err
	}()

	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("runner %d failed: %v", i, err)
		}
	}

	if got := ledgerCount(t, db); got != 2 {
		t.Errorf("ledger count = %d, want exactly 2 (each migration applied exactly once)", got)
	}
	if !tableExists(t, db, "t2") {
		t.Error("t2 missing after concurrent runs")
	}

	// The lock must be released after both runners finish.
	var released bool
	if err := db.QueryRow("SELECT pg_try_advisory_lock($1)", advisoryLockKey).Scan(&released); err != nil {
		t.Fatal(err)
	}
	if !released {
		t.Fatal("advisory lock was not released after the runners finished")
	}
	_, _ = db.Exec("SELECT pg_advisory_unlock($1)", advisoryLockKey)
}

func TestPostgresNonTransactionalAttemptBlockAndReconcile(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	writeMigrationFile(t, dir, "001_a.sql", "CREATE TABLE t1 (id int);\n")
	writeMigrationFile(t, dir, "002_nt.sql", "CREATE INDEX CONCURRENTLY idx_t2_probe ON t2 (id);\n")
	writeMigrationFile(t, dir, "003_c.sql", "CREATE TABLE t3 (id int);\n")

	meta := &Metadata{
		SchemaVersion:        1,
		DefaultTransactional: true,
		NonTransactional: []NonTransactionalMigration{{
			Filename:      "002_nt.sql",
			Reason:        "CONCURRENTLY cannot run in a transaction",
			Reviewer:      "test",
			ReviewedAt:    "2026-08-04T00:00:00Z",
			Preconditions: []SQLCondition{{Description: "test allows execution", Query: "SELECT true"}},
			Postconditions: []SQLCondition{{
				Description: "idx_t2_probe is valid",
				Query:       "SELECT EXISTS (SELECT 1 FROM pg_class c JOIN pg_index i ON i.indexrelid = c.oid WHERE c.relname = 'idx_t2_probe' AND i.indisvalid)",
			}},
			IdempotentResume: "IF NOT EXISTS",
			Reconciliation:   "approve with -ReconcileVersions 2",
		}},
	}
	runner := newRunner(t, db, dir, meta)

	// Run 1: 001 applies, 002 fails (t2 does not exist), 003 must not run.
	_, err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("non-transactional migration with missing table must fail")
	}
	if !tableExists(t, db, "t1") || tableExists(t, db, "t3") {
		t.Error("001 must be applied and 003 must be blocked after the failed non-transactional migration")
	}
	var class, digest string
	if err := db.QueryRow(
		"SELECT error_class, error_digest FROM schema_migration_attempts WHERE version = 2 AND status = 'failed'",
	).Scan(&class, &digest); err != nil {
		t.Fatalf("failed attempt not recorded: %v", err)
	}
	if class == "" || digest == "" {
		t.Error("failed attempt must record a redacted error class and digest")
	}
	if strings.Contains(digest, "pq:") {
		t.Error("raw error text must not leak into the attempt digest")
	}

	// Run 2: blind retry must be blocked until reconciliation.
	_, err = runner.Run(context.Background())
	if err == nil {
		t.Fatal("a failed attempt must block blind retry and later migrations")
	}
	if !strings.Contains(err.Error(), "reconciliation") {
		t.Errorf("blocking error must mention reconciliation, got: %v", err)
	}

	// Fix the file, then approve retry explicitly.
	writeMigrationFile(t, dir, "002_nt.sql", "CREATE TABLE t2 (id int);\nCREATE INDEX CONCURRENTLY IF NOT EXISTS idx_t2_probe ON t2 (id);\n")
	runner.ReconcileVersions = map[int]bool{2: true}
	runner.ReconciliationApproval = "test-approval:nontransactional-retry"
	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("run after explicit reconciliation: %v", err)
	}
	if len(result.Applied) != 2 {
		t.Errorf("applied %v, want 002 and 003", result.Applied)
	}
	if !tableExists(t, db, "t2") || !tableExists(t, db, "t3") {
		t.Error("t2/t3 must exist after reconciled retry")
	}

	var reconciled int
	if err := db.QueryRow(
		"SELECT count(*) FROM schema_migration_attempts WHERE version = 2 AND status = 'reconciled'",
	).Scan(&reconciled); err != nil {
		t.Fatal(err)
	}
	if reconciled != 1 {
		t.Errorf("reconciled attempts = %d, want exactly 1 (approval evidence)", reconciled)
	}
	var approvalRef string
	if err := db.QueryRow(
		"SELECT approval_ref FROM schema_migration_attempts WHERE version = 2 AND status = 'reconciled'",
	).Scan(&approvalRef); err != nil {
		t.Fatal(err)
	}
	if approvalRef != "test-approval:nontransactional-retry" {
		t.Errorf("reconciliation approval_ref = %q, want durable test reference", approvalRef)
	}
}

func TestPostgresHistoricalFixtureToLatest(t *testing.T) {
	db := openTestDB(t)
	fixtureSQL := filepath.Join(testdataDir, "historical-050.sql")
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := LoadFixtureSQL(context.Background(), conn, fixtureSQL); err != nil {
		t.Fatalf("load historical fixture: %v", err)
	}
	_ = conn.Close()

	runner := newRunner(t, db, repoMigrationsDir, realMetadata(t))
	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("upgrade fixture to latest: %v", err)
	}
	for _, name := range result.Applied {
		version := name[:3]
		if version <= "050" {
			t.Errorf("upgrade re-applied fixture migration %s", name)
		}
	}
	if len(result.Applied) != len(expectedAllMigrations())-50 {
		t.Errorf("upgrade applied %d migrations, want %d", len(result.Applied), len(expectedAllMigrations())-50)
	}
	var tags int
	if err := db.QueryRow("SELECT count(*) FROM tags").Scan(&tags); err != nil {
		t.Fatal(err)
	}
	if tags < 3 {
		t.Errorf("fixture seed data lost: tags = %d, want >= 3", tags)
	}
	if err := runner.Verify(context.Background()); err != nil {
		t.Errorf("Verify after fixture upgrade: %v", err)
	}
}

func TestPostgresUnknownAttemptBlocksUntilReconcile(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	writeMigrationFile(t, dir, "001_a.sql", "CREATE TABLE t1 (id int);\n")
	writeMigrationFile(t, dir, "002_nt.sql", "CREATE INDEX CONCURRENTLY idx_t2_probe ON t2 (id);\n")

	meta := &Metadata{
		SchemaVersion:        1,
		DefaultTransactional: true,
		NonTransactional: []NonTransactionalMigration{{
			Filename: "002_nt.sql", Reason: "CONCURRENTLY", Reviewer: "test",
			ReviewedAt:    "2026-08-04T00:00:00Z",
			Preconditions: []SQLCondition{{Description: "test allows execution", Query: "SELECT true"}},
			Postconditions: []SQLCondition{{
				Description: "idx_t2_probe is valid",
				Query:       "SELECT EXISTS (SELECT 1 FROM pg_class c JOIN pg_index i ON i.indexrelid = c.oid WHERE c.relname = 'idx_t2_probe' AND i.indisvalid)",
			}},
			IdempotentResume: "IF NOT EXISTS", Reconciliation: "reconcile 002",
		}},
	}
	runner := newRunner(t, db, dir, meta)

	// First run applies 001 then fails 002 (t2 does not exist). Replace the
	// recorded 'failed' attempt with 'started' to model a runner that crashed
	// mid-statement, whose outcome is unknown.
	if _, err := runner.Run(context.Background()); err == nil {
		t.Fatal("first run must fail while t2 is missing")
	}
	if _, err := db.Exec("UPDATE schema_migration_attempts SET status = 'started' WHERE version = 2"); err != nil {
		t.Fatal(err)
	}

	_, err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("an unknown (started) attempt must block blind retry and later migrations")
	}
	if !strings.Contains(err.Error(), "reconciliation") {
		t.Errorf("blocking error must mention reconciliation, got: %v", err)
	}

	// Fix the file, then approve retry explicitly.
	writeMigrationFile(t, dir, "002_nt.sql", "CREATE TABLE t2 (id int);\nCREATE INDEX CONCURRENTLY IF NOT EXISTS idx_t2_probe ON t2 (id);\n")
	runner.ReconcileVersions = map[int]bool{2: true}
	runner.ReconciliationApproval = "test-approval:unknown-attempt"
	if _, err := runner.Run(context.Background()); err != nil {
		t.Fatalf("run after reconciling the unknown attempt: %v", err)
	}
	if !tableExists(t, db, "t2") {
		t.Error("t2 must exist after reconciled retry")
	}
}

func TestPostgresUnknownAttemptBlocksLaterMigrationsEvenWhenLedgerExists(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	writeMigrationFile(t, dir, "001_a.sql", "CREATE TABLE t1 (id int);\n")
	runner := newRunner(t, db, dir, transactionalMetadata())
	if _, err := runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_migration_attempts
        (version, filename, checksum, status) VALUES (1, '001_a.sql', 'unknown', 'started')`); err != nil {
		t.Fatal(err)
	}
	writeMigrationFile(t, dir, "002_b.sql", "CREATE TABLE t2 (id int);\n")
	_, err := runner.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "reconciliation") {
		t.Fatalf("unknown attempt must block later migrations even with a ledger row, got: %v", err)
	}
	if tableExists(t, db, "t2") {
		t.Fatal("later migration ran despite unresolved unknown attempt")
	}
}

func TestPostgresVerifyDetectsMissingLedgerRow(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	writeMigrationFile(t, dir, "001_a.sql", "CREATE TABLE t1 (id int);\n")
	writeMigrationFile(t, dir, "002_b.sql", "CREATE TABLE t2 (id int);\n")
	runner := newRunner(t, db, dir, transactionalMetadata())
	if _, err := runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Simulate a partially restored database missing one ledger row.
	if _, err := db.Exec("DELETE FROM schema_migrations WHERE version = 2"); err != nil {
		t.Fatal(err)
	}
	if err := runner.Verify(context.Background()); err == nil {
		t.Fatal("Verify must fail when a ledger row is missing after restore")
	}
}

func TestHistoricalFixtureValidation(t *testing.T) {
	sqlPath := filepath.Join(testdataDir, "historical-050.sql")
	shaPath := filepath.Join(testdataDir, "historical-050.sha256")
	manifestPath := filepath.Join(testdataDir, "historical-050.manifest.json")

	manifest, err := ValidateFixture(sqlPath, shaPath, manifestPath, repoMigrationsDir)
	if err != nil {
		t.Fatalf("committed fixture must validate: %v", err)
	}
	if manifest.Baseline != "050" {
		t.Errorf("baseline = %q, want 050", manifest.Baseline)
	}
	if len(manifest.Ledger) != 50 {
		t.Errorf("ledger rows = %d, want 50", len(manifest.Ledger))
	}
}

func TestHistoricalFixtureRejectsTamperedDump(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "historical-050.sql")
	contents, err := os.ReadFile(filepath.Join(testdataDir, "historical-050.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sqlPath, append(contents, []byte("-- hand-edited\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	copyFixtureSidecars(t, dir)
	_, err = ValidateFixture(sqlPath,
		filepath.Join(dir, "historical-050.sha256"),
		filepath.Join(dir, "historical-050.manifest.json"),
		repoMigrationsDir)
	if err == nil {
		t.Fatal("a hand-edited dump must be rejected by checksum validation")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("error must mention checksum mismatch, got: %v", err)
	}
}

func TestHistoricalFixtureRejectsMismatchedManifest(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"historical-050.sql", "historical-050.sha256"} {
		contents, err := os.ReadFile(filepath.Join(testdataDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifestContents, err := os.ReadFile(filepath.Join(testdataDir, "historical-050.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(manifestContents),
		`"image": "pgvector/pgvector:pg16"`, `"image": "postgres:16"`, 1)
	if err := os.WriteFile(filepath.Join(dir, "historical-050.manifest.json"), []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = ValidateFixture(
		filepath.Join(dir, "historical-050.sql"),
		filepath.Join(dir, "historical-050.sha256"),
		filepath.Join(dir, "historical-050.manifest.json"),
		repoMigrationsDir)
	if err == nil {
		t.Fatal("a mismatched manifest must be rejected")
	}
}

func TestHistoricalFixtureRejectsLedgerTamperWithSynchronizedChecksums(t *testing.T) {
	dir := t.TempDir()
	contents, err := os.ReadFile(filepath.Join(testdataDir, "historical-050.sql"))
	if err != nil {
		t.Fatal(err)
	}
	contents = []byte(strings.Replace(string(contents),
		"INSERT INTO public.schema_migrations VALUES (50,", "INSERT INTO public.schema_migrations VALUES (51,", 1))
	checksum := fixtureChecksum(contents)
	if err := os.WriteFile(filepath.Join(dir, "historical-050.sql"), contents, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "historical-050.sha256"), []byte(checksum+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := loadFixtureManifest(filepath.Join(testdataDir, "historical-050.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest.DumpChecksum = checksum
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "historical-050.manifest.json"), manifestJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateFixture(
		filepath.Join(dir, "historical-050.sql"),
		filepath.Join(dir, "historical-050.sha256"),
		filepath.Join(dir, "historical-050.manifest.json"),
		repoMigrationsDir); err == nil {
		t.Fatal("fixture validation must compare ledger rows inside the SQL dump, not only synchronized sidecars")
	}
}

func TestHistoricalFixtureRejectsMissingSourceChecksum(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"historical-050.sql", "historical-050.sha256"} {
		contents, err := os.ReadFile(filepath.Join(testdataDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifestContents, err := os.ReadFile(filepath.Join(testdataDir, "historical-050.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(manifestContents), `"filename": "001_users.sql"`, `"filename": "001_tampered.sql"`, 1)
	if err := os.WriteFile(filepath.Join(dir, "historical-050.manifest.json"), []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = ValidateFixture(
		filepath.Join(dir, "historical-050.sql"),
		filepath.Join(dir, "historical-050.sha256"),
		filepath.Join(dir, "historical-050.manifest.json"),
		repoMigrationsDir)
	if err == nil {
		t.Fatal("a manifest with a missing/edited source checksum must be rejected")
	}
}

func copyFixtureSidecars(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"historical-050.sha256", "historical-050.manifest.json"} {
		contents, err := os.ReadFile(filepath.Join(testdataDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
