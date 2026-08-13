# OmniCraft Single-Server Production Runbook

> **Status: PRODUCTION** (Web-only scope). This document is the operational
> runbook for the single-server production deployment. The filename
> `single-server-beta-runbook.md` is retained only for link compatibility and
> is referenced as `docs/deploy/single-server-beta-runbook.md` throughout the
> repository; do not rename it.
>
> **Release discipline:** every release candidate must pass
> `scripts/release/preflight.sh` (production configuration contract) and the
> staging deploy/rollback drill (`scripts/release/staging-drill.sh`) before
> it is deployed to production. Deployment images are referenced by immutable
> sha256 digests (`release/deployment-manifest.schema.json`); rollback never
> runs destructive down SQL. Desktop release capabilities remain disabled
> (Ops-09 deferred).

Target server:

- Ubuntu Server 24.04 LTS
- Public IP: `43.139.183.130`
- Domains: `app.leeppp.online`, `api.leeppp.online`

## 1. Directory Layout

```bash
sudo mkdir -p /opt/omnicraft /var/lib/omnicraft /var/backups/omnicraft /var/www/certbot
sudo chown -R "$USER:$USER" /opt/omnicraft /var/lib/omnicraft /var/backups/omnicraft /var/www/certbot
```

## 2. Clone Or Upload Code

Use whichever source is convenient. Example with git:

```bash
cd /opt
git clone <repo-url> omnicraft
cd /opt/omnicraft
```

## 3. Create Server Secret File

Create `/opt/omnicraft/.env` on the server. Keep it out of Git.

```dotenv
POSTGRES_USER=omnicraft
POSTGRES_PASSWORD=<strong-postgres-password>
POSTGRES_DB=omnicraft
# Registry-verified immutable image reference; never use :latest or a tag.
PGBOUNCER_IMAGE=edoburu/pgbouncer@sha256:4c1ca296ef525f108f5d3552cc337c0c09587cf8dae7f0067fd93349e47dc1cd

REDIS_PASSWORD=<strong-redis-password>
REDIS_DB=0

JWT_SECRET=<openssl-rand-base64-64>
LLM_KEY_ENCRYPTION_SECRET=<openssl-rand-base64-64>

ALLOWED_ORIGINS=https://app.leeppp.online

OSS_ENDPOINT=https://oss-<region>.aliyuncs.com
OSS_ACCESS_KEY_ID=<oss-access-key-id>
OSS_ACCESS_KEY_SECRET=<oss-access-key-secret>
OSS_BUCKET_NAME=<bucket-name>
OSS_DOMAIN=https://<bucket-name>.oss-<region>.aliyuncs.com

GREEN_ACCESS_KEY_ID=<green-access-key-id>
GREEN_ACCESS_KEY_SECRET=<green-access-key-secret>
GREEN_REGION=cn-shanghai
GREEN_CALLBACK_URL=https://api.leeppp.online/api/v1/internal/ai-callback
# Callback signature seed ([A-Za-z0-9_], max 64 chars). A generated template
# value is provided; deployers may replace it before first launch.
GREEN_SEED=eGvqrYixTEzFRDUToSd1lgy3plgaMJDqr0X5Ji7P4TY
# Aliyun MAIN account UID from the console top-right account info (not the RAM UID).
GREEN_UID=<aliyun-main-account-uid>

CAPTCHA_ACCESS_KEY_ID=<captcha-access-key-id>
CAPTCHA_ACCESS_KEY_SECRET=<captcha-access-key-secret>

SMTP_HOST=<smtp-host>
SMTP_PORT=587
SMTP_USERNAME=<smtp-username>
SMTP_PASSWORD=<smtp-password>
SMTP_FROM_EMAIL=noreply@example.com

AGENT_LLM_PROVIDER=openai_compat
AGENT_LLM_MODEL=
AGENT_LLM_API_BASE=
AGENT_LLM_API_KEY=

CONFIG_OVERRIDE_PATH=/app/config_override.yaml
```

Generate strong local secrets on the server:

```bash
openssl rand -base64 64
openssl rand -base64 64
openssl rand -base64 32
openssl rand -base64 32
```

