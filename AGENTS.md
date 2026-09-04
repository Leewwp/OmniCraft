# OmniCraft 万象工坊 - Agent 工作指南

## 项目概览

全民创意分享平台，技术栈：Next.js（前端）+ Go/Gin（后端）+ PostgreSQL + Redis + 阿里云 OSS + Tauri（PC 客户端）。

详细架构：`architecture.md` | 社区功能设计：`docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md` | 历史任务账本（已归档，仅在用户明确指定历史任务 ID 时查阅）：`docs/archive/task-ledger-mvp.json`

## 工具链版本（强制）

| 工具 | 版本 | 备注 |
|------|------|------|
| Go | 1.22+ | 后端 / Tauri sidecar |
| Node.js | 20+ | 前端 / Tauri 客户端 |
| pnpm 或 npm | pnpm 9+ / npm 10+ | 前端包管理（推荐 pnpm） |
| PostgreSQL | 16+ | 需 pgvector ≥ 0.7 |
| Redis | 7+ | — |
| Rust | 1.75+ | 仅 Tauri 客户端需要 |

CI 与本地 `go.mod` / `package.json` 的 `engines` 字段必须与上表一致。CI 精确固定 Go 1.25.13 / Node 20（见 `.github/workflows/ci.yml` 与 `tauri-ci.yml`），本地只需满足上表最低版本；二者不等价。

---

## 活计划注册表（任务的唯一来源）

当前 Web-only 范围内的待办工作登记在下表。会话开始先查此表，选择**优先级最高且依赖已满足**的计划执行；用户明确指定任务时以用户指定为准。计划完成后从表中移除并归档该计划文件到 `docs/archive/plans/`。表中「执行单元」按 tracker ticket 或计划顶层 Task 计数，不计 Plan Self-Check 复核框。

> **两阶段范围框架（2026-08-27 确认）**：项目按目标分两阶段管理。
> **第一阶段（简历/面试最小实现，当前活跃）**：本地可运行、主要功能可用、支撑页面浏览与 Agent 功能展示，服务于简历撰写与共享屏幕面试演示。范围 = #207（T01/T02/T03 已于 2026-08-28 完成本地实现与验证）+ #204（面试证据编排收口）+ #32（简历表述与 live demo 脚本）；当前目标窗口为 2026-09-03（用户确认面试约一周后）。允许忽略局部细节，不要求云部署。第一阶段完成前不新增注册表计划。**例外追加（2026-09-01）**：UI 精细化批次（SP-12 = #276，见下表优先级 0 行）经用户明确拍板纳入第一阶段收口范围；同日第二例外：Agent 工作台重造批次（SP-13 = #282，见下表优先级 0 行）经用户明确拍板纳入第一阶段（约两周面试窗口），「两个重构」中的 Agent 工作台重构由 SP-13 承担。同日用户裁决：启动简历投递以「全部页面改动落地（U-01~U-05 + T 票 + 两个重构）」为新门槛，原 2026-09-03 目标窗口作废。
> **第二阶段（长期演进 future work）**：统一收容于 Phase 2 roadmap issue **#208**（Agent 技术深化、RAG 准确率、可观测性真实全链路、内容安全真实验证、生产部署与真实凭证、新品类、桌面/App 多端）。任何一项启动时先立 spec/plan 并登记进本表，同时把对应条目移出 roadmap；生产相关门（#76/#134/#151）在启动前一律维持 deferred。
> 两阶段判定以用户 2026-08-27 明确口径为准；与「本地开发模式」条款叠加生效，不互相覆盖。


> **本地开发模式（2026-08-13 起）**：短期不真实上线生产，以本地可运行、可测试、功能完成（本地 live demo）为目标（与 issue #22 基线一致）。影响：① #76（067 drop release）与 #134（Web-only 生产发布冻结门）整体延期——#76 已标 `deferred`，本地不执行 067、不以 mock/local 证据冒充生产证据；#135~#150 本地开发不再被 #134 阻塞（GitHub 依赖与本文同步放宽），仅生产发布仍受 #134/#76 门约束，恢复生产部署时重新激活。② 真实凭证/外部服务缺失（阿里云 AccessKey、Green 内容安全、SMTP、LLM provider 等）在本地开发期不再阻塞，走既有 fail-open/mock/测试替身路径（如 A4 审核语义、fake scanner、`verify-project.sh --full` 的 mocked contracts）；仅当用户明确要求真实发布时才恢复「阻塞处理」的凭证检查。

