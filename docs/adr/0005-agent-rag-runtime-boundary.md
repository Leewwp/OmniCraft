# Agent/RAG 运行时边界：模块化单体 + 独立 Worker

- **状态**：accepted；T10 复核于 2026-08-19 完成，当前决策维持
- **日期**：2026-08-11
- **决策所有者**：GitHub #30/#28/#31
- **复核入口**：RAG 路线图 T10

## Context

OmniCraft 当前是 Go/Gin 模块化单体，Agent 通过站内工具与 SSE 提供能力。面试展示需要可靠异步、混合检索、引用安全与可观测性，但当前没有独立扩缩容或跨团队发布证据足以证明拆分 Agent/Search 微服务的必要性。过早拆服务会新增部署、契约、网络失败与数据一致性成本，也会弱化对真实业务闭环的投入。

## Decision

1. HTTP API、Agent orchestration、RAG retriever 和 OpenSearch adapter 保持同一 Go 代码库内的深模块；PostgreSQL 是业务与评测真相源，OpenSearch/embedding 是可重建投影。
2. 只抽出有明确故障隔离价值的独立 `cmd/worker` 进程，承载 Outbox relay、Redis Streams 消费、索引和扫描任务。server 不保留进程内 Worker fallback。
3. server 与 Worker 通过数据库 Outbox/Inbox 和 Redis Streams 交互；外部 Agent 合同继续使用 REST + SSE，不引入 gRPC。
4. 当前运行时只实现单 Agent + 确定性 RAG 工作流。`Research Agent -> Answer Agent` 仅作 T10 对照实验，不构建通用多智能体平台。

## Consequences

- 本地默认 Compose 仍需 backend + Worker，但 OpenSearch/OTel/Jaeger/ClamAV 只在对应 full-infra/security profile 启动。
- 模块接口必须允许替换检索投影与测试故障注入，但不得为未来微服务预建空 RPC 层。
- 可在简历中描述“设计了可演进边界”；只有 Worker、RAG、追踪和评测真实完成后才能写“实现”。

## T10 Review (2026-08-19)

T09 的本地 closure evidence 已复核：63 个 golden cases、34 个本地投影内容，hybrid `Recall@10=0.476`、`MRR=0.365`、`nDCG@10=0.393`、visibility leak count `0`，最新本地 retrieval P95 为 `5.8 ms`。这些数字使用 deterministic SHA-256 embedding stand-in 和本地 OpenSearch/fixture corpus；同 run keyword baseline 的 MRR 为 `0.370`，因此 RRF 指标门未由 stand-in 满足。证据包明确没有真实 embedding provider、LLM answer evaluation、生产样本或 Jaeger 全链可视化。

基于这组证据，T10 不触发 Agent/Search 拆分，也不引入 gRPC。HTTP API、Agent orchestration、RAG retriever 和 OpenSearch adapter 继续作为模块化单体内的深模块，独立 `cmd/worker` 继续承担异步故障隔离边界，外部 Agent 合同继续使用 REST + SSE。当前没有独立扩缩容资源曲线、跨团队所有权、独立发布边界或网络化成本优于进程内调用的测量证据。

`Research Agent -> Answer Agent` 对照实验未晋升为 Implemented：T09 没有真实 LLM answer-eval 或同一 golden set 的双节点对照结果，无法证明引用正确率、groundedness、relevance、P95、token 成本和工具失败率同时满足晋升条件。当前运行时维持单 Agent + 确定性 RAG；未来只有在补齐真实 provider/生产样本并满足本 ADR 的触发条件后，才重新评估拆分或双节点 workflow。

Evidence: `docs/working/2026-08-19-t144-rag-closure-evidence.md`, `docs/working/2026-08-19-t144-rag-closure-metrics.json`, `docs/working/2026-08-19-t144-rag-closure-raw.txt`.

## Revisit Triggers

T10 仅在 T09 有真实指标后复核。满足下列至少一项才考虑拆 Agent/Search + gRPC：

- Agent/Search 需要独立扩缩容，且资源曲线与 API server 显著不同；
- 检索或模型故障无法通过模块隔离/超时/降级控制，影响核心 Web SLO；
- 独立发布频率、团队所有权或安全边界已形成；
- 网络化后的延迟、可用性与运维成本经原型测量仍优于进程内调用。

多智能体只有在同一 golden set 上相对单 Agent 同时改善引用正确率/groundedness/relevance，visibility leak 保持 0，且 P95、token 成本和工具失败率不越过预算时才可晋升。否则维持当前决策。
