# OmniCraft Community Collaboration Invites Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 支持发布后邀请联合创作者，被邀请者通过私信卡片接受或拒绝，接受后幂等加入 `content_contributors`。

**Architecture:** 新增 `collaboration_invites` 表、`users.accept_collab_invites` 和 `messages.msg_type/metadata`；发送邀请走完整防骚扰链路（权限、信誉、Redis 日限、同用户日限、双向拉黑、接收开关、active duplicate）；邀请通过现有 1:1 私信会话发送 typed message 卡片。邀请卡片不是普通文本私信，允许冷启动创建会话并豁免普通文本的 `DM_REPLY_REQUIRED`，但不得绕过本计划的协作邀请防骚扰链路。接受邀请使用独立幂等插入，不复用 PR 的 contributor upsert，避免错误增加 `pr_count`。邀请过期 scheduler 使用 `time.AfterFunc` 自循环，并必须在 graceful shutdown 时停止。

**Tech Stack:** Go/Gin/GORM/PostgreSQL/Redis, Next.js App Router, next-intl, React Testing Library, Playwright.

---

## Cross-Plan Coordination

- Execution source: this is part of the 2026-06-30 community feature plan family, derived from `docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md`. It is not a historical `task.json` task and not a 2026-05-30 Beta roadmap checkbox; executing it requires an explicit user request naming this plan or the community feature family.
- Shared-file integration and migration order for the six community plans is: messages-notifications (`057`) -> browse-history (no migration) -> collections (`058`) -> content-series (`059`) -> source-linkage (`060`) -> collaboration-invites (`061`).
- `frontend/app/(protected)/messages/page.tsx`, `frontend/components/social/ChatWindow.tsx`, and `frontend/components/social/ConversationList.tsx` must already use the messages-notifications contract before this plan adds typed invite cards.
- `frontend/components/content/ContentDetail.tsx` changes must land collections before content-series before source-linkage.
- `frontend/components/studio/PublishForm.tsx` source fields from source-linkage must already be present before this plan adds the collaborator picker.
- `backend/config/config.go` and `backend/config.yaml` changes from browse-history and this plan must be implemented serially and rebased before verification.
- Before any UI code, grep `design/ui-spec.md` for the exact `## Page:` / `## Component:` sections named by this plan and follow those sections as the visual authority. As of 2026-06-30, `CollabUserPicker`, `CollabInviteCard`, `/messages`, `/settings`, and `/studio/publish/fanwork` are present; do not rewrite `design/ui-spec.md` unless an implementation-time check proves a required section is absent or stale.
- Expected-result convention: any "Run and confirm red" step expects FAIL for the behavior under test; any "Verify green" / "Run ... tests" step expects PASS. If the observed result differs, stop and update the plan before proceeding.
- Frontend focused test convention: current `frontend/package.json` defines `npm run test` as a fixed suite, so focused TS/TSX tests in this plan use `node --import tsx --test <file>` directly. Do not write `npm run test -- <file>` unless the package script is changed first.
- Before implementation, run `git status --short`, reserve exact files, and stage only exact touched files. Do not use directory-level staging such as `git add backend`, `git add frontend`, `git add design`, `git add screenshots`, or `git add docs/superpowers/plans`.
- Staging note: the sample `git add` command at the end must be reduced to files actually changed in that implementation. Omit `design/ui-spec.md` when it was only read/verified; omit generated docs such as `architecture.md` unless `doc-validator` changed them during this task.

---

## Dependency Note

This plan depends on the messages foundation from `2026-06-30-omnicraft-community-messages-notifications.md`:

- frontend message components must use `/api/v1/messages`
- `ChatWindow` must be ready to render typed messages
- cold-start DM rules must already be in place
- typed collaboration invite cards are explicitly exempt from the normal text-message cold-start guard (`DM_REPLY_REQUIRED`) because the invite endpoint has its own permission, reputation, Redis rate-limit, blocklist, recipient preference, and duplicate checks. This exemption is scoped to `msg_type='collab_invite'`; normal text messages must still return `DM_REPLY_REQUIRED` until the recipient replies.

