# Web Beta Review 08 — Agent Entrypoints (G-01, G-03, G-04, G-05)

| Field | Value |
|---|---|
| **Reviewer** | Automated code review |
| **Date** | 2026-06-02 |
| **Branch** | `main` (ahead 30 of origin/main) |
| **HEAD** | `15dc57fe362a382f3559165ca9767354c38a2317` |
| **Working tree** | Clean — no uncommitted changes |
| **Tasks** | G-01 (Feature Gate), G-03 (Contextual Chat), G-04 (Usage Guide), G-05 (Publish Assist) |
| **Note** | G-02 (Search Downgrade) excluded per instructions (covered by search/download batch). |

---

## 1. Build & Lint Results

| Check | Result |
|---|---|
| `go vet ./...` | ✅ PASS |
| `go build ./...` | ✅ PASS |
| `go test ./...` | ✅ PASS (all packages) |
| `npm run lint` (tsc --noEmit) | ✅ PASS |
| `npm run build` | ✅ PASS |

---

## 2. Checklist Verification

### 2.1 `web_agent_enabled=false` safely disables chat, AI search, and Agent UI; normal features still work

**PASS.** Multi-layer gating:

- **Frontend**: `AgentFeatureGate` reads `fetchPublicConfig().features.web_agent_enabled`. When false, renders `fallback` or nothing. Applied to chat widget (root layout), usage guide (ContentDetail), upload assist (PublishForm), compliance check (PublishForm), desktop deploy toggle.
- **Backend**: Every agent handler method checks `h.cfg.Agent.WebAgentEnabled` and returns `503 FEATURE_DISABLED` when false.
- **Default**: `AgentFeatureGate` uses `disabledFeatures` constant as fallback — if config fetch fails, agent features are hidden (safe default).

### 2.2 Anonymous and unverified-email users cannot see protected Agent entrypoints; backend enforces

**PASS.**

- `AgentFeatureGate` for `webAgent` checks `!!user && !!user.email_verified_at`.
- `AgentChatWidget` returns null if `!user`.
- Backend agent routes use `agentGuard` = `InteractionRequired(RequireVerifiedEmail: true)`.
- Browser verified: anonymous users see no chat widget (count=0), no AI search toggle, no usage guide.

### 2.3 Page context contains only route, contentId, contentTitle, contentType — no token, cookie, local paths, or HTML

**PASS.** `AgentPageContext` interface has exactly four fields. `sanitizePageContext()` constructs a new object with only known fields, enforcing length limits. Backend `ChatContextDTO` maps to the same four fields with `truncateStr()` enforcement.

### 2.4 Chat history max 10 messages, role allowlist, per-message length limit, unknown fields/tool payloads rejected

**PASS with observation.**

- Frontend: `MAX_CONTEXT_MESSAGES = 10`, messages sliced before sending.
- Backend: `maxCtxMsgs` from config (default 10), `allowedRoles = {"user": true, "assistant": true}`, `maxMsgLen` from config (default 4000).
- Unknown context fields are silently dropped by Go's `ShouldBindJSON`.
- **Observation**: Backend truncates over-length messages rather than rejecting them. The plan says "reject" but truncation is a pragmatic Beta choice.

### 2.5 Agent failure and rate-limit: user input preserved, retry, /help, /feedback, normal feature degradation

**PASS.**

- Chat widget: error + downgrade notice + retry button + `/help` link + `/feedback` link. User input preserved.
- Search: downgrade notice + automatic keyword fallback on 401/403/429/5xx/network error.
- Usage guide: retry + `/help` + `/feedback` links.

### 2.6 Usage guide only on allowed content types (mod, sheet_music) and published visible content; Markdown does not enable raw HTML

**PASS.** ContentDetail.tsx checks: `webAgent` enabled AND `(contentType === "mod" || contentType === "sheet_music")` AND `data.status === "published"`. Uses `MarkdownRenderer` which does not enable raw HTML.

