# OmniCraft Code Review Log

## Coverage Index

| Task IDs | Title / Batch | Status | Last Reviewed At | Base SHA | Head SHA | Notes |
|---|---|---|---|---|---|---|
| 0 | Review baseline and metadata | Complete | 2026-05-07 12:39 +08:00 | a384a51fc280da80bd3caf3fa561be2826295639 | a384a51fc280da80bd3caf3fa561be2826295639 | Established review log and task coverage policy. |
| 1,2,3,4,5,43,52,63,75 | Backend foundation, config, database, deployment | Pass with Follow-up | 2026-05-07 20:32 +08:00 | a384a51fc280da80bd3caf3fa561be2826295639 | a384a51fc280da80bd3caf3fa561be2826295639 | Frontend Docker build chain fixed and verified; remaining deployment follow-ups are nginx TLS/cert wiring, npm audit registry, and security hardening. |
| 6,7,8,9,10,17,18,19,40,44,45,53,54,55,56,57,58,60,61,65,66,67,68,69,78,79,80,81,82 | Backend APIs, auth, moderation, admin, Agent | Pass with Follow-up | 2026-05-07 21:27 +08:00 | d1f6731bf0e2e118f882f37f12efd1d70a76e3c1 | d1f6731bf0e2e118f882f37f12efd1d70a76e3c1 | Important Batch 2 findings remediated and backend checks pass; minor route-contract cleanup can be handled separately. |
| 11,12,13,14,15,16,24,27,28,85,86 | Content, versioning, PR, social, cross-stack content flows | Not Started | - | - | - | Planned Batch 3. |
| 20,21,22,23,25,26,29,30,31,32,70,71 | Completed frontend public, protected, admin, messaging pages | Not Started | - | - | - | Planned Batch 4. Requires browser verification. |
| 33,34,35 | Tauri client | Not Started | - | - | - | Planned Batch 5. Task 35 is now marked complete. |
| 36,37,38,39,41,42,46,47,48,49,50,51,59,62,64,72,73,74,76,77,83,84 | Pending tasks | Pending | 2026-05-07 21:01 +08:00 | 64f4ed6dc111c95b6b4fc15e6a1d165495b64f83 | 64f4ed6dc111c95b6b4fc15e6a1d165495b64f83 | `passes: false` in `task.json`; Task 40 is no longer pending and is covered by Batch 2. |

## Review Entries

### 2026-05-07 12:39 +08:00 - Batch 0: Review Baseline And Metadata

**Task IDs:** 0

**Reviewer:** Codex using `superpowers:requesting-code-review` process locally

**Base SHA:** `a384a51fc280da80bd3caf3fa561be2826295639`

**Head SHA:** `a384a51fc280da80bd3caf3fa561be2826295639`

**Scope:** `task.json`, git status, current review plan.

**Verification:**
- `git status --short --branch`
- `git rev-parse HEAD`
- Node parse of `task.json`

**Result:** Complete.

**Findings:**
- Critical: none.
- Important: none.
- Minor: none.

**Notes:**
- At this baseline, `task.json` has 86 tasks: 63 complete and 23 pending.
- Pending task IDs: 36, 37, 38, 39, 40, 41, 42, 46, 47, 48, 49, 50, 51, 59, 62, 64, 72, 73, 74, 76, 77, 83, 84.
- Completed-task code review should proceed by batches and update the Coverage Index immediately after each batch.

### 2026-05-07 12:49 +08:00 - Batch 1: Backend Foundation, Config, Database, Deployment

**Task IDs:** 1, 2, 3, 4, 5, 43, 52, 63, 75

**Reviewer:** Codex using `superpowers:requesting-code-review` process locally

**Base SHA:** `a384a51fc280da80bd3caf3fa561be2826295639`

**Head SHA:** `a384a51fc280da80bd3caf3fa561be2826295639`

**Scope:** backend config, database initialization, migrations 001-036, Docker Compose, backend/frontend Dockerfiles, nginx, PgBouncer.

**Verification:**
- `cd backend; go test ./...` passed.
- `cd backend; go vet ./...` passed.
- `cd backend; go build ./...` passed.
- Fresh PostgreSQL initdb with `pgvector/pgvector:pg16` and mounted `backend/migrations` completed successfully through migration 036.
- `docker compose config` parsed successfully, with only an obsolete `version` attribute warning.
- `cd frontend; npm run build` passed, but did not generate `.next/standalone`.
- `docker build -t omnicraft-backend-review-check ./backend` passed.
- `docker build -t omnicraft-frontend-review-check ./frontend` failed at `npm ci` because `package-lock.json` is out of sync with `package.json` / dependency metadata.

