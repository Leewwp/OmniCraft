package repository

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/testutil"
)

func TestSendWithColdStartGuardSerializesConcurrentNewPair(t *testing.T) {
	db := setupMessageRepositoryPostgresTest(t)
	const attempts = 8
	repo := NewMessageRepository(db)
	start := make(chan struct{})
	errs := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := repo.SendWithColdStartGuard(1, 2, fmt.Sprintf("hello %d", i))
			errs <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	replyRequired := 0
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrDMReplyRequired):
			replyRequired++
		default:
			t.Fatalf("unexpected send error: %v", err)
		}
	}
	if successes != 1 || replyRequired != attempts-1 {
		t.Fatalf("results = %d successes and %d reply-required errors, want 1/%d", successes, replyRequired, attempts-1)
	}

	var messageCount int64
	if err := db.Model(&model.Message{}).Count(&messageCount).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if messageCount != 1 {
		t.Fatalf("message count = %d, want 1", messageCount)
	}

	var conversationCount int64
	if err := db.Model(&model.Conversation{}).Count(&conversationCount).Error; err != nil {
		t.Fatalf("count conversations: %v", err)
	}
	if conversationCount != 1 {
		t.Fatalf("conversation count = %d, want 1", conversationCount)
	}
}

func TestSendWithColdStartGuardWaitsForPostgresPairAdvisoryLock(t *testing.T) {
	db := setupMessageRepositoryPostgresTest(t)
	key := conversationPairAdvisoryKey(conversationPairKey(1, 2))
	lockTx := db.Begin()
	if lockTx.Error != nil {
		t.Fatalf("begin lock transaction: %v", lockTx.Error)
	}
	lockReleased := false
	t.Cleanup(func() {
		if !lockReleased {
			_ = lockTx.Rollback().Error
		}
	})
	if err := lockTx.Exec("SELECT pg_advisory_xact_lock(?)", key).Error; err != nil {
		t.Fatalf("hold pair advisory lock: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := NewMessageRepository(db).SendWithColdStartGuard(1, 2, "hello")
		done <- err
	}()

	waitForBlockedAdvisoryLock(t, db, key, done)

	var messageCount int64
	if err := db.Model(&model.Message{}).Count(&messageCount).Error; err != nil {
		t.Fatalf("count messages before lock release: %v", err)
	}
	if messageCount != 0 {
		t.Fatalf("message count before lock release = %d, want 0", messageCount)
	}

	if err := lockTx.Commit().Error; err != nil {
		t.Fatalf("release pair advisory lock: %v", err)
	}
	lockReleased = true

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("send after advisory lock release: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for send after advisory lock release")
	}
}

func setupMessageRepositoryPostgresTest(t *testing.T) *gorm.DB {
	t.Helper()

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	if err := db.AutoMigrate(&model.User{}, &model.Conversation{}, &model.ConversationParticipant{}, &model.Message{}); err != nil {
		t.Fatalf("migrate message tables: %v", err)
	}
	users := []model.User{
		{ID: 1, Email: "alice@example.test", PasswordHash: "hash", Username: "alice"},
		{ID: 2, Email: "bob@example.test", PasswordHash: "hash", Username: "bob"},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	return db
}

func waitForBlockedAdvisoryLock(t *testing.T, db *gorm.DB, key int64, done <-chan error) {
	t.Helper()

	classID, objectID := advisoryLockParts(key)
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			t.Fatalf("send completed before waiting on the pair advisory lock: %v", err)
		case <-ticker.C:
			var waiting bool
			if err := db.Raw(`
				SELECT EXISTS (
					SELECT 1
					FROM pg_locks
					WHERE locktype = 'advisory'
					  AND NOT granted
					  AND database = (
						SELECT oid
						FROM pg_database
						WHERE datname = current_database()
					  )
					  AND classid::bigint = ?
					  AND objid::bigint = ?
					  AND objsubid = 1
				)
			`, int64(classID), int64(objectID)).Scan(&waiting).Error; err != nil {
				t.Fatalf("check waiting advisory lock: %v", err)
			}
			if waiting {
				return
			}
		case <-deadline.C:
			t.Fatal("timed out waiting for send to block on the pair advisory lock")
		}
	}
}

func advisoryLockParts(key int64) (uint32, uint32) {
	unsigned := uint64(key)
	return uint32(unsigned >> 32), uint32(unsigned)
}
