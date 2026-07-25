# OmniCraft Beta 计划集审查报告

> 审查日期：2026-05-30 | 审查范围：`docs/superpowers/plans/` 下 7 个 2026-05-30 开头的计划文件
>
> 历史说明（2026-05-31）：本报告保留审查当日的通用 Captcha 字段提案。维护者已经确定使用阿里云验证码 2.0，后续实现以路线图中的 `captcha.provider`、`captcha.prefix`、`captcha.scene_id`、`captcha.region` 和私有 AccessKey 配置为准。
>
> **状态更新（2026-06-28）**：报告中的 24 项问题大部分已在后续 plan 修订中解决（F-01 至 R-01 全部完成）。详细的逐项状态标注需交叉对比 6 份 subsystem plan 的当前版本，此处不做逐一标注。本报告保留作审查方法论参考。

## 审查概览

对双轨 Beta 计划集（路线图 + 6 个子系统计划）进行了全面审查，共发现 **24 项**歧义或不明确细节。按计划文件分组如下：

| 计划 | 问题数 | 高优先级 | 中优先级 | 低/文档 |
|--------|------------|---------------|-----------------|----------|
| foundation | 7 | 1 | 3 | 3 |
| verification-feedback | 6 | 2 | 2 | 2 |
| admin-operations | 4 | 0 | 2 | 2 |
| agent-entrypoints | 4 | 0 | 2 | 2 |
| desktop-deploy-security | 5 | 2 | 1 | 2 |
| release-validation | 2 | 0 | 1 | 1 |
| roadmap (cross-plan) | 4 | 0 | 0 | 4 |

---

## 1. Foundation (`beta-foundation.md`)

### F-02 — Fail Closed & Interaction Eligibility

#### A. `RuntimeUserStatus` 结构体应放在哪个包？ `[中]`

计划说"使用显式解析器"但未指定 `ResolveRuntimeUserStatus` 应放在 `middleware`、`service` 还是新建包。放在 middleware 会将该包与 GORM/Redis 紧耦合；放在 service 更清晰。

**建议：** 指定为 `backend/internal/service/runtime_status.go`。

#### B. 缓存失效责任边界模糊 `[中]`

规则说"当邮箱验证、信誉分、封禁状态或角色变更时，失效或刷新缓存的运行时状态"——但*谁*负责失效？验证处理器、封禁处理器、角色变更处理器都需要知道要触碰运行时状态缓存。这是一个横切关注点。

**建议：** 明确指定 `RuntimeStatusCache.Invalidate(userID)` 方法，并列出必须调用它的具体处理器：`auth.go`（验证/封禁/角色变更）、`verification_service.go`（邮箱验证）、`social_service.go`（信誉分变更）。

#### C. `cfg.Reputation.MinScoreForInteraction` 缺失时的回退行为 `[低]`

计划说使用 `cfg.Reputation.MinScoreForInteraction` 而非硬编码 3。`social_service.go` 中已存在 `defaultMinScoreForInteraction = 3` 的回退。交互守卫是复用此回退，还是在配置缺失时安全失败（返回 503）？

**建议：** 如果 `MinScoreForInteraction <= 0`，守卫应安全失败（503），绝不使用硬编码回退，因为零表示配置错误。

#### D. "策略组"未显式枚举 `[低]`

计划要求"对每个策略组至少测试一个端点"，但未列出各组。

**建议：** 显式列出：`publish`、`edit_delete`、`comments`、`reactions`、`favorites`、`reports`、`pull_requests`、`judge`、`follows`、`messages`、`downloads`、`agent`、`deploy_grant`（共 13 组）。

---

### F-03 — Cookie Session Migration

#### E. 本地开发中 `__Host-` 前缀需要 HTTPS `[高]`

`__Host-` 前缀要求 `Secure` 标志，意味着本地开发需要 HTTPS。计划说本地开发使用 `refresh_token`（不带前缀），生产环境使用 `__Host-refresh_token`。但如何切换未明确。

