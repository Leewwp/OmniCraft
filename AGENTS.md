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

CI 与本地 `go.mod` / `package.json` 的 `engines` 字段必须与上表一致。CI 精确固定 Go 1.25.12 / Node 20（见 `.github/workflows/ci.yml` 与 `tauri-ci.yml`），本地只需满足上表最低版本；二者不等价。

---

## 活计划注册表（任务的唯一来源）

当前 Web-only 范围内的待办工作登记在下表。会话开始先查此表，选择**优先级最高且依赖已满足**的计划执行；用户明确指定任务时以用户指定为准。计划完成后从表中移除并归档该计划文件到 `docs/archive/plans/`。表中「执行单元」按 tracker ticket 或计划顶层 Task 计数，不计 Plan Self-Check 复核框。

| 优先级 | 计划文件 | 执行单元 | 车道 | 备注 |
|--------|----------|------|------|------|
| 1 | `docs/superpowers/specs/2026-08-07-omnicraft-web-experience-corrections-design.md`（执行追踪：GitHub issues #64/#65~#76；heavy 细化：`docs/superpowers/plans/2026-08-08-omnicraft-ip-visit-history.md`、`docs/superpowers/plans/2026-08-08-omnicraft-favorites-cutover.md`） | 12 tickets | 混合 | #65 先统一 #64/#80 与 `ui-spec` 权威；#66/#67/#70 可在其后按文件预约并行，#68 承担 C2+C6 的 ContentDetail 收敛和共享 overlay hook 后再接正式转场。媒体显示链、source-linkage/#90、collaboration-invites 已全部串行合并，#71/#69/#72 无业务硬依赖但共享文件须预约串行，三票全部完成后再执行 #73 heavy 独立迁移 `066_ip_visit_history.sql`；#74→#75→#76，#76 为 heavy `067_drop_legacy_favorites.sql` 且在云端日志+可恢复备份人工门满足前标 `ready-for-human`。改 config/routes/migrations 后跑 doc-validator `--fix` |
| 2 | `docs/superpowers/specs/2026-08-08-omnicraft-media-experience-design.md`（执行追踪：GitHub issues #80/#81~#90） | 10 tickets | 混合 | ✅ 全部完成并合并（#81~#90，PR #132/#133/#152~#156/#165）；计划已归档 `docs/archive/plans/`。媒体显示链、来源联动与相关块收口，无遗留执行单元 |
| 3 | `docs/superpowers/plans/2026-06-30-omnicraft-community-source-linkage.md`（执行追踪：GitHub issue #96） | 9 tasks | mixed | ✅ 全部完成并合并（#157~#164）；计划已归档 `docs/archive/plans/`。来源校验/契约/related-fanworks/picker/归因行收口，无遗留执行单元 |
| 3 | `docs/superpowers/specs/2026-08-08-omnicraft-content-safety-callback-fix-design.md`（执行追踪：GitHub issues #103/#104~#109；#104~#109 为 #103 原生子票） | 6 tickets | heavy | 安全修复批次：回调契约改 form+checksum（#106/#107）、删 `green.callback_allowed_ips` 加 `green.seed`/`green.uid` 配置与 release gate（#104）、`068_add_ai_review_records_task_id.sql` 幂等+banned 终态守卫（#105）、评论/讨论文本审核（#109）。内部依赖已挂原生边：#106/#107←#104、#108←#105/#106/#107；#112←#109。无跨计划业务依赖；`config.go`/`config.yaml`/`routes.go` 与 Web Agent 计划串行预约（改后跑 doc-validator `--fix`）。#110 已晋升为后续独立安全计划（优先级 6 归档扫描，父票 #110），不属本行 |
| 4 | grilling 2026-08-08 moderation 批次（执行追踪：GitHub issues #111/#112/#113；追踪：#114/#115） | 3 tickets | 混合 | #111 头像审核 heavy（OSS grant 强制 + ImageModeration + 存量迁移）；#112 私信文本审核 light（复用 #109 的 A4 分环境语义，已挂 blocked-by #109）；#113 反馈附件图片审核 light。无迁移编号占用；#111 需重试/审计语义与 #104 配置面不重叠 |
| 5 | `docs/superpowers/plans/2026-08-11-omnicraft-reliable-async-observability-foundation-roadmap.md`（执行追踪：冻结门 #134；T00/T02/T03/T08 = #135/#137/#138/#143） | 4 tickets | heavy | 全部被 Web-only 冻结门 #134 阻塞；#134 原生等待 #76/#108/#111/#112/#113。顺序：#135 治理；#137→#138；#143 等 #135/#138/#142。独立 `cmd/worker` 是唯一 Worker 模型；固定迁移 070；改 config/routes/migrations 后跑 doc-validator `--fix` |
| 5 | `docs/superpowers/plans/2026-08-11-omnicraft-rag-minimal-slice-roadmap.md`（设计输入：`docs/superpowers/specs/2026-08-11-omnicraft-rag-deepening-design.md`；执行追踪：T01/T04/T05/T06/T07/T09/T10 = #136/#139/#140/#141/#142/#144/#145） | 7 tickets | 混合 | 全部由 #134 原生直接阻塞；顺序 #136，#138→#139，#135/#138/#139→#140，#136/#140→#141，#136/#141→#142，#143 在共享文件上排 #142 后，#144 等 #136~#143，#145 只在 #144 后更新 ADR 0005且不阻塞 #32。固定迁移 069/071；T01 也是 heavy |
| 6 | `docs/superpowers/specs/2026-08-11-omnicraft-archive-malware-scanning-design.md`（执行追踪：父票 #110；最小实现 #146~#150；Future rollout #151） | 5 tickets | heavy | #146 被 #134 阻塞；#147←#146；#148←#146/#135/#138；#149←#147/#148/#108；#150←#149。ClamAV 仅 full-infra/security profile；固定迁移 072。#151 存量回扫不属于当前最小实现，已标 `deferred` 并以 `not planned` 关闭；只有用户明确恢复生产存量范围后才重开并登记为执行单元 |

跨计划可执行 DAG 以 GitHub 原生 `blocked_by` 为机器可读门，本文为人类摘要：`#65` →（`#66/#67/#70/#82` 与媒体 `#83` 按文件并行）→ `#68` 与媒体 `#81/#84/#85~#89` → source-linkage `#96`（已完成）→ 媒体 `#90`（已完成）→ collaboration-invites `#97`（已完成）→ Web `#71` → `#69` → `#72` → `#73` → `#74/#75/#76`。其中 #71/#69/#72 共享筛选、ContentDetail 与 i18n 文件，必须按该顺序串行；Web/safety 五个末端 #76/#108/#111/#112/#113 全部关闭后才关闭 #134，之后执行 #135~#150 的原生 DAG。发现本文与 GitHub 原生边不一致时先停止并修正二者，不自行选择较宽松的一方。

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
cd frontend && npm run dev &
```

**DO NOT skip this step.** 确保相关服务运行后再实现功能。

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

**以下情况必须停止任务，输出阻塞信息，等待人工介入**：

1. 缺少外部服务配置：阿里云 AccessKey、OSS Bucket、Redis 连接失败
2. 需要真实密钥：内容安全 API、OSS 直传凭证、真实 LLM Provider
3. 数据库迁移冲突：迁移文件执行失败
4. 发布输入缺失：SMTP、验证码、HTTPS 证书、Allowed Origins、正式域名、法律文本版本、Ed25519 密钥缺失
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
