-- IP visit history (ip-visit-history #73):
--   * one row per (user_id, ip_id) pair; visited_at tracks the most recent
--     visit so repeated visits refresh recency without growing the table;
--   * direct visits record the server receive time; anonymous login merge
--     upserts with GREATEST so an old replay can never lower recency;
--   * the composite primary key keeps the account/IP pair unique;
--   * both foreign keys cascade so account deletion or IP removal cleans
--     history with the owning row;
--   * the ordering index serves the recent-six list with a stable
--     same-timestamp tie-break (ip_id DESC).
-- Forward-only: anonymous history stays in the browser; account history
-- starts empty for existing users.
-- This file must not be renumbered.

CREATE TABLE IF NOT EXISTS ip_visit_history (
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip_id      BIGINT      NOT NULL REFERENCES ips(id) ON DELETE CASCADE,
    visited_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, ip_id)
);

CREATE INDEX IF NOT EXISTS idx_ip_visit_history_user_recent
  ON ip_visit_history (user_id, visited_at DESC, ip_id DESC);

-- ROLLBACK:
-- Local test databases only. Do not run after this migration reaches a shared environment.
-- DROP TABLE IF EXISTS ip_visit_history;
