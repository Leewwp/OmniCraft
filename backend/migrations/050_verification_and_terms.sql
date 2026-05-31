-- 050: Add terms/privacy acceptance columns to users
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS accepted_terms_version VARCHAR(32),
  ADD COLUMN IF NOT EXISTS accepted_terms_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS accepted_privacy_version VARCHAR(32),
  ADD COLUMN IF NOT EXISTS accepted_privacy_at TIMESTAMPTZ;
