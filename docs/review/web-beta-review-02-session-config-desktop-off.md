# Web Beta Review 02 - Session, Config & Desktop-Off

## Baseline

- **Branch**: `main` (ahead of origin by 30 commits)
- **HEAD**: `15dc57fe362a382f3559165ca9767354c38a2317`
- **Tasks under review**: F-03 (HttpOnly cookie session), F-04 (public runtime config), D-01 (disable unsafe desktop route)
- **Plan references**:
  - `docs/superpowers/plans/2026-05-30-omnicraft-beta-foundation.md` F-03, F-04
  - `docs/superpowers/plans/2026-05-30-omnicraft-beta-desktop-deploy-security.md` D-01
- **Roadmap status**: All three `[x]` (marked complete)
- **Frozen contracts verified**:
  - Production Web origin: `https://app.leeppp.online`
  - Production API origin: `https://api.leeppp.online`
  - `features.desktop_deploy_enabled: false`
  - Public config DTO is allowlisted, not reusing admin `model.PublicConfig`

## Findings

### P1-01: CORS unconditionally adds localhost variants even in release mode

**File**: `backend/internal/middleware/cors.go` lines 18–28

```go
localhostVariants := []string{
    "http://localhost:3000",
    "http://localhost:3001",
    "http://127.0.0.1:3000",
    "http://127.0.0.1:3001",
}
for _, v := range localhostVariants {
    if !allowed[v] {
        allowed[v] = true
    }
}
```

The F-03 plan states: "Localhost variants remain development-only and wildcard origins are rejected." In release mode, these localhost origins should not be added to the credentialed CORS allowlist. An attacker on the same network could spin up a localhost page and make credentialed requests to the API.

**Impact**: In production release mode, any page served from localhost can make credentialed cross-origin requests to the API, bypassing the intended origin restriction.

**Fix**: Guard the localhost variant injection with `cfg.Server.Mode != "release"`.

---

### P1-02: Refresh handler falls back to reading refresh_token from JSON body

**File**: `backend/internal/handler/auth.go` lines 158–170

```go
func (h *AuthHandler) Refresh(c *gin.Context) {
    refreshToken, _ := c.Cookie(refreshCookieName(h.cfg))
    if refreshToken == "" {
        var body struct {
            RefreshToken string `json:"refresh_token"`
        }
        if err := c.ShouldBindJSON(&body); err == nil && body.RefreshToken != "" {
            refreshToken = body.RefreshToken
        }
    }
    // ...
}
```

The F-03 plan states: "Refresh reads only the cookie, rotates it, and returns a new access token." The JSON body fallback contradicts the cookie-only contract. It allows an attacker who steals a refresh token via XSS or log leakage to refresh it without the HttpOnly cookie.

**Impact**: The cookie-only security model is undermined by the JSON fallback. An XSS attacker can use a stolen refresh token without needing the HttpOnly cookie.

**Fix**: Remove the JSON body fallback. Return `401 INVALID_TOKEN` when the cookie is missing.

---

### P1-03: Logout handler accepts refresh_token from JSON body

**File**: `backend/internal/handler/auth.go` lines 147–153

```go
var body struct {
    RefreshToken string `json:"refresh_token"`
}
if err := c.ShouldBindJSON(&body); err == nil && body.RefreshToken != "" {
    h.authService.Logout(body.RefreshToken)
}
```

Same issue as P1-02. The F-03 plan says "Never expose refresh tokens in JSON, URLs, logs or browser storage." While this is reading (not writing), accepting refresh tokens from JSON body maintains an attack surface that the cookie migration was designed to eliminate.

**Impact**: Maintains backward compatibility but contradicts the cookie-only security model.

**Fix**: Remove the JSON body path. Only read the refresh token from the HttpOnly cookie.

---

### P2-01: CSRF middleware skips deploy-grants and payments paths entirely

**File**: `backend/internal/middleware/csrf.go` lines 61–77

