# 第一轮修复验证报告

**验证日期**: 2026-06-28
**验证对象**: 基于 `2026-06-25-full-documentation-health-check.md` 报告的第一轮修复
**验证基准**: 19 项阻塞级问题 + 4 个决策点（A3/D1/J6/F2F3）
**验证方法**: 逐项核对源代码、配置文件、文档实际内容
**验证人**: GLM-5.2 Agent

---

## 一、执行摘要

本轮修复**整体有效但覆盖不全**：19 项阻塞级问题中**仅 7 项完全修复、2 项部分修复、10 项未修复**。4 个决策点中 A3 完全按建议落地，J6 部分落地，D1/F2F3 未落地。

| 修复状态 | 数量 | 占比 | 详情 |
|---------|------|------|------|
| ✅ 完全修复 | 7 | 37% | A1、A2、A3、A4、B1、B2、B3（主体） |
| ⚠️ 部分修复 | 2 | 11% | B3（模板未同步）、D3（端点未统一） |
| ❌ 未修复 | 10 | 53% | C1、C2、D1、D2、E1、E2、E3、F1、F2、F3、G1、G2 |

**修复亮点**：A1（reputation 硬编码）采用了"配置优先 + 硬编码回退"的优雅设计，既满足可配置化要求，又保留了安全默认值；A3 完全采纳了字段路径纠正建议。

**主要缺口**：C/D 系列（数据库与部署）完全未动；E 系列（完成状态追踪）完全未动；F 系列（文档过时）完全未动；G 系列（分析文档错误）完全未动。

---

## 二、4 个决策点修复情况

### A3: web_agent_enabled 默认值 → ✅ **完全按建议修复**

