# S03 (#148) ClamAV Worker Evidence

> Created: 2026-08-19
> **预计失效日期**: 2026-10-19
> Scope: local development only; this record is not production evidence.

## Verified locally

- Private protocol conformance tests passed for `zPING`, `zVERSION`, `zINSTREAM`, 32 KiB framing, clean/blocked responses, context timeout, disconnect, and bounded responses:

  ```text
  cd backend
  go test ./internal/pkg/clamav -count=1
  PASS
  ```

- Worker seam tests passed for clean hashing/audit, quarantine-before-delete, sanitized failure retry, archive-owned backoff, exhausted retry acknowledgement, malformed messages, and terminal duplicate delivery:

  ```text
  cd backend
  go test ./internal/worker -run TestArchiveScanWorker -count=1
  PASS
  ```

- `go test ./...`, `go vet ./...`, `go build ./...`, frontend lint, UI governance, 461 frontend unit tests, and frontend build passed in the local worktree.
- Compose default, `security`, and `full-infra` profiles passed `docker compose config` parsing. ClamAV has no host port, uses the private `archive_security` network, and persists signatures in `clamav_signatures`.

## Environment gap

The real ClamAV smoke was attempted with:

```text
docker compose --profile security up -d clamav
```

Docker Hub image authorization failed before container creation because the Docker registry token request hit a TLS handshake timeout. Therefore this worktree has no real ClamAV signature database, EICAR result, clamd recovery result, or production-like scan evidence. The fake-socket results above must not be described as those things.

Re-run the real smoke after registry access is available:

```text
docker compose --profile security up -d clamav
docker compose ps clamav
docker compose exec clamav clamdscan --version
```

Then run the S05 EICAR and recovery procedure from the archive-malware-scanning specification and append the real engine/signature versions and outputs here.
