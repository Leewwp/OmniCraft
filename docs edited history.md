# OmniCraft 文档修复记录

> 本次修复基于全量文档校验报告，涵盖 architecture.md、UI Design.md、task.json 三份文件。
> 修复时间：2025-01

---

## 修改总览

| 模块 | 修改文件 | 改动类型 |
|------|---------|---------|
| 1 | architecture.md | DB Schema 修复 |
| 2 | task.json | 新增任务 |
| 3 | architecture.md | 内容类型筛选对齐 |
| 4 | task.json | 响应式断点修复 |
| 5 | task.json | Task 75 修复 + depends_on 补充 |
| 7.4 | task.json | 后端优先执行依赖链 |
| 8 | UI Design.md | 关注/粉丝列表补充 |

---

## 模块 1：DB Schema 修复

### 1.1 dislike_count — 无需修复（误报）
- **文件**：architecture.md
- **结论**：`content_items` 表 DDL 已包含 `dislike_count INT NOT NULL DEFAULT 0`，原报告为误报。

### 1.2 violation_type 命名统一
- **文件**：architecture.md（§4 rehab_courses DDL 注释）
- **改动**：`malicious_report` → `malicious_report_tag`
- **原因**：与 config.yaml 和 task.json Task 63 seed 数据中的 `malicious_report_tag` 保持一致。

### 1.3 content_type 枚举 template — 无需修复（误报）
- **文件**：architecture.md
- **结论**：`content_items` 表 DDL 已包含 `template` 枚举值，原报告为误报。

### 1.4 Task 63/75 重复步骤
- **文件**：task.json（Task 75）
- **改动**：
  - 移除 Task 75 中与 Task 63 重复的步骤：`add_reason_to_judge_votes.sql`、`create_judge_reason_votes.sql`、`create_categories.sql`
  - 移除 GORM 模型中已由 Task 63 覆盖的部分：`Category`、`JudgeReasonVote`、`JudgeVote（Reason）`
  - 更新 description 明确标注"categories、judge_reason_votes、judge_votes.reason 已由 Task 63 覆盖"
  - Task 75 精简为仅包含：`add_support_info_to_users.sql`、`create_llm_configs.sql`、`LLMConfig` 模型和 `User（SupportInfo）` 扩展

---

## 模块 2：新增缺失任务

### 2.1 用户主页前端（Task 84）
- **文件**：task.json
- **改动**：新增 Task 84「Web 前端 — 用户主页」
- **内容**：实现 `/user/[userId]` 页面，含 UserProfileCard、FollowButton、FollowerListModal（关注/粉丝分页列表弹窗）、内容 Tab（发布/收藏/讨论）
- **depends_on**：[20, 4, 60]

### 2.2 浏览历史（Task 85）
- **文件**：task.json
- **改动**：新增 Task 85「Go API + Web 前端 — 浏览历史」（方案 A：完整实现）
- **内容**：含 DB 迁移（`browse_history` 表）、后端 CRUD API、前端 `/history` 页面（按日期分组展示）
- **depends_on**：[3, 20]

### 2.3 关注系统前端
- **结论**：已由 Task 84 覆盖（FollowButton + FollowerListModal），无需单独任务。

### 2.4 收藏功能前端
- **结论**：UI Design.md P08 已定义收藏为用户主页 Tab，由 Task 84 步骤 5 实现，无需单独任务。

---

## 模块 3：原创区内容类型筛选对齐

- **文件**：architecture.md（§10.6 原创区首页）
- **改动**：`全部 / 图片 / 视频 / 音频 / 文字 / 效率模板 / 乐谱 / 其他` → `全部 / 图片 / 视频 / 音频（含乐谱） / 文字 / 效率模板 / 模型与设计 / 其他`
- **原因**：与 task.json Task 64 步骤和 UI Design.md 的定义保持一致。

---

## 模块 4：响应式断点修复

- **文件**：task.json（Task 36 步骤 6）
- **改动**：`375px / 768px / 1440px 三种宽度` → `375px / 700px / 1100px / 1440px 四种宽度`
- **原因**：与 UI Design.md 定义的断点对齐（移动 ≤ 700px / 平板 ≤ 1100px / PC > 1100px）。

