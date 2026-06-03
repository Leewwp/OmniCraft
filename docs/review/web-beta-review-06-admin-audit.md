# Review: Beta Tasks A-01, A-02 — Admin Audit Logs

**Reviewer**: Automated code review
**Date**: 2026-06-02
**Commit**: `15dc57fe362a382f3559165ca9767354c38a2317`
**Branch**: `main` (ahead 30)

---

## 1. Build & Test Gate

| Check | Result |
|-------|--------|
| `go test ./internal/service -run TestAdminAudit -v -short` | ⚠️ "no tests to run" — test names don't match pattern |
| `go test ./internal/service -v -short` | ✅ PASS (all 37 tests) |
| `go test ./internal/handler -v -short` | ✅ PASS |
| `go test ./... -short` | ✅ PASS (all 7 packages) |
| `go vet ./...` | ✅ PASS |
| `go build ./...` | ✅ PASS |

**Note**: The F-05 plan specified `go test ./internal/service -run TestAdminAudit -v` but the actual test functions are named `TestFilterMetadata_*` and `TestIsSensitiveKey`, not `TestAdminAudit*`. The `-run TestAdminAudit` pattern matches nothing.

---

## 2. Verification Checklist

### 2.1 Audit log append-only; no update/delete API exposed

**Verdict: ✅ PASS**

