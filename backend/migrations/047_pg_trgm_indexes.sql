CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_content_items_title_trgm
    ON content_items USING GIN (title gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ips_name_trgm
    ON ips USING GIN (name gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_username_trgm
    ON users USING GIN (username gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tags_name_trgm
    ON tags USING GIN (name gin_trgm_ops);
