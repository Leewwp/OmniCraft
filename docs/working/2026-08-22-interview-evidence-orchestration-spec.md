# 面试证据补充与开发编排规格

Part of #22

> Created: 2026-08-22
> **预计失效日期**: 2026-10-22
> Scope: local development and interview evidence only; not a production release plan.

## Phase 1 execution alignment (2026-08-27)

The user confirmed that the first-phase delivery window ends around 2026-09-03,
with target roles ordered as AI application development, Agent development, and
AI full-stack development. #207 T01/T02/T03 are now the active implementation
track; this evidence ticket remains a downstream closure track for the same
local interview package. The repository-root `.env` contains the user's real
MiniMax configuration. Provider evidence may be refreshed locally, but secrets
must never be persisted in Git, logs, screenshots, or reports.

## Current decision update (2026-08-28)

The Phase 1 implementation slice is complete: #207 T01/T02/T03 was closed on
2026-08-28. The remaining Phase 1 work is evidence and interview packaging in
#204 and #32, not another core Agent/RAG runtime feature.

OpenSearch projection is **not a Phase 1 completion requirement**. The current
repository already has PostgreSQL FTS/pg_trgm, pgvector, RRF, visibility
revalidation, OpenSearch projection contracts, and PostgreSQL fallback. The
default `rag_hybrid_enabled=false` path and the authenticated MiniMax Chat SSE
demo do not require a real OpenSearch read alias. The real Worker embedding
trace proves provider execution, but the final RAG-generation/OpenSearch write
is not verified and must not be completed by mutating the baseline generation
or alias during interview preparation.

Any future OpenSearch projection work belongs in Phase 2/#208 as an isolated,
reversible experiment: a new generation and index, a small local corpus, no
initial alias cutover, explicit mapping/model/dimension/version checks, and a
recorded rollback target. A production-like OpenSearch deployment is justified
later by scale, query isolation, complex filtering, or index-lifecycle needs;
its enterprise popularity alone does not make it a current release gate.

## Problem Statement

当前 OmniCraft 的 RAG、回答 Agent、可靠异步和 OTel 实现主线已经完成本地实现与自动化合同，但面试证据仍有两个关键缺口：

1. MiniMax 真实 provider 运行已经证明 Chat/Embedding HTTP 合同可用，但真实 hybrid 评测的 `citation_precision=0.052` 与已提交 keyword baseline 的 `0.913` 不能直接比较。现有运行同时存在 content-level 与 chunk-level 检索对象、不同有效 corpus、不同 top-K 和不同可见性统计口径。若不先归因，任何“真实 embedding 质量提升/下降”的简历表述都不可靠。
2. OTel 应用层合同、W3C propagation、队列/DB/LLM span 已实现，但真实 Collector 投递和 Jaeger UI 可视化尚未完成。固定的 Collector 镜像 tag 当前无法从 Docker Hub 拉取；此外，当前 Agent 问答是 API 进程内同步调用 MiniMax，不经过 Worker，不能直接声称单个问答请求形成“HTTP → Redis Streams → Worker → MiniMax LLM → 返回”的链路。

面试导向要求优先补齐能证明排查能力、真实外部 provider 接入、异步系统可观测性和边界判断的证据；生产发布冻结门、遗留 favorites 云端 cutover、存量恶意文件回扫和桌面能力不属于当前优先范围。

## Solution

建立一条可复核、可解释、面试优先的证据编排路线：

1. 先恢复或重建 253-content benchmark 的 provenance，封存代码、fixture、corpus identity、chunking/index 配置和评测参数；如果无法恢复历史 253 corpus，就创建新的明确版本，不把新 corpus 冒充旧 baseline。
2. 在同一 corpus 上运行 content-level legacy baseline、chunk-level keyword、deterministic hybrid 和 MiniMax hybrid 的差分矩阵，同时输出 K=10/K=20、raw hit 与按 content 去重后的 citation 指标。
3. 对代表性 case 生成脱敏 Top-K 诊断报告，区分指标口径问题、候选数量/截断问题、visibility/corpus 问题和真实 embedding 语义发散。只有最后一种被证实后，才把 cross-encoder rerank 作为后续架构优化方向。
4. 修复并验证一个固定版本的 OTel Collector/Jaeger full-infra 栈，不使用 `latest`。真实证据分为同步 Agent Chat trace 和异步内容投影 trace；不为了截图改变当前模块化单体与独立 Worker 的架构边界。
5. 将两批证据合并到 #32 的简历与 live demo 叙事中，并明确区分 implemented、local real provider、local full-infra、mocked contract 和 production evidence。

## User Stories

