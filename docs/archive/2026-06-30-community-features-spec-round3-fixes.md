# OmniCraft 社区功能增强设计规格书 — 第三轮修复提示词

> 创建日期：2026-06-30
> 预计失效日期：2026-08-30
> 目标文档：`docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md`
> 关联文档：`docs/GLOSSARY.md`
> 审查来源：schema 审查（第一份报告）+ 语义审查（第二份报告）+ GLOSSARY.md 交叉验证

---

## 执行前说明

本文档基于两份审查报告与 GLOSSARY.md 交叉验证结果，在第二轮修复基础上识别出剩余 23 项问题。请按致命 → 高 → 中的顺序逐一修复，修复完成后同步更新 `docs/GLOSSARY.md`。

---

## 🔴 致命修复（阻塞实现，5 项）

### 1. 术语全局替换：收藏夹 → 收藏集

**背景**：GLOSSARY.md 第 98 行明确规定「收藏集 | 收藏夹、收藏合集 | 统一使用"收藏集"」。当前 spec 全文使用"收藏夹"约 40 处，直接违反项目术语权威文档。

**修改 `docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md`**：

全文中文字符串替换（共约 35 处，含标题、正文、表格、ASCII 图、附录），规则如下：

| 原标题/原文 | 替换为 |
|-------------|--------|
| `### 3. 收藏夹文件夹系统` | `### 3. 收藏集文件夹系统` |
| 正文及表格中所有 `收藏夹` | `收藏集` |
| ASCII 图中 `选择收藏夹` | `选择收藏集` |
| ASCII 图中 `默认收藏夹` | `默认收藏集` |
| 附录 A DEC-004 `收藏夹按 zone 分区` | `收藏集按 zone 分区` |
| 附录 A DEC-010 `收藏夹文件夹数据模型` | `收藏集文件夹数据模型` |
| 附录 D `收藏夹不分 zone` | `收藏集不分 zone` |

SQL 表名和代码标识符（`collections`、`collection_items`、`content_item_id`）不变。

### 2. 术语重命名：合集 → 内容系列

**背景**：GLOSSARY.md 第 98 行将「收藏合集」列为「收藏集」的禁用同义词。「合集」与「收藏合集」高度近似，易与「收藏集」混淆，不能再用于新概念。本修复同步在 GLOSSARY.md 第 98 行禁用同义词列追加「合集」。spec 将 `content_series` 功能命名为"合集"，需重命名为「内容系列」。

**修改 spec**：

| 原标题/原文 | 替换为 |
|-------------|--------|
| `## 4. 合集功能` | `## 4. 内容系列 (Content Series) 功能` |
| 正文中所有独立使用的 `合集`（不含 SQL 标识符） | `内容系列` |
| `合集详情页` | `内容系列详情页` |
| `合集管理页` | `内容系列管理页` |
| `合集导航栏` | `内容系列导航栏` |
| `合集封面` | `内容系列封面` |
| `合集列表` | `内容系列列表` |
| 附录 A DEC-006 `合集仅作者管理` | `内容系列仅作者管理` |
| 附录 D `合集允许非作者添加内容` | `内容系列允许非作者添加内容` |

SQL 表名和字段名（`content_series`、`content_series_items`）不变。

**同步更新 `docs/GLOSSARY.md`**：

在第 98 行禁用同义词列追加「合集」（将原文 `收藏夹、收藏合集` 改为 `收藏夹、收藏合集、合集`）。

在「功能模块」表（约第 23 行附近）新增一行：

```
| 内容系列 | Content Series | 创作者将自有内容组织为有序连载的功能。用户在内容详情页可直接导航上一章/下一章 | content_series / content_series_items 表 |
```

在「禁止混用的术语对」表（约第 89 行附近）新增一行：

```
| 内容系列 | 合集 | 合集已被收藏集占用，内容系列是独立概念 |
```

### 3. 协作邀请过期机制

**背景**：`collaboration_invites.status` CHECK 约束包含 `'expired'`（L698），但全文无任何过期规则定义。这个状态完全不可达，构成设计盲区。

