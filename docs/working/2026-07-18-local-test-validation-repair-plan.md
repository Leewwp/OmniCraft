# Local Test Validation Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Separate local mocked and real cross-stack verification, and make unsafe or unavailable local dependencies fail truthfully before a test is started.

**Architecture:** Browser tests use two explicit Playwright configurations. A PowerShell runner validates safe local prerequisites before starting the real stack and reports each layer independently. PostgreSQL test helpers reject non-loopback or non-administration DSNs before they can create temporary databases.

**Tech Stack:** Playwright, Next.js, Go testing/GORM, PowerShell, Docker Compose.

Created: 2026-07-18  
Expires: 2026-09-18

---

### Task 1: Enforce temporary PostgreSQL DSN safety

**Files:**
- Modify: `backend/internal/testutil/postgres.go`
- Modify: `backend/internal/testutil/postgres_test.go`

- [ ] **Step 1: Write failing tests**

Add table-driven tests for a validator that accepts loopback `postgres` admin DSNs and rejects remote hosts and `dbname=omnicraft`.

- [ ] **Step 2: Run the focused test red**

Run: `$env:GOCACHE='C:\tmp\omnicraft-go-cache-local-validation'; go test ./internal/testutil -run TestValidateEphemeralPostgresAdminDSN -count=1`

Expected: FAIL because the validator does not exist.

- [ ] **Step 3: Implement the minimum validator**

Validate the resolved connection settings before `OpenEphemeralPostgres` opens a connection. Permit only `127.0.0.1`, `localhost`, or `::1`, require database `postgres`, and preserve the existing default DSN.

- [ ] **Step 4: Run focused and dependent Go tests**

Run the focused validator test, then `go test ./internal/model ./internal/repository -count=1` with the isolated cache and a safe local test admin DSN.

### Task 2: Support an explicitly injected test-stack database

**Files:**
- Modify: `backend/config/config.go`
- Modify: `backend/config/config_test.go`

- [ ] **Step 1: Write failing configuration tests**

Prove test-only PostgreSQL and Redis overrides applied after every normal configuration source, including config overrides, are accepted only in test mode. PostgreSQL must target loopback and name an ephemeral `omnicraft_test_` database; Redis must target loopback and use a non-zero dedicated DB. Prove ordinary development database/Redis DB 0 and remote hosts are rejected, and prove a configured `DB_READ_DSN` is cleared in test mode.

- [ ] **Step 2: Run the configuration test red**

Run: `$env:GOCACHE='C:\tmp\omnicraft-go-cache-local-validation'; go test ./config -run TestLoadTestDatabaseOverride -count=1`

Expected: FAIL because no safe test-stack override exists.

- [ ] **Step 3: Implement the minimum test-only override**

After all dotenv, environment, and config-override processing, allow `OMNICRAFT_TEST_MODE=1` plus `OMNICRAFT_TEST_DB_DSN` and `OMNICRAFT_TEST_REDIS_DB` to override backend dependencies only when they meet the local ephemeral PostgreSQL and non-zero Redis rules. Clear `cfg.Database.ReadDSN` unconditionally in test mode so no replica can target a non-test database. Reject invalid values before database or Redis initialization.

- [ ] **Step 4: Run the focused test green**

Run the focused configuration test with the isolated Go cache.

### Task 3: Split Playwright entry points

**Files:**
- Create: `frontend/playwright.mocked.config.ts`
- Create: `frontend/playwright.cross-stack.config.ts`
- Modify: `frontend/playwright.config.ts`
- Modify: `frontend/package.json`
- Modify: `frontend/e2e/contract-smoke.spec.ts`
- Create: `frontend/e2e/cross-stack-health.integration.spec.ts`
- Create: `frontend/scripts/playwright-config.test.mjs`

- [ ] **Step 1: Write a failing configuration contract test**

Test the standard mocked npm entry point and mocked config source: it must name the mocked config and contain only a Next web server. Test the cross-stack config source selects only `*.integration.spec.ts`, starts no web servers, and has at least one matching integration spec. Test the runner rejects a zero-test Playwright result.

