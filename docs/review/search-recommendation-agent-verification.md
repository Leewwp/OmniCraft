# OmniCraft Search × Recommendation × Agent — Deep Verification Report

**Date**: 2026-05-22  
**Scope**: Full-text search (tsvector/GIN), recommendation engine, hot rank service, agent endpoints, LLM config CRUD, GoSafe panic recovery  
**Method**: Static code analysis of all relevant backend files

---

## 1. Full-Text Search (Migrations 041 & 042)

### Files
- `backend/migrations/041_content_search_vector.sql`
- `backend/migrations/042_ips_search_vector.sql`

### Status: PARTIALLY FUNCTIONAL — Chinese search is broken

| Check | Result |
|-------|--------|
| tsvector column | Added to `content_items` and `ips` |
| GIN index | `idx_content_items_search_vector` and `idx_ips_search_vector` |
| Weight tiers | A(title) > B(description) > C(tags) — correct |
| Auto-update trigger | `BEFORE INSERT OR UPDATE OF title, description` |
| Backfill | Existing NULL rows backfilled |

### Issues

#### CRITICAL — Chinese segmentation broken
Both migrations use `to_tsvector('simple', ...)`. The `simple` dictionary does zero word segmentation. For Chinese text (no spaces between words), an entire title becomes a single token. `tsquery` will never match partial Chinese terms except via `:*` prefix matching, which is slow and unreliable.

```sql
-- Current (broken for Chinese):
setweight(to_tsvector('simple', coalesce(NEW.title, '')), 'A')
-- "我爱北京天安门" → single token "我爱北京天安门"
-- Query "北京" → no match
```

**Fix**: Install `zhparser` + `pg_jieba` extension and use `to_tsvector('zhparser', ...)`, or use `pg_trgm` GIN index + `ILIKE` as fallback for Chinese content.

#### MEDIUM — Tag changes don't reindex
Trigger only fires on `UPDATE OF title, description`. Adding/removing tags in `content_tags`/`ip_tags` (weight tier C) does NOT recalculate `search_vector`.

**Fix**: Add `AFTER INSERT OR DELETE OR UPDATE ON content_tags` trigger:
```sql
CREATE OR REPLACE FUNCTION content_tags_search_vector_update() RETURNS trigger AS $$
BEGIN
  UPDATE content_items SET search_vector =
    setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
    setweight(to_tsvector('simple', coalesce(description, '')), 'B') ||
    setweight(to_tsvector('simple', coalesce(
      (SELECT string_agg(t.tag, ' ') FROM content_tags t WHERE t.content_item_id = content_items.id), ''
    )), 'C')
  WHERE id = COALESCE(NEW.content_item_id, OLD.content_item_id);
  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER content_tags_search_vector_trigger
  AFTER INSERT OR DELETE OR UPDATE ON content_tags
  FOR EACH ROW EXECUTE FUNCTION content_tags_search_vector_update();
```
Same fix needed for `ip_tags`.

---

## 2. Recommendation Service

### File
- `backend/internal/service/recommendation_service.go`

### Status: FUNCTIONAL — Minor issues

| Check | Result |
|-------|--------|
| Cold start | `isColdStart()` checks browse + favorites + likes UNION count vs `MinInteractionForPersonalize` |
| Config-driven thresholds | All from `RecommendationConfig` with sensible fallbacks |
| Weighted profile | Browse=1.0, Favorites=2.0, Likes=1.5 — matches CLAUDE.md spec |
| Final score formula | `alpha×sim + (1−alpha)×hot` at line 289, alpha = `PersonalizationWeight` |
| Redis cache key | `rec:original:{userID}:{page}`, TTL from `RefreshIntervalH` |
| Vector dimension | Dynamically detected via `EstimateEmbeddingDim()` — no hardcoded 1536 |
| Fallback chain | Profile fail → hot; Vector search fail → hot; Score compute fail → hot |
| Anonymous users | `RecommendForAnonymous` → `fallbackToHot` |

### Issues

#### MEDIUM — Profile vector not normalized
After weighted averaging in `buildUserProfile` (line 253: `profile[i] = float32(v / totalWeight)`), the result is NOT L2-normalized. pgvector `<=>` operator (cosine similarity) expects normalized vectors for correct comparison. Mismatched vector magnitudes bias similarity scores.

