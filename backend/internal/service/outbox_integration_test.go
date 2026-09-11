package service

import (
	"context"
	"encoding/json"
	"errors"
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
	"omnicraft/backend/internal/pkg/events"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/testutil"
)

const (
	integrationTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	integrationTracestate  = "congo=t61rcWkgMzE"
)

// setupOutboxIntegrationReviewService wires an ephemeral Postgres (013 + 068 +
// 070), miniredis, a ReviewService with the transactional outbox attached and
// the repeat-violation threshold at 1.
func setupOutboxIntegrationReviewService(t *testing.T) (*ReviewService, *repository.OutboxRepository, *gorm.DB) {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	createReviewBaseSchema(t, db)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "013_ai_review.sql"))
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "068_add_ai_review_records_task_id.sql"))
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "070_outbox_inbox.sql"))

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
	outbox := repository.NewOutboxRepository(db)
	svc.SetOutboxRepository(outbox)
	return svc, outbox, db
}

// setupOutboxIntegrationContentService wires an ephemeral Postgres (base
// content schema + 070) and a ContentService with the transactional outbox
// attached. rdb stays nil so no cache/redis side effects run.
func setupOutboxIntegrationContentService(t *testing.T) (*ContentService, *repository.OutboxRepository, *gorm.DB) {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	createReviewBaseSchema(t, db)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "070_outbox_inbox.sql"))

	outbox := repository.NewOutboxRepository(db)
	svc := NewContentServiceWithOSS(repository.NewContentRepository(db), nil, nil, nil, nil)
	svc.SetOutboxRepository(outbox)
	return svc, outbox, db
}

// failingOutboxWriter is the fault-injection seam proving that an outbox write
// failure rolls the whole business transaction back: every CreateTx fails.
type failingOutboxWriter struct{}

func (failingOutboxWriter) CreateTx(_ context.Context, _ *gorm.DB, _ *model.OutboxEvent) error {
	return errors.New("injected outbox write failure")
}

func outboxEventRows(t *testing.T, db *gorm.DB) []model.OutboxEvent {
	t.Helper()
	var rows []model.OutboxEvent
	require.NoError(t, db.Order("id ASC").Find(&rows).Error)
	return rows
}

// --- content.published producer (review pass) ---

func TestReviewPassEmitsContentPublishedInSameTransaction(t *testing.T) {
	svc, outbox, db := setupOutboxIntegrationReviewService(t)
	authorID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, authorID)

	require.NoError(t, svc.ProcessAICallback(context.Background(), AICallbackInput{
		TargetType: "content",
		TargetID:   contentID,
		Result:     "pass",
	}))

	require.Equal(t, "published", reviewContentStatus(t, db, contentID))

	rows := outboxEventRows(t, db)
	require.Len(t, rows, 1, "exactly one outbox event for the pass transition")
	event := rows[0]
	require.Equal(t, events.TopicContentPublished, event.EventType)
	require.Equal(t, contentID, event.AggregateID)
	require.Equal(t, model.OutboxStatusPending, event.Status)
	require.Equal(t, events.ContentSchemaVersion, event.SchemaVersion)

	var payload events.ContentEventPayload
	require.NoError(t, json.Unmarshal(event.Payload, &payload))
	require.Equal(t, contentID, payload.ContentID)
	require.Equal(t, authorID, payload.AuthorID)
	require.Equal(t, "published", payload.Status)

	_ = outbox
}

func TestReviewPassCarriesW3CTraceContextIntoOutbox(t *testing.T) {
	svc, _, db := setupOutboxIntegrationReviewService(t)
	authorID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, authorID)

	ctx := events.WithTraceContext(context.Background(), integrationTraceparent, integrationTracestate)
	require.NoError(t, svc.ProcessAICallback(ctx, AICallbackInput{
		TargetType: "content",
		TargetID:   contentID,
		Result:     "pass",
	}))

	rows := outboxEventRows(t, db)
	require.Len(t, rows, 1)
	require.Equal(t, integrationTraceparent, rows[0].Traceparent)
	require.Equal(t, integrationTracestate, rows[0].Tracestate)

	envelope := events.FromOutboxEvent(rows[0])
	require.Equal(t, integrationTraceparent, envelope.Traceparent, "trace context must survive the full service -> outbox round-trip")
	require.Equal(t, integrationTracestate, envelope.Tracestate)
}

