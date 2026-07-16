# OmniCraft 万象工坊 - Agent 工作指南

## 任务来源与开发模式

本仓库同时支持四套任务来源。每次新 Agent 会话必须先确定当前模式，再选择任务。不要混用多套跟踪规则。

### 模式 A：双轨 Beta 计划集（当前公开 Beta 加固工作）

适用于 `docs/superpowers/plans/` 中的 2026-05-30 双轨 Beta 计划。用户要求继续 Beta 工作、指定 roadmap 任务，或当前分支为 `codex/beta/*` 时，使用此模式。

- 设计输入：`docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md`
- 执行索引：`docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md`
- 协作说明：`docs/superpowers/plans/2026-05-30-omnicraft-beta-implementation-notes.md`
- 子系统计划：
  - `docs/superpowers/plans/2026-05-30-omnicraft-beta-foundation.md`
  - `docs/superpowers/plans/2026-05-30-omnicraft-beta-verification-feedback.md`
  - `docs/superpowers/plans/2026-05-30-omnicraft-beta-admin-operations.md`
  - `docs/superpowers/plans/2026-05-30-omnicraft-beta-agent-entrypoints.md`
  - `docs/superpowers/plans/2026-05-30-omnicraft-beta-desktop-deploy-security.md`
  - `docs/superpowers/plans/2026-05-30-omnicraft-beta-release-validation.md`

规则：

1. 先读 roadmap，选择依赖已完成的最小 ID 未勾选任务，再读对应子系统计划。
2. 一个 roadmap 任务对应一个 agent 会话、一个 `codex/beta/<task-id>` 分支、一个独立 worktree 和一个 commit。
3. 使用 `codex/beta-integration` 作为唯一集成分支。任务分支合并前必须 rebase 到最新集成分支，重新验证后执行 `git merge --ff-only`。
4. 任务开始前按 implementation notes 预约写入范围。共享文件和代码区域不得由多个实现代理同时编辑。
5. 实现遵循 TDD：先写失败测试并确认预期失败，再实现最小改动，最后重构。
6. 每个任务完成后必须经过两阶段审查：先规格符合性审查，再代码质量审查。审查问题必须修复并复审。
7. 收到 `DONE_WITH_CONCERNS` 时先处理 concerns。正确性、安全、范围或契约疑问未解决前不得进入审查或合并。
8. 完成后同步勾选子系统计划和 roadmap checkbox，更新 `progress.txt`。本模式禁止修改 `task.json`。
9. 历史 `passes: true` 只代表曾经完成，不能替代 Beta 回归验证，尤其不能跳过历史 Task 156-168 的质量门槛。
10. 桌面端一键部署：D-02 至 D-05 完成后开启 `features.desktop_deploy_enabled: true`；R-02 验证通过前不得发布到生产环境。

### 模式 B：历史 `task.json` 任务账本

适用于维护历史 MVP、处理 `task.json` 中新增任务，或用户明确指定 `task.json` 任务时。

> **注意**：`task.json` 中 100+ 个历史任务已全部完成（`passes: true`），仅作历史记录保留。新工作请按用户指定使用模式 A（Beta 路线图）、模式 C（社区功能计划）或模式 D（项目卓越/Web Agent）。仅在用户明确指定历史任务 ID 时使用此模式。

规则：

1. 读取 `task.json`，优先选择 `passes: false`、依赖已完成且 ID 最小的任务。
2. 仔细阅读任务的 `description`、`steps`、`depends_on` 和 `ui_spec_ref`。
3. 完成后只将该任务的 `passes: false` 改为 `passes: true`，更新 `progress.txt`。
4. 永远不要删除任务、修改任务描述或移除历史记录。

### 模式 C：2026-06-30 社区功能计划集

适用于 `docs/superpowers/plans/` 中的 2026-06-30 社区功能计划。用户要求实现消息通知、浏览历史、收藏集、内容系列、来源联动或协作邀请时，使用此模式。

- 设计输入：`docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md`
- 计划文件（建议执行顺序）：
  1. `docs/superpowers/plans/2026-06-30-omnicraft-community-messages-notifications.md`
  2. `docs/superpowers/plans/2026-06-30-omnicraft-community-browse-history.md`
  3. `docs/superpowers/plans/2026-06-30-omnicraft-community-collections.md`
  4. `docs/superpowers/plans/2026-06-30-omnicraft-community-content-series.md`
  5. `docs/superpowers/plans/2026-06-30-omnicraft-community-source-linkage.md`
  6. `docs/superpowers/plans/2026-06-30-omnicraft-community-collaboration-invites.md`

规则：

1. 先读设计输入，再读目标计划全文；不得只按计划片段实现。
2. 一个计划文件对应一个 agent 会话、一个 `codex/community/<plan-slug>` 分支、一个独立 worktree 和一个 commit。
3. 本模式不使用 Beta roadmap checkbox，不修改 `task.json`。
4. 实现遵循 TDD：先写失败测试并确认预期失败，再实现最小改动，最后重构。
5. 每个计划完成后必须经过两阶段审查：先规格符合性审查，再代码质量审查。审查问题必须修复并复审。
6. 多代理并行时必须先明确每个代理的文件范围；共享文件（routes、config、DTO、UI spec、翻译文件、迁移序号）必须串行预约，禁止静默抢写。
7. 完成后更新计划内 checkbox 和 `progress.txt`，精确暂存本计划涉及文件并提交。

### 模式 D：2026-07 项目卓越与 Web Agent 产品化

适用于以下两份计划；用户指定项目工程加固、验证门、文档治理、HTTP composition root 或 Web Agent 产品化时使用：

