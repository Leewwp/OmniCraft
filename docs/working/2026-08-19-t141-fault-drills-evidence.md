# T06 (#141) Fault-Drill Evidence

> Created: 2026-08-19
> Expected expiry: 2026-10-19
> Scope: local development only; this record is not production evidence.

## Environment

- Worktree: `OmniCraft-wt-141`
- Branch: `codex/141-hybrid-retrieval`
- PostgreSQL and Redis: local Docker services, healthy.
- OpenSearch: `opensearchproject/opensearch:2.19.3`, local container `omnicraft-opensearch`, reachable at `http://127.0.0.1:9200`.
- Query and corpus embeddings: deterministic SHA-256 stand-in because no real embedding provider credentials are configured.
- Golden set: 63 cases loaded from `backend/testdata/rag_eval_seed.sql`.

## Results

1. Real OpenSearch repository contract passed:

   ```text
   OMNICRAFT_OPENSEARCH_URL=http://127.0.0.1:9200 \
   go test ./internal/repository -run TestOpenSearchRepositoryRealContainerContract -count=1 -v
   PASS
   ```

   The contract covered health, strict index mapping, bulk replacement, validation, alias swap/read, and deletion.

2. OpenSearch-down fallback drill passed:

   ```text
   docker stop omnicraft-opensearch
   OMNICRAFT_RAG_HYBRID_EVAL=1 \
   OMNICRAFT_RAG_EMBEDDING_MODE=standin \
   go test ./internal/service/rag_eval -run TestHybridGoldenSetEval -count=1 -v
   docker start omnicraft-opensearch
   ```

   The run returned `PASS`, `degradation=1.000`, and `degraded=keyword_pg` through the unavailable OpenSearch path. The container was restarted and returned yellow health before the recovery contract rerun.

3. Embedding failure and visibility failure paths are covered by deterministic injected-error tests:

   ```text
   go test ./internal/service/rag -run 'TestHybridRetriever' -count=1 -v
   ```

   The tests cover keyword-only fallback, vector-only direct return, both-provider failure, visibility fail-closed behavior, top-20 visibility checks, and ordered one-at-a-time backfill. No external embedding API was stopped because none is configured in this local environment.

4. OpenSearch recovery consistency:

   ```text
   curl -fsS 'http://127.0.0.1:9200/_cluster/health?wait_for_status=yellow'
   OMNICRAFT_OPENSEARCH_URL=http://127.0.0.1:9200 \
   go test ./internal/repository -run TestOpenSearchRepositoryRealContainerContract -count=1 -v
   ```

   Both health and the real repository contract passed after restart.

## Golden-set measurements

The same-run local baseline gate passed with the current local corpus:

| Run | Recall@10 | MRR | nDCG@10 | Visibility leaks |
| --- | ---: | ---: | ---: | ---: |
| Keyword-only | 0.413 | 0.370 | 0.380 | 0 |
| Vector-only | 0.365 | 0.281 | 0.303 | 3 |
| Hybrid RRF, OpenSearch unavailable | 0.429 | 0.371 | 0.385 | 0 |

The real OpenSearch-backed run used the same stand-in embedding and measured `Recall@10=0.476`, `MRR=0.365`, `nDCG@10=0.393`, and zero leaks. Fixed RRF missed the same-run keyword MRR by `0.005` on four ambiguous/duplicate cases. The test records this as a stand-in gate gap and would fail for a non-stand-in provider. It must not be described as production acceptance.

The committed historical baseline artifact reports a larger corpus and different local measurements. The test logs that comparison separately; it does not overwrite the artifact or treat the drift as a pass.

## Remaining environment gap

Production-level T06 evidence still requires a real embedding provider and a production-like corpus rerun:

```text
OMNICRAFT_RAG_HYBRID_EVAL=1 \
OMNICRAFT_RAG_EMBEDDING_MODE=real \
OMNICRAFT_RAG_OPENSEARCH_URL=http://<real-opensearch> \
go test ./internal/service/rag_eval -run TestHybridGoldenSetEval -count=1 -v
```

The command above is a reproducible shape only; `<real-opensearch>` and provider credentials are intentionally not configured in local development. The local real-mode attempt fails during projection with `content projection unavailable` because no provider credentials are configured. No local mock or stand-in result in this document is a production claim.
