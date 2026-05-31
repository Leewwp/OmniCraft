CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_content_tags_tag_trgm
  ON content_tags USING gin (tag gin_trgm_ops);