**修改 §5.2**：在 DDL 代码块之后、`#### users 表新增字段` 之前插入：

```markdown
#### 邀请过期规则

| 项目 | 规则 |
|------|------|
| 过期时长 | config.yaml > collaboration.invite_expire_days 天（默认 7 天） |
| 触发方式 | scheduler 定时任务每日扫描 status='pending' 且 created_at 早于 invite_expire_days 天前的记录，批量 UPDATE status='expired' |
| 过期后前端 | 邀请卡片灰色显示「已过期」，无操作按钮 |
| 过期后重新邀请 | 允许邀请者重新发送（走完整 7 步校验链，视为新邀请）。**注意**：需要将 UNIQUE 约束改为 PostgreSQL 部分唯一索引 `CREATE UNIQUE INDEX idx_collab_invites_active ON collaboration_invites (content_id, invitee_id) WHERE status IN ('pending', 'accepted')`，否则旧 expired 记录仍会阻止新邀请插入 |
| 迁移文件 | `xxx_collaboration_invites.sql` 的 CREATE TABLE 语句中**不使用**行内 `UNIQUE (content_id, invitee_id)` 约束；表创建后用 `CREATE UNIQUE INDEX idx_collab_invites_active ON collaboration_invites (content_id, invitee_id) WHERE status IN ('pending', 'accepted')` 建立部分唯一索引 |
```

**修改 §5.4**：在卡片 UI 描述中增加一条：

```
- status='expired' → 卡片灰色，显示「已过期」，无操作按钮
```

**修改附录 B**：在 `collaboration:` 下增加一行：

```yaml
  invite_expire_days: 7            # 邀请过期天数
```

**修改附录 C**：联合创作行的说明文本中「+ messages.msg_type」后追加「+ 部分唯一索引（WHERE status IN ('pending','accepted')）」。

**修改 §5.6 涉及文件清单**：在 config.yaml 行之后增加一行（如 Fix #7 已增加 config.go 行，则在其后）：

```
| 后端 Scheduler | `internal/pkg/scheduler/collab_invite_expiry.go` | **新增**邀请过期定时清理任务（采用 Fix #17 方案 B 的 time.AfterFunc 模式，每日扫描 status='pending' 且过期的记录） |
```

### 4. §1 章节编号修复 + 补写业务约束

**背景**：TOC 第 20–21 行声明了 `1.5 业务约束` 和 `1.6 涉及文件清单`，但 body 中没有 `### 1.5 业务约束` 节——body 直接将 `1.5` 编号给了涉及文件清单（第 230 行）。body 与 TOC 脱节。**修复方向是让 body 匹配 TOC**（TOC 已经正确）。

**修改**：在 §1.4 最后一行的空行之后、当前 `### 1.5 涉及文件清单` 之前，插入以下内容：

```markdown
### 1.5 业务约束

| 约束 | 说明 |
|------|------|
| 广播不可撤回 | 通知一旦逐用户创建即不可撤回；前端需二次确认后才能发送 |
| 冷启动限制 | 详见 §1.2（纯业务逻辑，不改表） |
| 广播权限与批量 | 详见 §1.3（仅 admin、CreateInBatches 每批 500） |
| 系统通知视觉区分 | 详见 §1.4（蓝色左边框） |
```

然后将原来的 `### 1.5 涉及文件清单` 改为 `### 1.6 涉及文件清单`。

> **注意**：本次修复完成后，需通查整个 TOC（第 8–60 行）与 body 所有章节标题的一致性。以下变更会影响 TOC：
> - 新增 `0.5 成功指标（KPI）` 条目（第 6 项）
> - §3 标题 `收藏夹文件夹系统` → `收藏集文件夹系统`（第 1 项，同时检查所有子章节标题）
> - §4 标题 `合集功能` → `内容系列 (Content Series) 功能`（第 2 项）
> - §1 子章节确认：`1.5 业务约束` 已存在、`1.6 涉及文件清单` 编号正确

### 5. 错误码体系

**背景**：只有 §1.2 系统性地列出了错误码。§3（8 个端点）、§4（8 个端点）、§5（多个端点）、§6（2 个端点）全部没有错误码文档。

