# OmniCraft UI Polish Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变业务语义的前提下，修复已证实的设计令牌、交互反馈、可访问性、响应式、国际化和用户错误暴露问题，并建立可重复的 UI 治理门。

**Architecture:** 先修正设计权威与基础组件，再按互不重叠的页面所有权并行整改，最后把静态检查接入项目级验证入口。每个文件只归一个 Task；跨 Task 的新文案集中到 i18n Task，跨前后端的信誉能力契约单独处理。

**Tech Stack:** Next.js 16.2 / React 19 / TypeScript 5 / Tailwind CSS 4 CSS-first / next-intl / Base UI / Playwright / Node test / PowerShell.

---

## Status And Authority

**Audit date:** 2026-07-17  
**Status:** reviewed draft; no implementation Task has started.

- This is a proposed Mode D UI hardening source. Before implementation, add this plan to the Mode D source list in `AGENTS.md`; until then, a future agent cannot reliably select it without an explicit user instruction.
- The production-readiness specification and plan named for the same review were not present in the audited worktree. Do not resolve security/release-policy conflicts by inference; re-run the coordination review after those files are restored.
- Visual authority order: `design/design-system.md`, then the matching `## Component:` / `## Page:` section in `design/ui-spec.md`, then this plan. If an implementation needs a new token, update the design-system table in U-01 before consuming it.
- Base branch observed during review: `codex/productization-integration` and `main` both at `b1aea75`. Re-check before creating any worktree.
- Branch format: `codex/excellence/u-XX`. One Task equals one worktree, one branch, one reviewed commit, and one progress entry.
- Do not modify `task.json`, Beta/community checkboxes, or Web Agent completion state.

## Verified Baseline

The former severity totals were removed because their cited “three audit appendices” were not present. The following facts are reproducible from the current repository; counts are inventory signals, not automatic replacement targets.

| Finding | Verified evidence on 2026-07-17 | Planning consequence |
|---|---|---|
| Tailwind setup | Tailwind 4 is configured through `frontend/app/globals.css`; no `tailwind.config.*` exists | Do not create or edit a fictitious Tailwind config |
| Radius/font contract drift | CSS derives 4.8/6.4/11.2px radii from `--radius`, while the design system requires 3/4/8/12px; the CSS sans stack omits the documented Chinese fallbacks | U-01 owns the authority and CSS correction |
| Missing/undefined tokens | `--border-destructive` and `--radius-full` are documented but not emitted; `IPBrowseClient` consumes undefined `--accent-hover` | Emit documented tokens; replace the undefined hover with an existing authoritative token instead of inventing one silently |
| Palette utility inventory | The command below finds 287 palette-class matches in 42 TS/TSX/CSS files, including 24 legacy white/gray matches in 10 files | Record command + before/after count per Task; replace only semantically verified matches |
| Arbitrary font sizes | 95 `text-[…]` matches in 30 TS/TSX/CSS files | Do not promise a blanket conversion; preserve intentional compact metrics and prove each changed surface |
| Native confirms/prompts | 7 calls in 5 files | U-04 and U-11 own all seven |
| Raw JSX SVG icons | 5 instances in 4 component files; generated placeholder SVG strings are intentional and excluded | U-06/U-09/U-02 own the four files; do not rewrite `coverPlaceholder.ts` |
| Confirm modal | Existing component already supports `requireReason`, dialog semantics, focus trap, Esc, and focus restoration | Test and preserve these behaviors; do not plan them as new features |
| Loading/empty state | 15 `common.loading` usages exist; `EmptyState` is already used by several list surfaces | Replace only page-level naked loading states; inline loading labels remain valid |
| Error exposure | Direct user rendering is verified in `app/error.tsx`, verify-email, and `DownloadButton`; console logging in utilities is not user exposure | U-05/U-11 use a precise allowlist instead of a stale “nine files” claim |
| Reputation threshold | Four interaction components hard-code `3`; the public-config contract explicitly rejects `reputation`, `threshold`, and `min_score` fields | U-11 must use a server-derived capability, not a frontend config hook |
| CI integration | No repository GitHub workflow exists; `scripts/verify-project.ps1` is the project gate | U-12 integrates `npm run lint:ui` into the project verifier instead of writing `frontend/.github/**` |

