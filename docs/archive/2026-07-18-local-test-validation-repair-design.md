# Local Test Validation Repair Design

**创建日期**: 2026-07-18
**预计失效日期**: 2026-09-18

## Purpose

Make local test outcomes honest and repeatable without changing CI, production-readiness tracking, or UI/community business behavior. A mocked browser result must never be reported as a real cross-stack result.

## Scope

- Restore the documented local Docker PostgreSQL and Redis host-port prerequisites without deleting volumes or development data.
- Split mocked browser contracts from real cross-stack browser tests.
- Add local preflight and readiness checks that fail with actionable output when required dependencies are unavailable.
- Emit separate outcomes for mocked browser, Go unit, PostgreSQL integration, and real cross-stack layers.

## Non-goals

- No GitHub Actions, CI, Ops plan, project-level verifier, UI/community business code, or task-status changes.
- No Docker volume deletion, remote/production database access, or production secrets.

## Design

### Test configurations

`playwright.mocked.config.ts` starts only Next.js and runs mocked/contract specs. It must not launch Go, PostgreSQL, or Redis.

`playwright.cross-stack.config.ts` is used only after local prerequisites pass and selects integration specs. The layer runner, rather than Playwright, owns the real Go backend and Next.js processes so it can reject pre-existing listeners, retain process provenance, and stop both processes in cleanup. Cross-stack specs must not use `page.route`, an in-process mock API server, or other API substitutes.

The legacy npm entry points are migrated deliberately: `test:contracts` must select the mocked configuration, while the real cross-stack entry point must select the cross-stack configuration through the layer runner. A regression test proves that the ordinary mocked command never starts the Go web server.

### Environment gates

The runner has two phases:

1. Preflight checks Docker and Compose availability, service health, actual localhost port bindings for PostgreSQL and Redis, a writable isolated Go build cache, a usable Go standard library, safe local test connection settings, and that test ports 3000/8080 are unoccupied before the runner starts its own processes. The PostgreSQL admin DSN must target loopback only and its `postgres` administration database; non-loopback hosts and the development database name `omnicraft` are rejected before any temporary database can be created or dropped. Redis must be loopback-only and use a dedicated non-zero test DB.
2. Readiness runs after the backend and frontend start, checking process ownership, backend `/healthz`, frontend reachability, and selected real API endpoints.

Any unmet prerequisite returns a non-zero result and produces `BLOCKED`; tests do not start in that case.

### Reporting

Each layer reports one of `PASS`, `FAIL`, or `BLOCKED` with its executed command, exit code, and a concise blocker. A mocked browser pass is explicitly labeled `mocked browser contract` and cannot change the cross-stack result.

## Safety

- Real database integration and the cross-stack backend use only a local loopback PostgreSQL test DSN and a unique temporary test database. The runner creates it, applies migrations, injects it through a test-only configuration override after all normal configuration sources have loaded, clears any read-replica DSN, and drops it in `finally` cleanup. Redis uses a dedicated non-zero test DB injected at the same final configuration stage; the runner clears only that test DB before and after execution. Tests prove that unsafe PostgreSQL and Redis inputs are rejected.
- When the declared Compose port mappings do not match the running containers, the recovery path verifies the named Compose project/services, runs `docker compose up -d --force-recreate postgres redis`, preserves named volumes, and rechecks host-port bindings. Scripts must not run `down -v`, remove volumes, or clear a development database.
- Connection strings and environment diagnostics are redacted.

## Verification

Tests first prove that mocked configuration does not require the Go backend and that a missing host port produces a non-zero blocked result. After implementation, verify mocked browser, Go unit, PostgreSQL integration, and cross-stack paths independently. The final report must leave any unrun layer blocked rather than inferred as passing.