```go
func isInternalPath(path string) bool {
    // ...
    if path == "/api/v1/deploy-grants" {
        return true
    }
    if len(path) >= len("/api/v1/payments/") && path[:len("/api/v1/payments/")] == "/api/v1/payments/" {
        return true
    }
    // ...
}
```

The `POST /deploy-grants` issue endpoint (D-02) will require CSRF protection when enabled. Currently the entire path is exempted. While the D-03 exchange endpoint is designed to work without a Web JWT (and thus without CSRF), the issue endpoint should require CSRF since it's a standard authenticated Web request.

**Impact**: When D-02 enables deploy grants, the issue endpoint will lack CSRF protection unless this exemption is scoped to only the exchange path.

**Fix**: Remove `/api/v1/deploy-grants` from the CSRF exemption. When D-03 adds the exchange endpoint, add a targeted exemption for `POST /api/v1/deploy-grants/exchange` only.

---

### P2-02: setRefreshCookie does not set Domain attribute explicitly

**File**: `backend/internal/handler/auth.go` line 47

```go
c.SetCookie(name, token, maxAge, "/", "", isSecure, true)
```

The F-03 plan states: "release mode cookie ... no Domain." Gin's `SetCookie` with empty `Domain` parameter is correct — it defaults to the current host only, which is the desired behavior for `__Host-` prefixed cookies. However, the plan also notes: "Keep the refresh and CSRF cookies host-only on the API hostname." In a cross-origin deployment (Web on `app.leeppp.online`, API on `api.leeppp.online`), the cookie will be set on `api.leeppp.online` only, which is correct.

**Impact**: No actual issue — the empty Domain parameter produces the correct host-only behavior. This finding is informational only.

---

### P2-03: Public config test does not verify Legal DTO absence of secrets

**File**: `backend/internal/handler/public_config_test.go` line 97

The forbidden word list includes `"threshold"` and `"min_score"`, which would match `current_terms_version` and `current_privacy_version` if they contained the substring "version". However, the `PublicLegalDTO` was added after F-04 (by V-02/V-04) and contains only version strings, which are safe. The test should explicitly verify the Legal DTO fields are present and contain no secrets.

**Impact**: Low — the Legal DTO is safe, but the test should be updated to account for the new DTO section.

---

### P3-01: fetchPublicConfig throws on failure instead of returning safe defaults

**File**: `frontend/lib/public-config.ts` lines 37–49

```ts
export async function fetchPublicConfig(): Promise<PublicConfig> {
    if (cachedConfig) return cachedConfig;
    const res = await fetch(`${API_URL}/api/v1/config/public`, { credentials: "include" });
    if (!res.ok) {
        throw new Error(`failed to fetch public config: ${res.status}`);
    }
    // ...
}
```

The F-04 plan states: "Use a stable fallback with all optional capabilities disabled." While `AgentFeatureGate` correctly catches the error and uses `disabledFeatures`, the `fetchPublicConfig` function itself throws. Other consumers that don't catch the error will crash.

**Impact**: Low — `AgentFeatureGate` handles it correctly, but the helper function should ideally return the safe default on failure rather than throwing.

---

### P3-02: proxy.ts no longer uses cookie-dependent routing

**File**: `frontend/proxy.ts`

The proxy middleware has been cleaned up and does not inspect cookies for routing decisions. It only handles locale resolution and protected-path detection. This is correct per the F-03 plan: "remove cookie-dependent proxy routing and let AuthProvider plus protected layouts perform the refresh/redirect decision."

**Impact**: No issue — informational confirmation that the plan requirement is met.

---

## Cookie/CSRF/CORS Matrix

