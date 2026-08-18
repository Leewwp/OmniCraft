# OmniCraft 站内内容问答/内容发现 RAG 深化设计

> **创建日期**：2026-08-11
> **状态**：`accepted`（2026-08-11；#28 决策输入已全部裁决）
> **上游约束**：GitHub issue #22（wayfinder 地图）、#30（四能力整合设计，已关闭，resolution 2026-08-11）、#28（本设计）
> **配套文档**：`docs/superpowers/specs/2026-07-16-omnicraft-dual-surface-agent-productization-design.md`（Web Agent 产品化）、`/Users/pp/Desktop/file/code/project/OmniCraft-tech-selection-analysis.md`（设计输入，§8 OpenSearch+pgvector、§11 落地顺序；确认后的权威合同由本文承接）
> **状态约定**：本设计不落 migration、不修改运行时代码；所有 Implemented 能力均以当前 `origin/main` 代码真相为准。

---

## 0. 定位与范围

本设计解决「OmniCraft 站内内容问答/内容发现 RAG」：

- **真实数据源**：OmniCraft 已发布站内内容（标题、正文 description、标签、IP 归属、来源/衍生关系、分类、内容类型）。
- **Markdown 仅作本地演示导入适配器**：用于 golden set 构造与无网络环境演示，不形成第二套知识库产品，不进入生产索引合同。
- **不在范围内**：通用知识库、多智能体平台（基线为单 Agent + 确定性 RAG 工作流）、Agent 自治研究循环、模型微调。

### 0.1 最小纵切片（本设计的目标链路）

```
OmniCraft 已发布内容（PostgreSQL 真相源）
  -> 结构化分块（标题/段落/Markdown heading 感知）
  -> chunk 元数据 + embedding 落 PostgreSQL（chunk 真相投影）
  -> OpenSearch BM25/过滤召回（full-infra profile 投影）
  -> pgvector 语义召回（可重建投影）
  -> RRF 融合（应用层，Go 实现）
  -> 服务端 viewer-aware 可见性复核（最终结果阶段再次过滤）
  -> Agent 工具调用（search_content 合同扩展）
  -> SSE 回答 + 站内引用（citation DTO 扩展）
  -> OpenTelemetry trace（跨 HTTP/DB/LLM）
  -> golden-set eval（确定性 harness + 指标归档）
```

### 0.2 证据分级（沿用 #30 resolution 的三层分级）

- **Implemented**：代码进入主线、Compose 可启动、有自动化测试、关键指标和至少一个失败/降级演练；简历可写「实现/使用」。
- **Designed**：有 ADR、契约、数据所有权、失败场景、降级恢复、选型比较、实施与回滚计划；简历只写「设计/规划/评估」。
- **Future option**：只记录触发条件、替代方案与不采用原因，不写入已用技术栈。

---

## 1. 现有能力审计（以代码真相为准）

审计基线：`origin/main`（0f0cc3e），全部结论有 file:line 支撑。

### 1.1 关键词检索（Implemented）

| 能力 | 现状 | 位置 |
|---|---|---|
| tsvector 全文检索 | `content_items.search_vector`（jiebacfg 探测，回退 simple），权重 A/B/C，GIN 索引 | migrations/041、060 |
| 查询构造 | `to_tsquery('simple', ...)` 前缀 AND + `title ILIKE` + 标签 EXISTS 兜底；`ts_rank_cd` 评分 + `ts_headline` 摘要 | repository/search_repo.go:142-203 |
| 过滤 | zone/category/content_type/time_range/tags/sort（relevance/most_views/best_rated/newest/hot） | search_repo.go:82-140 |
| 可见性 | `ApplyContentVisibilityScope`：status=published、非软删、作者未封禁、IP 未封禁、is_public OR 作者本人 | repository/content_visibility.go:7-20 |
| pg_trgm | 索引已建（047/049），检索路径未显式使用 `%` 相似度算子 | migrations/047、049 |

### 1.2 pgvector 语义检索（Implemented，但有缺口）

| 能力 | 现状 | 位置 |
|---|---|---|
| 表 | `content_embeddings`（content_item_id PK，vector(1536)，IVFFlat lists=100） | migrations/019、022 |
| 检索 | `1 - (embedding <=> ?)` cosine 相似度，topK 无过滤谓词 | repository/embedding_repo.go:31-40 |
| 写入 | 推荐服务 1 分钟补缺扫描（zone=original）；`content.embedding` 队列通道存在但无生产发布者 | service/recommendation_service.go:408-449；agent_service.go:555-573 |
| **缺口 1** | 向量检索无 status/visibility 过滤，可见性全靠调用方事后过滤 | embedding_repo.go:31-40 |
| **缺口 2** | 每 content 一行 embedding（text=title+description），无 chunk/文档概念 | migrations/022 |
| **缺口 3** | 维度 1536 写死于建表，模型换型需迁移 | migrations/022 |

### 1.3 Agent tool calling 与引用（Implemented，作为纵切片基线）