Baseline command for palette inventory:

```powershell
$pattern = '(?:bg|text|border|ring|from|via|to)-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-'
rg -o --glob '!node_modules/**' --glob '!package-lock.json' --glob '*.{ts,tsx,css}' $pattern frontend
```

## Coordination And Parallelism

Tasks may run in parallel only after all listed dependencies are merged into the current integration base and the reservation ledger has no active overlap.

```text
Wave 1:  U-01 U-05                              (parallel; disjoint foundations)
Wave 2:  U-02                                   (depends on U-01)
Wave 3:  U-03 U-04 U-06 U-07 U-08 U-09 U-10 U-11
                                                  (parallel; U-11 also waits for restored parent docs)
Wave 4:  U-12                                   (depends on U-01 through U-11)
```

Maximum useful parallelism is two workers in Wave 1 and up to the available worker limit in Wave 3. U-01, U-02, and U-12 are serialization points. A Task may not edit another Task's files “for convenience”; report the needed path/symbol/reason and transfer ownership in the reservation ledger first.

## Common Execution And Verification Contract

Every Task follows this sequence:

- [ ] Reserve exact files in `docs/working/2026-07-18-ui-polish-file-reservation.md` with real owner, branch, base commit, and timestamp.
- [ ] Read every relevant component/page section in `design/ui-spec.md` in addition to the task files.
- [ ] Start required services from the task worktree; if the deterministic test uses mocked API routes, record that explicitly.
- [ ] Write a failing unit/contract test that demonstrates the specific defect. A screenshot alone is not the red test.
- [ ] Confirm the expected failure, implement the minimum correction, then refactor.
- [ ] Run focused tests, `npm.cmd run lint`, `npm.cmd test`, and `npm.cmd run build`.
- [ ] For visible changes, exercise keyboard and pointer interaction in Playwright and save deterministic screenshots under `screenshots/ui-polish/u-XX/`.
- [ ] Run specification-conformance review, then code-quality review; fix and re-review all correctness, security, scope, and contract findings.
- [ ] Update only this Task's checkboxes and `progress.txt`; stage exact files and create one commit.

Project-level `go test ./...`, `go vet ./...`, and `go build ./...` are required for U-11 and U-12 and for any other Task that touches backend or shared verification scripts.

## Task U-01: Align Design Authority And CSS Tokens

**Depends on:** none  
**Files:**
- Modify: `design/design-system.md`
- Modify: `frontend/app/globals.css`
- Create: `frontend/scripts/ui-governance/check-tokens.mjs`
- Create: `frontend/scripts/ui-governance/check-tokens.test.mjs`

- [ ] Write tests that parse the authoritative tables and assert emitted light/dark values, exact 3/4/8/12/full radii, Chinese sans fallbacks, modal-only shadow tokens, and absence of undefined token references.
- [ ] Confirm the tests fail against current CSS.
- [ ] Emit `--border-destructive` and `--radius-full`; replace computed radius drift with exact values; align font stacks.
- [ ] Resolve `--accent-hover` by using an already-authoritative semantic value or documenting a new value in the design system before CSS consumption.
- [ ] If semantic success/warning/info tokens are introduced, specify both foreground and subtle-background light/dark values in the design system; do not add unnamed color constants only to satisfy a checker.
- [ ] Verify the token test, frontend build, home/search/admin/studio light+dark screenshots, and no unexpected layout shift.

## Task U-02: Harden Shared UI Primitives

