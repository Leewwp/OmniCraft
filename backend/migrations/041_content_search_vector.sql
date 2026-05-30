CREATE EXTENSION IF NOT EXISTS pg_jieba SCHEMA public;

ALTER TABLE content_items ADD COLUMN IF NOT EXISTS search_vector tsvector;

CREATE INDEX IF NOT EXISTS idx_content_items_search_vector ON content_items USING GIN (search_vector);

CREATE OR REPLACE FUNCTION content_items_search_vector_update() RETURNS trigger AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_jieba') THEN
    NEW.search_vector :=
      setweight(to_tsvector('jiebacfg', COALESCE(NEW.title, '')), 'A') ||
      setweight(to_tsvector('jiebacfg', COALESCE(NEW.description, '')), 'B') ||
      setweight(to_tsvector('jiebacfg', COALESCE(
        (SELECT string_agg(t.tag, ' ') FROM content_tags t WHERE t.content_item_id = NEW.id),
        ''
      )), 'C');
  ELSE
    NEW.search_vector :=
      setweight(to_tsvector('simple', COALESCE(NEW.title, '')), 'A') ||
      setweight(to_tsvector('simple', COALESCE(NEW.description, '')), 'B') ||
      setweight(to_tsvector('simple', COALESCE(
        (SELECT string_agg(t.tag, ' ') FROM content_tags t WHERE t.content_item_id = NEW.id),
        ''
      )), 'C');
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS content_items_search_vector_trigger ON content_items;
CREATE TRIGGER content_items_search_vector_trigger
  BEFORE INSERT OR UPDATE OF title, description ON content_items
  FOR EACH ROW EXECUTE FUNCTION content_items_search_vector_update();

UPDATE content_items SET search_vector =
  CASE WHEN EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_jieba') THEN
    setweight(to_tsvector('jiebacfg', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('jiebacfg', COALESCE(description, '')), 'B') ||
    setweight(to_tsvector('jiebacfg', COALESCE(
      (SELECT string_agg(ct.tag, ' ') FROM content_tags ct WHERE ct.content_item_id = content_items.id),
      ''
    )), 'C')
  ELSE
    setweight(to_tsvector('simple', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('simple', COALESCE(description, '')), 'B') ||
    setweight(to_tsvector('simple', COALESCE(
      (SELECT string_agg(ct.tag, ' ') FROM content_tags ct WHERE ct.content_item_id = content_items.id),
      ''
    )), 'C')
  END
WHERE search_vector IS NULL;
