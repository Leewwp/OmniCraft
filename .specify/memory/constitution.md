<!--
SYNC IMPACT REPORT
==================
Version change    : 1.1.1 → 1.2.0 (MINOR — new principles & specifications added)
Modified sections :
  Principle VI   — expanded: admin config leak prevention, CORS policy, auth state check, error message sanitization
  Principle XV    — NEW: Security Hardening (Task 99–105 specifications)
  Principle XVI   — NEW: Performance & Reliability (Task 128–130, 135, 141–143 specifications)
  Principle XVII  — NEW: Data Integrity & Soft Delete (Task 103, 127 specifications)
  Principle XVIII — NEW: Request Validation & i18n (Task 108, 110 specifications)
Templates updated :
  ✅ .specify/templates/plan-template.md   — 1.3.0 变更已传播（此标记为手动确认，模板本身为通用结构无需修改）
  ✅ .specify/templates/spec-template.md    — 同上
  ✅ .specify/templates/tasks-template.md   — 同上
Deferred TODOs    : none
-->

# OmniCraft（万象工坊）Constitution

## Core Principles

### I. Tech Stack Conformance (NON-NEGOTIABLE)

The following stack is FROZEN for all phases unless amended via a formal constitution update:

| Layer | Technology |
|---|---|
| Frontend | Next.js (App Router · SSR · TypeScript strict) |
| Backend | Go 1.22+, Gin framework, GORM ORM |
| Primary DB | PostgreSQL 16+ via PgBouncer connection pool |
| Cache / Rate-limit | Redis 7+ |
| File Storage | Aliyun OSS — direct client-side upload via pre-signed URL |
| PC Client | Tauri (Rust core + WebView frontend) |
| Content Review | Aliyun Content Safety API |
| Deployment P0 | Docker Compose single-node |
| Deployment P2 | Kubernetes (Aliyun ACK) — pre-reserved, not yet active |
| Vector Search | pgvector extension for PostgreSQL — no separate vector DB |

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
- Defined flags (initial): `payment_enabled`, `ad_enabled`, `agent_enabled`, `judge_enabled`,
  `web_agent_enabled` (web-side LLM agent, defaults `false`, independent of Tauri `agent_enabled`),
  `creator_support_enabled` (creator support module — donation QR / external links, defaults `false`,
  P1 phase)

### VI. Security Non-Negotiables

- ALL client-server communication MUST use HTTPS; HTTP is permitted only on `localhost` in local dev
- ALL state-mutating API routes MUST require a valid JWT; public read endpoints MAY allow anonymous
  access
- File uploads MUST use OSS direct upload (pre-signed URL); the Go backend MUST NOT proxy raw file
  bytes through itself
- User passwords MUST be hashed with bcrypt (cost ≥ 12); plaintext or MD5/SHA1 storage is FORBIDDEN
- Rate limiting MUST be applied to auth endpoints (login, register) via Redis sliding-window counter
- **Admin config leak prevention (Task 99)**: `GET /api/v1/admin/config` responses MUST mask sensitive
  fields (JWT secrets, OSS keys, LLM API keys) as `***REDACTED***`; the frontend Admin settings page
  MUST NOT depend on full secret values for rendering (display masked values or "configured" status only)
- **CORS hardening (Task 100)**: Production `AllowedOrigins` MUST be read from
  `config.yaml > security.allowed_origins`; wildcard `*` is FORBIDDEN in production; the CORS
  middleware MUST be mounted before route registration
- **Auth state real-time check (Task 101)**: Protected-route auth middleware MUST check both
  `users.is_banned` and `users.role` on every request; ban or role changes MUST take effect on
  the next request (no reliance on client-side cache); the frontend `AuthProvider` MUST
  silently poll `GET /api/v1/auth/me` at a 5-minute interval to refresh user state
