# Review: Beta Tasks F-05, F-06, G-02 — Search, Download, Agent Entrypoints

**Reviewer**: Automated code review
**Date**: 2026-06-02
**Commit**: `15dc57fe362a382f3559165ca9767354c38a2317`
**Branch**: `main` (ahead 30)

---

## 1. Build & Test Gate

| Check | Result |
|-------|--------|
| `go test ./... -short` | ✅ PASS (all 7 packages) |
| `go vet ./...` | ✅ PASS |
| `go build ./...` | ✅ PASS |
| `npm run lint` (frontend) | ✅ PASS |
| `npm run build` (frontend) | ✅ PASS (60 routes) |
| Focused: `TestDownload*` | ✅ 8/8 PASS, 1 SKIP (e2e) |
| Focused: `TestFilterMetadata*` | ✅ 7/7 PASS |
| Focused: `TestToTSQuery*` | ✅ 3/3 PASS |
| Focused: `TestContentVisibilityWhere*` | ✅ 2/2 PASS, 1 SKIP (DB) |
| Focused: `TestNLSearch*` | ✅ 2/2 PASS (source-level) |

**Browser testing**: Skipped — PostgreSQL/Redis/OSS services not running in review environment. Static analysis findings are sufficient.

---

## 2. Verification Checklist

### 2.1 Chinese keyword search hits titles and tags without spaces

**Verdict: ✅ PASS (with caveats)**

