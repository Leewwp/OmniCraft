-- Agent link observability (SP-21 T1, wayfinder map #549 / issue #553):
--   * agent_trace_runs stores one row per agent chat turn keyed by the OTel
--     trace id (the same id the SSE start/done events carry), linked back to
--     conversation / message / user so the admin list page can filter and the
--     conversation view can jump into a trace. Rows are written by the async
--     batch writer, never on the request path: a run starts as RUNNING and is
--     upserted to its terminal state when the turn ends, so a crash keeps the
--     half-written scene (polyu lesson).
--   * agent_trace_nodes stores the phase tree of one run (retrieval, llm
--     rounds, tools, side calls). parent_node_key + depth rebuild the tree
--     for the waterfall view. node_key is the writer-side stable per-trace
--     identity, so "insert RUNNING, update terminal" is one idempotent
--     upsert even when the start row was already flushed. A database id
--     cannot play this role: async upserts never know it.
--   * prompt_digest / completion_digest are truncated server-side
--     (observability.agent_trace.digest_max_runes); full bodies are stored
--     only when keep_full_prompt is explicitly enabled.
--   * No foreign keys by design: trace rows are write-aside telemetry and
--     must not fail on referential drift (deleted conversations, replayed
--     fixtures); the nullable link columns tolerate it.
-- Forward-only: pure addition, no backfill.
-- This file must not be renumbered.

CREATE TABLE IF NOT EXISTS agent_trace_runs (
    id              BIGSERIAL PRIMARY KEY,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    trace_id        VARCHAR(64) NOT NULL,
    conversation_id BIGINT,
    message_id      BIGINT,
    user_id         BIGINT,
    surface         VARCHAR(32) NOT NULL DEFAULT '',
    status          VARCHAR(16) NOT NULL DEFAULT 'RUNNING',
    error_code      VARCHAR(64) NOT NULL DEFAULT '',
    error_message   TEXT NOT NULL DEFAULT '',
    started_at      TIMESTAMPTZ NOT NULL,
    ended_at        TIMESTAMPTZ,
    duration_ms     BIGINT,
    ttft_ms         BIGINT,
    model           VARCHAR(100) NOT NULL DEFAULT '',
    answer_kind     VARCHAR(32) NOT NULL DEFAULT '',
    routing_events  JSONB NOT NULL DEFAULT '[]'::jsonb,
    prompt_name     VARCHAR(100) NOT NULL DEFAULT '',
    prompt_version  INT,
    extra           JSONB NOT NULL DEFAULT '{}'::jsonb,
    -- One row per trace: RUNNING start and terminal update upsert on this key.
    CONSTRAINT uq_agent_trace_runs_trace_id UNIQUE (trace_id),
    -- Terminal states only; RUNNING marks an in-flight or crashed turn.
    CONSTRAINT agent_trace_runs_status_check CHECK (status IN ('RUNNING', 'SUCCESS', 'ERROR', 'CANCELLED'))
);

-- Supports the admin request list default ordering (newest first).
CREATE INDEX IF NOT EXISTS idx_agent_trace_runs_started
    ON agent_trace_runs (started_at DESC);
-- Supports conversation -> trace drill-down from the chat history.
CREATE INDEX IF NOT EXISTS idx_agent_trace_runs_conversation
    ON agent_trace_runs (conversation_id) WHERE conversation_id IS NOT NULL;
-- Supports per-user filtering on the admin request list.
CREATE INDEX IF NOT EXISTS idx_agent_trace_runs_user_started
    ON agent_trace_runs (user_id, started_at DESC) WHERE user_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS agent_trace_nodes (
    id                BIGSERIAL PRIMARY KEY,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    trace_id          VARCHAR(64) NOT NULL,
    node_key          VARCHAR(96) NOT NULL,
    parent_node_key   VARCHAR(96),
    depth             INT NOT NULL DEFAULT 0,
    node_type         VARCHAR(32) NOT NULL,
    node_name         VARCHAR(100) NOT NULL DEFAULT '',
    status            VARCHAR(16) NOT NULL DEFAULT 'RUNNING',
    error_code        VARCHAR(64) NOT NULL DEFAULT '',
    error_message     TEXT NOT NULL DEFAULT '',
    started_at        TIMESTAMPTZ NOT NULL,
    ended_at          TIMESTAMPTZ,
    duration_ms       BIGINT,
    model             VARCHAR(100) NOT NULL DEFAULT '',
    prompt_digest     TEXT NOT NULL DEFAULT '',
    completion_digest TEXT NOT NULL DEFAULT '',
    tokens_in         BIGINT,
    tokens_out        BIGINT,
    cost_estimate     DOUBLE PRECISION,
    extra             JSONB NOT NULL DEFAULT '{}'::jsonb,
    -- Stable per-trace node identity: RUNNING insert + terminal update is one idempotent upsert.
    CONSTRAINT uq_agent_trace_nodes_key UNIQUE (trace_id, node_key),
    -- Terminal states only; RUNNING marks a node whose turn ended before it reported.
    CONSTRAINT agent_trace_nodes_status_check CHECK (status IN ('RUNNING', 'SUCCESS', 'ERROR', 'CANCELLED'))
);

-- Supports the trace detail waterfall: all nodes of one trace in start order.
CREATE INDEX IF NOT EXISTS idx_agent_trace_nodes_trace
    ON agent_trace_nodes (trace_id, started_at);
