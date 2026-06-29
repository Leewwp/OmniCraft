# OmniCraft 文档治理体系设计

**设计日期**: 2026-06-29
**状态**: confirmed
**目标**: 解决项目文档体系长期存在的重复/矛盾/腐烂问题，建立可持续的文档治理机制

---

## 一、问题诊断

经过 3 轮独立审查（2026-06-25 健康检查 168 项 → 2026-06-28 两轮合计 261 项 → 合并去重 201 项），确认核心问题不是“问题太多”，而是**系统性问题**：

1. **多技能独立生成文档** → 多份设计稿/计划并存 → Agent 读到矛盾信息 → 自行发挥导致错误
2. **文档与代码脱节**：architecture.md 的手工维护内容与 config.yaml、migrations、routes.go 不一致
3. **缺乏权威源声明**：Agent 不知道该以哪份文档为准
4. **缺乏文件组织规范**：docs/ 根目录堆积 80+ 份文档，新旧混杂
5. **修复是点状的**：针对单个问题打补丁，没有系统性治理

---

## 二、解决方案总览

三层手段组合：

| 阶段 | 做什么 | 解决什么问题 |
|------|--------|-------------|
| **Phase 1**（本周） | 修复 48 项 P0 阻塞问题 | 安全问题 + 架构文档与代码的严重脱节 |
| **Phase 2**（本周—下周）| 文档精简 + 目录重组 + 权威源声明 | 多份文档并存导致的矛盾 |
| **Phase 3**（2 周内） | architecture.md 自动生成区 + doc-validator | 防止代码与文档再次脱节 |

---

## 三、文档目录结构（重组后）

```
OmniCraft/
|-- .specify/memory/constitution.md    [宪法] 最高设计原则
|
|-- CLAUDE.md                          [权威] Claude Code Agent 指令
|-- AGENTS.md                          [权威] 其他 Agent 指令（已存在，与 CLAUDE.md 共生维护）
|-- architecture.md                    [权威] 系统设计唯一真相源
|
|-- design/
|   |-- design-system.md               [权威] 视觉设计 token 唯一定义
|   |-- ui-spec.md                     [权威] UI 组件/页面规格
|
|-- backend/
|   |-- config.yaml                    [代码权威] 运行时配置
|   |-- config/config.go               [代码权威] 配置结构体
|   |-- migrations/                    [代码权威] 数据库 Schema
|
|-- docs/
|   |-- GLOSSARY.md                    [权威] [待创建] 项目术语表
|   |
|   |-- superpowers/
|   |   |-- specs/                     [权威] 已确认设计规格
|   |   |-- plans/                     [权威] Beta 路线图和计划
|   |
|   |-- deploy/                        [权威] 部署运维
|   |   |-- single-server-beta-runbook.md
|   |   |-- docker-compose.single-server.yml
|   |   |-- production-config-template.md
|   |
|   |-- working/                       [工作] 临时审查报告和修复记录
|   |
|   |-- adr/                           [架构决策记录]
|   |
|   |-- archive/                       [归档] 已过时或完成使命的文档
|
|-- task.json                          (historical - 模式 B 专用)
|-- progress.txt
|
|-- tools/
|   |-- doc-validator/                 [文档自动校验和生成工具]
```

### 关键变化

| 变化 | 原因 |
|------|------|
| docs/GLOSSARY.md 新增 | 解决术语不统一问题 |
| docs/adr/ 新增 | 架构决策集中管理 |
| docs/working/ 新增 | 临时文档统一入口 |
| docs/archive/ 统一归档 | 旧报告集中存放 |
| tools/doc-validator/ 新增 | 自动校验 + 自动生成 architecture.md 自动区 |
| design/ui-design-prompt.md 归档 | 不再作为开发参考 |
| design/doc-review-prompt.md 归档 | 校验逻辑已迁移到 doc-validator |
| docs/agents/ 归档 | 内容已合并到 CLAUDE.md/AGENTS.md |
| docs/futureWork/ 归档 | 内容完全过时 |
| docs/iteration-review/ 归档 | 审查报告已过使用周期 |
| 根目录所有 docs/2026-06-*.md 归档 | 临时分析文档归档 |

