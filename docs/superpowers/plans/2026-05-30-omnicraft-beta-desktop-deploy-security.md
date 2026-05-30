# OmniCraft Desktop Deploy Security Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the unsafe desktop one-click deployment prototype with a short-lived grant and Ed25519-verified execution pipeline.

**Architecture:** First remove the unsafe prototype route for every Web Beta release. Optionally, Web requests an opaque, single-use deploy grant. Tauri exchanges the grant without a Web JWT, receives exact canonical Ed25519-signed script bytes, verifies signature before strict deserialization using an embedded public key, displays actions to the user, and executes only audited Rust commands against logical paths rooted inside fixed allowlisted directories. Keep `features.desktop_deploy_enabled=false` until D-05 passes end to end.

**Tech Stack:** Go/Gin/Redis/OSS presigned URLs, Ed25519, Tauri 2, Rust, React/Vite.

---

## File Structure

### Backend

- Modify: `backend/config/config.go`, `backend/config.yaml`, `.env.example`.
- Create: `backend/internal/service/deploy_grant_service.go`.
- Create: `backend/internal/handler/deploy_grant.go`.
- Modify: `backend/internal/handler/routes.go`.
- Modify: `backend/internal/service/agent_service.go`.
- Add tests in `backend/internal/service` and `backend/internal/handler`.

### Web

- Modify: `frontend/components/content/ContentDetail.tsx`.
- Modify: `frontend/messages/zh.json`, `frontend/messages/en.json`.

### Tauri

- Modify: `tauri-client/src/App.tsx`.
- Modify: `tauri-client/src-tauri/src/url_scheme.rs`.
- Replace: `tauri-client/src-tauri/src/commands/security.rs`.
- Modify: `tauri-client/src-tauri/src/commands/file_ops.rs`.
- Modify: `tauri-client/src-tauri/src/lib.rs`.
- Modify: `tauri-client/src-tauri/capabilities/default.json`.
- Modify: `tauri-client/src-tauri/Cargo.toml`.
- Modify: `tauri-client/src/App.tsx`.
- Modify: `tauri-client/src-tauri/tauri.conf.json`.
- Create: `tauri-client/.env.example`.

## Task D-01: Disable Unsafe Desktop Entry By Default

**Release classification:** Required for the initial Web Beta even when D-02 through D-05 are deferred.

**Files:**
- Modify: `backend/config/config.go`
- Modify: `backend/config.yaml`
- Modify: `backend/internal/handler/agent.go`
- Modify: `backend/internal/handler/routes.go`
- Create: `backend/internal/handler/agent_deploy_disabled_test.go`
- Modify: `.env.example`

- [ ] **Step 1: Confirm the feature flag defaults off**

```yaml
features:
  desktop_deploy_enabled: false
```

F-04 introduces the flag and public DTO. This task verifies the backend source, environment override behavior and disabled-by-default production value rather than creating a second competing config shape.

- [ ] **Step 2: Confirm prototype UI is hidden**

Before editing UI, run:

```powershell
rg -n "## Component: ContentDetail|## Page: /studio/publish" design/ui-spec.md
```

G-01 owns the Web UI gate. Confirm one-click deploy links and creator enablement toggles are absent while the flag is false. `/client` may remain as an information/manual-download page. Do not duplicate the G-01 component edits in this task.

- [ ] **Step 3: Remove the unsafe backend route**

Remove the `/api/v1/agent/script/:id` registration from `routes.go` and delete its handler function. It is an unsafe prototype, not a supported API. Add a direct route test:

```text
GET /api/v1/agent/script/:id -> 404
```

The future supported deploy API is `/api/v1/deploy-grants`; when that feature is disabled, that endpoint returns `503 FEATURE_DISABLED`. Do not preserve the legacy script route solely to return a feature-flag error.

