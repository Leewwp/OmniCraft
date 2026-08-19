# OmniCraft 可靠异步与可观测性基础路线图

> **创建日期**：2026-08-11
> **状态**：`accepted`（2026-08-11）。本路线图只编排任务；运行时实现必须等待 GitHub 原生冻结门解除。
> **修订（2026-08-13）**：本地开发模式（无生产部署）下，本计划不再等待 #134 冻结门（#134 仅作生产发布门，deferred）。按注册表备注的 lane 内 DAG 在本地执行；恢复生产部署时重新受 #134/#76 门约束。
> **来源（消费输入）**：
> - GitHub issue #31（执行编排，本票主体）、#28（RAG 深化设计，确认后生效）
> - `/Users/pp/Desktop/file/code/project/OmniCraft-tech-selection-analysis.md` §6/§7/§11（设计输入，不是权威源）
> - `#30` resolution（2026-08-11）：运行时增强等 Web-only DAG 收口后执行
> - 代码审计：`backend/internal/pkg/queue/`、`backend/internal/worker/`、`backend/internal/container/`、`backend/internal/observability/`（2026-08-11）
> **执行追踪**：冻结门 #134；T00 #135；T02 #137；T03 #138；T08 #143；总体 closure #144

## Goal

在 Web-only DAG 收口后，把现有「进程内 goroutine 队列 + 未接线的幂等 + 无独立 Worker + 零 OTel」的现状，升级为可演示、可解释、可追问的可靠异步基座：

1. Transactional Outbox（数据库事务内写事件，relay 投递 Redis Streams，at-least-once）。
2. 数据库级消费幂等（Inbox 完成记录与业务副作用同事务）+ 有界重试 + DLQ 可 replay。
3. 独立 Worker 进程（`cmd/worker`，与 backend 解耦，REST/SSE 外部合同不变）。
4. OpenTelemetry 跨 HTTP/队列/DB/LLM 追踪（**Jaeger**，已裁决）。

## 与现有计划的关系

- 本计划排在当前 Web-only 活计划注册表之后；`#31` 的 GitHub 原生 `blocked_by` 指向 #28；本计划每个运行时 Ticket 再被「Web-only 合同冻结门」阻塞（terminal blockers 见下方）。
- 不改变现有业务票顺序；不占用 `064–068` 迁移编号；不新增共享业务合同改造。
- 新 tickets 创建时作为 #22 子票、带 `wayfinder:` 标签、使用原生 `blocked_by`（用户裁决，仓库卫生）。

## Web-only 合同冻结门（terminal blockers，2026-08-11 DAG 实算，已修订）

从 GitHub 原生 `issue_dependencies_summary` 逐条审计后，当前 Web-only 注册表 DAG 的**叶子节点**（terminal blockers）为：

- **#76**（favorites cutover，heavy，`ready-for-human`）——链尾：...→#73→#74→#75→#76；**#97（collaboration-invites）是 #76 的传递上游（#97→#69/#71/#72→#73→…→#76），不重复挂边**：冻结门挂 #76 即传递覆盖 #97/#96/#90 等整条链
- **#108**（内容安全 Ticket 05 端到端验证与文档收口）——链尾：#104→#106/#107、#105→#108
- **#112**（私信文本审核）——链尾：#109→#112
- **#111**（头像图片审核，heavy）
- **#113**（反馈附件图片审核）

已核对的非阻塞项：#65/#66/#67/#70/#82/#83/#84 已关闭；#101（Load gate）已于 2026-08-10 关闭且 CI Load gate 全绿；#64/#80/#103 为 spec 父票不阻塞；#114/#115 为追踪票不阻塞。

**冻结门定义**：单张 GitHub 追踪 issue「Web-only DAG 收口冻结门（Agent/RAG 运行时增强前置）」，原生 `blocked_by` 上述 **5 个** terminal blockers（#76/#108/#111/#112/#113）；**全部运行时 Ticket（T02–T08）以及 RAG 纵切片的 T01 可执行 harness/fixture/基线运行** 都 `blocked_by` 该冻结门。冻结门未关闭前不得开工。

## 宪法修订/批准前置项（新系统服务，已修订 2026-08-11）

宪法 Principle I 冻结技术栈；引入 OpenSearch、OTel Collector、Jaeger 三个系统服务属于栈外新增，**必须在各自实现 Ticket 开始前完成宪法 MINOR 修订**（栈表增补 + 依赖管理文件 + changelog），修订正文须含以下与现有方案的比较：

