# Single-Server Web Beta Runbook

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
GREEN_CALLBACK_ALLOWED_IPS=<comma-separated-ip-list>

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

## 6. Start Services

Copy the templates:

```bash
cp docs/deploy/docker-compose.single-server.yml docker-compose.single-server.yml
cp docs/deploy/nginx.omnicraft.single-server.conf nginx/omnicraft.single-server.conf
```

Then start:

```bash
docker compose --env-file .env -f docker-compose.single-server.yml up -d --build
docker compose --env-file .env -f docker-compose.single-server.yml ps
```

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
- Real Aliyun OSS versioning/off-host storage and approved numeric RPO/RTO
  targets are Ops-08 blockers; `release/recovery-objectives.json` currently
  records measured baselines only.

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

Real Aliyun OSS versioning, off-host storage and service-level RPO/RTO remain
Ops-08 blockers; the local MinIO stand-in proves adapter behavior only.
