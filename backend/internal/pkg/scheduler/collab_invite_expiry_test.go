package scheduler

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/testutil"
)

// collabInviteExpiryFixedNow is 2026-08-11 10:00 Asia/Shanghai; injected as
// the scheduler clock so created_at cutoffs are deterministic.
var collabInviteExpiryFixedNow = time.Date(2026, 8, 11, 10, 0, 0, 0, testShanghaiLocation())

func testShanghaiLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}

func TestCollabInviteExpiryMarksPendingOlderThanConfiguredDays(t *testing.T) {
	db := setupCollabInviteExpiryDB(t)
	now := collabInviteExpiryFixedNow
	expiry := NewCollabInviteExpiry(db, &config.CollaborationConfig{InviteExpireDays: 7})
	expiry.now = func() time.Time { return now }

	expired, err := expiry.runOnce()
	if err != nil {
		t.Fatalf("runOnce() error = %v", err)
	}
	if expired != 1 {
		t.Fatalf("expired = %d, want 1", expired)
	}

	assertCollabInviteExpiryStatus(t, db, 1, "expired")
	assertCollabInviteExpiryStatus(t, db, 2, "pending")
	assertCollabInviteExpiryStatus(t, db, 3, "pending")
	assertCollabInviteExpiryStatus(t, db, 4, "accepted")
	assertCollabInviteExpiryStatus(t, db, 5, "declined")
}

func TestCollabInviteExpiryUsesConfiguredDuration(t *testing.T) {
	db := setupCollabInviteExpiryDB(t)
	now := collabInviteExpiryFixedNow
	require.NoError(t, db.Exec("UPDATE collaboration_invites SET created_at = ? WHERE id = 1", now.AddDate(0, 0, -4)).Error)
	require.NoError(t, db.Exec("UPDATE collaboration_invites SET created_at = ? WHERE id = 2", now.AddDate(0, 0, -3)).Error)
	require.NoError(t, db.Exec("UPDATE collaboration_invites SET created_at = ? WHERE id = 3", now.AddDate(0, 0, -2)).Error)

	expiry := NewCollabInviteExpiry(db, &config.CollaborationConfig{InviteExpireDays: 3})
	expiry.now = func() time.Time { return now }

	expired, err := expiry.runOnce()
	if err != nil {
		t.Fatalf("runOnce() error = %v", err)
	}
	if expired != 1 {
		t.Fatalf("expired = %d, want 1", expired)
	}
	assertCollabInviteExpiryStatus(t, db, 1, "expired")
	assertCollabInviteExpiryStatus(t, db, 2, "pending")
	assertCollabInviteExpiryStatus(t, db, 3, "pending")
}

func TestCollabInviteExpiryStopCancelsPendingCallback(t *testing.T) {
	expiry := NewCollabInviteExpiry(nil, &config.CollaborationConfig{InviteExpireDays: 7})
	called := make(chan struct{}, 1)
	expiry.scheduleAfter(10*time.Millisecond, func() {
		called <- struct{}{}
	})

	expiry.Stop()
	time.Sleep(30 * time.Millisecond)

	select {
	case <-called:
		t.Fatal("expiry callback ran after Stop")
	default:
	}
}

