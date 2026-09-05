# Agent Runtime Matrix（唯一运行时真相）

> 本文是 Agent 运行时选型的**唯一权威**：模型、适配器、端点、维度、索引表、开关默认值与验证口径都以本文为准。
> `backend/config.yaml` 与 `.env.example` 是本文的镜像；`docs/specs/web-agent-v0.4-mvp.md` 等规格文档引用本文，不自行维护第二套选型。
> 修订历史见文末决策日志。凭证永不写入本文或仓库（仅根目录 `.env`，`chmod 600`）。

## 1. Canonical Profile（当前唯一正式配置）

| 层 | 选型 | 端点/规格 | 状态 |
|---|---|---|---|
| Chat | **MiniMax-M3**，经 `minimax` 适配器 | `https://api.minimaxi.com`（OpenAI 兼容端点，**不带尾部 `/v1`**） | 2026-09-05 真实冒烟三项判据全过（工具调用、流式 `<think>` 分流、4 用例表面探针） |
| Embedding | **text-embedding-v4 @ 1536 维**，经独立 `openai_compat` embedding provider（拆分接线） | `https://dashscope.aliyuncs.com/compatible-mode`（不带尾部 `/v1`）；批量上限 10 条/请求 | A-03 落地 + 冒烟复测 1536 维 |
| Rerank | **qwen3-rerank**（DashScope）主 + SiliconFlow `bge-reranker-v2-m3` 备，失败降级回 RRF 序 | DashScope rerank API；key 仅 env（`RAG_RERANK_API_KEY` / `RAG_RERANK_FALLBACK_API_KEY`） | A-03 落地；gte-rerank 已下线勿用 |
| 词法检索 | **Postgres + pg_jieba**（`rag.hybrid.keyword_source: postgres` 默认）；OpenSearch 为可选回退/显式反转 | viewer-scoped 预过滤（真实 viewerID 透传） | SP-13 既定决策的接线收口 |
| 融合 | RRF（k=60）on pgvector chunk 向量 + pg_jieba 词法 | `rag_hybrid_enabled` 门控 | 三开关默认 off，A-04 消融数据定默认 |
| 查询扩展 | chat 模型短调用扩展 3–5 个检索词 | `rag_query_expansion_enabled` 门控 | 同上 |
| SSE | v2 **八事件** registry（见 §5） | — | A-02/A-06 已落地 |
| 向量表 | `rag_chunks` / `chunk_embeddings` / `index_projection_status`（迁移 **071**，chunk 级，Agent RAG 域） | 双世代投影 | A-03 全量重嵌入 v4（169/169） |
| 遗留向量表 | `content_embeddings`（迁移 **022**，content 级） | 推荐系统 / legacy 读域 | 写路径退役排在 A-04 定默认之后 |

**接线方式**：`CompositeProvider`（`internal/pkg/llm/composite.go`）——chat/流式/工具走 `minimax` 适配器，单条与**批量** embedding 走 `openai_compat`（`GetEmbeddings` 批量转发，保持 v4 批量契约）。OTel span 随各委托方携带各自的 `gen_ai.request.model`。

## 2. 命名变体（一等公民，非默认）

| 变体 | Chat | Embedding | 用途/状态 |
|---|---|---|---|
| cloud-lean（已上线） | MiniMax-M3（同 canonical） | 同左 | 线上 lean 部署实跑配置 |
| DashScope 单 key 变体 | qwen-plus @ `openai_compat`（compatible-mode） | 同一 key 的 v4 | 本地已验证（agent-smoke 4/4 含工具 + 浏览器 17/17）；**无思考通道**（仓库未发送 `enable_thinking`，think_delta 为空属预期） |
| CI/单测 | deterministic fake（测试替身） | 本地 standin | PR 快速门，永不接触真实 key |

**已退役/弃用**（仅保留历史记录）：原生 `qwen` 适配器（无 tools/max_tokens、不解析 tool_calls/reasoning，工厂 fail-closed 拒绝）、`deepseek-chat` 默认值、`embo-01`（MiniMax legacy embedding，仅 minimax embedding provider 显式选用时可用）、`text-embedding-3-small` / `text-embedding-v3`。

## 3. 接线红线（违反即 fail-closed 或功能死路）

