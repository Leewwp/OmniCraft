-- Migration 002: judge_qualifications table
CREATE TABLE IF NOT EXISTS judge_qualifications (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content_type    VARCHAR(50) NOT NULL,
    -- content_type: 'article' | 'image' | 'video' | 'audio' | 'prompt' | 'comment' | 'mod'
    --             | 'sheet_music' | 'template' | 'other'
    qualified_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at      TIMESTAMPTZ,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE(user_id, content_type)
);

CREATE INDEX IF NOT EXISTS idx_judge_qual_user ON judge_qualifications(user_id);
CREATE INDEX IF NOT EXISTS idx_judge_qual_type ON judge_qualifications(content_type, is_active);
