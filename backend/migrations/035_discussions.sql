ALTER TABLE discussions
    ADD COLUMN IF NOT EXISTS is_pinned BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE discussions
    ADD COLUMN IF NOT EXISTS last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE discussions
SET last_active_at = COALESCE(last_active_at, updated_at, created_at, NOW());

CREATE INDEX IF NOT EXISTS idx_discussions_ip_last_active
    ON discussions (ip_id, last_active_at DESC);

CREATE INDEX IF NOT EXISTS idx_discussions_author ON discussions (author_id);

CREATE INDEX IF NOT EXISTS idx_discussions_search
    ON discussions USING GIN (to_tsvector('simple', title || ' ' || COALESCE(body, '')));
