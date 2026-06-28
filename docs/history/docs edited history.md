# OmniCraft 文档修复记录

> 本次修复基于全量文档校验报告，涵盖 architecture.md、UI Design.md、task.json 三份文件。
> 修复时间：2026-01

> 📌 迁移编号信息截至 035（2026-04）。当前实际迁移已至 055+。最新编号见 `architecture.md` §15.9。

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
| "模型与设计"二级分类无 content_type 枚举对应值 | 🟡 | Task 25 已约束严格按枚举渲染，二级分类映射由 Task 64 动态加载时用 tag 过滤解决（第四轮已完整落实，见模块 AB） |
| `llm_configs.extra_params` 结构注释 | 🟢 | 保持"预留扩展参数"，MVP 不规范化 |
| CLAUDE.md 追加 Agent 白名单 / 限流速率 | 🟢 | 第四轮已补齐 Tauri 白名单与工具链版本表（见模块 AC） |
| PRD P1 Agent 低代码编辑器任务占位 | 🟢 | 属 P1 迭代，暂不纳入 task.json |

---

# 第四轮文档修复（2026-04-20）

> 基于第四轮全量校验报告，修复 2 个 🔴 阻塞问题 + 10 项 🟡 警告。重点：跨文档枚举冲突、通知机制命名冲突、迁移文件缺失、depends_on 持续补齐。

## 修改总览（第四轮）

| 模块 | 修改文件 | 改动类型 |
|------|---------|---------|
| AA | architecture.md | 🔴 `web_agent_enabled` 配置路径统一到 `agent.*` |
| AB | task.json + architecture.md | 🔴 Task 14 PR 拒绝通知 channel/type 语义修正 |
| AC | architecture.md + task.json | 🟡 原创区"推荐"伪分类 + "模型与设计" tag 过滤方案 |
| AD | CLAUDE.md | 🟡 工具链版本锁定 + Tauri 白名单完整 7 操作 |
| AE | task.json | 🟡 补齐 18 个任务 depends_on |
| AF | task.json | 🟡 Task 67 去重 rehab seed + Task 11 去重 video 截帧 |
| AG | task.json | 🟡 Task 60/61 补建表迁移（033_follows / 034_appeals） |
| AH | design/ui-spec.md + task.json | 🟡 删除重复 Header 节点 + Task 30 `ui_spec_ref` 裁剪 |
| AI | task.json | 🟡 Task 45 标签建议速率限制 |
| AJ | architecture.md | 🟡 `content_items.category` 应用层校验说明 |

---

## 模块 AA：🔴 `web_agent_enabled` 配置路径冲突修正

- **文件**：`architecture.md:1653`
- **问题**：§11.1 写作 `features.web_agent_enabled`，但 §11.2 / `config.yaml` 示例 / `task.json` Task 52 均把该开关置于 `agent:` 顶层节。开发者按 §11.1 从 `features.*` 读取会得到 `nil`，功能开关失效。
- **改动**：
  - 原：``Feature Flag 控制：`features.web_agent_enabled: false`（默认关闭，独立于 Tauri Agent）``
  - 新：``Feature Flag 控制：`agent.web_agent_enabled: false`（默认关闭，独立于 Tauri `features.agent_enabled`；配置位置见 §11.3）``

## 模块 AB：🔴 Task 14 PR 拒绝通知 channel/type 语义冲突

- **文件**：`task.json` Task 14 step 5
- **问题**：第三轮模块 I.7 补全时写作 `CreateNotification(channel='pr_rejected', ...)`；但 Task 65 定义的 `notifications.channel` 枚举仅为 `reply | like | system`，PR 通知按 Task 65 归入 `channel=system`。两处直接冲突，后端若按 Task 14 实现会把 `pr_rejected` 写入 channel 字段，导致消息中心按 channel 过滤时该通知不显示。
- **改动**（Task 14 step 5）：
  - 原：`CreateNotification(channel='pr_rejected', recipient=pr.submitter, reason=rejection_reason)`
  - 新：`CreateNotification(type='pr_rejected', channel='system', recipient=pr.submitter, reason=rejection_reason)`（并补充说明：channel 必须为 reply/like/system 枚举之一，`pr_rejected` 是 `notifications.type` 值）