**修改 §3.4**：在 API 表格之后、请求体示例之前，插入：

```markdown
#### 错误码

| 错误码 | HTTP | 场景 |
|--------|------|------|
| `COLLECTION_NOT_FOUND` | 404 | 收藏集不存在或无权访问（私有收藏集非所有者） |
| `ZONE_MISMATCH` | 400 | 跨 zone 添加内容（如将 original 内容加入 fanwork 收藏集） |
| `DUPLICATE_COLLECTION_ITEM` | 409 | 同一收藏集内重复添加同一内容 |
| `DEFAULT_COLLECTION_PROTECTED` | 400 | 不允许删除默认收藏集 |
| `ZONE_IMMUTABLE` | 400 | 不允许修改已创建收藏集的 zone 字段 |
| `INVALID_CONTENT` | 400 | 内容不存在或 status 非 published |
| `REPUTATION_TOO_LOW` | 403 | 信誉分低于 min_score_for_interaction |
```

**修改 §4.4**：在 API 表格之后、PUT reorder 请求体之前，插入：

```markdown
#### 错误码

| 错误码 | HTTP | 场景 |
|--------|------|------|
| `SERIES_NOT_FOUND` | 404 | 内容系列不存在或无权访问（私有系列非所有者） |
| `NOT_SERIES_OWNER` | 403 | 非系列 owner 无权管理（添加/移除/排序/编辑） |
| `CONTENT_NOT_CONTRIBUTED` | 400 | 添加的内容在 content_contributors 中不包含系列 owner |
| `ZONE_MISMATCH` | 400 | 跨 zone 添加内容到系列 |
| `DUPLICATE_SERIES_ITEM` | 409 | 同一系列内重复添加同一内容 |
| `COVER_NOT_IN_SERIES` | 400 | cover_content_id 指向的内容不在该系列中 |
```

**修改 §5.3**：在校验链之后、频率限制实现之前，插入：

```markdown
#### 错误码

| 错误码 | HTTP | 场景 |
|--------|------|------|
| `NOT_CONTENT_OWNER` | 403 | 当前用户非内容作者或已确认 contributor |
| `REPUTATION_TOO_LOW` | 403 | 信誉分低于 min_score_for_interaction |
| `INVITE_DAILY_LIMIT` | 429 | 今日邀请次数已达 collaboration.invite_daily_limit 上限 |
| `INVITE_DUPLICATE_USER` | 409 | 今日已向该 invitee 发送过邀请 |
| `INVITE_BLOCKED` | 403 | inviter 与 invitee 存在任意方向的拉黑关系（author_blocklist 双向检查） |
| `INVITE_NOT_ACCEPTING` | 403 | invitee.accept_collab_invites 为 FALSE |
| `INVITE_ALREADY_EXISTS` | 409 | 该内容已向该 invitee 发送过邀请（UNIQUE 约束冲突） |
| `INVITE_EXPIRED` | 400 | 邀请已过期，无法执行 accept/decline |
| `INVITE_NOT_FOUND` | 404 | 邀请记录不存在 |
```

**同步修改 §5.3 步骤 5**：将原文「invitee 未被当前用户拉黑（author_blocklist 检查）→ 否则 403」改为：

```
5. inviter 与 invitee 之间不存在任意方向的拉黑关系（author_blocklist 双向检查）
   - 检查 (author_id=inviter, blocked_id=invitee) 或 (author_id=invitee, blocked_id=inviter)
   - 任一方向命中即拒绝 403 INVITE_BLOCKED
```

**修改 §6.3**：在 API 表格之后、查询逻辑增强之前，插入：

