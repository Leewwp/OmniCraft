# OmniCraft Production Readiness Design

> **2026-07-25 execution scope:** The current release target is Web-only. Ops-00 through Ops-08 define the active Web production-ready path; Ops-09 and the Web + Desktop extension remain preserved but deferred until Desktop scope is explicitly restored.

**状态：** 已批准执行（2026-07-18）  
**日期：** 2026-07-17  
**适用范围：** Web/API、PostgreSQL、Redis、容器部署、Tauri 桌面制品  
**执行计划：** `docs/superpowers/plans/2026-07-17-omnicraft-production-readiness.md`

---

## 1. 文档地位与边界

本文是 OmniCraft 生产就绪能力的权威设计输入。它定义发布门、证据、恢复语义和 Ops-00～Ops-09 的边界，不实现任何代码。发生冲突时遵循 `specs > plans > runbooks`：执行计划从属于本文；`docs/deploy/single-server-beta-runbook.md` 仍只是 Beta 操作输入，在 Ops-08 完成生产权威修订前不得被当作生产 runbook。

本文明确**不是 Hardening Task 6**。`2026-07-08-omnicraft-project-excellence-hardening.md` 在 Task 5 结束；不得把 CI、迁移、可观测性或发布工程重新塞入已放弃的 Task 6。本文也不替代：

- 模式 A 的 Desktop D-02～D-05 与 R-02 安全任务；
- Web Agent 产品化中的 Provider、预算和引用质量门；
- 社区功能计划中的业务迁移和用户体验验收；
- `task.json` 历史账本。

Ops 工作使用 `AGENTS.md` 的 heavy 车道，禁止修改历史任务账本、Beta roadmap、社区计划完成状态或 Web Agent 完成状态。

## 2. 当前基线

已具备：

- `scripts/verify-project.sh` 的 default/full/release/Tauri 分层与 fail-fast 合同（2026-08-03 由 PowerShell 移植为 bash，历史 `.ps1` 已删除）；
- Go、Next.js、doc-validator、Playwright 和 Tauri 的本地验证入口；
- release 配置 fail-fast 校验；
- 单机 Docker Compose、nginx 模板、数据库备份脚本和 Beta runbook；
- HTTP composition root 与具名 interaction policy；
- 仓库默认关闭 Web Agent、Desktop Deploy 和支付能力。

未具备：

- 正式 GitHub Actions workflow 与 required checks；
- 可升级已有数据库的 migration ledger；
- 被演练证明可恢复的备份；
- 统一日志字段、低基数指标、告警和 runbook；
- 持续安全扫描、SBOM、provenance；
- 可重复的负载/压力基线；
- 不可变制品部署与回滚演练；
- Tauri updater 签名和 Windows 代码签名发布链。

`/docker-entrypoint-initdb.d` 只初始化空数据卷，不是生产迁移器。现有 `scripts/init-db.sh` 会尝试顺序执行全部 SQL，也不构成已应用状态、checksum 或并发安全证明。

## 3. 目标与非目标

### 3.1 目标

1. 每个发布候选都能关联 commit、测试、迁移集合、镜像 digest、SBOM 和签名证据。
2. PR 在不接触生产密钥的情况下运行确定性验证。
3. 数据库迁移可检测漂移、并发执行和缺失的低编号文件。
4. 备份必须通过恢复演练证明，而不是只证明文件生成。
5. 日志、指标和告警可回答“发生了什么、影响多大、现在该做什么”。
6. Web 和 Desktop 发布均使用不可变制品并可执行有边界的回滚。

### 3.2 非目标

- 本计划不采购云服务、不生成生产密钥、不直接部署生产环境。
- 不把真实 LLM、OSS、Green、CAPTCHA 或 SMTP 凭证注入普通 PR。
- 不承诺 Kubernetes、多地域高可用或零停机数据库破坏性变更。
- 不伪造 down migration；共享环境优先兼容回滚、forward-fix 和备份恢复。
- 不因为当前没有生产数据库而降低迁移、备份或恢复门。

## 4. 独立模式 E 与分支规则

