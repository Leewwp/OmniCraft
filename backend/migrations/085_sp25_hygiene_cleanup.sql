-- Migration 085: SP-25 FR-12 hygiene cleanup (低-36/低-37)
-- 低-36: password_reset_tokens is a dead table — password reset flows through
-- the Redis verification pipeline (verification_service); the table has had
-- zero Go references since 040 introduced it. Dead tables invite accidental
-- future use of an un-maintained surface, so it goes.
DROP TABLE IF EXISTS password_reset_tokens;

-- 低-37: redundant indexes. users.email / users.username carry UNIQUE
-- constraints (001:4/001:6) whose backing unique indexes are equivalent to
-- these plain b-trees — one extra write amplification per user mutation for
-- zero query benefit. idx_users_role stays (no UNIQUE on role).
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_users_username;
