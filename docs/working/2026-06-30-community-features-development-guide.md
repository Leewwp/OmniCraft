# 2026-06-30 Community Features Development Guide

创建日期：2026-06-30
预计失效日期：2026-08-30
适用范围：`docs/superpowers/plans/2026-06-30-omnicraft-community-*.md` 六个社区功能计划的分轮开发、审查、集成。
当前状态：计划一 Messages And Notifications 已完成，完成 commit `f9f536d`；计划二 Browse History 已完成，完成 commit `b1fba4e`，审查修复 commit `8523fe6` / `0933feb`；下一步按顺序进入计划三 Collections。计划文档和权威规则已在 commit `a6171ab` 修复对齐。

---

## 快速启动卡片

> **已读完整文档的 agent 可跳过本节，直接从 §1 开始。**

```
请按模式 C 开发 OmniCraft 2026-06-30 社区功能计划。

先读：
- docs/working/2026-06-30-community-features-development-guide.md（本文档全文）
- AGENTS.md
- docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md
- 本轮目标计划文件：<填入六份计划之一>
- design/ui-spec.md 中目标计划提到的 Page/Component

要求：
- 使用 using-git-worktrees 创建或进入本计划隔离 worktree（默认路径 C:\tmp\omnicraft-worktrees\<branch-slug>）。
- 使用 subagent-driven-development，按计划 Task 逐个实现。
- 每个 Task 内 TDD：先失败测试，再实现，再验证。
- 每个 Task 后双重审查：规格符合性审查，然后代码质量审查。
- 不并行改共享文件；严格遵守 §6 共享文件写入顺序和 §7 跨计划业务规则矩阵。
- 只暂存本计划精确文件，不使用 git add .。
- 完成后更新计划 checkbox 和 progress.txt，最终一个计划一个 commit。
```

---

## 阅读导航

| 你的角色 | 必读 | 可跳过 |
|----------|------|--------|
| 控制 agent（分配任务、管理 worktree） | §1, §2, §3, §4, §5, §6, §7, §14 | §8-§13 细节 |
| 实现 agent（任意一个计划） | §1, §2, §5, §6, §7, 对应计划章节, §14 | 其他计划章节, §15, §16 |
| 审查 agent | §5, §6, §7, 目标计划的关键修复点, §14 | §1-§4, §15, §16 |

---

## 1. 开发模式

使用 `AGENTS.md` / `CLAUDE.md` 中的 **模式 C：2026-06-30 社区功能计划集**。完整规则见 CLAUDE.md「模式 C」段，本文档仅补充六计划特有的串行约束和共享文件写入顺序。

**关键禁令**：不要修改 `task.json`，不要勾选 Beta roadmap checkbox。

**命令约定**：本文档命令以 Windows PowerShell 为主，因为当前工作区运行在 Windows。若使用 Git Bash，可使用等价 POSIX 命令；例如 `Get-ChildItem backend\migrations | Sort-Object Name | Select-Object -Last 5 -ExpandProperty Name` 等价于 `ls backend/migrations/ | sort | tail -5`。不论 shell 类型如何，执行前后都要核对当前目录和 `git status --short`。

每次新 agent 会话必须先读：

```powershell
Get-Content docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md
Get-Content docs/superpowers/plans/2026-06-30-omnicraft-community-<plan>.md
Get-Content AGENTS.md
```

前端任务还必须读 `design/ui-spec.md` 对应 `## Page:` / `## Component:` 段落，并主动 `Select-String`（或 `rg`）检索将要实现的组件名。

---

## 2. 六个计划的执行顺序和当前进度

必须按下列顺序**串行集成**。不要六个实现代理同时开工——迁移编号、路由、配置、内容详情、发布表单、翻译文件和消息组件存在共享写入区域。

| 顺序 | 计划文件 | 分支建议 | 迁移 | 当前状态 | 主要交付 |
|---|---|---|---|---|---|
| 1 | `…community-messages-notifications.md` | `codex/community/messages-notifications` | `057_add_broadcast_channel.sql` | 已完成：`f9f536d` | 私信冷启动、消息 API、通知广播、管理员广播页、消息中心 UI |
| 2 | `…community-browse-history.md` | `codex/community/browse-history` | 无新迁移 | 已完成：`b1fba4e`；修复：`8523fe6`/`0933feb` | 浏览历史筛选/删除/保留期、每日清理 scheduler、历史页 UI |
| 3 | `…community-collections.md` | `codex/community/collections` | `058_create_collections.sql` | 下一步 | 收藏集模型、旧 favorites 兼容、推荐兼容、收藏集详情、Studio 收藏集管理 |
| 4 | `…community-content-series.md` | `codex/community/content-series` | `059_create_content_series.sql` | 未开始 | 内容系列模型、系列 API、详情页 SeriesNav、公开系列页、Studio 系列管理 |
| 5 | `…community-source-linkage.md` | `codex/community/source-linkage` | `060_add_source_fanwork_id.sql` | 未开始 | 二创来源链、`source_fanwork_id`、来源归因、相关二创/衍生作品、二创发布来源选择 |
| 6 | `…community-collaboration-invites.md` | `codex/community/collaboration-invites` | `061_collaboration_invites.sql` | 未开始 | 联合创作邀请、防骚扰链路、邀请私信卡片、设置开关、发布后邀请 |

