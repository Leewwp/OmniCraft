-- Media set metadata contract (media experience #83):
--   * content_items gains nullable cover_width/cover_height so list endpoints
--     can render natural aspect ratios with zero joins;
--   * content_attachments gains nullable sort_order with a unique partial
--     index per content so new media sets have a stable, unique order.
-- Forward-only: legacy rows keep NULL values and stay readable through the
-- deterministic read order (sort_order ASC NULLS LAST, id ASC). The 061
-- version is deliberately absent from this repository (re-numbered to 066);
-- this file must not be renumbered.

ALTER TABLE content_items
    ADD COLUMN IF NOT EXISTS cover_width INT,
    ADD COLUMN IF NOT EXISTS cover_height INT;

ALTER TABLE content_attachments
    ADD COLUMN IF NOT EXISTS sort_order INT;

-- New media content carries a unique per-content sort_order; the partial
-- unique index leaves legacy NULL sort_order rows (multiple NULLs) readable.
CREATE UNIQUE INDEX IF NOT EXISTS uq_content_attachments_item_sort_order
    ON content_attachments (content_item_id, sort_order)
    WHERE sort_order IS NOT NULL;

-- ROLLBACK:
-- Local test databases only. Do not run after this migration reaches a shared environment.
-- ALTER TABLE content_items DROP COLUMN IF EXISTS cover_width, DROP COLUMN IF EXISTS cover_height;
-- ALTER TABLE content_attachments DROP COLUMN IF EXISTS sort_order;
-- DROP INDEX IF EXISTS uq_content_attachments_item_sort_order;
