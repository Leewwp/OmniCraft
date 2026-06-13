# OmniCraft Test Script Review - 2026-06-12

This file is the running review log for test-script quality. Each reviewed batch is appended to the end so the task can resume safely after context compaction.

## Review Scope

- Backend Go tests under `backend/**`
- Frontend node tests under `frontend/tests`, `frontend/lib`, and component-local `*.test.cjs`
- Frontend Playwright tests under `frontend/e2e`
- Root E2E and helper scripts under `e2e` and `scripts`

## Review Criteria

A test is treated as weak or invalid when it can pass while the real feature is broken. The main patterns checked are:

- Over-broad accepted outcomes, such as treating unrelated status codes as success.
- Self-fulfilled or self-authored results, where the test creates the response it claims to verify.
- Direct DB mutation or environment mutation that bypasses the API under test.
- Source-code string scanning used as a substitute for runtime behavior.
- Mock-only UI tests presented as backend contract or end-to-end coverage.
- Screenshot or page-load checks that do not assert the business result.
- Skips or unconditional passes when required seed data is missing.
- Test files that are not wired into normal verification commands.

## Batch 1 - Root Python E2E Scripts

Reviewed:

- `e2e/test_search_download.py`
- `e2e/test_admin_journey.py`

### Finding TST-001 - Critical - Download authorization check accepts unrelated outcomes

File: `e2e/test_search_download.py:267`

The unauthenticated download check falls back to `content_id=1` when no real content is found, then treats `404` as a passing unauthorized result:

```python
is_unauthorized = r.status_code in (401, 403, 404)
```

This can pass when the route does not enforce auth at all, as long as the dummy content is missing. It does not prove the download endpoint rejects unauthenticated access for an existing published attachment.

Required repair:

- Seed or create a known published content item with a downloadable attachment.
- Call `GET /api/v1/contents/{id}/download` without auth.
- Assert the exact expected auth failure, preferably `401` with `code == "UNAUTHORIZED"`.
- Treat missing seed content as setup failure, not as success.

### Finding TST-002 - Critical - Authenticated download test treats success, forbidden, and missing content as equally valid

File: `e2e/test_search_download.py:295`

The authenticated download check accepts `200`, `403`, and `404`:

```python
is_valid = r.status_code in (200, 403, 404)
```

Those outcomes represent mutually different product states: success, permission failure, and missing resource. This test cannot detect broken signed URL generation, broken attachment lookup, or accidental authorization denial.

Required repair:

- Split this into separate scenarios: verified user success, unverified user rejection, low-reputation rejection, missing content.
- For the success path, assert `200`, `download_url`, `expires_in`, and no direct redirect.
- For rejection paths, assert exact status and error `code`.

### Finding TST-003 - Critical - Unverified-user download check always passes

File: `e2e/test_search_download.py:331`

The test records `PASS` unconditionally:

```python
record("8-Download-Unverified", True, detail)
```

The inline comment says `200` is acceptable even though the step title says the user should be rejected. This turns a security regression into a passing result with a warning string.

Required repair:

- Create or select a definitely unverified user.
- Assert the endpoint rejects the request with the expected status and error code.
- Fail on `200`; do not record warnings as passing test results.

### Finding TST-004 - High - Search UI check only proves the page did not show a generic 500

File: `e2e/test_search_download.py:177`

The Chinese search browser step passes when `has_error` is false. It does not verify that a request was sent with the Chinese query, that the backend returned matching content, or that the UI rendered the result. A page that silently shows an empty state or never sends the search request can pass.

Required repair:

- Seed content with a known Chinese title.
- Capture the request URL or query parameter from the browser.
- Assert the response contains the seeded item and the UI renders its title.
- Assert hidden, banned, or deleted content is absent with controlled seed data.

### Finding TST-005 - High - Screenshot saved is treated as a page test

File: `e2e/test_search_download.py:121`

The search page step records success immediately after taking a screenshot. A screenshot file proves navigation did not throw; it does not prove the page loaded the expected UI or any business behavior worked.

Required repair:

- Assert stable page landmarks, search input, results panel, and no auth/error boundary.
- Keep screenshots as evidence, not as the pass condition.

### Finding TST-006 - Critical - Admin journey mutates the database to bypass an API failure

File: `e2e/test_admin_journey.py:331`

When resolving a report via API fails with a missing-column error, the script directly updates the database and records the step as passed:

```python
UPDATE reports SET status='resolved' WHERE id={report_id};
record("C1", True, "Resolve report ... via DB (API has action_taken column bug)")
```

This invalidates the stated goal of testing admin report resolution and audit behavior. It can hide the exact regression the script claims to verify.

Required repair:

- Remove DB fallback from the assertion path.
- Seed data through migrations, fixtures, or public/admin APIs before the test starts.
- Fail when the report resolution API returns `500`.
- Assert the report state and audit row are produced by the API transaction.

### Finding TST-007 - High - Missing reports or feedback tickets are treated as passing coverage

File: `e2e/test_admin_journey.py:380`
File: `e2e/test_admin_journey.py:433`

The script records success when there is no report or feedback data:

```python
record("C1", True, "No reports exist in system - skip (no data to test)")
record("C3", True, "No feedback tickets exist - skip (no data to test)")
```

This means the audit-log assertions can pass without exercising report resolution or feedback closure.

Required repair:

- Create required report and feedback fixtures as setup.
- Fail fast if fixture creation fails.
- Only mark the workflow passed after the mutation and audit assertions both pass.

### Finding TST-008 - High - Audit response shape drift is accepted as success

File: `e2e/test_admin_journey.py:402`
File: `e2e/test_admin_journey.py:453`

The script treats a non-list audit log response as acceptable. That hides response-contract regressions in an admin audit endpoint.

Required repair:

- Define the accepted audit response schema.
- Assert the exact top-level collection key and item fields.
- Fail on unknown response shapes.

### Finding TST-009 - Medium - Admin setup changes roles directly in the production-like database

File: `e2e/test_admin_journey.py:151`

The script promotes `admin09@test.com` through a direct SQL update. That makes the test dependent on a mutable shared database and bypasses the user-management or seed-data contract.

Required repair:

- Move user creation and role setup into a deterministic fixture step.
- Use an isolated test database or a dedicated seed script with cleanup.
- Make setup failures explicit.

### Finding TST-010 - Medium - Root E2E scripts are not wired into normal test commands

Files:

- `e2e/test_search_download.py`
- `e2e/test_admin_journey.py`
- `frontend/package.json:11`

`frontend/package.json` exposes `test:e2e` as `playwright test`, which runs `frontend/e2e`. The root Python scripts are not part of that command, so their results are manual evidence only unless a separate CI or release script invokes them.

Required repair:

- Add an explicit documented command for root E2E scripts, or migrate them into Playwright/API tests that run in CI.
- If they remain manual release checks, label them as such and do not count them as automated gates.

## Batch 2 - Static Source-Scanning Tests

Reviewed representative files:

- `frontend/tests/register-captcha.test.mjs`
- `frontend/tests/feedback-form-upload.test.mjs`
- `frontend/tests/captcha-ticket-reset.test.mjs`
- `frontend/tests/sheet-music-viewer-download.test.mjs`
- `frontend/components/verification/CaptchaWidget.test.cjs`
- `backend/cmd/server/main_test.go`
- `backend/internal/service/agent_chat_test.go`
- `backend/internal/service/agent_publish_assist_test.go`
- `backend/internal/service/agent_visibility_test.go`
- `backend/internal/handler/routes_security_test.go`
- `backend/internal/handler/agent_deploy_disabled_test.go`
- `backend/internal/handler/content_download_test.go`
- `backend/internal/repository/search_repo_test.go`

### Finding TST-011 - High - Frontend captcha/register tests scan source instead of exercising behavior

File: `frontend/tests/register-captcha.test.mjs:12`
File: `frontend/tests/captcha-ticket-reset.test.mjs:15`

These tests inspect source order with `indexOf` and regexes. They do not render the form, click submit, or observe whether a network request is actually blocked or sent. Dead code, duplicated strings, or a refactor that preserves text while changing behavior can pass.

Required repair:

- Keep a small structural guard only if it protects a brittle integration contract.
- Add runtime tests with a mocked API client or Playwright route capture.
- For `aliyun_v2` empty-token behavior, assert no `/api/v1/auth/register` request is made.
- For one-time captcha reset, assert the widget key/state changes after a completed or failed submission according to the product contract.

### Finding TST-012 - High - CaptchaWidget test does not prove token exchange semantics

File: `frontend/components/verification/CaptchaWidget.test.cjs:5`

The test scans `CaptchaWidget.tsx` for snippets such as `initAliyunCaptcha`, `captcha_verify_param`, and `onToken(verifyResult.captcha_token)`. This does not prove the Aliyun callback parameter is sent to the backend, that the returned application token is used, or that bypass is impossible when the provider is `aliyun_v2`.

Required repair:

- Execute the component with a stubbed `window.initAliyunCaptcha`.
- Simulate the Aliyun callback.
- Assert the backend verify endpoint receives `captcha_verify_param`.
- Assert `onToken` receives only the backend `captcha_token`.
- Assert `provider: "aliyun_v2"` never emits `"bypass"`.

### Finding TST-013 - High - Feedback upload test checks source order, not failed-upload behavior

File: `frontend/tests/feedback-form-upload.test.mjs:8`

The test checks that `fetch`, `if (!uploadRes.ok)`, and `setScreenshotGrant` appear in a particular order. It does not prove a failed OSS `PUT` prevents grant storage or prevents feedback submission with a stale attachment grant.

Required repair:

- Render `FeedbackForm` or extract the upload helper into a testable function.
- Mock the presign response and an OSS `PUT` failure.
- Assert no screenshot grant is stored after the failed `PUT`.
- Assert the user-facing error is sanitized and the captcha reset behavior still follows the intended contract.

### Finding TST-014 - Critical - Agent publish-assist tests skip when the feature is missing

File: `backend/internal/service/agent_publish_assist_test.go:11`
File: `backend/internal/service/agent_publish_assist_test.go:18`

The tests named as feature checks call `t.Skip` when `UploadAssist` or `ComplianceCheck` is absent. That means deleting the feature can turn the test into a skip instead of a failure.

Required repair:

- Replace `t.Skip` with `t.Fatal` for required Beta behavior.
- Add service-level tests that call upload assist and compliance check with controlled LLM output.
- Assert field length validation, JSON parsing, sanitization, risk-level mapping, and error handling.
- Add handler tests that verify route status codes and response schemas.

### Finding TST-015 - High - Agent chat/context tests only assert token presence

File: `backend/internal/service/agent_chat_test.go:11`
File: `backend/internal/service/agent_chat_test.go:46`

These tests check for substrings such as `pageCtx`, `contextType`, `allowedRoles`, and `maxCtxMsgs`. They do not prove the handler rejects invalid roles, truncates oversized messages, limits context messages, or injects page context into the LLM request.

Required repair:

- Use a fake LLM provider that records received messages.
- Call the handler/service with valid and invalid roles.
- Assert oversized messages are truncated or rejected according to the contract.
- Assert only the configured number of context messages reaches the provider.
- Assert page context changes the stored conversation `context_type` and the outbound system prompt.

### Finding TST-016 - Medium - Route security tests are brittle string checks

File: `backend/internal/handler/routes_security_test.go:14`

