# OmniCraft Web Beta 分批审查计划

> 日期：2026-06-01
>
> 目标：把已完成的非阻塞 Web Beta 工作拆成可在单次上下文内完成的代码审查和功能测试批次。每个新对话只审一个风险域，只写审查报告，不修复实现代码。

## 1. 当前基线

- Beta 模式来源：`docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md`
- 当前检查到的提交：`6523938 Beta R-01: validate public web beta release - completed`
- 已完成 Web Beta 范围：
  - `F-01` 至 `F-06`
  - `V-01` 至 `V-06`
  - `A-01` 至 `A-05`
  - `G-01` 至 `G-05`
  - `D-01`
  - `R-01`
- 明确延期且不纳入本轮完成性审查：
  - `D-02` 至 `D-05`
  - `R-02`
- 桌面部署必须继续保持：`features.desktop_deploy_enabled=false`
- 桌面客户端下载必须继续保持：`client.download_enabled=false`

### 已发现的证据风险

`R-01` 已在 roadmap 中勾选并提交，但当前仓库中没有预期的
`docs/review/beta-release-validation-2026-05-30.md`。同时，
`docs/superpowers/plans/2026-05-30-omnicraft-beta-release-validation.md`
中的 `R-01 Step 12: Commit` 仍未勾选。批次 `00` 必须先核验这组状态是否只是文档遗漏，
还是发布验证证据未完整保存。

## 2. 拆分方案

采用“风险域代码审查 + 独立端到端复核”：

| 批次 | 范围 | Roadmap 任务 | 输出报告 |
|---|---|---|---|
| `00` | 发布证据、迁移、工程门槛 | `F-01`, `R-01` | `docs/review/web-beta-review-00-release-evidence.md` |
| `01` | 实时身份状态与交互权限 | `F-02` | `docs/review/web-beta-review-01-authz-runtime.md` |
| `02` | Cookie 会话、CSRF、CORS、公共配置、桌面关闭态 | `F-03`, `F-04`, `D-01` | `docs/review/web-beta-review-02-session-config-desktop-off.md` |
| `03` | 邮箱验证、验证码、密码重置、协议接受 | `V-01`, `V-02`, `V-03` | `docs/review/web-beta-review-03-verification.md` |
| `04` | 帮助、法律、客户端说明、反馈闭环 | `V-04`, `V-05`, `V-06` | `docs/review/web-beta-review-04-feedback-public-pages.md` |
| `05` | 中文搜索、统一下载、Agent 搜索降级 | `F-05`, `F-06`, `G-02` | `docs/review/web-beta-review-05-search-download.md` |
| `06` | Admin 审计日志与敏感写操作 | `A-01`, `A-02` | `docs/review/web-beta-review-06-admin-audit.md` |
| `07` | Admin 运营页面与权限边界 | `A-03`, `A-04`, `A-05` | `docs/review/web-beta-review-07-admin-console.md` |
| `08` | Web Agent 入口、上下文聊天、使用指南、发布辅助 | `G-01`, `G-03`, `G-04`, `G-05` | `docs/review/web-beta-review-08-agent-entrypoints.md` |
| `09` | 独立跨栈端到端复核 | Web Beta 全范围 | `docs/review/web-beta-review-09-cross-stack-e2e.md` |
| `10` | 汇总所有报告并排优先级 | 读取 `00` 至 `09` | `docs/review/web-beta-review-summary.md` |

建议按编号执行。`01` 至 `08` 可以在不同新对话中并行进行；`09` 应在它们都完成后执行；
`10` 最后执行。

## 3. 每个审查对话的通用规则

每个提示词已经包含以下约束：

