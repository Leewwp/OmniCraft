-- Keep full-text search functional when pg_jieba is absent or installed
-- without registering its jiebacfg text-search configuration. PostgreSQL
-- validates both CASE branches during planning, so the previous trigger could
-- fail even while its fallback branch was logically selected.

CREATE OR REPLACE FUNCTION content_items_search_vector_update() RETURNS trigger AS $$
DECLARE
  search_config REGCONFIG;
BEGIN
  SELECT COALESCE((SELECT cfg.oid::regconfig FROM pg_ts_config cfg WHERE cfg.cfgname = 'jiebacfg'), 'simple'::regconfig) INTO search_config;
  NEW.search_vector :=
    setweight(to_tsvector(search_config, COALESCE(NEW.title, '')), 'A') ||
    setweight(to_tsvector(search_config, COALESCE(NEW.description, '')), 'B') ||
    setweight(to_tsvector(search_config, COALESCE(
      (SELECT string_agg(t.tag, ' ') FROM content_tags t WHERE t.content_item_id = NEW.id),
      ''
    )), 'C');
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION content_tags_search_vector_update() RETURNS trigger AS $$
DECLARE
  search_config REGCONFIG;
BEGIN
  SELECT COALESCE((SELECT cfg.oid::regconfig FROM pg_ts_config cfg WHERE cfg.cfgname = 'jiebacfg'), 'simple'::regconfig) INTO search_config;
  UPDATE content_items
  SET search_vector =
    setweight(to_tsvector(search_config, COALESCE(title, '')), 'A') ||
    setweight(to_tsvector(search_config, COALESCE(description, '')), 'B') ||
    setweight(to_tsvector(search_config, COALESCE(
      (SELECT string_agg(t.tag, ' ') FROM content_tags t WHERE t.content_item_id = content_items.id),
      ''
    )), 'C')
  WHERE id = COALESCE(NEW.content_item_id, OLD.content_item_id);
  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION ip_tags_search_vector_update() RETURNS trigger AS $$
DECLARE
  search_config REGCONFIG;
BEGIN
  SELECT COALESCE((SELECT cfg.oid::regconfig FROM pg_ts_config cfg WHERE cfg.cfgname = 'jiebacfg'), 'simple'::regconfig) INTO search_config;
  UPDATE ips
  SET search_vector =
    setweight(to_tsvector(search_config, COALESCE(name, '')), 'A') ||
    setweight(to_tsvector(search_config, COALESCE(description, '')), 'B') ||
    setweight(to_tsvector(search_config, COALESCE(
      (SELECT string_agg(t.tag, ' ') FROM ip_tags t WHERE t.ip_id = ips.id),
      ''
    )), 'C')
  WHERE id = COALESCE(NEW.ip_id, OLD.ip_id);
  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;
