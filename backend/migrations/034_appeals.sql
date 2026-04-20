CREATE TABLE IF NOT EXISTS appeals (
    id             BIGSERIAL    PRIMARY KEY,
    user_id        BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type    VARCHAR(20)  NOT NULL CHECK (target_type IN ('content','comment')),
    target_id      BIGINT       NOT NULL,
    reason         TEXT         NOT NULL,
    status         VARCHAR(20)  NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
    admin_response TEXT,
    resolved_by    BIGINT       REFERENCES users(id),
    resolved_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_appeals_user ON appeals (user_id);
CREATE INDEX IF NOT EXISTS idx_appeals_status ON appeals (status);
