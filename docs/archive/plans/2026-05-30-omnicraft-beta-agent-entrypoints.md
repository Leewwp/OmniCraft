# OmniCraft Beta Agent Entrypoints Implementation Plan

> **归档说明**：G-01~G-05 全部完成（roadmap 状态表全 [x]）。2026-07-23 近完成清零归档。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a controlled, understandable Agent experience that degrades cleanly to normal community features.

**Architecture:** Keep Beta Agent capabilities narrow: global chat, search enhancement, content usage guide, upload metadata suggestions and compliance hints. Read runtime feature flags from the public-config endpoint, require normal backend authorization, send page context explicitly, and never permit direct local file actions or silent content writes.

**Tech Stack:** Next.js App Router, React, `next-intl`, Go Agent APIs, SSE, MCP Playwright.

---

## File Structure

- Modify: `frontend/app/layout.tsx`.
- Modify: `frontend/components/agent/AgentChatWidget.tsx`.
- Modify: `frontend/components/agent/SearchAgentInput.tsx`.
- Modify: `frontend/components/agent/UsageGuidePanel.tsx`.
- Modify: `frontend/components/agent/UploadAssistPanel.tsx`.
- Modify: `frontend/components/agent/ComplianceCheckBadge.tsx`.
- Modify: `frontend/components/content/ContentDetail.tsx`.
- Modify: `frontend/components/studio/PublishForm.tsx`.
- Create: `frontend/components/agent/AgentFeatureGate.tsx`.
- Create: `frontend/lib/agent-context.ts`.
- Modify: `frontend/lib/useSSE.ts`.
- Modify: `frontend/messages/zh.json`, `frontend/messages/en.json`.
- Modify: `backend/internal/service/agent_service.go`.
- Modify: `backend/internal/handler/agent.go`.
- Add focused backend tests.

## Task G-01: Gate Global Agent Chat And Desktop Deploy UI

**Files:**
- Create: `frontend/components/agent/AgentFeatureGate.tsx`
- Modify: `frontend/app/layout.tsx`
- Modify: `frontend/components/agent/AgentChatWidget.tsx`
- Modify: `frontend/components/content/ContentDetail.tsx`
- Modify: `frontend/components/studio/PublishForm.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [x] **Step 1: Read UI specs**

```powershell
rg -n "## Component: AgentChatWidget|## Component: ContentDetail|## Page: /studio/publish" design/ui-spec.md
```

**File conflict note:** `ContentDetail.tsx` is modified by F-06, G-01, G-04 and D-05. Execute dependent tasks after rebasing onto the integrated branch. Before editing `ContentDetail.tsx`, re-read the current file state to ensure you are working with the latest merged version.

- [x] **Step 2: Implement feature gate**

```tsx
export function AgentFeatureGate({
  capability,
  children,
  fallback,
}: {
  capability: "webAgent" | "desktopDeploy";
  children: React.ReactNode;
  fallback?: React.ReactNode;
}) {}
```

Read `fetchPublicConfig()` with disabled-by-default fallback. Added optional `fallback` prop for graceful degradation.

- [x] **Step 3: Gate entrypoints**

- Hide global chat when `features.web_agent_enabled=false`.
- Hide AI search mode when `features.web_agent_enabled=false`.
- Hide publish `agent_enabled` toggle and detail-page one-click deploy when `features.desktop_deploy_enabled=false`.
- Hide global chat and AI-search selection for anonymous or unverified-email users. The backend interaction guard remains authoritative.
- Keep backend feature checks authoritative even when frontend hides UI.

- [x] **Step 4: Run checks and browser-test**

```powershell
cd frontend
npm run lint
npm run build
```

Use MCP Playwright with public config flags off and on. Save screenshots under `screenshots/beta-g01-*`.

- [x] **Step 5: Commit**

```powershell
git add frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta G-01: gate Agent and desktop entrypoints - completed"
```

## Task G-02: Default Search To Keyword And Degrade Agent Failures

**Files:**
- Modify: `frontend/components/agent/SearchAgentInput.tsx`
- Modify: `frontend/app/(public)/search/page.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Modify: `backend/internal/service/agent_service.go`
- Create: `backend/internal/service/agent_visibility_test.go`

- [x] **Step 1: Write backend visibility test**

Seed vector-search hits containing published, deleted, banned-IP and private content. Assert NL search returns only content allowed by the shared visibility scope introduced in F-05.

- [x] **Step 2: Change search defaults**

```ts
const [mode, setMode] = useState<"keyword" | "agent">("keyword");
```

Anonymous users and unverified-email users must stay on keyword mode. Logged-in verified users may explicitly select Agent mode when the public feature flag is on.

- [x] **Step 3: Add downgrade behavior**

When Agent search returns `401`, `403`, `429`, `503` or a network error:

- Show an i18n notice.
- Execute normal keyword search with the same query.
- Preserve category, tag and sort filters.
- Never leave the page with an empty failed state if keyword search is available.

- [x] **Step 4: Run checks and browser-test**

```powershell
cd backend
go test ./internal/service -run TestAgentSearchVisibility -v
go test ./...
cd ..\frontend
npm run lint
npm run build
```

Use MCP Playwright as anonymous and logged-in users, then simulate Agent failure. Save screenshots under `screenshots/beta-g02-*`.

- [x] **Step 5: Commit**

```powershell
git add backend frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta G-02: add keyword-first Agent search downgrade - completed"
```

## Task G-03: Make Global Chat Contextual And Recoverable