| 新服务 | 现有方案（替代） | 引入理由 | 代价 | 裁决 |
|---|---|---|---|---|
| OpenSearch（T05 前置） | PostgreSQL FTS（tsvector/pg_trgm）+ pgvector 全栈检索 | 过滤聚合（zone/category/tags）与 BM25 质量分离、索引生命周期与内容库隔离、多字段过滤聚合能力强 | +1 容器（full-infra only，默认 profile 不启）；内存 512MB–1GB | 保留现有 PG 检索为降级路径（D9），不替换 |
| OTel Collector（T08 前置） | 应用 OTLP 直出 Jaeger（不加 collector） | 集中采样控制、批量/重试/缓冲、未来多服务共用 exporter 拓扑；与现有 Prometheus scrape 面分离 | +1 容器；一处配置面 | 应用只配置 Collector OTLP 端点；Collector 离线时丢弃遥测并告警，不切换业务拓扑 |
| Jaeger（T08 前置） | Tempo（#24 曾以 Tempo 为优先候选，与 Grafana 衔接更好） | 用户裁决；all-in-one 单容器本地演示完整（Query/UI/Collector/Agent），无外部存储依赖（Tempo 全功能需对象存储），Docker 生态成熟 | +1 容器（full-infra only）；内存 256–512MB | **已裁决 Jaeger**；记录 Tempo 为替代方案，不双引入 |

ClamAV 同属新系统服务，由同一治理票覆盖。宪法修订为 MINOR，执行顺序固定为：更新 Principle I 栈表与版本/日期 → 更新顶部 Sync Impact Report → 核对 `.specify/templates/{plan,spec,tasks}-template.md` 是否需要传播 → 运行相关文档校验 → 以 `docs: amend constitution for interview infrastructure` 单独提交。T00 关闭前，OpenSearch、OTel Collector、Jaeger、ClamAV 的实现票不得开工。

## 车道与执行纪律

- T00 为 governance-only；T02/T03/T08 全部 **heavy**：一 Task 一 worktree、一分支、一 commit；先红测试（TDD），两阶段审查（规格符合性 → 代码质量）。
- **heavy ticket 内所有「或」选项必须在建票前裁决**（本轮已裁决项：relay 拓扑、Jaeger、GORM span 方式、rebuild 入口、embedding 写入路径，见各 Task）。
- 涉及 migration / config / routes / worker 进程 / 可观测性基线 → 一律 heavy。
- 真实凭证或外部依赖缺失时按 AGENTS.md 阻塞处理，不以模拟证据替代验证门。
- 修改 config/migrations/routes 后运行 `cd tools/doc-validator && go run . --fix`。

---

## T00 [governance] 宪法新增系统服务修订

**输入**：本路线图的新服务比较表、RAG 路线图 T05/T08、archive malware scanning spec。
**输出**：一张独立治理 ticket 和一次宪法 MINOR 修订，明确 OpenSearch、OTel Collector、Jaeger、ClamAV 的定位、替代方案、默认/full-infra profile 边界与退出条件。

**精确文件范围**：`.specify/memory/constitution.md`；仅在传播检查确有需要时修改 `.specify/templates/{plan,spec,tasks}-template.md`；`progress.txt`。

**完成标准**：版本、日期、Sync Impact Report 与正文一致；四项服务逐项给出现有替代、引入理由和运行成本；文档校验通过；独立提交 `docs: amend constitution for interview infrastructure`。T00 是 T05、T08 及 #110 ClamAV 实现票的原生 blocker。

---

## T02 [heavy] Outbox/Inbox migration 与四类内容事件接线

**输入**：#28 确认的事件面（content.published / content.updated / content.banned / content.deleted 为 RAG 消费所需最小事件集）；tech-selection §7.1 的 outbox 语义。
**输出**：可靠事件写入层 + 稳定事件 schema + 幂等消费记录；`content.published/updated/banned/deleted` 四类状态变化全部在真实业务事务内接入。

### Migration 合同