| 优先级 | 计划文件 | 执行单元 | 车道 | 备注 |
|--------|----------|------|------|------|
| 0 | `docs/superpowers/specs/2026-09-01-omnicraft-ui-refinement-design.md`（执行追踪：GitHub SP-12 总览 #276 + U-01~U-05 = #277~#281，5 张原生子票） | 5 tickets | light | 2026-09-01 grill-with-docs 会话（三轮 16 项决策）产物：六维设计语言裁决（颜色不换 indigo；表面分层亮 #f5f5f5 画布+纯白卡片 / 暗画布 #010409 卡片不动；操作控件 8px 矩形+高度三档 28/36/44-48+「同排同高」；筛选=药丸、IP 库现有效果即选中态基准；150ms 动效；筛选就地切换+URL 同步）；FIX-05 dark token 裁决并入 U-01/U-02，T18 (#238) 收缩为代码修复 blocked_by #278；**本批次插队 T 票 frontier 最前**（U-01→U-02→{U-03,U-04,U-05}）；✅ 执行冻结已解除（2026-09-02 用户整合裁决，跨批次总序唯一权威 = `docs/working/2026-09-02-integrated-execution-order.md`；U-04 已按裁决 C-01 剔除 Agent 工作台 sweep 移交 A-06）；依赖边以票内 `## Blocked by` 正文约定为准（dependencies API 已 404） |
| 0 | `docs/superpowers/specs/2026-09-01-omnicraft-agent-workspace-rework-design.md`（执行追踪：GitHub SP-13 总览 #282 + A-01~A-07 = #283~#289，7 张原生子票） | 7 tickets | 混合（A-01 heavy 含迁移，其余 light） | 2026-09-01 grill-with-docs 会话（R1~R4 四轮 15 项决策）产物：Agent 工作台重造。产品线 = DeepSeek 交互 1:1（完整生成形态：思考折叠流式+工具步骤+逐字真流式、侧边栏选中态/⋯菜单重命名置顶删除、输入自动增高、markdown+行内 [1][2] 锚定），视觉 token 映射本站设计语言（不推翻 SP-12 裁决；保留全局导航+反冗余原则）；架构线 = 会话续写模型（title/pinned_at 迁移+服务端上下文组装+自动标题+停止保留）、SSE 契约 v2（真流式替换伪流式、think 转发不进引用复验、工具步骤事件；同步 web-agent-v0.4-mvp.md）、检索升级（DashScope text-embedding-v4@1536 零 DDL + hybrid on Postgres 不用 OpenSearch + 查询扩展 + qwen3-rerank，三开关由 A-04 消融数据定默认值；gte-rerank 已下线勿用）、护栏补口（chat 输入前置 Green+输出事后异步审核）。**第二第一阶段例外**：拉回 #208 的 Session History/SSE 合同变更与 rag-deepening Future 项（查询改写/rerank，评测门控激活）；「两个重构」中的 Agent 工作台重构即由本批承担。✅ 执行冻结已解除（2026-09-02 用户整合裁决，总序见 `docs/working/2026-09-02-integrated-execution-order.md` 第 6/7 段；A-06 跨批次前置已定为 A-01+A-02+U-02+T18/T19/T20，并承接 U-04 剔除的工作台 sweep）；批内依赖边以票内 `## Blocked by` 正文约定为准。**进度**：✅ A-01（#283，fd4c33e）+ ✅ A-02（#284，8517032）+ ✅ A-05（#287，6cef43b，2026-09-03 PR #294——chat 输入前置 Green 门（A4 语义 422/503/本地 fail-open，先于配额）+ 输出事后异步审核标记 + 历史脱敏）+ ✅ A-03（#285，619cda9，2026-09-03 PR #296——v4 批量嵌入+jieba 词法路消费 041 现役向量+查询扩展+qwen3-rerank 降级链，三开关默认 off 待 A-04；真实冒烟 v4 重嵌入 169/169+verify --full 81/81）（34/69；第 6/7 段已完成 + 第 8 段 D 批 6 票 2026-09-03 道 3 收口；余 33 张 T 票 + #291 + A-04——#291 注入完毕（PR #316，1600/1600）待用户 golden set 复核冻结，冻结后关 #291 → A-04（原 A-04 前置 #319 已修，PR #326；2026-09-04 审计检查点 PR #327 不构成冻结））——A-01 迁移 074/conversation_id 续写/服务端 token 预算组装/自动标题/置顶重命名/停止保留；A-02 SSE v2 真流式/think_delta/工具步骤升级/message_id；请求体已硬切换新契约（旧前端到 A-06 前不可用，用户裁决）；✅ A-06（#288，5010ac7，2026-09-03 PR #302——工作台 DeepSeek 化全量落地：续写契约请求体切换（chat 恢复可用）+ 三层生成形态（思考折叠/工具步骤/逐字）+ 侧边栏 ⋯ 三交互（重命名/置顶/删除）+ 行内 [1][2] 锚定 + markdown 高亮复制外链安全 + 消息复制/重新生成 + 输入自动增高 + ui-spec 章节更新；单测 470 + contracts 84/84 + 浏览器真跑 17/17 + 截图 a06-*×8）；✅ A-07（#289，a0fc7da，2026-09-03 PR #306——搜索页 keyword-only 受控 GlobalSearchInput + 退役删除 SearchAgentInput + 门控「问 AI 助手」入口 /agent?q= 预填与工作台 no_evidence CTA 成闭环；单测 479 + contracts 84/84 + 隔离栈 rig 12/12 + 截图 a07-*×7；道 2 前端清票道执行，同批顺带收 T35（#255，#307，acaeba0）+ 修 ChatWindow 坏 i18n key（#308，43eb31f）+ NLSearch 无消费者开清理票 #309）；SP-13 前端实现票 6/7 全完（余 A-04 #286）；下一前沿 = 第 8 段长尾 T 票（D 批已清，余 B/C/E 批 33 张；2026-09-04 派发道 A 产品 bug 批 #319→#318→#320/#321/#323 与道 B 闲时 B 批子链，提示词见 docs/working/2026-09-04-longtask-laneb-seg8b-prompt.md；道 A 前四票同日收口（PR #326/#328/#329/#330，main=7326ad9），#323 CI 闪断残余开票、道 B 未开工）；语料 v2（#291）注入完成（1600/1600，PR #316）golden set 复核中→冻结后关票→A-04（#286）待其收口 |
| 0 | `docs/working/2026-09-01-design-audit-fix-plan.md`（内容权威，§0 为 Phase 6 修订版；执行入口：GitHub SP-00~SP-11 = #209~#220 模块票 + T01~T55 = #221~#275 垂直切片票；**执行以 T 票为准**） | 55 tickets | 混合（触及迁移/安全的票按 heavy，如 T30 软删除） | 2026-09-01 全站设计审计闭环产物（96+21 条发现 → 44 项修复 → Phase 6 全量复核 → 55 张切片票）；硬前置仅 FIX-19（T03 #223）→ FIX-29①（T45 #265）。✅ 2026-09-02 纳入整合总序：T18/T19/T20/T21/T17 已分别编排进第 2/5/7 段，其余按第 3 段（快赢批次 A）+ 第 8 段（长尾 B→C→D→E）执行；总序唯一权威 = `docs/working/2026-09-02-integrated-execution-order.md` |
| 0 | `docs/working/2026-09-01-ip-detail-refactor-handoff.md`（设计交接 + 事实索引；执行追踪：GitHub #290，45 条 user stories） | 1 ticket（+T17 #237 收窄票同窗） | heavy（迁移 073+，共治提案新域） | ✅ **#290 已完成收口**（2026-09-02，commit 6e6d77e，squash codex/heavy/290；GitHub `CLOSED/COMPLETED`；全量门 EXIT=0 + 浏览器实测 15/15 + 两阶段审查闭环：首轮双 REJECTED→必须项全修复→增量复审 DONE_WITH_CONCERNS 且遗留 MINOR 已修）。落地内容：单页 query 驱动三模块 Hub（tab/type/sort/status/q/d）+ 共治提案独立域（迁移 073，min_votes=10/pass_threshold=0.6/deadline 7d 走 config，AI 审核异步 fail-open）+ IP 内搜索（tab/chips 计数随命中收缩）+ 讨论浮层 + 置顶权收归 admin + 旧子路由 301 + GLOSSARY/ui-spec 改写；原型已移除。记录偏差（MVP）：新提案通知走 Notify 逐关注者异步扇出而非 outbox 事件。同窗 T17（#237 收窄版：admin IP 状态变更缓存失效 + B-004）已于 2026-09-02 收口（18830a8，light+TDD，20/69），**第 5 段完结**（段边界 verify --full EXIT=0）；下一前沿 = 第 6 段 A-01（#283） |
| 1 | `docs/superpowers/specs/2026-08-07-omnicraft-web-experience-corrections-design.md`（执行追踪：GitHub issues #64/#65~#76；heavy 细化：`docs/superpowers/plans/2026-08-08-omnicraft-ip-visit-history.md`、`docs/superpowers/plans/2026-08-08-omnicraft-favorites-cutover.md`） | 12 tickets | 混合 | #65~#75 全部完成（#71/#69/#72 共享面并行合并、#73 heavy 独立迁移 `066_ip_visit_history.sql`、#74→#75 收藏状态以成员关系为唯一源并退役旧 favorites 运行时依赖，均经 `verify-project.sh --full` 与全量验证门）；仅剩 #76（heavy `067_drop_legacy_favorites.sql`），2026-08-13 起标 `deferred`：随生产部署延期，云端日志+可恢复备份人工门在恢复生产前不执行（本地也不产生 mock 证据）。改 config/routes/migrations 后跑 doc-validator `--fix` |
| 2 | `docs/archive/plans/2026-08-08-omnicraft-media-experience-design.md`（执行追踪：GitHub issues #80/#81~#90） | 10 tickets | 混合 | ✅ 全部完成并合并（#81~#90，PR #132/#133/#152~#156/#165）；计划已归档 `docs/archive/plans/`。媒体显示链、来源联动与相关块收口，无遗留执行单元 |
| 3 | `docs/archive/plans/2026-06-30-omnicraft-community-source-linkage.md`（执行追踪：GitHub issue #96） | 9 tasks | mixed | ✅ 全部完成并合并（#157~#164）；计划已归档 `docs/archive/plans/`。来源校验/契约/related-fanworks/picker/归因行收口，无遗留执行单元 |
| 3 | `docs/superpowers/specs/2026-08-08-omnicraft-content-safety-callback-fix-design.md`（执行追踪：GitHub issues #103/#104~#109；#104~#109 为 #103 原生子票） | 6 tickets | heavy | ✅ 全部完成并合并（#104~#109，merge 77b9570/e37de43/076a64f/2efdfd6/318f1ae/ef6b1de；#103 spec 随批次关闭）：回调契约改 form+checksum 且 checksum 为唯一入站认证（#106）、提交侧补 seed/dataId 且封面纳入图片审核（#107）、删 `green.callback_allowed_ips` 加 `green.seed`/`green.uid` 配置与 release gate（#104）、`068_add_ai_review_records_task_id.sql` 幂等+banned 终态守卫+同步单次落库（#105）、评论/讨论文本审核含 A4 分环境语义与 EditComment（#109）。全量验证门通过（24 包 + verify-project.sh --full 73 例 + doc-validator 幂等）。#110 已晋升为后续独立安全计划（优先级 6 归档扫描，父票 #110），不属本行；#111/#112/#113 见优先级 4 行 |
| 4 | grilling 2026-08-08 moderation 批次（执行追踪：GitHub issues #111/#112/#113；追踪：#114/#115） | 3 tickets | 混合 | ✅ 全部完成并合并（#111 heavy 头像 OSS 强制+ImageModeration+存量 avatar-audit 脚本、#112 light 私信 A4 文本审核、#113 light 反馈附件图片审核；merge 8ee85b4/eaf530f/771aaba；共享 seam `ReviewImageURL` 在 44943d3，前端兼容修复 1e6c25e 暴露 `oss_domain`）。全量验证门通过（26 包 + verify-project.sh --full 73 例 + doc-validator 幂等）。#134 冻结门仅剩 #76 |
| 5 | `docs/superpowers/plans/2026-08-11-omnicraft-reliable-async-observability-foundation-roadmap.md`（执行追踪：冻结门 #134；T00/T02/T03/T08 = #135/#137/#138/#143） | 4 tickets | heavy | 本地开发已解冻（2026-08-13：#134 仅作生产发布门且 deferred，#135~#150 本地执行不再被其阻塞）；#134 生产门原生等待 #76（deferred）。✅ #135（T00 宪法 3.1.0 修订）已合并（08d07ac）；✅ #137（T02 outbox/inbox + 四类内容事件同事务）已合并（b5c6793）；✅ #138（T03 独立 cmd/worker + relay + 幂等 + DLQ replay）已合并（467c25c，含 4 项故障演练证据）；✅ #143（T08 OTel 应用合同、测试与 full-infra 配置）已合并（2d3dbaf）。本地 closure 已完成；真实 Collector/Jaeger UI 全链路仍未验证，不勾选该证据。独立 `cmd/worker` 是唯一 Worker 模型；固定迁移 070；改 config/routes/migrations 后跑 doc-validator `--fix` |
| 6 | `docs/superpowers/specs/2026-08-11-omnicraft-archive-malware-scanning-design.md`（执行追踪：父票 #110；最小实现 #146~#150；Future rollout #151） | 5 tickets | heavy | 本地已解冻（2026-08-13：#134 仅作生产发布门且 deferred，本地不阻塞）；✅ #146（S01 schema/状态机/审计）已合并（d1e7378）；✅ #147（S02 zip 结构/配额 validator）已合并（ac21b53）；✅ #148（S03 ClamAV adapter/scan worker）已完成（见 progress.txt 2026-08-19）；✅ #149（S04 发布/下载门禁与 admin review）已完成（见 progress.txt 2026-08-19）；✅ #150（S05 本地证据合同与验证门）已完成（见 progress.txt 2026-08-19）；ClamAV 真实签名库/EICAR/生产回扫仍未验证，仅 full-infra/security profile；固定迁移 072。#151 存量回扫不属于当前最小实现，已标 `deferred` 并以 `not planned` 关闭；只有用户明确恢复生产存量范围后才重开并登记为执行单元 |
| 6 | 2026-08-16 双轴审查修复批次（执行追踪：父票 #193；子票 #194~#203） | 10 tickets | 混合 | ✅ 全部完成并合并（#194~#203，#193 已于 2026-08-16 关闭归档）：#194 （dev DB 迁移 4→69 + 分支清理）…（merge 7ffc07d/3bae1e7/efaa32a/81bb147/f37a5ee/6413870/ed1fae9/f8e074f；#202 由负责人裁决「允许」published→under_review 降级，security 语义按 #105 banned 终态守卫，现网行为不变）。全量验证门通过（29 包 go build/vet/test + verify-project.sh --full 73/73 + doc-validator --fix 幂等）。批次已归档，无遗留执行单元 |

