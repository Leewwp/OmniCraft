package service

import (
	"context"
	"fmt"
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
	"omnicraft/backend/internal/pkg/rediskeys"
	"omnicraft/backend/internal/testutil"
)

// setupReviewServiceTest wires a real ephemeral Postgres (013 + 068 applied),
// an in-memory Redis and a ReviewService with the repeat-violation threshold
// at 1 so a single block already triggers the extra penalty and freeze.
func setupReviewServiceTest(t *testing.T) (*ReviewService, *gorm.DB, *miniredis.Miniredis) {
	t.Helper()

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	createReviewBaseSchema(t, db)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "013_ai_review.sql"))
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "068_add_ai_review_records_task_id.sql"))

	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })

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
	svc := NewReviewService(db, rdb, cfg, NewReputationService(db))
	return svc, db, mr
}

// fakeGreenScanner is the injectable seam for ReviewService.green: it records
// the scan calls so tests can assert both the parameters sent to Green and the
// rejection semantics (no scan call at all for non-platform cover URLs).
type fakeGreenScanner struct {
	textResult  *aliyun.GreenScanResult
	imageResult *aliyun.GreenScanResult
	videoResult *aliyun.GreenScanResult
	videoErr    error
	imageErr    error

	imageCalls []string
	videoCalls []aliyun.VideoScanParams
}

func (f *fakeGreenScanner) TextModeration(ctx context.Context, text string) (*aliyun.GreenScanResult, error) {
	if f.textResult != nil {
		return f.textResult, nil
	}
	return &aliyun.GreenScanResult{Result: "pass"}, nil
}

func (f *fakeGreenScanner) ImageModeration(ctx context.Context, imageURL string) (*aliyun.GreenScanResult, error) {
	f.imageCalls = append(f.imageCalls, imageURL)
	if f.imageErr != nil {
		return nil, f.imageErr
	}
	if f.imageResult != nil {
		return f.imageResult, nil
	}
	return &aliyun.GreenScanResult{Result: "pass"}, nil
}

func (f *fakeGreenScanner) VideoAsyncScan(ctx context.Context, params aliyun.VideoScanParams) (*aliyun.GreenScanResult, error) {
	f.videoCalls = append(f.videoCalls, params)
	if f.videoErr != nil {
		return nil, f.videoErr
	}
	if f.videoResult != nil {
		return f.videoResult, nil
	}
	return &aliyun.GreenScanResult{Result: "pass"}, nil
}

func createReviewBaseSchema(t *testing.T, db *gorm.DB) {
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
}

func seedReviewUser(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	var id int64
	require.NoError(t, db.Raw(`
		INSERT INTO users (email, password_hash, username, reputation)
		VALUES (?, 'hash', ?, 10) RETURNING id
	`, "review-"+nonce+"-user@example.com", "review-"+nonce).Scan(&id).Error)
	return id
}

func seedReviewContent(t *testing.T, db *gorm.DB, authorID int64, ipID ...*int64) int64 {
	t.Helper()
	var id int64
	if len(ipID) > 0 && ipID[0] != nil {
		require.NoError(t, db.Raw(`
			INSERT INTO content_items (title, author_id, ip_id, zone, content_type, status)
			VALUES ('review fixture', ?, ?, 'meta', 'video', 'pending') RETURNING id
		`, authorID, *ipID[0]).Scan(&id).Error)
		return id
	}
	require.NoError(t, db.Raw(`
		INSERT INTO content_items (title, author_id, zone, content_type, status)
		VALUES ('review fixture', ?, 'meta', 'video', 'pending') RETURNING id
	`, authorID).Scan(&id).Error)
	return id
}

func seedReviewIP(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.Raw(`
		INSERT INTO ips (name, slug, status)
		VALUES (?, ?, 'pending') RETURNING id
	`, "review ip", fmt.Sprintf("review-ip-%d", time.Now().UnixNano())).Scan(&id).Error)
	return id
}

func reviewRecordsCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&model.AIReviewRecord{}).Count(&count).Error)
	return count
}

func reviewContentStatus(t *testing.T, db *gorm.DB, contentID int64) string {
	t.Helper()
	var status string
	require.NoError(t, db.Raw(`SELECT status FROM content_items WHERE id = ?`, contentID).Scan(&status).Error)
	return status
}

func reviewIPStatus(t *testing.T, db *gorm.DB, ipID int64) string {
	t.Helper()
	var status string
	require.NoError(t, db.Raw(`SELECT status FROM ips WHERE id = ?`, ipID).Scan(&status).Error)
	return status
}

func reputationLogReasons(t *testing.T, db *gorm.DB, userID int64) []string {
	t.Helper()
	var reasons []string
	require.NoError(t, db.Model(&model.ReputationLog{}).
		Where("user_id = ?", userID).Order("id ASC").Pluck("reason", &reasons).Error)
	return reasons
}

func userReputation(t *testing.T, db *gorm.DB, userID int64) int {
	t.Helper()
	var reputation int
	require.NoError(t, db.Raw(`SELECT reputation FROM users WHERE id = ?`, userID).Scan(&reputation).Error)
	return reputation
}

func openJudgeCaseCount(t *testing.T, db *gorm.DB, contentID int64) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&model.JudgeCase{}).
		Where("target_id = ? AND status = ?", contentID, "open").Count(&count).Error)
	return count
}

// Idempotency: a duplicate callback (same provider_task_id) returns success
// without adding a record, re-charging reputation or re-freezing.
func TestProcessAICallbackDuplicateTaskIDIsIdempotent(t *testing.T) {
	svc, db, mr := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)
	freezeKey := rediskeys.PublishFreezeKey(userID)

	first := AICallbackInput{
		TargetType:     "content",
		TargetID:       contentID,
		Result:         "block",
		ProviderTaskID: "task-block-1",
	}
	require.NoError(t, svc.ProcessAICallback(ctx, first))

	require.Equal(t, int64(1), reviewRecordsCount(t, db), "first callback records once")
	require.Equal(t, "banned", reviewContentStatus(t, db, contentID))
	require.Equal(t, []string{"ai_violation", "repeat_violation"}, reputationLogReasons(t, db, userID))
	require.Equal(t, 6, userReputation(t, db, userID), "10 - 3 (ai_violation) - 1 (repeat_violation)")
	require.True(t, mr.Exists(freezeKey), "publish freeze must be set after the block")

	// The retried delivery carries the same task id but a different result:
	// it must be ignored entirely, including the status flip.
	duplicate := AICallbackInput{
		TargetType:     "content",
		TargetID:       contentID,
		Result:         "pass",
		ProviderTaskID: "task-block-1",
	}
	require.NoError(t, svc.ProcessAICallback(ctx, duplicate), "duplicate callback must succeed (HTTP 200 semantics)")

	require.Equal(t, int64(1), reviewRecordsCount(t, db), "duplicate callback must not add a record")
	require.Equal(t, "banned", reviewContentStatus(t, db, contentID), "duplicate callback must not resurrect content")
	require.Equal(t, []string{"ai_violation", "repeat_violation"}, reputationLogReasons(t, db, userID))
	require.Equal(t, 6, userReputation(t, db, userID), "reputation must be charged exactly once")
	require.True(t, mr.Exists(freezeKey), "freeze must not be re-set by the duplicate")
}

// Callbacks without a provider task id (synchronous internal path) keep the
// previous behavior: every delivery is recorded and applied.
func TestProcessAICallbackWithoutTaskIDIsNotIdempotent(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)

	for i := 0; i < 2; i++ {
		require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
			TargetType: "content",
			TargetID:   contentID,
			Result:     "pass",
		}))
	}
	require.Equal(t, int64(2), reviewRecordsCount(t, db))
	require.Equal(t, "published", reviewContentStatus(t, db, contentID))
	require.Empty(t, reputationLogReasons(t, db, userID))
}

