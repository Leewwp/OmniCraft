# 设计审查笔记 A：账号生命周期 / 内容发布全链路 / 审核处罚 / 举报 / 申诉

- 创建日期：2026-09-01
- 预计失效日期：2026-11-01
- 审查范围：J1 账号生命周期、J2 内容发布全链路（A-F 分支）、J5 举报、J6 申诉（含封禁死链检测）
- 审查方式：代码走查（文件:行号）+ 运行时 curl/psql 复现。只审计不修复。
- 环境：backend :8080（仅 `cmd/server` 在跑，无 `cmd/worker` 进程）、captcha provider=bypass、green 走本地通过语义、`archive_malware_scan_enabled=false`。
- 测试账号：seed-ui-005（id 317，主）、seed-ui-006（id 318，封禁视角，已恢复）、admin seed-ui-001。所有构造状态只涉及自建内容 id 182 与自有记录。

---

## 一、发现明细

### A-01 [A][P0] 被封禁用户无法通过产品内任何路径发起解封申诉；封禁屏死链
- 证据：
  - 登录即被拒：`backend/internal/handler/auth.go:233-236`（ErrUserBanned → 403 USER_BANNED）。运行时：seed-ui-006 psql 置 `is_banned=true` 后登录返回 `{"code":"USER_BANNED"} HTTP 403`。
  - 持有仍有效 access token 也全部被拦：`backend/internal/middleware/auth.go:100-104`（authReq 统一 401 USER_BANNED，无任何豁免端点）。运行时实测 `/auth/me`、`/appeals/me`、`POST /appeals` 均 `401 USER_BANNED`。
  - 前端拿不到 banned 用户对象：`frontend/contexts/AuthContext.tsx:136-151`（fetchMe 任何异常 → clearTokens + setUser(null)）→ `(protected)/layout.tsx:22-28` 将其重定向到 `/login?redirect=...`。
  - 封禁屏不可达：`frontend/app/(protected)/layout.tsx:47-58` 的 `user.is_banned` 分支（含「提交申诉」/appeals 链接）永远无法渲染——登录与 `/auth/me` 都不可能给前端一个 is_banned=true 的 user。即使渲染，链接指向的 `/appeals` 也在 (protected) 内，会被同一封禁屏再次拦截（自引用死链）。
  - 无账号级申诉类型：`backend/internal/handler/appeal.go:31`（`oneof=content comment`），实测 `target_type=account` → 400 VALIDATION_ERROR。
  - 规格不符：`docs/reference/business-rules.md` L260 要求「封禁用户访问受保护路由时展示『账号已被封禁』EmptyState 并提供申诉链接」——设计的封禁屏实际不可达。
- 影响：被封禁用户在产品内零出路。唯一可行出路是公开 `/feedback` 匿名工单（`routes.go:231` optAuth；实测匿名提交 201，需 captcha ticket，表单含 CaptchaWidget）或站外邮件（产品内无任何申诉邮箱披露）。登录页文案 zh.json:97「账号已被封禁，请联系管理员」无链接、无指引。
- 修复建议方向：① `/auth/me` 或专用端点对 banned 用户返回可读状态（而非 401），使封禁屏可达；② 封禁屏链接改为公开的账号申诉入口（feedback 预填或独立 account appeal 类型 + 公开提交端点）；③ 登录页 USER_BANNED 文案附工单入口链接。

### A-02 [A][P0] 作者在 Studio 看不到自己 pending/under_review/banned 的内容——内容从作者视野直接消失
- 证据：
  - `backend/internal/handler/user.go:266-284`（GetMyContents 不传 Status）→ `backend/internal/repository/content_repo.go:154-160`（无 Status 时默认 `status='published'`）。
  - 运行时：用户 317 的 draft 内容 id=150 不出现在 `/users/me/contents`；内容 182 被 auto-hide 转 under_review 后也从该列表消失（total 8→7）。
  - 前端 `/studio/contents` 唯一数据源即该端点（`frontend/app/(protected)/studio/contents/page.tsx:35`），且列表项不渲染 status 字段、i18n 无任何「审核中/待审/已封禁」作者侧文案（messages/zh.json 全文 grep 无）。
