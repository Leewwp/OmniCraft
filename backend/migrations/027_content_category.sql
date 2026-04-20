ALTER TABLE content_items ADD COLUMN IF NOT EXISTS category VARCHAR(50) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_content_items_category ON content_items (category) WHERE category <> '';