Do not implement this plan before the messages plan. If execution order is accidentally reversed, stop and either complete the messages plan first or split out a separate prerequisite task; do not silently fold message API corrections into this task.

This plan also depends on the publish-form source fields from `2026-06-30-omnicraft-community-source-linkage.md` when editing `frontend/components/studio/PublishForm.tsx`:

- source-linkage owns IP/source-original/source-fanwork field layout and validation copy
- collaboration-invites may add the collaborator picker only after those source fields are present; for fanwork it sits after source fields, and for original publish it sits after the main content fields and before submit actions
- if source-linkage has not landed, split this plan and defer Task 6's `PublishForm.tsx` changes

---

## File Structure

### Backend

- Create: `backend/migrations/061_collaboration_invites.sql`
- Create: `backend/internal/model/collab_invite.go`
- Create: `backend/internal/repository/collab_invite_repo.go`
- Create: `backend/internal/service/collab_invite_service.go`
- Create: `backend/internal/handler/collab_invite.go`
- Create: `backend/internal/pkg/scheduler/collab_invite_expiry.go`
- Create: `backend/internal/service/collab_invite_service_test.go`
- Create: `backend/internal/handler/collab_invite_test.go`
- Create: `backend/internal/pkg/scheduler/collab_invite_expiry_test.go`
- Modify: `backend/internal/model/notification.go` - `Message` gains `MsgType` and `Metadata`.
- Modify: `backend/internal/model/user.go` - add `AcceptCollabInvites`.
- Modify: `backend/internal/repository/message_repo.go` - add typed send helper.
- Modify: `backend/internal/repository/content_repo.go` - contributor/owner invite eligibility helpers.
- Modify: `backend/internal/handler/user.go`, `backend/internal/handler/auth.go` - include/update `accept_collab_invites`.
- Modify: `backend/internal/handler/routes.go`
- Modify: `backend/config/config.go`, `backend/config.yaml`
- Modify: `backend/cmd/server/main.go`

### Frontend

- Create: `frontend/components/content/CollabUserPicker.tsx`
- Create: `frontend/components/social/CollabInviteCard.tsx`
- Create: `frontend/tests/collab-invite-card.test.tsx`
- Create: `frontend/tests/settings-collab-invites.test.tsx`
- Create: `frontend/tests/publish-collab-picker.test.tsx`
- Create: `frontend/e2e/collab-invite-flow.spec.ts`
- Modify: `frontend/contexts/AuthContext.tsx`
- Modify: `frontend/app/(protected)/settings/page.tsx`
- Modify: `frontend/components/studio/PublishForm.tsx`
- Modify: `frontend/components/social/ChatWindow.tsx`
- Modify: `frontend/components/social/ConversationList.tsx`
- Modify: `frontend/messages/zh.json`, `frontend/messages/en.json`
- Read before UI code: `design/ui-spec.md` sections for `CollabUserPicker`, `CollabInviteCard`, `/messages`, `/settings`, and `/studio/publish/fanwork`.

---

## Task 1: Add Config, Migration, And Models

**Files:**
- Create: `backend/migrations/061_collaboration_invites.sql`
- Create: `backend/internal/model/collab_invite.go`
- Modify: `backend/internal/model/user.go`
- Modify: `backend/internal/model/notification.go`
- Modify: `backend/config/config.go`
- Modify: `backend/config.yaml`
- Test: backend migration/model tests

- [ ] **Step 1: Re-check migration number**

Run:

```powershell
Get-ChildItem backend\migrations | Sort-Object Name | Select-Object -Last 10 -ExpandProperty Name
```

Expected number is `061_collaboration_invites.sql` because source-linkage owns `060_add_source_fanwork_id.sql`. If `061_` is occupied by an unrelated migration at implementation time, stop and update all six community plans plus the source spec migration table before continuing.

