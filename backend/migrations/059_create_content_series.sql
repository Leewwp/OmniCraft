BEGIN;

CREATE TABLE IF NOT EXISTS content_series (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    cover_content_id BIGINT REFERENCES content_items(id) ON DELETE SET NULL,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    zone VARCHAR(10) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT content_series_zone_check CHECK (zone IN ('original', 'fanwork')),
    CONSTRAINT content_series_title_not_blank CHECK (length(btrim(title)) > 0)
);

CREATE TABLE IF NOT EXISTS content_series_items (
    id BIGSERIAL PRIMARY KEY,
    series_id BIGINT NOT NULL REFERENCES content_series(id) ON DELETE CASCADE,
    content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    sort_order INT NOT NULL DEFAULT 0,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (series_id, content_item_id)
);

CREATE INDEX IF NOT EXISTS idx_content_series_owner
    ON content_series (owner_id);

CREATE INDEX IF NOT EXISTS idx_series_items_series
    ON content_series_items (series_id, sort_order, id);

CREATE INDEX IF NOT EXISTS idx_series_items_content
    ON content_series_items (content_item_id);

COMMIT;

-- ROLLBACK:
-- Local test databases only. Do not run after this migration reaches a shared environment.
-- DROP TABLE IF EXISTS content_series_items;
-- DROP TABLE IF EXISTS content_series;