```markdown
#### 错误码

| 错误码 | HTTP | 场景 |
|--------|------|------|
| `SOURCE_NOT_ALLOWED_FOR_ORIGINAL` | 400 | 原创内容不能绑定 source_original_id |
| `FANWORK_SOURCE_NOT_ALLOWED_FOR_ORIGINAL` | 400 | 原创内容不能绑定 source_fanwork_id |
| `FANWORK_SOURCE_REQUIRED` | 400 | 二创内容必须指定 ip_id 或 source_original_id 或 source_fanwork_id 之一 |
| `MULTIPLE_SOURCE_CONFLICT` | 400 | 二创内容不能同时指定 source_original_id 和 source_fanwork_id |
| `SOURCE_ORIGINAL_UNAVAILABLE` | 400 | 指定的 source_original_id 内容不存在或非 published 原创内容 |
| `SOURCE_FANWORK_UNAVAILABLE` | 400 | 指定的 source_fanwork_id 内容不存在或非 published 二创内容 |
```

---

## 🟠 高严重度修复（6 项）

### 6. 补充成功指标（KPI）章节

**背景**：两个审查报告都指出文档缺少可衡量的成功指标。没有 KPI，上线后无法评估功能效果，也无法指导迭代优先级。

**修改**：在 §0.4 设计原则之后、§1 之前，插入：

```markdown
### 0.5 成功指标（KPI）

以下指标用于上线后评估各子系统的采用率和健康度，不作为硬性上线门禁：

| 子系统 | 指标 | 测量方式 |
|--------|------|---------|
| 私信冷启动 | 冷启动拒绝率 < 5%（被 DM_REPLY_REQUIRED 拦截的消息占比） | 后端计数，每周统计 |
| 管理员广播 | 广播覆盖率 ≥ 95%（收到广播的用户 / 当前活跃用户数） | 广播发送时记录 recipient_count 与活跃用户总数对比 |
| 浏览足迹 | 足迹页面周活 ≥ 30% 活跃用户 | 前端埋点 / 路由访问统计 |
| 收藏集 | 收藏集创建率 ≥ 30% 活跃用户至少拥有 2 个以上收藏集 | 数据库查询，每月统计 |
| 内容系列 | 系列采用率 ≥ 15% 活跃创作者至少创建 1 个系列 | 数据库查询，每月统计 |
| 联合创作 | 邀请接受率 ≥ 50%（accepted / (accepted + declined)） | 数据库查询，每月统计 |
| 联动增强 | 来源绑定率：≥ 60% 新二创使用了 source_fanwork_id 或 source_original_id | 数据库查询，每月统计 |
```

### 7. config.go 结构体改动说明

**背景**：附录 B 只给出了 YAML 新增 key，但 `backend/config/config.go` 需要新增对应的 Go 结构体和 Config 字段才能让 viper 反序列化这些配置。不提这一点会导致实现 agent 加完 YAML key 后发现运行时读不到配置。

**修改附录 B**：先将 YAML 代码块中的 `cleanup_cron: "0 3 * * *"` 改为 `cleanup_time: "03:00"`（与 Fix #17 方案 B 对齐，scheduler 不支持 cron 表达式），再在 YAML 代码块之后追加说明，明确需要在 `backend/config/config.go` 中新增 `CollaborationConfig` 和 `BrowseHistoryConfig` 两个 struct 类型（分别包含 `InviteDailyLimit`/`InviteExpireDays` 和 `RetentionDays`/`CleanupTime` 字段，均使用 `mapstructure` tag），并在 `Config` struct 中增加 `Collaboration` 和 `BrowseHistory` 两个字段，tag 分别对应 `mapstructure:"collaboration"` 和 `mapstructure:"browse_history"`。

**修改 §5.6 涉及文件清单**：在 config.yaml 行之后增加一行：

```
| 后端 Config | `backend/config/config.go` | 新增 CollaborationConfig / BrowseHistoryConfig 结构体及 Config 字段映射 |
```

**修改 §2.6 涉及文件清单**：同样增加 config.go 行（浏览足迹依赖 BrowseHistoryConfig）。

### 8. PUT /collections/:id 禁止修改 zone

**背景**：§3.3 业务约束说 zone 创建时锁定不可更改，但 API 表格中 PUT 行的权限列只说「仅所有者」，没有明确禁止修改 zone。实现 agent 可能会放行 zone 字段的修改。

**修改 §3.4 API 表格**：将 PUT 行的「权限」列内容改为：

