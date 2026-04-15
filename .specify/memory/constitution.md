<!--
SYNC IMPACT REPORT
==================
Version change    : none → 1.0.0 (initial ratification)
Modified sections : N/A — first ratification
Added sections    :
  Principles I–XII (tech stack, code quality, DB integrity, config,
  feature flags, security, performance, UI design, agent workflow,
  business rules, surgical changes, testing discipline)
  § Business Rules Reference
  § Agent Workflow Reference
  § Governance
Templates updated :
  ⚠ .specify/templates/plan-template.md   — PENDING (not yet created)
  ⚠ .specify/templates/spec-template.md   — PENDING (not yet created)
  ⚠ .specify/templates/tasks-template.md  — PENDING (not yet created)
Deferred TODOs    : none
-->

# OmniCraft（万象工坊）Constitution

## Core Principles

### I. Tech Stack Conformance (NON-NEGOTIABLE)

The following stack is FROZEN for all phases unless amended via a formal constitution update:

| Layer | Technology |
|---|---|
| Frontend | Next.js (App Router · SSR · TypeScript strict) |
| Backend | Go 1.22+, Gin/Fiber framework, GORM ORM |
| Primary DB | PostgreSQL 15+ via PgBouncer connection pool |
| Cache / Rate-limit | Redis 7+ |
| File Storage | Aliyun OSS — direct client-side upload via pre-signed URL |
| PC Client | Tauri (Rust core + WebView frontend) |
| Content Review | Aliyun Content Safety API |
| Deployment P0 | Docker Compose single-node |
| Deployment P2 | Kubernetes (Aliyun ACK) — pre-reserved, not yet active |

Introducing a new runtime dependency (npm package, Go module, or system service) requires explicit
justification against an existing alternative and MUST be added to the dependency management file
before merging. No library may be introduced for convenience if the standard library covers the case.

### II. Code Quality Standards

**Go (backend)**:
- ALL files MUST pass `gofmt` before commit; CI gate enforces this
- Errors MUST be handled at every call site; bare `panic` is FORBIDDEN except in `init()` for fatal
  misconfiguration
- Unified HTTP error envelope: `{ "code": "ERROR_CODE", "message": "..." }` — no raw error strings
  in API responses
- Package layout: `cmd/` → entry points; `internal/handler` → HTTP; `internal/service` → business
  logic; `internal/repository` → DB access; `internal/model` → GORM structs; `internal/pkg` → utils

**Next.js / TypeScript (frontend)**:
- TypeScript `strict: true` is MANDATORY; the `any` type is FORBIDDEN; use `unknown` + type guards
- Named page/layout components MUST use `function` declaration, not arrow-function assignment
- ALL styling via Tailwind CSS utility classes; inline `style=` is allowed only for genuinely dynamic
  computed values that Tailwind cannot express
- `npm run build` and `npm run lint` MUST pass with zero errors before any commit

### III. Database Integrity

- ALL schema changes MUST be introduced as a new numbered migration file under `backend/migrations/`
  (e.g., `019_xxx.sql`); direct `ALTER TABLE` outside a migration is FORBIDDEN in any non-local env
- ALL database queries MUST use GORM parameterized bindings; SQL string concatenation with user input
  is FORBIDDEN
- Foreign key constraints MUST be declared in DDL; orphaned references MUST NOT be tolerated at the
  DB layer