## 模块 AC：原创区"推荐"伪分类 + "模型与设计" tag 过滤方案

### AC.1 architecture.md §10.6 完整重写内容类型映射
- **文件**：`architecture.md:1623-1642`
- **改动要点**：
  - 明确 11 个落库 `category` 枚举值（不含"推荐"）
  - 新增说明：`推荐` 为**前端伪分类**，不落库 `category`；选中时调用 `GET /contents?zone=original&sort=recommended`（不传 `category`）
  - 新增 **UI 二级 Tab → content_type 映射表**：
    | Tab | 查询策略 |
    |-----|---------|
    | 音频（含乐谱） | `content_type IN ('audio','sheet_music')` |
    | 文字 | `content_type IN ('article','prompt')` |
    | 模型与设计 | 不传 `content_type`，改传 `tag=3D模型` 或 `tag=设计素材`（无对应枚举值） |
    | 其他 Tab | 直接对应枚举值 |

### AC.2 Task 64 step 2/3 对齐映射表
- **文件**：`task.json` Task 64 step 2, step 3
- **改动**：显式注明"推荐"的 sort=recommended 调用方式，以及二级 Tab 与后端参数的精确映射，禁止前端传 `content_type=model`

### AC.3 Task 69 step 2 支持 content_type 多值 IN 查询
- **文件**：`task.json` Task 69 step 2
- **改动**：`content_type` 参数支持逗号分隔多值（`audio,sheet_music` 或 `article,prompt`），后端解析为 IN 查询；`model` 值由前端映射为 tag 参数，本接口不处理

## 模块 AD：CLAUDE.md 工具链版本 + Tauri 白名单

### AD.1 顶部新增「工具链版本（强制）」表格
- **文件**：`CLAUDE.md:9-20`
- **新增**：Go 1.22+ / Node 20+ / pnpm 9+ / PostgreSQL 16+ (pgvector ≥ 0.7) / Redis 7+ / Rust 1.75+（仅 Tauri）
- **原因**：消除 CI 与本地环境漂移风险

### AD.2 「支付模块」后新增「Tauri 客户端文件操作白名单」小节
- **文件**：`CLAUDE.md:332-348`
- **新增**：完整列出 7 种白名单操作（6 种用户授权 + `backup_file` 系统自动触发），标注路径校验、HMAC 签名要求、权威来源（architecture.md §3.3 / §6.1）
- **关键约束**：`backup_file` 禁止 LLM 直接调用，只能作为 `write_config`/`move_file` 的前置副作用

## 模块 AE：补齐 18 个任务 depends_on

| Task | 新增 depends_on | 说明 |
|------|-----------------|------|
| 44 | [43] | 标签分面搜索依赖 tags 表迁移 |
| 45 | [43] | 标签建议/标签组依赖 015-018 迁移 |
| 46 | [20] | 全量 i18n 依赖 next-intl 初始化 |
| 47 | [20] | 设计系统补全依赖 Task 20 token 扩展 |
| 48 | [20, 22] | 封面图系统依赖 ContentCard |
| 49 | [20, 44] | FacetedSidebar 调用 `/tags/faceted` |
| 50 | [20, 45, 49] | 标签组 UI 依赖标签组后端 + 侧边栏 |
| 51 | [20] | SheetMusicViewer 依赖前端初始化 |
| 52 | [3] | pgvector schema 在 users 表之后 |
| 54 | [52, 53] | upload-assist 依赖 LLM Provider |
| 55 | [9, 52, 53] | compliance-check 依赖 Aliyun Green + LLM |
| 56 | [11, 52, 53] | NL 搜索依赖 content + embedding |
| 57 | [11, 52, 53] | usage-guide 依赖 content + LLM |
| 58 | [9, 52, 53] | moderate 依赖 Aliyun + LLM |
| 59 | [20, 54, 55, 56, 57, 58] | 前端 Agent 集成依赖全部后端 API |
| 60 | [3, 4] | follows 表依赖 users/ips |
| 61 | [3, 11] | appeals 依赖 users/content |
| 68 | [6, 7] | 注销/改密依赖 auth + user 基础 |
| 79 | [5, 15, 63] | 讨论区后端依赖社交表 + 评论基础 + 讨论迁移 |
| 81 | [3, 7] | 信誉分日志依赖 users + reputation 基础 |