- [ ] **Step 4: Run checks and browser-test**

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
cd ..\frontend
npm run lint
npm run build
```

Use MCP Playwright and save `screenshots/beta-d01-desktop-hidden.png`.

- [ ] **Step 5: Commit**

```powershell
git add backend .env.example screenshots docs/superpowers/plans progress.txt
git commit -m "Beta D-01: disable unsafe desktop deploy prototype - completed"
```

## Task D-02: Add Short-Lived Single-Use Deploy Grants

**Files:**
- Create: `backend/internal/service/deploy_grant_service.go`
- Create: `backend/internal/service/deploy_grant_service_test.go`
- Create: `backend/internal/handler/deploy_grant.go`
- Create: `backend/internal/handler/deploy_grant_test.go`
- Modify: `backend/internal/handler/routes.go`
- Modify: `backend/internal/container/container.go`
- Modify: `backend/config/config.go`
- Modify: `backend/config.yaml`

- [ ] **Step 1: Write failing service tests**

Cover:

```go
func TestDeployGrantStoresOnlyHash(t *testing.T) {}
func TestDeployGrantExpires(t *testing.T) {}
func TestDeployGrantCanBeConsumedOnce(t *testing.T) {}
func TestDeployGrantRechecksUserAndContentEligibility(t *testing.T) {}
```

- [ ] **Step 2: Define grant service**

```go
type DeployGrantClaims struct {
    UserID    int64
    ContentID int64
    Purpose   string
    IssuedAt  time.Time
}