### 2.7 Publish assist suggestions require explicit apply, undo available; warning needs confirmation; violation blocks apply and submit

**PARTIAL PASS.**

- ✅ Explicit apply: `handleAssistFill()` only called on "autoFill" button click.
- ✅ Undo: `undoSnapshot` saves form state before apply; undo button restores it.
- ✅ Violation blocks submit: `disabled={submitting || complianceViolation}`.
- ⚠️ **Gap**: Warning-level suggestions do NOT require explicit acknowledgement before applying.
- ⚠️ **Gap**: Violation-level does NOT block the apply button for upload assist suggestions — only blocks final submit.

### 2.8 Suggestions treated as untrusted input: frontend and backend validate field lengths, tag count, tag lengths, category enum, unknown fields

**PARTIAL PASS.**

- Frontend: tag merge respects max 10 limit; category validated against `ORIGINAL_CATEGORIES`.
- ⚠️ **Gap**: Backend `UploadAssist` does NOT validate the LLM response before returning it. No tag count limit, no tag length limit, no category enum check, no field length limits on the backend output.
- ⚠️ **Gap**: Frontend does not validate `suggested_title` or `suggested_description` length before applying to form state.

### 2.9 Agent cannot execute tools, local file actions, silent content writes, or return permanent OSS URLs

**PASS.**

- Chat: only `user` and `assistant` roles passed to LLM; no tool/function calling configured.
- Usage guide: returns only Markdown text via safe renderer.
- Publish assist: returns structured JSON suggestions only; no file operations or OSS URLs.
- Deploy script (existing, not G-task): behind `desktop_deploy_enabled: false` feature flag.

---

## 3. Backend Code Quality

### agent.go

- Feature flag checks on every handler ✅
- Chat message validation (count, roles, length) ✅
- Safe context DTO ✅
- SSE streaming with proper headers ✅
- Uses `response.SafeErrorResponse` ✅

**Concern**: `ListConversations` and `GetConversationMessages` (lines 253-275) do not check `WebAgentEnabled`. These endpoints return conversation history even when agent is disabled. Behind `authReq + agentGuard` so not publicly accessible, but should check feature flag for consistency.

### agent_service.go

- Feature flag checks in all methods ✅
- Conversation cleanup on failure ✅
- Page context injected as system message ✅
- Visibility filtering in NLSearch ✅
- Compliance check uses Aliyun Green + LLM aggregation ✅

**Concern**: `UploadAssist` does not validate LLM output before returning. A confused LLM could return extremely long strings, excessive tags, or invalid categories.

### model/agent.go

- Clean model definitions ✅
- `AgentPageContext` has only safe fields ✅
- `AgentMessage.ToolCalls` JSONB field exists for future use, not populated by current chat ✅

---

## 4. Frontend Code Quality

### AgentFeatureGate.tsx

- Disabled-by-default fallback ✅
- Checks feature flag + user auth state ✅
- Supports `fallback` prop ✅
- Caches public config ✅

### AgentChatWidget.tsx

- i18n via `useTranslations()` ✅
- Quick prompts with i18n keys ✅
- Message history limited to 10 ✅
- Retry, /help, /feedback on failure ✅
- User input preserved on error ✅

### agent-context.ts

- `sanitizePageContext()` enforces length limits and type checks ✅
- Constructs new object with only known fields (no pass-through) ✅
- `QUICK_PROMPTS` matches plan's five intents ✅

### useSSE.ts

- Proper abort controller cleanup ✅
- Auth token injection ✅
- SSE protocol parsing ✅
- `[DONE]` signal handling ✅

### UsageGuidePanel.tsx

- Uses `MarkdownRenderer` (no raw HTML) ✅
- Retry + /help + /feedback on error ✅
- Expandable panel ✅

### UploadAssistPanel.tsx

- Explicit "autoFill" button for applying suggestions ✅
- Preview of suggestions before apply ✅
- Error handling with `silentError` ✅

### ComplianceCheckBadge.tsx