- 影响：① 提交后 pending 阶段（真实阿里云环境秒级~分钟级）作者找不到内容；② 被举报 auto-hide / 众裁 / AI ban 后作者完全失明，无法知道内容处于何种状态、为何不可见，与 A-08 叠加构成「内容消失且无解释」。
- 修复建议方向：GetMyContents 对本人放开状态过滤（或加 status 筛选参数），studio 列表加状态徽标与说明文案。

### A-03 [A][P0] 众裁闭案不回写内容状态——under_review 内容永久滞留无出口
- 证据：
  - `backend/internal/service/judge_service.go:143-166`：票数达标只调 `CloseCase`；`backend/internal/repository/judge_repo.go:98-105` CloseCase 仅更新 judge_cases 行。
  - 全库 grep `closed_approve|closed_reject` 无任何消费者将其映射回 content_items（仅 judge_repo 统计与 judge 题库同步 `internal/pkg/scheduler/judge_question_sync.go:57`）。
  - 内容状态写 `published` 的位置仅三处：`review_service.go:366-368`（AI pass）、`admin.go:273`（admin restore）、`appeal_helpers.go:14`（申诉批准）。
  - 举报 auto-hide 路径甚至不创建 judge case：`backend/internal/service/social_service.go:296-307` 只 `UpdateContent(status=under_review)`（对比 AI review 路径 `review_service.go:353` 会 ensureJudgeCase）。
- 影响：AI 判 review → under_review + 众裁立案 → 满票闭案后内容仍是 under_review（列表不可见、作者不可见、无通知）；举报 auto-hide 的 under_review 连众裁路径都没有。两类都只能等 admin 手动 restore/ban。
- 修复建议方向：闭案时同步回写内容状态（closed_approve→published、closed_reject→banned）并通知作者；举报 auto-hide 补 ensureJudgeCase 或明确的 admin 审核流。

### A-04 [A][P1] GET /contents/:id 无可见性校验——banned/under_review/pending 内容对所有人（含未登录）完整可读
- 证据：
  - `backend/internal/handler/content.go:302-317`：GetContent 用 `contentSvc.GetContent` → `FindByID`（`content_repo.go:55-65`，仅 `deleted_at IS NULL` 过滤，另有 Redis 详情缓存）。
  - 对比同文件 related-fanworks 用 `GetVisibleContent`（`content_service.go:682-691` + `repository/content_visibility.go:7-19`：published + 作者/IP 未封禁 + is_public 或本人）。
  - 运行时：banned 内容 182 匿名 `GET /api/v1/contents/182` → 200 全量返回（含 description、作者信息）；前端详情页 SSR 直接渲染（`frontend/app/(public)/content/[contentId]/page.tsx:33-37` 无状态检查）。
  - 附带：该 handler 对隐藏内容也计浏览数（`content.go:319-322`），污染 view_count（亦即举报 auto-hide 比例的分母）。
- 影响：封禁/审核中的内容靠 URL 直达仍然完全公开，审核下架在详情页层面失效；作者 ironically 靠这个洞才能看到自己被封内容的 id（申诉要手填 id）。
- 修复建议方向：详情端点改走可见性 scope（作者本人豁免 pending/under_review/banned 读取，其他人仅 published）。

### A-05 [A][P1] 已发布内容编辑（标题/封面）完全绕过复审；甚至可编辑 banned 内容
- 证据：
  - `backend/internal/handler/content.go:519-554`：PATCH 接受 title/cover_image_url/is_public/allow_copy/agent_enabled（ip/source 拒绝 SOURCE_IMMUTABLE）。
  - `backend/internal/service/content_service.go:898-926`：UpdateContent 无状态检查、不触发审核（published 时仅发 RAG updated 事件）。
  - 运行时：PATCH published 182 改标题 → 200，status 仍 published，`ai_review_records` 计数不变（1 条）；psql 置 banned 后 PATCH → 200 仍可改标题。
  - 注意：`cover_image_url` 经 PATCH 可写入任意 URL（发布时 image/video 封面服务端派生，但编辑通道无校验）。
- 影响：先发合规内容过审，再改成违规标题/封面即绕过审核（真实阿里云环境下复审不存在）；封禁内容仍可被作者改写。
- 修复建议方向：published 内容编辑 title/cover 后重置 pending 或触发增量复审；banned 内容禁 PATCH。