func (s *DeployGrantService) Issue(ctx context.Context, userID, contentID int64) (string, error)
func (s *DeployGrantService) Consume(ctx context.Context, rawGrant string) (*DeployGrantClaims, error)
```

Redis stores:

```text
deploy:grant:<sha256(grant)>
```

with `deploy.grant_ttl_sec` (default `300`). Use a high-entropy random grant. Redis 7+ is mandatory for OmniCraft, so exchange MUST atomically consume the key with `GETDEL`; raw grants must never appear in logs.

- [ ] **Step 3: Recheck authorization before issue and exchange**

Require:

- Feature flag enabled.
- Existing, non-deleted, non-banned, verified user with sufficient reputation.
- Published content.
- `allow_copy=true`.
- `agent_enabled=true`.
- Every attachment belongs to the content.

- [ ] **Step 4: Mount the issue endpoint**

```text
POST /api/v1/deploy-grants
```

Issue requires authenticated interaction eligibility and returns `503 FEATURE_DISABLED` while `features.desktop_deploy_enabled=false`. Implement and test `Consume`, but do not mount `/deploy-grants/exchange` yet: D-03 owns the signed-script service needed by that response. D-01 has already removed the legacy `/agent/script/:id` route.

- [ ] **Step 5: Run checks**

```powershell
cd backend
go test ./internal/service ./internal/handler -run TestDeployGrant -v
go test ./...
go vet ./...
go build ./...
cd ..\frontend
npm run lint
npm run build
```

- [ ] **Step 6: Commit**

```powershell
git add backend docs/superpowers/plans progress.txt
git commit -m "Beta D-02: add single-use desktop deploy grants - completed"
```

## Task D-03: Replace HMAC With Canonical Ed25519 Scripts

**Files:**
- Modify: `backend/config/config.go`
- Modify: `backend/config.yaml`
- Modify: `.env.example`
- Modify: `backend/internal/model/config_public.go`
- Modify: `backend/internal/handler/admin.go`
- Modify: `backend/internal/handler/deploy_grant.go`
- Modify: `backend/internal/handler/routes.go`
- Modify: `frontend/app/(protected)/admin/config/page.tsx`
- Create: `backend/internal/service/deploy_script.go`
- Create: `backend/internal/service/deploy_script_test.go`
- Modify: `backend/internal/service/agent_service.go`
- Create: `testdata/deploy_script_fixture.json`

- [ ] **Step 1: Define strict script schema**

```go
type DeployScript struct {
    SchemaVersion int            `json:"schema_version"`
    ScriptID      string         `json:"script_id"`
    ContentID     int64          `json:"content_id"`
    UserID        int64          `json:"user_id"`
    IssuedAt      time.Time      `json:"issued_at"`
    ExpiresAt     time.Time      `json:"expires_at"`
    Nonce         string         `json:"nonce"`
    Actions       []DeployAction `json:"actions"`
}
```

Allowed actions: `download_file`, `extract_archive`, `move_file`, `create_dir`, `read_config`, `write_config`. `backup_file` is internal only.

- [ ] **Step 2: Write failing signing tests**

Cover stable canonical JSON, valid signature, tamper rejection, expiry, unknown action rejection, logical path validation and the checked-in Go/Rust-compatible signing fixture at `testdata/deploy_script_fixture.json`.

- [ ] **Step 3: Sign with Ed25519 private key**

Before editing the Admin config UI, run:

```powershell
rg -n "## Page: /admin/config" design/ui-spec.md
rg -n "HMAC|hmac_secret" backend/internal/handler/admin.go
```

The second command identifies existing Admin config fields that reference the old HMAC secret and must be replaced with Ed25519 signing-key status fields.

Load the private key from `deploy.ed25519_private_key_b64` or the `DEPLOY_ED25519_PRIVATE_KEY_B64` environment override. Load the active key ID from `deploy.ed25519_key_id` or `DEPLOY_ED25519_KEY_ID`. Document both overrides in `.env.example`. Return the exact canonical UTF-8 bytes used for signing:

```json
{"script_json":"{\"schema_version\":1,...}","signature":"base64...","key_id":"desktop-2026-01"}
```

Encode the script from a strict struct with deterministic field order. Tauri verifies the returned `script_json` bytes before parsing; it must not reserialize a JSON object and hope field order matches. Never return or log the private key. Remove HMAC use from deploy script generation.

Replace Admin config status fields that refer to the old HMAC secret with masked Ed25519 signing-key configuration status. The Admin page may show configured/not-configured and `key_id`, never key bytes.

Use one active Beta signing key. Add `deploy.ed25519_key_id` and return that configured ID in the signed envelope. The client build embeds the matching public-key ID and public key. Overlapping multi-key rotation is not part of the Beta plan unless maintainers explicitly require it.

- [ ] **Step 4: Mount grant exchange**

Mount:

```text
POST /api/v1/deploy-grants/exchange
```

The exchange endpoint is intentionally callable by Tauri without a Web JWT. It accepts only the opaque grant, remains covered by the existing global request rate limiter, atomically consumes the Redis record through `DeployGrantService.Consume`, rechecks current persisted authorization and returns the signed script envelope. It must not restore the legacy `/agent/script/:id` route.

- [ ] **Step 5: Generate safe actions**

- Use short-lived OSS signed HTTPS URLs, not OSS keys.
- Restrict archives to `.zip`.
- Emit logical destinations such as `sandbox/downloads/file.zip`, never arbitrary absolute paths.
- Use strict per-action payload DTOs shared with Rust fixtures. Do not use free-form `map[string]string`; reject unknown payload fields.

- [ ] **Step 6: Run checks**

```powershell
cd backend
go test ./internal/service -run TestDeployScript -v
go test ./...
go vet ./...
go build ./...
cd ..\frontend
npm run lint
npm run build
```

- [ ] **Step 7: Stop for real key provisioning**

If the production Ed25519 private key and client public-key distribution path are not provisioned, record the blocker. Keep the feature flag false.

- [ ] **Step 8: Commit**

```powershell
git add backend frontend testdata .env.example docs/superpowers/plans progress.txt
git commit -m "Beta D-03: sign deploy scripts with Ed25519 - completed"
```

## Task D-04: Harden Tauri Signature, Schema And Filesystem Boundaries

**Files:**
- Modify: `tauri-client/src-tauri/Cargo.toml`
- Create: `tauri-client/src-tauri/build.rs`
- Create: `tauri-client/.env.example`
- Modify: `tauri-client/src/App.tsx`
- Modify: `tauri-client/src-tauri/tauri.conf.json`
- Replace: `tauri-client/src-tauri/src/commands/security.rs`
- Modify: `tauri-client/src-tauri/src/commands/file_ops.rs`
- Modify: `tauri-client/src-tauri/src/url_scheme.rs`
- Modify: `tauri-client/src-tauri/src/lib.rs`
- Modify: `tauri-client/src-tauri/capabilities/default.json`

- [ ] **Step 1: Write failing Rust tests**

Cover:

- Deep link accepts only one `grant` value in `omnicraft://deploy?grant=...`; reject missing, duplicate and unknown query parameters.
- Signature verify accepts Ed25519 public-key signature and rejects tamper.
- Expired scripts, repeated `script_id`, unknown actions and extra payload fields fail.
- New directories and files are validated by canonicalizing nearest existing parent.
- Zip-slip paths such as `../escape.txt` fail.
- Non-HTTPS downloads and non-allowlisted hosts fail.
- Oversized or timed-out downloads fail.

