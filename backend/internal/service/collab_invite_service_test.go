package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/testutil"
)

// collabInviteFixedNow is 2026-08-11 10:00 Asia/Shanghai; used as the injected
// service clock so Redis keys and expires_at are deterministic.
var collabInviteFixedNow = time.Date(2026, 8, 11, 2, 0, 0, 0, time.UTC)

func setupCollabInviteServiceTest(t *testing.T) (*CollabInviteService, *gorm.DB, *miniredis.Miniredis) {
	t.Helper()

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	createCollabInviteBaseSchema(t, db)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "065_collaboration_invites.sql"))

	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })

	cfg := &config.Config{
		Collaboration: config.CollaborationConfig{
			InviteDailyLimit:       20,
			InviteExpireDays:       7,
			MaxInviteesPerPublish:  5,
			MaxContributorsPerItem: 10,
		},
		Reputation: config.ReputationConfig{MinScoreForInteraction: 3},
	}
	svc := NewCollabInviteService(
		repository.NewContentRepository(db),
		repository.NewCollabInviteRepository(db),
		repository.NewMessageRepository(db),
		repository.NewUserRepository(db),
		rdb,
		cfg,
	)
	svc.now = func() time.Time { return collabInviteFixedNow }
	return svc, db, mr
}

func createCollabInviteBaseSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
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
		CREATE TABLE author_blocklist (
			author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			blocked_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (author_id, blocked_id)
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
}

func seedCollabUser(t *testing.T, db *gorm.DB, id int64, username string, reputation int, banned bool, deletedAt *time.Time, accepts *bool) {
	t.Helper()
	acceptsValue := true
	if accepts != nil {
		acceptsValue = *accepts
	}
	require.NoError(t, db.Exec(`
		INSERT INTO users (id, email, password_hash, username, reputation, is_banned, deleted_at, accept_collab_invites)
		VALUES (?, ?, 'hash', ?, ?, ?, ?, ?)
	`, id, fmt.Sprintf("%s@example.test", username), username, reputation, banned, deletedAt, acceptsValue).Error)
}

func seedCollabContent(t *testing.T, db *gorm.DB, id, authorID int64, status string, deletedAt *time.Time, title string) {
	t.Helper()
	if title == "" {
		title = fmt.Sprintf("content-%d", id)
	}
	require.NoError(t, db.Exec(`
		INSERT INTO content_items (id, title, author_id, zone, content_type, status, is_public, deleted_at)
		VALUES (?, ?, ?, 'original', 'article', ?, TRUE, ?)
	`, id, title, authorID, status, deletedAt).Error)
}

func seedCollabContributor(t *testing.T, db *gorm.DB, contentID, userID int64) {
	t.Helper()
	require.NoError(t, db.Exec(`
		INSERT INTO content_contributors (content_item_id, user_id, pr_count, first_at)
		VALUES (?, ?, 1, NOW())
	`, contentID, userID).Error)
}

func seedCollabInvite(t *testing.T, db *gorm.DB, id, contentID, inviterID, inviteeID int64, status string) {
	t.Helper()
	require.NoError(t, db.Exec(`
		INSERT INTO collaboration_invites (id, content_id, inviter_id, invitee_id, status, expires_at)
		VALUES (?, ?, ?, ?, ?, NOW() + INTERVAL '7 days')
	`, id, contentID, inviterID, inviteeID, status).Error)
}

func collabInviteFixedCountKey(inviterID int64) string {
	return fmt.Sprintf("collab_invite_count:%d:2026-08-11", inviterID)
}

func collabInviteFixedUserKey(inviterID, inviteeID int64) string {
	return fmt.Sprintf("collab_invite_user:%d:%d:2026-08-11", inviterID, inviteeID)
}

func collabInviteCounts(t *testing.T, db *gorm.DB) (invites int64, conversations int64, messages int64) {
	t.Helper()
	require.NoError(t, db.Model(&model.CollabInvite{}).Count(&invites).Error)
	require.NoError(t, db.Model(&model.Conversation{}).Count(&conversations).Error)
	require.NoError(t, db.Model(&model.Message{}).Count(&messages).Error)
	return
}

