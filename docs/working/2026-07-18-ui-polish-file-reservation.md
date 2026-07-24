# 2026-07-18 UI Polish 文件预约清单

**创建日期**: 2026-07-17  
**预计失效日期**: 2026-09-17

> 适用范围：`docs/superpowers/plans/2026-07-18-omnicraft-ui-polish-hardening.md` 的 U-01 至 U-12。  
> 当前状态：按 2026-07-25 修订后的执行顺序推进；U-05、U-02A、U-04 已完成释放，下一步为 P-01 人工原型评审。

## 预约规则

1. 只有依赖已经合并到当前集成基线的 Task 才能预约。
2. 启动前填写真实 owner、分支、base commit、预约时间和精确路径；`Day 1` 之类占位时间不构成预约。
3. 同一路径只能有一个活动 owner。需要越界时，先追加释放/转移记录，不得静默编辑。
4. 按当前计划使用共享 light 车道分支 `codex/ui-refinement`，每个 Task 保持独立逻辑 commit 与审查记录。
5. 状态只使用 `待依赖`、`可预约`、`进行中`、`阻塞`、`已释放`、`已合并`。中止/阻塞必须写时间和原因。
6. 每次提交精确暂存，不得使用 `git add .`。预约信息写入 commit body 或 progress 记录；不强制破坏仓库既有的 commit subject 格式。
7. 本文件保留全部历史，只追加记录；完成本专项或到期后移动到归档流程，不删除审计轨迹。

## 依赖与并行波次

| 波次 | Task | 条件 | 可并行性 |
|---|---|---|---|
| 1 | U-01、U-05 | 无代码依赖 | 两者文件不重叠，可并行 |
| 2 | U-02 | U-01 已合并 | 独占 `components/ui/**` |
| 3 | U-03、U-04、U-06、U-07、U-08、U-09、U-10、U-11 | U-01、U-02、U-05 已合并；U-11 还需恢复生产就绪父文档 | 文件所有权已拆开，可按可用 worker 并行 |
| 4 | U-12 | U-01 至 U-11 已合并 | 独占治理脚本、package 与项目验证入口 |

## 当前任务状态

| Task | 文件所有权摘要 | 依赖 | 状态 | 当前 owner / 分支 / base / 时间 |
|---|---|---|---|---|
| U-01 | `design-system.md`、`globals.css`、token checker | — | 可预约 | — |
| U-02A | Button、ConfirmModal、TagBadge、Toast 行为与无障碍 | U-05 | 已释放 | Codex / `codex/ui-refinement` / `c4b24f9` / 2026-07-25 03:11–03:44 +08:00 |
| U-02B | `frontend/components/ui/**` 视觉应用 | P-01/U-01/U-02A | 待依赖 | — |
| U-03 | Header、Sidebar、StudioSidebar、admin layout、collapse hook | U-01/U-02/U-05 | 待依赖 | — |
| U-04 | dashboard destructive pages、settings、agent-config、VersionHistory | U-01/U-02/U-05 | 已释放 | Codex / `codex/ui-refinement` / `4693cc3` / 2026-07-25 |
| U-05 | error utilities + audited error surfaces | — | 已释放 | Codex / `codex/ui-refinement` / `d1f53cd` / 2026-07-25 02:52–03:06 +08:00 |
| U-06 | feedback/discussion detail、SheetMusicViewer、VerdictDetail | U-01/U-02/U-05 | 待依赖 | — |
| U-07 | DataList + history/appeals/rehab/feedback/studio/discussion lists | U-01/U-02/U-05 | 待依赖 | — |
| U-08 | remaining settings/admin/discussion forms | U-01/U-02/U-05 | 待依赖 | — |
| U-09 | content/home/search/IP discovery surfaces | U-01/U-02/U-05 | 待依赖 | — |
| U-10 | messages catalogs + communication/agent surfaces | U-01/U-02/U-05 | 待依赖 | — |
| U-11 | auth capability contract + interaction consumers | U-01/U-02/U-05 + production-readiness coordination | 阻塞 | 缺少生产就绪父文档，2026-07-17 |
| U-12 | UI governance scripts、package、project verifier | U-01…U-11 | 待依赖 | — |

精确文件列表以计划中对应 Task 的 `Files` 段为准。预约时必须复制为明确路径；摘要不能代替路径清单。

## 活动预约记录