| Aspect | Debug Mode | Release Mode | Plan Compliance |
|---|---|---|---|
| Refresh cookie name | `refresh_token` | `__Host-refresh_token` | ✅ |
| Refresh cookie HttpOnly | `true` | `true` | ✅ |
| Refresh cookie Secure | `false` | `true` | ✅ |
| Refresh cookie SameSite | `Lax` | `Lax` | ✅ |
| Refresh cookie Path | `/` | `/` | ✅ |
| Refresh cookie Domain | (empty = host-only) | (empty = host-only) | ✅ |
| CSRF cookie name | `csrf-token` | `__Host-csrf` | ✅ |
| CSRF cookie HttpOnly | `false` (readable for double-submit) | `false` | ✅ |
| CSRF cookie Secure | `false` | `true` | ✅ |
| CSRF validation | `X-CSRF-Token` header vs cookie | Same | ✅ |
| CSRF bootstrap | `GET /api/v1/auth/csrf` returns token in body | Same | ✅ |
| CORS credentialed | Explicit origins + localhost | Explicit origins + **localhost** ⚠️ | ❌ P1-01 |
| CORS wildcard | Never used | Never used | ✅ |

## Public Config Allowlist

| Field | Source Config | Exposed | Safe |
|---|---|---|---|
| `features.web_agent_enabled` | `cfg.Agent.WebAgentEnabled` | ✅ | ✅ |
| `features.payment_enabled` | `cfg.Features.PaymentEnabled` | ✅ | ✅ |
| `features.creator_support_enabled` | `cfg.Features.CreatorSupportEnabled` | ✅ | ✅ |
| `features.desktop_deploy_enabled` | `cfg.Features.DesktopDeployEnabled` | ✅ | ✅ |
| `captcha.provider` | `cfg.Captcha.Provider` | ✅ | ✅ |
| `captcha.prefix` | `cfg.Captcha.Prefix` | ✅ | ✅ |
| `captcha.scene_id` | `cfg.Captcha.SceneID` | ✅ | ✅ |
| `captcha.region` | `cfg.Captcha.Region` | ✅ | ✅ |
| `client.download_enabled` | `cfg.Client.DownloadEnabled` | ✅ | ✅ |
| `client.download_url` | `cfg.Client.DownloadURL` | ✅ | ✅ |
| `client.latest_version` | `cfg.Client.LatestVersion` | ✅ | ✅ |
| `legal.current_terms_version` | `cfg.Legal.CurrentTermsVersion` | ✅ | ✅ |
| `legal.current_privacy_version` | `cfg.Legal.CurrentPrivacyVersion` | ✅ | ✅ |

**Not exposed (verified by test + code inspection)**:
- `secret`, `access_key`, `api_key`, `dsn`, `password`, `hmac`, `private` — absent from DTO
- OSS CDN, TTL, rate-limit internals — absent from DTO
- Redis address/password — absent from DTO
- JWT secret — absent from DTO
- Captcha AccessKeyID/Secret — absent from DTO

**Test verification**: `TestPublicConfigAllowlist` passes, checking for forbidden substrings and absent top-level keys.

## Desktop-Off Proof

| Check | Evidence | Status |
|---|---|---|
| `features.desktop_deploy_enabled` defaults to `false` | `config.yaml` line 47 | ✅ |
| `POST /deploy-grants` returns 503 FEATURE_DISABLED | `routes.go` line 364–366 | ✅ |
| `/api/v1/agent/script/:id` route removed | `agent_deploy_disabled_test.go` confirms absence | ✅ |
| `GenerateDeployScript` handler removed | `agent_deploy_disabled_test.go` confirms absence | ✅ |
| `AgentFeatureGate` hides desktop deploy UI when flag is false | `AgentFeatureGate.tsx` line 53 checks `features.desktop_deploy_enabled` | ✅ |
| Frontend `getRefreshToken()` returns null | `auth.ts` line 15–17 | ✅ |
| No `localStorage.setItem("refresh_token", ...)` | Grep confirms only `getRefreshToken` stub in `auth.ts` | ✅ |
| Access token stored in memory only | `api.ts` line 48: `let inMemoryAccessToken` | ✅ |

## Commands Run

