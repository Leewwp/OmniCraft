# OmniCraft 万象工坊 - Agent 工作指南

## 项目概览

全民创意分享平台，技术栈：Next.js（前端）+ Go/Gin（后端）+ PostgreSQL + Redis + 阿里云 OSS + Tauri（PC 客户端）

详细架构：`architecture.md` | 任务列表：`task.json`

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

### Step 1: 读取任务列表

```bash
cat task.json
```

选择任务标准（按优先级）：
1. 选择 `passes: false` 的任务
2. 优先选择基础设施任务（id 较小的任务通常是依赖），确保依赖已完成
3. 选择 ID 最小的未完成任务

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

- 仔细阅读 `task.json` 中该任务的 `description` 和 `steps`
- 参考 `architecture.md` 中对应模块的设计
- 严格遵循现有代码规范和目录结构

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
- [ ] Go: `go build ./...` 无错误
- [ ] Go: `go vet ./...` 无警告
- [ ] Next.js: `npm run build` 无错误
- [ ] Next.js: `npm run lint` 无错误
- [ ] 功能在浏览器/接口测试中验证通过

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

### Step 6: 提交（一个 commit 包含所有改动）

**IMPORTANT: task.json + progress.txt + 所有代码改动必须在同一个 commit**

```bash
# 1. 更新 task.json 中该任务的 passes: false → passes: true
# 2. 更新 progress.txt
# 3. 一次性提交
git add .
git commit -m "Task [ID]: [任务标题] - completed"
```

**规则**：
- 只有所有 steps 验证通过才标记 `passes: true`
- 永远不要删除或修改任务描述
- 永远不要从列表中移除任务

---

## ⚠️ 阻塞处理

**以下情况必须停止任务，输出阻塞信息，等待人工介入**：

1. **缺少外部服务配置**：阿里云 AccessKey、OSS Bucket 未创建、Redis 连接失败
2. **需要真实密钥**：内容安全 API、OSS 直传凭证
3. **数据库迁移冲突**：迁移文件执行失败

**阻塞时禁止**：
- ❌ 提交 git commit
- ❌ 将 task.json 的 passes 设为 true
- ❌ 假装任务已完成

**阻塞时必须**：
- ✅ 在 progress.txt 记录当前进度和阻塞原因
- ✅ 输出清晰说明（格式如下）

```
🚫 任务阻塞 - 需要人工介入

**当前任务**: Task [ID] - [标题]

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
├── task.json                # 任务列表（工作来源）
├── CLAUDE.md                # 本文件
├── progress.txt             # 进度日志
├── .specify/memory/constitution.md  # 项目宪法（设计原则和约束）
├── design/
│   └── ui-spec.md           # UI 设计规格（Gemini 生成，前端 Task 实现必读）
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

- 第一版采用单来源模型：每个二创最多绑定一个原创内容，通过 `content_items.source_original_id` 指向源原创。
- 只有 `zone='fanwork'` 允许填写 `source_original_id`；`zone='original'` 携带该字段必须拒绝。
- 被绑定的源内容必须满足 `zone='original'` 且 `status='published'`；不存在、非原创、未发布内容都返回 400。
- 旧二创内容不强制回填来源，`source_original_id = NULL` 时不会出现在相关二创列表。
- 原创详情页只在相关二创 `total > 0` 时展示「相关二创」入口；点击进入 `/original/[contentId]/fanworks`。
- 从原创详情发起二创发布时使用 `/publish?zone=fanwork&source_original_id=<id>` 预填来源原创。
- 前端内容列表必须经过统一 DTO normalize，兼容 snake_case 与旧 PascalCase 字段；缺少有效 `id/title/zone` 的内容不得渲染为可点击卡片。

### 文件上传限制（从 config.yaml 读取，不要硬编码）
- 视频：≤ 300MB，时长 ≤ 180 秒
- 图片：≤ 20MB
- 文本：≤ 10MB
- Mod 包：≤ 500MB
- 乐谱：≤ 50MB，允许扩展名：mid, midi, xml, mxl, mscz, mscx, pdf

### 赛博判官
- 题库不存在时：该类型内容不开放众裁
- 考核通过线：≥ 80% 正确率
- 错误率撤权：最近 N 次（N > 10）判定中错误率 > 50%，撤权 + 扣 1 信誉分
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

#### 侧边栏结构（可折叠，展开 w-56 / 收起 w-16）

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
- Go 下发的动作脚本必须附带 HMAC-SHA256 签名（详见 `task.json` Task 34）
- Agent 工具白名单、限流速率等详细规范见 `architecture.md` §3.3 / §6.1

---

## Key Rules

1. **One task per session** — 专注完成一个任务
2. **Test before marking complete** — 所有 steps 验证通过才标记完成
3. **Browser testing for UI** — 新建或大幅修改页面必须浏览器测试
4. **Document in progress.txt** — 帮助后续 Agent 理解工作内容
5. **One commit per task** — 代码 + progress.txt + task.json 同一个 commit
6. **Never remove tasks** — 只改 passes: false → true，不删任务
7. **Stop if blocked** — 需要人工介入时输出阻塞信息并停止
8. **Read config, not hardcode** — 所有限制从 config.yaml 读取
9. **Clarify before coding** — 明确假设与疑问，不擅自解读需求、不隐藏困惑
10. **Simplicity first** — 仅实现需求功能，拒绝过度设计与冗余代码
11. **Surgical changes only** — 精准修改必要代码，匹配现有风格不随意重构
12. **Clean up self-generated debris** — 仅清理自身修改产生的冗余引用 / 变量
13. **Define success criteria** — 明确验证标准，分步执行并完成校验
14. **Consult ui-spec for frontend** — 实现前端页面/组件前必须读取 `design/ui-spec.md` 对应章节（通过 `ui_spec_ref` 字段或主动 grep 检索组件名），严格遵照全局无阴影等 UI 规范。文件为空时跳过。
15. **Design spec overrides prose** — 视觉细节以 `design/ui-spec.md` 为准，优先级高于 task steps 的文字描述

---

## Agent skills

### Issue tracker

Issues live in the repo's GitHub Issues, managed via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Standard five-role vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