- 工程加固：`docs/superpowers/plans/2026-07-08-omnicraft-project-excellence-hardening.md`
- Agent 设计：`docs/superpowers/specs/2026-07-16-omnicraft-dual-surface-agent-productization-design.md`
- Web Agent：`docs/superpowers/plans/2026-07-16-omnicraft-web-agent-productization.md`

规则：

1. 一个编号 Task 对应一个 Agent 会话、一个独立 worktree、一个分支和一个 commit；分支分别使用 `codex/excellence/<task-id>` 或 `codex/web-agent/<task-id>`。
2. 按计划依赖顺序执行；同一计划内共享 service/config/route/UI/翻译文件必须串行预约。Hardening Task 3 前后，所有计划都必须探测当前唯一 route owner，不得重建已删除的 `handler/routes.go`。
3. 实现遵循 TDD和两阶段审查；完成当前 Task 的全部验证后，只勾选该 Task 的步骤，更新 `progress.txt` 并精确提交。
4. 本模式不修改 `task.json`、Beta roadmap 或社区计划完成状态。Desktop D-02～D-05/R-02 仍属于模式 A；不得在模式 D 复制或代替其安全任务。
5. Web Agent 真实 Provider smoke 缺少密钥时按阻塞规则记录，仓库默认开关保持关闭；确定性 fake-provider 测试仍必须完成。

### 模式选择优先级

1. 用户明确指定的任务来源或任务 ID。
2. 当前任务分支和 worktree 的既定用途。
3. 若用户要求继续 2026-05-30 公开 Beta 加固，使用模式 A。
4. 若用户要求实现 2026-06-30 社区功能计划，使用模式 C。
5. 若用户要求执行 2026-07 工程加固或 Web Agent 产品化，使用模式 D。
6. 其他未明确场景使用模式 B。
7. 无法可靠判断时，先询问用户，不要擅自改动任一任务跟踪文件。

## 项目概览

全民创意分享平台，技术栈：Next.js（前端）+ Go/Gin（后端）+ PostgreSQL + Redis + 阿里云 OSS + Tauri（PC 客户端）

详细架构：`architecture.md` | Beta 路线图：`docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md` | 社区功能设计：`docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md` | 历史任务账本：`task.json`

## 工具链版本（强制）

| 工具 | 版本 | 备注 |
|------|------|------|
| Go | 1.22+ | 后端 / Tauri sidecar |
| Node.js | 20+ | 前端 / Tauri 客户端 |
| pnpm 或 npm | pnpm 9+ / npm 10+ | 前端包管理（推荐 pnpm） |
| PostgreSQL | 16+ | 需 pgvector ≥ 0.7 |
| Redis | 7+ | — |
| Rust | 1.75+ | 仅 Tauri 客户端需要 |

CI 与本地 `go.mod` / `package.json` 的 `engines` 字段必须与上表一致。

---

## MANDATORY: Agent 工作流程

每次新 Agent 会话必须严格按以下步骤执行：

### Step 1: 确定模式并读取任务来源

**模式 A：双轨 Beta 计划集**

```bash
cat docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md
cat docs/superpowers/plans/2026-05-30-omnicraft-beta-implementation-notes.md
```

选择依赖已完成的最小 ID 未勾选任务，再读取对应子系统计划的完整任务段落。

**模式 B：历史 `task.json`**

```bash
cat task.json
```

选择 `passes: false`、依赖已完成且 ID 最小的任务。

**模式 C：2026-06-30 社区功能计划集**

```bash
cat docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md
cat docs/superpowers/plans/2026-06-30-omnicraft-community-<plan>.md
```

选择用户指定的社区计划；若未指定，按模式 C 的建议顺序选择尚未完成的最早计划。

**模式 D：项目卓越与 Web Agent 产品化**

```bash
cat docs/superpowers/plans/2026-07-08-omnicraft-project-excellence-hardening.md
# Web Agent 任务还必须读取：
cat docs/superpowers/specs/2026-07-16-omnicraft-dual-surface-agent-productization-design.md
cat docs/superpowers/plans/2026-07-16-omnicraft-web-agent-productization.md
```

只选择用户指定计划中依赖已满足的一个编号 Task。Desktop Agent 安全任务切回模式 A 执行。

- 若遇到两份文档内容矛盾 → 查阅上方文档权威源表格，以权威文档为准。将矛盾记录为 issue，**不做自行发挥**。

### Step 2: 初始化开发环境

**后端**（首次或新依赖）：
```bash
cd backend && go mod tidy
```

**前端**（首次或新依赖）：
```bash
cd frontend && npm install
```

**启动服务**（开发时）：
```bash
docker compose up -d postgres redis
cd backend && go run cmd/server/main.go &
cd frontend && npm run dev &
```

**DO NOT skip this step.** 确保相关服务运行后再实现功能。

### Step 3: 实现任务

- 仔细阅读当前模式对应任务来源中的完整任务描述、步骤、依赖和验收标准
- 参考 `architecture.md` 中对应模块的设计
- 严格遵循现有代码规范和目录结构
- 新增功能、修复缺陷和行为变更必须遵循 TDD：先写失败测试并确认预期失败，再实现最小改动
- 模式 A 的实现代理不得静默扩大写入范围；需要改动预约范围外文件时，先报告文件、符号、原因和受影响任务

**前端任务补充**：
- 实现任何页面或组件前，必须先读取 `design/ui-spec.md` 中该任务 `ui_spec_ref` 字段列出的章节
- **关键**：除 `ui_spec_ref` 外，必须主动使用 `grep` 在 `design/ui-spec.md` 中检索你将要实现的子组件名（如 `grep -A 50 "## Component: TagBadge" design/ui-spec.md`）
- 只要 `design/ui-spec.md` 中存在对应的 `## Component:` 或 `## Page:`，就必须严格遵守，它是唯一视觉权威
- 视觉实现以 `design/ui-spec.md` 为唯一视觉权威；与 steps 文字描述冲突时，**设计规格优先**
- `design/ui-spec.md` 未生成时（文件为空），跳过此步，按 steps 描述实现，完成后补充注释

