package testutil

import (
	"regexp"
	"sync"
	"testing"
)

func TestNewEphemeralDatabaseNameIsSafeAndUniqueUnderConcurrency(t *testing.T) {
	const workers = 256
	validName := regexp.MustCompile(`^omnicraft_test_[a-f0-9]{32}$`)
	names := make(chan string, workers)
	errors := make(chan error, workers)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name, err := newEphemeralDatabaseName()
			if err != nil {
				errors <- err
				return
			}
			names <- name
		}()
	}
	wg.Wait()
	close(names)
	close(errors)

	for err := range errors {
		t.Fatalf("newEphemeralDatabaseName: %v", err)
	}
	seen := make(map[string]struct{}, workers)
	for name := range names {
		if !validName.MatchString(name) {
			t.Fatalf("database name %q is not a safe fixed-width identifier", name)
		}
		if _, exists := seen[name]; exists {
			t.Fatalf("duplicate database name %q", name)
		}
		seen[name] = struct{}{}
	}
	if len(seen) != workers {
		t.Fatalf("generated %d unique names, want %d", len(seen), workers)
	}
}

func TestValidateEphemeralAdminDSNRequiresLoopbackPostgresDatabase(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{"default", defaultAdminDSN, false},
		{"localhost", "host=localhost port=5432 user=omnicraft dbname=postgres sslmode=disable", false},
		{"ipv6 loopback", "host=::1 port=5432 user=omnicraft dbname=postgres sslmode=disable", false},
		{"remote host", "host=db.example.com port=5432 user=omnicraft dbname=postgres sslmode=disable", true},
		{"non-admin database", "host=127.0.0.1 port=5432 user=omnicraft dbname=omnicraft sslmode=disable", true},
		{"remote url", "postgres://omnicraft:secret@db.example.com:5432/postgres?sslmode=disable", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEphemeralAdminDSN(tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateEphemeralAdminDSN(%q) error = %v, wantErr %v", tt.dsn, err, tt.wantErr)
			}
		})
	}
}

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
