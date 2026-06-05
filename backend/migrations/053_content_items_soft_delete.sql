ALTER TABLE content_items
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_content_items_deleted_at
    ON content_items (deleted_at);