func TestCollabInviteSendOwnerCanInvite(t *testing.T) {
	svc, db, mr := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "Alice Novel")

	invite, err := svc.SendInvite(context.Background(), 100, 1, 2)
	require.NoError(t, err)
	require.NotNil(t, invite)
	require.Equal(t, model.CollabInviteStatusPending, invite.Status)
	require.Equal(t, collabInviteFixedNow.Add(7*24*time.Hour), invite.ExpiresAt)
	require.NotNil(t, invite.MessageID)

	invites, conversations, messages := collabInviteCounts(t, db)
	require.EqualValues(t, 1, invites)
	require.EqualValues(t, 1, conversations)
	require.EqualValues(t, 1, messages)

	var msg model.Message
	require.NoError(t, db.First(&msg, invite.MessageID).Error)
	require.Equal(t, "collab_invite", msg.MsgType)
	require.Equal(t, "联合创作邀请", msg.Body)
	require.Len(t, msg.Metadata, 5)
	require.Equal(t, float64(invite.ID), msg.Metadata["invite_id"])
	require.Equal(t, float64(100), msg.Metadata["content_id"])
	require.Equal(t, "Alice Novel", msg.Metadata["content_title"])
	require.Equal(t, float64(1), msg.Metadata["inviter_id"])
	require.Equal(t, "alice", msg.Metadata["inviter_username"])

	var participant model.ConversationParticipant
	require.NoError(t, db.Where("conversation_id = ? AND user_id = ?", msg.ConversationID, 2).First(&participant).Error)
	require.Equal(t, 1, participant.UnreadCount)

	got, gerr := mr.Get(collabInviteFixedCountKey(1))
	require.NoError(t, gerr)
	require.Equal(t, "1", got)
	require.Equal(t, 86400*time.Second, mr.TTL(collabInviteFixedCountKey(1)))
	require.True(t, mr.Exists(collabInviteFixedUserKey(1, 2)))
	require.Equal(t, 86400*time.Second, mr.TTL(collabInviteFixedUserKey(1, 2)))
}

func TestCollabInviteSendPendingContentAllowed(t *testing.T) {
	svc, db, _ := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "pending", nil, "")

	invite, err := svc.SendInvite(context.Background(), 100, 1, 2)
	require.NoError(t, err)
	require.NotNil(t, invite)
}

func TestCollabInviteSendConfirmedContributorCanInvite(t *testing.T) {
	svc, db, _ := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "carol", 10, false, nil, nil)
	seedCollabUser(t, db, 3, "bob", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")
	seedCollabContributor(t, db, 100, 2)

	invite, err := svc.SendInvite(context.Background(), 100, 2, 3)
	require.NoError(t, err)
	require.NotNil(t, invite)
	require.Equal(t, int64(2), invite.InviterID)
}

func TestCollabInviteSendRejectsSelf(t *testing.T) {
	svc, db, _ := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")

	_, err := svc.SendInvite(context.Background(), 100, 1, 1)
	require.ErrorIs(t, err, ErrInviteSelfNotAllowed)
}

func TestCollabInviteSendRejectsAuthor(t *testing.T) {
	svc, db, _ := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "carol", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")
	seedCollabContributor(t, db, 100, 2)

	_, err := svc.SendInvite(context.Background(), 100, 2, 1)
	require.ErrorIs(t, err, ErrInviteAuthorNotAllowed)
}

func TestCollabInviteSendRejectsExistingContributor(t *testing.T) {
	svc, db, _ := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "carol", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")
	seedCollabContributor(t, db, 100, 2)

	_, err := svc.SendInvite(context.Background(), 100, 1, 2)
	require.ErrorIs(t, err, ErrInviteAlreadyContributor)
}

func TestCollabInviteSendRejectsUnavailableInvitee(t *testing.T) {
	svc, db, _ := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")

	t.Run("banned", func(t *testing.T) {
		seedCollabUser(t, db, 2, "banned-user", 10, true, nil, nil)
		_, err := svc.SendInvite(context.Background(), 100, 1, 2)
		require.ErrorIs(t, err, ErrInviteeUnavailable)
	})

	t.Run("soft-deleted", func(t *testing.T) {
		deletedAt := time.Now().UTC()
		seedCollabUser(t, db, 3, "deleted-user", 10, false, &deletedAt, nil)
		_, err := svc.SendInvite(context.Background(), 100, 1, 3)
		require.ErrorIs(t, err, ErrInviteeUnavailable)
	})

	t.Run("missing", func(t *testing.T) {
		_, err := svc.SendInvite(context.Background(), 100, 1, 9999)
		require.ErrorIs(t, err, ErrInviteeUnavailable)
	})
}

