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
- A real local smoke attempt was made with `docker compose --profile full-infra up -d jaeger otel-collector`, but Docker Hub authorization failed while resolving `otel/opentelemetry-collector-contrib:0.134.0` (`Get https://auth.docker.io/token: EOF`). `docker compose ... ps --all` confirmed no container was created. No claim is made for real OTel Collector delivery, Jaeger UI rendering, or a full HTTP-to-worker-to-DB-to-LLM trace. Re-run after registry access is restored:

```text
cp .env.example .env
docker compose --profile full-infra config --quiet
docker compose --profile full-infra up -d postgres redis pgbouncer migrate backend worker jaeger otel-collector
```

- Collector-off business continuity remains covered by provider design and unit seams; it is not a production availability drill.