func TestCollabInviteExpiryTwoInstancesCannotSweepSameBatch(t *testing.T) {
	db := setupCollabInviteExpiryDB(t)
	blocked := make(chan struct{})
	release := make(chan struct{})
	var blockFirst sync.Once
	if err := db.Callback().Update().Before("gorm:update").Register("test:block_first_invite_expiry", func(tx *gorm.DB) {
		if tx.Statement.Table != "collaboration_invites" {
			return
		}
		blockFirst.Do(func() {
			close(blocked)
			<-release
		})
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}

	now := collabInviteExpiryFixedNow
	first := NewCollabInviteExpiry(db, &config.CollaborationConfig{InviteExpireDays: 7})
	second := NewCollabInviteExpiry(db, &config.CollaborationConfig{InviteExpireDays: 7})
	first.now = func() time.Time { return now }
	second.now = func() time.Time { return now }

	type outcome struct {
		result collabInviteExpiryRunResult
		err    error
	}
	firstDone := make(chan outcome, 1)
	go func() {
		result, err := first.runOnceWithStatus()
		firstDone <- outcome{result: result, err: err}
	}()

	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("first sweep did not reach the update while holding the leader lock")
	}

	secondResult, err := second.runOnceWithStatus()
	if err != nil {
		t.Fatalf("second sweep returned error: %v", err)
	}
	if secondResult.AcquiredLeader || secondResult.Expired != 0 {
		t.Fatalf("second sweep result = %#v, want skipped without expiry", secondResult)
	}

	close(release)
	select {
	case outcome := <-firstDone:
		if outcome.err != nil {
			t.Fatalf("first sweep returned error: %v", outcome.err)
		}
		if !outcome.result.AcquiredLeader || outcome.result.Expired != 1 {
			t.Fatalf("first sweep result = %#v, want leader expiring one row", outcome.result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first sweep did not finish after release")
	}

	afterCommit, err := second.runOnceWithStatus()
	if err != nil {
		t.Fatalf("sweep after commit returned error: %v", err)
	}
	if !afterCommit.AcquiredLeader {
		t.Fatalf("sweep after commit result = %#v, want released lock", afterCommit)
	}
}

func setupCollabInviteExpiryDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			username VARCHAR(64) UNIQUE NOT NULL,
			reputation INT NOT NULL DEFAULT 10,
			is_banned BOOLEAN NOT NULL DEFAULT FALSE,
			deleted_at TIMESTAMPTZ,
			accept_collab_invites BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE content_items (
			id BIGSERIAL PRIMARY KEY,
			title VARCHAR(500) NOT NULL,
			author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			zone VARCHAR(10) NOT NULL,
			content_type VARCHAR(20) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			is_public BOOLEAN NOT NULL DEFAULT TRUE,
			deleted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE content_contributors (
			content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			pr_count INT NOT NULL DEFAULT 1,
			first_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (content_item_id, user_id)
		);
		CREATE TABLE conversations (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE conversation_participants (
			conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			last_read_at TIMESTAMPTZ,
			unread_count INTEGER NOT NULL DEFAULT 0,
			left_at TIMESTAMPTZ,
			PRIMARY KEY (conversation_id, user_id)
		);
		CREATE TABLE messages (
			id BIGSERIAL PRIMARY KEY,
			conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			sender_id BIGINT NOT NULL REFERENCES users(id),
			body TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`).Error)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "..", "migrations", "065_collaboration_invites.sql"))
	seedCollabInviteExpiryRows(t, db)
	return db
}

func seedCollabInviteExpiryRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`
		INSERT INTO users (id, email, password_hash, username)
		VALUES (1, 'expiry-author@example.test', 'hash', 'expiry-author')
	`).Error)
	for i := 10; i <= 14; i++ {
		require.NoError(t, db.Exec(`
			INSERT INTO users (id, email, password_hash, username)
			VALUES (?, ?, 'hash', ?)
		`, i, fmt.Sprintf("expiry-user-%d@example.test", i), fmt.Sprintf("expiry-user-%d", i)).Error)
	}
	require.NoError(t, db.Exec(`
		INSERT INTO content_items (id, title, author_id, zone, content_type, status, is_public)
		VALUES (100, 'expiry content', 1, 'original', 'article', 'published', TRUE)
	`).Error)

	now := collabInviteExpiryFixedNow
	seedCollabInviteExpiryRow(t, db, 1, 100, 1, 10, "pending", now.AddDate(0, 0, -8))
	seedCollabInviteExpiryRow(t, db, 2, 100, 1, 11, "pending", now.AddDate(0, 0, -7))
	seedCollabInviteExpiryRow(t, db, 3, 100, 1, 12, "pending", now.AddDate(0, 0, -6))
	seedCollabInviteExpiryRow(t, db, 4, 100, 1, 13, "accepted", now.AddDate(0, 0, -8))
	seedCollabInviteExpiryRow(t, db, 5, 100, 1, 14, "declined", now.AddDate(0, 0, -8))
}

func seedCollabInviteExpiryRow(t *testing.T, db *gorm.DB, id, contentID, inviterID, inviteeID int64, status string, createdAt time.Time) {
	t.Helper()
	require.NoError(t, db.Exec(`
		INSERT INTO collaboration_invites (id, content_id, inviter_id, invitee_id, status, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, contentID, inviterID, inviteeID, status, createdAt.Add(7*24*time.Hour), createdAt).Error)
}

func assertCollabInviteExpiryStatus(t *testing.T, db *gorm.DB, id int64, want string) {
	t.Helper()
	var statuses []string
	require.NoError(t, db.Raw("SELECT status FROM collaboration_invites WHERE id = ?", id).Pluck("status", &statuses).Error)
	if len(statuses) != 1 || statuses[0] != want {
		t.Fatalf("invite %d status = %v, want %q", id, statuses, want)
	}
}