// A2: a synchronous review produces exactly one ai_review_records row.
func TestSyncReviewProducesExactlyOneRecord(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType: "content",
		TargetID:   contentID,
		Result:     "pass",
	}))
	require.Equal(t, int64(1), reviewRecordsCount(t, db))
	require.Equal(t, "published", reviewContentStatus(t, db, contentID))
}

// A2: the synchronous record (no task id) and the async result (with task id)
// are separate stages and must not share an idempotency key.
func TestSyncAndAsyncRecordsUseSeparateIdempotencyKeys(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType: "content",
		TargetID:   contentID,
		Result:     "pass",
	}))
	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType:     "content",
		TargetID:       contentID,
		Result:         "pass",
		ProviderTaskID: "task-async-1",
	}))
	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType:     "content",
		TargetID:       contentID,
		Result:         "pass",
		ProviderTaskID: "task-async-1",
	}))
	require.Equal(t, int64(2), reviewRecordsCount(t, db),
		"sync record and async record are distinct stages; async duplicate is short-circuited")
}

// A1: a synchronous block is terminal; a later async pass must not resurrect
// the content (sync image block -> async video pass).
func TestProcessAICallbackSyncBlockThenAsyncPassKeepsBanned(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType: "content",
		TargetID:   contentID,
		Result:     "block",
	}))
	require.Equal(t, "banned", reviewContentStatus(t, db, contentID))

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType:     "content",
		TargetID:       contentID,
		Result:         "pass",
		ProviderTaskID: "task-video-pass",
	}))
	require.Equal(t, "banned", reviewContentStatus(t, db, contentID), "banned is the AI channel terminal state")
	require.Equal(t, int64(2), reviewRecordsCount(t, db))
}

// A1: the reverse order also ends banned (pass first, then block).
func TestProcessAICallbackPassThenBlockEndsBanned(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType:     "content",
		TargetID:       contentID,
		Result:         "pass",
		ProviderTaskID: "task-pass-1",
	}))
	require.Equal(t, "published", reviewContentStatus(t, db, contentID))

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType:     "content",
		TargetID:       contentID,
		Result:         "block",
		ProviderTaskID: "task-block-1",
	}))
	require.Equal(t, "banned", reviewContentStatus(t, db, contentID))
	require.Equal(t, []string{"ai_violation", "repeat_violation"}, reputationLogReasons(t, db, userID),
		"single block charges ai_violation and, at threshold 1, the repeat violation penalty")
	require.Equal(t, 6, userReputation(t, db, userID))
}

// A1: review first opens a judge case, then pass publishes.
func TestProcessAICallbackReviewThenPassPublishes(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType:     "content",
		TargetID:       contentID,
		Result:         "review",
		ProviderTaskID: "task-review-1",
	}))
	require.Equal(t, "under_review", reviewContentStatus(t, db, contentID))
	require.Equal(t, int64(1), openJudgeCaseCount(t, db, contentID))

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType:     "content",
		TargetID:       contentID,
		Result:         "pass",
		ProviderTaskID: "task-pass-1",
	}))
	require.Equal(t, "published", reviewContentStatus(t, db, contentID))
}

// A1: a review result on banned content is ignored and does not open a
// judge case (the terminal state guards both pass and review branches).
func TestProcessAICallbackReviewCannotReviveBanned(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType: "content",
		TargetID:   contentID,
		Result:     "block",
	}))
	require.Equal(t, "banned", reviewContentStatus(t, db, contentID))

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType:     "content",
		TargetID:       contentID,
		Result:         "review",
		ProviderTaskID: "task-review-1",
	}))
	require.Equal(t, "banned", reviewContentStatus(t, db, contentID))
	require.Equal(t, int64(0), openJudgeCaseCount(t, db, contentID), "no judge case for banned content")
}

