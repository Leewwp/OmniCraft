package model

import (
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"

	"omnicraft/backend/internal/testutil"
)

// TestAgentConversationsGuestOwnerMigration covers the empty-database upgrade
// path for 089 (#854): guest conversations are owned by a signed device key,
// never by a shared sentinel user id and never by a pseudo user row. The
// CHECK constraint makes the two ownership shapes mutually exclusive and the
// partial index keeps per-device listings cheap.
func TestAgentConversationsGuestOwnerMigration(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)

	for _, name := range []string{
		"001_users.sql",
		"020_agent_conversations.sql",
		"021_agent_messages.sql",
	} {
		testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", name))
	}

	migration := filepath.Join("..", "..", "migrations", "089_agent_conversations_guest_owner.sql")
	testutil.ApplyMigrationFile(t, db, migration)
	// idempotent re-apply must succeed (runner reconciliation path)
	testutil.ApplyMigrationFile(t, db, migration)

	// user_id dropped NOT NULL; is_guest / guest_device_key added.
	for _, tc := range []struct {
		column   string
		nullable bool
	}{
		{"user_id", true},
		{"is_guest", false},
		{"guest_device_key", true},
	} {
		_, nullable := testutil.ColumnMetadata(t, db, "agent_conversations", tc.column)
		if nullable != tc.nullable {
			t.Fatalf("agent_conversations.%s nullable=%v, want %v", tc.column, nullable, tc.nullable)
		}
	}

	def := tableCheckConstraintDefinition(t, db, "agent_conversations", "agent_conversations_owner_check")
	for _, fragment := range []string{"is_guest", "guest_device_key", "user_id"} {
		if !strings.Contains(def, fragment) {
			t.Fatalf("owner CHECK %q must mention %q", def, fragment)
		}
	}

	if !testutil.IndexExists(t, db, "agent_conversations", "idx_agent_conversations_guest_device") {
		t.Fatal("expected the per-device guest listing index to exist")
	}

	// Existing-shape user row still inserts (is_guest defaults false).
	if err := db.Exec(`INSERT INTO users (username, email, password_hash, role) VALUES ('u1', 'u1@example.com', 'x', 'user')`).Error; err != nil {
		t.Fatalf("seed user row: %v", err)
	}
	if err := db.Exec(`INSERT INTO agent_conversations (user_id) VALUES ((SELECT id FROM users WHERE username='u1'))`).Error; err != nil {
		t.Fatalf("insert user-owned conversation: %v", err)
	}

	// Guest shape: user_id NULL + guest_device_key set.
	if err := db.Exec(`INSERT INTO agent_conversations (is_guest, guest_device_key) VALUES (TRUE, 'a1b2c3')`).Error; err != nil {
		t.Fatalf("insert guest-owned conversation: %v", err)
	}

	mustReject := func(name, stmt string) {
		t.Helper()
		if err := db.Exec(stmt).Error; err == nil {
			t.Fatalf("%s: expected rejection, got success", name)
		}
	}
	// Guest row carrying a user id: rejected (no shared ownership).
	mustReject("guest row with user_id",
		`INSERT INTO agent_conversations (is_guest, guest_device_key, user_id) VALUES (TRUE, 'zz', (SELECT id FROM users WHERE username='u1'))`)
	// Guest row without a device key: rejected (unownable).
	mustReject("guest row without device key",
		`INSERT INTO agent_conversations (is_guest) VALUES (TRUE)`)
	// User row carrying a device key: rejected (mixed ownership).
	mustReject("user row with device key",
		`INSERT INTO agent_conversations (user_id, guest_device_key) VALUES ((SELECT id FROM users WHERE username='u1'), 'yy')`)
	// Non-guest row without user_id: rejected.
	mustReject("user row without user_id",
		`INSERT INTO agent_conversations (is_guest) VALUES (FALSE)`)

	// Messages cascade with the guest conversation (existing FK semantics).
	var convID int64
	if err := db.Raw(`SELECT id FROM agent_conversations WHERE is_guest`).Scan(&convID).Error; err != nil || convID == 0 {
		t.Fatalf("read guest conversation id: %v (%d)", err, convID)
	}
	if err := db.Exec(`INSERT INTO agent_messages (conversation_id, role, content) VALUES (?, 'user', 'hi')`, convID).Error; err != nil {
		t.Fatalf("seed guest message: %v", err)
	}
	if err := db.Exec(`DELETE FROM agent_conversations WHERE id = ?`, convID).Error; err != nil {
		t.Fatalf("delete guest conversation: %v", err)
	}
	var count int64
	db.Raw(`SELECT count(*) FROM agent_messages WHERE conversation_id = ?`, convID).Scan(&count)
	if count != 0 {
		t.Fatalf("guest messages must cascade on conversation delete, got %d", count)
	}
}

// tableCheckConstraintDefinition reads a table-level CHECK constraint by
// name (the package helper keys on a single column, which a multi-column
// table constraint does not hit).
func tableCheckConstraintDefinition(t *testing.T, db interface {
	Raw(string, ...interface{}) *gorm.DB
}, table, constraint string) string {
	t.Helper()
	var def string
	if err := db.Raw(
		`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid = ?::regclass AND conname = ?`,
		table, constraint,
	).Scan(&def).Error; err != nil {
		t.Fatalf("read constraint %s: %v", constraint, err)
	}
	return def
}
