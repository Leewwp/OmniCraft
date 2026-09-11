package model

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"omnicraft/backend/internal/testutil"
)

// TestRagEvaluationMigration covers the empty-database upgrade path for 069:
// the eval_golden_cases and eval_runs tables, their column contracts (jsonb
// evidence/metrics payloads, NOT NULL query/runtime keys), the UNIQUE
// case_key / run_key constraints, the idempotent forward-only re-application
// contract and jsonb round-trip fidelity. PostgreSQL is the single source of
// truth for golden sets and eval runs, so the schema must never silently
// truncate or reorder structured payloads.
func TestRagEvaluationMigration(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)

	migration := filepath.Join("..", "..", "migrations", "069_rag_evaluation.sql")
	testutil.ApplyMigrationFile(t, db, migration)
	testutil.ApplyMigrationFile(t, db, migration)

	assertRagEvaluationTables(t, db)

	// --- eval_golden_cases column contract ---
	assertColumn(t, db, "eval_golden_cases", "id", "bigint", false)
	assertColumn(t, db, "eval_golden_cases", "created_at", "timestamp with time zone", false)
	assertColumn(t, db, "eval_golden_cases", "case_key", "character varying", false)
	assertColumn(t, db, "eval_golden_cases", "schema_version", "integer", false)
	assertColumn(t, db, "eval_golden_cases", "query", "text", false)
	assertColumn(t, db, "eval_golden_cases", "query_language", "character varying", false)
	assertColumn(t, db, "eval_golden_cases", "viewer_context", "jsonb", false)
	assertColumn(t, db, "eval_golden_cases", "relevant_evidence", "jsonb", false)
	assertColumn(t, db, "eval_golden_cases", "relevant_content_ids", "jsonb", false)
	assertColumn(t, db, "eval_golden_cases", "expected_citations", "jsonb", false)
	assertColumn(t, db, "eval_golden_cases", "forbidden_content_ids", "jsonb", false)
	assertColumn(t, db, "eval_golden_cases", "answer_rubric", "jsonb", false)
	assertColumn(t, db, "eval_golden_cases", "classification", "jsonb", false)
	assertColumn(t, db, "eval_golden_cases", "is_active", "boolean", false)

	if !testutil.IndexExists(t, db, "eval_golden_cases", "uq_eval_golden_cases_case_key") {
		t.Fatal("expected unique index uq_eval_golden_cases_case_key")
	}

	// --- eval_runs column contract ---
	assertColumn(t, db, "eval_runs", "id", "bigint", false)
	assertColumn(t, db, "eval_runs", "created_at", "timestamp with time zone", false)
	assertColumn(t, db, "eval_runs", "run_key", "character varying", false)
	assertColumn(t, db, "eval_runs", "dataset_checksum", "character varying", false)
	assertColumn(t, db, "eval_runs", "retriever_version", "character varying", false)
	assertColumn(t, db, "eval_runs", "chunking_version", "character varying", false)
	assertColumn(t, db, "eval_runs", "index_version", "character varying", false)
	assertColumn(t, db, "eval_runs", "metrics", "jsonb", false)
	assertColumn(t, db, "eval_runs", "environment", "jsonb", false)
	assertColumn(t, db, "eval_runs", "artifact_path", "text", false)

	if !testutil.IndexExists(t, db, "eval_runs", "uq_eval_runs_run_key") {
		t.Fatal("expected unique index uq_eval_runs_run_key")
	}

	// --- UNIQUE case_key enforcement ---
	caseID := seedRagGoldenCase(t, db, "case-a")
	_ = caseID
	if err := db.Exec(`INSERT INTO eval_golden_cases (case_key, query, query_language) VALUES ('case-a', 'dup', 'zh')`).Error; err == nil {
		t.Fatal("duplicate case_key insert must be rejected by the unique index")
	}

	// --- UNIQUE run_key enforcement ---
	seedRagEvalRun(t, db, "run-a")
	if err := db.Exec(`
		INSERT INTO eval_runs (run_key, dataset_checksum, retriever_version, chunking_version, index_version, artifact_path)
		VALUES ('run-a', 'sha256:abc', 'k', 'c', 'i', '/tmp/a.json')
	`).Error; err == nil {
		t.Fatal("duplicate run_key insert must be rejected by the unique index")
	}

	// --- jsonb round-trip fidelity: stored payloads come back unchanged ---
	var payloads struct {
		ViewerContext    string
		RelevantEvidence string
		AnswerRubric     string
		Classification   string
	}
	if err := db.Raw(`
		SELECT viewer_context::text, relevant_evidence::text, answer_rubric::text, classification::text
		FROM eval_golden_cases WHERE case_key = 'case-a'
	`).Scan(&payloads).Error; err != nil {
		t.Fatalf("read back golden case jsonb payloads: %v", err)
	}
	viewerContext, evidence, rubric, classification := payloads.ViewerContext, payloads.RelevantEvidence, payloads.AnswerRubric, payloads.Classification
	if !json.Valid([]byte(viewerContext)) || !json.Valid([]byte(evidence)) || !json.Valid([]byte(rubric)) || !json.Valid([]byte(classification)) {
		t.Fatalf("jsonb payloads must round-trip as valid JSON: viewer=%q evidence=%q rubric=%q classification=%q", viewerContext, evidence, rubric, classification)
	}

	var metrics string
	if err := db.Raw(`SELECT metrics::text FROM eval_runs WHERE run_key = 'run-a'`).Scan(&metrics).Error; err != nil {
		t.Fatalf("read back eval run metrics: %v", err)
	}
	if !json.Valid([]byte(metrics)) {
		t.Fatalf("eval_runs.metrics must round-trip as valid JSON: %q", metrics)
	}

	// --- is_active defaults to true; schema_version defaults to 1 ---
	var defaults struct {
		IsActive      bool
		SchemaVersion int
	}
	if err := db.Raw(`SELECT is_active, schema_version FROM eval_golden_cases WHERE case_key = 'case-a'`).Scan(&defaults).Error; err != nil {
		t.Fatalf("read back golden case defaults: %v", err)
	}
	if !defaults.IsActive || defaults.SchemaVersion != 1 {
		t.Fatalf("eval_golden_cases defaults = (is_active=%v, schema_version=%d), want (true, 1)", defaults.IsActive, defaults.SchemaVersion)
	}
}

func assertRagEvaluationTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range []string{"eval_golden_cases", "eval_runs"} {
		var exists bool
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema() AND table_name = ?
			)
		`, table).Scan(&exists).Error; err != nil {
			t.Fatalf("lookup table %s: %v", table, err)
		}
		if !exists {
			t.Fatalf("expected table %s to exist after migration 069", table)
		}
	}
}

func assertColumn(t *testing.T, db *gorm.DB, table, column, wantType string, wantNullable bool) {
	t.Helper()
	dataType, nullable := testutil.ColumnMetadata(t, db, table, column)
	if dataType != wantType || nullable != wantNullable {
		t.Fatalf("%s.%s = (%s, nullable=%v), want (%s, nullable=%v)", table, column, dataType, nullable, wantType, wantNullable)
	}
}

func seedRagGoldenCase(t *testing.T, db *gorm.DB, caseKey string) int64 {
	t.Helper()

	var id int64
	if err := db.Raw(`
		INSERT INTO eval_golden_cases (
			case_key, schema_version, query, query_language,
			viewer_context, relevant_evidence, relevant_content_ids,
			expected_citations, forbidden_content_ids, answer_rubric, classification
		) VALUES (?, 1, ?, ?,
			'{"viewer_user_id": 0}'::jsonb,
			'[{"content_id": 1001, "content_version": 1, "source_start": 12, "source_end": 48}]'::jsonb,
			'[1001]'::jsonb,
			'[{"content_id": 1001, "content_version": 1}]'::jsonb,
			'[9999]'::jsonb,
			'{"deterministic_assertions": ["must cite content 1001"], "min_judge_score": 4}'::jsonb,
			'{"content_type": "mod", "popularity": "cold"}'::jsonb
		) RETURNING id
	`, caseKey, "Blender 插件安装教程", "zh").Scan(&id).Error; err != nil {
		t.Fatalf("seed eval_golden_cases row: %v", err)
	}
	return id
}

func seedRagEvalRun(t *testing.T, db *gorm.DB, runKey string) {
	t.Helper()

	if err := db.Exec(`
		INSERT INTO eval_runs (run_key, dataset_checksum, retriever_version, chunking_version, index_version, metrics, environment, artifact_path)
		VALUES (?, 'sha256:dataset', 'keyword-v1', 'none', 'content-search-v1',
			'{"recall_at_10": 0.5}'::jsonb,
			'{"go": "go1.25"}'::jsonb,
			'backend/testdata/rag_eval_baseline.json')
	`, runKey).Error; err != nil {
		t.Fatalf("seed eval_runs row: %v", err)
	}
}