1. As an interview candidate, I want to reproduce the historical benchmark identity, so that I can defend every metric with a stable corpus and code revision.
2. As an interview candidate, I want to know whether the 253-content corpus is recoverable from existing seeds or requires a historical database snapshot, so that I do not make a false reproducibility claim.
3. As an evaluator, I want the golden-set checksum and corpus identity to be recorded together, so that a passing metric cannot be detached from the data that produced it.
4. As an evaluator, I want content-level and chunk-level retrieval runs to be labeled separately, so that citation precision is not compared across incompatible entities.
5. As an evaluator, I want K=10 and K=20 results for every relevant retriever, so that top-K truncation cannot silently explain a metric delta.
6. As an evaluator, I want raw hit counts and content-deduplicated hit counts, so that denominator inflation from multiple chunks is visible.
7. As an interview candidate, I want representative Top-K chunks with titles, headings, source offsets and redacted text excerpts, so that I can explain a retrieval failure using concrete evidence.
8. As an evaluator, I want old baseline vectors preserved while real MiniMax vectors are written to a new model/generation, so that re-embedding does not destroy the comparison point.
9. As an interview candidate, I want a falsifiable diagnosis of the citation precision drop, so that I can distinguish metric-contract drift from genuine semantic retrieval weakness.
10. As an interview candidate, I want to state a reranker proposal only when the same-corpus, same-K experiment proves semantic candidate drift, so that the architecture follow-up is evidence-driven.
11. As an operator, I want the Collector image and configuration pinned and validated, so that the Jaeger evidence can be rerun on another machine.
12. As an operator, I want Collector health checks to reflect the actual enabled extensions, so that a healthy process is not confused with a reachable telemetry pipeline.
13. As an interview candidate, I want a real synchronous Agent trace from HTTP through database and MiniMax Chat to SSE, so that the GenAI span contract is demonstrated with a real provider.
14. As an interview candidate, I want a real asynchronous projection trace from HTTP/outbox through relay, Redis Stream, Worker, MiniMax Embedding and PostgreSQL/OpenSearch, so that reliable async processing and W3C propagation are visible in Jaeger.
15. As an interview candidate, I want the evidence to state that Agent Chat is synchronous while content projection is asynchronous, so that the demo reflects the actual architecture rather than a fabricated chain.
16. As a reviewer, I want trace IDs, request IDs, provider model names and latency visible without prompts, API keys or raw content leakage, so that the evidence is safe to share.
17. As a maintainer, I want the new evidence work to consume the already completed #135~#150 implementation rather than reopen completed runtime tickets, so that the repository history remains coherent.
18. As a maintainer, I want #134 and #76 to remain production-only gates, so that local interview work is not blocked by deferred deployment decisions.
19. As an interview candidate, I want the final resume/demo package to distinguish real provider contracts, local retrieval measurements, OTel application contracts and real Jaeger UI evidence, so that no local result is overstated as production evidence.

## Implementation Decisions

- The primary interview task is a new evidence/diagnosis slice, not a new product feature. It consumes the completed RAG, Agent, Worker and OTel seams.
- Benchmark provenance is a first-class artifact. An annotated Git tag may freeze repository state, but the tag must be paired with a corpus manifest containing content ID checksum, searchable/published predicates, golden-set checksum, seed revision, chunking version, index version, embedding model and K values.
- The historical `253` corpus must not be inferred from the current local database. The current local state is known to differ, and the existing baseline artifact is historical evidence rather than a complete corpus snapshot.
- The existing baseline remains immutable. Real MiniMax vectors are written into an isolated RAG generation and model identity; legacy content-level vectors are not overwritten.
- The evaluation matrix compares like with like: legacy content-level keyword/vector runs are reported separately from chunk-level keyword/hybrid runs. Retrieval metrics use content identity; chunk citation metrics require a chunk-level oracle and are not invented from content-level expected citations.
- Citation reporting has two explicit views: raw produced-hit precision and content-deduplicated precision. The report must show both when a chunk retriever is evaluated against content-level expected citations.
- The evaluation keeps visibility leaks as a hard safety metric. A quality improvement with non-zero visibility leaks is rejected for interview claims.
- Top-K is an experiment parameter, not an implicit configuration detail. The report records candidate pool sizes, final K, deduplication policy and the exact retriever version.
- Top-K inspection uses the highest existing evaluation seam: the retrieval evaluator and hybrid retriever contract. A new production API is unnecessary; a diagnostic report adapter may be added only if the existing evaluator cannot expose the required redacted fields.
- Reranking is a conditional follow-up. It is out of the critical path until a same-corpus, same-K run proves that semantically related but answer-insufficient chunks dominate the result.
- Collector selection is pinned and validated. `latest` is not acceptable for evidence. The selected image must contain the configured OTLP receiver, batch processor and OTLP exporter, and its binary/config path and healthcheck must be verified.
- Collector health checking must be internally consistent: the health-check extension is enabled when the Compose healthcheck probes its endpoint, or the healthcheck is changed to a supported image-native probe.
- Jaeger evidence does not change the architecture. The synchronous trace is HTTP → DB → MiniMax Chat → SSE. The asynchronous trace is HTTP/outbox → relay → Redis Stream → Worker → MiniMax Embedding → PostgreSQL/OpenSearch. A single Agent Chat request is not described as passing through Worker.
- OTel evidence keeps `X-Request-ID` separate from the W3C/OTel `trace_id`. GenAI spans expose model, usage, temperature and safe status metadata only; prompts, API keys, raw provider bodies and unredacted content are excluded.
- The work is organized as two heavy evidence tasks: benchmark diagnosis first, Jaeger full-chain second. Each task has one evidence commit after its focused tests and validation gates.
- Existing implementation tickets #135~#150 remain closed/completed. #144 and #32 are evidence-closure consumers; #145 is already a decision gate and is not reopened.