| 能力 | 现状 | 位置 |
|---|---|---|
| tool 白名单 | search_content / get_content_detail / get_usage_guide / suggest_publish_metadata | service/agent_tools.go:128-172 |
| 严格解码 | `DisallowUnknownFields` + 参数校验 | agent_tools.go:176-183 |
| viewer-aware 解析 | `resolveVisibleContent`（统一错误防探测） | agent_tools.go:188-203 |
| 引用服务端复核 | `RevalidateCitations`：模型输出逐条复核，无效即丢弃，超限截断 | agent_tools.go:354-370 |
| grounded 分类 | 无有效引用 → `no_evidence` | agent_tools.go:375-380 |
| SSE 契约 | start/tool_status/delta/citation/usage/done/error + trace_id + degraded | service/agent_stream.go:20-64 |
| 降级 | NLSearch：embedding 失败 → viewer-aware 关键词回退（Degraded 标记） | agent_service.go:274-322 |
| 评测 | 确定性 fake provider + 12 case JSON fixture，`go test` 内运行 | service/agent_evaluation_test.go；testdata/agent_eval_cases.json |

### 1.4 队列 / Worker / 可观测性（部分 Implemented，RAG 依赖的加固面）

| 能力 | 现状 | 位置 |
|---|---|---|
| Redis Streams | 组消费/重试退避/DLQ/积压指标完备，默认 `queue.enabled:false` | pkg/queue/redis_stream.go |
| 幂等 | `IdempotentCheck`（SetNX 24h）存在但 ReviewWorker 未注入 rdb，实际未生效 | pkg/queue/idempotent.go；worker/review_worker.go:14-23 |
| DLQ replay | Replay 方法存在但无 HTTP 路由挂载；DLQ TTL 清理无消费方 | worker/dlq_worker.go:51-94 |
| Worker 进程 | 全部在 backend 进程内启动，无独立 Worker 进程 | container.go:192-212；main.go:86 |
| OTel | **完全不存在**（grep 0 命中，go.mod 无依赖）；现状为 slog JSON + Prometheus | observability/ |

### 1.5 结论

1. 站内关键词检索 + 可见性过滤 + Agent 引用复核已经 **Implemented**，是纵切片的不变基线。
2. embedding 是内容级（无 chunk）、无过滤谓词、无稳定写入事件 —— 是 RAG 深化主要缺口。
3. 队列可靠性（幂等接线、replay 挂载、独立 Worker）是异步索引链路的前置加固面。
4. 可观测性零 OTel —— 独立批次（可靠异步与可观测性基础计划）先行。

---

## 2. 能力分级总表

| 能力 | 分级 | 说明 |
|---|---|---|
| 站内关键词检索（tsvector + 过滤 + 可见性） | Implemented | 现状基线，RAG 降级目标 |
| 内容级 pgvector 语义召回 | Implemented | 现状可用，但无 chunk/过滤谓词 |
| Agent tool calling + 服务端引用复核 + SSE | Implemented | 纵切片接入点 |
| deterministic eval fixture（12 case） | Implemented | 纵切片评测基线 |
| **chunk 化（结构感知分块 + chunk 表）** | **Designed** | 本文 4.5/4.6 |
| **RRF 混合召回** | **Designed** | 本文 4.2 |
| **Reranker（cross-encoder/LLM）** | **Future option** | 本文 4.4（不实现接口/NoOp，仅保留升级门） |
| **alpha 加权融合** | **Designed 实验（文档层，无运行时开关）** | 本文 4.3 |
| **OpenSearch full-infra profile + indexer + full rebuild** | **Designed** | 本文 4.1/4.9、9.3（宪法修订前置，见可靠异步计划） |
| **golden set + eval harness + 指标归档** | **Designed** | 本文 6/7 |
| **两节点 Research Agent → Answer Agent** | **Designed 实验（对照评测通过才升级）** | 本文 4.10 |
| 多智能体编排平台（LangGraph sidecar 等） | Future option | #25 决策；触发条件见 4.10 |
| cross-encoder/LLM reranker 实现 | Future option | 4.4 升级门通过后实现 |
| Markdown 知识库产品化 | Future option | 仅本地演示适配器 |
| Kafka 替代 Redis Streams | Future option | 触发条件见 tech-selection §6.2 |

---

## 3. 目标数据流（纵切片端到端）

```
[写入面]
发布/审核通过（content status → published）
  → outbox 事件 content.published（Web-only DAG 收口后落 Outbox）
  → indexer 消费：
      1) 读取已发布版本正文（content_versions 最新 is_latest 版本）
      2) 结构化分块（heading/段落感知）→ 写 chunk 表（content_version 绑定）
      3) embedding（每个 chunk）→ 写 chunk_embedding 投影
      4) OpenSearch 索引 chunk 文档（BM25 字段 + 过滤字段）
      5) 记录 index_projection 状态（index_version + content_version + 状态）

[查询面]
用户 query
  → POST /agent/chat/stream（或 /agent/search）
  → Agent 工具 search_content：
      1) OpenSearch BM25/过滤召回（topK=200）
      2) pgvector 语义召回（topK=200）
      3) 应用层 RRF 融合（k=60，同 chunk 跨检索器累加贡献，确定性 tie-break）→ top 20
      4) 服务端可见性复核（ApplyContentVisibilityScope + content_version 校验）→ 不足补位
      5) 组装 citation 候选（content_id/version/chunk_key/title/excerpt/source）
  → LLM 生成回答（单 Agent）
  → RevalidateCitations（输出后复核，trace 记录复核结果）→ SSE citation 事件
  → OTel trace（span：retrieval/rrf/llm/visibility）
  → eval（golden set 抽样/CI 回归）
```