跨计划可执行 DAG 以 GitHub 原生 `blocked_by` 为机器可读门，本文为人类摘要：`#65` →（`#66/#67/#70/#82` 与媒体 `#83` 按文件并行）→ `#68` 与媒体 `#81/#84/#85~#89` → source-linkage `#96`（已完成）→ 媒体 `#90`（已完成）→ collaboration-invites `#97`（已完成）→ Web `#71` → `#69` → `#72` → `#73` → `#74/#75/#76`。其中 #71/#69/#72 共享筛选、ContentDetail 与 i18n 文件，必须按该顺序串行；Web/safety 末端 #111/#112/#113 已全部关闭（2026-08-13），#76 已标 `deferred`（恢复生产部署时激活，本地不执行）。2026-08-13 起本地开发不再被 #134 冻结门阻塞：#135~#150 按各自 lane 内 DAG（见上表备注）在本地执行，仅生产发布仍受 #134/#76 门约束。发现本文与 GitHub 原生边不一致时先停止并修正二者，不自行选择较宽松的一方。

### 暂缓计划（不是当前任务来源）

2026-07-25 用户决定面试展示阶段只推进 Web 开发与部署，暂不实现或宣传桌面能力。以下计划保留原路径和未勾选状态，只有用户明确恢复桌面范围并将其重新登记到上方活计划注册表后才能执行：