## 模块 AF：Task 67 / Task 11 去重

### AF.1 Task 67 step 5 去除重复 rehab seed
- **问题**：Task 63 step 4 已创建 `migrations/026_rehab_seed.sql`，Task 67 step 5 又新建 seed 迁移 → 编号冲突或重复 INSERT
- **改动**：Task 67 step 5 由"创建 migrations/ 初始 seed 数据"改为"校验 Task 63 `026_rehab_seed.sql` 已生效（本任务不新建迁移）；如需改进课程内容，通过后台 content_i18n 扩展 API 更新而非新增迁移"

### AF.2 Task 11 step 10 去除重复 video 截帧
- **问题**：Task 11 step 10 与 Task 48 step 3 均实现"video 发布后异步截帧写入 cover_image_url"
- **改动**：Task 11 对应步骤由实现动作改为备注"注：video 封面异步截帧写入 cover_image_url 由 Task 48 step 3 统一实现，本任务不重复"，保留单一实现位置

## 模块 AG：Task 60/61 补建表迁移

### AG.1 Task 60 补 `migrations/033_follows.sql`
- **问题**：`architecture.md` §4.5 DDL 已定义 `follows` 表，但 `task.json` 无任何 Task 创建对应迁移文件，Task 60 后端代码将在 DB 未建表情况下运行失败
- **改动**：Task 60 step 1 前插入"创建迁移文件 migrations/033_follows.sql" 步骤，完整 DDL 含 `CHECK(target_type IN ('user','ip'))`、`UNIQUE(follower_id, target_type, target_id)` 与两条索引

### AG.2 Task 61 补 `migrations/034_appeals.sql`
- **问题**：同上，`appeals` 表在架构文档有 DDL 但无对应迁移任务
- **改动**：Task 61 step 1 前插入"创建迁移文件 migrations/034_appeals.sql" 步骤，含 `target_type` / `status` CHECK 约束和两条索引

**迁移编号更新**：本轮新增 033/034 接续第二轮模块 H 结束的 032（browse_history），编号仍连续。后续新表应从 035 起。

## 模块 AH：ui-spec.md 重复节 + Task 30 `ui_spec_ref`

### AH.1 删除 ui-spec.md 第二个 `## Component: Header`
- **文件**：`design/ui-spec.md`（原 L168）
- **问题**：顶部全局组件区 L24 已有 `## Component: Header`，§3.6 附近再次出现同名节，Agent `grep` 命中两处生成内容混淆
- **改动**：删除第二个 `## Component: Header` 占位节

### AH.2 Task 30 `ui_spec_ref` 移除 `## Component: ContentDetail`
- **问题**：Task 30 模块 G 裁剪后仅做账号设置骨架 + 收藏按钮嵌入，不应重写 ContentDetail 整体规格（那是 Task 24 职责）
- **改动**：`ui_spec_ref` 从 `["## Page: /settings 账号设置", "## Component: ContentDetail"]` 精简为 `["## Page: /settings 账号设置"]`

## 模块 AI：Task 45 标签建议速率限制

- **文件**：`task.json` Task 45
- **新增步骤**（插入到 step 3 之后）："速率限制：POST /contents/:id/tags/suggest 通过 Redis 令牌桶限流「每用户每内容每日最多 10 条」（key: `tag_suggest:{user_id}:{content_id}:{date}`），超限返回 429 `TAG_SUGGEST_RATE_LIMIT_EXCEEDED`，防止恶意刷屏骚扰创作者"
- **原因**：原去重规则（同 tag+action 不重复）不能阻止攻击者对同内容刷大量不同 tag 的建议

## 模块 AJ：`content_items.category` 应用层校验说明

- **文件**：`architecture.md:569`
- **新增注释**：`-- 应用层约束（GORM Validate Tag）：zone='original' 时 category 必填且须为上述枚举；zone='fanwork' 时 category 须为 NULL。枚举扩展时在 config/categories 种子数据中补充，不加 DB CHECK 约束（避免迁移负担）`
- **原因**：原 DDL 仅注释列举枚举值，缺少"与 zone 联动"与"扩展策略"的明确约束说明

