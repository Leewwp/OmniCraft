package migration

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	ledgerTable   = "schema_migrations"
	attemptsTable = "schema_migration_attempts"
)

// LedgerEntry is one row of the forward-only migration ledger.
type LedgerEntry struct {
	Version   int
	Filename  string
	Checksum  string
	AppliedAt time.Time
}

// Ledger is the set of migrations already applied to a database, keyed by
// filename. It intentionally compares applied filename/version/checksum sets
// rather than a maximum version alone, so a later backfill of a missing
// lower-numbered migration is still applied.
type Ledger struct {
	entries []LedgerEntry
	byName  map[string]LedgerEntry
}

// Entries returns the ledger rows in repository order (version, filename).
func (l *Ledger) Entries() []LedgerEntry {
	if l == nil {
		return nil
	}
	return l.entries
}

// Checksum returns the recorded checksum for an applied filename.
func (l *Ledger) Checksum(filename string) (string, bool) {
	if l == nil || l.byName == nil {
		return "", false
	}
	entry, ok := l.byName[filename]
	return entry.Checksum, ok
}

func (l *Ledger) add(entry LedgerEntry) {
	if l.byName == nil {
		l.byName = make(map[string]LedgerEntry)
	}
	l.byName[entry.Filename] = entry
	l.entries = append(l.entries, entry)
}

// LoadLedger reads the applied migration ledger from schema_migrations.
func LoadLedger(ctx context.Context, conn *sql.Conn) (*Ledger, error) {
	rows, err := conn.QueryContext(ctx, fmt.Sprintf(
		"SELECT version, filename, checksum, applied_at FROM %s ORDER BY version, filename", ledgerTable))
	if err != nil {
		return nil, fmt.Errorf("query migration ledger: %w", err)
	}
	defer rows.Close()

	ledger := &Ledger{}
	for rows.Next() {
		var entry LedgerEntry
		if err := rows.Scan(&entry.Version, &entry.Filename, &entry.Checksum, &entry.AppliedAt); err != nil {
			return nil, fmt.Errorf("scan migration ledger row: %w", err)
		}
		ledger.add(entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate migration ledger: %w", err)
	}
	return ledger, nil
}

// EnsureSchema creates the ledger and attempt-audit tables when missing.
func EnsureSchema(ctx context.Context, conn *sql.Conn) error {
	statements := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			version INTEGER NOT NULL,
			filename TEXT NOT NULL,
			checksum TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (version),
			UNIQUE (filename)
		)`, ledgerTable),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id BIGSERIAL PRIMARY KEY,
			version INTEGER NOT NULL,
			filename TEXT NOT NULL,
			checksum TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('started', 'succeeded', 'failed', 'reconciled')),
			error_class TEXT NOT NULL DEFAULT '',
			error_digest TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			approval_ref TEXT NOT NULL DEFAULT '',
			attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`, attemptsTable),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS approval_ref TEXT NOT NULL DEFAULT ''`, attemptsTable),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_version_status ON %s (version, status)`, attemptsTable, attemptsTable),
	}
	for _, statement := range statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("ensure migration schema: %w", err)
		}
	}
	return nil
}