**代码规范**：
- Go：`gofmt` 格式化，错误必须处理，禁止裸 panic，统一错误返回格式 `{ "code": "ERROR_CODE", "message": "..." }`
- Next.js：TypeScript strict mode，组件使用 function 声明，Tailwind CSS 样式，禁止 any 类型
- 数据库操作：所有查询使用 GORM 参数化，禁止 SQL 字符串拼接

- 若修改了 config.go / migrations / routes.go → 在提交前运行 `cd tools/doc-validator && go run . --fix`。

### Step 4: 测试（强制要求）

**UI 相关任务**（新建页面、修改核心交互）：
- **必须使用 MCP Playwright 在浏览器中测试**
- 验证页面正常加载和渲染
- 验证核心交互功能（表单提交、按钮点击）
- 截图确认 UI 正确，截图保存至screenshots文件夹

**后端 API 任务**：
- 使用 curl 或 Playwright 调用接口验证
- 验证正常路径和错误路径

**所有任务必须通过**：
- [ ] Go: `go test ./...` 无错误
- [ ] Go: `go build ./...` 无错误
- [ ] Go: `go vet ./...` 无警告
- [ ] Next.js: `npm run build` 无错误
- [ ] Next.js: `npm run lint` 无错误
- [ ] 功能在浏览器/接口测试中验证通过
- [ ] Tauri 相关任务：`npm run build`、`cargo test --manifest-path src-tauri/Cargo.toml` 无错误

### Step 5: 更新 progress.txt

```
## [YYYY-MM-DD] - Task [ID]: [任务标题]

### What was done:
- [具体修改的文件和改动]

### Testing:
- [测试方式和结果]

### Notes:
- [给未来 Agent 的重要说明]
```

- 若创建了新 .md 文件 → 放在 docs/working/ 目录下，按规范格式命名，在文件头部注明创建日期和预计失效日期；禁止在 docs/ 根目录创建新 .md 文件。

### Step 6: 更新跟踪文件并提交（一个任务一个 commit）

**模式 A：双轨 Beta 计划集**

```bash
# 1. 勾选对应子系统计划 checkbox
# 2. 勾选 roadmap checkbox
# 3. 更新 progress.txt
# 4. 精确暂存本任务文件并提交
git add <本任务精确文件列表> docs/superpowers/plans progress.txt
git commit -m "Beta [ID]: [任务标题] - completed"
```

本模式不要修改 `task.json`。任务分支合并前必须完成双重审查、rebase 和重新验证。

**模式 B：历史 `task.json`**

```bash
# 1. 更新 task.json 中该任务的 passes: false → passes: true
# 2. 更新 progress.txt
# 3. 精确暂存本任务文件并提交
git add <本任务精确文件列表> task.json progress.txt
git commit -m "Task [ID]: [任务标题] - completed"
```

**模式 C：2026-06-30 社区功能计划集**

```bash
# 1. 勾选对应社区计划 checkbox
# 2. 更新 progress.txt
# 3. 精确暂存本计划文件并提交
git add <本计划精确文件列表> docs/superpowers/plans/2026-06-30-omnicraft-community-<plan>.md progress.txt
git commit -m "Community [plan-slug]: completed"
```

本模式不要修改 `task.json`，也不要勾选 Beta roadmap。任务分支合并前必须完成双重审查、rebase 和重新验证。

**模式 D：项目卓越与 Web Agent 产品化**

```bash
# 1. 只勾选当前编号 Task 已实际完成的步骤
# 2. 更新 progress.txt
# 3. 精确暂存当前 Task 文件和对应计划，不触碰 A/B/C 跟踪状态
git add <本任务精确文件列表> <当前模式D计划文件> progress.txt
git commit -m "Productization [plan]-[task-id]: completed"
```

本模式不要修改 `task.json`、Beta roadmap 或社区计划 checkbox。真实 Provider/发布输入缺失时不得勾选 release task 或开启 feature flag。

**通用规则**：
- 只有所有步骤和验证通过后，才能更新完成状态
- 使用 `git add <精确文件列表>`，不要在脏工作区使用 `git add .`
- 永远不要删除任务、修改任务描述或从任务来源中移除记录

---

## ⚠️ 阻塞处理

**以下情况必须停止任务，输出阻塞信息，等待人工介入**：

1. **缺少外部服务配置**：阿里云 AccessKey、OSS Bucket 未创建、Redis 连接失败
2. **需要真实密钥**：内容安全 API、OSS 直传凭证
3. **数据库迁移冲突**：迁移文件执行失败
4. **Beta 发布输入缺失**：SMTP、验证码、PostgreSQL、HTTPS 证书、Allowed Origins、正式域名、法律文本版本或 Ed25519 密钥缺失
5. **多代理契约冲突**：共享文件预约冲突、无法安全 rebase、两个任务需求语义不兼容

**阻塞时禁止**：
- ❌ 提交 git commit
- ❌ 更新当前模式的完成状态（roadmap checkbox、社区计划 checkbox 或 `task.json passes`）
- ❌ 假装任务已完成

**阻塞时必须**：
- ✅ 在 progress.txt 记录当前进度和阻塞原因
- ✅ 输出清晰说明（格式如下）

```
🚫 任务阻塞 - 需要人工介入

**当前模式**: [双轨 Beta 计划集 / 历史 task.json / 2026-06-30 社区功能计划集 / 项目卓越与 Web Agent 产品化]
**当前任务**: [ID] - [标题]

**已完成的工作**:
- [列出已完成内容]

**阻塞原因**:
- [具体原因]

**需要人工操作**:
1. [步骤 1]
2. [步骤 2]

**解除阻塞后**: 需记录**阻塞已解除的原因与时间**，后运行 [命令] 继续
```

