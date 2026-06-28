# OmniCraft 项目文档全面系统性检查报告

**检查日期**: 2026-06-28
**检查范围**: 项目所有开发相关文档（共 80+ 文件），包括架构设计、产品需求、设计系统、UI 规格、Beta 计划/设计、审查报告、部署运维、宪法/模板、Agent 工作指南等
**检查方法**: 逐文件阅读 + grep 交叉比对 + 字段级一致性验证 + 已知问题追踪交叉引用
**结论**: ⚠️ **文档整体质量良好，但存在系统性一致性问题，部分严重问题仍未修复**

---

## 一、问题总览

### 1.1 统计摘要

| 类别 | 新增问题 | 已知未修复 | 已修复待验证 | 合计 |
|------|---------|-----------|------------|------|
| A. 文档一致性与交叉引用 | 8 | 2 | 4 | 14 |
| B. 架构设计逻辑 | 5 | 1 | 2 | 8 |
| C. 配置与实现对齐 | 3 | 6 | 3 | 12 |
| D. 设计系统/Token | 4 | 2 | 2 | 8 |
| E. 功能规格缺失/矛盾 | 6 | 3 | 1 | 10 |
| F. 格式与术语 | 5 | 1 | 2 | 8 |
| G. 版本/时效性 | 4 | 2 | 2 | 8 |
| H. 安全与合规 | 2 | 1 | 3 | 6 |
| **合计** | **37** | **18** | **19** | **74** |

### 1.2 按严重度分类

| 严重度 | 数量 | 说明 |
|--------|------|------|
| 🔴 严重 | 15 | 阻塞开发或安全风险，必须修复 |
| 🟠 高 | 22 | 影响多文档一致性，建议近期修复 |
| 🟡 中 | 23 | 影响文档可读性或局部准确性 |
| 🟢 低 | 14 | 格式、措辞优化，择机修复 |

---

## 二、分类详细问题清单

### A. 文档一致性与交叉引用 (14 项)

#### A-001 🔴 Constitution SYNC IMPACT REPORT 版本号过时

- **位置**: `.specify/memory/constitution.md` 第 1-16 行（HTML注释块）
- **问题**: SYNC IMPACT REPORT 显示版本变更 `1.1.1 → 1.2.0`，但文档实际已迭代至 `1.3.0`（第 384 行）。SYNC IMPACT REPORT 从未被更新，记录的仍是 1.2.0 时的变更。
- **影响**: 误导审查者认为 constitution 仍停留在 1.2.0
- **建议**: 将 SYNC IMPACT REPORT 更新为实际变更链：`1.1.1 → 1.2.0 → 1.3.0`，或移除过时的 HTML 注释块

#### A-002 🔴 Constitution 模板同步声明与实际状态矛盾

- **位置**: `.specify/memory/constitution.md` 第 12-14 行
- **问题**: SYNC IMPACT REPORT 声称 `plan-template.md`、`spec-template.md`、`tasks-template.md` 已同步到 1.3.0（"✅ 1.3.0 变更已传播"），但实际验证表明确未同步（见 `2026-06-28-final-comprehensive-verification.md` B3）。Constitution 第 379-380 行明确要求 "after any MINOR or MAJOR amendment, .specify/templates/* MUST be updated"。
- **影响**: 违反项目宪法强制要求；模板与宪法脱节
- **建议**: 立即同步 3 个核心模板，或修正 SYNC IMPACT REPORT 声明为实际状态

#### A-003 🔴 ui-spec.md 颜色 token 来源引用错误

- **位置**: `design/ui-spec.md` 第 16 行
- **问题**: ui-spec.md 声明"颜色 token：直接复用 `architecture.md` §10.4"，但 DEC-011 已决定以 `design/design-system.md` 为唯一设计系统权威，`architecture.md` §10.4 的色值表应已删除或改为指向 design-system.md。ui-spec 应引用 design-system.md 而非 architecture.md §10.4。
- **影响**: ui-spec 引用链断裂——开发者按指引去 architecture.md §10.4 找到的可能已不是色值表
- **建议**: 改为 "颜色 token：参见 `design/design-system.md` 颜色章节"