- 权威设计输入：本文。对应执行计划必须服从本文并提供可执行步骤。
- Ops-00 使用 `codex/ops/ops-00`；后续使用 `codex/ops/ops-01`～`codex/ops/ops-09`。
- Ops-00 通过后建立 `codex/ops-integration` 作为唯一 Ops 集成分支。
- 一个 Ops Task 对应一个 Agent 会话、独立 worktree、一个 reviewed commit。
- 任务分支从最新 `codex/ops-integration` 创建；合并前 rebase、重新验证并 `git merge --ff-only`。
- 每个 Task 先规格符合性审查，再代码质量审查；未解决的正确性、安全、证据或恢复疑问阻止合并。
- 外部凭证缺失不得降低 deterministic fake/ephemeral 测试要求；但只要当前 Task 的验收条件包含真实环境证据，该 Task 就保持未完成且不得提交“完成”commit。确定性部分可以先开发和记录到 `progress.txt`，不能用它替代真实证据。

## 5. 环境与发布候选身份

### 5.1 环境

| 环境 | 数据 | 密钥 | 用途 |
|---|---|---|---|
| PR CI | 临时 PostgreSQL/Redis、fake provider | 无生产密钥 | 确定性门、迁移和合约测试 |
| Staging | 合成或脱敏数据 | 独立低权限密钥 | E2E、负载、告警和回滚演练 |
| Production | 真实数据 | secret store/主机文件 | 经批准发布 |

### 5.2 发布候选清单

一次 release 必须固定：

- Git commit SHA 和语义版本；
- backend/frontend/container digest；
- migration 文件名、版本和 checksum 集合；
- 配置 schema/模板版本与 feature flags；
- 测试、扫描、SBOM、provenance artifact 标识；
- Desktop 安装包、更新清单和签名 digest（如发布 Desktop）。

任何构建后重新打包都会产生新候选，不能沿用旧证据。

## 6. 发布门总览

| 门 | 日常 PR | Release | 阻塞原则 |
|---|---|---|---|
| CI | required | required | 任一必需 job 非零即阻塞 |
| 数据库迁移 | ephemeral integration | 空库+历史 fixture+并发+漂移 | checksum/锁/升级失败阻塞 |
| 备份恢复 | 脚本合约 | 真实 PostgreSQL drill | 只备份未恢复视为失败 |
| 日志指标 | schema/unit | staging smoke | 泄密或关键指标缺失阻塞 |
| 告警 | rule lint | fire-and-resolve drill | 无 runbook/无法送达阻塞 |
| 安全扫描 | dependency/secret/SAST | image + full scan | 未豁免高危或密钥命中阻塞 |
| SBOM/provenance | 生成验证 | 与制品 digest 绑定 | 缺失/无法校验阻塞 |
| 负载 | 小型 smoke | load+stress+容量记录 | SLO 阈值失败阻塞 |
| 生产配置 | 模板/静态检查 | preflight | 占位符、默认密钥、HTTP 域名阻塞 |
| 部署回滚 | 脚本合约 | staging drill | 无法回到已知健康版本阻塞 |
| Desktop 签名 | build/audit | updater+Authenticode 验证 | 缺签名或安全前置未完成阻塞 |

## 7. CI 设计

GitHub Actions 是正式 CI。`main` 面向单人开发采用轻量保护：禁止 force-push/删除，变更通过 PR，required checks 必须通过，但不要求额外人工批准。

CI 必须：

- 固定 action commit SHA，不使用浮动 `@main`；
- 使用 Go `1.25.11`、Node 20、npm lockfile、Rust lockfile；
- 拆分 backend、frontend、docs 和按路径触发的 Windows Tauri job；
- 通过项目 verifier 的 scope 运行，不维护第二套命令真相；
- 上传日志和机器可读 summary，即使失败也执行 artifact upload；
- 使用最小 `permissions`，fork/普通 PR 不获取生产 environment secrets；
- 并发取消同一 PR 的旧运行，但不取消已开始的 release；
- required checks 名称稳定，避免改名导致分支保护静默失效。

分支保护的最低稳定集合为 Ops-01 的 `project-gate`；Ops-05 合并后加入稳定 security gate。`tauri-windows` 必须在声明 **Web + Desktop production-ready** 前设为 required；仅声明 Web production-ready 时它仍须对相关路径运行并留证，但可不阻塞纯 Web 变更。required check 的阶段性集合必须导出为证据，不能留给实现者临时决定。

