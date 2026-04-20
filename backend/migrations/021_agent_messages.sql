CREATE TABLE IF NOT EXISTS agent_messages (
    id              BIGSERIAL    PRIMARY KEY,
    conversation_id BIGINT       NOT NULL REFERENCES agent_conversations(id) ON DELETE CASCADE,
    role            VARCHAR(20)  NOT NULL CHECK (role IN ('system','user','assistant','tool')),
    content         TEXT,
    tool_calls      JSONB,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_messages_conv ON agent_messages (conversation_id, created_at ASC);
