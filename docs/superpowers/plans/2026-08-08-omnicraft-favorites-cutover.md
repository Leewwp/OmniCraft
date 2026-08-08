# OmniCraft Legacy Favorites Cutover Implementation Plan

**Goal:** 在旧 favorites 运行时依赖已经退役并上线后，通过可审计、可恢复、forward-only 的 `067_drop_legacy_favorites.sql` 删除旧表，且不破坏收藏集、内容详情和推荐行为。

**Lane:** heavy + manual production gate。该计划对应 GitHub issue #76，代码部分使用独立 worktree、分支和单一提交；云端日志、备份恢复和生产 cutover 必须由有权限的人工操作者确认。人工门满足前 issue 保持 `ready-for-human`，不得把 mock、local dump 或测试日志当成生产证据。

**Source:** `docs/superpowers/specs/2026-08-07-omnicraft-web-experience-corrections-design.md` 的 Implementation Decisions 25–32、Testing Decisions 11–16/19，以及 `docs/deploy/single-server-beta-runbook.md` 的 migration ledger、backup/restore 与 release 流程。

## Non-negotiable release topology

本 cutover 是两个已经分票的发布阶段，不得在一次首次部署中同时移除运行时依赖并先删表：

1. **Compatibility release (#75):** 合入并部署完全不读写 `favorites` 的应用；旧端点已经不注册，但旧表仍存在。完成收藏集、内容详情、推荐和旧端点 404 smoke。
2. **Drop release (#76):** 经过云端日志与可恢复备份人工门后，发布只新增 forward-only `067_drop_legacy_favorites.sql` 及必要 migration tests/docs 的候选版本。现有 release runner 可先执行迁移，因为线上正在运行的 #75 版本已经与无表 schema 兼容。

若 #75 尚未在目标环境成功运行，禁止执行 #76 migration。

## Manual gate evidence contract

### Access logs

- 精确查询旧 endpoint family：
  - `POST /api/v1/favorites`
  - `DELETE /api/v1/favorites/:contentId`
  - `GET /api/v1/users/:id/favorites`
- 查询窗口从 #75 deployed-at 开始，到 cutover approval 时结束，且至少覆盖一个完整正常流量周期（24 小时）。同时查询日志系统可用的更长保留窗口，以发现仍存活的旧客户端/脚本。
- 允许的命中只有带已登记 operator user-agent/request-id 的主动 404 smoke；任何其他命中均阻塞 cutover。报告必须保留 UTC start/end、查询文本或 dashboard permalink、命中数和排除项身份，不记录 token、Cookie 或用户私密 payload。

### Recoverable backup

- 紧邻 migration 前使用 `scripts/backup-db.sh` 生成 custom-format dump 与 manifest；manifest 必须绑定源数据库、SHA-256 与完整 migration ledger，head 至少为 `066_ip_visit_history.sql`。
- 使用 `scripts/restore-db.sh` 恢复到一个**新数据库**，验证 checksum、ledger 和 smoke；额外断言恢复库仍包含 `favorites` 及预期行数摘要。不得把 restore drill 指向生产数据库。
- 记录 backup ID/path 的脱敏标识、manifest SHA-256、restore target 的临时名称、restore run URL/日志和完成时间。原始 dump、DSN 与访问日志不得提交到 Git。
- 将 redacted evidence 放入 cutover report/artifact，并使用现有 durable evidence archive 流程；#76 评论只写 durable URL、hash、commit 和结论。

## Expected code files

- Create: `backend/migrations/067_drop_legacy_favorites.sql`
- Modify: existing PostgreSQL migration integration tests that assert latest schema; prefer a focused `backend/internal/model/favorites_cutover_migration_test.go` when no suitable seam exists.
- Modify if generated: `architecture.md`, `docs/reference/schema.md`, `docs/reference/api.md`, `docs/reference/config.md` through `tools/doc-validator` only.
- Modify: `progress.txt` after all required local gates; production evidence is appended only after real cutover.

Historical migrations, `backend/internal/migration/testdata/historical-050.sql`, and their recorded checksums are read-only evidence and must not be edited.

## Task C1 [heavy]: Preflight and red test

- [ ] Confirm #75 is merged and deployed; record its immutable image digest/commit and successful collection/detail/recommendation/legacy-404 smoke.
- [ ] Confirm #73/#97 have landed so `066_ip_visit_history.sql` exists and is applied; `067_` must be unoccupied.
- [ ] Run repository scan. Outside historical migrations/fixtures/archive docs and the planned drop migration, supported runtime, current tests, seeds and maintenance commands must have no `Favorite`, `favorites`, `/favorites` dependency.
- [ ] Write a PostgreSQL migration test that expects the latest empty-db and historical-fixture upgrade schemas to omit `favorites` while retaining `collections`, `collection_items` and `ip_visit_history`.
- [ ] Run the focused migration test and record the expected failure because `067_drop_legacy_favorites.sql` is absent.

## Task C2 [heavy]: Minimal forward-only migration

- [ ] Add `067_drop_legacy_favorites.sql` using an idempotent `DROP TABLE IF EXISTS favorites`; do not edit `011_social.sql` or `058_create_collections.sql`.
- [ ] Include a `-- ROLLBACK:` note stating that shared/production rollback is not an automatic down migration: restore data into an isolated database, investigate, and ship an approved forward-fix if a legacy table must be reconstructed.
- [ ] Run focused migration tests for empty DB, historical fixture and already-applied 066 ledger; verify `to_regclass('public.favorites') IS NULL` and required collection/IP tables remain.
- [ ] Run `cd tools/doc-validator && go run . --fix`, `go test ./...`, `go vet ./...`, `go build ./...` and `bash scripts/verify-project.sh --full`.
- [ ] Complete specification review and code-quality review before producing the candidate commit.

## Task C3 [manual gate]: Authorize production execution

- [ ] Attach the access-log evidence contract above and verify zero non-operator hits.
- [ ] Capture the pre-migration backup and complete restore-to-new-database verification; verify backup ledger head and `favorites` presence.
- [ ] Confirm the deployed #75 digest is schema-compatible with 067 and remains available as the safe application fallback. A pre-#75 digest that reads `favorites` is explicitly ineligible for rollback.
- [ ] Record candidate commit/image digest, migration manifest hash, approver and UTC approval time in #76. Only now may the issue transition from waiting-for-human to execution.

## Task C4 [manual heavy]: Execute cutover

- [ ] Run the standard release preflight and dry-run migration plan; the plan must contain exactly the expected pending migrations and end at `067_drop_legacy_favorites.sql`.
- [ ] Deploy through the existing immutable release workflow. Preserve migrate logs and `migration-summary.json`; confirm the ledger records 067 with the repository checksum.
- [ ] Verify database invariants: `favorites` absent; `collections`, `collection_items`, `ip_visit_history` present; no invalid constraints.
- [ ] Run authenticated smoke for collection add/remove, zero/one/multiple collection membership in content detail, collection picker state, and recommendation profile/cold-start behavior.
- [ ] Verify all three retired endpoint shapes return the intended 404/method-not-registered result and do not leak raw errors.
- [ ] Save post-cutover application logs/metrics and archive the evidence with durable URLs/hashes.

## Failure and recovery boundaries

- **Before migration starts:** any missing/ambiguous log, backup, restore, digest or dry-run evidence means stop with no production mutation.
- **Migration failed or attempt state unknown:** freeze later migrations and do not blind-retry. Inspect `schema_migration_attempts`/ledger and follow the explicit reconciliation approval flow. Because the migration is transactional, first determine whether the table still exists; do not assume either outcome.
- **Migration succeeded, application smoke failed:** keep or redeploy the already proven #75-compatible application digest and forward-fix the application/schema. Never roll back to an older digest that reads `favorites`; never run destructive down SQL.
- **Legacy data investigation required:** restore the captured backup only into an isolated database. Production restoration/reconstruction requires a separate approved forward-fix; do not overwrite the live database from the backup.
- **Post-cutover old-client traffic appears:** the endpoints remain retired. Record the caller, assess support impact and ship an explicit compatibility decision; do not silently recreate the table or routes.

## Closure

- [ ] Local red→green evidence, two-stage reviews and all verification gates are recorded in `progress.txt`.
- [ ] #76 contains access-log query evidence, backup/restore identity, #75 deployed digest, #76 candidate/migration checksum, migration summary and post-smoke links.
- [ ] Production database and a fresh empty database both have the intended latest schema; historical checksums remain unchanged.
- [ ] Close #76 only after the durable evidence archive is readable and the post-cutover observation window shows no regression.