---

## 4. 核心设计决策

### D1. 数据真相源与投影边界（确认）

**决策**：PostgreSQL 是内容与 chunk 元数据的唯一真相源；OpenSearch 与 embedding 都是**可重建投影**（rebuildable projections）。

- `content_items` / `content_versions` / `content_tags` / `ips` / `categories`：真相源，业务写入唯一入口。
- `rag_chunks`（+ 索引投影状态表）：PostgreSQL 内的 chunk 真相投影；包含 chunk 全量字段（4.6），查询与重建都从它出发。
- `chunk_embeddings`：向量投影，可随时从 chunk 文本 + embedding model 重建。
- OpenSearch 索引：关键词/过滤投影，可随时从 `rag_chunks` full rebuild。
- 投影重建不阻塞业务：任何投影失败只影响检索质量，不影响发布链路。
- **不采用**：OpenSearch 作真相源（内容生命周期事务性太强，双真相源会造成不可调和漂移）；Milvus/Qdrant（见 #22 决策，向量库只选 pgvector）。

### D2. 混合召回融合算法：RRF 为基线（已确认 2026-08-11）

**决策**：基线使用 RRF（Reciprocal Rank Fusion），应用层 Go 实现，不依赖 OpenSearch 内部 fusion 能力（因为语义召回在 pgvector，不在 OpenSearch）。

公式：

```
RRF(d) = Σ_{r ∈ R} 1 / (k + rank_r(d))
```

- k = 60、候选集 BM25 topK = 200 / pgvector topK = 200：**已确认作为可配置初值**（用户裁决 1），纳入评测参数扫描（k ∈ {20, 60, 100}）。
- **同 chunk 跨检索器合并**：同一 chunk 同时出现在两个检索器结果中时，贡献**累加**（两个秩各贡献一个 `1/(k+rank)`，不取其一）；去重单位是稳定 chunk identity（见 D6），融合后按累计 RRF 分降序取 top 20。
- **tie-break**：RRF 分数相同 → 按稳定 chunk identity 字典序升序（确定性，可复现评测）；不允许随机排序进入评测。
- **可见性过滤后补位**：top 20 过 `ApplyContentVisibilityScope` + content_version 校验后，若不足目标数量（最终 top 10），从被过滤候选之后的下一候选**按序补位**，直到达到目标数量或候选耗尽；补位进来的候选同样先过可见性复核。
- **分数归一化**：不归一化原始相似度分数（这正是 RRF 的意义——秩融合对未校准分数鲁棒）。
- **不采用**：线性加权 `alpha*s_bm25 + (1-alpha)*s_vector` 作为基线——两个检索器分数分布不可比（BM25 分数无上界、embedding 相似度是 cosine），直接加权需要校准；保留为 Designed 实验（D3）。
- **不采用**：OpenSearch hybrid query + RRF search pipeline——只有关键词在 OpenSearch 内、语义在 pgvector 外，融合必须在应用层完成；若未来向量也迁入 OpenSearch 再重新评估。

### D3. alpha 加权融合（Designed 实验，不设运行时开关）

- 触发条件：RRF 在 golden set 上召回达到上限或出现可解释缺陷（如 BM25 弱词召回被秩压制）。
- 前置：检索器分数校准（BM25 归一化 + embedding 相似度校准），否则加权无意义。
- 判定：同一 golden set 上加权融合 Recall@K/MRR 显著优于 RRF（统计显著 + 人工抽查无回退）才可能替换基线；否则关闭实验，保留 RRF。
- **不写实现计划、不设评测开关**（用户裁决：删除 T06 的 alpha 实现开关）；本条目仅为设计记录，保留在文档层。

### D4. Reranker：Future option（已确认 2026-08-11）

**决策**：reranker（cross-encoder/LLM rerank）整体保持 **Future option**，不实现接口、不写 NoOp 占位（用户裁决 3）。保留评测门思想作为未来升级条件：

- **升级门**（全部满足才实现）：
  1. golden set 上 `nDCG@10` 绝对提升 ≥ **0.05**（0.05 绝对差，非相对百分比）；
  2. **延迟不回退门**：加入 reranker 后的 P95 检索延迟 ≤ 基线 P95 × 1.15（15% 预算），且不超总体延迟预算（SSE first-token latency 目标不变）；
  3. citation precision / groundedness 不回退；
  4. 无 GPU 预算下成本可解释（本地 bge-reranker CPU 推理延迟验证；付费 API 按量）。
- 触发条件记录于「未决项/未来选项」追踪，不进入本纵切片里程碑。
- **不采用**：LLM rerank（成本高、延迟大、不可复现），记录为 Future option 内的备选，同样不实现。

