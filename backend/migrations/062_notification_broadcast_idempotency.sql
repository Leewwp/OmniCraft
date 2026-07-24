BEGIN;

CREATE TABLE IF NOT EXISTS notification_broadcast_requests (
    id BIGSERIAL PRIMARY KEY,
    actor_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash VARCHAR(64) NOT NULL,
    payload_hash VARCHAR(64) NOT NULL,
    recipient_count INT NOT NULL,
    broadcast_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_notification_broadcast_requests_actor_key
    ON notification_broadcast_requests (actor_id, key_hash);

COMMIT;

-- ROLLBACK:
-- Local test databases only. Do not run after this migration reaches a shared environment.
-- DROP TABLE IF EXISTS notification_broadcast_requests;