> **迁移编号说明**：以上迁移编号（057-061）基于 `a6171ab` 快照。每个计划开始前必须先查看最近迁移：PowerShell 用 `Get-ChildItem backend\migrations | Sort-Object Name | Select-Object -Last 5 -ExpandProperty Name`，Git Bash 用 `ls backend/migrations/ | sort | tail -5`。若编号已被占用，停止并更新本表及对应计划文件中的迁移引用，详见 §14.2。

每完成一个计划，更新该计划内 checkbox、`progress.txt`，并记录最终 commit hash。

---

## 3. 工作区和分支策略

### 集成分支

```powershell
git switch main
git pull --ff-only
git switch -c codex/community-integration
```

如果 `codex/community-integration` 已存在，则切换并 `git pull --ff-only` 更新。后续每个计划从该集成分支创建独立 worktree。

### Worktree 位置（默认：仓库外路径）

**默认使用仓库外路径**，不需要改 `.gitignore`：

```
C:\tmp\omnicraft-worktrees\<branch-slug>
```

备选（需先改 `.gitignore`）：项目内 `.worktrees/<branch-slug>`。选择此方案时，必须先把 `.worktrees/` 加入 `.gitignore` 并单独提交一个准备 commit。

### 创建单个计划 worktree

```powershell
git worktree add C:\tmp\omnicraft-worktrees\messages-notifications -b codex/community/messages-notifications codex/community-integration
Set-Location C:\tmp\omnicraft-worktrees\messages-notifications
```

每个 worktree 创建后执行 `git status --short`。预期干净（或只有明确可解释的本地环境文件）。若不干净，停止并确认来源。

---

## 4. 依赖安装和基线验证

每个新 worktree 首次启动都要确认依赖可用。

后端：
```powershell
Set-Location backend
go mod tidy
go test ./...
go build ./...
go vet ./...
```

前端：
```powershell
Set-Location frontend
npm.cmd install
npm.cmd ls tsx
npx.cmd tsx --version
npm.cmd run lint
npm.cmd run build
```

`npm.cmd ls tsx` 或 `npx.cmd tsx --version` 任一失败都说明 focused TS/TSX 测试环境不可用。先修复 `frontend` 依赖或计划中的测试命令，再开始写前端测试；不要把 `node --import tsx --test ...` 失败误判为业务测试失败。

服务依赖：
```powershell
docker compose up -d postgres redis
```

### 基线测试失败处理

| 失败类型 | 判断 | 处理 |
|----------|------|------|
| 缺少 Docker 服务（postgres/redis） | 环境问题 | `docker compose up -d postgres redis`，等待就绪后重试 |
| `go mod tidy` 报错（依赖下载失败） | 网络/环境问题 | 检查 Go proxy 配置，重试 |
| main 分支已有测试失败 | 基线问题 | 记录到 `progress.txt`，确认是 main 的已知问题后继续；若是本计划引入的回归则阻塞 |
| 编译/构建失败 | 代码问题 | **阻塞**，不开始实现。确认 main 分支是否干净 |

---

## 5. 每个计划的标准执行流程

1. 控制 agent 读取本指南、源设计 spec、目标计划全文、`AGENTS.md`。
2. 控制 agent 从目标计划提取 Task 列表和文件范围（不让实现子代理自己重新解释整份计划）。
3. 每个 Task 派 fresh implementer subagent。
4. implementer 必须 TDD：先写失败测试 → 确认按预期失败 → 写最小实现 → 确认通过 → 必要时重构。
5. Task 完成后先做**规格符合性审查**。
6. 规格通过后再做**代码质量审查**。
7. 任一审查发现问题，回到同一 Task 修复并复审。
8. 一个计划的所有 Task 完成后，跑 focused gates → full gates → doc-validator → 浏览器验证。
9. 更新计划 checkbox 和 `progress.txt`。
10. 精确暂存文件，生成一个最终计划 commit。

**focused gates 定义**：只运行本计划直接覆盖的最小测试集合，例如 `go test ./internal/handler -run TestBrowseHistory -v`、`go test ./internal/service -run TestSeries -v`、`node --import tsx --test tests/history-page.test.tsx`。focused gates 不能替代 full gates；它们只用于快速确认当前切片已闭环。

