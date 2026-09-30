-- #728: auto-generated usage-guide cache (independent of author rows).
--   * content_usage_guides keeps author-confirmed rows only (Upsert fully
--     overwrites); auto results MUST NOT mix in, hence a separate table.
--   * Invalidation key: input fingerprint (title/description/type; focus
--     derives from the type, so hashing it adds nothing)
--     + prompt registry version; content_updated_at guards stale writers
--     (an older task never overwrites a newer cache row).
CREATE TABLE content_usage_guide_cache (
    id BIGSERIAL PRIMARY KEY,
    content_id BIGINT NOT NULL,
    locale VARCHAR(8) NOT NULL,
    prompt_version INT NOT NULL,
    input_fingerprint VARCHAR(64) NOT NULL,
    content_updated_at TIMESTAMPTZ NOT NULL,
    guide_markdown TEXT NOT NULL,
    source VARCHAR(16) NOT NULL DEFAULT 'auto',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_usage_guide_cache_content_locale UNIQUE (content_id, locale)
);