- [ ] **Step 2: Replace HMAC verification**

Embed one active Beta public key and its ID. **Public key embedding method:** Use compile-time environment variables `DEPLOY_PUBLIC_KEY_ID` and `DEPLOY_PUBLIC_KEY_B64`; read them with `option_env!()` constants and reject release startup when either value is missing. Use `build.rs` to make Cargo rebuild when any Rust-side deploy build variable changes. Decode the base64 public key once during startup, require exactly 32 bytes, construct the Ed25519 verifying key and reject signed envelopes whose `key_id` does not match the embedded ID. This allows key rotation by rebuilding the client without modifying source code. Overlapping multi-key rotation requires a separate design decision. Example in `build.rs`:

```rust
// build.rs
fn main() {
    println!("cargo:rerun-if-env-changed=DEPLOY_PUBLIC_KEY_ID");
    println!("cargo:rerun-if-env-changed=DEPLOY_PUBLIC_KEY_B64");
    println!("cargo:rerun-if-env-changed=DEPLOY_ALLOWED_DOWNLOAD_HOSTS");
    println!("cargo:rerun-if-env-changed=DEPLOY_DOWNLOAD_TIMEOUT_SEC");
}
```

```rust
// In security.rs
const DEPLOY_PUBLIC_KEY_ID: Option<&str> = option_env!("DEPLOY_PUBLIC_KEY_ID");
const DEPLOY_PUBLIC_KEY_B64: Option<&str> = option_env!("DEPLOY_PUBLIC_KEY_B64");
```

Parse build values into an explicit runtime policy object during application startup. Unit tests construct that policy from deterministic test values rather than depending on a developer machine's environment. Verify the exact UTF-8 `script_json` bytes before parsing into strict Rust structs with `#[serde(deny_unknown_fields)]`.

- [ ] **Step 3: Replace absolute-path command inputs**

Rust resolves logical destinations beneath fixed roots. Web/backend cannot select arbitrary absolute paths. Validate the nearest existing parent for new files and directories, then append only normalized path components.

- [ ] **Step 4: Tighten filesystem commands**

- Restrict extraction to zip.
- Validate extracted entry paths structurally before writing.
- Enforce the existing configured upload limit for the attachment content type and support an optional `sha256` field. When `sha256` is present, verify it before applying the operation. Generating checksums for every command is not a Beta gate.
- Configure the native HTTP client timeout through build-time `DEPLOY_DOWNLOAD_TIMEOUT_SEC` with default `30`.
- Restrict HTTPS download hosts to the comma-separated, exact-hostname build-time allowlist `DEPLOY_ALLOWED_DOWNLOAD_HOSTS`. Document the release values in `tauri-client/.env.example`; do not use a wildcard host suffix.
- Replace the hardcoded `http://localhost:8080/api/v1` in `tauri-client/src/App.tsx` with build-time `VITE_API_BASE_URL`. Release builds must reject a missing or non-HTTPS value.
- Update `tauri-client/src-tauri/tauri.conf.json` so `app.security.csp` for release packaging allows only the confirmed HTTPS API host and confirmed OSS download hosts. Put localhost allowances only in Tauri's dedicated `app.security.devCsp`; do not leave localhost in the release `csp`.
- Require explicit confirmation in UI before sensitive write/move actions.
- Stop subsequent actions after first failure.
- Keep `download_file`, `extract_archive`, `move_file`, `create_dir`, `read_config`, `write_config` and automatic `backup_file` as internal Rust operations. Remove the raw file-operation functions from Tauri's public `invoke_handler`.
- Expose `verify_deploy_script`, which stores a verified strict script in Rust state and returns an opaque in-memory handle plus a sanitized action preview. Expose one narrow `execute_verified_script(handle)` command that resolves that stored handle, invokes a Rust/native script-wide confirmation dialog, enforces second native confirmation for `write_config` and `move_file`, and runs internal operations sequentially. A compromised WebView must not be able to invoke arbitrary file operations directly or bypass confirmation by constructing a payload.

