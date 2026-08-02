# OmniCraft UI Polish Hardening Implementation Plan

> **For agentic workers:** Follow the active light-lane workflow in `AGENTS.md`; use the available UI/UX review and browser-testing skills for the relevant Task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变业务语义的前提下，修复已证实的设计令牌、交互反馈、可访问性、响应式、国际化和用户错误暴露问题，并建立可重复的 UI 治理门。

**Architecture:** 先完成不依赖视觉方向的错误安全、基础组件可访问性和破坏性操作修复，再用当前 Indigo 隔离原型验证推荐流、独立 Agent 工作台、共享内容详情浮层和重新设计的 IP 库。用户于 2026-07-28 确认生产二创页 `/`、原创页 `/original` 与 IP 详情 `/ip/[ipId]` 的现有 UI 可直接沿用，A2 不再重复构建这些页面；Hallmark Tally 方案 B 同样已取消。A2 只构建新的 IP 库原型，用户批准后才把视觉决策回写为设计 Token、共享组件和生产 `/ips`，随后推广到剩余页面并接入治理门。跨阶段重复文件必须在前一提交合入后显式转移所有权，禁止并发修改。

**Tech Stack:** Next.js 16.2 / React 19 / TypeScript 5 / Tailwind CSS 4 CSS-first / next-intl / Base UI / Playwright / Node test / PowerShell.

---

## Status And Authority

**Audit date:** 2026-07-17；execution order revised 2026-07-25；P-01 approved 2026-08-01

**Status:** active light-lane plan; U-05, U-02A, U-04, U-01, U-02B and U-03 are complete; P-01 approved on 2026-08-01 (IP library = variant B Indigo 精修), and U-06 is next.

