# OmniCraft RAG 最小纵切片路线图

> **创建日期**：2026-08-11
> **状态**：`accepted`（2026-08-11）。本路线图只编排任务；运行时实现必须等待 GitHub 原生冻结门解除。
> **来源（消费输入）**：
> - GitHub issue #31（执行编排，本票主体）、#28（RAG 深化设计 —— **本计划唯一 RAG 合同来源，不得并行发明另一套合同**）
> - `/Users/pp/Desktop/file/code/project/OmniCraft-tech-selection-analysis.md` §8/§11（设计输入，不是权威源）
> - `#30` resolution（2026-08-11）：纵切片为站内单 Agent RAG；Markdown 仅演示适配器；多 Agent 仅对照实验
> - 代码审计：`backend/internal/repository/{search_repo,embedding_repo,content_visibility}.go`、`backend/internal/service/{agent_tools,agent_stream,agent_contract}.go`（2026-08-11）
> **执行追踪**：T01 #136；T04 #139；T05 #140；T06 #141；T07 #142；T09 #144；T10 #145；#32 blocked_by #136–#144（不含 #145）

## Goal

在 Web-only DAG 收口、可靠异步基础（T02/T03）就绪后，实现可评测、可降级、可重建的站内内容 RAG 纵切片：

```
已发布内容 → 结构化分块 → chunk 表 + embedding → OpenSearch BM25/过滤召回
  → pgvector 语义召回 → 应用层 RRF → 服务端可见性复核 → Agent 工具调用
  → SSE 回答与引用 → OTel trace → golden-set eval
```

## 与现有计划的关系