```
仅所有者；仅允许修改 title/description/is_public/sort_order；修改 zone 返回 400 ZONE_IMMUTABLE
```

### 9. 管理员广播 N+1 窗口说明

**背景**：§1.3 步骤 2（SELECT 活跃用户）和步骤 3（批量 INSERT）之间存在极短时间窗口。窗口期内新注册的用户会漏收通知。当前规模可接受，但应记录为已知限制。

**修改 §1.3**：在步骤 6 之后、`为什么逐用户创建` 之前，插入一个 blockquote：

```markdown
> **已知限制**：步骤 2（SELECT 活跃用户）与步骤 3（批量 INSERT 通知）之间存在极短的竞态窗口。
> 窗口期内新注册的用户不会收到本次广播。当前 <1000 用户规模下该窗口可忽略。
> 未来扩容时可改用 REPEATABLE READ 事务快照或消息队列模式消除此窗口。
```

**补充：§1.3 broadcast channel 约束提示**

**背景**：当前 `notifications.channel` CHECK 约束（migration 038）为 `CHECK (channel IN ('reply','like','system','pr','follow'))`，不含 `'broadcast'`。spec §1.3 步骤 3 使用 `channel = 'broadcast'` 但 body 未交叉引用附录 C 的迁移说明。

**修改 §1.3**：在步骤 3 之后增加注解：

```markdown
> **实现前**：需先执行 `xxx_add_broadcast_channel.sql` 迁移（见附录 C），解除并重建 `notifications_channel_check` 约束，新增 `'broadcast'` 值。否则 INSERT 会因违反 CHECK 约束失败。
```

### 10. content_type 参数枚举校验

**背景**：§2.3 GET /history 新增了 `content_type` 查询参数，但未说明需要校验传入值是否为合法枚举值。传入非法值应被拒绝而非静默忽略。

**修改 §2.3**：在 GET API 参数表格之后、Response 示例之前，增加一行说明：

```markdown
`content_type` 仅接受标准内容类型枚举值（`image` / `article` / `video` / `audio` / `template` / `sheet_music` / `mod` / `prompt` / `other`）。非法值返回 `400 INVALID_CONTENT_TYPE`。
```

### 11. "已失效内容"精确定义

**背景**：当前引用块覆盖 `status != 'published' 或 deleted_at IS NOT NULL`，但未明确列出已知非 published 状态，实现 agent 可能对 `status` 取值边界有疑问。此外 `content_items` 表无 `is_banned` 字段（仅 `users`/`ips` 表有），需在 spec 中明确。

> **实现前验证**：`content_items` 表（migration 006 + 053）的 `status` 字段为 `VARCHAR(20) NOT NULL DEFAULT 'pending'`，**无 CHECK 约束**。已知非 published 状态包括 `pending`、`under_review`（代码已实现）、`author_deleted`（设计文档规定，Task 103 完成后启用，当前代码尚未使用）。`is_banned` 字段仅存在于 `users` 和 `ips` 表，`content_items` **没有**此字段。

**修改 §2.3**（当前引用块）：替换为：

```markdown
> **已失效内容处理**：满足以下任一条件的内容视为「已失效」：
> - `content_items.status != 'published'`（已知非 published 状态包括 `pending`、`under_review`、`author_deleted`；后者将在 Task 103 完成后启用）
> - `content_items.deleted_at IS NOT NULL`
>
> 对于已失效内容，API 响应中 `content` 字段返回 `null`，`viewed_at` 正常返回。
> 前端渲染为灰色占位卡片，显示「内容已删除或下架」，不可点击跳转。
> 仅影响浏览足迹列表的展示；该足迹记录本身正常保留至过期清理。
```

---

## 🟡 中严重度修复（6 项）

### 12. "B 站式布局"模糊表述替换

**背景**：5 处「对标 B 站」/「B 站式布局」不够精确。B 站布局随时变化，Agent 无法据此实现。

修改以下 5 处：

