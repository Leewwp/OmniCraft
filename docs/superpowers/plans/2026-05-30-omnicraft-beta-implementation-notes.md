# OmniCraft Dual-Track Beta Implementation Notes

> Use with `2026-05-30-omnicraft-dual-track-beta-roadmap.md` and the six domain plans. The design source of truth is `docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md`.

## 1. Execution Rules

1. Execute one roadmap task per agent session and per task branch.
2. Start a task only when its roadmap prerequisites are complete and its planned write set has been reserved.
3. Keep Web Beta safety tasks separate from the optional desktop one-click deployment track. `D-01` is a Web Beta gate; `D-02` through `D-05` remain disabled until their full security chain passes.
4. Follow the repository `AGENTS.md` for initialization, verification, `progress.txt` updates and one-task commits. For this separate Beta plan set, update the subsystem-plan checkbox and roadmap checkbox instead of `task.json`; the roadmap keeps `task.json` as the historical MVP ledger unless a maintainer explicitly migrates the Beta tasks later.
5. Stop on unresolved product or deployment decisions. Do not hide uncertainty behind an implementation default.

## 2. Multi-Agent Collaboration

### 2.1 Coordinator Responsibilities

Use one coordinator agent on `codex/beta-integration`. The coordinator:

- maintains the dependency graph and task reservation table;
- assigns one task ID and one write set to each implementation agent;
- records blockers and decisions in a shared status note or issue;
- integrates one completed branch at a time;
- reruns cross-cutting verification after every security-sensitive integration.

Implementation agents must not broaden scope silently. When a task discovers a required contract change outside its reservation, report the file, symbol, reason and affected task IDs to the coordinator before editing.

### 2.2 Task Assignment Principles

Assign work by dependency and write set, not only by subsystem. Two tasks may run concurrently only when:

- every prerequisite is already integrated;
- their migrations have distinct reserved numbers;
- their planned files and code regions do not overlap;
- neither changes a shared contract consumed by the other;
- the coordinator has reserved any shared registry edits.

Prefer short-lived task branches. Do not keep long-lived backend, frontend or desktop branches: they defer shared-file conflicts until integration and make security contract drift harder to detect.

### 2.3 Communication Contract

Each agent reports:

```text
Task:
Branch and worktree:
Reserved files / code regions:
Contract changes:
Migration numbers:
Config keys:
Verification run:
Blocked decisions:
```

Report contract changes immediately, not only at task completion. The highest-risk examples are auth middleware behavior, route mounting, public config DTOs, migration numbers, Redis key formats, i18n namespaces and desktop script schemas.

### 2.4 Results Integration

The coordinator integrates only complete task commits. Before integration, the task owner rebases onto the latest `codex/beta-integration`, resolves conflicts against the source design and roadmap contracts, reruns the task checks, then asks the coordinator for an integration slot. Merge with fast-forward only after the rebase:

```powershell
git switch codex/beta-integration
git merge --ff-only codex/beta/<task-id>
```

If the integration branch advanced before merge, rebase and verify again. Stage the exact intended files; avoid `git add .` in a dirty workspace.

## 3. Worktree Strategy

### 3.1 Worktree Layout

Create one worktree per active task from the latest integration branch:

```powershell
git switch codex/beta-integration
git worktree add ..\OmniCraft-<task-id> -b codex/beta/<task-id>
```

Remove a task worktree only after integration and verification. Keep `codex/beta-integration` in its own clean worktree.

### 3.2 Recommended Split

Use the roadmap dependency graph as the authority. Practical parallel windows include:

| Window | Tasks that may run concurrently | Constraints |
|---|---|---|
| After `F-02` | `F-05`, `F-06`, `A-01` | Reserve route edits and migration numbers before starting. |
| After `F-04` | `V-01`, `G-01` | `V-01` owns verification config; `G-01` owns frontend public-config gating. |
| After `V-02` | `V-03`, `V-05` | `V-03` is frontend-heavy and `V-05` is backend-heavy. Reserve config edits for `V-05`; serialize any incidental shared-file edit. |
| After `V-03` | `V-04` while `V-05` finishes | `V-04` may overlap only while its Footer and i18n files remain reserved to that task. |
| After `V-04` and `V-05` | `V-06` | `V-06` consumes both completed contracts. |
| After `V-05` | `A-04` | `A-04` must finish before `A-03`. |
| Agent track | `G-02` then `G-03` then `G-04` then `G-05` | Keep serial: these tasks overlap Agent service, chat UI or i18n files. |
| Desktop track | `D-01` then `D-02` then `D-03` then `D-04` then `D-05` | Keep serial: each task tightens the previous security boundary. |

This table lists safe starting points, not a replacement for file reservations. When a planned file overlap appears, serialize the affected tasks.

### 3.3 Parallel Environment Isolation

Use per-worktree override files outside the repository, for example `C:\tmp\omnicraft-worktrees\<task-id>.yaml`, and point the backend to them with the existing `CONFIG_OVERRIDE_PATH` environment variable.

Give each worktree:

- a distinct PostgreSQL database through `database.dsn`;
- a distinct Redis logical database through `redis.db`, or a distinct Redis instance through `redis.addr`;
- a distinct backend port through `server.port`;
- a distinct frontend dev port using the supported Next.js CLI argument;
- an origin entry matching that frontend port in `security.allowed_origins`.

Example:

```yaml
server:
  port: "8181"
database:
  dsn: "host=localhost user=postgres password=postgres dbname=omnicraft_f05 port=5432 sslmode=disable"
redis:
  addr: "localhost:6379"
  db: 5
security:
  allowed_origins:
    - "http://localhost:3105"
```

Do not invent `REDIS_KEY_PREFIX` or a backend `--port` flag: neither is an established repository contract. Before setting a frontend API URL, inspect the current frontend environment contract and use the name already implemented.

For container isolation, use a distinct Compose project name and explicit non-conflicting host ports. Never run destructive database reset commands against a shared database.

## 4. Conflict Prevention

### 4.1 Shared File Reservations

Reserve these files or regions before editing:

| Shared Surface | Ownership Rule |
|---|---|
| `backend/internal/handler/routes.go` | One active editor at a time. Integrate route mounts immediately after the owning task. |
| `backend/config/config.go`, `backend/config.yaml`, env examples | One config-changing task at a time. Update the roadmap registry with every accepted key. |
| `frontend/messages/zh.json`, `frontend/messages/en.json` | Prefer serial edits. If parallel work is unavoidable, reserve unique namespaces and perform a structural JSON merge after rebase. |
| `frontend/components/content/ContentDetail.tsx` | Integrate in order `F-06` -> `G-01` -> `G-04` -> `D-05`. |
| publish form files | Integrate in order `G-01` -> `G-05`. |
| `backend/internal/service/agent_service.go` | Serialize `F-05`, `G-02`, `G-03`, `G-05` and `D-03`; rebase and rerun focused Agent tests before integration. |
| Admin route subtree | `A-04` owns the move into `frontend/app/(protected)/admin/*`; later Admin tasks consume that location. |
| migrations | Reserve numbers `049` through `052` exactly as listed in the roadmap. Do not renumber after branches diverge. |
| desktop deploy schema and fixtures | `D-03` owns the server schema and fixture; `D-04` consumes the exact fixture bytes in Rust tests. |

### 4.2 Code-Block Reservations

When different tasks must touch the same file, reserve named symbols or sections. Example:

```text
routes.go:
  V-01 -> auth verification and reset routes
  V-05 -> feedback routes
  A-04 -> admin feedback routes
  D-02 -> POST /deploy-grants
  D-03 -> POST /deploy-grants/exchange
```

Integrate those branches serially. After every route edit, rerun route-mount tests and inspect the final route table.

## 5. Sync And Merge Procedure

For each completed task:

1. Fetch the latest local integration state and rebase the task branch onto `codex/beta-integration`.
2. Resolve conflicts using the priority rules below.
3. Run task-local tests.
4. Run required Go and frontend gates from `AGENTS.md`; run Rust gates for desktop tasks.
5. Browser-test UI changes with MCP Playwright and store the required screenshots.
6. Update the subsystem-plan checkbox, roadmap checkbox and `progress.txt` in the task commit only after verification succeeds. Do not modify `task.json` for this Beta program unless a maintainer explicitly migrates the task set.
7. Integrate with `git merge --ff-only`.
8. Run a coordinator smoke test for the affected cross-cutting area.

Do not batch several unverified task branches into one merge. Do not resolve semantic conflicts by choosing whichever branch was merged later.

## 6. Conflict Resolution Priority

Apply this order:

1. The source design, repository `AGENTS.md`, and the roadmap's frozen contracts.
2. Fail-closed security behavior, privacy protection and least privilege.
3. Preserve all compatible behavior already integrated.
4. Structurally merge registries and translations by key.
5. Stop and ask for a maintainer decision when two requirements remain semantically incompatible.

Security-sensitive conflicts include auth cookies, Redis fail-closed behavior, Admin authorization, download authorization, desktop public-key distribution, filesystem allowlists and grant consumption. These require focused regression tests after resolution.

## 7. Technical Risks

### 7.1 Confirmed Maintainer Decisions

- Production Web origin: `https://app.leeppp.online`.
- Production API origin: `https://api.leeppp.online`.
- CAPTCHA provider: Alibaba Cloud CAPTCHA 2.0. Keep the backend `CaptchaVerifier` interface provider-agnostic, expose only `provider`, `prefix`, `scene_id` and `region` publicly, and keep RAM AccessKeys in private runtime configuration.
- Initial Web Beta does not advertise desktop one-click deployment. Keep `features.desktop_deploy_enabled=false`, complete D-01 for the Web Beta safety gate, and defer D-02 through D-05 plus R-02 until a later desktop release decision.
- Web Beta does not expose a desktop-client download. Keep `client.download_enabled=false`; defer platform, installation, URL and version facts until a later client-release decision.

### 7.2 Remaining Human Decisions Required Before Execution

- Provide approved `/terms` and `/privacy` copy plus the exact version identifiers used by `legal.current_terms_version` and `legal.current_privacy_version`. Do not draft legal text during implementation.
- Deferred until desktop deploy is advertised: confirm whether overlapping Ed25519 key rotation is required. The current plan supports one active embedded public key and requires a rebuilt client for rotation.
- Deferred until desktop deploy is advertised: provide the desktop release `VITE_API_BASE_URL` and exact OSS hostnames for `DEPLOY_ALLOWED_DOWNLOAD_HOSTS`.

### 7.3 External Blockers

- Real SMTP credentials and Alibaba Cloud CAPTCHA 2.0 `prefix`, `scene_id`, region and RAM AccessKeys are required before verification is Beta-ready. Local logger or bypass modes are development-only.
- Redis must be available for protected auth state, verification tokens, refresh rotation, password reset tokens and desktop one-use grants.
- Production HTTPS and explicit allowed origins are required before public release.
- Desktop one-click deployment stays disabled until private-key provisioning, public-key embedding and the full `D-02` through `D-05` chain pass.

### 7.4 Migration And Data Risks

- Execute migrations in lexical filename order. The reserved Beta migrations are `049` through `052`.
- Validate both a fresh database and a representative upgrade database built by applying existing migrations `001` through `048` before the Beta migrations.
- Keep search seed data deterministic and owned by `F-05`; release validation consumes that seed instead of creating an ad hoc dataset.

### 7.5 Regression Hotspots

- Auth: fresh browser CSRF bootstrap, cookie refresh rotation, logout, stale JWT rejection, Redis outage and banned-user behavior.
- Admin: the route-group move must preserve `/admin/*` URLs while inheriting the shared protected layout.
- Feedback: screenshot presigning, internal-note visibility and reply notifications.
- Agent: public config downgrade, authorized tool allowlist and localized quick prompts.
- Desktop: legacy route remains removed, grants are one-use, signatures verify exact bytes, logical paths stay under fixed roots and raw filesystem commands are not exposed to WebView code.