### D5. 结构化分块策略（已确认 2026-08-11）

**决策**：标题/段落/Markdown heading 感知的结构化切分，受 token 上限约束。

- 切分单位：站内内容正文（description 字段，支持 Markdown）→ 按 heading 层级（`#`/`##`/`###`）与段落（空行分隔）形成层次树 → 叶子段落作为基础单元。
- 合并：基础单元在 token 预算内顺序合并为 chunk；超限单元按句子边界硬切。
- **约束**：
  - `chunk_max_tokens`（默认 512，配置化）与 `chunk_overlap_tokens`（默认 48，配置化）必须从 config.yaml 读取，通过评测确定，不硬编码；
  - 标题链（H1→H2→…）作为 chunk 的 `heading` 字段与文本前缀，提升召回上下文；
  - chunk 文本在 embedding 前拼接「heading + 文本」，但存储保留原始偏移（4.6）；
  - token 计数固定使用 `cl100k_base` encoding，与当前 `text-embedding-3-small` 对齐；实现采用维护中的 `github.com/pkoukk/tiktoken-go`。embedding model 或 tokenizer 改变时递增 `chunking_version` 并重建，不使用未定义的近似算法。
- **不采用**：固定字符数滑窗（无结构语义，标题被切断）；递归字符切分 + 超长容忍（引用定位不精确）；LLM 语义切分（成本高、不可复现，作为 Future option）。

### D6. chunk 数据模型（已确认 2026-08-11，双世代/宪法合规修正）

新表一律满足宪法 Principle III：每表至少 `id BIGSERIAL PRIMARY KEY` + `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`；业务键用 UNIQUE 约束表达。

`rag_chunks` 表（迁移编号在 #31 建票时最终占位，顺序约束：位于 068 之后）：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | BIGSERIAL PK | 宪法要求的代理主键（不进入引用合同） |
| `content_id` | BIGINT FK | 所属内容 |
| `content_version` | INT | 绑定 `content_versions.version_number`（chunk 内容的版本快照） |
| `chunk_index` | INT | 同一内容版本与分块版本内的有序序号 |
| `chunking_version` | INT | 分块算法/参数/tokenizer 版本；分块语义改变时递增 |
| `chunk_key` | CHAR(64) | 对 `content_id/content_version/chunking_version/source_start/source_end/text_hash` 做 SHA-256 得到的确定性身份 |
| `heading` | TEXT | 标题链（H1/H2/… 拼接，可为空） |
| `text` | TEXT | chunk 文本 |
| `source_start` / `source_end` | INT | 正文 Unicode code point 的半开区间 `[start,end)`（引用定位与评测锚点；最小实现不做 UI 高亮） |
| `zone` | VARCHAR | original / fanwork |
| `content_type` | VARCHAR | 内容类型（mod/guide/…） |
| `category` | VARCHAR(50) | 分类业务键快照（跟随 `content_items.category`，可空） |
| `ip` | BIGINT | 所属 IP（ips.id，可空） |
| `tags` | TEXT[] | 内容标签快照（分块时刻） |
| `created_at` / `updated_at` | TIMESTAMPTZ | 宪法 + 审计 |
| UNIQUE | — | `(index_version, chunk_key)`；以及 `(content_id, content_version, chunking_version, index_version, chunk_index)`，允许两个索引世代并存 |

**稳定 chunk identity（T04 裁决）**：引用使用确定性 `chunk_key`，计算 `content_id/content_version/chunking_version/source_start/source_end/text_hash` 的 SHA-256；同一内容版本、分块版本、源区间和文本重复 rebuild 时身份稳定，文本变化必然产生新身份。`index_version` 属于投影世代，不进入身份 hash，因此同一 chunk 可在多个世代共存。golden set 以 `content_id/content_version/source_start/source_end` 标注相关证据，运行时按目标 `chunking_version` 解析为 chunk_key，因此分块升级后必须重新生成并人工复核受影响标注。

投影表（Designed）：

- `chunk_embeddings`：`id` PK、`created_at`、`chunk_id` BIGINT FK、`embedding vector(1536)`、`embedding_model`、`embedded_at`，UNIQUE `(chunk_id, embedding_model)`。
  - **pgvector 最小实现固定 `vector(1536)`**（已确认）：维度变化不做热切换，走新 migration + 新世代重建（embedding model 升级 = 新 index_version）。
- `index_projection_status`：`id` PK、`created_at`、UNIQUE `(content_id, index_version)`（**双世代共存**）、`chunking_version`、`embedding_model`、`state`、`error_summary`（脱敏）、`last_indexed_at`；当前世代由 `is_current` 布尔 + `UNIQUE(content_id) WHERE is_current` 表达。OpenSearch 与 pgvector 查询都必须 join/过滤 `is_current=true`，不得混读旧世代。
- `rag_chunks.category` 跟随现有 `content_items.category` 的 `VARCHAR(50)` 业务键快照，不在 T04 引入分类表重构。
- 内容版本绑定：`content_items.status=published` 是可索引性的权威门；正文从该内容的 latest active `content_versions` 重建。071 为缺少版本记录的既有内容回填 version 1 full snapshot，避免已发布内容因历史数据缺版本而无法投影。

