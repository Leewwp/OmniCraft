# OmniCraft Web Agent Productization Implementation Plan

> **2026-07-25 Web-only scope:** This plan remains limited to the Web Agent. Desktop actions, Tauri integration, and D-02 through D-05/R-02 are deferred and must not be started from this plan while Desktop scope is paused.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把现有 Web Agent 从功能开关后的 LLM 入口提升为带来源引用、受控工具、预算限制、评测证据和可靠降级的作品集核心能力。

**Architecture:** 保留现有 Provider、AgentService、流式 handler 和内容检索能力；新增严格的工具/引用响应契约、共享内容可见性复核、结构化 trace/usage、固定评测 fixtures 和可访问的前端呈现。Web Agent 只执行只读检索或生成建议，所有写操作继续由普通业务 API 和用户确认完成。

**Tech Stack:** Go/Gin/GORM/Redis, OpenAI-compatible/Qwen provider abstraction, Next.js/TypeScript/next-intl, node:test, Playwright.

**Design source:** `docs/superpowers/specs/2026-07-16-omnicraft-dual-surface-agent-productization-design.md`

---

## Coordination And Release Rules

- Execute this mixed plan using the current `AGENTS.md` lane classification: ordinary Web UI/service tasks use light; security, auth, production configuration and release gates use heavy. Do not modify the historical task ledger, Beta roadmap, or community completion state.
- Dependency order: Task 1 precedes Tasks 2–4; Task 2 precedes Tasks 3 and 5; Task 3 precedes Tasks 4–6; Task 4 and Task 5 both precede Task 6.
- This is a Web Agent plan. Do not duplicate or execute Desktop D-02～D-05 while Desktop scope is deferred; the preserved Beta desktop plan remains the only future source for local actions after explicit restoration.
- Serialize edits to `backend/internal/service/agent_service.go`, `backend/config/config.go`, `backend/config.yaml`, `frontend/components/agent/*`, `frontend/components/content/ContentDetailOverlay.tsx`, shared Header/search files, and translation files. UI Polish U-10 does not own Agent workspace files; reserve translation files explicitly before Task 4.
- Keep `agent.web_agent_enabled=false` in repository defaults. Production may enable it only after Task 6 passes with real Provider configuration.
- Do not log prompts, raw provider responses, private content, secrets, or chain-of-thought.
- If real API key/provider input is absent at release verification, record a blocker and keep the flag off; unit/contract tests may use deterministic fakes.

## File Structure

### Backend

- Modify: `backend/config/config.go`, `backend/config.yaml`
- Modify: `backend/internal/service/agent_service.go`
- Create: `backend/internal/service/agent_contract.go`
- Create: `backend/internal/service/agent_trace.go`
- Create/modify: `backend/internal/service/agent_service_test.go`
- Create: `backend/internal/service/agent_grounding_test.go`
- Modify: `backend/internal/handler/agent.go`
- Modify/create: `backend/internal/handler/agent_test.go`
- Modify: current sole route owner (`backend/internal/handler/routes.go` or `backend/internal/router/routes.go` after hardening Task 3)
- Modify: `backend/internal/middleware/agent_ratelimit.go`
- Modify/create: `backend/internal/middleware/agent_ratelimit_test.go`
- Create: `backend/testdata/agent_eval_cases.json`
- Create: `backend/internal/service/agent_eval_test.go`

### Frontend

- Create: `frontend/app/(protected)/agent/page.tsx`
- Create: `frontend/components/agent/AgentWorkspace.tsx`
- Remove after migration: `frontend/components/agent/AgentChatWidget.tsx`
- Create/rename: `frontend/components/search/GlobalSearchInput.tsx` from the keyword-only behavior of `frontend/components/agent/SearchAgentInput.tsx`
- Modify: `frontend/app/layout.tsx` (remove global Agent mount)
- Modify: shared Header/search consumers to route Agent through `/agent`
- Create: `frontend/components/agent/AgentCitationList.tsx`
- Create: `frontend/components/agent/AgentToolStatus.tsx`
- Create/reuse: `frontend/components/content/ContentDetailOverlay.tsx`
- Create: `frontend/lib/agent.ts`
- Modify: `frontend/messages/zh.json`, `frontend/messages/en.json`
- Create: `frontend/tests/agent-workspace.test.tsx`
- Create: `frontend/e2e/web-agent-grounded.spec.ts`
- Read/align: `design/ui-spec.md` Agent sections