**Depends on:** U-01  
**Files:**
- Modify: `frontend/components/ui/badge.tsx`
- Modify: `frontend/components/ui/button.tsx`
- Modify: `frontend/components/ui/card.tsx`
- Modify: `frontend/components/ui/checkbox.tsx`
- Modify: `frontend/components/ui/confirm-modal.tsx`
- Modify: `frontend/components/ui/dropdown-menu.tsx`
- Modify: `frontend/components/ui/empty-state.tsx`
- Modify: `frontend/components/ui/field.tsx`
- Modify: `frontend/components/ui/input.tsx`
- Modify: `frontend/components/ui/label.tsx`
- Modify: `frontend/components/ui/select.tsx`
- Modify: `frontend/components/ui/separator.tsx`
- Modify: `frontend/components/ui/skeleton.tsx`
- Modify: `frontend/components/ui/switch.tsx`
- Modify: `frontend/components/ui/tabs.tsx`
- Modify: `frontend/components/ui/TagBadge.tsx`
- Modify: `frontend/components/ui/textarea.tsx`
- Modify: `frontend/components/ui/Toast.tsx`
- Create: `frontend/tests/ui-primitives-accessibility.test.tsx`

- [ ] Add failing tests for Button accessible names, icon-button sizing, disabled semantics, ConfirmModal reason validation/focus/Esc/restoration, Toast live-region/close labels, and documented z-index/shadow rules.
- [ ] Preserve existing ConfirmModal behavior rather than reimplementing it.
- [ ] Align primitive radii, colors, focus rings, shadows, and semantic status colors with U-01.
- [ ] Apply 44px mobile/coarse-pointer targets to icon-only or isolated actions without globally inflating dense desktop controls; WCAG 2.2 AA 24px is the minimum fallback.
- [ ] Replace the raw remove SVG in `TagBadge.tsx` with the established icon system and an accessible name.
- [ ] Verify primitive unit tests plus keyboard-only ConfirmModal and Toast Playwright flows.

## Task U-03: Unify Navigation Shells

**Depends on:** U-01, U-02, U-05  
**Files:**
- Modify: `frontend/components/layout/Header.tsx`
- Modify: `frontend/components/layout/Sidebar.tsx`
- Modify: `frontend/components/studio/StudioSidebar.tsx`
- Modify: `frontend/app/(protected)/admin/layout.tsx`
- Create: `frontend/lib/use-sidebar-collapse.ts`
- Create: `frontend/tests/navigation-shells.test.tsx`

- [ ] Test distinct persistence keys, 228/48px desktop states, accessible toggle labels, tooltip/title behavior, keyboard operation, Header `z-40`, and mobile navigation/drawer reachability.
- [ ] Extract only collapse-state behavior; keep public, studio, and admin information architectures separate where their navigation models differ.
- [ ] Replace the admin 220/52px drift and horizontal mobile overflow with the approved mobile navigation pattern.
- [ ] Verify public/studio/admin screenshots at 375, 768, 1024, and 1440 CSS pixels.

## Task U-04: Replace Native Destructive Dialogs

**Depends on:** U-01, U-02, U-05  
**Files:**
- Modify: `frontend/app/(protected)/dashboard/contents/page.tsx`
- Modify: `frontend/app/(protected)/dashboard/contributors/page.tsx`
- Modify: `frontend/app/(protected)/dashboard/pr-requests/page.tsx`
- Modify: `frontend/app/(protected)/settings/page.tsx`
- Modify: `frontend/app/(protected)/admin/agent-config/page.tsx`
- Modify: `frontend/components/content/VersionHistory.tsx`
- Create: `frontend/tests/destructive-dialogs.test.tsx`

- [ ] Test every current `window.confirm`/`window.prompt` path except ReactionBar, which belongs to U-11.
- [ ] Use ConfirmModal `requireReason` for PR rejection and any other domain action whose API requires a reason; do not add reason collection to unrelated confirmations.
- [ ] Preserve cancellation, busy, success, failure, and retry behavior; API failures must remain visible and must not close the modal as if successful.
- [ ] Verify `rg -n 'window\.(confirm|prompt)\('` reports only the U-11-owned ReactionBar call before U-11 merges and zero afterward.

