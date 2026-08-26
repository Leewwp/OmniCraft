# T08 OTel Evidence

> Created: 2026-08-19
> **预计失效日期**: 2026-10-19
> Scope: local development only; this file is not production evidence.

## Implemented Contracts

- `go.opentelemetry.io/otel` provider with head-based ratio sampling and OTLP/gRPC export.
- Gin HTTP root spans extract W3C context and keep `X-Request-ID` independent from `trace_id`.
- Redis Stream producer, relay, and consumer spans carry `traceparent`/`tracestate` only in message metadata.
- GORM tracing plugin is configured without query variables.
- LLM chat, stream, and embedding spans record model, temperature, and token usage only; prompt and embedding text are not span attributes.
- Agent SSE uses the request OTel trace ID; direct untraced service tests use an explicit all-zero marker.

## Reproducible Local Checks

```text
cd backend
go test ./...
go vet ./...
go build ./...
cd ../tools/doc-validator && go run . --fix
cd ../tools/doc-validator && go run . --fix
cd ../tools/doc-validator && go run . --check
```

The first three commands passed in the #143 worktree. The doc-validator fix was idempotent and the config snapshot was updated manually for the new tracing fields.

## Environment Gaps

- `docker compose config --quiet` and `docker compose --profile full-infra config --quiet` passed using a temporary copy of `.env.example` as `.env`; the temporary file was removed and no containers were started.
- The former `otel/opentelemetry-collector-contrib:0.134.0` image was not resolvable (`manifest unknown`). The Compose full-infra profile now uses the pinned standard `otel/opentelemetry-collector:0.120.0` image, its standard `/etc/otelcol/config.yaml` path, and an explicitly enabled `health_check` extension. On 2026-08-26 the replacement Collector and Jaeger containers started successfully; the Collector reached ready state and Jaeger API returned an application HTTP span from a tracing-enabled local backend. This validates real Collector delivery, but only a `/healthz` single-span smoke was produced. No claim is made for a real Agent Chat or asynchronous Worker full-chain trace yet.

Re-run the remaining full-chain evidence after authenticated Agent and Worker triggers are available:

```text
cp .env.example .env
docker compose --profile full-infra config --quiet
docker compose --profile full-infra up -d postgres redis pgbouncer migrate backend worker jaeger otel-collector
```

- Collector-off business continuity remains covered by provider design and unit seams; it is not a production availability drill.
