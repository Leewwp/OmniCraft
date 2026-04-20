CREATE TABLE IF NOT EXISTS tags (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE,
    category    VARCHAR(50)  NOT NULL DEFAULT '',
    usage_count INT          NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tags_category_usage ON tags (category, usage_count DESC);
CREATE INDEX IF NOT EXISTS idx_tags_name_gin ON tags USING GIN (to_tsvector('simple', name));
