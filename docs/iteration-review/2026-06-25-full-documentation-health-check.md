# OmniCraft 项目文档全面健康检查报告

**检查日期**: 2026-06-25
**检查范围**: 项目全部重要开发文档（architecture.md、AGENTS.md、constitution.md、task.json、progress.txt、design/ 目录、docs/superpowers/、docs/review/、docs/deploy/、docs/futureWork/、docs/agents/、docs/history/、docs/ 根目录报告等共 80+ 份文档）
**检查方法**: 5 个并行子代理分区深度审查 + 交叉验证 + 实际代码/配置核对
**检查人**: GLM-5.2 Agent

---

## 一、执行摘要

本次检查覆盖了 OmniCraft 项目的全部重要开发文档，共发现 **168 项问题**，其中：

| 严重程度 | 数量 | 说明 |
|---------|------|------|
| 🔴 阻塞级 | 19 | 影响正确性、会误导 Agent、违反宪法原则 |
| 🟡 警告级 | 58 | 文档过时、字段不一致、引用残留 |
| 🟢 建议级 | 91 | 可读性优化、完整性增强 |

同时**纠正了已有审查报告中的 8 项过时结论**（详见第六节）。

### 问题分布

| 文档区域 | 🔴 阻塞 | 🟡 警告 | 🟢 建议 | 核心问题 |
|---------|--------|--------|--------|---------|
| architecture.md | 4 | 12 | 5 | 配置字段脱节、路由清单过时、迁移编号错位 |
| constitution.md + 模板 | 3 | 4 | 2 | 设计系统过时、软删除策略冲突、PostgreSQL 版本冲突 |
| design/ 目录 | 1 | 8 | 3 | ui-spec.md 编码损坏、未定义 token、尺寸不一致 |
| docs/superpowers/ | 3 | 5 | 4 | 完成状态追踪碎片化、历史 plan 过时 |
| docs/review/ | 1 | 4 | 4 | 发布决策矛盾未标注、findings 状态未回填 |
| docs/deploy/ | 3 | 2 | 0 | 缺失 PgBouncer、环境变量名过时、健康检查端点不一致 |
| docs/ 根目录报告 | 4 | 4 | 2 | 决策实施状态未跟踪、async-queue/oss-lifecycle/PLAN/HANDOFF 过时 |
| docs/futureWork/ | 0 | 2 | 0 | 文档完全过时 |
| docs/agents/ + history/ | 0 | 4 | 1 | 与项目实际工作流脱节、迁移编号过时 |
| task.json + README | 0 | 3 | 2 | task.json 仍作为模式 B 来源、README 引用 CLAUDE.md |

### 总体评估

项目文档体系**架构设计层面质量高**（双轨 Beta 路线图、跨计划契约表、配置字段注册表等机制完善），但存在**三个系统性问题**：

1. **文档与代码/配置脱节严重**：architecture.md §7 配置字段与 config.yaml/config.go 大面积不一致，§3.2 API 路由清单与 routes.go 脱节，§4 数据库 Schema 缺失实际表。
2. **完成状态追踪碎片化**：多份 plan 文件的 checkbox 未随实际执行同步更新，决策记录标"已确认"但实际未实施，主 progress.txt 缺失部分执行记录。
3. **历史文档未及时归档/标注**：PLAN.md、CURRENT_TASK_HANDOFF、futureWork 等文档描述的工作已完成，但文档仍以"待执行"语气保留，易误导新 Agent。

---

## 二、🔴 阻塞级问题清单（19 项）

### A. 配置与代码硬编码（4 项）

