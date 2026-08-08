# OmniCraft IP Visit History Implementation Plan

**Goal:** 为匿名与登录用户提供连续、可重试且账号隔离的最近 IP 访问体验；登录用户使用独立的 `ip_visit_history`，不复用内容浏览历史。

**Lane:** heavy。该计划对应 GitHub issue #73，使用独立 worktree、分支和单一提交；必须先保留预期失败的 migration/HTTP/browser 测试证据，再做最小实现和两阶段审查。

**Source:** `docs/superpowers/specs/2026-08-07-omnicraft-web-experience-corrections-design.md` 的 Implementation Decisions 16–18、Testing Decisions 10，以及 `design/ui-spec.md` 的 `/ip/[ipId]`、`IPCard` 和首页最近访问区域。

## Fixed contract

### Persistence

- Migration: `backend/migrations/066_ip_visit_history.sql`。`063`/`064`/`065` 分别归 media metadata、source-linkage、collaboration-invites；实现时若任一预留变化，停止并同步注册表、计划与 GitHub dependencies。
- Table: `ip_visit_history(user_id, ip_id, visited_at)`，复合主键 `(user_id, ip_id)`；两个外键均 `ON DELETE CASCADE`。
- Index: `(user_id, visited_at DESC, ip_id DESC)`，保证相同时间戳下稳定排序。
- 重复访问使用 upsert；直接访问使用服务端当前 UTC 时间，匿名合并使用 `GREATEST(existing.visited_at, incoming.visited_at)`，旧重放不得降低 recency。
- 首版只限制返回最近六项，不主动清理更老的账号记录；匿名 `recent_ips` 仍最多保存六项。

### HTTP API

三条路由均需要认证，并挂在现有 `/api/v1/users/me/...` 资源下：

1. `GET /api/v1/users/me/ip-visits`
   - `200 { "items": [{ "ip": { ...public IP summary... }, "visited_at": "RFC3339" }], "limit": 6 }`
   - 只返回当前用户记录，按 `visited_at DESC, ip_id DESC`，最多六项；不得接受任意 `user_id`。
2. `PUT /api/v1/users/me/ip-visits/:ipId`
   - 以服务端接收时间记录或刷新一次访问，成功返回 `204`；重复请求幂等。
   - IP 不存在或当前不可公开访问返回 `404 IP_NOT_FOUND`，数据库错误返回脱敏 `500 IP_VISIT_RECORD_FAILED`。
3. `POST /api/v1/users/me/ip-visits/merge`
   - Request: `{ "visits": [{ "ip_id": 123, "visited_at": "RFC3339" }] }`，最多六项；空数组合法且返回当前最近列表。
   - 先校验整个 payload 的结构、时间格式和 batch cap；格式错误、缺字段或超过六项返回 `400 INVALID_IP_VISIT_MERGE`，且不写入任何记录。
   - 同一 payload 内重复 `ip_id` 先折叠为最新时间。客户端未来时间统一 clamp 到本次请求的服务端接收时间；旧时间允许，因为只影响该用户自己的排序。
   - 格式正确但 IP 已不存在/不可公开访问时，不让该项永久阻塞登录合并：有效项在一个事务内 upsert，失效项返回到 `discarded_ip_ids`。成功响应为 `200 { "accepted_ip_ids": [...], "discarded_ip_ids": [...], "items": [...] }`。
   - 事务失败返回脱敏 `500 IP_VISIT_MERGE_FAILED`；不得返回部分成功响应。

客户端只有在 merge 返回 `200` 后，才删除本次提交中服务端明确列入 `accepted_ip_ids` 或 `discarded_ip_ids` 的本机项。网络错误、401、400 或 500 均保留原始本机记录并允许重试。

## Expected files

### Backend

- Create: `backend/migrations/066_ip_visit_history.sql`
- Create: `backend/internal/model/ip_visit_history.go`
- Create: `backend/internal/repository/ip_visit_history_repo.go`
- Create: `backend/internal/handler/ip_visit_history.go`
- Modify: `backend/internal/router/routes.go`
- Test: `backend/internal/model/ip_visit_history_migration_test.go`
- Test: `backend/internal/handler/ip_visit_history_test.go`

### Frontend