- **Error message sanitization (Task 102)**: `err.Error()` MUST NOT appear in API responses or
  frontend UI; backend MUST use the standard error envelope `{ "code": "ERROR_CODE", "message":
  "user-friendly description" }`; frontend MUST display errors via toast, never via
  `console.error(err)` to end users

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

The UI design system uses an **Indigo-based flat design** with 1px borders and no box-shadows
(see `design/design-system.md` for color tokens and `architecture.md §10.4` for component mapping):

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

1. Determine the active work mode per `CLAUDE.md` (Beta roadmap mode A or task.json mode B); select the appropriate task source and the next available task
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
- Scoring tiers (impact high → low, read from `config.yaml > reputation`):
  - **Content-related**: quality content (likes ≥ threshold) +3, PR accepted +3 | malicious/plagiarised content, malicious PR contribution −3
  - **Comment-related**: quality comment (likes ≥ threshold) +2 | malicious comment, malicious report of normal comment −2
  - **Tag-related**: successful report or tag addition +1 | malicious tag report −1
  - **Judge-related**: judge accuracy bonus +1 | judge error −1
  - **Rehab**: rehab course completed +1 (each course once only)
- "Malicious" vs "valid" report: determined by the **final resolution outcome**
- Quality content / quality comment thresholds MUST be read from `config.yaml > reputation`
- Rehab courses: users below threshold (3) can complete violation-specific courses to recover
  reputation; each course may be completed once; minimum reading time ≥ 180s
- Yellow/gambling/drug content: direct permanent ban; the reputation deduction path does NOT apply

**User Roles**: `user` | `creator` | `admin` — judge status is determined by `judge_qualifications`
table, NOT stored as a role value; there is no separate "hardcore contributor" role

**Cyber Judge**:
- A content type MUST NOT open crowd-review until its question bank is initialised by an admin
- Qualification passing threshold: ≥ 80% correct
- Revocation: error rate > 50% across the most recent N > 10 judgments → revoke privilege + deduct
  1 reputation point; re-qualification required before reinstatement
- Verdict closure: total voter count ≥ threshold (MVP default: 20, configurable, target: 100)
- Verdict outcome: ≥ 60% "not violation" → content restored; < 60% → controversial, hidden
  (admin may manually restore); results displayed on verdict detail page with reason voting
- Judges may optionally submit a reason; after voting, the verdict detail page shows current
  vote distribution and other judges' reasons (upvote/downvote, sorted by net likes)

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

### XIII. Web Agent Constraints

Applies when `agent.web_agent_enabled: true`. These constraints are non-negotiable for any
web Agent implementation:

- **LLM Provider abstraction**: all LLM calls MUST go through the `LLMProvider` interface
  (`internal/pkg/llm/`); the active provider is selected by `llm_configs` table (is_active=TRUE),
  falling back to `config.yaml > agent.llm_provider` (`qwen` or `openai_compat`) if no DB config
  exists; API key MUST come from env var `AGENT_LLM_API_KEY` (config.yaml fallback) or encrypted
  `api_key_enc` column (DB path) — plaintext API keys MUST NOT appear in any committed file
- **Tool whitelist**: the web Agent MUST operate in Tool-Call mode using only the approved tool
  whitelist defined in `architecture.md §11.6`; arbitrary code execution by the Agent is FORBIDDEN;
  adding a new tool requires a constitution amendment
- **SSE only**: Agent streaming responses MUST use Server-Sent Events; WebSocket is FORBIDDEN for
  Agent communication at MVP stage
- **Rate limiting**: every Agent API call MUST be gated by the Redis rate limiter
  (`agent:rl:{user_id}:{date}`); limits are read from `config.yaml > agent.rate_limit_*`;
  hardcoded limits are FORBIDDEN
- **Vector pipeline**: content embeddings MUST use pgvector (existing PostgreSQL); no separate
  vector DB is permitted; embedding failures MUST NOT block content publishing
- **MVP scope**: only the five approved MVP features (upload-assist, compliance-check, search,
  usage-guide, moderate) may be implemented under Tasks 52–59; Tier 2/3 features require a new
  explicit PRD section and constitution amendment before implementation begins
