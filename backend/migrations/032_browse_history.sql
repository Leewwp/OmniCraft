CREATE INDEX IF NOT EXISTS idx_browse_history_user_time
    ON browse_history (user_id, viewed_at DESC);
