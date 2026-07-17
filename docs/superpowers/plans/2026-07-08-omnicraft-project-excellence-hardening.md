# OmniCraft Project Excellence Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 提升默认验证门、文档治理和后端 HTTP 装配边界的可信度，让项目具备可重复验证、可解释演进的工程质量。

**Architecture:** 前端使用跨平台测试发现器执行全部单元测试；doc-validator 将当前发布真相与归档卫生分开，同时修复当前文档误报；后端把 HTTP 路由装配移入专用 composition-root 包，并消除其中临时构造 repository/service 的做法；统一项目级验证入口负责聚合各子工程门禁。

**Tech Stack:** Go 1.22+/Gin/GORM, Node.js 20+/Next.js/TypeScript/tsx/node:test/Playwright, PowerShell, existing `tools/doc-validator`.

---

## Status And Verified Evidence

**Review date:** 2026-07-16

- This plan was recovered from commit `b2bc3a6` on `codex/hardening/plan-review`; it was not present on `main`. This rewritten copy is the current implementation source.
- `frontend/package.json > scripts.test` names a fixed subset and omits six existing TSX test files: admin notifications, collection detail, collection picker, history, messages, and user collections.
- `go run . --check --diff` in `tools/doc-validator` reported 123 issues before this review. Most are archive-only broken references, while the remaining active-scope issue is a false bare-file-URI match in the documentation-governance plan. The malformed community guide expiry header has been normalized by this documentation revision but remains covered by Task 2 regression tests.
- `backend/internal/handler/routes.go` owns route mapping, middleware construction, handler construction, and at least one inline repository/service construction path.
- The former Task 6 architecture-backlog note is intentionally removed. `AdminHandler`, `ContentService`, and large frontend modules are only split when a user-visible task needs that seam.

## Scope And Coordination

- This plan is a Mode D engineering-hardening source. Do not modify `task.json`, Beta roadmap checkboxes, or community completion checkboxes.
- Implement each numbered task as an independently reviewable commit.
- Tasks 1 and 2 are independent. Task 5 depends on Tasks 1 and 2.
- Task 3 must not run while a community branch edits route registration. Task 4 depends on Task 3.
- Community plans 4–6 are route-owner adaptive: if Task 3 lands first they must edit `internal/router/routes.go` and must not recreate `internal/handler/routes.go`. Re-read the latest target plan before reserving the route file.
- Preserve the user's unrelated untracked/modified files. Stage exact files only.

---

## Task 1: Make Frontend Unit Tests Discoverable And Complete

**Files:**
- Create: `frontend/scripts/run-tests.mjs`
- Create: `frontend/scripts/run-tests.test.mjs`
- Modify: `frontend/package.json`

- [x] **Step 1: Write failing discovery tests**

Cover these contracts in `run-tests.test.mjs`:

- recursively discover `*.test.cjs`, `*.test.mjs`, `*.test.js`, `*.test.jsx`, `*.test.ts`, and `*.test.tsx` below `frontend/`;
- exclude `node_modules`, `.next`, `coverage`, `playwright-report`, `test-results`, and `e2e`;
- normalize Windows separators and return stable sorted paths;
- group native JS/CJS/MJS tests separately from TS/TSX tests;
- return a non-zero result when a child test command fails;
- never require shell glob expansion.

- [x] **Step 2: Confirm red**

```powershell
cd frontend
node --test scripts/run-tests.test.mjs
```

Expected: FAIL because `run-tests.mjs` does not exist.

- [x] **Step 3: Implement the runner**

Requirements:

- use `process.execPath` instead of resolving a platform-specific `node` command;
- use `spawnSync(..., { shell: false, stdio: "inherit" })`;
- run native files with `node --test` and TS/TSX files with `node --import tsx --test`;
- export pure discovery/grouping functions for self-tests;
- throw/return failures from library functions; call `process.exitCode = 1` only in the CLI entrypoint;
- reject unknown modes. The only initial mode is `unit`.

- [x] **Step 4: Confirm runner tests green**

```powershell
cd frontend
node --test scripts/run-tests.test.mjs
```

- [x] **Step 5: Update package scripts without replacing unrelated keys**

Merge these keys into `frontend/package.json`:

```json
{
  "test": "node scripts/run-tests.mjs unit",
  "test:unit": "node scripts/run-tests.mjs unit",
  "test:contracts": "playwright test --project=mocked",
  "verify": "npm run lint && npm run test:unit && npm run build",
  "verify:release": "npm run verify && npm run test:e2e"
}
```

Run `git diff -- frontend/package.json` and prove no existing script key was dropped.

- [x] **Step 6: Run the complete unit gate**

```powershell
cd frontend
npm.cmd test
```

Expected: output includes every currently checked-in unit test, including the six files omitted by the old fixed list.

- [x] **Step 7: Run the frontend verification gate**

```powershell
cd frontend
npm.cmd run verify
```

Expected: TypeScript, all discovered unit tests, and Next.js build PASS.