| 计划文件 | 暂缓余项 | 恢复条件 |
|----------|----------|----------|
| `docs/superpowers/plans/2026-05-30-omnicraft-beta-release-validation.md` | R-02（5 steps） | D-02~D-05 完成，且具备真实 Ed25519、HTTPS API 与签名分发输入 |
| `docs/superpowers/plans/2026-05-30-omnicraft-beta-desktop-deploy-security.md` | D-02~D-05（28 steps） | 用户明确恢复桌面开发；使用 heavy 车道逐任务执行 |
| `docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md` | D-02~D-05、R-02 及跟踪项 | 上述两份计划恢复时一并恢复 |
| `docs/superpowers/plans/2026-07-17-omnicraft-production-readiness.md` | Ops-09（桌面范围） | 用户明确恢复桌面范围后从最新 main 开 `codex/ops/ops-09`；Ops-00~Ops-08 已全部完成（Ops-08 Step 5 于 2026-08-07 以真实 staging drill + issue #77 approved RPO/RTO 证据收口）；计划文件保留 Ops-09 段不归档 |

暂缓期间必须保持 `features.desktop_deploy_enabled=false` 和 `client.download_enabled=false`；不得勾选上述余项，不得将桌面能力写入 Web-only 发布声明。Production Readiness 的 Ops-09 同样暂缓，但 Ops-00~Ops-08 仍属 Web 发布范围。

