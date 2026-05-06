ALTER TABLE follows
    DROP CONSTRAINT IF EXISTS follows_target_type_check;

ALTER TABLE follows
    ADD CONSTRAINT follows_target_type_check CHECK (target_type IN ('user','ip'));

CREATE INDEX IF NOT EXISTS idx_follows_follower ON follows (follower_id);
CREATE INDEX IF NOT EXISTS idx_follows_target ON follows (target_type, target_id);