- 固定迁移 `070_outbox_inbox.sql`；若执行前 `069` 或 `070` 已被占用，停止并同步修订两份路线图与 tickets。
- `outbox_events`：`id BIGSERIAL PRIMARY KEY` 同时作为稳定 `event_id`，另含 `aggregate_id`、`event_type`、`schema_version`、`payload jsonb`、`traceparent`、`tracestate`、`status`、`attempts`、`next_attempt_at`、`sent_at`、`created_at`；索引 `(status, next_attempt_at)`、`(aggregate_id, event_type)`。重试投递复用同一 `id`。
- `inbox_consumers`：`id`、`created_at`、`consumer_group`、`event_id BIGINT`、`consumed_at`，UNIQUE `(consumer_group, event_id)`；存量 Redis SetNX 幂等迁移到新机制。
- 事件 schema：JSON 文件 + Go struct（`backend/internal/pkg/events/`），envelope 固定为 `{event_id,event_type,schema_version,aggregate_id,occurred_at,traceparent,tracestate,payload}`；每个事件类型有 schema 校验，W3C trace context 可跨 HTTP → outbox → Redis Streams → Worker 传播。
- **四类真实生产者（本 Task 必须全部接入）**：`review_service.go` 负责发布审核通过产生 `content.published`、封禁终态产生 `content.banned`；`content_service.go` 负责已发布内容修改产生 `content.updated`、删除产生 `content.deleted`。每类业务状态更新与 outbox 插入必须在同一数据库事务内提交。批处理或历史直接更新由 T05 full rebuild/reconciliation 收敛，不伪装成实时事件。

**精确文件范围**：
- Create：`backend/migrations/070_outbox_inbox.sql`
- Create：`backend/internal/pkg/events/`（envelope struct + 校验 + topic 常量）
- Create：`backend/internal/repository/outbox_repo.go`（事务内写事件、领取待投递、标记 sent、失败计数）
- Modify：`backend/internal/container/container.go`（事件写入器装配）、`backend/internal/model/`（outbox/inbox model）
- Modify：`backend/internal/service/review_service.go`、`backend/internal/service/content_service.go`（四类生产者）
- Test：`backend/internal/model/outbox_migration_test.go`、`backend/internal/repository/outbox_repo_test.go`、四类业务事务接入的集成测试

**测试**：
- 迁移测试：空库与 historical fixture 升级；UNIQUE 约束、索引存在性、宪法字段（id/created_at）存在性。
- 单元：同事务写事件与业务写入原子性（回滚场景事件不残留）；领取 pending 事件的行级锁语义（`FOR UPDATE SKIP LOCKED`）；失败重试计数与 `next_attempt_at` 退避；event_id 稳定（重试/重复投递不变）。
- 集成：四类真实事务逐项验证提交→正确事件入 outbox、回滚→outbox 无残留；relay 故障不丢事件；trace context round-trip 不变。

**完成标准**：
- 红测试先绿（先写预期失败的迁移/仓储测试再实现）。
- 四类业务事务接入测试通过（各自提交/回滚两个方向）。
- `go test ./...`、`go vet ./...`、`go build ./...` 通过；doc-validator `--fix` 通过。
- 两阶段审查通过；单一提交落 worktree 分支。

---

## T03 [heavy] relay、幂等消费、replay 与独立 Worker

**输入**：T02 的 outbox/inbox 表与事件 schema；现有 `pkg/queue` Redis Streams 封装（重试退避/DLQ/积压指标已具备）。
**输出**：Outbox relay + 幂等消费（at-least-once）+ DLQ replay 管理端点（admin 鉴权 + 审计）+ 独立 Worker 进程。

### 合同

- **relay 拓扑（已裁决）**：relay 固定运行在独立 `cmd/worker` 进程内（不保留「进程或 goroutine 二选一」）；轮询 outbox pending → `XAdd` 到 `omnicraft:<topic>` → 标记 sent。
- **投递语义（已裁决）**：**at-least-once**——relay 在「已 XAdd 但未标记 sent」崩溃时允许重复投递，由消费者侧幂等兜底；不承诺 exactly-once。
- **幂等（已裁决）**：消费者**在同一数据库事务内**完成「Inbox 完成记录写入 + 业务副作用提交」；`inbox_consumers` UNIQUE 冲突即跳过（重复事件不重复执行副作用）；存量 Redis SetNX 幂等迁移到新机制，不双轨长期并存。
- **重试/DLQ**：沿用现有 `RetryBackoffSec` 指数退避与 `omnicraft:dead-letter`；DLQ 条目新增 `consumer_group` 字段（schema 兼容扩展）。
- **replay（已裁决补齐安全面）**：`POST /api/v1/admin/queue/dlq/:id/replay`（挂载现有 `DLQWorker.Replay`，当前无路由）；**admin 鉴权（admin 角色中间件）** + **审计日志**（操作者/时间/条目 ID，遵循现有 audit-logs 模式）；正路径（replay 后消费成功）与反路径（条目不存在/非法 ID/无权限）测试。
- **独立 Worker（唯一运行模型）**：新增 `cmd/worker/main.go`，启动 review/notification/count/embedding/indexer worker 与 relay；`docker-compose` 默认 profile 必须启动 `worker`，full-infra 复用同一服务。backend 不再启动任何异步消费者或 relay，不提供 `worker.external=false` fallback。
- **配置**：`config.yaml` 新增 `worker.enabled/concurrency`；只由 `cmd/worker` 消费。server 可在 Worker 暂停时继续接受请求并积压 outbox，但“完整默认 Compose”必须包含 Worker。

