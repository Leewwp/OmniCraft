CREATE TABLE pull_requests (
    id                  BIGSERIAL PRIMARY KEY,
    content_item_id     BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    submitter_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    base_version_id     BIGINT NOT NULL REFERENCES content_versions(id),
    proposed_version_id BIGINT REFERENCES content_versions(id),
    status              VARCHAR(20) NOT NULL DEFAULT 'open',
    message             TEXT,
    reject_reason       TEXT,
    resolved_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pr_content ON pull_requests(content_item_id, status);
CREATE INDEX idx_pr_submitter ON pull_requests(submitter_id);

CREATE TABLE content_contributors (
    content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pr_count        INT NOT NULL DEFAULT 1,
    first_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(content_item_id, user_id)
);

CREATE TABLE author_blocklist (
    author_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(author_id, blocked_id)
);
