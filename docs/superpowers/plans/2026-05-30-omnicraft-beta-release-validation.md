# OmniCraft Beta Release Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove the Web Beta and optional desktop deploy track meet their frozen release gates with reproducible evidence.

**Architecture:** Treat release validation as a separate deliverable. Run clean-database migrations, static checks, API negative-path checks, Playwright journeys and secret-leak scans. The Web Beta may ship with desktop deploy disabled; desktop advertising requires the separate R-02 gate.

**Tech Stack:** Docker Compose, Go, PostgreSQL, Redis, Next.js, MCP Playwright, Tauri/Rust.

---

## File Structure

- Create: `docs/review/beta-release-validation-2026-05-30.md`.
- Create: `docs/review/desktop-deploy-validation-2026-05-30.md` only when desktop deploy is enabled.
- Add screenshots under `screenshots/beta-release-*`.
- Update: `progress.txt`.
- Update checkboxes in this plan and `docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md`.

## Task R-01: Validate Web Beta

**Files:**
- Create: `docs/review/beta-release-validation-2026-05-30.md`
- Add: `screenshots/beta-release-*`

- [x] **Step 1: Confirm prerequisite task status**

Verify roadmap tasks F-01 through F-06, V-01 through V-06, A-01 through A-05, G-01 through G-05 and D-01 are checked. Desktop tasks D-02 through D-05 may remain unchecked only if `desktop_deploy_enabled=false`.

- [x] **Step 2: Initialize dependencies**

```powershell
docker compose up -d postgres redis
cd backend
go mod tidy
cd ..\frontend
npm install
```

Expected: PostgreSQL and Redis become healthy. Stop and report blocker if required services do not start.

- [x] **Step 3: Validate migrations from an empty database**

Run every `backend/migrations/*.sql` in lexical order against a disposable empty PostgreSQL database. Then create a second disposable upgrade database by applying migrations `001` through `048` in lexical order, seed representative pre-Beta rows, and apply `049` through `052`. Do not depend on an undeclared external snapshot. Note: physical file creation history is irrelevant; the runner must sort migration filenames lexically and record which files were applied.

Expected: both empty-database and existing-database upgrade paths succeed, including `pgvector`, existing `047` trigram indexes, new content-tag fallback index, verification, feedback and audit additions.