**Result:** Issues Found. Do not consider the deployment portion of Task 2 reviewed as passing until the frontend Docker build is fixed.

**Findings:**
- Critical: none.
- Important:
  - Frontend Docker image cannot build because `npm ci` rejects the current lockfile. The Docker build error reports a missing `@swc/helpers@0.5.21` entry while `package-lock.json` still records `@swc/helpers@0.5.15` for Next.js 16.2.4. This blocks `docker compose up --build` for the frontend service.
  - Even after the lockfile is fixed, `frontend/Dockerfile` copies `/app/.next/standalone`, but `frontend/next.config.ts` does not enable `output: "standalone"`. A local `npm run build` completed without creating `.next/standalone`, so the Dockerfile will fail at the standalone copy step unless the Next config or Dockerfile strategy changes.
  - The frontend Docker context is too large. The Docker build transferred about 1.48 GB because `frontend/.dockerignore` is absent or effectively empty, so `node_modules`, `.next`, and other local artifacts are sent to Docker.
- Minor:
  - `docker-compose.yml` uses obsolete top-level `version: "3.9"`; Compose accepts it but warns that it is ignored.
  - `docker-compose.yml` sets `NEXT_PUBLIC_API_URL` only as a runtime environment value for the frontend service. Public Next.js environment variables are normally baked into client bundles at build time, so this setting should be reviewed with the Docker build args and browser-facing API URL strategy.
  - PgBouncer is exposed on host port 6432 while configured with `auth_type = trust`. This may be acceptable for local-only development, but it should be revisited in Task 40 security hardening or before shared deployment.

**Follow-up:**
- Regenerate `frontend/package-lock.json` with the package manager/version expected by the project and re-run `docker build ./frontend`.
- Add `output: "standalone"` to `frontend/next.config.ts` or change `frontend/Dockerfile` to use a non-standalone production strategy.
- Add a frontend `.dockerignore`.
- Decide whether client API calls should use a same-origin `/api` path through nginx or an explicit public API origin, then wire build-time and runtime env accordingly.

### 2026-05-07 20:32 +08:00 - Batch 1 Rerun: Frontend Docker Build Chain Fix

**Task IDs:** 1, 2, 3, 4, 5, 43, 52, 63, 75

**Reviewer:** Codex using `superpowers:receiving-code-review` and `superpowers:requesting-code-review` process locally

**Base SHA:** `a384a51fc280da80bd3caf3fa561be2826295639`

**Head SHA:** `a384a51fc280da80bd3caf3fa561be2826295639`

**Scope:** frontend Docker build chain, frontend lockfile/install behavior, standalone output, Docker context, Docker Compose frontend build args, backend and migration regression checks.

**Changes Applied:**
- Added `frontend/.dockerignore` to exclude `node_modules`, `.next`, build outputs, logs, local env files, and git metadata from Docker context.
- Added `output: 'standalone'` to `frontend/next.config.ts`.
- Changed `frontend/Dockerfile` default `NEXT_PUBLIC_API_URL` build arg to an empty string for same-origin nginx `/api` deployments.
- Changed `frontend/lib/api.ts` from `||` to `??` so an intentionally empty `NEXT_PUBLIC_API_URL` remains same-origin instead of falling back to `http://localhost:8080`.
- Added frontend Compose build arg `NEXT_PUBLIC_API_URL: ""` and matching runtime environment.
- Added `@swc/helpers@^0.5.21` as a frontend dev dependency so Docker `npm ci` satisfies the optional peer required by the `next-intl` SWC toolchain.

**Verification:**
- `cd frontend; npm install --ignore-scripts` completed and restored missing local npm binaries.
- `cd frontend; npm run lint` passed.
- `cd frontend; npm run build` passed and produced `.next/standalone`.
- `docker build -t omnicraft-frontend-review-check ./frontend` passed. Build context dropped from about 1.48 GB before the fix to about 329 KB on the uncached verified build.
- `docker compose build frontend` passed.
- `cd backend; go test ./...` passed.
- `cd backend; go vet ./...` passed.
- `cd backend; go build ./...` passed.
- `docker build -t omnicraft-backend-review-check ./backend` passed.
- Fresh PostgreSQL initdb with `pgvector/pgvector:pg16` and mounted `backend/migrations` completed successfully through migration 036.
- `docker compose config` parsed successfully with no obsolete `version` warning after removing the top-level Compose version key.

**Result:** Pass with Follow-up. The original frontend Docker build blockers are fixed and verified. Full deployment still has follow-up issues outside the frontend Docker build chain.