- Create: `frontend/lib/ip-visit-history.ts`，集中管理旧 `recent_ips` 兼容解析、去重、六项 cap、record/merge/list 合同。
- Modify: `frontend/components/ip/IPCard.tsx`
- Modify: `frontend/components/home/HomePageClient.tsx`
- Modify: 现有认证状态入口（优先 `frontend/contexts/AuthContext.tsx`；实现前以实际登录成功 seam 为准）以触发一次幂等 merge。
- Modify: `frontend/messages/zh.json`, `frontend/messages/en.json` only if visible failure/empty copy is required.
- Test: focused frontend contract test and browser journey under `frontend/e2e/`.
- Read before UI changes: `design/ui-spec.md` sections for `/ip/[ipId]`, `IPCard`, and the page that owns the recent-IP list.

## Task H1 [heavy]: Red tests and migration

- [ ] 确认 #97、#69/#71/#72 的共享文件序列已完成，`065_collaboration_invites.sql` 已存在，且 `066_` 未被占用。
- [ ] 写 PostgreSQL migration test：复合主键、两个 cascade FK、稳定排序索引、空库升级与 historical fixture 升级。
- [ ] 运行 focused test 并记录因 `066_ip_visit_history.sql` 不存在而预期失败的证据。
- [ ] 添加幂等 forward-only migration 与 model；migration 含本地测试用 `-- ROLLBACK:` 注释，共享环境只允许 forward-fix。
- [ ] 运行 migration focused test 至 green。

## Task H2 [heavy]: Repository and HTTP contract

- [ ] 先写 HTTP seam 失败测试，覆盖 record、重复刷新、当前用户隔离、稳定六项排序、未来时间 clamp、payload 内重复、失效 IP、batch > 6、数据库失败不暴露原始错误。
- [ ] 实现 repository upsert/list/merge transaction；所有 SQL 通过 GORM 参数化表达，不拼接用户输入。
- [ ] 实现三条认证路由与统一安全错误 envelope；GET/merge 只返回 public IP summary。
- [ ] 重跑 focused handler/router tests；运行 `go test ./...`、`go vet ./...`、`go build ./...`。

## Task H3 [heavy]: Browser integration

- [ ] 先写前端失败测试：匿名去重/六项、登录 merge 失败保留、本次 200 只清理 acknowledged IDs、登录后列表改读 server。
- [ ] 将 `recent_ips` 的读写从 `IPCard`/`HomePageClient` 收敛到 `frontend/lib/ip-visit-history.ts`；兼容读取当前本机旧结构，不做破坏性一次性清空。
- [ ] IP 激活仍即时写本机记录；已登录时调用 record API。登录状态由未认证转为认证时触发一次 merge，401 不循环重试。
- [ ] 浏览器验证匿名访问六个以上 IP、重复访问置顶、登录成功合并、故意制造 500 后本机记录保留、重试成功后清理；保存桌面与移动截图到 `screenshots/`。
- [ ] 运行 frontend focused tests、`npm run lint`、`npm run build` 与相关 Playwright journey。

## Task H4 [closure]: Verification and tracking

- [ ] 修改 migration/routes 后运行 `cd tools/doc-validator && go run . --fix`，核对自动生成 API/schema 文档包含新路由与 `066`。
- [ ] 运行 `bash scripts/verify-project.sh --full`，以及 migration historical-fixture gate。
- [ ] 完成规格符合性审查，再完成代码质量审查；处理所有 blocking 与 `DONE_WITH_CONCERNS` 项。
- [ ] 在 `progress.txt` 记录 red→green、浏览器截图、迁移验证和审查结果；只勾选 #73/本计划实际完成项。
- [ ] 精确 stage 实际改动并创建该 heavy task 的唯一提交；不得混入 #75/#76 或其他计划文件。

## Stop conditions

- `066_` 被占用、`065` 尚未落地或 GitHub dependency 与注册表不一致：停止并先修正规划。
- PostgreSQL migration/fixture 失败：按 AGENTS.md 阻塞处理，不以 SQLite 或手写 schema 模拟替代。
- 浏览器认证 seam 无法在不改动共享未合并文件的情况下安全接线：记录文件冲突并等待上游合并，不并行覆盖。