The route tests assert an exact source line for middleware composition. They can fail after harmless formatting changes and can pass if the string remains in a comment or unused block. They also do not verify runtime middleware behavior.

Required repair:

- Prefer route behavior tests: call protected routes repeatedly and assert rate-limit responses.
- For credential routes, assert register/login are rate-limited while non-credential auth routes keep their intended behavior.
- If a route-table structural test remains, parse registered routes from Gin instead of matching exact source text.

### Finding TST-017 - High - Content download response schema test validates a local map, not the handler

File: `backend/internal/handler/content_download_test.go:56`

`TestDownloadContent_ResponseSchema` marshals a locally-created map containing `download_url` and `expires_in`. The handler can return a different body and this test will still pass.

Required repair:

- Use a fake OSS signer or service dependency.
- Call `DownloadContent` with a known content item and attachment.
- Assert the actual HTTP response contains `download_url` and `expires_in`.
- Assert ambiguous attachment, attachment mismatch, missing content, low reputation, unverified email, banned author, and banned IP paths separately.

### Finding TST-018 - Medium - Download route tests skip when the route is absent

File: `backend/internal/handler/content_download_test.go:80`
File: `backend/internal/handler/content_download_test.go:95`

Both route tests skip if no download route is found. For a required download endpoint, absence should fail.

Required repair:

- Replace route absence skips with failures.
- Add an integration-style route registration test that builds the actual route group and calls the endpoint without auth.

### Finding TST-019 - Medium - Main release-validation test only checks source order

File: `backend/cmd/server/main_test.go:12`

The test proves `cfg.ValidateRelease()` appears after `config.Load()` in `main.go`. It does not prove release mode exits or refuses unsafe defaults. This is a useful guardrail, but weak evidence for release hardening.

Required repair:

- Keep the source-order guard if it catches accidental deletion.
- Add focused config tests for `ValidateRelease()` rejection cases, if not already present.
- Consider an executable-level test only if the project can run `main` safely with injected config and without binding shared ports.

### Finding TST-020 - Medium - Search repository mixes strong behavior tests with weak source guards

File: `backend/internal/repository/search_repo_test.go:235`
File: `backend/internal/repository/search_repo_test.go:341`

This file has useful SQLite-backed behavior tests for visibility and search. The weaker parts are source or SQL-file scans, such as checking `search_repo.go` does not contain a particular `tags.status` filter and checking migration text contains required fragments. These are reasonable as temporary regression guards, but they are not proof that the runtime query works against the current schema.

Required repair:

- Keep the behavior tests as the primary evidence.
- Replace the `tags.status` source scan with a runtime suggestions test that includes tag rows and fails if the query references a missing column.
- For migration checks, prefer applying migrations to an empty and upgrade database in release validation.

## Batch 3 - Frontend Playwright Tests With Mocked APIs

Reviewed:

- `frontend/e2e/helpers/mock-public-apis.ts`
- `frontend/e2e/search-filter-contract.spec.ts`
- `frontend/e2e/studio-publish-fanwork.spec.ts`
- `frontend/e2e/admin-control-semantics.spec.ts`
- `frontend/e2e/ui-layout.spec.ts`

### Finding TST-021 - High - Shared mock helper can make broken public APIs invisible

File: `frontend/e2e/helpers/mock-public-apis.ts:3`

`mockPublicApis()` fulfills core endpoints such as public config, CSRF, refresh, IP lists, content search, content lists, stats, and tags. That is useful for deterministic UI tests, but it means tests using this helper do not verify those backend endpoints or frontend/backend response compatibility.

Required repair:

- Mark these tests as mocked UI tests in names or documentation.
- Add at least one non-mocked smoke suite for public config, search, auth bootstrap, and protected route behavior.
- Add a Playwright fixture that fails unexpected network calls so the mock surface stays intentional.

### Finding TST-022 - Medium - Search filter contract test verifies URL construction only

File: `frontend/e2e/search-filter-contract.spec.ts:8`

The test intercepts `GET /api/v1/contents/search` and returns its own empty `200` response. It proves the frontend sends `content_type=image` and `time_range=week`; it does not prove the backend accepts those parameter names, applies the filters, or returns a compatible schema.

Required repair:

- Rename or document it as a frontend request-shape test.
- Add backend handler/repository tests that assert `content_type` and `time_range` filtering.
- Add one integration/API smoke test against the real backend that seeds mixed content and verifies filtered results.

### Finding TST-023 - High - Fanwork publish test self-fulfills content creation

File: `frontend/e2e/studio-publish-fanwork.spec.ts:77`

The test captures `POST /api/v1/contents`, stores `postDataJSON()`, and fulfills `{ id: 123 }`. This verifies the frontend payload shape, which is useful, but it does not prove the backend accepts `source_original_id`, rejects invalid source originals, validates `zone='fanwork'`, or persists the relation.

Required repair:

- Keep this as a frontend payload test.
- Add backend handler/service tests for fanwork source validation: valid original, missing source, non-original source, unpublished source, and original-zone request carrying `source_original_id`.
- Add a real API smoke test that creates an original, publishes a fanwork, and reads back the relation.

### Finding TST-024 - Medium - Fanwork auth/session state is entirely mocked

File: `frontend/e2e/studio-publish-fanwork.spec.ts:4`

`mockCreatorSession()` returns synthetic refresh and `/auth/me` responses. The test does not exercise protected layout redirects, HttpOnly-cookie refresh behavior, CSRF bootstrap, verified-email restrictions, or low-reputation restrictions.

Required repair:

- Keep this fixture for UI-only publish form tests.
- Add separate protected-route and publish-permission tests against real auth middleware or backend handler tests.
- In UI tests, consider adding negative mocked cases for banned, unverified, and low-reputation users to verify the frontend state gates render correctly.

### Finding TST-025 - Medium - Admin control semantics test mocks all admin data and mutation outcomes

