# OmniCraft Production Configuration Template

This template contains placeholders only. Keep real secrets in the server,
deployment platform, or secret manager. Do not commit filled secrets.

## Public Endpoints

- Frontend: `https://app.leeppp.online`
- API: `https://api.leeppp.online`

DNS records:

```text
app.leeppp.online  A/AAAA or CNAME  <frontend-or-nginx-target>
api.leeppp.online  A/AAAA or CNAME  <api-or-nginx-target>
```

## Backend Runtime Environment

These names are read by `backend/config/config.go`.

```dotenv
DB_DSN=host=<db-host> port=5432 user=<db-user> password=<db-password> dbname=<db-name> sslmode=require
DB_READ_DSN=

REDIS_ADDR=<redis-host>:6379
REDIS_PASSWORD=<redis-password>
REDIS_DB=0

JWT_SECRET=<base64-or-hex-random-at-least-32-bytes>
LLM_KEY_ENCRYPTION_SECRET=<base64-or-hex-random-at-least-32-bytes>

ALLOWED_ORIGINS=https://app.leeppp.online

OSS_ENDPOINT=https://oss-<region>.aliyuncs.com
OSS_ACCESS_KEY_ID=<oss-ram-access-key-id>
OSS_ACCESS_KEY_SECRET=<oss-ram-access-key-secret>
OSS_BUCKET_NAME=<private-bucket-name>
OSS_DOMAIN=https://<bucket-name>.oss-<region>.aliyuncs.com

GREEN_ACCESS_KEY_ID=<green-ram-access-key-id>
GREEN_ACCESS_KEY_SECRET=<green-ram-access-key-secret>
GREEN_REGION=cn-shanghai
GREEN_CALLBACK_URL=https://api.leeppp.online/api/v1/internal/ai-callback
GREEN_CALLBACK_ALLOWED_IPS=<comma-separated-callback-ip-list>

CAPTCHA_ACCESS_KEY_ID=<captcha-ram-access-key-id>
CAPTCHA_ACCESS_KEY_SECRET=<captcha-ram-access-key-secret>

SMTP_PASSWORD=<smtp-password-or-app-password>

# Optional. Keep disabled for Web Beta unless budget and provider are ready.
AGENT_LLM_PROVIDER=openai_compat
AGENT_LLM_MODEL=<model-name>
AGENT_LLM_API_BASE=<https-api-base>
AGENT_LLM_API_KEY=<llm-api-key>

CONFIG_OVERRIDE_PATH=/var/lib/omnicraft/config_override.yaml
```

## Backend Config Override YAML

Use this for fields that do not currently have env overrides, such as
`server.mode`, `web.public_base_url`, SMTP mode/host/user/from, captcha public
fields, legal versions, and feature flags.

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
  prefix: "<aliyun-captcha-prefix>"
  scene_id: "<aliyun-captcha-scene-id>"
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

## Frontend Build Environment

`NEXT_PUBLIC_API_URL` is used at build time by the frontend Dockerfile.
`NEXT_PUBLIC_SITE_URL` is read by sitemap/metadata code, but the current
Dockerfile does not expose it as a build arg yet.

```dotenv
NEXT_PUBLIC_API_URL=https://api.leeppp.online
NEXT_PUBLIC_SITE_URL=https://app.leeppp.online
```

## Nginx TLS Template

Replace the example certificate path in `nginx/nginx.conf` before production.

```nginx
server {
    listen 443 ssl http2;
    server_name app.leeppp.online api.leeppp.online;

    ssl_certificate /etc/letsencrypt/live/leeppp.online/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/leeppp.online/privkey.pem;
    ssl_trusted_certificate /etc/letsencrypt/live/leeppp.online/chain.pem;

    location /api/ {
        proxy_pass http://backend;
    }

    location / {
        proxy_pass http://frontend;
    }
}
```

If frontend and API are served from separate virtual hosts, use two `server`
blocks instead: `app.leeppp.online` proxies `/` to frontend, and
`api.leeppp.online` proxies `/` and `/api/` to backend.

## Quick Verification Commands

```powershell
Resolve-DnsName app.leeppp.online
Resolve-DnsName api.leeppp.online

curl.exe -I https://app.leeppp.online
curl.exe https://api.leeppp.online/health
curl.exe https://api.leeppp.online/api/v1/config/public

openssl s_client -connect app.leeppp.online:443 -servername app.leeppp.online
openssl s_client -connect api.leeppp.online:443 -servername api.leeppp.online
```

## Known Repository Caveats

- `.env.production` is stale for OSS. Use `OSS_ACCESS_KEY_ID`,
  `OSS_ACCESS_KEY_SECRET`, and `OSS_DOMAIN`, not `ALIYUN_ACCESS_KEY_ID`,
  `ALIYUN_ACCESS_KEY_SECRET`, or `OSS_CDN_DOMAIN`.
- `ValidateRelease()` exists but is not currently called at backend startup.
  Verify release-only constraints explicitly until that code is repaired.
- The current CAPTCHA adapter should be tested against Alibaba Cloud before
  release; if verification fails with valid credentials, the adapter may need
  request-signing work.
- `backend/cmd/seed_admin/main.go` uses a hardcoded local DSN and default
  password. Do not run it directly for production admin creation.