### 子代理返回特殊状态的处理

| 返回状态 | 含义 | 处理方式 |
|----------|------|----------|
| `DONE_WITH_CONCERNS` | 任务完成但有疑虑 | **必须先读 concerns**。正确性、安全、范围、数据契约或迁移疑问未解决前，不得进入审查或合并 |
| `BLOCKED` | 遇到阻塞 | **不要用同一 prompt 盲目重试**。先判断：上下文不足？任务太大？计划错误？环境缺失？→ 拆分或升级处理 |

`BLOCKED` 必须按 `AGENTS.md` 的「任务阻塞 - 需要人工介入」模板输出：当前模式、当前任务、已完成工作、阻塞原因、需要人工操作、解除阻塞后的继续命令。控制 agent 记录到 `progress.txt` 后等待人工输入，不得更新 checkbox、不得提交。

---

## 6. 共享文件写入顺序

这些文件最容易冲突，必须按计划顺序串行改。**违反顺序的后果**：迁移编号冲突导致数据库迁移失败、routes.go 合并冲突需手动解决、翻译文件 key 冲突导致 lint 报错。

| 文件/区域 | 计划 1 (messages) | 计划 2 (history) | 计划 3 (collections) | 计划 4 (series) | 计划 5 (source-linkage) | 计划 6 (collaboration) |
|---|---|---|---|---|---|---|
| `backend/migrations/` | 057 | —（不涉及） | 058 | 059 | 060 | 061 |
| `backend/…/routes.go` | 注册消息+通知路由 | —（不涉及） | 注册收藏集路由 | 注册系列路由 | 注册来源链路由 | 注册协作邀请路由 |
| `backend/config/config.go` + `config.yaml` | —（不涉及） | 添加保留期+清理配置 | —（不涉及） | —（不涉及） | —（不涉及） | 添加协作邀请配置 |
| `backend/cmd/server/main.go` | —（不涉及） | 注册清理 scheduler | —（不涉及） | —（不涉及） | —（不涉及） | 注册过期 scheduler |
| `frontend/…/ContentDetail.tsx` | —（不涉及） | —（不涉及） | 添加收藏入口 | 添加 SeriesNav | 添加来源归因+相关行 | —（不涉及） |
| `frontend/…/ContentDetailClient.tsx` | —（不涉及） | —（不涉及） | 收藏入口行为（后续计划基于此） | —（不涉及） | —（不涉及） | —（不涉及） |
| `frontend/…/PublishForm.tsx` | —（不涉及） | —（不涉及） | —（不涉及） | —（不涉及） | 添加来源字段 | 添加协作邀请选择器 |
| `frontend/…/ChatWindow.tsx` | 稳定文本消息+cold-start | —（不涉及） | —（不涉及） | —（不涉及） | —（不涉及） | 添加 collab_invite typed card |
| `frontend/…/ConversationList.tsx` | 基础消息列表 | —（不涉及） | —（不涉及） | —（不涉及） | —（不涉及） | 扩展（不得破坏消息计划契约） |
| `frontend/messages/zh.json` + `en.json` | 消息+通知 key | 浏览历史 key | 收藏集 key | 系列 key | 来源链 key | 协作邀请 key |
| `design/ui-spec.md` | **默认只读**（已在 a6171ab 补齐；仅实现分支发现确实缺失/过期时才单独修复） |
| `AGENTS.md` / `CLAUDE.md` / `architecture.md` | **默认只读**（已在 a6171ab 对齐；source-linkage Task 0 只需验证，除非分支落后） |

> **路由注册规则**：修改 `routes.go` 后必须运行 `tools/doc-validator --fix`。
> **翻译文件规则**：每次 rebase 后检查 key 命名空间，禁止硬编码 UI 字符串。
> **配置修改规则**：`config.go` 和 `config.yaml` 均被两个计划修改（browse-history、collaboration），必须串行 rebase，后者基于前者的最终状态。
> **Scheduler 规则**：两个 scheduler（browse_history_cleanup、collab_invite_expiry）都必须保存实例并在 graceful shutdown 时调用 `Stop()`。

---

## 7. 跨计划业务规则矩阵

### 7.1 私信冷启动和 DM_REPLY_REQUIRED 豁免

| 消息类型 | cold-start 第一条 | 未回复时第二条 | DM_REPLY_REQUIRED 豁免 | 定义位置 |
|----------|------------------|---------------|----------------------|----------|
| 普通文本私信 | 允许 | 拒绝（`DM_REPLY_REQUIRED`） | **否** | 计划一 |
| `collab_invite` typed card | 允许 | 允许 | **是**（但仍需过权限/信誉/限流/拉黑/开关/去重检查） | 计划六 |
| 系统广播（`channel=broadcast`） | N/A（单向推送） | N/A | N/A | 计划一 |

