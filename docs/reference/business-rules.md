# OmniCraft 业务规则参考

> **来源**：2026-07-23 文档瘦身从 `AGENTS.md`「关键业务规则」章节原样抽取。
> **用法**：实现某子系统前，必须先完整阅读本文件对应小节。检索方式：
> `grep -A 40 "### 信誉分体系" docs/reference/business-rules.md`。
> **覆盖范围**：信誉分体系、原创区内容分类、原创/二创来源联动、文件上传、内容下载、
> 收藏集、通知系统、搜索增强、密码重置、赛博判官、自动风控、支付模块、创作者工作室（/studio）、
> 双端 Agent 产品化边界、Tauri 文件操作白名单、安全规则（Task 99-105）、i18n、结构化日志、优雅关机。

---

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
- 黄赌毒永封用户不适用信誉分恢复，`users.is_banned = TRUE` 状态下所有功能禁用（users 表使用布尔字段而非 status 枚举，详见 docs/reference/schema.md §4.1）
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
  - 推荐引擎详细设计见 `docs/specs/recommendation-page.md`

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
- 搜索建议/热搜（Task 120）：搜索建议通过 tags 和 content_items 的前缀匹配实时计算（详见 docs/reference/implementation-notes.md §15.5）；热门搜索词从 Redis hot_rank 数据提取；`GET /api/v1/search/suggestions` 返回 Top 10 建议词；前端搜索框输入时下拉显示建议

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
