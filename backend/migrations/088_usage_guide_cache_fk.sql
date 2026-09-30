-- Migration 088: backfill the content foreign key dropped from 087 (#745).
-- Constitution Principle III: foreign keys MUST be declared in DDL; orphaned
-- references must not be tolerated. Mirrors 078's ON DELETE CASCADE so a
-- hard-deleted content item clears its cached auto guides. The orphan sweep
-- first keeps the ALTER applicable to databases that already drifted.
DELETE FROM content_usage_guide_cache AS cache
WHERE NOT EXISTS (
    SELECT 1 FROM content_items AS item WHERE item.id = cache.content_id
);

ALTER TABLE content_usage_guide_cache DROP CONSTRAINT IF EXISTS fk_usage_guide_cache_content;
ALTER TABLE content_usage_guide_cache
    ADD CONSTRAINT fk_usage_guide_cache_content
    FOREIGN KEY (content_id) REFERENCES content_items(id) ON DELETE CASCADE;