## Task U-05: Make User-Facing Errors Safe And Consistent

**Depends on:** none  
**Files:**
- Modify: `frontend/lib/error-handler.ts`
- Modify: `frontend/lib/user-facing-error.ts`
- Modify: `frontend/app/error.tsx`
- Modify: `frontend/app/(public)/verify-email/page.tsx`
- Modify: `frontend/app/(public)/user/[userId]/UserProfileClient.tsx`
- Modify: `frontend/components/content/ContentDetail.tsx`
- Modify: `frontend/components/feedback/FeedbackForm.tsx`
- Modify: `frontend/tests/error-handler-i18n.test.ts`
- Create: `frontend/tests/user-facing-error-surfaces.test.tsx`

- [ ] Write failing tests proving raw backend/exception messages are never rendered, known API codes map to translation keys, and unknown errors use a safe fallback.
- [ ] Keep expected background/polling failures silent; do not convert every catch into a Toast.
- [ ] Provide a small reusable mapper/helper only where it reduces duplicated policy. Avoid a wrapper that hides control flow or swallows rejected promises.
- [ ] Verify error boundary, verify-email, profile, detail, and feedback normal/error paths.

## Task U-06: Correct Detail-Level Loading, Empty, And Icon States

**Depends on:** U-01, U-02, U-05  
**Files:**
- Modify: `frontend/app/(protected)/feedback/[feedbackId]/page.tsx`
- Modify: `frontend/app/(public)/ip/[ipId]/discussions/[discussionId]/page.tsx`
- Modify: `frontend/components/content/SheetMusicViewer.tsx`
- Modify: `frontend/components/judge/VerdictDetail.tsx`
- Create: `frontend/tests/detail-state-contracts.test.tsx`

- [ ] Test loading, not-found/empty, retriable error, and success states without layout collapse.
- [ ] Replace raw JSX vote icons in VerdictDetail; keep generated data-URI placeholder SVGs outside scope.
- [ ] Use inline spinner text only for local operations; use documented skeletons for page/detail fetches.
- [ ] Verify keyboard and screen-reader status announcements plus desktop/mobile screenshots.

## Task U-07: Standardize Paginated List Surfaces

**Depends on:** U-01, U-02, U-05  
**Files:**
- Create: `frontend/components/ui/data-list.tsx`
- Create: `frontend/tests/data-list.test.tsx`
- Modify: `frontend/app/(protected)/history/page.tsx`
- Modify: `frontend/app/(protected)/appeals/page.tsx`
- Modify: `frontend/app/(protected)/rehab/page.tsx`
- Modify: `frontend/app/(protected)/feedback/mine/page.tsx`
- Modify: `frontend/app/(protected)/studio/overview/page.tsx`
- Modify: `frontend/app/(protected)/studio/followers/page.tsx`
- Modify: `frontend/app/(protected)/studio/contents/page.tsx`
- Modify: `frontend/app/(public)/ip/[ipId]/discussions/page.tsx`

- [ ] Define DataList as presentation/state composition only; API page size, cursor/page state, and fetching remain owned by each page.
- [ ] Test first load, empty, error+retry, next page, duplicate-request prevention, end-of-list, filter reset, and retained prior results during pagination failure.
- [ ] Do not call a hard-coded page size “configurable” unless a real prop or backend contract controls it.
- [ ] Preserve existing history tests and add focused tests for every newly paginated surface.

## Task U-08: Repair Form Accessibility In Remaining Surfaces

