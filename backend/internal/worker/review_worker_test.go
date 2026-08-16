package worker

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/service"
	"omnicraft/backend/internal/testutil"
)

// countingGreenScanner records every scan entry so the worker redelivery test
// can assert Green is hit exactly once. It injects through ReviewService's
// SetGreenScanner seam, which accepts any value implementing the service
// greenScanner method set without exposing the interface type.
type countingGreenScanner struct {
	textScans int
}

func (c *countingGreenScanner) TextModeration(ctx context.Context, text string) (*aliyun.GreenScanResult, error) {
	c.textScans++
	return &aliyun.GreenScanResult{Result: "pass"}, nil
}

func (c *countingGreenScanner) ImageModeration(ctx context.Context, imageURL string) (*aliyun.GreenScanResult, error) {
	return &aliyun.GreenScanResult{Result: "pass"}, nil
}

func (c *countingGreenScanner) VideoAsyncScan(ctx context.Context, params aliyun.VideoScanParams) (*aliyun.GreenScanResult, error) {
	return &aliyun.GreenScanResult{Result: "review"}, nil
}

func setupReviewWorkerTest(t *testing.T) (*ReviewWorker, *gorm.DB, *countingGreenScanner) {
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
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE content_items (
			id BIGSERIAL PRIMARY KEY,
			title VARCHAR(500) NOT NULL,
			author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			ip_id BIGINT,
			zone VARCHAR(10) NOT NULL,
			content_type VARCHAR(20) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			is_public BOOLEAN NOT NULL DEFAULT TRUE,
			deleted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE ips (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			slug VARCHAR(255) UNIQUE NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE judge_cases (
			id BIGSERIAL PRIMARY KEY,
			target_type VARCHAR(20) NOT NULL,
			target_id BIGINT NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'open',
			vote_approve INT NOT NULL DEFAULT 0,
			vote_reject INT NOT NULL DEFAULT 0,
			min_votes INT NOT NULL DEFAULT 20,
			closed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE reputation_logs (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			delta INT NOT NULL,
			reason VARCHAR(100) NOT NULL,
			related_id BIGINT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`).Error)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "013_ai_review.sql"))
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "068_add_ai_review_records_task_id.sql"))
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "070_outbox_inbox.sql"))

	cfg := &config.Config{
		Reputation: config.ReputationConfig{
			RepeatViolationWindowDays:   7,
			RepeatViolationThreshold:    1,
			RepeatViolationExtraPenalty: -1,
		},
		Judge: config.JudgeConfig{MinVotesRequired: 3},
		OSS:   config.OSSConfig{Domain: "https://cdn.example.test"},
		Green: config.GreenConfig{
			Seed:        "seed_test_value",
			CallbackURL: "https://api.leeppp.online/api/v1/internal/ai-callback",
		},
	}
	svc := service.NewReviewService(db, nil, cfg, service.NewReputationService(db))
	scanner := &countingGreenScanner{}
	svc.SetGreenScanner(scanner)
	return NewReviewWorker(svc, db), db, scanner
}

func seedReviewWorkerContent(t *testing.T, db *gorm.DB) (userID, contentID int64) {
	t.Helper()
	require.NoError(t, db.Raw(`
		INSERT INTO users (email, password_hash, username, reputation)
		VALUES ('review-worker-test@example.com', 'hash', 'review-worker-test', 10) RETURNING id
	`).Scan(&userID).Error)
	require.NoError(t, db.Raw(`
		INSERT INTO content_items (title, author_id, zone, content_type, status)
		VALUES ('worker fixture', ?, 'meta', 'video', 'pending') RETURNING id
	`, userID).Scan(&contentID).Error)
	return userID, contentID
}

func reviewWorkerRecordsCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&model.AIReviewRecord{}).Count(&n).Error)
	return n
}

// #195 worker-level redelivery proof: simulate a crash after the side effect
// (the sync review record) committed but before the inbox completion record
// was written, by delivering the exact same stream message id twice. The sync
// record count must stay at 1 and Green must not be scanned twice.
func TestReviewWorkerSubmitAIReviewRedeliveryIsIdempotent(t *testing.T) {
	ww, db, scanner := setupReviewWorkerTest(t)
	ctx := context.Background()

	_, contentID := seedReviewWorkerContent(t, db)

	msgID := "1724600000000-0"
	payload, err := json.Marshal(map[string]interface{}{
		"action":          "submit_ai_review",
		"target_type":     "content",
		"target_id":       contentID,
		"content_type":    "video",
		"title":           "worker fixture",
		"description":     "redelivery must be idempotent",
		"author_id":       0,
		"cover_image_url": "",
	})
	require.NoError(t, err)

	msg := queue.Message{
		ID:      msgID,
		Topic:   "content.review",
		Group:   "omnicraft-content-review",
		Payload: payload,
	}

	require.NoError(t, ww.Handle(ctx, msg))
	require.NoError(t, ww.Handle(ctx, msg))

	require.Equal(t, int64(1), reviewWorkerRecordsCount(t, db), "redelivery must not add a second sync record")
	require.Equal(t, 1, scanner.textScans, "redelivery must not re-run the Green text scan")

	var status string
	require.NoError(t, db.Raw(`SELECT status FROM content_items WHERE id = ?`, contentID).Scan(&status).Error)
	require.Equal(t, "published", status)
}

// TestReviewWorkerUnknownActionReturnsError proves an unknown action is a
// permanent failure, not a silent skip: Handle returns an error so the broker
// can retry and dead-letter the message, and never writes an inbox completion
// row for an unhandled action.
func TestReviewWorkerUnknownActionReturnsError(t *testing.T) {
	ww, db, _ := setupReviewWorkerTest(t)

	err := ww.Handle(context.Background(), queue.Message{
		ID: "1-0", Topic: "content.review", Group: "omnicraft-content-review",
		Payload: []byte(`{"action":"no_such_action","target_type":"content","target_id":1}`),
	})
	require.Error(t, err, "an unknown action must fail so it can be DLQ'd instead of being swallowed")
	require.Contains(t, err.Error(), "unknown action")
	require.Contains(t, err.Error(), "no_such_action")

	var inbox int64
	require.NoError(t, db.Model(&model.InboxConsumer{}).Count(&inbox).Error)
	require.Zero(t, inbox, "an unhandled action must never leave an inbox completion row")
}

// TestReviewWorkerUnknownActionLandsInDLQ proves the end-to-end retry->DLQ
// path for an unknown action: the broker retries the failing message and,
// once attempts are exhausted, dead-letters it with topic/group/error
// metadata. No inbox completion row is written along the way.
func TestReviewWorkerUnknownActionLandsInDLQ(t *testing.T) {
	ww, db, _ := setupReviewWorkerTest(t)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	broker := queue.NewRedisStreamBroker(rdb, &queue.QueueConfig{
		Enabled:         true,
		MaxAttempts:     2,
		RetryBackoffSec: []int{0, 0},
		MaxLen:          100000,
	})
	defer broker.Stop()

	mgr := NewWorkerManager(broker)
	mgr.Register("content.review", "omnicraft-content-review", ww.Handle)
	require.NoError(t, mgr.Start(context.Background()))

	payload, err := json.Marshal(map[string]interface{}{
		"action":      "no_such_action",
		"target_type": "content",
		"target_id":   1,
	})
	require.NoError(t, err)
	require.NoError(t, broker.Publish(context.Background(), "content.review", payload))

	var dlqEntries []redis.XMessage
	deadline := time.Now().Add(8 * time.Second)
	for {
		entries, err := rdb.XRange(context.Background(), "omnicraft:dead-letter", "-", "+").Result()
		require.NoError(t, err)
		if len(entries) > 0 {
			dlqEntries = entries
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("unknown action message never reached the DLQ")
		}
		time.Sleep(50 * time.Millisecond)
	}

	var entry struct {
		OriginalTopic string `json:"original_topic"`
		OriginalID    string `json:"original_id"`
		ConsumerGroup string `json:"consumer_group"`
		Error         string `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(dlqEntries[len(dlqEntries)-1].Values["data"].(string)), &entry))
	require.Equal(t, "content.review", entry.OriginalTopic)
	require.Equal(t, "omnicraft-content-review", entry.ConsumerGroup)
	require.Contains(t, entry.Error, "unknown action")
	require.Contains(t, entry.Error, "no_such_action")

	var inbox int64
	require.NoError(t, db.Model(&model.InboxConsumer{}).Count(&inbox).Error)
	require.Zero(t, inbox, "a dead-lettered message must never leave an inbox completion row")
}