- [x] **Step 4: Run engineering gates**

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
cd ..\frontend
npm run lint
npm run build
cd ..
docker compose config
```

Expected: all commands exit `0`.

- [x] **Step 5: Run secret-leak scans**

```powershell
rg -n "err\\.Error\\(\\)|refresh_token|AGENT_HMAC_SECRET|private_key|access_key_secret|api_key|token.*console|console\\.error" backend frontend tauri-client
```

Review every match. Expected:

- No raw errors returned to clients.
- No refresh token in frontend storage or JSON API responses.
- No Ed25519 private key in client code.
- No raw token, secret, grant or cookie in logs.
- No permanent OSS download URL exposed as a user download CTA.

- [x] **Step 6: API-test authorization matrix**

Verify normal, anonymous, unverified-email, low-reputation, banned-user and dependency-failure paths for:

- Publish.
- Comment.
- Reaction/favorite.
- Judge vote.
- Download.
- Agent chat/search.
- Admin routes.
- Removed `/api/v1/agent/script/:id` prototype.
- Disabled `POST /api/v1/deploy-grants`.

Expected: protected operations reject safely with a stable error envelope.

- [x] **Step 7: API-test verification and feedback**

Verify:

- Registration records terms and sends mail without returning raw token.
- Verification links are single use.
- Resend has captcha and cooldown.
- Forgot-password response is uniform.
- Reset token is single use.
- Anonymous feedback requires captcha.
- Feedback screenshot presign requires the feedback-specific purpose flow and rejects content-upload keys.
- Logged-in user can inspect own feedback.
- Admin can reply, add an internal note and close; users never see internal notes.
- Logged-in users receive site notifications and anonymous users receive email for reply/close.
- Existing content-review, appeal-result and report-resolution notifications still reach the affected logged-in user.

- [x] **Step 8: Browser-test public journeys**

Use MCP Playwright:

1. Footer to `/help`, `/privacy`, `/terms`, `/feedback`, `/client`.
2. Register, verify email, login, logout.
3. Pending verification page, resend cooldown and password reset.
4. Chinese keyword search for a no-space title. Seed test data first:

```powershell
psql $env:DB_DSN -f backend/testdata/search_seed.sql
```

Use the deterministic `backend/testdata/search_seed.sql` created in F-05. It includes 5-10 published content items containing Chinese titles and tags plus at least one deleted and one banned-IP item to verify visibility filtering.
5. Content detail download success and denied download.
6. Agent off state and Agent failure downgrade.
7. Publish assistance with explicit apply.

Save screenshots under `screenshots/beta-release-public-*`.

- [x] **Step 9: Browser-test admin journeys**

Use MCP Playwright:

1. Normal user denied from `/admin`.
2. Admin dashboard.
3. Resolve report.
4. Reply and close feedback.
5. Inspect queue read-only page.
6. Inspect audit record.
7. Confirm config page shows configured/masked secrets only.

Save screenshots under `screenshots/beta-release-admin-*`.

- [x] **Step 10: Validate production configuration**

These checks apply only to the production deployment environment. Local development using HTTP localhost is acceptable and does not need to satisfy these requirements.

Confirm:

- HTTPS certificate exists in the production environment.
- Production `security.allowed_origins` has explicit origins and no wildcard.
- SMTP/captcha/OSS/PostgreSQL/Redis settings are present.
- Web and API use the approved HTTPS subdomains under `leeppp.online`; credentialed CORS permits only the exact Web origin and unsafe API requests reject other origins.
- `client.download_enabled=false`; Web Beta does not expose a client download URL or version.
- `desktop_deploy_enabled=false` unless R-02 is complete.
- Direct calls to the removed legacy `/api/v1/agent/script/:id` route return `404`.
- `POST /api/v1/deploy-grants` returns `503 FEATURE_DISABLED` while desktop deploy is off.
- `payment_enabled=false`.
- `creator_support_enabled=false` unless separately approved.

- [x] **Step 11: Write release report**

Record SHA, commands, API cases, screenshot paths, configuration confirmations, residual risks and explicit release decision.

- [ ] **Step 12: Commit**

```powershell
git add docs/review/beta-release-validation-2026-05-30.md screenshots docs/superpowers/plans progress.txt
git commit -m "Beta R-01: validate public web beta release - completed"
```

## Task R-02: Validate Desktop Deploy Before Advertising It

**Files:**
- Create: `docs/review/desktop-deploy-validation-2026-05-30.md`

- [ ] **Step 1: Confirm desktop prerequisites**

Require D-01 through D-05 checked, real Ed25519 private key provisioned server-side, public key embedded in client, HTTPS API base URL configured, and signed client distribution path defined.

- [ ] **Step 2: Run desktop gates**

```powershell
cd tauri-client
npm run build
cargo test --manifest-path src-tauri/Cargo.toml
cargo clippy --manifest-path src-tauri/Cargo.toml -- -D warnings
npm run tauri build
```

- [ ] **Step 3: Run desktop security matrix**

Verify:

- Valid grant works once.
- Replayed and expired grants fail.
- Deep link contains grant only.
- Grant exchange succeeds without a Web JWT, consumes the grant atomically and rejects replay.
- Signed `script_json` bytes verify before strict deserialization; the Go/Rust fixture matches.
- Tampered, expired and unknown-action scripts fail before any write.
- Only Ed25519 public key ships in client.
- New directories, new files, move targets and archive entries stay within allowlist.
- Zip-slip fails.
- Non-HTTPS and non-allowlisted download hosts fail.
- WebView has no direct filesystem write capability.
- WebView cannot invoke raw file-operation commands; only the verified-script executor is public.
- Each action is shown before execution.
- First failed action stops subsequent actions.

- [ ] **Step 4: Write release report**

Record build artifact, SHA, configured API base URL, public-key ID, test results and decision. Only after approval may `desktop_deploy_enabled` become true.

- [ ] **Step 5: Commit**

```powershell
git add docs/review/desktop-deploy-validation-2026-05-30.md docs/superpowers/plans progress.txt
git commit -m "Beta R-02: validate secure desktop deploy release - completed"
```