---

## 模块 5：Task 75 位置修正 + depends_on 补充

### 5.1 Task 75 位置
- **文件**：task.json
- **改动**：将 Task 75 从 Task 77 之后移至 Task 74 与 Task 76 之间，恢复 ID 顺序。

### 5.2 depends_on 字段补充
- **文件**：task.json
- **改动**：为 18 个有明确前置依赖的任务添加 `depends_on` 字段。

**倒序依赖（低 ID 依赖高 ID，必须修复）：**
| Task | depends_on | 说明 |
|------|-----------|------|
| 64 | [20, 69] | 原创区前端依赖内容分类后端 |
| 76 | [20, 79] | 讨论区前端依赖讨论区后端 |
| 77 | [20, 75, 80] | 创作者支持前端依赖 DB + 后端 |

**后端→后端依赖：**
| Task | depends_on | 说明 |
|------|-----------|------|
| 65 | [63] | 通知系统依赖通知表迁移 |
| 66 | [63] | 私信系统依赖私信表迁移 |
| 67 | [63] | 素质建设依赖课程表迁移 |
| 75 | [63] | 补全迁移依赖基础迁移完成 |
| 78 | [63] | 分类管理依赖分类表迁移 |
| 80 | [75] | 创作者支持后端依赖 support_info 字段 |
| 82 | [75] | LLM 配置后端依赖 llm_configs 表 |

**前端→后端依赖（确保后端先行）：**
| Task | depends_on | 说明 |
|------|-----------|------|
| 62 | [20, 44, 49] | 搜索页依赖标签搜索 + 分面侧边栏 |
| 70 | [20, 63, 65, 66] | 消息中心依赖通知 + 私信后端 |
| 71 | [20, 61] | 申诉页依赖申诉后端 |
| 72 | [20, 63, 67] | 素质建设页依赖课程后端 |
| 73 | [20, 23, 60] | IP 关注依赖 IP 详情页 + 关注后端 |
| 74 | [20, 68] | 账号设置依赖注销/改密后端 |
| 83 | [20, 75, 82] | Agent 管理页依赖 LLM 配置后端 |

---

## 模块 7.4：后端优先执行

- **方式**：通过 depends_on 字段实现，未物理重排任务 ID。
- **效果**：所有前端任务（62, 64, 70-74, 76, 77, 83, 84）均显式依赖其对应后端任务，agent 或开发者按依赖图执行时自动保证后端先行。

---

## 模块 8：UI Design.md 补充

### 8.1 收藏功能 — 无需修复
- **结论**：UI Design.md P08 已定义"收藏（瀑布流展示收藏内容）"作为用户主页 Tab，功能已覆盖。

### 8.2 关注/粉丝列表补充
- **文件**：UI Design.md（P08 用户主页）
- **改动**：关注按钮描述从 `显示关注数/粉丝数` 扩展为 `显示关注数/粉丝数（点击计数弹出关注列表/粉丝列表 Modal，支持分页加载）`
- **原因**：原定义仅有计数显示，缺少列表查看交互。

---

## 未修改项

| 模块 | 原因 |
|------|------|
| 6 (CLAUDE.md / ui-spec.md) | 用户确认不操作，ui-spec.md 后续单独编辑 |
| 7.1-7.3 (功能简化建议) | 用户确认不降级 |
| progress.txt | 校验无问题 |

---

# 第二轮文档修复（2026-04）

> 基于第二轮全量校验报告，修复阻塞问题（3 个）和警告项（约 10 个）。

## 修改总览（第二轮）

| 模块 | 修改文件 | 改动类型 |
|------|---------|---------|
| A | architecture.md | 枚举一致性修复 |
| B | architecture.md + task.json | favorites API 补全 |
| C | architecture.md + task.json | browse_history schema 对齐 |
| D | architecture.md | DDL 执行顺序修复 |
| E | architecture.md | 补全缺失索引 |
| F | architecture.md | JSONB 结构文档化 |
| G | task.json | Task 30 裁剪，消除与 Task 84/85 重复 |
| H | task.json | 迁移文件编号统一 |
| I | task.json | Task 22/24/49 细节对齐 |
| J | UI Design.md | 补充缺失组件 |

