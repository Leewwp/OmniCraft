# 2026-06-30 Community Features Development Guide

创建日期：2026-06-30
预计失效日期：2026-08-30
适用范围：`docs/superpowers/plans/2026-06-30-omnicraft-community-*.md` 六个社区功能计划的分轮开发、审查、集成。
当前状态：开发尚未开始；计划文档和权威规则已在 commit `a6171ab` 修复对齐。

---

## 1. 当前开发模式

本轮工作使用 `AGENTS.md` / `CLAUDE.md` 中的 **模式 C：2026-06-30 社区功能计划集**。

这不是历史 `task.json` 任务，也不是 2026-05-30 Beta roadmap checkbox。不要修改 `task.json`，不要勾选 Beta roadmap。

每次新 agent 会话必须先读：

```powershell
Get-Content docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md
Get-Content docs/superpowers/plans/2026-06-30-omnicraft-community-<plan>.md
Get-Content AGENTS.md
```

前端任务还必须读 `design/ui-spec.md` 对应 `## Page:` / `## Component:` 段落，并主动 `rg` 检索将要实现的组件名。

---

## 2. 六个计划的执行顺序和当前进度

必须按下列顺序串行集成。不要六个实现代理同时开工，因为迁移编号、路由、配置、内容详情、发布表单、翻译文件和消息组件存在共享写入区域。

| 顺序 | 计划文件 | 分支建议 | 迁移 | 当前状态 | 主要交付 |
|---|---|---|---|---|---|
| 1 | `2026-06-30-omnicraft-community-messages-notifications.md` | `codex/community/messages-notifications` | `057_add_broadcast_channel.sql` | 未开始 | 私信冷启动、消息 API 稳定、通知广播、管理员广播页、消息中心 UI |
| 2 | `2026-06-30-omnicraft-community-browse-history.md` | `codex/community/browse-history` | 无新迁移 | 未开始 | 浏览历史筛选、删除、保留期、每日清理 scheduler、历史页 UI |
| 3 | `2026-06-30-omnicraft-community-collections.md` | `codex/community/collections` | `058_create_collections.sql` | 未开始 | 收藏集模型、旧 favorites 兼容、推荐兼容、收藏集详情、Studio 收藏集管理 |
| 4 | `2026-06-30-omnicraft-community-content-series.md` | `codex/community/content-series` | `059_create_content_series.sql` | 未开始 | 内容系列模型、系列 API、详情页 SeriesNav、公开系列页、Studio 系列管理 |
| 5 | `2026-06-30-omnicraft-community-source-linkage.md` | `codex/community/source-linkage` | `060_add_source_fanwork_id.sql` | 未开始 | 二创来源链、`source_fanwork_id`、来源归因、相关二创/衍生作品、二创发布来源选择 |
| 6 | `2026-06-30-omnicraft-community-collaboration-invites.md` | `codex/community/collaboration-invites` | `061_collaboration_invites.sql` | 未开始 | 联合创作邀请、防骚扰链路、邀请私信卡片、设置开关、发布后邀请 |

每完成一个计划，更新该计划内 checkbox、`progress.txt`，并记录最终 commit hash。本文档可在后续轮次继续追加“已完成计划”记录。

---

## 3. 工作区和分支策略

推荐使用一个社区集成分支作为基线：

```powershell
git switch main
git pull --ff-only
git switch -c codex/community-integration
```

如果 `codex/community-integration` 已存在，则切换并更新它。后续每个计划从该集成分支创建独立 worktree/分支。

### Worktree 位置

当前仓库没有 `.worktrees/` 目录，且 `.worktrees` 未被 `.gitignore` 忽略。创建项目内 worktree 前必须先处理这个问题。

有两种安全选择：

1. 推荐给长期本地开发：先把 `.worktrees/` 加入 `.gitignore`，单独提交一个准备提交，然后使用 `.worktrees/<branch-slug>`。
2. 推荐给不想改 repo 元数据的临时开发：使用仓库外路径，例如 `C:\tmp\omnicraft-worktrees\<branch-slug>`。

不要在未忽略的项目目录下创建 `.worktrees/`，否则 worktree 文件可能污染主工作区状态。

### 创建单个计划 worktree

示例：

```powershell
git worktree add C:\tmp\omnicraft-worktrees\messages-notifications -b codex/community/messages-notifications codex/community-integration
Set-Location C:\tmp\omnicraft-worktrees\messages-notifications
```

