# OmniCraft Resume and Live Demo Evidence

> Created: 2026-08-22
> **预计失效日期**: 2026-10-22
> Status: local evidence package maintained with explicit verification limits; no production claim.

## Project positioning

OmniCraft is a Web-only local development project for a creator community. The relevant interview slice is a modular-monolith Agent/RAG path backed by PostgreSQL, Redis Streams, an independent Go Worker, optional OpenSearch full-infra retrieval, SSE, and application-level OpenTelemetry contracts.

## Responsibility and architecture

The implementation scope represented by the repository is:

- RAG evaluation truth source and deterministic local evaluation harness: `backend/migrations/069_rag_evaluation.sql`, `backend/internal/service/rag_eval/`, and `backend/testdata/`.
- Transactional Outbox/Inbox and content event contracts: `backend/migrations/070_outbox_inbox.sql`, `backend/internal/pkg/events/`, and `backend/internal/repository/outbox_repo.go`.
- Independent Worker, relay, Inbox idempotency and DLQ replay: `backend/cmd/worker/`, `backend/internal/worker/`, and the admin queue route.
- Chunking, current-generation projection, visibility revalidation, OpenSearch alias/rebuild, RRF and degradation: `backend/migrations/071_rag_chunks.sql`, `backend/internal/service/rag/`, `backend/internal/repository/{rag_chunk_repo,opensearch_repo}.go`.
- Agent citation/SSE boundary and UI: `backend/internal/service/agent_*`, `backend/internal/handler/agent*.go`, `frontend/app/(protected)/agent/`, and `frontend/components/agent/`.
- Phase 1 Agent workspace seams: server-owned Citation Verification, internal streaming-only Provider capability, and fixed four-tool registry (GitHub #207, completed 2026-08-28).
- OTel spans and W3C context propagation: `backend/internal/observability/`, queue tracing, database tracing, LLM observability, and `ops/observability/otel-collector.yml`.

The runtime boundary remains modular monolith + one independent Worker + REST/SSE + one Agent. #145's ADR does not authorize gRPC, Kafka, multi-agent runtime, or a split Search service.

## Agent/RAG flow

```text
viewer request -> Agent handler -> viewer-aware retrieval -> citation revalidation
-> provider/fallback path -> SSE start/tool/delta/citation/done -> ContentDetail overlay
```

Citation fields are server-owned: `content_id`, `content_version`, `chunk_key`, `title`, `zone`, `route`, `excerpt`, and `source`. The final read checks published visibility, latest active content version, current ready projection, chunk identity, and viewer scope before emitting a citation.

## Reliable async flow

```text
business transaction -> Outbox row -> Worker relay -> Redis Stream
-> consumer Inbox row + side effect in one DB transaction -> ack or bounded retry/DLQ
```

Outbox event IDs are stable across retries. Queue trace context uses W3C `traceparent`/`tracestate`; `X-Request-ID` remains independent of the OTel `trace_id`.

## Local demo procedure

The repeatable local procedure is documented in [2026-08-22-omnicraft-local-live-demo.md](2026-08-22-omnicraft-local-live-demo.md) and runs with:

```bash
bash scripts/live-demo.sh start
bash scripts/live-demo.sh start --full-infra
bash scripts/live-demo.sh verify --full-infra
```

The first command is the baseline local stack. The second is the optional OpenSearch/Jaeger/Collector attempt. The script uses the existing rich UI and RAG evaluation fixtures, writes only to local seed namespaces, and does not overwrite `.env`.

## Evidence status

### 2026-08-28 evidence closure update

One authenticated local Agent Chat request reached MiniMax-M3 and returned SSE
with the contractual event order `start -> tool_status -> delta -> citation ->
usage -> done`. Jaeger trace `0a9b971022bc994ca0f031932e478704` (service
`omnicraft-server`) contained HTTP, DB, `llm.chat`, `llm.chat.stream` and
`llm.embedding` spans; request ID `66e4dce62ec98c84` remained distinct. This is
`local real provider` evidence for synchronous Chat, not a Worker trace. The
runtime citation emitted `content_id` and display fields but omitted
`content_version`, `chunk_key`, `route` and `source`, so that contract remains
`partial`.

An authenticated content update initially exposed a route-wiring defect. After
the correction, the event was relayed and consumed by the standalone Worker,
which produced real `embo-01` embedding and Inbox spans. The final
PostgreSQL/OpenSearch projection remains unverified by deliberate Phase 1 scope
decision, so async evidence remains `local full-infra partial`; production is
unavailable.

| Claim | Code/test evidence | Local live evidence | Production status |
| --- | --- | --- | --- |
| RAG evaluation schema/harness | #136 tests and 63-case artifacts | fixture ran twice; `content_items=38`, `content_versions=38`, `eval_golden_cases=63` | not verified with real provider/corpus |
| Outbox/Worker/DLQ | #137/#138 tests and fault-drill evidence | standalone Worker started; subscriptions and outbox polling visible in `artifacts/live-demo/logs/worker.log` | not a production deployment claim |
| Hybrid retrieval and citation revalidation | #141/#142 tests and local OpenSearch evidence | local keyword projection created with 30 documents; Agent returned 5 validated citations in degraded mode | real embedding/LLM not verified |
| OpenSearch mapping, generation and read alias | repository tests and `ops/opensearch/seed.sh` | `omnicraft-rag-v1` and `omnicraft-rag-read` verified at `127.0.0.1:9200` | no production index claim |
| OTel application contracts | #143 tests and `docs/working/2026-08-19-t143-otel-evidence.md` | Pinned standard Collector + Jaeger started; authenticated Chat and Worker/embedding spans queried | Final OpenSearch projection is not verified; no production claim |
| Agent browser interaction | existing mocked contracts/screenshots | real login, `/agent`, grounded/degraded query, no-evidence state, citation overlay, desktop/mobile screenshots passed | no production UX claim |

## Browser screenshot ledger

Real browser screenshots, when successfully captured, belong only in `screenshots/omnicraft-live-demo/`:

| File | Viewport | URL | Question/state | Stand-in/mock | Result |
| --- | ---: | --- | --- | --- | --- |
| `agent-grounded-desktop-1710x983.png` | 1710x983 | `/agent` | grounded keyword query with five citations | local keyword projection; provider disabled | passed |
| `agent-citation-detail-desktop-1710x983.png` | 1710x983 | `/agent` | click citation and open content detail | local seed content; provider disabled | passed |
| `agent-desktop-1710x983.png` | 1710x983 | `/agent` | no-evidence query state | no provider claim | passed |
| `agent-mobile-390x844.png` | 390x844 | `/agent` | authenticated Agent empty state and responsive layout | local UI; provider disabled | passed |

These are separate from mocked contract screenshots such as `screenshots/web-agent-grounded-desktop.png`.

## OTel status

T08 is accurately described as “OTel application span contracts, W3C propagation,
local Collector/Jaeger export, and selected server/worker traces completed.” It
must not be described as “Jaeger full-chain tracing completed”: the final
PostgreSQL/OpenSearch projection was not verified and no alias was cut over.
`X-Request-ID` and `trace_id` must remain distinct.

## Validation commands

The final evidence gate is:

```bash
cd backend && go build ./... && go vet ./... && go test ./...
cd ../frontend && npm run lint && npm run build
cd .. && bash scripts/verify-project.sh --full
cd tools/doc-validator && go run . --fix && go run . --fix && go run . --check
git diff --check
```

The backend build/vet/test, frontend lint/build, `scripts/live-demo.tests.sh`, compose config, and doc-validator idempotence checks passed. The current mocked browser contract suite passed 74/74 with `cd frontend && npm run test:contracts`; the full project gate was subsequently rerun after transient port conflicts cleared. These are mocked browser contracts, not production evidence.

## Resume wording guardrails

Allowed local wording includes: “implemented and tested a transactional Outbox/Inbox and standalone Worker path”, “built a viewer-aware RAG citation contract with deterministic local evaluation”, “integrated and locally verified MiniMax Chat plus `embo-01` embedding boundaries”, and “added OTel application contracts and a reproducible full-infra configuration.”

Forbidden unless independently verified: production launch, real-user scale, guaranteed RRF uplift, real LLM groundedness, real embedding-provider comparison, complete PostgreSQL/OpenSearch projection, and Jaeger full-chain completion.

## Run log

Initial bootstrap tooling was added on 2026-08-22. The runtime/API/browser/Jaeger results are intentionally appended only after execution, with explicit labels for deterministic stand-in, mocked contract, local real service, and production evidence.

### 2026-08-22 executed local run

- `bash scripts/live-demo.tests.sh`: passed.
- `docker compose exec -T postgres psql ... < backend/testdata/rag_eval_seed.sql`: passed twice consecutively with `ON_ERROR_STOP=1`.
- `bash scripts/live-demo.sh start --full-infra`: passed on backend `8091`, metrics `9092`, and frontend `3000`. PostgreSQL and Redis were healthy, migrations skipped 70 already-applied rows, and the rich seed completed with 24 users, 180 contents, 600 comments, and 131 published/public contents.
- `/healthz`: `200` with `{"service":"omnicraft-backend","status":"ok"}`. `/api/v1/config/public` exposed `web_agent_enabled=true`, `desktop_deploy_enabled=false`, and `download_enabled=false`.
- `GET /api/v1/agent/conversations` without credentials: `401 UNAUTHORIZED`. A POST without CSRF was rejected with `403 CSRF_TOKEN_INVALID` before auth, as required by middleware ordering.
- Real browser login with `seed-ui-001@seed.omnicraft.local` / `password123`: redirected to `/agent`; the navigation exposed `AI Agent`.
- Real browser query `Blender 插件安装教程`: entered search-degraded mode and displayed five validated citations. Real browser query `明天的天气怎么样`: displayed the no-summary/no-evidence degraded state with no citations. Clicking a citation opened the shared content detail overlay for `Blender 插件安装教程`.
- Screenshots saved under `screenshots/omnicraft-live-demo/` are real browser captures, not mocked contract captures.
- Jaeger/OTel Collector now starts with the pinned standard Collector image `otel/opentelemetry-collector:0.120.0`; a tracing-enabled backend `/healthz` span was queried from Jaeger API. Authenticated Agent Chat and asynchronous Worker trace evidence remain unverified.
- The 2026-08-22 baseline demo projection reports `embedding=none`; this is
  run-specific and does not contradict the later isolated real `embo-01`
  Worker evidence. No real embedding quality or LLM groundedness claim is made.
- The held demo session was stopped with `bash scripts/live-demo.sh stop`; Docker data was preserved. A separate local process owning port `18080` was left untouched.
