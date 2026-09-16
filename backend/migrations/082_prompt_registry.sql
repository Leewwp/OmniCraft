-- Prompt registry (SP-21 T5, wayfinder map #549 / issue #554):
--   * prompt_registry stores immutable (name, version) rows of every prompt
--     template the agent stack renders. Version rows are insert-only: edits
--     create a new version, never mutate an old one, so any production
--     pointer can roll back to an exact historical byte sequence.
--   * prompt_labels stores named pointers (production / staging) from a
--     prompt name to a version. Runtime resolution reads the production
--     pointer; rolling back = pointing the label at an older version with
--     zero code changes (polyu slot-hot-swap semantics plus versioning).
--   * required_placeholders is snapshotted onto each row for audit: the
--     save-time validation contract that produced the row stays inspectable
--     even if the code-side slot definition later evolves.
-- Forward-only: pure addition; v1 rows are seeded from code builtins at
-- process start (Go is the single source of the v1 byte sequence).
-- This file must not be renumbered.

CREATE TABLE IF NOT EXISTS prompt_registry (
    id                    BIGSERIAL PRIMARY KEY,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    name                  VARCHAR(100) NOT NULL,
    version               INT NOT NULL,
    content               TEXT NOT NULL,
    required_placeholders JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by            BIGINT,
    -- Versions are immutable: one row per (name, version), insert-only.
    CONSTRAINT uq_prompt_registry_name_version UNIQUE (name, version)
);

-- Supports version history listings for the admin prompt page.
CREATE INDEX IF NOT EXISTS idx_prompt_registry_name_version
    ON prompt_registry (name, version DESC);

CREATE TABLE IF NOT EXISTS prompt_labels (
    id         BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    name       VARCHAR(100) NOT NULL,
    label      VARCHAR(32) NOT NULL,
    version    INT NOT NULL,
    -- One pointer per (name, label): moving a label is an update, not a new row.
    CONSTRAINT uq_prompt_labels_name_label UNIQUE (name, label),
    -- Only the two known deployment pointers may exist.
    CONSTRAINT prompt_labels_label_check CHECK (label IN ('production', 'staging'))
);

-- Supports runtime production-pointer lookups.
CREATE INDEX IF NOT EXISTS idx_prompt_labels_name_label
    ON prompt_labels (name, label);
