CREATE TABLE IF NOT EXISTS browse_history (
    id              BIGSERIAL   PRIMARY KEY,
    user_id         BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content_item_id BIGINT      NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    viewed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, content_item_id)
);

CREATE INDEX IF NOT EXISTS idx_browse_history_user_time ON browse_history (user_id, viewed_at DESC);
