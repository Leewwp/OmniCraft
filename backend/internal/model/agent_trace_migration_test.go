package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"omnicraft/backend/internal/testutil"
)

// TestAgentTraceMigration verifies 081_agent_trace.sql: idempotent apply,
// contract comments present, unique upsert keys enforced for the async
// batch writer (run start -> terminal update, node RUNNING -> terminal).
func TestAgentTraceMigration(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)

	migration := filepath.Join("..", "..", "migrations", "081_agent_trace.sql")
	migrationSQL, err := os.ReadFile(migration)
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		"-- One row per trace: RUNNING start and terminal update upsert on this key.\n    CONSTRAINT uq_agent_trace_runs_trace_id UNIQUE (trace_id)",
		"-- Terminal states only; RUNNING marks an in-flight or crashed turn.\n    CONSTRAINT agent_trace_runs_status_check",
		"-- Supports the admin request list default ordering (newest first).\nCREATE INDEX IF NOT EXISTS idx_agent_trace_runs_started",
		"-- Supports conversation -> trace drill-down from the chat history.\nCREATE INDEX IF NOT EXISTS idx_agent_trace_runs_conversation",
		"-- Supports per-user filtering on the admin request list.\nCREATE INDEX IF NOT EXISTS idx_agent_trace_runs_user_started",
		"-- Stable per-trace node identity: RUNNING insert + terminal update is one idempotent upsert.\n    CONSTRAINT uq_agent_trace_nodes_key UNIQUE (trace_id, node_key)",
		"-- Supports the trace detail waterfall: all nodes of one trace in start order.\nCREATE INDEX IF NOT EXISTS idx_agent_trace_nodes_trace",
	} {
		if !strings.Contains(string(migrationSQL), contract) {
			t.Fatalf("migration contract missing %q", contract)
		}
	}
	testutil.ApplyMigrationFile(t, db, migration)
	testutil.ApplyMigrationFile(t, db, migration)

	for _, table := range []string{"agent_trace_runs", "agent_trace_nodes"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing table %s", table)
		}
	}
	if !db.Migrator().HasIndex("agent_trace_runs", "idx_agent_trace_runs_user_started") {
		t.Fatal("missing agent_trace_runs(user_id, started_at DESC) index")
	}

	// The writer upserts run rows on trace_id: a second insert with the same
	// trace_id must be rejected so the ON CONFLICT path stays authoritative.
	requireExec(t, db, `INSERT INTO agent_trace_runs (trace_id, status, started_at) VALUES ('a1b2c3', 'RUNNING', NOW())`)
	if err := db.Exec(`INSERT INTO agent_trace_runs (trace_id, status, started_at) VALUES ('a1b2c3', 'SUCCESS', NOW())`).Error; err == nil {
		t.Fatal("duplicate agent_trace_runs.trace_id must be rejected")
	}
	requireExec(t, db, `INSERT INTO agent_trace_nodes (trace_id, node_key, node_type, status, started_at)
		VALUES ('a1b2c3', 'llm_round_1', 'llm', 'RUNNING', NOW())`)
	if err := db.Exec(`INSERT INTO agent_trace_nodes (trace_id, node_key, node_type, status, started_at)
		VALUES ('a1b2c3', 'llm_round_1', 'llm', 'SUCCESS', NOW())`).Error; err == nil {
		t.Fatal("duplicate (trace_id, node_key) must be rejected")
	}
	// Same node_key under a different trace is a different node.
	requireExec(t, db, `INSERT INTO agent_trace_nodes (trace_id, node_key, node_type, status, started_at)
		VALUES ('d4e5f6', 'llm_round_1', 'llm', 'RUNNING', NOW())`)

	// Status check constraint keeps the state machine closed.
	if err := db.Exec(`INSERT INTO agent_trace_runs (trace_id, status, started_at) VALUES ('badstatus', 'PAUSED', NOW())`).Error; err == nil {
		t.Fatal("unknown agent_trace_runs.status must be rejected")
	}
	if err := db.Exec(`INSERT INTO agent_trace_nodes (trace_id, node_key, node_type, status, started_at)
		VALUES ('a1b2c3', 'n2', 'tool', 'FAILED', NOW())`).Error; err == nil {
		t.Fatal("unknown agent_trace_nodes.status must be rejected")
	}

	// Nullable link columns and jsonb defaults round-trip.
	requireExec(t, db, `INSERT INTO agent_trace_runs (trace_id, status, started_at, routing_events, extra)
		VALUES ('nullable-links', 'RUNNING', NOW(), '[{"from":"minimax-m3","to":"deepseek-chat"}]'::jsonb, '{}'::jsonb)`)
	var routing string
	requireExecScan(t, db, `SELECT routing_events::text FROM agent_trace_runs WHERE trace_id = 'nullable-links'`, &routing)
	if !strings.Contains(routing, "deepseek-chat") {
		t.Fatalf("routing_events round-trip lost payload: %s", routing)
	}
}
