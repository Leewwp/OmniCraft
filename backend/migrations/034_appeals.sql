-- Depends on: 001_initial_schema.sql (users table must exist)
-- Depends on: 025_content_items.sql (content_items table must exist for content appeals)
-- Depends on: 030_comments.sql (comments table must exist for comment appeals)
-- Modifies: appeals table constraints and indexes

ALTER TABLE appeals
    DROP CONSTRAINT IF EXISTS appeals_target_type_check;

ALTER TABLE appeals
    ADD CONSTRAINT appeals_target_type_check CHECK (target_type IN ('content','comment'));

ALTER TABLE appeals
    DROP CONSTRAINT IF EXISTS appeals_status_check;

ALTER TABLE appeals
    ADD CONSTRAINT appeals_status_check CHECK (status IN ('pending','approved','rejected'));

CREATE INDEX IF NOT EXISTS idx_appeals_user ON appeals (user_id);
CREATE INDEX IF NOT EXISTS idx_appeals_status ON appeals (status);