> **重要**：计划六的 typed message 豁免仅针对 `collab_invite` 类型。普通 text DM 仍必须遵守 `DM_REPLY_REQUIRED`。实现计划六时必须读过计划一的私信契约。

### 7.2 来源字段互斥规则（计划五）

| 字段 | 适用 zone | 互斥关系 |
|------|----------|----------|
| `ip_id` | 仅 fanwork | 无互斥 |
| `source_original_id` | 仅 fanwork | 与 `source_fanwork_id` **互斥**（二选一或都不填） |
| `source_fanwork_id` | 仅 fanwork | 与 `source_original_id` **互斥**（二选一或都不填） |

- `zone='original'` 携带任一来源字段 → **必须拒绝**。
- `source_original_id` 指向的内容必须 `zone='original'` 且 `status='published'`。
- `source_fanwork_id` 指向的内容必须 `zone='fanwork'` 且 `status='published'`。
- 来源摘要必须包含 `id/title/zone`，否则前端不能渲染可点击来源卡片。

### 7.3 收藏集 auth 策略（计划三）

| 端点 | 认证要求 | 行为 |
|------|----------|------|
| `GET /api/v1/collections`（无 `owner_id`） | **必须登录** | 匿名返回 `401 AUTH_REQUIRED` |
| `GET /api/v1/collections?owner_id=:userId` | **可选登录** | 有有效 token → 以该用户身份（owner 看全部，非 owner 只看公开）；无 token → 匿名（只看公开） |

### 7.4 社区功能错误码汇总

实现时必须复用下表错误码，不得在 handler 中临时发明同义码。若实施分支发现计划遗漏错误码，先更新目标计划和本表，再写代码。

| 子系统 | 错误码 | HTTP | 首次定义位置 |
|---|---|---|---|
| messages-notifications | `DM_REPLY_REQUIRED` | 403 | 计划一 Task 1 |
| messages-notifications | `VALIDATION_ERROR` | 400 | 计划一 Task 5 |
| browse-history | `INVALID_CONTENT_TYPE` | 400 | 计划二 Task 3 |
| browse-history | `INVALID_DATE` | 400 | 计划二 Task 3 |
| browse-history | `TOO_MANY_IDS` | 400 | 计划二 Task 3 |
| collections | `COLLECTION_NOT_FOUND` | 404 | 计划三 Task 2/4 |
| collections | `ZONE_MISMATCH` | 400 | 计划三 Task 2 |
| collections | `DUPLICATE_COLLECTION_ITEM` | 409 | 计划三 Task 2 |
| collections | `DEFAULT_COLLECTION_PROTECTED` | 400 | 计划三 Task 2 |
| collections | `ZONE_IMMUTABLE` | 400 | 计划三 Task 2 |
| collections | `INVALID_CONTENT` | 400 | 计划三 Task 2 |
| content-series | `SERIES_NOT_FOUND` | 404 | 计划四 Task 3 |
| content-series | `NOT_SERIES_OWNER` | 403 | 计划四 Task 3 |
| content-series | `CONTENT_NOT_OWNED_OR_CONTRIBUTED` | 400 | 计划四 Task 3 |
| content-series | `ZONE_MISMATCH` | 400 | 计划四 Task 3 |
| content-series | `DUPLICATE_SERIES_ITEM` | 409 | 计划四 Task 3 |
| content-series | `COVER_NOT_IN_SERIES` | 400 | 计划四 Task 3 |
| source-linkage | `FANWORK_SOURCE_REQUIRED` | 400 | 计划五 Task 2 |
| source-linkage | `SOURCE_NOT_ALLOWED_FOR_ORIGINAL` | 400 | 计划五 Task 2 |
| source-linkage | `MULTIPLE_SOURCE_CONFLICT` | 400 | 计划五 Task 2 |
| source-linkage | `SOURCE_ORIGINAL_UNAVAILABLE` | 400 | 计划五 Task 2 |
| source-linkage | `SOURCE_FANWORK_UNAVAILABLE` | 400 | 计划五 Task 2 |
| collaboration-invites | `INVITE_BLOCKED` | 403 | 计划六 Task 2 |
| collaboration-invites | `INVITE_DAILY_LIMIT` | 429 | 计划六 Task 2 |
| collaboration-invites | `INVITE_DUPLICATE_USER` | 409 | 计划六 Task 2 |
| collaboration-invites | `INVITE_NOT_ACCEPTING` | 403 | 计划六 Task 2 |
| collaboration-invites | `INVITE_ALREADY_EXISTS` | 409 | 计划六 Task 2 |
| collaboration-invites | `INVITE_EXPIRED` | 400 | 计划六 Task 3 |

### 7.5 新增前端组件和 UI spec 覆盖清单

