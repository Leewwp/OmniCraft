CREATE TABLE ai_review_records (
    id              BIGSERIAL PRIMARY KEY,
    target_type     VARCHAR(20) NOT NULL,
    target_id       BIGINT NOT NULL,
    provider        VARCHAR(50) NOT NULL DEFAULT 'aliyun',
    result          VARCHAR(20) NOT NULL,
    raw_response    JSONB,
    scanned_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ai_review_target ON ai_review_records(target_type, target_id, scanned_at DESC);
