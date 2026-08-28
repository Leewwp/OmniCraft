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

## Status update (2026-08-28)

#207 T01/T02/T03 completed and the issue is closed. No Phase 1 core runtime
implementation task remains. The active work is #204/#32 evidence and packaging:
use the captured authenticated MiniMax Chat SSE trace, the corrected
outbox/Worker/`embo-01` embedding boundary, the current-v1 RAG diagnosis, and
the existing browser screenshots to finalize resume wording, demo flow, and
interview answers. A complete OpenSearch projection or read-alias cutover is
not required for this phase and is deferred to an isolated Phase 2 experiment.

## Delivery sequence

| Window | Focus | Exit evidence |
|---|---|---|
| Aug 27-28 | #207 T01/T02/T03 | Completed and closed; implementation evidence is available for downstream use |
| Aug 28-31 | #204 evidence reconciliation | Completed authenticated Chat and Worker/embedding evidence; retain explicit partial labels for citation fields and final projection |
| Sep 1-3 | #32 resume/demo closure | Authenticated browser/API walk-through, screenshots, resume wording and interview follow-up answers aligned to evidence grades |

## Evidence and wording guardrails

- Keep implemented behavior, local real-provider evidence, mocked contracts, and production evidence as separate grades.
- Do not claim production SLA, retrieval uplift, full Jaeger business-chain completion, or desktop/App capability.
- Real MiniMax checks may report provider/model, status, latency, token/cost summary, and trace identifiers only; never prompt bodies, raw responses, API keys, or GroupId values.
- Preserve existing HTTP/SSE event names, error normalization, visibility checks, fallback behavior, and compatibility shells.