1. 只审查和测试，不修改实现代码，不更新 roadmap、子计划、`task.json` 或 `progress.txt`，不提交 commit。
2. 允许新增或覆盖该批次指定的 `docs/review/web-beta-review-*.md` 报告，以及指定截图目录中的截图。
3. 开始时运行 `git status --short --branch` 和 `git rev-parse HEAD`。报告中记录 SHA 和脏文件；不得回滚其他人的修改。
4. 审查当前工作区实际状态。若脏文件影响本批次，报告必须标注“未提交改动参与了审查”。
5. 代码审查 findings 优先，按 `P0`、`P1`、`P2`、`P3` 排序，提供文件和行号、触发方式、影响、建议修复方向。
6. 对无法执行的测试写明 `BLOCKED` 和原因。不得把未执行测试写成通过。
7. UI 批次使用 MCP Playwright 或 Browser 插件验证；截图保存到
   `screenshots/review-web-beta/<batch-id>/`。
8. 外部 SMTP、阿里云 CAPTCHA、OSS 或生产 HTTPS 配置缺失时，区分：
   - 本地 fake/bypass/logger 模式可验证的实现行为；
   - 只有真实环境才能验证的发布条件；
   - 当前无法验证的阻塞项。

## 4. 可复制提示词

### Prompt 00：发布证据、迁移和工程门槛

```text
你正在审查 OmniCraft 的 Web Beta 发布证据。当前模式是 AGENTS.md 中的模式 A：双轨 Beta 计划集。

只做代码审查和功能验证，不修复实现代码，不更新 roadmap、子计划、task.json 或 progress.txt，不提交 commit。
唯一允许写入的文件是：
- docs/review/web-beta-review-00-release-evidence.md
- screenshots/review-web-beta/00-release-evidence/*

先读取：
- AGENTS.md
- docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-release-validation.md
- docs/review/beta-baseline-2026-05-30.md
- progress.txt 中 Beta R-01 段落

开始时运行：
- git status --short --branch
- git rev-parse HEAD
- git show --stat --oneline 6523938

重点核验：
1. roadmap 已勾选 R-01，但 docs/review/beta-release-validation-2026-05-30.md 当前缺失，且 release-validation 子计划中的 R-01 Step 12 仍未勾选。判断这是证据遗漏、跟踪不一致，还是验证并未完整落盘。
2. 检查 049-052 Beta migration，以及 R-01 提交中被修改的 041/042 migration。评估历史 migration 被改写是否影响升级一致性。
3. 使用独立 disposable 数据库验证：
   - 空库按文件名字典序执行全部 migration；
   - 先执行 001-048，再执行 049-052 的升级路径。
   不得破坏共享数据库。
4. 运行完整工程门槛：
   - backend: go test ./..., go vet ./..., go build ./...
   - frontend: npm run lint, npm run build
   - docker compose config
5. 运行 secret leak 静态扫描并人工检查匹配项：
   rg -n "err\\.Error\\(\\)|refresh_token|AGENT_HMAC_SECRET|private_key|access_key_secret|api_key|token.*console|console\\.error" backend frontend tauri-client
6. 核验 config 中 desktop_deploy_enabled=false、client.download_enabled=false、payment_enabled=false。
7. 报告当前无法在本地证明的生产配置项：SMTP、阿里云 CAPTCHA、OSS、PostgreSQL、Redis、HTTPS、Allowed Origins、法律版本。

报告格式：
# Web Beta Review 00 - Release Evidence
## Baseline
## Findings
## Migration Verification
## Engineering Gates
## Secret Scan
## Production-Only Blockers
## Evidence Inventory
## Verdict

Findings 按 P0/P1/P2/P3 排序，引用文件和行号。即使没有缺陷，也要明确写出残余风险。
```

### Prompt 01：实时身份状态和交互权限