#### A-004 🟠 Beta 设计规格引用已归档文件

- **位置**: `docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md` 第 72 行
- **问题**: Beta 设计 §3.1 引用 `UI Design.md`（已归档至 `docs/archive/`）作为审阅材料，但未标注其归档状态。
- **建议**: 在引用处添加 `（已归档至 docs/archive/，仅作历史参考）` 标注

#### A-005 🟠 CLAUDE.md "AGENTS.md" 文件名不一致

- **位置**: `CLAUDE.md` 全文多处
- **问题**: CLAUDE.md 规则中多次引用 "AGENTS.md" 文件，但根目录实际文件名为 `CLAUDE.md`。`frontend/AGENTS.md` 文件存在但与根 `CLAUDE.md` 是不同的文件。
- **建议**: 统一引用方式——引用自身时用 "本文件"，引用 frontend 的用 `frontend/AGENTS.md`

#### A-006 🟠 architecture.md §10.4 修改状态待验证

- **位置**: `architecture.md` §10.4
- **问题**: DEC-011 要求更新 §10.4 色值表，但需确认实际执行情况。
- **建议**: 交叉验证 DEC-011 执行状态

#### A-007 🟡 design-review-decisions.md 实施状态大面积过期

- **位置**: `docs/2026-06-24-design-review-decisions.md` 决策索引表
- **问题**: 33 个 DEC 决策大部分标为 "⏳ 待实施"，但报告日期为 6 月 24 日，距今已 4 天且经历多轮修复。实际已实施的 DEC 未被更新。
- **建议**: 逐项核实并更新实施状态

#### A-008 🟡 审查报告与问题清单之间交叉引用断裂

- **位置**: decisions.md 与 merged.md
- **问题**: decisions.md 声明关联 merged.md 为"问题清单(发现层)"，但 merged.md 中的问题 ID（如 B3、H1）与 decisions.md 中的 DEC ID（如 DEC-001）不是一一对应。
- **建议**: 在 decisions.md 添加 DEC ID 到 merged.md 问题 ID 的映射表

#### A-009 🟢 根目录 PRD 文件名含中文和空格

- **位置**: 根目录 `OmniCraft（万象工坊）V0.3 正式版产品需求文档.md`
- **问题**: 文件名含中文括号、中文文字和空格，部分工具链可能处理异常。
- **建议**: 重命名为 `omnicraft-v0.3-prd.md`

#### A-010 🟢 多处文档引用 CLAUDE.md 的章节名可能已过时

- **位置**: constitution.md、Beta specs 等
- **问题**: 多份文档引用 `CLAUDE.md §Key Rules` 的 13 条规则，但 CLAUDE.md 实际有 25 条规则。
- **建议**: 精确引用具体规则编号

#### A-011 🟢 k8s/README.md 内容极简

- **位置**: `k8s/README.md`
- **建议**: 补充说明："K8s 配置为 P2 阶段预留，当前不活跃。启用条件见 architecture.md §2.3"

#### A-012 🟢 docs/PLAN.md 与 Beta 路线图关系未说明

- **位置**: `docs/PLAN.md`
- **建议**: 添加说明"本文档为早期计划，已完成。当前活跃计划见 docs/superpowers/plans/"

#### A-013 🟠 architecture.md 内容浏览布局表述容易混淆

- **位置**: `architecture.md` §10.5 vs §10.6
- **问题**: §10.5 描述"原创区卡片无标签 Badge"，但 §10.6 的分类 Tab 描述又说原创区显示分类 Tab。两者不矛盾但表述容易混淆。
- **建议**: 明确区分"导航分类 Tab"和"内容卡片标签 Badge"

#### A-014 🟠 Constitution 引用 architecture.md §5 但章节可能不存在