### Interview-priority orchestration

1. P0: benchmark provenance recovery and citation diagnosis. This has the highest interview value because it demonstrates detecting an invalid comparison instead of tuning a number.
2. P1: reconcile the captured authenticated Chat and Worker evidence, and decide whether any additional trace is worth the risk before the interview deadline. The final OpenSearch projection is optional and must not block the package.
3. P2: update #32 resume/demo evidence with the diagnosis, metrics matrix, Top-K examples, screenshots, and interview answers.
4. P3: keep Phase 2 candidates (OpenSearch projection, reranker, production-like Collector/Jaeger and real security/provider evidence) explicitly deferred until a new spec/plan is registered.
5. P4: optional ClamAV/EICAR full-infra evidence only if the target role or resume narrative emphasizes content safety/security scanning.
6. Deferred: #134 production freeze closure, #76 legacy favorites cloud cutover, #151 historical archive rescan and desktop/Tauri work. These do not improve the current interview signal enough to displace P0–P3.

### Dependencies and sequencing

- P0 depends on the completed T01/T04/T05/T06 evaluation and projection contracts, and must not modify their historical baseline artifact until the diagnosis is complete.
- P1 consumes the already captured local MiniMax Chat and Worker evidence; it does not require an OpenSearch alias cutover.
- P2 consumes the completed T03 Worker and T08 OTel application contracts and updates the interview evidence package; it does not reopen completed implementation tickets.
- P3 is a Phase 2 planning decision only and requires a new spec/plan before any isolated projection or production-like infrastructure work starts.
- #134 remains a production release gate only in the current local-development mode. It does not block P0–P3.

## Testing Decisions

- The main red-capable diagnosis test is the existing golden-set retrieval evaluator run with an explicit corpus manifest, embedding mode, retriever version and K. It must fail on identity mismatch and report the mismatch before any metric comparison.
- Differential tests cover the same golden cases through legacy keyword, legacy vector, chunk keyword, deterministic hybrid and real MiniMax hybrid paths. The test asserts that raw and deduplicated citation metrics are labeled separately.
- Projection tests assert that real MiniMax vectors use the selected model/generation, have the expected dimension, and do not overwrite or mix with legacy vectors.
- The Top-K diagnostic report is tested for deterministic ordering, redacted text output, expected citation visibility and absence of secrets. It tests external report behavior rather than internal helper calls.
- OTel Compose validation tests assert that the pinned image can start, the Collector config validates, the health endpoint is reachable, and Jaeger receives a real trace.
- Browser/API evidence uses the existing authenticated login and Agent/content routes. No auth or CSRF bypass is allowed to manufacture a trace.
- Jaeger verification must assert a trace query returns the expected service/span set, that the OTel `trace_id` matches the API/SSE evidence, and that `X-Request-ID` remains distinct.
- Existing prior art is reused: the golden-set contract/evaluation suites, projection generation audits, T03 queue propagation tests, T08 OTel span tests, authenticated live-demo checks and the existing evidence archive format.
- Repository gates remain proportional but mandatory after implementation: focused Go tests first, then `go test ./...`, `go vet ./...`, `go build ./...`, Compose config validation, doc-validator when configuration changes, and the relevant live API/Jaeger verification.

## Out of Scope

- Do not tune thresholds or alter the metric definition solely to make `citation_precision` increase.
- Do not overwrite the committed legacy baseline or erase historical provider/stand-in evidence.
- Do not claim production traffic, production quality, SLA/QPS, groundedness or answer relevance from retrieval-only local runs.
- Do not introduce a cross-encoder reranker before the diagnosis proves semantic candidate drift.
- Do not add an async Agent job architecture solely to create a screenshot.
- Do not use an unpinned Collector `latest` image or treat a failed image pull as Jaeger completion.
- Do not reopen completed #135~#150 implementation tickets, #145, or production-only #134/#76 work for this evidence slice.
- Do not perform ClamAV/EICAR, historical archive rescan, production deployment, desktop/Tauri or unrelated UI work unless the interview narrative explicitly changes.

## Further Notes

- The current evidence supports resume claims about implementing real MiniMax provider contracts, versioned hybrid RAG, visibility-leak protection, provider fallback, OTel application contracts, and a locally captured synchronous Chat plus async Worker/embedding boundary. It does not support a retrieval-uplift claim or a complete PostgreSQL-to-OpenSearch projection trace.
- The strongest interview narrative is the diagnosis itself: the apparent `0.913 → 0.052` drop led to discovery that the numbers came from different retriever entities, corpus predicates, candidate cardinalities and citation semantics.
- If the same-corpus corrected run still shows weak fine-grained citation precision, the honest conclusion is a semantic retrieval bottleneck and a justified reranker follow-up, not a fabricated pass.
- This document is a local interview-evidence plan. The project is sufficient for scoped resume writing and a guarded local demo; production deployment gates and any real OpenSearch cutover remain governed by the active registry and Phase 2 planning rules.
