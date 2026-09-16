package repository

import (
	"context"
	"sort"
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

// GetRunByTraceID returns the single run row of one trace.
func (r *AgentTraceRepository) GetRunByTraceID(ctx context.Context, traceID string) (*model.AgentTraceRun, error) {
	var row model.AgentTraceRun
	err := r.db.WithContext(ctx).Where("trace_id = ?", traceID).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ListNodesByTrace returns every node of one trace in start order (the
// waterfall ordering).
func (r *AgentTraceRepository) ListNodesByTrace(ctx context.Context, traceID string) ([]model.AgentTraceNode, error) {
	var rows []model.AgentTraceNode
	err := r.db.WithContext(ctx).
		Where("trace_id = ?", traceID).
		Order("started_at ASC, id ASC").
		Find(&rows).Error
	return rows, err
}

// AgentTraceRunFilter is the admin list query surface (SP-21 T3). Zero
// values skip the filter; Page defaults to 1, PageSize to 20 capped at 100.
type AgentTraceRunFilter struct {
	TraceID        string
	ConversationID *int64
	UserID         *int64
	Model          string
	Status         string
	AnswerKind     string
	Surface        string
	From           *time.Time
	To             *time.Time
	Page           int
	PageSize       int
}

func (f AgentTraceRunFilter) page() (int, int) {
	page := f.Page
	if page < 1 {
		page = 1
	}
	size := f.PageSize
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

func (r *AgentTraceRepository) applyRunFilter(db *gorm.DB, f AgentTraceRunFilter) *gorm.DB {
	if f.TraceID != "" {
		db = db.Where("trace_id = ?", f.TraceID)
	}
	if f.ConversationID != nil {
		db = db.Where("conversation_id = ?", *f.ConversationID)
	}
	if f.UserID != nil {
		db = db.Where("user_id = ?", *f.UserID)
	}
	if f.Model != "" {
		db = db.Where("model = ?", f.Model)
	}
	if f.Status != "" {
		db = db.Where("status = ?", f.Status)
	}
	if f.AnswerKind != "" {
		db = db.Where("answer_kind = ?", f.AnswerKind)
	}
	if f.Surface != "" {
		db = db.Where("surface = ?", f.Surface)
	}
	if f.From != nil {
		db = db.Where("started_at >= ?", *f.From)
	}
	if f.To != nil {
		db = db.Where("started_at < ?", *f.To)
	}
	return db
}

// ListRuns returns one page of runs (newest first) plus the unpaginated
// total under the same filter.
func (r *AgentTraceRepository) ListRuns(ctx context.Context, f AgentTraceRunFilter) ([]model.AgentTraceRun, int64, error) {
	page, size := f.page()
	var total int64
	if err := r.applyRunFilter(r.db.WithContext(ctx).Model(&model.AgentTraceRun{}), f).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.AgentTraceRun
	err := r.applyRunFilter(r.db.WithContext(ctx), f).
		Order("started_at DESC, id DESC").
		Offset((page - 1) * size).Limit(size).
		Find(&rows).Error
	return rows, total, err
}

// AgentTraceStats aggregates the GLOBAL window (not the current page, the
// polyu shortcoming): success rate over terminal runs, mean duration,
// P95 duration (computed in Go over a bounded recent sample — portable
// across PostgreSQL and the sqlite test harness), mean TTFT and the model
// fallback rate (any routing event present).
type AgentTraceStats struct {
	TotalRuns        int64   `json:"total_runs"`
	TerminalRuns     int64   `json:"terminal_runs"`
	SuccessRuns      int64   `json:"success_runs"`
	SuccessRate      float64 `json:"success_rate"`
	AvgDurationMs    float64 `json:"avg_duration_ms"`
	P95DurationMs    float64 `json:"p95_duration_ms"`
	AvgTTFTMs        float64 `json:"avg_ttft_ms"`
	TTFTRuns         int64   `json:"ttft_runs"`
	RoutingFallbacks int64   `json:"routing_fallbacks"`
	RoutingRate      float64 `json:"routing_rate"`
}

// statsSampleCap bounds the duration sample pulled for the Go-side P95.
const statsSampleCap = 5000

func (r *AgentTraceRepository) Stats(ctx context.Context, from, to *time.Time) (AgentTraceStats, error) {
	scope := r.db.WithContext(ctx).Model(&model.AgentTraceRun{})
	if from != nil {
		scope = scope.Where("started_at >= ?", *from)
	}
	if to != nil {
		scope = scope.Where("started_at < ?", *to)
	}
	var agg struct {
		Total     int64
		Terminal  int64
		Success   int64
		AvgDur    *float64
		AvgTTFT   *float64
		TTFTRuns  int64
		Fallbacks int64
	}
	err := scope.Select(`
		COUNT(*) AS total,
		SUM(CASE WHEN status <> 'RUNNING' THEN 1 ELSE 0 END) AS terminal,
		SUM(CASE WHEN status = 'SUCCESS' THEN 1 ELSE 0 END) AS success,
		AVG(duration_ms) AS avg_dur,
		AVG(ttft_ms) AS avg_ttft,
		SUM(CASE WHEN ttft_ms IS NOT NULL THEN 1 ELSE 0 END) AS ttft_runs,
		SUM(CASE WHEN routing_events IS NOT NULL AND routing_events <> '[]' THEN 1 ELSE 0 END) AS fallbacks`).
		Scan(&agg).Error
	if err != nil {
		return AgentTraceStats{}, err
	}
	stats := AgentTraceStats{
		TotalRuns:        agg.Total,
		TerminalRuns:     agg.Terminal,
		SuccessRuns:      agg.Success,
		AvgTTFTMs:        derefFloat(agg.AvgTTFT),
		TTFTRuns:         agg.TTFTRuns,
		RoutingFallbacks: agg.Fallbacks,
	}
	if agg.Terminal > 0 {
		stats.SuccessRate = float64(agg.Success) / float64(agg.Terminal)
	}
	if agg.Total > 0 {
		stats.RoutingRate = float64(agg.Fallbacks) / float64(agg.Total)
	}
	sampleScope := r.db.WithContext(ctx).Model(&model.AgentTraceRun{}).
		Where("duration_ms IS NOT NULL")
	if from != nil {
		sampleScope = sampleScope.Where("started_at >= ?", *from)
	}
	if to != nil {
		sampleScope = sampleScope.Where("started_at < ?", *to)
	}
	var durations []float64
	if err := sampleScope.Order("started_at DESC").Limit(statsSampleCap).
		Pluck("duration_ms", &durations).Error; err != nil {
		return AgentTraceStats{}, err
	}
	stats.AvgDurationMs = derefFloat(agg.AvgDur)
	stats.P95DurationMs = percentile95(durations)
	return stats, nil
}

func derefFloat(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// percentile95 sorts a copy and interpolates the 95th percentile
// (nearest-rank with linear interpolation; empty sample = 0).
func percentile95(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	rank := 0.95 * float64(len(sorted)-1)
	lo := int(rank)
	hi := lo + 1
	if hi >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := rank - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
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