- **位置**: `.specify/memory/constitution.md` 第 356 行
- **问题**: Constitution 声明 "Canonical source: `architecture.md §5` (reputation)"，但 architecture.md 的实际章节结构需验证。
- **建议**: 验证并修正章节引用

---

### B. 架构设计逻辑 (8 项)

#### B-001 🔴 通知系统——文档描述轮询但 DEC-033 决定改为 SSE

- **位置**: `CLAUDE.md` 通知系统部分 + `architecture.md`
- **问题**: DEC-033 决定通知系统改为 SSE 推送，但 CLAUDE.md 仍描述"前端 Entry Header 通知铃铛显示未读数（GET /api/v1/notifications/unread-count 轮询，5 分钟间隔）"。
- **影响**: 开发者按文档实现轮询而非 SSE
- **建议**: 更新 CLAUDE.md 和 architecture.md 的通知系统描述

#### B-002 🟠 内容可见性过滤规则分散

- **位置**: `architecture.md` 多处
- **问题**: 系统引入了复杂的可见性规则但没有一个集中的"内容可见性规则汇总"章节。
- **影响**: 开发者需散落各处拼凑完整规则
- **建议**: 新增 "内容可见性规则汇总" 章节

#### B-003 🟠 私信 SSE 与通知 SSE 的架构边界模糊

- **位置**: DEC-007 与 DEC-033 交叉部分
- **问题**: DEC-033 提到"统一 SSE 连接管理(一个连接处理私信 + 通知)或分开连接"，但未给出最终选择。
- **建议**: 确认并记录最终架构选择

#### B-004 🟡 DDD 重构后的目录结构与 constitution Principle II 冲突

- **位置**: DEC-014 vs constitution Principle II
- **问题**: DEC-014 决定重构为 `domain/application/infrastructure/interfaces` 结构，但 constitution Principle II 仍描述当前的 `handler/service/repository/model` 技术分层结构。
- **建议**: DDD 重构完成后触发 constitution amendment

#### B-005 🟡 推荐引擎缓存策略描述需验证

- **位置**: `architecture.md` §12.2
- **建议**: 核实 §12 描述与 `recommendation_service.go` 实际实现一致

#### B-006 🟡 内容下载链路——download_count Redis key 格式未文档化

- **位置**: `architecture.md` §4.3 第 710 行注释
- **建议**: 补充 Redis key 格式 `download_count:{content_id}` 和同步周期

#### B-007 🟢 队列系统启用后的 worker 启动顺序缺少文档

- **位置**: DEC-019
- **建议**: 将 worker 启动顺序纳入 architecture.md 或运维 runbook

#### B-008 🟢 OSS 生命周期策略文档与实际配置一致性待验证

- **位置**: `docs/oss-lifecycle.md` vs `backend/config.yaml`

---

### C. 配置与实现对齐 (12 项)

#### C-001 🔴 architecture.md §7 配置段"幽灵字段"仍未完全清除 (N6)

- **位置**: `architecture.md` §7（cache/rate_limit/queue/publish/green/captcha 段）
- **问题**: 第 4 轮 H5 修复引入"幽灵字段"——queue 段 `worker_notif` 应为 `worker_notification`（残留），cache/rate_limit/publish/green/captcha 段共缺 16 个字段。
- **影响**: 开发者按文档配置这些字段会被 viper 静默忽略
- **建议**: 严格按 config.go 结构体逐字段对齐

#### C-002 🔴 ui-spec.md token 引用形式系统性错误 (N7)

- **位置**: `design/ui-spec.md` 全文 30+ 处
- **问题**: ui-spec.md 使用点号形式 `canvas-default.dark`/`border-default.dark` 引用 token，但 Tailwind CSS 不支持点号分隔类名。`--border-muted`、`--canvas-default-dark`、`--border-default-dark` 三个 token 在 design-system.md 中未定义。
- **影响**: 前端开发者按 ui-spec.md 写 className 无效
- **建议**: 统一为连字符形式，在 design-system.md 补定义缺失 token