---

## 第四轮未修复项

| 问题 | 严重程度 | 处理方式 |
|------|---------|---------|
| Prometheus/OTel 可观测性任务 | 🟢 | 纳入 P1 迭代（部署前追加 1 个任务），MVP 不阻塞 |
| Task 43 + Task 52 schema 扩展合并 | 🟢 | 当前独立任务粒度更利于 Agent 执行，保留 |
| Task 55 + Task 58（合规检测 + 内容初审）合并 | 🟢 | 业务入口不同（发布前 vs 发布后），保留独立 |
| pgvector IVFFlat `lists=100` MVP 数据量过度设计 | 🟢 | 维持当前参数，数据量上量后再优化 |
| Redis sorted set 热榜缓存 | 🟢 | MVP 访问量下 PG 索引足够，保留为可选优化 |

---

## 第四轮校验总结

- **🔴 阻塞修复**：2 / 2 完成（AA `web_agent_enabled` 路径、AB PR 通知 channel/type）
- **🟡 警告修复**：10 / 10 完成（AC ~ AJ）
- **🟢 优化建议**：全部记录于"未修复项"，按优先级留待 P1/P2 迭代

**一致性结论**：所有文档在完成本轮修复后，经交叉校验**无阻塞问题**。可进入下一步开发。

---

# 第五轮文档修复（2026-04-20）

> 基于第五轮全量校验报告，修复 9 个 🟡 警告项。重点：CLAUDE.md 字段命名/权限矩阵自洽、PRD §6.2 二次违规规则落地、补齐 5 个任务依赖、Task 76 搜索框、conversations/messages 索引补全。

## 修改总览（第五轮）

| 模块 | 修改文件 | 改动类型 |
|------|---------|---------|
| AK | CLAUDE.md | 🟡 `users.status` → `users.is_banned` 字段名修正 |
| AL | CLAUDE.md | 🟡 信誉分 < 3 时内容展示规则与 PRD §6.3 权限矩阵对齐 |
| AM | CLAUDE.md + task.json + architecture.md | 🟡 PRD §6.2 恶意内容二次发布累计扣分规则落地 |
| AN | task.json | 🟡 Task 22/23/64 补 `depends_on` 69（best_rated/most_views）+ 78（动态分类 API） |
| AO | task.json | 🟡 Task 50 补 `depends_on` 29（dashboard 概览卡片由 29 创建） |
| AP | task.json | 🟡 Task 30 `ui_spec_ref` 追加 `## Component: Header`（Header 下拉菜单改动） |
| AQ | task.json | 🟡 Task 76 补讨论区列表搜索框步骤（UI Design.md P04a 工具栏） |
| AR | architecture.md + task.json | 🟡 conversations.updated_at + messages.sender_id 索引补全（迁移 035） |

---

## 模块 AK：CLAUDE.md `users.status` → `users.is_banned` 字段名修正

- **文件**：`CLAUDE.md`「排除条件」段
- **问题**：CLAUDE.md 写作 `users.status = 'banned'`，但 `architecture.md:469-470` users 表使用 `is_banned BOOLEAN` + `ban_reason TEXT`，**无 status 列**。开发者按 CLAUDE.md 实现会编译失败
- **改动**：`users.status = 'banned'` → `users.is_banned = TRUE`，并补充注释指明 users 表使用布尔字段而非 status 枚举（content_items / ips 才使用 status='banned'）

## 模块 AL：信誉分 < 3 内容展示规则自洽

- **文件**：`CLAUDE.md`「信誉分与内容展示的关系」
- **问题**：原文「创作者信誉分 < 3 时，新发布内容仍可显示」与 PRD §6.3「低于 3 分：仅浏览内容，禁止发布」语义自相矛盾
- **改动**：改为「**已发布内容仍可见**，但作者**无法新建发布、无法编辑/删除已有内容**（与 PRD §6.3 权限矩阵一致）；恢复至 ≥ 3 后立即解锁」

## 模块 AM：恶意内容二次发布累计扣分规则落地（PRD §6.2）

PRD §6.2 第 3 条「恶意内容二次发布 → 直接扣除信誉分，限制发布权限」原本无任何 task 显式承载，本轮全链路落地。

