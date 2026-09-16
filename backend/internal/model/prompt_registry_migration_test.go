package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"omnicraft/backend/internal/testutil"
)

// TestPromptRegistryMigration verifies 082_prompt_registry.sql: idempotent
// apply, contract comments present, immutable (name, version) uniqueness and
// one label pointer per (name, label).
func TestPromptRegistryMigration(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)

	migration := filepath.Join("..", "..", "migrations", "082_prompt_registry.sql")
	migrationSQL, err := os.ReadFile(migration)
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		"-- Versions are immutable: one row per (name, version), insert-only.\n    CONSTRAINT uq_prompt_registry_name_version UNIQUE (name, version)",
		"-- Supports version history listings for the admin prompt page.\nCREATE INDEX IF NOT EXISTS idx_prompt_registry_name_version",
		"-- One pointer per (name, label): moving a label is an update, not a new row.\n    CONSTRAINT uq_prompt_labels_name_label UNIQUE (name, label)",
		"-- Only the two known deployment pointers may exist.\n    CONSTRAINT prompt_labels_label_check",
		"-- Supports runtime production-pointer lookups.\nCREATE INDEX IF NOT EXISTS idx_prompt_labels_name_label",
	} {
		if !strings.Contains(string(migrationSQL), contract) {
			t.Fatalf("migration contract missing %q", contract)
		}
	}
	testutil.ApplyMigrationFile(t, db, migration)
	testutil.ApplyMigrationFile(t, db, migration)

	for _, table := range []string{"prompt_registry", "prompt_labels"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing table %s", table)
		}
	}

	requireExec(t, db, `INSERT INTO prompt_registry (name, version, content, required_placeholders)
		VALUES ('agent_system', 1, '[OmniCraft Agent Context] {{surface_context}}', '["surface_context"]'::jsonb)`)
	if err := db.Exec(`INSERT INTO prompt_registry (name, version, content)
		VALUES ('agent_system', 1, 'duplicate')`).Error; err == nil {
		t.Fatal("duplicate (name, version) must be rejected")
	}
	// A new version of the same name is the supported edit path.
	requireExec(t, db, `INSERT INTO prompt_registry (name, version, content)
		VALUES ('agent_system', 2, 'rewritten')`)

	requireExec(t, db, `INSERT INTO prompt_labels (name, label, version) VALUES ('agent_system', 'production', 2)`)
	if err := db.Exec(`INSERT INTO prompt_labels (name, label, version) VALUES ('agent_system', 'production', 1)`).Error; err == nil {
		t.Fatal("second pointer for the same (name, label) must be rejected (moves are updates)")
	}
	requireExec(t, db, `INSERT INTO prompt_labels (name, label, version) VALUES ('agent_system', 'staging', 1)`)
	if err := db.Exec(`INSERT INTO prompt_labels (name, label, version) VALUES ('agent_system', 'canary', 1)`).Error; err == nil {
		t.Fatal("unknown label must be rejected by the check constraint")
	}
}