- **Priority**: implementation priority is governed by the active work mode (see Principle IX). All historical Tasks 1–51 are complete; new work follows the Beta roadmap.

### XIV. UI Design Spec Authority

- `design/ui-spec.md` is the sole visual authority for frontend implementation
- The `ui_spec_ref` field in `task.json` lists headings to reference before coding
- Design specifications take precedence over prose descriptions in `steps`
- If `design/ui-spec.md` section is empty, implement based on `steps` and add comments

### XV. Security Hardening

Applies to Tasks 99–105. These specifications are non-negotiable for production deployment:

- **Admin config leak prevention (Task 99)**: API responses containing admin configuration MUST mask
  sensitive fields. The following categories of fields MUST be redacted in `GET /api/v1/admin/config`
  responses: JWT signing keys, OSS access keys/secrets, LLM API keys, database passwords, and any
  other fields tagged as `sensitive` in the configuration schema. Redacted fields MUST return the
  literal string `***REDACTED***`. The frontend MUST function correctly with redacted values (e.g.,
  showing "已配置" status instead of the actual key).
- **CORS policy (Task 100)**: The origin allowlist MUST be loaded from `config.yaml > security.
  allowed_origins` as an array. Wildcard `*` is FORBIDDEN in any non-development environment.
  The CORS middleware MUST be registered in the Gin router before route handlers to ensure it
  applies to all responses including error responses.
- **Auth real-time state check (Task 101)**: The `AuthMiddleware` MUST verify `is_banned = false`
  and `role` on every protected-route request. Ban or role changes MUST NOT require user
  re-login to take effect. The frontend `AuthProvider` MUST poll `/api/v1/auth/me` at a
  configurable interval (default 5 minutes) to refresh user state.
- **Error message sanitization (Task 102)**: Direct exposure of `err.Error()` to clients is
  FORBIDDEN. Backend MUST use the standard error envelope. Frontend MUST use toast notifications
  for error display; `console.error` is acceptable for development only and MUST be removed
  before production.
- **Account deletion & OSS path isolation (Task 103)**: User deletion MUST be soft-delete only
  (`deleted_at` timestamp). Associated content MUST be marked `author_deleted` (visible but
  author-attributed as "已注销用户"). OSS user directories MUST follow the pattern
  `/uploads/{user_id}/` to enable lifecycle-based cleanup without blocking the deletion API.
- **Goroutine panic recovery (Task 104)**: The Gin HTTP server MUST include a recovery middleware
  that catches panics in handler goroutines and returns HTTP 500. Background goroutines (embedding,
  notifications, etc.) MUST include `defer func() { if r := recover(); r != nil { ... } }()`.
  A panic MUST NEVER crash the server process.
- **Protected route guard (Task 105)**: Frontend routes under `/studio/*`, `/settings/*`,
  `/messages/*`, `/rehab/*`, `/judge/*`, `/admin/*` MUST be wrapped in a `(protected)` layout
  group. Unauthenticated access MUST redirect to `/login?redirect=<original_path>`. Banned users
  MUST see an EmptyState with "账号已被封禁" message and an appeal link.

### XVI. Performance & Reliability

Applies to Tasks 128–130, 135, 141–143. These are binding performance requirements:

- **N+1 query elimination (Task 128–130)**: Hot-path list queries (content list, IP list,
  recommendation feed) MUST use GORM `Preload()` or `Joins()` to batch-load associations.
  The `SELECT N+1` pattern is FORBIDDEN in any handler that returns a list endpoint.
  Affected handlers: content list, IP detail, recommendation engine, hot content queries.
- **Structured logging (Task 141)**: All backend logging MUST use `slog` (Go 1.21+ structured
  logging). `log.Printf` and `fmt.Println` are FORBIDDEN in production code. Log format MUST be
  JSON with snake_case field names. Required fields: `time`, `level`, `msg`, `trace_id`. Request
  logs MUST include `method`, `path`, `status`, `duration_ms`, `client_ip`. Sensitive fields
  (passwords, tokens, API keys) MUST be redacted.