func TestCollabInviteSendRejectsUnavailableContent(t *testing.T) {
	svc, db, _ := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)

	for _, tc := range []struct {
		id     int64
		name   string
		status string
	}{
		{id: 100, name: "banned", status: "banned"},
		{id: 101, name: "under_review", status: "under_review"},
		{id: 102, name: "author_deleted", status: "author_deleted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seedCollabContent(t, db, tc.id, 1, tc.status, nil, "")
			_, err := svc.SendInvite(context.Background(), tc.id, 1, 2)
			require.ErrorIs(t, err, ErrContentUnavailable)
		})
	}

	t.Run("soft-deleted", func(t *testing.T) {
		deletedAt := time.Now().UTC()
		seedCollabContent(t, db, 103, 1, "published", &deletedAt, "")
		_, err := svc.SendInvite(context.Background(), 103, 1, 2)
		require.ErrorIs(t, err, ErrContentUnavailable)
	})

	t.Run("missing", func(t *testing.T) {
		_, err := svc.SendInvite(context.Background(), 99999, 1, 2)
		require.ErrorIs(t, err, ErrContentUnavailable)
	})
}

func TestCollabInviteSendRejectsNonOwner(t *testing.T) {
	svc, db, _ := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)
	seedCollabUser(t, db, 3, "carol", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")

	_, err := svc.SendInvite(context.Background(), 100, 3, 2)
	require.ErrorIs(t, err, ErrNotContentOwner)
}

func TestCollabInviteSendRejectsLowReputation(t *testing.T) {
	svc, db, _ := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 2, false, nil, nil)
	seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")

	_, err := svc.SendInvite(context.Background(), 100, 1, 2)
	require.ErrorIs(t, err, ErrReputationTooLow)
}

func TestCollabInviteSendRejectsContributorLimit(t *testing.T) {
	svc, db, _ := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")
	for i := 3; i <= 12; i++ {
		seedCollabUser(t, db, int64(i), fmt.Sprintf("contrib-%d", i), 10, false, nil, nil)
		seedCollabContributor(t, db, 100, int64(i))
	}
	seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)

	_, err := svc.SendInvite(context.Background(), 100, 1, 2)
	require.ErrorIs(t, err, ErrContributorLimitReached)
}

func TestCollabInviteSendCapacityCountsPendingDistinctNonContributors(t *testing.T) {
	svc, db, _ := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")
	for i := 3; i <= 11; i++ {
		seedCollabUser(t, db, int64(i), fmt.Sprintf("contrib-%d", i), 10, false, nil, nil)
		seedCollabContributor(t, db, 100, int64(i))
	}

	t.Run("pending invite to existing contributor does not consume a slot", func(t *testing.T) {
		seedCollabInvite(t, db, 500, 100, 1, 3, "pending")
		seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)
		invite, err := svc.SendInvite(context.Background(), 100, 1, 2)
		require.NoError(t, err)
		require.NotNil(t, invite)
	})

	t.Run("pending invite to distinct non-contributor consumes a slot", func(t *testing.T) {
		seedCollabUser(t, db, 12, "pending-invitee", 10, false, nil, nil)
		seedCollabInvite(t, db, 501, 100, 1, 12, "pending")
		seedCollabUser(t, db, 13, "new-invitee", 10, false, nil, nil)
		_, err := svc.SendInvite(context.Background(), 100, 1, 13)
		require.ErrorIs(t, err, ErrContributorLimitReached)
	})
}

