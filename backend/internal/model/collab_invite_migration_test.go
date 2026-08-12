package model

import (
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"

	"omnicraft/backend/internal/testutil"
)

// TestCollaborationInviteMigration covers the empty-database upgrade path for
// 065: the collaboration_invites table, the users.accept_collab_invites opt-in
// column, the messages msg_type CHECK / metadata JSONB columns, and the active
// partial unique index that allows re-inviting after an invite expires.
func TestCollaborationInviteMigration(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	requireCollaborationBaseTables(t, db)

	migration := filepath.Join("..", "..", "migrations", "065_collaboration_invites.sql")
	testutil.ApplyMigrationFile(t, db, migration)
	testutil.ApplyMigrationFile(t, db, migration)

	if !tableExists(t, db, "collaboration_invites") {
		t.Fatal("expected collaboration_invites table to exist")
	}

	dataType, nullable := testutil.ColumnMetadata(t, db, "collaboration_invites", "message_id")
	if dataType != "bigint" || !nullable {
		t.Fatalf("collaboration_invites.message_id = (%s, nullable=%v), want bigint nullable", dataType, nullable)
	}
	if !testutil.ForeignKeyExists(t, db, "collaboration_invites", "message_id", "messages") {
		t.Fatal("expected message_id foreign key to reference messages")
	}

	dataType, nullable = testutil.ColumnMetadata(t, db, "users", "accept_collab_invites")
	if dataType != "boolean" || nullable {
		t.Fatalf("users.accept_collab_invites = (%s, nullable=%v), want boolean NOT NULL", dataType, nullable)
	}

	var userID int64
	if err := db.Raw(`INSERT INTO users (email, username) VALUES ('a@example.com', 'alice') RETURNING id`).Scan(&userID).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	var accepts bool
	if err := db.Raw(`SELECT accept_collab_invites FROM users WHERE id = ?`, userID).Scan(&accepts).Error; err != nil {
		t.Fatalf("read accept_collab_invites default: %v", err)
	}
	if !accepts {
		t.Fatal("users.accept_collab_invites must default to TRUE")
	}

	dataType, nullable = testutil.ColumnMetadata(t, db, "messages", "msg_type")
	if dataType != "character varying" || nullable {
		t.Fatalf("messages.msg_type = (%s, nullable=%v), want varchar NOT NULL", dataType, nullable)
	}
	def := checkConstraintDefinition(t, db, "messages", "msg_type")
	if !strings.Contains(def, "collab_invite") {
		t.Fatalf("messages.msg_type CHECK %q must allow 'collab_invite'", def)
	}

	statusDef := checkConstraintDefinition(t, db, "collaboration_invites", "status")
	for _, want := range []string{"pending", "accepted", "declined", "expired"} {
		if !strings.Contains(statusDef, want) {
			t.Fatalf("collaboration_invites.status CHECK %q must allow '%s'", statusDef, want)
		}
	}
	if strings.Contains(statusDef, "rejected") {
		t.Fatalf("collaboration_invites.status CHECK %q must not allow 'rejected'", statusDef)
	}

	dataType, nullable = testutil.ColumnMetadata(t, db, "messages", "metadata")
	if dataType != "jsonb" || nullable {
		t.Fatalf("messages.metadata = (%s, nullable=%v), want jsonb NOT NULL", dataType, nullable)
	}

	if !testutil.IndexExists(t, db, "collaboration_invites", "idx_collab_invites_inviter") {
		t.Fatal("expected idx_collab_invites_inviter to exist")
	}
	if !testutil.IndexExists(t, db, "collaboration_invites", "idx_collab_invites_active") {
		t.Fatal("expected idx_collab_invites_active to exist")
	}
	if !indexIsUnique(t, db, "idx_collab_invites_active") {
		t.Fatal("idx_collab_invites_active must be a UNIQUE index")
	}
	indexDef := indexDefinition(t, db, "idx_collab_invites_active")
	for _, want := range []string{"content_id", "invitee_id", "status", "pending", "accepted"} {
		if !strings.Contains(indexDef, want) {
			t.Fatalf("idx_collab_invites_active definition %q must be partial on status IN ('pending','accepted') (missing %q)", indexDef, want)
		}
	}

	var convID int64
	if err := db.Raw(`INSERT INTO conversations DEFAULT VALUES RETURNING id`).Scan(&convID).Error; err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	var msgID int64
	if err := db.Raw(`
		INSERT INTO messages (conversation_id, sender_id, body)
		VALUES (?, ?, 'hello')
		RETURNING id
	`, convID, userID).Scan(&msgID).Error; err != nil {
		t.Fatalf("seed message: %v", err)
	}
	var meta string
	if err := db.Raw(`SELECT metadata::text FROM messages WHERE id = ?`, msgID).Scan(&meta).Error; err != nil {
		t.Fatalf("read messages.metadata default: %v", err)
	}
	if meta != "{}" {
		t.Fatalf("messages.metadata must default to '{}', got %q", meta)
	}
}

func requireCollaborationBaseTables(t *testing.T, db *gorm.DB) {
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
		CREATE TABLE content_items (
			id BIGSERIAL PRIMARY KEY,
			title TEXT NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error; err != nil {
		t.Fatalf("create content_items base table: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE conversations (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error; err != nil {
		t.Fatalf("create conversations base table: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE messages (
			id BIGSERIAL PRIMARY KEY,
			conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			sender_id BIGINT NOT NULL REFERENCES users(id),
			body TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error; err != nil {
		t.Fatalf("create messages base table: %v", err)
	}
}

func indexIsUnique(t *testing.T, db *gorm.DB, index string) bool {
	t.Helper()

	var unique bool
	if err := db.Raw(`
		SELECT i.indisunique
		FROM pg_index i
		JOIN pg_class c ON c.oid = i.indexrelid
		WHERE c.relname = ?
	`, index).Scan(&unique).Error; err != nil {
		t.Fatalf("lookup unique flag of %s: %v", index, err)
	}
	return unique
}

func checkConstraintDefinition(t *testing.T, db *gorm.DB, table, column string) string {
	t.Helper()

	var definition string
	if err := db.Raw(`
		SELECT pg_get_constraintdef(c.oid)
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
		WHERE t.relname = ?
		  AND c.contype = 'c'
		  AND a.attname = ?
	`, table, column).Scan(&definition).Error; err != nil {
		t.Fatalf("lookup check constraint on %s.%s: %v", table, column, err)
	}
	if definition == "" {
		t.Fatalf("no CHECK constraint found on %s.%s", table, column)
	}
	return definition
}