| 位置 | 原文 | 替换为 |
|------|------|--------|
| §1.1 第三点 | `前端消息中心重构对标 B 站` | `前端消息中心重构为双栏布局` |
| §1.4 布局标题 | `#### 布局（对标 B 站）` | `#### 布局（左侧导航 + 右侧内容双栏结构）` |
| §1 涉及文件清单 messages 页行 | `重构为 B 站式布局` | `重构为左侧会话列表 + 右侧对话窗口双栏布局` |
| §2 涉及文件清单 history 页行 | `重构为 B 站式布局` | `重构为分类筛选标签 + 卡片列表布局` |
| 附录 A DEC-005 | `B 站式独立页面` | `带分类筛选标签的独立页面` |

### 13. CollectionPicker 搜索功能

**背景**：用户可能有 50+ 个收藏集，弹窗中纯列表滚动不可用。

**修改 §3.5**：将 ASCII 弹窗图替换为包含搜索框的版本：

```
+----------------------------------+
|  选择收藏集         [+ 新建]     |
|  [🔍 搜索...]                    |
|                                  |
|  ● 默认收藏集 (12)              |
|  ○ 神作收藏集 (8)               |
|  ○ 教程收藏集 (3)               |
|                                  |
|  ---- 已添加 神作收藏集 ----    |
+----------------------------------+
```

并在 ASCII 图之后的描述中增加一条：

```
- 收藏集数量 ≥ 10 个时，列表顶部显示搜索框，支持按标题实时过滤
```

### 14. 内容系列封面降级逻辑归属

**背景**：当前说 `cover_content_id` 为空时「使用第一个内容的封面」，未明确是后端计算还是前端判断。这会让前后端互相推诿。

**修改 §4.2**：将 `cover_content_id` 的字段说明替换为：

```markdown
| `cover_content_id` | 可选。后端 GET /series/:id 响应中的 `cover` 字段逻辑：① 若 `cover_content_id` 非空且对应内容存在 → 返回该内容的首图（cover_image_url）；② 否则 → 返回 `sort_order` 最小的内容的封面；③ 合集无内容时 → `cover: null`。**前端直接使用 `cover` 字段，不做封面来源判断。** |
```

### 15. 子系统交叉引用

**背景**：§5 依赖 §1 的私信基础设施（conversations / messages 表），但 §5 起始处没有引用 §1。实现 agent 如果从 §5 开始做会遗漏前置条件。

**修改 §5.1**：在背景段落末尾增加：

```markdown
> **前置依赖**：本子系统依赖 §1 的私信基础设施（conversations / conversation_participants / messages 表）。
> 实现前需熟悉 §1.2 冷启动限制规则（对方未回复时限发 1 条）和 §1.4 私信对话窗口设计。
> 联合创作邀请卡片通过 messages 表的 msg_type='collab_invite' 字段承载。
```

### 16. 迁移文件占位符说明

**背景**：附录 C 中所有迁移文件名使用 `xxx_` 前缀，对实现 agent 来说不够明确。

**修改附录 C**：在表格上方增加一行说明：

```markdown
> `xxx_` 为占位符前缀。实际文件名编号按实现时 `backend/migrations/` 目录下已有最大编号 +1 顺序分配（如已有 056，则下一迁移为 057_xxx.sql）。迁移文件创建后需回填本表将占位符替换为实际编号。
```

### 17. Scheduler 不支持 cron 调度

**背景**：§2.2 说「每天凌晨 3:00 执行」，但现有 `internal/pkg/scheduler/` 中 4 个同步任务全部是固定间隔 ticker 模式（如每 60 秒 flush 一次），不支持 cron 表达式。

**修改 §2.2**：在「定时任务复用现有 scheduler 框架」段落后增加：

```markdown
> **实现注意**：当前 scheduler 框架为 ticker 固定间隔模式，不支持 cron 表达式。实现浏览足迹清理任务时需选择以下方案之一（**推荐方案 B**，实现简单、无框架改动风险；方案 A 为长期方向，后续可统一迁移）：
> - 方案 A（长期方向）：扩展 scheduler 框架增加 CronJob 接口，支持 cron 标签
> - **方案 B（MVP 推荐）**：读取 `config.yaml > browse_history.cleanup_time`（格式 `"HH:MM"`，如 `"03:00"`），使用 time.AfterFunc 计算到次日该时刻的首次延迟执行清理，**在清理函数末尾重新调用 time.AfterFunc 计算下一个 24h 延迟以形成自循环**（time.AfterFunc 为一次性触发，不会自动重复）
> - 方案 C：将清理逻辑作为 24h 间隔 ticker，在每日首次 tick 时检查是否已过 cleanup_time
```

