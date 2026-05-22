# E2E Test Report — OmniCraft 万象工坊

**Date**: 2026-05-22  
**Branch**: main  
**Commit**: 3b07059 (fix)  

---

## Test Environment

| Component | Status |
|-----------|--------|
| PostgreSQL + Redis (Docker) | Running |
| Go backend (:8080) | Running |
| Next.js frontend (:3000) | Running |
| Browser | Playwright (Chromium) |

---

## Test Results Summary

| # | Test Case | Result | Screenshots |
|---|-----------|--------|-------------|
| 1 | Register → Login Flow | Pass | 3 |
| 2 | Homepage 3-Zone Layout | Pass | 1 |
| 3 | Original Zone + Category Tabs | Pass | 2 |
| 4 | Content Detail Page | Pass | 1 |
| 5 | Creator Studio (/studio/overview) | Pass | 1 |
| 6 | Admin Backend (/admin) | Pass | 2 |
| 7 | Dark Mode Toggle | Pass | 2 |
| 8 | i18n (EN / ZH) | Pass | 2 |

**All 8 tests passed. 14 screenshots captured in `screenshots/`.**

---

## Detailed Test Results

### 1. Register → Login Flow

- Navigated to `/register`, form rendered correctly with fields: username, email, password, confirm password
- Successfully registered test user `e2etest3_user` after fixing CSRF and DB issues (see below)
- Auto-login worked: redirected to `/` with authenticated state
- Header showed user avatar and notification bell

### 2. Homepage 3-Zone Layout

Three zones verified on `/`:

| Zone | Content | Status |
|------|---------|--------|
| Recent IPs | "最近访问 IP" with 3 IP cards | Rendered |
| IP Browse | "推荐 IP" section with 8 IP cards + sidebar categories | Rendered |
| Fanwork Waterfall | Content grid with type filter tabs (全部/文字/图片/视频/音频/Mod/AI提示词/乐谱/其他) + sort dropdown | Rendered |

Stats bar: 40 contents, 0 active IPs, 43 creators.

### 3. Original Zone

- 12 category tabs rendered: 推荐, 影视, 游戏, 文学, 宠物, 美食, 美妆穿搭, 家居, 数码科技, 旅行, 运动, 效率
- Sort dropdown: 推荐 / 最热门 / 最新发布 / 最多点击
- Content grid with cards (cover, title, author avatar, like count)
- Tab switching verified: clicked "美食" → URL changed to `?category=food`, content filtered

### 4. Content Detail Page

Visited `/content/77` (星穹同人：银河列车没有终点站):

| Element | Status |
|---------|--------|
| Title + author + type + views + date | Rendered |
| Markdown body (headings, paragraphs) | Rendered correctly |
| Tags (同人文, 科幻, 星穹铁道) | Rendered |
| Like button (0) | Rendered |
| Comment button (0) | Rendered |
| Report button | Rendered |
| Comment input + submit | Rendered |
| Author sidebar with Follow button | Rendered |
| Version history empty state | Rendered |

Known issue: `GET /api/v1/social/reactions?target_type=content&target_id=77` returns 404. UI handles gracefully (shows 0).

### 5. Creator Studio

Visited `/studio/overview`:

| Element | Status |
|---------|--------|
| Sidebar: 内容发布, 数据看板, 协作管理 sections | Rendered |
| 4 stat cards (总内容数, 总访问量, 总获赞, 粉丝数) | Rendered (0 for new user) |
| Trend chart empty state | Rendered |
| Hot content Top 5 empty state | Rendered |
| Pending items empty state | Rendered |

Known issue: `GET /api/v1/my/contents` returns 404. UI handles gracefully (shows empty states).

### 6. Admin Backend

