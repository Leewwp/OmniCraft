# OmniCraft Community Messages And Notifications Implementation Plan

> **归档说明**：Tasks 1-8 已于原计划执行期完成；2026-07-24 完成 Task 9 持久化广播幂等跟进，覆盖真实 PostgreSQL 并发唯一边界、事务回滚、CORS 预检、前端 key 生命周期及真实浏览器同 key 重放。全部验证门通过后归档。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 增强私信冷启动防骚扰、管理员系统通知广播，以及 `/messages` 双栏消息中心体验。

**Architecture:** 在不改变 conversations/messages 基础模型的前提下，为 `POST /api/v1/messages` 增加冷启动校验；为 notifications 增加 `broadcast` channel 和逐用户广播服务；前端统一改用现有 `/api/v1/messages` / `/api/v1/messages/:id` API，并新增管理员广播页面。广播正文允许 Markdown，但必须复用安全 Markdown 渲染链路。

**Tech Stack:** Go/Gin/GORM/PostgreSQL, Next.js App Router, next-intl, React Testing Library, Playwright.

---

## Cross-Plan Coordination

- Execution source: this is part of the 2026-06-30 community feature plan family, derived from `docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md`. It is not a historical `task.json` task and not a 2026-05-30 Beta roadmap checkbox; executing it requires an explicit user request naming this plan or the community feature family.
- Shared-file integration and migration order for the six community plans is: messages-notifications (`057`) -> browse-history (no migration) -> collections (`058`) -> content-series (`059`) -> source-linkage (`060`) -> collaboration-invites (`061`).
- `frontend/app/(protected)/messages/page.tsx`, `frontend/components/social/ChatWindow.tsx`, and `frontend/components/social/ConversationList.tsx` must land in this plan before collaboration-invites extends typed invite cards.
- `frontend/components/content/ContentDetail.tsx` changes must land collections before content-series before source-linkage.
- `frontend/components/studio/PublishForm.tsx` changes must land source-linkage before collaboration-invites; this plan must not edit `PublishForm.tsx`.
- `backend/config/config.go` and `backend/config.yaml` changes from browse-history and collaboration-invites must be implemented serially and rebased before verification.
- Before any UI code, grep `design/ui-spec.md` for the exact `## Page:` / `## Component:` sections named by this plan and follow those sections as the visual authority. As of 2026-06-30, `## Page: /messages`, `## Page: /admin/notifications`, `## Component: NotificationList`, `## Component: ConversationList`, `## Component: ChatWindow`, `## Component: MarkdownRenderer`, `## Component: ConfirmModal`, and `## Component: Toast` are present as standalone sections. `MarkdownEditor` is referenced within page sections (e.g. `/admin/notifications` and `/studio/publish/*`) but has no standalone `## Component:` block — use those page-level specs and the existing `MarkdownEditor` component code as the visual authority. Do not rewrite `design/ui-spec.md` unless an implementation-time check proves a required section is absent or stale.
- Expected-result convention: any "Run and confirm red" step expects FAIL for the behavior under test; any "Verify green" / "Run ... tests" step expects PASS. If the observed result differs, stop and update the plan before proceeding.
- Frontend focused test convention: current `frontend/package.json` defines `npm run test` as a fixed suite, so focused TS/TSX tests in this plan use `node --import tsx --test <file>` directly. Do not write `npm run test -- <file>` unless the package script is changed first.
- Before implementation, run `git status --short`, reserve exact files, and stage only exact touched files. Do not use directory-level staging such as `git add backend`, `git add frontend`, `git add design`, `git add screenshots`, or `git add docs/superpowers/plans`.
- Staging note: the sample `git add` command at the end must be reduced to files actually changed in that implementation. Omit `design/ui-spec.md` when it was only read/verified; omit generated docs such as `architecture.md` unless `doc-validator` changed them during this task.

---

## File Structure

### Backend

