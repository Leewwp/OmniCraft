CREATE INDEX IF NOT EXISTS idx_follows_target_created ON follows(target_type, target_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_items_zone_status ON content_items(zone, status);
CREATE INDEX IF NOT EXISTS idx_browse_history_user_viewed ON browse_history(user_id, viewed_at DESC);
CREATE INDEX IF NOT EXISTS idx_reactions_user_type ON reactions(user_id, target_type, reaction);
