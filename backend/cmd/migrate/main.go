package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"omnicraft/backend/internal/migration"
	"omnicraft/backend/internal/observability"

	_ "github.com/lib/pq"
)

func main() {
	started := time.Now().UTC()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	if err := run(os.Args[1:], os.Stdout); err != nil {
		writeMigrationMetrics(false, time.Now().UTC())
		slog.Error("migrate failed", "error", err.Error(), "started_at", started, "finished_at", time.Now().UTC())
		os.Exit(1)
	}
	writeMigrationMetrics(true, time.Now().UTC())
	slog.Info("migrate finished", "started_at", started, "finished_at", time.Now().UTC())
}

// writeMigrationMetrics emits bounded success/failure/last-success metrics to
// the textfile collector directory when METRICS_TEXTFILE_DIR is configured.
func writeMigrationMetrics(ok bool, at time.Time) {
	dir := strings.TrimSpace(os.Getenv("METRICS_TEXTFILE_DIR"))
	if dir == "" {
		return
	}
	lines := []string{
		"# HELP omnicraft_migration_status Latest migration run outcome (1 success, 0 failure).",
		"# TYPE omnicraft_migration_status gauge",
	}
	lastSuccess := ""
	if ok {
		lines = append(lines, "omnicraft_migration_status 1")
		lastSuccess = fmt.Sprintf("%.3f", float64(at.UnixNano())/1e9)
	} else {
		lines = append(lines, "omnicraft_migration_status 0")
		if existing, readErr := os.ReadFile(filepath.Join(dir, "omnicraft_migration.prom")); readErr == nil {
			for _, line := range strings.Split(string(existing), "\n") {
				if strings.HasPrefix(line, "omnicraft_migration_last_success_timestamp_seconds ") {
					lastSuccess = strings.TrimSpace(strings.TrimPrefix(line, "omnicraft_migration_last_success_timestamp_seconds "))
					break
				}
			}
		}
	}
	if lastSuccess != "" {
		lines = append(lines, "# HELP omnicraft_migration_last_success_timestamp_seconds Unix time of the last successful migration run.")
		lines = append(lines, "# TYPE omnicraft_migration_last_success_timestamp_seconds gauge")
		lines = append(lines, "omnicraft_migration_last_success_timestamp_seconds "+lastSuccess)
	}
	if err := observability.WriteTextfile(dir, "omnicraft_migration.prom", lines); err != nil {
		slog.Error("failed to write migration textfile metrics", "error", err.Error())
	}
}

func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(out)
	dsn := fs.String("DSN", os.Getenv("DB_DSN"), "PostgreSQL DSN; defaults to DB_DSN")
	dir := fs.String("Dir", "", "migrations directory")
	metadataPath := fs.String("Metadata", "", "metadata.json path; defaults to <Dir>/metadata.json")
	reconcileVersions := fs.String("ReconcileVersions", "", "comma-separated migration versions the operator explicitly approves for retry")
	reconcileApproval := fs.String("ReconcileApproval", "", "durable ticket/change/incident reference approving reconciliation")
	dryRun := fs.Bool("DryRun", false, "compute the plan without applying migrations")
	verifyOnly := fs.Bool("VerifyOnly", false, "verify the ledger matches the repository without applying")
	summaryPath := fs.String("SummaryPath", "", "write a machine-readable migration-summary.json")
	timeout := fs.Duration("Timeout", 10*time.Minute, "maximum run duration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	if strings.TrimSpace(*dsn) == "" {
		return fmt.Errorf("a DSN is required (-DSN or DB_DSN)")
	}
	if strings.TrimSpace(*dir) == "" {
		return fmt.Errorf("a migrations directory is required (-Dir)")
	}
	if *metadataPath == "" {
		*metadataPath = *dir + "/metadata.json"
	}
	if *dryRun && *verifyOnly {
		return fmt.Errorf("-DryRun and -VerifyOnly are mutually exclusive")
	}

	meta, err := migration.LoadMetadata(*metadataPath)
	if err != nil {
		return err
	}

	reconcile, err := parseReconcileVersions(*reconcileVersions)
	if err != nil {
		return err
	}
	if len(reconcile) > 0 && strings.TrimSpace(*reconcileApproval) == "" {
		return fmt.Errorf("-ReconcileApproval is required with -ReconcileVersions")
	}
	if len(reconcile) == 0 && strings.TrimSpace(*reconcileApproval) != "" {
		return fmt.Errorf("-ReconcileApproval requires -ReconcileVersions")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	db, err := sql.Open("postgres", *dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	runner := &migration.Runner{
		DB:                     db,
		Dir:                    *dir,
		Metadata:               meta,
		ReconcileVersions:      reconcile,
		ReconciliationApproval: strings.TrimSpace(*reconcileApproval),
	}

	if *dryRun {
		return dryRunPlan(ctx, runner, *summaryPath)
	}
	if *verifyOnly {
		return verifyLedger(ctx, runner, *summaryPath)
	}

	result, err := runner.Run(ctx)
	if err != nil {
		return err
	}
	if *summaryPath != "" {
		if err := writeSummary(*summaryPath, result); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "applied %d migrations (%d skipped)\n", len(result.Applied), result.Skipped)
	return nil
}

func parseReconcileVersions(raw string) (map[int]bool, error) {
	versions := make(map[int]bool)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		version, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid -ReconcileVersions value %q: %w", part, err)
		}
		versions[version] = true
	}
	return versions, nil
}

func dryRunPlan(ctx context.Context, runner *migration.Runner, summaryPath string) error {
	plan, err := runner.Plan(ctx)
	if err != nil {
		return err
	}
	if summaryPath != "" {
		summary := map[string]any{
			"mode":        "dry-run",
			"to_apply":    plan,
			"finished_at": time.Now().UTC(),
		}
		return writeJSON(summaryPath, summary)
	}
	return nil
}

func verifyLedger(ctx context.Context, runner *migration.Runner, summaryPath string) error {
	if err := runner.Verify(ctx); err != nil {
		return err
	}
	if summaryPath != "" {
		summary := map[string]any{
			"mode":        "verify-only",
			"status":      "verified",
			"finished_at": time.Now().UTC(),
		}
		return writeJSON(summaryPath, summary)
	}
	return nil
}

func writeSummary(path string, result migration.Result) error {
	summary := map[string]any{
		"mode":        "migrate",
		"applied":     result.Applied,
		"skipped":     result.Skipped,
		"started_at":  result.StartedAt,
		"finished_at": result.FinishedAt,
	}
	return writeJSON(path, summary)
}

func writeJSON(path string, value any) error {
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode summary: %w", err)
	}
	contents = append(contents, '\n')
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		return fmt.Errorf("write summary %s: %w", path, err)
	}
	return nil
}