## 8. 增量数据库迁移与恢复

### 8.1 Ledger

`schema_migrations` 至少记录：

- `version`；
- `filename`；
- SHA-256 `checksum`；
- `applied_at`；
- 可选执行耗时和执行器版本，但不得作为身份判断依据。

迁移器扫描全部合法 `NNN_name.sql`，按版本和文件名稳定排序，对照“已应用文件集合”执行缺失项，不能只比较最大版本。因此未来 062 已存在而 059～061 后补时，缺失文件仍会执行。

规则：

- PostgreSQL advisory lock 保证同一数据库同时只有一个迁移器；
- 已应用文件 checksum 变化立即拒绝；
- 同版本重复文件、非法名称、ledger 指向缺失文件立即拒绝；
- 单个迁移默认事务执行；明确不能事务化的迁移必须通过受审元数据声明，不能靠静默重试；
- ledger 记录与迁移在同一事务提交；
- 已进入共享环境的迁移文件不可编辑，只能新增 forward-fix。

非事务迁移还必须声明可机器验证的 precondition、postcondition、幂等/续跑策略和人工 reconciliation 步骤。独立 `schema_migration_attempts`（或等价审计表）记录 started/succeeded/failed、文件/checksum、时间和脱敏错误摘要；任何 failed/unknown attempt 都阻止自动重试和后续迁移，只有完成 reconciliation 并留下批准证据后才能再次执行。

### 8.2 测试基线

当前没有历史生产数据库，因此 Ops-02 使用两个确定性输入：

1. 空 PostgreSQL 16 + pgvector，从 001 应用至最新；
2. 仓库内受 checksum 保护的合成历史 fixture，表示既有迁移子集及少量非敏感种子数据，再升级至最新。fixture 必须由固定 PostgreSQL/pgvector 版本从真实迁移 `001`～基线版本顺序生成，随附生成命令、源 migration checksum manifest、数据库镜像 digest 和 ledger 行；禁止手写一个“看起来像历史库”的 SQL 快照。

fixture 是升级测试输入，不冒充真实生产快照。未来首次生产发布后，必须用脱敏生产快照补充演练，不能删除合成 fixture。

### 8.3 备份与恢复

- 使用 PostgreSQL custom-format dump，保存校验和、数据库版本、开始/完成时间和迁移集合；
- 恢复到新数据库，不覆盖源数据库；
- 恢复后运行 schema ledger、关键表计数、约束和应用 smoke；
- PostgreSQL 至少每日备份一次并在每次生产迁移前额外备份；本机保留最近 7 份，异地主存储保留至少 30 天；
- 异地备份必须在传输中和静态加密，启用对象版本/不可变保留或等价防误删能力，并对上传后对象重新校验 SHA-256；
- 至少每月执行一次新数据库恢复演练；每次生产 schema 变更前必须有最近 30 天内成功演练；
- 首次没有业务 RPO/RTO 承诺时，记录实测值作为基线，不把实测值自动当作目标；Ops-08 前由用户明确批准数值目标，最近一次演练必须同时满足目标 RPO 和 RTO；
- 恢复失败、未记录耗时或未验证应用可连接，都不算演练完成。

数据恢复分类与顺序：

1. PostgreSQL 是账户、内容元数据、权限和业务状态的事实源，必须先恢复；
2. OSS 保存用户上传字节，不可由 PostgreSQL 重建，生产前必须启用版本控制/保留策略，并在数据库恢复后校验引用对象、恢复缺失版本；
3. Redis 的缓存、会话、限流和可重建排行不是事实源，灾难恢复时清空并重建；队列任务必须通过 PostgreSQL 状态/reconciliation 重新生成，不能把 Redis dump 当作唯一恢复手段；
4. PostgreSQL 与 OSS 校验完成后再启动 Redis、worker 和应用写流量。

Ops-02 必须在本地版本化对象存储替身中演练对象删除→历史版本恢复、PostgreSQL attachment/OSS key 全量 reconciliation，以及 Redis 清空后的缓存/队列 reconciliation。真实 Aliyun OSS 的版本恢复、异地备份和跨 PostgreSQL+OSS 的服务级 RPO/RTO 在 Ops-08 staging 演练；缺少真实凭证时必须记录 release blocker，不能用本地替身证明生产恢复。Ops-02 的 `recovery-objectives` 可以处于 `baseline_only` 并记录实测值；只有 Ops-08 才要求用户批准数值目标并机器比较全部存储恢复结果。