**Files:**
- Modify: `backend/config/config.go`
- Modify: `backend/config.yaml`
- Create: `frontend/lib/agent-context.ts`
- Modify: `frontend/components/agent/AgentChatWidget.tsx`
- Modify: `frontend/lib/useSSE.ts`
- Modify: `backend/internal/handler/agent.go`
- Modify: `backend/internal/service/agent_service.go`
- Create: `backend/internal/service/agent_chat_test.go`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [x] **Step 1: Define safe page context**

```ts
export interface AgentPageContext {
  route: string;
  contentId?: number;
  contentTitle?: string;
  contentType?: string;
}
```

Do not send cookies, tokens, local paths or raw page HTML.

- [x] **Step 2: Send bounded conversation history**

The existing widget sends only the newest message. Send at most the latest 10 current-page messages plus safe page context. Cap each message at the backend-configured length limit (`agent.max_user_message_chars`, default `4000`) and reject unsupported roles. **Beta chat scope:** Agent chat state is per-page-mount lifecycle. Navigating away from the page clears the conversation history. Cross-page chat persistence is P1 and is listed in the roadmap's "Deferred P1 Scope".

- [x] **Step 3: Add quick prompts and downgrade CTA**

When Agent fails or rate limits:

- Keep the user's text visible.
- Show retry.
- Link to `/help`.
- Link to `/feedback`.
- Where relevant, offer normal search or normal download.

Add the frozen quick-prompt intents below. Render their labels through `next-intl`; do not hardcode these English strings in TSX:

- `page_help`
- `download_help`
- `publish_help`
- `desktop_client_help`
- `report_problem`

- [x] **Step 4: Add backend validation**

Validate message count, role allowlist, per-message length and page-context fields. Reject unsupported role/tool payloads and unknown context fields. Beta chat must never execute tools, write content or return permanent OSS URLs.

- [x] **Step 5: Run checks and browser-test**

```powershell
cd backend
go test ./internal/service -run TestAgentChat -v
go test ./...
go vet ./...
go build ./...
cd ..\frontend
npm run lint
npm run build
```

Use MCP Playwright to send multiple messages, inspect fallback state and save `screenshots/beta-g03-chat.png`.

- [x] **Step 6: Commit**

```powershell
git add backend frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta G-03: add contextual recoverable Agent chat - completed"
```

## Task G-04: Mount Usage Guide On Content Detail

**Files:**
- Modify: `frontend/components/content/ContentDetail.tsx`
- Modify: `frontend/components/agent/UsageGuidePanel.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [x] **Step 1: Read specs**

```powershell
rg -n "## Component: ContentDetail|## Component: UsageGuidePanel" design/ui-spec.md
```

- [x] **Step 2: Mount the existing panel**

Show usage-guide entry only when:

- Web Agent is enabled.
- Content is published and viewable.
- Content type is `mod` or `sheet_music` for the Beta allowlist. Add more types only through an explicit product decision.

The panel must display retry, `/help` and `/feedback` fallback. Render Markdown through the existing safe `MarkdownRenderer`; do not enable raw HTML.

- [x] **Step 3: Run checks and browser-test**

```powershell
cd frontend
npm run lint
npm run build
```

Use MCP Playwright to open a mod or sheet-music detail page and request a guide. Save `screenshots/beta-g04-usage-guide.png`.

- [x] **Step 4: Commit**

```powershell
git add frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta G-04: mount content usage guide - completed"
```

## Task G-05: Mount User-Confirmed Publish Assistance

**Files:**
- Modify: `frontend/components/studio/PublishForm.tsx`
- Modify: `frontend/components/agent/UploadAssistPanel.tsx`
- Modify: `frontend/components/agent/ComplianceCheckBadge.tsx`
- Modify: `backend/internal/handler/agent.go`
- Modify: `backend/internal/service/agent_service.go`
- Create: `backend/internal/service/agent_publish_assist_test.go`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [x] **Step 1: Read specs**

```powershell
rg -n "## Component: UploadAssistPanel|## Component: ComplianceCheckBadge|## Page: /studio/publish" design/ui-spec.md
```

- [x] **Step 2: Mount upload assistance**

Place metadata suggestions after upload and before submit. Reuse the `frontend/components/agent/*` panels, not the similarly named static `frontend/components/content/*` hints. Suggestions must be previewed and applied only when the user clicks an explicit apply button. Preserve an undo snapshot so the user can revert the applied suggestion. **Undo implementation:** Use `useRef` to store a `structuredClone(formState)` snapshot immediately before applying a suggestion. The apply button saves the snapshot to the ref; the undo button restores form state from the ref. Only one level of undo is required for Beta.

- [x] **Step 3: Mount compliance hint**

Show pre-submit compliance status using a stable backend enum: `safe`, `warning`, or `violation`. Apply `safe` suggestions only after the user's explicit apply click; require explicit acknowledgement before applying a `warning`; block application and final submit for `violation`. A hint may not silently rewrite title, description or tags. Final publish still goes through the existing content-review path.

- [x] **Step 4: Validate suggestions as untrusted input**

Validate suggestion field lengths, tag count/tag lengths and category enum in the backend before returning structured suggestions. Validate again before applying to form state. Add tests for oversized text, invalid category, excessive tags and unknown fields.

- [x] **Step 5: Add graceful fallback**

When Agent is off or unavailable, the normal publishing form must remain usable.

- [x] **Step 6: Run checks and browser-test**

```powershell
cd backend
go test ./internal/service -run TestAgentPublishAssist -v
go test ./...
cd ..\frontend
npm run lint
npm run build
```

Use MCP Playwright to upload, inspect suggestion, apply it, undo/edit it, and submit with Agent unavailable. Save screenshots under `screenshots/beta-g05-*`.

- [x] **Step 7: Commit**

```powershell
git add backend frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta G-05: mount user-confirmed publish assistance - completed"
```