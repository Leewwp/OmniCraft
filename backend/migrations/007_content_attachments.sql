CREATE TABLE content_attachments (
    id              BIGSERIAL PRIMARY KEY,
    content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    file_type       VARCHAR(30) NOT NULL,
    oss_key         TEXT NOT NULL,
    file_size       BIGINT,
    mime_type       VARCHAR(100),
    duration_sec    INT,
    width           INT,
    height          INT,
    is_primary      BOOLEAN DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_content_attachments_item ON content_attachments(content_item_id);
