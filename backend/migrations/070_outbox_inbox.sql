-- Reliable async foundation (reliable-async-observability roadmap T02, issue
-- #137):
--   * outbox_events is the transactional outbox. Business status transitions
--     write one row per event inside the same database transaction as the
--     business write (content.published / content.updated / content.banned /
--     content.deleted for the RAG event surface, rag-deepening design §3).
--     id BIGSERIAL doubles as the stable event_id: retried deliveries reuse
--     the same row and therefore the same event id, which is what the inbox
--     idempotency key (consumer_group, event_id) is built on.
--   * Delivery bookkeeping: status pending -> sent, attempts accumulates
--     failed deliveries, next_attempt_at gates the retry backoff; the relay
--     claims due rows with FOR UPDATE SKIP LOCKED.
--   * W3C trace context (traceparent/tracestate) is carried per event so a
--     trace can span HTTP -> outbox -> Redis Streams -> Worker unchanged.
--   * inbox_consumers records one row per (consumer_group, event_id) delivery
--     already applied: the UNIQUE constraint is the database-level
--     at-least-once idempotency guard, replacing the legacy Redis SetNX
--     dedup for migrated consumers.
-- Forward-only: existing rows keep their payloads; no backfill needed.
-- This file must not be renumbered.

CREATE TABLE IF NOT EXISTS outbox_events (
    id              BIGSERIAL PRIMARY KEY,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    aggregate_id    BIGINT       NOT NULL,
    event_type      VARCHAR(128) NOT NULL,
    schema_version  INTEGER      NOT NULL DEFAULT 1,
    payload         JSONB        NOT NULL DEFAULT '{}'::jsonb,
    traceparent     VARCHAR(55),
    tracestate      VARCHAR(512),
    status          VARCHAR(16)  NOT NULL DEFAULT 'pending',
    attempts        INTEGER      NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    sent_at         TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_status_next_attempt
    ON outbox_events (status, next_attempt_at);

CREATE INDEX IF NOT EXISTS idx_outbox_events_aggregate_event_type
    ON outbox_events (aggregate_id, event_type);

CREATE TABLE IF NOT EXISTS inbox_consumers (
    id             BIGSERIAL   PRIMARY KEY,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    consumer_group VARCHAR(64) NOT NULL,
    event_id       BIGINT      NOT NULL,
    consumed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_inbox_consumers_group_event
    ON inbox_consumers (consumer_group, event_id);

-- ROLLBACK:
-- Local test databases only. Do not run after this migration reaches a shared environment.
-- DROP TABLE IF EXISTS inbox_consumers;
-- DROP TABLE IF EXISTS outbox_events;