The first three are for `JWT_SECRET`, `LLM_KEY_ENCRYPTION_SECRET` and `LOG_IP_HASH_SECRET`; the fourth generates the optional `GREEN_SEED` replacement (take the first 48 `[A-Za-z0-9_]` characters). The generated template `GREEN_SEED` is already valid — regenerating is optional, do it before the first launch if you want a server-local secret.

## 4. Create Backend Override YAML

Create `/var/lib/omnicraft/config_override.yaml`.

```yaml
server:
  mode: "release"
  port: "8080"
  shutdown_timeout: 15

web:
  public_base_url: "https://app.leeppp.online"

security:
  allowed_origins:
    - "https://app.leeppp.online"

features:
  payment_enabled: false
  creator_support_enabled: false
  desktop_deploy_enabled: false

client:
  download_enabled: false
  download_url: ""
  latest_version: ""

captcha:
  provider: "aliyun_v2"
  prefix: "<captcha-prefix>"
  scene_id: "<captcha-scene-id>"
  region: "cn"

smtp:
  mode: "smtp"
  host: "<smtp-host>"
  port: 587
  user: "<smtp-user>"
  from_address: "<verified-from-address>"

legal:
  current_terms_version: "2026-06-05"
  current_privacy_version: "2026-06-05"

agent:
  web_agent_enabled: false
  llm_provider: "openai_compat"
  llm_model: ""
  llm_api_base: ""
  rate_limit_per_day: 0
  upload_assist_max_file_mb: 10
  max_user_message_chars: 4000
  chat_max_context_messages: 10
```

## Release Gate Checklist

- `server.mode: "release"` is set through `CONFIG_OVERRIDE_PATH`.
- `web.public_base_url` and all `security.allowed_origins` entries use HTTPS production domains.
- `JWT_SECRET`, `REDIS_PASSWORD`, `POSTGRES_PASSWORD`, `LLM_KEY_ENCRYPTION_SECRET`, OSS keys, Green keys, CAPTCHA keys, and SMTP password are real secrets stored outside Git.
- `captcha.provider` is not `bypass`.
- `smtp.mode` is not `logger`.
- `legal.current_terms_version` and `legal.current_privacy_version` are non-empty.
- `features.desktop_deploy_enabled` remains `false`.
- `features.payment_enabled` remains `false` unless payment is separately approved.

## 5. Install TLS Certificate

Before starting the production nginx container, issue a certificate on the host:

```bash
sudo apt update
sudo apt -y install certbot
sudo certbot certonly --standalone \
  -d app.leeppp.online \
  -d api.leeppp.online
```

The compose nginx template expects:

```text
/etc/letsencrypt/live/app.leeppp.online/fullchain.pem
/etc/letsencrypt/live/app.leeppp.online/privkey.pem
/etc/letsencrypt/live/app.leeppp.online/chain.pem
```

If Certbot creates a different live directory, update
`nginx/omnicraft.single-server.conf`.

## 6. Select and Start a Service Profile

Copy the templates:

```bash
cp docs/deploy/docker-compose.single-server.yml docker-compose.single-server.yml
cp docs/deploy/nginx.omnicraft.single-server.conf nginx/omnicraft.single-server.conf
```

### 6.1 Full production profile

The committed single-server compose defines 17 services: six resident Web
core services, the one-shot `migrate` release gate, and ten resident
observability/alerting services. Its declared resident memory limits total
about 5.7 GiB before Ubuntu, Docker and filesystem cache, so it must not be
started as a full stack on the current 3.6 GiB interview host.

On a host with sufficient capacity (8 GiB minimum is the current operational
target, or when observability is hosted separately), start the full profile:

```bash
docker compose --env-file .env -f docker-compose.single-server.yml up -d --build
docker compose --env-file .env -f docker-compose.single-server.yml ps
```

### 6.2 Resource-constrained interview profile (3.6 GiB)

The Web-only interview host uses this explicit service boundary:

- resident Web core: `postgres`, `redis`, `pgbouncer`, `backend`, `frontend`,
  `nginx`;
- release-time one-shot gate: `migrate`;
- resident application metrics: `prometheus`, with a dedicated lean config
  that scrapes only `backend:9091`;
- deferred on this host: `alertmanager`, `postgres-exporter`,
  `redis-exporter`, `cadvisor`, `blackbox`, `node-exporter`, `loki`, `alloy`
  and `loki-gate`.

