# 2026-07-16 Worktree Rescue Audit

**创建日期**: 2026-07-16
**预计失效日期**: 2026-09-16
**审计基线**: `codex/docs/productization-baseline` at `be6db7e`
**用途**: 在 Phase 1 开始前记录旧 worktree 的保留、移植、过期和可清理结论；本文不是新的功能计划或完成状态来源。

---

## 1. 审计规则

- 未执行 `reset`、`clean`、强制删除、整分支合并或 cherry-pick。
- `__pycache__`、`frontend/test-results` 等生成物不作为业务改动。
- 历史报告只有在当前 release commit 上重跑后才能作为发布证据。
- 旧测试只有在最新基线上仍能稳定暴露缺口时才移植；移植时必须从最新 integration 新建分支并重新完成 red-green-refactor。
- 分支含非 patch-equivalent 提交时，即使 worktree 干净也不得直接删除分支。

## 2. 结论矩阵

| Worktree / 分支 | 相对 `main` | 工作区状态 | 结论 | 后续动作 |
|---|---:|---|---|---|
| `browse-history` / `codex/community/browse-history` | `main` 多 4，分支独有 0 | 干净 | 可清理 worktree | 确认不再需要本地运行后可非强制移除；分支提交已是 `main` 祖先 |
| `collections` / `codex/community/collections` | `main` 多 1，分支独有 0 | 仅 `frontend/test-results/.last-run.json` | 可清理，但先处理生成物 | 确认测试产物无需保留后再非强制移除；不得提交该产物 |
| `content-series` / `codex/community/content-series` | 审计时与 `main` 相同 | 干净 | 可清理并重建 | Phase 1 和早期 Ops 完成后，从最新 `codex/community-integration` 重建，不能继续使用旧基线 |
| `repair-feedback-reports` | `main` 多 110，分支独有 2 | 3 个未跟踪测试、1 个修改测试、2 个 pyc | 不移植当前未提交测试；保留分支 | 当前基线已有更完整覆盖；旧 migration test 错指 `053_web_beta_review_repairs.sql`，真实文件为 `055_web_beta_review_repairs.sql`；清理前仍需单独决定两个独有 commit 的去留 |
| `repair-validation-evidence` | `main` 多 110，分支独有 4 | 未跟踪失败报告、`progress.txt`、2 个 pyc | 报告过期，不可用于发布 | 2026-06-03 报告明确基于 `dbbd55f` 且结论为 FAIL；三个 repair commit 已 patch-equivalent 进入 `main`，剩余旧反馈 commit 不得整分支合并 |
| `repair-verification-lifecycle-task2` | `main` 多 105，分支独有 1 | staged 测试重命名和 progress、2 个 pyc | 未提交改动已被当前测试语义取代 | 当前基线已有 cache invalidation、并发 double-submit、single-use 测试并通过；不移植仅重命名的旧 staged diff，删除前需人工确认 |
| `ui-detail-repair` / `codex/ui-detail-repair` | `main` 多 110，分支独有 27 | 干净 | 必须保留，专项救援 | `git cherry` 显示 16 个 patch-equivalent、11 个非 patch-equivalent；三点 diff 横跨 138 个文件，禁止 merge/cherry-pick 整分支，需按 11 个提交逐项重放测试和 UI 规格核验 |

## 3. 当前基线复核证据

以下命令在 `be6db7e` 文档基线代码祖先上重新运行，用于判断旧未提交测试是否仍暴露缺口：

```text
go test ./internal/repository -run "TestReportUpdateStatusPersistsActionTaken" -v
PASS

go test ./internal/handler -run "TestFeedback|TestAdminPatchFeedback|TestAdminFeedback" -v
PASS

go test ./internal/service -run "TestFeedback|TestVerifyEmail|TestResetPassword" -v
PASS
```

首次冷缓存 handler 编译超过 304 秒工具上限；在 service 构建缓存完成后原命令重跑用时约 2.3 秒并通过。这是本机冷编译耗时，不是测试失败。

## 4. `repair-feedback-reports` 详细判断

- 旧 `search_repo_test.go` 新增的 `action_taken` 持久化测试，当前 `main` 已由 `TestReportUpdateStatusPersistsActionTaken` 覆盖且通过。
- 旧 `feedback_test.go` 仅覆盖匿名 `user_id` 和 OSS 未配置；当前 handler 测试包含等价行为并通过。
- 旧 `feedback_service_test.go` 覆盖范围小于当前文件；当前服务测试还覆盖 grant namespace、失败恢复、通知和邮件失败。
- 未跟踪 `feedback_migration_test.go` 检查已不存在的 `053_web_beta_review_repairs.sql`，不能移植；当前修复迁移为 `055_web_beta_review_repairs.sql`。

结论：未发现需要从该 worktree 直接移植的未提交测试。两个独有历史 commit 仍需在删除分支前进行提交级语义审计。

## 5. `ui-detail-repair` 专项救援入口

11 个非 patch-equivalent 提交为：

1. `4fb78c6` — restore feedback and report workflows
2. `5cda58e` — harden auth session and release gates
3. `bd3e1c8` — fix verification token lifecycle
4. `e9705a7` — fix validation blocker search suggestions
5. `db9f2c8` — complete validation evidence follow-up
6. `636e990` — harden release secrets
7. `7672891` — add UI detail regression coverage
8. `0e187e0` — repair auth form feedback and accessibility
9. `a7c663d` — finish flat interaction cleanup
10. `8183a06` — repair mobile verification blockers
11. `ce9fce9` — verify UI detail repair

专项审计必须按提交逐个回答：

1. 当前 `main` 是否已用不同实现满足同一行为；
2. 旧测试能否在最新基线上稳定失败；
3. 改动是否仍符合当前 `design/ui-spec.md`；
4. 是否与 Hardening、社区和 Web Agent 计划的共享文件冲突；
5. 应重写为新任务、仅移植测试，还是判定过期。

在该审计完成前，不得删除 `codex/ui-detail-repair` 分支或 worktree。

## 6. Phase 1 入口

- Phase 1 不复用上述任何旧 worktree。
- 从文档基线合入后的最新 `main` 创建 `codex/productization-integration`。
- Hardening Task 1/2 使用新的 `codex/excellence/task-1`、`codex/excellence/task-2` worktree。
- Hardening Task 3 开始前冻结 route owner、container、server main 和 doc-validator route sync 的并行写入。
