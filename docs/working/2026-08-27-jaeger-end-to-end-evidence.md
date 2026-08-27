# Jaeger/Collector Local Evidence

> Created: 2026-08-27
> **预计失效日期**: 2026-10-27
> Scope: local development evidence; not a production release artifact

## Result

The local Jaeger and Collector stack is runnable. OTel resources now use explicit
configuration-backed names: `omnicraft-server` for the HTTP process and
`omnicraft-worker` for the async process. Jaeger's service query returned both
names on 2026-08-27. Historical `unknown_service:server` data remains in Jaeger
as old telemetry and is not treated as a current target service.

Evidence grading uses `local real`, `local mocked`, `designed`, and
`production unavailable`.

## Environment and reproduction

```bash
docker compose up -d postgres redis
docker compose --profile full-infra up -d jaeger otel-collector
docker exec omnicraft-otel-collector /otelcol validate --config=/etc/otelcol/config.yaml
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS 'http://127.0.0.1:16686/api/services'
```

The backend was run locally with `OBSERVABILITY_TRACING_ENABLED=true`,
`OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317`, and
`OTEL_SERVICE_NAME=omnicraft-server`. The worker used the same endpoint and
`OTEL_SERVICE_NAME=omnicraft-worker`. API keys and cookies are intentionally not
recorded.

## Service-name evidence

Jaeger query result included `omnicraft-server` and `omnicraft-worker`.
Representative current trace IDs:

| Process | Trace ID | Observed spans | Grade |
|---|---|---|---|
| server | `318d91b427253409dde1ba2afd051cc6` | `GET /healthz` | local real |
| worker | `7368c7e37e98828454ad0448311cb057` | `queue.consume` | local real |
| worker | `da03b3895a3e44c60d516ddf9d501cba` | `queue.relay`, `select outbox_events` | local real |

The trace list proves export and service naming, not the complete business
workflow claimed below.

## Authentication Agent Chat

Route mapping is `POST /api/v1/agent/chat/stream` through the authenticated
`/agent` group, then `AgentHandler.ChatStream` and `AgentService.ChatStream`,
which writes conversation data and emits SSE events. Existing tests cover the
route contract with a fake provider.

**Grade: local mocked / production unavailable.** This run did not execute a
real authenticated request through MiniMax Chat, DB persistence, and SSE with a
single captured trace ID. No claim of a complete Agent Chat Jaeger trace is
made. The missing evidence is a disposable authenticated account and a valid
local MiniMax Chat configuration, followed by a Jaeger trace export query.

## Async content projection

The running worker emitted real `queue.relay` and `queue.consume` spans. The
code path is transactional outbox -> relay -> Redis Streams -> worker, with
W3C traceparent stored in the envelope. Existing integration tests cover
commit-after-outbox and inbox idempotency contracts.

**Grade: local full-infra partial.** This run did not complete a disposable
content event through real MiniMax `embo-01` embedding and PostgreSQL/OpenSearch
projection, so no complete async trace ID is asserted. OpenSearch is not
required by the default stack and was not used as evidence here. The available
worker trace IDs above are queue-boundary evidence only.

## Collector offline drill

1. `docker stop omnicraft-otel-collector`
2. `curl -fsS http://127.0.0.1:8080/healthz` returned `{"service":"omnicraft-backend","status":"ok"}`.
3. `docker start omnicraft-otel-collector`
4. Native Collector config validation passed and the container returned to
   `healthy`.

No request timeout or business failure occurred. The SDK's bounded batch
exporter dropped telemetry while the Collector was unavailable; this is an
observability loss, not a business failure. Grade: **local real** for the
health-path drill.

## Limitations

- No production Collector, Jaeger, MiniMax, OpenSearch, or SLA evidence.
- `/healthz` and queue spans do not establish Agent Chat or projection
  end-to-end completion.
- Old `unknown_service:server` spans remain historical data in Jaeger.
- Real-provider RAG numbers and provenance are documented separately in the
  2026-08-26 working evidence files.
