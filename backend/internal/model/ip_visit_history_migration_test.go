package model

import (
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"

	"omnicraft/backend/internal/testutil"
)

// TestIPVisitHistoryMigration covers the empty-database upgrade path for 066:
// the ip_visit_history table with composite primary key (user_id, ip_id),
// both cascade foreign keys (users and ips), the stable recent-ordering index
// and the idempotent forward-only re-application contract.
func TestIPVisitHistoryMigration(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	requireIPVisitHistoryBaseTables(t, db)

	migration := filepath.Join("..", "..", "migrations", "066_ip_visit_history.sql")
	testutil.ApplyMigrationFile(t, db, migration)
	testutil.ApplyMigrationFile(t, db, migration)

	if !tableExists(t, db, "ip_visit_history") {
		t.Fatal("expected ip_visit_history table to exist")
	}

	dataType, nullable := testutil.ColumnMetadata(t, db, "ip_visit_history", "visited_at")
	if dataType != "timestamp with time zone" || nullable {
		t.Fatalf("ip_visit_history.visited_at = (%s, nullable=%v), want timestamptz NOT NULL", dataType, nullable)
	}

	pkColumns := primaryKeyColumns(t, db, "ip_visit_history")
	if len(pkColumns) != 2 || pkColumns[0] != "user_id" || pkColumns[1] != "ip_id" {
		t.Fatalf("ip_visit_history primary key columns = %v, want [user_id ip_id]", pkColumns)
	}

	if !testutil.ForeignKeyExists(t, db, "ip_visit_history", "user_id", "users") {
		t.Fatal("expected user_id foreign key to reference users")
	}
	if deleteRule(t, db, "ip_visit_history", "user_id") != "cascade" {
		t.Fatal("user_id foreign key must be ON DELETE CASCADE")
	}
	if !testutil.ForeignKeyExists(t, db, "ip_visit_history", "ip_id", "ips") {
		t.Fatal("expected ip_id foreign key to reference ips")
	}
	if deleteRule(t, db, "ip_visit_history", "ip_id") != "cascade" {
		t.Fatal("ip_id foreign key must be ON DELETE CASCADE")
	}

	if !testutil.IndexExists(t, db, "ip_visit_history", "idx_ip_visit_history_user_recent") {
		t.Fatal("expected idx_ip_visit_history_user_recent to exist")
	}
	indexDef := indexDefinition(t, db, "idx_ip_visit_history_user_recent")
	for _, want := range []string{"user_id", "visited_at", "DESC", "ip_id"} {
		if !strings.Contains(indexDef, want) {
			t.Fatalf("idx_ip_visit_history_user_recent definition %q must order by user_id, visited_at DESC, ip_id DESC (missing %q)", indexDef, want)
		}
	}

	userID := seedIPVisitHistoryUser(t, db)
	ipID := seedIPVisitHistoryIP(t, db)
	if err := db.Exec(
		`INSERT INTO ip_visit_history (user_id, ip_id, visited_at) VALUES (?, ?, NOW())`,
		userID, ipID,
	).Error; err != nil {
		t.Fatalf("seed ip_visit_history row: %v", err)
	}

	// Upsert keeps one row per (user_id, ip_id) pair.
	if err := db.Exec(
		`INSERT INTO ip_visit_history (user_id, ip_id, visited_at)
		 VALUES (?, ?, NOW() - INTERVAL '1 hour')
		 ON CONFLICT (user_id, ip_id)
		 DO UPDATE SET visited_at = GREATEST(ip_visit_history.visited_at, EXCLUDED.visited_at)`,
		userID, ipID,
	).Error; err != nil {
		t.Fatalf("upsert ip_visit_history row: %v", err)
	}
	var rows int64
	if err := db.Model(&IPVisitHistory{}).Where("user_id = ?", userID).Count(&rows).Error; err != nil {
		t.Fatalf("count ip_visit_history rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("ip_visit_history rows = %d, want 1 after upsert", rows)
	}

	// Deleting the user cascades history away.
	if err := db.Exec(`DELETE FROM users WHERE id = ?`, userID).Error; err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if err := db.Model(&IPVisitHistory{}).Where("ip_id = ?", ipID).Count(&rows).Error; err != nil {
		t.Fatalf("count ip_visit_history rows after user delete: %v", err)
	}
	if rows != 0 {
		t.Fatalf("ip_visit_history rows = %d after user delete, want 0 (cascade)", rows)
	}

	// Deleting the IP cascades history away as well.
	if err := db.Exec(
		`INSERT INTO ip_visit_history (user_id, ip_id, visited_at) VALUES (?, ?, NOW())`,
		seedIPVisitHistoryUser(t, db), ipID,
	).Error; err != nil {
		t.Fatalf("re-seed ip_visit_history row: %v", err)
	}
	if err := db.Exec(`DELETE FROM ips WHERE id = ?`, ipID).Error; err != nil {
		t.Fatalf("delete ip: %v", err)
	}
	if err := db.Model(&IPVisitHistory{}).Count(&rows).Error; err != nil {
		t.Fatalf("count ip_visit_history rows after ip delete: %v", err)
	}
	if rows != 0 {
		t.Fatalf("ip_visit_history rows = %d after ip delete, want 0 (cascade)", rows)
	}
}

func requireIPVisitHistoryBaseTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			username VARCHAR(64) UNIQUE NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error; err != nil {
		t.Fatalf("create users base table: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE ips (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			slug VARCHAR(255) UNIQUE NOT NULL,
			description TEXT,
			cover_url TEXT,
			category VARCHAR(50),
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error; err != nil {
		t.Fatalf("create ips base table: %v", err)
	}
}

func seedIPVisitHistoryUser(t *testing.T, db *gorm.DB) int64 {
	t.Helper()

	var id int64
	if err := db.Raw(`INSERT INTO users (email, username) VALUES ('a@example.com', 'alice') RETURNING id`).Scan(&id).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func seedIPVisitHistoryIP(t *testing.T, db *gorm.DB) int64 {
	t.Helper()

	var id int64
	if err := db.Raw(`INSERT INTO ips (name, slug, status) VALUES ('Stardust', 'stardust', 'approved') RETURNING id`).Scan(&id).Error; err != nil {
		t.Fatalf("seed ip: %v", err)
	}
	return id
}

func primaryKeyColumns(t *testing.T, db *gorm.DB, table string) []string {
	t.Helper()

	var columns []string
	if err := db.Raw(`
		SELECT a.attname
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
		JOIN unnest(c.conkey) WITH ORDINALITY AS u(attnum, ord) ON u.attnum = a.attnum
		WHERE t.relname = ?
		  AND c.contype = 'p'
		ORDER BY u.ord
	`, table).Scan(&columns).Error; err != nil {
		t.Fatalf("lookup primary key columns of %s: %v", table, err)
	}
	return columns
}

func deleteRule(t *testing.T, db *gorm.DB, table, column string) string {
	t.Helper()

	var rule string
	if err := db.Raw(`
		SELECT CASE c.confdeltype
			WHEN 'c' THEN 'cascade'
			WHEN 'a' THEN 'no action'
			WHEN 'r' THEN 'restrict'
			WHEN 'n' THEN 'set null'
			WHEN 'd' THEN 'set default'
			ELSE 'unknown'
		END
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
		WHERE t.relname = ?
		  AND c.contype = 'f'
		  AND a.attname = ?
	`, table, column).Scan(&rule).Error; err != nil {
		t.Fatalf("lookup delete rule of %s.%s: %v", table, column, err)
	}
	if rule == "" {
		t.Fatalf("no foreign key found on %s.%s", table, column)
	}
	return rule
}