```text
你正在审查 OmniCraft Beta Task F-02：Fail Closed And Centralize Interaction Eligibility。

只做代码审查和功能验证，不修复实现代码，不更新跟踪文件，不提交 commit。
唯一允许写入：
- docs/review/web-beta-review-01-authz-runtime.md

先读取：
- AGENTS.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-foundation.md 中 F-02 完整段落
- docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md 中 F-02 frozen contracts

开始时运行 git status --short --branch 和 git rev-parse HEAD。

重点代码：
- backend/internal/service/runtime_status.go
- backend/internal/middleware/auth.go
- backend/internal/middleware/interaction.go
- backend/internal/handler/routes.go
- backend/internal/service/reputation_service.go
- backend/internal/handler/admin.go
- backend/internal/handler/user.go
- 对应测试文件

必须检查：
1. AuthRequired 在 Redis 撤销状态不可确认、Redis 与 DB 都无法确认用户状态时是否 fail closed。
2. OptionalAuth 是否安全降级匿名，且不会保留 JWT claim 派生权限。
3. role、ban、soft delete、email verified、reputation 是否使用实时状态。
4. reputation=0 是否被错误当成 cache miss。
5. 状态变更后是否统一失效缓存。
6. InteractionRequired 是否覆盖计划列出的 publish、edit/delete、comments、reactions、favorites、reports、PR、judge、follow、messages、downloads、agent 等路由组。
7. publish freeze 是否只用于需要的能力。
8. 错误信封是否稳定且不暴露 err.Error()。

运行至少：
- cd backend
- go test ./internal/middleware ./internal/handler ./internal/service -v
- go test ./...
- go vet ./...
- go build ./...

报告保存到 docs/review/web-beta-review-01-authz-runtime.md，格式：
# Web Beta Review 01 - Runtime Authorization
## Baseline
## Findings
## Route Coverage Matrix
## Commands Run
## Untested Or Blocked
## Verdict

Findings 优先，按 P0/P1/P2/P3 排序，提供文件和行号。
```

### Prompt 02：Cookie 会话、CSRF、CORS、公共配置和桌面关闭态

```text
你正在审查 OmniCraft Beta Tasks F-03、F-04、D-01。

只做代码审查和功能验证，不修复实现代码，不更新跟踪文件，不提交 commit。
唯一允许写入：
- docs/review/web-beta-review-02-session-config-desktop-off.md
- screenshots/review-web-beta/02-session-config-desktop-off/*

先读取：
- AGENTS.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-foundation.md 中 F-03、F-04
- docs/superpowers/plans/2026-05-30-omnicraft-beta-desktop-deploy-security.md 中 D-01
- roadmap 中 Web/API 域名、public config 和桌面关闭态 frozen contracts

开始时运行 git status --short --branch 和 git rev-parse HEAD。

重点代码：
- backend/internal/handler/auth.go
- backend/internal/service/auth_service.go
- backend/internal/middleware/csrf.go
- backend/internal/middleware/cors.go
- backend/internal/handler/public_config.go
- backend/internal/handler/routes.go
- backend/config/config.go
- backend/config.yaml
- frontend/lib/api.ts
- frontend/lib/auth.ts
- frontend/contexts/AuthContext.tsx
- frontend/lib/useSSE.ts
- frontend/lib/public-config.ts
- frontend/proxy.ts
- frontend/components/agent/AgentFeatureGate.tsx
- backend/internal/handler/agent.go

必须检查和验证：
1. refresh token 只存在 HttpOnly cookie，不进入 JSON、localStorage、URL 或日志。
2. CSRF bootstrap、unsafe request header、refresh、logout 的 cookie/header contract。
3. release 模式 cookie 为 __Host-*、Secure、SameSite=Lax、Path=/、无 Domain。
4. credentialed CORS 仅允许明确 origin，不接受通配符。
5. /api/v1/config/public 严格 allowlist，不泄露 secret、TTL、DSN、OSS 私有配置。
6. 公共配置失败时所有可选能力默认关闭。
7. /api/v1/agent/script/:id 已移除并返回 404。
8. desktop_deploy_enabled=false 时 UI 隐藏，后端不得暴露旧部署入口。

运行 focused Go tests、go test ./...、go vet ./...、go build ./...、npm run lint、npm run build。
使用 Browser/MCP Playwright 验证登录、刷新、退出、公配关闭态和桌面部署入口隐藏；保存截图。

报告保存到 docs/review/web-beta-review-02-session-config-desktop-off.md。
报告必须包含 Findings、Cookie/CSRF/CORS Matrix、Public Config Allowlist、Desktop-Off Proof、Commands、Screenshots、Blocked、Verdict。
```