- Create: `backend/migrations/057_add_broadcast_channel.sql` - 重建 `notifications.channel` CHECK 约束，加入 `broadcast`。
- Create: `backend/internal/handler/message_test.go` - 私信冷启动和消息 DTO 合约测试。
- Create: `backend/internal/service/notification_service_test.go` - 广播收件人筛选、批量创建和审计摘要测试。
- Create: `backend/internal/handler/admin_notification_broadcast_test.go` - 管理员广播路由权限、校验和响应测试。
- Modify: `backend/internal/handler/message.go` - 在发送前执行冷启动校验；返回可供前端直接使用的消息 DTO。
- Modify: `backend/internal/repository/message_repo.go` - 增加 conversation summary、对方消息计数、最后消息查询。
- Modify: `backend/internal/service/notification_service.go` - 增加 `BroadcastSystemNotification`，并接入 `AdminAuditService` 记录广播审计。
- Modify: `backend/internal/repository/notification_repo.go` - 增加活跃用户查询和 `CreateInBatches(..., 500)`。
- Modify: `backend/internal/service/admin_audit_service.go` - 为 `broadcast_notification` 新增 metadata allowlist，避免完整 `body` 通过默认过滤路径落库。
- Modify: `backend/internal/container/container.go` - 将 `AdminAuditService` 注入或设置到 `NotificationService`。
- Modify: `backend/internal/handler/notification.go` or `backend/internal/handler/admin.go` - 增加管理员广播 handler。
- Modify: `backend/internal/handler/routes.go` - 挂载 `POST /api/v1/admin/notifications/broadcast`。

### Frontend

- Create: `frontend/app/(protected)/admin/notifications/page.tsx` - 管理员广播编辑、预览、确认和发送页面。
- Create: `frontend/tests/messages-components.test.tsx` - 私信组件 API 合约和错误提示测试。
- Create: `frontend/tests/admin-notifications-page.test.tsx` - 管理员广播表单和预览测试。
- Create: `frontend/e2e/messages-notifications.spec.ts` - 消息中心和广播 Playwright 验证。
- Read: `design/ui-spec.md` - 对照 `/messages`、`/admin/notifications`、`NotificationList`、`ConversationList`、`ChatWindow`、`MarkdownRenderer`、`MarkdownEditor`、`ConfirmModal`、`Toast`；当前计划不应主动修改 UI spec。
- Modify: `frontend/app/(protected)/messages/page.tsx` - 调整为稳定双栏布局和通知/私信切换。
- Modify: `frontend/components/social/ChatWindow.tsx` - 改用 `/api/v1/messages/:id` 读取，`POST /api/v1/messages` 发送。
- Modify: `frontend/components/social/ConversationList.tsx` - 改用 `/api/v1/messages`，展示 participant、last_message、unread_count。
- Modify: `frontend/components/social/NotificationList.tsx` - 支持 `broadcast` 视觉区分和 Markdown 正文。
- Modify: `frontend/messages/zh.json`, `frontend/messages/en.json` - 新增所有 UI 文案和错误提示。

---

## Task 1: Write Backend Failing Tests For DM Cold Start

**Files:**
- Create: `backend/internal/handler/message_test.go`
- Read: `backend/internal/handler/message.go`
- Read: `backend/internal/repository/message_repo.go`

- [x] **Step 1: Add cold-start route tests**

Create table-driven tests covering these exact scenarios:

```go
func TestMessageColdStartAllowsFirstMessage(t *testing.T) {}
func TestMessageColdStartRejectsSecondUnansweredMessage(t *testing.T) {}
func TestMessageColdStartAllowsRecipientReply(t *testing.T) {}
func TestMessageColdStartAllowsSenderAfterRecipientHasReplied(t *testing.T) {}
```

The second test must assert:

```json
{"code":"DM_REPLY_REQUIRED","message":"对方尚未回复，请等待回复后再发送新消息"}
```