### AM.1 CLAUDE.md「信誉分计算规则补充说明」新增子节
- 同一用户 `repeat_violation_window_days`（默认 7）日内 ≥ `repeat_violation_threshold`（默认 2）次 block/violation → 标准 −3 基础上**额外扣 1 分**并冻结 7 日发布权限
- 阈值/惩罚/冻结时长全部从 `config.yaml` 读取，禁止硬编码

### AM.2 task.json Task 9 step 5/6 扩展
- 新 step 5：`ProcessAICallback` 处理 block/violation 后查询滑动窗口违规计数，达阈值时调用 `reputation_service.AddReputation(delta=-1, reason='repeat_violation')`，并在 Redis 写入 `publish_freeze:{user_id}` 7 日过期键；`content_service.PublishContent` 入口检测此键存在则返回 `403 PUBLISH_FROZEN`
- 验证 step 补充「连续两次违规后信誉分额外扣 1 分、发布接口返回 PUBLISH_FROZEN」

### AM.3 architecture.md §7 reputation 配置项扩展
- `reputation` 节追加 4 个配置：`repeat_violation_window_days: 7` / `repeat_violation_threshold: 2` / `repeat_violation_extra_penalty: -1` / `publish_freeze_seconds: 604800`

## 模块 AN：Task 22/23/64 补 `depends_on`

| Task | 原 depends_on | 新 depends_on | 原因 |
|------|--------------|---------------|------|
| 22 | [10, 11, 20] | [10, 11, 20, 69, 78] | 步骤要求「最高好评率」排序（Task 69）和分类 Tab 动态加载（Task 78） |
| 23 | [10, 11, 20] | [10, 11, 20, 69, 78] | IP 详情页类目排序同样依赖 Task 69 的 best_rated/most_views |
| 64 | [20, 69] | [20, 69, 78] | 原创区两级导航需调用 `GET /categories?zone=original`（Task 78），并新增 step 1.5 显式说明动态加载逻辑 |

## 模块 AO：Task 50 补 `depends_on` 29

- **问题**：Task 50 step 7 修改 `app/(protected)/dashboard/page.tsx` 概览卡片新增「待审标签建议」入口，但 dashboard 页面在 Task 29 才创建，Task 50 原 `depends_on:[20,45,49]` 缺 29
- **改动**：`depends_on` 改为 `[20, 29, 45, 49]`

## 模块 AP：Task 30 `ui_spec_ref` 追加 Header 组件

- **问题**：Task 30 step 3 在 Header 下拉菜单增加 4 个跳转入口，但 `ui_spec_ref` 仅 `["## Page: /settings 账号设置"]`，Agent grep `## Component: Header` 时无法关联到 Task 30
- **改动**：`ui_spec_ref` 追加 `"## Component: Header"`

## 模块 AQ：Task 76 补讨论区列表搜索框步骤

- **问题**：UI Design.md P04a 工具栏明确含「右侧讨论区内搜索框（搜索标题+内容）」，Task 79 后端已实现 `GET /ips/:id/discussions/search?q=`，但 Task 76 步骤列表无对应实现，前端会少功能
- **改动**：Task 76 在创建讨论区列表页步骤后插入新步骤：「工具栏右侧增加搜索输入框，本地节流 300ms 后调用 `GET /ips/:id/discussions/search?q=`，q 为空返回默认列表，无结果显示「未找到相关讨论」EmptyState」

## 模块 AR：conversations / messages 索引补全

### AR.1 architecture.md §4.5 DDL 补全
- 在 `messages` 表 DDL 后追加：
  - `CREATE INDEX idx_messages_sender ON messages(sender_id);`（支撑「我发送的全部私信」/ 举报溯源场景）
  - `CREATE INDEX idx_conversations_updated ON conversations(updated_at DESC);`（支撑 `GetConversationsByUser` 按 updated_at 排序的高频查询）

### AR.2 task.json Task 66 step 1 新增迁移
- Task 66 步骤列表在最前插入：「创建迁移文件 `migrations/035_conversation_indexes.sql`：补充 `idx_conversations_updated` + `idx_messages_sender` 两条索引（与 architecture.md §4.5 对齐）」
- 迁移编号 035 接续第 4 轮模块 AG 结束的 034（appeals），编号连续

