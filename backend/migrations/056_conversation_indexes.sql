-- 补建私信查询索引（原计划在 035 创建，实际未执行）
-- depends_on: 017_conversations.sql, 039_conversation_unread_count.sql

CREATE INDEX IF NOT EXISTS idx_messages_sender
    ON messages (sender_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_conversations_updated
    ON conversations (updated_at DESC);
