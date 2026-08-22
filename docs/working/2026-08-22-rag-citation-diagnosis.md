# RAG Citation Precision 归因记录

> Created: 2026-08-22
> **预计失效日期**: 2026-10-22
> Scope: local interview evidence; not a production quality report

## 结论

当前不能把 `citation_precision=0.913` 与 MiniMax 真实 hybrid 的 `0.052`
写成“真实 embedding 使精度下降”。两者不是同一评测对象：

| 维度 | 已提交历史 baseline | 当前 hybrid 诊断运行 |
|---|---|---|
| 检索对象 | content-level | chunk-level |
| 有效 corpus | 253 contents / 201 embeddings | 169 published contents / 169 chunk embeddings |
| final K | keyword 观测最大 5；vector 观测最大 20 | 10 |
| citation identity | content/version | chunk hit 按 content/version 归并后再与 content-level oracle 比较 |
| visibility/corpus scope | 历史 artifact 的固定环境 | `content_items.status=published AND deleted_at IS NULL`，再经过 viewer visibility filter |

这四项差异已经足以使两个 precision 数字不可直接比较。当前本地诊断报告
还显示 raw 与 content-deduplicated precision 都是 `0.043`，平均每 case
产生 10 个 hit、去重后仍为 10 个；因此“重复 chunk 造成分母膨胀”不是本次
主要原因。

## 已运行证据

诊断命令使用 deterministic stand-in，仅用于验证诊断闭环，不代表 MiniMax
质量：

```bash
cd backend
OMNICRAFT_RAG_HYBRID_EVAL=1 \
OMNICRAFT_RAG_EMBEDDING_MODE=standin \
OMNICRAFT_RAG_DIAG_REPORT=/absolute/path/to/docs/working/2026-08-22-rag-citation-diagnosis-standin.json \
go test ./internal/service/rag_eval -run TestHybridGoldenSetEval -count=1 -v
```

本次运行结果：63 cases，Recall@10 `0.429`，MRR `0.372`，nDCG@10 `0.386`，
citation precision `0.043`，coverage `0.429`，visibility leaks `0`。同一运行的
keyword baseline Recall@10 为 `0.413`，vector baseline 为 `0.365`；这些数字只
能说明当前 169-content fixture 上的相对行为，不能回填历史 253-content
baseline。

脱敏 Top-K 报告保存在
`docs/working/2026-08-22-rag-citation-diagnosis-standin.json`，只保留 8 个
代表性 case 的 chunk excerpt，完整 63-case 指标仍写入 `current_metrics`。
报告记录了 golden-set checksum、content-ID checksum、chunking/index/model、
BM25/vector/final K 和历史 baseline identity。

## 归因判断

1. **已确认：实体不一致。** 历史 keyword/vector artifact 是 content-level，
   当前 hybrid 是 chunk-level。content-level expected citations 不能直接当作
   chunk-level 的精确命中 oracle。
2. **已确认：corpus 不一致。** 当前仓库和本地数据库无法重建历史 253-content
   快照；现有 artifact 是历史结果，不是 corpus 快照。当前报告明确标记 169，
   不伪造 253。
3. **已确认：候选数量不一致。** keyword、vector 与当前 hybrid 的 K/观测 hit
   数不同；任何 precision 分母比较都必须先锁定 K。
4. **尚未确认：真实 embedding 语义发散。** stand-in 诊断只能证明上述口径问题，
   不能推出 rerank 必要性。只有同一 corpus、同一 chunking/index、同一 K、同一
   visibility predicate 下 MiniMax 与 keyword/chunk baseline 仍出现“语义相近但
   不含答案”的 Top-K，才把 cross-encoder rerank 作为证据驱动的后续方案。

## 简历边界

现在可以写：

- 完成 MiniMax Chat/Embedding HTTP contract，`embo-01` 返回 1536 维向量；
- 建立带 corpus、K、实体和脱敏 Top-K 证据的 RAG 评测诊断闭环；
- 发现并解释历史 `0.913` 与当前 `0.052` 不可直接比较的 provenance/metric
  contract 问题，且 visibility leaks 为 0。

现在不能写：

- “MiniMax embedding 将 citation precision 从 0.913 降到 0.052”；
- “真实 embedding 在 253 条语料上达到/超过某个 Recall 或 citation uplift”；
- 生产质量、线上 QPS、SLA、groundedness 或 answer relevance。

## 后续编排

### P0：封存 provenance（当前进行中）

- 保持 `backend/testdata/rag_eval_baseline.json` 不变；
- 从历史数据库备份或导出中寻找 253-content corpus manifest；
- 若找不到，正式创建新的 `current-corpus-v1` baseline，不把 169 冒充 253；
- 为每个运行固定 golden checksum、content-ID checksum、scope、chunking、index、
  embedding model、BM25/vector/final K。

### P1：同口径真实 MiniMax differential

- 使用独立数据库和新 generation/model identity，保留历史 content-level vectors；
- 在同一 corpus 上运行 content-level legacy、chunk keyword、deterministic hybrid、
  MiniMax hybrid；
- 固定 K（至少 K=10，必要时追加 K=20），同时输出 raw/deduplicated citation；
- 抽检代表性 Top-5，先做归因再决定是否提出 rerank。

### P2：Jaeger full-chain

- 使用 pinned Collector 镜像和实际启用的 healthcheck；
- 采集同步 Agent Chat trace（HTTP → DB → MiniMax Chat → SSE）以及异步投影
  trace（HTTP/outbox → relay → Redis Stream → Worker → MiniMax Embedding → DB/OpenSearch）；
- 不把同步 Agent Chat 描述成经过 Worker；
- 真实 UI trace 截图完成后，才更新 #32 面试素材。

ClamAV/EICAR、生产发布门、遗留 favorites 云端 cutover、桌面/Tauri 和历史存量
回扫继续保持低优先级或 deferred，不抢占 P0-P2。