---

## 项目结构

```
OmniCraft/
├── architecture.md          # 技术架构设计（必读）
├── task.json                # 历史 MVP 任务账本
├── AGENTS.md                # 本文件
├── progress.txt             # 进度日志
├── docs/
│   ├── archive/             # 已归档文档（不再作为开发参考）
│   ├── superpowers/
│   │   ├── specs/           # 已确认的设计输入
│   │   └── plans/           # Beta roadmap、社区功能计划、子系统计划、协作说明
│   └── 2026-06-24-*.md      # 设计审查报告（问题清单 + 决策记录）
├── .specify/memory/constitution.md  # 项目宪法（设计原则和约束）
├── design/
│   ├── design-system.md     # 设计系统（色彩/字体/间距，唯一设计权威）
│   ├── ui-spec.md           # UI 规格书（页面和组件规格，前端 Task 实现必读）
│   ├── ui-design-prompt.md  # UI 设计生成提示词
│   └── doc-review-prompt.md # 文档校验提示词
├── backend/                 # Go 后端
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── handler/
│   │   ├── service/
│   │   ├── repository/
│   │   ├── model/
│   │   ├── middleware/
│   │   └── pkg/
│   ├── migrations/
│   ├── config.yaml
│   └── docker-compose.yml
├── frontend/                # Next.js 前端
│   ├── app/
│   ├── components/
│   └── lib/
├── tauri-client/            # Tauri PC 客户端
│   ├── src-tauri/
│   └── src/
└── k8s/                     # K8s 配置（P2 预留）
```

## 文件命名与存放规范

- 新建 .md 文件放在 docs/working/ 目录下，文件名格式 YYYY-MM-DD-<scope>-<type>.md
- 禁止在 docs/ 根目录创建新 .md 文件（docs/GLOSSARY.md 除外）
- 禁止创建与已有权威文档同领域的第二份文档
- 临时文档在头部注明预计失效日期（默认 +2 月）

## 常用命令

```bash
# 后端
cd backend
go run cmd/server/main.go          # 启动开发服务器
go build ./...                     # 编译检查
go vet ./...                       # 静态检查
go test ./...                      # 运行测试

# 数据库迁移
psql $DB_DSN -f migrations/001_users.sql

# 前端
cd frontend
npm run dev                        # 启动开发服务器（http://localhost:3000）
npm run build                      # 生产构建
npm run lint                       # Lint 检查

# Tauri 客户端
cd tauri-client
npm run tauri dev                  # 开发模式
npm run tauri build                # 构建安装包

# Docker
docker compose up -d               # 启动所有服务
docker compose logs -f backend     # 查看后端日志
```

---

## 关键业务规则（实现时必须遵守）

### 信誉分体系

#### 初值 & 权限阈值
- **初始值**：10 分
- **权限限制**：低于 3 分时禁止发布内容、评论、众裁、协同修改（PR）、点赞点踩（仅可浏览）
- **永久封禁**：黄赌毒内容直接永久封禁用户，不走信誉分机制

#### 加分规则（三级分类，影响从高到低）

| 分类 | 行为 | 加分 | 说明 |
|------|------|------|------|
| 创作内容相关 | 发布优质创作（获赞 ≥ 阈值） | +3 | 阈值从 config.yaml > reputation.quality_content_threshold 读取 |
| | PR 被接受合并 | +3 | 原创区无此项（无 PR 机制） |
| 评论相关 | 发布优质评论（获赞 ≥ 阈值） | +2 | 同上，从 config 读取阈值 |
| 标签相关 | 成功举报 or 添加标签被认可 | +1 | — |
| 判官相关 | 判官准确率奖励 | +1 | 投票后统计判决准确率 |
| 素质建设 | 完成素质建设课程（每门仅一次） | +1 | 最低阅读 3 分钟（180 秒）|

#### 扣分规则

| 分类 | 行为 | 扣分 | 说明 |
|------|------|------|------|
| 创作内容相关 | 发布恶意/抄袭内容 | −3 | AI 审核 block 或管理员确认 |
| | 恶意贡献（PR 篡改内容取向） | −3 | — |
| 评论相关 | 发布恶意评论 | −2 | — |
| | 恶意举报正常评论 | −2 | 举报经审核认定无问题 |
| 标签相关 | 恶意举报标签 | −1 | 举报无效 |
| 判官相关 | 判官错误 | −1 | 投票后覆盖则不扣 |

#### 「恶意」vs「正常」判定
- **标准**：以最终处理结果为准
  - 内容被审核认定违规 → 举报人为"正常举报"(+1)
  - 内容被审核认定正常 → 举报人为"恶意举报"(-N)

#### 恢复机制
- **限次素质建设课程**：信誉分低于 3 的用户通过学习对应违规类型课程恢复加分（每门课程仅一次）
- **人工申诉通道**：用户可对被下架内容提起申诉，管理员终审（Task 61、71）

#### 信誉分计算规则补充说明

**权限释放时机**：
- 信誉分从低于 3 分恢复至 ≥ 3 分时，前端立即释放被锁定功能（评论、发布、众裁、点赞等）
- 后端在更新用户信誉分后需返回新分值，前端据此更新 UI 状态

**排除条件**：
- 黄赌毒永封用户不适用信誉分恢复，`users.is_banned = TRUE` 状态下所有功能禁用（users 表使用布尔字段而非 status 枚举，详见 architecture.md §4.1）
- IP 被永久封禁（`ips.status = 'banned'`）时，关联内容全部下架，作者信誉分不因此扣分（由管理员或 AI 审核独立判定）

