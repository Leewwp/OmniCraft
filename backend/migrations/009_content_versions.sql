CREATE TABLE content_versions (
    id                  BIGSERIAL PRIMARY KEY,
    content_item_id     BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    parent_version_id   BIGINT REFERENCES content_versions(id) ON DELETE SET NULL,
    author_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version_number      INT NOT NULL,
    storage_type        VARCHAR(10) NOT NULL,
    storage_key         TEXT,
    diff_summary        TEXT,
    status              VARCHAR(20) NOT NULL DEFAULT 'active',
    is_latest           BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(content_item_id, version_number)
);

CREATE INDEX idx_versions_content ON content_versions(content_item_id);
CREATE INDEX idx_versions_status ON content_versions(status);
CREATE INDEX idx_versions_content_latest ON content_versions(content_item_id) WHERE is_latest = TRUE;