### Prompt 03：邮箱验证、验证码、密码重置和协议接受

```text
你正在审查 OmniCraft Beta Tasks V-01、V-02、V-03。

只做代码审查和功能验证，不修复实现代码，不更新跟踪文件，不提交 commit。
唯一允许写入：
- docs/review/web-beta-review-03-verification.md
- screenshots/review-web-beta/03-verification/*

先读取：
- AGENTS.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-verification-feedback.md 中 V-01 至 V-03 完整段落
- design/ui-spec.md 中 /register、/forgot-password、/reset-password、/settings 及 CaptchaWidget 相关章节

开始时运行 git status --short --branch 和 git rev-parse HEAD。

重点代码：
- backend/internal/pkg/mail/*
- backend/internal/pkg/captcha/*
- backend/internal/service/verification_service.go
- backend/internal/service/auth_service.go
- backend/internal/handler/auth.go
- backend/internal/handler/routes.go
- backend/internal/handler/public_config.go
- backend/migrations/050_verification_and_terms.sql
- frontend/components/verification/CaptchaWidget.tsx
- frontend/app/(public)/register/page.tsx
- frontend/app/(public)/forgot-password/page.tsx
- frontend/app/(public)/reset-password/page.tsx
- frontend/app/(public)/verify-email/page.tsx
- frontend/app/(public)/verify-email/pending/page.tsx
- frontend/app/(protected)/settings/page.tsx

必须检查和验证：
1. Redis 只保存 token hash；替换 token 会失效旧 token；消费为原子单次使用。
2. verification/reset TTL 不混用；resend cooldown 生效。
3. 邮箱规范化和限流 key 不泄露邮箱 PII。
4. 注册、忘记密码、重发需要 captcha；登录达到阈值后需要 captcha。
5. release 模式拒绝 smtp logger 和 captcha bypass。
6. 注册记录 terms/privacy 当前版本，版本不匹配时拒绝。
7. 注册后不创建可交互会话；reset 成功后的自动登录 contract 正确。
8. 已知和未知邮箱的 forgot-password 响应一致。
9. UI 不把完整邮箱或 token 放进不必要的 URL、浏览器存储或错误展示。
10. 计划中的 V-03 Browser-test checkbox 仍未勾选，必须独立补测并记录。

运行 focused Go tests、go test ./...、go vet ./...、go build ./...、npm run lint、npm run build。
使用本地 fake/logger/bypass 只验证本地流程；将真实 SMTP 和阿里云 CAPTCHA 发布验证列为 production-only。
使用 Browser/MCP Playwright 验证注册、pending、verify、resend、forgot-password、reset-password、settings 状态并保存截图。

报告保存到 docs/review/web-beta-review-03-verification.md。
```

### Prompt 04：公共页面和反馈闭环