---

## 模块 A：`reputation_logs.reason` 枚举对齐

- **文件**：`architecture.md:487-491`
- **改动**：注释中的 `'malicious_report'` 改为 `'malicious_report_tag'`，并补全所有 reason 枚举值（`quality_comment` / `contribution_accepted` / `valid_report` / `judge_accuracy_bonus` / `rehab_course_completed`）
- **原因**：与 `config.yaml` §7 和 `rehab_courses.violation_type` 保持统一命名

## 模块 B：收藏 API 端点补全

### B.1 architecture.md §3.2 API 路由
- **改动**：在 social 路由块中追加
  ```
  POST   /api/v1/favorites              # 收藏内容
  DELETE /api/v1/favorites/:contentId   # 取消收藏
  GET    /api/v1/users/:id/favorites    # 用户收藏列表（分页）
  ```

### B.2 task.json Task 16
- **改动**：扩展步骤，显式说明 `internal/handler/favorite.go` 实现上述三个端点；移除重复的浏览历史职责

## 模块 C：browse_history Schema 对齐

- **文件**：`architecture.md:767-776`
- **改动**：将 browse_history 表从多态设计（`target_type` + `target_id`）改为直接 FK（`content_item_id REFERENCES content_items`），并增加 `UNIQUE(user_id, content_item_id)` 约束支持 upsert
- **原因**：与 Task 85 的实现方案对齐；PRD 和 UI 的 `/history` 页面仅覆盖内容浏览历史，多态设计是过度设计
- **同步修复**：Task 16 描述更新为"浏览历史由 Task 85 独立实现"，移除 `RecordBrowse` 步骤；Task 85 迁移编号由 030 改为 032

## 模块 D：DDL 执行顺序修复

- **文件**：`architecture.md` §4.5
- **改动**：调换 `comments` 和 `discussions` 两表的 DDL 顺序，使被引用表（discussions）先于引用表（comments）
- **原因**：`comments.discussion_id REFERENCES discussions(id)`，顺序执行时 discussions 必须先创建
- **附加改动**：为 `discussions` 表补充 `idx_discussions_ip` 和 `idx_discussions_author` 索引

## 模块 E：补全缺失索引

为下列表补全高频查询场景的索引：

| 表 | 新增索引 |
|----|----------|
| `comments` | `idx_comments_author(author_id)`, `idx_comments_parent(parent_id)` |
| `discussions` | `idx_discussions_ip(ip_id, updated_at DESC)`, `idx_discussions_author(author_id)` |
| `reports` | `idx_reports_target`, `idx_reports_reporter`, `idx_reports_status(status, created_at DESC)` |
| `pull_requests` | `idx_pr_content(content_item_id, status)`, `idx_pr_submitter(submitter_id)` |
| `ip_review_logs` | `idx_ip_review_logs_ip(ip_id, created_at DESC)` |
| `ai_review_records` | `idx_ai_review_target(target_type, target_id, scanned_at DESC)` |
| `judge_questions` | `idx_judge_questions_type_active(content_type, is_active)` |
| `judge_cases` | `idx_judge_cases_status`, `idx_judge_cases_target` |
| `judge_exam_records` | `idx_judge_exam_user_type(user_id, content_type, taken_at DESC)` |

## 模块 F：JSONB 结构文档化

- **文件**：`architecture.md:892-898`（judge_questions 表）
- **改动**：在 `question_data JSONB` 字段下补充结构注释：
  ```
  { "stem": "...", "options": [{"key":"A","text":"..."}, ...],
    "correct_key": "A", "explanation": "..." }
  ```
- **附加改动**：`ai_review_records.raw_response` 注释补充"原始响应（provider 自身格式，不做结构约定）"

## 模块 G：Task 30 裁剪（消除与 Task 84/85 的重复）