**Fix**: After computing the weighted average, L2-normalize:
```go
var norm float64
for _, v := range profile {
    norm += float64(v) * float64(v)
}
norm = math.Sqrt(norm)
if norm > 0 {
    for i := range profile {
        profile[i] = float32(float64(profile[i]) / norm)
    }
}
```

#### MEDIUM — `topK×2` multiplier hardcoded
`recommendation_service.go:93`: `s.embeddingRepo.VectorSearch(profile, topK*2)` — the 2× oversampling factor is hardcoded. Should be configurable via `RecommendationConfig.EmbeddingOversampleFactor`.

#### MINOR — Dimension mismatch silently dropped
`buildUserProfile:228-229`: If `content_embeddings` has mixed embedding dimensions, vectors with wrong length are silently skipped with no warning log. Add `slog.Warn` here.

---

## 3. Hot Rank Service

### File
- `backend/internal/service/hot_rank_service.go`

### Status: FUNCTIONAL — Missing graceful shutdown

| Check | Result |
|-------|--------|
| Interval | 10 minutes |
| Overlap prevention | `sync.Mutex.TryLock()` |
| Context timeout | 5 min (rank update), 2 min (IP counts) |
| Embedding gap fill | 1 min ticker, separate `embedMu` mutex |
| Config-driven params | `TrendingWindowDays`, `HotDecayHours` |

### Issues

#### HIGH — No stop signal / graceful shutdown
`Run()` contains an infinite `for { select { case <-rankTicker.C: ... case <-embedCh: ... } }` loop with no `context.Context` or stop channel. On server shutdown, this goroutine leaks and continues running indefinitely.

**Fix**: Add a stop channel:
```go
func (s *HotRankService) Run(stop <-chan struct{}) {
    // ...
    for {
        select {
        case <-stop:
            slog.Info("[hot_rank] shutting down")
            return
        case <-rankTicker.C:
            s.runAll()
        case <-embedCh:
            s.fillEmbeddings()
        }
    }
}
```

#### MEDIUM — 10-minute interval hardcoded
`hot_rank_service.go:51`: `rankInterval := 10 * time.Minute`. Should read from `RecommendationConfig.RankIntervalMin` or similar.

---

## 4. Agent Endpoints

### Files
- `backend/internal/handler/agent.go`
- `backend/internal/service/agent_service.go`
- `backend/internal/handler/routes.go` (lines 221–233)

### Status: FUNCTIONAL — Method mismatch + error exposure

### Registered Routes

All behind `AuthRequired` + `AgentRateLimit` middleware:

| Endpoint | Method | Handler | Matches Spec? |
|----------|--------|---------|---------------|
| `/agent/chat/stream` | POST | `ChatStream` (SSE) | OK |
| `/agent/upload-assist` | POST | `UploadAssist` | OK |
| `/agent/compliance-check` | POST | `ComplianceCheck` | OK |
| `/agent/search` | POST | `NLSearch` | OK |
| `/agent/usage-guide/:id` | **POST** | `UsageGuide` (SSE + sync) | **METHOD MISMATCH** |
| `/agent/moderate/:id` | POST | `Moderate` | Bonus |
| `/agent/conversations` | GET | `ListConversations` | Bonus |
| `/agent/conversations/:id` | GET | `GetConversationMessages` | Bonus |
| `/agent/script/:id` | GET | `GenerateDeployScript` | Bonus |

### Issues

#### HIGH — Task 102 violation: raw error messages exposed to clients
12+ locations in `agent.go` expose raw `err.Error()` to clients, violating the project's error sanitization requirement:

| Line | Endpoint | Current | Required |
|------|----------|---------|----------|
| 59 | `UploadAssist` | `err.Error()` | `"validation failed"` |
| 65 | `UploadAssist` | `err.Error()` | `"agent service error"` |
| 82 | `ComplianceCheck` | `err.Error()` | `"validation failed"` |
| 87 | `ComplianceCheck` | `err.Error()` | `"agent service error"` |
| 102 | `NLSearch` | `err.Error()` | `"validation failed"` |
| 107 | `NLSearch` | `err.Error()` | `"agent service error"` |
| 139 | `UsageGuide` | `err.Error()` | `"agent service error"` |
| 146 | `UsageGuide` | `err.Error()` | `"agent service error"` |
| 159 | `Moderate` | `err.Error()` | `"agent service error"` |
| 178 | `GenerateDeployScript` | `err.Error()` | `"agent service error"` |
| 194 | `ChatStream` | `err.Error()` | `"validation failed"` |
| 212 | `ChatStream` | `err.Error()` | `"agent service error"` |