**Resolved Findings:**
- Resolved: stale lockfile blocked frontend `npm ci`.
- Resolved: Dockerfile expected `.next/standalone` without Next standalone output.
- Resolved: frontend Docker context was too large.
- Resolved: `NEXT_PUBLIC_API_URL` is now provided at build time and supports same-origin `/api` when intentionally empty.

**Remaining Issues / Optimization Points:**
- Important: current `nginx/nginx.conf` references Let's Encrypt certificate paths under `/etc/letsencrypt/live/omnicraft.example.com`, but `docker-compose.yml` does not mount those cert files. `nginx -t` with frontend/backend host aliases fails because the certificate files are absent. Either add certbot/cert volume wiring or provide a dev HTTP-only nginx config/profile.
- Important: `docker compose config` expands local `.env` values into command output. `.env` is gitignored, but operators should avoid pasting this command output into logs because it includes local secrets.
- Important: `npm audit --audit-level=moderate` could not run against the configured `npmmirror` registry because the audit endpoint returns `[NOT_IMPLEMENTED]`. Re-run audit against a registry that supports npm security advisory endpoints before release.
- Minor: nginx logs `listen ... http2` as deprecated; use the newer `http2` directive form.
- Minor: Next build warns that the `middleware` file convention is deprecated and should move to `proxy`.
- Minor: `npm install` on Windows reported cleanup warnings for locked native modules under `node_modules`; this did not affect Git-tracked files or verification, but a clean install may require stopping processes that hold those files.
- Minor: the lockfile changed broadly while adding the missing SWC helper peer. Review dependency drift before committing if strict dependency minimalism is required.

**Follow-up:**
- Decide whether to split nginx into dev/prod configs or add certbot volume support in Compose.
- Run `npm audit` with a supported registry.
- Consider pinning broader frontend dependency ranges if future lockfile churn is undesirable.

### 2026-05-07 21:01 +08:00 - Batch 2: Backend Auth, User, Moderation, Admin, Agent

**Task IDs:** 6, 7, 8, 9, 10, 17, 18, 19, 40, 44, 45, 53, 54, 55, 56, 57, 58, 60, 61, 65, 66, 67, 68, 69, 78, 79, 80, 81, 82

**Reviewer:** Codex using `superpowers:requesting-code-review` process locally

**Base SHA:** `64f4ed6dc111c95b6b4fc15e6a1d165495b64f83`

**Head SHA:** `64f4ed6dc111c95b6b4fc15e6a1d165495b64f83`

**Scope:** backend route wiring, auth/user APIs, security rate limits and headers, IP/moderation services, tag/reputation/social/admin flows, Agent/LLM APIs, messaging, rehab courses, LLM config management.

**Verification:**
- `git status --short --branch` showed branch `main` ahead of `origin/main` by 28 commits, with unrelated frontend working tree changes.
- `git rev-parse HEAD` returned `64f4ed6dc111c95b6b4fc15e6a1d165495b64f83`.
- Parsed `task.json` for Batch 2 task details; Task 40 now has `passes: true`.
- `cd backend; go test ./internal/...` passed.
- `cd backend; go test ./...` passed.
- `cd backend; go vet ./...` passed.
- `cd backend; go build ./...` passed.

**Result:** Issues Found. Static checks pass, but this batch should not be marked passing until the Important findings below are remediated and covered by focused tests/API checks.