- **文件**：`task.json` Task 30
- **原问题**：Task 30 创建 `FollowButton.tsx`、`user/[userId]/page.tsx`、`history/page.tsx`，与 Task 84（用户主页）和 Task 85（浏览历史）完全重复
- **改动**：Task 30 重命名为「账号设置骨架 + 收藏按钮」，steps 精简为：
  1. 创建 `settings/page.tsx` 基础版（密码/注销由 Task 74 扩展）
  2. 在 ContentDetail 集成收藏按钮（调用 POST/DELETE /favorites）
  3. Header 下拉菜单增加各入口跳转链接
- **保留**：Task 84 作为用户主页唯一实现、Task 85 作为浏览历史唯一实现

## 模块 H：迁移文件编号统一

- **Task 75**：`add_support_info_to_users.sql` → `migrations/030_add_support_info_to_users.sql`；`create_llm_configs.sql` → `migrations/031_create_llm_configs.sql`
- **Task 85**：`migrations/030_browse_history.sql` → `migrations/032_browse_history.sql`
- **原因**：统一编号规范，避免与 Task 63 的 023-029 及 Task 75 新增的 030-031 冲突

---

## 模块 I：信誉分规则、分类体系、PR拒绝通知统一（2026-04-18）

修复时间：2026-04 | 修复内容：CLAUDE.md 信誉分规则完整化、architecture.md 分类说明统一、Task 参数澄清、PR 拒绝流程补全

### I.1 CLAUDE.md 信誉分规则完整化

- **文件**：`CLAUDE.md:255-280`（信誉分体系）
- **改动**：在「恢复机制」后补充四个新小节：
  1. **信誉分计算规则补充说明**
     - 权限释放时机：信誉分从 < 3 恢复至 ≥ 3 时，前端立即释放功能
     - 排除条件：黄赌毒永封用户不适用恢复；IP 被永封时不扣分
  2. **特殊场景**：多渠道举报、自我举报、PR 冲突、判决覆盖等 5 种边界情况
  3. **信誉分与内容展示关系**：创作者 < 3 时新发布内容仍可见但无法编辑；被标 banned 内容全部隐藏

### I.2 categories 表索引建议补充

- **文件**：`architecture.md:985-992`
- **改动**：替换原来的单一索引为分层索引建议：
  ```sql
  -- 高频查询索引
  CREATE INDEX idx_categories_zone_level ON categories(zone, level, sort_order);
  CREATE INDEX idx_categories_zone_level_active ON categories(zone, level) WHERE is_active = TRUE;
  
  -- 层级导航索引
  CREATE INDEX idx_categories_parent ON categories(parent_id);
  CREATE INDEX idx_categories_parent_active ON categories(parent_id, sort_order) WHERE is_active = TRUE;
  ```

### I.3 国际化字段规范（JSONB i18n）

- **文件**：`architecture.md:4.9`（新增）
- **新增**：统一 JSONB 结构约定
  - 短字符串：`name_i18n: { "zh-CN": "游戏", "en-US": "Gaming" }`
  - 长内容：`content_i18n: { "zh-CN": "<Markdown>", "en-US": "<Markdown>" }`
  - 扩展参数：`extra_params: { "temperature": 0.7, "max_tokens": 1000 }`
  - 应用层 fallback 策略：读取时按 `preferred_locale`（默认 'zh-CN'）访问，缺失时 fallback

### I.4 IP 被永久封禁时的关联内容处理

- **文件**：`architecture.md:4.10`（新增）
- **规则**：IP status=banned 时，所有关联二创内容自动 status=banned，但作者不扣信誉分
- **实现**：在 `ip_service.BanIP()` 中执行级联更新；用户可查询 status=banned 内容后发起申诉

### I.5 原创区内容分类说明统一