---

## 第五轮未修复项

| 问题 | 严重程度 | 处理方式 |
|------|---------|---------|
| `oauth_accounts.access_token` 加密注释 | 🟢 | P1 启用 OAuth 时补充，MVP 暂不启用 |
| `/judge/queue` 跳过本案例操作 | 🟢 | 优化项，纳入 P1 判官体验迭代 |
| 移动端 ChatWindow 键盘弹出布局 | 🟢 | 沿用第 2 轮决定，由 Task 36 整体覆盖 |
| `browse_history` 90 天自动清理 | 🟢 | 部署前在 `config.yaml > limits` 增加 `browse_history_retention_days: 90` 与定时任务，本轮不动 |
| Prometheus / OTel 任务 | 🟢 | 沿用第 4 轮决定，纳入 P1 |

---

## 第五轮校验总结

- **🔴 阻塞修复**：0 / 0（本轮无阻塞问题）
- **🟡 警告修复**：9 / 9 完成（AK ~ AR）
- **🟢 优化建议**：全部记录于「未修复项」，按优先级留待 P1 处理

**一致性结论**：所有文档在完成第五轮修复后，经交叉校验**无阻塞问题、无遗漏功能、无依赖倒序**。可进入下一步开发。

---

# 第六轮文档修复（2026-04-20）

> 基于第六轮全量校验报告，修复 4 项 🟡 警告（W1-W4）。

## 修改总览（第六轮）

| 模块 | 修改文件 | 改动类型 |
|------|---------|---------|
| AS | architecture.md | 🟡 §3.4 三级审核流程图措辞与 PRD §6.1/CLAUDE.md 对齐 |
| AT | architecture.md + CLAUDE.md + task.json | 🟡 补全 `social.report_auto_hide_rate` / `comment_fold_threshold` 配置项及引用 |
| AU | task.json | 🟡 补全 13 个任务 `depends_on`（21/25/28/36/37/38/39/40/41/42/43/63/69） |
| AV | architecture.md | 🟡 §4.5 私信表索引创建时机注释明确 |

---

## 模块 AS：审核链路措辞统一（W1）

- **文件**：`architecture.md:435-436`
- **问题**：原作 `支持率 ≥ 60% → 上架` / `< 60% → 永久下架 → 可申诉`，与 PRD §6.1「不予展示（争议内容可由管理员手动恢复）」与 `CLAUDE.md:329` 措辞冲突，开发者会误解为「永久下架」不可逆
- **改动**：
  - 原：`支持率 ≥ 60% → 上架` / `支持率 < 60% → 永久下架 → 可申诉`
  - 新：`「不违规」比例 ≥ 60% → 恢复展示` / `「不违规」比例 < 60% → 不予展示（管理员可手动恢复）→ 可申诉`

## 模块 AT：social 配置项补全（W2）

### AT.1 architecture.md §7 新增 social 节
- **文件**：`architecture.md:1223-1225`
- **新增**：
  ```yaml
  social:
    report_auto_hide_rate: 0.10        # PRD §6.2 自动隐藏阈值
    comment_fold_threshold: 0.30       # PRD §6.2 评论区折叠阈值
  ```

### AT.2 CLAUDE.md「自动风控阈值」速查
- **文件**：`CLAUDE.md:332-335`（赛博判官与支付模块之间新增）
- **新增**：明确「严禁硬编码 0.10 / 0.30」并指明配置 key

### AT.3 task.json Task 16 step 2/5/6 引用配置
- **文件**：`task.json` Task 16
- **改动**：
  - step 2 内 `检查是否达到自动隐藏阈值 10% 举报率` → `检查是否达到自动隐藏阈值 social.report_auto_hide_rate`
  - step 5 `举报数/点击率 ≥ 10%` → `举报数/点击数 ≥ 阈值（config.yaml > social.report_auto_hide_rate，默认 0.10）`
  - step 6 已使用配置项 key，本轮补充默认值说明

## 模块 AU：13 个任务 depends_on 补全（W3）