This profile is a deliberate interview/demo capacity trade-off, not evidence
that the full production observability gate is running. It must preserve all
current release and security contracts: immutable image references, Redis
authentication, PgBouncer SCRAM, external secret/config override files,
one-shot ledger-backed migrations, JSON log rotation, health/readiness
checks, backups, and Nginx as the only public port owner.

Do **not** obtain the lean profile by merely listing those services on the
current compose command. The committed `ops/observability/prometheus.yml`
also targets Alertmanager, PostgreSQL/Redis exporters, cAdvisor, Blackbox and
node-exporter and loads rules backed by those targets. Starting it unchanged
with those services absent produces blind/missing targets and is not an
acceptable green monitoring state. Before changing the server, provide and
statically validate a dedicated backend-only Prometheus config and a compose
override/profile that mounts it. Until that deploy artifact exists, the
3.6 GiB profile is an approved deployment decision, not an executable release
command.

The existing compose limits for the six Web core services plus Prometheus add
up to about 3.9 GiB. Limits are caps rather than reservations, but that still
leaves no safe host headroom. The deploy artifact must use measured peak RSS
to keep aggregate container limits at or below about 2.6 GiB, leaving roughly
1 GiB for Ubuntu, Docker and page cache. Capture `docker stats`, host available
memory and OOM evidence during smoke. Build immutable frontend/backend images
in CI rather than concurrently on this host.

The compose stack runs the forward-only migrations as a one-shot `migrate`
container before the backend starts (`backend.depends_on.migrate:
service_completed_successfully`). The backend refuses to start when the
migration job fails; no `/docker-entrypoint-initdb.d` mount is used because
init scripts only apply to a brand-new empty data volume and silently ignore
later migration files. `docker compose logs migrate` shows the applied
migration set and `migration-summary.json` in the container.

## 7. Verify

```bash
curl -I https://app.leeppp.online
curl https://api.leeppp.online/healthz
curl https://api.leeppp.online/api/v1/config/public
docker compose --env-file .env -f docker-compose.single-server.yml logs --tail=100 backend
```

Expected public config posture:

```json
{
  "features": {
    "web_agent_enabled": false,
    "payment_enabled": false,
    "creator_support_enabled": false,
    "desktop_deploy_enabled": false
  },
  "captcha": {
    "provider": "aliyun_v2"
  },
  "client": {
    "download_enabled": false
  }
}
```

### 7.1 Observability (logs, metrics, readiness)

The stack ships container logs through Docker `json-file` rotation (10 MB x 5
per service) into Grafana Alloy, which tails them from a **read-only** log
directory mount and forwards them to Loki (30-day retention, durable named
volume). Prometheus scrapes the backend's internal `:9091/metrics` endpoint
(30-day time + 10 GB disk retention); neither Prometheus nor Loki publishes a
public port.

Operator access to Loki goes through `loki-gate` (authenticated, audited),
bound to `127.0.0.1:13100` on the host:

```bash
# required variables for the observability services
OBS_GATE_TOKEN="<operator token>"
OBS_LOG_DIR="/var/lib/docker/containers"   # read-only mount for Alloy

ssh -N -L 13100:127.0.0.1:13100 <server>   # tunnel, then query:
curl -H "Authorization: Bearer $OBS_GATE_TOKEN" \
  "http://127.0.0.1:13100/loki/api/v1/query_range?query=%7Bjob%3D%22containers%22%7D&limit=20"
```

Every gate query is appended to `loki_gate_audit` (durable volume,
`access-audit.jsonl`); unauthenticated queries return 401 and are also
audited. Inside the network the backend exposes liveness `/healthz` (process
only) and dependency-aware `/readyz` on `:9091`; both stay unreachable from
the public network. `migrate`, `backup-db.sh` and `recovery-drill.sh` write
bounded success/failure/last-success metrics to
`METRICS_TEXTFILE_DIR` (Prometheus textfile collector) when configured.

The observability drill that proves this posture runs with:

```bash
bash scripts/ops/observability-drill.sh -Environment Local -ReportDir artifacts/ops-03
```

## 8. Database Migrations and Recovery

Migrations are forward-only and ledger-backed. `schema_migrations` records
version, filename, SHA-256 checksum and applied time; the runner compares the
applied file/version/checksum set (never the max version alone), so a later
backfill of a missing lower-numbered migration is still applied. A session
advisory lock serializes concurrent runners.