- **Graceful shutdown (Task 142)**: The Go server MUST use `http.Server.Shutdown(ctx)` with a
  timeout read from `config.yaml > server.shutdown_timeout`. During shutdown, new requests MUST
  be rejected; in-flight requests MUST complete within the timeout. The frontend (Next.js) has
  no special shutdown requirement in serverless mode.
- **Frontend data caching / SWR (Task 135)**: The frontend MUST use SWR (stale-while-revalidate)
  for all list and detail API calls. Cache keys MUST include query parameters. The `revalidate`
  interval MUST be configurable per data type (e.g., 30s for feed, 60s for user profile).
  Optimistic updates MUST be applied for like/bookmark operations to avoid UI flicker.

### XVII. Data Integrity & Soft Delete

- **Soft delete preferred (DEC-031)**: User-facing delete operations (user account deletion,
  content deletion by author) MUST use soft delete (`deleted_at` column) by default. Physical
  deletion (`DELETE FROM`) is permitted ONLY for low-analytic-value data (browse_history,
  read notifications) and MUST be explicitly documented at the deletion site with a reference
  to DEC-031. Content soft-delete MUST set `status = 'deleted'` and `deleted_at = NOW()`;
  queries MUST filter `deleted_at IS NULL` by default. Admin purge operations are exempt but
  MUST be logged.
- **Content soft-delete (Task 127)**: When a content author deletes their own content, the row
  MUST NOT be removed. Instead, set `status = 'deleted'` and `deleted_at = NOW()`. The deleted
  content MUST be excluded from all public list/detail queries. The author MAY view their own
  deleted content via a special `/studio/contents?status=deleted` filter. Admin content purge
  (permanent removal) is a separate operation requiring explicit confirmation.

### XVIII. Request Validation & Internationalization

- **i18n mandatory (Task 108)**: After Task 108 is complete, ALL newly added UI string literals
  MUST be referenced via `next-intl`'s `useTranslations()`. Hardcoded Chinese or English text in
  components is FORBIDDEN. Translation files reside in `frontend/messages/` organized by locale
  (`zh.json`, `en.json`). New feature pages MUST namespace translation keys by page path (e.g.,
  `messages.zh.notifications`). Missing i18n keys are treated as lint errors.
- **Dark mode consistency (Task 107)**: ALL pages and components MUST pass dark mode rendering
  checks. The Sidebar, StudioSidebar, and Footer components MUST use design tokens (`canvas`,
  `border`, `fg`, `accent`, `tag` with `light`/`dark` variants) from `tailwind.config.ts`. Raw
  hex colors in these components are FORBIDDEN. The `next-themes` provider MUST be present at the
  root layout level.
- **Frontend data validation (Task 110)**: Frontend API call functions MUST validate response
  structure using a type guard or schema validation (zod). Invalid responses MUST be handled
  gracefully (show error toast, do not crash). The `normalizeContentItem` function MUST filter
  out items missing `id`, `title`, or `zone` before rendering.

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

**Version**: 1.3.0 | **Ratified**: 2026-04-15 | **Last Amended**: 2026-06-28

### 1.3.0 Changelog (2026-06-28)

- **Principle VIII**: Updated UI design reference from GitHub visual language to Indigo-based flat design (see `design/design-system.md`).
- **Principle XVII**: Updated soft-delete policy from "only" to "preferred" per DEC-031. Physical deletion now permitted for low-analytic-value data (browse_history, read notifications).
- **Toolchain**: PostgreSQL minimum version raised from 15+ to 16+ (matching `AGENTS.md` and `docker-compose.yml`).
- **Principle XIII**: Fixed `features.web_agent_enabled` → `agent.web_agent_enabled` (field path correction).