**精确文件范围**：
- Create：`backend/cmd/worker/main.go`
- Create：`backend/internal/service/relay_service.go`
- Create：`backend/internal/worker/relay_worker.go`
- Modify：`backend/internal/pkg/queue/redis_stream.go`（DLQ 条目 consumer_group）、`backend/internal/worker/dlq_worker.go`（Replay 路由挂载）、`backend/internal/handler/admin.go`（replay 端点 + admin 鉴权）、`backend/internal/router/routes.go`
- Modify：`backend/internal/container/container.go`（worker 装配拆分）、`backend/config/config.go` + `backend/config/config.yaml`（worker 段）
- Modify：`docker-compose.yml`（worker 服务 + profile）
- Modify：`backend/internal/worker/review_worker.go` 等（Inbox 幂等同事务接线，修复 IdempotentCheck 未注入）
- Test：relay 集成测试（redis container）、replay 端点测试（**正/反路径 + 鉴权 + 审计断言**）、幂等消费并发测试、worker 优雅关机测试

**故障演练（完成前必须演示）**：
- 停 redis → relay 暂停不丢事件 → 恢复后追平。
- 手动重复投递同 event_id → 消费幂等不重复执行（副作用只发生一次）。
- 永久失败事件 → DLQ → 修复后 replay 成功（含审计记录可见）。
- backend 停、worker 独立存活 → 消费不中断；反之亦然。

**完成标准**：
- 红→绿测试证据齐全；故障演练 4 项全部演示并截图/日志归档。
- replay 端点鉴权/审计/正反路径测试通过。
- 独立 Worker 进程启动、消费、关机日志完整；默认 Compose 的 backend + worker 可独立启停。
- 两阶段审查通过；单一提交。

---

## T08 [heavy][completed-local] OpenTelemetry 跨 HTTP/队列/DB/LLM 追踪

**输入**：T03 的队列链路；现有 slog JSON + Prometheus 基线（`observability/`）；#24 选型结论被本票修正：**Jaeger（已裁决）**，Tempo 不引入。
**输出**：OTel SDK 接入 + GenAI 语义约定 + 端到端 trace（HTTP → worker → DB/LLM/队列）+ 本地演示栈（Jaeger）。

### 合同

- 依赖（已裁决）：`go.opentelemetry.io/otel` + OTLP exporter + `gorm.io/plugin/opentelemetry/tracing` + `go.opentelemetry.io/contrib/instrumentation/net/http`。
- **`X-Request-ID` 与 OTel `trace_id` 分离（已裁决）**：现有 `X-Request-ID` 中间件保持独立 request-correlation 语义（8 字节 hex，日志/审计沿用，不废除）；OTel `trace_id` 为独立 128-bit 标识；SSE 事件 `trace_id` 字段承载 OTel trace id（与日志 trace 字段一致），请求日志同时记录 X-Request-ID 与 trace_id 两列并互相可查；废除 Agent `newTraceID()` 独立 32-hex。
- 队列 span：produce（XAdd）/consume（XReadGroup）/relay 轮询；GenAI span：LLM provider（`gen_ai.*` 语义约定：model/usage/temperature 脱敏，不记录 prompt 正文）。
- DB span：GORM plugin span（SQL 参数脱敏，遵守现有日志脱敏规则）。
- 采样（已裁决）：**仅 head-based ratio**（配置 `observability.tracing.sample_ratio`，开发 1.0 / 生产建议 ≤0.1）；**不承诺「错误全采样」**（head 采样在错误发生前决策，无法保证错误 span 全覆盖；错误率由 Prometheus 指标承担）。
- 演示栈：`docker-compose` full-infra profile 增加 `otel-collector` + `jaeger`（all-in-one）；`config.yaml` 新增 `observability.tracing:` 段（enabled/endpoint/sample_ratio/backend=jaeger）。
- 中间件：HTTP 入口创建 root span（复用现有 request_id 中间件 seam，两个 id 并存）。
- 前置：Jaeger/OTel Collector 的宪法 MINOR 修订在 T08 开工前完成（见「宪法修订/批准前置项」）。