**特殊场景**：
- **多渠道举报同一内容**：仅第一次有效举报加分，重复举报无奖励
- **自我举报**：用户不得举报自己的内容，系统过滤此类请求
- **PR 冲突**：两个 PR 彼此冲突无法同时合并时，后合并的 PR 视为被拒绝，不得分
- **判决覆盖**：如众裁后被管理员手动覆盖要素判决，原投票中的判官错误扣分被取消

**信誉分与内容展示的关系**：
- 创作者信誉分 < 3 时，**已发布内容仍可见**，但作者**无法新建发布、无法编辑/删除已有内容**（与 PRD §6.3 权限矩阵一致）；恢复至 ≥ 3 后立即解锁
- 被标记为 'banned' 的内容所有用户均不可见，内容作者可通过申诉页面发起恢复请求

**恶意内容二次发布累计扣分（PRD §6.2）**：
- 同一用户 7 日内 ≥ 2 次内容被 AI 审核 block 或管理员判定违规 → 在标准 −3 基础上**额外扣 1 分**并冻结 7 日发布权限
- 阈值（`reputation.repeat_violation_window_days` 默认 7、`repeat_violation_threshold` 默认 2、`repeat_violation_extra_penalty` 默认 −1）从 `config.yaml` 读取，不硬编码

---

### 原创区内容分类说明

**分类体系**：单层分类（仅一级领域分类，已移除二级内容类型筛选）

- **一级分类**（按潜在用户量排序）：
  影视 | 游戏 | 文学 | 宠物 | 美食 | 美妆穿搭 | 家居 | 数码科技 | 旅行 | 运动 | 效率
  
  对应 `content_items.category` 枚举值，用户发布时自选

- **浏览体验**（小红书式推荐流）：
  - 默认进入「推荐」Tab：算法驱动的个性化推送（兴趣猜测 + 热门趋势混合），非手动筛选选项
  - 点击分类 Tab（如"美食"）→ 只看该分类下的热门内容
  - **无二级内容类型筛选**（不区分图文/视频/音频等），所有内容类型混合展示
  - 卡片采用简化设计：封面自适应高度 + 标题 + @作者 + 点赞数，无标签 Badge

- **推荐算法**（`sort=recommended`）：
  - 新用户（< 10 次互动）：纯热门趋势推荐
  - 有互动历史：个性化向量相似度（60%）+ 热门度（40%）混合排序
  - 用户画像构建：浏览历史 + 收藏（x2 权重）+ 点赞（x1.5 权重）
  - 所有参数从 `config.yaml > recommendation` 读取，严禁硬编码权重
  - 推荐引擎详细设计见 `architecture.md`  sec 12

- **发布时保留分类选择**：`content_items.category` 仍为必填，推荐算法使用该字段作为内容特征
- **`content_type` 保留用途**：记录内容格式用于附件渲染策略（MarkdownRenderer / SheetMusicViewer 等），不对外暴露为筛选维度
- 所有分类由后端 API 动态加载，管理员后台统一管理（增删改排序）

### 原创/二创来源联动规则

- 二创来源采用三选一模型：`ip_id`、`source_original_id`、`source_fanwork_id` 至少填写一个；`source_original_id` 与 `source_fanwork_id` 互斥。
- 首版来源归因仅在创建时设置；`ip_id`、`source_original_id`、`source_fanwork_id` 创建后不可变，更新请求携带任一来源字段返回 `400 SOURCE_IMMUTABLE`。
- 只有 `zone='fanwork'` 允许填写 `ip_id`、`source_original_id` 或 `source_fanwork_id`；`zone='original'` 携带任一来源字段必须拒绝。
- `source_original_id` 指向的源内容必须满足 `zone='original'` 且 `status='published'`；`source_fanwork_id` 指向的源内容必须满足 `zone='fanwork'` 且 `status='published'`。
- 旧二创内容不强制回填来源，`source_original_id = NULL` 且 `source_fanwork_id = NULL` 时不会出现在来源链列表。
- 原创详情页只在相关二创 `total > 0` 时展示「相关二创」入口；点击进入 `/original/[contentId]/fanworks`。
- 从原创详情发起二创发布时使用 `/studio/publish/fanwork?source_original_id=<id>` 预填来源原创；从二创详情发起衍生创作时使用 `/studio/publish/fanwork?source_fanwork_id=<id>` 预填来源二创。
- 内容详情 DTO 可返回 `source_original: { id, title, zone }` 或 `source_fanwork: { id, title, zone }` 摘要；来源不可见时仅返回对应 id 或 null，不渲染可点击卡片。
- 前端内容列表必须经过统一 DTO normalize，兼容 snake_case 与旧 PascalCase 字段；缺少有效 `id/title/zone` 的内容不得渲染为可点击卡片。

### 文件上传限制（从 config.yaml 读取，不要硬编码）
- 视频：≤ 300MB，时长 ≤ 180 秒
- 图片：≤ 20MB
- 文本：≤ 10MB
- Mod 包：≤ 500MB
- 乐谱：≤ 50MB，允许扩展名：mid, midi, xml, mxl, mscz, mscx, pdf

### 内容下载（Task 121）

- 下载 API：`GET /api/v1/contents/:id/download`，需认证且校验内容 `status=published`
- 下载计数：每次成功下载 `download_count + 1`（异步写入，不阻断下载流程）
- 权限控制：信誉分 < `config.yaml > reputation.min_score_for_interaction`（默认 3）用户禁止下载；封禁用户禁止下载。**所有信誉分门槛统一使用 `min_score_for_interaction`**（发布、评论、众裁、点赞、下载等），不再为下载单独定义阈值
- 前端下载按钮位于内容详情页 ReactionBar 区域，调用后端获取 OSS 临时签名 URL 后触发浏览器下载