// IP channel: block bans the IP and its contents; the duplicate callback is
// short-circuited and the pass result has no revival surface.
func TestProcessAICallbackIPBlockIsIdempotentAndTerminal(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	ipID := seedReviewIP(t, db)
	contentID := seedReviewContent(t, db, userID, &ipID)

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType:     "ip",
		TargetID:       ipID,
		Result:         "block",
		ProviderTaskID: "task-ip-block",
	}))
	require.Equal(t, "banned", reviewIPStatus(t, db, ipID))
	require.Equal(t, "banned", reviewContentStatus(t, db, contentID))
	require.Equal(t, int64(1), reviewRecordsCount(t, db))

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType:     "ip",
		TargetID:       ipID,
		Result:         "block",
		ProviderTaskID: "task-ip-block",
	}))
	require.Equal(t, int64(1), reviewRecordsCount(t, db), "duplicate IP callback must not add a record")

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType:     "ip",
		TargetID:       ipID,
		Result:         "pass",
		ProviderTaskID: "task-ip-pass",
	}))
	require.Equal(t, "banned", reviewIPStatus(t, db, ipID))
	require.Equal(t, "banned", reviewContentStatus(t, db, contentID), "IP pass has no revival surface")
}

// The sync submission path must not write a review record before the scan
// completes (A2: the record is only written by the processing entry point).
func TestSubmitForAIReviewUnconfiguredGreenWritesNoRecord(t *testing.T) {
	_, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)

	svc := NewReviewService(db, nil, &config.Config{}, NewReputationService(db))
	err := svc.SubmitForAIReview(ctx, SubmitReviewInput{
		TargetType:  "content",
		TargetID:    contentID,
		Title:       "sync fixture",
		Description: "no scan available",
	})
	require.ErrorIs(t, err, aliyun.ErrGreenNotConfigured)
	require.Equal(t, int64(0), reviewRecordsCount(t, db), "no record before the scan result is processed")
}

// Human channel: the appeal approval path (handler/admin.go applies
// AppealTargetUpdates directly, outside ProcessAICallback) can still flip a
// banned content back to published.
func TestAppealApprovedCanFlipBannedToPublishedOutsideReviewFlow(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)

	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType: "content",
		TargetID:   contentID,
		Result:     "block",
	}))
	require.Equal(t, "banned", reviewContentStatus(t, db, contentID))

	// This mirrors handler/admin.go appeal resolution: direct Updates with
	// AppealTargetUpdates, never through ProcessAICallback.
	updates := AppealTargetUpdates("content", "approved")
	require.Equal(t, map[string]interface{}{"status": "published"}, updates)
	require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ?", contentID).Updates(updates).Error)
	require.Equal(t, "published", reviewContentStatus(t, db, contentID), "appeal approval overturns the AI ban")
}

// B2: a content cover scanned by ImageModeration with a block result rejects
// publication: the content transitions to banned and the review record keeps
// the block result.
func TestSubmitForAIReviewContentCoverBlockBansContent(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)

	green := &fakeGreenScanner{imageResult: &aliyun.GreenScanResult{Result: "block", Reason: "violent_content"}}
	svc.green = green

	require.NoError(t, svc.SubmitForAIReview(ctx, SubmitReviewInput{
		TargetType:    "content",
		TargetID:      contentID,
		Title:         "cover fixture",
		Description:   "cover block must reject publication",
		CoverImageURL: "https://cdn.example.test/uploads/42/image/poster.png",
	}))

	require.Equal(t, "banned", reviewContentStatus(t, db, contentID), "cover block must reject publication")
	require.Equal(t, []string{"https://cdn.example.test/uploads/42/image/poster.png"}, green.imageCalls,
		"the cover URL must be the only image scanned")
	require.Equal(t, []string{"ai_violation", "repeat_violation"}, reputationLogReasons(t, db, userID))
	require.Equal(t, 6, userReputation(t, db, userID))

	var record model.AIReviewRecord
	require.NoError(t, db.Where("target_type = ? AND target_id = ?", "content", contentID).First(&record).Error)
	require.Equal(t, "block", record.Result, "the merged result must stay block")
}