每个 worktree 创建后执行：

```powershell
git status --short
```

预期：干净，或只有明确可解释的本地环境文件。若不干净，先停止并确认来源。

---

## 4. 依赖安装和基线验证

每个新 worktree 首次启动都要安装依赖或确认依赖可用。

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
npm.cmd run lint
npm.cmd run build
```

服务依赖：

```powershell
docker compose up -d postgres redis
```

如果 baseline 测试失败，不要直接开始实现。先记录失败命令、失败摘要和是否为已知环境问题。后续实现验证必须能区分“基线已失败”和“本计划引入的失败”。

---

## 5. 每个计划的标准执行流程

每个计划使用 `superpowers:subagent-driven-development`，但要服从本仓库模式 C 的“一计划一分支、一计划一最终 commit”规则。

标准流程：

1. 控制 agent 读取本指南、源设计 spec、目标计划全文、`AGENTS.md`。
2. 控制 agent 从目标计划提取 Task 列表和文件范围，不让实现子代理自己重新解释整份计划。
3. 每个 Task 派 fresh implementer subagent。
4. implementer 必须 TDD：
   - 先写失败测试。
   - 跑 focused test，确认按预期失败。
   - 写最小实现。
   - 跑 focused test，确认通过。
   - 必要时重构。
5. Task 完成后先做规格符合性审查。
6. 规格通过后再做代码质量审查。
7. 任一审查发现问题，回到同一 Task 修复并复审。
8. 一个计划的所有 Task 完成后，跑该计划要求的 focused gates、full gates、doc-validator、浏览器验证。
9. 更新计划 checkbox 和 `progress.txt`。
10. 精确暂存文件，生成一个最终计划 commit。

如果子代理返回 `DONE_WITH_CONCERNS`，必须先读 concerns。正确性、安全、范围、数据契约和迁移疑问未解决前，不得进入审查或合并。

如果子代理返回 `BLOCKED`，不要让同一个 prompt 盲目重试。先判断是上下文不足、任务太大、计划错误还是环境缺失，再拆分或升级处理。

---

## 6. 共享文件写入顺序

这些文件最容易冲突，必须按计划顺序串行改：

| 文件/区域 | 写入顺序 | 注意事项 |
|---|---|---|
| `backend/migrations/` | 057 -> 058 -> 059 -> 060 -> 061 | 每个计划开始先重新列出最后迁移编号；若编号被占用，停止并更新所有社区计划和 spec |
| `backend/internal/handler/routes.go` | messages -> collections -> series -> source-linkage -> collaboration | 修改后必须跑 `tools/doc-validator --fix` |
| `backend/config/config.go` / `backend/config.yaml` | browse-history -> collaboration | 两个计划都改配置，必须串行 rebase |
| `backend/cmd/server/main.go` | browse-history -> collaboration | 两个 scheduler 都必须保存实例并在 graceful shutdown 调 `Stop()` |
| `frontend/components/content/ContentDetail.tsx` | collections -> content-series -> source-linkage | 收藏入口、SeriesNav、来源归因/相关行依次叠加 |
| `frontend/components/content/ContentDetailClient.tsx` | collections 优先 | 后续计划基于它的收藏入口行为 |
| `frontend/components/studio/PublishForm.tsx` | source-linkage -> collaboration | 先来源字段，再协作邀请选择器 |
| `frontend/components/social/ChatWindow.tsx` | messages -> collaboration | 先稳定文本消息和 cold-start，再加 `collab_invite` typed card |
| `frontend/components/social/ConversationList.tsx` | messages -> collaboration | 协作邀请只能扩展，不得破坏消息计划契约 |
| `frontend/messages/zh.json` / `frontend/messages/en.json` | 所有计划 | 每次 rebase 后检查 key 命名空间，禁止硬编码 UI 字符串 |
| `design/ui-spec.md` | 默认只读 | 已补齐社区功能 UI spec；只有实现分支发现确实缺失/过期时才单独修复 |
| `AGENTS.md` / `CLAUDE.md` / `architecture.md` | 默认只读 | 已在 `a6171ab` 对齐模式 C 和来源链；source-linkage Task 0 只需验证，除非分支落后 |

---

## 7. 计划一：Messages And Notifications

计划文件：`docs/superpowers/plans/2026-06-30-omnicraft-community-messages-notifications.md`

目标：
- 私信 cold-start：第一条允许，未回复前第二条拒绝 `DM_REPLY_REQUIRED`。
- 稳定 `/api/v1/messages` DTO，修正前端消息中心调用路径。
- 新增通知 `channel='broadcast'`。
- 管理员广播通过 service 统一校验、发送和写审计。
- 前端新增 `/admin/notifications`，消息中心展示系统广播。

重点文件范围：
- Backend：`message.go`、`message_repo.go`、`notification_service.go`、`notification_repo.go`、`admin_audit_service.go`、`routes.go`、广播迁移。
- Frontend：`/messages/page.tsx`、`/admin/notifications/page.tsx`、`ChatWindow`、`ConversationList`、`NotificationList`、i18n。

关键修复点：
- `NotificationService.BroadcastSystemNotification(ctx, title, body, channel, actorID)` 拥有最终校验。
- handler 鉴权后不能提前拒绝无效 title/body/channel；必须委托 service，让 rejected audit row 能写入。
- 审计 metadata 只存长度、字段名和安全错误码，绝不存完整 Markdown body。

验证重点：
- `go test ./internal/handler -run TestMessage -v`
- `go test ./internal/service -run TestBroadcastSystemNotification -v`
- `go test ./internal/handler -run TestAdminNotificationBroadcast -v`
- `node --import tsx --test tests/messages-components.test.tsx tests/admin-notifications-page.test.tsx`
- Playwright 覆盖管理员广播、普通用户收到广播、cold-start 错误提示。

---

## 8. 计划二：Browse History

计划文件：`docs/superpowers/plans/2026-06-30-omnicraft-community-browse-history.md`

目标：
- 浏览历史按保留期筛选，支持 content_type、日期范围、批量删除。
- API 同时返回新 `items` 和旧 `history` 字段。
- 不可见内容保留历史行，返回 `content: null`。
- 新增每日清理 scheduler，Asia/Shanghai，支持 graceful shutdown。
- 前端 `/history` 支持筛选、日期范围、批量管理和失效占位卡片。

重点文件范围：
- Backend：`config.go`、`config.yaml`、`browse_history.go` handler/repo、`browse_history_cleanup.go`、`main.go`。
- Frontend：`/history/page.tsx`、`frontend/lib/api.ts` DELETE body helper、i18n、tests/e2e。

关键修复点：
- 保留期测试必须使用固定时间源：`BrowseHistoryListOptions.Now` 或注入 clock。
- `DeleteExpired(retentionDays, now)` 不要直接依赖 SQL `NOW()`。
- scheduler 用 `time.AfterFunc` 自循环，但 `main.go` 必须保存实例并 shutdown 时 `Stop()`。

验证重点：
- `go test ./internal/repository -run TestBrowseHistory -v`
- `go test ./internal/handler -run TestBrowseHistory -v`
- `go test ./internal/pkg/scheduler -run TestBrowseHistoryCleanup -v`
- `node --import tsx --test tests/history-page.test.tsx`
- Playwright 覆盖筛选、日期范围、批量删除、失效内容占位。

---

## 9. 计划三：Collections

计划文件：`docs/superpowers/plans/2026-06-30-omnicraft-community-collections.md`

目标：
- 将旧收藏升级为按 zone 分区的收藏集。
- 每个用户有 original/fanwork 两个默认收藏集，不可删除。
- 回滚窗口内保留旧 `/favorites` 接口并双写默认收藏集。
- 推荐画像读取 `collection_items` 与历史 `favorites` 的去重并集。
- 前端新增 CollectionPicker、收藏集详情、用户收藏集列表、Studio 收藏集管理页。

重点文件范围：
- Backend：`collection.go` model/repo/service/handler、`routes.go`、social favorite 兼容、recommendation service、content detail 收藏状态。
- Frontend：`frontend/lib/collections.ts`、`CollectionPicker`、`CollectionCard`、`CollectionInfoCard`、`ContentTypeFilter`、`/studio/favorites`、`/collections/[id]`、`/user/[userId]/collections`。

关键修复点：
- `GET /api/v1/collections` 无 `owner_id` 时必须登录；匿名返回 `401 AUTH_REQUIRED`。
- `GET /api/v1/collections?owner_id=:userId` 使用 auth-if-present；匿名/非 owner 只看公开，owner 看全部。
- `content_item_id` 缺失时每项返回 `contains_item:false`，省略 `item_id`。
- `CollectionPicker` 是 canonical，旧 modal 不再被新代码导入。

验证重点：
- `go test ./internal/model -run TestCollectionMigration -v`
- `go test ./internal/repository -run TestCollection -v`
- `go test ./internal/service -run TestCollection -v`
- `go test ./internal/handler -run TestCollection -v`
- `go test ./internal/service -run TestRecommendation -v`
- `node --import tsx --test tests/collection-picker.test.tsx tests/collection-detail.test.tsx tests/user-collections-page.test.tsx`
- Playwright 覆盖公开/私有、owner/非 owner、默认收藏集保护、添加到收藏集。

---

## 10. 计划四：Content Series

计划文件：`docs/superpowers/plans/2026-06-30-omnicraft-community-content-series.md`

目标：
- 创建公开内容系列，支持 owner 管理系列和条目排序。
- 内容详情返回 series membership，用 `SeriesNav` 展示上一条/下一条。
- 新增公开系列详情页和 Studio 系列管理页。

重点文件范围：
- Backend：`series.go` model/repo/service/handler、`routes.go`、content detail membership。
- Frontend：`SeriesNav`、`frontend/lib/series.ts`、`/series/[id]`、`/studio/series`、`StudioSidebar`、`ContentDetail`。

关键修复点：
- 首版只做公开系列，不做私有/草稿。
- 添加内容时，owner 可添加自己发布或自己已确认贡献的内容；贡献者不能管理别人的 series。
- append `sort_order` 时必须在事务里锁 parent `content_series` 行，再算 `MAX(sort_order)`。
- reorder 必须事务、行锁、完整 item set 校验；首版使用上/下移动按钮，不做拖拽。

验证重点：
- `go test ./internal/model -run TestContentSeriesMigration -v`
- `go test ./internal/service -run TestSeries -v`
- `go test ./internal/handler -run TestSeries -v`
- `go test ./internal/handler -run TestContentDetailSeries -v`
- `node --import tsx --test tests/series-nav.test.tsx tests/series-detail-page.test.tsx tests/studio-series-page.test.tsx`
- Playwright 覆盖公开详情、空系列、首尾导航、Studio 创建/添加/排序/删除。

---

## 11. 计划五：Source Linkage

计划文件：`docs/superpowers/plans/2026-06-30-omnicraft-community-source-linkage.md`

目标：
- 支持二创基于二创的 `source_fanwork_id`。
- fanwork 来源规则升级为 `ip_id / source_original_id / source_fanwork_id` 三选一。
- 内容详情展示 `source_original` 或 `source_fanwork` 来源归因。
- related-fanworks 对原创查二创，对二创查衍生作品。
- 二创发布页使用 `SourceContentPicker`，支持 query prefill。

重点文件范围：
- Backend：`content.go` model/repo/service/handler、safe error mapping、迁移 `060_add_source_fanwork_id.sql`。
- Frontend：`SourceContentPicker`、`SourceAttribution`、`RelatedFanworks`、`PublishForm`、二创发布页、内容详情页、原创相关二创页、normalizer、i18n。

关键修复点：
- `AGENTS.md`、`CLAUDE.md`、`architecture.md` 已在 `a6171ab` 修复；Task 0 通常只需验证旧单来源规则不存在。
- `source_original` 和 `source_fanwork` 摘要必须包含 `id/title/zone`，否则前端不能渲染可点击来源。
- `source_original_id` 与 `source_fanwork_id` 互斥；两个来源 ID 不能同时进入 payload。
- UI 文案使用“衍生作品”，不使用“三创”。

验证重点：
- `go test ./internal/model -run TestContentSourceMigration -v`
- `go test ./internal/service -run TestContentSource -v`
- `go test ./internal/handler -run TestContentPublish -v`
- `go test ./internal/handler -run TestRelatedFanworks -v`
- `node --import tsx --test tests/studio-publish-fanwork.test.tsx tests/source-linkage-components.test.tsx`
- Playwright 覆盖 original-source、fanwork-source、IP-only、来源不可见、相关二创/衍生作品。

---

## 12. 计划六：Collaboration Invites

计划文件：`docs/superpowers/plans/2026-06-30-omnicraft-community-collaboration-invites.md`

目标：
- 内容发布后邀请联合创作者。
- 邀请走完整防骚扰链路，生成 typed message card。
- 被邀请者在私信中接受/拒绝，接受后幂等加入 `content_contributors`。
- 用户可在设置里关闭协作邀请。
- 邀请过期 scheduler 支持 graceful shutdown。

重点文件范围：
- Backend：`collab_invite.go` model/repo/service/handler、message typed send helper、content eligibility helper、user setting、auth/me、routes、scheduler、config、main。
- Frontend：`CollabInviteCard`、`CollabUserPicker`、settings page、AuthContext、PublishForm、ChatWindow、ConversationList、i18n。

关键修复点：
- 此计划必须最后做，依赖 messages 的消息契约和 source-linkage 的发布表单来源字段。
- `collab_invite` typed message 不是普通文本私信，允许冷启动并豁免 `DM_REPLY_REQUIRED`；但必须走本计划的权限、信誉、Redis 限流、同用户日限、双向拉黑、接收开关、active duplicate 检查。
- 普通 text DM 仍必须遵守 `DM_REPLY_REQUIRED`。
- Redis 不可用 fail-closed；DB 失败后要 best-effort 回滚 Redis 计数/键。
- `accept_collab_invites` 从 migration/model/PATCH/auth/me/AuthContext/settings UI 全链路覆盖。

验证重点：
- `go test ./internal/model -run TestCollabInviteMigration -v`
- `go test ./internal/service -run TestCollabInviteSend -v`
- `go test ./internal/service -run TestCollabInviteResponse -v`
- `go test ./internal/pkg/scheduler -run TestCollabInviteExpiry -v`
- `go test ./internal/handler -run TestCollabInvite -v`
- `node --import tsx --test tests/collab-invite-card.test.tsx tests/settings-collab-invites.test.tsx tests/publish-collab-picker.test.tsx`
- Playwright 覆盖 pending、accepted、declined、expired、settings disabled、发布后部分邀请失败 warning。

---

## 13. 全局验证门槛

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

UI 相关计划必须运行浏览器验证并保存计划要求的截图到 `screenshots/`。截图目录被 `.gitignore` 忽略；是否纳入提交以计划最终指令为准。如果截图被忽略但计划要求记录，可在 `progress.txt` 写明本地截图路径和验证结果。

---

## 14. 每轮新对话的接手模板

把下面这段发给新 agent，可快速恢复上下文：

```text
请按模式 C 开发 OmniCraft 2026-06-30 社区功能计划。