Rules enforced by `scripts/init-db.sh` / the `migrate` binary:

- Applied-file checksum drift, duplicate versions, invalid filenames and
  ledger entries whose file disappeared are rejected.
- Every migration runs in one transaction (ledger row committed together
  with the migration), except files declared in `backend/migrations/metadata.json`.
- `047_pg_trgm_indexes.sql` and `049_search_trigram_fallback.sql` use
  `CREATE INDEX CONCURRENTLY` and are declared non-transactional with reason,
  reviewer, machine-checkable pre/postconditions and reconciliation steps.
  Their attempts are audited in `schema_migration_attempts`; a failed or
  unknown attempt blocks blind retry and all later migrations until the
  operator reconciles with evidence:
  `scripts/init-db.sh -ReconcileVersions 047,049 -ReconcileApproval CHG-1234`.
  The approval value must identify a durable ticket, incident or change record;
  it is stored in `schema_migration_attempts.approval_ref`.
- A migration that reached a shared environment is never edited; changes are
  new forward-fix migrations.

Local bootstrap:

```bash
docker compose up -d postgres
bash scripts/init-db.sh
```

`scripts/init-db.sh` is a thin wrapper around
`cd backend && go run ./cmd/migrate -DSN "$DB_DSN" -Dir backend/migrations`.
Databases created by the old initdb flow (tables present, no ledger) must be
recreated once with `docker compose down -v` before the ledger runner can be
used; verify with `bash scripts/db/build-historical-fixture.tests.sh` and the
migration integration suite (`go test ./internal/migration ./cmd/migrate`).

Upgrade drill requirements (see `release/backup-policy.json`):

- A full backup must exist within 24 hours, and a fresh backup must be taken
  immediately before every production migration.
- A restore drill into a new database must have succeeded within the last
  30 days before any schema change.
- Recovery order: PostgreSQL first (source of truth), then OSS object version
  restore and reconciliation, then Redis clear-and-rebuild.