- [x] **Step 8: Commit**

```powershell
git add frontend/package.json frontend/scripts/run-tests.mjs frontend/scripts/run-tests.test.mjs
git commit -m "Chore: make frontend unit tests discoverable"
```

---

## Task 2: Split Doc Validator Profiles Without Hiding Current Debt

**Files:**
- Modify: `tools/doc-validator/main.go`
- Modify: `tools/doc-validator/rules/cross_refs.go`
- Create: `tools/doc-validator/main_test.go`
- Create: `tools/doc-validator/rules/cross_refs_test.go`
- Modify: `tools/doc-validator/README.md`
- Modify: `docs/working/2026-06-30-community-features-development-guide.md`

- [x] **Step 1: Write failing cross-reference tests**

Add tests proving:

- release profile skips `docs/archive/**` only;
- archive profile checks only `docs/archive/**`;
- active docs, design, specs, plans, and working docs remain checked by release profile;
- absolute Windows-drive and POSIX-path forms of the file URI scheme are recognized;
- prose containing only a bare file URI scheme, without an absolute path, is ignored;
- Markdown links still support anchors without treating the anchor as part of the filename.

- [x] **Step 2: Confirm red**

```powershell
cd tools/doc-validator
go test ./rules -run "TestCheckCrossRefs|TestFileReference" -v
```

- [x] **Step 3: Add explicit cross-reference options and strict file URI parsing**

Introduce:

```go
type CrossRefOptions struct {
    IncludeArchive bool
    OnlyArchive    bool
}
```

The file-URI matcher must require an absolute Windows drive path or an absolute POSIX path after the scheme. Do not silence arbitrary active-document failures.

- [x] **Step 4: Write failing profile tests**

Cover default `release`, explicit `archive`, explicit `all`, and unknown-profile rejection.

- [x] **Step 5: Implement profiles**

- `release`: config/schema/route/token/current cross-reference checks plus working-document expiry checks; excludes archive cross-references.
- `archive`: archive cross-references only; non-release-blocking.
- `all`: release checks plus archive cross-references.

Print the active profile in command output.

- [x] **Step 6: Normalize the verified malformed working header**

Change only the top metadata of `docs/working/2026-06-30-community-features-development-guide.md` to:

```markdown
**创建日期**: 2026-06-30
**预计失效日期**: 2026-08-30
```

The other current working documents already use the validator-compatible expiry format and must not be rewritten without a new finding.

- [x] **Step 7: Run tool tests**

```powershell
cd tools/doc-validator
go test ./... -v
```

- [x] **Step 8: Verify profiles against the repository**

```powershell
cd tools/doc-validator
go run . --check --profile release
go run . --check --profile archive
```

Expected: release PASS. Archive may fail while historical links remain, and that debt stays visible in archive output.

- [x] **Step 9: Update README and commit**

Document all profiles and explicitly state that release excludes only archive debt.

```powershell
git add tools/doc-validator/main.go tools/doc-validator/main_test.go tools/doc-validator/rules/cross_refs.go tools/doc-validator/rules/cross_refs_test.go tools/doc-validator/README.md docs/working/2026-06-30-community-features-development-guide.md
git commit -m "Chore: split current and archive documentation checks"
```

---

## Task 3: Establish A Dedicated HTTP Composition Root

**Files:**
- Create: `backend/internal/router/routes.go`
- Create: `backend/internal/router/routes_test.go`
- Move/modify: `backend/internal/handler/routes_security_test.go` -> `backend/internal/router/routes_security_test.go`
- Move/modify: `backend/internal/handler/agent_deploy_disabled_test.go` -> `backend/internal/router/agent_deploy_disabled_test.go`
- Modify: `backend/internal/container/container.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/main_test.go`
- Modify: `tools/doc-validator/rules/route_sync.go`
- Delete after verification: `backend/internal/handler/routes.go`
- Generated: `architecture.md`

- [ ] **Step 1: Freeze route behavior with failing ownership tests**

Tests must require `main.go` to call `router.RegisterRoutes`, retain trusted-proxy/CORS ordering, preserve representative public/protected/admin/disabled routes, and reject a second route owner in the handler package.

- [ ] **Step 2: Confirm red**

```powershell
cd backend
go test ./cmd/server -run TestMainRegistersRoutesThroughRouterPackage -v
```

- [ ] **Step 3: Complete reusable dependencies in the service container**

Before moving routes, identify every `repository.New...` or `service.New...` currently called inside `handler/routes.go`. Add the reusable dependency to `ServiceContainer` and its constructor/tests. The new router package may construct HTTP handlers from container-owned dependencies, but must not create repositories or domain services ad hoc.

- [ ] **Step 4: Move route composition**

Create `internal/router/routes.go`, preserve route paths and middleware order, import handler constructors explicitly, and update `main.go`.

- [ ] **Step 5: Move route-level tests and add composition checks**

In addition to existing security tests, assert source-level or constructor-level invariants:

- `internal/router/routes.go` contains no `repository.New` or `service.New`;
- all disabled desktop/payment routes retain 503 behavior;
- protected routes retain auth and interaction middleware.

- [ ] **Step 6: Update doc-validator route ownership before deleting the old file**

Change `route_sync.go` and the generated marker source from `internal/handler/routes.go` to `internal/router/routes.go`. Tests must prove `--fix` replaces the existing generated section instead of appending a duplicate.

- [ ] **Step 7: Delete the old route owner and verify focused tests**

```powershell
cd backend
go test ./internal/handler ./internal/router ./cmd/server -v
```

- [ ] **Step 8: Refresh generated architecture and run full backend gates**

```powershell
cd tools/doc-validator
go test ./...
go run . --fix
cd ../../backend
go test ./...
go vet ./...
go build ./...
```

- [ ] **Step 9: Commit exact moved/generated files**

Commit message: `Refactor: establish HTTP composition root`.

---

## Task 4: Centralize Interaction Policy Archetypes

**Files:**
- Create: `backend/internal/router/policies.go`
- Create: `backend/internal/router/policies_test.go`
- Modify: `backend/internal/router/routes.go`

- [ ] **Step 1: Write failing policy and route-attachment tests**

Define only the distinct policy archetypes currently required:

- standard verified interaction: verified email + reputation;
- publishing: standard verified interaction + no publish freeze.

Tests must check both the policy values and representative routes that use each archetype. Do not encode twelve identical policy structs merely because there are twelve route groups.

- [ ] **Step 2: Confirm red**

```powershell
cd backend
go test ./internal/router -run "TestInteractionPolicy|TestRoutePolicy" -v
```

- [ ] **Step 3: Implement named archetypes and route-specific guards**

Keep route-specific guard variable names for readability, but construct them from the two canonical archetypes. If a route genuinely needs a third policy, add it only with a failing behavior test and documented reason.

- [ ] **Step 4: Run router and backend gates**

```powershell
cd backend
go test ./internal/router -v
go test ./...
go vet ./...
go build ./...
```

- [ ] **Step 5: Commit**

```powershell
git add backend/internal/router/policies.go backend/internal/router/policies_test.go backend/internal/router/routes.go
git commit -m "Refactor: centralize interaction policy archetypes"
```

---

## Task 5: Add Project-Level Verification Commands

**Files:**
- Create: `scripts/verify-project.ps1`
- Create: `scripts/verify-project.tests.ps1`
- Modify: `README.md`
- Modify: `AGENTS.md` and `CLAUDE.md` only if their command guidance needs synchronization

- [ ] **Step 1: Write failing script-contract tests**

The tests must prove non-zero external command exit codes stop the script, locations are restored after failures, `-Release` adds Playwright, and `-Tauri` adds Tauri frontend/Rust gates.

- [ ] **Step 2: Implement fail-fast verification script**

Default gates:

- backend `go test ./...`, `go vet ./...`, `go build ./...`;
- frontend `npm.cmd run verify`;
- doc-validator `go test ./...` and `go run . --check --profile release`.

Optional switches:

- `-Release`: frontend Playwright release suite; requires documented running-service prerequisites;
- `-Tauri`: `tauri-client npm.cmd run build` and `cargo test --manifest-path src-tauri/Cargo.toml`.

Use a helper that checks `$LASTEXITCODE` after every external command. `$ErrorActionPreference = "Stop"` alone is not sufficient.

- [ ] **Step 3: Verify the script tests fail before implementation and pass after implementation**

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-project.tests.ps1
```

- [ ] **Step 4: Run the aggregate default gate**

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-project.ps1
```

- [ ] **Step 5: Document verification tiers**

README must distinguish default engineering verification, release/browser verification, Tauri verification, and archive-hygiene checks. Aggregate commands do not replace required screenshots or external-service verification.

- [ ] **Step 6: Commit**

Commit message: `Chore: add project verification gate`.

---

## Final Verification

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-project.ps1
```

When release services and credentials are available:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-project.ps1 -Release
```

When Tauri files are touched:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-project.ps1 -Tauri
```

## Non-Goals

- Do not implement community business features in this plan.
- Do not implement desktop D-02 through D-05 here; their priority and safety requirements remain in the Beta desktop plan.
- Do not split `AdminHandler`, `ContentService`, or large frontend modules without a feature-driven seam.
- Do not create a standalone architecture-deepening backlog document.
- Do not change product behavior beyond verification, documentation checking, and behavior-preserving composition refactoring.

## Review Checklist

- [ ] Default frontend test gate discovers all supported unit-test extensions without shell globs.
- [ ] Release doc profile excludes only archive debt and remains strict for current documents.
- [ ] The current false bare-file-URI match has a regression test.
- [ ] Router package owns route composition but not repository/domain-service construction.
- [ ] Policy tests cover archetypes and actual route attachment.
- [ ] Project verifier fails fast on every external command.
- [ ] Task 6/backlog note is absent.
- [ ] No roadmap/community/task.json completion state is changed.