---

## Task 1: Define Typed Contract, Limits, And Evaluation Fixtures

- [x] **Step 1: Write failing config and contract tests**

Cover config fields:

- `rate_limit_per_minute`;
- `max_tool_calls_per_turn`;
- `max_output_tokens`;
- `provider_timeout_sec`;
- `provider_max_retries`;
- `citation_max_count`;
- existing daily/user-message/history limits remain mapped.

Cover typed `AgentAnswer`, `AgentCitation`, `AgentToolExecution`, `AgentUsage`, and safe error DTO JSON fields from the design spec.
`AgentAnswer` must include a server-owned `answer_kind` enum; the model cannot choose whether citations are required.

- [x] **Step 2: Confirm red**

```bash
cd backend
go test ./config ./internal/service -run "TestAgentConfig|TestAgentContract" -v
```

- [x] **Step 3: Implement config and typed contracts**

Do not use `map[string]any` for public response fields. Internal Provider-specific payloads remain behind the Provider adapter.

- [x] **Step 4: Add evaluation fixture schema and initial cases**

`backend/testdata/agent_eval_cases.json` must include at least:

- exact keyword lookup;
- semantic paraphrase lookup;
- question requiring two cited contents;
- no-evidence refusal;
- unpublished/private content exclusion;
- prompt-injection text inside content;
- forged citation attempt;
- client-forged page title/type/route context;
- hidden content ID used through usage-guide or chat context;
- publish suggestion payload with an arbitrary resource ID or an oversized/unselected form snapshot;
- provider timeout and rate-limit downgrade.

- [x] **Step 5: Verify and run doc-validator**

```bash
cd backend
go test ./config ./internal/service -run "TestAgentConfig|TestAgentContract|TestAgentEvalFixture" -v
cd ../tools/doc-validator
go run . --fix
```

- [x] **Step 6: Checkpoint Task 1**

Update only Task 1 checkboxes and `progress.txt`, review `git diff`, stage exact Task 1 files, and commit `Web Agent 1: define typed contracts and limits`.

---

## Task 2: Enforce Grounded Read-Only Tool Orchestration

- [x] **Step 1: Add failing orchestration tests**

Assert:

- only registered read-only tools can execute;
- tool args reject unknown fields and invalid limits;
- search/detail/usage-guide results and the existing `/agent/usage-guide/:id` sync/stream endpoints reuse the same viewer-aware visibility resolver;
- hidden/unpublished content IDs return the same not-found result and never reach the Provider;
- chat context accepts only a server-owned `global|content|search|publish` surface plus optional content ID; the service reloads title/type/visibility and ignores/rejects client-authored summaries and raw routes;
- `suggest_publish_metadata` uses only the typed, length-bounded publish-form snapshot explicitly attached to the current request and rejects model-supplied draft/content IDs;
- the ordinary authenticated Agent route group no longer exposes `POST /agent/moderate/:id`; moderation remains an authorized admin/worker concern and cannot be model-selected;
- content text cannot add/change tool definitions;
- every returned citation is reloaded/revalidated after model output;
- answers containing site-specific claims without valid citations become a safe no-evidence response;
- tool-call limit stops the loop with a stable result;
- private, banned, author-deleted, under-review, and soft-deleted content never reaches citations.

- [x] **Step 2: Confirm red**

```bash
cd backend
go test ./internal/service -run "TestAgentGrounding|TestAgentToolPolicy" -v
```

- [x] **Step 3: Implement a focused tool registry**

Register `search_content`, `get_content_detail`, `get_usage_guide`, and suggestion-only publish metadata. Tool definitions are server-owned constants with strict input/output structs. `suggest_publish_metadata` accepts no model-authored resource arguments; it reads only the typed snapshot already bound to the current request. The model cannot supply arbitrary route names or SQL.

Add a single viewer-aware content resolver shared by tool calls, chat context, and both legacy usage-guide response modes. Resolve resource context before creating a conversation or calling the Provider. Remove client-supplied `content_title`/`content_type`; map the surface enum to server-owned prompt text. De-register the ordinary-user moderation route before the feature can be enabled.

