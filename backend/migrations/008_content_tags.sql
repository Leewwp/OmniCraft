CREATE TABLE content_tags (
    content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    tag             VARCHAR(50) NOT NULL,
    PRIMARY KEY(content_item_id, tag)
);
