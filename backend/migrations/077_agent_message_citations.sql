-- N4 history citations (2026-09-07): the agent answer's validated citation
-- list persists on the assistant row so GET /agent/conversations/{id} can
-- replay jump entries for historical turns (the frontend backfill merge was
-- a temporary session-only workaround, PR #395).
--   * agent_messages gains a nullable citations JSONB column (array of the
--     server-owned AgentCitation contract objects);
--   * legacy rows keep NULL and the history projection treats NULL as "no
--     citations" — no backfill migration is needed;
--   * answer rows flagged by the post-turn output audit are still redacted
--     in the API projection, citations included.
-- Forward-only. This file must not be renumbered.

ALTER TABLE agent_messages
    ADD COLUMN IF NOT EXISTS citations JSONB;