File: `frontend/e2e/admin-control-semantics.spec.ts:4`
File: `frontend/e2e/admin-control-semantics.spec.ts:156`

The admin suite is valuable for checking visual control primitives, but it also captures category create payloads and self-fulfills a successful response. It does not verify admin authorization, category validation, audit logging, or persistence.

Required repair:

- Rename or document this suite as UI semantics coverage.
- Add backend/API tests for category creation, admin-only access, validation failures, and audit rows.
- Add a real browser/admin smoke test only if the environment can provide deterministic admin credentials and isolated data.

### Finding TST-026 - Medium - Mocked admin fixture can hide response schema drift

File: `frontend/e2e/admin-control-semantics.spec.ts:87`
File: `frontend/e2e/admin-control-semantics.spec.ts:95`
File: `frontend/e2e/admin-control-semantics.spec.ts:148`

The test supplies fixed schemas for audit logs, feedback, reports, and categories. If backend response keys drift, these tests still pass because the browser never sees the real responses.

Required repair:

- Add API-level schema tests for admin list/detail endpoints.
- Generate or share DTO fixtures from typed client contracts where practical.
- Keep mocked fixtures minimal and update them from real API examples during release validation.

### Finding TST-027 - Low - UI layout tests are valid UI coverage but should not count as functional E2E

File: `frontend/e2e/ui-layout.spec.ts:5`
File: `frontend/e2e/ui-layout.spec.ts:40`

These tests make useful assertions about mobile layout, sanitized frontend error rendering, and password reveal controls. The mocked API failure test is a good UI sanitization check, but it does not prove backend errors are sanitized at the source.

Required repair:

- Keep these tests in the UI test bucket.
- Pair the raw-error rendering test with backend safe-error tests and one real API error-path smoke test.
- Avoid reporting this suite as proof of cross-stack release behavior.

## Batch 4 - Test Data, Browser Smoke, And Script Safety Helpers

Reviewed:

- `scripts/browser_test.py`
- `scripts/seed-test-data.sh`
- `scripts/seed_data.py`
- `scripts/seed_extra.py`
- `scripts/beta/tests/Test-BetaScripts.ps1`
- Representative backend behavior tests using fakes and in-memory stores

Syntax note:

- `scripts/browser_test.py`, `scripts/seed_data.py`, and `scripts/seed_extra.py` parse as valid Python via `ast.parse`.

### Finding TST-028 - Critical - Shell seed script uses a hard-coded bearer token and does not fail on API errors

File: `scripts/seed-test-data.sh:6`
File: `scripts/seed-test-data.sh:13`

The script embeds a bearer token and then runs many `curl -s` calls without checking HTTP status, response error codes, or whether extracted IDs are non-empty. A failed create call can still let the script continue with blank IDs, producing misleading "Created ..." output.

Required repair:

- Remove hard-coded JWTs; login during setup or accept a token through an environment variable.
- Use `set -euo pipefail`.
- Capture HTTP status separately from response body.
- Fail immediately when an API call is not the expected `2xx`.
- Validate every extracted ID before using it.
- Prefer JSON parsing with a real parser instead of `grep -o '"id":[0-9]*'`.

### Finding TST-029 - High - Python browser smoke script never exits non-zero on collected browser errors

File: `scripts/browser_test.py:16`
File: `scripts/browser_test.py:46`

The script collects console warnings, page errors, and navigation errors in `errors`, but after printing them it does not call `sys.exit(1)`. A CI or release wrapper can treat the script as successful even when every page produced errors.

Required repair:

- Exit with non-zero status when `errors` is non-empty.
- Add assertions for expected page landmarks per route.
- Treat missing content as failure when testing seeded pages.
- Keep screenshots as artifacts, not as the only evidence.

### Finding TST-030 - High - Python browser smoke script relies on hard-coded mutable IDs

File: `scripts/browser_test.py:8`

The page list includes fixed IDs such as `/ip/27`, `/original/63`, and `/content/77`. These IDs depend on a specific database state and are not tied to a deterministic fixture created during the run.

Required repair:

- Create or discover fixture records during setup.
- Build URLs from returned IDs.
- Fail setup if fixtures cannot be created or discovered.
- Avoid relying on shared developer databases for release evidence.

### Finding TST-031 - High - Python seed scripts continue after failed helper calls

File: `scripts/seed_extra.py:16`
File: `scripts/seed_extra.py:22`

`api()` returns `None` on any exception, but the login path immediately indexes `r["tokens"]["access_token"]`. Later fixture creation prints `FAILED` for an item but does not produce a failing process exit. This makes the scripts awkward as test setup: failures are partly fatal, partly informational.

Required repair:

- Make setup helpers consistently fail fast when required data cannot be created.
- Return typed success/failure results for optional fixtures.
- Exit non-zero when any required fixture fails.
- Print a final machine-readable summary with created IDs.

### Finding TST-032 - Medium - Seed scripts are not safe to rerun deterministically

File: `scripts/seed_data.py:35`
File: `scripts/seed_extra.py:31`

The seed scripts create data by display names against a live local API. They do not consistently clean up prior data, use unique run IDs, or verify whether existing data matches the intended fixture contract. Re-running can create duplicates or bind later tests to old data.

Required repair:

- Use a unique test-run prefix or idempotent fixture keys.
- Check for existing records by stable slug/key and validate fields before reuse.
- Provide cleanup or isolated database guidance.
- Keep fixture IDs in a generated summary consumed by browser/API tests.

### Finding TST-033 - Medium - Beta script tests mostly use text guards, but include useful parser and temp-fixture checks

File: `scripts/beta/tests/Test-BetaScripts.ps1:61`
File: `scripts/beta/tests/Test-BetaScripts.ps1:68`
File: `scripts/beta/tests/Test-BetaScripts.ps1:125`

