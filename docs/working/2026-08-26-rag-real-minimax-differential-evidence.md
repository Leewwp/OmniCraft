# MiniMax 真实 Embedding Differential 证据

> Created: 2026-08-26
> **预计失效日期**: 2026-10-26
> Scope: local interview evidence; not a production release artifact

## 结论

历史 `253-content` / `content-level` keyword baseline（citation precision `0.913`）与本轮
真实 MiniMax 结果不是同一实验，不能写成 `0.913 -> 0.052` 的 provider 质量回归。历史
语料快照不可恢复；本轮使用新命名的 `current-local-published-v1`，并固定了 63 条 golden
case、169 条 published/non-deleted content、169 个 chunk、`embo-01` 1536 维向量和
generation 2。

## 可复现实验契约

- Database: `omnicraft_test_minimax_20260822`（隔离本地库）
- Corpus identity: `sha256:240b3bac94916e791f41436abf9ab73fbb39702cd9b25f86cda264990a3765be`
- Golden-set identity: `sha256:e759d965ae70cb2bd0cb1022735207c307b13b6d23c9946dbc88c01225521b04`
- Retriever: `hybrid-rrf-pg-fallback-v1`，chunk entity，BM25/vector candidate K=200，RRF K=60
- Provider: MiniMax `embo-01`；query embedding 为真实 provider 调用；报告不含 provider secret
- Reports: `2026-08-26-rag-citation-diagnosis-real-k10.json`、
  `2026-08-26-rag-citation-diagnosis-real-k20.json`

运行时使用 `RAG_HYBRID_FINAL_TOPK=10|20` 选择 final K；该覆盖有配置单测，不能修改历史
baseline。评测末尾仍会因为历史 253-content gate 被拒绝，这是预期的 provenance guard，
不是 provider 请求失败。

## 结果

| final K | Recall@10 | MRR | nDCG@10 | citation precision | citation coverage | mean hits | p95 retrieval |
|---:|---:|---:|---:|---:|---:|---:|---:|
| 10 | 0.492 | 0.437 | 0.450 | 0.170 | 0.492 | 6.46 | 164.8 ms |
| 20 | 0.492 | 0.438 | 0.450 | 0.162 | 0.508 | 9.97 | 163.8 ms |

两档均为 63 cases、169 contents/chunks、0 visibility leak、degradation success rate `1.000`。
同一运行内的 chunk keyword baseline 为 Recall@10 `0.413`、MRR `0.370`、nDCG@10 `0.380`；
legacy content-level vector baseline 在隔离库中没有 `content_embeddings`，全零是存储对象缺失，
不得解读为 MiniMax 失败。

## 归因判断

K=20 只让 coverage 从 `0.492` 增至 `0.508`，citation precision 从 `0.170` 降至 `0.162`，
且平均返回 hits 接近 10。因而“旧运行只取了 Top-1/Top-3”不是主要解释；结果更支持：

1. 历史与当前的 content/chunk entity、corpus size、candidate cardinality 和可见性统计口径不一致；
2. 在统一 current-v1 口径后，MiniMax 语义排序对细粒度 citation 的命中仍不稳定；
3. 目前可以提出“需要进一步做 chunk-level citation 对齐、query/answer 证据判定和 rerank
   评估”的工程方向，但不能把 reranker 说成已验证的修复，也不能宣称真实 embedding 的生产收益。

## 面试表述边界

可以写：完成真实 MiniMax Chat/Embedding provider contract 和 current-v1 differential 评测闭环，
定位历史 baseline 与真实 hybrid 指标不可比，并用 K=10/K=20 对照排除了单纯 Top-K 截断解释。

暂不写：真实 MiniMax 在 253 条语料上的 Recall、`0.913 -> 0.052` 回归、生产质量/QPS/SLA，
或“已通过 reranker 修复”。