- **文件**：`architecture.md:1624-1654`
- **改动**：替换原碎片化描述为统一的**内容类型统一说明**小节：
  1. **原创区分类体系**（一级：推荐|影视|游戏|...；二级：image|video|audio|text|template|model|other）
  2. **数据库映射**（`content_items.content_type` := VARCHAR 二级 slug）
  3. **特殊处理**（表情包/动图 → image，标签筛选）
  4. **IP 分类**（二创区：gaming|film_tv|...）
  5. **分类查询规则**（原创 vs 二创区分）

### I.6 Task 69 参数描述澄清

- **文件**：`task.json` Task 69
- **改动**：
  - **description**：从"...category、content_type 筛选，best_rated/most_views 排序，IP 分类筛选"
    → "...GET /ips 增加 category、most_content 排序；GET /contents 增加 category/content_type 筛选、most_views/best_rated 排序（zone=original 时有效）"
  - **steps**：重新分组为 4 个清晰步骤：
    1. 扩展 `/ips`（category 筛选 + most_content 排序）
    2. 扩展 `/contents`（category/content_type 筛选）
    3. 扩展排序逻辑（most_views/best_rated 计算规则）
    4. 验证标准
  - 明确说明 `content_type` 值域：all|image|video|audio|text|template|model|other

### I.7 Task 14/27 PR 拒绝通知机制补全

- **文件**：`task.json` Task 14（协同 PR 引擎）
- **改动**：
  - Step 1：`RejectPR` 职责扩展为"删除 pending 版本、记录原因、**创建拒绝通知**"
  - Step 3：POST `/pr/:id/reject` 请求体新增 `rejection_reason` 参数
  - Step 5（新增）：**PR 拒绝通知机制** - RejectPR 执行时调用 `notification_service.CreateNotification(channel='pr_rejected', recipient=pr.submitter, reason=rejection_reason)`，通知包含拒绝理由文本
  - 验证步骤补充："拒绝 PR 后提交者收到包含拒绝理由的通知"

**同步**：Task 27（前端 PR 协同创作）步骤 5 本已包含"拒绝按钮 → 二次确认 Modal + 必填理由 → 自动通知"，与本次 Task 14 更新相呼应

## 模块 I：task.json 细节对齐

### I.1 Task 22 — 首页内容类型筛选补全"乐谱"
- **改动**：`全部/文字/图片/视频/音频/Mod/AI提示词/其他` → `全部/文字/图片/视频/音频/Mod/AI提示词/乐谱/其他`
- **原因**：与 architecture.md §10.6 和 content_type 枚举（含 sheet_music）对齐

### I.2 Task 24 — 追加 depends_on
- **改动**：Task 24 增加 `depends_on: [20, 51]`
- **原因**：steps 显式引用 Task 51 的 SheetMusicViewer，应声明依赖关系

### I.3 Task 49 — ui_spec_ref 前缀修正
- **改动**：`## Layout: FacetedSearchSidebar` → `## Component: FacetedSearchSidebar`
- **原因**：与 ui-spec.md 统一使用 `## Component:` 前缀的约定一致

## 模块 J：UI Design.md 组件补全

在 §3.6（社交组件）和 §3.7（判官组件）表格中追加下列漏登记的组件：

| 章节 | 新增组件 | 引用任务 |
|------|----------|----------|
| §3.6 | `UserProfileCard.tsx` | Task 84 |
| §3.6 | `FollowerListModal.tsx` | Task 84 |
| §3.6 | `CreatorSupportPanel.tsx` | Task 77 |
| §3.7 | `VerdictDetail.tsx` | Task 31 |

> `ReputationDetail.tsx` 已存在于 §3.10 rehab 组件表，无需补充。

---

## 第二轮未修复项

| 问题 | 严重程度 | 处理方式 |
|------|---------|---------|
| 报告→自动审核阈值未进入 config.yaml | 🟡 | 已在 Task 16 步骤中注明"10% 举报率"为硬编码常量，MVP 可延后提升为配置项 |
| 枚举字段 CHECK 约束（除 judge_reason_votes 外） | 🟡 | 应用层 GORM Validate Tag 已能保证，避免 CHECK 约束在枚举扩展时造成迁移负担 |
| Task 25/64 合并建议 | 🟢 | 深度改动范围大，保留现状不重排 |
| 移动端 ChatWindow / AgentChatWidget 布局细化 | 🟢 | 由 Task 36 的响应式任务整体覆盖 |
| `judge_qualifications` 增补 sheet_music/template 类型 | 🟢 | 第三轮已重新评估并修复（见模块 L） |

