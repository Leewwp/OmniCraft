# OmniCraft Code Review Log

## Coverage Index

| Task IDs | Title / Batch | Status | Last Reviewed At | Base SHA | Head SHA | Notes |
|---|---|---|---|---|---|---|
| 0 | Review baseline and metadata | Complete | 2026-05-07 12:39 +08:00 | a384a51fc280da80bd3caf3fa561be2826295639 | a384a51fc280da80bd3caf3fa561be2826295639 | Established review log and task coverage policy. |
| 1,2,3,4,5,43,52,63,75 | Backend foundation, config, database, deployment | Issues Found | 2026-05-07 12:49 +08:00 | a384a51fc280da80bd3caf3fa561be2826295639 | a384a51fc280da80bd3caf3fa561be2826295639 | Backend checks and migrations pass; deployment is blocked by frontend Docker build issues. |
| 6,7,8,9,10,17,18,19,44,45,53,54,55,56,57,58,60,61,65,66,67,68,69,78,79,80,81,82 | Backend APIs, auth, moderation, admin, Agent | Not Started | - | - | - | Planned Batch 2. Task 40 remains pending and is excluded from completed-work review. |
| 11,12,13,14,15,16,24,27,28,85,86 | Content, versioning, PR, social, cross-stack content flows | Not Started | - | - | - | Planned Batch 3. |
| 20,21,22,23,25,26,29,30,31,32,70,71 | Completed frontend public, protected, admin, messaging pages | Not Started | - | - | - | Planned Batch 4. Requires browser verification. |
| 33,34,35 | Tauri client | Not Started | - | - | - | Planned Batch 5. Task 35 is now marked complete. |
| 36,37,38,39,40,41,42,46,47,48,49,50,51,59,62,64,72,73,74,76,77,83,84 | Pending tasks | Pending | 2026-05-07 12:39 +08:00 | a384a51fc280da80bd3caf3fa561be2826295639 | a384a51fc280da80bd3caf3fa561be2826295639 | `passes: false` in `task.json`; do not mark reviewed as completed work. |

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
