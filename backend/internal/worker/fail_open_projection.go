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
//
// #806 A4：reason 区分两种静默跳过（hybrid 关 / index 关），容器侧永远
// 接线非 nil 投影，indexer 不再有 nil 分支语义。
type FailOpenProjection struct {
	reason string
}

// NewFailOpenProjection returns the fail-open no-op ContentProjection; the
// reason lands verbatim in the skip log line (e.g. "rag.index.enabled=false").
func NewFailOpenProjection(reason string) *FailOpenProjection {
	return &FailOpenProjection{reason: reason}
}

// SyncContent implements ContentProjection by skipping the projection.
func (p *FailOpenProjection) SyncContent(ctx context.Context, contentID int64) error {
	slog.WarnContext(ctx, "indexer: content projection skipped ("+p.reason+")",
		"content_id", contentID)
	return nil
}