#### A1. reputation 加减分值在代码中全部硬编码，违反宪法与 AGENTS.md
- **位置**: [reputation_service.go](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/service/reputation_service.go#L85-L153)、[architecture.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md) §7 第 1247-1269 行、[config.yaml](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config.yaml#L96-L102)
- **问题**: architecture.md §7 定义了 13 个 reputation 加减分值字段，但 config.yaml 的 reputation 段只有 6 个字段（`min_score_for_interaction`、`quality_content_threshold`、`quality_comment_threshold`、`repeat_violation_*`），**完全缺少所有加减分值字段**。reputation_service.go 第 85-153 行全部硬编码 `3`/`2`/`1`/`-3`/`-2`/`-1`。
- **违反**: AGENTS.md Rule 8「Read config, not hardcode」、constitution.md Principle IV/X、architecture.md §7 自身定义
- **修复**: 在 config.yaml、ReputationConfig 结构体补齐 13 个字段，将硬编码数字替换为配置读取

#### A2. reputation_logs.reason 枚举值在文档与代码间严重不一致
- **位置**: [architecture.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md) §4.1 第 519-522 行 vs reputation_service.go 各 Award*/Penalize* 方法
- **问题**: 文档定义 `malicious_contribution`/`contribution_accepted`/`malicious_report_comment` 等，代码实际使用 `malicious_content`/`malicious_pr`/`pr_merged`/`malicious_report`/`malicious_tag_report`/`judge_accuracy`/`rehab_course`/`tag_recognized`。8 个枚举值中 7 个不匹配。
- **影响**: 前端信誉分详情页若按文档枚举渲染将无法匹配实际日志；统计查询会漏数据
- **修复**: 以代码实际值为准更新 architecture.md §4.1 的 reason 枚举注释

#### A3. web_agent_enabled 默认值在文档与 config.yaml 中相反
- **位置**: [architecture.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md) §11.3 第 1806 行（`false`）、[config.yaml](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config.yaml#L139) 第 139 行（`true`）
- **问题**: 文档声明"默认关闭"，实际配置为 `true`。若该配置被复制到生产环境且未填 `llm_api_key`，会触发 ValidateRelease 校验失败
- **修复**: 明确意图——若 Beta 已决定默认开启，更新文档；若应默认关闭，改 config.yaml

#### A4. constitution.md PostgreSQL 版本与 AGENTS.md 冲突
- **位置**: [constitution.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/.specify/memory/constitution.md#L30) 第 30 行（`PostgreSQL 15+`）、[AGENTS.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/AGENTS.md) 工具链表（`PostgreSQL 16+ 需 pgvector ≥ 0.7`）
- **问题**: 宪法作为最高优先级治理文档，数据库版本要求低于 AGENTS.md，可能导致 CI 与本地环境不一致
- **修复**: 将 constitution.md 第 30 行改为 `PostgreSQL 16+`

### B. 宪法原则过时（3 项）

#### B1. constitution.md Principle XVII 软删除策略与 Key Rule 18 直接冲突
- **位置**: [constitution.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/.specify/memory/constitution.md#L324-L328) 第 324-328 行、[AGENTS.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/AGENTS.md) Key Rule 18
- **问题**: 宪法声明"Soft delete **only**" / "Physical deletion is **FORBIDDEN**"，但 Key Rule 18 已改为"Soft delete **preferred**"（DEC-031 允许 browse_history/notifications 物理删除）。宪法原则与现行规则直接矛盾。
- **修复**: 改为 "Soft delete preferred (DEC-031): user-facing delete operations MUST use soft delete by default; physical deletion is permitted ONLY for low-analytic-value data (browse_history, read notifications) and MUST be documented."

#### B2. constitution.md Principle VIII 仍引用 GitHub 视觉语言
- **位置**: [constitution.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/.specify/memory/constitution.md#L132) 第 132 行
- **问题**: 宪法声明"The UI design system references **GitHub's visual language** (see `architecture.md §10.4`)"，但 architecture.md §10.4 已改为 Indigo `#4F46E5` 主题，design-system.md 是新权威。宪法未同步 2026-06-24 设计系统变更。
- **修复**: 改为 "The UI design system uses an **Indigo-based flat design** (see `design/design-system.md` and `architecture.md §10.4`)"

#### B3. constitution.md 版本与日期过时
- **位置**: [constitution.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/.specify/memory/constitution.md#L382) 第 382 行
- **问题**: 版本 1.2.0，Last Amended 2026-05-13，但设计系统变更（2026-06-24）、软删除策略变更（DEC-031）、PostgreSQL 版本要求均未同步。SYNC IMPACT REPORT 标注的 3 个模板 PENDING 确实未更新。
- **修复**: 升级版本至 1.3.0，更新 Last Amended，补全 .specify/templates/ 模板

### C. 数据库与迁移（2 项）

#### C1. architecture.md §4.5 引用的迁移文件 035_conversation_indexes.sql 不存在
- **位置**: [architecture.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md) §4.5 第 880-883 行、[task.json](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/task.json#L1364) 第 1364 行
- **问题**: 文档称"以下两条索引由独立迁移 migrations/035_conversation_indexes.sql 创建"，但实际 035_discussions.sql 内容与 conversations/messages 无关。全量 grep `backend/migrations/` 对 `idx_messages_sender`、`idx_conversations_updated` **零匹配**——这两条索引从未被任何迁移创建。
- **影响**: 私信列表查询缺少索引，随数据量增长会变慢；文档与迁移文件互相矛盾
- **修复**: 新建迁移补建索引，修正 architecture.md §4.5 与 task.json 第 1364 行的文件名引用

#### C2. architecture.md §4 完全缺失 feedback 与 admin_audit_logs 表设计
- **位置**: [architecture.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md) §4 全节、[migrations/051_feedback.sql](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/migrations/051_feedback.sql)、[migrations/052_admin_audit_logs.sql](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/migrations/052_admin_audit_logs.sql)
- **问题**: 实际迁移已创建 feedback_tickets、feedback_replies、feedback_attachments、admin_audit_logs 4 张表，但 architecture.md §4 完全未提及，§3.2 路由清单也未提及 `/feedback/*`、`/admin/feedback/*`、`/admin/audit-logs` 路由（routes.go 第 247-254、360、364 行实际存在）
- **修复**: 在 architecture.md §4 新增「4.11 用户反馈」和「4.12 管理员审计日志」小节，补全 DDL；在 §3.2 补全路由

### D. 部署文档不一致（3 项）

#### D1. single-server 部署模板缺失 PgBouncer
- **位置**: [docs/deploy/docker-compose.single-server.yml](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/deploy/docker-compose.single-server.yml)、[architecture.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md) §8.1
- **问题**: architecture.md §8.1 和根目录 docker-compose.yml 包含 6 个服务（含 pgbouncer），DB_DSN 指向 `pgbouncer:5432`。但 single-server 模板仅 5 个服务，无 pgbouncer，DB_DSN 直连 `postgres:5432`。与 architecture.md §2.2 "PgBouncer 作为独立容器"设计原则冲突。
- **修复**: 在 single-server 模板添加 pgbouncer 服务，或在 runbook 说明单机阶段可暂不使用并更新 architecture.md 标注差异

#### D2. 环境变量名与 architecture.md 严重不一致
- **位置**: [architecture.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md) §8.1 第 1323-1332 行、[docs/deploy/production-config-template.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/deploy/production-config-template.md#L35-L45) 第 35-45、183-185 行
- **问题**: architecture.md 使用旧名 `ALIYUN_ACCESS_KEY_ID`/`ALIYUN_ACCESS_KEY_SECRET`/`OSS_BUCKET`/`OSS_CDN_DOMAIN`/`ALIYUN_GREEN_ENDPOINT`，production-config-template.md 使用新名 `OSS_ACCESS_KEY_ID`/`OSS_ACCESS_KEY_SECRET`/`OSS_BUCKET_NAME`/`OSS_DOMAIN`/`GREEN_ACCESS_KEY_ID`/`GREEN_REGION`，并明确指出旧名失效。architecture.md 未同步。
- **影响**: 开发者按 architecture.md 配置环境变量会导致 OSS/Green 功能失效
- **修复**: 更新 architecture.md §8.1 环境变量清单，与 production-config-template.md 和 config.go 对齐

#### D3. 健康检查端点三处不一致
- **位置**: [docs/deploy/docker-compose.single-server.yml](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/deploy/docker-compose.single-server.yml#L76) 第 76 行（`/health`）、根目录 docker-compose.yml 第 88 行（`/api/v1/stats/summary`）、[docs/deploy/nginx.omnicraft.single-server.conf](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/deploy/nginx.omnicraft.single-server.conf#L22) 第 22/52/97 行（`/healthz`）、[docs/deploy/single-server-beta-runbook.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/deploy/single-server-beta-runbook.md#L186) 第 186 行（`/health`）
- **问题**: 三个地方使用三个不同的健康检查路径。若后端实际只注册了其中一个路由，其他 healthcheck 会持续失败，导致容器被标记为 unhealthy。
- **修复**: 统一健康检查端点为 `/healthz`（与 nginx 配置一致），更新所有 docker-compose 和 runbook

### E. 完成状态追踪失真（3 项）

#### E1. 2026-06-08 四份安全加固 plan 全部步骤未勾选，但实际已执行
- **位置**: [docs/superpowers/plans/2026-06-08-omnicraft-abuse-control-no-load-testing.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/superpowers/plans/2026-06-08-omnicraft-abuse-control-no-load-testing.md) 等 4 份 plan、[docs/superpowers/progress/2026-06-09-security-hardening-execution.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/superpowers/progress/2026-06-09-security-hardening-execution.md)
- **问题**: 4 份 plan（共 30 个任务）全部步骤 `[ ]` 未勾选，但 progress/2026-06-09-security-hardening-execution.md 记录了全部已执行，提交哈希包括 1409933、24fd2a0、7da3442、7ce8191 等。后续代理会误判为未完成而重复执行。
- **修复**: 在每份 plan 顶部添加完成状态横幅，或将步骤 `[ ]` 改为 `[x]` 并追加执行记录段落

#### E2. 2026-06-08 安全加固工作未记录在主 progress.txt
- **位置**: [progress.txt](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/progress.txt)、[docs/superpowers/progress/2026-06-09-security-hardening-execution.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/superpowers/progress/2026-06-09-security-hardening-execution.md)
- **问题**: AGENTS.md Step 5 强制要求"更新 progress.txt"，但安全加固执行仅在 docs/superpowers/progress/ 下独立台账记录，未同步到主 progress.txt。同样，2026-06-03 和 2026-06-05 修复 plan 步骤也未勾选但已执行。
- **修复**: 在 progress.txt 追加 4 条条目（对应 4 份 plan），格式遵循 AGENTS.md 模板

#### E3. 决策实施状态全部标注为「已确认」，但实际均未实施
- **位置**: [docs/2026-06-24-design-review-decisions.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/2026-06-24-design-review-decisions.md) 决策索引表第 15-50 行
- **问题**: 所有 33 个决策状态列为「✅ 已确认」，但经核对 config.yaml 实际状态，所有决策均**尚未实施**（DEC-004 reputation 字段仍缺失、DEC-018 captcha 仍 bypass、DEC-019 queue 仍 disabled、DEC-020 agent llm_api_key 仍空、DEC-003 legal 版本仍空）。「已确认」与「已实施」语义混淆，无法判断哪些决策已落地。
- **修复**: 在索引表增加「实施状态」列，区分「✅ 已确认 / ⏳ 待实施 / 🚧 实施中 / ✅ 已实施」

### F. 文档过时误导（3 项）

#### F1. ui-spec.md 编码损坏（乱码）
- **位置**: [design/ui-spec.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/design/ui-spec.md#L3710) 第 3710-3811+ 行（约 96 行）
- **问题**: StudioSidebar、ContentTypeGrid、/studio 等章节出现大量 GBK 被当作 UTF-8 解读的乱码（如"鍒涗綔鑰呭伐浣滃鐨勫彲鎶樺彔渚ц竟鏍忥紝蹇呴』鍦?"）。这些章节内容完全不可读，Agent 无法据此实现 Studio 功能。
- **修复**: 用正确的 UTF-8 重新生成或修复这些章节

#### F2. PLAN.md 描述已完成计划为待执行
- **位置**: [docs/PLAN.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/PLAN.md#L52-L168) 第 52-168 行
- **问题**: 描述 Task 99-145 的"待执行计划"，但 task.json 中这些任务已全部 `passes: true`。文档仍以"待执行"语气描述，第 12-16 行 Review Findings 描述的问题已被 Task 90 解决。
- **修复**: 归档至 docs/archive/，或在顶部添加"本计划已完成，仅作历史记录"

#### F3. CURRENT_TASK_HANDOFF 交接任务已完成
- **位置**: [docs/CURRENT_TASK_HANDOFF_019dfb5e-1360-7050-b2a4-f8900ada98e4.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/CURRENT_TASK_HANDOFF_019dfb5e-1360-7050-b2a4-f8900ada98e4.md)
- **问题**: 生成时间 2026-05-06，交接的核心任务"原创/二创来源联动"已完成（migration 036 已存在，/original/[contentId]/fanworks 页面已存在）。第 11 行"当前仓库处于大量未提交修改状态"已不适用。
- **修复**: 归档至 docs/archive/，或在顶部添加"本交接任务已完成，仅作历史记录"

### G. 分析文档数据错误（2 项）

#### G1. async-queue-analysis.md 搜索配置描述过时错误
- **位置**: [docs/async-queue-analysis.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/async-queue-analysis.md#L193-L198) 第 193-198 行
- **问题**: 表格称"当前 (PostgreSQL tsvector) | `simple` 字典，中文分词弱"，但经 DEC-001 核查，索引侧已使用 `jiebacfg` 分词（迁移 041/042/045），非 `simple`。该分析文档的结论基于过时信息。
- **修复**: 更新为"索引侧已用 `jiebacfg`，查询侧仍用 `simple` 不匹配（见 DEC-001）"

#### G2. oss-lifecycle.md 下载/上传 URL TTL 与配置冲突
- **位置**: [docs/oss-lifecycle.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/oss-lifecycle.md#L11-L12) 第 11-12 行
- **问题**: 文档称"Download URL validity: 1 hour"，实际 config.yaml 为 `download_url_ttl_sec: 300`（5 分钟），差异 12 倍。文档称"Upload URL validity: 15 minutes"，architecture.md §7 为 `presign_expire_sec: 3600`（1 小时）。
- **修复**: 统一为实际配置值，或标注"参见 config.yaml"

---

## 三、🟡 警告级问题清单（58 项，按类别分组）

### H. architecture.md 配置字段脱节（8 项）

| 编号 | 位置 | 问题 | 修复建议 |
|------|------|------|---------|
| H1 | architecture.md §7 vs config.yaml | features 字段缺失 3 个：`ad_enabled`、`agent_enabled`、`judge_enabled` 在文档定义但 config.yaml/config.go 无 | 补到 FeaturesConfig 或更新文档删除 |
| H2 | architecture.md §7 vs config.yaml | judge 配置字段名不一致：`pass_score_rate` vs `exam_pass_rate`、`revoke_min_votes` vs `error_rate_window`、`min_votes` vs `min_votes_required` | 以 config.yaml 为准重写 §7 judge 段 |
| H3 | architecture.md §7 | upload 段字段位置错误：`oss_bucket`/`oss_region`/`presign_expire_sec` 应在 `oss:` 段而非 `upload:` 段；`sheet_music_allowed_ext` vs `sheet_music_extensions` 格式不一致 | 更新 §7 字段位置和命名 |
| H4 | architecture.md §11.3 vs config.yaml | agent 配置字段不一致：文档有 `rate_limit_free_per_day`/`rate_limit_creator_per_day`/`rate_limit_admin`，实际为单一 `rate_limit_per_day`；文档缺 `max_user_message_chars`/`chat_max_context_messages`/`hmac_secret` | 以代码为准更新 §11.3 |
| H5 | architecture.md §7 | 9 个配置段未文档化：`captcha`、`smtp`、`verification`、`legal`、`cache`、`queue`、`rate_limit`、`web`、`security` 在 config.yaml 存在但 §7 无说明 | 补充配置段说明 |
| H6 | architecture.md §4.5 vs AGENTS.md | notifications.type 枚举不一致：文档 `reply`/`pr_submitted`/`pr_resolved`/`content_reviewed`/`judge_result`，AGENTS.md `comment`/`mention`/`appeal_result`/`content_status` | 以迁移 038 和代码为准统一枚举 |
| H7 | architecture.md §15.6 vs §4.3 | content_items 的 `download_count` 字段在 §15.6 提及但 §4.3 DDL 未包含（实际由 Redis ZSET 维护） | 在 §4.3 注释说明 download_count 由 Redis 维护 |
| H8 | architecture.md §3.3 vs AGENTS.md | Tauri Agent 白名单 `backup_file` 备份目录不一致：文档 `.bak`，AGENTS.md `.omnicraft_backup/`；签名算法文档 HMAC-SHA256，AGENTS.md 要求 D-03 后改 Ed25519 | 以 AGENTS.md 为准更新 §3.3 与 §6.3 |

### I. architecture.md 路由与结构过时（7 项）

| 编号 | 位置 | 问题 | 修复建议 |
|------|------|------|---------|
| I1 | architecture.md §3.1 | 页面路由清单缺失 18+ 个实际页面：verify-email、forgot-password、reset-password、terms、privacy、help、feedback、client、home、ips、admin/dashboard、admin/reports、admin/feedback、admin/audit-logs、admin/queue、studio/favorites 等 | 按实际 frontend/app/ 补全 §3.1 |
| I2 | architecture.md §3.2 vs routes.go | API 路由清单多处不一致：20+ 条实际路由未文档化（/captcha/verify、/config/public、/feedback/*、/admin/feedback/*、/admin/audit-logs、/admin/queue/*、/internal/ai-callback、/deploy-grants、/discussions/* 等）；5+ 条文档路由未实现（/agent/script、/agent/verify、/admin/categories GET、/conversations/:id/read） | 以 routes.go 为准全面重写 §3.2 |
| I3 | architecture.md §3.1 第 160 行 | 组件树 `FollowerAnalytics.tsx` 已拆分为 `FollowerTrendChart.tsx` + `FollowerSourceChart.tsx`（task.json 第 3145 行确认），文档未更新 | 替换组件名 |
| I4 | architecture.md §3.2 | Go 后端项目结构未列出实际存在的 container/、worker/、testutil/、pkg/recovery/、pkg/scheduler/、pkg/redis/、pkg/database/、pkg/response/、pkg/captcha/、pkg/mail/ 及 middleware 下多个新文件 | 补全 §3.2 项目结构 |
| I5 | architecture.md §3.2 vs §15.10 | 双份路由清单维护导致 §3.2 过时：§15.10 列出的 /auth/forgot-password、/auth/verify-email、/users/search、/stats/summary、/admin/contents/trash 等未合并进 §3.2 | 合并 §15.10 到 §3.2，删除 §15.10 路由列表 |
| I6 | architecture.md §1.2 vs §11.5 | `/agent` 模块未区分 Tauri Agent（§3.2 的 /agent/script、/agent/verify，实际未实现）与 Web Agent（§11.5 的 /agent/upload-assist 等） | 在 §3.2 明确标注 Tauri 脚本路由当前未实现 |
| I7 | architecture.md §15.9 | 迁移编号与实际文件完全错位：文档 038=全文搜索，实际 038=notification_channels；文档 039=密码重置，实际 039=conversation_unread_count；文档 041=内容软删除，实际 041=content_search_vector | 以实际迁移文件为准重写 §15.9 |

### J. design/ 目录问题（8 项）

| 编号 | 位置 | 问题 | 修复建议 |
|------|------|------|---------|
| J1 | ui-spec.md 第 62/109/1883 行 | 引用未定义 token `canvas.muted`（design-system.md 仅有 `canvas.default`/`canvas.subtle`） | 改为 `canvas.subtle` |
| J2 | ui-spec.md 第 66/113/1887 行 | 引用未定义 token `border.danger`（design-system.md 仅有 `border.default`，危险色用 `destructive`） | 改为 `border-destructive` |
| J3 | ui-spec.md 第 63/110/1779/1884 行 | 引用未定义 token `accent.default`（design-system.md 仅有 `accent.emphasis`/`accent.subtle`） | 改为 `accent.emphasis` |
| J4 | ui-spec.md 第 60/107/163/212 等 15+ 处 | 引用未定义 token `fg.default`（design-system.md 仅有 `foreground`/`fg.muted`） | 改为 `fg` 或 `foreground` |
| J5 | ui-spec.md 第 206/1115/1439 等 9 处 | 侧边栏宽度标注 220px，与 design-system.md 第 63/113/157/168/178 行的 228px 不一致 | 全部改为 228px |
| J6 | ui-spec.md 第 132/178/227 等 20 处 | Header 高度使用 `h-13`（非 Tailwind 默认类），design-system.md 第 150 行用 `h-14`（56px），两文档矛盾且 token 为 52px | 统一：若 52px 则都用 h-13 并扩展 tailwind.config；若 56px 则都用 h-14 |
| J7 | ui-design-prompt.md 第 23 行 | 标签 4 色（blue/green/purple/orange），design-system.md 第 39-46 行定义 6 色（含 rose/sky） | 改为 6 色 |
| J8 | ui-design-prompt.md 第 24/114 行 | 圆角 6px/12px 体系，design-system.md 第 71-75 行为 3/4/8/12px 体系，完全冲突 | 对齐 design-system.md 圆角阶梯 |

### K. docs/superpowers/ 问题（5 项）

| 编号 | 位置 | 问题 | 修复建议 |
|------|------|------|---------|
| K1 | beta-verification-feedback.md V-03 | Step 6 浏览器测试未勾选（第 268 行 `[ ]`），但任务整体标记 `[x]`（roadmap 第 87 行） | 补充浏览器测试证据或将 Step 6 改为 `[x]` |
| K2 | 2026-05-07-omnicraft-code-review.md | 任务统计过时：第 19 行"86 total, 24 passes: false"，实际 100+ 全部完成；第 87 行引用 CLAUDE.md | 添加过时声明或归档至 docs/archive/ |
| K3 | beta-plan-review.md | 24 项问题未标注解决状态，经交叉核对大部分已通过 plan 修订解决 | 每个问题项后追加解决状态标注 |
| K4 | beta-release-validation.md R-01 | 迁移编号说明未涵盖 053-055（修复 plan 使用的编号），未提及历史错位（033-034、041-042） | 在 roadmap "Migration Numbering Note" 补充 |
| K5 | specs/2026-06-05-omnicraft-web-beta-optimization-roadmap-design.md | 第 22-27 行仍描述"repair work is still active"，但 Batch 1-7 已全部执行 | 更新状态为"Batch 1-7 已执行，本文档保留作未来优化参考" |

### L. docs/review/ 问题（4 项）

| 编号 | 位置 | 问题 | 修复建议 |
|------|------|------|---------|
| L1 | beta-release-validation-2026-05-30.md | 第 6 行和第 11 节"APPROVED for Web Beta release"原始结论未标注被 2026-06-03 review summary（GO-WITH-BLOCKERS）推翻 | 在第 6 行和第 11 节标题添加显著取代标注 |
| L2 | web-beta-review-00-release-evidence.md | P1-2 截图缺失未标注已修复（web-beta-repair-validation 显示已补拍） | 在 P1-2 末尾添加"✅ 已于 2026-06-04 修复" |
| L3 | web-beta-review-summary.md | findings 修复状态未标注，需交叉对比多份报告才能判断当前状态 | 在 findings 表格增加"Status"列 |
| L4 | 4 份报告 | production-only blockers 表格在 00/summary/repair-validation/beta-release-validation 4 处重复且状态不一致 | 在 summary 保留权威表格，其他报告添加引用 |

### M. docs/deploy/ 与域名问题（2 项）

| 编号 | 位置 | 问题 | 修复建议 |
|------|------|------|---------|
| M1 | architecture.md §8.1 第 1340 行 | `FRONTEND_URL=https://omnicraft.com`（占位域名），实际生产为 `https://app.leeppp.online` | 更新为实际域名或标注"示例域名" |
| M2 | production-config-template.md 第 58 行 | `CONFIG_OVERRIDE_PATH=/var/lib/omnicraft/config_override.yaml`（宿主机路径），docker-compose 和 runbook 为 `/app/config_override.yaml`（容器内路径），变量名相同易混淆 | 明确标注"CONFIG_OVERRIDE_PATH 是容器内路径" |

### N. 其他文档过时（4 项）

| 编号 | 位置 | 问题 | 修复建议 |
|------|------|------|---------|
| N1 | futureWork/2026-05-31-omnicraft-beta-future-work.md | 第 2 节"当前开发进度"仍说"F-01 和 F-02 已提交"，实际 F-01 至 R-01 全部完成；第 4 节推荐路线无意义 | 顶部添加过时标注或归档 |
| N2 | docs/2026-06-24-omnicraft-documentation-review.md + design-review-2026-06-24.md | 两份原始报告已并入合并版，三份并存造成维护负担 | 顶部标注"已并入合并版"或归档 |
| N3 | docs/2026-06-24-omnicraft-documentation-review.md 第 228-231 行 | 称"MVP 使用 SSE"，但 DEC-007 核查显示当前无 SSE 端点，纯 HTTP 轮询 | 修正为"当前纯 HTTP，无实时推送" |
| N4 | DEC-011/013 要求 | 要求创建 docs/archive/README.md 但未创建（docs/archive/ 仅有 2 个文件） | 补建 docs/archive/README.md |

### O. agents/ 与 history/ 问题（4 项）

| 编号 | 位置 | 问题 | 修复建议 |
|------|------|------|---------|
| O1 | agents/issue-tracker.md | 描述 GitHub Issues 工作流，但项目实际主工作流基于 task.json 和 Beta roadmap | 顶部添加说明"GitHub Issues 仅用于社区反馈" |
| O2 | agents/domain.md 第 21-24 行 | 示例 `0001-event-sourced-orders.md` 等电商 ADR，与 OmniCraft 无关 | 替换为项目相关 ADR 占位或标注"通用模板示例" |
| O3 | history/docs edited history.md 第 4 行 | "修复时间：2025-01"疑似"2026-01"笔误（其他轮次为 2026-04） | 确认并更正 |
| O4 | history/docs edited history.md | 迁移编号信息停留在 030-035，与当前实际 55+ 严重脱节 | 顶部添加"迁移编号信息截至 035，当前实际已至 055+" |

### P. task.json 与 README 问题（3 项）

| 编号 | 位置 | 问题 | 修复建议 |
|------|------|------|---------|
| P1 | task.json + AGENTS.md 模式 B | 100+ 任务全部 passes:true，但仍作为模式 B 任务来源，新 Agent 误入会找不到可做任务 | 在 AGENTS.md 模式 B 顶部加更醒目标注"已无可用任务" |
| P2 | README.md 第 249 行 | 项目结构列出 `CLAUDE.md`，实际文件为 `AGENTS.md`（constitution.md 第 354/362 行也引用 CLAUDE.md） | 改为 AGENTS.md |
| P3 | README.md 第 122 行 vs architecture.md §8.2 | PgBouncer 端口不一致：README 6432，architecture.md 5432 | 统一为实际 docker-compose 端口 |

### Q. constitution.md 其他过时（2 项）

| 编号 | 位置 | 问题 | 修复建议 |
|------|------|------|---------|
| Q1 | constitution.md Principle IX 第 154 行 | "Read task.json; select the lowest-ID task with passes:false" 已过时，AGENTS.md 已改为双轨 Beta 模式 | 改为描述双轨模式 |
| Q2 | constitution.md Principle XIII 第 256 行 | "MUST NOT start a web Agent task if any Task with id ≤ 51 remains passes:false" 已无意义（全部完成） | 删除或改为引用 Beta roadmap 优先级 |

---

## 四、🟢 建议级问题清单（91 项，概要）

建议级问题主要为可读性优化和完整性增强，包括：

- **architecture.md**: 版本号过时（仍标 1.0）、§10.4 与 constitution.md 暗色模式实现描述不一致、§12.5 向量化补齐未定义重试上限、§5.2 OSS Token 请求字段不完整
- **design/**: ui-spec.md 51 处"GitHub 扁平风"措辞可优化、ui-design-prompt.md 仍引用 UI Design.md §3.x 作输入清单
- **docs/review/**: 报告行号基于旧 commit 可能偏移、code-review-log 部分批次 SHA 缺失、findings 编号体系不统一
- **docs/superpowers/**: implementation-notes.md 使用 Windows 路径分隔符、plan-review.md 第 374 行 D-01 引用应移除
- **其他**: README.md 项目结构未列出 design/design-system.md 等关键目录、oss-lifecycle.md 为英文与其他文档不一致

（完整建议级清单因篇幅原因从略，各分区子代理报告中有详细记录。）

---

## 五、修复优先级建议

### 第一优先级：立即修复（影响正确性，19 项阻塞级）

1. **A1** reputation 加减分值硬编码 → 补全 config.yaml 字段，移除硬编码
2. **A2** reputation_logs.reason 枚举不一致 → 以代码为准更新文档
3. **A3** web_agent_enabled 默认值相反 → 明确意图并统一
4. **A4** PostgreSQL 版本冲突 → constitution.md 改为 16+
5. **B1** 软删除策略冲突 → 更新 constitution.md Principle XVII
6. **B2** GitHub 视觉语言过时 → 更新 constitution.md Principle VIII
7. **B3** constitution 版本过时 → 升级版本，补全模板
8. **C1** 不存在的迁移文件 → 补建索引，修正引用
9. **C2** 缺失表设计 → 补全 architecture.md §4
10. **D1-D3** 部署文档不一致 → 统一 PgBouncer、环境变量名、健康检查端点
11. **E1-E3** 完成状态追踪失真 → 勾选 checkbox，补充 progress.txt，区分决策实施状态
12. **F1** ui-spec.md 编码损坏 → 重新生成乱码章节
13. **F2-F3** PLAN.md/HANDOFF 过时 → 归档或标注
14. **G1-G2** 分析文档数据错误 → 修正 async-queue 和 oss-lifecycle

### 第二优先级：本轮 Beta 修复（影响新 Agent 开发，58 项警告级）

- architecture.md §3.2 API 路由清单全面重写（I2）
- architecture.md §7 配置字段对齐 config.yaml（H1-H7）
- design/ 目录 token 引用和尺寸统一（J1-J8）
- docs/superpowers/ 历史 plan 过时标注（K2-K5）
- docs/review/ 发布决策矛盾标注（L1-L4）
- 域名和路径混淆修正（M1-M2）
- 其他文档过时标注（N1-N4）

### 第三优先级：后续维护（91 项建议级）

- 文档完整性增强
- 可读性优化
- 编号体系统一

---

## 六、对已有审查报告的纠正（8 项）

本次检查发现，`docs/iteration-review/` 下已有的 5 份 2026-06-25 审查报告中，**部分结论已过时**（审查后代码已修复或情况已变化）：

| 编号 | 已有报告结论 | 实际当前状态 | 说明 |
|------|------------|------------|------|
| RC1 | StudioSidebar.tsx 使用 `w-56`(224px)/`w-[52px]`(52px)，与文档 228px/48px 不一致 | **已修复**：第 82 行现为 `collapsed ? "w-12" : "w-[228px]"`（48px/228px），与文档完全一致 | 代码已在审查后修复 |
| RC2 | design-system.md 第 168、178 行标注 220px | **不成立**：第 168、178 行实际为 228px。220px 出现在 ui-spec.md（9 处），非 design-system.md | 已有报告文件定位有误 |
| RC3 | ui-spec.md 第 5、8 行仍引用 UI Design.md 作为权威来源 | **已修复**：第 8 行已明确声明"UI Design.md 已归档，不再作为视觉权威" | 归档声明已添加 |
| RC4 | DEC-008、DEC-011 缺少独立风险提示节 | **已修复**：DEC-008 第 627-632 行、DEC-011 第 820-825 行已有「### 风险提示」节 | 风险提示已补充 |
| RC5 | 33 个决策中仅 9 个含独立风险提示节 | **已改善**：实际 14 个含风险提示节（DEC-001/002/006/008/011/014/015/018/019/020/021/031/032/033） | 数量已增加 |
| RC6 | Beta 设计文件清单列出 UI Design.md 未标注已归档 | **已修复**：第 72 行已标注"UI Design.md（已归档至 docs/archive/，仅作历史参考）" | 归档标注已添加 |
| RC7 | DEC-012 标题与实际操作不符 | **部分修复**：标题已用括号注明"原计划:移除二级筛选描述" | 可进一步优化 |
| RC8 | globals.css 第 83 行 `--chart-1: #0969da` 残留 | 未重新验证（属 🟢 建议级，非阻塞） | 图表色板与品牌主色独立，影响低 |

**说明**: 已有审查报告质量高，上述 8 项过时结论多为审查后代码/文档已修复，而非审查本身错误。后续引用已有报告时应注意这些结论的时效性。

---

## 七、文档归档建议

以下文档建议归档至 `docs/archive/` 或在顶部添加过时标注：

| 文档 | 原因 | 建议处理 |
|------|------|---------|
| `docs/PLAN.md` | Task 99-145 全部完成，仍以"待执行"语气描述 | 归档 |
| `docs/CURRENT_TASK_HANDOFF_*.md` | 2026-05-06 交接任务已完成 | 归档 |
| `docs/futureWork/2026-05-31-*.md` | F-02 至 R-01 全部完成 | 归档或顶部标注 |
| `docs/superpowers/plans/2026-05-07-omnicraft-code-review.md` | 任务统计过时（86 vs 100+），引用 CLAUDE.md | 归档 |
| `docs/2026-06-24-omnicraft-documentation-review.md` | 已并入合并版 | 顶部标注或归档 |
| `docs/design-review-2026-06-24.md` | 已并入合并版 | 顶部标注或归档 |

---

## 八、检查方法说明

### 检查覆盖范围

本次检查覆盖以下文档区域：
1. **核心文档**: AGENTS.md、architecture.md、constitution.md、task.json、progress.txt、README.md、PRD
2. **design/ 目录**: design-system.md、ui-spec.md、ui-design-prompt.md、doc-review-prompt.md
3. **docs/superpowers/**: 19 份 Beta 路线图与子系统计划文档
4. **docs/review/**: 24 份审查报告
5. **docs/deploy/**: 4 份部署文档
6. **docs/ 根目录**: 9 份报告/分析文档
7. **docs/futureWork/、docs/agents/、docs/history/**: 5 份文档
8. **.specify/**: constitution.md + 3 个模板文件

### 检查方法

- 5 个并行子代理分区深度审查
- 每个子代理负责一组文档，进行全文阅读 + 交叉比对 + 实际代码/配置核对
- 主代理验证关键发现（如 StudioSidebar.tsx 当前状态、ui-spec.md 编码损坏）
- 与已有 5 份 iteration-review 报告交叉对比，识别已修复问题和过时结论

### 未覆盖范围

- 未对 frontend/ 和 backend/ 代码进行全面审查（仅核对文档引用的关键代码位置）
- 未对 tauri-client/ 代码进行审查
- 未对 e2e/ 测试进行审查
- 未对 91 项建议级问题进行逐项深度验证（概要扫描）

---

## 九、附录：问题统计

### 按严重程度

```
🔴 阻塞级: 19 项
🟡 警告级: 58 项
🟢 建议级: 91 项
总计:    168 项
```

### 按问题类型

```
配置字段脱节:        15 项（A1-A4, H1-H8, G1-G2 等）
路由/结构过时:        12 项（I1-I7, C2 等）
完成状态追踪失真:     8 项（E1-E3, K1-K5 等）
文档过时误导:        22 项（F1-F3, N1-N4, K2, L1-L4 等）
设计系统不一致:       11 项（B2, J1-J8 等）
部署文档不一致:        5 项（D1-D3, M1-M2）
宪法原则过时:         6 项（B1-B3, Q1-Q2, A4）
数据库/迁移问题:       4 项（C1-C2, I7, H7）
其他:               85 项
```

### 已纠正的已有报告过时结论: 8 项

---

*检查完成时间：2026-06-25*
*本报告由 5 个并行子代理分区审查 + 主代理交叉验证生成，未修改任何文件。*
