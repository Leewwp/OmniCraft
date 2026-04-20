CREATE TABLE IF NOT EXISTS follows (
    id          BIGSERIAL    PRIMARY KEY,
    follower_id BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type VARCHAR(20)  NOT NULL CHECK (target_type IN ('user','ip')),
    target_id   BIGINT       NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (follower_id, target_type, target_id)
);

CREATE INDEX IF NOT EXISTS idx_follows_follower ON follows (follower_id);
CREATE INDEX IF NOT EXISTS idx_follows_target ON follows (target_type, target_id);
