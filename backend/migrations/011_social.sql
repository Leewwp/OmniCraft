CREATE TABLE discussions (
    id              BIGSERIAL PRIMARY KEY,
    ip_id           BIGINT REFERENCES ips(id) ON DELETE CASCADE,
    content_item_id BIGINT REFERENCES content_items(id) ON DELETE CASCADE,
    author_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title           VARCHAR(500) NOT NULL,
    body            TEXT,
    status          VARCHAR(20) NOT NULL DEFAULT 'published',
    view_count      BIGINT NOT NULL DEFAULT 0,
    reply_count     INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_discussions_ip ON discussions(ip_id, updated_at DESC);
CREATE INDEX idx_discussions_author ON discussions(author_id);

CREATE TABLE comments (
    id              BIGSERIAL PRIMARY KEY,
    content_item_id BIGINT REFERENCES content_items(id) ON DELETE CASCADE,
    discussion_id   BIGINT REFERENCES discussions(id) ON DELETE CASCADE,
    parent_id       BIGINT REFERENCES comments(id) ON DELETE CASCADE,
    author_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body            TEXT NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'published',
    like_count      INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_comments_content ON comments(content_item_id);
CREATE INDEX idx_comments_discussion ON comments(discussion_id);
CREATE INDEX idx_comments_author ON comments(author_id);
CREATE INDEX idx_comments_parent ON comments(parent_id);

CREATE TABLE reactions (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type     VARCHAR(20) NOT NULL,
    target_id       BIGINT NOT NULL,
    reaction        VARCHAR(10) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, target_type, target_id)
);

CREATE TABLE favorites (
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(user_id, content_item_id)
);

CREATE TABLE browse_history (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    viewed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, content_item_id)
);

CREATE INDEX idx_browse_history_user ON browse_history(user_id, viewed_at DESC);

CREATE TABLE follows (
    id              BIGSERIAL PRIMARY KEY,
    follower_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type     VARCHAR(20) NOT NULL,
    target_id       BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(follower_id, target_type, target_id)
);

CREATE INDEX idx_follows_follower ON follows(follower_id);
CREATE INDEX idx_follows_target ON follows(target_type, target_id);

CREATE TABLE appeals (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type     VARCHAR(20) NOT NULL,
    target_id       BIGINT NOT NULL,
    reason          TEXT NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    admin_response  TEXT,
    resolved_by     BIGINT REFERENCES users(id) ON DELETE SET NULL,
    resolved_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_appeals_user ON appeals(user_id);
CREATE INDEX idx_appeals_status ON appeals(status);