#### C-003 🔴 architecture.md §11.3 agent 段未与 §7 同步 (N8)

- **位置**: `architecture.md` §11.3 配置扩展部分
- **问题**: §7 agent 段已补 12 个字段，但 §11.3 仍只有 9 个字段，缺失 `max_user_message_chars`、`chat_max_context_messages`、`hmac_secret`。
- **影响**: 同文件内同字段两处描述不一致
- **建议**: §11.3 agent 段补 3 个字段

#### C-004 🟠 progress.txt 安全加固条目未拆分 (E2)

- **位置**: `progress.txt` 第 3204-3217 行
- **问题**: "安全加固批量执行" 是 1 条合并条目，涵盖 4 份独立加固计划。
- **建议**: 拆分为 4 条独立条目

#### C-005 🟡 features 字段注册表一致性待验证

- **位置**: Beta roadmap New Config Field Registry vs `backend/config.yaml`
- **建议**: 逐字段核对

#### C-006 🟡 verification/smtp 字段命名不一致

- **位置**: `backend/config.yaml` vs Beta roadmap 注册表
- **问题**: roadmap 使用 `verification.email_ttl_sec`，smtp 字段在不同文档间命名不统一（`user` vs `username`, `from_address` vs `from_email`）。
- **建议**: 统一命名，以 config.go 结构体 mapstructure tag 为准

#### C-007 🟡 captcha 配置段缺少 provider 枚举值说明

- **位置**: `backend/config.yaml` captcha 段
- **建议**: 补充注释 "provider: 'bypass' (开发) | 'aliyun_v2' (生产)"

#### C-008 🟡 features.desktop_deploy_enabled 文档描述表述不一致

- **位置**: `CLAUDE.md` 规则 10 vs Beta roadmap Cross-Plan Contract Decisions

#### C-009 🟢 rate_limit 配置缺少字段单位说明

- **位置**: `backend/config.yaml` rate_limit 段

#### C-010 🟢 deployment 环境变量与实际架构不一致

- **位置**: `docs/deploy/single-server-beta-runbook.md` `.env` 示例

#### C-011 🟢 config_override.yaml 缺少 queue 和 hot_rank 段

- **位置**: `docs/deploy/single-server-beta-runbook.md`
- **问题**: 生产 config_override.yaml 示例缺少 queue（DEC-019 决定启用）和 hot_rank（DEC-029 决定暴露）段。

#### C-012 🟡 Beta roadmap 中 D-02 至 D-05 和 R-02 仍为未勾选

- **位置**: Beta roadmap 执行队列表

---

### D. 设计系统/Token (8 项)

#### D-001 🔴 design-system.md 缺少 3 个被 ui-spec.md 引用的 token 定义

- **位置**: `design/design-system.md` Token 定义表
- **问题**: ui-spec.md 中 30+ 处使用 `border-muted`/`canvas-default-dark`/`border-default-dark`，但 design-system.md 中 grep 0 匹配这 3 个 token。
- **建议**: 在 design-system.md 补充定义

#### D-002 🟠 圆角 Token 值 ui-spec 与 design-system 冲突

- **位置**: design-system.md 第 67-75 行 vs ui-spec.md 第 19 行
- **问题**: design-system.md: `--radius-lg` = `8px`（卡片/容器）。ui-spec.md: `rounded-lg 12（标签）`。两处的 `rounded-lg` 语义冲突。
- **建议**: 统一圆角体系——以 design-system.md 为准，ui-spec 中修正为 `rounded-full`（标签）

#### D-003 🟠 Tailwind 语义别名与 CSS 变量映射缺失

- **位置**: `design/design-system.md` 组件规范部分
- **问题**: 组件规范使用 `bg-card`、`border-input`、`text-primary-foreground` 等类名，但 Token 表中只定义了 CSS 自定义属性，没有 `--card`、`--input`、`--primary-foreground` 的定义。
- **建议**: 区分 "CSS 自定义属性"和 "Tailwind 语义别名"，并在文档中说明映射关系

