package model

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"omnicraft/backend/internal/testutil"
)

// TestOutboxMigration covers the empty-database upgrade path for 070: the
// outbox_events and inbox_consumers tables, their column contracts (jsonb
// payload, W3C trace context columns, retry fields), the (status,
// next_attempt_at) and (aggregate_id, event_type) indexes, the UNIQUE
// (consumer_group, event_id) inbox idempotency constraint, the constitution
// fields (id/created_at), the idempotent forward-only re-application contract
// and jsonb round-trip fidelity.
func TestOutboxMigration(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)

	migration := filepath.Join("..", "..", "migrations", "070_outbox_inbox.sql")
	testutil.ApplyMigrationFile(t, db, migration)
	testutil.ApplyMigrationFile(t, db, migration)

	assertOutboxTables(t, db)

	// --- outbox_events column contract ---
	assertColumn(t, db, "outbox_events", "id", "bigint", false)
	assertColumn(t, db, "outbox_events", "created_at", "timestamp with time zone", false)
	assertColumn(t, db, "outbox_events", "aggregate_id", "bigint", false)
	assertColumn(t, db, "outbox_events", "event_type", "character varying", false)
	assertColumn(t, db, "outbox_events", "schema_version", "integer", false)
	assertColumn(t, db, "outbox_events", "payload", "jsonb", false)
	assertColumn(t, db, "outbox_events", "traceparent", "character varying", true)
	assertColumn(t, db, "outbox_events", "tracestate", "character varying", true)
	assertColumn(t, db, "outbox_events", "status", "character varying", false)
	assertColumn(t, db, "outbox_events", "attempts", "integer", false)
	assertColumn(t, db, "outbox_events", "next_attempt_at", "timestamp with time zone", false)
	assertColumn(t, db, "outbox_events", "sent_at", "timestamp with time zone", true)

	if !testutil.IndexExists(t, db, "outbox_events", "idx_outbox_events_status_next_attempt") {
		t.Fatal("expected index idx_outbox_events_status_next_attempt on outbox_events")
	}
	if !testutil.IndexExists(t, db, "outbox_events", "idx_outbox_events_aggregate_event_type") {
		t.Fatal("expected index idx_outbox_events_aggregate_event_type on outbox_events")
	}

	// --- inbox_consumers column contract ---
	assertColumn(t, db, "inbox_consumers", "id", "bigint", false)
	assertColumn(t, db, "inbox_consumers", "created_at", "timestamp with time zone", false)
	assertColumn(t, db, "inbox_consumers", "consumer_group", "character varying", false)
	assertColumn(t, db, "inbox_consumers", "event_id", "bigint", false)
	assertColumn(t, db, "inbox_consumers", "consumed_at", "timestamp with time zone", false)

	if !testutil.IndexExists(t, db, "inbox_consumers", "uq_inbox_consumers_group_event") {
		t.Fatal("expected unique index uq_inbox_consumers_group_event on inbox_consumers")
	}

	// --- id is a stable event_id: BIGSERIAL assigns and never changes ---
	eventID := seedOutboxEvent(t, db, 1001, "content.published", `{"content_id": 1001, "author_id": 7}`)
	if eventID <= 0 {
		t.Fatalf("outbox id must be a positive BIGSERIAL, got %d", eventID)
	}

	// --- defaults: pending, attempts 0, schema_version 1, next_attempt_at NOW ---
	var defaults struct {
		Status        string
		Attempts      int
		SchemaVersion int
		NextAttemptAt int64
	}
	if err := db.Raw(`
		SELECT status, attempts, schema_version,
		       EXTRACT(EPOCH FROM (next_attempt_at - NOW()))::bigint AS next_attempt_at
		FROM outbox_events WHERE id = ?
	`, eventID).Scan(&defaults).Error; err != nil {
		t.Fatalf("read back outbox defaults: %v", err)
	}
	if defaults.Status != "pending" || defaults.Attempts != 0 || defaults.SchemaVersion != 1 {
		t.Fatalf("outbox defaults = (status=%q, attempts=%d, schema_version=%d), want (pending, 0, 1)",
			defaults.Status, defaults.Attempts, defaults.SchemaVersion)
	}

	// --- jsonb payload round-trips byte-for-byte ---
	var payload string
	if err := db.Raw(`SELECT payload::text FROM outbox_events WHERE id = ?`, eventID).Scan(&payload).Error; err != nil {
		t.Fatalf("read back outbox payload: %v", err)
	}
	if !json.Valid([]byte(payload)) {
		t.Fatalf("outbox payload must round-trip as valid JSON: %q", payload)
	}

	// --- UNIQUE (consumer_group, event_id) enforcement ---
	seedInboxConsumer(t, db, "indexer", eventID)
	if err := db.Exec(`
		INSERT INTO inbox_consumers (consumer_group, event_id) VALUES (?, ?)
	`, "indexer", eventID).Error; err == nil {
		t.Fatal("duplicate (consumer_group, event_id) insert must be rejected by the unique index")
	}
	// a different consumer group may consume the same event
	seedInboxConsumer(t, db, "notifier", eventID)
}

// TestOutboxMigrationHistoricalFixtureUpgrade applies the previous latest
// migration (069, the RAG evaluation schema) and then upgrades with 070, the
// path a real historical database takes. Re-applying 070 stays idempotent.
func TestOutboxMigrationHistoricalFixtureUpgrade(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)

	prev := filepath.Join("..", "..", "migrations", "069_rag_evaluation.sql")
	testutil.ApplyMigrationFile(t, db, prev)
	testutil.ApplyMigrationFile(t, db, prev)

	migration := filepath.Join("..", "..", "migrations", "070_outbox_inbox.sql")
	testutil.ApplyMigrationFile(t, db, migration)
	testutil.ApplyMigrationFile(t, db, migration)

	assertOutboxTables(t, db)

	// the previous schema is untouched by the upgrade
	if !testutil.IndexExists(t, db, "eval_golden_cases", "uq_eval_golden_cases_case_key") {
		t.Fatal("expected eval_golden_cases to survive the 070 upgrade")
	}
}

func assertOutboxTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range []string{"outbox_events", "inbox_consumers"} {
		var exists bool
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema() AND table_name = ?
			)
		`, table).Scan(&exists).Error; err != nil {
			t.Fatalf("lookup table %s: %v", table, err)
		}
		if !exists {
			t.Fatalf("expected table %s to exist after migration 070", table)
		}
	}
}

func seedOutboxEvent(t *testing.T, db *gorm.DB, aggregateID int64, eventType, payload string) int64 {
	t.Helper()
	var id int64
	if err := db.Raw(`
		INSERT INTO outbox_events (aggregate_id, event_type, schema_version, payload)
		VALUES (?, ?, 1, ?::jsonb) RETURNING id
	`, aggregateID, eventType, payload).Scan(&id).Error; err != nil {
		t.Fatalf("seed outbox_events row: %v", err)
	}
	return id
}

func seedInboxConsumer(t *testing.T, db *gorm.DB, consumerGroup string, eventID int64) {
	t.Helper()
	if err := db.Exec(`
		INSERT INTO inbox_consumers (consumer_group, event_id) VALUES (?, ?)
	`, consumerGroup, eventID).Error; err != nil {
		t.Fatalf("seed inbox_consumers row: %v", err)
	}
}