**建议：** 基于 `config.yaml > server.mode` 自动选择 cookie 名称前缀：`production` → `__Host-refresh_token`，其他 → `refresh_token`。

#### F. 第 4 步 UI 规范查找目的不明确 `[低]`

步骤说运行 `rg -n "## Page: /register" design/ui-spec.md`，但未说明在注册页面应修改什么。注册页面修改（添加验证待处理跳转）主要属于 V-03 范围，此处的引用令人困惑。

**建议：** 澄清：此步骤仅验证注册页面是否已有 UI 规范，以便了解需要保留哪些现有视觉模式。实际的验证 UI 变更在 V-03 中。

#### G. "反向代理 `/api` 到 Web 源站下"的部署说明 `[低]`

这在开发中（Next.js proxy）是默认行为，但生产部署架构说明模糊。对于分开的 API 主机名，计划说"移除依赖 cookie 的代理路由"——这是对 `proxy.ts` 的重大重构。

**建议：** 添加注释说明当前假设 Web 和 API 由同一源站提供服务。如果是分开的主机名，需要单独的 CSRF 方案（基于 `Origin` 验证的 header token）。

---

### F-04 — Minimal Public Config

_（未发现重大问题——结构体协调已通过跨计划契约妥善处理。）_

---

### F-05 — Chinese Keyword Search

#### H. `total` 计数的 bug 描述不精确 `[低]`

计划说当前实现"在应用文本查询之前"就计数了。这是一个具体的 bug，但未说明修复后应表现为什么。

**建议：** 明确指定：`total` 应统计匹配全文/三元组查询的行数，应用相同的可见性过滤条件。计数和行查询必须使用相同的 WHERE 子句。

---

### F-06 — Unified Download

#### I. `oss.download_url_ttl_sec` 在代码库中不存在 `[中]`

已通过 grep 确认：当前代码中没有 `download_url_ttl` 或 `DownloadURLTTL`。F-06 需要在 `config.yaml` 和 `config.go` 中添加此字段。

**建议：** 在 `oss` 节中添加 `download_url_ttl_sec: 300`，替换当前硬编码的 TTL（似乎是 3600 秒）。

#### J. `is_primary` 逻辑含混 `[低]`

确认 `content_attachments.is_primary BOOLEAN DEFAULT TRUE` 存在。F-06 的"明确的主附件"逻辑可以使用此字段，但计划未提及。当多个附件中没有一个 `is_primary=true` 时发生什么？或当多个都有 `is_primary=true` 时？

**建议：** 明确定义：当 `attachment_id` 被省略时，选择唯一的 `is_primary=true` 附件。如果没有或存在多个主附件，返回验证错误而非选择任意一个。

---

## 2. Verification & Feedback (`beta-verification-feedback.md`)

### V-01 — Verification Schema & Adapters

#### K. Redis key 双向查找关系未明确说明 `[低]`

计划显示了 key 模式但未说明双向查找：
- `verify:email:<sha256(token)>` → 存储 user_id
- `verify:email:user:<user_id>` → 存储活跃 token 摘要

**建议：** 添加明确说明：验证时，先查 `verify:email:user:<user_id>` 获取活跃摘要，将传入 token 的 SHA256 与之比对防止时序攻击，然后删除两个 key 以保证单词使用。

#### L. 邮件规范化算法未指定 `[高]`

`captcha:login-failures:<normalized-email>` ——如何规范化？`Lowercase(TrimSpace(email))`？Unicode 规范化（NFC/NFD）？对于中文邮件地址来说这很重要。

**建议：** 指定规范化为 `strings.ToLower(strings.TrimSpace(email))`。对于非 ASCII 本地部分（极少使用），Beta 版本中保持原样。

---

### V-02 — Verification & Reset Flows

#### M. 条款/隐私版本来源未定义 `[高]`

注册输入包含 `accepted_terms_version: "2026-05-30"` 和 `accepted_privacy_version: "2026-05-30"`。有效的版本字符串是什么？来自哪里？后端如何验证？

