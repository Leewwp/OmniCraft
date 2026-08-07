# OmniCraft CI Gate Recovery Implementation Plan

**Goal:** 恢复 `project-gate`、SBOM/provenance 与 Security 三条在 `main` 上预存全红的发布门，使后续 PR 不再依赖 admin merge。

**Lane:** heavy。三个修复是相互独立的重车道任务；每个任务使用独立 worktree、分支和单一提交，先保留/补充能在 Linux CI 复现的失败测试，再做最小修复和两阶段审查。父 issue #92 只负责聚合，不承载实现提交。

**Source:** GitHub issue #92、`.github/workflows/{ci,sbom,security}.yml`、`scripts/{ops,release,security}/` 与 `docs/superpowers/plans/2026-07-17-omnicraft-production-readiness.md` 的既有门禁合同。

## Task G1 [heavy]: Restore Ops alerting gates

- [x] 在 Ubuntu/Linux 环境复现 `verify-alerts.tests.sh` 的失败，并让失败输出保留 promtool/amtool 根因。
- [x] 修复容器运行、挂载或平台差异，不降低 Prometheus/Alertmanager 配置校验强度。
- [x] 本地合同测试与对应 GitHub Actions job 通过；完成规格符合性和代码质量审查。
- [x] 精确更新 `progress.txt`，独立提交并关闭对应子票。

## Task G2 [heavy]: Restore SBOM and provenance gates

- [x] 保留 GitHub runner 上 root-owned bind mount 的回归测试或等价 Linux seam，确认修复前失败。
- [x] 让 Syft 容器以调用者 UID/GID 写入，并提供可写 HOME/TMPDIR；不得放宽 SBOM 内容、确定性或 provenance 校验。
- [x] `generate-sbom.tests.sh`、实际生成命令与对应 GitHub Actions job 通过；完成两阶段审查。
- [x] 先检查并保留工作区已有 `scripts/release/generate-sbom.sh` 修改，独立提交并关闭对应子票。

## Task G3 [heavy]: Restore Security gates

- [x] 用当前存量 fixture 复现 gitleaks `generic-api-key` 误报，同时保留“真实 fake secret 必须失败”的正向检出合同。
- [x] 使用最窄的路径/指纹 allowlist 或等价机制排除有意测试数据；不得把所有 `*.tests.sh`、`docs/archive/**` 或 generic-api-key 规则整体关闭。
- [x] `verify-security.tests.sh`、完整 `verify-security.sh -BuildImages` 与对应 GitHub Actions job 通过；完成两阶段审查。
- [x] 精确更新 `progress.txt`，独立提交并关闭对应子票。

## Parent closure gate [heavy]

- [ ] 三个子票全部合入 `main`。
- [ ] `project-gate`、SBOM/provenance 和 Security 在 `main` 连续两次全绿。
- [ ] 任意一个后续普通 PR（不使用 admin merge）通过 required checks。
- [ ] 将 run URL、commit 和本地合同命令记录到 #92 与 `progress.txt`，再关闭父 issue。