```powershell
# Backend focused tests
cd backend; go test ./internal/middleware ./internal/handler ./internal/service -v
# Result: ALL PASS

# Backend full suite
cd backend; go test ./...
# Result: ALL PASS

# Backend static analysis
cd backend; go vet ./...
# Result: CLEAN

# Backend build
cd backend; go build ./...
# Result: CLEAN

# Frontend lint (TypeScript check)
cd frontend; npm run lint
# Result: PASS (tsc --noEmit clean)

# Frontend build
cd frontend; npm run build
# Result: PASS (Next.js 16.2.4 build successful, 60 pages generated)

# Grep for residual refresh_token exposure
# frontend: only auth.ts:15 getRefreshToken() stub (returns null)
# backend: agent/script only in test assertions (confirming removal)
```

## Screenshots

Browser testing was performed with Playwright against live backend (port 8080) and frontend (port 3000) dev servers. All 10 screenshots saved to `screenshots/review-web-beta/02-session-config-desktop-off/`.

| # | Test | Screenshot | Result |
|---|---|---|---|
| 1 | Public config endpoint | `01-public-config.png` | ✅ 200, `desktop_deploy_enabled: false`, no secrets |
| 2 | CSRF bootstrap | `02-csrf-bootstrap.png` | ✅ 200, token present, `csrf-token` cookie set |
| 3 | Login | `03-login-result.png` | ✅ 200, `refresh_token` cookie set (httpOnly=True, secure=False=debug, sameSite=Lax, path=/) |
| 4 | Refresh via cookie | `04-refresh-result.png` | ✅ 200, new `access_token` returned |
| 5 | Logout with Authorization header | `05-logout-result.png` | ✅ 200, refresh cookie cleared |
| 6 | Token blacklisted after logout | `06-token-blacklisted.png` | ✅ 401 `token has been revoked` |
| 7 | Agent script route removed | `07-agent-script-404.png` | ✅ 404 |
| 8 | Deploy grants disabled | `08-deploy-grants-503.png` | ✅ 503 `FEATURE_DISABLED` |
| 9 | Frontend login page | `09-frontend-login.png` | ✅ Renders correctly |
| 10 | Frontend home (no desktop deploy) | `10-frontend-home.png` | ✅ No "Desktop" or "Download" text visible |

### Key Browser Test Observations

1. **Refresh cookie attributes (debug mode)**: `refresh_token`, httpOnly=True, secure=False (expected in debug/HTTP), sameSite=Lax, path=/. In release mode, the code switches to `__Host-refresh_token` with Secure=True.

2. **Logout correctly blacklists access token**: When the `Authorization: Bearer <token>` header is included in the logout request, the access token is immediately blacklisted in Redis and subsequent requests with that token return 401.

3. **Frontend does not expose desktop deploy UI**: No "Desktop" or "Download" text found on the home page when `desktop_deploy_enabled: false`.

4. **Agent script route returns 404**: Confirmed that `GET /api/v1/agent/script/test-id` returns 404, verifying D-01 route removal.

## Blocked

1. **Production cookie attribute verification**: The `__Host-` prefix requires HTTPS. In local development (HTTP), the release-mode cookie name cannot be fully tested. This is expected and documented in the plan.

2. **CORS release-mode verification**: Cannot test CORS behavior in release mode without a production-like HTTPS setup. Code review confirms P1-01 (localhost variants unconditionally added).

## Verdict

**CONDITIONAL PASS** — The session migration to HttpOnly cookies is correctly implemented and verified via browser testing: login sets the cookie with proper attributes (httpOnly=True, sameSite=Lax, path=/), refresh reads from the cookie and returns a new access token, logout blacklists both the access token (via Authorization header) and the refresh token (via cookie) and clears the cookie. The public config endpoint uses a dedicated allowlisted DTO that does not expose secrets. The unsafe desktop deploy route has been removed and returns 404, the deploy-grants endpoint returns 503 FEATURE_DISABLED, and the frontend hides desktop deploy UI when the flag is false.

However, **P1-01 (CORS localhost variants in release mode)** violates the production origin restriction, and **P1-02/P1-03 (JSON body fallback for refresh/logout)** undermine the cookie-only security model. These three findings should be addressed before the Web Beta ships to production.