每个实现分支写 UI 代码前，先用 `rg` 检查对应 `design/ui-spec.md` 段落。若缺失，先做单独的 docs/design 修复，不要在 TSX 中临场发明视觉结构。

| 计划 | 组件/页面 | UI spec 段落 | 目标文件 |
|---|---|---|---|
| messages-notifications | `/messages` | `## Page: /messages` | `frontend/app/(protected)/messages/page.tsx` |
| messages-notifications | `/admin/notifications` | `## Page: /admin/notifications` | `frontend/app/(protected)/admin/notifications/page.tsx` |
| messages-notifications | `NotificationList`, `ConversationList`, `ChatWindow` | `## Component: ...` | `frontend/components/social/*.tsx` |
| browse-history | `/history` | `## Page: /history` | `frontend/app/(protected)/history/page.tsx` |
| collections | `CollectionPicker`, `CollectionCard`, `CollectionInfoCard`, `ContentTypeFilter` | `## Component: ...` | `frontend/components/content/*.tsx` |
| collections | `/collections/[id]`, `/user/[userId]/collections` | `## Page: ...` | `frontend/app/(public)/**/page.tsx` |
| content-series | `SeriesNav` | `## Component: SeriesNav` | `frontend/components/content/SeriesNav.tsx` |
| content-series | `/series/[id]`, `/studio/series` | `## Page: ...` | `frontend/app/(public)/series/[id]/page.tsx`, `frontend/app/(protected)/studio/series/page.tsx` |
| source-linkage | `SourceContentPicker`, `SourceAttribution`, `RelatedFanworks` | `## Component: ...` | `frontend/components/studio/SourceContentPicker.tsx`, `frontend/components/content/*.tsx` |
| collaboration-invites | `CollabUserPicker`, `CollabInviteCard` | `## Component: ...` | `frontend/components/content/CollabUserPicker.tsx`, `frontend/components/social/CollabInviteCard.tsx` |

---

## 8. 计划一：Messages And Notifications

计划文件：`docs/superpowers/plans/2026-06-30-omnicraft-community-messages-notifications.md`

**完成状态**：已完成，commit `f9f536d`（`Community messages-notifications: completed`）。

**完成摘要**：
- 后端完成 DM cold-start 保护、稳定 `/api/v1/messages` DTO、通知 `broadcast` channel、管理员广播 service/handler/audit 链路。
- 前端完成 `/admin/notifications` 管理员广播页、`/messages` 消息中心通知/私信 UI 更新、广播 Markdown 安全渲染、未读数稳定更新和移动端聊天体验。
- 最终审查修复了并发首发私信绕过 cold-start guard 的竞态：Postgres 使用事务级 advisory lock；非 Postgres fallback 使用进程内 pair mutex 覆盖完整 transaction。
- 计划 checkbox 和 `progress.txt` 已更新；未修改 `task.json`，未勾选 Beta roadmap。

**完成验证**：
- Backend：`go test ./...`、`go vet ./...`、`go build ./...` 通过。
- Frontend：`npm run test`、`node --import tsx --test tests/messages-components.test.tsx`、`node --import tsx --test tests/admin-notifications-page.test.tsx`、`npm run lint`、`npm run build` 通过。
- Doc sync：`cd tools/doc-validator && go run . --fix` 通过。
- Browser verification：Playwright MCP 使用 mocked APIs 验证管理员广播、普通用户广播展示、移动端消息中心和 `DM_REPLY_REQUIRED`；截图位于 `screenshots/community-messages-notifications-*.png`。
- 已完成规格符合性审查和代码质量复审；最终无剩余 Critical/Important issue。

**目标**：
- 私信 cold-start：第一条允许，未回复前第二条拒绝 `DM_REPLY_REQUIRED`。
- 稳定 `/api/v1/messages` DTO，修正前端消息中心调用路径。
- 新增通知 `channel='broadcast'`。
- 管理员广播通过 service 统一校验、发送和写审计。
- 前端新增 `/admin/notifications`，消息中心展示系统广播。

**重点文件范围**：
- Backend：`message.go`、`message_repo.go`、`notification_service.go`、`notification_repo.go`、`admin_audit_service.go`、`routes.go`、广播迁移（057）。
- Frontend：`/messages/page.tsx`、`/admin/notifications/page.tsx`、`ChatWindow`、`ConversationList`、`NotificationList`、i18n。

**关键约束**：
- `NotificationService.BroadcastSystemNotification(ctx, title, body, channel, actorID)` 拥有最终校验。
- handler 鉴权后**不能**提前拒绝无效 title/body/channel；必须委托 service，让 rejected audit row 能写入。
- 审计 metadata 只存长度、字段名和安全错误码，**绝不存完整 Markdown body**。

