# OmniCraft Community Messages And Notifications Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 增强私信冷启动防骚扰、管理员系统通知广播，以及 `/messages` 双栏消息中心体验。

**Architecture:** 在不改变 conversations/messages 基础模型的前提下，为 `POST /api/v1/messages` 增加冷启动校验；为 notifications 增加 `broadcast` channel 和逐用户广播服务；前端统一改用现有 `/api/v1/messages` / `/api/v1/messages/:id` API，并新增管理员广播页面。广播正文允许 Markdown，但必须复用安全 Markdown 渲染链路。

**Tech Stack:** Go/Gin/GORM/PostgreSQL, Next.js App Router, next-intl, React Testing Library, Playwright.

---

## Cross-Plan Coordination

- Shared-file integration and migration order for the six community plans is: messages-notifications (`057`) -> browse-history (no migration) -> collections (`058`) -> content-series (`059`) -> source-linkage (`060`) -> collaboration-invites (`061`).
- `frontend/app/(protected)/messages/page.tsx`, `frontend/components/social/ChatWindow.tsx`, and `frontend/components/social/ConversationList.tsx` must land in this plan before collaboration-invites extends typed invite cards.
- `frontend/components/content/ContentDetail.tsx` changes must land collections before content-series before source-linkage.
- `frontend/components/studio/PublishForm.tsx` changes must land source-linkage before collaboration-invites; this plan must not edit `PublishForm.tsx`.
- `backend/config/config.go` and `backend/config.yaml` changes from browse-history and collaboration-invites must be implemented serially and rebased before verification.
- Before any UI code, grep `design/ui-spec.md` for the exact `## Page:` / `## Component:` sections named by this plan and follow those sections as the visual authority. As of 2026-06-30, `/messages`, `/admin/notifications`, `NotificationList`, `ConversationList`, `ChatWindow`, `MarkdownRenderer`, `MarkdownEditor`, `ConfirmModal`, and `Toast` are present; do not rewrite `design/ui-spec.md` unless an implementation-time check proves a required section is absent or stale.
- Expected-result convention: any "Run and confirm red" step expects FAIL for the behavior under test; any "Verify green" / "Run ... tests" step expects PASS. If the observed result differs, stop and update the plan before proceeding.
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
- Modify: `backend/internal/service/notification_service.go` - 增加 `BroadcastSystemNotification`。
- Modify: `backend/internal/repository/notification_repo.go` - 增加活跃用户查询和 `CreateInBatches(..., 500)`。
- Modify: `backend/internal/service/admin_audit_service.go` / `backend/internal/repository/admin_audit_repo.go` only if the existing admin audit API cannot record `success` / `rejected` / `failed` broadcast attempts with sanitized metadata.
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

- [ ] **Step 1: Add cold-start route tests**

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

- [ ] **Step 2: Run tests and confirm red**

Run:

```powershell
cd backend
go test ./internal/handler -run TestMessageColdStart -v
```

Expected: the second unanswered message is currently accepted, so at least one test fails.

- [ ] **Step 3: Add repository helpers**

Implement:

```go
func (r *MessageRepository) CountMessagesFromOtherParticipant(convID, currentUserID int64) (int64, error)
func (r *MessageRepository) LastMessageSender(convID int64) (*int64, error)
```

Both helpers must use parameterized GORM/SQL and must not assume conversation IDs are globally visible to non-participants.

- [ ] **Step 4: Implement cold-start guard**

In `MessageHandler.SendMessage`, after `FindOrCreateConversation` and before `Send`:

1. Count messages in the conversation where `sender_id != callerID`.
2. If count is greater than zero, send normally.
3. If count is zero, fetch latest message sender.
4. If latest sender exists and equals `callerID`, return `403 DM_REPLY_REQUIRED`.
5. Otherwise send normally.

- [ ] **Step 5: Verify green**

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

- [ ] **Step 1: Add DTO contract tests**

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

Also assert `GET /api/v1/messages/:id` returns messages with both `text` and `body` during the compatibility window.

- [ ] **Step 2: Run tests and confirm red**

Run:

```powershell
cd backend
go test ./internal/handler -run TestMessageAPIContract -v
```

Expected: current code returns raw conversations and message `body` only.

- [ ] **Step 3: Implement conversation summary query**

Add a repository method that returns conversation IDs for the caller, preloads active participants, last message, unread count from `conversation_participants`, and `updated_at`. Keep one query path for list pagination and one for count if a total is needed later.

- [ ] **Step 4: Return DTOs from handlers**

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

- [ ] **Step 5: Update frontend API paths**

In `ConversationList.tsx`, replace `/api/v1/conversations` with `/api/v1/messages`.

In `ChatWindow.tsx`, replace:

- `GET /api/v1/conversations/:id/messages` with `GET /api/v1/messages/:id`
- `POST /api/v1/conversations/:id/messages` with `POST /api/v1/messages` and request body `{ recipient_id, text }`

Use the selected conversation's participant list to identify `recipient_id`.

- [ ] **Step 6: Add frontend component tests**

In `frontend/tests/messages-components.test.tsx`, mock `api.get` / `api.post` and assert:

- conversation list calls `/api/v1/messages`
- chat window calls `/api/v1/messages/:id`
- sending calls `POST /api/v1/messages`
- `DM_REPLY_REQUIRED` displays localized toast

- [ ] **Step 7: Run focused tests**

Run:

```powershell
cd backend
go test ./internal/handler -run "TestMessageColdStart|TestMessageAPIContract" -v
cd ..\frontend
npm run test -- tests/messages-components.test.tsx
```

Expected: PASS.

---

## Task 3: Add Broadcast Channel Migration

**Files:**
- Create: `backend/migrations/057_add_broadcast_channel.sql`
- Test: migration smoke via backend model/handler tests

- [ ] **Step 1: Re-check migration number**

Run:

```powershell
Get-ChildItem backend\migrations | Sort-Object Name | Select-Object -Last 5 -ExpandProperty Name
```

Expected today: latest is `056_conversation_indexes.sql`. If a newer migration exists, use the next available number and update this plan's references while implementing.

- [ ] **Step 2: Write idempotent migration**

Migration must:

```sql
BEGIN;
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_channel_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_channel_check
  CHECK (channel IN ('reply', 'like', 'system', 'pr', 'follow', 'broadcast'));
COMMIT;
```

Do not remove existing channels.

- [ ] **Step 3: Verify migration is referenced by tests**

Add or update a backend test that inserts a `notifications.channel='broadcast'` row after migrations are applied.

---

## Task 4: Implement Broadcast Service And Repository

**Files:**
- Modify: `backend/internal/service/notification_service.go`
- Modify: `backend/internal/repository/notification_repo.go`
- Read/modify only if needed: `backend/internal/service/admin_audit_service.go`
- Read/modify only if needed: `backend/internal/repository/admin_audit_repo.go`
- Test: `backend/internal/service/notification_service_test.go`

- [ ] **Step 1: Add failing service tests**

Cover:

```go
func TestBroadcastSystemNotificationCreatesOneNotificationPerActiveUser(t *testing.T) {}
func TestBroadcastSystemNotificationSkipsBannedAndDeletedUsers(t *testing.T) {}
func TestBroadcastSystemNotificationUsesBatchSize500(t *testing.T) {}
func TestBroadcastSystemNotificationAuditMetadataDoesNotStoreFullBody(t *testing.T) {}
```

- [ ] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/service -run TestBroadcastSystemNotification -v
```

Expected: missing method failures.

- [ ] **Step 3: Add repository methods**

Implement:

```go
func (r *NotificationRepository) ListActiveRecipientIDs() ([]int64, error)
func (r *NotificationRepository) CreateBroadcastBatch(rows []model.Notification) error
```

`ListActiveRecipientIDs` must filter `users.is_banned = FALSE` and `users.deleted_at IS NULL`.

`CreateBroadcastBatch` must call `CreateInBatches(rows, 500)`.

- [ ] **Step 4: Add service method**

Implement:

```go
func (s *NotificationService) BroadcastSystemNotification(ctx context.Context, title, body string, actorID int64) (recipientCount int, broadcastAt time.Time, err error)
```

Each notification uses:

- `Type = "system"`
- `Channel = "broadcast"`
- `Title = &title`
- `Body = &body`

Always write an `admin_audit_logs` row through the existing admin audit service/repository. Do not make this conditional, and do not fall back to ad hoc SQL from the handler.

- Success audit row: `action='broadcast_notification'`, `target_type='users'`, `result='success'`, metadata with only `recipient_count`, `title_length`, `body_length`, and safe target filter summary.
- Rejected validation audit row: `result='rejected'`, metadata with validation error code, field names, `title_length`, `body_length`; never store the full Markdown body.
- Failed service audit row: `result='failed'`, metadata with safe error code and counts only; never store the full Markdown body.

- [ ] **Step 5: Verify green**

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

- [ ] **Step 1: Add failing route tests**

Cover:

- non-admin cannot call the route
- admin can broadcast and receives `{ data: { recipient_count, broadcast_at } }`
- blank title returns `400 VALIDATION_ERROR`
- blank body returns `400 VALIDATION_ERROR`
- title longer than 120 chars returns `400 VALIDATION_ERROR`
- body longer than 5000 chars returns `400 VALIDATION_ERROR`
- invalid `channel` returns `400 VALIDATION_ERROR`
- success, failed, and rejected attempts write audit rows with no full body

- [ ] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/handler -run TestAdminNotificationBroadcast -v
```

- [ ] **Step 3: Implement request validation**

Request body:

```go
type broadcastRequest struct {
    Title   string `json:"title"`
    Body    string `json:"body"`
    Channel string `json:"channel"`
}
```

Trim `Title` and `Body`. Reject empty `Title` or empty `Body`. Accept `Channel == ""` or `Channel == "broadcast"` only. Do not trust arbitrary channel values.

- [ ] **Step 4: Register route**

In the admin route group:

```go
admin.POST("/notifications/broadcast", adminHandler.BroadcastNotification)
```

- [ ] **Step 5: Run handler tests**

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