**建议：** 在 `config.yaml` 中添加 `legal.current_terms_version` 和 `legal.current_privacy_version`。后端验证提交的版本与当前版本是否匹配，不匹配则拒绝。

#### N. 第 6 步阻塞条件未区分环境 `[中]`

"如果缺少真实的 SMTP 或 captcha 凭证"——这是否仅适用于 `mode: production`？测试/预发布环境可能使用 fakes。

**建议：** 明确阻塞仅适用于 `server.mode == "production"`。在开发和预发布环境中，允许使用 SMTP logger fake 和 captcha bypass 继续。

---

### V-05 — Feedback Model & API

#### O. `feedback_replies` 缺少 XOR 约束 `[中]`

表中有 `author_user_id` 和 `author_admin_id`，没有 CHECK 约束强制 XOR。两者都可以为 NULL、都设置或只设置一个。

**建议：** 添加 `CHECK (author_user_id IS NOT NULL OR author_admin_id IS NOT NULL)` 确保至少有一个作者。添加 `CHECK (NOT (author_user_id IS NOT NULL AND author_admin_id IS NOT NULL))` 强制执行 XOR。`author_admin_id IS NOT NULL` 的回复默认将 `is_internal_note` 设为用户可见控制。

#### P. 附件预签名 TTL 未指定 `[低]`

计划说"在 Redis 中发放一个不透明的上传授权"——TTL 是多少？

**建议：** 使用 `feedback.upload_grant_ttl_sec`，默认 `300`（与其他 grant TTL 一致）。

---

### V-06 — Feedback Pages

#### Q. 匿名提交无追踪能力 `[低]`

"匿名提交在 Beta 版本故意不可追踪"意味着没有工单列表。匿名用户如何查看状态？在 Beta 版本中可以接受，但对于通过邮件收到回复的用户来说是一个明显的 UX 缺口。

**建议：** 在路线图的"Beta 后续跟进"节中记录：允许匿名用户通过提交后收到的令牌链接查看工单状态。

---

## 3. Admin Operations (`beta-admin-operations.md`)

### A-01 — Admin Audit Log

#### R. 按操作类型的元数据白名单未定义 `[中]`

计划说"使用按操作类型的白名单过滤元数据"，但对于不同操作类型，哪些字段是允许的从未定义。

**建议：** 在 `admin_audit_service.go` 中添加白名单映射：

```go
var auditMetadataAllowlist = map[string][]string{
    "content_ban":     {"content_id", "reason", "author_id"},
    "user_ban":        {"target_user_id", "reason"},
    "ip_approve":      {"ip_address", "decision"},
    "appeal_resolve":  {"appeal_id", "decision", "reason"},
    "category_create": {"name", "slug", "display_order"},
    // ...
}
```

#### S. 审计失败时的事务回滚与外部副作用 `[低]`

"如果审计插入失败，回滚"——但外部副作用（OSS 文件操作、通知发送）无法回滚。

**建议：** 将外部副作用移至审计成功提交之后的异步执行。如果审计写入是事务的一部分，外部操作必须在 `tx.Commit()` 之后执行。

---

### A-02 — Audit Existing Mutations

#### T. 类别变更的步骤不完整 `[低]`

步骤 1 列出了类别操作（创建/更新/删除/重排），但步骤 2 中的实现指导未提及 `category_repo.go`/`category_service.go` 的变更。这些文件已在文件列表中列出，但步骤中缺少具体的实现说明。

**建议：** 添加子步骤：重构 `category_service.go` 以接受可选的事务参数 `*gorm.DB`，使审计服务能够在同一事务中写入审计行。

---

### A-03 — Admin Dashboard

#### U. "现有 stats API"未列举 `[中]`

计划说"首先使用现有 stats API"——但未指定哪些端点。预期的看板指标及其 API 来源未明确定义。

