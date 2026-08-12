-- Collaboration invites (collaboration-invites #97):
--   * collaboration_invites tracks publish-time invites to co-authors; the
--     active partial unique index guarantees at most one pending/accepted
--     invite per (content_id, invitee_id) while still allowing a re-invite
--     once a previous invite expires or is declined;
--   * message_id links the invite card in messages (msg_type='collab_invite')
--     back to the invite and is cleared when the message is deleted;
--   * users.accept_collab_invites is the recipient opt-in switch, on by
--     default so existing users keep receiving invites;
--   * messages.msg_type + messages.metadata carry typed invite cards: only
--     'text' and 'collab_invite' are allowed, and collab_invite cards store
--     their invite_id in JSONB metadata.
-- Forward-only: existing messages keep msg_type 'text' and empty metadata.
-- This file must not be renumbered.

CREATE TABLE IF NOT EXISTS collaboration_invites (
    id            BIGSERIAL   PRIMARY KEY,
    content_id    BIGINT      NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    inviter_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message_id    BIGINT      REFERENCES messages(id) ON DELETE SET NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'pending'
                              CHECK (status IN ('pending', 'accepted', 'declined', 'expired')),
    expires_at    TIMESTAMPTZ NOT NULL,
    responded_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_collab_invites_inviter
  ON collaboration_invites (inviter_id);

CREATE INDEX IF NOT EXISTS idx_collab_invites_invitee
  ON collaboration_invites (invitee_id, status, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_collab_invites_active
  ON collaboration_invites (content_id, invitee_id)
  WHERE status IN ('pending', 'accepted');

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS accept_collab_invites BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE messages
  ADD COLUMN IF NOT EXISTS msg_type VARCHAR(20) NOT NULL DEFAULT 'text'
    CHECK (msg_type IN ('text', 'collab_invite'));

ALTER TABLE messages
  ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}'::jsonb;

-- ROLLBACK:
-- Local test databases only. Do not run after this migration reaches a shared environment.
-- DROP TABLE IF EXISTS collaboration_invites;
-- ALTER TABLE users DROP COLUMN IF EXISTS accept_collab_invites;
-- ALTER TABLE messages DROP COLUMN IF EXISTS msg_type;
-- ALTER TABLE messages DROP COLUMN IF EXISTS metadata;