### A-06 [A][P1] 作者编辑/删除内容在 UI 无任何入口（studio 死按钮 + 全前端无 PATCH/DELETE 调用）
- 证据：
  - `frontend/app/(protected)/studio/contents/page.tsx:104-111`：Edit/Trash2 两个 Button 只有 title 提示，无 onClick。
  - 全前端 grep：无任何 `PATCH /api/v1/contents/:id`、`DELETE /api/v1/contents/:id` 调用（components/、app/ 均无）；`ContentDetailOverlayHost.tsx` 无 owner 编辑/删除控件。
  - 后端能力齐备：`routes.go:101-102` + handler 实现。
- 影响：作者对自己内容的全部后续管理（改标题、封面、可见性、allow_copy、agent_enabled、删除）在 UI 层不可达；与 A-02 叠加，studio「管理你发布的所有内容」名不副实。
- 修复建议方向：studio 列表接通 PATCH/DELETE（含确认弹窗），或详情页加 owner 操作菜单。

### A-07 [A][P1] 举报完全黑箱：举报者无结果反馈；评论举报无前端入口
- 证据：
  - 路由表穷尽：`routes.go` 全文无 `reports/me` 类端点（举报相关仅 `POST /contents/:id/report`、`POST /comments/:id/report`、`/admin/reports*`）；前端无「我的举报」页面。
  - `backend/internal/handler/admin.go:736-773` ResolveReport 只更新 report 行（status/action_taken），不通知举报者、不联动内容。
  - 评论举报后端存在（`routes.go:142`）但前端 grep 无 `comments/:id/report` 调用——功能对用户不存在。
  - 重复举报 409 前端处理正确：`frontend/components/social/ReactionBar.tsx:114-118`（409 → 置 reported）。运行时实测 201 → 409 ALREADY_REPORTED。
  - 内容举报入口在 ReactionBar 的 ConfirmModal（自由文本 reason，无结构化选项；`ReactionBar.tsx:172-180`）。
- 影响：方法论「结果反馈/触达闭环」双缺：举报者提交后永远不知道结果；「有效举报 +1 分」的信誉规则（business-rules）用户也无从感知。
- 修复建议方向：加 GET /reports/me + 状态通知；评论组件补举报入口。

### A-08 [A][P1] 内容被封禁的原因对作者完全不可达（ban_reason / ai_review_records 均不透出）
- 证据：
  - `content_items.ban_reason` 列存在（migration `040_p0_fixes.sql:8`）且 admin 写入（`admin.go:248-251`），但 `backend/internal/model/content.go:5-36` ContentItem 无 BanReason 字段 → 任何 API（含 admin 列表）都不返回该列。
  - `ai_review_records.raw_response`（含阿里云判定细节）无任何用户侧端点引用（grep handler/ 无 AIReviewRecord 引用）。
  - banned 无作者通知：`review_service.go:326-342` block 分支仅写 outbox `content.banned`，唯一消费者是 indexer（`container.go:312`）。
  - 与 business-rules.md L68「内容作者可通过申诉页面发起恢复请求」冲突：作者既看不到内容（A-02），也看不到原因，申诉表单还要手填数字 target_id（`frontend/app/(protected)/appeals/page.tsx`，input type=number）。
- 影响：「不通过 + 具体原因」这条信息在产品内为 0 触达；申诉质量必然低下（用户不知道被封理由只能盲写）。
- 修复建议方向：ContentItem 增加 ban_reason 序列化（至少对作者本人）；banned 时给作者发通知（复用 notification.create）。

### A-09 [A][P2] 发布冻结（PUBLISH_FROZEN）无剩余时间告知、无提前解冻路径、rehab 不解冻
- 证据：
  - 冻结写入：`review_service.go:470-522`（窗口 7 天、阈值 2、额外 -1 分、`publish:freeze:<uid>` TTL `cache.publish_freeze_ttl=604800`）。
  - 全库 grep `PublishFreezeKey`：只有 set（review_service:511）与 get（interaction.go:88、content_service.go:201）——无任何删除/解冻代码；admin 也无解冻端点。
  - rehab 完成只加信誉分（`rehab` 文案 zh.json:1341-1343「恢复信誉分」，service 无 freeze 引用）。
  - 前端：grep 全前端无 PUBLISH_FROZEN 处理；`PublishForm.tsx:421-422` catch 吞掉错误码只报通用「发布失败」。
  - 运行时：手工 SET freeze key 后 POST /contents → `403 PUBLISH_FROZEN "publishing is temporarily frozen due to recent violations"`（已复现并清理）。
