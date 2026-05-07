# OmniCraft Code Review Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:requesting-code-review to review each batch. If later executing remediation work, use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a repeatable batch code-review workflow for OmniCraft that maps review coverage to `task.json`, records results in `code_review.md`, and avoids reviewing the same task twice without a reason.

**Architecture:** Treat `task.json` as the source of review scope, but do not review all 86 tasks as one unit. Review by dependency-aware subsystem batches, using baseline verification commands and targeted file inspection for each batch, then append structured evidence to `code_review.md`.

**Tech Stack:** Go/Gin/GORM/PostgreSQL backend, Next.js frontend, Tauri client, Docker Compose, PowerShell/Node for task metadata extraction, MCP Playwright or Codex in-app Browser Playwright for UI verification.

---

## Current Project Snapshot

- Project root: `C:\Users\16278\Desktop\file\code\project\OmniCraft`
- Task source: `C:\Users\16278\Desktop\file\code\project\OmniCraft\task.json`
- Existing progress log: `C:\Users\16278\Desktop\file\code\project\OmniCraft\progress.txt`
- Planned review log: `C:\Users\16278\Desktop\file\code\project\OmniCraft\code_review.md`
- Task count: 86 total, 62 marked `passes: true`, 24 marked `passes: false`
- Current git state at planning time: branch `main`, ahead of `origin/main` by 14 commits, with uncommitted changes in:
  - `backend/config/config.go`
  - `backend/internal/handler/agent.go`
  - `backend/internal/handler/routes.go`
  - `backend/internal/service/agent_service.go`

## Review Strategy Decision

Do not review one task at a time for the whole project. That would create 86 review sessions, repeat shared architecture checks, and bury cross-task integration bugs.

Do not review the whole repository in one pass either. The repository spans backend, frontend, Tauri, deployment, database migrations, and UI flows; a full pass would be too broad to produce actionable findings tied to `task.json`.

Use a hybrid batch strategy:

- Review foundational completed backend and schema tasks in dependency batches.
- Review completed frontend/UI tasks in route or workflow batches, with browser verification for UI-related batches.
- Review completed cross-stack tasks as end-to-end workflow batches.
- Do not perform full implementation review for tasks still marked `passes: false`; record them as pending/unreviewed unless the review goal is specifically to diagnose why they are incomplete.
- Always record reviewed task IDs and git SHAs in `code_review.md` before moving to the next batch.

## `code_review.md` Record Format

Create `code_review.md` with a stable top-level index and append-only entries:

```markdown
# OmniCraft Code Review Log

## Coverage Index

| Task IDs | Title / Batch | Status | Last Reviewed At | Base SHA | Head SHA | Notes |
|---|---|---|---|---|---|---|

## Review Entries

### YYYY-MM-DD HH:mm - Batch Name

**Task IDs:** 1, 2, 3
**Reviewer:** superpowers:requesting-code-review
**Base SHA:** ...
**Head SHA:** ...
**Scope:** files/modules reviewed
**Verification:** commands and browser/API checks run
**Result:** Pass / Pass with issues / Blocked / Pending

**Findings:**
- Critical:
- Important:
- Minor:

**Follow-up:**
- ...
```

Rules:

- A task is considered reviewed only when it appears in the Coverage Index with a timestamp and SHA range.
- If code changes after a review, re-review only the affected tasks and update the same Coverage Index row with the latest entry link.
- If a batch includes UI changes, the entry must include tested URL, interactions performed, screenshot path if captured, and console error check result.
- If a task remains `passes: false`, record `Pending` instead of inventing a review result.

## Batch Plan

### Batch 0: Review Baseline And Metadata

**Files:**
- Read: `C:\Users\16278\Desktop\file\code\project\OmniCraft\task.json`
- Read: `C:\Users\16278\Desktop\file\code\project\OmniCraft\progress.txt`
- Read: `C:\Users\16278\Desktop\file\code\project\OmniCraft\CLAUDE.md`
- Create/Modify: `C:\Users\16278\Desktop\file\code\project\OmniCraft\code_review.md`

- [ ] **Step 1: Capture git state**

Run:

```powershell
git status --short --branch
git rev-parse HEAD
```

Expected: record branch, dirty files, and `HEAD` in `code_review.md`.

- [ ] **Step 2: Extract task status**

