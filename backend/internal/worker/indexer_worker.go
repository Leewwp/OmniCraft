package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"omnicraft/backend/internal/pkg/events"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
)

// Embedder triggers the async re-embedding of one content's text. In cmd/worker
// this is AgentService (publishes a content.embedding job); tests inject a
// counting stub.
type Embedder interface {
	EmbedContentAsync(contentItemID int64, text string)
}

// IndexerWorker consumes the four content event topics
// (content.published/updated/banned/deleted) relayed from the outbox. Each
// event is applied idempotently inside the inbox transaction:
//   - published / updated  -> the content row must exist; the projection side
//     effect (re-embedding) is triggered after the transaction commits. The
//     actual chunk/index projection lands in T04/T05; the embedding job is the
//     real seam already available.
//   - banned / deleted     -> the content's embedding rows are deleted in the
//     same transaction as the inbox completion record.
//
// A permanently failing event (unknown content, schema-invalid envelope)
// exhausts the broker retries into the dead-letter stream, where the admin
// replay endpoint can re-deliver it after the root cause is fixed.
type IndexerWorker struct {
	db            *gorm.DB
	embedder      Embedder
	embeddingRepo *repository.EmbeddingRepository
}

func NewIndexerWorker(db *gorm.DB, embedder Embedder, embeddingRepo *repository.EmbeddingRepository) *IndexerWorker {
	return &IndexerWorker{db: db, embedder: embedder, embeddingRepo: embeddingRepo}
}

// Handle consumes one relayed envelope. The stream message id is irrelevant:
// the idempotency key is the envelope event_id, so a duplicate delivery (same
// event_id, new stream id) is skipped after the first application.
func (w *IndexerWorker) Handle(ctx context.Context, msg queue.Message) error {
	var envelope events.Envelope
	if err := json.Unmarshal(msg.Payload, &envelope); err != nil {
		return fmt.Errorf("indexer: invalid envelope json: %w", err)
	}
	if err := envelope.Validate(); err != nil {
		return fmt.Errorf("indexer: invalid envelope: %w", err)
	}
	if envelope.EventID <= 0 {
		return fmt.Errorf("indexer: envelope without event_id is not replayable")
	}

	slog.Info("indexer: consuming content event",
		"event_id", envelope.EventID, "event_type", envelope.EventType, "content_id", envelope.AggregateID)

	switch envelope.EventType {
	case events.TopicContentPublished, events.TopicContentUpdated:
		var text string
		alreadyConsumed, err := ConsumeInboxTx(ctx, w.db, msg.Group, envelope.EventID, func(ctx context.Context, tx *gorm.DB) error {
			var content struct {
				ID          int64
				Title       string
				Description string
			}
			if err := tx.WithContext(ctx).Table("content_items").First(&content, envelope.AggregateID).Error; err != nil {
				return fmt.Errorf("indexer: load content %d: %w", envelope.AggregateID, err)
			}
			text = content.Title + "\n" + content.Description
			return nil
		})
		if err != nil {
			return err
		}
		if alreadyConsumed {
			slog.Info("indexer: duplicate delivery skipped", "event_id", envelope.EventID, "event_type", envelope.EventType)
			return nil
		}
		// Projection side effect after the inbox commit: re-embedding is
		// fire-and-forget by design (rebuildable projection, ADR 0005).
		if w.embedder != nil {
			w.embedder.EmbedContentAsync(envelope.AggregateID, text)
		}
		return nil

	case events.TopicContentBanned, events.TopicContentDeleted:
		_, err := ConsumeInboxTx(ctx, w.db, msg.Group, envelope.EventID, func(ctx context.Context, tx *gorm.DB) error {
			return w.embeddingRepo.DeleteByContentIDTx(tx, envelope.AggregateID)
		})
		if err != nil {
			return err
		}
		return nil

	default:
		return fmt.Errorf("indexer: unsupported event type %q", envelope.EventType)
	}
}