- [search_repo.go:121-187](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/repository/search_repo.go#L121-L187): `searchContentsWithQuery` uses three OR branches:
  1. `tsvector @@ to_tsquery('simple', ?)` — full-text prefix match
  2. `content_items.title ILIKE ?` — trigram-backed substring match (`%query%`)
  3. `EXISTS (... content_tags.tag ILIKE ?)` — tag substring match
- Migration [049_search_trigram_fallback.sql](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/migrations/049_search_trigram_fallback.sql) creates `idx_content_tags_tag_trgm` GIN index.
- Test seed [search_seed.sql](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/testdata/search_seed.sql) includes Chinese titles (`春日穿搭指南`) and tags (`桌面改造`).
- `toTSQuery` appends `:*` for prefix matching; `splitAndNormalize` keeps CJK characters intact.

**Caveat**: No integration test actually runs the SQL against a seeded DB. The `search_repo_test.go` only tests `toTSQuery` and `ContentVisibilityWhere` unit-level. The F-05 plan required `TestSearchContentsMatchesChineseSubstring` but this test does not exist in the repo.

### 2.2 keyword, suggestion, Agent hydration share visibility constraints

**Verdict: ⚠️ PARTIAL — Agent NL search visibility is incomplete**

| Code path | Published | Not soft-deleted | Author not banned | Author not deleted | IP not banned | is_public / viewer |
|-----------|-----------|-----------------|-------------------|-------------------|---------------|---------------------|
| Keyword search (`ApplyContentVisibilityScope`) | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
| Search suggestions (`SearchSuggestions`) | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ (public only) |
| Agent NL search (`NLSearch`) | ✅ | ❌ | ✅ | ❌ | ❌ | ✅ |

**Findings**:

1. **`ApplyContentVisibilityScope` does not check IP ban status** ([content_visibility.go:9-21](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/repository/content_visibility.go#L9-L21)). The visibility scope checks `status = published`, `deleted_at IS NULL`, author not banned/deleted, and `is_public`/viewer, but does NOT filter out content whose associated IP has `status = 'banned'`. Per AGENTS.md: "IP 被永久封禁时，关联内容全部下架". This is a **gap**.

2. **Agent `NLSearch` does not check `deleted_at` on content** ([agent_service.go:222-232](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/service/agent_service.go#L222-L232)). It only checks `c.Status != "published"` and `c.Author.IsBanned`, missing:
   - `c.DeletedAt` (soft-deleted content leaks)
   - Author `deleted_at` (deleted author's content leaks)
   - IP ban status

3. **`SearchSuggestions` duplicates visibility logic inline** ([search_repo.go:36-62](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/repository/search_repo.go#L36-L62)) instead of reusing `ApplyContentVisibilityScope`. The inline version hardcodes `ci.is_public = true` (no viewer context) which is correct for public suggestions, but the duplication risks divergence.

### 2.3 total count and result rows use same filter conditions

**Verdict: ✅ PASS**

- [search_repo.go:147-158](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/repository/search_repo.go#L147-L158): Count query and data query share the same `visibilityClause` and `filterClause`. The only difference is count omits `LIMIT`/`OFFSET` and `ORDER BY`.
- For the no-query path ([search_repo.go:77-118](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/repository/search_repo.go#L77-L118)), `baseQuery.Count(&total)` and `baseQuery.Find(&results)` share the same GORM scope.

### 2.4 All SQL parameterized, no user-input concatenation

**Verdict: ⚠️ PARTIAL — `ContentVisibilityWhere` uses `fmt.Sprintf`**

- [content_visibility.go:23-28](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/repository/content_visibility.go#L23-L28): `ContentVisibilityWhere` uses `fmt.Sprintf` to interpolate `viewerID` (int64) directly into the SQL string:
  ```go
  fmt.Sprintf("... content_items.author_id = %d", viewerID)
  ```
  While `viewerID` is an `int64` derived from authenticated context (not user input), this pattern violates the project's "禁止 SQL 字符串拼接" rule. It should use parameterized queries instead.

- All other SQL in `search_repo.go` uses `?` placeholders with parameterized args. ✅

### 2.5 Agent search not anonymous; failure preserves filters and degrades to keyword

**Verdict: ✅ PASS**

- [SearchAgentInput.tsx:19](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/components/agent/SearchAgentInput.tsx#L19): Default mode is `"keyword"`.
- [AgentFeatureGate.tsx:48-49](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/components/agent/AgentFeatureGate.tsx#L48-L49): `webAgent` requires `features.web_agent_enabled && !!user && !!user.email_verified_at`. Anonymous users see the keyword-only fallback.
- [SearchAgentInput.tsx:59-74](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/components/agent/SearchAgentInput.tsx#L59-L74): On 401/403/429/5xx/network error, shows i18n downgrade notice and calls `doKeywordSearch(trimmed)`.
- Backend route: [routes.go:292](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/handler/routes.go#L292): Agent endpoints require `authReq` + `agentGuard` (verified email + reputation).

**Minor gap**: The `SearchAgentInput` downgrade calls `onResults?.([], q)` then `onKeywordFallback?.(q)`, which triggers `doSearch(q, filterConfig)` in the search page. The current filter state is preserved because `filterConfig` is captured in the `doSearch` callback. ✅

### 2.6 All download CTAs go through GET /api/v1/contents/:id/download?attachment_id=...

**Verdict: ⚠️ PARTIAL — PDFViewer still uses direct oss_url link**

- [DownloadButton.tsx:49-53](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/components/content/DownloadButton.tsx#L49-L53): Correctly calls `/api/v1/contents/${contentId}/download?attachment_id=${attachmentId}`.
- [ContentDetail.tsx:259-267](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/components/content/ContentDetail.tsx#L259-L267): Attachment download uses `DownloadButton`. ✅
- [SheetMusicViewer.tsx:262-281](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/components/content/SheetMusicViewer.tsx#L262-L281): `DownloadPrompt` and `AttachmentRow` use `DownloadButton`. ✅

**FINDING — PDFViewer direct link**: [SheetMusicViewer.tsx:243-258](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/components/content/SheetMusicViewer.tsx#L243-L258):
```tsx
function PDFViewer({ ossUrl }: { ossUrl: string }) {
  return (
    <div className="space-y-2">
      <embed src={ossUrl} type="application/pdf" ... />
      <a href={ossUrl} target="_blank" rel="noopener noreferrer">
        <Button variant="outline" size="sm">{t("content.download")}</Button>
      </a>
    </div>
  );
}
```
The `<a href={ossUrl}>` is a **direct download CTA using `oss_url`**, violating F-06 Step 4 requirement. This should use `DownloadButton` instead. The `<embed>` for preview is acceptable if the backend provides a separate preview URL, but the download button must go through the authorization endpoint.

### 2.7 Download re-validates user, email, reputation, ban, content status, IP, allow_copy, attachment ownership

**Verdict: ⚠️ PARTIAL — Missing author-ban and IP-ban checks in handler**

[content.go:423-528](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/handler/content.go#L423-L528) `DownloadContent` handler checks:

| Check | Handler | Middleware |
|-------|---------|------------|
| Auth required | ✅ `callerID == 0` → 401 | ✅ `authReq` |
| Verified email | — | ✅ `downloadsGuard` |
| Reputation ≥ threshold | — | ✅ `downloadsGuard` |
| User not banned | — | ✅ `authReq` (RuntimeUserStatus) |
| Content published | ✅ `content.Status != "published"` → 403 | — |
| Content allow_copy | ✅ `!content.AllowCopy` → 403 | — |
| Attachment belongs to content | ✅ `ATTACHMENT_MISMATCH` | — |
| OSS configured | ✅ `ossSvc == nil` → 503 | — |
| **Author not banned/deleted** | ❌ NOT CHECKED | — |
| **IP not banned** | ❌ NOT CHECKED | — |

The download handler does not verify that the content's author is not banned/deleted, nor does it check if the content's associated IP is banned. These checks are part of the shared visibility scope but are not applied in the download path.

### 2.8 OSS URL TTL from config; count async after signing

**Verdict: ✅ PASS**

- [content.go:496-500](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/handler/content.go#L496-L500): `ttlSec := h.cfg.OSS.DownloadURLTTL` with fallback to 300.
- [config.yaml:33](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config.yaml): `download_url_ttl_sec: 300`.
- [config.go:80](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config/config.go#L80): `DownloadURLTTL int mapstructure:"download_url_ttl_sec"`.
- [content.go:508-523](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/handler/content.go#L508-L523): Download count is published asynchronously via `recovery.GoSafe` after signing succeeds. ✅

### 2.9 No attachment_id → unique primary; ambiguity → stable 400

**Verdict: ✅ PASS**

- [content.go:481-494](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/handler/content.go#L481-L494): When `attachment_id` is omitted, counts primary attachments. If exactly 1 → use it. If 0 or >1 → return `400 AMBIGUOUS_ATTACHMENT` with stable message. ✅

### 2.10 Frontend no longer uses oss_url as user download CTA

**Verdict: ❌ FAIL — PDFViewer still uses oss_url**

As documented in 2.6, `SheetMusicViewer.tsx` PDFViewer renders `<a href={ossUrl}>` as a download button. This is a direct `oss_url` download CTA that bypasses the authorization endpoint.

Additionally, the `grep` for `oss_url.*download|download.*oss_url|href.*oss_url` returned no matches, but this is because the PDFViewer uses `ossUrl` (camelCase variable) not the literal string `oss_url`. Manual code inspection confirms the violation.

---

## 3. Findings Summary

| # | Severity | Task | Finding | Location |
|---|----------|------|---------|----------|
| F05-1 | 🔴 HIGH | F-05 | `ContentVisibilityWhere` uses `fmt.Sprintf` to interpolate `viewerID` into SQL string, violating parameterized query rule | `content_visibility.go:25` |
| F05-2 | 🟡 MEDIUM | F-05 | `ApplyContentVisibilityScope` does not filter content with banned IP (`ips.status = 'banned'`) | `content_visibility.go:9-21` |
| F05-3 | 🔴 HIGH | F-05/G-02 | Agent `NLSearch` does not check content `deleted_at` or author `deleted_at`, leaking soft-deleted content and deleted-author content | `agent_service.go:222-232` |
| F05-4 | 🟡 MEDIUM | F-05 | `SearchSuggestions` duplicates visibility logic inline instead of reusing shared scope | `search_repo.go:36-62` |
| F05-5 | 🟡 MEDIUM | F-05 | Missing integration tests: `TestSearchContentsMatchesChineseSubstring`, `TestSearchContentsExcludesNonPublishedAndDeletedContent`, `TestAgentAndKeywordSearchShareVisibilityRules`, `TestSearchContentsCountsOnlyMatchedRows`, `TestSearchSuggestionsDoNotLeakHiddenContentTitles` do not exist | `search_repo_test.go` |
| F06-1 | 🔴 HIGH | F-06 | `PDFViewer` in `SheetMusicViewer.tsx` uses `<a href={ossUrl}>` as download CTA, bypassing authorization endpoint | `SheetMusicViewer.tsx:252` |
| F06-2 | 🟡 MEDIUM | F-06 | `DownloadContent` handler does not check if content author is banned/deleted or if content's IP is banned | `content.go:423-528` |
| G02-1 | 🟡 MEDIUM | G-02 | `agent_visibility_test.go` only does source-string scanning, not actual DB-backed visibility tests | `agent_visibility_test.go` |

---

## 4. Recommendations

1. **F05-1**: Replace `ContentVisibilityWhere` with a parameterized approach. Return clause fragments and args separately, or use GORM's `Where` builder consistently (as `ApplyContentVisibilityScope` already does). The `searchContentsWithQuery` raw-SQL path should accept `viewerID` as a query parameter.

2. **F05-2**: Add IP ban check to `ApplyContentVisibilityScope`:
   ```go
   db = db.Where("content_items.ip_id IS NULL OR content_items.ip_id NOT IN (SELECT id FROM ips WHERE status = 'banned')")
   ```

3. **F05-3**: Refactor `NLSearch` to use `ApplyContentVisibilityScope` or equivalent GORM-based filtering instead of manual field checks. At minimum, add `c.DeletedAt.IsZero()` and author `deleted_at` checks.

4. **F05-4**: Refactor `SearchSuggestions` to reuse `ApplyContentVisibilityScope` with `viewerID = 0` (public-only).

5. **F05-5**: Add the five missing integration tests from the F-05 plan using a disposable DB seeded with `search_seed.sql`.

6. **F06-1**: Replace `PDFViewer`'s `<a href={ossUrl}>` with a `DownloadButton` component. Keep `<embed src={ossUrl}>` for preview only if the backend provides a separate short-lived preview URL.

7. **F06-2**: Add author ban/deleted and IP ban checks to `DownloadContent` handler, or apply `ApplyContentVisibilityScope` (with IP ban fix) when fetching the content.

8. **G02-1**: Add DB-backed integration tests for Agent NL search visibility that actually seed test data and verify filtered results.