`Test-BetaScripts.ps1` has stronger checks than many source-scanning tests: it parses PowerShell scripts and exercises `Test-RoadmapTaskChecked` with a temporary fixture. Several safety checks are still regex-based source guards, so they should be treated as safety lint rather than proof that the scripts behave correctly in all execution paths.

Required repair:

- Keep the parser and temp-fixture checks.
- Add execution tests for dry-run bootstrap/run/integrate paths where possible.
- For destructive-operation guards, keep the regex checks but label them as static safety lint.

### Finding TST-034 - Low - Backend behavior tests are generally stronger where they use in-memory DB/Redis and fake providers

Files sampled:

- `backend/internal/service/verification_service_test.go`
- `backend/internal/handler/auth_test.go`
- `backend/internal/service/agent_service_test.go`
- `backend/internal/repository/search_repo_test.go`
- `backend/internal/handler/public_config_test.go`

The stronger backend tests call real service or handler code with SQLite, miniredis, or fake providers and assert state changes, response codes, and side effects. These fakes are mostly appropriate because they replace external providers while preserving the code path under test.

Residual risk:

- Some tests still assert error body substrings rather than decoding `{code, message}`.
- Some schema tests use in-memory SQLite patches that can drift from PostgreSQL migrations.
- Release validation should still include migration application on PostgreSQL and a small set of real HTTP smoke tests.

## Batch 5 - Backend Package Tests And Tauri Client Tests

Reviewed:

- `backend/internal/handler/admin_audit_transaction_test.go`
- `backend/internal/handler/feedback_test.go`
- `backend/internal/handler/auth_cookie_test.go`
- `backend/internal/middleware/auth_test.go`
- `backend/internal/middleware/cors_test.go`
- `backend/internal/middleware/interaction_test.go`
- `backend/internal/pkg/aliyun/oss_test.go`
- `backend/internal/pkg/captcha/verifier_test.go`
- `backend/internal/pkg/captcha/ticket_store_test.go`
- `backend/internal/pkg/llm/provider_test.go`
- `backend/internal/pkg/mail/smtp_test.go`
- `backend/internal/pkg/queue/queue_test.go`
- `backend/internal/pkg/response/safe_error_test.go`
- `tauri-client/src-tauri/src/commands/security.rs`
- `tauri-client/src-tauri/src/commands/file_ops.rs`
- `tauri-client/src-tauri/src/commands/env_detect.rs`
- `tauri-client/src-tauri/src/url_scheme.rs`

Positive evidence:

- `admin_audit_transaction_test.go`, `auth_cookie_test.go`, `middleware/auth_test.go`, and `middleware/interaction_test.go` exercise real handlers/middleware with SQLite or miniredis and assert state, status codes, and security side effects.
- `captcha/ticket_store_test.go`, `mail/smtp_test.go`, and `aliyun/oss_test.go` are appropriately narrow unit tests around single-use tickets, SMTP transport selection, and JSON redaction.

### Finding TST-035 - High - Captcha verifier tests legitimize over-broad success values

File: `backend/internal/pkg/captcha/verifier_test.go:125`
File: `backend/internal/pkg/captcha/verifier.go:184`
File: `backend/internal/pkg/captcha/verifier.go:193`

`TestAliyunV2VerifierAcceptsSuccessResultResponse` fabricates a successful provider response with `Code: "Success"` and `Result.VerifyResult: true`. The implementation also accepts empty code, `"200"`, `"success"`, string `"true"`, string `"pass"`, and numeric `1` as successful values. The local project docs confirm Aliyun CAPTCHA 2.0 and fail-closed requirements, but do not document these alternate success shapes.

This is exactly the class of test that can turn contract drift into a passing suite: the test creates the permissive fixture and then proves the permissive parser accepts it.

Required repair:

- Anchor accepted Aliyun response shapes to official documentation or recorded provider fixtures.
- Add negative tests proving `"200"`, empty code, `"pass"`, string `"true"`, and numeric `1` are rejected unless the provider contract explicitly allows them.
- Decode provider fixtures into typed structs instead of permissive `map[string]interface{}` where possible.
- Mark compatibility fixtures with a source link or fixture provenance.

### Finding TST-036 - Medium - LLM provider tests only test the local mock provider

File: `backend/internal/pkg/llm/provider_test.go:11`
File: `backend/internal/pkg/llm/provider_test.go:41`

The tests define `mockProvider` inside the test file and then verify that the mock returns the fields it was initialized with. Apart from the compile-time interface assertion, this does not exercise the production provider factory, request mapping, error normalization, streaming parser, or embedding path.

Required repair:

- Keep the interface compile check, but do not count these tests as runtime LLM provider coverage.
- Add tests around production provider construction and request serialization using an `httptest.Server`.
- Verify error status mapping, malformed JSON, stream chunk parsing, and embedding shape validation.
- Reserve hand-written mocks for services that consume `LLMProvider`, not for proving the provider package itself works.

### Finding TST-037 - Medium - Queue retry test does not prove DLQ behavior

File: `backend/internal/pkg/queue/queue_test.go:87`
File: `backend/internal/pkg/queue/queue_test.go:90`
File: `backend/internal/pkg/queue/queue_test.go:92`

`TestRetryAndDLQ` ignores `Subscribe` and `Publish` errors, sleeps for two seconds, then only asserts that the handler was attempted at least twice. Despite the test name, it does not inspect the DLQ stream, confirm the exhausted message payload, or prove the retry count reached the configured terminal behavior.

Required repair:

- Require `Subscribe` and `Publish` to return nil.
- Replace fixed sleep with a bounded polling helper.
- Assert the DLQ stream contains the failed message and expected metadata after retries are exhausted.
- Add a complementary success-after-retry test that proves the message is not sent to DLQ.