- [ ] **Step 2: Write failing migration/model tests**

Assert:

- `collaboration_invites` exists
- `users.accept_collab_invites` exists with default `TRUE`
- `messages.msg_type` exists with `text/collab_invite` CHECK
- `messages.metadata` exists as JSONB default `{}`
- active partial unique index exists:

```sql
CREATE UNIQUE INDEX idx_collab_invites_active
ON collaboration_invites (content_id, invitee_id)
WHERE status IN ('pending', 'accepted');
```

- [ ] **Step 3: Implement migration**

Do not create a normal `UNIQUE (content_id, invitee_id)` constraint; expired invites must allow re-invite.

- [ ] **Step 4: Add config**

Add:

```go
type CollaborationConfig struct {
    InviteDailyLimit int `mapstructure:"invite_daily_limit"`
    InviteExpireDays int `mapstructure:"invite_expire_days"`
}
```

Mount as `Config.Collaboration`. Add YAML defaults:

```yaml
collaboration:
  invite_daily_limit: 20
  invite_expire_days: 7
```

Do not add this config to `SaveOverride`, admin config, or public config.

- [ ] **Step 5: Verify**

Run:

```powershell
cd backend
go test ./internal/model -run TestCollaborationInviteMigration -v
go test ./config -run TestCollaborationConfig -v
```

---

## Task 2: Implement Invite Service Anti-Abuse Chain

**Files:**
- Create: `backend/internal/repository/collab_invite_repo.go`
- Create: `backend/internal/service/collab_invite_service.go`
- Modify: `backend/internal/repository/message_repo.go`
- Modify: `backend/internal/repository/content_repo.go`
- Test: `backend/internal/service/collab_invite_service_test.go`

- [ ] **Step 1: Write failing service tests**

Cover all exact outcomes:

- owner can invite
- confirmed contributor can invite
- non-owner/non-contributor -> `NOT_CONTENT_OWNER`
- low reputation -> `REPUTATION_TOO_LOW`
- daily count >= limit -> `INVITE_DAILY_LIMIT`
- same invitee same day -> `INVITE_DUPLICATE_USER`
- blocklist either direction -> `INVITE_BLOCKED`
- invitee disabled invites -> `INVITE_NOT_ACCEPTING`
- active duplicate for same content/invitee -> `INVITE_ALREADY_EXISTS`
- expired invite for same content/invitee can be invited again as a new row
- success creates invite, typed message, and backfills `message_id`
- success can create the first 1:1 conversation and `msg_type='collab_invite'` message even when a normal text DM would return `DM_REPLY_REQUIRED`
- normal text send still returns `DM_REPLY_REQUIRED` in the same cold-start relationship until the recipient replies
- Redis unavailable during rate-limit reservation aborts before DB write and returns a generic service-unavailable error
- DB failure after Redis reservation best-effort compensates counters/keys and logs any compensation failure

> **实施决策**：Redis 不可用时采用 fail-closed 策略（拒绝邀请操作）。这是最安全的选择，确保防骚扰链路在存储不可用时不会出现缺口。如果产品要求不同降级行为（如降级为仅 DB 校验），需单独讨论并修改设计规格。