Run:

```powershell
node -e "const fs=require('fs'); const j=JSON.parse(fs.readFileSync('task.json','utf8')); console.log(j.tasks.map(t=>`${t.id}\t${t.passes}\t${t.title}`).join('\n'))"
```

Expected: all 86 tasks printed with `passes` state.

- [ ] **Step 3: Initialize `code_review.md`**

Create the Coverage Index and first metadata entry. Mark tasks 35, 36, 37, 38, 39, 40, 41, 42, 46, 47, 48, 49, 50, 51, 59, 62, 64, 72, 73, 74, 76, 77, 83, 84 as `Pending` unless they are intentionally included for incomplete-work diagnosis.

### Batch 1: Backend Foundation, Config, Database, Deployment

**Task IDs:** 1, 2, 3, 4, 5, 43, 52, 63, 75

**Files:**
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\backend\config`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\backend\migrations`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\docker-compose.yml`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\nginx`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\pgbouncer`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\k8s`

- [ ] **Step 1: Run static backend checks**

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
```

- [ ] **Step 2: Review schema consistency**

Check migration order, table/column names against GORM models, foreign keys, indexes, and feature flag/config references.

- [ ] **Step 3: Request review**

Use `superpowers:requesting-code-review` with task IDs, implementation expectations from `task.json`, SHA range, and verification results.

- [ ] **Step 4: Update `code_review.md`**

Record findings and mark the batch status.

### Batch 2: Backend Auth, User, Moderation, Reputation, Admin

**Task IDs:** 6, 7, 8, 9, 10, 17, 18, 19, 40, 44, 45, 53, 54, 55, 56, 57, 58, 60, 61, 65, 66, 67, 68, 69, 78, 79, 80, 81, 82

**Files:**
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\backend\internal\handler`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\backend\internal\service`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\backend\internal\repository`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\backend\internal\middleware`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\backend\internal\pkg`

- [ ] **Step 1: Split internally if needed**

If review context is too large, split into:

- Auth/user/reputation/admin: 6, 7, 17, 18, 19, 60, 61, 68, 81
- Content moderation/search/Agent: 8, 9, 10, 44, 45, 53, 54, 55, 56, 57, 58, 82
- Notifications/social/course/category/support: 65, 66, 67, 69, 78, 79, 80

- [ ] **Step 2: Run focused tests**

```powershell
cd backend
go test ./internal/...
go vet ./...
go build ./...
```

- [ ] **Step 3: Request review**

Ask the reviewer to prioritize authorization bugs, trust-boundary issues, transaction boundaries, GORM query correctness, Redis/cache invalidation, external service fallback behavior, and missing tests.

- [ ] **Step 4: Update `code_review.md`**

Record whether Task 40 is pending or reviewed. At planning time it is still `passes: false`, so it should not be marked complete-review unless explicitly audited as an incomplete security task.

### Batch 3: Backend Content, PR, Versioning, Social, Cross-Stack Content Flows

**Task IDs:** 11, 12, 13, 14, 15, 16, 24, 27, 28, 85, 86

**Files:**
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\backend\internal\service\content_service.go`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\backend\internal\service\version_service.go`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\backend\internal\service\pr_service.go`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\frontend\app`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\frontend\components\content`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\frontend\components\pr`

- [ ] **Step 1: Verify backend behavior**

```powershell
cd backend
go test ./internal/service ./internal/handler
```

- [ ] **Step 2: Verify frontend type checks**

```powershell
cd frontend
npm run lint
```

- [ ] **Step 3: Browser verification for UI routes**

Start or reuse frontend dev server. Test content detail, PR actions, version history, browsing history, and original/fanwork source linkage routes that exist in the app.

- [ ] **Step 4: Request review and update log**

Include UI verification evidence required by root `AGENTS.md` instructions.

### Batch 4: Completed Frontend Public, Protected, Admin, Messaging Pages

**Task IDs:** 20, 21, 22, 23, 25, 26, 29, 30, 31, 32, 70, 71

