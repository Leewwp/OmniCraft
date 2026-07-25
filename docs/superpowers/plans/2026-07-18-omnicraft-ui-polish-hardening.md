# OmniCraft UI Polish Hardening Implementation Plan

> **For agentic workers:** Follow the active light-lane workflow in `AGENTS.md`; use the available UI/UX review and browser-testing skills for the relevant Task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变业务语义的前提下，修复已证实的设计令牌、交互反馈、可访问性、响应式、国际化和用户错误暴露问题，并建立可重复的 UI 治理门。

**Architecture:** 先完成不依赖视觉方向的错误安全、基础组件可访问性和破坏性操作修复，再用隔离原型完成首页 Feed 与内容详情页的桌面/移动/暗色评审。用户明确批准原型后，才把视觉决策回写为设计 Token、共享组件和导航壳层，随后推广到剩余页面并接入治理门。跨阶段重复文件必须在前一提交合入后显式转移所有权，禁止并发修改。

**Tech Stack:** Next.js 16.2 / React 19 / TypeScript 5 / Tailwind CSS 4 CSS-first / next-intl / Base UI / Playwright / Node test / PowerShell.

---

## Status And Authority

**Audit date:** 2026-07-17；execution order revised 2026-07-25

**Status:** active light-lane plan; U-05, U-02A and U-04 are complete, and P-01 is the next approval gate.

- This plan is priority 1 in the `AGENTS.md` active-plan registry and uses the light lane. U-05, U-02A and U-04 are complete; P-01 is the next human-approval gate. Numeric Task IDs remain stable identifiers; the wave order below, not numeric order, is authoritative for execution.
- Production Readiness is present and registered after this plan. Re-run coordination before U-11/U-12 or any edit that expands into config, release, verifier, or backend capability paths.
- Before P-01 approval, visual authority remains `design/design-system.md` then matching `design/ui-spec.md` sections; correctness Tasks must not restyle surfaces. After the user approves P-01, the approved prototype decision record becomes the input for U-01/U-02B updates to those authorities.
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
Gate:    P-01                                   home/detail visual prototype + user approval
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

## Task U-01: Align Design Authority And CSS Tokens

**Depends on:** P-01 approval

**Files:**
- Modify: `design/design-system.md`
- Modify: `frontend/app/globals.css`
- Create: `frontend/scripts/ui-governance/check-tokens.mjs`
- Create: `frontend/scripts/ui-governance/check-tokens.test.mjs`

- [ ] Translate the approved P-01 decisions into authoritative light/dark color, radius, typography, spacing and three-tier subtle-elevation tables; do not preserve the obsolete modal-only-shadow rule merely because current CSS implements it.
- [ ] Write tests that parse the approved authoritative tables and assert emitted light/dark values, the approved radius scale, Chinese sans fallbacks, approved elevation tokens, and absence of undefined token references.
- [ ] Confirm the tests fail against current CSS.
- [ ] Emit `--border-destructive` and `--radius-full`; replace computed radius drift with the approved exact values; align font stacks.
- [ ] Resolve `--accent-hover` by using an already-authoritative semantic value or documenting a new value in the design system before CSS consumption.
- [ ] If semantic success/warning/info tokens are introduced, specify both foreground and subtle-background light/dark values in the design system; do not add unnamed color constants only to satisfy a checker.
- [ ] Verify the token test, frontend build, home/search/admin/studio light+dark screenshots, and no unexpected layout shift.

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

- [ ] Write the approved primitive appearance, states and responsive rules back to the matching `ui-spec.md` component sections before implementation.
- [ ] Add contract tests for approved radii, colors, focus rings, elevation, status colors and empty/loading states without duplicating U-02A behavior tests.
- [ ] Apply U-01 tokens to primitives while preserving every U-02A accessibility and interaction guarantee.
- [ ] Verify representative primitive states in desktop/mobile and light/dark screenshots with no unexpected layout shift.

## Task U-03: Unify Navigation Shells

**Depends on:** P-01 approval, U-01, U-02B, U-05

**Files:**
- Modify: `frontend/components/layout/Header.tsx`
- Modify: `frontend/components/layout/Sidebar.tsx`
- Modify: `frontend/components/studio/StudioSidebar.tsx`
- Modify: `frontend/app/(protected)/admin/layout.tsx`
- Create: `frontend/lib/use-sidebar-collapse.ts`
- Create: `frontend/tests/navigation-shells.test.tsx`

- [ ] Test distinct persistence keys, the P-01-approved expanded/collapsed desktop widths (228/48px is only the current baseline), accessible toggle labels, tooltip/title behavior, keyboard operation, approved Header stacking, and mobile navigation/drawer reachability.
- [ ] Extract only collapse-state behavior; keep public, studio, and admin information architectures separate where their navigation models differ.
- [ ] Replace the admin 220/52px drift and horizontal mobile overflow with the approved mobile navigation pattern.
- [ ] Verify public/studio/admin screenshots at 375, 768, 1024, and 1440 CSS pixels.

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

## Gate P-01: Approve Home Feed And Content Detail Visual Prototype

**Depends on:** U-05, U-02A, U-04

**Production files:** none; use an isolated throwaway prototype so rejected visual directions do not churn production pages.

**Evidence:**
- Create: `docs/working/2026-07-25-ui-prototype-review.md` (include creation date and expiry)
- Create: `screenshots/ui-polish/prototype/` evidence

- [ ] Build an isolated prototype for the home Feed and content detail page using real information density and representative cover/content states.
- [ ] Cover desktop, mobile and dark mode; include navigation context, real cover cards, hover/focus behavior, loading/empty states and reduced-motion behavior.
- [ ] Present screenshots to the user and iterate until the user explicitly approves one direction. Do not treat an Agent self-review as approval.
- [ ] Record the approved typography, spacing, radius, border, color, three-tier subtle elevation, card, empty/loading and restrained brand-accent decisions without editing production CSS or components in this gate.
- [ ] Only after explicit approval, mark P-01 complete and unblock U-01.

## Task U-06: Correct Detail-Level Loading, Empty, And Icon States

**Depends on:** P-01 approval, U-01, U-02B, U-05

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

- [ ] Define DataList as presentation/state composition only; API page size, cursor/page state, and fetching remain owned by each page.
- [ ] Test first load, empty, error+retry, next page, duplicate-request prevention, end-of-list, filter reset, and retained prior results during pagination failure.
- [ ] Do not call a hard-coded page size “configurable” unless a real prop or backend contract controls it.
- [ ] Preserve existing history tests and add focused tests for every newly paginated surface.

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

- [ ] Test every control has an accessible name through explicit label, implicit label, aria-label, or aria-labelledby; do not require `htmlFor` when valid implicit association is used.
- [ ] Connect errors with `aria-describedby`, mark invalid controls, announce submission errors, and focus the first invalid field.
- [ ] Add accessible names to icon-only table actions and confirm 44px isolated mobile targets.
- [ ] Run an automated accessibility scan if an existing dependency supports it; otherwise use Testing Library roles/names plus Playwright keyboard evidence without adding an unapproved dependency.

## Task U-09: Normalize Content And Discovery Visuals

**Depends on:** P-01 approval, U-01, U-02B, U-05

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
