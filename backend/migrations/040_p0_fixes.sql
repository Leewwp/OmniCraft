-- P0 fixes: report dedup UNIQUE, ban_reason on content_items, download_count, email_verified_at, left_at
-- Migration 040

-- 1. Reports UNIQUE constraint for dedup
ALTER TABLE reports ADD CONSTRAINT reports_unique_report UNIQUE (reporter_id, target_type, target_id);

-- 2. ban_reason column on content_items (for Task 138)
ALTER TABLE content_items ADD COLUMN IF NOT EXISTS ban_reason TEXT;

-- 3. download_count column on content_items (for Task 124)
ALTER TABLE content_items ADD COLUMN IF NOT EXISTS download_count INTEGER NOT NULL DEFAULT 0;

-- 4. email_verified_at on users (for Task 137)
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;

-- 5. left_at on conversation_participants (for Task 140)
ALTER TABLE conversation_participants ADD COLUMN IF NOT EXISTS left_at TIMESTAMPTZ;

-- 6. Password reset tokens table (for Task 137)
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token ON password_reset_tokens(token);