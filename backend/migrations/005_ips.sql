CREATE TABLE ips (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(255) UNIQUE NOT NULL,
    description     TEXT,
    cover_url       TEXT,
    category        VARCHAR(50),
    creator_id      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ips_status ON ips(status);
CREATE INDEX idx_ips_category ON ips(category);
CREATE INDEX idx_ips_name ON ips USING GIN(to_tsvector('simple', name));

CREATE TABLE ip_review_logs (
    id              BIGSERIAL PRIMARY KEY,
    ip_id           BIGINT NOT NULL REFERENCES ips(id) ON DELETE CASCADE,
    reviewer_id     BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action          VARCHAR(20) NOT NULL,
    reason          TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ip_review_logs_ip ON ip_review_logs(ip_id, created_at DESC);

CREATE TABLE ip_tags (
    ip_id           BIGINT NOT NULL REFERENCES ips(id) ON DELETE CASCADE,
    tag             VARCHAR(50) NOT NULL,
    PRIMARY KEY(ip_id, tag)
);
