package model

import (
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"

	"omnicraft/backend/internal/testutil"
)

// TestAgentAccessTokenMigration covers the empty-database upgrade path for
// 079 (SP-16 #450, spec D2): the agent_access_tokens table backing the PAT
// machine identity. Idempotency, column shapes, the scopes CHECK, the
// token_hash uniqueness and the per-user partial index over live tokens are
// all asserted here.
func TestAgentAccessTokenMigration(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	requireAgentTokenBaseTables(t, db)

	migration := filepath.Join("..", "..", "migrations", "079_agent_access_tokens.sql")
	testutil.ApplyMigrationFile(t, db, migration)
	// idempotent re-apply must succeed (runner reconciliation path)
	testutil.ApplyMigrationFile(t, db, migration)

	if !tableExists(t, db, "agent_access_tokens") {
		t.Fatal("expected agent_access_tokens table to exist")
	}

	for _, tc := range []struct {
		column   string
		dataType string
		nullable bool
	}{
		{"id", "bigint", false},
		{"user_id", "bigint", false},
		{"name", "character varying", false},
		{"token_hash", "character varying", false},
		{"token_prefix", "character varying", false},
		{"scopes", "character varying", false},
		{"last_used_at", "timestamp with time zone", true},
		{"revoked_at", "timestamp with time zone", true},
		{"created_at", "timestamp with time zone", false},
	} {
		dataType, nullable := testutil.ColumnMetadata(t, db, "agent_access_tokens", tc.column)
		if dataType != tc.dataType || nullable != tc.nullable {
			t.Fatalf("agent_access_tokens.%s = (%s, nullable=%v), want (%s, nullable=%v)",
				tc.column, dataType, nullable, tc.dataType, tc.nullable)
		}
	}

	if !testutil.ForeignKeyExists(t, db, "agent_access_tokens", "user_id", "users") {
		t.Fatal("expected user_id foreign key to reference users")
	}

	def := checkConstraintDefinition(t, db, "agent_access_tokens", "scopes")
	for _, allowed := range []string{"download", "upload"} {
		if !strings.Contains(def, allowed) {
			t.Fatalf("scopes CHECK %q must allow %q", def, allowed)
		}
	}

	var userID int64
	if err := db.Raw(`INSERT INTO users (email, username) VALUES ('pat-owner@example.com', 'pat_owner') RETURNING id`).Scan(&userID).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// valid scope combinations must insert
	for _, scopes := range []string{"download", "upload", "download,upload"} {
		if err := db.Exec(`
			INSERT INTO agent_access_tokens (user_id, name, token_hash, token_prefix, scopes)
			VALUES (?, ?, ?, ?, ?)
		`, userID, "tok-"+scopes, "hash-"+scopes, "oc_pat_ex", scopes).Error; err != nil {
			t.Fatalf("insert with scopes=%q must succeed: %v", scopes, err)
		}
	}

	// an unknown scope must be rejected by the CHECK
	if err := db.Exec(`
		INSERT INTO agent_access_tokens (user_id, name, token_hash, token_prefix, scopes)
		VALUES (?, 'bad', 'hash-bad', 'oc_pat_ex', 'read')
	`, userID).Error; err == nil {
		t.Fatal("scopes CHECK must reject unsupported scope 'read'")
	}

	// token_hash is unique: a duplicate hash must fail
	if err := db.Exec(`
		INSERT INTO agent_access_tokens (user_id, name, token_hash, token_prefix, scopes)
		VALUES (?, 'dup', 'hash-download', 'oc_pat_ex', 'download')
	`, userID).Error; err == nil {
		t.Fatal("token_hash must be unique")
	}

	// partial index over live (non-revoked) tokens per user
	if !testutil.IndexExists(t, db, "agent_access_tokens", "idx_agent_access_tokens_user_active") {
		t.Fatal("expected idx_agent_access_tokens_user_active index")
	}
	idxDef := indexDefinition(t, db, "idx_agent_access_tokens_user_active")
	if !strings.Contains(idxDef, "revoked_at IS NULL") {
		t.Fatalf("user_active index %q must be partial on revoked_at IS NULL", idxDef)
	}
}

func requireAgentTokenBaseTables(t *testing.T, db *gorm.DB) {
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
}
