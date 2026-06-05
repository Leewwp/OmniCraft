# OmniCraft Web Beta Optimization Roadmap Design

**Date:** 2026-06-05
**Mode:** AGENTS.md Mode A, Dual-Track Beta plan set
**Status:** Draft for review

## Purpose

This design defines a non-conflicting, evidence-based roadmap for optimizing OmniCraft after the Web Beta review. It is not an implementation batch. Its job is to help future agents and maintainers decide what to fix, improve, defer, or revalidate without colliding with the active production-readiness repair work.

The immediate product goal is to bring Web Beta to a production-launchable state while keeping these flags disabled unless a maintainer explicitly changes the release decision:

- `features.desktop_deploy_enabled=false`
- `client.download_enabled=false`
- `features.payment_enabled=false`

Tauri desktop distribution, one-click deploy, payment, and production infrastructure automation are later tracks, not part of this Web Beta optimization roadmap.

## Current Coordination Boundary

An implementation agent is actively repairing production-readiness blockers across these batches:

- Batch 1: P0/P1 security fixes for auth, captcha, release validation, safe errors, CORS, and cookie-only refresh/logout.
- Batch 2: critical business path fixes for login captcha threshold, verification cache, publish freeze, feedback, report resolution, sheet music download authorization, and Agent visibility.
- Batch 3: admin audit transaction integrity and feedback reply/close notification completeness.
- Batch 4: P2 security and quality fixes around content visibility, downloads, feedback upload grants, OSS serialization, creator-support flag clarity, audit metadata, UploadAssist validation, token atomicity, and Agent feature flag checks.

This roadmap must not modify implementation files in `backend/`, `frontend/`, or `tauri-client/` while that repair work is active. It must not modify `task.json`, Beta checkboxes, `progress.txt`, or release validation evidence unless it is later converted into an approved implementation plan.

## Evidence Sources

Primary evidence:

- `docs/review/web-beta-review-summary.md`
- `docs/review/web-beta-review-00-release-evidence.md`
- `docs/review/web-beta-review-01-authz-runtime.md`
- `docs/review/web-beta-review-02-session-config-desktop-off.md`
- `docs/review/web-beta-review-03-verification.md`
- `docs/review/web-beta-review-04-feedback-public-pages.md`
- `docs/review/web-beta-review-05-search-download.md`
- `docs/review/web-beta-review-06-admin-audit.md`
- `docs/review/web-beta-review-07-admin-console.md`
- `docs/review/web-beta-review-08-agent-entrypoints.md`
- `docs/review/web-beta-review-09-cross-stack-e2e.md`
- `docs/superpowers/plans/2026-06-03-omnicraft-web-beta-review-repair-plan.md`

Supporting evidence:

- `docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md`
- `docs/superpowers/plans/2026-05-30-omnicraft-beta-release-validation.md`
- `architecture.md`
- `progress.txt`
- `frontend/app/`, `frontend/components/`
- `backend/internal/`, `backend/migrations/`

The review summary currently reports `GO-WITH-BLOCKERS`: the architecture is sound and prior gates passed, but P0/P1 code defects and production infrastructure gaps prevent launch.

## Prioritization Model

Use four priority levels:

- **Must:** Required before production Web Beta launch. Includes security fail-open behavior, captcha enforcement, release config validation, raw error leaks, broken feedback/report paths, direct download bypasses, and visibility leaks.
- **Should:** Strongly recommended before or immediately after launch because it affects trust, operator workflows, or core product usability.
- **Could:** Product polish, UI/interaction improvements, maintainability cleanup, and evidence improvements that do not block launch.
- **Later:** Deferred tracks that should not distract Web Beta launch, including Tauri distribution, one-click deploy, payment, creator revenue, and deeper deployment automation.

Severity labels from the review are inputs, not the only decision rule. A P2 data-leak edge can become Must or Should if it touches public visibility or download authorization. A P3 UI gap can remain Could even if it is visible.

## Roadmap Shape

### Phase 0: Production Launch Blockers

Goal: remove hard blockers before any production Web Beta deployment.

Evidence:

- P0-01 through P0-03 in `web-beta-review-summary.md`
- P1-01 through P1-17 where they affect auth, release safety, feedback, reports, downloads, visibility, or admin audit integrity
- Failed journeys in Review 09

Recommended handling:

- Treat the active Batch 1-4 repair work as the occupied implementation path.
- Do not create parallel tasks against the same files.
- After implementation finishes, re-run R-01 release validation rather than relying on the older approved report.

Success criteria:

- P0 findings are fixed with regression tests.
- Known failed E2E journeys pass or are clearly marked as external-service blocked.
- Release validation report is updated with real commands, screenshot paths, residual risks, and a new decision.

### Phase 1: Core Web User Paths

Goal: make the public Beta feel reliable for first-time and returning users.

Paths to review:

- anonymous browse and content detail
- register, verify email, login, logout
- forgot/reset password
- search and search suggestions
- content download
- public feedback and feedback tracking

Evidence-backed opportunities:

- Reset-password auto-login contract mismatch.
- Masked email persisted in URL query string after register redirect.
- `fetchPublicConfig` throws instead of returning safe defaults.
- Search has missing DB-backed integration tests.
- Anonymous Agent search UI may be visible without a clear downgrade.
- Direct or stale download paths must be eliminated from user-facing CTAs.

Suggested future tasks:

- User-path regression suite for auth and verification flows.
- Search reliability suite with Chinese titles, deleted content, banned IPs, and private visibility.
- UX pass for verification pending, resend cooldown, and reset success states.

### Phase 2: Creator And Publish Experience

Goal: reduce publishing mistakes and make Agent assistance trustworthy.

Paths to review:

- `/studio/publish/original`
- `/studio/publish/fanwork`
- content type selection
- file upload and preview
- Agent upload assist
- compliance warnings and violation blocking
- `/studio/contents`

Evidence-backed opportunities:

- UploadAssist backend needs output validation.
- Warning-level compliance suggestions need explicit acknowledgement.
- Violation-level suggestions must block apply and submit actions consistently.
- Suggested title and description length validation should exist on both backend and frontend.
- File upload limits and OSS unavailable states need clear user feedback.

Suggested future tasks:

- Publish workflow Playwright journeys for original and fanwork.
- Agent suggestion acceptance tests covering warning, violation, overlong text, invalid category, and too many tags.
- UI polish pass for upload progress, failed upload recovery, and draft-like unsaved state warnings if product wants that behavior later.

### Phase 3: Admin Operations

Goal: make the admin console trustworthy for real operations.

Paths to review:

- dashboard
- reports
- feedback
- queue
- audit logs
- config
- categories and judge administration

Evidence-backed opportunities:

- Audit writes must be atomic with sensitive mutations.
- Failed audit attempts should be recorded with sanitized reason codes.
- Category/Judge audit rows should use `middleware.GetUserID(c)` and include trace IDs.
- Feedback page lacks assignee UI despite backend field support.
- Some feedback filters use hardcoded English strings, violating i18n rules.
- Admin layout has redundant redirect plus EmptyState behavior for non-admin users.

Suggested future tasks:

- Admin mutation transaction tests, one representative per mutation pattern.
- Admin feedback assignment UI and filter/i18n cleanup.
- Admin denied-state simplification.
- Audit-log evidence view improvements after transaction integrity is fixed.

### Phase 4: Product UI And Interaction Polish

Goal: improve perceived quality without expanding launch scope.

Areas to review:

- homepage and original feed
- masonry grid responsiveness
- search page
- content detail and sheet music viewer
- feedback form
- client information page
- empty states, loading states, and mobile layouts

Evidence-backed opportunities:

- `/client` can overstate unavailable desktop features; copy should match `client.download_enabled=false`.
- Feedback diagnostics currently use raw `navigator.userAgent` as platform; structured fields would be cleaner.
- SearchAgentInput visible to anonymous users may be confusing if disabled or non-functional.
- Screenshot evidence exists in some folders but release report references should be made reproducible.

Suggested future tasks:

- UI evidence pass with screenshots at mobile, tablet, and desktop widths.
- Copy accuracy pass for unavailable features and external-service dependent states.
- Empty/loading/error-state audit across public and protected pages.

### Phase 5: Quality System And Later Tracks

Goal: make future regressions harder to introduce, then split deferred tracks cleanly.

Evidence-backed opportunities:

- Missing route-level policy group tests.
- Missing search integration tests.
- Agent visibility tests should move beyond source-string scanning.
- Public config tests should verify legal DTO and secret absence more explicitly.
- Migration 041/042 corrective strategy needs clean evidence if any target database might have run the old migration.

Later tracks:

- Tauri D-02 through D-05 and R-02.
- One-click deploy grants and Ed25519 contract.
- Production deployment and infrastructure hardening.
- Payment and creator revenue.
- Long-term observability, load testing, and operational dashboards.

Suggested future tasks:

- Route authorization matrix tests.
- DB-backed search and visibility integration tests.
- Release evidence automation script that writes command output, API matrix results, and screenshot paths.
- Separate desktop deploy design/update only after Web Beta is stable and maintainers opt into client distribution.

## Output Format For Future Optimization Items

Each future item should use this shape:

```text
Title:
Priority: Must | Should | Could | Later
Source:
User/System Impact:
Affected Area:
Suggested Action:
Verification:
Conflict Notes:
```

`Conflict Notes` must state whether the item overlaps the active implementation batches. If it does, future agents should wait for that work to land and revalidate before editing.

## Non-Goals

This roadmap does not:

- Implement code changes.
- Mark Beta tasks complete.
- Update `task.json`.
- Enable desktop deploy, client download, payment, or creator support.
- Claim real SMTP, CAPTCHA, OSS, HTTPS, or production database success without actual configuration.
- Replace the existing Web Beta repair plan.

## Approval And Next Step

After human review, this design can be converted into an implementation plan only if the active production-readiness repair work has either completed or the maintainer assigns a non-overlapping follow-up task.

The safest immediate next step is to let the active Batch 1-4 implementation finish, then use this roadmap as the checklist for:

1. R-01 revalidation.
2. post-blocker Web user-path review.
3. non-conflicting UI/product polish tasks.