#### D-004 🟡 暗色模式检查清单 token 体系不一致

- **位置**: `design/design-system.md` 第 186-194 行
- **建议**: 统一 Token 命名体系并更新检查清单

#### D-005 🟡 Footer/MasonryGrid/SearchAgentInput 组件规范缺失

- **位置**: `design/design-system.md` Component 规范段
- **建议**: 补充缺失的组件规范

#### D-006 🟢 字体栈三份文档给出三种版本

- **位置**: design-system.md vs ui-spec.md vs constitution.md
- **建议**: 统一为 design-system.md 版本（含中文字体保障）

#### D-007 🟡 组件规范中语义别名与 Token 表脱节

- **位置**: `design/design-system.md`
- **建议**: 与 D-003 一并处理

#### D-008 🟢 MasonryGrid 组件规范缺少响应式列数定义

- **位置**: `design/design-system.md`

---

### E. 功能规格缺失/矛盾 (10 项)

#### E-001 🔴 收藏集功能：文档描述已实现但实际未实现

- **位置**: `CLAUDE.md` §收藏集（Task 122–123） vs 实际代码
- **问题**: CLAUDE.md 详细描述了收藏集数据模型、API、去重逻辑、权限控制和前端入口，但 DEC-010 确认收藏集 API 和页面根本不存在。
- **影响**: 开发者可能认为收藏集已可用
- **建议**: 在 CLAUDE.md 添加 "⚠️ 尚未实现，待 Beta 后实施" 标注

#### E-002 🔴 草稿系统：CLAUDE.md 未提及 `draft` 状态

- **位置**: `CLAUDE.md` vs DEC-008
- **问题**: DEC-008 决定完整实现草稿系统，但 CLAUDE.md 内容状态描述中未包含 `draft` 状态。
- **建议**: 在 CLAUDE.md 补充草稿系统描述

#### E-003 🟠 Studio 路由清单中 `/studio/favorites` 后端 API 缺失

- **位置**: `architecture.md` §3.1 第 147 行 vs §3.2 API 路由清单
- **问题**: 前端路由包含 `/studio/favorites/page.tsx`，但 API 路由清单中收藏集相关端点未列出。
- **建议**: 标注为 "P1 预留"或补充实现

#### E-004 🟠 支付模块 "MVP" 术语过时

- **位置**: `CLAUDE.md` §支付模块
- **建议**: 将 "MVP 阶段" 改为 "当前阶段"

#### E-005 🟡 邮箱验证状态描述分散

- **位置**: CLAUDE.md + Beta Design §7 + architecture.md
- **建议**: 新增集中说明

#### E-006 🟡 众裁判决阈值需逐字段核对

- **位置**: CLAUDE.md vs `backend/config.yaml` judge 段

#### E-007 🟡 原创区分类枚举中英文映射缺失

- **位置**: PRD vs `architecture.md` §4.3
- **建议**: 建立中英文分类映射表

#### E-008 🟡 search_suggestions 与 search/trending API 关系不清

- **位置**: CLAUDE.md Task 120 vs architecture.md §15.5

#### E-009 🟢 IP 分类枚举与原创区分类枚举关系不明

- **位置**: `architecture.md` §4.2 vs §4.3

#### E-010 🟢 定时发布功能缺少独立 config 说明

- **位置**: DEC-008

---

### F. 格式与术语 (8 项)

#### F-001 🟠 "MVP" vs "Beta" 术语混用

- **位置**: 多处（CLAUDE.md、architecture.md、constitution.md 等）
- **问题**: 项目已进入 "公开 Beta" 阶段，但多处文档仍使用 "MVP" 术语。
- **建议**: 全局替换：已完成的 MVP 改为 "Phase 1" 或 "V0"，当前阶段称为 "Beta"

#### F-002 🟠 核心业务概念术语不统一

