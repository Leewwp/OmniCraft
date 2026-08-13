-- AI review idempotency (content-safety-callback-fix #105):
--   * ai_review_records gains a nullable provider_task_id so async scan
--     results can be keyed by the provider's task id;
--   * the unique index (provider, provider_task_id) lets the review flow
--     short-circuit duplicate callbacks (Alibaba Cloud retries delivery up
--     to 16 times) without re-recording, re-penalizing or re-freezing;
--   * PostgreSQL unique indexes ignore NULLs, so synchronous records without
--     a task id never collide with each other or with async results: the
--     initial submission record and the async result are distinct stages and
--     must not share an idempotency key.
-- Forward-only: existing rows keep NULL provider_task_id and stay readable.
-- This file must not be renumbered.

ALTER TABLE ai_review_records
    ADD COLUMN IF NOT EXISTS provider_task_id VARCHAR(128);

CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_review_records_provider_task
    ON ai_review_records (provider, provider_task_id);

-- ROLLBACK:
-- Local test databases only. Do not run after this migration reaches a shared environment.
-- DROP INDEX IF EXISTS uq_ai_review_records_provider_task;
-- ALTER TABLE ai_review_records DROP COLUMN IF EXISTS provider_task_id;
