# OmniCraft Beta 半自动协调脚本

这组脚本用于安全地执行 `docs/superpowers/plans/` 下的双轨 Beta 计划。默认行为偏保守：

- 不自动 stash、commit、push 或删除 worktree。
- 不自动启动 Claude。只有传入 `-LaunchClaude` 才启动。
- 不自动合并。只有传入 `-ConfirmMerge` 才运行完整验证并执行 `git merge --ff-only`。
- 默认只允许一个活动任务。只有明确传入 `-AllowParallel` 才允许无共享锁冲突的并行任务。
- 任务分支必须经过规格审查和代码质量审查，留下本地审批标记后才能合并。

## 首次准备

在干净的主工作区运行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\bootstrap.ps1
```

如需在集成 worktree 中安装依赖：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\bootstrap.ps1 -InstallDependencies
```

该命令默认安装 Go 后端和 Web 前端依赖。只有准备桌面端任务时，才额外安装 Tauri/Rust 依赖：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\bootstrap.ps1 -InstallDependencies -InstallDesktopDependencies
```

脚本会创建同级目录 `OmniCraft-beta-integration`，对应分支 `codex/beta-integration`。主工作区存在未提交改动时会直接停止。

## 准备任务

先准备任务，不启动 Claude：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\run-task.ps1 -TaskId F-02
```

检查生成的 prompt 后，再显式启动：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\run-task.ps1 -TaskId F-02 -Resume -LaunchClaude
```

需要首次安装任务 worktree 的依赖时添加 `-InstallDependencies`。脚本只会在桌面端任务中自动追加 Tauri/Rust 依赖。低成本模型可通过 `-Model` 显式传入。DeepSeek API Key 只放在当前终端环境变量中，不要写入仓库。

## 合并任务

先只运行 preflight：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\integrate.ps1 -TaskId F-02
```

确认后运行完整验证和 fast-forward 合并：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\integrate.ps1 -TaskId F-02 -ConfirmMerge
```

脚本不会自动 rebase。若集成分支已前进，任务代理必须先 rebase、解决冲突并重新验证。

## 释放预约

任务取消或人工接管时，可只释放锁：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\integrate.ps1 -TaskId F-02 -ReleaseReservationOnly
```

## 并行规则

默认保持串行。确需并行时，使用：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\run-task.ps1 -TaskId F-06 -AllowParallel
```

脚本会依据 `beta-tasks.psd1` 拒绝共享文件锁冲突。并行只适合依赖已集成且写入范围不重叠的任务。

## 脚本测试

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\tests\Test-BetaScripts.ps1
```
