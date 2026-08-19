# S04 (#149) Archive Gate Evidence

> Created: 2026-08-19
> **预计失效日期**: 2026-10-19
> Scope: local development only; this record is not production evidence.

## Verified locally

- TDD red-to-green covered the archive state matrix, clean-only content gate, mod/legacy bypass rejection, zip structure rejection before job creation, corrupt-entry mapping, mod publish job creation, clean-only download URL signing, quarantine-key rejection even when scanning is disabled, admin role guard and rate-limit route wiring, mandatory review reason, quarantine-presence checks, illegal outcome/state handling, failed retry, latest-review gating, digest-checked quarantine restoration on false-positive release, and audit persistence. The shared Redis limiter's count and fail-closed behavior are covered by the existing middleware tests; this S04 evidence does not claim a live Redis route drill.
- Admin routes are documented by the generated API route section in `architecture.md`:

  ```text
  GET  /api/v1/admin/archive-scan-jobs/:id
  POST /api/v1/admin/archive-scan-jobs/:id/manual-review
  POST /api/v1/admin/archive-scan-jobs/:id/resolve
  POST /api/v1/admin/archive-scan-jobs/:id/retry
  ```

- Focused API/state tests:

  ```text
  cd backend
  go test ./internal/service -run 'TestArchiveScanGate|TestPublishModCreatesPendingArchiveJob' -count=1
  go test ./internal/handler -run 'TestContentDownload_ArchiveGate|TestAdminArchiveScan' -count=1
  PASS
  ```

- Repository and application verification:

  ```text
  go test ./...
  go vet ./...
  go build ./...
  npm run lint
  npm run lint:ui
  npm run test:unit
  npm run build
  bash scripts/verify-project.sh --full
  ```

  The Go and frontend commands passed locally. The full project gate passed its Go, frontend and mocked-contract stages after route documentation was synchronized; no external archive-engine result is implied.

- Route/config documentation synchronization was run twice and was idempotent:

  ```text
  cd tools/doc-validator
  go run . --fix
  go run . --fix
  ```

## Evidence boundary

The tests use local SQLite state, fake upload/OSS seams, and HTTP test routers. They prove application state transitions and stable response contracts only. They do not prove a real ClamAV signature database, EICAR detection, quarantine object lifecycle in OSS, or production release behavior.

Real engine evidence remains S05 scope. The reproducible follow-up is:

```text
docker compose --profile security up -d clamav
docker compose ps clamav
docker compose exec clamav clamdscan --version
```

The current local registry TLS/authorization failure recorded in `docs/working/2026-08-19-s03-clamav-worker-evidence.md` still applies; no EICAR or recovery claim is made here.