**验证**：
- `go test ./internal/handler/... -v`（消息 handler 测试）
- `go test ./internal/service/... -v`（通知和广播 service 测试）
- `node --import tsx --test tests/messages-components.test.tsx tests/admin-notifications-page.test.tsx`
- Playwright 覆盖：管理员广播、普通用户收到广播、cold-start 错误提示。

---

## 9. 计划二：Browse History

计划文件：`docs/superpowers/plans/2026-06-30-omnicraft-community-browse-history.md`

**完成状态**：已完成，commit `b1fba4e`（`Community 2: browse history enhancement`）；审查修复 commit `8523fe6`（URL query 初始化 + 无效日期范围前端拦截）和 `0933feb`（移动端筛选 chip 横向滚动修复）。已合入 `main` 和 `codex/community-integration`。

**完成摘要**：
- 后端完成 `browse_history.retention_days` / `cleanup_time` 配置映射、保留期过滤、`items` + legacy `history` 兼容响应、`content` + legacy `content_item` alias、筛选/分页/批量删除，以及 Asia/Shanghai 每日清理 scheduler。
- 前端完成 `/history` 筛选 chips、日期范围、批量删除、清空确认、失效内容占位、刷新失败保留旧数据、URL query 初始化和无效日期范围内联错误。
- 审查中修复移动端 chip 换行问题，确保筛选项保持单行并横向滚动。
- 计划 checkbox、`progress.txt` 和 doc-validator 同步已完成；未修改 `task.json`，未勾选 Beta roadmap。

**完成验证**：
- Backend：`go test ./...`、`go vet ./...`、`go build ./...` 通过。
- Frontend：`npm run test`、`node --import tsx --test tests/history-page.test.tsx`、`npm run lint`、`npm run build` 通过。
- Doc sync：`cd tools/doc-validator && go run . --fix` 通过。
- Browser verification：本地登录后验证 `/history` 留存记录、失效占位、`content_type` 筛选、同日 `start_date` / `end_date`、选中删除、清空后 EmptyState；截图保存于 `screenshots/community-browse-history-*.png`（该目录被忽略，不随提交同步）。

**目标**：
- 浏览历史按保留期筛选，支持 content_type、日期范围、批量删除。
- API 响应包含两个字段：`items`（新格式数组，推荐使用）和 `history`（旧格式数组，向后兼容，计划三完成后可废弃）。前端 normalize 优先读取 `items`。
- 不可见内容保留历史行，返回 `content: null`。
- 新增每日清理 scheduler（Asia/Shanghai），支持 graceful shutdown。
- 前端 `/history` 支持筛选、日期范围、批量管理和失效占位卡片。

**重点文件范围**：
- Backend：`config.go`、`config.yaml`、`browse_history.go` handler/repo、`browse_history_cleanup.go`、`main.go`。
- Frontend：`/history/page.tsx`、`frontend/lib/api.ts` DELETE body helper、i18n、tests/e2e。

**关键约束**：
- 保留期测试必须使用固定时间源：`BrowseHistoryListOptions.Now` 或注入 clock。
- `DeleteExpired(retentionDays, now)` 不要直接依赖 SQL `NOW()`。
- scheduler 用 `time.AfterFunc` 自循环，`main.go` 必须保存实例并在 shutdown 时 `Stop()`。

**验证**：
- `go test ./internal/repository/... -v`（浏览历史 repo 测试）
- `go test ./internal/handler/... -v`（浏览历史 handler 测试）
- `go test ./internal/pkg/scheduler/... -v`（清理 scheduler 测试）
- `node --import tsx --test tests/history-page.test.tsx`
- Playwright 覆盖：筛选、日期范围、批量删除、失效内容占位。

---

## 10. 计划三：Collections

计划文件：`docs/superpowers/plans/2026-06-30-omnicraft-community-collections.md`

**目标**：
- 将旧收藏升级为按 zone 分区的收藏集。
- 每个用户有 original/fanwork 两个默认收藏集，不可删除。
- 回滚窗口内保留旧 `/favorites` 接口并双写默认收藏集。
- 推荐画像读取 `collection_items` 与历史 `favorites` 的去重并集。
- 前端新增 CollectionPicker、收藏集详情、用户收藏集列表、Studio 收藏集管理页。

**重点文件范围**：
- Backend：`collection.go` model/repo/service/handler、`routes.go`、social favorite 兼容、recommendation service、content detail 收藏状态。
- Frontend：`frontend/lib/collections.ts`、`CollectionPicker`、`CollectionCard`、`CollectionInfoCard`、`ContentTypeFilter`、`/studio/favorites`、`/collections/[id]`、`/user/[userId]/collections`。

**关键约束**：
- 收藏集 auth 策略见 §7.3。
- `content_item_id` 缺失时每项返回 `contains_item: false`，省略 `item_id`。
- `CollectionPicker` 是 canonical，旧 modal 不再被新代码导入。