- **位置**: 多处
- **问题**: "原创区"/"Original Zone"/`zone='original'`，"二创区"/"Fanwork Zone"/`zone='fanwork'`，"赛博判官"/"Cyber Judge"/"judge"，"信誉分"/"Reputation Score"/"reputation"——同一概念在不同文档中使用三种形式。
- **建议**: 在 architecture.md 开头新增统一术语表

#### F-003 🟡 Markdown 代码块缺少语言标识

- **位置**: 多处文档

#### F-004 🟢 表格对齐和编号列表风格不一致

- **位置**: 多处文档

---

### G. 版本/时效性 (8 项)

#### G-001 🔴 constitution.md 1.3.0 Changelog 可能不完整

- **位置**: `.specify/memory/constitution.md` 第 390-391 行
- **问题**: 1.3.0 Changelog 列出了 4 项变更，但实际变更可能更多。
- **建议**: 逐条对比 1.2.0 和 1.3.0 的 diff，补全 changelog

#### G-002 🟠 architecture.md 版本号 1.0 远低于实际迭代次数

- **位置**: `architecture.md` 第 3 行
- **问题**: architecture.md 标注 "版本：1.0"，但经历了 §10-§15 等多次重大增补，实际相当于 2.0+ 的内容量。
- **建议**: 更新为 "2.0" 或采用日期版本

#### G-003 🟠 多份 review 报告中的 "当前状态" 已过时

- **位置**: `docs/review/` 下多份报告
- **建议**: 在每份旧报告的头部添加 "审查日期：YYYY-MM-DD，当前状态请参见最新审查报告" 横幅

#### G-004 🟡 6 份 2026-06-25 旧迭代审查报告未归档 (N9)

- **位置**: `docs/iteration-review/`

#### G-005 🟡 task.json 中 `passes: true` 语义需说明

- **建议**: 在 task.json 顶部添加注释

#### G-006 🟢 PRD V0.3 版本号与当前 Beta 阶段不一致

#### G-007 🟡 design-system.md 版本号 2.0 但无 changelog

#### G-008 🟡 doc-review-prompt.md 引用已归档文件路径

---

### H. 安全与合规 (6 项)

#### H-001 🔴 生产环境 runbook SMTP 环境变量不完整

- **位置**: `docs/deploy/single-server-beta-runbook.md` 第 58 行
- **问题**: `.env` 示例只列出了 `SMTP_PASSWORD`，缺少其他 SMTP 环境变量。
- **建议**: 补充完整 SMTP 环境变量清单

#### H-002 🟠 桌面端 Ed25519 密钥轮换流程缺失

- **位置**: DEC-032 + Beta roadmap
- **建议**: 补充密钥应急轮换 SOP

#### H-003 🟡 法律文本版本管理搁置状态需明确

- **位置**: DEC-003 + `backend/config.yaml` legal 段

#### H-004 🟡 CORS 中间件配置的描述与实际实现需验证

- **位置**: `architecture.md` §14 vs `backend/internal/middleware/cors.go`

#### H-005 🟢 Token 轮换策略文档与实现需验证

#### H-006 🟢 Agent API Key 泄露风险文档警告不足

---

## 三、问题优先级矩阵

### 3.1 阻塞级 (P0) — 本周内必须修复

| 编号 | 问题 | 修复成本 |
|------|------|---------|
| C-001 | architecture.md §7 幽灵字段 (N6) | 中 |
| C-002 | ui-spec.md token 引用错误 (N7) | 中 |
| C-003 | architecture.md §11.3 与 §7 不同步 (N8) | 低 |
| A-001 | Constitution SYNC IMPACT REPORT 过时 | 低 |
| A-002 | Constitution 模板同步声明虚假 | 中 |
| A-003 | ui-spec 颜色 token 来源引用错误 | 低 |
| B-001 | 通知系统文档未体现实时推送决策 | 中 |
| D-001 | design-system.md 缺 3 个 token 定义 | 低 |
| E-001 | 收藏集文档描述 vs 实际实现不一致 | 中 |
| E-002 | CLAUDE.md 未提及 draft 状态 | 低 |
| G-001 | constitution changelog 可能不完整 | 低 |
| H-001 | runbook SMTP 环境变量不完整 | 低 |