## 9. 日志、指标与隐私

日志使用结构化 JSON，并兼容仓库现有日志合同。稳定字段为 `time`、`level`、`msg`、`service`、`environment`、`version`、`trace_id`、`request_id`、`route`、`method`、`status`、`duration_ms`、`client_ip` 和 `error_class`。`trace_id` 与请求 ID 可同值但两个字段语义固定；不得改用 `timestamp`/`latency` 形成第二套名称。`client_ip` 只保存由独立轮换密钥 HMAC-SHA256 后截取的 128-bit 标识（32 个小写十六进制字符），不保存原始 IP；日志同时记录非敏感 `client_ip_key_id`，previous key 只在显式起止时间的轮换窗口内用于受控关联。真实代理链只用于进程内可信来源判断。

禁止记录：

- access/refresh token、cookie、authorization、密码和密钥；
- CAPTCHA ticket、上传签名 URL 查询串；
- 完整私信、反馈正文、Agent 完整对话和原始 Provider 响应；
- 用户邮箱/IP 等直接标识，除非经过明确脱敏且有运维必要。

指标必须低基数。路由使用模板而不是原始 URL；不得把 user ID、content ID、request ID 或错误正文作为 label。最低指标集：请求量/错误率/延迟、panic、DB pool、Redis、队列积压、worker 失败、迁移状态，以及 OSS、Green、CAPTCHA、SMTP、LLM 等外部依赖的成功/失败/延迟（只按依赖名和结果类别聚合）。

`/healthz` 只表示进程存活；readiness 必须验证必要依赖且不得泄露连接信息；metrics 端点只在内部网络暴露。

单机指标参考栈固定为 Prometheus + Alertmanager：Prometheus 仅在内部 Docker 网络抓取 metrics，默认保留 30 天并设置磁盘上限；生产 receiver 由当前仓库 owner 负责，使用环境注入的 SMTP 和可选 operator webhook；本地演练使用无密钥 webhook sink。Prometheus/Alertmanager 不直接暴露公网端口，远程访问必须经受控隧道或单独认证层。

日志参考栈固定为应用 JSON stdout → Docker `json-file` rotation → Grafana Alloy → Loki。Promtail 已进入停止维护阶段，不用于新的生产基线。Alloy 只获得读取容器日志所需的只读挂载/发现权限，不得取得宿主 Docker 控制权限。单文件、文件数量和 Loki 30 天 retention/磁盘上限必须显式配置；warning/error 审计摘要额外进入加密异地归档。Loki 只在内部网络，查询通过受控隧道/认证入口并记录 operator access。Ops-03 必须演练日志 ingestion、按 `trace_id` 查询、rotation、retention 配置和无权限访问拒绝。

## 10. 告警

Prometheus 负责规则评估，Alertmanager 只负责分组、抑制和通知路由。指标来源固定映射为：应用/迁移/备份 custom metrics、postgres/redis exporter、node exporter、cAdvisor、blackbox exporter 和证书探测；不得为没有数据源的告警伪造规则。单机栈之外必须有独立 failure-domain 的外部 HTTPS heartbeat，才能检测整机/整栈失联。

每条告警必须包含：owner、severity、持续窗口、摘要、影响、首个检查步骤和 runbook 链接。告警覆盖：

- API 5xx/延迟/不可用；
- PostgreSQL/Redis 不可用或连接池耗尽；
- 队列积压、worker 连续失败；
- 备份过期、恢复演练过期、迁移失败；
- 证书临期、磁盘不足、容器反复重启；
- Green 审核、OSS、邮件、CAPTCHA、LLM 等外部依赖持续失败。

告警必须演练 firing 和 resolved。仅通过 YAML lint、没有送达证据，不算完成。

## 11. 安全扫描与例外

扫描覆盖：

- Go：`govulncheck`；
- Node：lockfile audit；
- Rust：`cargo audit`；
- secret：gitleaks；
- filesystem/container/IaC：Trivy；
- GitHub Actions 固定 SHA 和最小权限静态检查。

