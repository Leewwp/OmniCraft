BEGIN;

CREATE TABLE IF NOT EXISTS collections (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    zone VARCHAR(10) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INT NOT NULL DEFAULT 0,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT collections_zone_check CHECK (zone IN ('original', 'fanwork')),
    CONSTRAINT collections_title_not_blank CHECK (length(btrim(title)) > 0)
);

CREATE TABLE IF NOT EXISTS collection_items (
    id BIGSERIAL PRIMARY KEY,
    collection_id BIGINT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    note TEXT NOT NULL DEFAULT '',
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (collection_id, content_item_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_collections_one_default_per_zone
    ON collections (user_id, zone)
    WHERE is_default = TRUE;

CREATE INDEX IF NOT EXISTS idx_collections_user_zone_sort
    ON collections (user_id, zone, sort_order, id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_collection_items_content_item
    ON collection_items (content_item_id);

INSERT INTO collections (user_id, title, zone, is_default, is_public, sort_order)
SELECT users.id, defaults.title, defaults.zone, TRUE, FALSE, 0
FROM users
CROSS JOIN (
    VALUES
        ('original', '默认原创收藏'),
        ('fanwork', '默认二创收藏')
) AS defaults(zone, title)
ON CONFLICT (user_id, zone) WHERE is_default = TRUE DO NOTHING;

INSERT INTO collection_items (collection_id, content_item_id, added_at)
SELECT collections.id, favorites.content_item_id, favorites.created_at
FROM favorites
JOIN content_items ON content_items.id = favorites.content_item_id
JOIN collections
  ON collections.user_id = favorites.user_id
 AND collections.zone = content_items.zone
 AND collections.is_default = TRUE
WHERE content_items.zone IN ('original', 'fanwork')
ON CONFLICT (collection_id, content_item_id) DO NOTHING;

COMMIT;

-- ROLLBACK:
-- Local test databases only. Do not run against shared or production data after release.
-- DELETE FROM collection_items;
-- DELETE FROM collections WHERE is_default = TRUE AND title IN ('默认原创收藏', '默认二创收藏');
-- DROP TABLE IF EXISTS collection_items;
-- DROP TABLE IF EXISTS collections;
-- The legacy favorites table is intentionally retained by this migration.