历史 `task.json` 模式已废弃：账本 100+ 任务全部完成并归档，不再作为任务来源。

---

## 执行车道

- **light（默认）**：可在一条 feature 分支上批量完成多个相关小项；按逻辑分批 commit；编写/更新与改动直接相关的测试并通过项目验证门；自查替代正式两阶段审查。
- **heavy**：涉及安全、发布门、数据库迁移结构、支付、auth、生产配置或桌面制品时必须使用。规则：一任务一 worktree 一分支一 commit；TDD（先写失败测试确认预期失败，再最小实现）；合并前两阶段审查（规格符合性 → 代码质量）并处理 `DONE_WITH_CONCERNS`。
- 计划文件可用 `[light]` / `[heavy]` 标注单个 Task 覆盖默认值；真实凭证/外部输入缺失时**阻塞**（见下节），不得以模拟证据替代真实验证门。
- **合并 main 一律走 PR（2026-09-03 Phase 2 起）**：push 分支 → `gh pr create` → `gh pr merge --squash --auto`（CI `project-gate` 绿后自动合并）。main 已启用分支保护（required check `project-gate` + `enforce_admins`），直推 main 会被拒；分支基线早于 0d34757（2026-09-02）的旧分支开 PR 前须先 sync main，否则 PR 不触发 CI。

