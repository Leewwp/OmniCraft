-- Migration 003: reputation_logs table
CREATE TABLE IF NOT EXISTS reputation_logs (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    delta           INT NOT NULL,
    reason          VARCHAR(100) NOT NULL,
    related_id      BIGINT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reputation_logs_user ON reputation_logs(user_id, created_at DESC);
