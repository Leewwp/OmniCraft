package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"omnicraft/backend/internal/model"
)

// defaultTraceWriteBatch caps one INSERT statement for trace upserts; the
// async writer already bounds the flush size, this is the hard floor for
// very large flushes (retention catch-ups, replays).
const defaultTraceWriteBatch = 200

// AgentTraceRepository owns the agent trace tables (agent_trace_runs,
// agent_trace_nodes) written by the async batch writer
// (internal/observability/agenttrace) and read by the admin observability
// pages. Writes are idempotent upserts keyed on trace_id / (trace_id,
// node_key) so "RUNNING first, terminal update later" replays safely.
type AgentTraceRepository struct {
	db *gorm.DB
}

func NewAgentTraceRepository(db *gorm.DB) *AgentTraceRepository {
	return &AgentTraceRepository{db: db}
}

// UpsertRuns inserts or updates run rows on trace_id. The update arm covers
// every mutable column so a terminal record replaces a flushed RUNNING row
// completely; rows arriving out of order (terminal before start replay)
// still produce a coherent row because callers send full merged records.
func (r *AgentTraceRepository) UpsertRuns(ctx context.Context, runs []model.AgentTraceRun) error {
	if len(runs) == 0 {
		return nil
	}
	for i := range runs {
		if len(runs[i].RoutingEvents) == 0 {
			runs[i].RoutingEvents = model.JSONB("[]")
		}
		if len(runs[i].Extra) == 0 {
			runs[i].Extra = model.JSONB("{}")
		}
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "trace_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"conversation_id", "message_id", "user_id", "surface", "status",
			"error_code", "error_message", "started_at", "ended_at",
			"duration_ms", "ttft_ms", "model", "answer_kind", "routing_events",
			"prompt_name", "prompt_version", "extra",
		}),
	}).CreateInBatches(&runs, defaultTraceWriteBatch).Error
}

// UpsertNodes inserts or updates node rows on (trace_id, node_key) with the
// same full-row update semantics as UpsertRuns.
func (r *AgentTraceRepository) UpsertNodes(ctx context.Context, nodes []model.AgentTraceNode) error {
	if len(nodes) == 0 {
		return nil
	}
	for i := range nodes {
		if len(nodes[i].Extra) == 0 {
			nodes[i].Extra = model.JSONB("{}")
		}
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "trace_id"}, {Name: "node_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"parent_node_key", "depth", "node_type", "node_name", "status",
			"error_code", "error_message", "started_at", "ended_at",
			"duration_ms", "model", "prompt_digest", "completion_digest",
			"tokens_in", "tokens_out", "cost_estimate", "extra",
		}),
	}).CreateInBatches(&nodes, defaultTraceWriteBatch).Error
}

// PurgeBefore enforces observability.agent_trace.retention_days: nodes first
// (they hold no FK but belong to runs), then runs. Returns rows removed.
func (r *AgentTraceRepository) PurgeBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	nodes := r.db.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Delete(&model.AgentTraceNode{})
	if nodes.Error != nil {
		return 0, nodes.Error
	}
	runs := r.db.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Delete(&model.AgentTraceRun{})
	if runs.Error != nil {
		return nodes.RowsAffected, runs.Error
	}
	return nodes.RowsAffected + runs.RowsAffected, nil
}