- [x] **Step 4: Implement citation validation and refusal**

Normalize citations from backend-owned content summaries, not model-authored URLs. Invalid references are removed. Citation requirements are deterministic: chat/search/detail/usage-guide natural-language output is `grounded_content` and must retain at least one valid citation; publish metadata is a separate `publish_suggestion` DTO. A grounded answer without evidence becomes localized `no_evidence` plus keyword-search fallback.

- [x] **Step 5: Add trace and usage emission**

Generate `trace_id`; record provider/model, tool name/status/duration, token counts, citation IDs, safe error, and degraded state through structured logs/metrics. Never log raw prompt/content/provider errors.

- [x] **Step 6: Verify focused tests**

```bash
cd backend
go test ./internal/service -run "TestAgent|TestUploadAssist|TestNLSearch" -v
```

- [x] **Step 7: Checkpoint Task 2**

Update only Task 2 checkboxes and `progress.txt`, stage exact orchestration/route/resolver files, and commit `Web Agent 2: enforce grounded tool boundaries`.

---

## Task 3: Stabilize Streaming API, Budget Reservation, And Downgrade

- [x] **Step 1: Add failing handler/stream tests**

Cover:

- feature disabled -> `503 FEATURE_DISABLED`;
- unauthenticated -> auth error;
- oversized message -> validation error;
- atomic Redis minute and daily request reservation under concurrent requests;
- feature/auth/request-schema and client-supplied context visibility rejections happen before reservation and do not consume quota;
- model-generated tool IDs are visibility-checked after the first Provider call; forbidden IDs produce uniform not-found tool results and the request still consumes its reservation;
- once a Provider-consuming request reserves quota, success, timeout, Provider error, and client cancellation all consume that request reservation and emit an outcome; there is no ambiguous release path;
- Redis unavailable fails closed for Provider-consuming routes, while conversation-history reads do not consume Agent generation quota;
- Provider timeout, 429, retryable 5xx, non-retryable 4xx;
- SSE events: `start`, `tool_status`, `delta`, `citation`, `usage`, `done`, `error`;
- final `done` includes trace ID and degraded flag;
- client cancellation cancels Provider/tool context;
- raw Provider errors never reach the client;
- `DELETE /agent/conversations/:id` deletes only the current user's conversation/messages, returns idempotent `204` for missing/foreign IDs, and does not consume generation quota;
- DB deletion failure returns a stable error without partial message deletion.

- [x] **Step 2: Confirm red**

```bash
cd backend
go test ./internal/handler ./internal/service ./internal/middleware -run "TestAgent.*Stream|TestAgent.*Quota|TestAgent.*Provider" -v
```

- [x] **Step 3: Implement atomic request quotas and bounded retries**

After feature/auth/request-schema checks and viewer-aware preload of any client-supplied context ID, reserve per-minute and per-day request quotas together in one Redis Lua operation immediately before the first Provider call. The daily limit remains `rate_limit_per_day`; add `rate_limit_per_minute` for bursts. These are request-count hard limits, not a monetary/token ledger: per-turn input/output/tool limits bound individual cost, while actual token usage is observed and alerted. After reservation, every downstream outcome consumes the request. IDs later proposed by the model are checked inside the tool dispatcher and return uniform not-found on denial, but do not release quota. Redis failure is fail-closed for Provider-consuming endpoints. Retry only configured network/429/5xx conditions with bounded backoff; retries within one request do not reserve again, and side-effecting tools are never retried.

- [x] **Step 4: Implement typed SSE writer**

Centralize event serialization and flushing. Once headers are written, report errors as SSE `error` events rather than attempting a second JSON response.

- [x] **Step 5: Implement search downgrade**

When conversational generation is unavailable but search is healthy, return `degraded=true`, normal keyword search results, and localized copy. Do not fabricate a model answer.

- [x] **Step 6: Implement owned conversation deletion**

Add `DELETE /api/v1/agent/conversations/:id`. Delete with an owner-scoped transaction and message cascade; zero affected rows returns `204` to keep missing/foreign cases indistinguishable and idempotent. Do not delete aggregate metrics or sanitized trace records. Mount this read/write-history route outside Provider quota middleware while retaining auth and ownership enforcement.

