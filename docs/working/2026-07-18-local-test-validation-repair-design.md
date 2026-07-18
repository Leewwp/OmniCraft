# Local Test Validation Repair Design

Created: 2026-07-18  
Expires: 2026-09-18

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

`playwright.cross-stack.config.ts` is used only after local prerequisites pass. It launches the real Go backend and Next.js and selects integration specs. Cross-stack specs must not use `page.route`, an in-process mock API server, or other API substitutes.

### Environment gates

The runner has two phases:

1. Preflight checks Docker and Compose availability, service health, actual localhost port bindings for PostgreSQL and Redis, a writable isolated Go build cache, a usable Go standard library, and safe local test connection settings.
2. Readiness runs after the backend and frontend start, checking process ownership, backend `/healthz`, frontend reachability, and selected real API endpoints.

Any unmet prerequisite returns a non-zero result and produces `BLOCKED`; tests do not start in that case.

### Reporting

Each layer reports one of `PASS`, `FAIL`, or `BLOCKED` with its executed command, exit code, and a concise blocker. A mocked browser pass is explicitly labeled `mocked browser contract` and cannot change the cross-stack result.

## Safety

- Real database integration uses only a local loopback PostgreSQL test DSN and unique temporary test databases.
- Scripts only inspect or start the named Compose data services; they must not run `down -v`, remove volumes, or clear a development database.
- Connection strings and environment diagnostics are redacted.

## Verification

Tests first prove that mocked configuration does not require the Go backend and that a missing host port produces a non-zero blocked result. After implementation, verify mocked browser, Go unit, PostgreSQL integration, and cross-stack paths independently. The final report must leave any unrun layer blocked rather than inferred as passing.