---

# 第三轮文档修复（2026-04-18）

> 基于第三轮全量校验报告，修复 3 个阻塞问题 + 剩余警告项。重点：回归修复（前两轮声明已修但不完整的项）。

## 模块 K：UI Design.md P01/P03 内容类型 Tab 对齐（回归修复 I.1）

- **问题**：第二轮 I.1 修复了 task.json Task 22，但 UI Design.md P01/P03 仍把"乐谱"并入音频
- **改动**：
  - `UI Design.md:38`（P01 首页内容类型 Tab）：`音频（含乐谱 / MIDI / 多音轨）` → `音频 / Mod / AI提示词 / 乐谱 / 其他`，乐谱作为独立 Tab
  - `UI Design.md:60`（P03 IP 详情页类目 Tab）：同步调整，补充说明"与首页 Tab 一致，乐谱（sheet_music）作为独立类目"

## 模块 L：judge_qualifications.content_type 扩展

- **问题**：第二轮延后决策依据"由 other 类型兜底"，但该枚举本身没有 `other` 值，乐谱/模板/其他类型内容无法进入众裁
- **改动**：
  - `architecture.md:481-483`：枚举追加 `'sheet_music' | 'template' | 'other'`，并注释"与 content_items.content_type 对齐，额外含 'comment' 用于评论众裁"
  - `task.json` Task 17 步骤 4：题库 seed 扩展"需覆盖全部 content_type 枚举，包含新增的 sheet_music / template / other"

## 模块 M：ui-spec.md `## Layout:` → `## Component:` 前缀修复（回归修复 I.3）

- **问题**：第二轮 I.3 仅改 task.json 侧，ui-spec.md 存根 `## Layout: Header` / `## Layout: FacetedSearchSidebar` 未同步，导致 Agent `grep "## Component:"` 无法命中
- **改动**：`design/ui-spec.md:24,28` 两处 `## Layout:` 全部改为 `## Component:`

## 模块 N：ui-spec.md 补全缺失组件占位

- **问题**：第二轮模块 J 在 UI Design.md §3.6/3.7 登记新组件，但未同步追加到 ui-spec.md 存根；task.json ui_spec_ref 引用 `## Component: UserProfileCard` 会找不到
- **改动**：在 `design/ui-spec.md` 末尾追加四个占位章节：
  - `## Component: UserProfileCard`
  - `## Component: FollowerListModal`
  - `## Component: CreatorSupportPanel`
  - `## Component: JudgeQualBadge`

## 模块 O：architecture.md §3.2 追加 author_blocklist API

- **问题**：Task 14 步骤引用 `POST /dashboard/contributors/:userId/block` 但 API 清单未列
- **改动**：`architecture.md:294-295` 追加：
  ```
  POST   /api/v1/dashboard/contributors/:userId/block    # 作者拉黑贡献者（写 author_blocklist）
  DELETE /api/v1/dashboard/contributors/:userId/block    # 解除拉黑
  ```
- **同步改动**：Task 14 步骤 4 补充"配套 DELETE /dashboard/contributors/:userId/block 解除拉黑"

## 模块 P：content_versions 补 is_latest 部分索引

- **文件**：`architecture.md:644-645`
- **改动**：追加 `CREATE INDEX idx_versions_content_latest ON content_versions(content_item_id) WHERE is_latest = TRUE;`
- **原因**：获取内容最新版本是高频查询

## 模块 Q：agent_messages.tool_calls JSONB 结构文档化

- **文件**：`architecture.md:1700-1703`
- **改动**：在 `tool_calls JSONB` 字段下补充注释，明确遵循 OpenAI Chat Completions `tool_calls` 数组格式

## 模块 R：discussions.content_item_id 标注 P1 预留

