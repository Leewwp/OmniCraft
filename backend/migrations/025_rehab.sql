CREATE TABLE IF NOT EXISTS rehab_courses (
    id             BIGSERIAL    PRIMARY KEY,
    violation_type VARCHAR(100) NOT NULL UNIQUE,
    content_i18n   JSONB        NOT NULL DEFAULT '{}',
    min_reading_sec INT         NOT NULL DEFAULT 60,
    reward_points  INT          NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS rehab_completions (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id  BIGINT      NOT NULL REFERENCES rehab_courses(id),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, course_id)
);

CREATE INDEX IF NOT EXISTS idx_rehab_completions_user ON rehab_completions (user_id);