---

## 四、文档权威层级

### 权威文档登记表

当多份文档内容矛盾时，以下表决定以谁为准：

| 领域 | 唯一权威文档 |
|------|------------|
| 设计原则与不可妥协约束 | .specify/memory/constitution.md |
| 系统架构（API / Schema / 配置架构） | architecture.md |
| Agent 工作流与业务规则（Claude Code） | CLAUDE.md |
| Agent 工作流与业务规则（其他 Agent） | AGENTS.md |
| 视觉设计 token | design/design-system.md |
| UI 组件和页面规格 | design/ui-spec.md |
| 运行时配置真实值 | backend/config.yaml |
| 数据库 Schema 真实定义 | backend/migrations/*.sql |
| 功能设计输入 | docs/superpowers/specs/*.md | 优先级高于 plans |
| Beta 路线图和实施计划 | docs/superpowers/plans/*.md | — |
| 部署运维 | docs/deploy/single-server-beta-runbook.md |
| 术语定义 | docs/GLOSSARY.md [待创建] |

### 冲突解决规则

1. 表内的权威文档 > 其他任何文档
2. 宪法 > 所有其他文档和代码（宪法中声明的不可妥协约束不可被任何文档或代码覆盖）
3. 生产代码（config.yaml / migrations / routes.go）> 领域权威文档——当文档描述与代码实际行为不一致时，以代码为准，文档视为 bug
4. 领域权威文档之间：architecture.md > design/ > specs/ > plans/
5. 同目录内多份文档：日期最新的优先

### CLAUDE.md 与 AGENTS.md 的关系

两者是共生文档（co-authoritative pair），分别面向 Claude Code 和其他 Agent。修改其中一个时，**必须检查另一个是否需要同步**并同步更新。

### 元规则：本文档的权威地位

本治理设计文档（`docs/superpowers/specs/2026-06-29-omnicraft-documentation-governance-design.md`）
是文档治理体系的 **root source of truth**。CLAUDE.md 和 AGENTS.md 中关于文档治理的规则
（权威表、命名规范、Agent 工作流嵌入规则等）是本文档的**执行副本**，为方便 Agent 日常
工作而内嵌。若执行副本与本文档冲突，**以本文档为准**。

---

## 五、文件命名与存放规范

### 目录规则

docs/ 子目录结构详见 §3 文档目录结构。除 §3 中列出的 docs/ 子目录外，docs/ 根目录**不再新增 .md 文件**。

### 命名规则

| 文档类型 | 存放位置 | 命名格式 | 示例 |
|---------|---------|---------|------|
| 设计规格 | docs/superpowers/specs/ | YYYY-MM-DD-<topic>-design.md | 2026-05-30-omnicraft-dual-track-beta-design.md |
| 实施计划 | docs/superpowers/plans/ | YYYY-MM-DD-<topic>-roadmap.md | 2026-05-30-omnicraft-dual-track-beta-roadmap.md |
| 审查报告 | docs/working/ | YYYY-MM-DD-<scope>-review.md | 2026-07-01-config-sync-review.md |
| 修复记录 | docs/working/ | YYYY-MM-DD-<scope>-fix-log.md | 2026-07-01-config-sync-fix-log.md |
| 架构决策 | docs/adr/ | NNNN-<kebab-case-title>.md | 0001-use-ed25519-for-signing.md |
| 部署文档 | docs/deploy/ | <kebab-case-descriptive-name>.md | single-server-beta-runbook.md |
| 术语表 | docs/（固定位置） | GLOSSARY.md | - |

### 禁止清单

- 在 docs/ 根目录创建新 .md 文件
- 创建与已有权威文档同领域、同主题的第二份文档
- 文件名使用空格、中文、特殊字符
- working/ 目录下的文件名不带日期

### 临时文档清理规则

- 所有 docs/working/ 下的文档创建时应在文件头部注明 **预计失效日期**: YYYY-MM-DD
- 失效日期过后，Agent 应将文件移至 docs/archive/
- 默认失效日期为创建日期 + 2 个月

---

## 六、architecture.md 自动生成区

architecture.md 中以下区段与代码强耦合，人工维护注定腐烂。将这些区段标记为自动生成区。

### 三个自动生成区

| 区段 | 源码来源 | 内容 |
|------|---------|------|
| §3.2 API 路由清单 | backend/internal/handler/routes.go | 所有路由的 method、path、中间件、handler |
| §4 数据库 Schema | backend/migrations/*.sql | 所有表的 DDL、列定义、索引、外键 |
| §7 配置字段注册表 | backend/config/config.go | 所有配置字段的路径、类型、默认值、描述 |

### 工作方式

代码变更（config/migration/route）→ Agent 在提交前跑一次 `cd tools/doc-validator && go run . --fix` → doc-validator 读取源码并刷新 architecture.md 中的自动区 → Agent 将自动区变更一并提交。

### 防冲突标记

自动区用 HTML 注释标记边界，Agent 不得手工编辑自动区内的内容：

```markdown
<!-- AUTO-GENERATED: §3.2 API 路由清单 | source: backend/internal/handler/routes.go | DO NOT EDIT MANUALLY -->
...生成的表格...
<!-- END AUTO-GENERATED: §3.2 -->
```

### 可选 CI 检查

不设置 pre-commit hook。CI 中仅做**非阻断警告**：检测到源码变了但自动区未刷新时发出提示，不阻止合并。

---

## 七、文档校验工具

tools/doc-validator/ 是一个 Go 命令行工具，两个核心功能：

### 功能 1：自动生成（--fix）

从源码提取信息，刷新 architecture.md 的自动区。

```bash
cd tools/doc-validator && go run . --fix
```

### 功能 2：一致性校验（--check）

校验以下内容，输出问题清单（不做修改）：

| 检查项 | 说明 |
|--------|------|
| config 同步 | architecture.md §7 的字段列表是否与 config.go 一致 |
| schema 同步 | architecture.md §4 的表定义是否与 migrations 一致 |
| 路由同步 | architecture.md §3.2 的路由清单是否与 routes.go 一致 |
| Token 引用 | ui-spec.md 引用的设计 token 是否在 design-system.md 中有定义 |
| 交叉引用 | 文档中的文件链接是否指向存在的文件 |
| 过期文档 | docs/working/ 下超过预计失效日期的文档（warning） |

```bash
cd tools/doc-validator && go run . --check
cd tools/doc-validator && go run . --check --diff
```

---

## 八、Agent 工作流嵌入

以下规则写入 CLAUDE.md 和 AGENTS.md，嵌入现有 Agent 工作步骤中。

### 规则 1：修改代码后刷新自动区（嵌入代码实现步骤末尾）

若修改了 config.go / migrations / routes.go，在提交前运行 `cd tools/doc-validator && go run . --fix`

### 规则 2：新建文档时按规范存放（嵌入提交步骤之前）

若创建了新 .md 文件：放在 docs/working/ 目录下（或其他指定目录）；按规范格式命名；在文件头部注明创建日期和预计失效日期；禁止在 docs/ 根目录创建新 .md 文件。

### 规则 3：发现矛盾时查阅权威表（嵌入任务选择步骤中）

若遇到两份文档内容矛盾，查阅 CLAUDE.md 中的文档权威源表格，以权威文档为准。将矛盾记录为 issue 或审查报告，**不做自行发挥**。

---

## 九、实施计划

> **任务代码说明**：以下 CS-/AD-/DS-/DC-/FS-/CF-/TT- 系列代码来自
> `docs/2026-06-29-consolidated-fix-collection.md`（合并去重后的修复清单）。
> 本文档不重复完整清单，仅标注每批修复的范围；详细问题描述见该文件。

### Phase 1: 修复阻塞级安全问题 + 结构性准备（本周）

| # | 任务 | 产出 |
|---|------|------|
| P1-1 | 修复 CS-001~007（AuthRequired fail-open、SQL 注入、err.Error() 暴露、信誉分硬编码、nginx 安全头、SMTP 环境变量） | 代码修复 + 测试验证 |
| P1-2 | 修复 AD-001~007（schema 同步、幽灵字段、URL Scheme 统一、下载链路等） | architecture.md 与代码对齐 |
| P1-3 | 创建 docs/GLOSSARY.md | 首次填入已有术语 |
| P1-4 | 创建 docs/working/ 和 docs/adr/ 目录；移动所有临时/过时文件到 docs/archive/ | 目录结构重组完成 |
| P1-5 | 更新 CLAUDE.md 和 AGENTS.md | 加入文档权威表 + 文件命名规范 + 3 条 Agent 规则 |

### Phase 2: 文档精简与内容统一（本周—下周）

| # | 任务 | 产出 |
|---|------|------|
| P2-1 | 修复 DS-001~005（token 引用、圆角、字体栈、组件规范补全） | design-system.md 和 ui-spec.md 统一 |
| P2-2 | 修复 DC-001~005（Constitution 版本号、Changelog、模板同步） | 宪法和核心文档一致性修复 |
| P2-3 | 修复 FS-001~003（收藏集标注、搜索建议、众裁阈值对齐） | 功能规格矛盾消除 |
| P2-4 | 修复 CF-001（features 注册表对齐）+ config.yaml 字段补全 | 配置文档对齐 |
| P2-5 | 修复 TT-001, TT-003（Task 151-169 progress.txt 补全、安全加固条目拆分）+ progress.txt 拆分清理 | 任务跟踪补齐 |
| P2-6 | 编写 docs/adr/ 首批记录（从现有 DEC 决策迁移） | 架构决策集中管理 |

### Phase 3: 自动化工具（2 周内）

| # | 任务 | 产出 |
|---|------|------|
| P3-1 | 实现 tools/doc-validator/ 核心：config/schema/路由同步的 --check 和 --fix | 自动生成 architecture.md 三个自动区 |
| P3-2 | 为 architecture.md 添加 AUTO-GENERATED 标记边界 | 文档中标记自动区 |
| P3-3 | 实现 token 引用校验、交叉引用检查 | 补充校验能力 |

---

## 十、成功标准

| 指标 | 当前状态 | 目标状态 |
|------|---------|---------|
| docs/ 根目录 .md 文件 | ~20 份混杂 | 0 份 |
| 权威文档登记表条目数 | 不明确 | 12 个权威来源类别（详见 §4 权威文档登记表） |
| architecture.md 与代码脱节 | 40+ 处不一致 | 0 |
| 术语混用 | 3+ 种同概念不同称呼 | 统一在 GLOSSARY.md |
| Agent 读矛盾文档后自行发挥 | 频繁 | 查阅权威表按规则处理 |
| 新文档堆放位置 | 直接扔 docs/ 根目录 | 按规范放对应子目录 |
| doc-validator --check 发现问题 | 201 项 | 0 项 |

---

## 十一、本文档的维护

### 修订触发条件

以下任一情况发生时，需修订本文档：

- 目录结构变更（新增/移除/重命名 docs/ 子目录）
- 权威文档登记表条目新增、移除或权威层级变化
- Agent 工作流步骤重构（影响 §8 的嵌入位置描述）
- doc-validator 工具的功能范围变更

### 修订流程

1. 修订需经项目负责人确认
2. 更新文档顶部的**设计日期**为修订日期
3. 在下方 Changelog 中记录修订摘要

### Changelog

| 日期 | 修订内容 |
|------|---------|
| 2026-06-29 | 初版发布 |

---

*设计文档结束*
