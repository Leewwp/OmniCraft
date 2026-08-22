# MiniMax Real Provider and RAG Evidence

> Created: 2026-08-22
> **预计失效日期**: 2026-10-22
> Scope: local development evidence only

## Provenance

- Commit: 10fa0ab
- Provider: minimax
- Chat model: MiniMax-M3
- Chat base: https://api.minimaxi.com
- Embedding model: embo-01
- Embedding base: https://api.minimax.chat
- Embedding route: /v1/embeddings
- Embedding request: type=query, GroupId query parameter present; the value is intentionally not recorded
- Golden set: 63 active cases
- Golden set checksum: sha256:e759d965ae70cb2bd0cb1022735207c307b13b6d23c9946dbc88c01225521b04

The repository preflight passed after its output was hardened to report only
api_key=present and embedding_group_id=present. No key content is included
in this record. Running go run ./cmd/agent-smoke from backend/ loaded the
repository root environment file and reported MiniMax-M3; the stale
backend/.env was not selected.

## Direct Embedding Contract

The direct MiniMax request returned HTTP 200 with provider status 0, one
vector, and vector dimension 1536. The request used embo-01 and included
the GroupId query parameter. The response body and vector values were not
recorded.

The dedicated real-provider RAG database was
omnicraft_test_minimax_20260822. OpenSearch generation omnicraft-rag-v2 was
used during the run. PostgreSQL verification after the run showed:

| Generation | Embedding model | Vectors | Min dimension | Max dimension | State |
|---|---|---:|---:|---:|---|
| 2 | embo-01 | 169 | 1536 | 1536 | 169 ready/current rows |

The fixed OpenSearch read alias was restored to omnicraft-rag-v1 after the
evaluation. The real vectors remain isolated in the dedicated database and
generation. No stand-in vector was inserted into generation 2.

## Chat Smoke

go run ./cmd/agent-smoke completed with 4/4 cases successful. Only
case-level metadata was retained.

| Case | Status | Latency | Input tokens | Output tokens | Requested tools | Estimated cost |
|---|---|---:|---:|---:|---|---:|
| cited_search | success | 1857 ms | 454 | 155 | search_content | $0.0004595 |
| no_evidence | success | 4914 ms | 199 | 400 | none | $0.0006995 |
| injection | success | 7167 ms | 218 | 398 | none | $0.0007060 |
| timeout_downgrade | success | 5732 ms | 414 | 81 | search_content | $0.0003285 |

Totals were 1285 input tokens, 1034 output tokens, and an estimated
$0.0021935. The estimate uses the smoke command's generic fallback rate of
$0.50/M input tokens and $1.50/M output tokens because no MiniMax-specific
price was configured. It is not a billing record. The embedding response did
not expose token usage, so no embedding token or cost estimate is claimed.

No provider error occurred in the four smoke cases. The injection case did not
request a tool. User questions, model output, API keys, and raw provider
responses were not retained.

## Real RAG Evaluation

Command shape:

~~~bash
OMNICRAFT_RAG_HYBRID_EVAL=1 \
OMNICRAFT_RAG_EMBEDDING_MODE=real \
OMNICRAFT_RAG_OPENSEARCH_URL=http://127.0.0.1:9200 \
OMNICRAFT_RAG_EVAL_DSN=<dedicated local loopback DSN> \
go test ./internal/service/rag_eval -run TestHybridGoldenSetEval -count=1 -v
~~~

The run completed real embedding projection for 63 golden cases, 169 local
published corpus contents, and 169 chunks. The retrieval result was:

| Metric | Real MiniMax hybrid |
|---|---:|
| Recall@10 | 0.508 |
| MRR | 0.440 |
| nDCG@10 | 0.456 |
| Citation precision | 0.052 |
| Citation coverage | 0.508 |
| Visibility leak count | 0 |
| P95 retrieval latency | 389.8 ms |
| Degradation success rate | 1.000 |

This is retrieval-only evidence. groundedness, answer relevance, and answer
token usage were not evaluated by this test.

The same-run keyword baseline was:

| Metric | Same-run keyword |
|---|---:|
| Recall@10 | 0.413 |
| MRR | 0.370 |
| nDCG@10 | 0.380 |

The same-run hybrid comparison was above the current keyword baseline and
visibility leaks were zero. The overall real-provider gate nevertheless
failed. The harness rejected comparison with the committed baseline because
the committed baseline corpus identity is 253 contents while this dedicated
evaluation corpus is 169 contents. The test therefore returned FAIL; this
record preserves that failure and does not relabel it as a pass. The committed
baseline keyword values are not treated as a valid cross-corpus comparison.

The test's same-run vector baseline reported zero because it measures the
legacy content-level vector table, while this run deliberately populated the
chunk-level real generation. That result is not evidence that MiniMax returned
zero-dimensional or empty vectors; the direct contract and generation audit
confirmed 169 real 1536-dimensional vectors.

## Stand-in Boundary

The prior deterministic stand-in uses a local SHA-256 token projection and
does not call an external provider. It is reproducible and useful for local
contract tests, but it has no provider latency, quota, model semantics, or
embedding API failure behavior.

This run used real MiniMax HTTP calls for chat and embedding. It has provider
latency and quota behavior, but its retrieval numbers are local measurements
over a dedicated seed corpus. The real and stand-in vectors were kept in
different index generations; no production quality, QPS, SLA, deployment, or
online metric claim follows from this document.

## Setup Notes

The first evaluation attempt stopped before provider projection because the
dedicated rich seed lacked active/latest content_versions. The dedicated
database was repaired using the existing migration 071 initialization
contract, then the real run was repeated from an empty generation. This setup
failure is not classified as a MiniMax provider error.

The host backend launch attempt also encountered an already occupied local
:9091 metrics port and exited. It did not affect the provider smoke or the
standalone RAG evaluation. Jaeger full-chain and ClamAV/EICAR evidence remain
out of scope for this run.

## Repository Validation Note

The focused provider and RAG checks above completed. The supplemental full
project gate was not clean: `bash scripts/verify-project.sh --full` ended with
56 passed, 17 failed, and 1 not run. The failures were in mocked UI and
real-HTTP contract suites, including an occupied local `:18080` test port;
they are recorded here as validation failures and are not counted as MiniMax
provider failures.