- Ops-08 completed the real Aliyun OSS versioning/off-host storage drill,
  encrypted archive receipt, and approved numeric RPO/RTO comparison on
  2026-08-07. The committed `release/recovery-objectives.json` is now
  `approved` and points to the API-verifiable approval record (issue #77); the
  original recovery records remain in the local/archived Ops-08 evidence set.
  Repeat the drill for future release changes rather than treating these
  measurements as a permanent substitute.

## 9. Backup

Database backup uses the policy-compliant script (custom-format dump plus
checksum manifest and migration ledger manifest):

```bash
# Daily cron (policy: daily + 7 local copies):
0 2 * * * DB_HOST=postgres DB_PORT=5432 DB_USER=omnicraft DB_PASSWORD=... \
  BACKUP_DIR=/var/backups/omnicraft/postgres \
  bash /srv/omnicraft/scripts/backup-db.sh >> /var/log/omnicraft-backup.log 2>&1

# Before every production migration (policy: pre_migration):
DB_HOST=postgres ... bash /srv/omnicraft/scripts/backup-db.sh
```

Each backup produces `<dump>.custom` plus `<dump>.custom.manifest.json`
(dump SHA-256, source database, pg version and applied migration set).
Restores always target a NEW database and refuse to overwrite the source:

```bash
bash scripts/restore-db.sh -Backup /var/backups/omnicraft/postgres/omnicraft_*.custom \
  -TargetDB omnicraft_restore_20260805 \
  -AdminDSN "host=127.0.0.1 port=5432 user=postgres dbname=postgres" \
  -VerifyDSN "host=127.0.0.1 port=5432 user=omnicraft dbname=omnicraft_restore_20260805 sslmode=disable"
```

`restore-db.sh` verifies the dump checksum, restores, then runs the migration
ledger verifier and smoke checks; any failure drops the partial target.

Off-host copies (policy: 30-day encrypted, versioned/immutable retention with
post-upload SHA-256 verification) and the monthly restore drill are driven by
the recovery drills:

```bash
bash scripts/db/recovery-drill.sh -ReportDir artifacts/ops-02
bash scripts/db/object-recovery-drill.sh -ComposeFile ops/recovery/docker-compose.recovery.yml -ReportDir artifacts/ops-02
bash scripts/db/redis-reconciliation-drill.sh -ComposeFile ops/recovery/docker-compose.recovery.yml -ReportDir artifacts/ops-02
```

Ops-08's real Aliyun OSS versioning, off-host storage and service-level RPO/RTO
evidence is complete; the local MinIO stand-in continues to prove adapter
behavior only and is not a substitute for a future production release drill.

## 10. Alerting (Ops-04)

Prometheus evaluates `ops/observability/prometheus-rules.yml` every 15s and
routes alerts through Alertmanager. Alertmanager's receiver VALUES live only
in operator files, never in Git: render the runtime config at deploy/drill
time from the committed placeholder template plus the server env file:

```bash
# On the server (values come from /opt/omnicraft/.env, kept 0600):
SMTP_HOST=... SMTP_PORT=465 SMTP_FROM_EMAIL=... SMTP_USERNAME=... SMTP_PASSWORD=... \
  OPS_EMAIL_TO=ops@example.com \
  python3 - <<'PY'
import os
text = open("ops/observability/alertmanager.yml").read()
for k, v in {
    "SMTP_HOST_PLACEHOLDER": os.environ.get("SMTP_HOST", "SMTP_HOST_PLACEHOLDER"),
    "SMTP_PORT_PLACEHOLDER": os.environ.get("SMTP_PORT", "587"),
    "SMTP_FROM_PLACEHOLDER": os.environ.get("SMTP_FROM_EMAIL", "SMTP_FROM_PLACEHOLDER"),
    "SMTP_USERNAME_PLACEHOLDER": os.environ.get("SMTP_USERNAME", "SMTP_USERNAME_PLACEHOLDER"),
    "SMTP_PASSWORD_PLACEHOLDER": os.environ.get("SMTP_PASSWORD", "SMTP_PASSWORD_PLACEHOLDER"),
    "OPS_EMAIL_TO_PLACEHOLDER": os.environ.get("OPS_EMAIL_TO", "OPS_EMAIL_TO_PLACEHOLDER"),
}.items():
    text = text.replace(k, v)
open("/opt/omnicraft/alertmanager.yml", "w").write(text)
PY
chmod 600 /opt/omnicraft/alertmanager.yml
# Single-server compose requires ALERTMANAGER_CONFIG_FILE to point at it.
```

### Alerting: API
- `ApiUnavailable` / `ApiHigh5xxRate` / `ApiHighLatency`: blackbox probe of
  `https://api.leeppp.online` plus backend `omnicraft_http_requests_total`.
  First step: nginx/backend logs, upstream health, DB/Redis pool saturation.
- Owner: platform. Severity: critical (unavailable/5xx), warning (latency).

### Alerting: DB
- `PostgresDown` (`pg_up`), `RedisDown` (`redis_up`),
  `DatabasePoolExhausted` (in-use/max-open > 90%) and `RedisPoolExhausted`
  (active pool with no idle connections). First step: inspect slow queries,
  blocked clients and configured pool limits; restart a dependency only when
  it is actually unavailable, and do not restart backend while DB is down.

### Alerting: backend
- `QueueBacklogHigh` / `WorkerFailures` / `MigrationFailed` / `RestartLoop`.
  First step: worker logs, migration summary artifact, container restart logs.

### Alerting: backup
- `BackupStale` (24h) / `RecoveryDrillOverdue` (30d). First step: rerun
  `scripts/backup-db.sh` / `scripts/db/recovery-drill.sh` and confirm metrics.

### Alerting: cert / disk
- `CertExpirySoon` (7d, blackbox TLS probe) → renew via certbot (deploy hook
  reloads nginx). `DiskSpaceLow` (root fs < 10%) → prune images/cache, check
  Loki retention. node-exporter mounts host `/` read-only at `/host` and uses
  `--path.rootfs=/host`; confirm that mapping before acting on disk alerts.

### Alerting: external
- `ExternalDependencyFailures` (OSS/Green/CAPTCHA/SMTP/LLM > 50% failure for
  15m). First step: credentials/quota of the failing dependency.

### Verification and drills
```bash
bash scripts/ops/verify-alerts.sh -ConfigDir ops/observability
bash scripts/ops/verify-alerts.tests.sh
bash scripts/ops/alert-drill.tests.sh
bash scripts/ops/alert-drill.sh -Environment Local \
  -ComposeFile ops/observability/docker-compose.observability.yml \
  -WebhookSink http://alert-sink:8080/events -ReportDir artifacts/ops-04 \
  -HeartbeatNotificationEvidence /secure/path/current-healthchecks-delivery-redacted.png
```

The drill requires the real independent-failure-domain heartbeat credentials
(`~/.config/omnicraft/ops-04-healthchecks.env`, Healthchecks.io, 0600) and
proves firing/resolved delivery to the in-network alert-sink plus a real
missing-heartbeat down-flip with the operator email channel attached. After the
DOWN event, the drill waits up to five minutes for the evidence path to be
created or refreshed and rejects files older than the current run. Use either
an opaque-redacted email screenshot retaining the provider, DOWN subject,
temporary check name and delivery timestamp, or the provider's redacted latest
delivery-status view paired with `heartbeat-evidence.json`. Remove recipient,
project contact and source IP. Every release drill creates and waits for a fresh
provider event in the same run; prior heartbeat evidence cannot be reused.

## 11. Release preflight and staging drill (Ops-08)

### 11.1 Production configuration preflight

```bash
bash scripts/release/preflight.tests.sh
bash scripts/release/preflight.sh \
  -EnvironmentFile /opt/omnicraft/.env \
  -OverrideFile /var/lib/omnicraft/config_override.yaml \
  -Schema release/production-config.schema.json -ReportDir artifacts/ops-08
```

The preflight merges `.env` + override YAML into one effective config and
rejects: placeholders, HTTP/localhost/wildcard origins, default DB/JWT/Redis
secrets, `bypass` CAPTCHA, `logger` SMTP, missing legal versions, floating
images, unsafe feature flags (`desktop_deploy_enabled`, `download_enabled`),
missing frontend build URLs, loopback-only trusted proxies, invalid callback
IPs and non-`verify-full` database TLS (except `OMNICRAFT_PRIVATE_DB_HOSTS`).
The summary (`preflight-summary.json`) is redacted and never echoes secrets.

### 11.2 Staging deploy/rollback drill

```bash
bash scripts/release/staging-drill.tests.sh
bash scripts/release/staging-drill.sh \
  -EnvironmentFile "$OMNICRAFT_STAGING_ENV_FILE" \
  -OverrideFile "$OMNICRAFT_STAGING_OVERRIDE_FILE" \
  -CandidateManifest "$OMNICRAFT_CANDIDATE_MANIFEST" \
  -PreviousManifest "$OMNICRAFT_PREVIOUS_MANIFEST" \
  -ComposeFile "$OMNICRAFT_STAGING_COMPOSE_FILE" \
  -ReportDir artifacts/ops-08 \
  -RecoveryObjectives "$OMNICRAFT_RECOVERY_OBJECTIVES" \
  -Measured "$OMNICRAFT_MEASURED"
```

Required real staging inputs (the drill blocks with exit 3 when missing or
placeholder): `OMNICRAFT_STAGING_ENV_FILE`, `OMNICRAFT_STAGING_OVERRIDE_FILE`,
`OMNICRAFT_CANDIDATE_MANIFEST`, `OMNICRAFT_PREVIOUS_MANIFEST`,
`OMNICRAFT_STAGING_COMPOSE_FILE`, `OMNICRAFT_STAGING_OSS_BUCKET`,
`OMNICRAFT_OFFSITE_ARCHIVE_URI`, `GITHUB_RELEASE_TAG`,
`OMNICRAFT_RECOVERY_OBJECTIVES`, `OMNICRAFT_MEASURED` and
`OMNICRAFT_SMOKE_URL`. The drill sequence is: preflight → deploy candidate
digest → verify readiness/smoke → rollback to the previous digest (schema
compatible only) → verify → redeploy candidate. Rollback refuses unknown or
schema-incompatible digests and never runs destructive down SQL. Real staging
OSS/versioning credentials and the encrypted off-site archive destination are
release blockers when absent; the drill must not be replaced by simulated
evidence. `-RecoveryObjectives` must be an approved, commit-bound file and
`-Measured` must be the output of the same real staging recovery exercise;
`baseline_only` or hand-written measurements fail closed.

The measured JSON must contain `drill_id: "ops-08-staging-recovery"`, the
candidate `source_commit`, numeric minute values for all five RPO/RTO metrics,
and a non-empty `source_evidence` array. Each source entry is a relative file
path beside the measured JSON plus its SHA-256; the drill verifies those files
before comparing targets.

The durable archive step additionally requires an operator-scoped GitHub
release token (`OMNICRAFT_RELEASE_ARCHIVE_TOKEN`) and the off-site archive
credentials (`OMNICRAFT_OFFSITE_ARCHIVE_AK_ID`,
`OMNICRAFT_OFFSITE_ARCHIVE_AK_SECRET`, plus `OFFSITE_ARCHIVE_ENDPOINT` or
`OFFSITE_ARCHIVE_REGION`). `archive-release-evidence.sh` accepts the Ops-08
deployment manifest and archives the complete evidence directory; it refuses
to report success unless both the GitHub Release assets and every OSS object
are remotely verified.

## 12. Load, Stress and Capacity (Ops-07)

The k6 suite lives in `tests/load/k6/`; the runner is
`scripts/load/run-load-tests.sh`. Three tiers are defined: Smoke (script and
critical-path validation), Load (target concurrency, 20 VUs) and Stress
(step ramp to 50 VUs). Threshold objectives live in
`tests/load/k6/thresholds.json` and are approved in
`tests/load/k6/release-profile.json`; a measured baseline is recorded per
tier but is never automatically the passing target. Threshold changes need a
reviewed reason.

### Load-test environment

The application rate-limits per client IP (`rate_limit.normal_per_minute`,
default 100). A single-IP k6 run would measure the rate limiter instead of
system capacity, so the load-test target runs with the override in
`ops/load/config-override.yaml` (rate limiting disabled; this file is outside
the production config volume and never used by production). The release
profile records that the baseline environment runs with rate limiting
disabled.

### Running the tiers locally

```bash
# k6 is required on the host (brew install k6). PostgreSQL must be running
# (docker compose up -d postgres redis) and the backend must be started with
# the load-test override:
#   cd backend && CONFIG_OVERRIDE_PATH=../ops/load/config-override.yaml \
#     go run cmd/server/main.go

bash scripts/load/run-load-tests.tests.sh
bash scripts/load/run-load-tests.sh -Environment Local -Tier Smoke \
  -Target http://127.0.0.1:8080 -Profile tests/load/k6/release-profile.json \
  -ReportDir artifacts/ops-07 -RunName smoke -SeedDb omnicraft-postgres
bash scripts/load/run-load-tests.sh -Environment Local -Tier Load \
  -Target http://127.0.0.1:8080 -Profile tests/load/k6/release-profile.json \
  -ReportDir artifacts/ops-07 -RunName load -SeedDb omnicraft-postgres
bash scripts/load/run-load-tests.sh -Environment Local -Tier Stress \
  -Target http://127.0.0.1:8080 -Profile tests/load/k6/release-profile.json \
  -ReportDir artifacts/ops-07 -RunName stress -SeedDb omnicraft-postgres
```

The runner seeds five isolated test identities (`load-test-001..005@omnicraft.local`,
bcrypt hash from `tests/load/k6/testdata.json`, reputation 10 for publishing)
directly in PostgreSQL, waits for `/healthz`, executes the tier against the
target, captures resource metrics and deletes the identities via a trap on
EXIT (cascade removes their content; a failed cleanup fails the run). Use `-DbDsn <dsn>` instead of `-SeedDb` when
the database is only reachable over TCP (CI service containers; requires psql).
Production targets are refused unless `-AllowProduction` plus a confirmation
token matching `OMNICRAFT_LOAD_PROD_CONFIRM_TOKEN` are provided. The GitHub
workflow `.github/workflows/performance.yml` runs the smoke tier on PRs that
touch load paths and the full load/stress set on schedule/manual dispatch.

### Interpreting results

k6 emits `http_req_duration` percentiles, `http_req_failed` and the summary
export into `artifacts/ops-07/<run>-k6*.json`; the runner always writes
`<run>-summary.json` and `<run>-resources.json`, even when k6 fails. A k6
threshold failure propagates a nonzero exit, so a drill or CI run that
crosses a threshold fails the gate.
