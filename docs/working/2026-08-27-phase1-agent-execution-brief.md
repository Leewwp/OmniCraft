# OmniCraft Phase 1 Agent Execution Brief

> Created: 2026-08-27
> **预计失效日期**: 2026-09-10
> Scope: first-phase local interview and resume preparation; not a production plan

## User-confirmed inputs

- Interview window: approximately one week, target completion 2026-09-03 (Asia/Shanghai).
- Target roles, in priority order: AI application development, Agent development, AI full-stack development.
- #207 T01, T02 and T03 are all in scope. T01 remains heavy/TDD; T02 and T03 are light and follow T01.
- The repository-root `.env` has a real MiniMax API configuration. Values are local-only and must never be copied into source, commits, logs, screenshots, prompts, or evidence artifacts.

## Phase 1 delivery boundary

The first phase is a locally runnable, interview-ready Web experience: core page browsing, the protected Agent workspace, citation safety behavior, provider seams, the fixed tool set, and a reproducible live-demo path. It does not require cloud deployment or production credentials. #204 and #32 consume the implementation evidence; #208 remains the long-term roadmap.

## Delivery sequence

| Window | Focus | Exit evidence |
|---|---|---|
| Aug 27-29 | #207 T01 Citation Verification, heavy/TDD | Red tests first; server-owned citation identity, visibility, version/chunk checks, stable `no_evidence`; focused tests pass |
| Aug 30 | #207 T02 Provider seam, light | Agent depends only on streaming chat capability; fake adapter contract passes |
| Aug 31 | #207 T03 tool registry, light | Four-tool allowlist, validation, visibility and call limits remain unchanged |
| Sep 1 | Integration and repository gates | `go test ./...`, `go vet ./...`, `go build ./...`, relevant frontend checks, `bash scripts/verify-project.sh --full`, doc-validator when required |
| Sep 2-3 | #204/#32 evidence and demo closure | Authenticated browser/API walk-through, screenshots, resume wording and interview follow-up answers aligned to evidence grades |

## Evidence and wording guardrails

- Keep implemented behavior, local real-provider evidence, mocked contracts, and production evidence as separate grades.
- Do not claim production SLA, retrieval uplift, full Jaeger business-chain completion, or desktop/App capability.
- Real MiniMax checks may report provider/model, status, latency, token/cost summary, and trace identifiers only; never prompt bodies, raw responses, API keys, or GroupId values.
- Preserve existing HTTP/SSE event names, error normalization, visibility checks, fallback behavior, and compatibility shells.