- 影响：用户被冻结 7 天只知道「发布失败」，不知道原因、时长、也无任何缩短路径。
- 修复建议方向：错误响应带剩余时间；PublishForm 按 code 分文案；考虑 rehab 完成/信誉恢复后解冻。

### A-10 [A][P2] 申诉批准「强制 published」的边界风险 + 不清 ban_reason + 无目标校验
- 证据：
  - `backend/internal/service/appeal_helpers.go:7-17`：approved + content → 无条件 `status=published`；`admin.go:392-407` 直接 Updates。
  - 运行时：appeal#1 approve → 182 变 published 但 `ban_reason` 残留（对比 RestoreContent 会清空，`admin.go:273`）。
  - 无目标存在性/归属校验：`handler/appeal.go:28-57` 只查重复 pending；实测 `target_id=999999`（不存在）→ 201 创建成功（污染 admin 队列，已清理）。
  - 边界推演：批准已软删内容 → 状态 published 但 deleted_at 仍在（仍不可见）；批准作者已被封禁的内容 → published 但被可见性 scope 隐藏；批准正在众裁的 under_review 内容会跳过众裁直接发布。
- 修复建议方向：批准前校验目标存在/未删除/作者未封禁；批准时同步清 ban_reason；拒绝路径保持不动（当前正确）。

### A-11 [A][P2] admin 封禁/恢复/申诉批准/举报 auto-hide 均不失效内容缓存——封禁内容最长 300s 仍从详情可读
- 证据：`admin.go:236-263`（BanContent）、`admin.go:265-282`（RestoreContent）、`admin.go:364-420`（ResolveAppeal）、`social_service.go:305`（auto-hide）都不调用 `invalidateContentCache`（只有作者侧 UpdateContent/DeleteContent 调，`content_service.go:922-923`）。
- 运行时：先 GET 预热缓存 → admin ban → 立即 GET 仍返回旧 `status=published`（已复现；`content_detail_ttl=300`）。
- 修复建议方向：四处状态写点补缓存失效。

### A-12 [A][P2] 登录失败验证码门：无 UI 支撑、实际被凭据限流掩蔽、窗口仅 60 秒
- 证据：
  - 后端门：`handler/auth.go:119-137`（阈值 `login_captcha_threshold=3`）+ `:222-224`（缺 captcha_token → 400 CAPTCHA_REQUIRED）。
  - 登录页无 CaptchaWidget：`frontend/app/(public)/login/page.tsx` 全文无 captcha（register/forgot-password 都有）；CAPTCHA_REQUIRED 在 `frontend/lib/user-facing-error.ts` 无映射 → 显示通用「登录失败」。
  - 计数器 TTL 复用 `resend_cooldown_sec=60`（`handler/auth.go:111-117`）——防爆破窗口极短。
  - 运行时：连续错密先触发 `CREDENTIAL_RATE_LIMIT_EXCEEDED 429`（`credential_per_minute=5`，限流在 middleware 先于 handler），captcha 门被掩蔽难现；65s 后正确密码正常登录（计数器已过期，实测 Redis key val=3 ttl=50s）。
- 修复建议方向：登录页接 CaptchaWidget + CAPTCHA_REQUIRED 文案；计数器 TTL 独立配置（如 15min）；与凭据限流阈值协调。

### A-13 [A][P2] access token 过期无请求级静默刷新——依赖后端从不返回的 TOKEN_EXPIRED code
- 证据：
  - `frontend/lib/api.ts:195`：仅 `res.status===401 && errBody.code==="TOKEN_EXPIRED"` 才刷新重试；后端全库 grep 无 TOKEN_EXPIRED（过期返回 code UNAUTHORIZED，`middleware/auth.go:42-47`）→ 该分支为死代码。
  - 实际兜底：AuthContext 启动/每 5 分钟轮询 fetchMe，客户端 `isTokenExpired` 预判后 refresh（`AuthContext.tsx:124-135,152-159`）。
  - 缺口：两次轮询间（最长 5 分钟）到期的 token 发起的任意 api 调用 → 401 UNAUTHORIZED 直接抛错不重试；fetchMe 自身 401 → clearTokens + setUser(null) → protected 布局踢到 `/login?redirect=<pathname>`（`layout.tsx:22-28`；redirect 只保 pathname 不保 query，丢筛选/预填参数）。
