-- Add tsvector column for full-text search on content_items
ALTER TABLE content_items ADD COLUMN IF NOT EXISTS search_vector tsvector;

-- Create GIN index for fast full-text search
CREATE INDEX IF NOT EXISTS idx_content_items_search_vector ON content_items USING GIN (search_vector);

-- Create trigger function to automatically update search_vector
CREATE OR REPLACE FUNCTION content_items_search_vector_update() RETURNS trigger AS $$
BEGIN
  NEW.search_vector :=
    setweight(to_tsvector('simple', coalesce(NEW.title, '')), 'A') ||
    setweight(to_tsvector('simple', coalesce(NEW.description, '')), 'B') ||
    setweight(to_tsvector('simple', coalesce(
      (SELECT string_agg(t.tag, ' ') FROM content_tags t WHERE t.content_item_id = NEW.id),
      ''
    )), 'C');
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger
DROP TRIGGER IF EXISTS content_items_search_vector_trigger ON content_items;
CREATE TRIGGER content_items_search_vector_trigger
  BEFORE INSERT OR UPDATE OF title, description ON content_items
  FOR EACH ROW EXECUTE FUNCTION content_items_search_vector_update();

-- Backfill existing content_items
UPDATE content_items SET search_vector =
  setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
  setweight(to_tsvector('simple', coalesce(description, '')), 'B') ||
  setweight(to_tsvector('simple', coalesce(
    (SELECT string_agg(ct.tag, ' ') FROM content_tags ct WHERE ct.content_item_id = content_items.id),
    ''
  )), 'C')
WHERE search_vector IS NULL;