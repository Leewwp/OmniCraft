---
name: omnicraft
description: Search, browse, download and publish creative content (sheet music, mods, templates, articles and more) on OmniCraft. Use these tools when the user wants to find creative works, install them following usage guides, or publish locally generated files back to the platform. Downloads and uploads need a personal access token and MUST pass the confirmation gate defined below.
---

# OmniCraft — creative content search, guides, download & publish

OmniCraft is a creative sharing platform (原创 IP、乐谱、Mod、模板、图文…). This skill
teaches your agent to **search** the public catalog, **read** content details and
structured usage guides (anonymous), **download** files and **publish** new works
(both need a personal access token, PAT).

## Setup

```bash
export OMNICRAFT_BASE_URL="https://app.leeppp.online"
export OMNICRAFT_PAT="oc_pat_..."   # optional; enables download/upload below
```

Get a PAT in the platform's account settings → Agent tokens ("Agent 令牌").
Scopes: `download`, `upload` (a token can carry one or both). The token is shown
once at creation; revoke it in settings if leaked.

Plain REST works for the read surface (`$OMNICRAFT_BASE_URL/api/v1/...`). For the
authenticated tools, point an MCP client (Streamable HTTP) at
`$OMNICRAFT_BASE_URL/api/v1/mcp` and send `Authorization: Bearer $OMNICRAFT_PAT`
on every request — the tool list adapts to the token's scopes at connect time.

## 1. Search & browse (anonymous)

```bash
curl "$OMNICRAFT_BASE_URL/api/v1/contents/search?q=<keywords>&page_size=10"
curl "$OMNICRAFT_BASE_URL/api/v1/contents/<id>"          # detail + attachments
```

Filters: `type` (mod / sheet_music / template / audio / video / image / article /
prompt / other). Full contract: `GET /api/v1/openapi.json` (OpenAPI 3.1).

## 2. Usage guides (install / setup steps)

```bash
curl "$OMNICRAFT_BASE_URL/api/v1/contents/<id>/guide?locale=zh"   # or locale=en
```

The guide merges **author specifics** (requirements, ordered steps, notes) with
the platform **safety template**. When following a guide: show the user the
requirements and the step list before doing anything, execute one step at a
time, and stop on any mismatch between requirements and the local environment.

## 3. Download (PAT, download scope)

MCP tool: `omnicraft_request_download {content_id, attachment_id?}` — returns
`{url, filename, size, content_type, expires_in, usage_guide}`. The URL is a
short-lived (~5 min) signed OSS URL; fetch it directly:

```bash
curl -L -o "$filename" "$url"   # no auth header needed, the URL is pre-signed
```

Downloads are metered and malware-scanned by the platform; a content whose
archive failed the scan is refused. After downloading, follow the returned
`usage_guide` (requirements → steps → notes) to install/configure, one step at
a time, and confirm each non-reversible action with the user.

## 4. Publish (PAT, upload scope) — one sentence, but never without consent

The full flow, in order:

1. **Inventory** the local files to publish (name, type, size).
2. `omnicraft_suggest_publish_metadata {file_name, content_type?, title?, description?}`
   → suggested title/description/category/tags. This is advice only.
3. **⛔ CONFIRMATION GATE (mandatory, no exceptions)** — present the complete
   draft to the user and get an explicit go-ahead BEFORE any write tool call:
   - title / description / category / tags
   - zone (original vs fanwork + its source), visibility (`is_public`), `allow_copy`
   - the full file list (names, sizes, which is primary)
   - usage-guide specifics (requirements / steps / notes) you will persist
   - the license/copy policy implied by `allow_copy`
   A vague "okay" earlier in the conversation is NOT consent. If the user has
   not seen this exact summary, show it and wait. Never batch the gate with
   other steps, never skip it because the user "already said upload it".
4. After consent only:
   1. `omnicraft_request_upload_url {file_name, file_type, mime_type, file_size}`
      → `{upload_url, grant_id}` for each file; PUT the bytes:
      `curl -X PUT --data-binary @<file> "$upload_url"` (no auth header).
   2. `omnicraft_create_content {title, description, zone, content_type,
      category, tags, attachments: [{grant_id, file_type, ...}], usage_guide?}`
      → `{content_id, status: "pending"}`.
5. Report back the content id and status. The draft enters the platform's
   standard review pipeline — same rules as web uploads, no exemption and no
   acceleration for agent uploads. It becomes publicly searchable only after
   approval.

Uploads share the web studio's validation: type whitelist, size cap, per-user
hourly quota, malware scan for archives. A rejected call explains why; fix the
files or wait for the quota window, never retry in a loop.

## Confirmation gate (applies to everything)

Before any action that changes the user's machine or account (installs,
downloads to a specific path, uploads), present the full plan (what, where,
which steps) and get an explicit go-ahead. The publish flow's gate in §4 step 3
is the strictest form: the exact draft summary, then consent, then write tools.

## Troubleshooting

- `token lacks required scope: download/upload` → the PAT was minted without
  that scope; create one with both in settings.
- `upload quota exceeded` → per-user hourly window; wait, don't hammer.
- `attachment must pass the malware scan` → the archive was flagged; nothing
  an agent should retry.
- MCP session from earlier lost the write tools? Reconnect with the token —
  the tool set is fixed per session at initialize time.

## MCP tools reference

Anonymous: `omnicraft_search`, `omnicraft_get_content`, `omnicraft_get_usage_guide`,
`omnicraft_list_categories`. PAT(download): `omnicraft_request_download`.
PAT(upload): `omnicraft_suggest_publish_metadata`, `omnicraft_create_content`,
`omnicraft_request_upload_url`. See `reference.md` for the REST surface and
versioning policy.