- 修复建议方向：401+UNAUTHORIZED 且有 refresh cookie 时也尝试一次刷新重试；redirect 保留 search。

### A-14 [B][P1] 注销账号实现为 is_banned=true + ban_reason='self_deleted'（非软删 deleted_at）
- 证据：`backend/internal/handler/user.go:347-363`。后果：① 已注销用户若再触发任何鉴权路径得到的是「account has been banned」语义（误导）；② 邮箱改写为 `deleted_<id>@anon.local`、密码随机化 → 忘记密码流程静默死路（「if the email exists...」文案），注销不可逆且无恢复渠道；③ 内容不删除但被可见性 scope 隐藏（`content_visibility.go:11` is_banned OR deleted_at 均排除——行为符合预期）。
- 修复建议方向：改用 deleted_at 软删标记（auth 已支持 ErrUserStatusDeleted 语义，`middleware/auth.go:85-88`），保留 is_banned 给真封禁。

### A-15 [B][P2] 主内容列表不排除被封禁作者/IP 的内容（与 related-fanworks 可见性口径不一致）
- 证据：`content_repo.go:152-161` 普通列表分支仅 `deleted_at IS NULL + status='published'`；集中式可见性 scope（`content_visibility.go:7-19`，含作者/IP 封禁排除）只在 source-linkage 分支与 related-fanworks 生效。运行时：封禁作者 9202（eval_banned）的 published 内容 1032「封禁作者的教程」出现在公共列表（清缓存后仍复现）。
- 修复建议方向：统一口径——主列表要么应用同一 scope，要么明确「封号不连坐内容」并同步 scope。

### A-16 [B][P2] 分支 E（压缩包带毒）作者侧区分度缺失【代码级+推演】
- 证据：`features.archive_malware_scan_enabled=false`（config.yaml:78）本地无该链路；模型有 `scan_status/scan_required`（`model/content.go:57-65`，migration 072）且 GET /contents/:id 的 attachments 会带出 scan_status，但前端 messages/{zh,en}.json 无任何 scan 相关文案、无 UI 消费（grep 无）。被 blocked 的 mod 内容对作者呈现的仍是 A-02/A-08 的「消失且无解释」链路，无法区分「恶意软件拦截」与「内容违规」。
- 修复建议方向：作者侧 attachment scan_status 文案 + banned 原因透出时区分来源（green vs clamav）。

### A-17 [C][P2] 分支 F：IP 被 ban 级联下架作者无知情链路【代码级】
- 证据：`review_service.go:453-468` processIPReviewResult 批量 `Update status='banned'`（不写 ban_reason、无通知、无 outbox 事件）。业务规则 L58 定义了级联语义但无触达要求。作者的同步内容集体消失且无任何提示。
- 修复建议方向：级联下架时批量通知受影响作者。

### A-18 [C][P3] 发布类型清单/顺序前端硬编码，与 config `publish.type_order_*` 漂移；该配置无消费者
- 证据：config.yaml:216-218（original 不含 prompt、顺序 image,article,video,audio,template,sheet_music,other）vs `frontend/app/(protected)/studio/publish/original/page.tsx:8-17`（含 prompt、顺序 image,video,article,...）；fanwork 同理漂移；grep 全库无 type_order 消费者（死配置）。
- 修复建议方向：前端从 /config/public 读取或删掉死配置。

### A-19 [C][P3] 上传大小上限前端硬编码且服务端超限报错不指明原因
- 证据：`PublishForm.tsx:434`（500/50/300/20 硬编码；config.yaml limits 为真源且 admin 可运行时 PATCH `/admin/config`）；服务端 presign 校验失败返回通用 `VALIDATION_ERROR "invalid request parameters"`（`handler/content.go:114-117`）。FileUploader 前置校验文案存在（`FileUploader.tsx:358-369`）但基于同一硬编码值。
- 修复建议方向：maxMB 从 /config/public 读取；服务端超限返回专用 code。

