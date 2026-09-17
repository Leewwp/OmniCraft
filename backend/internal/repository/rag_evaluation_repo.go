package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"omnicraft/backend/internal/model"
)

// ErrRagEvalNotFound is returned by GetEvalRunByKey when no run row exists
// for the requested run key.
var ErrRagEvalNotFound = errors.New("rag eval run not found")

// RagEvaluationRepository owns exactly the two evaluation tables
// (eval_golden_cases, eval_runs). It never touches production content or
// search behaviour: PostgreSQL is the single source of truth for golden sets
// and eval run records, and retrieval runs read content through the existing
// production repositories.
type RagEvaluationRepository struct {
	db *gorm.DB
}

func NewRagEvaluationRepository(db *gorm.DB) *RagEvaluationRepository {
	return &RagEvaluationRepository{db: db}
}

// ListGoldenCases returns every golden case ordered by case_key, the
// canonical deterministic order used by exports and eval runs.
func (r *RagEvaluationRepository) ListGoldenCases(ctx context.Context) ([]model.EvalGoldenCase, error) {
	var cases []model.EvalGoldenCase
	err := r.db.WithContext(ctx).Order("case_key ASC").Find(&cases).Error
	return cases, err
}

// ListActiveGoldenCases returns only the frozen evaluation set
// (is_active=true AND status='frozen') in canonical case_key order. The
// status filter is load-bearing: drafts landing mid-flight must never drift
// a frozen run's dataset checksum (SP-22 E5).
func (r *RagEvaluationRepository) ListActiveGoldenCases(ctx context.Context) ([]model.EvalGoldenCase, error) {
	var cases []model.EvalGoldenCase
	err := r.db.WithContext(ctx).
		Where("is_active = ? AND status = ?", true, "frozen").
		Order("case_key ASC").
		Find(&cases).Error
	return cases, err
}

// ListGoldenDrafts returns the curation queue (status='draft', newest
// first) for the admin evals page.
func (r *RagEvaluationRepository) ListGoldenDrafts(ctx context.Context, limit int) ([]model.EvalGoldenCase, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var cases []model.EvalGoldenCase
	err := r.db.WithContext(ctx).
		Where("status = ?", "draft").
		Order("created_at DESC").
		Limit(limit).
		Find(&cases).Error
	return cases, err
}

// CreateGoldenDraft inserts a draft case. On case_key conflict (the same
// trace fed twice) the existing draft is kept untouched and returned —
// the feedback button is idempotent by design.
func (r *RagEvaluationRepository) CreateGoldenDraft(ctx context.Context, c *model.EvalGoldenCase) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "case_key"}},
		DoNothing: true,
	}).Create(c).Error
}

// GetGoldenCaseByKey returns one case regardless of status.
func (r *RagEvaluationRepository) GetGoldenCaseByKey(ctx context.Context, caseKey string) (*model.EvalGoldenCase, error) {
	var c model.EvalGoldenCase
	err := r.db.WithContext(ctx).Where("case_key = ?", caseKey).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRagEvalNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// DeleteGoldenDraft removes a draft case. Frozen cases are rejected: the
// frozen set is retired via is_active flips in the curation flow, never by
// a delete from the feedback queue.
func (r *RagEvaluationRepository) DeleteGoldenDraft(ctx context.Context, caseKey string) error {
	res := r.db.WithContext(ctx).
		Where("case_key = ? AND status = ?", caseKey, "draft").
		Delete(&model.EvalGoldenCase{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrRagEvalNotFound
	}
	return nil
}

// ListEvalRuns returns eval run rows newest first for the trend page.
func (r *RagEvaluationRepository) ListEvalRuns(ctx context.Context, limit int) ([]model.EvalRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var runs []model.EvalRun
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Find(&runs).Error
	return runs, err
}

// UpsertGoldenCase inserts a golden case or refreshes it on case_key
// conflict, making seed re-runs idempotent. status/source_trace_id are
// deliberately outside the update set: seeding never flips a draft back to
// frozen, and a re-import never erases a draft's trace origin.
func (r *RagEvaluationRepository) UpsertGoldenCase(ctx context.Context, c *model.EvalGoldenCase) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "case_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"schema_version", "query", "query_language", "viewer_context",
			"relevant_evidence", "relevant_content_ids", "expected_citations",
			"forbidden_content_ids", "answer_rubric", "classification", "is_active",
		}),
	}).Create(c).Error
}

// UpsertEvalRun inserts an eval run or refreshes it on run_key conflict so
// re-running the same baseline never duplicates history.
func (r *RagEvaluationRepository) UpsertEvalRun(ctx context.Context, run *model.EvalRun) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "run_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"dataset_checksum", "retriever_version", "chunking_version",
			"index_version", "metrics", "environment", "artifact_path",
		}),
	}).Create(run).Error
}

// GetEvalRunByKey returns the run row for a run key.
func (r *RagEvaluationRepository) GetEvalRunByKey(ctx context.Context, runKey string) (*model.EvalRun, error) {
	var run model.EvalRun
	err := r.db.WithContext(ctx).Where("run_key = ?", runKey).First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRagEvalNotFound
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}