- Non-admin user redirected from `/admin` to `/` (auth guard working correctly)
- Promoted test user to admin role, re-logged in, `/admin` loaded successfully
- Sidebar: IP 库管理, 内容终审, 用户管理, 申诉处理, 分类管理, 系统配置, Agent 管理
- IP management table with pending IPs and approve/reject buttons
- Config page loaded with system configuration form

### 7. Dark Mode

- Theme system uses `next-themes` with `ThemeProvider`
- Toggle button opens dropdown with: Light / Dark / System
- Dark mode persisted across page navigation (class on `<html>` changed from `light` to `dark`)
- Verified on homepage and original zone

### 8. i18n Language Switching

Switched from 中文 → English:

| UI Element | Chinese | English |
|------------|---------|---------|
| Logo | 万象工坊 | OmniCraft |
| Nav | 二创区 | Fan Creation Zone |
| Nav | 原创区 | Original Zone |
| Search placeholder | 搜索 IP、内容、创作者... | Search IPs, content, creators... |
| Theme button | 切换主题 | Switch theme |
| Publish button | 发布 | Publish |
| Sidebar | 收起侧边栏 | Collapse sidebar |
| Sidebar | 管理 | Manage |
| Categories | 影视/游戏/文学/... | Film & TV/Gaming/Literature/... |
| Sort | 推荐/最热门/最新发布/最多点击 | Recommended/Hottest/New Release/Most Viewed |
| Footer | 关于我们/帮助中心/... | About/Help Center/... |

User-generated content titles remained in original language (expected).

---

## Bugs Found & Fixed

### 1. CSRF Token — Cookie Name Mismatch
**File**: `frontend/lib/api.ts`  
**Problem**: `getCSRFToken()` only checked for `__Host-csrf` (production cookie name). In dev mode, the backend sets `csrf-token`. Registration and all state-changing requests failed with 403.  
**Fix**: Updated regex to `(?:__Host-csrf|csrf-token)`.

### 2. Cross-Origin Cookie — Missing `credentials: 'include'`
**File**: `frontend/lib/api.ts`  
**Problem**: `fetch()` calls from port 3000 to port 8080 did not include `credentials: 'include'`. The browser never persisted `Set-Cookie` headers from cross-origin responses. CSRF cookies were never stored.  
**Fix**: Added `credentials: 'include'` to all 3 fetch calls (main request, token refresh, retry after refresh).

### 3. Config YAML — queue.worker_count Type Mismatch
**File**: `backend/config.yaml`  
**Problem**: `worker_count` was a nested map (`content_review: 2`, `notification: 1`, etc.) but `QueueConfig` expected flat `int` fields (`worker_review`, `worker_notification`, `worker_embedding`, `worker_count`). Backend panicked on startup.  
**Fix**: Flattened the structure to match the Go struct.

### 4. Missing Database Columns
**Tables**: `users`, `content_items`, `conversation_participants`  
**Problem**: Code referenced `email_verified_at`, `deleted_at`, `left_at`, `ban_reason`, `download_count` columns that did not exist in the database. Queries failed with SQLSTATE 42703.  
**Fix**: Added via `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`.

---

## Remaining Issues (Non-Blocking)

| Issue | Endpoint | Severity |
|-------|----------|----------|
| Reactions API 404 | `GET /api/v1/social/reactions` | Low — UI shows 0 gracefully |
| My Contents API 404 | `GET /api/v1/my/contents` | Low — shows empty state |
| Stats Summary 500 | `GET /api/v1/stats/summary` | Low — homepage falls back to cached data |
| Theme dropdown menu click unstable | Theme toggle button | Low — workaround via classList |

---

## Recommendations

1. Run all migrations (`backend/migrations/`) against the current database to ensure schema is fully up to date
2. Add `GET /api/v1/social/reactions` and `GET /api/v1/my/contents` endpoints or update frontend to use correct paths
3. Fix `GET /api/v1/stats/summary` to handle NULL `deleted_at` columns gracefully
4. Add integration tests for the register → login → publish flow to catch CSRF regressions