func TestCollabInviteSendConcurrentCapacity(t *testing.T) {
	svc, db, mr := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")
	for i := 3; i <= 11; i++ {
		seedCollabUser(t, db, int64(i), fmt.Sprintf("contrib-%d", i), 10, false, nil, nil)
		seedCollabContributor(t, db, 100, int64(i))
	}
	seedCollabUser(t, db, 12, "invitee-a", 10, false, nil, nil)
	seedCollabUser(t, db, 13, "invitee-b", 10, false, nil, nil)

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, inviteeID := range []int64{12, 13} {
		wg.Add(1)
		go func(inviteeID int64) {
			defer wg.Done()
			<-start
			_, err := svc.SendInvite(context.Background(), 100, 1, inviteeID)
			results <- err
		}(inviteeID)
	}
	close(start)
	wg.Wait()
	close(results)

	successes, limitReached := 0, 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrContributorLimitReached):
			limitReached++
		default:
			t.Fatalf("unexpected concurrent send error: %v", err)
		}
	}
	if successes != 1 || limitReached != 1 {
		t.Fatalf("results = %d successes and %d limit-reached, want 1/1", successes, limitReached)
	}

	invites, conversations, messages := collabInviteCounts(t, db)
	require.EqualValues(t, 1, invites)
	require.EqualValues(t, 1, conversations)
	require.EqualValues(t, 1, messages)

	got, gerr := mr.Get(collabInviteFixedCountKey(1))
	require.NoError(t, gerr)
	require.Equal(t, "1", got)
	userKeys := 0
	for _, key := range mr.Keys() {
		if len(key) > len("collab_invite_user:") && key[:len("collab_invite_user:")] == "collab_invite_user:" {
			userKeys++
		}
	}
	require.Equal(t, 1, userKeys)
}

func TestCollabInviteSendRejectsDailyLimit(t *testing.T) {
	svc, db, mr := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")
	mr.Set(collabInviteFixedCountKey(1), "20")

	_, err := svc.SendInvite(context.Background(), 100, 1, 2)
	require.ErrorIs(t, err, ErrInviteDailyLimit)
	got, gerr := mr.Get(collabInviteFixedCountKey(1))
	require.NoError(t, gerr)
	require.Equal(t, "20", got)
	require.False(t, mr.Exists(collabInviteFixedUserKey(1, 2)))
}

func TestCollabInviteSendRejectsDuplicateUser(t *testing.T) {
	svc, db, mr := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")
	mr.Set(collabInviteFixedUserKey(1, 2), "token")

	_, err := svc.SendInvite(context.Background(), 100, 1, 2)
	require.ErrorIs(t, err, ErrInviteDuplicateUser)
	require.False(t, mr.Exists(collabInviteFixedCountKey(1)))
}

func TestCollabInviteSendRejectsBlocked(t *testing.T) {
	svc, db, _ := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")

	t.Run("inviter blocks invitee", func(t *testing.T) {
		require.NoError(t, db.Exec(`
			INSERT INTO author_blocklist (author_id, blocked_id)
			VALUES (1, 2)
		`).Error)
		_, err := svc.SendInvite(context.Background(), 100, 1, 2)
		require.ErrorIs(t, err, ErrInviteBlocked)
	})

	t.Run("invitee blocks inviter", func(t *testing.T) {
		require.NoError(t, db.Exec(`
			INSERT INTO author_blocklist (author_id, blocked_id)
			VALUES (2, 1)
		`).Error)
		_, err := svc.SendInvite(context.Background(), 100, 1, 2)
		require.ErrorIs(t, err, ErrInviteBlocked)
	})
}

func TestCollabInviteSendRejectsNotAccepting(t *testing.T) {
	svc, db, _ := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	accepts := false
	seedCollabUser(t, db, 2, "bob", 10, false, nil, &accepts)
	seedCollabContent(t, db, 100, 1, "published", nil, "")

	_, err := svc.SendInvite(context.Background(), 100, 1, 2)
	require.ErrorIs(t, err, ErrInviteNotAccepting)
}

func TestCollabInviteSendRejectsActiveDuplicate(t *testing.T) {
	svc, db, mr := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")
	seedCollabInvite(t, db, 500, 100, 1, 2, "pending")

	_, err := svc.SendInvite(context.Background(), 100, 1, 2)
	require.ErrorIs(t, err, ErrInviteAlreadyExists)
	require.False(t, mr.Exists(collabInviteFixedCountKey(1)))
}

func TestCollabInviteSendAllowsReinviteAfterExpired(t *testing.T) {
	svc, db, _ := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")
	seedCollabInvite(t, db, 500, 100, 1, 2, "expired")

	invite, err := svc.SendInvite(context.Background(), 100, 1, 2)
	require.NoError(t, err)
	require.NotNil(t, invite)
	require.NotEqual(t, int64(500), invite.ID)

	var total int64
	require.NoError(t, db.Model(&model.CollabInvite{}).Count(&total).Error)
	require.EqualValues(t, 2, total)
}

