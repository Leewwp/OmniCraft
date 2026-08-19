# T09 (#144) RAG Closure Evidence

> Created: 2026-08-19
> **预计失效日期**: 2026-10-19
> Scope: local development only. This record is not production evidence.

## Run context

- Worktree: `OmniCraft-wt-144`
- Branch: `codex/144-rag-closure`
- Base commit: `2d3dbaf` (T08/#143)
- PostgreSQL/Redis: existing local Docker services, healthy; migrations 069-072 are present.
- Corpus: 63 golden cases, 34 local published projection rows.
- Embeddings: deterministic SHA-256 stand-in; no external embedding provider credentials are configured.
- OpenSearch: local `omnicraft-opensearch`, `yellow`, canonical `omnicraft-rag-v1` mapping and `omnicraft-rag-read` alias.

The run IDs below are local evidence identifiers assigned to the command
invocations in this worktree; they are not provider, production, or CI run
IDs. Terminal excerpts and exact command provenance are archived in
`docs/working/2026-08-19-t144-rag-closure-raw.txt`; structured aggregate metrics
are recorded in `docs/working/2026-08-19-t144-rag-closure-metrics.json`. All runs
use base commit `2d3dbaf`; the #144 changes were uncommitted while the commands
ran. The staged-tree hash for the final single-commit content is recorded below
after staging; the final commit hash binds the archived evidence and reviewed
changes together without claiming that the tests ran after the commit.

## Six-drill matrix

### 1. Content import -> index -> Agent answer/citation

Run ID: `t144-d1-20260819-a01`

Local automated seams passed:

```text
go test ./internal/service/rag -run 'TestProjectionSyncsPublishedContentToCurrentGeneration|TestProjectionFirstIncrementalSeedsFixedReadAlias' -count=1 -v
go test ./internal/service -run 'TestAgentEvaluation|TestAgentChatStreamFlagOnEmitsExpandedCitationAndDegradedStatus' -count=1 -v
```

The 12-case Agent evaluation passed. Grounded cases emitted server-owned citations; forged, hidden, unpublished, and no-evidence cases did not enter the citation output. This is an application contract result against local fixtures, not a production import or LLM result.

Archived excerpt: `docs/working/2026-08-19-t144-rag-closure-raw.txt` (`DRILL 1`).

### 2. OpenSearch and embedding degradation/recovery

Run ID: `t144-d2-20260819-a01`

The prior T06 local drill remains the reproducible external-dependency record:
`docs/working/2026-08-19-t141-fault-drills-evidence.md`.

```text
docker stop omnicraft-opensearch
OMNICRAFT_RAG_HYBRID_EVAL=1 OMNICRAFT_RAG_EMBEDDING_MODE=standin \
  OMNICRAFT_RAG_OPENSEARCH_URL=http://127.0.0.1:9200 \
  go test ./internal/service/rag_eval -run TestHybridGoldenSetEval -count=1 -v
docker start omnicraft-opensearch
go test ./internal/repository -run TestOpenSearchRepositoryRealContainerContract -count=1 -v
```

The current local degradation matrix and visibility tests also passed:

```text
go test ./internal/service/rag -run 'TestHybridRetriever' -count=1 -v
```

OpenSearch-down used the PostgreSQL keyword path. Embedding failure used an injected provider error; no real provider was stopped because none is configured. The local hybrid run after restoring the canonical index measured Recall@10 `0.476`, MRR `0.365`, nDCG@10 `0.393`, P95 `5.8ms` in the latest rerun (an earlier run measured `55.7ms`), and visibility leak count `0`. The stand-in MRR did not exceed the same-run keyword baseline (`0.370`); the test records that as a local gate gap.

Archived excerpt: `docs/working/2026-08-19-t144-rag-closure-raw.txt` (`DRILL 2`).

### 3. Duplicate delivery and DLQ replay convergence

Run ID: `t144-d3-20260819-a01`

Existing T03 drill evidence is archived at:

- `docs/working/2026-08-14-t138-fault-drills-evidence.md`
- `docs/working/2026-08-14-t138-fault-drills-raw.txt`

The focused local contracts also passed:

```text
go test ./internal/worker ./internal/pkg/queue ./internal/pkg/events \
  -run 'Test(Indexer|ConsumeInbox|RedisStream|TraceContext|Replay|DLQ|Idempot)' -count=1 -v
```

They cover duplicate event IDs, inbox uniqueness, retry/DLQ behavior, replay payload preservation, Redis group recreation, and W3C trace metadata round-trip. T03's process-level Redis stop/recovery and DLQ replay evidence remains local-only.

Archived excerpts: `docs/working/2026-08-19-t144-rag-closure-raw.txt` (`DRILL 3`),
`docs/working/2026-08-14-t138-fault-drills-raw.txt`.

### 4. Rebuild generation switch and admin auth/audit

Run ID: `t144-d4-20260819-a01`

```text
go test ./internal/service/rag -run 'TestProjection(Rebuild|.*Alias|.*Generation)' -count=1 -v
go test ./internal/handler -run 'TestAdminRAGRebuild' -count=1 -v
```

All selected projection/rebuild and admin rebuild tests passed. They cover staging, validation, alias swap, incomplete-alias recovery, retention, authenticated admin access, stable safe errors, and audit rows. The local derived index was rebuilt after detecting an older mapping that omitted `chunk_index`; the source database was not changed. The fixed `ops/opensearch/seed.sh` now includes and checks that field.

Archived excerpt: `docs/working/2026-08-19-t144-rag-closure-raw.txt` (`DRILL 4`).

### 5. Visibility enforcement and leak count

Run ID: `t144-d5-20260819-a01`

The same-run hybrid evaluation reported visibility leak count `0`. The Agent evaluation and citation revalidation contracts passed hidden-content, unpublished-content, forged-context, wrong-version, wrong-route, and wrong-source cases:

```text
go test ./internal/service/rag_eval -run TestHybridGoldenSetEval -count=1 -v
go test ./internal/service -run 'TestAgentEvaluation|TestRevalidateCitationsRejectsForgedFieldsAndLogsReasons' -count=1 -v
```

The vector baseline's historical local artifact has two leaks, while the current visibility-aware hybrid path has zero. Neither number is a production claim.

Archived excerpt: `docs/working/2026-08-19-t144-rag-closure-raw.txt` (`DRILL 5`).

The bilingual selector fallback in `frontend/e2e/contract-smoke.spec.ts` is a
test-only compatibility correction: the real HTTP contract starts the default
Chinese locale, while the selectors were English-only and caused the full gate
to fail before reaching the search assertions. The product UI and locale files
were not changed; the final `73/73` run exercised the corrected default-locale
contract.

### 6. OTel full-chain Jaeger visualization

Run ID: `t144-d6-20260819-a01`

OTel application, queue, DB, and LLM contract tests passed:

```text
go test ./internal/observability ./internal/middleware ./internal/pkg/events \
  ./internal/pkg/queue ./internal/pkg/database ./internal/pkg/llm \
  -run 'Test(NewTracer|Tracing|TraceContext|FromContext|GORM|LLM|Provider)' -count=1 -v
```

The real full-infra smoke was attempted but stopped before container creation because Docker could not resolve `otel/opentelemetry-collector-contrib:0.134.0` (`manifest ... not found`). No Jaeger UI screenshot or HTTP -> worker -> DB -> LLM visualization is claimed. Re-run after the image tag/registry access is corrected:

```text
cp .env.example .env
docker compose --profile full-infra config --quiet
docker compose --profile full-infra up -d postgres redis pgbouncer migrate backend worker jaeger otel-collector
```

The existing T08 record is `docs/working/2026-08-19-t143-otel-evidence.md`.

Archived excerpt: `docs/working/2026-08-19-t144-rag-closure-raw.txt` (`DRILL 6`).

## Local measured metrics

These are real local measurements from the commands above, explicitly not production evidence.

| Metric | Local result | Source/qualification |
|---|---:|---|
| Recall@10 | 0.476 | 63-case hybrid eval; stand-in embedding; 34-content corpus |
| MRR | 0.365 | Same run; below keyword baseline 0.370 |
| nDCG@10 | 0.393 | Same run |
| Citation precision | 0.049 | Retrieval citation oracle, not generated-answer quality |
| Citation coverage | 0.476 | Retrieval citation oracle |
| Groundedness / answer relevance | not evaluated | No real LLM answer-eval run |
| Visibility leak count | 0 | Hybrid visibility revalidation; local fixture |
| Retrieval P95 | 5.8 ms latest (55.7 ms prior run) | Same local OpenSearch run; stand-in embedding |
| SSE first-token / tokens per request | not measured | No real external LLM |
| OpenSearch degradation | passed | One local stop/restart drill; not a general rate |
| Embedding degradation | passed | Injected provider-error contract; no external provider stopped |
| `verify-project.sh --full` contracts | 73/73 | Mocked browser contracts |

The committed T01 baseline artifact remains the larger local corpus record (`253` contents, `201` embeddings) and is not overwritten by this 34-content run. No metric is promoted to an Implemented production claim until the real-provider, production-like corpus and full-infra evidence are rerun.

## Known gaps and deferred work

- Real Jaeger UI visualization and Collector delivery: the Collector image tag could not be resolved (`manifest ... not found`) before container creation.
- Real embedding provider, real LLM answer evaluation, production-like corpus, and OSS/production data: not configured in local development.
- `#76` legacy favorites migration remains deferred until production deployment is explicitly resumed.
- `#151` historical archive rescan remains deferred/not planned.
- Desktop/Tauri work remains outside the Web-only batch.