### 收藏集（Task 122–123）

- **实现状态**：迁移 `058_create_collections.sql` 和原计划 Tasks 1–10 已完成；默认集自愈、legacy reconciliation/cutover 仍以计划 Task 11 为准，未完成前保留旧 `favorites` 兼容路径。
- 收藏集数据模型：`collections` 表（id, user_id, title, description, zone, is_default, is_public, sort_order, created_at, updated_at），`collection_items` 关联表（collection_id, content_item_id, added_at, note）
- 收藏集去重：同一收藏集内不允许添加重复内容（`collection_items` 表 UNIQUE 约束 `(collection_id, content_item_id)`）
- 收藏集筛选：支持按 `content_type` 过滤收藏集内内容
- 权限：私有收藏集仅创建者可见；公开收藏集所有用户可浏览
- 前端入口：用户主页新增「收藏集」Tab 和内容详情页「添加到收藏集」按钮

### 通知系统（Task 114–116）

- 通知自动创建：后端在评论、点赞、关注、系统公告等事件发生时自动创建 `notifications` 记录
- 通知类型枚举：`comment`、`like`、`follow`、`system`、`mention`、`appeal_result`、`content_status`
- 前端 Entry Header 通知铃铛显示未读数（`GET /api/v1/notifications/unread-count` 轮询，5 分钟间隔）
- 消息中心页面 `/messages` 包含通知列表和私信对话列表两个 Tab
- 私信 UI（Task 116）：左侧对话列表 + 右侧对话窗口；Websocket 或 SSE 实现实时消息（MVP 使用 SSE）

### 搜索增强（Task 119–120）

- 全文搜索（Task 119）：使用 PostgreSQL `tsvector` + `tsquery` 实现中文全文搜索；迁移 038 创建 `content_search_idx` GIN 索引
- 搜索建议/热搜（Task 120）：搜索建议通过 tags 和 content_items 的前缀匹配实时计算（详见 architecture.md §15.5）；热门搜索词从 Redis hot_rank 数据提取；`GET /api/v1/search/suggestions` 返回 Top 10 建议词；前端搜索框输入时下拉显示建议

### 密码重置（Task 117）

- 密码重置流程：用户输入邮箱 → 后端生成重置 token（有效期 1h，存 Redis）→ 发送含 token 的重置链接到邮箱 → 用户点击链接 → 前端 `/reset-password?token=xxx` 页面 → 输入新密码 → 提交验证 token 并更新密码
- 前端页面：`/forgot-password` 输入邮箱页 + `/reset-password` 输入新密码页

### 赛博判官
- 题库不存在时：该类型内容不开放众裁
- 考核通过线：≥ 80% 正确率
- 错误率撤权：最近 N 次（配置窗口从 config.yaml > judge.error_rate_window 读取且最小为 10；累计有效判定需 > 10）判定中错误率 > 50%，撤权 + 扣 1 信誉分
- 判决结束条件：总投票人数 ≥ 阈值（MVP 默认 20，可配置，目标 100）
- 判决结果：「不违规」比例 ≥ 60% → 恢复展示；< 60% → 有争议，不予展示（管理员可手动恢复）
- 判决详情页：投票后展示当前投票分布 + 其他判官提交的理由列表（可点赞/点踩，按赞数排序）

### 自动风控阈值（PRD §6.2，从 config.yaml > social 读取）
- 内容自动隐藏：举报数 / 点击数 ≥ `social.report_auto_hide_rate`（默认 0.10）→ status 改为 under_review，触发众裁
- 评论区折叠：点踩 / 点赞 比 ≥ `social.comment_fold_threshold`（默认 0.30）→ 自动折叠并触发审核
- 严禁硬编码 0.10 / 0.30，必须读配置

### 支付模块
- MVP 阶段：`features.payment_enabled: false`，所有支付相关 UI 不渲染，接口返回 503
- 不要删除支付相关代码，仅通过 feature flag 控制

### 创作者工作室（/studio）

> **注意**：此功能将旧的 `/publish` 和 `/dashboard/*` 路由整合为统一的创作者工作空间。旧路由保留 301 重定向，**新版以 `/studio/*` 为准**。详细设计见 `architecture.md` §13。

#### 路由迁移对照

| 旧路由 | 新路由 | 说明 |
|--------|--------|------|
| `/publish` | `/studio/publish/original` | 301 重定向 |
| `/dashboard` | `/studio/overview` | 301 重定向 |
| `/dashboard/contents` | `/studio/contents` | 301 重定向 |
| `/dashboard/pr-requests` | `/studio/pr-requests` | 301 重定向 |
| `/dashboard/contributors` | `/studio/contributors` | 301 重定向 |
| `/dashboard/tag-suggestions` | `/studio/tag-suggestions` | 301 重定向 |
| — | `/studio/publish/fanwork` | 新增 |
| — | `/studio/followers` | 新增（粉丝分析） |
| — | `/studio/revenue` | 新增（P1 预留，显示「即将开放」） |

#### 侧边栏结构（可折叠，展开 228px / 收起 48px）

- **内容发布**：发布原创 → 内容类型选择 → 发布表单；发布二创 → 内容类型选择 → 发布表单（含 IP 选择器 + 来源原创搜索器）
- **数据看板**：概览（统计卡片 + 趋势图）、内容管理、粉丝分析、收益数据（P1 占位）
- **协作管理**：PR 申请、贡献者、标签建议

#### 发布流程

1. 点击侧边栏「发布原创」或「发布二创」→ 主区域显示 ContentTypeGrid（按 `config.yaml > publish.type_order_*` 排序）
2. 选择内容类型 → 主区域切换为发布表单（zone + content_type 锁定）
3. 填写表单 → 提交 → Toast 提示 → 跳转 `/studio/contents`