**验证**：
- `go test ./internal/model/... -v`（collection migration 测试）
- `go test ./internal/repository/... -v`（collection repo 测试）
- `go test ./internal/service/... -v`（collection service + recommendation 测试）
- `go test ./internal/handler/... -v`（collection handler 测试）
- `node --import tsx --test tests/collection-picker.test.tsx tests/collection-detail.test.tsx tests/user-collections-page.test.tsx`
- Playwright 覆盖：公开/私有、owner/非 owner、默认收藏集保护、添加到收藏集。

---

## 11. 计划四：Content Series

计划文件：`docs/superpowers/plans/2026-06-30-omnicraft-community-content-series.md`

**目标**：
- 创建公开内容系列，支持 owner 管理系列和条目排序。
- 内容详情返回 series membership，用 `SeriesNav` 展示上一条/下一条。
- 新增公开系列详情页和 Studio 系列管理页。

**重点文件范围**：
- Backend：`series.go` model/repo/service/handler、`routes.go`、content detail membership。
- Frontend：`SeriesNav`、`frontend/lib/series.ts`、`/series/[id]`、`/studio/series`、`StudioSidebar`、`ContentDetail`。

**关键约束**：
- 首版只做公开系列，**不做**私有/草稿。
- 添加内容时，owner 可添加自己发布或已确认贡献的内容；贡献者不能管理别人的 series。
- append `sort_order` 时必须在事务里锁 parent `content_series` 行，再算 `MAX(sort_order)`。
- reorder 必须事务、行锁、完整 item set 校验；首版使用上/下移动按钮，不做拖拽。

**验证**：
- `go test ./internal/model/... -v`（content_series migration 测试）
- `go test ./internal/service/... -v`（series service 测试）
- `go test ./internal/handler/... -v`（series + content detail series handler 测试）
- `node --import tsx --test tests/series-nav.test.tsx tests/series-detail-page.test.tsx tests/studio-series-page.test.tsx`
- Playwright 覆盖：公开详情、空系列、首尾导航、Studio 创建/添加/排序/删除。

---

## 12. 计划五：Source Linkage

计划文件：`docs/superpowers/plans/2026-06-30-omnicraft-community-source-linkage.md`

**目标**：
- 支持二创基于二创的 `source_fanwork_id`。
- fanwork 来源规则升级为 `ip_id / source_original_id / source_fanwork_id` 三选一（详见 §7.2）。
- 内容详情展示 `source_original` 或 `source_fanwork` 来源归因。
- related-fanworks 对原创查二创，对二创查衍生作品。
- 二创发布页使用 `SourceContentPicker`，支持 query prefill。

**重点文件范围**：
- Backend：`content.go` model/repo/service/handler、safe error mapping、迁移 `060_add_source_fanwork_id.sql`。
- Frontend：`SourceContentPicker`、`SourceAttribution`、`RelatedFanworks`、`PublishForm`、二创发布页、内容详情页、原创相关二创页、normalizer、i18n。

**关键约束**：
- `AGENTS.md`、`CLAUDE.md`、`architecture.md` 已在 `a6171ab` 修复；Task 0 通常只需验证旧单来源规则不存在。
- `source_original` 和 `source_fanwork` 摘要必须包含 `id/title/zone`，否则前端不能渲染可点击来源。
- `source_original_id` 与 `source_fanwork_id` 互斥；两个来源 ID 不能同时进入 payload。
- UI 文案使用"衍生作品"，不使用"三创"。

**验证**：
- `go test ./internal/model/... -v`（content source migration 测试）
- `go test ./internal/service/... -v`（content source + related fanworks 测试）
- `go test ./internal/handler/... -v`（content publish + related fanworks handler 测试）
- `node --import tsx --test tests/studio-publish-fanwork.test.tsx tests/source-linkage-components.test.tsx`
- Playwright 覆盖：original-source、fanwork-source、IP-only、来源不可见、相关二创/衍生作品。

---

## 13. 计划六：Collaboration Invites

计划文件：`docs/superpowers/plans/2026-06-30-omnicraft-community-collaboration-invites.md`

**目标**：
- 内容发布后邀请联合创作者。
- 邀请走完整防骚扰链路，生成 typed message card。
- 被邀请者在私信中接受/拒绝，接受后幂等加入 `content_contributors`。
- 用户可在设置里关闭协作邀请。
- 邀请过期 scheduler 支持 graceful shutdown。

**重点文件范围**：
- Backend：`collab_invite.go` model/repo/service/handler、message typed send helper、content eligibility helper、user setting、auth/me、routes、scheduler、config、main。
- Frontend：`CollabInviteCard`、`CollabUserPicker`、settings page、AuthContext、PublishForm、ChatWindow、ConversationList、i18n。