禁止用 `|| true` 隐藏必需扫描失败。误报/暂不可修复项必须进入机器可读 exception ledger，包含漏洞 ID、受影响组件及版本/digest、风险说明、补偿控制、author、独立批准人、批准日期、到期日和可验证 `approval_ref`。`approval_ref` 必须绑定具体 commit SHA 与 GitHub PR review 或 protected-environment approval event；GitHub issue/评论 URL 本身可编辑或删除，不得声称“不可变”。automation/Agent 可作为 author，批准者必须是已验证的仓库 human owner 且 actor 与 author 不同；release gate 通过 GitHub API 校验 commit、actor、当前批准状态和未撤销状态。单人仓库没有第二位合格 human owner 时，High 例外机制不可用，必须修复/降级风险或引入独立批准者。过期例外自动阻塞。secret 命中不可豁免；release 制品中的 Critical 漏洞不可豁免；High 仅允许用户明确批准、有补偿控制且未过期的限时例外。

## 12. SBOM 与 provenance

每个发布候选生成 CycloneDX 或 SPDX SBOM，至少覆盖 Go module、frontend npm、Tauri npm/Rust 和容器 OS packages。SBOM 必须能关联 artifact digest，不能只上传与制品无关的目录扫描结果。

GitHub release/container 使用 provenance attestation。验证步骤必须从已下载制品重新计算 digest，并验证 attestation/签名主体、仓库和 workflow identity。

## 13. 负载、压力与容量

负载测试分层：

- smoke：小流量，验证脚本和关键路径；
- load：目标并发/吞吐下验证 p95/p99、错误率和资源；
- stress：逐级增压，识别拐点和恢复行为；
- soak 在有长期 staging 后再启用，不作为首次门的伪要求。

测试默认只指向本地或 staging，生产目标必须显式 allowlist。数据使用隔离测试账号和可清理前缀。阈值进入版本控制；修改阈值需要说明基线变化，禁止为了通过而放宽。

Ops-07 完成前必须形成经用户批准的 release performance profile，至少固定环境 CPU/内存/数据库规格、数据集规模、测试时长、关键端点比例、目标吞吐/并发、p95/p99 和错误率目标。测得的当前基线只能帮助制定目标，不能自动成为 passing threshold。每份证据记录 profile 版本、环境资源、数据集、commit 和阈值。

## 14. 生产配置、部署与回滚

生产 secret 不进入 Git。仓库只提交 placeholder 模板和可验证 schema。preflight 必须拒绝：

- debug、localhost、HTTP 公网 URL、通配 CORS；
- 默认/短 JWT、空 Redis 密码、默认数据库密码；
- bypass CAPTCHA、logger SMTP、缺失法律版本；
- 未完成安全任务却开启 Desktop/Web Agent/支付 flag；
- `.env` 占位符、镜像浮动 tag、未固定 migration manifest。

preflight 必须从同一 schema 验证 `ValidateRelease()` 要求的全部字段，并额外验证：`security.trusted_proxies` 与实际 nginx/Docker 拓扑一致；Green callback 使用 HTTPS 且 callback IP allowlist 非空；production config volume 存在且只读；前后端公开 URL/DNS 一致。外部 PostgreSQL 必须使用 `sslmode=verify-full` 或等价证书校验；只有未暴露宿主端口的单机私有 Docker 网络可使用明确记录的内部非 TLS 例外。

部署顺序遵循 expand/contract：先部署向后兼容 schema，再部署应用，再单独清理旧结构。回滚优先回到兼容旧 schema 的上一应用 digest；若 schema 已不兼容则 forward-fix。只有灾难恢复场景才从备份恢复，并必须明确数据丢失窗口。

发布必须在 staging 演练：preflight → 备份 → 迁移 → 部署 → readiness/smoke → 回滚上一 digest → 再次 smoke。生产部署不由普通 PR workflow 自动触发。

## 15. Desktop 制品签名

Ops-09 只建立发布签名链，不替代模式 A 的 D-02～D-05/R-02。D-02～D-05/R-02 是 Ops-09 的硬依赖；未完成时不得启动或部分完成 Ops-09。此时：