### A-20 [C][P3] 申诉页信息密度不足：不显示 admin_response、目标只有裸 id；admin 申诉列表只有 pending
- 证据：`appeals/page.tsx` renderItem 只渲染 reason/status/created_at（API 实际返回 admin_response，实测 appeals/me 含该字段但页面丢弃）；`admin.go:358-361` ListAppeals 硬编码 `status='pending'`，无已处理历史视图。409 APPEAL_EXISTS 前端映射缺失（user-facing-error.ts 无 case → 通用文案）。
- 修复建议方向：页面渲染 admin_response；admin 列表加状态筛选。

### A-21 [C][P3] 环境观察：本地无 worker 进程，notification.create 积压 8 条——通知触达在本地 demo 不闭环
- 证据：`ps` 仅见 `go run cmd/server/main.go`；Redis stream `omnicraft:notification.create` len=8；appeal_result 通知未落库（notifications 最新为 2026-08-07 seed）。ADR0005 设计上 worker 独立进程，非产品缺陷，但影响本地演示的「触达闭环」与面试叙事（需启动 `cmd/worker`）。

### A-22 [C][P3] 版本历史仅 PR 合并产生；发布/编辑不建版本
- 证据：`CreateInitialVersion/CreateNewVersion`（`version_service.go:32,50`）全库无调用方；唯一创建点是 PR 合并 `pr_service.go:197`。详情页 VersionHistory 组件存在（`ContentDetailOverlayHost.tsx:7`）但普通作者内容永远为空列表。

---

## 二、断言 F1-F5 验证结论

- **F1 证实（含补充）**：状态机 pending → published|under_review|banned 由 `applyContentReviewResult`（`review_service.go:316-386`）驱动；banned 对 AI 通道是终态——pass/review 更新均带 `status <> 'banned'` 守卫（:344-346、:366-368），迟到异步结果不能复活；block 幂等不重复扣分（:327-329）。回 published 仅三处：admin RestoreContent（admin.go:273）、申诉批准（appeal_helpers.go:14）、AI pass。补充：举报 auto-hide（social_service.go:296-307）是 AI 之外的 under_review 入口，且不带 banned 守卫（对 banned 内容 Update 为 under_review 会把终态打回——低风险但与终态语义相悖，因 Report 前置 FindByID 不查 status）。
- **F2 证实**：无 rejected 状态；重提=全新 POST /contents；PATCH 仅 title/cover_image_url/is_public/allow_copy/agent_enabled（+SOURCE_IMMUTABLE 拒绝 ip/source），从不重置 status、不触发复审（runtime：编辑后 ai_review_records 不变、status 不变）。「编辑已发布内容绕过再审核」成立，且 banned 内容也可编辑（A-05）。
- **F3 证实**：窗口默认/实际 7 天、阈值 2、额外 -1 信誉分、Redis `publish:freeze:<uid>` 冻结 7 天（publish_freeze_ttl=604800），期间 POST /contents 双重拦截（middleware:64-76 先 403 PUBLISH_FROZEN；service:200-205 再兜底）——runtime 已复现 403。补充：无任何解冻代码路径（A-09）。
- **F4 证实**：申诉仅 content|comment（binding oneof，runtime account→400）；同 target 同 user 仅一个 pending（409 APPEAL_EXISTS，runtime 复现）；admin 批准 content 申诉强制 published（runtime 复现）。补充：无目标存在性/归属校验（runtime 999999→201）。
- **F5 证实**：封禁用户全部 authReq 401 USER_BANNED（middleware/auth.go:100-104，无豁免；登录本身 403）；(protected)/layout.tsx 有封禁屏+ /appeals 链接但**不可达**（依赖被 401 的 /auth/me 提供 user），且 /appeals 本身在 (protected) 内——死链成立（详见 A-01）。

---

## 三、content×角色 可见/可行动矩阵（实测+代码）