**不采用**：把 chunk 全部塞进 content_embeddings 换列（已有表结构为 content 级，改语义风险大）；OpenSearch 作为 chunk 元数据唯一持有者（违反 D1）。

### D7. 引用 DTO 契约（已确认 2026-08-11，移除公开校验/分数字段）

现有 `AgentCitation`（agent_contract.go:31-36）为 `content_id/title/zone/excerpt`。扩展为（**不包含** `visibility_checked` 与 `score`——已确认移除）：

```jsonc
// SSE citation 事件 payload（向后兼容：content_id/title/zone 保留）
{
  "content_id": 123,
  "content_version": 7,              // 引用对应的内容版本
  "chunk_key": "4f2d...9a10",       // 服务端生成的确定性 chunk 身份
  "chunk_index": 2,                  // 当前分块版本内的位置，仅用于展示/排查
  "title": "简约北欧风家具包 v2.0",
  "zone": "original",
  "route": "/original/123",          // 服务端生成的 route，不信任模型
  "excerpt": "…chunk 开头摘要…",
  "source": "hybrid_rrf"             // bm25 | vector | hybrid_rrf
}
```

- **服务端复核语义**：复核通过 = 引用**能被输出**（能输出即已复核）；被 `RevalidateCitations` 丢弃的引用不进入 SSE。不再向客户端暴露 `visibility_checked` 布尔（避免信任标记语义）；**服务端内部在 trace/日志记录每次复核结果**（通过/丢弃原因：不可见/缺字段/超限），供审计与评测。
- 模型伪造/越权引用一律丢弃（现有行为保持）；`route` 由服务端构建。
- 前端 UI：复用 `AgentCitationList` 与 ContentDetailOverlay（#68 收敛后的共享浮层），excerpt 只做开头摘要（已确认最小实现不做高亮定位，`source_start/source_end` 仅存储）。
- **不采用**：模型直接提供 URL（现有 §11.10 原则，继续遵守）；引用全文（只给 excerpt）。

### D8. 最终结果阶段可见性过滤（确认，双重保险）

- 隐藏、删除、未审核（非 published）、封禁作者内容**必须在最终结果阶段再次过滤**，不依赖索引是否更新：
  1. 召回阶段：OpenSearch 索引写入时只索引 published；pgvector 查询带状态过滤（最小实现内 VectorSearch 增加 WHERE 谓词——现有缺口 1 的修复）。
  2. **最终阶段（强制）**：RRF 结果 top20 一律再过 `ApplyContentVisibilityScope`（含 content_version 与最新状态），与现有 `listVisibleNLSearchContents` / `RevalidateCitations` 同一权威路径；过滤后不足目标数量时从剩余候选**按序补位**并逐一复核（D2）。
  3. 双保险原因：索引延迟窗口（删除→投影更新）与越权探测试图；最终阶段是唯一信任边界。

### D9. 降级策略（确认）

| 故障 | 行为 | 降级标记 |
|---|---|---|
| OpenSearch 不可用 | 降级到 PostgreSQL 现有关键词检索（searchRepo.SearchContents，viewer-aware） | `degraded=keyword_pg` |
| embedding 不可用（API/Provider 失败） | 返回关键词结果（OpenSearch BM25 或 PG 关键词），不进入语义召回 | `degraded=keyword_only` |
| pgvector 查询失败 | 同上 | `degraded=keyword_only` |
| LLM Provider 失败 | 现有契约：错误事件 + 前端降级为普通搜索（agent_stream.go:331-342） | `degraded=provider_error` |
| RRF 一侧为空 | 单侧结果直接返回（分数保留） | 不标记 degraded，`source=bm25|vector` |

- 降级不返回伪造内容；每次降级写 trace 与指标（降级成功率 = 降级时仍成功返回结果的请求占比）。
- **故障演练**（#31 closure 阶段）：停 OpenSearch → 验证关键词降级；停 embedding → 验证关键词降级；恢复后索引一致。

### D10. 多智能体边界（确认，Designed 实验）

- 最小实现 = 单 Agent + 确定性 RAG 工作流（查询改写、关键词/向量召回、过滤、RRF 融合、可见性复核均为确定性代码，不包装成 Agent；无 reranker 阶段）。
- 两节点 `Research Agent -> Answer Agent` 仅保留为 Designed 实验，**必须通过 golden-set 对照评测**（引用正确率、groundedness、answer relevance、泄漏率、P95、token 成本、tool 成功率）后才能升级为 Implemented（升级标准初定：引用正确率 +5pt 且成本/时延在预算内且泄漏率 0），否则关闭实验。
- 触发条件记录在 #22 雾区与 tech-selection §9，不作为本纵切片里程碑。

---

## 5. 失败流 / 重建流 / 权限流

### 5.1 失败流