**Files:**
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\frontend\app`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\frontend\components`
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\frontend\lib`
- Read as visual authority when relevant: `C:\Users\16278\Desktop\file\code\project\OmniCraft\design\ui-spec.md`

- [ ] **Step 1: Run frontend checks**

```powershell
cd frontend
npm run lint
npm run build
```

- [ ] **Step 2: Browser verification**

Use MCP Playwright or Codex in-app Browser Playwright. Verify page load, core interactions, visible render success, console errors, and screenshots for each representative workflow.

- [ ] **Step 3: Request review**

Ask reviewer to check Next.js App Router conventions, server/client component boundaries, TypeScript strictness, i18n usage, responsive behavior, and design-spec compliance.

- [ ] **Step 4: Update `code_review.md`**

Record tested URLs and interactions. Do not mark pending frontend tasks as reviewed in this batch.

### Batch 5: Tauri Client

**Task IDs:** 33, 34, 35

**Files:**
- Review: `C:\Users\16278\Desktop\file\code\project\OmniCraft\tauri-client`

- [ ] **Step 1: Separate completed and pending scope**

Task 33 and 34 are marked complete. Task 35 is marked pending, so review it only as incomplete-work diagnosis unless its status changes.

- [ ] **Step 2: Run checks**

```powershell
cd tauri-client
npm run build
cargo check --manifest-path src-tauri/Cargo.toml
```

- [ ] **Step 3: Request review**

Prioritize Tauri command permissions, allowlist/sandbox assumptions, filesystem access, sidecar process handling, and UI workflow consistency.

- [ ] **Step 4: Update `code_review.md`**

Record any platform/toolchain blockers explicitly.

### Batch 6: Pending Frontend And Quality Tasks

**Task IDs:** 36, 37, 38, 39, 41, 42, 46, 47, 48, 49, 50, 51, 59, 62, 64, 72, 73, 74, 76, 77, 83, 84

**Files:**
- Review after implementation: frontend routes/components named by each task
- Review after implementation: `C:\Users\16278\Desktop\file\code\project\OmniCraft\design\ui-spec.md`
- Review after implementation: deployment files for Task 42

- [ ] **Step 1: Record pending status**

Do not review these as completed work while `passes: false`.

- [ ] **Step 2: Prioritize next review order**

When tasks become complete, review in this order:

- Platform quality: 36, 37, 38, 39, 41, 42, 46, 47, 48
- Search/navigation/content UI: 49, 50, 51, 62, 64
- Agent and social/profile UI: 59, 72, 73, 74, 76, 77, 83, 84

- [ ] **Step 3: Use UI browser verification**

Every UI task in this batch must include browser verification before completion is claimed.

## Reviewer Prompt Template

Use this structure for each `superpowers:requesting-code-review` call:

```text
WHAT_WAS_IMPLEMENTED:
Review OmniCraft Batch <N>: <batch name>, covering task IDs <ids>.

PLAN_OR_REQUIREMENTS:
Use task.json entries for these task IDs as the acceptance criteria. Also enforce CLAUDE.md workflow rules and frontend/AGENTS.md browser verification rules for UI tasks.

BASE_SHA:
<base sha>

HEAD_SHA:
<head sha>

DESCRIPTION:
Audit implementation quality, correctness, security, tests, and user-visible behavior. Return findings by severity with file paths and line references. Explicitly state which task IDs are reviewed, which remain pending, and what verification evidence is missing.
```

## Quality Gates

Minimum gates before marking a completed-task batch as reviewed:

- Backend batch: `go test ./...`, `go vet ./...`, `go build ./...`
- Frontend batch: `npm run lint`, `npm run build`
- Tauri batch: `npm run build`, `cargo check --manifest-path src-tauri/Cargo.toml`
- UI batch: real browser page load, core interaction, screenshot, and console error check
- Cross-stack batch: backend API verification plus frontend workflow verification

If a gate cannot run because of local services, credentials, or environment mismatch, record the exact blocker in `code_review.md` and mark the batch `Blocked`, not `Pass`.

## Execution Handoff

Recommended execution order:

1. Create `code_review.md` index and record all pending task IDs.
2. Review Batch 0 to establish baseline.
3. Review Batch 1 and Batch 2 before frontend-heavy batches, because many UI tasks depend on backend contracts.
4. Review Batch 3 for cross-stack content flows.
5. Review Batch 4 for completed frontend workflows using browser verification.
6. Review Batch 5 for Tauri.
7. Revisit Batch 6 only after pending tasks are implemented.

For each batch, finish by appending to `code_review.md` before starting the next batch.
