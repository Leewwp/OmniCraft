# Web Beta Review 01 - Runtime Authorization

## Baseline

- **Branch**: `main` (ahead of origin by 30 commits)
- **HEAD**: `15dc57fe362a382f3559165ca9767354c38a2317`
- **Task under review**: F-02 — Fail Closed And Centralize Interaction Eligibility
- **Plan reference**: `docs/superpowers/plans/2026-05-30-omnicraft-beta-foundation.md` F-02 section
- **Roadmap status**: `[x]` (marked complete)
- **Untracked files**: `docs/review/web-beta-review-00-release-evidence.md`, `docs/review/web-beta-review-01-authz-runtime.md`

## Findings

### P0-01: AuthRequired fails open on Redis token-revocation check

**File**: `backend/internal/middleware/auth.go` lines 54–62

When Redis is available but returns an error for the blacklist check (e.g., connection timeout, cluster failover), the code treats the error as "token not revoked" and continues processing. The F-02 plan explicitly states: "Protected requests must also fail closed when access-token revocation state cannot be checked in Redis."

```go
if rdb != nil {
    blacklistKey := fmt.Sprintf("blacklist:token:%s", tokenStr)
    val, redisErr := rdb.Get(c.Request.Context(), blacklistKey).Result()
    if redisErr == nil && val == "1" {
        // only reached if Redis confirms revocation
        c.JSON(401, ...)
        c.Abort()
        return
    }
    // if redisErr != nil (Redis down), execution continues — FAIL OPEN
}
```

**Impact**: A revoked token (e.g., after logout or ban) is accepted if Redis is temporarily unreachable. This directly contradicts the fail-closed contract.

**Fix**: When `rdb != nil` and `redisErr` is not `redis.Nil`, the middleware should return `503 AUTH_STATUS_UNAVAILABLE`.

---

### P1-01: verification_service.go missing cache invalidation after email verification

**File**: `backend/internal/service/verification_service.go` (entire file)

The F-02 plan explicitly requires: "verification_service.go (email verification)" must call `RuntimeStatusCache.Invalidate(userID)` after email verification. Grep confirms zero occurrences of `Invalidate` or `cache` in this file.

**Impact**: After a user verifies their email, the cached `RuntimeUserStatus` still has `EmailVerifiedAt == nil`. Until the cache TTL expires (default 300s), `InteractionRequired` with `RequireVerifiedEmail` will reject the newly verified user from all interaction-gated endpoints.

**Fix**: Add `middleware.InvalidateUserStatusCache(rdb, userID)` after the `email_verified_at` DB update in the verification service, or inject the `RuntimeStatusCache` and call `Invalidate`.

---

### P1-02: SafeErrorMsg may expose err.Error() to client

**File**: `backend/internal/pkg/response/safe_error.go` line 45

```go
func SafeErrorMsg(err error, fallback string) string {
    // ...known business errors checked...
    if strings.Contains(err.Error(), "sql:") || strings.Contains(err.Error(), "pq:") ||
        strings.Contains(err.Error(), "dsn") || strings.Contains(err.Error(), "record not found") {
        return fallback
    }
    return err.Error()  // <-- non-SQL, non-business errors leak raw message
}
```

If the error is not a known business error and doesn't contain SQL/pq/dsn/record-not-found substrings, the raw `err.Error()` is returned to the client. This can leak internal details (file paths, connection strings, stack traces from third-party libraries).

**Impact**: Task 102 (error message desensitization) is partially satisfied but this fallback path is an escape hatch.

**Fix**: Change line 45 to `return fallback`. Only return `err.Error()` for explicitly allowlisted error types.

---

### P1-03: isPublishFrozen fails open when Redis is nil

**File**: `backend/internal/middleware/interaction.go` lines 83–90

```go
func isPublishFrozen(c *gin.Context, rdb *redis.Client, userID int64) bool {
    if rdb == nil {
        return false  // FAIL OPEN — frozen user can publish
    }
    // ...
}
```

When `rdb == nil`, a publish-frozen user bypasses the freeze check. While the plan doesn't explicitly call this out, the overall fail-closed principle suggests that inability to check the freeze status should block publishing, not allow it.

**Impact**: If Redis is unavailable, users with active publish freezes can publish content.

**Fix**: When `rdb == nil` and `RequireNoPublishFreeze` is true, return `503 AUTH_STATUS_UNAVAILABLE` instead of allowing the request.

---

### P2-01: deploy_grant route missing interaction guard

**File**: `backend/internal/handler/routes.go` line 364

```go
v1.POST("/deploy-grants", func(c *gin.Context) {
    c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FEATURE_DISABLED", "message": "desktop deploy is not enabled"})
})
```

The F-02 plan lists `deploy_grant` as a policy group requiring verified email and reputation. Currently the route is a bare 503 handler without `authReq` or any interaction guard. Since the feature is disabled, this is not exploitable today, but D-02 must add the guard when enabling the route.

**Impact**: Low now (feature disabled), but must be addressed before D-02 enables deploy grants.

---

### P2-02: AuthRequired has unreachable code path

**File**: `backend/internal/middleware/auth.go` lines 73–77