- [admin_audit_repo.go:19-25](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/repository/admin_audit_repo.go#L19-L25): Repository exposes only `Create` and `CreateTx` methods. No `Update` or `Delete` methods.
- [admin_audit_service.go:74-128](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/service/admin_audit_service.go#L74-L128): Service exposes only `Record`, `RecordTx`, and `List`. No update/delete APIs.
- [admin_audit.go:7-17](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/model/admin_audit.go#L7-L17): Model is a simple struct with no update/delete hooks.
- Migration [052_admin_audit_logs.sql](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/migrations/052_admin_audit_logs.sql) creates the table with no UPDATE/DELETE grants specified (relies on app-level enforcement).

### 2.2 Metadata uses per-action allowlist; sensitive fields stripped

**Verdict: ✅ PASS**

- [admin_audit_service.go:14-39](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/service/admin_audit_service.go#L14-L39): `auditMetadataAllowlist` defines 18 action-specific allowlists matching the A-01 spec exactly.
- [admin_audit_service.go:130-157](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/service/admin_audit_service.go#L130-L157): `filterMetadata` function:
  - If action has an allowlist → only allowlisted keys are kept
  - If action has no allowlist → all keys pass (with sensitive pattern check)
  - `isSensitiveKey` always runs as a second filter
- [admin_audit_service.go:41-44](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/service/admin_audit_service.go#L41-L44): `sensitiveKeyPatterns` includes: `token`, `cookie`, `password`, `api_key`, `secret`, `grant`, `access_key`, `private_key`, `authorization`, `header`.
- [admin_audit_service.go:159-166](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/service/admin_audit_service.go#L159-L166): `isSensitiveKey` uses case-insensitive substring matching via `containsIgnoreCase`.
- Tests `TestFilterMetadata_AllowlistStripsUnknownKeys`, `TestFilterMetadata_SensitiveKeysAlwaysStripped`, `TestFilterMetadata_CaseInsensitiveSensitiveDetection` all pass. ✅

**Minor note**: The allowlist approach means unknown actions (not in `auditMetadataAllowlist`) pass ALL non-sensitive keys. This is by design per the code comment, but could be tightened to deny-by-default for unknown actions.

### 2.3 trace_id recorded

**Verdict: ✅ PASS**

- [admin.go:723](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/handler/admin.go#L723): `auditOrFail` helper sets `TraceID: c.GetString("trace_id")`.
- [admin_feedback.go:137](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/handler/admin_feedback.go#L137): Feedback handler also sets `TraceID: c.GetString("trace_id")`.
- [judge.go:181](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/handler/judge.go#L181): Judge handler does NOT set `TraceID` — uses `c.GetInt64("user_id")` for admin ID instead of `middleware.GetUserID(c)`, and omits `TraceID` field.

**FINDING**: Judge handler `CreateQuestions` does not set `TraceID` in audit entries.

### 2.4 Local DB sensitive writes and success audit row in same transaction; audit write failure rolls back domain mutation

**Verdict: ❌ FAIL — Audit writes are NOT in the same transaction as domain mutations**

This is the most critical finding. The A-01 spec explicitly requires:

> "For every local-database Beta-sensitive mutation listed in A-02, execute the domain mutation and success audit row in one database transaction. If the audit insert fails, roll back and return a safe 500 AUDIT_WRITE_FAILED."

**Current implementation pattern** (all admin handlers):

```go
// 1. Domain mutation — committed independently
h.contentRepo.UpdateContent(id, updates)

// 2. Audit write — separate transaction
if !h.auditOrFail(c, "content_ban", ...) {
    return  // Too late — domain mutation already committed
}
```

The `auditOrFail` helper ([admin.go:713-731](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/handler/admin.go#L713-L731)) calls `h.auditSvc.Record()` which uses `repo.Create()` — a separate DB call, not in the same transaction as the domain mutation.

The `RecordTx` method exists in the service ([admin_audit_service.go:96-116](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/service/admin_audit_service.go#L96-L116)) and repository (`CreateTx`), but **no handler uses it**. All handlers use `Record` (non-transactional).

**Affected handlers**:
| Handler | Domain Write | Audit | Same TX? |
|---------|-------------|-------|----------|
| `ApproveIP` | `h.ipSvc.ApproveIP(id)` | `auditOrFail` → `Record` | ❌ |
| `RejectIP` | `h.ipSvc.RejectIP(id)` | `auditOrFail` → `Record` | ❌ |
| `BanContent` | `h.contentRepo.UpdateContent(id, ...)` | `auditOrFail` → `Record` | ❌ |
| `RestoreContent` | `h.contentRepo.UpdateContent(id, ...)` | `auditOrFail` → `Record` | ❌ |
| `BanUser` | `h.userRepo.UpdateFields(id, ...)` | `auditOrFail` → `Record` | ❌ |
| `UnbanUser` | `h.userRepo.UpdateFields(id, ...)` | `auditOrFail` → `Record` | ❌ |
| `ResolveAppeal` | `db.Transaction(...)` | `auditOrFail` → `Record` (after tx) | ❌ |
| `ResolveReport` | `searchRepo.UpdateReportStatus(id, ...)` | `auditOrFail` → `Record` | ❌ |
| `PatchConfig` | `h.cfg.SaveOverride(...)` | `auditOrFail` → `Record` | ❌ |
| `CreateLLMConfig` | `h.llmConfigSvc.CreateConfig(...)` | `auditOrFail` → `Record` | ❌ |
| `UpdateLLMConfig` | `h.llmConfigSvc.UpdateConfig(id, ...)` | `auditOrFail` → `Record` | ❌ |
| `DeleteLLMConfig` | `h.llmConfigSvc.DeleteConfig(id)` | `auditOrFail` → `Record` | ❌ |
| `ActivateLLMConfig` | `h.llmConfigSvc.ActivateConfig(id)` | `auditOrFail` → `Record` | ❌ |
| `TestLLMConfig` | External check | `auditOrFail` → `Record` | N/A (external) |
| `AdminCreateCategory` | `h.catSvc.AdminCreateCategory(&cat)` | `auditSvc.Record(...)` | ❌ |
| `AdminUpdateCategory` | `h.catSvc.AdminUpdateCategory(id, ...)` | `auditSvc.Record(...)` | ❌ |
| `AdminDeleteCategory` | `h.catSvc.AdminDeleteCategory(id)` | `auditSvc.Record(...)` | ❌ |
| `AdminReorderCategories` | `h.catSvc.AdminReorderCategories(...)` | `auditSvc.Record(...)` | ❌ |
| `CreateQuestions` | `h.judgeRepo.CreateQuestion(...)` | `auditSvc.Record(...)` | ❌ |
| `PatchFeedback` | `h.feedbackSvc.PatchTicket(...)` | `auditSvc.Record(...)` | ❌ |
| `ReplyFeedback` | `h.feedbackSvc.AdminReply(...)` | `auditSvc.Record(...)` | ❌ |

**Exception**: `ResolveAppeal` uses a GORM transaction for the domain mutation, but the audit call is outside that transaction. If the audit fails, the appeal is already resolved.

### 2.5 External side effects executed after commit

**Verdict: ⚠️ PARTIAL**

- `ResolveAppeal` ([admin.go:303-331](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/handler/admin.go#L303-L331)): The notification dispatch (`h.notifSvc.Notify(...)`) is correctly placed after the transaction. ✅
- However, the audit write is also after the transaction, not inside it. If the audit fails, the notification has already been sent and the appeal is resolved with no audit trail. ❌

### 2.6 Failed attempts record sanitized reason code only

**Verdict: ❌ FAIL — No failed-attempt audit rows are recorded**

The `auditOrFail` helper always records `Result: "success"`. There is no code path that records `result: "failed"` audit rows for:
- Rejected operations (e.g., validation failures before the domain write)
- Domain write failures
- Authorization failures

The A-01 spec requires: "Record rejected or failed attempts as best-effort result=failed audit rows with a sanitized reason code. Never include raw provider or database errors in metadata."

### 2.7 Coverage of sensitive operations

**Verdict: ✅ PASS (all covered)**

| Operation | Audited | Handler | Location |
|-----------|---------|---------|----------|
| IP approve | ✅ | `ApproveIP` | admin.go:101 |
| IP reject | ✅ | `RejectIP` | admin.go:117 |
| Content ban | ✅ | `BanContent` | admin.go:180 |
| Content restore | ✅ | `RestoreContent` | admin.go:196 |
| User ban | ✅ | `BanUser` | admin.go:219 |
| User unban | ✅ | `UnbanUser` | admin.go:236 |
| Appeal resolve | ✅ | `ResolveAppeal` | admin.go:327 |
| Report resolve | ✅ | `ResolveReport` | admin.go:652 |
| Config patch | ✅ | `PatchConfig` | admin.go:451 |
| Category create | ✅ | `AdminCreateCategory` | category.go:56 |
| Category update | ✅ | `AdminUpdateCategory` | category.go:84 |
| Category delete | ✅ | `AdminDeleteCategory` | category.go:111 |
| Category reorder | ✅ | `AdminReorderCategories` | category.go:137 |
| LLM config create | ✅ | `CreateLLMConfig` | admin.go:521 |
| LLM config update | ✅ | `UpdateLLMConfig` | admin.go:549 |
| LLM config delete | ✅ | `DeleteLLMConfig` | admin.go:569 |
| LLM config activate | ✅ | `ActivateLLMConfig` | admin.go:585 |
| LLM config test | ✅ | `TestLLMConfig` | admin.go:602 |
| Judge question create | ✅ | `CreateQuestions` | judge.go:178 |
| Feedback reply | ✅ | `ReplyFeedback` | admin_feedback.go:190 |
| Feedback close | ✅ | `PatchFeedback` | admin_feedback.go:130 |
| Feedback reopen | ✅ | `PatchFeedback` | admin_feedback.go:126 |
| Feedback priority | ✅ | `PatchFeedback` | admin_feedback.go:122 |
| Feedback assign | ✅ (via PatchFeedback) | `PatchFeedback` | admin_feedback.go:113 |

### 2.8 Admin config still masked/boolean; no plaintext secret dependency

**Verdict: ✅ PASS**

- [admin.go:333-360](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/handler/admin.go#L333-L360): `GetConfig` returns `PublicConfig` struct + `ConfigRedactStatus` booleans.
- `ConfigRedactStatus` ([admin.go:350-358](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/handler/admin.go#L350-L358)): Only boolean flags like `JWTSecretConfigured`, `OSSKeyConfigured`, etc. No raw secrets.
- `PatchConfig` ([admin.go:362-473](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/handler/admin.go#L362-L473)): `filterSensitivePatches` strips keys matching `sensitiveConfigFields` before applying. ✅

---

## 3. Sensitive Operation Coverage Matrix

| Operation | Action String | Allowlist Keys | TX Atomic? | Failed Audit? | TraceID? |
|-----------|--------------|----------------|------------|---------------|----------|
| IP approve | `ip_approve` | ip_id, decision | ❌ | ❌ | ✅ |
| IP reject | `ip_reject` | ip_id, decision | ❌ | ❌ | ✅ |
| Content ban | `content_ban` | content_id, reason, author_id | ❌ | ❌ | ✅ |
| Content restore | `content_restore` | content_id, reason | ❌ | ❌ | ✅ |
| User ban | `user_ban` | target_user_id, reason | ❌ | ❌ | ✅ |
| User unban | `user_unban` | target_user_id, reason | ❌ | ❌ | ✅ |
| Appeal resolve | `appeal_resolve` | appeal_id, decision, reason | ❌ | ❌ | ✅ |
| Report resolve | `report_resolve` | report_id, decision, reason | ❌ | ❌ | ✅ |
| Config patch | `config_patch` | field, old_value_masked, new_value_masked | ❌ | ❌ | ✅ |
| Category create | `category_create` | name, slug, display_order | ❌ | ❌ | ❌ |
| Category update | `category_update` | category_id, name, slug, display_order | ❌ | ❌ | ❌ |
| Category delete | `category_delete` | category_id, name | ❌ | ❌ | ❌ |
| Category reorder | `category_reorder` | order | ❌ | ❌ | ❌ |
| LLM config create | `llm_config_create` | provider, model | ❌ | ❌ | ✅ |
| LLM config update | `llm_config_update` | config_id, provider, model | ❌ | ❌ | ✅ |
| LLM config delete | `llm_config_delete` | config_id, provider, model | ❌ | ❌ | ✅ |
| LLM config activate | `llm_config_activate` | config_id | ❌ | ❌ | ✅ |
| LLM config test | `llm_config_test` | config_id | N/A (external) | ❌ | ✅ |
| Judge question create | `judge_question_create` | question_id, content_type | ❌ | ❌ | ❌ |
| Feedback reply | `feedback_reply` | ticket_id, is_internal_note | ❌ | ❌ | ✅ |
| Feedback close | `feedback_close` | ticket_id, reason | ❌ | ❌ | ✅ |
| Feedback reopen | `feedback_reopen` | ticket_id | ❌ | ❌ | ✅ |
| Feedback priority | `feedback_priority` | ticket_id, priority | ❌ | ❌ | ✅ |
| Feedback assign | `feedback_assign` | ticket_id, assignee_admin_id | ❌ | ❌ | ✅ |

---

## 4. Findings Summary

| # | Severity | Task | Finding | Location |
|---|----------|------|---------|----------|
| A01-1 | 🔴 CRITICAL | A-02 | **Audit writes are NOT atomic with domain mutations.** All handlers use `Record` (separate transaction) instead of `RecordTx` (same transaction). If audit write fails, domain mutation is already committed with no audit trail. This directly violates A-01 spec requirement. | All admin handlers |
| A01-2 | 🔴 HIGH | A-02 | **No failed-attempt audit rows are recorded.** `auditOrFail` always records `result: "success"`. The spec requires best-effort `result=failed` rows with sanitized reason codes. | `admin.go:713-731` |
| A01-3 | 🟡 MEDIUM | A-02 | **Category handler and Judge handler use `c.GetInt64("user_id")` instead of `middleware.GetUserID(c)`** for admin user ID. This may return 0 if the context key doesn't match, resulting in `admin_user_id = 0` in audit logs. | `category.go:57,84,111,137`; `judge.go:181` |
| A01-4 | 🟡 MEDIUM | A-02 | **Category handler and Judge handler omit `TraceID`** in audit entries. They don't call `c.GetString("trace_id")`. | `category.go:56-64,84-91,111-118,137-143`; `judge.go:179-188` |
| A01-5 | 🟡 MEDIUM | A-01 | **Test naming mismatch**: `go test -run TestAdminAudit` matches nothing. Tests are named `TestFilterMetadata_*` and `TestIsSensitiveKey`. | `admin_audit_service_test.go` |
| A01-6 | 🟢 LOW | A-01 | **Unknown actions pass all non-sensitive metadata keys** (deny-by-default would be stricter). Current behavior: actions not in `auditMetadataAllowlist` have all keys kept (except sensitive patterns). | `admin_audit_service.go:148-154` |
| A01-7 | 🟢 LOW | A-02 | **`PatchFeedback` audit action selection is fragile**: it determines action (`feedback_close`, `feedback_reopen`, `feedback_priority`) based on request field presence, not on actual state change. A PATCH with both `status` and `priority` always records as `feedback_close`/`feedback_reopen`, never `feedback_priority`. | `admin_feedback.go:122-128` |

---

## 5. Transactionality Deep Dive

The core architectural issue is that `auditOrFail` was designed as a post-mutation check rather than a co-transactional write. The correct pattern should be:

```go
// Desired pattern (NOT current):
err := h.db.Transaction(func(tx *gorm.DB) error {
    // 1. Domain mutation using tx
    if err := h.contentRepo.UpdateContentTx(tx, id, updates); err != nil {
        return err
    }
    // 2. Audit write using same tx
    if err := h.auditSvc.RecordTx(ctx, tx, entry); err != nil {
        return err  // rolls back both
    }
    return nil
})
// 3. External side effects AFTER commit
h.notifSvc.Notify(...)
```

**Current pattern** (all handlers):
```go
// 1. Domain mutation — auto-committed
h.contentRepo.UpdateContent(id, updates)
// 2. Audit write — separate transaction
h.auditOrFail(c, ...)  // If this fails, domain mutation persists un-audited
```

**To fix this**, each handler needs to be refactored to:
1. Accept a `*gorm.DB` transaction parameter in repository/service methods
2. Wrap both the domain mutation and audit write in a single `db.Transaction()`
3. Move external side effects (notifications, emails) after the transaction commits

This is a significant refactor affecting all 20+ admin mutation handlers.

---

## 6. Recommendations

1. **A01-1 (CRITICAL)**: Refactor all admin mutation handlers to use `db.Transaction()` wrapping both the domain write and `RecordTx`. The `RecordTx` method already exists but is unused. Each repository/service method needs a `*gorm.DB` tx parameter variant.

2. **A01-2**: Add failed-attempt audit recording. Before returning error responses for validation failures, authorization failures, or domain write failures, record a `result: "failed"` audit entry with a sanitized reason code (e.g., `VALIDATION_ERROR`, `NOT_FOUND`, `DB_ERROR`).

3. **A01-3**: Replace `c.GetInt64("user_id")` with `middleware.GetUserID(c)` in Category and Judge handlers for consistent admin user ID extraction.

4. **A01-4**: Add `TraceID: c.GetString("trace_id")` to all Category and Judge handler audit entries.

5. **A01-5**: Rename test functions to include `TestAdminAudit` prefix, or update the plan's test command to match actual test names.

6. **A01-7**: Refactor `PatchFeedback` audit action to be based on actual state transition, not request field presence.
