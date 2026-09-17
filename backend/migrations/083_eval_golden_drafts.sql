-- Eval golden-set drafts (SP-22 E5, wayfinder map #550 / issue #565):
--   * status separates the frozen evaluation set from badcase 反哺 drafts
--     (trace -> golden draft, the self-built Langfuse trace->dataset loop).
--     Existing rows default to 'frozen'; every frozen-set reader keeps its
--     semantics by filtering status = 'frozen' (the dataset checksum of a
--     frozen run must never drift because a draft landed).
--   * source_trace_id records where a draft came from (agent_trace_runs.
--     trace_id); NULL on curated/frozen cases.
-- Forward-only: additive columns, no backfill.
-- This file must not be renumbered.

ALTER TABLE eval_golden_cases
    ADD COLUMN IF NOT EXISTS status VARCHAR(16) NOT NULL DEFAULT 'frozen';
ALTER TABLE eval_golden_cases
    ADD COLUMN IF NOT EXISTS source_trace_id VARCHAR(64);

-- Only two lifecycle states today: frozen (evaluated) and draft (being
-- curated). Retired cases keep using is_active = false.
ALTER TABLE eval_golden_cases
    DROP CONSTRAINT IF EXISTS eval_golden_cases_status_check;
ALTER TABLE eval_golden_cases
    ADD CONSTRAINT eval_golden_cases_status_check CHECK (status IN ('frozen', 'draft'));

-- Draft curation queue (admin evals page); the frozen set is the majority
-- scan path, so a partial index keeps drafts cheap without bloating it.
CREATE INDEX IF NOT EXISTS idx_eval_golden_cases_drafts
    ON eval_golden_cases (created_at DESC) WHERE status = 'draft';
CREATE INDEX IF NOT EXISTS idx_eval_golden_cases_source_trace
    ON eval_golden_cases (source_trace_id) WHERE source_trace_id IS NOT NULL;
