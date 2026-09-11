# OmniCraft Agent Skill — reference

Version: v1 (SP-16 #449). Endpoints documented here mirror
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
Tools: `omnicraft_search`, `omnicraft_get_content`, `omnicraft_get_usage_guide`,
`omnicraft_list_categories`. Rate limit bucket: `mcp_per_minute`.

## Upcoming (requires a personal access token)

- `omnicraft_request_download` — metered, malware-scanned download URLs.
- One-sentence upload flow with a mandatory user confirmation gate.

Until then, treat this skill as strictly read-only.
