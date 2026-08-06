package main

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := run(args, &buf)
	return buf.String(), err
}

func TestCLIRejectsMissingDSN(t *testing.T) {
	_, err := runCLI(t, "-Dir", t.TempDir())
	if err == nil {
		t.Fatal("missing DSN must fail")
	}
	if !strings.Contains(err.Error(), "DSN") {
		t.Errorf("error must mention DSN, got: %v", err)
	}
}

func TestCLIRejectsMissingDir(t *testing.T) {
	_, err := runCLI(t, "-DSN", "host=localhost dbname=postgres sslmode=disable")
	if err == nil {
		t.Fatal("missing migrations directory must fail")
	}
	if !strings.Contains(err.Error(), "Dir") {
		t.Errorf("error must mention Dir, got: %v", err)
	}
}

func TestCLIRejectsUnknownFlag(t *testing.T) {
	_, err := runCLI(t, "-DSN", "x", "-Dir", ".", "-Nope")
	if err == nil {
		t.Fatal("unknown flag must fail")
	}
}

func TestCLIRejectsUnconnectedDSN(t *testing.T) {
	_, err := runCLI(t, "-DSN", "host=127.0.0.1 port=1 user=omnicraft password=omnicraft dbname=nope sslmode=disable",
		"-Dir", t.TempDir(), "-Metadata", filepath.Join("..", "..", "migrations", "metadata.json"))
	if err == nil {
		t.Fatal("an unreachable DSN must fail")
	}
}

func TestCLIRejectsDryRunWithVerifyOnly(t *testing.T) {
	_, err := runCLI(t, "-DSN", "x", "-Dir", ".", "-DryRun", "-VerifyOnly")
	if err == nil {
		t.Fatal("-DryRun with -VerifyOnly must fail")
	}
}

func TestCLIRejectsMissingMetadata(t *testing.T) {
	_, err := runCLI(t, "-DSN", "host=127.0.0.1 port=1 user=omnicraft dbname=nope sslmode=disable",
		"-Dir", t.TempDir(), "-Metadata", filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("missing metadata.json must fail")
	}
}

func TestParseReconcileVersions(t *testing.T) {
	got, err := parseReconcileVersions("47, 49")
	if err != nil {
		t.Fatalf("parseReconcileVersions: %v", err)
	}
	want := map[int]bool{47: true, 49: true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseReconcileVersions = %v, want %v", got, want)
	}
	if got, err := parseReconcileVersions(""); err != nil || len(got) != 0 {
		t.Errorf("empty reconcile must yield empty set, got %v, %v", got, err)
	}
	if _, err := parseReconcileVersions("abc"); err == nil {
		t.Error("non-numeric reconcile version must fail")
	}
}

func TestCLIRequiresApprovalReferenceForReconciliation(t *testing.T) {
	_, err := runCLI(t,
		"-DSN", "host=127.0.0.1 port=1 user=omnicraft dbname=nope sslmode=disable",
		"-Dir", filepath.Join("..", "..", "migrations"),
		"-ReconcileVersions", "47")
	if err == nil || !strings.Contains(err.Error(), "ReconcileApproval") {
		t.Fatalf("reconciliation without a durable approval reference must fail, got: %v", err)
	}
}

func TestCLIWithEnvDSN(t *testing.T) {
	t.Setenv("DB_DSN", "host=localhost dbname=postgres sslmode=disable")
	_, err := runCLI(t, "-Dir", t.TempDir(), "-Metadata", filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("missing metadata must fail")
	}
	if strings.Contains(err.Error(), "a DSN is required") {
		t.Errorf("DB_DSN from the environment must satisfy the DSN requirement, got: %v", err)
	}
}