| 状态\视角 | 匿名访客 | 作者本人 | 众裁判员 | Admin |
|---|---|---|---|---|
| pending（刚提交） | 详情 200（洞 A-04）；列表不可见 | 列表不可见（A-02）；详情 200 | 无 case（AI 未判 review 前） | /admin/contents 不含（仅 under_review） |
| published | 全可见 | 可见 | — | 可见/可 ban |
| under_review（AI review） | 详情 200（洞）；列表不可见 | 列表不可见；详情 200 | judge queue 可见（有 case） | /admin/contents 可见，可 ban/restore |
| under_review（举报 auto-hide） | 详情 200（洞）；列表不可见 | 列表不可见 | **无 case，不可见**（A-03） | /admin/contents 可见；report 可 resolve 但不影响内容状态 |
| banned | 详情 200（洞，缓存期内 200 且状态可能仍旧）；列表不可见 | 列表不可见；详情 200；可 PATCH/DELETE（A-05） | — | trash/restore；ban_reason 仅 admin DB 可见 |
| 软删 deleted | 详情 404 | 详情 404；无恢复入口（仅 admin /admin/contents/trash + restore） | — | trash 列表可见可 restore（但不清 deleted_at 的话列表仍显示？restore 仅改 status——deleted_at 仍置，实际仍不可见，A-10 边界） |

作者行动汇总：创建 ✓（pending 起）；编辑 ✗（UI 无入口，API 可）；删除 ✗（UI 无入口，API 可，banned 也可删）；申诉 ✓（需手填 id）；查看被拒原因 ✗；恢复 ✗。

## 四、API-UI 对照缺口（本旅程范围）

| 后端能力 | 前端消费 |
|---|---|
| PATCH /contents/:id | 无调用方（死 API） |
| DELETE /contents/:id | 无调用方（死 API；studio 按钮无 onClick） |
| POST /comments/:id/report | 无调用方（死 API） |
| GET /appeals/me 的 admin_response 字段 | 返回但页面不渲染 |
| content_items.ban_reason / ai_review_records | 无任何端点透出（模型缺字段） |
| /config/public limits（maxMB） | 未消费（前端硬编码） |
| publish.type_order_* | 无消费者（死配置） |
| GET /contents/:id/versions | 详情页消费 ✓（但仅 PR 内容有数据） |
| POST /feedback 匿名 | 公开页可用 ✓（封禁用户唯一出路，但无引导） |

---

## 五、附录：构造记录（psql / redis / curl）

所有构造只涉及 seed-ui-005（317）/seed-ui-006（318）自有数据，测后已恢复/清理。

1. `UPDATE content_items SET status='banned', ban_reason='audit: 测试封禁原因' WHERE id=182 AND author_id=317;`（banned 分支构造；后随申诉批准转 published，再 admin ban，最终 DELETE 软删）
2. `UPDATE content_items SET deleted_at=NULL, status='banned' ... WHERE id=182;`（撤销误删，恢复 banned 态）
3. `UPDATE users SET is_banned=true, ban_reason='audit: 封禁旅程测试' WHERE id=318;` + `DEL user:status:318`（封禁旅程；测毕 `UPDATE users SET is_banned=false, ban_reason='' WHERE id=318;` + `DEL user:status:318` 恢复，已确认 is_banned=f）
4. `DELETE FROM appeals WHERE id=2 AND user_id=317;`（清理 999999 假目标申诉）
5. `DELETE FROM feedback_tickets WHERE id=2 AND contact_email='seed-ui-006@seed.omnicraft.local';`（清理匿名工单测试行）
6. redis：`SET publish:freeze:317 1 EX 3600`（冻结复现，测毕 DEL）；`DEL cache:content:182`、批量 `DEL cache:content:list:*`（缓存实验）；`DEL captcha:login-failures:*`（清理失败计数）
7. curl 创建内容 182（POST /contents article）→ 编辑（PATCH）→ 被举报（006 report）→ auto-hide under_review → psql banned → 申诉（317）→ admin 批准（seed-ui-001）→ published → admin ban → 作者 DELETE 软删。全程见上文各发现证据。
8. 注册 pending 流程：audit-reg-001@example.com（仅 Redis pending，24h 自动过期，无 DB 行）。

未触碰：seed-ui-002/003/004/007~014、其他 seed 内容与 IP、admin 账号角色。006 的 report 记录（1 条，target 182）保留——属于正常业务数据。