### Finding TST-038 - High - Tauri script-signature tests validate the legacy HMAC path, not the Beta security contract

File: `tauri-client/src-tauri/src/commands/security.rs:7`
File: `tauri-client/src-tauri/src/commands/security.rs:31`
File: `tauri-client/src-tauri/src/commands/security.rs:59`

The current Tauri security tests sign and verify scripts with `AGENT_HMAC_SECRET`, then assert valid, invalid, and tampered HMAC behavior. The project guidance says the legacy Go-issued HMAC action script must be replaced by Ed25519 during the desktop security Beta path, with the client holding only a public key. The tests therefore risk preserving the old trust model as "green" evidence.

Required repair:

- Once the desktop security task is active, replace HMAC tests with Ed25519 fixture verification tests.
- Use a checked-in canonical JSON fixture shared with the Go signer.
- Assert unknown fields, unknown actions, expired scripts, path escape attempts, and tampered bytes are rejected.
- Keep a temporary HMAC test only if it is explicitly labeled as legacy coverage and not accepted as Beta release evidence.

### Finding TST-039 - Medium - Tauri file-operation tests cover happy paths but miss key attack paths

File: `tauri-client/src-tauri/src/commands/file_ops.rs:343`
File: `tauri-client/src-tauri/src/commands/file_ops.rs:372`
File: `tauri-client/src-tauri/src/commands/file_ops.rs:415`

The file operation tests check whitelist allow/reject, create, move, read extension rejection, and write backup. They do not cover zip-slip extraction, download failures, destination overwrite backup correctness, read/write outside whitelist on relative or symlinked paths, direct `backup_file` command denial, or all seven allowed operation boundaries from the agent guide.

Required repair:

- Add table tests for every allowed operation and representative rejected operation.
- Test archive entries such as `../escape.txt` and absolute paths.
- Test symlink/path-normalization attempts where the platform supports them.
- Use unique temp directories per test instead of a shared `temp/omnicraft/test` path.
- Assert returned `FileOpResult` fields and resulting file contents, not only `is_ok()`.

### Finding TST-040 - Low - Tauri environment detection test is only a smoke test

File: `tauri-client/src-tauri/src/commands/env_detect.rs:105`

`test_detect_environment_ok` only asserts that `detect_environment()` returns successfully and that `platform` equals `std::env::consts::OS`. It does not control HOME/APPDATA/TEMP fixtures or verify any game/runtime detection logic.

Required repair:

- Keep this as a smoke test.
- Add deterministic environment-variable fixtures for each supported platform branch.
- Verify detected paths and absence behavior without depending on the developer machine.

## Batch 6 - Backend Service/Model/Config Tests And Frontend Node Tests

Reviewed:

- `backend/config/config_test.go`
- `backend/internal/model/content_migration_test.go`
- `backend/internal/service/admin_audit_service_test.go`
- `backend/internal/service/agent_service_test.go`
- `backend/internal/service/content_source_test.go`
- `backend/internal/service/feedback_service_test.go`
- `backend/internal/service/reputation_gate_test.go`
- `backend/internal/service/review_remediation_test.go`
- `backend/internal/service/verification_service_test.go`
- `frontend/lib/api.csrf.test.cjs`
- `frontend/tests/sheet-music-viewer-download.test.mjs`
- `frontend/tests/register-captcha.test.mjs`
- `frontend/tests/captcha-ticket-reset.test.mjs`
- `frontend/tests/feedback-form-upload.test.mjs`
- `frontend/components/verification/CaptchaWidget.test.cjs`

Positive evidence:

- `verification_service_test.go` is credible behavior coverage: it uses SQLite, miniredis, a fake mail boundary, single-use token checks, cache invalidation checks, cooldown checks, and concurrent double-submit tests.
- `feedback_service_test.go` is credible service coverage for presign grants, grant single-use behavior, grant mismatch, notification side effects, and anonymous contact email delivery errors.
- `frontend/lib/api.csrf.test.cjs` executes the compiled `api.ts` module with a fake `fetch`, so it is stronger than pure source scanning for CSRF bootstrap/retry flow.

### Finding TST-041 - Medium - Migration and default-config tests only scan file text

File: `backend/config/config_test.go:31`
File: `backend/internal/model/content_migration_test.go:12`
File: `backend/internal/service/review_remediation_test.go:78`

These tests read YAML or SQL files and search for substrings such as `creator_support_enabled: false`, `source_original_id`, `description`, `started_at`, and `drop not null`. This can catch accidental deletion of text, but it does not prove the config loader maps the value correctly, the migration executes, the resulting schema matches the model, or the migration is safe on PostgreSQL.

Required repair:

- For config defaults, load the config through the production loader or a parser and assert the typed field value.
- For migrations, execute the migration against a real PostgreSQL test database for release evidence, or at least a migration runner fixture for local checks.
- After applying migrations, inspect resulting columns, nullability, indexes, and constraints.
- Keep text guards only as static lint, not as schema validation.

### Finding TST-042 - Medium - Agent visibility test patches schema by hand

File: `backend/internal/service/agent_service_test.go:153`
File: `backend/internal/service/agent_service_test.go:156`

`setupAgentVisibilityDB` runs `AutoMigrate` and then manually executes `ALTER TABLE users ADD COLUMN deleted_at` and `ALTER TABLE content_items ADD COLUMN deleted_at`. The behavior assertion is useful, but hand-patching the test schema can mask model/migration drift: the test can pass even if the real migration set is missing, incompatible, or different from production PostgreSQL.

Required repair:

- Build the test database with the same migration path used by the app or release validation.
- If SQLite remains necessary, isolate SQLite compatibility patches in a clearly named helper and add a PostgreSQL migration smoke test elsewhere.
- Add an assertion that the production model/migration source owns the soft-delete columns instead of relying on test-only DDL.

