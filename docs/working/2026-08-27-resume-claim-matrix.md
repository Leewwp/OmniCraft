# OmniCraft Resume and Interview Claim Matrix

> Created: 2026-08-27
> **预计失效日期**: 2026-10-27
> Scope: local interview evidence; not a production release artifact

## Executive summary

OmniCraft has verified local MiniMax Chat and legacy `embo-01` Embedding
provider contracts, a versioned current-v1 hybrid RAG differential evaluation,
provenance and visibility audits, and a runnable Jaeger/Collector stack with
explicit server/worker service names. Authenticated Chat and the async
outbox/Worker/embedding boundary are locally real; the final OpenSearch
projection remains intentionally unverified and is not a Phase 1 gate.

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
| Async projection trace | Fixed route wiring so authenticated content update creates/sends `content.updated`; Worker consumed it and Jaeger shows real `llm.embedding` plus Inbox completion. The final RAG/OpenSearch write remains unverified by deliberate Phase 1 scope decision. | Jaeger evidence 2026-08-28; outbox/inbox tests | Local full-infra partial | Embedding yes | Outbox/relay/Worker/embedding boundary | Do not claim complete projection or alias cutover |
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
technical boundary is the final PostgreSQL/OpenSearch projection and complete
citation-field payload; both are explicitly partial and intentionally do not
block the Phase 1 resume/demo package.

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

Jaeger/Collector export, explicit server/worker naming, health tracing,
authenticated MiniMax Chat, and outbox-to-Worker-to-embedding spans are locally
real. The final PostgreSQL/OpenSearch projection is not complete in this run,
so the overall claim remains local full-infra partial, not full end-to-end.

## Reproduction commands and evidence index

See `2026-08-27-jaeger-end-to-end-evidence.md` for telemetry commands and
`2026-08-26-rag-real-minimax-differential-evidence.md` plus
`2026-08-26-rag-provenance-and-current-corpus-v1.md` for provider/RAG evidence.
Run backend gates with `go build ./...`, `go vet ./...`, `go test ./...`, and
`bash scripts/verify-project.sh --full`.

## Production gaps and future work

The Phase 1 package can proceed with the captured local evidence. Future work
may capture a complete async projection trace in an isolated generation/index,
repeat against production-like Collector/Jaeger and OpenSearch, and evaluate a
reranker. None of those are required for the current resume/demo claim, and
ClamAV inventory rescan remains future work.