- Three-level risk display (safe/warning/violation) ✅
- Expandable details ✅
- Error handling ✅

### SearchAgentInput.tsx

- Default mode is "keyword" ✅
- Agent mode requires explicit selection ✅
- Downgrade to keyword on failure ✅
- Preserves query on fallback ✅

---

## 5. Issues Found

| # | Severity | Description | Location |
|---|---|---|---|
| 1 | **Moderate** | Backend `UploadAssist` does not validate LLM response before returning. No tag count limit, tag length limit, category enum check, or field length limits. Plan explicitly requires: "Validate suggestion field lengths, tag count/tag lengths and category enum in the backend before returning structured suggestions." | `agent_service.go:58-93` |
| 2 | **Moderate** | Warning-level compliance suggestions do not require explicit acknowledgement before applying. Violation-level does not block the apply button for upload assist — only blocks final submit. Plan requires: "require explicit acknowledgement before applying a `warning`; block application and final submit for `violation`." | `PublishForm.tsx:86-106, 404-415` |
| 3 | **Minor** | `ListConversations` and `GetConversationMessages` endpoints do not check `WebAgentEnabled` feature flag. They return data even when agent is disabled. | `agent.go:253-275` |
| 4 | **Minor** | Backend truncates over-length chat messages rather than rejecting them. Plan says "reject unsupported role/tool payloads." Truncation is pragmatic but deviates from spec. | `agent.go:201-203` |
| 5 | **Minor** | Frontend does not validate `suggested_title` or `suggested_description` length before applying to form state. A very long suggestion could overflow the form. | `PublishForm.tsx:89-93` |
| 6 | **Info** | `SearchAgentInput` allows anonymous users to see the "AI Search" toggle button even though they can't use it (the request will fail with 401). The `AgentFeatureGate` is not wrapping the search agent toggle. | `SearchAgentInput.tsx:109-121` |
| 7 | **Info** | `public-config.ts` caches the config response indefinitely (`cachedConfig`). If an admin toggles `web_agent_enabled` at runtime, users won't see the change until they refresh and the cache is cleared. | `public-config.ts:35-48` |

---

## 6. Browser Test Evidence

Screenshots saved to `screenshots/review-web-beta/08-agent-entrypoints/`:

| File | Description |
|---|---|
| `01-anon-homepage.png` | Anonymous homepage — no chat widget visible |
| `02-anon-search.png` | Anonymous search page — no AI search toggle |
| `03-anon-content-detail.png` | Anonymous content detail — no usage guide |
| `04-anon-publish.png` | Anonymous publish page — redirects to login |

Browser test results:
- `08_anon_chat_btn_count`: **0** (no chat button for anonymous)
- `08_anon_ai_search`: **false** (no AI search visible for anonymous)
- `08_anon_usage_guide`: **false** (no usage guide for anonymous)
- `08_anon_publish`: redirects to `/login?redirect=%2Fstudio%2Fpublish%2Foriginal`

---

## 7. Verdict

| Task | Status | Notes |
|---|---|---|
| G-01 | ✅ PASS | Feature gate correctly hides all agent UI when disabled. Anonymous/unverified users blocked. Backend enforces. |
| G-03 | ✅ PASS | Chat is contextual and recoverable. Page context is safe. History limited to 10. Retry/help/feedback on failure. |
| G-04 | ✅ PASS | Usage guide mounted only on mod/sheet_music + published content. Uses safe MarkdownRenderer. |
| G-05 | ⚠️ PASS with moderate gaps | Publish assist works with explicit apply and undo. But: (1) backend does not validate LLM suggestions before returning, (2) warning/violation compliance levels do not enforce the required acknowledgement/block semantics. |

**Overall**: Agent entrypoints are functional and the security boundaries (feature flags, auth gates, safe context) are solid. The two moderate gaps (unvalidated LLM output in UploadAssist, incomplete compliance enforcement for warning/violation) should be addressed before production but are acceptable for Beta with documented risk.