- [ ] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/service -run TestCollabInviteSend -v
```

- [ ] **Step 3: Implement Redis anti-abuse keys**

Use:

```text
collab_invite_count:{inviter_id}:{YYYY-MM-DD}
collab_invite_user:{inviter_id}:{invitee_id}:{YYYY-MM-DD}
```

Both expire after 86400 seconds. Use `SET NX EX` for the per-invitee key.

Daily key date semantics:

- `YYYY-MM-DD` is the `Asia/Shanghai` calendar date, matching the product's user-visible "today" and the production server timezone.
- Compute it from an injected/testable clock in the service where possible; tests must cover requests immediately before and after Asia/Shanghai midnight.
- Do not use UTC dates for these keys unless the design spec is explicitly changed first.

Reservation strategy:

1. Run cheap DB/user/content validation before consuming Redis quota.
2. Reserve Redis daily count and per-invitee key before DB writes; fail closed if Redis is unavailable.
3. If the DB transaction fails after Redis reservation, best-effort `DECR` the daily count and `DEL` the per-invitee key, then return the DB error.
4. Tests must prove the failure path does not create an invite row or typed message.

- [ ] **Step 4: Implement typed message send**

Add:

```go
func (r *MessageRepository) SendTyped(senderID, convID int64, body, msgType string, metadata model.JSONMap) (*model.Message, error)
```

Keep existing `Send` as a wrapper for `msgType="text"` and empty metadata.

`SendTyped` is for trusted service-level typed messages only. The collaboration invite service may call it after its anti-abuse chain passes, and this path is exempt from the regular text `DM_REPLY_REQUIRED` guard. Do not expose a generic handler that lets clients choose arbitrary `msg_type`; normal user-authored text messages must continue through the existing cold-start guard.

Metadata must include only:

- `invite_id`
- `content_id`
- `content_title`
- `inviter_id`
- `inviter_username`

No email, tokens, private settings, or full user profile.

- [ ] **Step 5: Implement service transaction**

On success:

1. create invite
2. find/create 1:1 conversation
3. create `msg_type='collab_invite'` message
4. update invite `message_id`

Use a DB transaction for DB writes. Redis counters should be applied in an order that avoids allowing spam if DB succeeds but counter fails; document and test the chosen rollback/compensation behavior.

- [ ] **Step 6: Verify send service**

Run:

```powershell
cd backend
go test ./internal/service -run TestCollabInviteSend -v
```

---

## Task 3: Implement Accept, Decline, And Expiry

**Files:**
- Modify: `backend/internal/service/collab_invite_service.go`
- Create: `backend/internal/pkg/scheduler/collab_invite_expiry.go`
- Test: `backend/internal/service/collab_invite_service_test.go`
- Test: `backend/internal/pkg/scheduler/collab_invite_expiry_test.go`

- [ ] **Step 1: Write failing response tests**

Cover:

- only invitee can accept or decline
- pending invite can be accepted
- pending invite can be declined
- expired invite returns `INVITE_EXPIRED`
- accepted/declined invite cannot be re-accepted
- accept inserts `content_contributors` idempotently
- accept does not increment `pr_count`

- [ ] **Step 2: Implement accept transaction**

Use:

1. `SELECT ... FOR UPDATE` invite row
2. check status `pending`
3. check not expired by `collaboration.invite_expire_days`
4. insert into `content_contributors(content_item_id, user_id, pr_count, first_at)` with upsert; set `pr_count=0` for collaboration-created rows and do not update/increment `pr_count` for existing contributor rows
5. update status to `accepted`, set `responded_at`
6. return latest DTO

- [ ] **Step 3: Implement decline transaction**

Lock row, verify invitee and pending status, update status to `declined`, set `responded_at`.

- [ ] **Step 4: Add expiry scheduler tests**

Scheduler marks `pending` invites older than configured days as `expired`. It does not change accepted or declined invites.
Tests must also prove the scheduler computes day boundaries using the configured expiration duration, and `Stop()` cancels the pending timer callback.

- [ ] **Step 5: Implement expiry scheduler**

Use the same `time.AfterFunc` self-rescheduling approach as browse history cleanup unless a shared helper already exists by implementation time.
Expose `Stop()` on the scheduler and make it idempotent.

- [ ] **Step 6: Wire scheduler**

In `backend/cmd/server/main.go`, save the instance:

```go
collabInviteExpiry := scheduler.NewCollabInviteExpiry(db, &cfg.Collaboration)
collabInviteExpiry.Start()
```

During graceful shutdown, after `stopWorkers()` and before closing database/Redis connections, call:

```go
collabInviteExpiry.Stop()
```

Do not start this scheduler as an anonymous temporary value; otherwise the process cannot stop its timer on shutdown.

- [ ] **Step 7: Verify**

Run:

```powershell
cd backend
go test ./internal/service -run "TestCollabInviteAccept|TestCollabInviteDecline" -v
go test ./internal/pkg/scheduler -run TestCollabInviteExpiry -v
```

---

## Task 4: Add Handler Routes And User Setting Contract

**Files:**
- Create: `backend/internal/handler/collab_invite.go`
- Modify: `backend/internal/handler/routes.go`
- Modify: `backend/internal/handler/user.go`
- Modify: `backend/internal/handler/auth.go`
- Test: `backend/internal/handler/collab_invite_test.go`

- [ ] **Step 1: Add failing route tests**

Routes:

```text
POST /api/v1/contents/:id/collab-invites
POST /api/v1/collab-invites/:id/accept
POST /api/v1/collab-invites/:id/decline
```

Also test `PATCH /api/v1/users/:id` can update `accept_collab_invites` for the current user, and `GET /api/v1/auth/me` returns it.

- [ ] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/handler -run TestCollabInvite -v
```