// B3: an IP cover scanned with a block result bans the IP and its contents.
func TestSubmitForAIReviewIPCoverBlockBansIP(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	ipID := seedReviewIP(t, db)
	contentID := seedReviewContent(t, db, userID, &ipID)

	green := &fakeGreenScanner{imageResult: &aliyun.GreenScanResult{Result: "block", Reason: "violent_content"}}
	svc.green = green

	require.NoError(t, svc.SubmitForAIReview(ctx, SubmitReviewInput{
		TargetType:    "ip",
		TargetID:      ipID,
		Title:         "ip cover fixture",
		Description:   "cover block must reject the ip",
		CoverImageURL: "https://cdn.example.test/uploads/42/ip/cover.png",
	}))

	require.Equal(t, "banned", reviewIPStatus(t, db, ipID), "cover block must reject the IP")
	require.Equal(t, "banned", reviewContentStatus(t, db, contentID), "IP block cascades to its contents")
	require.Equal(t, []string{"https://cdn.example.test/uploads/42/ip/cover.png"}, green.imageCalls)
}

// Decision 6: a cover that is not a platform-verified OSS object is never
// handed to Green; the submission fails with an explicit error and writes no
// record.
func TestSubmitForAIReviewCoverNotPlatformObjectReturnsExplicitError(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)

	green := &fakeGreenScanner{}
	svc.green = green

	err := svc.SubmitForAIReview(ctx, SubmitReviewInput{
		TargetType:    "content",
		TargetID:      contentID,
		Title:         "cover fixture",
		CoverImageURL: "https://evil.example.com/cover.png",
	})
	require.ErrorIs(t, err, ErrCoverNotPlatformOSSObject)
	require.Empty(t, green.imageCalls, "non-platform cover URLs must never reach Green")
	require.Equal(t, int64(0), reviewRecordsCount(t, db), "no record when the cover cannot be scanned")
}

// Regression: submissions without a cover keep the legacy behavior and never
// call the image scanner.
func TestSubmitForAIReviewWithoutCoverKeepsLegacyBehavior(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)

	green := &fakeGreenScanner{}
	svc.green = green

	require.NoError(t, svc.SubmitForAIReview(ctx, SubmitReviewInput{
		TargetType:  "content",
		TargetID:    contentID,
		Title:       "no cover fixture",
		Description: "no cover, no image scan",
	}))
	require.Equal(t, "published", reviewContentStatus(t, db, contentID))
	require.Empty(t, green.imageCalls, "no cover means no image scan")
}

// Decision 1: the video async scan request carries seed from green.seed and
// dataId in the {target_type}:<id> form the callback parser expects.
func TestSubmitForAIReviewVideoScanCarriesSeedAndDataID(t *testing.T) {
	svc, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)

	green := &fakeGreenScanner{}
	svc.green = green

	require.NoError(t, svc.SubmitForAIReview(ctx, SubmitReviewInput{
		TargetType:  "content",
		TargetID:    contentID,
		ContentType: "video",
		Title:       "video fixture",
		Attachments: []AttachmentInput{
			{FileType: "video", OSSKey: "uploads/42/video/v.mp4", MimeType: "video/mp4"},
		},
	}))

	require.Len(t, green.videoCalls, 1)
	require.Equal(t, "seed_test_value", green.videoCalls[0].Seed, "seed must come from green.seed")
	require.Equal(t, fmt.Sprintf("content:%d", contentID), green.videoCalls[0].DataID, "dataId must match the callback parser format")
	require.Equal(t, "https://api.leeppp.online/api/v1/internal/ai-callback", green.videoCalls[0].CallbackURL)
	require.Equal(t, "https://cdn.example.test/uploads/42/video/v.mp4", green.videoCalls[0].VideoURL)
}

