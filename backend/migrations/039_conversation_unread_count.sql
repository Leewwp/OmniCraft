ALTER TABLE conversation_participants ADD COLUMN IF NOT EXISTS unread_count INTEGER NOT NULL DEFAULT 0;
