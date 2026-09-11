-- Usage guides (SP-16 #447 / spec D4, 2026-09-11): the content-level
-- specifics layer for install/usage guidance. Authors fill it in studio
-- (optionally from an LLM draft, source=llm_assisted); the public
-- GET /api/v1/contents/:id/guide endpoint merges it over the repo-versioned
-- system template and serves the merged view with ETag + s-maxage.
--   * one row per (content_id, locale); requirements/steps are JSON string
--     arrays, notes is free Markdown;
--   * deleting a content cascades its guides;
--   * legacy rows: none — new table, no backfill.
-- Also settles open question §8-4: content_attachments gains a nullable
-- checksum_sha256 column so P5 (#451) can return an integrity checksum with
-- download URLs. Column lands here; writers arrive with the upload flow.
-- Forward-only. This file must not be renumbered.

CREATE TABLE IF NOT EXISTS content_usage_guides (
    id BIGSERIAL PRIMARY KEY,
    content_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    locale VARCHAR(10) NOT NULL,
    requirements JSONB NOT NULL DEFAULT '[]',
    steps JSONB NOT NULL DEFAULT '[]',
    notes TEXT NOT NULL DEFAULT '',
    source VARCHAR(20) NOT NULL DEFAULT 'author',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_content_usage_guides_content_locale UNIQUE (content_id, locale),
    CONSTRAINT content_usage_guides_source_check CHECK (source IN ('author', 'llm_assisted'))
);

CREATE INDEX IF NOT EXISTS idx_content_usage_guides_content ON content_usage_guides(content_id);

ALTER TABLE content_attachments
    ADD COLUMN IF NOT EXISTS checksum_sha256 VARCHAR(64);