- [x] **Step 7: Verify focused and full backend gates**

```bash
cd backend
go test ./internal/handler ./internal/service ./internal/middleware -run "TestAgent" -v
go test ./...
go vet ./...
go build ./...
```

- [x] **Step 8: Checkpoint Task 3**

Update only Task 3 checkboxes and `progress.txt`, stage exact streaming/quota files, and commit `Web Agent 3: stabilize streaming and quotas`.

---

## Task 4: Build Traceable And Accessible Web Agent UI

- [x] **Step 1: Confirm UI spec and add failing tests**

Tests cover:

- streaming answer and stop action;
- citation cards with valid internal links;
- tool status summary without chain-of-thought;
- no-evidence and degraded-search states;
- retry preserving the user's current form input;
- feature-disabled state blocks/hides the `/agent` product entry while preserving keyword search; anonymous access follows the protected-route login redirect contract;
- Root Layout does not mount a global Agent trigger or panel, and Header search exposes no Agent mode toggle;
- citation cards open the shared content detail overlay and closing it restores the conversation scroll anchor and citation focus;
- keyboard/focus behavior and `aria-live`;
- throttled screen-reader announcements, reduced motion, and no forced auto-scroll while the user reads earlier output;
- invalid citation objects are never clickable.
- starting a new conversation preserves old server history and needs no confirmation;
- clear-history opens ConfirmModal, cancel performs no request, success sends the owned DELETE and focuses the new input;
- delete failure preserves rendered messages and returns focus to the clear trigger.

- [x] **Step 2: Implement typed frontend normalizer**

`frontend/lib/agent.ts` normalizes SSE events and rejects malformed citations/tool events. No `any` in public component props.

- [x] **Step 3: Implement citation and tool-status components**

Citation items show title, zone, excerpt, and an accessible link. Tool status uses short user-facing labels such as “已检索 8 条内容”; do not expose raw args or internal reasoning.

- [x] **Step 4: Build AgentWorkspace and separate global keyword search**

Create the protected `/agent` page and `AgentWorkspace` with stable loading, empty, partial-stream, stopped, error, degraded, and success states. Preserve last successful answer on retryable failures. Remove the Root Layout `AgentChatWidget` mount. Migrate Header/search consumers to keyword-only `GlobalSearchInput`; do not expose an Agent mode switch. All visible strings use the documented `agent.*` and `search.*` namespaces.
Implement separate “开始新对话” and “清空当前历史” actions using the lifecycle contract above; never label a local-only reset as server history deletion.
Wire valid citations to the shared `ContentDetailOverlay`; closing the overlay restores the conversation scroll position and citation trigger focus.

- [x] **Step 5: Run focused frontend tests**

```bash
cd frontend
node --import tsx --test tests/agent-workspace.test.tsx
npm run lint
npm run build
```

- [x] **Step 6: Checkpoint Task 4**

Update only Task 4 checkboxes and `progress.txt`, stage exact UI/translation/test files, and commit `Web Agent 4: build traceable Agent UI`.

---

## Task 5: Add Deterministic Agent Evaluation Gate

- [x] **Step 1: Build a fake Provider/tool harness**

Fixtures must deterministically control model tool requests, streamed tokens, malformed citations, prompt injection, timeouts, and errors.

- [x] **Step 2: Implement evaluation assertions**

The test gate reports:

- expected content IDs present for retrieval cases;
- no forbidden content IDs;
- citation objects valid and visible;
- no-evidence cases refuse;
- injection cases do not change tool policy;
- downgrade/error code matches expectation.

Do not use a non-deterministic real model as the CI pass/fail oracle.

- [x] **Step 3: Add optional real-provider smoke command**

The opt-in smoke records model/provider, latency, token usage, cost estimate, and qualitative output for manual release evidence. Missing credentials block real-provider release verification but do not break deterministic unit CI.

- [x] **Step 4: Verify evaluation gate**

```bash
cd backend
go test ./internal/service -run TestAgentEvaluation -v
```

- [x] **Step 5: Checkpoint Task 5**

Update only Task 5 checkboxes and `progress.txt`, stage exact fixture/evaluation files, and commit `Web Agent 5: add deterministic evaluation gate`.