- [ ] **Step 2: Run the configuration test red**

Run: `node frontend/scripts/playwright-config.test.mjs`

Expected: FAIL because the scripts still select the shared configuration.

- [ ] **Step 3: Implement explicit configurations**

Move the shared browser defaults into a reusable local helper or retain them in the legacy config only where needed. Point `test:contracts` to the mocked config and add `test:cross-stack` for the real config. Ensure the contract-smoke server remains exclusive to mocked execution and cannot collide with a real Go backend. Add a real `cross-stack-health.integration.spec.ts` with no route mocks or in-process API server; it loads the browser frontend and asserts a safe real backend API response.

- [ ] **Step 4: Run the configuration test green**

Run: `node frontend/scripts/playwright-config.test.mjs`

Expected: PASS.

### Task 4: Add fail-fast local layer runner

**Files:**
- Create: `scripts/test/preflight.ps1`
- Create: `scripts/test/preflight.tests.ps1`
- Create: `scripts/test/readiness.ps1`
- Create: `scripts/test/readiness.tests.ps1`
- Create: `scripts/test/run-test-layer.ps1`
- Create: `scripts/test/run-test-layer.tests.ps1`
- Modify: `frontend/package.json`

- [ ] **Step 1: Write failing PowerShell contract tests**

Cover unsafe PostgreSQL/Redis input rejection, absent 5432/6379 bindings returning `BLOCKED` and non-zero, redacted output, mocked-layer isolation, a zero-test cross-stack result returning `FAIL`, and a required cross-stack layer not being converted to PASS when preflight fails. Cover an already-listening 3000 or 8080 returning `BLOCKED`; cover creation, migration, injection, and final cleanup of a unique ephemeral database plus a dedicated non-zero Redis DB for cross-stack execution.

- [ ] **Step 2: Run the runner tests red**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test/preflight.tests.ps1`

Expected: FAIL because the scripts do not exist.

- [ ] **Step 3: Implement preflight, readiness, and runner**

Preflight inspects only the current local Compose project and services, validates Docker health/host port bindings, Go/cache safety, safe test DSNs, a dedicated non-zero Redis test DB, and empty 3000/8080 test ports, then returns structured `PASS|FAIL|BLOCKED`. It prints the precise non-destructive recovery command for port drift: `docker compose up -d --force-recreate postgres redis`; it never executes destructive Docker commands. For CrossStack, the runner creates and migrates a unique local temporary database through the Compose PostgreSQL service, clears a dedicated Redis test DB, starts Go and Next itself with safe test-only overrides applied after all ordinary config sources and with read replicas disabled, records their process IDs, then calls the no-webserver Playwright configuration. `finally` cleanup stops only those owned processes, drops only the generated temporary database, and clears only the dedicated Redis DB. Readiness checks backend and frontend only after their processes start. The runner uses the mocked Playwright command for `MockedBrowser`, Go for `GoUnit`, safe PostgreSQL-dependent tests for `PostgresIntegration`, and the real cross-stack config for `CrossStack`.

- [ ] **Step 4: Run PowerShell tests green**

Run each `scripts/test/*.tests.ps1` file.

### Task 5: Restore local services and verify each layer

**Files:**
- No repository source changes required.

- [ ] **Step 1: Recover declared Docker data services safely**

Run `docker compose up -d --force-recreate postgres redis`, then confirm Compose health and loopback bindings for 5432/6379. Do not run `down -v` or delete volumes.

- [ ] **Step 2: Run isolated layers**

Run the mocked browser layer without Go/PostgreSQL, Go unit, PostgreSQL integration, and cross-stack layers separately. A missing external prerequisite is reported as `BLOCKED`, not converted into a pass.

- [ ] **Step 3: Run repository regression checks**

Run frontend unit/lint/build, focused Go tests, `go vet ./...`, and `go build ./...` using an isolated writable Go cache.

- [ ] **Step 4: Review and commit**

Perform specification and code-quality review, update this plan’s completed checkboxes only if every required deterministic check passed, and create one precise implementation commit. Do not update UI/community/Ops task status.
