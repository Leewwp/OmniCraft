-- Fanwork source linkage (source-linkage #96):
--   * content_items gains a nullable source_fanwork_id self-referencing
--     content_items(id), so fanworks can declare the fanwork they derive
--     from (in addition to source_original_id for originals);
--   * the partial index covers fanwork-source lookups filtered by status
--     and sorted by recency, while leaving NULL source rows (originals,
--     standalone fanworks) out of the index.
-- Forward-only: existing rows keep NULL source_fanwork_id. This file must
-- not be renumbered.

ALTER TABLE content_items
  ADD COLUMN IF NOT EXISTS source_fanwork_id BIGINT REFERENCES content_items(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_content_items_source_fanwork
  ON content_items (source_fanwork_id, status, created_at DESC)
  WHERE source_fanwork_id IS NOT NULL;

-- ROLLBACK:
-- Local test databases only. Do not run after this migration reaches a shared environment.
-- DROP INDEX IF EXISTS idx_content_items_source_fanwork;
-- ALTER TABLE content_items DROP COLUMN IF EXISTS source_fanwork_id;