- [x] **Step 2: Run tests and confirm red**

Run:

```powershell
cd backend
go test ./internal/handler -run TestMessageColdStart -v
```

Expected: the second unanswered message is currently accepted, so at least one test fails.

- [x] **Step 3: Add repository helpers**

Implement:

```go
func (r *MessageRepository) CountMessagesFromOtherParticipant(convID, currentUserID int64) (int64, error)
func (r *MessageRepository) LastMessageSender(convID int64) (*int64, error)
```

Both helpers must use parameterized GORM/SQL and must not assume conversation IDs are globally visible to non-participants.

- [x] **Step 4: Implement cold-start guard**

In `MessageHandler.SendMessage`, after `FindOrCreateConversation` and before `Send`:

1. Count messages in the conversation where `sender_id != callerID`.
2. If count is greater than zero, send normally.
3. If count is zero, fetch latest message sender.
4. If latest sender exists and equals `callerID`, return `403 DM_REPLY_REQUIRED`.
5. Otherwise send normally.

- [x] **Step 5: Verify green**

Run:

```powershell
cd backend
go test ./internal/handler -run TestMessageColdStart -v
```

Expected: all cold-start tests PASS.

---

## Task 2: Stabilize Message API Contracts

**Files:**
- Modify: `backend/internal/handler/message.go`
- Modify: `backend/internal/repository/message_repo.go`
- Modify: `frontend/components/social/ConversationList.tsx`
- Modify: `frontend/components/social/ChatWindow.tsx`
- Test: `backend/internal/handler/message_test.go`
- Test: `frontend/tests/messages-components.test.tsx`

- [x] **Step 1: Add DTO contract tests**

Extend `message_test.go` to assert `GET /api/v1/messages` returns:

```json
{
  "conversations": [
    {
      "id": 1,
      "participants": [{"id": 2, "username": "bob", "avatar_url": ""}],
      "last_message": {"id": 10, "text": "hello", "sender_id": 1, "created_at": "..."},
      "unread_count": 1,
      "updated_at": "..."
    }
  ],
  "page": 1,
  "page_size": 20
}
```

Also assert `GET /api/v1/messages/:id` returns messages with both `text` and `body` during the compatibility window. Message list pagination defaults to `page=1`, `page_size=20`, and clamps `page_size` to a maximum of `100`.

- [x] **Step 2: Run tests and confirm red**

Run:

```powershell
cd backend
go test ./internal/handler -run TestMessageAPIContract -v
```

Expected: current code returns raw conversations and message `body` only.

- [x] **Step 3: Implement conversation summary query**

Add a repository method that returns conversation IDs for the caller, preloads active participants, last message, unread count from `conversation_participants`, and `updated_at`. Keep one query path for list pagination and one for count if a total is needed later.

- [x] **Step 4: Return DTOs from handlers**

Do not expose raw GORM models for the new message-center contract. Add local DTO structs in `message.go` or a focused internal helper:

```go
type MessageDTO struct {
    ID        int64  `json:"id"`
    SenderID  int64  `json:"sender_id"`
    Text      string `json:"text"`
    Body      string `json:"body"`
    CreatedAt string `json:"created_at"`
}
```

> **字段语义说明**：`Text` 是 `Body` 的纯别名（`Text == Body`），仅在 Beta 兼容窗口期内同时返回。兼容窗口截止条件为：计划三 Collections 合入后，前端和测试中已无任何 `body` 字段引用；两者都满足时可由单独 cleanup 任务移除 `Body`，统一使用 `Text`。`text` 字段设计为面向新前端，`body` 兼容旧调用方；两端点在兼容期内返回完全相同的值。

- [x] **Step 5: Update frontend API paths**

In `ConversationList.tsx`, replace `/api/v1/conversations` with `/api/v1/messages`.

In `ChatWindow.tsx`, replace:

- `GET /api/v1/conversations/:id/messages` with `GET /api/v1/messages/:id`
- `POST /api/v1/conversations/:id/messages` with `POST /api/v1/messages` and request body `{ recipient_id, text }`

Use the selected conversation's participant list to identify `recipient_id`.

- [x] **Step 6: Add frontend component tests**

In `frontend/tests/messages-components.test.tsx`, mock `api.get` / `api.post` and assert:

- conversation list calls `/api/v1/messages`
- chat window calls `/api/v1/messages/:id`
- sending calls `POST /api/v1/messages`
- `DM_REPLY_REQUIRED` displays localized toast

- [x] **Step 7: Run focused tests**

Run:

```powershell
cd backend
go test ./internal/handler -run "TestMessageColdStart|TestMessageAPIContract" -v
cd ..\frontend
node --import tsx --test tests/messages-components.test.tsx
```

Expected: PASS.

---

## Task 3: Add Broadcast Channel Migration

**Files:**
- Create: `backend/migrations/057_add_broadcast_channel.sql`
- Test: migration smoke via backend model/handler tests

- [x] **Step 1: Re-check migration number**

Run:

```powershell
Get-ChildItem backend\migrations | Sort-Object Name | Select-Object -Last 5 -ExpandProperty Name
```

Expected today: latest is `056_conversation_indexes.sql`. If a newer migration exists, use the next available number and update this plan's references while implementing.

- [x] **Step 2: Write idempotent migration**

Migration must:

```sql
BEGIN;
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_channel_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_channel_check
  CHECK (channel IN ('reply', 'like', 'system', 'pr', 'follow', 'broadcast'));
COMMIT;
```

Do not remove existing channels.
Include a `-- ROLLBACK:` comment block that documents how to restore the previous notification channel CHECK constraint in local-test environments. Do not auto-run rollback in shared environments after broadcast data may exist.

- [x] **Step 3: Verify migration is referenced by tests**

Add or update a backend test that inserts a `notifications.channel='broadcast'` row after migrations are applied.

---

## Task 4: Implement Broadcast Service And Repository

**Files:**
- Modify: `backend/internal/service/notification_service.go`
- Modify: `backend/internal/repository/notification_repo.go`
- Modify: `backend/internal/service/admin_audit_service.go`
- Modify: `backend/internal/container/container.go`
- Test: `backend/internal/service/notification_service_test.go`

- [x] **Step 1: Add failing service tests**

Cover:

```go
func TestBroadcastSystemNotificationCreatesOneNotificationPerActiveUser(t *testing.T) {}
func TestBroadcastSystemNotificationSkipsBannedAndDeletedUsers(t *testing.T) {}
func TestBroadcastSystemNotificationUsesBatchSize500(t *testing.T) {}
func TestBroadcastSystemNotificationAuditMetadataDoesNotStoreFullBody(t *testing.T) {}
func TestBroadcastNotificationAuditAllowlistDropsUnexpectedBodyField(t *testing.T) {}
```