// The empty-seed semantic itself lives in the aliyun layer (a callback
// without seed is rejected by buildVideoServiceParams before the request is
// sent; see green_test.go). At the service layer the seed from green.seed is
// simply forwarded, and any scan error propagates out of SubmitForAIReview
// without status side effects or records.
func TestSubmitForAIReviewVideoScanErrorPropagatesWithoutSideEffects(t *testing.T) {
	_, db, _ := setupReviewServiceTest(t)
	ctx := context.Background()

	userID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, userID)

	svc := NewReviewService(db, nil, &config.Config{
		Reputation: config.ReputationConfig{
			RepeatViolationWindowDays: 7, RepeatViolationThreshold: 1, RepeatViolationExtraPenalty: -1,
		},
		Judge: config.JudgeConfig{MinVotesRequired: 3},
		OSS:   config.OSSConfig{Domain: "https://cdn.example.test"},
		Green: config.GreenConfig{CallbackURL: "https://api.leeppp.online/api/v1/internal/ai-callback"},
	}, NewReputationService(db))
	svc.green = &fakeGreenScanner{videoErr: aliyun.ErrGreenSeedRequired}

	err := svc.SubmitForAIReview(ctx, SubmitReviewInput{
		TargetType:  "content",
		TargetID:    contentID,
		ContentType: "video",
		Title:       "video fixture",
		Attachments: []AttachmentInput{
			{FileType: "video", OSSKey: "uploads/42/video/v.mp4", MimeType: "video/mp4"},
		},
	})
	require.ErrorIs(t, err, aliyun.ErrGreenSeedRequired)
	require.Equal(t, "pending", reviewContentStatus(t, db, contentID), "no status transition without a scan")
	require.Equal(t, int64(0), reviewRecordsCount(t, db))
}

// The sync image-only scan seam (#111/#113) must surface the raw Green result
// so callers apply their own environment-specific availability policy, and
// must pass through the scan error unchanged.
func TestReviewImageURL(t *testing.T) {
	ctx := context.Background()

	t.Run("unconfigured green returns ErrGreenNotConfigured", func(t *testing.T) {
		svc := NewReviewService(nil, nil, &config.Config{}, NewReputationService(nil))
		_, err := svc.ReviewImageURL(ctx, "https://cdn.example.test/a.png")
		require.ErrorIs(t, err, aliyun.ErrGreenNotConfigured)
	})

	t.Run("block result is surfaced unchanged", func(t *testing.T) {
		svc, _, _ := setupReviewServiceTest(t)
		scanner := &fakeGreenScanner{imageResult: &aliyun.GreenScanResult{Result: "block", Reason: "sexy"}}
		svc.green = scanner
		result, err := svc.ReviewImageURL(ctx, "https://cdn.example.test/a.png")
		require.NoError(t, err)
		require.Equal(t, "block", result)
		require.Equal(t, []string{"https://cdn.example.test/a.png"}, scanner.imageCalls)
	})

	t.Run("review and pass results are normalized and propagated", func(t *testing.T) {
		svc, _, _ := setupReviewServiceTest(t)
		scanner := &fakeGreenScanner{imageResult: &aliyun.GreenScanResult{Result: "REVIEW"}}
		svc.green = scanner
		result, err := svc.ReviewImageURL(ctx, "https://cdn.example.test/b.png")
		require.NoError(t, err)
		require.Equal(t, "review", result)
		scanner.imageResult = &aliyun.GreenScanResult{Result: "pass"}
		result, err = svc.ReviewImageURL(ctx, "https://cdn.example.test/c.png")
		require.NoError(t, err)
		require.Equal(t, "pass", result)
		require.Equal(t, []string{"https://cdn.example.test/b.png", "https://cdn.example.test/c.png"}, scanner.imageCalls)
	})

	t.Run("scan error is passed through untouched", func(t *testing.T) {
		svc, _, _ := setupReviewServiceTest(t)
		svc.green = &fakeGreenScanner{imageErr: aliyun.ErrGreenNotConfigured}
		_, err := svc.ReviewImageURL(ctx, "https://cdn.example.test/d.png")
		require.ErrorIs(t, err, aliyun.ErrGreenNotConfigured)
	})
}
