# 2026-07-18 UI Polish 文件预约清单

**创建日期**: 2026-07-17  
**预计失效日期**: 2026-09-17

> 适用范围：`docs/superpowers/plans/2026-07-18-omnicraft-ui-polish-hardening.md` 的 U-01 至 U-12。  
> 当前状态：文档审计中；生产就绪父文档尚未在当前 worktree 找到；未启动、未预约任何实现 Task。

## 预约规则

1. 只有依赖已经合并到当前集成基线的 Task 才能预约。
2. 启动前填写真实 owner、分支、base commit、预约时间和精确路径；`Day 1` 之类占位时间不构成预约。
3. 同一路径只能有一个活动 owner。需要越界时，先追加释放/转移记录，不得静默编辑。
4. 分支使用 `codex/excellence/u-XX`；一个 Task 对应一个 worktree、一个经双阶段审查的 commit。
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
| U-02 | `frontend/components/ui/**` | U-01 | 待依赖 | — |
| U-03 | Header、Sidebar、StudioSidebar、admin layout、collapse hook | U-01/U-02/U-05 | 待依赖 | — |
| U-04 | dashboard destructive pages、settings、agent-config、VersionHistory | U-01/U-02/U-05 | 待依赖 | — |
| U-05 | error utilities + audited error surfaces | — | 可预约 | — |
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
| — | — | — | — | — | — | 暂无活动预约 |

## 历史释放、阻塞与转移记录

| 时间（Asia/Shanghai） | Task | Owner | 事件 | 影响路径 | 说明 |
|---|---|---|---|---|---|
| 2026-07-17 | U-11 | — | 阻塞 | U-11 全部范围 | 当前 worktree 缺少生产就绪规格与计划，无法确认公开配置安全边界；未启动实现 |