## MANDATORY: Agent 工作流程

### Step 1: 选择任务并读取来源

1. 查「活计划注册表」，确定当前计划和车道。
2. 完整阅读该计划文件中当前任务的段落（不要只读片段）；若计划注明设计输入（specs/ 下文档），先读设计输入。
3. 若文档间矛盾 → 查「文档权威源」一节，以权威文档为准，记录为 issue，不做自行发挥。

### Step 2: 初始化开发环境

```bash
cd backend && go mod tidy          # 首次或新依赖
cd frontend && npm install         # 首次或新依赖
docker compose up -d postgres redis
cd backend && go run cmd/server/main.go &
cd backend && go run cmd/worker/main.go &   # 独立 worker（ADR0005）：只启一个；不开则评论/点赞/PR/申诉等动态通知滞留 Redis Stream 不落库
cd frontend && npm run dev &
```

**DO NOT skip this step.** 确保相关服务运行后再实现功能。

通知积压检查（worker 消费后 lag 应回 0）：`redis-cli xinfo groups omnicraft:notification.create`。

### Step 3: 实现任务

- 参考 `architecture.md` 对应模块设计；严格遵循现有代码规范和目录结构。
- **业务规则**：实现涉及信誉、来源联动、上传、收藏、通知、搜索、判官、风控、支付、/studio、Agent、Tauri、安全、i18n、日志等子系统前，必须先阅读 `docs/reference/business-rules.md` 对应小节（`grep -A 40 "### 信誉分体系" docs/reference/business-rules.md`）。
- **前端任务补充**：
  - 实现任何页面或组件前，必须读取 `design/ui-spec.md` 对应章节（`ui_spec_ref` 字段 + 主动 `grep -A 50 "## Component: TagBadge" design/ui-spec.md`）。
  - 存在对应 `## Component:` / `## Page:` 就必须严格遵守，它是唯一视觉权威，优先级高于 steps 文字描述。
- **代码规范**：
  - Go：`gofmt`，错误必须处理，禁止裸 panic，统一错误返回 `{ "code": "ERROR_CODE", "message": "..." }`。
  - Next.js：TypeScript strict，function 组件，Tailwind CSS，禁止 any。
  - 数据库：GORM 参数化查询，禁止 SQL 字符串拼接。
- 若修改了 config.go / migrations / routes.go → 提交前运行 `cd tools/doc-validator && go run . --fix`。

### Step 4: 测试（强制要求）

**UI 相关任务**：必须使用浏览器（MCP Playwright）验证页面加载、核心交互，并截图保存至 `screenshots/`。

**后端 API 任务**：curl 或 Playwright 验证正常路径和错误路径。