```text
你正在审查 OmniCraft Beta Tasks V-04、V-05、V-06。

只做代码审查和功能验证，不修复实现代码，不更新跟踪文件，不提交 commit。
唯一允许写入：
- docs/review/web-beta-review-04-feedback-public-pages.md
- screenshots/review-web-beta/04-feedback-public-pages/*

先读取：
- AGENTS.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-verification-feedback.md 中 V-04 至 V-06
- design/ui-spec.md 中 Footer、FeedbackForm、/feedback 相关章节

开始时运行 git status --short --branch 和 git rev-parse HEAD。

重点代码：
- backend/migrations/051_feedback.sql
- backend/internal/model/feedback.go
- backend/internal/repository/feedback_repo.go
- backend/internal/service/feedback_service.go
- backend/internal/handler/feedback.go
- backend/internal/service/notification_service.go
- backend/internal/handler/routes.go
- frontend/components/feedback/FeedbackForm.tsx
- frontend/app/(public)/feedback/page.tsx
- frontend/app/(protected)/feedback/mine/page.tsx
- frontend/app/(protected)/feedback/[feedbackId]/page.tsx
- frontend/app/(public)/help/page.tsx
- frontend/app/(public)/privacy/page.tsx
- frontend/app/(public)/terms/page.tsx
- frontend/app/(public)/client/page.tsx
- frontend/components/layout/Footer.tsx

必须检查和验证：
1. anonymous feedback 必须 captcha + contact_email；登录用户只能读取自己的 ticket。
2. /feedback/me 在 /feedback/:id 之前注册，避免路由误匹配。
3. diagnostic summary 严格 allowlist，不持久化 token、cookie、本地路径、header、stack trace。
4. screenshot 使用 feedback 专用 presign、Redis grant、feedback-staging 前缀；不能复用内容上传 key。
5. MIME、大小、key 归属和 grant 单次消费校验正确。
6. internal note 永不返回给用户。
7. Admin reply/close 的站内通知和匿名邮件行为可观察，失败不被静默吞掉。
8. Footer 可到达 /help、/privacy、/terms、/feedback、/client。
9. /client 在 client.download_enabled=false 时只显示不可用/说明态，不虚构下载信息。
10. 法律版本、有效日期和内容是否来自已批准输入；无法确认时写 BLOCKED。

运行 focused Go tests、go test ./...、go vet ./...、go build ./...、npm run lint、npm run build。
使用 Browser/MCP Playwright 验证 Footer、公共页面、anonymous feedback、登录用户反馈列表和详情，保存截图。

报告保存到 docs/review/web-beta-review-04-feedback-public-pages.md。
```

### Prompt 05：中文搜索、统一下载和 Agent 搜索降级

```text
你正在审查 OmniCraft Beta Tasks F-05、F-06、G-02。

只做代码审查和功能验证，不修复实现代码，不更新跟踪文件，不提交 commit。
唯一允许写入：
- docs/review/web-beta-review-05-search-download.md
- screenshots/review-web-beta/05-search-download/*

先读取：
- AGENTS.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-foundation.md 中 F-05、F-06
- docs/superpowers/plans/2026-05-30-omnicraft-beta-agent-entrypoints.md 中 G-02

开始时运行 git status --short --branch 和 git rev-parse HEAD。

重点代码：
- backend/migrations/049_search_trigram_fallback.sql
- backend/testdata/search_seed.sql
- backend/internal/repository/content_visibility.go
- backend/internal/repository/search_repo.go
- backend/internal/service/search_service.go
- backend/internal/service/agent_service.go
- backend/internal/handler/content.go
- backend/internal/service/oss_service.go
- frontend/app/(public)/search/page.tsx
- frontend/components/agent/SearchAgentInput.tsx
- frontend/components/content/DownloadButton.tsx
- frontend/components/content/ContentDetail.tsx
- frontend/components/content/SheetMusicViewer.tsx

必须检查和验证：
1. 无空格中文标题和标签可通过普通关键词搜索命中。
2. keyword、suggestion、Agent hydration 共用可见性约束：published、未软删、作者正常、IP 未封禁、隐私范围正确。
3. total count 和结果 rows 使用相同过滤条件。
4. 所有 SQL 参数化，不拼接用户输入。
5. Agent 搜索默认不是匿名用户入口；失败时保留筛选并降级到普通搜索。
6. 所有下载 CTA 经过 GET /api/v1/contents/:id/download?attachment_id=...。
7. 下载重新验证用户、邮箱、信誉分、封禁、内容状态、IP、allow_copy 和附件归属。
8. OSS URL TTL 从配置读取；签名成功后再异步计数。
9. 未指定 attachment_id 时仅允许唯一 primary attachment；歧义时返回稳定 400。
10. 前端不再用 oss_url 作为用户下载 CTA。

使用 disposable 数据库加载 backend/testdata/search_seed.sql。
运行 focused Go tests、go test ./...、go vet ./...、go build ./...、npm run lint、npm run build。
使用 Browser/MCP Playwright 验证中文搜索、Agent 失败降级、下载成功和拒绝路径，保存截图。

报告保存到 docs/review/web-beta-review-05-search-download.md。
```

