# OmniCraft Agent Skill — reference

Version: v2 (SP-16 #449 read surface, #451 PAT download/upload). Endpoints documented here mirror
`GET /api/v1/openapi.json` (OpenAPI 3.1, the authoritative contract).

## Anonymous read surface

| Endpoint | Purpose | Notes |
|---|---|---|
| `GET /contents/search?q=` | keyword search | 30 req/min per IP; `type` filter |
| `GET /contents?page=&page_size=` | listing | 300 req/min per IP bucket |
| `GET /contents/{id}` | detail + attachments | signed display URLs ~1h |
| `GET /contents/{id}/versions` | public version lineage | |
| `GET /contents/{id}/guide?locale=zh|en` | merged usage guide | ETag + s-maxage=300 |
| `GET /categories` | taxonomy | |
| `GET /tags/faceted` | tag facets | |
| `GET /ips` | approved IP hubs | |
| `GET /users/{id}` | public profile | no email |
| `GET /stats/summary` | site counters | |

All anonymous responses contain **published, public content only** — drafts,
private, in-review and banned items never appear.

## Caching

Anonymous GETs carry `Cache-Control: public, max-age=60, s-maxage=300` and an
`ETag`; send `If-None-Match` to revalidate cheaply (304).

## Versioning

Within v1 no field is removed or repurposed. Breaking changes ship as v2 with a
deprecation window — watch `GET /openapi.json` (`info.version`) for the
announcement.

## MCP (alternative transport)

Streamable HTTP endpoint: `POST /api/v1/mcp` (also GET/DELETE for the protocol).
Rate limit bucket: `mcp_per_minute` (per IP) plus the PAT per-token window.

Tool availability is honest about the token:

| Token | Tools |
|---|---|
| none (anonymous) | `omnicraft_search`, `omnicraft_get_content`, `omnicraft_get_usage_guide`, `omnicraft_list_categories` |
| PAT `download` scope | + `omnicraft_request_download` |
| PAT `upload` scope | + `omnicraft_suggest_publish_metadata`, `omnicraft_create_content`, `omnicraft_request_upload_url` |

Send `Authorization: Bearer oc_pat_...` on every request; the tool set is bound
at session initialize time, so reconnect after changing tokens. Every write
tool re-checks the scope per call — a download-only token can never publish.

## Authenticated REST equivalents (PAT)

| Endpoint | Scope | Purpose |
|---|---|---|
| `GET /contents/{id}/download` | download | same semantics as the MCP tool: visibility + malware gate + metered count, ~5 min signed URL |
| `POST /contents` | upload | create draft (enters standard review, status `pending`) |
| `POST /contents/oss-token` | upload | pre-signed PUT + grant (same as `omnicraft_request_upload_url`) |

PAT requests never create a web session and never receive cookies; token
management (create/revoke) is web-session-only by design — a leaked PAT cannot
mint more tokens.
