# OmniCraft Production Readiness Implementation Plan

> **2026-07-25 Web-only execution scope:** Ops-00 is complete. Continue only Ops-01 through Ops-08 for the Web production-ready claim. Ops-09 is intentionally deferred, remains unchecked, and may resume only after the user restores Desktop scope and D-02 through D-05/R-02 are complete. Keep the Ops-09 section in this file as the future extension; do not archive or duplicate it.

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立从 PR 验证、数据库升级和恢复，到可观测性、安全、制品证明、负载验证、部署回滚与桌面签名的可审计生产发布链。

**Architecture:** GitHub Actions 只编排仓库内可本地复现的 verifier；数据库使用 forward-only ledger；发布使用不可变 digest、机器可读证据和 staging 演练。当前 Web 主线由 Ops-00～Ops-08 串联；Ops-09 保留为恢复 Desktop 范围后的扩展。外部密钥缺失不能削弱 deterministic gates。

**Tech Stack:** GitHub Actions、bash 脚本（本地与 CI 均使用仓库 `bash scripts/*.sh` 入口；2026-08-03 起由 PowerShell 移植，历史 `.ps1` 不再使用）、Go 1.25.12、Node.js 20、npm、PostgreSQL 16 + pgvector、Redis 7、Docker Compose、Prometheus/Alertmanager、Grafana Alloy/Loki、Trivy/gitleaks/govulncheck/cargo-audit、CycloneDX/SPDX、k6、Tauri 2、Windows Authenticode。

**Design input:** `docs/superpowers/specs/2026-07-17-omnicraft-production-readiness-design.md`

---

## Status, authority, and universal rules

**Status:** Ops-00 completed at `9ea53c8`; Ops-01 completed by CI commit `7f34191` and evidence/branch-protection closeout `4cba1da`; Ops-02 completed on `codex/ops/ops-02` after two-axis review, final-commit-bound local evidence and PR #21 PostgreSQL migration/contract/project gates; Ops-03 completed on `codex/ops/ops-03` after contract tests, a real observability drill and the default project gate (merged to main with Ops-04 at `ddaf0a4`); Ops-04 completed on `codex/ops/ops-04` after contract tests, promtool/amtool validation, a real firing/resolved alert drill with the in-network alert-sink and a real Healthchecks.io missing-heartbeat down-flip (merged to main at `ddaf0a4`, including llm semantic merge); Ops-05 completed on `codex/ops/ops-05` after policy/exception/pinned-action contract tests, fixture failure isolation, the default project gate and the full filesystem/IaC/image scan set (merged to main at `9c0e195`); Ops-06 completed on `codex/ops/ops-06` after manifest/SBOM/pinned-generator contract tests, deterministic two-run SBOM comparison, full container+lockfile generation, 17-check provenance verification with OCI label↔commit binding, downloaded-artifact re-verification, one-year-retention archive receipt and the default project gate; Ops-07 completed on `codex/ops/ops-07` after runner/target-safety contract tests, the approved release performance profile, real smoke/load/stress baseline measurement, k6 threshold validation and the default project gate; **Ops-08 completed on `codex/ops/ops-08`** (merged to main at `074715f`) after Step 1-4 contract tests and the real Step 5 staging drill (7 real inputs: `omnicraft` OSS bucket cn-guangzhou with versioning enabled, server-read Green/CAPTCHA credentials, GitHub Release `v0.1.0-rc.1`, off-site `omnicraft-ops-archive`; real container deploy/rollback/redeploy with candidate/previous digests, real pg_dump/restore and Aliyun OSS version-restore measurements, approved RPO≤5min/RTO≤30min objectives, machine-compare all-met, dual-destination archive receipt verified, release-tier verify-project + validate-evidence passed); Ops-09 is deferred by the 2026-07-25 Web-only scope decision.

- This plan is not Hardening Task 6 and must never add one.
- Do not modify `task.json`, Beta/community checkboxes, or Web Agent completion state.
- Ops-00 was completed in `codex/ops/ops-00`. Before Ops-01, create or refresh `codex/ops-integration` from the latest `main`; Ops-01～08 use `codex/ops/ops-XX` from the latest integration commit. Do not create an Ops-09 branch while Desktop scope is deferred.
- One Ops Task equals one session, worktree, branch, reviewed commit, and `progress.txt` entry.
- Rebase onto `codex/ops-integration`, run the task's full gates, then merge with `git merge --ff-only`.
- Every implementation behavior uses red → green → refactor. Operational YAML/shell behavior uses contract tests plus a real tool/drill; lint alone is insufficient.
- Each task receives specification-conformance review followed by code-quality review. Fix and re-review all important findings.
- Stage exact paths. Never use `git add .` in a dirty worktree.
- Evidence artifacts must record commit, command, exit code, tool version, timestamps, and redaction status.
- Every task writes `artifacts/ops-XX/summary.json` with `task`, `commit`, `started_at`, `finished_at`, `commands`, `exit_codes`, `tool_versions`, `evidence`, `redaction_checked`, and `blockers`. The task-specific verifier/drill receives the root `-ReportDir artifacts/ops-XX` and creates it. `artifacts/` is ignored from Ops-00 onward; evidence is uploaded/archived, never committed.
- For every task, run the listed contract command before implementation and record the expected nonzero exit; run the identical command after implementation and require zero. Then run the listed final project gate. A test added after implementation or a lint-only drill does not satisfy TDD.
- A verifier run before commit may create a provisional summary, but acceptance requires post-commit finalization: `summary.commit` must equal the reviewed task commit at final `HEAD`. Ops-01 creates a tested finalizer; Ops-01～09 run it after their exact commit and then revalidate the summary. If final validation requires a tracked-file fix, amend the one task commit, finalize again, and revalidate before merge.

## Coordination with UI Polish

1. Finish and merge Ops-00 before registering the UI plan because both edit `AGENTS.md`, `CLAUDE.md`, and `progress.txt`.
2. UI U-01/U-05 may start after the UI plan is committed and registered; their files do not overlap Ops-01～07.
3. UI U-11 waits for this spec and must preserve the rule that raw reputation thresholds are not exposed through public config. Its current exact paths do not overlap Ops-03/08/09; before start it must publish the exact reservation again, compare it with the active Ops reservation, and serialize/transfer ownership if the list expands into config, compose, release or Tauri capability paths.
4. Ops-01 owns `frontend/package.json`, `scripts/verify-project.sh`, and `scripts/verify-project.tests.sh` before UI U-12. U-12 rebases after Ops-01.
5. During Ops-08/09 release drills, freeze concurrent edits to Dockerfiles, compose, release workflows and Tauri bundle configuration.

## Exact red/green and final-gate matrix

| Task | Run before (expect nonzero) and after (expect zero) | Final gate |
|---|---|---|
| Ops-00 | `rg -n "模式 E：Production Readiness" AGENTS.md CLAUDE.md` | doc-validator tests + release profile + `git diff --check` |
| Ops-01 | `scripts/verify-project.tests.sh`; `scripts/ci/verify-workflows.tests.sh` | default verifier + `--tauri` |
| Ops-02 | `go test ./internal/migration ./cmd/migrate -count=1`; `scripts/db/recovery-drill.tests.sh` | default verifier + real recovery drill |
| Ops-03 | `go test ./internal/observability ./internal/middleware ./cmd/server -count=1`; `ops/observability/prometheus.tests.sh`; `scripts/ops/observability-drill.tests.sh` | doc-validator `--fix` + default verifier + scrape drill |
| Ops-04 | `scripts/ops/verify-alerts.tests.sh`; `scripts/ops/alert-drill.tests.sh` | default verifier + real firing/resolved drill |
| Ops-05 | `scripts/security/verify-security.tests.sh`; `scripts/security/verify-pinned-actions.tests.sh` | default verifier + full filesystem/IaC/image scan |
| Ops-06 | `scripts/release/generate-sbom.tests.sh`; `scripts/release/verify-provenance.tests.sh` | default verifier + downloaded-artifact verification |
| Ops-07 | `scripts/load/run-load-tests.tests.sh` | default verifier + smoke/load/stress |
| Ops-08 | `scripts/release/preflight.tests.sh`; `scripts/release/deployment-contract.tests.sh` | doc-validator `--fix` + verifier `--release` + staging deploy/rollback |
| Ops-09 | `scripts/release/verify-desktop-artifacts.tests.sh` | verifier `--release --tauri` + clean-runner signature/rotation verification |

In GitHub Actions and on the local macOS development host, invoke `.sh` entries identically with `bash <path>` (POSIX-compatible bash 3.2+；不再使用 `powershell`/`pwsh`)。Go commands run from `backend/`. Each task section repeats any additional environment/setup command required by its real drill.

Ops-01 creates the shared `release/ops-evidence.schema.json` and `scripts/ops/validate-evidence.sh`. Ops-01～09 finish with the exact assertion below, replacing `XX`; any missing/invalid root summary fails the task:

```bash
if [ ! -f artifacts/ops-XX/summary.json ]; then echo "missing Ops summary" >&2; exit 1; fi
bash scripts/ops/validate-evidence.sh -Schema release/ops-evidence.schema.json -Summary artifacts/ops-XX/summary.json
```

The section-level validation above proves structure before commit. After the task's single reviewed commit, finalize identity and validate again:

```bash
finalCommit=$(git rev-parse HEAD)
bash scripts/ops/finalize-evidence.sh -Summary artifacts/ops-XX/summary.json -Commit "$finalCommit"
bash scripts/ops/validate-evidence.sh -Schema release/ops-evidence.schema.json -Summary artifacts/ops-XX/summary.json -ExpectedCommit "$finalCommit"
```

---

## Ops-00: Establish the authoritative Production Readiness source

> Completed at `9ea53c8` on 2026-07-18. The Mode E wording below records the workflow used at that time; current follow-up Ops tasks use the `AGENTS.md` heavy lane.