### Finding TST-043 - Medium - Sheet music download test checks JSX text, not rendered download behavior

File: `frontend/tests/sheet-music-viewer-download.test.mjs:5`
File: `frontend/tests/sheet-music-viewer-download.test.mjs:11`
File: `frontend/tests/sheet-music-viewer-download.test.mjs:34`

The test searches `SheetMusicViewer.tsx` for regex snippets proving that certain JSX fragments appear in a particular shape. It does not render the component with previewable and non-previewable attachments, click the download CTA, or assert that the authorized backend download endpoint is called instead of a direct OSS URL.

Required repair:

- Render `SheetMusicViewer` with attachments containing only `oss_key`, preview attachments with `oss_url`, and mixed attachments.
- Assert visible download CTAs for each case.
- Click the CTA and verify the call goes through the authorized download API path.
- Keep the direct-OSS-anchor negative check only as supplemental static lint.

### Finding TST-044 - Low - CSRF API test does not cover browser cookie integration

File: `frontend/lib/api.csrf.test.cjs:21`
File: `frontend/lib/api.csrf.test.cjs:43`
File: `frontend/lib/api.csrf.test.cjs:78`

`api.csrf.test.cjs` is stronger than most frontend source tests because it executes the compiled `api.ts` module and validates fetch ordering and headers. Its boundary is still a VM with a fake `fetch` and no real browser `document.cookie`, so it does not prove cookie parsing, credentialed browser behavior, or server-set CSRF cookie interaction.

Required repair:

- Keep the current unit test for deterministic request-order coverage.
- Add a browser-level or jsdom-style test for `document.cookie` parsing and CSRF header selection.
- Add one integration smoke path against the backend CSRF endpoint when release evidence is required.

## Batch 7 - Test Entry Points, Seed Assets, And Remaining Support Files

Reviewed:

- `scripts/seed_last.py`
- `backend/testdata/search_seed.sql`
- `backend/cmd/seed_admin/main.go`
- `backend/cmd/version_seed/main.go`
- `frontend/playwright.config.ts`
- `frontend/package.json`
- `tauri-client/package.json`
- `tauri-client/src-tauri/Cargo.toml`
- Remaining already-covered handler helpers: `backend/internal/handler/agent_test.go`, `backend/internal/handler/content_query_test.go`

Scope note:

- `design/ui-spec.md`, historical review docs, screenshots, and roadmap/spec files were excluded as source-of-truth or artifact files, not executable test scripts.
- `backend/testdata/search_seed.sql` is a fixture, not a test, but it affects search-test credibility and was reviewed as test support.
- `backend/migrations/026_rehab_seed.sql` is a product seed migration, not a test script or test fixture, and was excluded from this review.

### Finding TST-045 - High - Seed utilities hard-code credentials and mutate local databases

File: `scripts/seed_last.py:19`
File: `backend/cmd/seed_admin/main.go:14`
File: `backend/cmd/seed_admin/main.go:20`
File: `backend/cmd/version_seed/main.go:23`

`seed_last.py` logs in with a fixed demo account and assumes `r["tokens"]["access_token"]` exists. The Go seed commands embed the local PostgreSQL DSN and, in `seed_admin`, a fixed admin password. These are dangerous as test support because they mutate shared local state, can silently target the wrong database, and normalize hard-coded credentials in scripts that may be copied into release workflows.

Required repair:

- Require DSN, base URL, credentials, and target account IDs via environment variables or explicit flags.
- Fail fast when login, parsing, or required IDs are missing.
- Add dry-run output and an explicit confirmation guard for destructive/admin mutations.
- Keep seed utilities out of CI unless they run against isolated disposable databases.

### Finding TST-046 - Medium - Search SQL fixture can hide stale fixture drift

File: `backend/testdata/search_seed.sql:1`
File: `backend/testdata/search_seed.sql:12`
File: `backend/testdata/search_seed.sql:25`

The search fixture uses fixed IDs and `ON CONFLICT DO NOTHING`, then applies follow-up updates. If a previous run left rows with the same IDs but different field values, the insert will not repair them and tests can run against stale data. The fixture also contains mojibake text, which makes Chinese search expectations difficult to audit by reading the file.

Required repair:

- Prefer transactional fixture setup that deletes or upserts exact expected rows.
- Use unique test-run IDs or stable slugs with full field updates.
- Store fixture text in a valid, auditable encoding.
- Add a fixture validation query that fails if expected rows do not match the intended contract.

### Finding TST-047 - Medium - Playwright config only starts the frontend server

File: `frontend/playwright.config.ts:8`
File: `frontend/playwright.config.ts:14`
File: `frontend/package.json:11`

`npm run test:e2e` starts only `npm run dev` for the Next.js frontend. It does not start the Go API, PostgreSQL, Redis, migrations, or seed setup. This is acceptable for the current mocked Playwright specs, but it means `test:e2e` cannot be used as cross-stack release evidence unless the backend stack is started by an outer script and the tests avoid route fulfillment.

Required repair:

- Label the current Playwright suite as frontend-mock E2E.
- Add a separate command for true cross-stack E2E that starts or verifies backend dependencies.
- Fail early if the backend health endpoint is missing for cross-stack mode.
- Keep mocked and real-backend Playwright projects separate in config.

## Batch 8 - Abuse Controls And Upload Grant Tests

Reviewed:

- `frontend/tests/error-handler-i18n.test.mjs`
- `frontend/tests/content-upload-grant.test.mjs`
- `backend/internal/middleware/ratelimit_test.go`
- `backend/internal/middleware/body_limit_test.go`
- `backend/internal/handler/search_abuse_test.go`
- `backend/internal/handler/content_upload_grant_test.go`
- `backend/internal/service/content_upload_grant_test.go`
- `backend/internal/service/upload_grant_service_test.go`