**Findings:**
- Critical: none found.
- Important:
  - Agent endpoints are not protected by the daily Agent rate limiter. `middleware.AgentRateLimit` is implemented but never registered; `/agent/upload-assist`, `/agent/compliance-check`, `/agent/search`, `/agent/usage-guide/:id`, `/agent/moderate/:id`, and `/agent/chat/stream` only use `AuthRequired`, so Task 53-58 LLM cost controls can be bypassed except for the global IP limiter.
  - LLM provider API keys are stored as plaintext in `llm_configs.api_key_enc`. `CreateConfig` assigns the submitted `api_key` directly to `APIKeyEnc`, `UpdateConfig` forwards arbitrary updates without encryption, and list responses only mask that plaintext. This violates Task 82's AES-GCM-at-rest requirement.
  - Password change/account deletion do not invalidate issued refresh tokens. `invalidateUserTokens` deletes keys matching `refresh_token:{user_id}:*`, but the auth flow never stores refresh tokens under that pattern; refresh-token validation only checks `blacklist:token:<raw-token>`. Existing refresh tokens can still be exchanged after password change until expiry.
  - The tag suggestion abuse limit required by Task 45 is missing. `POST /contents/:id/tags/suggest` is routed with auth only, and `TagService.SuggestTag` writes a suggestion directly without the `tag_suggest:{user_id}:{content_id}:{date}` Redis limit or duplicate submission check.
  - Rehab course completion can be claimed immediately. The routes expose `GET /rehab/courses`, `GET /rehab/courses/:id`, `POST /rehab/courses/:id/complete`, and progress, but no `start` endpoint; `CompleteCourse` inserts completion and awards reputation without checking a prior `started_at` or `min_reading_sec`.
  - Admin appeal approval does not restore content. `ResolveAppeal` updates the appeal row only; when status is `approved`, it does not load the appeal target or set the banned/under-review content back to `published`, so Task 19/61's appeal workflow is functionally incomplete.
  - `GET /ips/:id/contents` returns an empty placeholder regardless of real content. `IPHandler.GetIPContents` ignores id and pagination and responds with empty `contents`, breaking Task 10's IP detail content list.
  - IP creation does not trigger AI review or apply banned-IP cascading. `IPService.CreateIP` creates a pending IP and returns; there is no call into the review service, and the banned-IP cascade path exists only as `BanIP` with no automatic invocation from IP AI review.
  - Login/register-specific throttling is not implemented. Task 6 requires Redis token bucket 5 requests/minute for auth endpoints, but `/auth/register` and `/auth/login` rely only on the global 100/minute/IP middleware.
- Minor:
  - Agent DB-backed config is not integrated into `llm.NewProvider`; the factory still reads only `config.yaml`, so activated DB LLM configs do not affect normal Agent calls.
  - Messaging API route names do not match Task 66's `/conversations` contract; current routes are under `/messages`.

**Follow-up:**
- Add focused tests or API checks for Agent rate limiting, tag suggestion limits, refresh-token revocation, rehab start/complete timing, appeal approval restore behavior, and IP content listing.
- Re-run this batch after remediation before starting Batch 3.

### 2026-05-07 21:27 +08:00 - Batch 2 Rerun: Backend API Remediation

**Task IDs:** 6, 7, 8, 9, 10, 17, 18, 19, 40, 44, 45, 53, 54, 55, 56, 57, 58, 60, 61, 65, 66, 67, 68, 69, 78, 79, 80, 81, 82

**Reviewer:** Codex using `superpowers:receiving-code-review`, `superpowers:test-driven-development`, and `superpowers:requesting-code-review` process locally

**Base SHA:** `d1f6731bf0e2e118f882f37f12efd1d70a76e3c1`

**Head SHA:** `d1f6731bf0e2e118f882f37f12efd1d70a76e3c1`

**Scope:** remediation for the nine Batch 2 findings: Agent and auth rate limiting, LLM API key encryption, refresh-token invalidation, tag suggestion abuse limits, rehab start/complete timing, appeal approval restore, IP content listing, and IP AI review cascade.

**Changes Applied:**
- Added credential-specific 5/minute Redis limiter to register/login routes.
- Added Agent daily Redis limiter to the `/agent` route group and nil-safe limiter guards.
- Stored refresh tokens by user-scoped SHA-256 Redis keys, required registered refresh tokens on refresh, and deleted them on logout/password change/account deletion.
- Encrypted LLM API keys with AES-GCM before storing `llm_configs.api_key_enc`, decrypting only for connection tests/provider construction, and ignoring direct `api_key_enc` update attempts.
- Added tag suggestion per-user/per-content/per-day Redis rate limiting.
- Added rehab `started_at` migration/model support, `POST /rehab/courses/:id/start`, and completion checks for prior start plus minimum reading time.
- Restored content status to `published` when admins approve content appeals.
- Implemented `/ips/:id/contents` using real content queries instead of a placeholder response.
- Triggered IP AI review after IP creation and added block/violation cascade to ban the IP and related content.
- Added regression tests for route limiter wiring and service-level remediation helpers.

**Verification:**
- `cd backend; go test ./internal/handler ./internal/service` passed after the new regression tests failed on the old behavior.
- `cd backend; go test ./...` passed.
- `cd backend; go vet ./...` passed.
- `cd backend; go build ./...` passed.

**Result:** Pass with Follow-up. The nine Important/P1-P2 findings from the review request are remediated and verified by backend tests/build checks.

**Remaining Follow-up:**
- The earlier minor messaging route naming mismatch (`/messages` vs `/conversations`) remains a compatibility cleanup item outside the nine requested findings.
- Browser verification was not required for this backend-only remediation batch.