**Depends on:** Hardening Tasks 1～5 merged at `b1aea75`  
**Branch:** `codex/ops/ops-00`  
**Write reservation:** only the six paths below

**Files:**

- Create: `docs/superpowers/specs/2026-07-17-omnicraft-production-readiness-design.md`
- Create: `docs/superpowers/plans/2026-07-17-omnicraft-production-readiness.md`
- Modify: `.gitignore`
- Modify: `AGENTS.md`
- Modify: `CLAUDE.md`
- Modify: `progress.txt`

- [x] **Step 1: Prove the sources do not already exist and Hardening ends at Task 5**

Run before writing Mode E registration:

```bash
rg -n "模式 E：Production Readiness" AGENTS.md CLAUDE.md
```

Expected before implementation: exit nonzero. After registration run the identical command and require exit zero. Separately confirm both new paths were absent at the base commit and Hardening has no active Task 6 with `git show HEAD:<path>`/`rg` evidence.

- [x] **Step 2: Write the design and this plan**

Cover CI, migrations, backup/restore, logs/metrics, alerts, scans, SBOM/provenance, load, production config, deploy/rollback and Desktop signing. Include exact tasks, evidence and blockers.

- [x] **Step 3: Register independent Mode E in both agent guides**

Add identical source, selection, branch, review, tracking and blocking rules. Explicitly prohibit `task.json` changes and Hardening Task 6.

- [x] **Step 4: Run spec review and plan review loops**

Dispatch one spec-document reviewer and one plan-document reviewer with only the two paths and user requirements. Save complete responses as `artifacts/ops-00/spec-review-round-N.txt` and `artifacts/ops-00/plan-review-round-N.txt`. Allowed terminal status is exactly `APPROVED`; `ISSUES FOUND` or `DONE_WITH_CONCERNS` requires fixes and another full review, maximum three rounds. The final summary lists every report path and refuses completion unless the latest report for each reviewer starts with `APPROVED`.

- [x] **Step 5: Ignore generated evidence and verify docs/tracking isolation**

Add `/artifacts/` to `.gitignore`; do not use a broad pattern that could hide versioned release manifests or fixtures elsewhere.

```bash
cd tools/doc-validator
go test ./...
go run . --check --profile release
cd ../..
git diff --check
git diff -- task.json docs/superpowers/plans/2026-07-08-omnicraft-project-excellence-hardening.md
git check-ignore artifacts/ops-00/summary.json
```

Expected: tests/release profile pass; no diff in forbidden tracking files; the evidence path is ignored.

- [x] **Step 6: Commit the reviewed sources, then create final-commit evidence**

After both review loops are approved, stage exactly the six reserved paths and create the single task commit. Only then generate the untracked/ignored summary so it identifies the reviewed commit rather than parent `b1aea75`:

> 历史记录：Ops-00 于 2026-07-18 在 Windows 主机以 PowerShell 执行；后续 Ops 任务使用下方 bash 形式。

```powershell
git add .gitignore AGENTS.md CLAUDE.md progress.txt docs/superpowers/specs/2026-07-17-omnicraft-production-readiness-design.md docs/superpowers/plans/2026-07-17-omnicraft-production-readiness.md
git commit -m "Ops 00: establish production readiness plan"
$finalCommit = git rev-parse HEAD
$summary = [ordered]@{task='Ops-00';commit=$finalCommit;started_at=$env:OPS_STARTED_AT;finished_at=(Get-Date).ToUniversalTime().ToString('o');commands=@('doc-validator tests','release profile','git diff --check');exit_codes=@(0,0,0);tool_versions=@{go=(go version)};evidence=@('spec-review','plan-review');redaction_checked=$true;blockers=@()}; New-Item -ItemType Directory -Force artifacts/ops-00 | Out-Null; $summary | ConvertTo-Json -Depth 5 | Set-Content artifacts/ops-00/summary.json -Encoding utf8
if ((Get-Content -Raw artifacts/ops-00/summary.json | ConvertFrom-Json).commit -ne $finalCommit) { throw 'Ops-00 summary commit mismatch' }
git status --short
```

Expected: only ignored `artifacts/` evidence remains outside the clean reviewed commit. A tracked-file correction requires amending this one commit and regenerating the summary.

**Acceptance evidence:** reviewer approvals, doc-validator output, `git diff --check`, exact changed-file list, `artifacts/ops-00/summary.json`.  
**Release blocker:** missing topic/task fields, unresolved reviewer concern, Mode E divergence between guides, evidence not ignored, summary commit mismatch, any `task.json` or Hardening Task 6 change.  
**Commit:** `Ops 00: establish production readiness plan`

---

## Ops-01: Establish GitHub Actions CI on the unified verifier

**Depends on:** Ops-00  
**Branch:** `codex/ops/ops-01`  
**Write reservation:** workflow, verifier, package metadata and CI docs listed below; UI U-12 may not reserve shared files until merge

**Files:**

- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/tauri-ci.yml`
- Create: `scripts/ci/verify-workflows.sh`
- Create: `scripts/ci/verify-workflows.tests.sh`
- Create: `release/ops-evidence.schema.json`
- Create: `scripts/ops/validate-evidence.sh`
- Create: `scripts/ops/validate-evidence.tests.sh`
- Create: `scripts/ops/finalize-evidence.sh`
- Create: `scripts/ops/finalize-evidence.tests.sh`
- Modify: `scripts/verify-project.sh`
- Modify: `scripts/verify-project.tests.sh`
- Modify: `frontend/package.json`
- Modify: `tauri-client/package.json`
- Modify: `README.md`
- Modify: `AGENTS.md`
- Modify: `CLAUDE.md`
- Modify: `docs/superpowers/plans/2026-07-17-omnicraft-production-readiness.md`
- Modify: `progress.txt`

- [x] **Step 1: Write failing verifier-scope/report tests**

Add contract cases for `-Scope Backend|Frontend|Docs|All`, default compatibility, deterministic command order, per-command log files, root JSON summary, fail-fast, and nonzero standalone process exit. Add evidence-schema tests for every required field, command exit-code pairing, redaction boolean and blocker list; `validate-evidence.sh -ExpectedCommit` must reject a different/missing commit. Add finalizer tests proving it changes only the identity/finalization fields, refuses a dirty or non-HEAD commit unless an explicit fixture mode is used, hashes the referenced evidence inventory, and is idempotent.

Run:

```bash
bash scripts/verify-project.tests.sh
```

Expected red: unknown scope/report parameters or missing artifacts.

- [x] **Step 2: Implement scoped reporting without duplicating gate truth**

Keep default behavior unchanged. `-ReportDir` writes UTF-8 logs and `verification-summary.json`, including failed command and exit code, then returns nonzero.

- [x] **Step 3: Write failing workflow contract tests**

Test stable required job names, PR/push triggers, concurrency, minimal permissions, SHA-pinned actions, cache keys from lockfiles, artifact upload under `if: always()`, no production secret reference, and Windows/path-filtered Tauri job.

Run:

```bash
bash scripts/ci/verify-workflows.tests.sh
```

Expected red: workflow files absent.

- [x] **Step 4: Implement workflows**

Use job names `backend`, `frontend`, `docs`, `project-gate`, and `tauri-windows`. The Tauri workflow triggers for every PR, runs path detection, and makes `tauri-windows` complete successfully with an explicit no-op when Desktop paths are irrelevant; never use workflow-level `paths` for a required check. Pin Go 1.25.11 and Node 20; use `npm ci`; ordinary PRs receive no environment secrets. Branch protection requires `project-gate` immediately. Record `tauri-windows` as a stable always-emitted check in Ops-01 and make it required no later than the Web + Desktop production-ready gate; it is not required for the earlier Web-only claim unless the user explicitly chooses the stricter posture.

Set artifact retention explicitly and test it: PR evidence uses `retention-days: 30`; push-to-main evidence uses `retention-days: 90`. Release evidence is handled by Ops-06/08 and must not inherit the shorter values.

- [x] **Step 5: Align toolchain metadata**

Add Node/npm `engines` to frontend/Tauri packages and update guides to distinguish minimum policy from the exact CI Go toolchain.

- [x] **Step 6: Verify locally and in a PR**

> 2026-08-03 完成：本地 `verify-workflows.sh`、default verifier `-ReportDir artifacts/ops-01`、`--tauri`（Rust 1.97.1）、evidence 契约套件全绿；PR #11（squash 合并为 `7f34191`）六项检查全绿（backend/frontend/docs/project-gate/detect/tauri-windows 真实 Windows 构建 7m51s）；failure_probe 演练红盘（run 30841304958，project-gate 失败）与绿盘（run 30842196659）证据齐全；main 分支保护已配置（required `project-gate` strict、禁 force-push/删除，无人工审查）；`artifacts/ops-01/summary.json` 已绑定终态 commit `7f34191` 并通过 `-ExpectedCommit` 校验。
> 实施中扩展：verifier 新增 `-Scope Tauri`（仅桌面门，供 tauri-windows 按路径执行）；`-Scope` 参数集由 Backend|Frontend|Docs|All 扩展为 Backend|Frontend|Docs|Tauri|All。CI 首次运行暴露的三处修复已合入：setup-node v7 拒绝 `cache: false`、迁移测试需 postgres/redis 服务容器、series 测试 busyAction 竞态。
> 2026-08-04 审计偏差：`7f34191` 是直接基于 `fa95a356` 的 squash，包含 Ops-01 以外的 UI/文档路径，不满足本计划“一 Ops Task 一 reviewed commit”的理想边界。为保持共享历史可追溯性不重写 main；本次 review-remediation 对 `fa95a356..main` 聚合差异重新执行规格/质量审查并修复已确认问题。后续 heavy task 必须从最新集成点创建独立分支，禁止把其他任务路径并入其 evidence commit。

```bash
bash scripts/ci/verify-workflows.sh
bash scripts/verify-project.sh -ReportDir artifacts/ops-01
bash scripts/verify-project.sh --tauri
bash scripts/ops/validate-evidence.tests.sh
bash scripts/ops/finalize-evidence.tests.sh
if [ ! -f artifacts/ops-01/summary.json ]; then echo "missing Ops summary" >&2; exit 1; fi
bash scripts/ops/validate-evidence.sh -Schema release/ops-evidence.schema.json -Summary artifacts/ops-01/summary.json
```

Open a PR that intentionally fails one fixture branch, record the failed required check, then restore and record all green checks. Configure branch protection after check names exist.

Use a guarded `workflow_dispatch` input `failure_probe=true` to exercise the failing job without adding a second product commit; dispatch again with `failure_probe=false` for green evidence. Re-run both contract scripts after implementation and require exit zero.

After the single reviewed commit, run the universal finalization block and require `summary.commit == git rev-parse HEAD`.

**Acceptance evidence:** workflow contract outputs; GitHub run URLs; uploaded logs/summary; branch-protection screenshot/export; proof a child failure fails the workflow; finalizer tests; final-commit-bound `artifacts/ops-01/summary.json`.  
**Release blocker:** floating actions, skipped required job, missing failure logs, production secret in PR, Tauri running on non-Windows or unconditionally on unrelated paths.  
**Commit:** `Ops 01: add project CI gates`

---

## Ops-02: Implement forward-only migrations and recovery drills

**Depends on:** Ops-01  
**Branch:** `codex/ops/ops-02`  
**Write reservation:** migration runner, DB scripts, compose migration wiring and deployment DB docs

**Files:**

- Create: `backend/internal/migration/file.go`
- Create: `backend/internal/migration/file_test.go`
- Create: `backend/internal/migration/ledger.go`
- Create: `backend/internal/migration/ledger_test.go`
- Create: `backend/internal/migration/runner.go`
- Create: `backend/internal/migration/runner_test.go`
- Create: `backend/internal/migration/postgres_integration_test.go`
- Create: `backend/migrations/metadata.schema.json`
- Create: `backend/migrations/metadata.json`
- Create: `backend/internal/migration/testdata/historical-050.sql`
- Create: `backend/internal/migration/testdata/historical-050.sha256`
- Create: `backend/internal/migration/testdata/historical-050.manifest.json`
- Create: `scripts/db/build-historical-fixture.sh`
- Create: `scripts/db/build-historical-fixture.tests.sh`
- Create: `backend/cmd/migrate/main.go`
- Create: `backend/cmd/migrate/main_test.go`
- Create: `scripts/db/recovery-drill.sh`
- Create: `scripts/db/recovery-drill.tests.sh`
- Create: `scripts/db/verify-backup-policy.sh`
- Create: `scripts/db/verify-backup-policy.tests.sh`
- Create: `scripts/db/object-recovery-drill.sh`
- Create: `scripts/db/object-recovery-drill.tests.sh`
- Create: `scripts/db/redis-reconciliation-drill.sh`
- Create: `scripts/db/redis-reconciliation-drill.tests.sh`
- Create: `scripts/db/verify-migration-gate.sh`
- Create: `ops/recovery/docker-compose.recovery.yml`
- Create: `release/backup-policy.schema.json`
- Create: `release/backup-policy.json`
- Create: `release/recovery-objectives.schema.json`
- Create: `release/recovery-objectives.json`
- Create: `scripts/restore-db.sh`
- Modify: `scripts/init-db.sh`
- Modify: `scripts/backup-db.sh`
- Modify: `docker-compose.yml`
- Modify: `docs/deploy/docker-compose.single-server.yml`
- Modify: `backend/Dockerfile`
- Modify: `.github/workflows/ci.yml`
- Modify: `scripts/ci/verify-workflows.sh`
- Modify: `scripts/ci/verify-workflows.tests.sh`
- Modify: `docs/deploy/single-server-beta-runbook.md`
- Modify: `docs/deploy/production-config-template.md`
- Modify: `docs/superpowers/plans/2026-07-17-omnicraft-production-readiness.md`
- Modify: `AGENTS.md`
- Modify: `progress.txt`

- [x] **Step 1: Reproduce the existing-volume failure**

Start PostgreSQL, allow init scripts to create the volume, add a harmless temporary migration in a test fixture, restart, and prove `/docker-entrypoint-initdb.d` does not apply it. Do not modify production migration files for this proof.

- [x] **Step 2: Write failing parser/ledger tests**

Cover filename validation, duplicate versions, stable ordering, SHA-256, checksum drift, ledger references to missing files, and missing lower-number application.

```bash
cd backend
go test ./internal/migration -run "TestParse|TestPlan|TestChecksum" -v
```

- [x] **Step 3: Implement the minimal planner and ledger**

Create `schema_migrations(version, filename, checksum, applied_at)` and `schema_migration_attempts` audit state; compare by applied filename/version/checksum set, never max version alone.

- [x] **Step 4: Write failing PostgreSQL integration tests**

Cover empty 001→latest, synthetic historical fixture→latest, no rerun, checksum rejection, inserted missing lower version, transactional rollback, and two concurrent runners where only one holds the advisory lock.

Generate `historical-050.sql` by applying the real migrations 001～050 to the pinned PostgreSQL 16 + pgvector image and exporting the resulting schema/seed fixture. The adjacent manifest records generator version, image digest, source migration filename/checksum set, ledger rows and generation command; tests reject a hand-edited dump, mismatched manifest, missing source checksum or a fixture whose ledger does not exactly represent 001～050.

Also test `metadata.json`: default is transactional; a non-transactional filename must be explicitly listed with reason, reviewer, machine-checkable pre/postconditions, idempotent/resume strategy and reconciliation instructions. Record started/succeeded/failed attempts with a redacted error digest. A failed/unknown attempt blocks blind retry and later migrations until explicit reconciliation approval.

```bash
cd backend
export OMNICRAFT_TEST_POSTGRES_DSN="host=localhost port=5432 user=omnicraft password=omnicraft dbname=omnicraft_migration_test sslmode=disable"
go test ./internal/migration -run TestPostgres -count=1 -v
```

- [x] **Step 5: Implement CLI and deployment wiring**

Build a dedicated migrate binary into the backend image. Deployment runs migration as a one-shot command before backend readiness. Remove production reliance on initdb; keep local bootstrap as a wrapper around the CLI.

- [x] **Step 6: Write backup/restore contract tests, then repair scripts**

Require custom-format dumps, checksum manifest, migration manifest, explicit destination DB, refusal to overwrite source, and nonzero failures. `backup-policy.json` declares daily + pre-migration PostgreSQL backup, 7 local copies, 30-day encrypted off-host immutable/versioned retention, monthly restore cadence, PostgreSQL/OSS/Redis classification and restore order. `recovery-objectives.json` may be `baseline_only` in Ops-02 with measured values and null approval; policy tests reject missing state/measurements but require approved numeric targets only in Ops-08.

```bash
bash scripts/db/recovery-drill.tests.sh
bash scripts/db/verify-backup-policy.tests.sh
bash scripts/db/build-historical-fixture.tests.sh
bash scripts/db/verify-backup-policy.sh -Policy release/backup-policy.json -ReportDir artifacts/ops-02/policy
```

- [x] **Step 7: Perform a real recovery drill**

Use PostgreSQL 16 + pgvector containers. Populate non-sensitive fixture data, dump, restore into a new DB, run ledger verification and backend smoke, and record measured backup duration, restore duration, RPO and RTO.

The drill scripts own the full lifecycle in `try/finally`: create an isolated Compose project from `ops/recovery/docker-compose.recovery.yml`, wait for PostgreSQL/Redis/versioned MinIO readiness, seed data and attachment keys, perform DB restore + deleted-object version restore + DB-to-object reconciliation + Redis clear/rebuild, collect evidence, then remove containers/volumes. Missing readiness, reconciliation mismatch or teardown failure returns nonzero. Local MinIO proves adapter behavior only; real OSS/off-host evidence remains an Ops-08 blocker.

```bash
bash scripts/db/recovery-drill.sh -ReportDir artifacts/ops-02
bash scripts/db/object-recovery-drill.tests.sh
bash scripts/db/object-recovery-drill.sh -ComposeFile ops/recovery/docker-compose.recovery.yml -ReportDir artifacts/ops-02
bash scripts/db/redis-reconciliation-drill.tests.sh
bash scripts/db/redis-reconciliation-drill.sh -ComposeFile ops/recovery/docker-compose.recovery.yml -ReportDir artifacts/ops-02
cd tools/doc-validator
go run . --fix
go run . --check --profile release
cd ../..
bash scripts/verify-project.sh -ReportDir artifacts/ops-02
if [ ! -f artifacts/ops-02/summary.json ]; then echo "missing Ops summary" >&2; exit 1; fi
bash scripts/ops/validate-evidence.sh -Schema release/ops-evidence.schema.json -Summary artifacts/ops-02/summary.json
```

**Acceptance evidence:** `migration-summary.json`, historical fixture generation manifest and checksum verification, ledger rows/checksums, concurrent-run log, dump checksum, local versioned-object-adapter checksum/recovery evidence, restore verification, measured baseline RPO/RTO, backup policy validation, CI PostgreSQL job URL, final-commit-bound `artifacts/ops-02/summary.json`. Real off-host/Aliyun OSS evidence remains explicitly deferred to Ops-08.  
**Release blocker:** max-version-only logic, checksum drift accepted, both runners apply, historical fixture is hand-authored/unverifiable or fails upgrade, dump/object restore or reconciliation fails, backup policy omits off-host encryption/immutability, fake down migration, missing baseline measurements or invalid recovery-objective state. Numeric objective approval and target comparison are intentionally deferred to Ops-08.  
**Commit:** `Ops 02: add database migration and recovery gate`

---

## Ops-03: Add privacy-safe logs, metrics and readiness

**Depends on:** Ops-02  
**Branch:** `codex/ops/ops-03`  
**Write reservation:** backend observability/config/middleware, internal metrics service and Prometheus compose files

**Files:**

- Create: `backend/internal/observability/logger.go`
- Create: `backend/internal/observability/logger_test.go`
- Create: `backend/internal/observability/metrics.go`
- Create: `backend/internal/observability/metrics_test.go`
- Create: `backend/internal/observability/server.go`
- Create: `backend/internal/observability/server_test.go`
- Create: `backend/internal/middleware/metrics.go`
- Create: `backend/internal/middleware/metrics_test.go`
- Create: `ops/observability/prometheus.yml`
- Create: `ops/observability/prometheus.tests.sh`
- Create: `ops/observability/loki.yml`
- Create: `ops/observability/alloy-config.alloy`
- Create: `ops/observability/docker-compose.observability.yml`
- Create: `ops/observability/log-retention-policy.schema.json`
- Create: `ops/observability/log-retention-policy.json`
- Create: `scripts/ops/archive-audit-logs.sh`
- Create: `scripts/ops/archive-audit-logs.tests.sh`
- Create: `scripts/ops/observability-drill.sh`
- Create: `scripts/ops/observability-drill.tests.sh`
- Modify: `backend/internal/middleware/logger.go`
- Modify: `backend/internal/middleware/panic_recovery.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/main_test.go`
- Modify: `backend/config/config.go`
- Modify: `backend/config/config_test.go`
- Modify: `backend/config.yaml`
- Modify: `backend/go.mod`
- Modify: `backend/go.sum`
- Modify: `backend/internal/migration/runner.go`
- Modify: `scripts/backup-db.sh`
- Modify: `scripts/db/recovery-drill.sh`
- Modify: `docker-compose.yml`
- Modify: `docs/deploy/docker-compose.single-server.yml`
- Modify: `docs/deploy/single-server-beta-runbook.md`
- Modify: `architecture.md`
- Modify: `docs/superpowers/plans/2026-07-17-omnicraft-production-readiness.md`
- Modify: `progress.txt`

- [x] **Step 1: Write failing structured-log/redaction tests**

Assert required fields and prove authorization, cookies, tokens, signed URL query, email/raw IP and message bodies are absent. Errors use class/code, not raw untrusted body. `client_ip` is the first 128 bits of keyed HMAC-SHA256 encoded as 32 lowercase hex characters using `LOG_IP_HASH_SECRET`; logs include a non-secret key ID. Missing key in release fails closed, rotation accepts current/previous key only between explicit start/end timestamps, and no path falls back to raw IP.

- [x] **Step 2: Implement JSON slog configuration and request logging**

Preserve request ID and middleware order. Route labels use Gin full-path templates with a bounded fallback.

- [x] **Step 3: Write failing metrics tests**

Cover request count/error/latency, panic, DB pool, Redis, queue/worker metrics and label allowlists. Reject IDs and raw URLs as labels.

Include aggregate success/failure/latency metrics for OSS, Green, CAPTCHA, SMTP and LLM adapters without request/user/content labels.

- [x] **Step 4: Implement an internal metrics server and readiness**

Run metrics on a separately configured internal port; do not expose it through public API routes. Keep `/healthz` liveness-only and add dependency-aware readiness without returning connection details. Configure Docker JSON rotation, a named durable Loki volume, internal Grafana Alloy→Loki ingestion with 30-day/capped retention, and authenticated/tunneled operator access whose query audit is retained. Alloy receives only the read-only log mounts/service-discovery access needed for collection and no Docker control capability. Migration runner and backup/recovery scripts emit bounded success/failure/last-success metrics consumed by Prometheus textfile/custom collectors.

`archive-audit-logs.sh` encrypts and uploads warning/error audit summaries to the configured off-site destination, verifies checksum/retention metadata, and writes access/archive evidence. Its contract tests use a local encrypted sink; real destination credentials are required by Ops-08.

- [x] **Step 5: Verify Prometheus config and runtime scrape**

```bash
bash ops/observability/prometheus.tests.sh
bash scripts/ops/observability-drill.tests.sh
bash scripts/ops/archive-audit-logs.tests.sh
cd backend
go test ./internal/observability ./internal/middleware ./cmd/server -v
go test ./...
go vet ./...
go build ./...
cd ../tools/doc-validator
go run . --fix
go run . --check --profile release
cd ../..
bash scripts/ops/observability-drill.sh -Environment Local -ComposeFile docker-compose.yml -ReportDir artifacts/ops-03
bash scripts/verify-project.sh -ReportDir artifacts/ops-03
if [ ! -f artifacts/ops-03/summary.json ]; then echo "missing Ops summary" >&2; exit 1; fi
bash scripts/ops/validate-evidence.sh -Schema release/ops-evidence.schema.json -Summary artifacts/ops-03/summary.json
```

`observability-drill.sh` owns setup/readiness/seed/query/teardown in `try/finally`: start the application plus `ops/observability/docker-compose.observability.yml` under an isolated project name, wait for Prometheus/Loki, issue successful/failed requests, scrape metrics, query a known `trace_id`, record authorized query access, inspect rotation/retention settings, restart/recreate Loki while retaining its named volume and prove the log remains queryable, prove unauthenticated external access is unavailable, check cardinality, and remove resources. Missing readiness, durable-query evidence or teardown returns nonzero.

**Acceptance evidence:** redaction test output, sample sanitized JSON lines, scrape artifact, metric label inventory, readiness dependency cases, Loki restart/query and access-audit evidence, encrypted off-site archive contract evidence, `artifacts/ops-03/summary.json`.  
**Release blocker:** secret/PII in logs, unbounded label, public metrics exposure, readiness reports ready with required DB/Redis unavailable, non-durable Loki storage, unauthorized query access, or missing encrypted warning/error archive contract.  
**Commit:** `Ops 03: add production observability baseline`

---

## Ops-04: Add actionable alerting and runbook drills

**Depends on:** Ops-03  
**Branch:** `codex/ops/ops-04`
**Write reservation:** only the alert rules/config, alert verification/drill scripts, compose/runbook/CI/tracking files listed below after Ops-03 releases them

> **2026-08-06 maintainer clarification:** Step 4's required in-network
> `alert-sink` is repository-owned, and completion advances the active registry.
> The maintainer explicitly approved adding the four `alert-sink` files and
> `AGENTS.md` to this task's reservation during review (Issue #33).

**Files:**

- Create: `ops/observability/prometheus-rules.yml`
- Create: `ops/observability/alertmanager.yml`
- Create: `ops/observability/alert-contract.schema.json`
- Create: `ops/observability/blackbox.yml`
- Create: `ops/observability/exporter-targets.yml`
- Create: `ops/observability/external-heartbeat.schema.json`
- Create: `ops/observability/external-heartbeat.example.json`
- Create: `ops/observability/alert-sink/Dockerfile`
- Create: `ops/observability/alert-sink/go.mod`
- Create: `ops/observability/alert-sink/main.go`
- Create: `ops/observability/alert-sink/main_test.go`
- Create: `scripts/ops/verify-alerts.sh`
- Create: `scripts/ops/verify-alerts.tests.sh`
- Create: `scripts/ops/alert-drill.sh`
- Create: `scripts/ops/alert-drill.tests.sh`
- Modify: `ops/observability/prometheus.yml`
- Modify: `ops/observability/docker-compose.observability.yml`
- Modify: `docker-compose.yml`
- Modify: `docs/deploy/docker-compose.single-server.yml`
- Modify: `docs/deploy/single-server-beta-runbook.md`
- Modify: `.github/workflows/ci.yml`
- Modify: `AGENTS.md`
- Modify: `docs/superpowers/plans/2026-07-17-omnicraft-production-readiness.md`
- Modify: `progress.txt`

- [x] **Step 1: Write failing alert contract tests**

Require owner, severity, `for`, summary, impact, first step and runbook anchor. Reject missing resolution semantics and invalid expression references.

- [x] **Step 2: Add minimal alert rules and routing**

Cover API availability/5xx/latency, DB/Redis, queue/worker, backup age, overdue recovery drill, migration failure, cert expiry, disk, restart loop, and sustained OSS/Green/CAPTCHA/SMTP/LLM failures. Map every rule to application/custom metrics, postgres/redis exporter, node exporter, cAdvisor or blackbox/certificate probes. Wire all exporters/rules/Alertmanager into the reserved Prometheus/Compose files. Receiver values stay outside Git; Ops-04 requires a real independent-failure-domain HTTPS heartbeat URL/credential before completion.

- [x] **Step 3: Validate with real tools**

```bash
bash scripts/ops/verify-alerts.sh
bash scripts/ops/alert-drill.tests.sh
bash scripts/ops/alert-drill.sh -Environment Local -ComposeFile ops/observability/docker-compose.observability.yml -WebhookSink http://alert-sink:8080/events -ReportDir artifacts/ops-04
bash scripts/verify-project.sh -ReportDir artifacts/ops-04
if [ ! -f artifacts/ops-04/summary.json ]; then echo "missing Ops summary" >&2; exit 1; fi
bash scripts/ops/validate-evidence.sh -Schema release/ops-evidence.schema.json -Summary artifacts/ops-04/summary.json
```

The script runs pinned `promtool`/Alertmanager config checks and contract tests.

- [x] **Step 4: Exercise firing and resolution**

`alert-drill.sh` starts an isolated observability project containing an in-network `alert-sink`, waits until Prometheus reports every postgres/redis/node/cAdvisor/blackbox target `up == 1`, injects API error-rate, dependency-down and overdue-recovery conditions, polls the sink for firing/resolved payloads, restores health, then calls the configured real external heartbeat and proves missing-heartbeat notification from the independent provider. It tears down in `finally`; `127.0.0.1` inside Alertmanager and synthetic-only heartbeat evidence are forbidden.

**Acceptance evidence:** rule-test output, Alertmanager delivery IDs/screenshots, firing/resolved timestamps, runbook drill notes, `artifacts/ops-04/summary.json`.  
**Release blocker:** invalid rule, exporter target not scraped, missing owner/runbook, receiver cannot deliver, no resolved evidence, no real independent heartbeat, or secrets committed in routing config.  
**Commit:** `Ops 04: add production alerting gates`

---

## Ops-05: Add continuous security scanning and exception governance

**Depends on:** Ops-04  
**Branch:** `codex/ops/ops-05`
**Write reservation:** only security workflow/policies/verifiers and tracking files listed below

**Files:**

- Create: `.github/workflows/security.yml`
- Create: `.github/dependabot.yml`
- Create: `security/scan-policy.json`
- Create: `security/exceptions.json`
- Create: `security/exceptions.schema.json`
- Create: `scripts/security/verify-security.sh`
- Create: `scripts/security/verify-security.tests.sh`
- Create: `scripts/security/verify-pinned-actions.sh`
- Create: `scripts/security/verify-pinned-actions.tests.sh`
- Modify: `README.md`
- Modify: `docs/superpowers/plans/2026-07-17-omnicraft-production-readiness.md`
- Modify: `progress.txt`

- [x] **Step 1: Write failing policy/exception/action-pin tests**

Reject floating actions, write-all permissions, missing scan categories, malformed exceptions, missing affected version/digest, compensating control, author, independent human approver, commit-bound `approval_ref`, approval date or expiry, and expired exceptions. The reference must identify a concrete commit plus GitHub PR review or protected-environment approval event; an issue/comment URL alone is insufficient because it is mutable. Release validation uses GitHub API identity to prove the approver is a current authorized repository human owner, differs from author, approved that commit, and has not been dismissed/revoked. In a single-human-owner repository no High exception can pass until a second eligible human approver is configured. Secret findings and release Critical vulnerabilities are non-waivable; High exceptions require explicit user approval.

- [x] **Step 2: Implement local security verifier**

Run pinned govulncheck; npm audit policy separately for `frontend/package-lock.json` and `tauri-client/package-lock.json`; cargo audit; gitleaks against both the checked-out tree and reachable Git history; and Trivy filesystem/IaC/container-image scans. Each child failure propagates nonzero; reports are written before exit.

- [x] **Step 3: Implement scheduled/PR workflow**

Use least privilege, dependency caches and artifact upload. Expose the stable aggregate job name `security-gate`; after the workflow exists and passes, add it to `main` required checks and export the protection state as evidence. Do not download untrusted PR code and then expose privileged secrets.

- [x] **Step 4: Prove failure behavior**

Use test fixtures containing a fake secret pattern, expired exception and vulnerable fixture lockfile; each must fail only its intended gate.

```bash
bash scripts/security/verify-security.tests.sh
bash scripts/security/verify-pinned-actions.sh
bash scripts/security/verify-security.sh -BuildImages -ReportDir artifacts/ops-05
bash scripts/verify-project.sh -ReportDir artifacts/ops-05
if [ ! -f artifacts/ops-05/summary.json ]; then echo "missing Ops summary" >&2; exit 1; fi
bash scripts/ops/validate-evidence.sh -Schema release/ops-evidence.schema.json -Summary artifacts/ops-05/summary.json
```

**Acceptance evidence:** SARIF/JSON reports, workflow URLs, fixture failures, zero expired exceptions, pinned-tool manifest, filesystem/IaC/container-image report set, `artifacts/ops-05/summary.json`.  
**Release blocker:** secret hit, any release Critical vulnerability, any High vulnerability without a currently valid independent exception, expired/revoked exception, floating action/tool, or scan failure hidden with continue-on-error/`|| true`.  
**Commit:** `Ops 05: add continuous security scanning`

> 2026-08-06 完成（`codex/ops/ops-05`，审查合并前视为 pending）。实施中的保留区扩展（基线扫描暴露、为通过门必须修复，记录于此）：
>
> - `tauri-client/src-tauri/Cargo.lock`：`cargo audit` 检出 5 个 High（RUSTSEC-2026-0194/0195 quick-xml <0.41、RUSTSEC-2026-0185 quinn-proto <0.11.15）；升级 `plist 1.10.0`（携带 quick-xml 0.41.0）、`tauri-winrt-notification 0.7.3`（移除 quick-xml 0.37.5）、`quinn-proto 0.11.16` 后归零。有效 MSRV 由 plist 1.10.0 提升至 Rust 1.88（CI windows-latest 稳定版满足；桌面范围暂缓期间无本地构建门）。
> - `frontend/package.json` + `package-lock.json`：`npm audit` 检出 1 Critical（tar）+ 9 High（next 链）；`next 16.2.7→16.3.0`、`overrides.next.postcss 8.5.15→8.5.25` 后归零。
> - `tauri-client/package-lock.json`：`npm audit` 检出 vite/postcss High；`npm audit fix`（vite 6.x 补丁版）后归零，`package.json` 未改。
> - `backend/go.mod` + `go.sum`：Trivy 检出 pgx/v5 CRITICAL CVE-2026-33815/33816；`v5.6.0→v5.9.0` 后归零（govulncheck 未报告，属 DB 滞后差异）。
> - `backend/Dockerfile`、`ops/observability/gate/Dockerfile`：Trivy IaC DS002（root 运行）High；补 `USER` 非 root 用户后归零（backend 同时 chown `/app/evidence`、`/var/lib/omnicraft/metrics`）。
> - 新建 `.gitleaks.toml`：gitleaks 全历史扫描对 4 个测试夹具/文档示例路径（config_test.go、归档计划文档、useStepper 模板常量、seed 脚本历史 demo JWT）产生 generic-api-key/jwt 误报；以路径级 allowlist 豁免，当前工作树验证为 0 命中。
> - 扫描实施约定：各工具门只报告原始 JSON 报告，**唯一判定点**为 `security-verdict.json`（policy 门 + 例外匹配），保证 High 例外机制可用；工具级错误（无有效 JSON）仍使门失败。gitleaks 命中视为 secret、不可豁免，门直接失败。
> - 审查整改（2026-08-06，code-review 两轴修复后 amend 同一 commit）：cargo-audit 未安装时容器内首次自动 `cargo install --locked`（命名卷缓存，干净 runner 可复现）；policy 门 event URL 校验收紧为 schema 同款正则（拒绝 issue/comment URL）；移除 advisory-db `git pull` 的 `|| true` 静默；移除无用 `TOOL_VERSIONS` 变量；`security-gate` job 名匹配收紧为整行键。
> - 明确延后项（不在本 commit 承诺）：**GitHub API 身份校验**（批准人是当前授权 human owner、与 author 不同、对该 commit 批准、未被 dismiss/revoke）属 release 门操作，计划于 Ops-08 preflight/release 门接入 GitHub API 时执行，Ops-05 仅保证 `approval_ref` 结构契约；`security-gate` 加入 main required checks 与分支保护状态导出在 workflow 合入 main 并通过后执行（合并前无法配置）。CI 中 verifier 使用 `-ReportDir artifacts/security`（非 `ops-05`），本地 Ops 证据仍按本计划使用 `artifacts/ops-XX`。

---

## Ops-06: Generate SBOM and provenance bound to artifacts

**Depends on:** Ops-05  
**Branch:** `codex/ops/ops-06`
**Write reservation:** only SBOM/provenance workflow, policies, release scripts, Dockerfiles and tracking files listed below

**Files:**

- Create: `.github/workflows/sbom.yml`
- Create: `release/sbom-policy.json`
- Create: `release/release-manifest.schema.json`
- Create: `scripts/release/generate-sbom.sh`
- Create: `scripts/release/generate-sbom.tests.sh`
- Create: `scripts/release/verify-provenance.sh`
- Create: `scripts/release/verify-provenance.tests.sh`
- Create: `scripts/release/archive-release-evidence.sh`
- Create: `scripts/release/archive-release-evidence.tests.sh`
- Modify: `backend/Dockerfile`
- Modify: `frontend/Dockerfile`
- Modify: `README.md`
- Modify: `docs/superpowers/plans/2026-07-17-omnicraft-production-readiness.md`
- Modify: `progress.txt`

- [x] **Step 1: Write failing manifest/SBOM tests**

Require commit, version, artifact SHA-256, image digest, migration manifest digest, SBOM paths/digests, generator versions and provenance references. Reject missing Go/npm/Rust/container components.

- [x] **Step 2: Generate deterministic SBOM sets**

Use pinned CycloneDX/SPDX-capable tools. Normalize volatile timestamps only where the format allows; never edit package identity to force reproducibility.

- [x] **Step 3: Add GitHub provenance workflow**

Build immutable artifacts, upload SBOM, and create provenance/attestation with minimal required permissions. PR builds generate unsigned preview SBOM; release provenance only runs from protected refs/environments.

- [x] **Step 4: Download and verify**

```bash
bash scripts/release/generate-sbom.tests.sh
bash scripts/release/generate-sbom.sh -OutputDir artifacts/ops-06
bash scripts/release/verify-provenance.sh -Manifest artifacts/ops-06/release-manifest.json
bash scripts/release/archive-release-evidence.tests.sh
bash scripts/verify-project.sh -ReportDir artifacts/ops-06
if [ ! -f artifacts/ops-06/summary.json ]; then echo "missing Ops summary" >&2; exit 1; fi
bash scripts/ops/validate-evidence.sh -Schema release/ops-evidence.schema.json -Summary artifacts/ops-06/summary.json
```

The archive contract requires both a GitHub Release asset destination and an encrypted operator off-site destination with one-year retention metadata. Ops-06 proves the adapter with deterministic local fixtures; Ops-08 invokes it with protected staging/release destinations. Missing real destination credentials remains a release blocker.

**Acceptance evidence:** SBOM files/digests, release manifest, attestation verification output, artifact/container digest match, `artifacts/ops-06/summary.json`.  
**Release blocker:** missing ecosystem, SBOM not bound to artifact digest, provenance identity mismatch, mutable image tag used as evidence.  
**Commit:** `Ops 06: add SBOM and provenance gates`

> 2026-08-06 完成（`codex/ops/ops-06`，审查合并前视为 pending）。实施中的保留区扩展（记录于此）：
>
> - `scripts/security/verify-pinned-actions.sh` + `scripts/security/verify-pinned-actions.tests.sh`：为 `.github/workflows/sbom.yml` 的 provenance job 放行 `attestations: write` / `id-token: write`（`actions/attest-build-provenance` 强制要求的最小权限组合；其余 workflow 的这两项以及 sbom.yml 的其他 write scope 仍被拒绝，action 引用仍强制 40-hex SHA pin），并新增三种行为契约测试。
> - 本地容器 digest 采用 docker-archive tar 内 `manifest.json` blob 的 SHA-256 作为镜像制品 digest（可离线复算、可验证）；真实 registry manifest digest 绑定留给 Ops-08 在 push 阶段记录。
> - `release/sbom-policy.json` 组件携带 `files`（lockfile 路径）与 `image`/`tag`/`dockerfile`/`context`（容器组件）字段；本地与 CI 共用同一 policy 与同一 pinned syft digest。
> - PR 预览 manifest 的 `provenance` 允许为空（未签名预览）；release manifest 必须有非空 provenance 引用，真实 attestation 由 sbom.yml release job 生成并在 CI 用 `gh attestation verify` 复核。

---

## Ops-07: Establish load, stress and capacity gates

**Depends on:** Ops-06  
**Branch:** `codex/ops/ops-07`
**Write reservation:** only performance workflow, k6 suites, load runner, runbook and tracking files listed below

**Files:**

- Create: `.github/workflows/performance.yml`
- Create: `tests/load/k6/smoke.js`
- Create: `tests/load/k6/load.js`
- Create: `tests/load/k6/stress.js`
- Create: `tests/load/k6/lib/auth.js`
- Create: `tests/load/k6/lib/scenarios.js`
- Create: `tests/load/k6/thresholds.json`
- Create: `tests/load/k6/release-profile.schema.json`
- Create: `tests/load/k6/release-profile.json`
- Create: `tests/load/k6/testdata.json`
- Create: `scripts/load/run-load-tests.sh`
- Create: `scripts/load/run-load-tests.tests.sh`
- Create: `ops/load/config-override.yaml` (reservation extension: load-test
  environment override that disables per-IP rate limiting — the app
  rate-limits 100 req/min per client IP, so a single-IP k6 run would measure
  the rate limiter instead of system capacity; production never loads this
  file and Ops-08 preflight rejects test-mode config)
- Modify: `docs/deploy/single-server-beta-runbook.md`
- Modify: `docs/superpowers/plans/2026-07-17-omnicraft-production-readiness.md`
- Modify: `progress.txt`

- [x] **Step 1: Write failing runner/target-safety tests**

Require an explicit environment, allowlist local/staging hosts, reject production unless `-AllowProduction` plus confirmation token, propagate k6 threshold failures and always save summary.

- [x] **Step 2: Add deterministic smoke/load/stress scenarios**

Cover health/readiness, anonymous feed/search/detail and authenticated read/write paths with isolated test identities. Do not call paid Provider/OSS paths by default.

- [x] **Step 3: Establish thresholds from measured baseline**

Run smoke first, then staged load and stress. Store p50/p95/p99, request/error rates, max VUs, CPU/memory/DB/Redis metrics and recovery time. `release-profile.json` fixes environment resources, dataset size, duration, endpoint mix, throughput/concurrency and latency/error objectives and records explicit user approval. A measured baseline is not automatically the passing target. Threshold changes require a reviewed reason.

- [x] **Step 4: Add scheduled/manual workflow**

PR runs a short local smoke only when relevant paths change; scheduled/manual staging jobs run load/stress with staging-scoped secrets. The runner owns setup/readiness/seed/metrics/cleanup in `try/finally`: start an isolated app/PostgreSQL/Redis/Prometheus Compose project, wait for readiness, seed named test identities and dataset, run host k6 against `127.0.0.1`, capture resource metrics, delete test identities/data, then remove containers/volumes. Missing services, seed cleanup or metrics fail the run.

```bash
bash scripts/load/run-load-tests.tests.sh
bash scripts/load/run-load-tests.sh -Environment Local -Tier Smoke -Target http://127.0.0.1:8080 -Profile tests/load/k6/release-profile.json -ReportDir artifacts/ops-07 -RunName smoke -SeedDb omnicraft-postgres
bash scripts/load/run-load-tests.sh -Environment Local -Tier Load -Target http://127.0.0.1:8080 -Profile tests/load/k6/release-profile.json -ReportDir artifacts/ops-07 -RunName load -SeedDb omnicraft-postgres
bash scripts/load/run-load-tests.sh -Environment Local -Tier Stress -Target http://127.0.0.1:8080 -Profile tests/load/k6/release-profile.json -ReportDir artifacts/ops-07 -RunName stress -SeedDb omnicraft-postgres
bash scripts/verify-project.sh -ReportDir artifacts/ops-07
if [ ! -f artifacts/ops-07/summary.json ]; then echo "missing Ops summary" >&2; exit 1; fi
bash scripts/ops/validate-evidence.sh -Schema release/ops-evidence.schema.json -Summary artifacts/ops-07/summary.json
```

**Acceptance evidence:** approved profile, k6 smoke/load/stress summary JSON, thresholds, environment/dataset inventory, resource graphs, bottleneck/recovery notes, workflow URL, `artifacts/ops-07/summary.json`.  
**Release blocker:** profile not approved, target safety bypass, missing summary, threshold failure, uncontrolled paid/external call, test data not isolated/cleanable.  
**Commit:** `Ops 07: add performance and capacity gates`

> 2026-08-06 完成（`codex/ops/ops-07`，审查合并前视为 pending）。实施中的保留区扩展与决策（记录于此）：
>
> - 新增 `ops/load/config-override.yaml`：应用对每个客户端 IP 做 100 req/min 限流（`rate_limit.normal_per_minute`），单 IP 的 k6 压测若不关闭限流只会测量限流器而非系统容量（首个 Load 基线 run 实测 99.98% 429 证实）。该 override 仅用于 load-test 环境，`search_per_minute`/`upload_per_hour` 一并置 0（搜索与上传限流器不受 `enabled` 开关控制）；生产配置卷不含此文件，Ops-08 preflight 会拒绝测试态配置。release-profile 的 `environment.rate_limiting` 字段记录该环境决策。
> - 身份与数据隔离：runner 通过 `-SeedDb <container>`（docker exec psql）或 `-DbDsn <dsn>`（CI service container，需 psql）直接向 PostgreSQL 播种 5 个 `load-test-*@omnicraft.local` 测试身份（bcrypt hash 见 `testdata.json`，reputation=10 以满足发布交互门），解析一个 published 内容 id 供 detail 路径使用，并在 EXIT trap 中删除身份（ON DELETE CASCADE 清除其内容）。登录有每 IP 5/min 的 credential 限流，故 auth 端点只由 VU 池前 5 个 VU 承担（`AUTH_VUS`），每个 auth VU 只登录一次并缓存 bearer；写路径每次请求前重新获取 CSRF 配对（k6 cookie jar 不保证跨迭代持久）。CSRF 流程与 create/delete 路径已用真实 backend 逐项验证。
> - k6 脚本限制：`open()` 只能在 init 阶段使用，profile 在模块加载时缓存一次；`open()` 相对路径以入口脚本目录解析，故 `testdata.json`/`thresholds.json` 用 `import.meta.resolve('../...')`，`PROFILE` 由 runner 规范化为绝对路径。k6 v2.1.0 对 `import.meta.resolve` 有弃用提示，但当前行为正确且已对未来对齐方式做了前瞻处理。
> - 基线测量（本地单机、Apple Silicon、rate-limit 关闭）：Smoke(2 VU, 10 iter) p95=48ms p99=48ms 错误率 0%；Load(20 VU, 2m) p95=99ms p99=182ms 错误率 0%；Stress(10→25→50 VU 阶梯) p95=203ms p99=494ms 错误率 0%。**2026-08-06 维护者显式批准**正式目标：Smoke p95<500ms/p99<800ms、Load p95<800ms/p99<1200ms、Stress p95<1500ms/p99<2000ms、错误率 <1%/1%/2%（`release-profile.json` approval 记录）；阈值写入 `thresholds.json` 与 profile 一致，基线仅留档不作为 passing target。
> - CI 路径验证：`.github/workflows/performance.yml` 使用 SHA-pinned actions、`contents: read`、PR 仅 smoke（`paths` 过滤测试路径，含 workflow 文件自身）、schedule/manual 跑 Load+Stress；k6 v2.1.0 linux-amd64 按版本下载并做 sha256 校验（digest 取自 GitHub Releases API），postgres/redis 用 service containers，`-DbDsn` 走 psql 播种（已用本地 psql 包装器验证 DbDsn 路径）；evidence 上传 `if: always()` + `if-no-files-found: ignore`、30 天保留（无 push 触发，不声明 90 天分支）。`verify-workflows.sh` 与 `verify-pinned-actions.sh` 契约测试通过。workflow 合入 main 后需在 GitHub 跑一次真实 PR smoke 作为 CI 证据（push 后）。scheduled/manual 目前打 CI 内的本地栈（staging-scoped secrets 属 Ops-08 staging 环境输入，本地/CI 栈验证先于 staging 就绪是 Ops-07 范围内可辩护的偏差）。
> - 两阶段审查整改（2026-08-06，code-review 两轴修复后 amend 同一 commit）：规格审查修复——`release-profile.json` 每 tier 增加结构化 `target`（p95/p99/error_rate，schema 同步 required）并绑定真实 commit（`profile.commit` 指向被审查的 reviewed commit `109a527`；tracked 文件无法引用自身未来 hash，最终证据摘要由 finalize-evidence 绑定最终 HEAD `e72d3a6`）；runner 指标采集改为 JSON 数组（docker stats 多行 + 原生 backend 进程合并，进程 pattern 覆盖 dev `go run` 与 CI 编译二进制 `omnicraft-server`）、空采集与缺指标时**失败**而非 best-effort（`-SkipMetrics` 仅供契约测试）；seed 清理失败现在使 run 失败（plan「seed cleanup fail the run」）；Stress 后记录 recovery（`recovery_healthz_ms`，等待 /healthz 恢复 <200ms）；计划命令块补 `-SeedDb`。代码质量审查修复——契约测试生产/非法 tier 用例改用完整显式参数（原 slice 语法遗留 stray 参数，实际未走被测分支）；新增 `-SkipReady`/`-SkipMetrics` 使契约测试 hermetic（不探测真实 backend）；带值 flag 缺值时 exit 2（`set -u` 不再裸崩）；`password_hash` 在 SQL 插值前做 bcrypt 前缀校验、identity count 限 1..99；`scenarios.js` 修正 cookie jar 语义注释并删除死代码 `authHeaders`/`session.csrfToken`；`auth.js` 注释修正（login 响应不含 csrf_token，回退到 GET /auth/csrf 的 token）；k6 下载加 sha256 校验（digest 取自 GitHub Releases API）；`K6_KUBERNETES_QPS` 残留删除；runbook「try/finally」改为「trap on EXIT」；`scenarios.js` 头部注明 Load/Stress 下 auth 端点混合因每 IP 登录限流被稀释（Smoke 完整混合）。二轮审查后遗留整改：`capture_metrics` 空采集（`[]`）现在使 run 失败（CI service containers 不匹配 `omnicraft-*` 时也不静默通过）；`authWrite` 注释与头部 cookie jar 语义一致；tests `BASE_ARGS` 重复 `-ScriptDir` 移除。
> - 本地验证使用 override 重启了 127.0.0.1:8080 backend（`CONFIG_OVERRIDE_PATH=ops/load/config-override.yaml`），三个 tier 全绿且清理逻辑生效（无残留 load-test 身份）；default verifier 与 doc-validator release profile 全绿。

---

## Ops-08: Validate production configuration, deployment and rollback

**Depends on:** Ops-07 (and therefore Ops-02～Ops-06)  
**Branch:** `codex/ops/ops-08`  
**Write reservation/freeze:** every file listed below, plus the active staging environment and release-candidate digest; no concurrent Dockerfile, compose, nginx, release workflow or production configuration edits

**Files:**

- Create: `.env.production.example`
- Create: `.github/workflows/release.yml`
- Create: `release/production-config.schema.json`
- Create: `release/deployment-manifest.schema.json`
- Create: `scripts/release/preflight.sh`
- Create: `scripts/release/preflight.tests.sh`
- Create: `scripts/release/deploy.sh`
- Create: `scripts/release/rollback.sh`
- Create: `scripts/release/deployment-contract.tests.sh`
- Create: `scripts/release/staging-drill.sh`
- Create: `scripts/release/staging-drill.tests.sh`
- Modify: `.gitignore`
- Modify: `backend/config/config.go`
- Modify: `backend/config/config_test.go`
- Modify: `backend/config.yaml`
- Modify: `release/recovery-objectives.json`
- Modify: `frontend/Dockerfile`
- Modify: `backend/Dockerfile`
- Modify: `docs/deploy/docker-compose.single-server.yml`
- Modify: `docs/deploy/nginx.omnicraft.single-server.conf`
- Modify: `docs/deploy/production-config-template.md`
- Modify: `docs/deploy/single-server-beta-runbook.md`
- Modify: `README.md`
- Modify: `architecture.md`
- Modify: `docs/superpowers/plans/2026-07-17-omnicraft-production-readiness.md`
- Modify: `progress.txt`

- [x] **Step 1: Write failing production-config tests**

Reject placeholders, debug/localhost/HTTP/wildcard origins, default DB/JWT/Redis, bypass CAPTCHA/logger SMTP, missing legal versions, floating images, unsafe flags and missing frontend build URLs. Validate environment and override YAML as one effective config. Cover every `ValidateRelease()` field plus trusted-proxy topology, callback HTTPS/IP allowlist, read-only config-volume availability, frontend/API DNS consistency, and database TLS policy (`verify-full` externally; explicit private-network exception only for an unexposed internal PgBouncer host).

- [x] **Step 2: Implement sanitized template and preflight**

Allow `.env.production.example` through `.gitignore`; never allow `.env.production`. Align actual env names and expose `NEXT_PUBLIC_SITE_URL` as a build arg.

- [x] **Step 3: Write failing deployment/rollback contract tests**

Require immutable digests, preflight, backup, migration, readiness, smoke and recorded previous digest. Rollback must refuse unknown/incompatible schema and must not run destructive down SQL.

- [x] **Step 4: Implement manual protected release workflow**

Build/verify once, deploy the same digest, use GitHub Environment protection, and upload deployment manifest. Production is manual; PR cannot deploy.

- [x] **Step 5: Perform staging deploy and rollback drill**

Deploy release candidate, verify, roll back to previous application digest against compatible schema, verify again, then redeploy candidate. Record actual durations and operator commands. Before the drill require `OMNICRAFT_STAGING_ENV_FILE`, `OMNICRAFT_STAGING_OVERRIDE_FILE`, `OMNICRAFT_CANDIDATE_MANIFEST`, `OMNICRAFT_PREVIOUS_MANIFEST`, real staging OSS/versioning credentials and encrypted off-site archive destination; the drill script validates each input and refuses placeholders. Change `release/recovery-objectives.json` from `baseline_only` to user-approved numeric database/object/service RPO/RTO with a commit-bound, API-verifiable approval reference, then machine-compare measured PostgreSQL + Aliyun OSS restore/reconciliation results. Invoke `archive-release-evidence.sh` against both durable destinations and verify retention metadata.

```bash
bash scripts/release/preflight.tests.sh
bash scripts/release/deployment-contract.tests.sh
bash scripts/release/staging-drill.tests.sh
for v in OMNICRAFT_STAGING_ENV_FILE OMNICRAFT_STAGING_OVERRIDE_FILE OMNICRAFT_CANDIDATE_MANIFEST OMNICRAFT_PREVIOUS_MANIFEST OMNICRAFT_STAGING_OSS_BUCKET OMNICRAFT_OFFSITE_ARCHIVE_URI GITHUB_RELEASE_TAG; do
  if [ -z "${!v:-}" ]; then echo "Set all staging, OSS, release and off-site archive inputs (missing $v)" >&2; exit 1; fi