- This plan is priority 1 in the `AGENTS.md` active-plan registry and uses the light lane. U-05, U-02A, U-04, U-01, U-02B and U-03 are complete; P-01 was explicitly approved by the user on 2026-08-01 with IP library variant B (Indigo 精修) as the final direction, so U-06 and later tasks are unblocked according to their listed dependencies. Numeric Task IDs remain stable identifiers; the wave order below, not numeric order, is authoritative for execution.
- Production Readiness is present and registered after this plan. Re-run coordination before U-11/U-12 or any edit that expands into config, release, verifier, or backend capability paths.
- Before P-01 approval, visual authority remains `design/design-system.md` then matching `design/ui-spec.md` sections; correctness Tasks must not restyle surfaces. After the user approved P-01 on 2026-08-01, the approved prototype decision record (`docs/working/2026-07-25-ui-prototype-review.md` §3/§6.3, with IP library variant B defined by `ip-library-b.css`) is the input for U-01/U-02B/U-09 updates to those authorities.
- Re-check the current `main` before implementation. The previous `codex/ui-refinement` branch was fully integrated and retired on 2026-07-25; create a fresh light-lane feature branch from the latest `main` when P-01 work resumes. Keep logical commits per Task/gate and preserve exact file reservations.
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
| Raw JSX SVG icons | 5 instances in 4 component files; generated placeholder SVG strings are intentional and excluded | U-06/U-09/U-02A own the four files; do not rewrite `coverPlaceholder.ts` |
| Confirm modal | Existing component already supports `requireReason`, dialog semantics, focus trap, Esc, and focus restoration | Test and preserve these behaviors; do not plan them as new features |
| Loading/empty state | 15 `common.loading` usages exist; `EmptyState` is already used by several list surfaces | Replace only page-level naked loading states; inline loading labels remain valid |
| Error exposure | Direct user rendering is verified in `app/error.tsx`, verify-email, PR requests, `VersionHistory`, and `DownloadButton`; console logging in utilities is not user exposure | U-05 must refresh and own the precise allowlist before U-04/U-11 receive overlapping files |
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
Wave 1:  U-05                                   safe user-facing errors
Wave 2:  U-02A                                  primitive behavior/accessibility only
Wave 3:  U-04                                   destructive-dialog correctness
Gate:    P-01                                   discovery/Agent/detail single-direction prototype + user approval
Wave 4:  U-01                                   approved design tokens and authority
Wave 5:  U-02B                                  approved primitive visuals
Wave 6:  U-03                                   navigation shells
Wave 7:  U-06 U-07 U-08 U-09 U-10 U-11         remaining surfaces; U-11 also waits for Production Readiness coordination
Wave 8:  U-12                                   enforceable governance gate
```

P-01 is a blocking human review gate: U-01, U-02B, U-03 and all visual propagation work must not start before explicit approval. Tasks in the same wave may run in parallel only when their reservations are disjoint. U-05 → U-04 and U-05 → U-11 intentionally transfer overlapping files after merge; U-02A → U-02B does the same for primitives. No concurrent ownership is allowed.

## Common Execution And Verification Contract

Every Task follows this sequence:

- [ ] Reserve exact files in `docs/working/2026-07-18-ui-polish-file-reservation.md` with real owner, branch, base commit, and timestamp.
- [ ] Read every relevant component/page section in `design/ui-spec.md` in addition to the task files.
- [ ] Start required services from the task branch; if the deterministic test uses mocked API routes, record that explicitly.
- [ ] Write a failing unit/contract test that demonstrates the specific defect. A screenshot alone is not the red test.
- [ ] Confirm the expected failure, implement the minimum correction, then refactor.
- [ ] Run focused tests, `npm.cmd run lint`, `npm.cmd test`, and `npm.cmd run build`.
- [ ] For visible changes, exercise keyboard and pointer interaction in Playwright and save deterministic screenshots under `screenshots/ui-polish/u-XX/`.
- [ ] Run specification-conformance review, then code-quality review; fix and re-review all correctness, security, scope, and contract findings.
- [ ] Update only this Task's checkboxes and `progress.txt`; stage exact files and create one commit.

Project-level `go test ./...`, `go vet ./...`, and `go build ./...` are required for U-11 and U-12 and for any other Task that touches backend or shared verification scripts.

## Maintenance N16-01: Restore Next 16 Page Build Contracts

**Reason:** The U-01 production build gate exposed pre-existing Next 16 page-module export and dynamic-route prop incompatibilities on the latest `main`. This is an independent light-lane prerequisite, not part of U-01's visual/token scope.

**Files:**
- Modify: `frontend/app/(protected)/settings/page.tsx`
- Create: `frontend/components/settings/VerificationReminderCard.tsx`
- Modify: `frontend/app/(public)/register/page.tsx`
- Modify: `frontend/app/(public)/forgot-password/page.tsx`
- Create: `frontend/components/auth/RegisterPageContent.tsx`
- Create: `frontend/components/auth/ForgotPasswordContent.tsx`
- Modify: `frontend/app/(public)/collections/[id]/page.tsx`
- Modify: `frontend/app/(public)/series/[id]/page.tsx`
- Modify: `frontend/app/(public)/user/[userId]/collections/page.tsx`
- Modify: the directly corresponding tests under `frontend/tests/`

- [x] Move testable named exports out of Page modules without changing rendering, API calls, validation, i18n or visual classes.
- [x] Align dynamic Page props with the Next 16 Promise-only contract and update direct-render tests to pass promised params.
- [x] Verify the focused 22 tests, full 112-test frontend suite, lint and `npm run build -- --webpack`.
- [x] Keep this maintenance change in a separate commit from U-01.

## Maintenance GOV-01: Ratify Approved Indigo Elevation Authority

**Reason:** U-01 exposed a direct conflict between the user-approved P-01 three-tier subtle-elevation direction and the constitution/UI-spec legacy global no-shadow rule. The highest visual authorities and Specify templates must be aligned before U-01 can rely on the approved tokens.

**Files:**
- Modify: `.specify/memory/constitution.md`
- Modify: `.specify/templates/plan-template.md`
- Modify: `.specify/templates/spec-template.md`
- Modify: `.specify/templates/tasks-template.md`
- Modify: `design/ui-spec.md`

- [x] Amend the constitution to v2.0.0 and propagate the frontend authority/elevation/reduced-motion gate to all three templates.
- [x] Replace obsolete global no-shadow wording in `design/ui-spec.md` with explicit component/page `shadow-none` decisions or the approved elevation tier.
- [x] Keep elevation-3 examples consistent with the mandatory 1px border contract.
- [x] Commit this governance amendment separately with the constitution-prescribed commit message.

## Task U-01: Align Design Authority And CSS Tokens

**Depends on:** P-01 approval

**Files:**
- Modify: `design/design-system.md`
- Modify: `frontend/app/globals.css`
- Create: `frontend/scripts/ui-governance/check-tokens.mjs`
- Create: `frontend/scripts/ui-governance/check-tokens.test.mjs`

- [x] Translate the approved P-01 decisions into authoritative light/dark color, radius, typography, spacing and three-tier subtle-elevation tables; do not preserve the obsolete modal-only-shadow rule merely because current CSS implements it.
- [x] Write tests that parse the approved authoritative tables and assert emitted light/dark values, the approved radius scale, Chinese sans fallbacks, approved elevation tokens, and absence of undefined token references.
- [x] Confirm the tests fail against current CSS.
- [x] Emit `--border-destructive` and `--radius-full`; replace computed radius drift with the approved exact values; align font stacks.
- [x] Resolve `--accent-hover` by using an already-authoritative semantic value or documenting a new value in the design system before CSS consumption.
- [x] If semantic success/warning/info tokens are introduced, specify both foreground and subtle-background light/dark values in the design system; do not add unnamed color constants only to satisfy a checker.
- [x] Verify the token test, frontend build, home/search/admin/studio light+dark screenshots, and no unexpected layout shift.

## Task U-02A: Harden Shared Primitive Behavior And Accessibility

**Depends on:** U-05

**Files:**
- Modify: `frontend/components/ui/button.tsx`
- Modify: `frontend/components/ui/confirm-modal.tsx`
- Modify: `frontend/components/ui/TagBadge.tsx`
- Modify: `frontend/components/ui/Toast.tsx`
- Create: `frontend/tests/ui-primitives-accessibility.test.tsx`

- [x] Add failing tests for Button accessible names, icon-button sizing and disabled semantics; ConfirmModal reason validation/focus/Esc/restoration; and Toast live-region/close labels.
- [x] Preserve existing ConfirmModal behavior rather than reimplementing it.
- [x] Apply 44px mobile/coarse-pointer targets to icon-only or isolated actions without globally inflating dense desktop controls; WCAG 2.2 AA 24px is the minimum fallback.
- [x] Replace the raw remove SVG in `TagBadge.tsx` with the established icon system and an accessible name.
- [x] Verify primitive unit tests plus keyboard-only ConfirmModal and Toast Playwright flows.
- [x] Do not change palette, radius, shadow, typography, empty-state composition or other visual direction in this Task.

## Task U-02B: Apply Approved Visuals To Shared UI Primitives

**Depends on:** P-01 approval, U-01, U-02A

**Files:**
- Modify: `design/ui-spec.md`
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
- Create: `frontend/tests/ui-primitives-visual-contracts.test.tsx`

- [x] Write the approved primitive appearance, states and responsive rules back to the matching `ui-spec.md` component sections before implementation.
- [x] Add contract tests for approved radii, colors, focus rings, elevation, status colors and empty/loading states without duplicating U-02A behavior tests.
- [x] Apply U-01 tokens to primitives while preserving every U-02A accessibility and interaction guarantee.
- [x] Verify representative primitive states in desktop/mobile and light/dark screenshots with no unexpected layout shift.

## Task U-03: Unify Navigation Shells

**Depends on:** P-01 approval, U-01, U-02B, U-05

**Files:**
- Modify: `frontend/components/layout/Header.tsx`
- Modify: `frontend/components/layout/Sidebar.tsx`
- Modify: `frontend/components/studio/StudioSidebar.tsx`
- Modify: `frontend/app/(protected)/admin/layout.tsx`
- Create: `frontend/lib/use-sidebar-collapse.ts`
- Create: `frontend/tests/navigation-shells.test.tsx`

- [x] Test distinct persistence keys, the P-01-approved expanded/collapsed desktop widths (228/48px is only the current baseline), accessible toggle labels, tooltip/title behavior, keyboard operation, approved Header stacking, and mobile navigation/drawer reachability.
- [x] Extract only collapse-state behavior; keep public, studio, and admin information architectures separate where their navigation models differ.
- [x] Replace the admin 220/52px drift and horizontal mobile overflow with the approved mobile navigation pattern.
- [x] Verify public/studio/admin screenshots at 375, 768, 1024, and 1440 CSS pixels.

## Task U-04: Replace Native Destructive Dialogs

**Depends on:** U-02A, U-05

**Files:**
- Modify: `frontend/app/(protected)/dashboard/contents/page.tsx`
- Modify: `frontend/app/(protected)/dashboard/contributors/page.tsx`
- Modify: `frontend/app/(protected)/dashboard/pr-requests/page.tsx`
- Modify: `frontend/app/(protected)/settings/page.tsx`
- Modify: `frontend/app/(protected)/admin/agent-config/page.tsx`
- Modify: `frontend/components/content/VersionHistory.tsx`
- Create: `frontend/tests/destructive-dialogs.test.tsx`

- [x] Test every current `window.confirm`/`window.prompt` path except ReactionBar, which belongs to U-11.
- [x] Use ConfirmModal `requireReason` for PR rejection and any other domain action whose API requires a reason; do not add reason collection to unrelated confirmations.
- [x] Preserve cancellation, busy, success, failure, and retry behavior; API failures must remain visible and must not close the modal as if successful.
- [x] Verify `rg -n 'window\.(confirm|prompt)\('` reports only the U-11-owned ReactionBar call before U-11 merges and zero afterward.

## Task U-05: Make User-Facing Errors Safe And Consistent

**Depends on:** none

**Files:**
- Modify: `frontend/lib/error-handler.ts`
- Modify: `frontend/lib/user-facing-error.ts`
- Modify: `frontend/app/error.tsx`
- Modify: `frontend/app/(public)/verify-email/page.tsx`
- Modify: `frontend/app/(protected)/dashboard/pr-requests/page.tsx` (ownership transfers to U-04 after U-05 merges)
- Modify: `frontend/app/(public)/user/[userId]/UserProfileClient.tsx`
- Modify: `frontend/components/content/ContentDetail.tsx`
- Modify: `frontend/components/content/VersionHistory.tsx` (ownership transfers to U-04 after U-05 merges)
- Modify: `frontend/components/content/DownloadButton.tsx` (ownership transfers to U-11 after U-05 merges)
- Modify: `frontend/components/feedback/FeedbackForm.tsx`
- Modify: `frontend/tests/error-handler-i18n.test.ts`
- Create: `frontend/tests/user-facing-error-surfaces.test.tsx`

- [x] Re-run the exact raw-error inventory at Task start and update the explicit test allowlist; do not rely on the 2026-07-17 file count.
- [x] Write failing tests proving raw backend/exception messages are never rendered, known API codes map to translation keys, and unknown errors use a safe fallback.
- [x] Keep expected background/polling failures silent; do not convert every catch into a Toast.
- [x] Provide a small reusable mapper/helper only where it reduces duplicated policy. Avoid a wrapper that hides control flow or swallows rejected promises.
- [x] Verify error boundary, verify-email, PR requests, profile, detail, version history, download and feedback normal/error paths.

## Gate P-01: Approve Discovery, Agent, And Content Detail Visual Prototype

**Depends on:** U-05, U-02A, U-04

**Production files:** none; use an isolated throwaway prototype so rejected visual directions do not churn production pages.

**Evidence:**
- Create: `docs/working/2026-07-25-ui-prototype-review.md` (include creation date and expiry)
- Create: `screenshots/ui-polish/prototype/` evidence

- [x] Build one isolated prototype direction aligned with the current OmniCraft Indigo design system. The Hallmark-led direction B was canceled by the user on 2026-07-28 and must not be implemented, regenerated or treated as a P-01 prerequisite.
- [x] Keep the primary audience fixed: signed-in content explorers/light creators who discover original and derivative works and use Agent to find, understand and extend on-site content; treat active creators as the secondary audience.
- [x] Keep the primary job fixed: discover content from either the recommendation stream or an Agent answer, understand it in the shared detail overlay, then close it and return without losing feed/conversation context. Following, profile hover, private-message and Agent conversation-search mock paths are required A1.5/A1.6 interaction coverage but are not the primary P-01 success metric; Agent write actions remain out of scope.
- [x] Treat production `/`, `/original` and `/ip/[ipId]` as user-approved visual references; do not duplicate them in the throwaway prototype or restyle their production implementations during P-01. The remaining isolated surfaces are the already-verified recommendation/Agent/detail flows plus the new IP library prototype.
- [x] Develop in reviewable batches: A1 fix-and-merge (complete), A1.5 shared-interaction revision (complete), A1.6 detail-interaction polish (complete and verified), then A2 IP library only. Begin A2 by removing the obsolete direction-B evaluator control and B-only duplicate capture matrix, add one current-Indigo IP-library view, and leave production files unchanged.
- [x] Preserve the product-level shared-detail-overlay contract for recommendation, zone, IP-detail and Agent entries, but do not build duplicate zone/IP-detail prototype pages solely to demonstrate it. P-01 A2 verifies the IP-library card link into the accepted production `/ip/[ipId]`; production overlay wiring remains post-P-01 implementation work.
- [x] Preserve the current `/ips` behavior contract in the prototype: title/description/total, keyword search, category filtering, hot/content-count/newest/name sorting, loading/empty/error states, pagination or load-more affordance, and IP-card navigation. Use representative IP names, covers, categories, content counts and trends rather than empty decorative cards.
- [x] Correct the measured density defect: at a 1280px viewport the current five-column grid places fixed 156px cards about 90.6px apart. The prototype must not put a fixed-width card inside a fractional track; regular-grid tracks/cards consume the available content width, use tokenized 12–16px small-screen and 16px desktop gutters (24px absolute maximum), keep the final row aligned to the grid, and avoid horizontal overflow or large distributed voids.
- [x] In A1.5, make the overlay shell and cover use one synchronized open/close progress (300/240ms with the existing easing; 100ms opacity-only for reduced motion), including Agent citation entries and the no-thumbnail/off-viewport fallback. Use source/previous-content titles for back labels; remove layer counts and explanatory return copy.
- [x] In A1.5, lock page scrolling and make dialog/shell non-scrollable so only the internal content body owns a scrollbar; keep the back/close header outside that scroll body and preserve per-layer scroll memory on the sole scroll container.
- [x] Keep the title area free of duplicate creator identity/actions; show avatar, ellipsized name and a fixed 80px × 32px frame-free Follow control only in the right creator rail (or one non-duplicated equivalent region on narrow layouts). Apply the user-profile hover/focus surface outside creator profile pages: 200ms desktop show/close, immediate keyboard focus, mobile tap direct to profile, navigable public statistics, blue/white Follow, shallow-gray/dark-text Following/Unfollow (also for selected/cancel Favorite and Subscribe), and self-profile Edit action.
- [x] In A1.6, center the creator profile surface below the content-detail creator-rail identity entry, flipping above when lower space is insufficient and clamping within the detail viewport; preserve the existing dynamic placement for comment-author profile surfaces.
- [x] Prototype the private-message chat overlay above the current context: simplified full history, fixed name header/composer, independently scrolling transcript, desktop 440px/70–75dvh centered in the current viewport without anchoring to the profile surface, and mobile full-screen behavior, top-layer-only close/focus restoration, cold-start one-text-message rule until the recipient follows or replies, and mock single-image states (JPEG/PNG/GIF/WebP, <=10MB, disabled during cold start).
- [x] Prototype top-level and reply comment single-image states (JPEG/PNG/GIF/WebP, <=5MB). Both composers start at one line, auto-grow through six lines then scroll internally, hard-limit text to 2000 characters, show the counter only while focused, and truncate over-limit paste; helper/image/publish controls appear only while focus remains in the composer, blur preserves text/image, and mouse focus does not thicken/scale the field. Text or image alone may publish.
- [x] For message/comment image mocks, cover immediate validation/temp upload, progress, retry/remove, pure-image send, independent image preview, optimistic sender-only visibility and recipient/public visibility only after moderation. Moderation failure shows only a sender-side red exclamation and atomically withholds text+image; production upload/migration/moderation remains a separate heavy task.
- [x] Revise the Agent workspace shell in A1.6: expanded sidebar orders visible “Collapse sidebar” first, then the search trigger, full-width New Conversation and history; the persisted 56–64px desktop rail keeps Expand/Search/New icons with tooltips, while mobile uses a dismissible drawer. Fix the workspace to the viewport below Header and allow only the conversation transcript to scroll. Search opens an owner-visible full-conversation mock across all retained conversation titles and message bodies, independent of sidebar loading; list every hit separately in reverse chronological order with conversation source, excerpt and date, then open the conversation at the exact hit with a short shallow-gray highlight. Cover desktop centered `min(720px,92vw)`/`<=80dvh`, mobile full-screen, `Ctrl/Cmd+K`, Esc, arrow/Enter navigation and focus restoration; no-results keeps only the search-field clear action.
- [x] Keep the accepted existing original/IP-detail masonry unchanged. The A2 IP library uses a regular grid with stable DOM/Tab/reading order and responsive card tracks at 320/375/768/1280/1440px.
- [x] Cover light/dark, keyboard focus, hover/active, loading/empty/error and reduced-motion behavior for the IP library; capture deterministic baseline/new screenshots at 375, 768, 1280 and 1440 CSS pixels.
- [x] Present the completed current-Indigo IP-library prototype to the user and iterate until explicit final P-01 approval. Existing `/`, `/original` and `/ip/[ipId]` are already accepted and are not additional prototype gates.
- [x] Record the approved typography, spacing, radius, border, color, three-tier subtle elevation, card, empty/loading and restrained brand-accent decisions without editing production CSS or components in this gate.
- [x] Only after explicit approval, mark P-01 complete and unblock U-01.

## Task U-06: Correct Detail-Level Loading, Empty, And Icon States

**Depends on:** P-01 approval, U-01, U-02B, U-05

**Files:**
- Modify: `frontend/app/(protected)/feedback/[feedbackId]/page.tsx`
- Modify: `frontend/app/(public)/ip/[ipId]/discussions/[discussionId]/page.tsx`
- Modify: `frontend/components/content/SheetMusicViewer.tsx`
- Modify: `frontend/components/judge/VerdictDetail.tsx`
- Create: `frontend/tests/detail-state-contracts.test.tsx`

- [x] Test loading, not-found/empty, retriable error, and success states without layout collapse.
- [x] Replace raw JSX vote icons in VerdictDetail; keep generated data-URI placeholder SVGs outside scope.
- [x] Use inline spinner text only for local operations; use documented skeletons for page/detail fetches.
- [x] Verify keyboard and screen-reader status announcements plus desktop/mobile screenshots.

## Task U-07: Standardize Paginated List Surfaces

**Depends on:** P-01 approval, U-01, U-02B, U-05

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

- [x] Define DataList as presentation/state composition only; API page size, cursor/page state, and fetching remain owned by each page.
- [x] Test first load, empty, error+retry, next page, duplicate-request prevention, end-of-list, filter reset, and retained prior results during pagination failure.
- [x] Do not call a hard-coded page size “configurable” unless a real prop or backend contract controls it.
- [x] Preserve existing history tests and add focused tests for every newly paginated surface.

## Task U-08: Repair Form Accessibility In Remaining Surfaces

**Depends on:** P-01 approval, U-01, U-02B, U-05

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

- [x] Test every control has an accessible name through explicit label, implicit label, aria-label, or aria-labelledby; do not require `htmlFor` when valid implicit association is used.
- [x] Connect errors with `aria-describedby`, mark invalid controls, announce submission errors, and focus the first invalid field.
- [x] Add accessible names to icon-only table actions and confirm 44px isolated mobile targets.
- [x] Run an automated accessibility scan if an existing dependency supports it; otherwise use Testing Library roles/names plus Playwright keyboard evidence without adding an unapproved dependency.

## Task U-09: Normalize Content And Discovery Visuals

**Depends on:** P-01 approval, U-01, U-02B, U-05

**Files:**
- Modify: `design/ui-spec.md`
- Modify: `frontend/components/content/ContentCard.tsx`
- Modify: `frontend/components/content/ContentSidebar.tsx`
- Modify: `frontend/components/home/HomePageClient.tsx`
- Modify: `frontend/components/layout/FacetedSearchSidebar.tsx`
- Modify: `frontend/components/ip/IPBrowseClient.tsx`
- Modify: `frontend/components/ip/IPCard.tsx`
- Modify: `frontend/app/(public)/search/page.tsx`
- Create: `frontend/tests/content-discovery-visual-contracts.test.tsx`

- [x] Write the approved P-01 IP-library page/grid/card contract into `design/ui-spec.md`, then apply it to `IPBrowseClient`/`IPCard`; remove the fixed 156px-in-fractional-track density defect without changing search/filter/sort/navigation semantics.
- [x] Replace the two raw JSX SVG icons in ContentCard/ContentSidebar and the undefined accent hover in IPBrowseClient.
- [x] Convert arbitrary sizes/colors only when the matching UI spec provides an authoritative replacement; record justified exceptions.
- [x] Test original/fanwork card distinctions, responsive facet behavior, IP-library grid density at 320/375/768/1280/1440px, loading/empty/error results, dark contrast, and keyboard operation.
- [x] Capture deterministic home/original/search/content screenshots in both themes.

## Task U-10: Align Communication Surfaces And I18n

**Depends on:** P-01 approval, U-01, U-02B, U-03, U-05

**Files:**
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Modify: `frontend/app/layout.tsx`
- Modify: `frontend/app/(protected)/layout.tsx`
- Modify: `frontend/app/(protected)/messages/page.tsx`
- Modify: `frontend/components/social/ConversationList.tsx`
- Modify: `frontend/components/social/ChatWindow.tsx`
- Modify: `frontend/components/social/NotificationDropdown.tsx`
- Create: `frontend/scripts/ui-governance/check-i18n-parity.mjs`
- Create: `frontend/scripts/ui-governance/check-i18n-parity.test.mjs`
- Create: `frontend/tests/communication-surfaces.test.tsx`

- [ ] Test exact zh/en key parity, non-empty leaf values, safe placeholder parity, and documented namespaces.
- [ ] Replace verified visible hard-coded copy, including the root skip link, without scanning source-code identifiers as false positives.
- [ ] Fix accent-on-white contrast and notification/chat status semantics using U-01 tokens.
- [ ] Preserve unread polling/SSE/message behavior and verify empty/error/reconnect/keyboard paths.
- [ ] Do not create or restyle the production Agent workspace here. `/agent`, `AgentWorkspace`, global Widget removal, Agent citations and their translations are owned by Web Agent Productization Task 4 after P-01 approval; coordinate translation-file ownership before either task starts.

## Task U-11: Replace Frontend Reputation Constants With Server Capabilities

**Depends on:** P-01 approval, U-01, U-02B, U-05, and current Production Readiness coordination inputs

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

**Depends on:** P-01, U-01, U-02A, U-02B, U-03 through U-11

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

- [ ] P-01 is explicitly approved by the user; U-01, U-02A, U-02B and U-03 through U-12 are complete, verified against the current integration base, and represented by logical light-lane commits.
- [ ] No native confirm/prompt remains in production TS/TSX code.
- [ ] No raw backend or exception message is rendered to users in the audited allowlist.
- [ ] Shared primitives and changed surfaces meet keyboard, focus, accessible-name, error-announcement, and mobile target requirements.
- [ ] Changed surfaces use only authoritative tokens; intentional palette/arbitrary-size exceptions are documented by file and reason.
- [ ] `npm.cmd run lint:ui`, frontend unit/lint/build, project verification, and required focused backend gates pass.
- [ ] Deterministic screenshots exist for home, original feed, content detail, studio overview, admin, messages, light/dark, and 375/768/1024/1440 viewports. Pixel-diff thresholds are required only if U-12 actually introduces stable snapshot tooling and baselines.
- [ ] Production-readiness coordination is re-reviewed after the missing parent documents are restored; unresolved security/release conflicts keep the affected Task unchecked.

## Non-Goals

- Do not redesign information architecture, introduce new product features, or change API pagination semantics solely for visual consistency.
- Do not edit production visual tokens, primitives, navigation shells or page styling before P-01 approval; pre-prototype Tasks are correctness-only.
- Do not replace generated data-URI placeholder SVGs with icon components.
- Do not expose the full backend reputation/config object to the browser.
- Do not create a GitHub Actions path under `frontend/`; the repository-level verifier is the current enforceable integration point.
- Do not update unrelated deferred Beta plans, community plans, or Web Agent completion state.