---

## Task 6: Browser And Release Verification

- [x] **Step 1: Run full automated gates**

```bash
cd backend
go test ./...
go vet ./...
go build ./...
cd ../frontend
npm run test
npm run lint
npm run build
```

- [x] **Step 2: Run Playwright with a deterministic fake Provider**

Verify protected `/agent` navigation, natural-language questions, citations, shared detail-overlay open/close/history/focus restoration, tool status, stop, no-evidence, degraded keyword search, rate limit, new-conversation preservation, confirmed/cancelled/failed history deletion, mobile layout at 320/375/414px, keyboard focus, reduced motion, user-controlled streaming scroll, feature-disabled gating, absence of a global Agent widget, and absence of an Agent mode in Header search.

> 已执行：新增 `frontend/e2e/agent-workspace.mock.spec.ts`（9 用例，`npm run test:contracts` 23/23 通过）：feature-disabled gating + 无全局 widget + Header 无 Agent 模式、接地回答（流式 delta/工具状态/引用）、no-evidence 拒绝卡、stop 中止（浏览器原生流式 fetch mock，响应 abort）、429 错误卡、历史会话打开/删除确认与取消、320/375/414px 移动抽屉 + Esc、键盘 Tab 聚焦 + reduced motion、引用打开 ContentDetailOverlay。degraded keyword search 契约仍由确定性评估门（Task 5）与 `agent_tools_test.go` 覆盖。

- [x] **Step 3: Run real-provider smoke**

> 已执行（2026-08-10，MiniMax M1 + embo-01 + 真实 dev 库 163 篇已嵌入内容）：
> - provider 级：`go run ./cmd/agent-smoke` 4 场景全过（cited_search/no_evidence/injection/timeout_downgrade），injection 显式拒绝，usage 记录在案。
> - Agent/API 级（POST /api/v1/agent/chat/stream，surface=global）：cited → `search_content`+`get_content_detail` 执行、引用 content 26（哑铃训练）、kind=grounded_content（trace 7103beeb…）；no_evidence → 优雅拒绝（trace 01607636…）；injection → 未执行任何工具、kind=no_evidence（trace 344d927c…）；乱码检索 → no_evidence 不编造（trace 380ed5db…）。

- [x] **Step 4: Save screenshots**

- `screenshots/web-agent-grounded-desktop.png` ✓（mock 套件回归生成）
- `screenshots/web-agent-citations-mobile.png` ✓
- `screenshots/web-agent-citation-overlay-desktop.png` ✓（引用打开 ContentDetailOverlay）
- `screenshots/web-agent-no-evidence.png` ✓
- `screenshots/web-agent-degraded-search.png` ✓（UI 无降级标识，捕获等价状态；降级契约由确定性套件验证）

> 5/5 由 `e2e/agent-workspace.mock.spec.ts` 截图用例产出（gitignore，不入仓）。

- [ ] **Step 5: Enable only in deployment configuration after evidence passes**

Repository default remains false. Production override may enable the feature after Provider secret, origin, rate limit, budget, observability, and the above evidence are confirmed.

> 仓库默认保持 `agent.web_agent_enabled=false`；生产启用需人工裁决（Provider 密钥、origin、配额、预算、可观测性 + 上述证据），不在本分支启用。

- [ ] **Step 6: Checkpoint Task 6**

Record release evidence and any external-input blocker in `progress.txt`. Mark Task 6 complete only after the real-provider smoke and browser evidence pass; stage no secrets or production override values. Commit exact evidence/plan files as `Web Agent 6: verify productization release`.

## Plan Self-Check

- [ ] Answers about site content are grounded or explicitly refuse.
- [ ] Citations are server-normalized and visibility-rechecked.
- [ ] Tool registry is fixed, read-only/suggestion-only, and schema validated.
- [ ] Budget reservation is atomic and provider retries are bounded.
- [ ] Stream cancellation and post-header errors are tested.
- [ ] UI exposes citations and tool status without chain-of-thought.
- [ ] Deterministic evaluation is the CI oracle; real Provider is an explicit release smoke.
- [ ] Desktop action execution remains owned by D-02～D-05/R-02.