- 消费 `docs/superpowers/specs/2026-08-11-omnicraft-rag-deepening-design.md` 的全部合同（chunk 模型、RRF 参数、citation DTO、golden set、指标口径）。
- 全部运行时 Ticket 被「Web-only 合同冻结门」阻塞（terminal blockers = {#76, #108, #111, #112, #113}；定义见可靠异步与可观测性基础路线图）。
- **T01 可执行 harness、fixture 与基线运行同样受冻结门阻塞（已裁决）**：冻结前只允许**文档级**产出——golden set schema 文档、样例 JSON、评测设计（#30 允许的「golden set 准备」仅指文档层设计，不落可执行代码）；可执行 harness 与真实基线运行在冻结门解除后开工。

## 车道与执行纪律

- T01/T04/T05/T06/T07 为 heavy：T01 包含数据库迁移与持久化评测真相源；T06 触碰生产检索运行时合同与降级语义。T09 为 closure（mixed）；T10 为 decision gate（文档决策）。
- 每个 heavy Task 一 worktree、一分支、一 commit；TDD 先红后绿；两阶段审查。
- **heavy ticket 内所有「或」选项在建票前裁决**（已裁决：rebuild 入口 = admin 端点；embedding 写入 = indexer 内联；详见 T05）。
- 迁移固定为 `069_rag_evaluation.sql`、`070_outbox_inbox.sql`、`071_rag_chunks.sql`；若执行前任一编号已被占用，立即停止并同步修订两份路线图与 GitHub tickets。

---

## T01 [heavy] 评测 schema、golden set、基线查询与 eval harness（冻结门控）

**输入**：#28 确认的 golden set 定义（§6，含语言口径）与指标口径（§7）。
**输出**：
- **冻结前（文档层）**：golden set JSON schema 文档（`docs/working/`，含字段定义与 query_language 判定规则）、≤10 条样例 case、评测设计（指标公式/harness 结构/CI 策略）。
- **冻结后（可执行）**：`069_rag_evaluation.sql` + PostgreSQL golden set/run 真相源 + 可校验的确定性 JSON 子集 + harness + 基线报告。

### 合同（冻结后部分）

- `eval_golden_cases`：`id BIGSERIAL PRIMARY KEY`、`created_at`、`case_key`、`schema_version`、`query`、`query_language`、`viewer_context jsonb`、`relevant_evidence jsonb`（content_id/content_version/source_start/source_end）、`relevant_content_ids`、`expected_citations`、`forbidden_content_ids`、`answer_rubric`、`classification`、`is_active`；UNIQUE `case_key`。
- `eval_runs`：`id BIGSERIAL PRIMARY KEY`、`created_at`、`run_key`、`dataset_checksum`、`retriever_version`、`chunking_version`、`index_version`、`metrics jsonb`、`environment jsonb`、`artifact_path`；UNIQUE `run_key`。PostgreSQL 是完整数据与 run 的唯一真相源。
- `backend/testdata/rag_golden_cases.json` 是从数据库确定性导出的 CI 子集（schema_version 1），文件内记录 dataset checksum；测试必须验证 schema/checksum 与导出逻辑一致，禁止人工维护出第二份真相。初始完整集 ≥50 case，冷门 ≥20%，zh ≥50% / en ≥20% / mixed ≥10%。
- 语料：站内真实 published 内容（本地 seed 数据集 + 人工标注）；Markdown 仅作无网络演示数据。
- Harness：`backend/internal/service/rag_eval/`（纯测试包）——固定 seed、确定性 tie-break；run 写入 `eval_runs`，再导出带 checksum 的 JSON artifact。指标计算：Recall@K、MRR、nDCG@10、citation precision/coverage、groundedness、answer relevance、visibility leak count、P95 检索延迟、降级成功率。
- 基线查询：对同一 golden set 跑「现状 keyword-only」与「现状 vector-only」，固定输出 `backend/testdata/rag_eval_baseline.json`。
- CI：确定性 subset（12 case 既有 agent eval + RAG case）进 `go test`；全量评测在 full-infra 下运行，不在 CI 常态执行。

**精确文件范围**：
- Create：`backend/migrations/069_rag_evaluation.sql`
- Create：`backend/internal/model/rag_evaluation.go`
- Create：`backend/internal/repository/rag_evaluation_repo.go`
- Create：`backend/testdata/rag_golden_cases.json`
- Create：`backend/internal/service/rag_eval/`（harness + 指标）
- Create：`backend/testdata/rag_eval_baseline.json`（基线结果，指标数字为真实运行测得，不预填）
- Test：harness 自身单元测试 + fixture schema 校验测试
- 不改生产 handler 或检索行为；评测 repository 仅拥有上述两张评测表。

**完成标准**：
- 冻结前：文档层 schema/样例/评测设计经用户确认归档。
- 冻结后：migration/model/repository/harness 可用；完整集、CI 导出子集与 checksum 一致；基线报告数字为真实测量；迁移/fixture/harness 测试与项目验证门全绿；两阶段审查通过。

---

## T04 [heavy] chunk/index projection migration 与内容版本合同

**输入**：#28 §4.5/§4.6（分块策略与 chunk 数据模型，含双世代唯一键与稳定 chunk identity）；T02 的事件 schema（content.published/updated/banned/deleted）。
**输出**：`rag_chunks` + `chunk_embeddings` + `index_projection_status` 三表与模型、chunker（结构感知切分）与内容版本绑定合同。

### Migration 合同

- 固定迁移 `071_rag_chunks.sql`；执行前重新核对 069/070/071 未冲突。
- `rag_chunks`：`id/created_at/content_id/content_version/chunk_index/chunk_key/chunking_version/heading/text/source_start/source_end/zone/content_type/category/ip/tags/index_version`；UNIQUE `(index_version, chunk_key)`，并以 `(content_id, content_version, chunking_version, chunk_index)` 保证同一世代顺序唯一。`chunk_key` 是 `content_id/content_version/chunking_version/source_start/source_end` 规范串的 SHA-256。
- `chunk_embeddings`：id/created_at + chunk_id FK UNIQUE + **`embedding vector(1536)`（固定，已裁决；维度变化走迁移+新世代，不做运行时可配维度）** + embedding_model + embedded_at。
- `index_projection_status`：id/created_at + UNIQUE `(content_id,index_version)` + `chunking_version/embedding_model/state/error_summary/last_indexed_at/is_current`；查询与引用只能读取 current generation，并同时过滤 index_version、chunking_version、embedding_model。
- 内容版本合同：分块只消费 `content_versions` 中该 content 的 `is_latest` 已发布版本；引用中的 content_version 即该版本号。
- chunker：标题/段落/Markdown heading 感知切分，`chunk_max_tokens=512`/`chunk_overlap_tokens=48`（已裁决初值，评测后定值）从 config.yaml 读取；纯段落无 heading 时以内容标题为前缀、heading 可空。

**精确文件范围**：
- Create：`backend/migrations/071_rag_chunks.sql`
- Create：`backend/internal/model/rag_chunk.go`
- Create：`backend/internal/service/rag/chunker.go`（深模块起点）
- Create：`backend/internal/repository/rag_chunk_repo.go`
- Modify：`backend/config/config.go` + `config.yaml`（`features.rag_hybrid_enabled` + `rag.chunking:` 段，见 #28 §8.1）
- Test：`backend/internal/model/rag_chunks_migration_test.go`（空库/历史 fixture、宪法字段、双世代唯一键）、chunker 单元测试（heading/段落/超限硬切/无 heading 场景）

**故障演练**：分块失败事件入 DLQ；`index_projection_status.state=failed` 可重试；不阻塞发布链路。

**完成标准**：红→绿；chunker 边界用例全覆盖；doc-validator `--fix`；两阶段审查；单一提交。

---

## T05 [heavy] OpenSearch full-infra profile、indexer 与 full rebuild

**输入**：T04 的 chunk 表与配置；T03 的队列/worker 基座；**宪法 MINOR 修订已完成（OpenSearch 入栈，见可靠异步计划「宪法修订/批准前置项」）**。
**输出**：OpenSearch 投影（BM25 关键词 + 过滤字段）与事件驱动增量索引 + 可审计全量重建。

### 合同

- `docker-compose.yml`：`opensearch` 服务挂 `full-infra` profile（+健康检查 + 最小 seed）；默认 profile 不启动；单节点模式（`DISABLE_SECURITY_PLUGIN=true` 演示用，生产语义另议）。
- 文档模型：`omnicraft-rag-v<index_version>` 索引 + 固定 alias 读侧；字段：`chunk_key/content_id/content_version/chunking_version/index_version/embedding_model`、title/heading/text、source_start/source_end、zone/content_type/category/ip/tags/status；只索引 published/current generation。
- Indexer：`indexer_worker`（消费 content.published/updated/banned/deleted）→ 读真相源最新版本 → 分块（复用 T04 chunker）→ 写 chunk 表 → **embedding 由 indexer 内联写入（已裁决：不复用内容级 `content.embedding` 通道，chunk 级 embedding 语义不同，改造既有通道成本高）** → OpenSearch bulk 写入 → 更新 `index_projection_status`（含世代）。
- Full rebuild：**admin 端点 `POST /api/v1/admin/rag/rebuild`（已裁决：不用独立 cmd；端点带 admin 鉴权 + 审计日志，与 DLQ replay 同一模式）**：递增 index_version → 全量重建 → alias 原子切换 + `is_current` 翻转 → 保留一个旧世代可回退。
- 降级检测：OpenSearch 健康检查（`cluster health` 轮询）；不可用 → 检索层降级到 PostgreSQL 关键词（T06 消费）。

**精确文件范围**：
- Modify：`docker-compose.yml`（opensearch + profile）
- Create：`backend/internal/repository/opensearch_repo.go`（bulk/mapping/alias/health）
- Create：`backend/internal/worker/indexer_worker.go`
- Create：`backend/internal/handler/admin_rag.go`（rebuild 端点，admin 鉴权 + 审计）
- Modify：`backend/internal/router/routes.go`、`backend/config/config.go` + `config.yaml`（`rag.index:` / `rag.hybrid:` 段，`features.rag_hybrid_enabled` 默认 false）
- Modify：`backend/internal/container/container.go`、`backend/cmd/worker/main.go`（注册 indexer）
- Test：mapping/alias/health 集成测试（opensearch container）、indexer 事件消费测试、rebuild 端点鉴权/审计/正反路径测试、rebuild 幂等与世代切换测试

**故障演练**：
- 停 OpenSearch → indexer 重试/DLQ；恢复后追平。
- 重建中途失败 → 旧 alias 继续服务；重跑 rebuild 收敛。
- 重复投递同事件 → 幂等不重复索引。

**完成标准**：full-infra profile 一键启动含 OpenSearch；增量索引 + rebuild 演练证据；rebuild 端点安全面测试通过；两阶段审查；单一提交。

---

## T06 [heavy] keyword + vector + RRF 与降级

**输入**：#28 §4.2/§4.9（RRF 合同含同 chunk 累加贡献与可见性过滤后补位；降级矩阵）；T04 chunk/embedding 投影；T05 OpenSearch 投影；T01 golden set 基线。
**输出**：应用层混合召回管线（hybrid retriever + RRF + 可见性复核补位 + 降级），并以 golden set 实测对照基线。

### 合同

- `rag.HybridRetriever`：BM25 topK=200（OpenSearch；不可用降级 PG keyword `SearchContents`）→ pgvector topK=200（修复 `VectorSearch` 缺口：查询增加状态/visibility 谓词）→ 应用层 RRF（k=60，**同 chunk 跨检索器累加贡献**，确定性 tie-break（稳定 chunk identity 字典序））→ top20 过 `ApplyContentVisibilityScope` + content_version 校验 → **不足目标数量从剩余候选按序补位并逐一复核** → top10 输出。
- 降级矩阵（#28 §4.9）：OpenSearch 挂 → PG keyword（`degraded=keyword_pg`）；embedding 挂 → keyword-only；一侧为空 → 单侧直返（`source=bm25|vector`）。
- 来源标记：结果携带 source（bm25/vector/hybrid_rrf）供 citation DTO 消费；**不输出 score 到客户端**（#28 裁决：score 仅存服务端 trace）。
- 评测门：同一 golden set 上 RRF 指标 ≥ 基线（keyword-only/vector-only），记录对比表。**不实现 alpha 加权融合、不设评测开关（已裁决：删除 T06 的 alpha 实现开关）**。

**精确文件范围**：
- Create：`backend/internal/service/rag/retriever.go`（hybrid + rrf）
- Modify：`backend/internal/repository/embedding_repo.go`（过滤谓词；维度保持 1536 固定）
- Modify：`backend/internal/repository/search_repo.go`（仅必要 seam，降级共用）
- Modify：`backend/config/config.go` + `config.yaml`（`rag.hybrid:` 实装生效）
- Test：RRF 单测（累加贡献/tie-break/空集/补位）、降级矩阵集成测试、golden set 对照 run
- 前端：无 UI 改动

**故障演练**：停 OpenSearch、停 embedding API、恢复后一致性 —— 三项截图/日志证据。

**完成标准**：golden set 对照报告（RRF vs 基线）真实数字；降级矩阵演示；`go test ./...`/vet/build 绿；两阶段审查；单一提交。

---

## T07 [heavy] Agent 工具、可见性复核、citation DTO 与 SSE 合同

**输入**：#28 §4.7（citation DTO 修订版：无 visibility_checked/score）与 §4.10（单 Agent 边界）；T06 retriever；现有 `agent_tools.go`/`agent_stream.go`/`agent_contract.go` 基线。
**ui_spec_ref**（宪法 XIV，编码前必读）：`design/ui-spec.md` **`## Page: /agent`（:3424）**——引用卡与 Agent 工作台的交互/状态变体/可访问性权威；`AgentCitationList` 类型扩展不得违反该节的「引用必须是站内有效 id/title/zone 形成的可聚焦引用卡片」与 no-evidence/degraded 状态机。
**输出**：`search_content` 工具接入混合召回；引用 DTO 扩展；SSE citation 事件携带新字段；端到端站内问答可用。

### 合同

- `search_content`：参数与可见性语义不变（只返回 viewer 可见 published），召回内部替换为 HybridRetriever；返回带 `source` 的摘要列表（扩展 `AgentContentSummary`，向后兼容；不含 score）。
- citation DTO（#28 §4.7 修订版逐字段）：`content_id`/`content_version`/`chunk_key`/`title`/`zone`/`route`（服务端构建）/`excerpt`/`source`；**无 `visibility_checked`、无 `score`**。
- 服务端复核：沿用 `RevalidateCitations`（能输出即已复核）；**复核结果（通过/丢弃原因）写入 trace/日志**（#28 D7），供审计与评测断言。
- SSE：`citation` 事件 payload 扩展；`done` 事件保持 trace_id + degraded；UI 复用 `AgentCitationList`（类型扩展，无视觉权威改动）。
- 评测：既有 12 case fixture 升级覆盖新 DTO；确定性断言（source 枚举、稳定 identity 存在性、丢弃路径计数）。
- 多智能体：不进入本 Task（对照实验另行设计，见 T10）。

**精确文件范围**：
- Modify：`backend/internal/service/agent_tools.go`（search_content 接入、citation 构建、复核 trace）
- Modify：`backend/internal/service/agent_stream.go`（citation 事件扩展）
- Modify：`backend/internal/service/agent_contract.go`（AgentCitation 扩展）
- Modify：`backend/internal/service/agent_service.go`（NLSearch 走 retriever）
- Modify：`backend/testdata/agent_eval_cases.json`（字段升级）、`backend/internal/service/agent_evaluation_test.go`
- Modify：`frontend/components/agent/AgentCitationList.tsx`（类型/展示字段，最小改动；遵守 ui-spec `## Page: /agent`）
- Test：citation 契约测试（伪造/越权/不可见丢弃）、SSE 事件断言、前端类型测试

**故障演练**：引用 content 在检索后被封禁/删除 → 输出前复核丢弃（trace 记录）；模拟越权引用 → 不进入 SSE。

**完成标准**：端到端问答 + 引用 + 可见性用例在浏览器与接口层双验证（截图，含引用卡交互）；泄漏 count=0；两阶段审查；单一提交。

---

## T09 [closure] 故障演练、全量验证、指标归档与文档同步

**输入**：T01–T08 全部完成。
**输出**：纵切片总体证据包。

### 演练清单（每项须有日志/截图证据）

1. 内容导入 → 建索引 → 正常问答与引用。
2. 停 OpenSearch → 关键词降级；停 embedding → keyword-only。
3. 重复投递事件 → 幂等；DLQ 事件 → replay 收敛。
4. 索引 rebuild（世代切换 + admin 端点鉴权/审计验证）。
5. 越权/隐藏内容全链路拦截（visibility leak count = 0）。
6. OTel trace 全链可视化（Jaeger UI 截图）。

### 验证门

- `bash scripts/verify-project.sh --full`（含契约测试）；`go test ./...`/vet/build；前端 `npm run build`/`lint`。
- 指标归档：T01 harness 全量 run 指标表/JSON 落盘（Recall@K/MRR/nDCG/citation precision/coverage/groundedness/relevance/leak/P95/first-token/token per request/降级成功率）—— **数字全部真实测量，供 #32 证据合同消费**。
- 文档同步：doc-validator `--fix` 刷新 `architecture.md`/`docs/reference/{schema,api,config}.md`；把真实证据回填 #32 证据合同。

## T10 [decision gate] 拆 Agent/Search、gRPC 与两节点 workflow 决策

**输入**：T09 证据包。
**输出**：更新固定 ADR `docs/adr/0005-agent-rag-runtime-boundary.md` 的证据与结论：

- 是否拆 `agent-search` 独立进程 + gRPC（触发条件：独立扩缩容/故障隔离证据，tech-selection §3/§5）；未满足则保留模块化单体并记录。
- 两节点 `Research Agent → Answer Agent` 对照评测结果（相对单 Agent 基线：引用正确率/groundedness/relevance/泄漏/P95/token 成本/tool 成功率）；达升级标准才登记 Implemented，否则关闭实验。
- 不引入运行时代码；只更新 ADR 0005；新设计文档必须由后续用户明确批准后另行创建。
- **T10 是后续 decision gate（已裁决）**：不参与 #32 的 blocked_by（#32 仅 blocked_by T01–T09）；T10 结论可能改写后续 Implemented 晋升判断，但本身不阻塞证据合同收口。

---

## 里程碑与 DAG（与可靠异步基础计划共表，详见该文档 §里程碑）

M0 批准 →（冻结门）→ M0.5 T00 → M1 T01 → M2 T02→T03 → M3 T04→T05 → M4 T06→T07 → M5 T08 → M6 T09 → M7 T10（非 #32 blocker）。

## Stop conditions

- 迁移编号冲突、冻结门未关闭、GitHub 依赖与路线图不一致 → 停止先修正。
- OpenSearch/embedding Provider 等外部输入缺失、宪法修订未完成 → 按 AGENTS.md 阻塞处理，不以 mock 证据替代真实验证门。
- 共享文件（config.yaml、container.go、docker-compose.yml、agent_contract.go）预约冲突 → 串行等待。

## Closure（对应 T09）

- [ ] 全部演练 6 项证据归档；验证门全绿；指标为真实测量。
- [ ] 本计划 Ticket 在 GitHub 全部关闭；冻结门边正确。
- [ ] #32 证据合同被真实指标填充，或明确记录未达到的证据缺口。
