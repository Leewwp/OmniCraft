# Archive

Documents in this directory are preserved for historical reference only. They are **not** part of the active development documentation set.

## Archive Policy

- Archived documents describe completed work, superseded designs, or deprecated specifications.
- Do **not** reference archived documents from active plans, architecture docs, or AGENTS.md.
- Before archiving, verify the document is no longer referenced by any active document.

## Archived Files

| File | Archived | Reason |
|------|----------|--------|
| `UI Design.md` | 2026-06-24 | Superseded by `design/design-system.md` and `design/ui-spec.md` (DEC-013) |
| `homepage-v0.html` | 2026-06-24 | Prototype homepage, superseded by Next.js implementation (DEC-011) |
| `CURRENT_TASK_HANDOFF_*.md` | 2026-06-28 | Handoff task (original/fanwork linking) completed; migration 036 created |

## How to Archive

1. Move the file into `docs/archive/`.
2. Add an entry to the table above with date and reason.
3. Verify no active documents still reference the archived file (`grep -r "filename" docs/ .specify/ architecture.md CLAUDE.md`).
4. If references remain, update them to point to the replacement document or remove them.