- [ ] **Step 3: Implement handlers**

Request:

```json
{"invitee_id":42}
```

Response:

```json
{"invite": {"id": 1, "status": "pending", "message_id": 10}}
```

Map service errors to exact HTTP/error codes from the design spec.

- [ ] **Step 4: Register routes**

`POST /contents/:id/collab-invites` must use auth and reputation interaction guard.

`POST /collab-invites/:id/accept|decline` must use auth.

- [ ] **Step 5: Verify handler**

Run:

```powershell
cd backend
go test ./internal/handler -run TestCollabInvite -v
```

---

## Task 5: Build Invite Card And Settings UI

**Files:**
- Read: `design/ui-spec.md`
- Create: `frontend/components/social/CollabInviteCard.tsx`
- Modify: `frontend/components/social/ChatWindow.tsx`
- Modify: `frontend/components/social/ConversationList.tsx`
- Modify: `frontend/contexts/AuthContext.tsx`
- Modify: `frontend/app/(protected)/settings/page.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: `frontend/tests/collab-invite-card.test.tsx`
- Test: `frontend/tests/settings-collab-invites.test.tsx`

- [ ] **Step 1: Confirm UI spec before component code**

Run:

```powershell
rg -n "## Component: CollabInviteCard|## Component: CollabUserPicker|## Page: /settings|## Page: /messages" design/ui-spec.md
```

Expected: invite card, user picker, `/settings`, and `/messages` sections are present. `/settings` must describe the `accept_collab_invites` switch, saving state, i18n keys, and screenshot checkpoints. If a future branch lacks one, stop and repair UI spec in an explicitly scoped docs/design step before UI work.

- [ ] **Step 2: Add failing invite-card tests**

Cover:

- pending card shows accept and decline buttons
- accepted card is read-only
- declined card is read-only
- expired card is gray/read-only
- accept button calls `/api/v1/collab-invites/:id/accept`
- decline button calls `/api/v1/collab-invites/:id/decline`
- local card state updates from returned invite DTO

- [ ] **Step 3: Implement card rendering**

`ChatWindow` must branch on `message.msg_type === "collab_invite"` and render `CollabInviteCard`. Normal text messages continue to render as bubbles.

- [ ] **Step 4: Add settings switch tests**

Assert settings page:

- initializes from `user.accept_collab_invites`
- sends PATCH with `accept_collab_invites`
- calls `refreshUser()` after save
- rolls back to the previous server value and shows localized feedback if save fails
- keeps password/delete-account settings unaffected while the collaboration switch is saving

- [ ] **Step 5: Implement settings UI**

Use an existing `Switch` component. No hardcoded visible strings.
The switch belongs in its own "联合创作邀请" settings group described by `design/ui-spec.md`; do not hide it inside dangerous actions or password settings.

- [ ] **Step 6: Run frontend tests**

Run:

```powershell
cd frontend
node --import tsx --test tests/collab-invite-card.test.tsx tests/settings-collab-invites.test.tsx
```

---

## Task 6: Confirm Collab Picker API And UI Spec

**Files:**
- Read: `design/ui-spec.md`
- Create: `frontend/components/content/CollabUserPicker.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: `frontend/tests/publish-collab-picker.test.tsx`