- `features.desktop_deploy_enabled` 保持 false；
- 不发布可执行本地写操作的 Desktop Agent；
- 允许 CI 编译和测试，但不把产物标记为生产可发布。

正式 Desktop release 同时要求：

- Tauri updater manifest 的 Ed25519 签名；
- Windows 安装包 Authenticode 代码签名和时间戳；
- 私钥只存在于受保护 release environment，日志不输出 key/password；
- 在干净 Windows runner 下载制品并验证 publisher、hash、updater signature 和版本单调性；
- signing key rotation 和撤销步骤有演练证据。

## 16. 证据与保留

每个 Ops Task 必须产生机器可读 summary 和必要原始日志。CI artifact 至少包含 commit、命令、退出码、工具版本、开始/结束时间；secret 必须脱敏。

`artifacts/` 必须在 Ops-00 起加入 `.gitignore`，证据只上传/归档、不进入产品 commit。验收 summary 的 `commit` 必须等于任务 reviewed commit 的最终 SHA，而不是开始任务时的父提交；因此先完成审查和精确 commit，再用最终 `HEAD` 完成/刷新 summary、校验 schema，最后才允许合并。若校验发现需修改受版本控制文件，必须修正或 amend 该唯一任务 commit，然后再次刷新 summary。

最低保留期：PR 证据 30 天、main 证据 90 天、release 与恢复演练证据 1 年。每次 release 的 `release-manifest.json`、SBOM、provenance、验证 summary 和恢复/回滚证据同时保存到 GitHub Release 与 operator 管理的异地归档；不能只依赖短期 Actions artifact。若需缩短，必须形成用户批准的替代保留策略。

## 17. 依赖顺序与并行

Ops 主线按 Ops-00 → Ops-01 → … → Ops-09 串行执行。原因是 compose、runbook、CI、权威计划和证据索引均为共享文件；串行比跨分支手工合并这些发布合同更可审计。UI 基础任务可在下述边界内并行。

UI Polish 协调：

- Ops-00 与 UI 计划登记均修改 `AGENTS.md`/`CLAUDE.md`/`progress.txt`，必须串行；
- UI U-11 必须在 Ops-00 规格落地后复核 capability/config 安全边界；
- UI U-12 与 Ops-01 共享 `frontend/package.json`、`scripts/verify-project.sh` 和测试，顺序固定为 Ops-01 先合并、U-12 后 rebase；
- U-01/U-05 在 UI 计划完成登记后可与 Ops-01～Ops-07 并行，因为写入范围分离；
- Ops-08/09 发布候选验证期间避免并行改动构建、Docker 或 Tauri 制品文件。

## 18. 全局发布阻塞条件

生产就绪声明分两级：Ops-00～Ops-08 可形成 **Web production-ready** 声明，此时 Desktop 必须保持 disabled 且明确排除；只有模式 A D-02～D-05/R-02 和 Ops-09 均完成，才能形成 **Web + Desktop production-ready** 声明。

以下任一条件存在时，不得宣称 production ready：

1. required CI 未启用或可被跳过；
2. migration checksum 漂移、并发锁、空库/历史升级未通过；
3. 没有最近成功恢复演练、RPO/RTO 未批准，或实测结果未满足批准目标；
4. 日志泄密、关键指标缺失、严重告警无法送达；
5. 未豁免高危漏洞、secret 命中或过期例外；
6. SBOM/provenance 与发布 digest 不匹配；
7. release performance profile 未批准，或 load/stress 阈值失败；
8. 生产配置含默认值/占位符或必要外部输入缺失；
9. staging 部署/回滚演练失败；
10. Desktop 被纳入发布时签名无效，或 D-02～D-05/R-02 未完成却开启桌面部署。

## 19. 已确认决策

- CI 平台使用当前 GitHub 仓库和 GitHub Actions。
- `main` 面向单人开发仍启用 PR + required checks，不要求额外批准。
- 当前没有需升级的生产数据库；Ops-02 使用合成历史 fixture，未来补充脱敏生产快照。
- Ops-00/01 不要求本机安装 PostgreSQL client；Ops-02 可使用 PostgreSQL 容器内工具。
- 真实 Provider/生产密钥不是普通 CI 输入。
