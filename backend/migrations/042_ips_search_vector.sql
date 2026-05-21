-- Add tsvector column for full-text search on ips
ALTER TABLE ips ADD COLUMN IF NOT EXISTS search_vector tsvector;

-- Create GIN index for fast full-text search on IPs
CREATE INDEX IF NOT EXISTS idx_ips_search_vector ON ips USING GIN (search_vector);

-- Create trigger function to automatically update IP search_vector
CREATE OR REPLACE FUNCTION ips_search_vector_update() RETURNS trigger AS $$
BEGIN
  NEW.search_vector :=
    setweight(to_tsvector('simple', coalesce(NEW.name, '')), 'A') ||
    setweight(to_tsvector('simple', coalesce(NEW.description, '')), 'B') ||
    setweight(to_tsvector('simple', coalesce(
      (SELECT string_agg(it.tag, ' ') FROM ip_tags it WHERE it.ip_id = NEW.id),
      ''
    )), 'C');
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS ips_search_vector_trigger ON ips;
CREATE TRIGGER ips_search_vector_trigger
  BEFORE INSERT OR UPDATE OF name, description ON ips
  FOR EACH ROW EXECUTE FUNCTION ips_search_vector_update();

-- Backfill existing IPs
UPDATE ips SET search_vector =
  setweight(to_tsvector('simple', coalesce(name, '')), 'A') ||
  setweight(to_tsvector('simple', coalesce(description, '')), 'B') ||
  setweight(to_tsvector('simple', coalesce(
    (SELECT string_agg(it.tag, ' ') FROM ip_tags it WHERE it.ip_id = ips.id),
    ''
  )), 'C')
WHERE search_vector IS NULL;