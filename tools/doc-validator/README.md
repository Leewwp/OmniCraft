# doc-validator

OmniCraft document validation and auto-generation tool.

## Usage

```bash
cd tools/doc-validator

# Auto-fix architecture.md auto-generated sections
go run . --fix

# Release-blocking checks (default profile)
go run . --check --profile release

# Check only git diff files
go run . --check --diff --profile release

# Historical archive cross-reference hygiene (non-release-blocking)
go run . --check --profile archive

# Release checks plus archive cross-references
go run . --check --profile all
```

## Validation Profiles

| Profile | Scope | Release blocking |
|---------|-------|------------------|
| `release` (default) | Config, schema, routes, design tokens, active-document cross-references, and working-document expiry | Yes |
| `archive` | Cross-references originating under `docs/archive/**` only | No |
| `all` | Complete `release` checks plus archive cross-references | No; diagnostic aggregate (use `release` for the release gate) |

The `release` profile excludes only cross-reference debt originating in `docs/archive/**`. Current documents under `docs/`, `design/`, `.specify/`, `docs/superpowers/specs/`, `docs/superpowers/plans/`, and `docs/working/` remain strictly checked.

## Checks

| Check | Description |
|-------|-------------|
| config sync | architecture.md §7 ↔ config.go mapstructure tags |
| schema sync | architecture.md §4 ↔ migrations/*.sql |
| route sync | architecture.md §3.2 ↔ routes.go |
| token refs | ui-spec.md tokens ↔ design-system.md definitions |
| cross refs | File-link validity, scoped by the selected validation profile |
| expired docs | docs/working/ expiry dates |

## Auto-Generated Sections

The tool manages three auto-generated sections in `architecture.md`, delimited by HTML comment markers:

- `<!-- AUTO-GENERATED: §3.2 API 路由清单 | ... -->` — Route table from `routes.go`
- `<!-- AUTO-GENERATED: §4 数据库 Schema | ... -->` — Table definitions from `migrations/*.sql`
- `<!-- AUTO-GENERATED: §7 配置字段注册表 | ... -->` — Config field table from `config/config.go`

Running `--fix` regenerates the content between these markers while preserving all hand-written content outside them.
