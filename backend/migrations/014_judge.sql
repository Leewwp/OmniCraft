CREATE TABLE judge_questions (
    id              BIGSERIAL PRIMARY KEY,
    content_type    VARCHAR(50) NOT NULL,
    source_case_id  BIGINT,
    question_data   JSONB NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_by      VARCHAR(20) NOT NULL DEFAULT 'admin',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_judge_questions_type_active ON judge_questions(content_type, is_active);

CREATE TABLE judge_cases (
    id              BIGSERIAL PRIMARY KEY,
    target_type     VARCHAR(20) NOT NULL,
    target_id       BIGINT NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'open',
    vote_approve    INT NOT NULL DEFAULT 0,
    vote_reject     INT NOT NULL DEFAULT 0,
    min_votes       INT NOT NULL DEFAULT 20,
    closed_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_judge_cases_status ON judge_cases(status, created_at DESC);
CREATE INDEX idx_judge_cases_target ON judge_cases(target_type, target_id);

CREATE TABLE judge_votes (
    id              BIGSERIAL PRIMARY KEY,
    case_id         BIGINT NOT NULL REFERENCES judge_cases(id) ON DELETE CASCADE,
    judge_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vote            VARCHAR(10) NOT NULL,
    reason          TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(case_id, judge_id)
);

CREATE TABLE judge_reason_votes (
    id                      BIGSERIAL PRIMARY KEY,
    reason_owner_vote_id    BIGINT NOT NULL REFERENCES judge_votes(id) ON DELETE CASCADE,
    voter_id                BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vote_type               VARCHAR(10) NOT NULL CHECK (vote_type IN ('up', 'down')),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(reason_owner_vote_id, voter_id)
);

CREATE INDEX idx_judge_reason_votes_owner ON judge_reason_votes(reason_owner_vote_id);

CREATE TABLE judge_exam_records (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content_type    VARCHAR(50) NOT NULL,
    score           INT NOT NULL,
    total           INT NOT NULL,
    passed          BOOLEAN NOT NULL,
    taken_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_judge_exam_user_type ON judge_exam_records(user_id, content_type, taken_at DESC);

-- Supplement preferred_locale to users (001_users.sql baseline already has it per architecture.md)
ALTER TABLE users ADD COLUMN IF NOT EXISTS preferred_locale VARCHAR(10) NOT NULL DEFAULT 'zh-CN';