func TestReviewPassOnAlreadyPublishedEmitsNoDuplicateEvent(t *testing.T) {
	svc, _, db := setupOutboxIntegrationReviewService(t)
	authorID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, authorID)
	require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ?", contentID).Update("status", "published").Error)

	require.NoError(t, svc.ProcessAICallback(context.Background(), AICallbackInput{
		TargetType: "content",
		TargetID:   contentID,
		Result:     "pass",
	}))

	require.Empty(t, outboxEventRows(t, db), "a pass on already-published content must not emit content.published again")
}

func TestReviewPassRollbackLeavesNoOutboxEvent(t *testing.T) {
	svc, _, db := setupOutboxIntegrationReviewService(t)
	authorID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, authorID)
	svc.SetOutboxRepository(failingOutboxWriter{})

	err := svc.ProcessAICallback(context.Background(), AICallbackInput{
		TargetType: "content",
		TargetID:   contentID,
		Result:     "pass",
	})
	require.Error(t, err, "a failing outbox write must fail the whole review transaction")

	require.Equal(t, "pending", reviewContentStatus(t, db, contentID), "status transition must roll back with the outbox write")
	require.Zero(t, reviewRecordsCount(t, db), "AI review record must roll back with the outbox write")
	require.Empty(t, outboxEventRows(t, db), "no outbox residue after rollback")
}

// --- content.banned producer (review block) ---

func TestReviewBlockEmitsContentBannedInSameTransaction(t *testing.T) {
	svc, _, db := setupOutboxIntegrationReviewService(t)
	authorID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, authorID)

	require.NoError(t, svc.ProcessAICallback(context.Background(), AICallbackInput{
		TargetType: "content",
		TargetID:   contentID,
		Result:     "block",
	}))

	require.Equal(t, "banned", reviewContentStatus(t, db, contentID))

	rows := outboxEventRows(t, db)
	require.Len(t, rows, 1, "exactly one outbox event for the banned transition")
	require.Equal(t, events.TopicContentBanned, rows[0].EventType)
	require.Equal(t, contentID, rows[0].AggregateID)

	var payload events.ContentEventPayload
	require.NoError(t, json.Unmarshal(rows[0].Payload, &payload))
	require.Equal(t, contentID, payload.ContentID)
	require.Equal(t, authorID, payload.AuthorID)
	require.Equal(t, "banned", payload.Status)
}

func TestReviewBlockOnAlreadyBannedEmitsNoDuplicateEvent(t *testing.T) {
	svc, _, db := setupOutboxIntegrationReviewService(t)
	authorID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, authorID)
	require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ?", contentID).Update("status", "banned").Error)

	require.NoError(t, svc.ProcessAICallback(context.Background(), AICallbackInput{
		TargetType: "content",
		TargetID:   contentID,
		Result:     "block",
	}))

	require.Empty(t, outboxEventRows(t, db), "a block on already-banned content must not emit content.banned again")
}

func TestReviewBlockRollbackLeavesNoOutboxEvent(t *testing.T) {
	svc, _, db := setupOutboxIntegrationReviewService(t)
	authorID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, authorID)
	svc.SetOutboxRepository(failingOutboxWriter{})

	err := svc.ProcessAICallback(context.Background(), AICallbackInput{
		TargetType: "content",
		TargetID:   contentID,
		Result:     "block",
	})
	require.Error(t, err)

	require.Equal(t, "pending", reviewContentStatus(t, db, contentID))
	require.Empty(t, outboxEventRows(t, db), "no outbox residue after rollback")
}

// --- content.updated producer (published content edit) ---

