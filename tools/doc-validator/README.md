# doc-validator

OmniCraft document validation and auto-generation tool.

## Usage

```bash
# Auto-fix architecture.md auto-generated sections
go run tools/doc-validator --fix

# Full consistency check
go run tools/doc-validator --check

# Check only git diff files
go run tools/doc-validator --check --diff
```

## Checks

| Check | Description |
|-------|-------------|
| config sync | architecture.md §7 ↔ config.go mapstructure tags |
| schema sync | architecture.md §4 ↔ migrations/*.sql |
| route sync | architecture.md §3.2 ↔ routes.go |
| token refs | ui-spec.md tokens ↔ design-system.md definitions |
| cross refs | File links validity |
| expired docs | docs/working/ expiry dates |

## Auto-Generated Sections

The tool manages three auto-generated sections in `architecture.md`, delimited by HTML comment markers:

- `<!-- AUTO-GENERATED: §3.2 API 路由清单 | ... -->` — Route table from `routes.go`
- `<!-- AUTO-GENERATED: §4 数据库 Schema | ... -->` — Table definitions from `migrations/*.sql`
- `<!-- AUTO-GENERATED: §7 配置字段注册表 | ... -->` — Config field table from `config/config.go`

Running `--fix` regenerates the content between these markers while preserving all hand-written content outside them.
