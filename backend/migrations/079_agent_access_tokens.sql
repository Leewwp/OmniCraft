-- Depends on: 001_users.sql (users table)
-- SP-16 #450 (spec D2): agent_access_tokens backs the PAT machine identity
-- for external agents. Design decisions recorded here:
--   * token_hash stores SHA-256 hex of the full "oc_pat_<43 base62>" token;
--     the plaintext is shown exactly once at creation and never persisted;
--   * token_prefix keeps the first 12 characters (including the oc_pat_
--     prefix) so the settings list can identify a token without the secret;
--   * scopes is a comma-separated subset of (download, upload) enforced by
--     the CHECK below;
--   * no expiry column by design (2026-09-11 user decision): tokens live
--     until revoked; last_used_at lets the owner spot anomalies;
--   * the partial index covers the auth lookup path (live tokens per user)
--     and the settings page list.
-- Forward-only. This file must not be renumbered.

CREATE TABLE IF NOT EXISTS agent_access_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(64) NOT NULL,
    token_hash VARCHAR(64) NOT NULL,
    token_prefix VARCHAR(12) NOT NULL,
    scopes VARCHAR(32) NOT NULL DEFAULT 'download',
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT agent_access_tokens_scopes_check
        CHECK (scopes IN ('download', 'upload', 'download,upload'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_access_tokens_token_hash
    ON agent_access_tokens (token_hash);

CREATE INDEX IF NOT EXISTS idx_agent_access_tokens_user_active
    ON agent_access_tokens (user_id)
    WHERE revoked_at IS NULL;