func TestCollabInviteSendExemptsTypedMessageFromColdStartGuard(t *testing.T) {
	svc, db, _ := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")

	invite, err := svc.SendInvite(context.Background(), 100, 1, 2)
	require.NoError(t, err)
	require.NotNil(t, invite)

	msgRepo := repository.NewMessageRepository(db)
	_, err = msgRepo.SendWithColdStartGuard(1, 2, "hello")
	require.ErrorIs(t, err, repository.ErrDMReplyRequired)

	reply, err := msgRepo.SendWithColdStartGuard(2, 1, "hi back")
	require.NoError(t, err)
	require.NotNil(t, reply)

	followup, err := msgRepo.SendWithColdStartGuard(1, 2, "how are you")
	require.NoError(t, err)
	require.NotNil(t, followup)
}

func TestCollabInviteSendRedisUnavailableAbortsBeforeDBWrite(t *testing.T) {
	svc, db, mr := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")
	mr.Close()

	_, err := svc.SendInvite(context.Background(), 100, 1, 2)
	require.ErrorIs(t, err, ErrInviteRateLimitUnavailable)

	invites, conversations, messages := collabInviteCounts(t, db)
	require.EqualValues(t, 0, invites)
	require.EqualValues(t, 0, conversations)
	require.EqualValues(t, 0, messages)
}

func TestCollabInviteSendDBFailureCompensatesReservation(t *testing.T) {
	svc, db, mr := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")

	require.NoError(t, db.Exec(`
		CREATE OR REPLACE FUNCTION fail_collab_invite_insert() RETURNS trigger
		LANGUAGE plpgsql AS $$
		BEGIN
			RAISE EXCEPTION 'forced collab invite insert failure';
		END;
		$$;
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TRIGGER fail_collab_invite_insert
		BEFORE INSERT ON collaboration_invites
		FOR EACH ROW EXECUTE FUNCTION fail_collab_invite_insert();
	`).Error)
	t.Cleanup(func() {
		_ = db.Exec("DROP TRIGGER IF EXISTS fail_collab_invite_insert ON collaboration_invites").Error
		_ = db.Exec("DROP FUNCTION IF EXISTS fail_collab_invite_insert()").Error
	})

	_, err := svc.SendInvite(context.Background(), 100, 1, 2)
	require.Error(t, err)

	invites, conversations, messages := collabInviteCounts(t, db)
	require.EqualValues(t, 0, invites)
	require.EqualValues(t, 0, conversations)
	require.EqualValues(t, 0, messages)

	require.False(t, mr.Exists(collabInviteFixedCountKey(1)))
	require.False(t, mr.Exists(collabInviteFixedUserKey(1, 2)))
}

func TestCollabInviteSendDateKeysUseAsiaShanghaiMidnight(t *testing.T) {
	svc, db, mr := setupCollabInviteServiceTest(t)
	seedCollabUser(t, db, 1, "alice", 10, false, nil, nil)
	seedCollabUser(t, db, 2, "bob", 10, false, nil, nil)
	seedCollabUser(t, db, 3, "carol", 10, false, nil, nil)
	seedCollabContent(t, db, 100, 1, "published", nil, "")

	beforeMidnight := time.Date(2026, 8, 11, 15, 59, 0, 0, time.UTC)
	afterMidnight := time.Date(2026, 8, 11, 16, 0, 0, 0, time.UTC)

	svc.now = func() time.Time { return beforeMidnight }
	_, err := svc.SendInvite(context.Background(), 100, 1, 2)
	require.NoError(t, err)
	require.True(t, mr.Exists("collab_invite_count:1:2026-08-11"))
	require.True(t, mr.Exists("collab_invite_user:1:2:2026-08-11"))
	require.False(t, mr.Exists("collab_invite_count:1:2026-08-12"))

	svc.now = func() time.Time { return afterMidnight }
	_, err = svc.SendInvite(context.Background(), 100, 1, 3)
	require.NoError(t, err)
	got, gerr := mr.Get("collab_invite_count:1:2026-08-11")
	require.NoError(t, gerr)
	require.Equal(t, "1", got)
	got, gerr = mr.Get("collab_invite_count:1:2026-08-12")
	require.NoError(t, gerr)
	require.Equal(t, "1", got)
	require.True(t, mr.Exists("collab_invite_user:1:3:2026-08-12"))
}