| Task | 新 depends_on | 依据 |
|------|--------------|------|
| 21 | `[6, 20]` | 登录注册页调用 /auth/* |
| 25 | `[11, 20, 69, 78]` | 与 22/23 同形态，原创区前端 |
| 28 | `[12, 20]` | 版本历史调用 Task 12 版本管理 |
| 36 | `[20, 22, 23, 24, 26, 29, 32]` | 响应式适配遍历前端页面 |
| 37 | `[20, 22, 23, 24]` | SEO 配置覆盖公开页 |
| 38 | `[20]` | 全局 ErrorBoundary / Skeleton |
| 39 | `[20, 22, 24]` | 性能优化覆盖核心页 |
| 40 | `[1, 6, 20]` | 安全加固覆盖前后端 |
| 41 | `[22, 23, 24, 26, 27, 29, 31, 32, 35]` | 集成测试需所有核心页面 |
| 42 | `[2, 40, 41]` | 部署上线需安全 + 集成测试通过 |
| 43 | `[4]` | 标签体系迁移引用 content_items |
| 63 | `[3, 4, 5]` | 通知/私信/分类迁移依赖基础表 |
| 69 | `[10, 11]` | 自述「在 Task 11 基础上扩展」 |

**校验结果**：拓扑无环、无破坏引用；剩余无 depends_on 的 23 个任务（1-19、20、33-35）均为基础设施 / 入口任务，按 ID 顺序执行无误。

## 模块 AV：messages/conversations 索引创建时机注释（W4）

- **文件**：`architecture.md:854-857`
- **问题**：`idx_conversations_updated` 与 `idx_messages_sender` 写在表 DDL 紧下方，但其实由独立迁移 035 创建，开发者会误以为它们随 024_conversations.sql 一并落库
- **改动**：在两条索引上方加注释「由 migrations/035_conversation_indexes.sql 创建（Task 66 step 1），不与表 DDL 同时执行」

---

## 第六轮未修复项

| 问题 | 严重程度 | 处理方式 |
|------|---------|---------|
| Task 1-19、33-35 显式化依赖（S1） | 🟢 | 按 ID 隐式顺序无误，MVP 不动；若 Agent 引入并行执行再补 |
| ui-spec.md 实质内容生成（S2） | 🟢 | 由 Gemini 单独生成，本轮不动 |
| Task 38 拆分（S5）/ Task 25 合入 64（S8） | 🟢 | 沿用前轮决定，保留现状 |
| Prometheus / OTel | 🟢 | P1 再处理 |
| browse_history 90 天清理 | 🟢 | 部署前在 config.yaml > limits 增加 |

---

## 第六轮校验总结

- **🔴 阻塞修复**：0 / 0
- **🟡 警告修复**：4 / 4 完成（AS / AT / AU / AV）
- **🟢 优化建议**：5 项保留为未修复项

**一致性结论**：所有文档在完成第六轮修复后，经交叉校验**无阻塞问题、无遗漏功能、无依赖倒序、无未定义配置项引用**。可进入下一步开发。

---

# 第七轮修复：UI 设计校验

> 校验重点：`UI Design.md` ↔ `task.json` ↔ `design/ui-spec.md` 三文件交叉一致性

## UI-3：错别字「脆敏化」→「脱敏化」（🔴 阻塞）

- **文件**：`UI Design.md` 第 344、352 行
- **问题**：「脆敏化」为错别字，应为「脱敏化」
- **改动**：全局替换为「脱敏化」

## UI-9：ui-spec.md 顶部存根警示（🟡 警告）

- **文件**：`design/ui-spec.md` 第 1-8 行
- **问题**：ui-spec.md 为模板批量生成的存根，组件清单与响应式规则存在已知偏差，Agent 直接引用可能生成错误代码
- **改动**：在文件顶部追加 `> ⚠️ 存根警示` 引用块，说明哪些内容可信、哪些需以 `UI Design.md` 为准

## UI-4：§2.4 私信对话 Modal 缺失（🟡 警告）

- **文件**：`UI Design.md` 第 361 行
- **问题**：§五补充设计约束表格中未列出私信对话 Modal 的触发入口与行为
- **改动**：在表格中补充「私信对话 Modal」行：用户主页「发私信」按钮（移动端跳转 /messages，桌面端可弹窗），复用 ChatWindow 组件

## UI-5：P13 原创区分类选择缺失（🟡 警告）

- **文件**：`UI Design.md` 第 208-209 行
- **问题**：发布页面仅描述二创区的 IP 选择，未覆盖原创区分类选择逻辑
- **改动**：补充「原创区分类选择」：一级分类 Select + 二级分类 Select（父级联动），调用 `GET /categories?zone=original` 接口

## UI-10：P14 dashboard 创作者支持设置（🟡 警告）

- **文件**：`UI Design.md` 第 224 行
- **问题**：创作者后台未提及打赏码/外部链接的编辑入口
- **改动**：补充「创作者支持设置区域」，说明 feature flag 控制和 CreatorSupportPanel 编辑模式

## UI-11：P20 判官投票撤回机制（🟡 警告）

- **文件**：`UI Design.md` 第 261-263 行
- **问题**：判官投票页面未定义投票撤回和判决详情页
- **改动**：补充 10 秒撤回机制（倒计时 Badge + DELETE /judge/votes/:voteId）和判决详情页（投票分布 + 理由列表）

## UI-12：P13 发布区域切换 ConfirmModal（🟡 警告）

- **文件**：`UI Design.md` 第 197-198 行
- **问题**：二创区→原创区切换无数据丢失保护
- **改动**：补充切换区域保护逻辑：若已上传文件或填写字段，弹出 ConfirmModal 确认后清空草稿

## UI-7：P21 浏览历史空状态/分页（🟢 优化）

- **文件**：`UI Design.md` 第 267-269 行
- **问题**：浏览历史页面未定义分页策略和空状态
- **改动**：补充每页 30 条滚动加载和 EmptyState 提示文案

## UI-8：P02 删除列表模式（🟢 优化）

- **文件**：`UI Design.md` 第 50 行
- **问题**：首页搜索结果区描述同时提及瀑布流和列表模式，但列表模式为 P1 功能
- **改动**：标注「MVP 仅瀑布流，列表模式切换留待 P1」

## UI-13：P28 消息中心单条已读/删除（🟢 优化）

- **文件**：`UI Design.md` 第 288 行
- **问题**：消息中心仅描述批量操作，缺少单条通知的已读/删除操作
- **改动**：补充 hover 显示「标记已读」/「删除」图标按钮及对应 API

## UI-14：P11 邮箱只读（🟢 优化）

- **文件**：`UI Design.md` 第 182 行
- **问题**：账号设置中邮箱字段未标注只读
- **改动**：标注「邮箱字段：只读展示（MVP 不开放修改，避免与 OAuth 绑定冲突；P1 再开放）」

## UI-1：Task 84 ui_spec_ref 追加 JudgeQualBadge（🟡 警告）

- **文件**：`task.json` Task 84 `ui_spec_ref`
- **问题**：用户主页 UserProfileCard 包含判官资质 Badge，但 `ui_spec_ref` 未引用 `JudgeQualBadge` 组件
- **改动**：在 `ui_spec_ref` 数组中 `ContentCard` 前追加 `"## Component: JudgeQualBadge"`

## UI-2：Task 79 补 GET /users/:id/discussions + Task 84 step 5 引用（🟡 警告）

- **文件**：`task.json` Task 79 steps、Task 84 steps[4]
- **问题**：Task 84「参与的讨论」Tab 需要接口 `GET /users/:id/discussions`，但 Task 79 未定义该端点
- **改动**：
  - Task 79 新增 step：`补充 GET /users/:id/discussions`（UNION 发帖+回复，按最后活跃时间倒序分页）
  - Task 79 验证步骤追加该接口的验证
  - Task 84 step 5 显式引用 `GET /users/:id/discussions，Task 79 提供`

---

## 第七轮校验总结

- **🔴 阻塞修复**：1 / 1 完成（UI-3 错别字）
- **🟡 警告修复**：8 / 8 完成（UI-1、UI-2、UI-4、UI-5、UI-9、UI-10、UI-11、UI-12）
- **🟢 优化修复**：4 / 4 完成（UI-7、UI-8、UI-13、UI-14）

**一致性结论**：三文件（`UI Design.md` / `task.json` / `design/ui-spec.md`）经 UI 设计专项校验后，所有已识别问题均已修复。`ui-spec.md` 已标注存根警示，待 Gemini 重生成后移除。