**Fix**: Replace all with `response.SafeErrorResponse(c, http.Status..., "ERROR_CODE", err)` or generic messages:
```go
// Before (vulnerable):
c.JSON(http.StatusInternalServerError, gin.H{"code": "AGENT_ERROR", "message": err.Error()})

// After (safe):
c.JSON(http.StatusInternalServerError, gin.H{"code": "AGENT_ERROR", "message": "agent service unavailable"})
```

#### MEDIUM — Spec: `GET /agent/usage-guide/:id` registered as POST
Task spec specifies `GET /agent/usage-guide/:id`, but `routes.go:227` registers it as `POST`. The handler correctly supports `?stream=true` SSE mode and non-stream mode, but the HTTP verb is wrong.

**Fix**: Change route registration to allow both GET and POST, or split into GET (non-stream) and POST (stream).

#### MINOR — Orphaned conversation on stream failure
`ChatStream` persists conversation and user messages before the LLM call (`agent_service.go:418-434`). If `ChatStream` fails, orphaned records remain in `agent_conversations` and `agent_messages`.

---

## 5. Admin LLM Config CRUD + PatchConfig

### Files
- `backend/internal/handler/admin.go`
- `backend/config/config.go`

### Status: FULLY FUNCTIONAL

### LLM Config CRUD Endpoints

| Endpoint | Method | Handler | Status |
|----------|--------|---------|--------|
| List | `GET /admin/llm-configs` | `ListLLMConfigs` | OK |
| Create | `POST /admin/llm-configs` | `CreateLLMConfig` | OK |
| Update | `PATCH /admin/llm-configs/:id` | `UpdateLLMConfig` — id/api_key_enc/is_active stripped | OK |
| Delete | `DELETE /admin/llm-configs/:id` | `DeleteLLMConfig` — active config protected | OK |
| Activate | `POST /admin/llm-configs/:id/activate` | `ActivateLLMConfig` | OK |
| Test connection | `POST /admin/llm-configs/:id/test` | `TestLLMConfig` | OK |

### PatchConfig Persistence

`SaveOverride("data/config_override.yaml")` at `config.go:280-296` writes all sections:

| Section | Persisted? | Patchable via API? |
|---------|-----------|-------------------|
| `features` | Yes | Yes (payment_enabled, creator_support_enabled) |
| `limits` | Yes | Yes (6 fields) |
| `reputation` | Yes | Yes (3 fields) |
| `social` | Yes | Yes (2 fields) |
| `agent` | Yes (partial) | Yes (web_agent_enabled, rate_limit_per_day) |
| `judge` | Yes | No |
| `recommendation` | Yes | Yes (personalization_weight, min_interaction_for_personalize) |
| `cache` | Yes | No |
| `rate_limit` | Yes | No |

### Issues

#### MINOR — Override path hardcoded in admin.go
`admin.go:421` hardcodes `"data/config_override.yaml"`. However, `config.go:179-182` supports `CONFIG_OVERRIDE_PATH` env var. If the env var is set, PatchConfig saves to the wrong location.

**Fix**: Change to use the same env var:
```go
path := "data/config_override.yaml"
if v := os.Getenv("CONFIG_OVERRIDE_PATH"); v != "" {
    path = v
}
h.cfg.SaveOverride(path)
```

---

## 6. GoSafe / Panic Recovery Audit

### File
- `backend/internal/pkg/recovery/recovery.go`

### Status: PARTIALLY COMPLIANT — 3 uncovered goroutines

### GoSafe Usage (7 instances — compliant)

| File | Line | Goroutine |
|------|------|-----------|
| `cmd/server/main.go` | 43 | Hot rank scheduler |
| `pkg/scheduler/view_count_sync.go` | 32 | View count sync |
| `pkg/scheduler/tag_usage_sync.go` | 21 | Tag usage sync |
| `pkg/scheduler/judge_question_sync.go` | 28 | Judge question sync |
| `pkg/scheduler/download_count_sync.go` | 32 | Download count sync |
| `service/content_service.go` | 588 | Async notification |
| `service/notification_service.go` | 20 | Push notification |
| `service/agent_service.go` | 393 | `EmbedContentAsync` |

### Raw `go func()` WITHOUT recovery (3 instances)