**关键约束**：
- 此计划**必须最后做**，依赖计划一的消息契约和计划五的发布表单来源字段。
- `collab_invite` typed message 不是普通文本私信，允许冷启动并豁免 `DM_REPLY_REQUIRED`（详见 §7.1）；但必须过本计划的权限、信誉、Redis 限流、同用户日限、双向拉黑、接收开关、active duplicate 检查。
- 普通 text DM 仍必须遵守 `DM_REPLY_REQUIRED`。
- Redis 不可用 fail-closed；DB 失败后要 best-effort 回滚 Redis 计数/键。
- `accept_collab_invites` 从 migration/model/PATCH/auth/me/AuthContext/settings UI **全链路覆盖**。

**验证**：
- `go test ./internal/model/... -v`（collab invite migration 测试）
- `go test ./internal/service/... -v`（collab invite send + response 测试）
- `go test ./internal/pkg/scheduler/... -v`（collab invite expiry scheduler 测试）
- `go test ./internal/handler/... -v`（collab invite handler 测试）
- `node --import tsx --test tests/collab-invite-card.test.tsx tests/settings-collab-invites.test.tsx tests/publish-collab-picker.test.tsx`
- Playwright 覆盖：pending、accepted、declined、expired、settings disabled、发布后部分邀请失败 warning。

---

## 14. 全局验证门槛

每个计划最终合并前至少运行：

```powershell
Set-Location backend
go test ./...
go build ./...
go vet ./...
```

```powershell
Set-Location frontend
npm.cmd run lint
npm.cmd run build
```

如果改了 `config.go`、`migrations` 或 `routes.go`，必须运行：

```powershell
Set-Location tools/doc-validator
go run . --fix
```

### 14.1 浏览器验证和截图

UI 相关计划必须运行浏览器验证并保存计划要求的截图到 `screenshots/`。截图目录被 `.gitignore` 忽略，不纳入提交。在 `progress.txt` 中写明本地截图路径和验证结果。

### 14.2 迁移编号冲突处理

每个计划开始前必须先确认迁移编号：

```powershell
Get-ChildItem backend\migrations | Sort-Object Name | Select-Object -Last 5 -ExpandProperty Name
```

若预期编号（如 057）已被占用：
1. 找出当前最大编号 N。
2. 将全部六个计划的迁移编号重新分配为 N+1 到 N+6。
3. 更新本文档 §2 表格中的迁移列。
4. 更新对应计划文件中的迁移编号引用。
5. 单独提交迁移编号修正（不混入功能代码）。

### 14.3 回滚一个已合并计划

若某个社区计划合并到 `codex/community-integration` 后发现严重问题，不要手动反向编辑散落文件。标准流程：

1. 保持在 `codex/community-integration`，确认 `git status --short` 干净。
2. 找到该计划唯一提交，例如 `git log --oneline --decorate -20`。
3. 执行 `git revert <commit>`，让 Git 生成反向提交；如果涉及迁移，先确认该迁移是否已在共享数据库执行。
4. 重新运行被回滚计划之前和之后受影响的 focused gates，再跑 full gates。
5. 在 `progress.txt` 记录回滚原因、回滚提交、验证命令和结果。
6. 若后续要重做该计划，从最新 `codex/community-integration` 重新开分支，不复用已回滚分支的旧工作区状态。

迁移回滚原则：已在共享或生产数据库执行的迁移不得自动 `DROP TABLE` 或删除列；计划中的 `-- ROLLBACK:` 注释只作为人工演练和本地测试参考。真实共享环境回滚必须先评估数据保留和向前修复路径。

---

## 15. 每轮新对话的接手模板

见文档顶部的「快速启动卡片」。复制那段文本发给新 agent 即可快速恢复上下文。

---

## 16. 未提交旧文档的状态

主工作区有三个 Beta 修复计划的 tracked 文档修改（措辞澄清），不是社区六计划的一部分，不影响社区功能开发：

1. `docs/superpowers/plans/2026-06-03-omnicraft-web-beta-review-repair-plan.md` — Redis 措辞澄清
2. `docs/superpowers/plans/2026-06-08-omnicraft-dependency-vulnerability-upgrades.md` — cargo audit 措辞澄清
3. `docs/superpowers/plans/2026-06-08-omnicraft-oss-upload-download-hardening.md` — strconv 措辞澄清

**处理建议**：开始社区 worktree 前先确认 `git status --short`。若这些旧文档仍有未提交改动，优先单独提交为 docs cleanup；若只是临时保存，可用 `git stash push --include-untracked -m "pre-community-doc-cleanup"`。只有明确要丢弃这些措辞澄清时，才对精确文件执行 `git checkout -- <file>`，不要对整个工作区执行批量还原。
