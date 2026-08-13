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

// ListActiveGoldenCases returns only is_active=true cases in canonical
// case_key order.
func (r *RagEvaluationRepository) ListActiveGoldenCases(ctx context.Context) ([]model.EvalGoldenCase, error) {
	var cases []model.EvalGoldenCase
	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("case_key ASC").
		Find(&cases).Error
	return cases, err
}

// UpsertGoldenCase inserts a golden case or refreshes it on case_key
// conflict, making seed re-runs idempotent.
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