- [ ] **Step 5: Remove direct WebView filesystem permissions**

**Before removing any permissions**, audit the current permission usage:

```powershell
cat tauri-client/src-tauri/capabilities/default.json
rg -n "fs:|readFile|writeFile|readDir|removeFile" tauri-client/src-tauri/src tauri-client/src
```

Document each `fs:*` permission's current purpose. Identify which permissions are used only by the verified-script executor (safe to remove from WebView) and which might be used by other features. Record findings as a risk assessment in the task output.

Then remove `fs:default`, `fs:allow-read`, `fs:allow-write`, `fs:allow-remove`, rename/copy/mkdir permissions and direct scope permissions unless a proven runtime need remains for non-deploy features. If any `fs:*` permission is required by a feature outside the verified-script executor, document it as a retained permission with justification. Keep the verified-script Rust executor as the file boundary. Remove unnecessary shell capability.

- [ ] **Step 6: Run Rust checks**

```powershell
cd tauri-client
npm run build
cargo test --manifest-path src-tauri/Cargo.toml
cargo clippy --manifest-path src-tauri/Cargo.toml -- -D warnings
```

- [ ] **Step 7: Commit**

```powershell
git add tauri-client docs/superpowers/plans progress.txt
git commit -m "Beta D-04: harden Tauri deploy execution boundary - completed"
```

## Task D-05: Update Web And Desktop Grant Flow

**Files:**
- Modify: `frontend/components/content/ContentDetail.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Modify: `tauri-client/src/App.tsx`
- Modify: `tauri-client/src-tauri/src/url_scheme.rs`

- [ ] **Step 1: Update Web deep link**

Before editing the Web detail UI, run:

```powershell
rg -n "## Component: ContentDetail" design/ui-spec.md
```

When the user confirms one-click deploy:

```text
POST /api/v1/deploy-grants {"content_id":123}
omnicraft://deploy?grant=<opaque-grant>
```

Do not put Web JWT, user ID or content metadata in the deep link.

- [ ] **Step 2: Update desktop exchange**

Tauri exchanges grant, verifies signed script bytes, strictly parses the script, shows environment and the complete action list, then asks for confirmation. **Sensitive operation confirmation rules:**
- `write_config` and `move_file` operations always trigger a second confirmation dialog showing the target path, in addition to the initial script-wide confirmation.
- `download_file` and `create_dir` operations require only the single script-wide confirmation.
- `read_config` and `extract_archive` require only the single script-wide confirmation.
- `backup_file` is system-automatic and never shown as a separate confirmation step.

Fetching a script must not start execution before the complete list has been rendered and confirmed.

- [ ] **Step 3: Add desensitized errors and feedback CTA**

Map Rust/backend failures to stable error codes. Do not display raw local paths, tokens or provider errors. Offer `/feedback` with an explicit user-confirmed diagnostic-summary draft limited to `app_version`, `platform`, `error_code` and `failed_action`.

- [ ] **Step 4: Run checks**

```powershell
cd frontend
npm run lint
npm run build
cd ..\tauri-client
npm run build
cargo test --manifest-path src-tauri/Cargo.toml
cargo clippy --manifest-path src-tauri/Cargo.toml -- -D warnings
```

- [ ] **Step 5: Run desktop end-to-end verification**

Verify valid grant, replayed grant, expired grant, tampered script, unknown action, invalid HTTPS host, zip-slip and destination escape. Keep `desktop_deploy_enabled=false` until every case passes.

- [ ] **Step 6: Commit**

```powershell
git add frontend tauri-client docs/superpowers/plans progress.txt
git commit -m "Beta D-05: complete secure desktop deploy grant flow - completed"
```