```
分块/embedding/OpenSearch 任一失败：
  -> index_projection_status.state = failed + error_summary（脱敏）
  -> 重试（有界退避，与队列 retry 合同一致，maxAttempts 后入 DLQ）
  -> DLQ 条目人工/脚本重放；修复代码后 full rebuild 单 content 或全量
  -> 检索质量指标（index_lag、投影缺失率）持续监控
```

### 5.2 重建流

```
全量重建（full rebuild）：
  -> 开启新 index_version（写侧原子递增）
  -> 扫描真相源所有 published content → 按目标 chunking_version 分块/embedding/索引
  -> 完成后切换读侧 index_version（is_current 翻转），旧投影可清理（先保留一个世代）
  -> 增量：事件驱动单 content 重建（content.published / content.updated / content.banned）
```

- 重建是**唯一**修复索引漂移的手段；重建期间读侧继续服务旧版本。
- **golden-set 重建稳定性**：同一 `chunking_version` 重建时 chunk_key 稳定；切分规则变化时，golden set 通过 source span 重解析，并在基线运行前人工复核差异，禁止把旧 chunk_index 静默解释为新文本。

### 5.3 权限流

```
viewer context（匿名=0 / user_id）
  -> 所有检索入口带 viewer 上下文
  -> 召回阶段：索引只含 published（粗过滤）
  -> RRF 后：ApplyContentVisibilityScope（权威细过滤：is_public OR 作者本人、封禁、软删、状态）
  -> 引用输出前：RevalidateCitations（同权威路径）
  -> 审计：可见性过滤不记录具体被挡内容（防侧信道），只记录计数与 trace
```

---

## 6. Golden set 定义（Designed，已确认口径 2026-08-11）

存放：PostgreSQL `eval_golden_cases`/`eval_runs` 为完整评测与运行记录真源；JSON 是由固定导出命令生成的、带 schema_version/checksum 的 CI 子集。T01 负责 migration、导出一致性测试与运行归档，按 heavy 车道执行。

每个 case 必须包含：

| 字段 | 说明 |
|---|---|
| `query` | 用户查询原文 |
| `query_language` | 查询语言标注（口径见下，禁止歧义） |
| `viewer_context` | 匿名 / 登录用户 A / 登录用户 B（用于越权用例） |
| `relevant_spans` | 期望证据的 `(content_id, content_version, source_start, source_end)` 列表；按当前 chunking_version 解析为 chunk_key |
| `relevant_content_ids` | 期望召回的内容级 ID（跨 chunk 断言） |
| `expected_citations` | 期望引用（content_id + content_version + 可选 source span） |
| `forbidden_content_ids` | 必须不出现的内容（隐藏/删除/未审核/封禁/他人私有） |
| `answer_rubric` | 答案质量判据（确定性断言 + judge 条目） |
| `classification` | 内容类型（mod/guide/…）、冷门/热门查询分类 |

**语言分类口径（已确认）**：
- `query_language ∈ {zh, en, mixed}`，判定规则：去除标点后中文字符占比 >50% 记为 `zh`；纯 ASCII（无中文）记为 `en`；两者均显著出现（中文占比 10%–50%）记为 `mixed`。
- 目标分布：`zh ≥ 50%`、`en ≥ 20%`、`mixed ≥ 10%`（三者按 case 计数，合计 100%）；`mixed` 用例如「中文问法 + 英文术语」的跨语言语义匹配。
- 内容语言与查询语言**分开记录**：content 侧语言属性用于 mixed 用例构造，不参与查询语言占比。

- 语料构造：从站内真实 published 内容采样 + 人工标注；Markdown 导入仅用于演示数据集；越权用例使用 fixture 用户与私有内容。
- 规模（已确认）：初始 ≥ 50 case（12 个既有 eval case 迁移合并 + RAG case），冷门查询 ≥ 20%。
- 重建稳定性：同分块版本重复重建不失效；分块版本变化必须重解析 source span、输出 diff 并完成人工复核。
- 维护：badcase 回灌（生产采样命中可见性/引用缺陷 → 回灌 golden set）为 Designed 流程。

---

## 7. 评测指标（Designed，口径定义，已确认 2026-08-11）

| 指标 | 口径 | 目标（评测后定值，不预填） |
|---|---|---|
| Recall@K（K=10/20） | 当前 chunking_version 下由 relevant_spans 解析出的 chunk_key 命中比例 | 占位 |
| MRR 或 nDCG@10 | 首相关秩 / 折扣累积增益 | 占位 |
| citation precision | 输出引用中属于 expected_citations 的比例（输出引用全部已经服务端复核——能输出即已复核） | 占位 |
| citation coverage | expected_citations 被至少引用一次的比例 | 占位 |
| groundedness | 回答句子能否在引用 chunk 文本中找到依据（judge + 确定性抽样） | 占位 |
| answer relevance | 回答与 query 相关性（judge） | 占位 |
| **visibility leak count** | 泄漏=引用/回答中出现 forbidden_content_ids 或非 viewer 可见内容；**必须为 0** | **0** |
| P95 检索延迟 | OpenSearch+pgvector+RRF 全链（不含 LLM）；reranker 加入时按 D4 延迟不回退门复核 | 占位 |
| SSE first-token latency | start 事件到首个 delta | 占位 |
| token/request | LLM 输入输出 token 合计（usage 事件统计） | 占位 |
| 降级成功率 | OpenSearch/embedding 故障注入下成功返回结果的比例 | 占位 |

