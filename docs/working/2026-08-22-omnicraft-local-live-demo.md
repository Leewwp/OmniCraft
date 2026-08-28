# OmniCraft Local Live Demo Runbook

> Created: 2026-08-22
> **预计失效日期**: 2026-10-22
> Scope: local development and interview demonstration only. This document is not a production runbook.

## Purpose

This runbook starts a repeatable local stack for the Agent/RAG surface:

```text
PostgreSQL -> Redis -> migration ledger -> local seed -> OpenSearch/full-infra
-> backend -> standalone worker -> frontend -> browser/API checks
```

The committed scripts do not create, rewrite, or delete `.env`, production configuration, Docker volumes, worktrees, branches, or screenshots. Runtime state and logs are placed under the ignored `artifacts/live-demo/` directory.

## Requirements

- Go 1.22+, Node.js 20+, npm 10+, Python 3, Docker Compose, and curl.
- Docker services `omnicraft-postgres` and `omnicraft-redis` use the local defaults from `.env.example`.
- A local `.env` is required by Compose. Create it manually from `.env.example` when absent; `scripts/live-demo.sh` refuses to create it.
- Real OSS, Green, SMTP, LLM, and embedding credentials are not required. The demo script forces an unsupported local provider so it cannot call an external LLM.

## Startup

Run from the repository root:

```bash
bash scripts/live-demo.sh start
```

This applies every forward-only migration through the migration ledger, verifies that 069, 070, 071, and 072 are present in order, seeds the existing rich UI fixture and the 63-case RAG fixture, and starts host backend/worker/frontend processes. Repeat `start` to reuse live processes and replace only the owned seed namespace.

The default host ports are backend `8080`, frontend `3000`, and metrics `9091`. If another local process already owns a port, the script uses backend `8091`, frontend `3001`, and/or metrics `9092` (or the explicit `OMNICRAFT_LIVE_DEMO_*_FALLBACK_PORT` values) and prints the selected ports.

For the OpenSearch profile and Jaeger/Collector attempt:

```bash
bash scripts/live-demo.sh start --full-infra
```

The full-infra path starts OpenSearch, the seed one-shot, Jaeger, and OTel Collector; waits for health, establishes `omnicraft-rag-v1` plus the fixed `omnicraft-rag-read` alias, then creates a local keyword-only projection from visible published seed rows. It intentionally leaves `chunk_embeddings` empty and labels the projection `local-keyword-seed-v1`; this is not a deterministic embedding provider.

Stop only the host processes with:

```bash
bash scripts/live-demo.sh stop
```

Docker data is preserved. The current status can be checked with:

```bash
bash scripts/live-demo.sh verify
bash scripts/live-demo.sh verify --full-infra
docker compose ps
```

## Test account and questions

The rich seed creates verified accounts with the same local password:

```text
email:    seed-ui-001@seed.omnicraft.local
password: password123
```

Recommended questions and expected local behavior:

| Question | Expected result | Evidence boundary |
| --- | --- | --- |
| `Blender 插件安装教程` | visible published content can be returned by keyword retrieval; citations must carry server-owned content/version/chunk fields when a ready RAG projection exists | local seed/projection; provider is disabled |
| `明天的天气怎么样` | `no_evidence` state with no citation and ordinary search CTA | application contract and local UI state |
| `Blender 插件安装教程` after stopping OpenSearch | request completes through PostgreSQL keyword fallback and marks `degraded` | local fault drill; no production claim |
| `请生成 1011 的使用指南` | unpublished/hidden content id 1011 must not become a citation | visibility predicates and fixture contract |

The normal grounded natural-language answer requires a configured provider. The local runbook deliberately does not call one; use the local keyword result and mocked/contract evidence for that part unless a provider-backed environment is explicitly authorized.

## API checks

Unauthenticated access must remain protected:

```bash
curl -i http://127.0.0.1:8080/api/v1/agent/conversations
```

Authenticated browser/API checks should use the login response's access token and CSRF cookie; never disable auth or CSRF to produce evidence. Verify all four paths: normal request, malformed input, no-auth request, and OpenSearch-down degraded request. SSE output should be checked for `trace_id`, `content_id`, and `degraded`; the expanded `content_version`, `chunk_key`, and `source` fields are emitted only when the runtime projection/citation path supplies them and remain a known partial boundary in the captured real-provider run.

## Idempotence checks

```bash
bash scripts/live-demo.sh start
bash scripts/live-demo.sh start
docker compose --profile full-infra config --quiet
```

The migration ledger rejects checksum drift and skips already-applied migrations; seed SQL uses conflict-safe keys; the rich fixture replaces only its namespace; the OpenSearch seed validates the canonical mapping and reapplies the read alias; the local RAG projection replaces only generation 1 rows for its eligible seed contents.

## Logs and evidence

- Host process logs: `artifacts/live-demo/logs/{backend,worker,frontend}.log`.
- Host process IDs: `artifacts/live-demo/pids/`.
- Migration summary: `artifacts/live-demo/migration-summary.json`.
- Real browser screenshots, when the authenticated browser run succeeds: `screenshots/omnicraft-live-demo/`.
- Mocked contract screenshots remain under their existing `screenshots/` paths and must not be copied into the live-demo directory.

## Known gaps

- The repository has OTel application contracts and Compose wiring; local
  Collector delivery, service identity, and selected Jaeger traces are
  documented separately. A complete business-chain screenshot is not claimed
  because the final OpenSearch projection was not verified.
- The demo uses local seed data and a keyword-only projection. A real
  OpenSearch projection is optional and is not required for the Phase 1 demo;
  the runbook does not switch the read alias or mutate the baseline generation.
  It does not claim real embedding quality, real LLM groundedness, production
  traffic, production OSS, ClamAV/EICAR, or production release readiness.
