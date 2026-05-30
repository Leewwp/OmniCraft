ALTER TABLE content_items
    ADD COLUMN IF NOT EXISTS hot_score DOUBLE PRECISION DEFAULT 0;

ALTER TABLE ips
    ADD COLUMN IF NOT EXISTS popularity_score DOUBLE PRECISION DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_content_items_hot_score
    ON content_items (hot_score DESC NULLS LAST);

CREATE INDEX IF NOT EXISTS idx_ips_popularity_score
    ON ips (popularity_score DESC NULLS LAST);
