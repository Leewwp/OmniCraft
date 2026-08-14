package worker

import (
	"context"
	"encoding/json"
	"log/slog"

	"gorm.io/gorm"

	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/service"
)

// EmbeddingWorker computes embeddings for the content.embedding topic. The
// LLM call cannot join the inbox transaction; the embedding upsert is a
// rebuildable projection (ADR 0005), so the completion record is written
// after the effect and a crash re-runs the embedding on redelivery
// (at-least-once).
type EmbeddingWorker struct {
	agentSvc *service.AgentService
	db       *gorm.DB
}

func NewEmbeddingWorker(agentSvc *service.AgentService, db *gorm.DB) *EmbeddingWorker {
	return &EmbeddingWorker{agentSvc: agentSvc, db: db}
}

func (w *EmbeddingWorker) Handle(ctx context.Context, msg queue.Message) error {
	var payload struct {
		ContentID int64  `json:"content_id"`
		Text      string `json:"text"`
	}

	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		slog.Error("embedding_worker: failed to unmarshal payload", "msg_id", msg.ID, "error", err)
		return err
	}

	slog.Info("embedding_worker: processing message",
		"msg_id", msg.ID, "content_id", payload.ContentID)

	if w.agentSvc == nil {
		slog.Warn("embedding_worker: agent service is nil, skipping")
		return nil
	}

	if err := w.agentSvc.EmbedContent(ctx, payload.ContentID, payload.Text); err != nil {
		slog.Error("embedding_worker: failed to embed content",
			"content_id", payload.ContentID, "error", err)
		return err
	}

	slog.Info("embedding_worker: embedding completed",
		"content_id", payload.ContentID)

	if err := MarkConsumedInbox(ctx, w.db, msg.Group, InboxEventID(msg.Group, msg)); err != nil {
		slog.Error("embedding_worker: failed to record inbox completion", "msg_id", msg.ID, "error", err)
		return err
	}
	return nil
}
