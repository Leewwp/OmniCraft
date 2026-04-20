CREATE TABLE IF NOT EXISTS agent_conversations (
    id           BIGSERIAL    PRIMARY KEY,
    user_id      BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    context_type VARCHAR(50)  NOT NULL DEFAULT '',
    context_id   BIGINT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_conversations_user ON agent_conversations (user_id, created_at DESC);