**所有任务必须通过**：
- [ ] Go: `go test ./...` / `go build ./...` / `go vet ./...`
- [ ] Next.js: `npm run build` / `npm run lint`
- [ ] 功能在浏览器/接口测试中验证通过
- [ ] Tauri 相关：`npm run build`、`cargo test --manifest-path src-tauri/Cargo.toml`

项目级入口：`bash scripts/verify-project.sh`（`--full` 加 mocked Playwright contracts，`--release` 加完整 E2E，互斥；`--tauri` 可叠加；契约测试见 `scripts/verify-project.tests.sh`）。2026-08-03 起验证体系由 PowerShell 移植为 bash（macOS/Linux 原生执行，与 GitHub Actions ubuntu runner 一致）；历史 `.ps1` 版本已删除，不再使用。聚合入口不替代 UI 截图、真实外部服务 smoke 或发布证据。heavy 车道必须先有失败测试再实现。

### Step 5: 更新 progress.txt

```
## [YYYY-MM-DD] - Task [ID]: [任务标题]
### What was done:
### Testing:
### Notes:
```

- `progress.txt` 仅保留最近约 30 天条目；超过 64KB 或每月轮换一次，旧条目移入 `docs/archive/progress/`。
- 新建 .md 文件放 `docs/working/`，头部注明创建日期和预计失效日期；禁止在 docs/ 根目录新建 .md。

### Step 6: 勾选跟踪并提交

1. 只勾选当前计划文件中已实际完成并验证的步骤；**不得**勾选其他计划的 checkbox，不得修改历史账本。
2. 使用 `git add <精确文件列表>`（不要 `git add .`），light 车道按逻辑分批 commit，heavy 车道一任务一 commit。
3. 所有步骤和验证通过前，不得更新完成状态；不得删除任务或修改任务描述。

---

## ⚠️ 阻塞处理

> **本地开发模式例外（2026-08-13 起）**：短期无生产部署。第 1/2/4 条（凭证、密钥、发布输入）在本地开发期**不再阻塞**——走既有 fail-open/mock/测试替身路径（审核 A4 语义、fake scanner、`verify-project.sh --full` mocked contracts），仅当用户明确要求真实发布时恢复检查。第 3/5 条（迁移冲突、多代理契约冲突）在任何模式下都阻塞。

**以下情况必须停止任务，输出阻塞信息，等待人工介入**：

1. 缺少外部服务配置：阿里云 AccessKey、OSS Bucket、Redis 连接失败（本地开发期例外，见上）
2. 需要真实密钥：内容安全 API、OSS 直传凭证、真实 LLM Provider（本地开发期例外，见上）
3. 数据库迁移冲突：迁移文件执行失败
4. 发布输入缺失：SMTP、验证码、HTTPS 证书、Allowed Origins、正式域名、法律文本版本、Ed25519 密钥缺失（本地开发期例外，见上）
5. 多代理契约冲突：共享文件冲突、无法安全 rebase、任务语义不兼容

**阻塞时禁止**：提交 commit、勾选任何 checkbox、假装任务完成。
**阻塞时必须**：在 progress.txt 记录进度和阻塞原因，并输出：

```
🚫 任务阻塞 - 需要人工介入

**当前计划**: [注册表中的计划文件]
**当前任务**: [ID] - [标题]
**已完成的工作**: [...]
**阻塞原因**: [...]
**需要人工操作**: 1. ... 2. ...
**解除阻塞后**: 记录解除原因与时间，运行 [命令] 继续
```

---

## 项目结构

```
OmniCraft/
├── architecture.md          # 系统架构概述（必读；Schema/API/配置细节见 docs/reference/）
├── AGENTS.md                # 本文件（唯一工作流权威；CLAUDE.md 为指向本文件的指针）
├── progress.txt             # 进度日志（近 30 天；旧记录见 docs/archive/progress/）
├── docs/
│   ├── reference/           # 按需查阅：business-rules / schema / api / config / implementation-notes
│   ├── specs/               # 版本功能规格
│   ├── archive/             # 已归档（历史计划、进度归档、历史账本，不再作为开发依据）
│   ├── superpowers/
│   │   ├── specs/           # 已确认的设计输入
│   │   └── plans/           # 活计划（见注册表）与已完成的子系统计划
│   └── working/             # 临时工作文档（注明失效日期）
├── .specify/memory/constitution.md  # 项目宪法（设计原则和约束）
├── design/
│   ├── design-system.md     # 设计系统（唯一设计 token 权威）
│   └── ui-spec.md           # UI 规格书（唯一视觉权威，按节 grep 使用）
├── backend/                 # Go 后端（cmd/server + internal/{handler,service,repository,model,middleware,pkg} + migrations/ + config.yaml）
├── frontend/                # Next.js 前端（app/ + components/ + lib/ + messages/）
├── tauri-client/            # Tauri PC 客户端
└── k8s/                     # K8s 配置（P2 预留）
```