Positive evidence:

- `ratelimit_test.go` uses miniredis and real Gin middleware to verify credential-key rate limiting across different source IPs.
- `content_upload_grant_test.go` and `upload_grant_service_test.go` exercise real upload grant issue/consume paths, wrong-user/wrong-purpose rejection, one-time consumption, object-verifier errors, and entropy failure behavior.
- `body_limit_test.go` and `search_abuse_test.go` cover focused abuse-control helpers and middleware with concrete HTTP status expectations.

### Finding TST-048 - Medium - New frontend upload/i18n tests are source scans

File: `frontend/tests/error-handler-i18n.test.mjs:5`
File: `frontend/tests/content-upload-grant.test.mjs:5`
File: `frontend/tests/content-upload-grant.test.mjs:18`

The error-handler i18n test reads `error-handler.ts` and checks for a translation key string. The content upload grant test reads `FileUploader.tsx` and `PublishForm.tsx` and checks for field names such as `grantId` and `grant_id: f.grantId`. These tests are useful static guards, but they do not execute the error handler, render the uploader, upload a file, or assert the actual publish request payload.

Required repair:

- Execute the error handler with a fake translator and a 429 `ApiRequestError`, then assert the translated toast key is used.
- Render or component-test `FileUploader` and `PublishForm` with a fake presign response containing `grant_id`.
- Submit the form and inspect the actual request body sent to `/api/v1/contents`.
- Keep source scans only as supplemental lint for preventing obvious regressions.

## Coverage Summary

Reviewed test assets by category:

- Backend Go tests: handler, middleware, service, repository, model, pkg, config, and server tests were inspected.
- Frontend tests: Node source tests, CSRF module test, and Playwright specs/helpers were inspected.
- Tauri tests: Rust unit tests in `src-tauri/src` plus package/Cargo test entry points were inspected.
- Scripted E2E/setup: root Python E2E scripts, browser smoke, seed scripts, beta PowerShell tests, and remaining seed utilities were inspected.
- Test support fixtures: search SQL fixture and major test command entry points were inspected.

Highest-risk areas:

- Root E2E and seed scripts can pass or continue after failed real API behavior.
- Frontend and several backend tests rely on source scanning rather than runtime behavior.
- Mocked Playwright tests are useful UI checks but are not backend contract evidence.
- Captcha/provider tests currently accept broad compatibility responses without documented provider-contract proof.
- Desktop/Tauri security tests still include legacy HMAC evidence and need Ed25519/path-boundary coverage before release.

## Recommended Test-Script Review Plan

Use this plan when reviewing future test scripts or when repairing the findings above.

### 1. Inventory and classify every test

Classify each file as one of:

- `credible behavior`: calls production handler/service/module code and asserts observable state, response, or side effect.
- `mock-only`: verifies local mocks or route-fulfilled browser fixtures; useful for UI state, not backend contract evidence.
- `static guard`: scans source text; useful as lint, not runtime proof.
- `smoke/artifact`: loads pages or saves screenshots; useful only with explicit assertions and non-zero failure exit.
- `fixture/setup`: prepares data; must fail fast and emit deterministic IDs.

### 2. Check contract strictness

For each test that validates an API, provider, or CLI contract:

- Assert exact success values and exact error envelope codes.
- Reject broad truthiness such as `true`, `"true"`, `"1"`, `1`, `"ok"`, empty code, or substring matching unless the contract explicitly allows them.
- Decode JSON into typed response structs where possible.
- Require fixture provenance for external provider response shapes.
- Add negative tests for near-miss values.

### 3. Check whether the test self-fulfills the result

Flag tests that:

- Return API responses from the test itself with `route.fulfill`, local maps, or inline fake handlers.
- Write directly to the database to bypass a failing public API.
- Assert generated screenshots exist instead of asserting page behavior.
- Test a hand-written mock instead of the production adapter.

Mocks are acceptable only at external boundaries, and the production code path under review must still be executed.

### 4. Check failure semantics

Every security, auth, upload, payment, captcha, email, OSS, queue, or desktop-file test should include:

- Positive path.
- Expected failure path.
- Provider unavailable path.
- Malformed response/input path.
- Assertion that the process or script exits non-zero when used as CI/release evidence.

Ban `t.Skip` for absent required behavior. If a feature is absent, the test should fail or be explicitly scoped out in the plan.

### 5. Check fixture integrity

Fixture and seed scripts must:

- Avoid hard-coded bearer tokens and mutable global IDs.
- Create or discover fixtures during setup and pass IDs forward.
- Fail immediately on unexpected HTTP status or missing IDs.
- Be safe to rerun with isolated run IDs or cleanup.
- Produce a machine-readable summary consumed by browser/API tests.

### 6. Check CI wiring and release-evidence labels

For each suite, record:

- Which command runs it locally.
- Whether CI/release scripts run it.
- Whether it requires external services.
- Whether it uses mocked APIs.
- Whether it is allowed to count as release evidence.

Do not count source scans, mocked Playwright route fixtures, or screenshot-only smoke tests as cross-stack release validation.

### 7. Recommended remediation order

1. Repair critical false positives: unconditional pass, broad accepted statuses, direct DB bypass, hard-coded tokens, and scripts that exit 0 on errors.
2. Replace source scans around auth/captcha/download/agent security with runtime handler or component tests.
3. Add strict provider contract fixtures for captcha, OSS, SMTP, and LLM adapters.
4. Split Playwright tests into UI-mock tests and true cross-stack E2E tests with real backend calls.
5. Harden seed/browser scripts so they are deterministic, fail-fast, and CI-wired.
6. Expand Tauri security/file-operation tests before treating desktop deploy as releasable.

Current finding count after this pass:

- Critical: 6
- High: 17
- Medium: 21
- Low: 4
