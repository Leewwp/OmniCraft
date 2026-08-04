package migration

import (
	"testing"
	"time"
)

func testLedger(t *testing.T, entries ...LedgerEntry) *Ledger {
	t.Helper()
	ledger := &Ledger{}
	for _, e := range entries {
		ledger.add(e)
	}
	return ledger
}

func testFile(version int, name string) File {
	return File{Version: version, Filename: name, Checksum: "sha256:" + name}
}

func TestLedgerChecksumLookup(t *testing.T) {
	ledger := testLedger(t, LedgerEntry{
		Version: 1, Filename: "001_a.sql", Checksum: "abc",
		AppliedAt: time.Now().UTC(),
	})
	if got, ok := ledger.Checksum("001_a.sql"); !ok || got != "abc" {
		t.Errorf("Checksum = %q, %v; want %q, true", got, ok, "abc")
	}
	if _, ok := ledger.Checksum("999_none.sql"); ok {
		t.Error("Checksum must report missing filename")
	}
}

func TestBuildPlanAppliesOnlyMissingFiles(t *testing.T) {
	ledger := testLedger(t, LedgerEntry{Version: 1, Filename: "001_a.sql", Checksum: "sha256:001_a.sql"})
	files := []File{testFile(1, "001_a.sql"), testFile(2, "002_b.sql")}
	plan, err := BuildPlan(ledger, files)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.ToApply) != 1 || plan.ToApply[0].Filename != "002_b.sql" {
		t.Errorf("ToApply = %+v, want only 002_b.sql", plan.ToApply)
	}
}

func TestBuildPlanAppliesMissingLowerVersionGap(t *testing.T) {
	ledger := testLedger(t,
		LedgerEntry{Version: 1, Filename: "001_a.sql", Checksum: "sha256:001_a.sql"},
		LedgerEntry{Version: 3, Filename: "003_c.sql", Checksum: "sha256:003_c.sql"},
	)
	files := []File{testFile(1, "001_a.sql"), testFile(2, "002_b.sql"), testFile(3, "003_c.sql")}
	plan, err := BuildPlan(ledger, files)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.ToApply) != 1 || plan.ToApply[0].Filename != "002_b.sql" {
		t.Errorf("ToApply = %+v, want only the missing lower 002_b.sql", plan.ToApply)
	}
}

func TestBuildPlanRejectsChecksumDrift(t *testing.T) {
	ledger := testLedger(t, LedgerEntry{Version: 1, Filename: "001_a.sql", Checksum: "sha256:old"})
	files := []File{testFile(1, "001_a.sql")}
	if _, err := BuildPlan(ledger, files); err == nil {
		t.Fatal("BuildPlan must reject checksum drift on applied files")
	}
}

func TestBuildPlanRejectsLedgerVersionDrift(t *testing.T) {
	ledger := testLedger(t, LedgerEntry{Version: 9, Filename: "001_a.sql", Checksum: "sha256:001_a.sql"})
	files := []File{testFile(1, "001_a.sql")}
	if _, err := BuildPlan(ledger, files); err == nil {
		t.Fatal("BuildPlan must reject version drift for an applied filename")
	}
}

func TestBuildPlanRejectsLedgerMissingFile(t *testing.T) {
	ledger := testLedger(t, LedgerEntry{Version: 7, Filename: "007_gone.sql", Checksum: "sha256:007_gone.sql"})
	files := []File{testFile(1, "001_a.sql")}
	if _, err := BuildPlan(ledger, files); err == nil {
		t.Fatal("BuildPlan must reject a ledger entry whose file is missing from the repository")
	}
}

func TestBuildPlanStableOrder(t *testing.T) {
	ledger := testLedger(t)
	files := []File{testFile(10, "010_j.sql"), testFile(2, "002_b.sql"), testFile(5, "005_e.sql")}
	plan, err := BuildPlan(ledger, files)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.ToApply) != 3 {
		t.Fatalf("ToApply = %+v", plan.ToApply)
	}
	for i, want := range []string{"002_b.sql", "005_e.sql", "010_j.sql"} {
		if plan.ToApply[i].Filename != want {
			t.Errorf("ToApply[%d] = %q, want %q", i, plan.ToApply[i].Filename, want)
		}
	}
}