- **文件**：`architecture.md:692-693`
- **改动**：补充注释"content_item_id 为 P1 预留字段；P0 所有讨论均通过 ip_id 关联，content_item_id 固定为 NULL"
- **原因**：消除字段存在但无 API 使用的歧义

## 模块 S：Task 34 description 表述修正

- **文件**：`task.json` Task 34
- **改动**：`实现 6 种白名单文件操作` → `实现 7 种白名单文件操作（6 种用户触发 + backup_file 自动备份）和 HMAC 签名验证`

## 模块 T：Task 48 响应式断点回归修复（回归修复模块 4）

- **文件**：`task.json` Task 48 步骤 5
- **改动**：`瀑布流在 375/768/1440px 宽度下列数正确` → `瀑布流在 375/700/1100/1440px 宽度下列数正确（移动 2 / 平板 3 / PC 4）`
- **原因**：第一轮模块 4 只修了 Task 36，Task 48 同类问题遗漏

## 模块 U：Task 25 步骤去除无枚举支撑措辞

- **文件**：`task.json` Task 25 步骤 2
- **改动**：`支持效率模板/学习资料/3D 打印文件等特有类型` → `严格按 content_items.content_type 枚举渲染，二级导航与动态分类由 Task 64 重构`
- **原因**：content_type 枚举无 `learning`/`3d_model`，原措辞产生歧义

## 模块 V：补齐 12 个任务的 depends_on

新增依赖字段的任务：

| Task | depends_on | 说明 |
|------|-----------|------|
| 22 | [10, 11, 20] | 首页调用 IP + 内容 API |
| 23 | [10, 11, 20] | IP 详情页 |
| 24 | [11, 15, 16, 20, 51] | 内容详情需 content/comments/reactions/sheet 链路 |
| 26 | [8, 11, 20] | 发布页需 OSS Token + content API |
| 27 | [13, 14, 20] | PR 前端依赖 PR 后端 |
| 29 | [11, 14, 20] | 创作者后台 |
| 30 | [16, 20] | 收藏按钮需 favorite API |
| 31 | [17, 18, 20] | 判官前端 |
| 32 | [19, 20, 61, 78] | 管理员含分类/申诉 |
| 53 | [52] | LLM Provider 依赖 pgvector schema |
| 84 | [4, 16, 20, 60] | 增补 favorites API 依赖 |
| 85 | [3, 11, 20] | 增补 content handler 依赖 |

## 模块 W：Task 77 / 84 ui_spec_ref 补齐

- **Task 77** 追加 `## Component: CreatorSupportPanel`
- **Task 84** 追加 `## Component: FollowerListModal`

## 模块 X：Task 29/32 破坏性操作 ConfirmModal

- **Task 29**：我的内容表格"下架"操作追加 ConfirmModal 二次确认
- **Task 32**：管理员封禁内容/封禁用户/拒绝 IP/拒绝申诉 等操作均要求 ConfirmModal（输入操作原因）

## 模块 Y：UI Design.md 移动端 FacetedSearchSidebar 行为

- **文件**：`UI Design.md:576`
- **改动**：在"响应式断点"下方新增条目说明移动端（≤700px）侧边栏折叠为 shadcn/ui Sheet 抽屉（85vw，从左侧滑入）

---

## 第三轮未修复项

| 问题 | 严重程度 | 处理方式 |
|------|---------|---------|
| Task 25/64 合并建议 | 🟢 | 沿用第二轮决定，保留现状 |
| "模型与设计"二级分类无 content_type 枚举对应值 | 🟡 | Task 25 已约束严格按枚举渲染，二级分类映射由 Task 64 动态加载时用 tag 过滤解决 |
| `llm_configs.extra_params` 结构注释 | 🟢 | 保持"预留扩展参数"，MVP 不规范化 |
| CLAUDE.md 追加 Agent 白名单 / 限流速率 | 🟢 | CLAUDE.md 已引用 architecture.md §3.3 / §6.1 作为权威来源 |
| PRD P1 Agent 低代码编辑器任务占位 | 🟢 | 属 P1 迭代，暂不纳入 task.json |