## 文件命名与存放规范

- 新建 .md 文件放在 docs/working/，格式 YYYY-MM-DD-<scope>-<type>.md
- 禁止在 docs/ 根目录创建新 .md 文件（docs/GLOSSARY.md 除外）
- 禁止创建与已有权威文档同领域的第二份文档
- 临时文档在头部注明预计失效日期（默认 +2 月）

## 常用命令

```bash
# 后端
cd backend && go run cmd/server/main.go   # 启动开发服务器
go build ./... && go vet ./... && go test ./...

# 数据库迁移
psql $DB_DSN -f migrations/001_users.sql

# 前端
cd frontend && npm run dev                 # http://localhost:3000
npm run build && npm run lint

# Tauri 客户端
cd tauri-client && npm run tauri dev       # 开发模式
npm run tauri build                        # 构建安装包

# Docker
docker compose up -d                       # 启动所有服务
docker compose logs -f backend
```

---

## 文档权威源（冲突时以此为准）

> 完整权威文档登记表和冲突解决规则以 `docs/superpowers/specs/2026-06-29-omnicraft-documentation-governance-design.md` §4 为准，以下为快速参考。

| 领域 | 唯一权威文档 |
|------|------------|
| 设计原则与不可妥协约束 | .specify/memory/constitution.md |
| Agent 工作流与任务来源 | AGENTS.md（本文件） |
| 系统架构概述 | architecture.md |
| 业务规则细节 | docs/reference/business-rules.md |
| Schema / API / 配置参考 | backend/migrations/*.sql（真源）；docs/reference/{schema,api,config}.md（同步快照） |
| 视觉设计 token | design/design-system.md |
| UI 组件和页面规格 | design/ui-spec.md |
| 运行时配置真实值 | backend/config.yaml |
| 功能设计输入 | docs/superpowers/specs/*.md |
| 执行计划与完成状态 | 本文件「活计划注册表」指向的计划文件 |
| 部署运维 | docs/deploy/single-server-beta-runbook.md |
| 术语定义 | docs/GLOSSARY.md |

### 冲突解决优先级（摘要）

1. 宪法（不可妥协约束）> 一切
2. 生产代码（config.yaml / migrations / routes.go）> 文档
3. architecture.md > design/ > specs/ > plans/
4. 同目录多份文档：日期最新优先

**遇到矛盾 → 查设计规格 §4 → 记录为 issue，不做自行发挥。**

---

## Key Rules

1. **Select task from registry first** — 会话开始先查活计划注册表确定计划和车道，不混用完成状态
2. **Test before marking complete** — 所有 steps 验证通过才标记完成
3. **Browser testing for UI** — 新建或大幅修改页面必须浏览器测试并截图
4. **Document in progress.txt** — 帮助后续 Agent 理解工作内容
5. **Stop if blocked** — 需要人工介入时输出阻塞信息并停止
6. **Read config, not hardcode** — 所有限制从 config.yaml 读取
7. **Clarify before coding** — 明确假设与疑问，不擅自解读需求
8. **Simplicity first** — 仅实现需求功能，拒绝过度设计
9. **Surgical changes only** — 精准修改必要代码，匹配现有风格
10. **Consult ui-spec for frontend** — 前端实现前必须读取 `design/ui-spec.md` 对应章节
11. **Design spec overrides prose** — 视觉细节以 ui-spec 为准，优先级高于 steps 文字
12. **No raw error exposure** — 禁止 `err.Error()` 直达客户端或前端 UI
13. **Protected routes require auth guard** — 受保护路由包裹在 `(protected)` layout group
14. **Soft delete preferred** — 删除以软删除为主，保留审计数据
15. **Structured logging** — 后端日志统一 `slog` JSON，禁止 `log.Printf` 残留
16. **i18n mandatory** — 新增 UI 字符串必须走 `next-intl`，禁止硬编码
17. **TDD for heavy lane** — heavy 车道必须先验证失败测试再实现
18. **Exact staging only** — 使用 `git add <精确文件列表>`，不混入他人改动

---

## Agent skills

### Issue tracker

Issues live in the repo's GitHub Issues, managed via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Standard five-role vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
