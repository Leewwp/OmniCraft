package worker

import (
	"context"
	"encoding/json"
	"log/slog"

	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/service"
)

type EmbeddingWorker struct {
	agentSvc *service.AgentService
}

func NewEmbeddingWorker(agentSvc *service.AgentService) *EmbeddingWorker {
	return &EmbeddingWorker{agentSvc: agentSvc}
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
	return nil
}