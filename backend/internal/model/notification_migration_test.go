package model

import (
	"path/filepath"
	"testing"

	"omnicraft/backend/internal/testutil"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestNotificationBroadcastChannelMigrationAllowsBroadcast(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	requireNotificationsBaseTable(t, db)

	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "057_add_broadcast_channel.sql"))

	if err := db.Exec(`
		INSERT INTO notifications (user_id, type, channel, body)
		VALUES (1, 'system', 'broadcast', 'system maintenance')
	`).Error; err != nil {
		t.Fatalf("insert broadcast notification after migration: %v", err)
	}

	for _, channel := range []string{"reply", "like", "system", "pr", "follow"} {
		if err := db.Exec(`
			INSERT INTO notifications (user_id, type, channel, body)
			VALUES (1, 'system', ?, 'existing channel')
		`, channel).Error; err != nil {
			t.Fatalf("insert preserved channel %q after migration: %v", channel, err)
		}
	}

	if err := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Exec(`
		INSERT INTO notifications (user_id, type, channel, body)
		VALUES (1, 'system', 'email', 'invalid channel')
	`).Error; err == nil {
		t.Fatal("expected invalid notification channel to be rejected after migration")
	}
}

func TestNotificationBroadcastIdempotencyMigration(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	requireNotificationBroadcastRequestBaseUsers(t, db)

	migration := filepath.Join("..", "..", "migrations", "062_notification_broadcast_idempotency.sql")
	testutil.ApplyMigrationFile(t, db, migration)
	testutil.ApplyMigrationFile(t, db, migration)

	for _, column := range []string{"key_hash", "payload_hash", "recipient_count", "broadcast_at"} {
		if dataType, _ := testutil.ColumnMetadata(t, db, "notification_broadcast_requests", column); dataType == "" {
			t.Fatalf("expected notification_broadcast_requests.%s", column)
		}
	}
	if !testutil.IndexExists(t, db, "notification_broadcast_requests", "uq_notification_broadcast_requests_actor_key") {
		t.Fatal("expected actor/key unique index")
	}
	if !testutil.ForeignKeyExists(t, db, "notification_broadcast_requests", "actor_id", "users") {
		t.Fatal("expected actor_id foreign key")
	}

	if err := db.Exec(`
		INSERT INTO notification_broadcast_requests
			(actor_id, key_hash, payload_hash, recipient_count, broadcast_at)
		VALUES (1, repeat('a', 64), repeat('b', 64), 2, NOW())
	`).Error; err != nil {
		t.Fatalf("insert idempotency row: %v", err)
	}
	if err := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Exec(`
		INSERT INTO notification_broadcast_requests
			(actor_id, key_hash, payload_hash, recipient_count, broadcast_at)
		VALUES (1, repeat('a', 64), repeat('c', 64), 3, NOW())
	`).Error; err == nil {
		t.Fatal("expected duplicate actor/key to be rejected")
	}
	if err := db.Exec(`
		INSERT INTO notification_broadcast_requests
			(actor_id, key_hash, payload_hash, recipient_count, broadcast_at)
		VALUES (2, repeat('a', 64), repeat('c', 64), 3, NOW())
	`).Error; err != nil {
		t.Fatalf("same key hash for another actor should be allowed: %v", err)
	}
}

func requireNotificationsBaseTable(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec(`
		CREATE TABLE notifications (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			type VARCHAR(50) NOT NULL,
			channel VARCHAR(20) NOT NULL,
			title VARCHAR(500),
			body TEXT,
			target_type VARCHAR(50),
			target_id BIGINT,
			sender_id BIGINT,
			is_read BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT notifications_channel_check
				CHECK (channel IN ('reply', 'like', 'system', 'pr', 'follow'))
		)
	`).Error; err != nil {
		t.Fatalf("create notifications base table: %v", err)
	}
}

func requireNotificationBroadcastRequestBaseUsers(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`
		CREATE TABLE users (
			id BIGINT PRIMARY KEY
		);
		INSERT INTO users (id) VALUES (1), (2);
	`).Error; err != nil {
		t.Fatalf("create notification broadcast request base users: %v", err)
	}
}
