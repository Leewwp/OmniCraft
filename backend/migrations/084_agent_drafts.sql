-- Agent workspace draft store (SP-23 M2, wayfinder map #551 / issue #567):
-- the document-editing MCP server (draft.read / draft.suggest /
-- draft.apply_edit) operates on these server-side drafts. Platform drafts in
-- the publish form today live in browser localStorage; this table is the
-- agent-editable counterpart, owned per user, never anonymous.
-- Forward-only: new table, no backfill.
-- This file must not be renumbered.

CREATE TABLE IF NOT EXISTS agent_drafts (
    id           BIGSERIAL PRIMARY KEY,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id      BIGINT NOT NULL,
    title        VARCHAR(200) NOT NULL DEFAULT '',
    content_type VARCHAR(20) NOT NULL DEFAULT 'article',
    body         TEXT NOT NULL DEFAULT '',
    tags         JSONB NOT NULL DEFAULT '[]'::jsonb,
    status       VARCHAR(16) NOT NULL DEFAULT 'active',
    CONSTRAINT agent_drafts_status_check CHECK (status IN ('active', 'archived'))
);

-- Owner-scoped listing (the MCP server only ever sees the bound user).
CREATE INDEX IF NOT EXISTS idx_agent_drafts_user
    ON agent_drafts (user_id, updated_at DESC) WHERE status = 'active';
