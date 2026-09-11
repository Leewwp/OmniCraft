---
name: omnicraft
description: Search, browse and follow usage guides for creative content (sheet music, mods, templates, articles and more) on OmniCraft. Use these tools when the user wants to find creative works, check compatibility requirements, or get step-by-step setup/install guidance. Downloads and uploads need a personal access token and are NOT part of this skill yet.
---

# OmniCraft — creative content search & usage guides

OmniCraft is a creative sharing platform (原创 IP、乐谱、Mod、模板、图文…). This skill
teaches your agent to **search** its public catalog, **read** content details and
**follow structured usage guides** — anonymously, no API key needed.

## Setup

Set the base URL (defaults to the public demo instance):

```bash
export OMNICRAFT_BASE_URL="https://app.leeppp.online"
```

Everything below is plain REST against `$OMNICRAFT_BASE_URL/api/v1/...` — you can
also point an MCP client at `$OMNICRAFT_BASE_URL/api/v1/mcp` (Streamable HTTP).

## 1. Search

```bash
curl "$OMNICRAFT_BASE_URL/api/v1/contents/search?q=<keywords>&page_size=10"
```

- Anonymous, rate-limited per IP (search: 30 req/min; regular reads: 300 req/min).
- Filters: `type` (mod / sheet_music / template / audio / video / image / article /
  prompt / other). Responses list `items[]` with `id`, `title`, `content_type`,
  `description`, `zone`.
- Response contract: `GET /api/v1/openapi.json` (OpenAPI 3.1).

## 2. Browse

```bash
curl "$OMNICRAFT_BASE_URL/api/v1/contents/<id>"        # detail + attachment metadata
curl "$OMNICRAFT_BASE_URL/api/v1/contents/<id>/versions" # public version lineage
```

Attachment entries carry short-lived signed display URLs (`oss_url`, ~1h) for
covers and previews. **Download URLs are not included** — downloads require the
owner's authorization (see "Not in this skill" below).

## 3. Usage guides (install / setup steps)

```bash
curl "$OMNICRAFT_BASE_URL/api/v1/contents/<id>/guide?locale=zh"   # or locale=en
```

The guide merges two layers:

- **author specifics** — requirements (`["Minecraft 1.20.1+", "Forge 47"]`),
  ordered `steps`, free-form `notes`; when absent the system template applies;
- **safety** — always served from the platform template; treat it as a hard
  floor and never skip it when executing steps.

When following a guide: show the user the requirements and the step list before
doing anything, execute one step at a time, and stop on any mismatch between the
requirements and the local environment.

## Confirmation gate (mandatory)

Before any action that changes the user's machine or account (downloads to a
specific path, installs, uploads), present the full plan (what, where, which
steps) and get an explicit go-ahead. Never batch destructive or irreversible
steps silently.

## Not in this skill (yet)

- **Downloads**: returning metered, scanned download URLs requires a personal
  access token (PAT). When the platform enables it, this skill will add
  `omnicraft_request_download`.
- **Uploads**: one-sentence publishing will arrive with upload-scope tokens and
  its own confirmation gate.

## MCP alternative

If your runtime speaks MCP (Streamable HTTP), connect to
`$OMNICRAFT_BASE_URL/api/v1/mcp` and use the four anonymous tools:
`omnicraft_search`, `omnicraft_get_content`, `omnicraft_get_usage_guide`,
`omnicraft_list_categories`.
