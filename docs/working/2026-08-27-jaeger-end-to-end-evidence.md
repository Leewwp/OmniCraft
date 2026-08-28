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

Evidence grading uses `implemented`, `local real provider`, `local full-infra`,
`local mocked`, `partial`, and `production unavailable`.

## Environment and reproduction

```bash
docker compose up -d postgres redis
docker compose --profile full-infra up -d jaeger otel-collector
docker exec omnicraft-otel-collector /otelcol validate --config=/etc/otelcol/config.yaml
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS 'http://127.0.0.1:16686/api/services'
```

The commands above reproduce the infrastructure health and service query only.
They do not recreate the representative trace IDs below: those IDs came from a
manual run of the locally configured server/worker processes and queue activity.
Reproducing a business trace additionally requires starting both processes with
the documented environment, creating a disposable local event, waiting for the
worker, and querying Jaeger by the resulting trace ID.

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

### 2026-08-28 authenticated capture

Normal seeded-account login with CSRF protection succeeded, followed by a real
MiniMax request to `POST /api/v1/agent/chat/stream`.

| Field | Redacted evidence |
|---|---|
| Provider/model | `minimax` / `MiniMax-M3` |
| HTTP status | `200` (`text/event-stream`) |
| X-Request-ID | `66e4dce62ec98c84` |
| OTel trace_id | `0a9b971022bc994ca0f031932e478704` |
| Server span | `POST /api/v1/agent/chat/stream`, ~12.6 s |
| Service | `omnicraft-server` |
| Key spans | `insert agent_conversations`, `insert agent_messages`, `llm.chat`, `llm.chat.stream`, `llm.embedding`, `update agent_conversations` |
| SSE order | `start -> tool_status -> delta -> citation -> usage -> done` |
| Citation | `content_id=1001`; title/zone/excerpt present, version/chunk/route/source absent in this runtime response |
| Grade | `local real provider`, with citation-field `partial` |

This confirms synchronous HTTP -> DB -> MiniMax Chat -> SSE. It does not pass
through the standalone Worker. Temporary API/SSE/Jaeger files were redacted and
were not added to the repository.

Route mapping is `POST /api/v1/agent/chat/stream` through the authenticated
`/agent` group, then `AgentHandler.ChatStream` and `AgentService.ChatStream`,
which writes conversation data and emits SSE events. Existing tests cover the
route contract with a fake provider.

The earlier mocked status is superseded for this run by the authenticated
capture above. Production evidence remains unavailable.

## Async content projection

The running worker emitted real `queue.relay` and `queue.consume` spans. The
code path is transactional outbox -> relay -> Redis Streams -> worker, with
W3C traceparent stored in the envelope. Existing integration tests cover
commit-after-outbox and inbox idempotency contracts.

**Grade: local full-infra partial.** A real authenticated content update returned
`200`, but the running API instance produced no `outbox_events` row, leaving the
Worker with no event to relay. No real `embo-01` embedding or final
PostgreSQL/OpenSearch projection trace is asserted. Existing worker trace IDs
remain queue-boundary evidence only.

### Wiring correction (2026-08-28)

The missing outbox row was traced to route wiring: `RegisterRoutes` constructed a
separate `ContentHandler` whose internal service had no outbox repository. The
handler now receives the shared container outbox explicitly. A repeat
authenticated update produced `content.updated` with status `sent`, a W3C
`traceparent`, and an `omnicraft-indexer` inbox record. The projection status for
the seeded content nevertheless remained `local-keyword-seed-v1`; the real
`embo-01` embedding/projection operation was not observed. The async grade is
therefore still `partial`, with the outbox -> relay -> Redis -> inbox boundary
now locally verified.

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
