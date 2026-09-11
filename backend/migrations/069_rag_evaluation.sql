-- RAG evaluation truth source (rag-minimal-slice T01, issue #136):
--   * eval_golden_cases holds the hand-annotated golden set. PostgreSQL is
--     the single source of truth for the full dataset; the committed CI
--     fixture (backend/testdata/rag_golden_cases.json) is a deterministic
--     export of a fixed subset, never a second hand-maintained copy.
--     Structured payloads are jsonb so evidence spans, expected citations,
--     answer rubrics and classification survive round-trips byte-for-byte.
--   * relevant_evidence entries are (content_id, content_version,
--     source_start, source_end) spans resolved to chunk keys under the
--     current chunking_version at eval time (rag design §6).
--   * classification carries the content type (mod/guide/article/...) and
--     the query popularity band (cold/hot/normal); the cold share of the
--     full set must stay >= 20%.
--   * eval_runs records one row per evaluated retriever snapshot: dataset
--     checksum, retriever/chunking/index versions, metric snapshot and the
--     exported artifact path. run_key is unique so re-running the same
--     baseline is an idempotent upsert instead of unbounded history.
-- Forward-only: existing rows keep their payloads; no backfill needed.
-- This file must not be renumbered.

CREATE TABLE IF NOT EXISTS eval_golden_cases (
    id                    BIGSERIAL PRIMARY KEY,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    case_key              VARCHAR(128) NOT NULL,
    schema_version        INTEGER     NOT NULL DEFAULT 1,
    query                 TEXT        NOT NULL,
    query_language        VARCHAR(16) NOT NULL,
    viewer_context        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    relevant_evidence     JSONB       NOT NULL DEFAULT '[]'::jsonb,
    relevant_content_ids  JSONB       NOT NULL DEFAULT '[]'::jsonb,
    expected_citations    JSONB       NOT NULL DEFAULT '[]'::jsonb,
    forbidden_content_ids JSONB       NOT NULL DEFAULT '[]'::jsonb,
    answer_rubric         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    classification        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    is_active             BOOLEAN     NOT NULL DEFAULT TRUE
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_eval_golden_cases_case_key
    ON eval_golden_cases (case_key);

CREATE TABLE IF NOT EXISTS eval_runs (
    id                BIGSERIAL PRIMARY KEY,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    run_key           VARCHAR(128) NOT NULL,
    dataset_checksum  VARCHAR(64) NOT NULL,
    retriever_version VARCHAR(64) NOT NULL,
    chunking_version  VARCHAR(64) NOT NULL,
    index_version     VARCHAR(64) NOT NULL,
    metrics           JSONB       NOT NULL DEFAULT '{}'::jsonb,
    environment       JSONB       NOT NULL DEFAULT '{}'::jsonb,
    artifact_path     TEXT        NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_eval_runs_run_key
    ON eval_runs (run_key);

-- ROLLBACK:
-- Local test databases only. Do not run after this migration reaches a shared environment.
-- DROP TABLE IF EXISTS eval_runs;
-- DROP TABLE IF EXISTS eval_golden_cases;
