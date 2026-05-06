ALTER TABLE content_items
    ADD COLUMN IF NOT EXISTS description TEXT;

ALTER TABLE content_items
    ADD COLUMN IF NOT EXISTS source_original_id BIGINT REFERENCES content_items(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_content_items_source_original
    ON content_items (source_original_id, status, created_at DESC)
    WHERE source_original_id IS NOT NULL;