| 时间（Asia/Shanghai） | Task | Owner | 分支 | Base commit | 精确预约路径 | 状态 / 原因 |
|---|---|---|---|---|---|---|
| 2026-07-25 02:52 +08:00 | U-05 | Codex | `codex/ui-refinement` | `d1f53cd` | `frontend/lib/error-handler.ts`; `frontend/lib/user-facing-error.ts`; `frontend/app/error.tsx`; `frontend/app/(public)/verify-email/page.tsx`; `frontend/app/(protected)/dashboard/pr-requests/page.tsx`; `frontend/app/(public)/user/[userId]/UserProfileClient.tsx`; `frontend/components/content/ContentDetail.tsx`; `frontend/components/content/VersionHistory.tsx`; `frontend/components/content/DownloadButton.tsx`; `frontend/components/feedback/FeedbackForm.tsx`; `frontend/tests/error-handler-i18n.test.ts`; `frontend/tests/user-facing-error-surfaces.test.tsx` | 已释放；原计划路径验证完成 |
| 2026-07-25 02:58 +08:00 | U-05 | Codex | `codex/ui-refinement` | `d1f53cd` | `frontend/app/(protected)/admin/agent-config/page.tsx`; `frontend/app/(protected)/admin/appeal/page.tsx`; `frontend/app/(protected)/admin/audit-logs/page.tsx`; `frontend/app/(protected)/admin/categories/page.tsx`; `frontend/app/(protected)/admin/config/page.tsx`; `frontend/app/(protected)/admin/feedback/page.tsx`; `frontend/app/(protected)/admin/ips/page.tsx`; `frontend/app/(protected)/admin/queue/page.tsx`; `frontend/app/(protected)/admin/reports/page.tsx`; `frontend/app/(protected)/admin/users/page.tsx`; `frontend/app/(protected)/appeals/page.tsx`; `frontend/app/(protected)/dashboard/contributors/page.tsx`; `frontend/app/(protected)/dashboard/tag-suggestions/page.tsx`; `frontend/app/(protected)/feedback/[feedbackId]/page.tsx`; `frontend/app/(protected)/feedback/mine/page.tsx`; `frontend/app/(protected)/ip/[ipId]/discussions/new/page.tsx`; `frontend/app/(protected)/judge/exam/page.tsx`; `frontend/app/(protected)/judge/queue/page.tsx`; `frontend/app/(protected)/rehab/page.tsx`; `frontend/app/(protected)/settings/page.tsx`; `frontend/app/(protected)/settings/tag-groups/page.tsx`; `frontend/app/(public)/ip/[ipId]/discussions/[discussionId]/page.tsx`; `frontend/app/(public)/ip/[ipId]/discussions/page.tsx`; `frontend/components/agent/UploadAssistPanel.tsx`; `frontend/components/judge/VerdictDetail.tsx`; `frontend/components/social/CreatorSupportPanel.tsx`; `frontend/components/social/ReplyList.tsx` | 已释放；精确库存发现并清除 27 个额外原始错误渲染面 |
| 2026-07-25 03:11 +08:00 | U-02A | Codex | `codex/ui-refinement` | `c4b24f9` | `frontend/components/ui/button.tsx`; `frontend/components/ui/confirm-modal.tsx`; `frontend/components/ui/TagBadge.tsx`; `frontend/components/ui/Toast.tsx`; `frontend/messages/en.json`; `frontend/messages/zh.json`; `frontend/tests/runtime-test-helpers.tsx`; `frontend/tests/ui-primitives-accessibility.test.tsx` | 已释放；原语行为、无障碍名称与触控目标验证完成 |
| 2026-07-25 03:37 +08:00 | U-02A | Codex | `codex/ui-refinement` | `c4b24f9` | `frontend/tests/admin-notifications-page.test.tsx`; `frontend/tests/collection-detail.test.tsx`; `frontend/tests/history-page.test.tsx`; `frontend/tests/messages-components.test.tsx`; `frontend/tests/series-detail-page-contract.test.tsx`; `frontend/tests/sheet-music-viewer-download.test.tsx`; `frontend/tests/studio-series-page.test.tsx`; `frontend/tests/user-collections-page.test.tsx` | 已释放；为既有 Toast 测试目录补齐 `common.close` 测试消息契约 |

## 历史释放、阻塞与转移记录

| 时间（Asia/Shanghai） | Task | Owner | 事件 | 影响路径 | 说明 |
|---|---|---|---|---|---|
| 2026-07-17 | U-11 | — | 阻塞 | U-11 全部范围 | 当前 worktree 缺少生产就绪规格与计划，无法确认公开配置安全边界；未启动实现 |
| 2026-07-25 03:06 +08:00 | U-05 | Codex | 完成并释放 | U-05 全部预约路径 | 35 个明确审计面改用本地化安全回退；重叠路径按计划依次转交 U-02A/U-04 |
| 2026-07-25 03:44 +08:00 | U-02A | Codex | 完成并释放 | U-02A 全部预约路径 | 5 项原语测试、真实键盘流和 44px 粗指针关闭目标通过；共享原语转交 U-02B，ConfirmModal 转交 U-04 使用 |
| 2026-07-25 | U-04 | Codex | 完成并释放 | `frontend/app/(protected)/dashboard/contents/page.tsx`; `frontend/app/(protected)/dashboard/contributors/page.tsx`; `frontend/app/(protected)/dashboard/pr-requests/page.tsx`; `frontend/app/(protected)/settings/page.tsx`; `frontend/app/(protected)/admin/agent-config/page.tsx`; `frontend/components/content/VersionHistory.tsx`; `frontend/components/ui/confirm-modal.tsx`; `frontend/messages/en.json`; `frontend/messages/zh.json`; `frontend/tests/destructive-dialogs.test.tsx` | 已释放；单元契约、全量前端测试、lint/build 通过；源码仅保留 U-11 的 ReactionBar prompt。受保护页面 Playwright mock 因会话依赖停留加载，未计为通过 |