- [x] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/service -run TestBroadcastSystemNotification -v
```

Expected: missing method failures.

- [x] **Step 3: Add repository methods**

Implement:

```go
func (r *NotificationRepository) ListActiveRecipientIDs() ([]int64, error)
func (r *NotificationRepository) CreateBroadcastBatch(rows []model.Notification) error
```

`ListActiveRecipientIDs` must filter `users.is_banned = FALSE` and `users.deleted_at IS NULL`.

`CreateBroadcastBatch` must call `CreateInBatches(rows, 500)`.

- [x] **Step 4: Register audit metadata allowlist**

In `backend/internal/service/admin_audit_service.go`, add an explicit allowlist entry:

```go
"broadcast_notification": {
    "recipient_count",
    "title_length",
    "body_length",
    "filter",
    "validation_error_code",
    "validation_fields",
    "error_code",
},
```

Tests must prove that accidental metadata keys such as `body`, `title`, `markdown`, `html`, or `recipients` are dropped. Do not rely on the default "drop sensitive key patterns only" branch for this action.

- [x] **Step 5: Wire audit service dependency**

Extend `NotificationService` with an `auditSvc *AdminAuditService` dependency or a focused setter:

```go
func (s *NotificationService) SetAdminAuditService(auditSvc *AdminAuditService)
```

Wire it in `backend/internal/container/container.go` after both services are constructed. The broadcast handler must call `NotificationService.BroadcastSystemNotification`; it must not perform audit writes itself except through that service method.

- [x] **Step 6: Add service method**

Implement:

```go
func (s *NotificationService) BroadcastSystemNotification(ctx context.Context, title, body, channel string, actorID int64) (recipientCount int, broadcastAt time.Time, err error)
```

This service method owns final broadcast validation. It must trim/validate `title`, `body`, and `channel` before creating notifications. If `channel == ""`, normalize it to `"broadcast"` before validation and persistence; any non-empty value other than `"broadcast"` is invalid. If validation fails, it must write the rejected `admin_audit_logs` row through `AdminAuditService`, return a safe validation error that the handler maps to `400 VALIDATION_ERROR`, and create no notifications.

Each notification uses:

- `Type = "system"`
- `Channel = "broadcast"`
- `Title = &title`
- `Body = &body`

Always write an `admin_audit_logs` row through the existing admin audit service/repository. Do not make this conditional, and do not fall back to ad hoc SQL from the handler.

- Success audit row: `action='broadcast_notification'`, `target_type='users'`, `result='success'`, metadata with only `recipient_count`, `title_length`, `body_length`, and safe target filter summary.
- Rejected validation audit row: `result='rejected'`, metadata with validation error code, field names, `title_length`, `body_length`; never store the full Markdown body. These rows are written by `NotificationService.BroadcastSystemNotification`, not by the handler.
- Failed service audit row: `result='failed'`, metadata with safe error code and counts only; never store the full Markdown body.

- [x] **Step 7: Verify green**

Run:

```powershell
cd backend
go test ./internal/service -run TestBroadcastSystemNotification -v
```

---

## Task 5: Add Admin Broadcast Handler And Route

**Files:**
- Modify: `backend/internal/handler/admin.go` or `backend/internal/handler/notification.go`
- Modify: `backend/internal/handler/routes.go`
- Test: `backend/internal/handler/admin_notification_broadcast_test.go`

- [x] **Step 1: Add failing route tests**

Cover:

- non-admin cannot call the route
- admin can broadcast and receives `{ data: { recipient_count, broadcast_at } }`
- blank title returns `400 VALIDATION_ERROR`
- blank body returns `400 VALIDATION_ERROR`
- title longer than 120 chars returns `400 VALIDATION_ERROR`
- body longer than 5000 chars returns `400 VALIDATION_ERROR`
- invalid `channel` returns `400 VALIDATION_ERROR`
- success, failed, and rejected attempts write audit rows with no full body

- [x] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/handler -run TestAdminNotificationBroadcast -v
```

- [x] **Step 3: Implement request parsing and service-owned validation**

Request body:

```go
type broadcastRequest struct {
    Title   string `json:"title"`
    Body    string `json:"body"`
    Channel string `json:"channel"`
}
```

The handler may trim for response consistency, but it must not return early for invalid `Title`, `Body`, or `Channel` after authentication succeeds. It must call `NotificationService.BroadcastSystemNotification(ctx, title, body, channel, actorID)` so the service can write success, failed, or rejected audit rows from one place. Map the service's validation error to `400 VALIDATION_ERROR`. Do not trust arbitrary channel values; the service accepts only `Channel == ""` or `Channel == "broadcast"`.

- [x] **Step 4: Register route**

In the admin route group:

```go
admin.POST("/notifications/broadcast", adminHandler.BroadcastNotification)
```

- [x] **Step 5: Run handler tests**

Run:

```powershell
cd backend
go test ./internal/handler -run TestAdminNotificationBroadcast -v
```

Expected: PASS.

---

## Task 6: Add Admin Broadcast Page

**Files:**
- Create: `frontend/app/(protected)/admin/notifications/page.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: `frontend/tests/admin-notifications-page.test.tsx`
- Read before implementation: `design/ui-spec.md`

- [x] **Step 1: Confirm UI spec before UI code**

Run:

```powershell
rg -n "## Page: /admin/notifications|## Component: MarkdownRenderer" design/ui-spec.md
```

Expected: both sections are present. `MarkdownEditor` has no standalone `## Component:` block but is specified inline within `## Page: /admin/notifications` (Markdown 编辑器交互) and `## Page: /studio/publish/*` (发布页编辑器)；use those page-level specs as the visual authority for editor behavior. If an implementation-time branch lacks `/admin/notifications`, stop and update the UI spec in a separate, explicitly scoped docs/design step before writing TSX; do not invent form layout, preview behavior, or confirmation states directly in code.

- [x] **Step 2: Add failing frontend tests**

Assert:

- title and body are required before submit
- preview renders through `MarkdownRenderer`
- clicking send opens `ConfirmModal` before any API call
- the modal states that broadcast notifications cannot be recalled
- focus is trapped inside the modal while it is open
- Esc closes the modal without calling the API
- confirming the modal calls `POST /api/v1/admin/notifications/broadcast`
- success toast includes recipient count
- API error shows localized failure toast

- [x] **Step 3: Implement page**

Use:

- `MarkdownEditor` for body input
- `MarkdownRenderer` for preview
- `ConfirmModal` for the irreversible send confirmation
- `useToast` for result
- `api.post("/api/v1/admin/notifications/broadcast", { title, body, channel: "broadcast" })`

No hardcoded Chinese or English strings; every visible string must use `useTranslations()`.

- [x] **Step 4: Run frontend tests**

Run:

```powershell
cd frontend
node --import tsx --test tests/admin-notifications-page.test.tsx
```

Expected: PASS.

---

## Task 7: Update Message Center And Notification Styling

**Files:**
- Modify: `frontend/app/(protected)/messages/page.tsx`
- Modify: `frontend/components/social/NotificationList.tsx`
- Modify: `frontend/components/social/ConversationList.tsx`
- Modify: `frontend/components/social/ChatWindow.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: `frontend/tests/messages-components.test.tsx`
- Test: `frontend/e2e/messages-notifications.spec.ts`

- [x] **Step 1: Add component tests for broadcast styling**

Assert a notification with `channel: "broadcast"`:

- has a blue left border class or equivalent tokenized accent marker
- renders `title`
- renders Markdown body through safe renderer
- remains clickable only when it has a valid target

- [x] **Step 2: Implement NotificationList changes**

Add `broadcast` to channel handling or render it under all/system filters. At minimum distinguish broadcast/system notifications visually with a blue left border.

- [x] **Step 3: Keep layout stable**

Messages page must provide:

- desktop: conversation list left, selected chat right
- mobile: list-or-chat single-column switch
- no nested UI cards inside cards
- stable sidebar width so unread badges do not shift layout

- [x] **Step 4: Run frontend tests**

Run:

```powershell
cd frontend
node --import tsx --test tests/messages-components.test.tsx
```

---

## Task 8: End-To-End Verification And Documentation Sync

**Files:**
- Modify if generated: `architecture.md`
- Modify: `progress.txt` only when executing this plan as an implementation task
- Screenshot outputs: `screenshots/community-messages-notifications-*.png`

- [x] **Step 1: Run backend full gates**

Run:

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
```

Expected: all PASS.

- [x] **Step 2: Run frontend full gates**

Run:

```powershell
cd frontend
npm run test
npm run lint
npm run build
```

Expected: all PASS.

- [x] **Step 3: Run doc-validator**

