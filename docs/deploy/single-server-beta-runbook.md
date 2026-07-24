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

## 8. Backup

Database backup example:

```bash
mkdir -p /var/backups/omnicraft/postgres
docker compose --env-file .env -f docker-compose.single-server.yml exec -T postgres \
  pg_dump -U omnicraft omnicraft | gzip > "/var/backups/omnicraft/postgres/omnicraft-$(date +%F-%H%M%S).sql.gz"
```

Keep OSS data in a private bucket and use Alibaba Cloud lifecycle policies for
object retention.
