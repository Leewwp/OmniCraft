CREATE TABLE content_items (
    id              BIGSERIAL PRIMARY KEY,
    title           VARCHAR(500) NOT NULL,
    author_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    zone            VARCHAR(10) NOT NULL,
    ip_id           BIGINT REFERENCES ips(id) ON DELETE SET NULL,
    category        VARCHAR(50),
    content_type    VARCHAR(20) NOT NULL,
    cover_image_url TEXT,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    view_count      BIGINT NOT NULL DEFAULT 0,
    like_count      INT NOT NULL DEFAULT 0,
    dislike_count   INT NOT NULL DEFAULT 0,
    is_public       BOOLEAN NOT NULL DEFAULT TRUE,
    allow_copy      BOOLEAN NOT NULL DEFAULT TRUE,
    agent_enabled   BOOLEAN NOT NULL DEFAULT FALSE,
    is_paid         BOOLEAN NOT NULL DEFAULT FALSE,
    price           NUMERIC(10,2) DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_content_items_author ON content_items(author_id);
CREATE INDEX idx_content_items_ip ON content_items(ip_id);
CREATE INDEX idx_content_items_zone ON content_items(zone);
CREATE INDEX idx_content_items_type ON content_items(content_type);
CREATE INDEX idx_content_items_category ON content_items(category);
CREATE INDEX idx_content_items_status ON content_items(status);