- [ ] **Step 1: Confirm UI spec before UI code**

Run:

```powershell
rg -n "## Page: /admin/notifications|## Component: MarkdownEditor|## Component: MarkdownRenderer" design/ui-spec.md
```

Expected: all three sections are present. If an implementation-time branch lacks `/admin/notifications`, stop and update the UI spec in a separate, explicitly scoped docs/design step before writing TSX; do not invent form layout, preview behavior, or confirmation states directly in code.

- [ ] **Step 2: Add failing frontend tests**

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

- [ ] **Step 3: Implement page**

Use:

- `MarkdownEditor` for body input
- `MarkdownRenderer` for preview
- `ConfirmModal` for the irreversible send confirmation
- `useToast` for result
- `api.post("/api/v1/admin/notifications/broadcast", { title, body, channel: "broadcast" })`

No hardcoded Chinese or English strings; every visible string must use `useTranslations()`.

- [ ] **Step 4: Run frontend tests**

Run:

```powershell
cd frontend
npm run test -- tests/admin-notifications-page.test.tsx
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

- [ ] **Step 1: Add component tests for broadcast styling**

Assert a notification with `channel: "broadcast"`:

- has a blue left border class or equivalent tokenized accent marker
- renders `title`
- renders Markdown body through safe renderer
- remains clickable only when it has a valid target

- [ ] **Step 2: Implement NotificationList changes**

Add `broadcast` to channel handling or render it under all/system filters. At minimum distinguish broadcast/system notifications visually with a blue left border.

- [ ] **Step 3: Keep layout stable**

Messages page must provide:

- desktop: conversation list left, selected chat right
- mobile: list-or-chat single-column switch
- no nested UI cards inside cards
- stable sidebar width so unread badges do not shift layout

- [ ] **Step 4: Run frontend tests**

Run:

```powershell
cd frontend
npm run test -- tests/messages-components.test.tsx
```

---

## Task 8: End-To-End Verification And Documentation Sync

**Files:**
- Modify if generated: `architecture.md`
- Modify: `progress.txt` only when executing this plan as an implementation task
- Screenshot outputs: `screenshots/community-messages-notifications-*.png`

- [ ] **Step 1: Run backend full gates**

Run:

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
```

Expected: all PASS.

- [ ] **Step 2: Run frontend full gates**

Run:

```powershell
cd frontend
npm run test
npm run lint
npm run build
```

Expected: all PASS.

- [ ] **Step 3: Run doc-validator**

Because this plan changes `backend/migrations/*.sql` and `backend/internal/handler/routes.go`, run:

```powershell
cd tools/doc-validator
go run . --fix
```

Expected: generated docs update cleanly or tool reports no changes needed.

- [ ] **Step 4: Browser verification**

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

- [ ] **Step 5: Commit when implementing**

```powershell
git add -- backend/migrations/057_add_broadcast_channel.sql backend/internal/handler/message.go backend/internal/handler/message_test.go backend/internal/repository/message_repo.go backend/internal/service/notification_service.go backend/internal/service/notification_service_test.go backend/internal/repository/notification_repo.go backend/internal/handler/notification.go backend/internal/handler/admin.go backend/internal/handler/admin_notification_broadcast_test.go backend/internal/handler/routes.go "frontend/app/(protected)/admin/notifications/page.tsx" "frontend/app/(protected)/messages/page.tsx" frontend/components/social/ChatWindow.tsx frontend/components/social/ConversationList.tsx frontend/components/social/NotificationList.tsx frontend/messages/zh.json frontend/messages/en.json frontend/tests/messages-components.test.tsx frontend/tests/admin-notifications-page.test.tsx frontend/e2e/messages-notifications.spec.ts design/ui-spec.md screenshots/community-messages-notifications-admin-desktop.png screenshots/community-messages-notifications-admin-mobile.png screenshots/community-messages-notifications-desktop.png screenshots/community-messages-notifications-mobile.png architecture.md docs/superpowers/plans/2026-06-30-omnicraft-community-messages-notifications.md progress.txt
git commit -m "Community 1: messages and notifications enhancement"
```

---

## Plan Self-Check

- [ ] Every backend behavior has an exact route, file, test, and expected error code.
- [ ] DM cold-start rules cover first send, repeated unanswered send, recipient reply, and unlocked conversation.
- [ ] Broadcast path explicitly filters banned and soft-deleted users.
- [ ] Broadcast writes mandatory `admin_audit_logs` rows for success, rejected validation, and failed send paths.
- [ ] Broadcast audit metadata excludes full Markdown body.
- [ ] Migration preserves existing notification channels and adds only `broadcast`.
- [ ] Frontend plan names the old API paths to remove and the new API paths to use.
- [ ] Markdown rendering is explicitly routed through existing safe renderer.
- [ ] Admin broadcast confirmation uses `ConfirmModal`, not the browser-native confirm API, and tests cover open, irreversible warning, focus lock, Esc close, and confirm API call.
- [ ] Browser verification includes both admin broadcast and normal user receipt.
- [ ] `doc-validator` is required because both routes and migrations change.