- Every new table MUST include at minimum `id BIGSERIAL PRIMARY KEY` and
  `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

### IV. Configuration over Hardcode

- ALL upload limits, timeout values, and feature flags MUST be read from `config.yaml` at runtime;
  hardcoded numeric limits are FORBIDDEN
- ALL secrets (DB passwords, OSS keys, JWT secrets, Aliyun API keys) MUST be injected via environment
  variables; no secret value may appear in any committed file
- ALL external service hostnames MUST be configurable via env vars; no hardcoded production URLs
- `config.yaml` is the single source of truth for runtime tunables

### V. Feature Flag Governance

- ALL features that are unfinished, monetization-related, or experimental MUST be guarded by a
  boolean flag under `config.yaml > features:`
- When a flag is `false`: the UI MUST NOT render the feature; the API endpoint MUST return HTTP 503
  with `{ "code": "FEATURE_DISABLED" }`
- Removing feature-flagged code is FORBIDDEN at MVP stage; disable via flag, not deletion
- Defined flags (initial): `payment_enabled`, `ad_enabled`, `agent_enabled`, `judge_enabled`

### VI. Security Non-Negotiables

- ALL client-server communication MUST use HTTPS; HTTP is permitted only on `localhost` in local dev
- ALL state-mutating API routes MUST require a valid JWT; public read endpoints MAY allow anonymous
  access
- File uploads MUST use OSS direct upload (pre-signed URL); the Go backend MUST NOT proxy raw file
  bytes through itself
- User passwords MUST be hashed with bcrypt (cost ≥ 12); plaintext or MD5/SHA1 storage is FORBIDDEN
- Rate limiting MUST be applied to auth endpoints (login, register) via Redis sliding-window counter

### VII. Performance Standards

- API P95 response latency target: **≤ 500 ms** for read endpoints under normal load (< 1,000
  concurrent users at P0 scale)
- ALL hot-path read queries (tag facets, content list, IP list) MUST have a Redis cache layer with
  an explicit TTL documented at the call site
- Pagination is MANDATORY for all list endpoints (cursor or offset+limit); unbounded full-table scans
  are FORBIDDEN in production code paths
- ALL foreign key columns MUST be indexed; composite indexes MUST include a comment explaining the
  target query pattern
- SSR MUST be used for all public content pages that require SEO indexing; client-side-only rendering
  is acceptable only for authenticated or private pages

### VIII. UI Design Conformance

The UI design system references **GitHub's visual language** (see `architecture.md §10.4`):

- **Color tokens**: `canvas`, `border`, `fg`, `accent`, `tag` (with `light`/`dark` variants) MUST be
  defined in `tailwind.config.ts` and used exclusively; raw hex colors in components are FORBIDDEN
- **Dark mode**: toggled via `class="dark"` on `<html>` using `next-themes`; theme stored in
  `localStorage` only — NOT persisted to the server
- **Typography**: system font stack `(-apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica,
  Arial)`; no custom web fonts at MVP stage
- **Tag badges**: MUST use the `TagBadge` component with `tag.*` low-saturation color tokens; raw
  colored `<span>` elements for tags are FORBIDDEN
- **Content browsing layout**: homepage, IP detail page, and original zone MUST use `MasonryGrid`
  (react-masonry-css); responsive columns: mobile 2 / tablet 3 / PC 4
- **Cover images**: every `ContentCard` MUST display a cover image; when `cover_image_url` is absent,
  the type-specific SVG from `lib/coverPlaceholder.ts` MUST be shown — blank image areas are
  FORBIDDEN
- **i18n**: after Task 46 is complete, ALL UI string literals MUST be referenced via
  `useTranslations()` from `next-intl`; hardcoded Chinese or English text in components is FORBIDDEN

### IX. Agent Workflow Discipline

Agents executing implementation tasks MUST follow this sequence without deviation:

1. Read `task.json`; select the lowest-ID task with `passes: false` whose dependencies are satisfied
2. Verify the dev environment is running before writing any code
3. Implement exactly the steps listed in the task; clarify ambiguity **before** coding, never after
4. Run ALL mandatory gates: `go build ./...`, `go vet ./...`, `npm run build`, `npm run lint`;
   UI tasks also require Playwright browser verification with screenshot
5. Update `progress.txt` with the standardised work log
6. Set `passes: true` only after ALL steps are verified; partial completion is NOT acceptable
7. Commit `task.json` + `progress.txt` + all code changes in a **single atomic commit** with message
   `Task [ID]: [title] - completed`

Blocked tasks MUST emit the standard blocker template from `CLAUDE.md` and MUST NOT be marked
complete. `git commit` is FORBIDDEN while a task is in a blocked state.

### X. Business Rule Enforcement

The following rules are LOCKED to PRD V0.3; changing them requires an explicit PRD amendment and
a constitution re-ratification:

**Reputation System**:
- Initial score: 10; minimum score to interact: 3
- Deductions (read from `config.yaml > reputation`): malicious report −2, malicious comment −2,
  malicious contribution −3, malicious report-of-comment −1, judge error −1
- Yellow/gambling/drug content: direct permanent ban; the reputation deduction path does NOT apply

**Cyber Judge**:
- A content type MUST NOT open crowd-review until its question bank is initialised by an admin
- Qualification passing threshold: ≥ 80% correct
- Revocation: error rate > 50% across the most recent N > 10 judgments → revoke privilege + deduct
  1 reputation point; re-qualification required before reinstatement

**Upload Limits** (ALL read from `config.yaml > limits`; hardcoding any value is FORBIDDEN):

| Content type | Limit |
|---|---|
| Video | ≤ 300 MB, ≤ 180 s |
| Image | ≤ 20 MB |
| Text / Markdown | ≤ 10 MB |
| Mod archive | ≤ 500 MB |
| Sheet music | ≤ 50 MB; allowed ext: mid, midi, xml, mxl, mscz, mscx, pdf |

### XI. Minimal & Surgical Changes

- Implement ONLY what the current task specifies; unsolicited "while I'm here" improvements are
  FORBIDDEN
- New code MUST match the style and patterns of the surrounding file; refactoring existing code
  requires a separate, explicitly scoped task
- Remove ONLY imports and symbols introduced by the **current task** that are unused; pre-existing
  dead code cleanup requires its own task
- Abstraction layers (interfaces, generics, middleware) MUST NOT be introduced unless the task
  explicitly requires them

### XII. Testing Discipline

- UI tasks (new pages or major interaction changes): MUST include a Playwright test capturing at
  minimum page load, one core interaction, and a screenshot; the screenshot MUST be recorded in
  `progress.txt`
- Backend API tasks: MUST include at least one positive-path and one error-path verification
  (curl or Go test)
- `go test ./...` MUST pass with zero failures before any backend commit
- Tests MUST NOT be deleted or weakened to make a build pass; if a test reveals a real defect,
  fix the defect

## Business Rules Reference

> Canonical source: `architecture.md §5` (reputation) and `CLAUDE.md §Key Business Rules`.
> This section summarises; `architecture.md` governs in case of wording conflict.

See Principle X for the authoritative rule set. All numeric thresholds MUST be read from
`config.yaml`; no threshold value may be hardcoded in application logic.

## Agent Workflow Reference

> Canonical source: `CLAUDE.md`.
> The 13 Key Rules listed in `CLAUDE.md §Key Rules` are binding and incorporated by reference.
> In case of conflict between `CLAUDE.md` and this constitution, the **most restrictive**
> interpretation applies.

## Governance

- This constitution is the **highest-priority** governing document for OmniCraft; it supersedes
  individual task descriptions, PR comments, and verbal instructions
- **Amendment procedure**: edit this file, increment `CONSTITUTION_VERSION` following semantic
  versioning (MAJOR = principle removal or redefinition; MINOR = new principle or section added;
  PATCH = wording or clarification), update `LAST_AMENDED_DATE`, and commit with message
  `docs: amend constitution to vX.Y.Z (<one-line summary>)`
- **Compliance review**: every task completion implicitly verifies no principle is violated; an
  agent raising a blocker caused by a principle constraint MUST cite the principle by number
- **Template propagation**: after any MINOR or MAJOR amendment, `.specify/templates/plan-template.md`,
  `spec-template.md`, and `tasks-template.md` MUST be updated to reflect the change
- **Conflict resolution**: if a task step conflicts with a principle, the principle wins; the
  conflicting step MUST be clarified before implementation begins

**Version**: 1.0.0 | **Ratified**: 2026-04-15 | **Last Amended**: 2026-04-15