**建议：** 明确列出预期使用的端点：
- `GET /api/v1/admin/reports/stats` → 待处理举报数
- `GET /api/v1/admin/queue/stats` → 队列深度/DLQ 健康
- 通过参数查询 `GET /api/v1/admin/feedback?status=open` → 待处理反馈数
- 如果缺少 API，在 A-03 范围内创建它们，而非推迟到其他任务。

---

## 4. Agent Entrypoints (`beta-agent-entrypoints.md`)

### G-01 — Feature Gates

#### V. `ContentDetail.tsx` 文件冲突风险 `[低]`

`ContentDetail.tsx` 被多个任务修改（F-06、G-01、D-01、D-05）。如果任务不按顺序执行，会产生合并冲突。

**建议：** 在路线图执行规则中添加注释：按任务 ID 顺序执行。修改 `ContentDetail.tsx` 的每个任务应在其步骤 1 中首先检查当前文件状态。

---

### G-03 — Contextual Chat

#### W. "当前页面消息"作用域未定义 `[中]`

"最多 10 条当前页面消息"——聊天历史记录是跨页面导航持久化，还是仅限单页？如果用户在页面间导航并返回，历史记录是保留还是清空？

**建议：** 明确指定：在 Beta 版本中，Agent 聊天状态是**每个页面挂载周期**的。页面导航会清空历史记录。跨页面持久化属于 P1 并在路线图的"已推迟 P1"中列出。

#### X. 后端消息长度限制的配置字段未指定 `[低]`

"将每条消息限制在后端配置的长度限制内"——`config.yaml` 中的字段名是什么？

**建议：** 添加 `agent.max_user_message_chars`，默认 `4000`。

---

### G-05 — Publish Assistance

#### Y. 撤销快照机制未指定 `[中]`

"保留撤销快照以便用户可以还原"——如何实现？`useRef` 存储 `structuredClone`？还是不可变 state diff？

**建议：** 指定使用 `useRef` 存储在应用建议前的 `structuredClone(formState)`。应用按钮将快照保存到 ref，撤销按钮从 ref 恢复。

---

## 5. Desktop Deploy Security (`beta-desktop-deploy-security.md`)

### D-02 — Deploy Grants

#### Z. "移除"与"使其永久禁用"含糊不清 `[低]`

计划说"移除旧的 `/api/v1/agent/script/:id` 实现或使其永久禁用"。哪个是正确的？

**建议：** 既移除处理器注册，也移除函数，不在 `routes.go` 中保留路由。D-01 已通过测试确保无法调用。D-02 应物理删除死代码。

---

### D-03 — Ed25519 Signing

#### AA. 客户端公钥嵌入方式未指定 `[高]`

D-04 说"嵌入仅含公钥，以 `key_id` 为键"——但在 Rust 中如何嵌入？编译时环境变量？`include_str!`？硬编码常量？对于密钥轮换来说很重要。

**建议：** 指定使用编译时环境变量 `DEPLOY_PUBLIC_KEY_B64`，在 `build.rs` 或 `env!()` 中嵌入。这样可以在不修改源代码的情况下进行密钥轮换。公钥作为 `&'static [u8; 32]` 嵌入。

#### AB. 旧 HMAC config 字段引用不明确 `[低]`

"Replace Admin config status fields that refer to the old HMAC secret"——当前存在哪些字段？计划未说明当前显示的内容。

**建议：** 添加发现步骤：`rg -n "HMAC|hmac_secret" backend/internal/handler/admin.go` 以找到需要替换的当前字段。

---

### D-04 — Tauri Hardening

#### AC. 移除 `fs:*` 权限的影响分析未进行 `[高]`

"Remove `fs:default`, `fs:allow-read`, `fs:allow-write`, `fs:allow-remove`"——当前哪些功能依赖这些权限？移除它们可能破坏非部署功能。

**建议：** 在 D-04 步骤 1 中添加发现步骤：审查 `tauri-client/src-tauri/capabilities/default.json` 中的当前权限，并记录每个 `fs:*` 权限的用途。如果任何权限是已验证脚本执行器之外的功能所需要的，将其记录为任务输出中的风险评估。