### Prompt 06：Admin 审计日志和敏感写操作

```text
你正在审查 OmniCraft Beta Tasks A-01、A-02。

只做代码审查和功能验证，不修复实现代码，不更新跟踪文件，不提交 commit。
唯一允许写入：
- docs/review/web-beta-review-06-admin-audit.md

先读取：
- AGENTS.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-admin-operations.md 中 A-01、A-02

开始时运行 git status --short --branch 和 git rev-parse HEAD。

重点代码：
- backend/migrations/052_admin_audit_logs.sql
- backend/internal/model/admin_audit.go
- backend/internal/repository/admin_audit_repo.go
- backend/internal/service/admin_audit_service.go
- backend/internal/handler/admin.go
- backend/internal/handler/category.go
- backend/internal/handler/judge.go
- backend/internal/container/container.go
- backend/internal/handler/routes.go
- 对应测试

必须检查和验证：
1. audit log append-only，不暴露 update/delete API。
2. metadata 使用 per-action allowlist，敏感字段 token、cookie、password、api_key、secret、grant、本地路径、header 被剥离。
3. trace_id 被记录。
4. 本地数据库敏感写操作与 success audit row 在同一事务中；audit 写失败时 domain mutation 回滚。
5. 外部副作用在 commit 后执行。
6. 失败尝试仅记录脱敏 reason code。
7. 覆盖 IP approve/reject、content ban/restore、user ban/unban、appeal、report、config、category、LLM config、judge question。
8. admin config 仍为掩码/布尔态，不依赖明文 secret。

运行：
- cd backend
- go test ./internal/service -run TestAdminAudit -v
- go test ./internal/handler -v
- go test ./...
- go vet ./...
- go build ./...

报告保存到 docs/review/web-beta-review-06-admin-audit.md，包含敏感操作覆盖矩阵和事务性判断。
```

### Prompt 07：Admin 运营页面和访问边界

```text
你正在审查 OmniCraft Beta Tasks A-03、A-04、A-05。

只做代码审查和功能验证，不修复实现代码，不更新跟踪文件，不提交 commit。
唯一允许写入：
- docs/review/web-beta-review-07-admin-console.md
- screenshots/review-web-beta/07-admin-console/*

先读取：
- AGENTS.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-admin-operations.md 中 A-03 至 A-05
- design/ui-spec.md 中 /admin、Sidebar、AdminFilterBar、AdminMetricCard 相关章节

开始时运行 git status --short --branch 和 git rev-parse HEAD。

重点代码：
- backend/internal/handler/admin_feedback.go
- backend/internal/handler/admin_audit.go
- backend/internal/handler/routes.go
- backend/internal/repository/feedback_repo.go
- backend/internal/service/feedback_service.go
- frontend/app/(protected)/admin/*
- frontend/components/admin/*

必须检查和验证：
1. admin subtree 全部位于 (protected) 路由组，URL 仍为 /admin/*。
2. 未登录、封禁用户、普通用户、管理员访问边界正确；后端 AdminRequired 是最终权威。
3. /admin 重定向 /admin/dashboard。
4. sidebar 和 mobile navigation 可到达 dashboard、reports、feedback、queue、audit logs 及原有页面。
5. dashboard 不使用伪造统计；缺失可选指标显示 unavailable。
6. reports 支持筛选、详情、显式 resolution action/explanation。
7. feedback 支持筛选、详情、reply、internal note、priority、assignment、close/reopen。
8. queue 页面只读，不暴露 replay 控件。
9. audit logs 页面只显示脱敏 metadata。
10. 检查原 frontend/app/admin/users/page.tsx 删除后是否造成 /admin/users 缺失或导航死链。

运行 focused Go tests、go test ./...、go vet ./...、go build ./...、npm run lint、npm run build。
使用 Browser/MCP Playwright 以匿名、普通用户、管理员验证访问边界和主要运营闭环，保存截图。

报告保存到 docs/review/web-beta-review-07-admin-console.md。
```

