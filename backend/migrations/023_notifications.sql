CREATE TABLE IF NOT EXISTS notifications (
    id          BIGSERIAL    PRIMARY KEY,
    user_id     BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        VARCHAR(50)  NOT NULL,
    channel     VARCHAR(20)  NOT NULL CHECK (channel IN ('reply','like','system')),
    title       VARCHAR(500),
    body        TEXT,
    target_type VARCHAR(50),
    target_id   BIGINT,
    sender_id   BIGINT       REFERENCES users(id),
    is_read     BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications (user_id, is_read, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_user_channel ON notifications (user_id, channel, created_at DESC);