- 引用复核结果（通过/丢弃原因）由服务端写入 trace/日志（D7），评测断言基于「能输出的引用」而非客户端可见布尔。
- 评测 harness：确定性执行（fixed seed、确定性 tie-break），judge 用固定 LLM 配置并缓存；CI 用 fake provider，真实 provider 仅 smoke。
- 指标归档：每次 eval run 落表（时间、commit、配置、指标快照），供 #32 证据合同消费（不预填数字）。

---

## 8. 技术落点（Designed，运行时代码全部等 Web-only DAG 收口）

### 8.1 配置（config.yaml 新增段，已确认 2026-08-11）

功能开关遵守宪法 Principle V（`features:` 下布尔开关，false 时接口 503 `FEATURE_DISABLED`）；参数段命名 `rag:`：

```yaml
features:
  rag_hybrid_enabled: false        # 混合检索与 RAG 扩展字段开关（默认 false）

rag:
  hybrid:
    bm25_topk: 200                 # 可配置初值（已确认）
    vector_topk: 200               # 可配置初值（已确认）
    rrf_k: 60                      # 可配置初值（已确认）
    final_topk: 10
  chunking:
    max_tokens: 512                # 可配置初值（已确认）
    overlap_tokens: 48             # 可配置初值（已确认）
    tokenizer_encoding: cl100k_base
    version: 1                     # 分块语义变化时递增
  index:
    generation_start: 1            # 与 index_projection_status 世代联动
```

- `agent.web_agent_enabled` 继续控制 `/agent` 页面与既有 Agent API；`features.rag_hybrid_enabled=false` 时不渲染 RAG 来源/版本扩展字段，检索走现有基线，`POST /api/v1/admin/rag/rebuild` 返回 503 `FEATURE_DISABLED`。这避免关闭 RAG 时误伤已实现的基础 Agent。
- 不设 `reranker.*` 配置（reranker 为 Future option，实现时才引入配置）。
- pgvector 维度固定 1536（最小实现），维度变化走迁移 + 新世代，不做运行时可配维度。

### 8.2 未来文件面（Designed，不落盘）

- `backend/internal/service/rag/`：chunker / hybrid retriever / citation builder（深模块边界；无 reranker 模块）
- `backend/internal/repository/rag_chunk_repo.go`、`rag_index_projection_repo.go`
- `backend/internal/repository/opensearch_repo.go`
- `backend/internal/worker/indexer_worker.go`（消费 content.published / 重建）
- RAG 迁移编号在 #31 建票时最终占位（顺序约束：在 068 之后、与可靠异步批次 Outbox 迁移编号互不冲突）
- 前端：`AgentCitation` 类型扩展、引用卡展示（最小实现不改视觉权威，仅类型/字段扩展）

### 8.3 OpenSearch 引入边界（确认）

- 仅 `full-infra` profile 启用（docker-compose `opensearch` 服务 + 健康检查 + 最小 seed）；默认 profile 保持轻量。
- 索引 alias 机制：`omnicraft-rag-v<index_version>`，读侧固定 alias，重建切 alias（原子切换）。
- OpenSearch 是投影，不承载任何业务事务。

---

## 9. 与现有计划的共享面与冲突（Designed 审计）

| 共享文件 | 现有计划占用 | 本设计的串行规则 |
|---|---|---|
| `config.go` / `config.yaml` | #97（collaboration-invites）、#104（green 配置面） | RAG 配置新增在 Web-only DAG 收口后；不抢当前迁移/配置编号 |
| `routes.go` | #96/#97/#103 批次 | 无新增路由（复用 /agent/chat/stream 与 /agent/search） |
| `content_service.go` / `review_service.go` | #83/#90/#96/#97/#108 | Outbox 生产者在 Web-only 收口后接入 published/updated/banned/deleted 状态迁移；索引副作用只在 worker |
| `agent_contract.go` / `agent_tools.go` | Web Agent 产品化（#103 批次相关） | citation DTO 向后兼容扩展，不破坏现有契约测试 |
| migrations | 064–068 已占用 | 固定 069=RAG evaluation、070=Outbox/Inbox、071=RAG chunks；执行前冲突即停止并修订规划 |
| `zh.json` / `en.json` | #96/#97/#71/#69 | 最小实现仅新增 citation 来源文案（如有），串行预约 |

---

## 10. Grilling 压力测试记录（2026-08-11，随设计提交）

> 对 RRF、分块、rerank、引用、评测五个主题的对抗性质询与设计回应。用户已对全部 7 项裁决（§11），grilling 结论相应修订。

### 10.1 RRF