**精确文件范围**：
- Modify：`backend/go.mod`（otel 依赖）、`backend/internal/observability/`（tracer provider、span 装配）、`backend/internal/middleware/`（trace 中间件）、`backend/internal/pkg/llm/*.go`（GenAI span）、`backend/internal/pkg/queue/`（队列 span）、`backend/cmd/server/main.go` + `backend/cmd/worker/main.go`
- Modify：`backend/config/config.go` + `backend/config/config.yaml`（tracing 段）、`docker-compose.yml`（otel-collector/jaeger full-infra）
- Modify：`backend/internal/service/agent_tools.go` / `agent_stream.go`（trace_id 收敛为 OTel）
- Test：trace 注入单元测试、GORM plugin span 测试、LLM span 语义测试、X-Request-ID/trace_id 并存断言

**故障演练**：
- 停 otel-collector → 应用无感继续（exporter 失败仅告警）；恢复后 trace 续写。
- SSE 请求全链 trace 在 **Jaeger UI** 可视化（截图）。

**完成标准**：
- Jaeger UI 展示「HTTP → worker → DB → LLM」完整链路截图；SSE trace_id 为 OTel trace id 且与日志一致；X-Request-ID 独立并存。
- 降级：collector 离线不影响业务请求成功。
- 两阶段审查通过；单一提交。

**完成记录（本地开发）**：T08 实现提交包含公共 seam 测试（head sampling、HTTP ID 分离、HTTP→outbox→queue W3C 往返、GORM 参数脱敏和 LLM GenAI span）。Collector/Jaeger 容器与真实 UI 链路未在当前环境执行，不宣称为生产证据；复验命令和缺口记录于 `docs/working/2026-08-19-t143-otel-evidence.md`。

---

## 里程碑与 DAG（与 RAG 纵切片计划共表）

| 里程碑 | 含义 | 依赖 |
|---|---|---|
| M0 | 计划批准 + tickets 创建 + 冻结门接线 | #31 已完成 |
| M0.5 | T00 宪法治理完成 | 冻结门；T05/T08/ClamAV 实现前置 |
| M1 | golden set 可用（纵切片 T01；**可执行 harness 受冻结门阻塞**，冻结前仅文档层 schema/样例/评测设计） | RAG design + 冻结门 |
| M2 | 可靠异步基础可用（T02+T03） | 冻结门；T03←T02 |
| M3 | chunk 投影 + OpenSearch 索引（T04+T05） | T04←T03；T05←T04 |
| M4 | hybrid 检索 + Agent 接入（T06+T07） | T06←T05+T01；T07←T06+T01 |
| M5 | OTel 全链追踪（T08） | 冻结门 + T03；与 T04–T07 按文件预约串行（默认 T07 后） |
| M6 | 总体 closure（T09：故障演练/全量验证/指标归档） | T01–T08 |
| M7 | 决策门（T10：拆服务/gRPC/两节点 workflow；**不作为 #32 的 blocked_by**） | T09 |

## Stop conditions

- Outbox/Chunk 迁移编号冲突、Web-only DAG 冻结门未关闭、GitHub 原生依赖与本文不一致 → 停止，先修正规划。
- Redis/PostgreSQL 集成环境缺失或真实外部输入缺失 → 按 AGENTS.md 阻塞处理。
- 宪法修订未完成（OpenSearch/OTel Collector/Jaeger）→ 对应 Ticket 不得开工。
- 与 RAG 纵切片计划共享文件（config.yaml、routes.go、docker-compose.yml、container.go）发生预约冲突 → 串行等待，不并行覆盖。

## Closure（对应纵切片 T09 前序检查）

- [ ] T02/T03 各自 red→green、两阶段审查、故障演练证据齐全（记录于 progress.txt）。
- [x] T08 本地实现 red→green、两阶段审查与 Collector schema/离线设计记录已完成（记录于 progress.txt）。
- [ ] T08 真实 Jaeger UI HTTP→worker→DB→LLM 截图（当前环境缺口，恢复 full-infra 后复验）。
- [ ] 本计划相关 Ticket 在 GitHub 关闭；冻结门边正确。
- [ ] 文档同步：`architecture.md`/`docs/reference/*` 经 doc-validator `--fix` 更新。
