# S05 (#150) Archive Scan Closure Evidence

> Created: 2026-08-19
> **预计失效日期**: 2026-10-19
> Scope: local development only; this record is not production evidence.

## Run context

- Worktree: `OmniCraft-wt-150`
- Branch: `codex/150-archive-scan-eicAR`
- Base commit: `a8bf1d4` (after #144)
- S03/S04 runtime implementation is already present from #148/#149; S05 is the full-infra/security acceptance, failure-drill, and evidence closure task.
- No real ClamAV signature database, EICAR result, OSS production object, or production release result is claimed below.

## TDD red -> green

The evidence contract was checked before this file existed:

```text
test -f docs/working/2026-08-19-s05-archive-scan-evidence.md
exit: 1
```

The smallest green result is this committed evidence package plus the local seam/full verification below. This is a documentation/acceptance task; no runtime behavior was changed in S05.

## Local green verification

Existing public seams passed:

```text
cd backend
go test ./internal/pkg/clamav -count=1
go test ./internal/worker -run TestArchiveScanWorker -count=1
go test ./internal/repository -run 'TestArchiveScan|TestCreateJob' -count=1
go test ./internal/service -run 'TestArchiveScanGate|TestPublishModCreatesPendingArchiveJob' -count=1
go test ./internal/handler -run 'TestContentDownload_ArchiveGate|TestAdminArchiveScan' -count=1
```

These contracts cover private `zPING`/`zVERSION`/`zINSTREAM` framing, bounded responses, timeout/disconnect mapping, clean/blocked/failed transitions, quarantine-before-delete, retry and idempotent completion, publish/download gates, admin manual review, audit, and safe errors. They use fake socket, SQLite, and OSS seams; they are not ClamAV engine evidence.

The final project gate was run from this worktree:

```text
GOCACHE=/tmp/omnicraft-gocache bash scripts/verify-project.sh --full
```

The first run reached `72 passed, 1 failed` in an unrelated existing #72 keyboard-focus contract; the same test passed twice in an isolated rerun. A complete second run then passed `73/73`. The exact result sequence is recorded in the companion raw file. The passing local gate covers backend test/vet/build, frontend unit/lint/UI governance/build, doc-validator release check, and mocked browser contracts `73/73`.

## Real security smoke attempt

Compose parsing passed before the smoke:

```text
cp .env.example .env
docker compose --profile security config --quiet
docker compose --profile full-infra config --quiet
```

The real attempt was:

```text
docker compose --profile security up -d clamav
```

On this Apple Silicon host the image has no native manifest:

```text
Image clamav/clamav:1.4.3 Error no matching manifest for linux/arm64/v8 in the manifest list entries: no match for platform in manifest: not found
Error response from daemon: no matching manifest for linux/arm64/v8 in the manifest list entries: no match for platform in manifest: not found
```

An explicit amd64 pull was then attempted:

```text
docker pull --platform linux/amd64 clamav/clamav:1.4.3
```

The registry subsequently failed TLS verification before producing a local image:

```text
failed to resolve reference "docker.io/clamav/clamav:1.4.3": failed to do request: Head "https://registry-1.docker.io/v2/clamav/clamav/manifests/1.4.3": tls: failed to verify certificate x509: certificate is valid for *.facebook.com ... not registry-1.docker.io
```

The container was not created. Therefore S05 has no real engine version, signature database, bare/single-layer/double-layer EICAR result, clamd stop/recovery result, or real quarantine/delete result. The fake-socket tests above must not be described as those results.

## Reproducible acceptance procedure

After registry access and a compatible image are available, run this procedure and append the real output here:

```text
docker compose --profile security up -d clamav
docker compose ps clamav
docker compose exec clamav clamdscan --version
docker compose exec -T clamav clamdscan --stream < eicar.com
docker compose exec -T clamav clamdscan --stream < eicar.com.zip
docker compose exec -T clamav clamdscan --stream < eicar.com-2.zip
docker compose stop clamav
# submit a pending archive job and confirm it remains blocked/failed closed
# while clamd is down; start the service and confirm retry convergence
docker compose start clamav
docker compose exec clamav clamdscan --ping 1 --fdpass /etc/hosts
```

The acceptance must also record normal clean mod, encrypted zip, zip-slip, symlink, quota/zip-bomb rejection before ClamAV, blocked quarantine/delete, clean URL residual-window behavior, and administrator false-positive audit.

## Deferred items

- `#76` production legacy-favorites deletion remains deferred.
- `#151` historical archive rescan remains deferred/not planned.
- Desktop/Tauri work remains outside this Web-only batch.

Raw commands and outputs are in `docs/working/2026-08-19-s05-archive-scan-raw.txt`.