先读：
- docs/working/2026-06-30-community-features-development-guide.md
- AGENTS.md
- docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md
- 本轮目标计划文件：<填入六份计划之一>
- design/ui-spec.md 中目标计划提到的 Page/Component

要求：
- 使用 using-git-worktrees 创建或进入本计划隔离 worktree。
- 使用 subagent-driven-development，按计划 Task 逐个实现。
- 每个 Task 内 TDD：先失败测试，再实现，再验证。
- 每个 Task 后双重审查：规格符合性审查，然后代码质量审查。
- 不并行改共享文件；严格遵守指南中的共享文件写入顺序。
- 只暂存本计划精确文件，不使用 git add .。
- 完成后更新计划 checkbox 和 progress.txt，最终一个计划一个 commit。
```

---

## 15. 未提交旧文档的状态

当前主工作区仍有三个 tracked 文档修改，它们不是本社区六计划的一部分：

1. `docs/superpowers/plans/2026-06-03-omnicraft-web-beta-review-repair-plan.md`
   - diff：把 “Production Redis with auth/TLS if available” 改为 “auth and TLS enabled where the managed Redis provider supports it”。
   - 判断：仍是 Beta/生产部署修复计划的措辞澄清，不影响社区计划。不是待执行社区任务。

2. `docs/superpowers/plans/2026-06-08-omnicraft-dependency-vulnerability-upgrades.md`
   - diff：把 “Run cargo audit if available” 改为 “Run cargo audit when the tool is installed”。
   - 判断：仍是依赖漏洞升级计划的工具可用性澄清，不影响社区计划。不是待执行社区任务。

3. `docs/superpowers/plans/2026-06-08-omnicraft-oss-upload-download-hardening.md`
   - diff：把 `strconv` import 说明改得更明确。
   - 判断：仍是 OSS 上传下载加固计划的实现细节澄清，不影响社区计划。不是待执行社区任务。

建议：不要把这三个文档混入社区功能实现分支。可以后续单独提交为小型 docs cleanup，或在确认它们已经无效后另行归档/恢复。开始社区 worktree 开发前，主工作区 tracked dirty 状态不影响从 HEAD 创建新 worktree，但容易干扰人工判断，最好先明确处理策略。