---

### D-05 — Desktop UI & E2E

#### AD. "敏感"写入/移动未定义 `[低]`

"敏感写入和移动需要额外确认"——什么算"敏感"？所有 `write_config` 和 `move_file`？`download_file` 呢？

**建议：** 在确认 UI 规范中明确：`write_config` 和 `move_file` 操作始终触发二次确认对话框，显示目标路径。`download_file` 和 `create_dir` 在脚本摘要中获得单次确认即可。

---

## 6. Release Validation (`beta-release-validation.md`)

### R-01 — Web Beta Validation

#### AE. 中文搜索测试数据缺失 `[中]`

"对无空格标题进行中文关键词搜索"——需要种子数据。使用哪些搜索词？测试数据从哪里来？

**建议：** 创建一个最小化的 `testdata/search_seed.sql` 脚本，包含 5-10 个有中文标题和标签的已发布内容项。放在 R-01 步骤 8 中：`psql $env:DB_DSN -f testdata/search_seed.sql`。

#### AF. "HTTPS 证书存在"步骤的上下文混淆 `[低]`

步骤 10 说"HTTPS 证书存在"——用于本地测试还是生产环境？本地测试可能没有 HTTPS。

**建议：** 将此步骤重命名为"Validate production configuration"，并明确这些是仅生产环境检查。本地验证使用 HTTP localhost 是可以的。

---

## 7. Roadmap — Cross-Plan Issues (`dual-track-beta-roadmap.md`)

#### AG. "Track A" 和 "Track B" 未定义 `[文档]`

路线图表格使用了 Track A 和 Track B 标签，但从未解释它们的含义。

**建议：** 在路线图顶部添加定义：
> **Track A** = Web Beta 关键路径（验证、管理、反馈）
> **Track B** = Agent 功能路径（聊天、搜索、发布协助）

#### AH. 配置字段爆炸——无汇总注册表 `[文档]`

至少有 15 个新的 config.yaml 字段分散在 6 个计划中。如果维护者想在合并前进行审查，需要一份汇总清单。

**建议：** 添加汇总表：

| 字段 | 计划 | 节 | 类型 | 默认值 |
|-------|------|---------|------|---------|
| `oss.download_url_ttl_sec` | F-06 | oss | int | 300 |
| `smtp.host` | V-01 | smtp | string | "" |
| `smtp.port` | V-01 | smtp | int | 587 |
| `smtp.user` | V-01 | smtp | string | "" |
| `smtp.password` | V-01 | smtp | string | "" |
| `smtp.from_address` | V-01 | smtp | string | "" |
| `captcha.provider` | V-01 | captcha | string | "" |
| `captcha.site_key` | V-01 | captcha | string | "" |
| `captcha.secret` | V-01 | captcha | string | "" |
| `verification.ttl_sec` | V-01 | verification | int | 3600 |
| `verification.reset_ttl_sec` | V-01 | verification | int | 3600 |
| `verification.resend_cooldown_sec` | V-01 | verification | int | 60 |
| `verification.login_captcha_threshold` | V-01 | verification | int | 3 |
| `verification.password_min_length` | V-01 | verification | int | 8 |
| `deploy.grant_ttl_sec` | D-02 | deploy | int | 300 |
| `deploy.ed25519_private_key_b64` | D-03 | deploy | string | "" |
| `agent.max_user_message_chars` | G-03 | agent | int | 4000 |
| `feedback.upload_grant_ttl_sec` | V-05 | feedback | int | 300 |
| `legal.current_terms_version` | V-02 | legal | string | "" |
| `legal.current_privacy_version` | V-02 | legal | string | "" |
| `client.download_url` | F-04 | client | string | "" |
| `client.latest_version` | F-04 | client | string | "" |

#### AI. 跨 plan 文件冲突风险 `[文档]`

多个文件被跨 plan 修改：