### Prompt 08：Web Agent 入口、聊天、使用指南和发布辅助

```text
你正在审查 OmniCraft Beta Tasks G-01、G-03、G-04、G-05。G-02 已由搜索下载批次覆盖。

只做代码审查和功能验证，不修复实现代码，不更新跟踪文件，不提交 commit。
唯一允许写入：
- docs/review/web-beta-review-08-agent-entrypoints.md
- screenshots/review-web-beta/08-agent-entrypoints/*

先读取：
- AGENTS.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-agent-entrypoints.md 中 G-01、G-03、G-04、G-05
- design/ui-spec.md 中 AgentChatWidget、ContentDetail、UsageGuidePanel、UploadAssistPanel、ComplianceCheckBadge、/studio/publish 相关章节

开始时运行 git status --short --branch 和 git rev-parse HEAD。当前工作区可能存在 backend/internal/handler/agent.go 未提交改动；若仍存在，必须在报告中明确区分已提交基线与未提交状态。

重点代码：
- frontend/components/agent/*
- frontend/app/layout.tsx
- frontend/components/content/ContentDetail.tsx
- frontend/components/studio/PublishForm.tsx
- frontend/lib/agent-context.ts
- frontend/lib/useSSE.ts
- backend/internal/handler/agent.go
- backend/internal/service/agent_service.go
- backend/internal/model/agent.go
- backend/config/config.go
- backend/config.yaml

必须检查和验证：
1. web_agent_enabled=false 时聊天、AI 搜索选择和 Agent UI 安全关闭；普通功能仍可使用。
2. 匿名和未验证邮箱用户看不到受保护 Agent 入口；后端仍强制权限。
3. page context 仅包含 route、contentId、contentTitle、contentType，不含 token、cookie、本地路径、HTML。
4. 聊天历史最多 10 条，角色白名单和每条长度上限正确，未知字段/工具 payload 被拒绝。
5. Agent 失败和限流时保留用户输入，提供 retry、/help、/feedback 和正常功能降级。
6. 使用指南只在允许内容类型 mod、sheet_music 和已发布可见内容上出现；Markdown 不启用 raw HTML。
7. 发布辅助建议必须用户显式 apply，可 undo；warning 需确认；violation 阻止 apply 和 submit。
8. 建议被视为不可信输入：前后端都校验字段长度、标签数量、标签长度、category enum、未知字段。
9. Agent 不得执行工具、本地文件动作、静默内容写入或返回永久 OSS URL。

运行 focused Go tests、go test ./...、go vet ./...、go build ./...、npm run lint、npm run build。
使用 Browser/MCP Playwright 验证 flag off、聊天失败态、使用指南、发布辅助 apply/undo、Agent 不可用时普通发布，保存截图。

报告保存到 docs/review/web-beta-review-08-agent-entrypoints.md。
```

### Prompt 09：独立跨栈端到端复核