#### 内容类型发布频率排序（静态预定义，从 config.yaml 读取）

- **原创区**：图文(`image`) > 纯文字(`article`) > 视频(`video`) > 音频(`audio`) > 效率模板(`template`) > 乐谱(`sheet_music`) > 其他(`other`)
- **二创区**：图文(`image`) > 纯文字(`article`) > 视频(`video`) > 音频(`audio`) > Mod(`mod`) > AI提示词(`prompt`) > 乐谱(`sheet_music`) > 其他(`other`)

#### 实施规则

- 所有 `/studio/*` 页面共享 `StudioLayout` 组件（`components/studio/StudioLayout.tsx`）
- 侧边栏折叠状态存入 `localStorage: studio_sidebar_collapsed`
- 发布表单复用现有 `/publish` 组件逻辑（FileUploader、MarkdownEditor、ComplianceCheckBadge 等）
- 粉丝分析数据来自 `GET /api/v1/users/:id/followers/stats?days=30`（新增 API）
- 收益页面在 `features.creator_support_enabled: false` 时显示占位 EmptyState

### 双端 Agent 产品化与发布边界（2026-07-16）

- Web Agent 是作品集主线能力：涉及站内事实的回答必须包含后端重新校验过的公开内容引用；无足够依据时拒答或降级普通关键词搜索，不得编造引用。
- Web Agent 默认只检索 viewer 可见的 published 内容；私有草稿、上传文件和个人数据仅在用户显式选择且业务 API 再次授权后进入上下文。
- 模型输出是不可信建议，不能直接成为权限决定或写操作；工具名、参数、调用轮数、内容可见性和预算均由服务端确定性校验。
- `agent.web_agent_enabled` 仓库默认保持 `false`；真实 Provider、原子限流/预算、引用评测、错误降级和浏览器证据通过后，生产配置才可开启。
- 当前 Tauri HMAC + WebView 直接文件命令属于禁用原型，不是可发布能力。D-02～D-05 与 R-02 未完成前必须保持 `features.desktop_deploy_enabled=false`，不得宣传客户端 Agent 可执行本地下载/配置。
- Desktop 发布后也只能执行 Ed25519 验签后的严格动作 schema；WebView/LLM 不得直接调用文件原语，敏感写/移动必须原生二次确认。
- 详细设计：`docs/superpowers/specs/2026-07-16-omnicraft-dual-surface-agent-productization-design.md`。

### Tauri 客户端文件操作白名单

客户端（`tauri-client/`）仅允许以下 7 种文件操作，其他一切操作必须拒绝：

| 操作 | 触发方 | 说明 |
|------|--------|------|
| `download_file` | 用户授权 | 下载 OSS 内容到白名单目录 |
| `extract_archive` | 用户授权 | 解压 zip（路径必须在白名单目录内） |
| `move_file` | 用户授权 | 移动文件到目标目录（白名单内） |
| `create_dir` | 用户授权 | 创建目录（白名单内） |
| `read_config` | 用户授权 | 读取游戏配置文件 |
| `write_config` | 用户授权 | 写入游戏配置文件（写前自动触发 `backup_file`） |
| `backup_file` | 系统自动 | write_config/move_file 执行前自动备份原文件到 `.omnicraft_backup/`，**禁止 LLM 直接调用** |

- 所有路径必须校验在白名单目录内（`tauri.conf.json > allowlist.fs.scope`），超出立即拒绝
- 历史实现中 Go 下发动作脚本使用 HMAC-SHA256（详见 `task.json` Task 34）；执行 Beta 桌面安全计划时，D-03 完成后必须替换为 Ed25519，客户端只能持有公钥
- Agent 工具白名单、限流速率等详细规范见 `architecture.md` §3.3 / §6.1

### 安全规则（Task 99–105）

**Task 99 — Admin Config 泄露防护**：`GET /api/v1/admin/config` 响应必须脱敏；JWT secret、OSS AccessKey/Secret、LLM API key 等敏感字段返回 `***REDACTED***`；前端 Admin 设置页不得依赖完整密钥渲染（仅显示掩码值或「已配置」状态）。

**Task 100 — CORS 策略**：生产环境 CORS `AllowedOrigins` 必须从 `config.yaml > security.allowed_origins` 读取，禁止 `*` 通配符；本地开发可使用 `localhost:3000`；中间件 `CORSMiddleware` 必须在路由注册之前挂载。

**Task 101 — Auth 实时状态检查**：所有受保护路由的 auth 中间件必须同时检查 `users.is_banned` 和 `users.role`；封禁或角色变更必须在下次请求时即时生效（不依赖客户端缓存）；前端还需在 `AuthProvider` 中添加 5 分钟间隔静默轮询 `/api/v1/auth/me` 以刷新用户状态。

**Task 102 — 错误消息脱敏**：所有 `err.Error()` 直接暴露给客户端的地方必须替换为通用错误消息；前后端共 173+ 处需逐一排查并替换；后端使用统一错误信封 `{ "code": "ERROR_CODE", "message": "用户友好描述" }`，前端使用 toast 展示错误而非 `console.error(err)` 展示原始错误给用户。

**Task 103 — 账号删除与 OSS 路径隔离**：用户删除账号时必须软删除（设置 `deleted_at`），关联内容标记为 `author_deleted`；OSS 用户目录采用 `/uploads/{user_id}/` 隔离，删除时通过生命周期规则或定时任务清理（不阻断删除操作本身）。

