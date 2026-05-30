# OmniCraft Beta Agent Entrypoints Implementation Plan

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

- [ ] **Step 1: Read UI specs**

```powershell
rg -n "## Component: AgentChatWidget|## Component: ContentDetail|## Page: /studio/publish" design/ui-spec.md
```

- [ ] **Step 2: Implement feature gate**

```tsx
export function AgentFeatureGate({
  capability,
  children,
}: {
  capability: "webAgent" | "desktopDeploy";
  children: React.ReactNode;
}) {}
```

Read `getPublicConfig()` with disabled-by-default fallback.

- [ ] **Step 3: Gate entrypoints**

- Hide global chat when `features.web_agent_enabled=false`.
- Hide AI search mode when `features.web_agent_enabled=false`.
- Hide publish `agent_enabled` toggle and detail-page one-click deploy when `features.desktop_deploy_enabled=false`.
- Keep backend feature checks authoritative even when frontend hides UI.

- [ ] **Step 4: Run checks and browser-test**

```powershell
cd frontend
npm run lint
npm run build
```

Use MCP Playwright with public config flags off and on. Save screenshots under `screenshots/beta-g01-*`.

- [ ] **Step 5: Commit**

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

- [ ] **Step 1: Write backend visibility test**

Seed vector-search hits containing published, deleted, banned-IP and private content. Assert NL search returns only content allowed by the shared visibility scope introduced in F-05.

- [ ] **Step 2: Change search defaults**

```ts
const [mode, setMode] = useState<"keyword" | "agent">("keyword");
```

Anonymous users must stay on keyword mode. Logged-in users may explicitly select Agent mode when the public feature flag is on.

- [ ] **Step 3: Add downgrade behavior**

When Agent search fails or is rate limited:

- Show an i18n notice.
- Execute normal keyword search with the same query.
- Preserve category, tag and sort filters.
- Never leave the page with an empty failed state if keyword search is available.

- [ ] **Step 4: Run checks and browser-test**

```powershell
cd backend
go test ./internal/service -run TestAgentSearchVisibility -v
go test ./...
cd ..\frontend
npm run lint
npm run build
```

Use MCP Playwright as anonymous and logged-in users, then simulate Agent failure. Save screenshots under `screenshots/beta-g02-*`.

- [ ] **Step 5: Commit**

```powershell
git add backend frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta G-02: add keyword-first Agent search downgrade - completed"
```

## Task G-03: Make Global Chat Contextual And Recoverable

**Files:**
- Create: `frontend/lib/agent-context.ts`
- Modify: `frontend/components/agent/AgentChatWidget.tsx`
- Modify: `frontend/lib/useSSE.ts`
- Modify: `backend/internal/handler/agent.go`
- Modify: `backend/internal/service/agent_service.go`
- Create: `backend/internal/service/agent_chat_test.go`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Define safe page context**

```ts
export interface AgentPageContext {
  route: string;
  contentId?: number;
  contentTitle?: string;
  contentType?: string;
}
```

Do not send cookies, tokens, local paths or raw page HTML.

- [ ] **Step 2: Send bounded conversation history**

The existing widget sends only the newest message. Send a bounded current-page message window plus safe page context. Persisting cross-page Agent chat is P1.

- [ ] **Step 3: Add quick prompts and downgrade CTA**

When Agent fails or rate limits:

- Keep the user's text visible.
- Show retry.
- Link to `/help`.
- Where relevant, offer normal search or normal download.

- [ ] **Step 4: Add backend validation**

Validate message count, role allowlist, per-message length and page-context fields. Reject unsupported role/tool payloads. Beta chat must never execute tools.

- [ ] **Step 5: Run checks and browser-test**

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

- [ ] **Step 6: Commit**

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

- [ ] **Step 1: Read specs**

```powershell
rg -n "## Component: ContentDetail|## Component: UsageGuidePanel" design/ui-spec.md
```

- [ ] **Step 2: Mount the existing panel**

Show usage-guide entry only when:

- Web Agent is enabled.
- Content is published and viewable.
- Content type benefits from a guide.

The panel must display retry and `/help` fallback.

- [ ] **Step 3: Run checks and browser-test**

```powershell
cd frontend
npm run lint
npm run build
```

Use MCP Playwright to open a mod or sheet-music detail page and request a guide. Save `screenshots/beta-g04-usage-guide.png`.

- [ ] **Step 4: Commit**

```powershell
git add frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta G-04: mount content usage guide - completed"
```

## Task G-05: Mount User-Confirmed Publish Assistance

**Files:**
- Modify: `frontend/components/studio/PublishForm.tsx`
- Modify: `frontend/components/agent/UploadAssistPanel.tsx`
- Modify: `frontend/components/agent/ComplianceCheckBadge.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Read specs**

```powershell
rg -n "## Component: UploadAssistPanel|## Component: ComplianceCheckBadge|## Page: /studio/publish" design/ui-spec.md
```

- [ ] **Step 2: Mount upload assistance**

Place metadata suggestions after upload and before submit. Suggestions must be previewed and applied only when the user clicks an explicit apply button.

- [ ] **Step 3: Mount compliance hint**

Show pre-submit compliance status. A hint may warn or block submit according to backend response, but it may not silently rewrite title, description or tags.

- [ ] **Step 4: Add graceful fallback**

When Agent is off or unavailable, the normal publishing form must remain usable.

- [ ] **Step 5: Run checks and browser-test**

```powershell
cd frontend
npm run lint
npm run build
```

Use MCP Playwright to upload, inspect suggestion, apply it, undo/edit it, and submit with Agent unavailable. Save screenshots under `screenshots/beta-g05-*`.

- [ ] **Step 6: Commit**

```powershell
git add frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta G-05: mount user-confirmed publish assistance - completed"
```