**Depends on:** U-01, U-02, U-05  
**Files:**
- Modify: `frontend/app/(protected)/settings/tag-groups/page.tsx`
- Modify: `frontend/app/(protected)/ip/[ipId]/discussions/new/page.tsx`
- Modify: `frontend/app/(protected)/admin/notifications/page.tsx`
- Modify: `frontend/app/(protected)/admin/categories/page.tsx`
- Modify: `frontend/app/(protected)/admin/queue/page.tsx`
- Modify: `frontend/app/(protected)/admin/ips/page.tsx`
- Modify: `frontend/app/(protected)/admin/feedback/page.tsx`
- Modify: `frontend/app/(protected)/admin/config/page.tsx`
- Create: `frontend/tests/form-accessibility.test.tsx`

- [ ] Test every control has an accessible name through explicit label, implicit label, aria-label, or aria-labelledby; do not require `htmlFor` when valid implicit association is used.
- [ ] Connect errors with `aria-describedby`, mark invalid controls, announce submission errors, and focus the first invalid field.
- [ ] Add accessible names to icon-only table actions and confirm 44px isolated mobile targets.
- [ ] Run an automated accessibility scan if an existing dependency supports it; otherwise use Testing Library roles/names plus Playwright keyboard evidence without adding an unapproved dependency.

## Task U-09: Normalize Content And Discovery Visuals

**Depends on:** U-01, U-02, U-05  
**Files:**
- Modify: `frontend/components/content/ContentCard.tsx`
- Modify: `frontend/components/content/ContentSidebar.tsx`
- Modify: `frontend/components/home/HomePageClient.tsx`
- Modify: `frontend/components/layout/FacetedSearchSidebar.tsx`
- Modify: `frontend/components/ip/IPBrowseClient.tsx`
- Modify: `frontend/app/(public)/search/page.tsx`
- Create: `frontend/tests/content-discovery-visual-contracts.test.tsx`

- [ ] Replace the two raw JSX SVG icons in ContentCard/ContentSidebar and the undefined accent hover in IPBrowseClient.
- [ ] Convert arbitrary sizes/colors only when the matching UI spec provides an authoritative replacement; record justified exceptions.
- [ ] Test original/fanwork card distinctions, responsive facet behavior, loading/empty/error results, dark contrast, and keyboard operation.
- [ ] Capture deterministic home/original/search/content screenshots in both themes.

## Task U-10: Align Communication Surfaces And I18n

**Depends on:** U-01, U-02, U-05  
**Files:**
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Modify: `frontend/app/layout.tsx`
- Modify: `frontend/app/(protected)/layout.tsx`
- Modify: `frontend/app/(protected)/messages/page.tsx`
- Modify: `frontend/components/social/ConversationList.tsx`
- Modify: `frontend/components/social/ChatWindow.tsx`
- Modify: `frontend/components/social/NotificationDropdown.tsx`
- Modify: `frontend/components/agent/AgentChatWidget.tsx`
- Modify: `frontend/components/agent/ComplianceCheckBadge.tsx`
- Create: `frontend/scripts/ui-governance/check-i18n-parity.mjs`
- Create: `frontend/scripts/ui-governance/check-i18n-parity.test.mjs`
- Create: `frontend/tests/communication-surfaces.test.tsx`

- [ ] Test exact zh/en key parity, non-empty leaf values, safe placeholder parity, and documented namespaces.
- [ ] Replace verified visible hard-coded copy, including the root skip link, without scanning source-code identifiers as false positives.
- [ ] Fix accent-on-white contrast and notification/chat status semantics using U-01 tokens.
- [ ] Preserve unread polling/SSE/message behavior and verify empty/error/reconnect/keyboard paths.

## Task U-11: Replace Frontend Reputation Constants With Server Capabilities

**Depends on:** U-01, U-02, U-05, and restored production-readiness coordination inputs  
**Files:**
- Modify: `backend/internal/handler/auth.go`
- Modify: `backend/internal/handler/auth_test.go`
- Modify: `backend/internal/handler/auth_cookie_test.go`
- Modify: `frontend/contexts/AuthContext.tsx`
- Modify: `frontend/components/social/ReactionBar.tsx`
- Modify: `frontend/components/social/CommentSection.tsx`
- Modify: `frontend/components/content/DownloadButton.tsx`
- Modify: `frontend/app/(protected)/judge/queue/page.tsx`
- Modify: `frontend/app/(protected)/judge/exam/page.tsx`
- Create: `frontend/tests/interaction-capabilities.test.tsx`