```text
你正在独立复核 OmniCraft Web Beta，不以 roadmap checkbox 或旧报告作为通过证据。

只做功能测试和少量必要的代码定位，不修复实现代码，不更新跟踪文件，不提交 commit。
唯一允许写入：
- docs/review/web-beta-review-09-cross-stack-e2e.md
- screenshots/review-web-beta/09-cross-stack-e2e/*

先读取：
- AGENTS.md
- docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-release-validation.md 中 R-01
- 已生成的 docs/review/web-beta-review-00-*.md 至 08-*.md

开始时运行 git status --short --branch 和 git rev-parse HEAD。

初始化本地 PostgreSQL、Redis、backend、frontend。不得使用或破坏共享生产数据。
使用 Browser/MCP Playwright 和 API 请求独立验证：

公共旅程：
1. Footer 到 /help、/privacy、/terms、/feedback、/client。
2. 注册、待验证、验证、登录、刷新、退出。
3. 重发冷却和密码重置。
4. 无空格中文关键词搜索及隐藏内容不可见。
5. 下载成功、未登录拒绝、未验证拒绝、低信誉拒绝、禁止复制拒绝。
6. Agent flag off；Agent 搜索失败降级。
7. 发布辅助必须显式 apply，且 Agent 不可用时普通发布可用。

反馈旅程：
8. 匿名反馈 captcha 路径。
9. 登录用户提交、列表、详情。
10. 管理员回复、internal note、关闭；用户详情不显示 internal note。

Admin 旅程：
11. 普通用户访问 /admin 被拒绝。
12. 管理员访问 dashboard、reports、feedback、queue、audit logs、config。
13. report resolve 和 feedback close 产生审计记录。
14. queue 页面无 replay 控件，config 页面无明文 secret。

安全关闭态：
15. /api/v1/agent/script/:id 返回 404。
16. desktop_deploy_enabled=false、client.download_enabled=false。
17. 页面不展示一键桌面部署或客户端下载入口。

报告中为每条用例记录 PASS、FAIL 或 BLOCKED，并附截图路径或 API 响应摘要。
本地无法证明的真实 SMTP、阿里云 CAPTCHA、OSS、HTTPS、Allowed Origins 和法律批准状态必须单列。

报告保存到 docs/review/web-beta-review-09-cross-stack-e2e.md。
```

### Prompt 10：审查结果汇总

```text
你正在汇总 OmniCraft Web Beta 的分批审查结果。只读取现有报告，不修复实现代码，不运行破坏性命令，不更新跟踪文件，不提交 commit。

允许写入：
- docs/review/web-beta-review-summary.md

读取：
- docs/review/web-beta-review-00-release-evidence.md
- docs/review/web-beta-review-01-authz-runtime.md
- docs/review/web-beta-review-02-session-config-desktop-off.md
- docs/review/web-beta-review-03-verification.md
- docs/review/web-beta-review-04-feedback-public-pages.md
- docs/review/web-beta-review-05-search-download.md
- docs/review/web-beta-review-06-admin-audit.md
- docs/review/web-beta-review-07-admin-console.md
- docs/review/web-beta-review-08-agent-entrypoints.md
- docs/review/web-beta-review-09-cross-stack-e2e.md

生成 docs/review/web-beta-review-summary.md，格式：
# OmniCraft Web Beta Review Summary
## Release Recommendation
只能使用：GO、NO-GO、GO-WITH-BLOCKERS。
## P0 Findings
## P1 Findings
## P2 Findings
## P3 Findings
## Production-Only Blockers
## Failed Or Blocked Test Matrix
## Duplicate Findings Consolidated
## Suggested Repair Order
## Reports Read

要求：
1. 合并重复发现，但保留所有来源报告引用。
2. 区分代码缺陷、测试证据缺口、真实外部配置阻塞、延期桌面范围。
3. D-02 至 D-05 和 R-02 不应误报为 Web Beta 缺陷，只要 desktop_deploy_enabled=false。
4. 对每个修复项给出建议优先级、受影响文件和建议回归批次。
5. 不要根据 roadmap checkbox 推断通过，只使用报告中的实际证据。
```

## 5. 使用说明

1. 新建对话后复制对应批次提示词。
2. 每个对话完成后检查对应 `docs/review/web-beta-review-*.md` 是否已写入。
3. 可以并行执行 `01` 至 `08`，但同一工作区同时运行多个会写数据库的测试时，应使用独立 disposable 数据库和独立端口。
4. 完成 `00` 至 `08` 后执行 `09`。
5. 完成 `09` 后执行 `10`，得到统一修复输入。

## 6. 修复阶段边界

本轮只收集证据，不直接修复。后续修复对话应：

- 以 `docs/review/web-beta-review-summary.md` 为入口；
- 每次只处理一个或一组强相关 findings；
- 继续使用 Beta 模式，不修改 `task.json`；
- 行为修复遵循 TDD；
- 修复后重跑来源批次和 `09` 中受影响的跨栈用例。