---

## 🟢 低严重度项（已知但暂不修复）

以下问题经评估后在当前阶段不阻塞实现，标记为已知设计债务，实现 plan 阶段再处理：

| # | 问题 | 暂不修复理由 |
|---|------|-------------|
| 18 | #6 依赖列「来源链模型（#6 自身）」措辞不佳 | 不影响理解，实现 agent 知道 #6 无外部依赖 |
| 19 | §4.5 多内容系列 Tab 选择逻辑「最多展示 3 个」未定义优先规则 | 实现时默认按 sort_order 升序取前 3 个即可，不产生歧义 |
| 20 | favorites 表 30 天回滚窗口后的清理方式未说明 | migrations 目录下 DROP 语句由 DBA 手动执行，不属于 spec 层面 |
| 21 | content DTO placeholder 未展开 | 已在 §2.3 GET /history 响应中定义了完整 DTO，实现 agent 可自行查找 |
| 22 | 内容系列封面降级 cover: null 的前端兜底展示未定义 | 实现时前端使用统一灰色默认封面组件即可 |
| 23 | 无移动端适配说明 | 属于 design/ui-spec.md 层面，不在本 spec 范围 |

---

## 执行清单

```
[ ] 1. 全 spec：收藏夹 → 收藏集（约 35 处）
[ ] 2. 全 spec：合集 → 内容系列（约 38 处，保留 SQL 标识符）
[ ] 3. 更新 docs/GLOSSARY.md：第 98 行追加「合集」为禁用同义词；新增「内容系列」术语 + 禁止混用规则
[ ] 4. §5.2：补写邀请过期规则（时长、触发、UX、重发、部分唯一索引迁移措辞）
[ ] 5. §5.3 步骤 5：明确 author_blocklist 双向检查
[ ] 6. §5.6：新增 collab_invite_expiry.go scheduler 文件清单行 + config.go 文件清单行（Fix #7）
[ ] 7. §1：补写 1.5 业务约束（去重版）+ 重编号 1.6 + 修正 TOC
[ ] 8. §§3.4/4.4/5.3/6.3：各插入错误码表格（INVITE_BLOCKED 改双向）
[ ] 9. §0：新增 0.5 成功指标（KPI）
[ ] 10. 附录 B：cleanup_cron → cleanup_time；补充 config.go 结构体改动说明；§2.6/§5.6 涉及文件清单各新增 config.go 行（Fix #7）
[ ] 11. §3.4：PUT /collections/:id 禁止修改 zone
[ ] 12. §1.3：管理员广播 N+1 窗口已知限制 + broadcast channel 约束提示
[ ] 13. §2.3：content_type 枚举校验说明
[ ] 14. §2.3：已失效内容定义保留 != 'published'，补充已知状态注释
[ ] 15. 全文 5 处：B 站式布局 → 具体布局描述
[ ] 16. §3.5：CollectionPicker 增加搜索框 + 描述
[ ] 17. §4.2：封面降级逻辑明确为后端职责
[ ] 18. §5.1：增加对 §1 私信系统的交叉引用（表名用 conversation_participants）
[ ] 19. 附录 C：迁移占位符说明；联合创作迁移说明追加「+ 部分唯一索引」
[ ] 20. §2.2：scheduler 方案 B（time.AfterFunc + cleanup_time 配置）
[ ] 21. TOC 同步：新增 0.5 KPI 条目、§3 标题改「收藏集」、§4 标题改「内容系列」、确认 §1 编号
```

---

> **下一步**：用户审阅本文档 → 交付 agent 执行修复 → 修复完成后运行 `doc-validator --fix` 验证 → 转入 writing-plans 阶段。