```go
if resolveErr != nil {
    // ...
    if status != nil && status.IsBanned {  // status is always nil when resolveErr != nil
        c.JSON(401, ...)
        c.Abort()
        return
    }
    // ...
}
```

When `ResolveRuntimeUserStatus` returns an error, `status` is always `nil`. The `status != nil && status.IsBanned` check is dead code.

**Impact**: No security impact, but indicates the banned-user-during-status-failure scenario was considered but not correctly implemented. The intent was likely to reject banned users even when status resolution partially fails.

---

### P3-01: No table-driven route tests for all policy groups

**File**: `backend/internal/middleware/interaction_test.go`

The F-02 plan requires: "Add table-driven route tests that exercise at least one endpoint from every policy group listed above. A middleware unit test alone is insufficient because the current defect includes incomplete route mounting."

The existing `TestInteractionPolicyTableDriven` tests middleware behavior in isolation but does not test actual route mounting. There are no route-level tests verifying that every policy group endpoint has the correct guard applied.

**Impact**: Route mounting regressions (e.g., accidentally removing a guard from an endpoint) would not be caught by tests.

---

### P3-02: InteractionRequired re-resolves status from DB on every request

**File**: `backend/internal/middleware/interaction.go` line 31

`InteractionRequired` calls `ResolveRuntimeUserStatus` which always queries the DB (or cache) even though `AuthRequired` already resolved the status earlier in the middleware chain. This is a double resolution.

**Impact**: Performance concern (extra DB query per interaction-gated request), not a correctness issue.

---

## Route Coverage Matrix

| Policy Group | Endpoint(s) | Guard Applied | Verified |
|---|---|---|---|
| `publish` | POST /contents | `publishGuard` (email+rep+freeze) | ✅ |
| `edit_delete` | PATCH /contents/:id, DELETE /contents/:id | `editDeleteGuard` (email+rep) | ✅ |
| `comments` | POST /social/comments, DELETE /social/comments/:id, PATCH /social/comments/:id, POST /discussions/:id/comments | `commentsGuard` (email+rep) | ✅ |
| `reactions` | POST /social/reactions | `reactionsGuard` (email+rep) | ✅ |
| `favorites` | POST /favorites, DELETE /favorites/:contentId | `favoritesGuard` (email+rep) | ✅ |
| `reports` | POST /contents/:id/report, POST /social/comments/:id/report | `reportsGuard` (email+rep) | ✅ |
| `pull_requests` | POST /pr, POST /pr/:id/accept, POST /pr/:id/reject, POST /pr/:id/merge | `prGuard` (email+rep) | ✅ |
| `judge` | POST /judge/exam/submit, POST /judge/vote, POST /judge/reasons/:id/vote | `judgeGuard` (email+rep) | ✅ |
| `follows` | POST /users/:id/follow, DELETE /users/:id/follow, POST /ips/:id/follow, DELETE /ips/:id/follow | `followsGuard` (email+rep) | ✅ |
| `messages` | GET /messages, POST /messages | `messagesGuard` (email+rep) | ✅ |
| `downloads` | GET /contents/:id/download | `downloadsGuard` (email+rep) | ✅ |
| `agent` | All /agent/* (group-level) | `agentGuard` (email+rep) | ✅ |
| `deploy_grant` | POST /deploy-grants | **None** (503 stub) | ⚠️ P2-01 |

## Commands Run

```powershell
cd backend; go test ./internal/middleware ./internal/handler ./internal/service -v
# Result: ALL PASS (19 middleware tests, 22 handler tests, 30+ service tests)

cd backend; go test ./...
# Result: ALL PASS

cd backend; go vet ./...
# Result: CLEAN (no warnings)

cd backend; go build ./...
# Result: CLEAN (no errors)
```

## Untested Or Blocked

1. **P0-01 fix verification**: No test covers the Redis-down-during-blacklist-check scenario for `AuthRequired`. The existing test `TestAuthRequiredRejectsWhenRedisAndDBCannotConfirmStatus` passes `nil` for Redis, which skips the blacklist check entirely. A test with a failing Redis mock is needed.

2. **P1-01 fix verification**: No test covers the email-verification cache invalidation gap. A test that verifies email verification + immediate interaction-gated request would catch this.

3. **Route-level policy group tests**: No automated test verifies that every endpoint in the route coverage matrix has the correct guard. Only source inspection was performed.

4. **Runtime integration test**: No end-to-end test verifies the full chain: JWT parse → Redis blacklist check → DB status resolution → interaction policy check → handler.

## Verdict

**CONDITIONAL PASS** — The core architecture is sound: `ResolveRuntimeUserStatus` correctly queries DB for real-time status, `InteractionRequired` properly enforces email verification and reputation thresholds, reputation=0 is not treated as a cache miss, and cache invalidation is called for reputation mutations, ban/unban, and account deletion.

However, **P0-01 (AuthRequired fails open on Redis blacklist check)** is a direct violation of the fail-closed contract and must be fixed before this task can be considered fully compliant with the F-02 specification. **P1-01 (missing cache invalidation after email verification)** will cause user-visible failures (verified users rejected from interactions for up to 5 minutes) and must also be addressed.