- **Q1：RRF 是否需要 k 值搜索？** 响应：k=60 为经典默认，作为配置项纳入评测参数扫描（k ∈ {20, 60, 100}），评测后定值。**已裁决：k=60/topK=200/200 作为可配置初值确认。**
- **Q2：两个检索器 topK=200 是否对称合理？** 响应：BM25 在 OpenSearch 上 200 代价低；pgvector IVFFlat 200 也低。不对称 topK 是 Designed 实验（向量侧可调），不预先优化。
- **Q3：RRF 对弱召回词的秩压制？** 响应：承认此局限，记录为 alpha 实验的触发条件之一（D3）。**已裁决：alpha 无运行时开关，仅文档层设计。**
- **Q13（裁决新增）：同 chunk 跨检索器出现时如何计分？** 响应：贡献累加（两个秩各计一次 `1/(k+rank)`）；可见性过滤后不足目标数量从剩余候选按序补位（D2）。

### 10.2 分块

- **Q4：chunk 合并会把不相关段落绑进一个 citation？** 响应：是，接受 —— citation 以 chunk 粒度输出，excerpt 取自 chunk 开头，超限段落按句子边界硬切，最小实现不做语义边界优化（Future option）。
- **Q5：站内正文是用户 Markdown，heading 缺失怎么办？** 响应：纯段落内容以「内容标题 + 段落」为 chunk，heading 字段可空；不伪造结构。
- **Q6：chunk 与 content_version 绑定是否过重？** 响应：不过重——引用必须能回溯到内容版本。稳定身份使用 chunk_key；chunk_index 只在同一 chunking_version 内有意义，切分规则变化时不得复用旧身份。

### 10.3 rerank

- **Q7：评测门阈值 5pt nDCG 是否武断？** 响应：修订为**绝对提升 ≥ 0.05（nDCG@10）**且新增**延迟不回退门**（P95 检索延迟 ≤ 基线 × 1.15）。**已裁决：reranker 整体降为 Future option，不实现接口/NoOp。**
- **Q8：CPU 本地 reranker 延迟风险？** 响应：计入 P95 检索延迟指标与预算门（D4 延迟不回退门）；不达标不升级。

### 10.4 引用

- **Q9：`visibility_checked` 布尔字段是否可被前端误用为信任标记？** 响应：**已裁决：移除公共字段**——复核语义改为「能被输出即已复核」，服务端内部 trace 记录复核结果；不引入签名引用。
- **Q10：citation 评分暴露是否泄露索引质量？** 响应：**已裁决：移除公共 score 字段**——source 保留（bm25/vector/hybrid_rrf），分数仅存在于服务端 trace。

### 10.5 评测

- **Q11：judge 自评是否有偏差？** 响应：groundedness/relevance 用固定模型 + 缓存 + 人工抽检 10%；引用/泄漏指标是确定性断言，不依赖 judge。
- **Q12：50 case 是否足够？** 响应：初始 50 是纵切片门槛，CI 回归用确定性 subset，全量评测在 full-infra 下运行；case 持续回灌。**已裁决：语言分类口径按 query_language zh/en/mixed 明确（zh≥50%、en≥20%、mixed≥10%），内容语言分开记录。**

---

## 11. 用户裁决记录（2026-08-11，7 项全部裁决）

| # | 原问题 | 裁决 | 落点 |
|---|---|---|---|
| 1 | RRF k/topK 初值 | **确认** k=60、topK=200/200 为可配置初值；同 chunk 跨检索器**累加贡献**；可见性过滤后**补位**到目标数量 | D2 |
| 2 | chunk 512/48 初值 | **确认**（评测后定值） | D5 |
| 3 | reranker 评测门 | **Future option**：不实现 NoOp/接口；阈值 = nDCG@10 绝对提升 ≥ 0.05 + 延迟不回退门（P95 ≤ 基线 × 1.15） | D4 |
| 4 | golden set 规模 | **确认** ≥50 case、冷门 ≥20%；语言口径明确（zh/en/mixed 判定规则与占比） | §6 |
| 5 | citation DTO | **确认**保留 content_id/content_version/chunk_key/title/zone/server-built route/excerpt/source；**移除**公共 visibility_checked 与 score；复核语义=能输出即已复核，trace 记录内部结果 | D7 |
| 6 | 配置/迁移 | **确认** `features.rag_hybrid_enabled` 开关 + `rag:` 参数段；pgvector 固定 vector(1536)，维度变化走迁移+新世代；固定 069=evaluation、070=outbox、071=chunks | §8 |
| 7 | 引用范围 | **确认**最小实现仅 excerpt，不做高亮定位（source_start/source_end 仅存储） | D7 |

> 用户已于 2026-08-11 再次确认；本文已晋升，#28 已关闭，实现由 #136–#145 追踪。

---

## 参考

- #22（wayfinder 地图）/ #23（评估闭环选型）/ #24（可观测性选型）/ #25（多智能体选型）/ #30（四能力整合，已关闭）
- `docs/superpowers/specs/2026-07-16-omnicraft-dual-surface-agent-productization-design.md`
- `/Users/pp/Desktop/file/code/project/OmniCraft-tech-selection-analysis.md` §8/§11
- 代码审计：`backend/internal/{handler,service,repository,worker,pkg/queue,observability}`、`backend/migrations/0{19,22,41,42,45,47,49,60}`（2026-08-11，origin/main 0f0cc3e）