func TestContentUpdateEmitsContentUpdatedInSameTransaction(t *testing.T) {
	svc, _, db := setupOutboxIntegrationContentService(t)
	authorID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, authorID)
	require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ?", contentID).Update("status", "published").Error)

	require.NoError(t, svc.UpdateContent(contentID, authorID, map[string]interface{}{"title": "edited title"}))

	var title string
	require.NoError(t, db.Raw(`SELECT title FROM content_items WHERE id = ?`, contentID).Scan(&title).Error)
	require.Equal(t, "edited title", title)

	rows := outboxEventRows(t, db)
	require.Len(t, rows, 1, "exactly one outbox event for the published edit")
	require.Equal(t, events.TopicContentUpdated, rows[0].EventType)
	require.Equal(t, contentID, rows[0].AggregateID)

	var payload events.ContentEventPayload
	require.NoError(t, json.Unmarshal(rows[0].Payload, &payload))
	require.Equal(t, contentID, payload.ContentID)
	require.Equal(t, authorID, payload.AuthorID)
}

func TestContentUpdateRollbackLeavesNoOutboxEvent(t *testing.T) {
	svc, _, db := setupOutboxIntegrationContentService(t)
	authorID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, authorID)
	require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ?", contentID).Update("status", "published").Error)
	svc.SetOutboxRepository(failingOutboxWriter{})

	err := svc.UpdateContent(contentID, authorID, map[string]interface{}{"title": "edited title"})
	require.Error(t, err, "a failing outbox write must fail the whole content update transaction")

	var title string
	require.NoError(t, db.Raw(`SELECT title FROM content_items WHERE id = ?`, contentID).Scan(&title).Error)
	require.Equal(t, "review fixture", title, "content update must roll back with the outbox write")
	require.Empty(t, outboxEventRows(t, db), "no outbox residue after rollback")
}

func TestContentUpdateOnNonPublishedEmitsNoEvent(t *testing.T) {
	svc, _, db := setupOutboxIntegrationContentService(t)
	authorID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, authorID)

	require.NoError(t, svc.UpdateContent(contentID, authorID, map[string]interface{}{"title": "edited title"}))

	require.Empty(t, outboxEventRows(t, db), "edits of non-published content must not emit content.updated")
}

// --- content.deleted producer (published content soft delete) ---

func TestContentDeleteEmitsContentDeletedInSameTransaction(t *testing.T) {
	svc, _, db := setupOutboxIntegrationContentService(t)
	authorID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, authorID)
	require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ?", contentID).Update("status", "published").Error)

	require.NoError(t, svc.DeleteContent(contentID, authorID))

	var deletedAt *time.Time
	require.NoError(t, db.Raw(`SELECT deleted_at FROM content_items WHERE id = ?`, contentID).Scan(&deletedAt).Error)
	require.NotNil(t, deletedAt, "soft delete must be applied in the same transaction")

	rows := outboxEventRows(t, db)
	require.Len(t, rows, 1, "exactly one outbox event for the published delete")
	require.Equal(t, events.TopicContentDeleted, rows[0].EventType)
	require.Equal(t, contentID, rows[0].AggregateID)

	var payload events.ContentEventPayload
	require.NoError(t, json.Unmarshal(rows[0].Payload, &payload))
	require.Equal(t, contentID, payload.ContentID)
	require.Equal(t, authorID, payload.AuthorID)
}

func TestContentDeleteRollbackLeavesNoOutboxEvent(t *testing.T) {
	svc, _, db := setupOutboxIntegrationContentService(t)
	authorID := seedReviewUser(t, db)
	contentID := seedReviewContent(t, db, authorID)
	require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ?", contentID).Update("status", "published").Error)
	svc.SetOutboxRepository(failingOutboxWriter{})

	err := svc.DeleteContent(contentID, authorID)
	require.Error(t, err, "a failing outbox write must fail the whole content delete transaction")

	var deletedCount int64
	require.NoError(t, db.Model(&model.ContentItem{}).Where("id = ? AND deleted_at IS NOT NULL", contentID).Count(&deletedCount).Error)
	require.Zero(t, deletedCount, "soft delete must roll back with the outbox write")
	require.Empty(t, outboxEventRows(t, db), "no outbox residue after rollback")
}
