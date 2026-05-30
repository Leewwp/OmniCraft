CREATE OR REPLACE FUNCTION content_tags_search_vector_update() RETURNS trigger AS $$
BEGIN
  UPDATE content_items SET search_vector =
    CASE WHEN EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_jieba') THEN
      setweight(to_tsvector('jiebacfg', COALESCE(title, '')), 'A') ||
      setweight(to_tsvector('jiebacfg', COALESCE(description, '')), 'B') ||
      setweight(to_tsvector('jiebacfg', COALESCE(
        (SELECT string_agg(t.tag, ' ') FROM content_tags t WHERE t.content_item_id = content_items.id), ''
      )), 'C')
    ELSE
      setweight(to_tsvector('simple', COALESCE(title, '')), 'A') ||
      setweight(to_tsvector('simple', COALESCE(description, '')), 'B') ||
      setweight(to_tsvector('simple', COALESCE(
        (SELECT string_agg(t.tag, ' ') FROM content_tags t WHERE t.content_item_id = content_items.id), ''
      )), 'C')
    END
  WHERE id = COALESCE(NEW.content_item_id, OLD.content_item_id);
  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_content_tags_search_vector
AFTER INSERT OR DELETE OR UPDATE ON content_tags
FOR EACH ROW EXECUTE FUNCTION content_tags_search_vector_update();

CREATE OR REPLACE FUNCTION ip_tags_search_vector_update() RETURNS trigger AS $$
BEGIN
  UPDATE ips SET search_vector =
    CASE WHEN EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_jieba') THEN
      setweight(to_tsvector('jiebacfg', COALESCE(name, '')), 'A') ||
      setweight(to_tsvector('jiebacfg', COALESCE(description, '')), 'B') ||
      setweight(to_tsvector('jiebacfg', COALESCE(
        (SELECT string_agg(t.tag, ' ') FROM ip_tags t WHERE t.ip_id = ips.id), ''
      )), 'C')
    ELSE
      setweight(to_tsvector('simple', COALESCE(name, '')), 'A') ||
      setweight(to_tsvector('simple', COALESCE(description, '')), 'B') ||
      setweight(to_tsvector('simple', COALESCE(
        (SELECT string_agg(t.tag, ' ') FROM ip_tags t WHERE t.ip_id = ips.id), ''
      )), 'C')
    END
  WHERE id = COALESCE(NEW.ip_id, OLD.ip_id);
  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ip_tags_search_vector
AFTER INSERT OR DELETE OR UPDATE ON ip_tags
FOR EACH ROW EXECUTE FUNCTION ip_tags_search_vector_update();
