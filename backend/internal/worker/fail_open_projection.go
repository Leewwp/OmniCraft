package worker

import (
	"context"
	"log/slog"
)

// FailOpenProjection is the #787 fail-open gate for stacks without OpenSearch
// (lean deployment, rag.index.enabled=false): SyncContent logs one structured
// skip line and returns nil, so the indexer ACKs the event — no broker
// retries, no dead-letter growth. Embedding side effects (pgvector, DB-only)
// keep running; only the lexical-index projection is skipped. The full-infra
// default never wires this wrapper and keeps the real projection's Health
// gate + retry + DLQ semantics unchanged.
type FailOpenProjection struct{}

// NewFailOpenProjection returns the fail-open no-op ContentProjection.
func NewFailOpenProjection() *FailOpenProjection { return &FailOpenProjection{} }

// SyncContent implements ContentProjection by skipping the projection.
func (p *FailOpenProjection) SyncContent(ctx context.Context, contentID int64) error {
	slog.WarnContext(ctx, "indexer: content projection skipped (rag.index.enabled=false)",
		"content_id", contentID)
	return nil
}