- [ ] **Step 1: Confirm UI spec before picker code**

Run:

```powershell
rg -n "## Component: CollabUserPicker|## Page: /studio/publish/fanwork" design/ui-spec.md
```

`CollabUserPicker` must be defined as an additive PublishForm section. For fanwork, it appears below source fields; for original, it appears after the main content fields and before submit actions. It must not alter the `SourceContentPicker` layout from source-linkage.

- [ ] **Step 2: Add failing picker tests**

Assert:

- picker searches users by username
- picker calls exactly `GET /api/v1/users/search?q=<query>&limit=8`
- picker result normalization keeps only `id`, `username`, and `avatar_url`
- picker drops results without numeric `id` or non-empty `username`
- selected collaborators can be removed
- duplicate selected users cannot be added
- loading, empty, and error states render localized text

- [ ] **Step 3: Implement `CollabUserPicker`**

Use existing API search route `GET /api/v1/users/search?q=<query>&limit=8`. The backend route already exists and returns safe search results; do not add a new user search endpoint for this plan.

- [ ] **Step 4: Add i18n keys**

Add visible strings under `collab.userPicker.*`.

- [ ] **Step 5: Run picker tests**

Run:

```powershell
cd frontend
node --import tsx --test tests/publish-collab-picker.test.tsx
```

Expected: picker-only tests PASS before editing `PublishForm.tsx`.

---

## Task 7: Add Collab Picker To Publish Flow

