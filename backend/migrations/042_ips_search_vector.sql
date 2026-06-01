ALTER TABLE ips ADD COLUMN IF NOT EXISTS search_vector tsvector;

CREATE INDEX IF NOT EXISTS idx_ips_search_vector ON ips USING GIN (search_vector);

CREATE OR REPLACE FUNCTION ips_search_vector_update() RETURNS trigger AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_jieba') THEN
    NEW.search_vector :=
      setweight(to_tsvector('jiebacfg', COALESCE(NEW.name, '')), 'A') ||
      setweight(to_tsvector('jiebacfg', COALESCE(NEW.description, '')), 'B') ||
      setweight(to_tsvector('jiebacfg', COALESCE(
        (SELECT string_agg(it.tag, ' ') FROM ip_tags it WHERE it.ip_id = NEW.id),
        ''
      )), 'C');
  ELSE
    NEW.search_vector :=
      setweight(to_tsvector('simple', COALESCE(NEW.name, '')), 'A') ||
      setweight(to_tsvector('simple', COALESCE(NEW.description, '')), 'B') ||
      setweight(to_tsvector('simple', COALESCE(
        (SELECT string_agg(it.tag, ' ') FROM ip_tags it WHERE it.ip_id = NEW.id),
        ''
      )), 'C');
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS ips_search_vector_trigger ON ips;
CREATE TRIGGER ips_search_vector_trigger
  BEFORE INSERT OR UPDATE OF name, description ON ips
  FOR EACH ROW EXECUTE FUNCTION ips_search_vector_update();

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_jieba') THEN
    EXECUTE $q$
      UPDATE ips SET search_vector =
        setweight(to_tsvector('jiebacfg', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('jiebacfg', COALESCE(description, '')), 'B') ||
        setweight(to_tsvector('jiebacfg', COALESCE(
          (SELECT string_agg(it.tag, ' ') FROM ip_tags it WHERE it.ip_id = ips.id),
          ''
        )), 'C')
      WHERE search_vector IS NULL
    $q$;
  ELSE
    EXECUTE $q$
      UPDATE ips SET search_vector =
        setweight(to_tsvector('simple', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('simple', COALESCE(description, '')), 'B') ||
        setweight(to_tsvector('simple', COALESCE(
          (SELECT string_agg(it.tag, ' ') FROM ip_tags it WHERE it.ip_id = ips.id),
          ''
        )), 'C')
      WHERE search_vector IS NULL
    $q$;
  END IF;
END $$;