| 检查项 | 修复前 | 修复后 | 状态 |
|--------|--------|--------|------|
| [config.yaml:152](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config.yaml#L152) | `web_agent_enabled: true` | `web_agent_enabled: false` | ✅ |
| [constitution.md:235](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/.specify/memory/constitution.md#L235) | `features.web_agent_enabled: true`（字段路径错误） | `agent.web_agent_enabled: true`（字段路径已纠正） | ✅ |

**评价**：完全采纳建议，配置默认值改为 `false` 符合"安全默认"原则，constitution 字段路径与 config.yaml 对齐。生产环境校验失败风险已消除。

---

### D1: 单机部署 PgBouncer → ❌ **未按建议修复**

| 检查项 | 期望 | 实际 | 状态 |
|--------|------|------|------|
| [single-server.yml](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/deploy/docker-compose.single-server.yml) services | 6 个（含 pgbouncer） | 仍为 5 个（postgres/redis/backend/frontend/nginx） | ❌ |
| backend `DB_DSN` | `host=pgbouncer port=5432` | 仍为 `host=postgres port=5432`（第 63 行） | ❌ |
| 健康检查端点（D3 关联） | 统一 `/healthz` | 仍为 `/health`（第 76 行），与 nginx `/healthz` 不一致 | ❌ |

**评价**：未采纳"补上 PgBouncer"建议，仍维持"保留现状 + 标注差异"的原计划。但 single-server.yml 本身也未添加任何差异标注注释，与 architecture.md §2.1「P0 即需 PgBouncer」的冲突依然存在。

---

### J6: Header 高度统一 52px → ⚠️ **部分修复**

| 检查项 | 期望 | 实际 | 状态 |
|--------|------|------|------|
| [Header.tsx:86](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/components/layout/Header.tsx#L86) | 用 `var(--header-h)` 或保持 `h-[52px]` | 仍为 `h-[52px]` | ✅（保持） |
| [design-system.md:64](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/design/design-system.md#L64) | `--header-h: 52px`（token 权威） | 仍为 `52px` | ✅（保持） |
| [design-system.md:150](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/design/design-system.md#L150) | 改为 `Header (sticky, h-[var(--header-h)] / 52px)` | **仍为 `Header (sticky, h-14)`**（56px，与 token 矛盾） | ❌ |

**评价**：代码侧已用 52px（无需改动），但 design-system.md 第 150 行的内部矛盾（同文档第 64 行定义 52px，第 150 行却写 h-14=56px）**未修复**。文档内部矛盾依然存在，新 Agent 仍可能被误导。

---

### F2/F3: 旧文档处理 → ❌ **未按建议修复**

| 检查项 | 期望 | 实际 | 状态 |
|--------|------|------|------|
| F2: [PLAN.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/PLAN.md) 顶部横幅 | 添加"本计划已完成，仅作历史记录" | 前 30 行无任何完成标注 | ❌ |
| F3: [CURRENT_TASK_HANDOFF](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/CURRENT_TASK_HANDOFF_019dfb5e-1360-7050-b2a4-f8900ada98e4.md) 归档 | 移至 `docs/archive/` | 仍在 `docs/` 原位 | ❌ |
| F3: HANDOFF 顶部横幅 | 添加"本交接任务已完成" | 前 15 行无任何完成标注 | ❌ |

**评价**：F2/F3 完全未动，既未按原计划"原位标注"，也未按建议"分类处理（F2 原位标注 + F3 归档）"。两份过时文档仍以"待执行/当前任务"姿态停留在 `docs/` 根目录显眼位置。

---

## 三、19 项阻塞级问题逐项验证

### A. 配置与代码硬编码（4 项）

#### A1. reputation 加减分值硬编码 → ✅ **已修复**

- [config.go](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config/config.go) `ReputationConfig` 结构体第 117-129 行**新增 13 个 `score_*` 字段**（ScoreQualityContent/ScorePRMerged/ScoreQualityComment/ScoreTagRecognized/ScoreJudgeAccuracy/ScoreRehabCourse/ScoreValidReport/ScoreMaliciousContent/ScoreMaliciousPR/ScoreMaliciousComment/ScoreMaliciousReport/ScoreMaliciousTagReport/ScoreJudgeError）。
- [config.yaml:103-115](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config.yaml#L103-L115) 同步补齐 13 个字段值。
- [reputation_service.go](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/service/reputation_service.go) 第 76-84 行新增 `score()` 辅助方法，所有 Award*/Penalize* 方法改为 `s.score(回退值, func() int { return s.cfg.Reputation.ScoreXxx })`，**配置优先 + 硬编码回退**。

**设计评价**：优秀。0 值回退到默认值的设计兼顾了向后兼容（旧 config.yaml 不含新字段时仍能工作）与可配置化。

#### A2. reputation_logs.reason 枚举不一致 → ✅ **已修复**

- [architecture.md:519-522](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md#L519-L522) 注释已更新为完整 13 个枚举值：`quality_content | pr_merged | quality_comment | tag_recognized | judge_accuracy | rehab_course | valid_report | malicious_content | malicious_pr | malicious_comment | malicious_report | malicious_tag_report | judge_error`。
- 与 `reputation_service.go` 各方法传入的 reason 字符串逐一比对一致。

#### A3. web_agent_enabled 默认值 → ✅ **已修复**（详见第二节）

#### A4. constitution PostgreSQL 版本冲突 → ✅ **已修复**

- [constitution.md:30](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/.specify/memory/constitution.md#L30) 已改为 `PostgreSQL 16+ via PgBouncer connection pool`，与 AGENTS.md 工具链表一致。

---

### B. 宪法原则过时（3 项）

#### B1. 软删除策略冲突 → ✅ **已修复**

- [constitution.md:325-331](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/.specify/memory/constitution.md#L325-L331) 已改为 `Soft delete preferred (DEC-031)`，明确允许 browse_history/已读通知物理删除，与 AGENTS.md Key Rule 18 对齐。

#### B2. GitHub 视觉语言引用 → ✅ **已修复**

- [constitution.md:132-133](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/.specify/memory/constitution.md#L132-L133) 已改为 `Indigo-based flat design`，指向 `design/design-system.md`。

#### B3. constitution 版本与日期过时 → ⚠️ **部分修复**

- ✅ [constitution.md:385](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/.specify/memory/constitution.md#L385) 版本升至 `1.3.0`，Last Amended 更新为 `2026-06-28`。
- ✅ 第 387-392 行新增 1.3.0 Changelog，含 4 条变更记录（Principle VIII/XVII/Toolchain/XIII）。
- ❌ **`.specify/templates/` 模板未同步**：grep 在 `.specify/templates/` 目录下对 `Indigo|DEC-031|PostgreSQL 16|soft delete preferred` 四个关键词**零匹配**。

**违规说明**：constitution.md 第 380-381 行明确规定"after any MINOR or MAJOR amendment, `.specify/templates/plan-template.md`, `spec-template.md`, and `tasks-template.md` MUST be updated"。1.3.0 是 MINOR 级修订，按 constitution 自身规则必须同步模板，但实际未执行。

---

### C. 数据库与迁移（2 项）

#### C1. 缺失 035_conversation_indexes.sql → ❌ **未修复**

- `backend/migrations/` 目录下仅存在 `035_discussions.sql`，无 `035_conversation_indexes.sql`。
- grep `idx_messages_sender|idx_conversations_updated` 在所有迁移文件中**零匹配**——索引从未被创建。
- [architecture.md:880-883](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md#L880-L883) 仍保留错误引用"以下两条索引由独立迁移 migrations/035_conversation_indexes.sql 创建"。

#### C2. 缺失 feedback/admin_audit_logs 表设计 → ❌ **未修复**

- grep `feedback_tickets|admin_audit_logs|feedback_replies|feedback_attachments` 在 architecture.md 中**零匹配**。
- grep `/feedback|/admin/feedback|/admin/audit-logs` 在 architecture.md 中**零匹配**。
- §4 完全缺失 4 张表设计，§3.2 路由清单完全缺失相关路由。

---

### D. 部署文档不一致（3 项）

#### D1. single-server 缺 PgBouncer → ❌ **未修复**（详见第二节）

#### D2. 环境变量名不一致 → ❌ **未修复**

- [architecture.md §8.1:1324-1332](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md#L1324-L1332) 仍使用旧名：`ALIYUN_ACCESS_KEY_ID`/`ALIYUN_ACCESS_KEY_SECRET`/`OSS_BUCKET`/`ALIYUN_GREEN_ENDPOINT`/`ALIYUN_GREEN_CALLBACK_URL`。
- `docs/deploy/production-config-template.md:36-43,183-185` 已使用新名：`OSS_ACCESS_KEY_ID`/`OSS_ACCESS_KEY_SECRET`/`OSS_BUCKET_NAME`/`GREEN_ACCESS_KEY_ID`/`GREEN_REGION`，并明确指出旧名失效。
- **冲突结论**：架构文档与部署模板双向不一致，开发者按 architecture.md 配置环境变量会导致 OSS/Green 功能失效。

#### D3. 健康检查端点不一致 → ⚠️ **部分修复（实际仍混乱）**

实际端点分裂为四套：

| 来源 | 端点 | 状态 |
|------|------|------|
| 后端注册（[main.go:73](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/cmd/server/main.go#L73)） | `/health` | 唯一注册端点 |
| nginx 暴露（[nginx.omnicraft.single-server.conf:22,52,97](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/deploy/nginx.omnicraft.single-server.conf#L22)） | `/healthz` | 与后端不一致 |
| single-server backend healthcheck（[single-server.yml:76](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/deploy/docker-compose.single-server.yml#L76)） | `/health` | 与 nginx 不一致 |
| 根 docker-compose backend healthcheck（[docker-compose.yml:88](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docker-compose.yml#L88)） | `/api/v1/stats/summary` | 用业务接口做健康检查（反模式） |
| 运维文档（runbook:186、production-config-template:174） | `/health` | 与 nginx 不一致，外部 curl 会 404 |

**结论**：未统一为 `/healthz`，反而暴露出更多分裂（根 compose 用业务接口做健康检查是新增的反模式）。

---

### E. 完成状态追踪失真（3 项）

#### E1. 2026-06-08 安全加固 plan 未勾选 → ❌ **未修复**

4 个 plan 文件勾选状态：

| 文件 | `- [ ]` 未勾选 | `- [x]` 已勾选 | 顶部横幅 |
|------|---------------|---------------|---------|
| 2026-06-08-omnicraft-abuse-control-no-load-testing.md | 40 | 0 | 无 |
| 2026-06-08-omnicraft-dependency-vulnerability-upgrades.md | 20 | 0 | 无 |
| 2026-06-08-omnicraft-release-gates-config-hardening.md | 18 | 0 | 无 |
| 2026-06-08-omnicraft-oss-upload-download-hardening.md | 37 | 0 | 无 |
| **合计** | **115** | **0** | — |

**结论**：115 个任务项全部未勾选，无完成横幅。后续代理仍会误判为未完成而重复执行。

#### E2. 主 progress.txt 缺失安全加固记录 → ❌ **未修复**

- grep `security-hardening|abuse-control|content-moderation|ip-protection` 在 progress.txt 中**零命中**。
- 2026-06-08 仅有 3 条无关记录（CAPTCHA 票据、CAPTCHA 重置、UI 修复），安全加固系列工作完全未记录。

#### E3. 决策索引表实施状态 → ❌ **未修复**

- [2026-06-24-design-review-decisions.md:15](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/2026-06-24-design-review-decisions.md#L15) 表头列名为"状态"，**不是**"实施状态"。
- 取值仅有 `✅ 已确认`/`⏸️ 搁置`/`⏸️ 暂缓`/`✅ 已关闭`，**无** `⏳ 待实施`/`🚧 实施中`/`✅ 已实施`。
- 33 个决策仍全部标"已确认"，无法区分"已确认但未实施"与"已实施"。

---

### F. 文档过时误导（3 项）

#### F1. ui-spec.md 编码损坏 → ❌ **未修复**

- [design/ui-spec.md:3710-3811](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/design/ui-spec.md#L3710) 仍为 GBK 乱码，第 3710 行原文：`- 鍒涗綔鑰呭伐浣滃鐨勫彲鎶樺彔渚ц竟鏍忥紝蹇呴』鍦?`/studio/*` 甯冨眬涓娇鐢ㄣ€?`
- StudioSidebar/ContentTypeGrid//studio 等章节完全不可读。

#### F2. PLAN.md 未标注已完成 → ❌ **未修复**（详见第二节）

#### F3. CURRENT_TASK_HANDOFF 未归档/标注 → ❌ **未修复**（详见第二节）

---

### G. 分析文档数据错误（2 项）

#### G1. async-queue-analysis.md 搜索配置描述过时 → ❌ **未修复**

- [async-queue-analysis.md:195](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/async-queue-analysis.md#L195) 仍为 `` `simple` 字典，中文分词弱 ``，未更新为"索引侧已用 jiebacfg，查询侧仍用 simple 不匹配（见 DEC-001）"。
- 与 DEC-001 实际决策矛盾。

#### G2. oss-lifecycle.md TTL 冲突 → ❌ **未修复**

- [oss-lifecycle.md:10](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/oss-lifecycle.md#L10) 仍为 `Download URL validity: 1 hour`（应 5 minutes / 300s 与 config.yaml 一致）。
- [oss-lifecycle.md:11](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/oss-lifecycle.md#L11) 仍为 `Upload URL validity: 15 minutes`（应 1 hour / 3600s 与 architecture.md 一致）。
- 两个 TTL 值与 config.yaml/architecture.md 差异 12 倍 / 4 倍，生产安全配置风险未消除。

---

## 四、修复质量评估

### 4.1 已修复项的质量评价

| 修复项 | 修复质量 | 评价 |
|--------|---------|------|
| A1 | ⭐⭐⭐⭐⭐ | "配置优先 + 硬编码回退"设计优秀，向后兼容 |
| A2 | ⭐⭐⭐⭐⭐ | 13 个枚举值逐一比对一致，无遗漏 |
| A3 | ⭐⭐⭐⭐⭐ | 完全采纳建议，含字段路径纠正 |
| A4 | ⭐⭐⭐⭐⭐ | 一行修改，正确 |
| B1 | ⭐⭐⭐⭐⭐ | 与 DEC-031 对齐，表述清晰 |
| B2 | ⭐⭐⭐⭐⭐ | 与 design-system.md 对齐 |
| B3（主体） | ⭐⭐⭐⭐ | 版本+changelog 完整，但模板未同步 |

### 4.2 未修复项的优先级建议

| 优先级 | 问题 | 理由 |
|--------|------|------|
| 🔴 P0 立即修复 | F1（ui-spec 乱码） | 直接影响前端实现权威文档可读性，Agent 无法据此实现 |
| 🔴 P0 立即修复 | G2（TTL 冲突） | 直接影响生产安全配置，差异 12 倍/4 倍 |
| 🔴 P0 立即修复 | D2（环境变量名） | 开发者按架构文档配置会导致 OSS/Green 功能失效 |
| 🟡 P1 本轮补齐 | B3（模板同步） | 违反 constitution 自身规则，应一并补齐 |
| 🟡 P1 本轮补齐 | J6（design-system 内部矛盾） | 一行修改即可消除文档内部矛盾 |
| 🟡 P1 本轮补齐 | F2/F3（过时文档） | 一行横幅或一次 mv 即可解决 |
| 🟡 P1 本轮补齐 | D3（健康端点统一） | 决定权威端点后批量替换 |
| 🟢 P2 下一轮 | C1/C2（迁移与表设计） | 需新建迁移文件与补章节，工作量较大 |
| 🟢 P2 下一轮 | D1（PgBouncer） | 需补 pgbouncer 服务定义并测试 |
| 🟢 P2 下一轮 | E1/E2/E3（状态追踪） | 批量勾选 + 追加记录，工作量集中 |
| 🟢 P2 下一轮 | G1（搜索描述） | 一行修改，可与 P1 合并 |

---

## 五、本轮修复未覆盖的决策点

### 5.1 修复计划未明确处理的 3 项补充建议

我在上一轮回复中提出的 3 项补强建议，本轮均未执行：

| # | 建议 | 状态 | 影响 |
|---|------|------|------|
| 1 | A3 修复中一并纠正 constitution 字段路径 | ✅ 已执行 | 字段路径已纠正 |
| 2 | J6 修复中同时改正 design-system.md 第 150 行 `h-14` | ❌ 未执行 | 文档内部矛盾依然存在 |
| 3 | Beta implementation notes 追加"合并时同步 checkbox 与 progress.txt"规则 | ❌ 未执行 | 完成状态追踪机制未建立 |

### 5.2 新发现的问题

| 编号 | 问题 | 严重度 | 位置 |
|------|------|--------|------|
| N1 | 根 docker-compose.yml backend 健康检查用 `/api/v1/stats/summary`（业务接口） | 🟡 警告 | [docker-compose.yml:88](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docker-compose.yml#L88) |
| N2 | constitution 1.3.0 Changelog 漏列"PostgreSQL 15+→16+"的 Toolchain 变更归属 | 🟢 建议 | constitution.md 第 387-392 行 |
| N3 | A1 修复新增 13 个字段，但 architecture.md §7 reputation 段未同步补字段说明 | 🟡 警告 | architecture.md §7 |

---

## 六、总体评价

### 6.1 修复效果

| 评估项 | 结果 |
|--------|------|
| 阻塞级问题修复率 | 7/19（37%）完全修复 + 2/19（11%）部分修复 |
| 4 决策点落地率 | 1/4（A3）完全落地 + 1/4（J6）部分落地 |
| 已修复项质量 | ⭐⭐⭐⭐⭐ 平均 5 星（设计优秀，无返工） |
| 新问题引入 | 3 项（N1/N2/N3，均为 🟡/🟢 级） |
| 内部一致性 | A1/A2/B1/B2 之间一致，但与未修复项之间仍存在断层 |

### 6.2 修复方向评价

**正确的修复方向**：
- A 系列（配置硬编码）修复彻底，A1 的"配置优先 + 回退"设计值得作为后续修复范式
- B 系列（宪法原则）主体已更新，与现行规则对齐
- A3 完全采纳字段路径纠正建议

**未达预期的修复方向**：
- D1（PgBouncer）未采纳补齐建议，维持现状但未标注差异，冲突依然存在
- F2/F3 未按建议分类处理，两份过时文档仍以"待执行"姿态停留
- J6 仅保持代码侧正确，未消除文档内部矛盾

### 6.3 建议

**立即执行（P0，本轮收尾）**：
1. 修复 F1 ui-spec.md 第 3710-3811 行 GBK 乱码（重新生成 UTF-8 内容）
2. 修复 G2 oss-lifecycle.md 第 10-11 行 TTL 值（与 config.yaml/architecture.md 对齐）
3. 修复 D2 architecture.md §8.1 环境变量名（与 production-config-template.md 对齐）
4. 补齐 B3 的 `.specify/templates/` 三个模板同步

**本轮补齐（P1）**：
5. 修复 J6 design-system.md 第 150 行 `h-14` → `h-[var(--header-h)] / 52px`
6. 修复 F2 PLAN.md 顶部加完成横幅
7. 修复 F3 CURRENT_TASK_HANDOFF 归档至 `docs/archive/`
8. 修复 D3 统一健康检查端点为 `/healthz`（含根 docker-compose.yml 的反模式）
9. 修复 N3 architecture.md §7 reputation 段补 13 个 score_* 字段说明

**下一轮（P2）**：
10. C1/C2 补迁移文件与表设计章节
11. D1 补 PgBouncer 到 single-server.yml
12. E1/E2/E3 批量勾选 + 追加 progress.txt + 决策索引表加"实施状态"列
13. G1 更新 async-queue-analysis.md 搜索描述

---

*验证完成时间：2026-06-28*