| 文件 | 修改的任务 |
|------|-------------------|
| `ContentDetail.tsx` | F-06、G-01、D-01、D-05 |
| `PublishForm.tsx` | G-01、G-05、D-01 |
| `routes.go` | F-02、F-04、V-02、V-05、A-02、A-04、A-05、D-02 |
| `config.go` / `config.yaml` | F-03、F-04、V-01、D-01、D-02、D-03 |
| `zh.json` / `en.json` | F-03、F-04、F-06、V-03、V-04、V-06、A-03、A-04、A-05、G-01、G-02、G-03、G-04、G-05、D-01、D-05 |

**缓解措施：** 在路线图中执行严格的顺序要求。每个任务在单个会话中运行。i18n 文件使用按 key 合并而非覆盖。Go 文件受益于显式结构体定义以避免字段漂移。

#### AJ. 迁移编号缺口 `[文档]`

存在编号错位（033-034 在 046 之后、041-042 在 046 之后）。计划使用 049-052，这些编号是空闲的。验证应考虑到错位的文件。

**建议：** 在 R-01 步骤 3 中添加关于错位迁移文件的说明注释。

---

## 按优先级汇总

### 需要澄清（实现前）

| # | 项目 | 计划 | 问题 |
|---|-------|------|-------|
| 1 | E | F-03 | `__Host-` cookie 与本地开发 HTTPS |
| 2 | L | V-01 | 邮件规范化算法 |
| 3 | M | V-02 | 条款/隐私版本来源（config 字段） |
| 4 | AA | D-03 | 客户端公钥嵌入机制 |
| 5 | AC | D-04 | 移除 `fs:*` 权限的影响分析 |

### 应在实现期间解决

| # | 项目 | 计划 | 问题 |
|---|-------|------|-------|
| 6 | A | F-02 | `RuntimeUserStatus` 包的位置 |
| 7 | O | V-05 | `feedback_replies` XOR 约束 |
| 8 | R | A-01 | 按操作类型的元数据白名单 |
| 9 | Y | G-05 | 撤销快照实现 |
| 10 | AE | R-01 | 中文搜索种子数据 |
| 11 | I | F-06 | `download_url_ttl_sec` config 字段 |
| 12 | K | V-01 | Redis key 双向查找文档 |
| 13 | N | V-02 | 按环境的阻塞条件 |
| 14 | U | A-03 | 现有 stats API 列表 |
| 15 | W | G-03 | 聊天历史记录作用域 |
| 16 | P | V-05 | 附件预签名 TTL |

### 文档改进

| # | 项目 | 计划 | 问题 |
|---|-------|------|-------|
| 17 | AG | roadmap | Track A/B 定义 |
| 18 | AH | roadmap | Config 字段注册表 |
| 19 | AI | roadmap | 跨 plan 文件冲突矩阵 |
| 20 | AJ | roadmap | 迁移编号缺口注释 |
| 21 | D | F-02 | 策略组枚举 |
| 22 | G | F-03 | 部署架构注释 |
| 23 | Q | V-06 | 匿名追踪 Beta 后续跟进 |
| 24 | AD | D-05 | "敏感"操作定义 |

---

## 总体评估

这七个计划在总体上结构良好，依赖关系经过仔细思考。跨计划契约（路线图第 119-131 行）解决了我的审查中最关键的集成模糊性。以下方面表现突出：

- **安全设计**：fail-closed 认证、Ed25519 替代 HMAC、原子化 grant 消费以及验证后 strict deserialization 体现了良好的威胁建模。
- **向后兼容性**：cookie session 迁移的分阶段方法（localStorage → 内存 → HttpOnly）和下载 API 的 `attachment_id` 可选特性降低了风险。
- **可观测性**：追加审计、结构化日志记录以及跨所有计划的诊断白名单为 Beta 运营提供了良好的基础。

主要风险不在于计划设计，而在于执行：16 个任务全部修改 i18n 文件和路由注册，这造成了合并冲突的可能性。严格的顺序执行（如路线图所要求的）是主要的缓解措施。
