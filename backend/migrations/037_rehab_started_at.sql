ALTER TABLE rehab_completions
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ;

UPDATE rehab_completions
SET started_at = COALESCE(started_at, completed_at)
WHERE started_at IS NULL;

ALTER TABLE rehab_completions
    ALTER COLUMN completed_at DROP NOT NULL;
