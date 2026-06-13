package testutil

import "testing"

func TestRewriteDSNDatabaseSupportsURLDSNs(t *testing.T) {
	got := rewriteDSNDatabase("postgres://omnicraft:secret@127.0.0.1:5432/postgres?sslmode=disable", "omnicraft_test")

	want := "postgres://omnicraft:secret@127.0.0.1:5432/omnicraft_test?sslmode=disable"
	if got != want {
		t.Fatalf("rewriteDSNDatabase() = %q, want %q", got, want)
	}
}

func TestSplitSQLStatementsKeepsSemicolonsInsideStringsAndDollarQuotes(t *testing.T) {
	sqlText := `
		INSERT INTO demo(body) VALUES ('alpha;beta');
		CREATE FUNCTION demo_fn() RETURNS void AS $fn$
		BEGIN
			PERFORM 1;
			PERFORM 2;
		END;
		$fn$ LANGUAGE plpgsql;
		INSERT INTO demo(body) VALUES ('omega');
	`

	got := splitSQLStatements(sqlText)
	if len(got) != 3 {
		t.Fatalf("splitSQLStatements() len = %d, want 3; got %#v", len(got), got)
	}
}