| File | Line | Risk | Description |
|------|------|------|-------------|
| `handler/content.go` | 473 | **HIGH** | Redis `ZIncrBy` download count — **unprotected panic** |
| `pkg/queue/redis_stream.go` | 54 | **HIGH** | Redis stream consumer loop — panic kills consumer silently |
| `cmd/server/main.go` | 71 | LOW | HTTP `ListenAndServe` — has `os.Exit(1)`, acceptable |

### Fixes Required

**`handler/content.go:473`**:
```go
// Before:
go func() {
    ctx := context.Background()
    h.rdb.ZIncrBy(ctx, "rank:download_counts", 1, fmt.Sprintf("%d", id))
}()

// After:
recovery.GoSafe(func() {
    ctx := context.Background()
    h.rdb.ZIncrBy(ctx, "rank:download_counts", 1, fmt.Sprintf("%d", id))
})
```

**`pkg/queue/redis_stream.go:54`**:
```go
// Before:
go func() {
    for {
        // ... consumer loop
    }
}()

// After:
recovery.GoSafe(func() {
    for {
        // ... consumer loop (with reconnect on panic)
    }
})
```

---

## 7. RecommendationConfig Structure

### File: `backend/config/config.go:146-154`

```go
type RecommendationConfig struct {
    Enabled                      bool    `mapstructure:"enabled"`
    HotDecayHours                float64 `mapstructure:"hot_decay_hours"`
    PersonalizationWeight        float64 `mapstructure:"personalization_weight"`
    MinInteractionForPersonalize int     `mapstructure:"min_interaction_for_personalize"`
    EmbeddingTopk                int     `mapstructure:"embedding_topk"`
    TrendingWindowDays           int     `mapstructure:"trending_window_days"`
    RefreshIntervalH             int     `mapstructure:"refresh_interval_h"`
}
```

**Missing fields** (consider adding):
- `RankIntervalMin` — hot rank update interval (currently hardcoded at 10 min)
- `EmbeddingOversampleFactor` — vector search oversampling (currently hardcoded at 2×)
- `EmbeddingDimensions` — already exists in `AgentConfig.EmbeddingDimensions` (line 126), but not referenced by recommendation service

---

## 8. Summary: Issue Tally

| Severity | Count | Key Items |
|----------|-------|-----------|
| **CRITICAL** | 1 | Chinese FTS broken (`to_tsvector('simple')` — no Chinese word segmentation) |
| **HIGH** | 4 | 2× raw goroutines without panic recovery (content.go:473, redis_stream.go:54); Task 102 error exposure in agent.go (12+ sites); hot rank no shutdown signal; tag changes don't reindex search_vector |
| **MEDIUM** | 5 | Profile vector not L2-normalized; topK×2 hardcoded; hot rank interval hardcoded; agent usage-guide registered as POST instead of GET; config override path hardcoded in admin.go |
| **MINOR** | 3 | Dimension mismatch silently dropped; orphaned conversation on ChatStream failure; config fallback defaults diverging from CLAUDE.md |

---

## 9. Recommended Fix Priority

| Priority | Item | Effort | Impact |
|----------|------|--------|--------|
| P0 | Install `zhparser`/`pg_jieba` for Chinese FTS segmentation | Medium | Unblocks Chinese search |
| P0 | Replace `err.Error()` with safe messages in agent.go (12 sites) | Small | Security compliance (Task 102) |
| P1 | Add `recovery.GoSafe()` to content.go:473 and redis_stream.go:54 | Small | Prevents silent goroutine death |
| P1 | Add `AFTER INSERT/DELETE/UPDATE` triggers on `content_tags` and `ip_tags` | Small | Fixes stale search index |
| P1 | Add stop channel to `HotRankService.Run()` | Small | Clean shutdown |
| P2 | L2-normalize profile vector in `buildUserProfile` | Small | Better recommendation accuracy |
| P2 | Fix `POST /agent/usage-guide/:id` → `GET` (or support both) | Small | Spec compliance |
| P2 | Make `topK×2` multiplier and rank interval configurable | Small | Config compliance |
| P2 | Use `CONFIG_OVERRIDE_PATH` env var in admin.go PatchConfig | Small | Consistency |
| P3 | Add warning log on dimension mismatch in buildUserProfile | Trivial | Observability |
| P3 | Add cleanup of orphaned agent conversations on stream failure | Small | Data hygiene |