Because this plan changes `backend/migrations/*.sql` and `backend/internal/handler/routes.go`, run:

```powershell
cd tools/doc-validator
go run . --fix
```

Expected: generated docs update cleanly or tool reports no changes needed.

- [x] **Step 4: Browser verification**

Use Playwright against a running local stack:

1. Admin logs in and opens `/admin/notifications`.
2. Admin validates empty form, writes Markdown body, previews it, opens `ConfirmModal`, verifies the irreversible-send warning, verifies focus lock, presses Esc to close, reopens it, confirms send, and sees recipient count.
3. Normal user opens `/messages`, sees broadcast in notifications with unread styling and blue marker.
4. User A sends first DM to B; second unanswered message shows `DM_REPLY_REQUIRED`.
5. User B replies; User A can send again.
6. Save screenshots under:
   - `screenshots/community-messages-notifications-admin-desktop.png`
   - `screenshots/community-messages-notifications-admin-mobile.png`
   - `screenshots/community-messages-notifications-desktop.png`
   - `screenshots/community-messages-notifications-mobile.png`

- [x] **Step 5: Commit when implementing**

```powershell
git add -- backend/migrations/057_add_broadcast_channel.sql backend/internal/handler/message.go backend/internal/handler/message_test.go backend/internal/repository/message_repo.go backend/internal/service/notification_service.go backend/internal/service/notification_service_test.go backend/internal/service/admin_audit_service.go backend/internal/repository/notification_repo.go backend/internal/container/container.go backend/internal/handler/notification.go backend/internal/handler/admin.go backend/internal/handler/admin_notification_broadcast_test.go backend/internal/handler/routes.go "frontend/app/(protected)/admin/notifications/page.tsx" "frontend/app/(protected)/messages/page.tsx" frontend/components/social/ChatWindow.tsx frontend/components/social/ConversationList.tsx frontend/components/social/NotificationList.tsx frontend/messages/zh.json frontend/messages/en.json frontend/tests/messages-components.test.tsx frontend/tests/admin-notifications-page.test.tsx frontend/e2e/messages-notifications.spec.ts screenshots/community-messages-notifications-admin-desktop.png screenshots/community-messages-notifications-admin-mobile.png screenshots/community-messages-notifications-desktop.png screenshots/community-messages-notifications-mobile.png docs/superpowers/plans/2026-06-30-omnicraft-community-messages-notifications.md progress.txt
# Also add architecture.md if doc-validator --fix modified it during this task.
git commit -m "Community 1: messages and notifications enhancement"
```

---

## Task 9: Follow-Up — Make Irreversible Broadcasts Idempotent

> **Review finding (2026-07-16):** DM cold-start concurrency was rechecked and is already protected by `withConversationPairTransaction`, a conversation row lock, and `TestSendWithColdStartGuardSerializesConcurrentNewPair`; no additional DM-lock task is required. The remaining confirmed risk is duplicate admin broadcast delivery when a client retries an ambiguous request.

**Files:**
- Create: `backend/migrations/062_notification_broadcast_idempotency.sql` (059–061 remain reserved by community plans 4–6; if 062 is occupied at implementation time, take the next free number and synchronize all references)
- Create: `backend/internal/model/notification_broadcast_request.go`
- Modify: `backend/internal/repository/notification_repo.go`
- Modify: `backend/internal/service/notification_service.go`
- Modify: `backend/internal/service/notification_service_test.go`
- Modify: `backend/internal/handler/admin.go`
- Modify: `backend/internal/handler/admin_notification_broadcast_test.go`
- Modify: `frontend/app/(protected)/admin/notifications/page.tsx`
- Modify: `frontend/tests/admin-notifications-page.test.tsx`

- [x] **Step 1: Add failing backend idempotency tests**

Cover:

- missing `Idempotency-Key` returns `400 IDEMPOTENCY_KEY_REQUIRED`;
- first request reserves the key and creates one notification per active user in one database transaction;
- retry with the same key and same normalized payload returns the stored `recipient_count`/`broadcast_at` without inserting again;
- same key with a different title/body returns `409 IDEMPOTENCY_KEY_REUSED`;
- concurrent requests with the same key serialize on the database uniqueness boundary and only one creates notifications;
- a failure at any point rolls back the request record, notifications, and success audit together, so a retry can safely own the key.

- [x] **Step 2: Confirm red**

```powershell
cd backend
go test ./internal/service ./internal/handler -run "TestBroadcast.*Idempot|TestAdminNotificationBroadcast.*Idempot" -v
```

- [x] **Step 3: Add a durable idempotency record**

Create a table with a unique `(actor_id, key_hash)` constraint and fields for `payload_hash`, `recipient_count`, `broadcast_at`, and timestamps. Hash the normalized key and normalized title/body/channel payload separately. Never store the raw key, title, or Markdown body in the request record. Reserve migration 062 so this follow-up does not steal 059–061 from the three pending community plans; re-check the migration directory immediately before implementation.

- [x] **Step 4: Reserve and complete inside the broadcast transaction**

The first transaction owns a newly inserted request row, creates the per-user notifications, writes the success audit, and stores the response summary before commit. A unique-key conflict must wait for/read the committed request: matching payload replays its response; a different payload returns `IDEMPOTENCY_KEY_REUSED`. Do not use Redis as the source of truth and do not leave a committed `pending` row that requires manual repair after a rolled-back broadcast.

- [x] **Step 5: Require and return idempotency metadata**

The handler reads the `Idempotency-Key` header, passes it into the service, and returns `replayed: true|false`. Audit metadata may include a hashed key fingerprint and replay status, never the raw key or body.

- [x] **Step 6: Add frontend key lifecycle**

Generate one UUID when the confirmation dialog opens, reuse it for retries of that exact draft, and rotate it after success or after the title/body changes. Do not generate a new key for an automatic network retry.

- [x] **Step 7: Verify focused and full gates**

```powershell
cd backend
go test ./internal/service ./internal/handler -run "TestBroadcast|TestAdminNotificationBroadcast" -v
go test ./...
go vet ./...
go build ./...
cd ../frontend
node --import tsx --test tests/admin-notifications-page.test.tsx
npm run lint
npm run build
cd ../tools/doc-validator
go run . --fix
```

- [x] **Step 8: Browser verification**

Send a broadcast, replay the same captured request/key, and prove each recipient still has exactly one notification while the second response reports `replayed: true`.

- [x] **Step 9: Update only the follow-up state and commit**

Check only Task 9 steps, append a Task 9 entry to `progress.txt`, stage the exact migration/model/repository/service/handler/frontend/test/plan files actually changed, and commit `Community messages: durable broadcast idempotency`. Do not rewrite the original completed Task 1–8 history or any Beta/task.json state.

---

## Plan Self-Check

- [x] Every backend behavior has an exact route, file, test, and expected error code.
- [x] DM cold-start rules cover first send, repeated unanswered send, recipient reply, and unlocked conversation.
- [x] Broadcast path explicitly filters banned and soft-deleted users.
- [x] Broadcast writes mandatory `admin_audit_logs` rows for success, rejected validation, and failed send paths.
- [x] Broadcast audit metadata excludes full Markdown body.
- [x] Migration preserves existing notification channels and adds only `broadcast`.
- [x] Frontend plan names the old API paths to remove and the new API paths to use.
- [x] Markdown rendering is explicitly routed through existing safe renderer.
- [x] Admin broadcast confirmation uses `ConfirmModal`, not the browser-native confirm API, and tests cover open, irreversible warning, focus lock, Esc close, and confirm API call.
- [x] Browser verification includes both admin broadcast and normal user receipt.
- [x] `doc-validator` is required because both routes and migrations change.
- [x] Follow-up broadcast requests are idempotent across ambiguous client retries.
