package migration

import (
	"context"
	"fmt"
	"testing"
)

// TestPostgresFixtureUpgradeAcrossRepoGapAndBackfill mirrors the repository's
// deliberate 061 gap (versions 001..060 and 062 exist in the migrations
// directory, 061 does not). The runner must tolerate the hole, apply a later
// 063 in a separate run, and backfill (补缺) a lower-version file inserted
// afterwards without rerunning anything already applied. It never adds the
// 061 file that the repository deliberately omits.
func TestPostgresFixtureUpgradeAcrossRepoGapAndBackfill(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()

	for i := 1; i <= 60; i++ {
		writeMigrationFile(t, dir, fmt.Sprintf("%03d_probe_%03d.sql", i, i),
			fmt.Sprintf("CREATE TABLE IF NOT EXISTS probe_%03d (id int);\n", i))
	}
	writeMigrationFile(t, dir, "062_self_managed.sql",
		"BEGIN;\nCREATE TABLE IF NOT EXISTS probe_062 (id int);\nCOMMIT;\n")

	meta := transactionalMetadata()
	meta.SelfTransactional = []NonTransactionalMigration{{
		Filename:   "062_self_managed.sql",
		Reason:     "self-managed transaction mirroring the repository's 062 shape",
		Reviewer:   "test",
		ReviewedAt: "2026-08-08T00:00:00Z",
		Preconditions: []SQLCondition{{
			Description: "runner schema is ready", Query: "SELECT true",
		}},
		Postconditions: []SQLCondition{{
			Description: "probe_062 exists", Query: "SELECT to_regclass('public.probe_062') IS NOT NULL",
		}},
		IdempotentResume: "transaction rolls back",
		Reconciliation:   "not required",
	}}
	runner := newRunner(t, db, dir, meta)

	// Run 1: versions 001..060 and 062 apply despite the 061 hole.
	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("first run across the 061 gap: %v", err)
	}
	if len(result.Applied) != 61 {
		t.Fatalf("first run applied %d migrations, want 61 (001..060 + 062)", len(result.Applied))
	}
	if !tableExists(t, db, "probe_060") || !tableExists(t, db, "probe_062") {
		t.Fatal("probe_060/probe_062 missing after first run")
	}
	if tableExists(t, db, "probe_061") {
		t.Fatal("probe_061 must not exist: version 061 is deliberately absent")
	}

	// Run 2: this ticket's 063 lands; only it must apply.
	writeMigrationFile(t, dir, "063_content_media_metadata.sql",
		"CREATE TABLE IF NOT EXISTS probe_063 (id int);\n")
	result, err = runner.Run(context.Background())
	if err != nil {
		t.Fatalf("second run applying 063: %v", err)
	}
	if len(result.Applied) != 1 || result.Applied[0] != "063_content_media_metadata.sql" {
		t.Fatalf("second run applied %v, want only 063", result.Applied)
	}
	if !tableExists(t, db, "probe_063") {
		t.Fatal("probe_063 missing after second run")
	}

	// Run 3: a lower-version file inserted later is backfilled (补缺) without
	// rerunning already-applied higher versions.
	writeMigrationFile(t, dir, "061_added_later.sql",
		"CREATE TABLE IF NOT EXISTS probe_061 (id int);\n")
	result, err = runner.Run(context.Background())
	if err != nil {
		t.Fatalf("third run backfilling 061: %v", err)
	}
	if len(result.Applied) != 1 || result.Applied[0] != "061_added_later.sql" {
		t.Fatalf("third run applied %v, want only the backfilled 061", result.Applied)
	}
	if !tableExists(t, db, "probe_061") {
		t.Fatal("probe_061 missing after backfill")
	}
	if err := runner.Verify(context.Background()); err != nil {
		t.Fatalf("Verify after backfill: %v", err)
	}
}