**Files:**
- Modify: `frontend/components/studio/PublishForm.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: `frontend/tests/publish-collab-picker.test.tsx`
- Test: `frontend/e2e/collab-invite-flow.spec.ts`

- [ ] **Step 1: Add failing publish-flow tests**

Assert:

- after content creation succeeds, one invite request is sent per selected user
- invite failure is surfaced without marking publish itself failed
- duplicate selected users cannot be added in the publish form integration
- original publish and fanwork publish both support selected collaborators
- source-linkage fields still submit exactly one of `source_original_id` / `source_fanwork_id`
- collaborator picker sits after source fields in fanwork publish and before submit actions in both zones

- [ ] **Step 2: Modify publish payload flow**

Current content creation returns a content ID. After successful `POST /api/v1/contents`, call:

```text
POST /api/v1/contents/:id/collab-invites
```

for each selected invitee.

Do not block content creation if one invite fails after content was created. Show a warning toast listing failed usernames.

- [ ] **Step 3: Run tests**

Run:

```powershell
cd frontend
node --import tsx --test tests/publish-collab-picker.test.tsx
```

---

## Task 8: Full Verification And Documentation Sync

**Files:**
- Modify if generated: `architecture.md`
- Screenshot outputs listed in Step 4.

- [ ] **Step 1: Run backend gates**

Run:

```powershell
cd backend
go test ./internal/service -run TestCollabInvite -v
go test ./internal/handler -run TestCollabInvite -v
go test ./internal/pkg/scheduler -run TestCollabInviteExpiry -v
go test ./...
go vet ./...
go build ./...
```

- [ ] **Step 2: Run frontend gates**

Run:

```powershell
cd frontend
npm run test
npm run lint
npm run build
npx playwright test e2e/collab-invite-flow.spec.ts
```

- [ ] **Step 3: Run doc-validator**

Because this plan changes `config.go`, migrations, and routes:

```powershell
cd tools/doc-validator
go run . --fix
```

- [ ] **Step 4: Browser verification**

1. Creator publishes content with a selected collaborator.
2. Invitee opens `/messages` and sees pending invite card.
3. Invitee accepts; card becomes accepted and contributor row exists.
4. Invitee declines another invite; card becomes declined.
5. Force an expired invite; card is read-only.
6. User toggles settings switch, saves, refreshes, and sees persisted value.
7. Save screenshots:
   - `screenshots/community-collab-picker-desktop.png`
   - `screenshots/community-collab-picker-mobile.png`
   - `screenshots/community-collab-invite-pending.png`
   - `screenshots/community-collab-invite-states.png`
   - `screenshots/community-collab-invite-mobile.png`
   - `screenshots/community-collab-settings-desktop.png`
   - `screenshots/community-collab-settings-mobile.png`

- [ ] **Step 5: Commit when implementing**

```powershell
git add -- backend/migrations/061_collaboration_invites.sql backend/internal/model/collab_invite.go backend/internal/model/user.go backend/internal/model/notification.go backend/internal/repository/collab_invite_repo.go backend/internal/repository/message_repo.go backend/internal/repository/content_repo.go backend/internal/service/collab_invite_service.go backend/internal/service/collab_invite_service_test.go backend/internal/handler/collab_invite.go backend/internal/handler/collab_invite_test.go backend/internal/handler/user.go backend/internal/handler/auth.go backend/internal/handler/routes.go backend/internal/pkg/scheduler/collab_invite_expiry.go backend/internal/pkg/scheduler/collab_invite_expiry_test.go backend/config/config.go backend/config.yaml backend/cmd/server/main.go frontend/components/content/CollabUserPicker.tsx frontend/components/social/CollabInviteCard.tsx frontend/tests/collab-invite-card.test.tsx frontend/tests/settings-collab-invites.test.tsx frontend/tests/publish-collab-picker.test.tsx frontend/e2e/collab-invite-flow.spec.ts frontend/contexts/AuthContext.tsx "frontend/app/(protected)/settings/page.tsx" frontend/components/studio/PublishForm.tsx frontend/components/social/ChatWindow.tsx frontend/components/social/ConversationList.tsx frontend/messages/zh.json frontend/messages/en.json screenshots/community-collab-picker-desktop.png screenshots/community-collab-picker-mobile.png screenshots/community-collab-invite-pending.png screenshots/community-collab-invite-states.png screenshots/community-collab-invite-mobile.png screenshots/community-collab-settings-desktop.png screenshots/community-collab-settings-mobile.png docs/superpowers/plans/2026-06-30-omnicraft-community-collaboration-invites.md progress.txt
# Also add architecture.md if doc-validator --fix modified it during this task.
git commit -m "Community 6: collaboration invites"
```

---

## Plan Self-Check

- [ ] Dependency on message-system correction is explicit.
- [ ] Dependency on source-linkage `PublishForm.tsx` changes is explicit.
- [ ] Migration number is `061`, after source-linkage `060`.
- [ ] Anti-abuse chain lists all seven checks and exact error codes.
- [ ] Redis keys and TTL are specified.
- [ ] Redis daily key date uses Asia/Shanghai and tests cover midnight boundaries.
- [ ] Redis reservation and DB failure compensation behavior is specified and tested.
- [ ] Accept path uses independent idempotent contributor insert and does not increment `pr_count`.
- [ ] Invite metadata is explicitly safe and minimal.
- [ ] User setting spans migration, model, PATCH, auth/me, AuthContext, and settings UI.
- [ ] `CollabUserPicker` uses existing `GET /api/v1/users/search?q=<query>&limit=8` and only safe user fields.
- [ ] Scheduler expiration and re-invite partial unique index are both covered.
- [ ] `main.go` stores the collaboration expiry scheduler instance and calls `Stop()` during graceful shutdown.
- [ ] Browser verification covers pending, accepted, declined, expired, and settings states.
