# OmniCraft Desktop Deploy Security Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the unsafe desktop one-click deployment prototype with a short-lived grant and Ed25519-verified execution pipeline.

**Architecture:** Web requests an opaque, single-use deploy grant. Tauri exchanges the grant with the backend, receives a canonical Ed25519-signed script, verifies schema and signature using an embedded public key, displays actions to the user, and executes only audited Rust commands against logical paths rooted inside fixed allowlisted directories. Keep `features.desktop_deploy_enabled=false` until D-05 passes end to end.

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
- Modify: `frontend/lib/public-config.ts`.
- Modify: `frontend/messages/zh.json`, `frontend/messages/en.json`.

### Tauri

- Modify: `tauri-client/src/App.tsx`.
- Modify: `tauri-client/src-tauri/src/url_scheme.rs`.
- Replace: `tauri-client/src-tauri/src/commands/security.rs`.
- Modify: `tauri-client/src-tauri/src/commands/file_ops.rs`.
- Modify: `tauri-client/src-tauri/src/lib.rs`.
- Modify: `tauri-client/src-tauri/capabilities/default.json`.
- Modify: `tauri-client/src-tauri/Cargo.toml`.

## Task D-01: Disable Unsafe Desktop Entry By Default

**Files:**
- Modify: `backend/config/config.go`
- Modify: `backend/config.yaml`
- Modify: `.env.example`
- Modify: `frontend/components/content/ContentDetail.tsx`
- Modify: `frontend/components/studio/PublishForm.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Add feature flag**

```yaml
features:
  desktop_deploy_enabled: false
```

- [ ] **Step 2: Hide prototype UI**

Do not render one-click deploy links or creator enablement toggles while the flag is false. `/client` may remain as an information/manual-download page.

- [ ] **Step 3: Add backend rejection**

The current `/api/v1/agent/script/:id` route must return:

```json
{"code":"FEATURE_DISABLED","message":"desktop deploy is not enabled"}
```

until secure grant exchange replaces it.

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
git add backend frontend .env.example screenshots docs/superpowers/plans progress.txt
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
    IssuedAt  time.Time
}

func (s *DeployGrantService) Issue(ctx context.Context, userID, contentID int64) (string, error)
func (s *DeployGrantService) Exchange(ctx context.Context, rawGrant string) (*SignedDeployScript, error)
```

Redis stores:

```text
deploy:grant:<sha256(grant)>
```

with five-minute configured TTL. Exchange must atomically consume the key.

- [ ] **Step 3: Recheck authorization before issue and exchange**

Require:

- Feature flag enabled.
- Existing, non-deleted, non-banned, verified user with sufficient reputation.
- Published content.
- `allow_copy=true`.
- `agent_enabled=true`.
- Every attachment belongs to the content.

- [ ] **Step 4: Mount endpoints**

```text
POST /api/v1/deploy-grants
POST /api/v1/deploy-grants/exchange
```

Both require authenticated interaction eligibility. Exchange accepts only opaque grant, not Web JWT.

- [ ] **Step 5: Run checks**

```powershell
cd backend
go test ./internal/service ./internal/handler -run TestDeployGrant -v
go test ./...
go vet ./...
go build ./...
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
- Create: `backend/internal/service/deploy_script.go`
- Create: `backend/internal/service/deploy_script_test.go`
- Modify: `backend/internal/service/agent_service.go`

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

Cover stable canonical JSON, valid signature, tamper rejection, expiry, unknown action rejection, and logical path validation.

- [ ] **Step 3: Sign with Ed25519 private key**

Load private key from production secret configuration. Return:

```json
{"script":{...},"signature":"base64...","key_id":"desktop-2026-01"}
```

Never return or log the private key. Remove HMAC use from deploy script generation.

- [ ] **Step 4: Generate safe actions**

- Use short-lived OSS signed HTTPS URLs, not OSS keys.
- Restrict archives to `.zip`.
- Emit logical destinations such as `sandbox/downloads/file.zip`, never arbitrary absolute paths.
- Use parameter names matching Rust DTOs.

- [ ] **Step 5: Run checks**

```powershell
cd backend
go test ./internal/service -run TestDeployScript -v
go test ./...
go vet ./...
go build ./...
```

- [ ] **Step 6: Stop for real key provisioning**

If the production Ed25519 private key and client public-key distribution path are not provisioned, record the blocker. Keep the feature flag false.

- [ ] **Step 7: Commit**

```powershell
git add backend .env.example docs/superpowers/plans progress.txt
git commit -m "Beta D-03: sign deploy scripts with Ed25519 - completed"
```

## Task D-04: Harden Tauri Signature, Schema And Filesystem Boundaries

**Files:**
- Modify: `tauri-client/src-tauri/Cargo.toml`
- Replace: `tauri-client/src-tauri/src/commands/security.rs`
- Modify: `tauri-client/src-tauri/src/commands/file_ops.rs`
- Modify: `tauri-client/src-tauri/src/url_scheme.rs`
- Modify: `tauri-client/src-tauri/src/lib.rs`
- Modify: `tauri-client/src-tauri/capabilities/default.json`

- [ ] **Step 1: Write failing Rust tests**

Cover:

- Deep link accepts only `omnicraft://deploy?grant=...`.
- Signature verify accepts Ed25519 public-key signature and rejects tamper.
- Expired scripts, repeated `script_id`, unknown actions and extra payload fields fail.
- New directories and files are validated by canonicalizing nearest existing parent.
- Zip-slip paths such as `../escape.txt` fail.
- Non-HTTPS downloads and non-allowlisted hosts fail.
- Oversized or timed-out downloads fail.

- [ ] **Step 2: Replace HMAC verification**

Embed only public keys keyed by `key_id`. Parse into strict Rust structs with `#[serde(deny_unknown_fields)]`.

- [ ] **Step 3: Replace absolute-path command inputs**

Rust resolves logical destinations beneath fixed roots. Web/backend cannot select arbitrary absolute paths.

- [ ] **Step 4: Tighten filesystem commands**

- Restrict extraction to zip.
- Validate extracted entry paths structurally before writing.
- Add configured max bytes, request timeout and optional checksum.
- Require explicit confirmation in UI before sensitive write/move actions.
- Stop subsequent actions after first failure.

- [ ] **Step 5: Remove direct WebView filesystem permissions**

Remove `fs:default`, `fs:allow-read`, `fs:allow-write`, `fs:allow-remove`, rename/copy/mkdir permissions and direct scope permissions unless a proven runtime need remains. Keep custom Rust commands as the file boundary. Remove unnecessary shell capability.

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

When the user confirms one-click deploy:

```text
POST /api/v1/deploy-grants {"content_id":123}
omnicraft://deploy?grant=<opaque-grant>
```

Do not put Web JWT, user ID or content metadata in the deep link.

- [ ] **Step 2: Update desktop exchange**

Tauri exchanges grant, verifies signed script, shows environment and complete action list, then asks for confirmation. Sensitive writes and moves require an additional confirmation.

- [ ] **Step 3: Add desensitized errors and feedback CTA**

Map Rust/backend failures to stable error codes. Do not display raw local paths, tokens or provider errors. Offer `/feedback`.

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