- [ ] First reconcile this contract with the restored production-readiness documents. The current public-config endpoint intentionally rejects threshold/min-score fields.
- [ ] Add a server-derived authenticated capability such as `capabilities.can_interact` (and `can_publish` only if required), computed from the configured threshold and ban/freeze state; do not expose the raw threshold through public config.
- [ ] Test login and `/auth/me` consistency, reputation recovery, banned users, publish freeze, and backward-compatible user fields.
- [ ] Consume the capability in ReactionBar, comments, downloads, and judge pages; use a non-numeric localized reason unless the server explicitly returns a safe display value.
- [ ] Replace ReactionBar's native prompt with ConfirmModal reason input and test success/cancel/failure behavior.
- [ ] Run focused backend tests, `go test ./...`, `go vet ./...`, `go build ./...`, frontend tests, lint, and build.

## Task U-12: Add Enforceable UI Governance Gates

**Depends on:** U-01 through U-11  
**Files:**
- Modify: `frontend/package.json`
- Create: `frontend/scripts/ui-governance/run-all.mjs`
- Create: `frontend/scripts/ui-governance/run-all.test.mjs`
- Create: `frontend/scripts/ui-governance/check-source-policy.mjs`
- Create: `frontend/scripts/ui-governance/check-source-policy.test.mjs`
- Modify: `scripts/verify-project.ps1`
- Modify: `scripts/verify-project.tests.ps1`
- Modify: `frontend/playwright.config.ts` only if new deterministic visual projects are required

- [ ] Add `lint:ui` without removing existing package scripts; run token, i18n parity, native-dialog, undefined-token, and narrowly scoped source-policy checks.
- [ ] Prefer AST/structured checks or explicit allowlists. Regex checks for hard-coded text, aria labels, touch targets, and colors must begin as measured reports until false positives and intentional exceptions are encoded.
- [ ] Add a failing verifier contract test, then invoke `npm.cmd run lint:ui` from `scripts/verify-project.ps1`.
- [ ] Do not claim real Safari coverage from Playwright WebKit. Record Chromium/Firefox/WebKit engine results separately from any manual Safari release evidence.
- [ ] Run the complete project gate and mocked browser contracts. Release/browser and external-service evidence remain separate requirements.

## Definition Of Done

- [ ] U-01 through U-12 are complete, individually reviewed twice, rebased/verified against the current integration base, and represented by one commit each.
- [ ] No native confirm/prompt remains in production TS/TSX code.
- [ ] No raw backend or exception message is rendered to users in the audited allowlist.
- [ ] Shared primitives and changed surfaces meet keyboard, focus, accessible-name, error-announcement, and mobile target requirements.
- [ ] Changed surfaces use only authoritative tokens; intentional palette/arbitrary-size exceptions are documented by file and reason.
- [ ] `npm.cmd run lint:ui`, frontend unit/lint/build, project verification, and required focused backend gates pass.
- [ ] Deterministic screenshots exist for home, original feed, content detail, studio overview, admin, messages, light/dark, and 375/768/1024/1440 viewports. Pixel-diff thresholds are required only if U-12 actually introduces stable snapshot tooling and baselines.
- [ ] Production-readiness coordination is re-reviewed after the missing parent documents are restored; unresolved security/release conflicts keep the affected Task unchecked.

## Non-Goals

- Do not redesign information architecture, introduce new product features, or change API pagination semantics solely for visual consistency.
- Do not replace generated data-URI placeholder SVGs with icon components.
- Do not expose the full backend reputation/config object to the browser.
- Do not create a GitHub Actions path under `frontend/`; the repository-level verifier is the current enforceable integration point.
- Do not update unrelated Mode A/B/C or Web Agent completion state.