**Task 104 — Goroutine Panic Recovery**：所有 HTTP handler 必须通过 Gin 中间件或 `recover()` 捕获 goroutine panic，返回 500 而非让进程崩溃；后台异步任务（如 embed 写入、通知发送）也必须在各自 goroutine 内添加 `defer func() { if r := recover(); r != nil { ... } }()`。

**Task 105 — 受保护路由守卫**：前端所有 `/studio/*`、`/settings/*`、`/messages/*`、`/rehab/*`、`/judge/*`、`/admin/*` 路由必须包裹在 `(protected)` layout group 中；未登录访问时重定向到 `/login?redirect=原路径`；封禁用户访问受保护路由时展示「账号已被封禁」EmptyState 并提供申诉链接。

### 国际化（i18n）规则

- Task 108 完成后，**所有新增 UI 字符串必须通过 `next-intl` 的 `useTranslations()` 引用**，禁止硬编码中文或英文文本
- 翻译文件位于 `frontend/messages/` 目录下，按语言代码组织（`zh.json`、`en.json`）
- 新增功能页面的翻译 key 必须按页面路径命名空间组织（如 `messages.zh.notifications`）
- i18n key 忘记添加的字符视为 lint 错误，`npm run lint` 必须覆盖检查

### 结构化日志规则（Task 141）

- 后端日志必须使用 `slog`（Go 1.21+ 标准结构化日志库），禁止 `log.Printf` / `fmt.Println` 调试输出残留
- 日志格式统一 JSON，字段命名 snake_case
- 必需字段：`time`、`level`、`msg`、`trace_id`（从请求上下文注入）
- 请求日志中间件必须记录：`method`、`path`、`status`、`duration_ms`、`client_ip`
- 敏感字段（密码、token、API key）必须标记为 `redacted`，不得出现在日志明文中

### 优雅关机（Task 142）

- Go 后端必须使用 `http.Server.Shutdown(ctx)` 实现优雅关机，关机超时从 `config.yaml > server.shutdown_timeout` 读取
- 关机期间不再接受新请求，已有请求必须在超时内响应完成
- 前端 Next.js 无需特殊关机逻辑（serverless 模式）

---

## 文档权威源（冲突时以此为准）

> **完整权威文档登记表和冲突解决规则**以
> `docs/superpowers/specs/2026-06-29-omnicraft-documentation-governance-design.md` §4 为准。
> Agent 遇到文档矛盾时必须查阅该文档获取完整规则，以下为快速参考摘要。

### 权威文档快速索引

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
| 功能设计输入 | docs/superpowers/specs/*.md |
| Beta 路线图和实施计划 | docs/superpowers/plans/*.md |
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

1. **One task per session** — 专注完成一个任务
2. **Test before marking complete** — 所有 steps 验证通过才标记完成
3. **Browser testing for UI** — 新建或大幅修改页面必须浏览器测试
4. **Document in progress.txt** — 帮助后续 Agent 理解工作内容
5. **One commit per task** — 代码 + `progress.txt` + 当前模式跟踪文件放入同一个 commit
6. **Never remove tasks** — Beta 模式勾选 roadmap checkbox；社区模式勾选对应计划 checkbox；历史模式只改 `passes: false → true`；三种模式都不删任务
7. **Stop if blocked** — 需要人工介入时输出阻塞信息并停止
8. **Read config, not hardcode** — 所有限制从 config.yaml 读取
9. **Clarify before coding** — 明确假设与疑问，不擅自解读需求、不隐藏困惑
10. **Simplicity first** — 仅实现需求功能，拒绝过度设计与冗余代码
11. **Surgical changes only** — 精准修改必要代码，匹配现有风格不随意重构
12. **Clean up self-generated debris** — 仅清理自身修改产生的冗余引用 / 变量
13. **Define success criteria** — 明确验证标准，分步执行并完成校验
14. **Consult ui-spec for frontend** — 实现前端页面/组件前必须读取 `design/ui-spec.md` 对应章节（通过 `ui_spec_ref` 字段或主动 grep 检索组件名），严格遵照全局无阴影等 UI 规范。文件为空时跳过。
15. **Design spec overrides prose** — 视觉细节以 `design/ui-spec.md` 为准，优先级高于 task steps 的文字描述
16. **No raw error exposure** — 禁止将 `err.Error()` 直接返回给客户端或展示在前端 UI 上；必须使用脱敏的通用错误消息（Task 102）
17. **Protected routes require auth guard** — 所有受保护路由必须包裹在 `(protected)` layout group 中，未登录时重定向 `/login`（Task 105）
18. **Soft delete preferred** — 删除操作以软删除为主（设置 `deleted_at`），保留数据用于审计和分析；仅对确无分析价值的数据（如浏览历史、已读通知）使用物理删除（Task 103，DEC-031）
19. **Structured logging** — 后端日志统一使用 `slog` JSON 格式，禁止 `log.Printf` / `fmt.Println` 调试输出（Task 141）
20. **i18n mandatory** — 新增 UI 字符串必须通过 `next-intl` 引用，禁止硬编码中英文字符串（Task 108）
21. **Select task source first** — 会话开始先确定 Beta roadmap、社区功能计划或历史 `task.json` 模式，禁止混用完成状态
22. **TDD for behavior changes** — 新增功能、修复缺陷和行为变更必须先验证失败测试，再实现
23. **Exact staging only** — 使用 `git add <精确文件列表>`，避免把其他代理或用户改动混入提交
24. **Worktree isolation** — Beta 任务使用独立 `codex/beta/<task-id>` worktree；社区计划使用独立 `codex/community/<plan-slug>` worktree
25. **Review gates** — Beta 和社区计划合并前必须完成规格符合性审查、代码质量审查，并处理 `DONE_WITH_CONCERNS`

---

## Agent skills

### Issue tracker

Issues live in the repo's GitHub Issues, managed via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Standard five-role vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