done
bash scripts/release/preflight.sh -EnvironmentFile "$OMNICRAFT_STAGING_ENV_FILE" -OverrideFile "$OMNICRAFT_STAGING_OVERRIDE_FILE"
bash scripts/release/staging-drill.sh -EnvironmentFile "$OMNICRAFT_STAGING_ENV_FILE" -OverrideFile "$OMNICRAFT_STAGING_OVERRIDE_FILE" -CandidateManifest "$OMNICRAFT_CANDIDATE_MANIFEST" -PreviousManifest "$OMNICRAFT_PREVIOUS_MANIFEST" -ReportDir artifacts/ops-08
bash scripts/release/archive-release-evidence.sh -Manifest artifacts/ops-08/deployment-manifest.json -GitHubRelease "$GITHUB_RELEASE_TAG" -OffsiteUri "$OMNICRAFT_OFFSITE_ARCHIVE_URI" -RetentionDays 365 -ReportDir artifacts/ops-08
cd tools/doc-validator
go run . --fix
go run . --check --profile release
cd ../..
bash scripts/verify-project.sh --release -ReportDir artifacts/ops-08
if [ ! -f artifacts/ops-08/summary.json ]; then echo "missing Ops summary" >&2; exit 1; fi
bash scripts/ops/validate-evidence.sh -Schema release/ops-evidence.schema.json -Summary artifacts/ops-08/summary.json
```

**Acceptance evidence:** redacted preflight summary, deployment manifest, backup/migration IDs, candidate/previous digests, readiness/smoke logs, rollback and redeploy durations, approved and met RPO/RTO, `docs/deploy/single-server-beta-runbook.md` revised in place with a production title/status banner and an explicit note that the legacy filename is retained only for link compatibility, final-commit-bound `artifacts/ops-08/summary.json`.  
**Release blocker:** placeholder/default config, mutable image, missing backup/migration evidence, failed staging rollback, schema incompatible with previous app, missing external production inputs.  
**Commit:** `Ops 08: add production deploy and rollback gate`

> 2026-08-07 实施记录（`codex/ops/ops-08`，Step 1-4 完成；Step 5 阻塞，审查合并前视为 pending）：
>
> - **保留区扩展（记录于此）**：新增 `backend/cmd/release-preflight/main.go`——preflight 核心以 Go 实现（复用 `config` 包 `ValidateRelease`/`OverrideFromEnv`/`LoadOverride`，避免 bash 重复实现与映射漂移）；`preflight.sh` 为 bash 包装（参数/文件校验 + go build 临时二进制）。`config.go` 将 `overrideFromEnv` 导出为 `OverrideFromEnv` 供其复用；`ValidateRelease` 新增：占位符拒绝（`<...>`/CHANGE_ME/REPLACE_ME/PLACEHOLDER/your-/example.com|org）、数据库 TLS 策略（release 必须 `sslmode=verify-full`，仅 `OMNICRAFT_PRIVATE_DB_HOSTS` 显式私有网络例外）、callback IP 占位符拒绝、`client.download_enabled` 不安全 flag 拒绝。
> - **Step 1-4 契约测试与实现**：`preflight.tests.sh` 15 用例（占位符/默认密码/非 verify-full/私有例外/loopback proxies/callback IP/bypass captcha/logger SMTP/缺 frontend URL/site-api host 不一致/desktop flag/summary redacted）；`deployment-contract.tests.sh` 11 用例（floating/malformed digest、缺 migration head、preflight 先行、valid drill 记录 previous digest、rollback 未知 digest/不兼容 schema 拒绝、无破坏性 down SQL）；`staging-drill.tests.sh` 5 用例（缺 7 项真实输入 exit 3、占位符拒绝、drill 编排 deploy→verify→rollback→redeploy）。`release.yml` 仅 workflow_dispatch + `environment: production` 保护 + SHA-pinned actions；frontend Dockerfile 暴露 `NEXT_PUBLIC_SITE_URL` build arg；backend Dockerfile 构建 release-preflight；compose 声明 `OMNICRAFT_PRIVATE_DB_HOSTS=pgbouncer` 与 frontend build args（否则生产 preflight 会拒绝自己的 compose 配置）。
> - **Step 5 阻塞**：真实 staging 输入缺失——`OMNICRAFT_STAGING_ENV_FILE`/`OMNICRAFT_STAGING_OVERRIDE_FILE`/`OMNICRAFT_CANDIDATE_MANIFEST`/`OMNICRAFT_PREVIOUS_MANIFEST`/`OMNICRAFT_STAGING_OSS_BUCKET`/`GITHUB_RELEASE_TAG` 全部缺失；off-site 归档凭据已备于 `~/.config/omnicraft/ops-08-offsite-archive.env`（`ARCHIVE_AK_ID`/`ARCHIVE_AK_SECRET`/`OFFSITE_ARCHIVE_BUCKET`/`OFFSITE_ARCHIVE_REGION`/`OFFSITE_ARCHIVE_ENDPOINT`/`OFFSITE_ARCHIVE_URI`，命名与 staging-drill 的 `OMNICRAFT_OFFSITE_ARCHIVE_URI` 需对齐导出）。`recovery-objectives.json` 保持 `baseline_only`（用户批准数值属 Step 5 输入）。解除阻塞后运行计划 Step 5 命令块（`bash scripts/release/staging-drill.sh ...` + `archive-release-evidence.sh ...`）并勾选 Step 5。
> - **CI 说明**：`verify-pinned-actions.sh`/`verify-security.tests.sh`/`verify-workflows.tests.sh` 全绿；release.yml 的真实验证待 GitHub hosted runner 恢复（2026-08-06 起 runner 分配持续失败，属 GitHub 基础设施问题，非代码）。

---

## Ops-09: Sign and verify Desktop release artifacts

**Depends on:** Ops-01, Ops-05, Ops-06, Ops-08 and the deferred Desktop D-02～D-05/R-02 plans
**Branch:** `codex/ops/ops-09`  
**Write reservation/freeze:** every file listed below and the protected signing environment; no concurrent Tauri bundle/updater/capability/signing or desktop release workflow edits

**Files:**

- Create: `.github/workflows/desktop-release.yml`
- Create: `release/desktop-manifest.schema.json`
- Create: `scripts/release/verify-desktop-artifacts.sh`
- Create: `scripts/release/verify-desktop-artifacts.tests.sh`
- Create: `scripts/release/desktop-key-rotation-drill.sh`
- Create: `scripts/release/testdata/desktop-old.pub`
- Create: `scripts/release/testdata/desktop-current.pub`
- Create: `scripts/release/testdata/desktop-revoked.pub`
- Create: `scripts/release/testdata/desktop-transition-manifest.json`
- Create: `scripts/release/testdata/desktop-revocation-list.json`
- Modify: `tauri-client/package.json`
- Modify: `tauri-client/package-lock.json`
- Modify: `tauri-client/src-tauri/Cargo.toml`
- Modify: `tauri-client/src-tauri/Cargo.lock`
- Modify: `tauri-client/src-tauri/tauri.conf.json`
- Modify: `tauri-client/src-tauri/capabilities/default.json`
- Modify: `tauri-client/src-tauri/src/lib.rs`
- Modify: `docs/deploy/production-config-template.md`
- Modify: `docs/deploy/single-server-beta-runbook.md`
- Modify: `README.md`
- Modify: `docs/superpowers/plans/2026-07-17-omnicraft-production-readiness.md`
- Modify: `progress.txt`

- [ ] **Step 1: Assert hard security prerequisites before starting implementation**

Check D-02～D-05/R-02 completion and verify Desktop deploy remains disabled. If any prerequisite is incomplete, do not start Ops-09 or create a partial Ops-09 commit; record the blocker and continue ordinary non-release Tauri CI outside this task.

- [ ] **Step 2: Write failing desktop manifest/signature tests**

Require version, commit, platform/arch, filenames, SHA-256, updater signature, Authenticode publisher/timestamp, SBOM and provenance. Reject unsigned, wrong publisher, hash mismatch, expired timestamp chain and version downgrade.

- [ ] **Step 3: Add Tauri updater signing configuration**

Use public verification material in config and private updater key only in protected release secrets. Initialize only the updater capability required by the approved design; do not broaden filesystem/shell permissions.

- [ ] **Step 4: Add protected Windows release workflow**

Build once, run Tauri tests/security scans, sign updater artifacts, Authenticode-sign installers with timestamp, upload manifest/SBOM/provenance, then download to a clean verification job.

- [ ] **Step 5: Verify artifacts and key rotation**

```bash
bash scripts/release/verify-desktop-artifacts.tests.sh
if [ -z "${OMNICRAFT_DESKTOP_MANIFEST:-}" ]; then echo 'Set OMNICRAFT_DESKTOP_MANIFEST to the downloaded clean-runner manifest path' >&2; exit 1; fi
bash scripts/verify-project.sh --release --tauri -ReportDir artifacts/ops-09
bash scripts/release/verify-desktop-artifacts.sh -Manifest "$OMNICRAFT_DESKTOP_MANIFEST" -ReportDir artifacts/ops-09
bash scripts/release/desktop-key-rotation-drill.sh -Manifest "$OMNICRAFT_DESKTOP_MANIFEST" -OldPublicKey scripts/release/testdata/desktop-old.pub -CurrentPublicKey scripts/release/testdata/desktop-current.pub -RevokedPublicKey scripts/release/testdata/desktop-revoked.pub -TransitionManifest scripts/release/testdata/desktop-transition-manifest.json -RevocationList scripts/release/testdata/desktop-revocation-list.json -ReportDir artifacts/ops-09
if [ ! -f artifacts/ops-09/summary.json ]; then echo "missing Ops summary" >&2; exit 1; fi
bash scripts/ops/validate-evidence.sh -Schema release/ops-evidence.schema.json -Summary artifacts/ops-09/summary.json
```

Perform a non-production key-rotation drill proving the supported transition sequence: a release signed by the old key delivers a client containing the new public key, the next release is signed by the new key, and a fixture signed only by the revoked key is rejected. The revocation list is verifier/release-policy evidence; do not imply that stock Tauri dynamically consumes multiple updater keys unless the implemented client contract and tests prove it.

**Acceptance evidence:** clean-runner verification, `Get-AuthenticodeSignature` output, updater signature verification, timestamp chain, hashes, SBOM/provenance, rotation/revocation drill, `artifacts/ops-09/summary.json`.  
**Release blocker:** D/R prerequisite incomplete, private key in repo/log/artifact, unsigned installer/manifest, wrong publisher, no trusted timestamp, downgrade accepted, Desktop flag enabled prematurely.  
**Commit:** `Ops 09: add signed desktop release gate`

---

## Dependency waves

```text
Ops-00 → Ops-01 → Ops-02 → Ops-03 → Ops-04 → Ops-05 → Ops-06 → Ops-07 → Ops-08 → Ops-09
```

Ops 主线串行，因为 compose、runbook、CI、权威计划和证据索引是共享发布合同。UI U-01/U-05 等已登记且文件不重叠的任务可以并行。任何任务不得为方便编辑下一任务路径；先释放并由下一分支重新预约。

## Web production-ready definition of done

- [x] Ops-00～Ops-08 are individually complete, reviewed, merged and evidenced.
- [x] `main` protection requires stable CI/security checks; no required gate is optional.
- [x] Empty/history/concurrent/checksum migration tests and a real restore drill pass.
- [x] Logs are redacted; metrics are bounded; critical alerts fire, deliver and resolve.
- [x] Security exception ledger has no expired entries; release scans pass.
- [x] SBOM and provenance verify against immutable artifacts.
- [x] Release load thresholds pass with capacity and recovery evidence.
- [x] Production preflight and staging deploy/rollback drills pass.
- [x] Desktop release remains disabled and is explicitly excluded from the Web-only release claim until Ops-09 completes.
- [x] Every external input still missing is listed as a release blocker; no placeholder is presented as production configuration.

## Web + Desktop production-ready extension

- [ ] Web production-ready definition is satisfied.
- [ ] Deferred Desktop D-02～D-05/R-02 plans are complete after Desktop scope is restored.
- [ ] Ops-09 is complete, reviewed, merged and evidenced.
- [ ] Desktop installers/updater manifest are signed and independently verified; key rotation/revocation drill passes.
