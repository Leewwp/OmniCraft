package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// JSONB is a pass-through jsonb value. Unlike JSONMap (object-only) it also
// carries arrays and raw numbers, so it can represent eval evidence spans,
// content id lists and metric snapshots without lossy normalization. It
// round-trips the exact stored bytes, which keeps PostgreSQL the byte-level
// single source of truth for golden sets and eval runs.
type JSONB json.RawMessage

// Value implements driver.Valuer for GORM insert/update paths.
func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "null", nil
	}
	return string(j), nil
}

// Scan implements sql.Scanner for GORM read paths.
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = JSONB("null")
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("cannot scan JSONB from non-string")
	}
	if !json.Valid(bytes) {
		return errors.New("cannot scan invalid JSON into JSONB")
	}
	*j = append((*j)[:0], bytes...)
	return nil
}

// EvalGoldenCase is one hand-annotated evaluation case. PostgreSQL is the
// single source of truth for the full golden set; the committed CI fixture is
// a deterministic export of a fixed subset (rag-minimal-slice T01).
type EvalGoldenCase struct {
	ID                  int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt           time.Time `gorm:"autoCreateTime" json:"created_at"`
	CaseKey             string    `gorm:"size:128;not null;uniqueIndex:uq_eval_golden_cases_case_key" json:"case_key"`
	SchemaVersion       int       `gorm:"not null;default:1" json:"schema_version"`
	Query               string    `gorm:"type:text;not null" json:"query"`
	QueryLanguage       string    `gorm:"size:16;not null" json:"query_language"`
	ViewerContext       JSONB     `gorm:"type:jsonb;not null;default:'{}'" json:"viewer_context"`
	RelevantEvidence    JSONB     `gorm:"type:jsonb;not null;default:'[]'" json:"relevant_evidence"`
	RelevantContentIDs  JSONB     `gorm:"type:jsonb;not null;default:'[]'" json:"relevant_content_ids"`
	ExpectedCitations   JSONB     `gorm:"type:jsonb;not null;default:'[]'" json:"expected_citations"`
	ForbiddenContentIDs JSONB     `gorm:"type:jsonb;not null;default:'[]'" json:"forbidden_content_ids"`
	AnswerRubric        JSONB     `gorm:"type:jsonb;not null;default:'{}'" json:"answer_rubric"`
	Classification      JSONB     `gorm:"type:jsonb;not null;default:'{}'" json:"classification"`
	IsActive            bool      `gorm:"not null;default:true" json:"is_active"`
}

func (EvalGoldenCase) TableName() string { return "eval_golden_cases" }

// EvalRun records one evaluated retriever snapshot (keyword-only baseline,
// vector-only baseline, hybrid RRF, ...) against a dataset checksum. run_key
// is unique so re-running the same baseline upserts instead of growing
// unbounded history.
type EvalRun struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
	RunKey           string    `gorm:"size:128;not null;uniqueIndex:uq_eval_runs_run_key" json:"run_key"`
	DatasetChecksum  string    `gorm:"size:64;not null" json:"dataset_checksum"`
	RetrieverVersion string    `gorm:"size:64;not null" json:"retriever_version"`
	ChunkingVersion  string    `gorm:"size:64;not null" json:"chunking_version"`
	IndexVersion     string    `gorm:"size:64;not null" json:"index_version"`
	Metrics          JSONB     `gorm:"type:jsonb;not null;default:'{}'" json:"metrics"`
	Environment      JSONB     `gorm:"type:jsonb;not null;default:'{}'" json:"environment"`
	ArtifactPath     string    `gorm:"type:text;not null" json:"artifact_path"`
}

func (EvalRun) TableName() string { return "eval_runs" }
