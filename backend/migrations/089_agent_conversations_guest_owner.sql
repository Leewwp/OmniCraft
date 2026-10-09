-- 089 (#854): guest ownership for agent conversations.
--
-- Guest conversations are owned by a signed anonymous device key, never by a
-- shared sentinel user id (user_id=0 must not become a catch-all owner) and
-- never by a pseudo user row. The two ownership shapes are made mutually
-- exclusive by a CHECK constraint; the existing user-owned rows keep their
-- shape untouched (is_guest defaults false, guest_device_key stays NULL).

ALTER TABLE agent_conversations ALTER COLUMN user_id DROP NOT NULL;

ALTER TABLE agent_conversations ADD COLUMN IF NOT EXISTS is_guest BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE agent_conversations ADD COLUMN IF NOT EXISTS guest_device_key VARCHAR(64);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'agent_conversations_owner_check'
          AND conrelid = 'agent_conversations'::regclass
    ) THEN
        ALTER TABLE agent_conversations ADD CONSTRAINT agent_conversations_owner_check CHECK (
            (is_guest = FALSE AND user_id IS NOT NULL AND guest_device_key IS NULL)
            OR (is_guest = TRUE AND user_id IS NULL AND guest_device_key IS NOT NULL AND guest_device_key <> '')
        );
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_agent_conversations_guest_device
    ON agent_conversations (guest_device_key, created_at DESC)
    WHERE is_guest;
