@AGENTS.md

CI 门：GitHub Actions workflow 契约由 `scripts/ci/verify-workflows.sh` 检查（稳定 job 名、SHA 固定 action、最小权限、锁文件缓存、证据保留策略）；`project-gate` 为分支保护必需检查，`tauri-windows` 为已记录的稳定检查。
