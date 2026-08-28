# OmniCraft Resume and Interview Claim Matrix

> Created: 2026-08-27
> **预计失效日期**: 2026-10-27
> Scope: local interview evidence; not a production release artifact

## Executive summary

OmniCraft has verified local MiniMax Chat and legacy `embo-01` Embedding
provider contracts, a versioned current-v1 hybrid RAG differential evaluation,
provenance and visibility audits, and a runnable Jaeger/Collector stack with
explicit server/worker service names. The Agent Chat and async projection
business traces remain partial in this environment; production is unavailable.

## Evidence grading legend

`Local real provider verified` means a real provider call was captured locally.
`Local full-infra partial` means infrastructure or a boundary was real but the
whole business chain was not completed. `Local mocked` uses a test double.
`Designed / documented` is a code/spec contract. `Production unavailable` has
no production evidence.

## Claim matrix

| Claim | Recommended wording | Evidence | Grade | Real provider | Reproducible | Limits / do not infer |
|---|---|---|---|---|---|---|
| MiniMax integration | Implemented MiniMax OpenAI-compatible Chat and legacy Embedding adapters, with separate endpoints and GroupId handling. | `scripts/real-provider-preflight.sh`; provider tests | Local real provider verified | Yes | Yes with credentials | No production SLA or quality claim |
| RAG differential | Ran current-v1 hybrid differential evaluation over 63 cases, 169 contents/chunks, generation 2, with a fixed golden-set checksum. | `docs/working/2026-08-26-rag-real-minimax-differential-evidence.md` | Local real provider verified | Embedding yes | Yes with isolated DB | Not historical 253 corpus |
| Retrieval result | K=10: Recall@10 0.492, MRR 0.437, citation precision 0.170; K=20: Recall@10 0.492, MRR 0.438, citation precision 0.162. | same RAG evidence | Local real provider verified | Yes | Yes | Citation precision remains low; no reranker fix |
| Baseline diagnosis | Chunk keyword baseline MRR was 0.370; K increase did not solve citation precision. | same RAG evidence | Local real provider verified | Partial | Yes | Historical 0.913 is not comparable |
| Provenance | Audited corpus identity and found the historical 253-content snapshot unrecoverable. | `docs/working/2026-08-26-rag-provenance-and-current-corpus-v1.md` | Local full-infra partial | N/A | Yes | Cannot claim results on 253 historical rows |
| Visibility | Two evaluation runs recorded 0 visibility leaks. | RAG differential evidence | Local real provider verified | Embedding yes | Yes | Local corpus only |
| OTel naming | Added resource-backed `service.name`; Jaeger now shows `omnicraft-server` and `omnicraft-worker`. | `docs/working/2026-08-27-jaeger-end-to-end-evidence.md`; tracing test | Local full-infra partial | N/A | Service query reproducible with running stack | Representative trace IDs require a manual server/worker run |
| Agent Chat trace | Captured authenticated HTTP -> DB -> MiniMax-M3 -> SSE request with Jaeger trace `0a9b971022bc994ca0f031932e478704`; citation version/chunk/route/source fields were absent in this runtime response. | jaeger evidence 2026-08-28 | Local real provider / partial citation contract | Yes | Reproducible with local credentials | Do not say production or complete citation-field contract |
| Async projection trace | Fixed route wiring so authenticated content update creates/sends `content.updated` and Worker records inbox consumption; projection remains `local-keyword-seed-v1`, with real `embo-01` embedding/final projection still unverified. | Jaeger evidence 2026-08-28; outbox/inbox tests | Local full-infra partial | No | Outbox/relay/inbox boundary | Do not claim full projection trace |
| Collector resilience | Health request remained successful while Collector was stopped and after restart. | Jaeger evidence | Local real | N/A | Health drill reproducible with running server | Telemetry may be lost during outage |

## Allowed resume wording

- Built MiniMax Chat/Embedding provider contracts and a reproducible current-v1
  hybrid RAG evaluation with provenance and visibility audits.
- Used K=10/K=20 controls to show that the historical baseline was not directly
  comparable and that increasing Top-K did not resolve citation precision.
- Added local OTel Collector/Jaeger service identity and validated server/worker
  queue-boundary telemetry plus a Collector-offline business-health drill.

## Forbidden wording

Do not say “completed Jaeger end-to-end tracing”, “MiniMax significantly
improved retrieval”, “real MiniMax on the historical 253 corpus”, “reranker
fixed citation quality”, or “production stable/metric-backed”. Do not turn a
mock provider, `/healthz` span, or Collector startup into a business-chain claim.

## Interview material

### Resume bullets

1. Integrated real MiniMax Chat and `embo-01` Embedding providers with separate
   endpoints and GroupId-aware configuration; built a versioned hybrid RAG
   differential evaluation with provenance and visibility audits.
2. Evaluated 63 golden cases over 169 current-v1 contents/chunks at K=10/K=20,
   identifying that historical metrics were not comparable and that citation
   precision remained the limiting metric.
3. Added explicit OTel service identity for server/worker and verified local
   Collector/Jaeger export plus business continuity during a Collector outage.

### 60-second narrative

I integrated MiniMax Chat and the legacy Embedding API behind separate,
configuration-driven provider contracts. To avoid a misleading benchmark, I
froze a current-v1 corpus and golden-set checksum, then ran 63 cases over 169
chunks at K=10 and K=20. Recall and MRR were measurable, but citation precision
stayed low, and I found the old 253-content baseline could not be reconstructed,
so I refused to call it a regression or improvement. I also repaired local OTel
resource identity so Jaeger distinguishes server and worker, and ran a
Collector-offline drill showing the health path still succeeds. The remaining
gap is a disposable authenticated Agent Chat and full async projection trace;
those are explicitly partial rather than presented as production evidence.

### Why no direct improvement claim?

The historical `0.913` result used a different corpus/entity and its source
snapshot is unrecoverable. Comparing it with current chunk-level citation
precision would mix experiments, so the honest result is a reproducible current
baseline and a diagnosis of non-comparability.

### How was non-comparability found?

The provenance audit inspected Git history, seeds, reports, eval runs, and the
local database. It found no 253-row body/ID manifest or backup, while the new
current-v1 identity checksum and golden checksum are reproducible.

### What is Jaeger complete to?

Jaeger/Collector export, explicit server/worker naming, health tracing, and
worker queue-boundary spans are locally real. The authenticated Chat and
outbox-to-embedding-to-projection traces are not complete in this run, so the
overall claim is local full-infra partial, not full end-to-end.

## Reproduction commands and evidence index

See `2026-08-27-jaeger-end-to-end-evidence.md` for telemetry commands and
`2026-08-26-rag-real-minimax-differential-evidence.md` plus
`2026-08-26-rag-provenance-and-current-corpus-v1.md` for provider/RAG evidence.
Run backend gates with `go build ./...`, `go vet ./...`, `go test ./...`, and
`bash scripts/verify-project.sh --full`.

## Production gaps and future work

Capture disposable-account Agent Chat and async projection traces with real
provider credentials, then repeat against a production-like Collector/Jaeger
and OpenSearch. Reranker evaluation and ClamAV inventory rescan remain future
work; neither is claimed here.