1. **MiniMax chat 只能走 `minimax` provider**：MiniMax 流式不发 `[DONE]`，`openai_compat` 接线流会提前终止且 `<think>` 原样漏进正文（`<think>` 仅 minimax 适配器的 thinkSplitter 分流）。
2. **DashScope 端点不带尾部 `/v1`**：provider 自行追加 `/v1/chat/completions` 与 `/v1/embeddings`。
3. **跨厂商拆分必须自带 embedding key**：`embedding_provider ≠ llm_provider` 且 `AGENT_EMBEDDING_API_KEY` 为空 → 启动校验（release）、工厂（任何模式）、preflight 三处 fail-closed；embedding key 永不回退 chat key（跨厂商回退必然 401）。
4. **hybrid 开启时 `rag.index.embedding_model` 必须等于 `agent.embedding_model`**（config 校验强制；两者随 canonical 同步为 v4）。
5. `max_output_tokens` 当前 1200；canonical 候选 4096（M 系思考消耗输出预算），**须先有思考/正文 token 实测再调整**。
6. `llm_configs`（admin 管理台）是**配置登记 + 连接测试**，不参与运行时 Provider 选择；激活不改变运行中的 Agent（运行时身份以 OTel span 与 `/api/v1/config/public` 为准）。动态 resolver 属 Phase 2。

## 4. 验证口径（三层门禁）

| 层 | 内容 | 触发 |
|---|---|---|
| L1 PR fast gate | go test/build/vet + fake provider 工具循环与 SSE 契约 + 检索/可见性/引用 fixture + 配置与文档一致性 | 每次 PR（CI，无真实 key） |
| L2 真实评测（本地/手动） | env 门控 answer eval（`OMNICRAFT_AGENT_ANSWER_EVAL=1`）+ A-04 五配置消融（C0 v4 纯向量 → C1 +hybrid → C2 +扩展 → C3 +rerank → C4 全开；dev split） | 人工/夜跑，证据归档本地；真实 Provider 失败只能记 `external-input-missing`，不得以 mock 冒充 |
| L3 rehearsal | `scripts/real-provider-preflight.sh`（静态：fail-closed 守卫、v4 维度=1536、rerank 配对）+ `agent-smoke`（**表面探针**：非流式 Chat×4 + embedding/rerank 单发；**不是完整 Agent 门**——流式/工具二轮/SSE 行为由 L1 单测与 L2 覆盖） | 真实证据前手动 |

**证据纪律**：所有评测工件必须记录 chat provider/model/base、embedding provider/model/维度/base、rerank model、三开关、dataset/split checksum、脚本版本；评测脚本禁止隐式替换模型（从 env 读取并断言，不得硬编码 pin）。

**Release final test**：golden set 冻结后只在冻结 split 上跑一次，禁止调参；主指标 Recall@5 / nDCG@5 / citation precision（Recall@10、MRR、nDCG@10 仅作诊断）；四类 visibility leak == 0 与 no-answer / forbidden citation / prompt injection 全过为硬门。

## 5. SSE v2 事件 Registry（8 事件，唯一清单）

`start` / `think_delta` / `tool_status` / `delta` / `citation` / `usage` / `done` / `error`

- `think_delta` 仅展示层：不进引用复验、不作为工具结果、不并入 answer；无思考通道的 provider（如 DashScope 单 key 变体的 qwen-plus）自然缺席，前端折叠区隐藏。
- 前端必须忽略未知事件类型（向前兼容）；`done` 为终态裁决（no_evidence/degraded 时 answer 置空，客户端以 done 替换已流出正文）。

## 6. 决策日志

- **2026-09-05**：M3 冒烟（minimax 适配器：工具 ✓ / 流式思考 thinkRunes=326 正文干净 ✓ / 表面探针 ✓；openai_compat 接线流式双败 → 红线 1）→ canonical chat 定 MiniMax-M3；单 provider 耦合（chat 与 embedding 共实例）由 CompositeProvider 解除；原生 qwen 退役；`keyword_pg` 降级标记更名 `keyword_fallback`（PG 词法已是规范路径而非降级态）。
- **2026-09-04/05**：审计裁定 A-04 采用五配置（embo-01 基线出局：①→② 双变量混淆 + 语料已全量 v4 化；embo 对照列为 Phase 2 独立实验）；`llm_configs` Phase 1 降级为登记+测试。
- **2026-09-01（SP-13）**：v4@1536 零 DDL 换代、qwen3-rerank 链、hybrid on Postgres 不启用 OpenSearch、三开关默认 off 待 A-04。

## 7. 待决项

- A-04 消融（golden set 冻结后）：定 `rag_hybrid/query_expansion/rerank` 三开关默认值。
- `max_output_tokens` 4096 候选值的思考/正文 token 实测。
- A-04 定默认后退役 `content_embeddings` 写路径（IndexerWorker 仅在 hybrid off 时维护旧表）。
- qwen-plus 变体的 `enable_thinking` 开关（如需让变体也具备思考形态）。
