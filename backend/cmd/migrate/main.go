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
	"strconv"
	"strings"
	"time"

	"omnicraft/backend/internal/migration"

	_ "github.com/lib/pq"
)

func main() {
	started := time.Now().UTC()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	if err := run(os.Args[1:], os.Stdout); err != nil {
		slog.Error("migrate failed", "error", err.Error(), "started_at", started, "finished_at", time.Now().UTC())
		os.Exit(1)
	}
	slog.Info("migrate finished", "started_at", started, "finished_at", time.Now().UTC())
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
