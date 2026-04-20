CREATE TABLE IF NOT EXISTS discussions (
    id             BIGSERIAL    PRIMARY KEY,
    ip_id          BIGINT       NOT NULL REFERENCES ips(id) ON DELETE CASCADE,
    author_id      BIGINT       NOT NULL REFERENCES users(id),
    title          VARCHAR(500) NOT NULL,
    body           TEXT         NOT NULL,
    is_pinned      BOOLEAN      NOT NULL DEFAULT FALSE,
    reply_count    INT          NOT NULL DEFAULT 0,
    last_active_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_discussions_ip ON discussions (ip_id, last_active_at DESC);
CREATE INDEX IF NOT EXISTS idx_discussions_author ON discussions (author_id);
CREATE INDEX IF NOT EXISTS idx_discussions_search ON discussions USING GIN (to_tsvector('simple', title || ' ' || body));