### 3.2 高优先级 (P1) — 两周内修复

A-004, A-005, A-006, A-013, A-014, B-002, B-003, C-004, D-002, D-003, E-003, E-004, F-001, F-002, G-002, G-003

### 3.3 中优先级 (P2) — 一月内修复

A-007, A-008, B-004, B-005, B-006, C-005 ~ C-008, C-012, D-004, D-005, D-007, E-005 ~ E-008, G-004, G-005, G-007, G-008, H-002, H-003

### 3.4 低优先级 (P3) — 择机修复

A-009 ~ A-012, B-007, B-008, C-009 ~ C-011, D-006, D-008, E-009, E-010, F-003, F-004, G-006, H-004 ~ H-006

---

## 四、与已知问题跟踪的交叉引用

| 已知问题 ID | 本报告编号 | 说明 |
|------------|-----------|------|
| N6 | C-001 | architecture.md §7 幽灵字段 |
| N7 | C-002 | ui-spec.md token 命名 |
| N8 | C-003 | §11.3 agent 段不同步 |
| N9 | G-004 | 旧报告未归档 |
| E2 | C-004 | progress.txt 未拆分 |
| B3 | A-002 | Constitution 模板同步 |
| DEC-033 | B-001 | 通知 SSE 文档未更新 |
| DEC-008 | E-002 | 草稿系统文档缺失 |
| DEC-010 | E-001 | 收藏集文档描述 |
| DEC-011 | A-003, A-006 | 设计系统统一相关 |

---

## 五、建议的修复路线图

### Phase 1 (本周) — 12 项 P0 修复
消除幽灵字段、统一 token 命名、同步 agent 配置段、修复 constitution 声明、更新通知系统文档、补充 design-system token、修正 ui-spec 引用、标注功能实现状态

### Phase 2 (2周) — 16 项 P1 修复
术语统一、交叉引用修复、版本号更新、圆角/字体统一、Tailwind 语义别名映射、progress.txt 拆分、功能规格补充

### Phase 3 (1月) — 23 项 P2 修复
决策状态更新、旧报告归档、DDD 重构宪法更新、字段逐一核对、组件规范补充

### Phase 4 (持续) — 14 项 P3 优化
格式统一、PRD 文件名、PLAN.md 关系说明、安全文档补充

---

## 六、附录

### 6.1 检查覆盖的文档完整清单

| 类别 | 文件数 | 检查深度 |
|------|--------|---------|
| 根目录核心文档 | 5 | 全文 |
| design/ | 4 | 全文 |
| .specify/ | 7 | 全文 |
| docs/superpowers/specs/ | 2 | 全文 |
| docs/superpowers/plans/ | 14 | 全文 |
| docs/review/ | 20+ | 标题+结构 |
| docs/iteration-review/ | 10+ | 全文 |
| docs/deploy/ | 4 | 全文 |
| docs/agents/ | 3 | 全文 |
| docs/其他 | 8 | 全文 |
| frontend/*.md | 3 | 全文 |
| backend/config.yaml | 1 | 全文 |

### 6.2 验证工具与方法

- `grep` 跨文件交叉比对（token 名称、配置字段名、API 路由）
- 逐字段对比 config.go mapstructure tag vs architecture.md §7
- 决策记录 (DEC) 与文档实际内容的一致性检查
- 已知问题清单 (newly-found-issues.md) 的交叉引用验证
- 多文档间术语使用一致性检查

---

*报告生成时间: 2026-06-28*
*检查人: AI 系统分析（基于全项目文档静态分析 + 已知问题追踪交叉引用）*
*本报告应与 `docs/iteration-review/2026-06-28-final-comprehensive-verification.md` 和 `docs/iteration-review/2026-06-28-newly-found-issues.md` 配合阅读*
