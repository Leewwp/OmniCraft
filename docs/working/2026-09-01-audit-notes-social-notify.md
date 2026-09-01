# 设计审查笔记（子任务 C）：社区互动 / 消息触达 / 反馈工单 / 用户主页

- 创建日期：2026-09-01
- 预计失效日期：2026-11-01
- 审查者：设计审查子任务 C（只审计不修复）
- 旅程范围：J8 社区互动、J11 消息与触达矩阵、J12 用户主页与他人视角、J13 反馈工单
- 测试账号：seed-ui-009（id=321，主）、seed-ui-010（id=322，互动对象）、seed-ui-001（admin）
- 环境注记：本地开发模式，评论/私信文本走 Green gate 为 fail-open；所有「真实阿里云会怎样」已逐条推演标注。通知经 Redis Stream（`omnicraft:notification.create`）由独立 worker 消费，本次环境 worker 未运行（见 C-02），不影响代码层结论，运行时证据以 stream payload 与 DB 直查双重确认。

## 一、发现明细

### C-01 [A] P0 — GET /users/:id 向任何人（含未登录）泄露用户 email
- 证据：
  - `backend/internal/handler/user.go:439-453` `sanitizeUser` 无条件返回 `"email": u.Email`；`GetUser`（user.go:71-96）对任意 id 直接套用。
  - 路由 `backend/internal/router/routes.go:71` `users.GET("/:id", optAuth, ...)`。
  - curl 复现（无任何凭证）：`curl -s http://localhost:8080/api/v1/users/322` → `"email":"seed-ui-010@seed.omnicraft.local"`。
- 影响：撞库、骚扰、批量爬取注册邮箱；与「登录用邮箱」体系叠加风险更高。
- 修复建议方向：`GetUser` 仅在 caller==id 或 admin 时带 email；或 sanitizeUser 拆 public/self 两档。
- 涉及文件：backend/internal/handler/user.go

### C-02 [B/A] P1 — 本地开发环境下通知链路整体断裂：Notify() 全部入队但 worker 未启动，通知永不落库
- 证据：
  - `backend/internal/service/notification_service.go:274-306`：`Notify` 在配置了 broker 时只 `queueProducer.Publish("notification.create", ...)`，无同步落库、无降级。
  - `backend/internal/container/container.go:275-281` 注释明确「API server never starts asynchronous consumers (ADR 0005) and no worker.external=false fallback exists」；消费端仅 `cmd/worker`（container.go:307 订阅表）。
  - AGENTS.md「Step 2: 初始化开发环境」只启动 `cmd/server`，不含 `cmd/worker`。
  - 运行时：`redis-cli xinfo groups omnicraft:notification.create` → `lag 8`（本次审计产生的评论/赞/关注/私信通知全部滞留 stream，DB `notifications` 表 0 新增）；stream payload 逐条核对内容正确。
  - 对照组：admin 广播走 `BroadcastSystemNotification`（notification_service.go:66-181）直写 DB，实测 26 收件人即时可见——同一系统内两条通知通路，一条不依赖 worker 一条依赖。
- 影响：按文档启动的本地环境中，铃铛/通知列表永远只有种子数据与广播；「新评论/新粉丝/新私信/申诉结果/PR 结果」全部静默丢失。面试演示该链路即翻车。真实生产（worker 常驻）语义正确，但发布失败仅 slog，无重试补偿以外的告警面。
- 修复建议方向：① AGENTS.md Step 2 / docker-compose 增加 `cmd/worker`；② 文档标注「通知需 worker」为部署硬依赖；③ 评估 Notify 在 broker 可用但 worker 缺席时的 lag 监控（现有 lag 可查但无人查）。
- 涉及文件：backend/internal/service/notification_service.go、backend/internal/container/container.go、AGENTS.md

### C-03 [A] P1 — 封禁用户的申诉出口三重断裂（UI 不可达 + API 401 + 链接自循环）
- 证据（实测，psql 置 321 is_banned=true 后逐条验证，测后已还原）：
  1. `POST /api/v1/appeals`（authReq）→ `{"code":"USER_BANNED"}`；`GET /auth/me` → 401 USER_BANNED。
  2. 前端 `frontend/contexts/AuthContext.tsx:139-148`：fetchMe 失败即 clearTokens+setUser(null) → `frontend/app/(protected)/layout.tsx:42` `if (user.is_banned)` 分支永远不可达（user 为 null 走 `return null`）——business-rules.md:260 Task 105 承诺的「封禁用户 EmptyState + 申诉链接」成为死代码。
  3. 该 EmptyState 的申诉链接指向 `/appeals`，而 appeals 页面本身在 `(protected)` 组内（`frontend/app/(protected)/appeals/page.tsx`），即使能渲染也会被同一 layout 再次拦截（自循环）。
  4. 附带：`POST /feedback`（optAuth）对封禁用户被 OptionalAuth 静默降级为匿名（middleware/auth.go:159-162）→ 要求 contact_email+验证码；`GET /feedback/me`（authReq）→ 401，被封用户连自己的工单历史都看不到。
- 影响：「用户为何被封/如何求助」的信息链完全断裂；被封用户唯一可行路径是以匿名身份重填邮箱+验证码提交不关联账号的反馈。
- 修复建议方向：appeals 提交对「封禁账号」放行（或提供独立公开申诉页 + 邮箱通道）；/auth/me 对封禁用户返回带 is_banned 标记的 200 而非 401；banned EmptyState 链接指向公开路由。
- 涉及文件：backend/internal/middleware/auth.go、backend/internal/handler/appeal.go、frontend/contexts/AuthContext.tsx、frontend/app/(protected)/layout.tsx

### C-04 [A] P1 — 讨论发帖绕过信誉门与文本审核（前端实际使用的是无守卫路由）
- 证据：
  - 两条发帖路由并存：`routes.go:274` `ips.POST("/:id/discussions", authReq, discHandler.CreateDiscussion)`（无 InteractionRequired、handler 直写 repo，`backend/internal/handler/discussion.go:44-68` 无 moderateText/ensureCanInteract）vs `routes.go:138` `social.POST("/discussions", authReq, socialHandler.PostDiscussion)`（service 层有 ensureCanInteract + moderateText，`backend/internal/service/social_service.go:155-175`）。
  - 前端实际调用无守卫版：`frontend/app/(protected)/ip/[ipId]/discussions/new/page.tsx:34` `api.post(\`/api/v1/ips/${ipId}/discussions\`)`。
  - 实测（清 Redis user:status 缓存后，rep 置 1）：`POST /social/comments`、`POST /discussions/1/comments`、`POST /messages` 均 403 INSUFFICIENT_REPUTATION，但 `POST /ips/217/discussions` → 201（discussion id=3 创建成功）。
  - 真实阿里云推演：release 模式下 Green 只覆盖评论/私信/编辑文本（RunModerationGate），讨论标题与正文从未过审 → 违规内容可经讨论区直发。
- 影响：信誉分<3 用户、以及被审核系统拒绝的文本均可经讨论帖发布；与 business-rules「所有信誉门槛统一 min_score_for_interaction」不符。
- 修复建议方向：ips/:id/discussions 路由改挂 commentsGuard 并复用 SocialService.PostDiscussion（或让 discHandler 走 service）。
- 涉及文件：backend/internal/router/routes.go、backend/internal/handler/discussion.go、frontend/app/(protected)/ip/[ipId]/discussions/new/page.tsx

### C-05 [A] P1 — 讨论回复路径不走审核也不通知楼主（同一动作两套语义）
- 证据：
  - `backend/internal/handler/discussion.go:89-116` `ReplyToDiscussion` 直接 `socialRepo.CreateComment`，无 moderateText；通知也不发（对比 `social_service.go:104-108`：经 /social/comments 且带 discussion_id 的回复会 Notify 楼主）。
  - 前端讨论回复用的是不通知不过审的路径：`frontend/components/social/ReplyList.tsx:40` `api.post(\`/api/v1/discussions/${discussionId}/comments\`)`。
  - 实测：321 回复讨论 1（楼主 322），stream 无新通知条目；评论内容未经任何 gate 直发（本地 fail-open 下两者表象一致，release 下分叉）。
  - 信誉门该路径有（路由挂 commentsGuard），实测 rep=1 → 403 ✓。
- 影响：release 模式下讨论回复成为文本审核旁路；楼主永远收不到回复提醒（通知缺口并入 J11 矩阵）。
- 修复建议方向：ReplyToDiscussion 复用 SocialService.PostComment（审核+通知一并补齐）。
- 涉及文件：backend/internal/handler/discussion.go、backend/internal/service/social_service.go

### C-06 [A] P1 — 内容状态转移零通知：content_status 类型在枚举里定义但全库 0 使用
- 证据：
  - `docs/reference/business-rules.md:140` 枚举声明 `content_status`；`grep -rn "content_status" backend frontend` → 0 处使用（后端与前端均无）。
  - AI 审核封禁：`backend/internal/service/review_service.go:330-345`（status→banned + outbox content.banned + 信誉 -3 "ai_violation"）无 Notify；outbox 事件消费者是 indexer（container.go:305-310），与通知无关。
  - admin 封禁：`backend/internal/handler/admin.go:236-262` BanContent 仅更新 status+ban_reason，无通知。
  - 举报率自动 under_review：`backend/internal/service/social_service.go:296-307`，无通知。
  - 判官闭案：`backend/internal/service/judge_service.go:150-172` 仅 CloseCase 更新案件，不通知作者；且 closed_approve 不自动恢复内容展示（business-rules.md:「不违规 ≥60% → 恢复展示」，实现只有 admin content_restore 手动路径 admin.go:271-281）。
  - admin 恢复展示：同样无通知。
- 影响：作者对内容被 ban/转审/判官结果/恢复全程无感，只能自己发现内容消失；叠加 C-07（studio 列表只见 published）后连自查入口都没有——「状态可知？触达闭环？」双失败。ban_reason 仅在直接访问内容详情时可见（实测 GET /contents/:id 对作者返回 status/ban_reason）。
- 修复建议方向：落一个 `Notify(author, "system", "content_status", ...)` 覆盖 banned/under_review/恢复/判官闭案四态；深链 target=content。
- 涉及文件：backend/internal/service/review_service.go、backend/internal/handler/admin.go、backend/internal/service/judge_service.go、backend/internal/service/social_service.go

### C-07 [A] P2 — /users/me/contents（studio 内容管理数据源）只返回 published，草稿/待审/被 ban 内容不可管理
- 证据：
  - `backend/internal/handler/user.go:266-283` GetMyContents 与 GetUserContents（241-264）共用 `ListContents`，未传 Status；`backend/internal/repository/content_repo.go:155-160` 默认 `status='published'`；handler 不读取 status 查询参数（实测 `?status=draft` 仍返回 5 条 published）。
  - 321 有 draft 178、pending 154/130，实测 `/users/me/contents` total=5 全 published；`/users/321/contents` 他人视角同样只有 published（他人不见非公开内容 ✓，但自己也不见 ✗）。
  - 前端消费点：`frontend/app/(protected)/studio/contents/page.tsx:34`。
- 影响：作者无法在 studio 看到草稿/待审/被 ban 条目进行管理或申诉发起；配合 C-06 使「内容消失」彻底不可知。
- 修复建议方向：GetMyContents 支持 status 集合（draft/pending/under_review/banned/published）并带屏蔽词区分展示；或提供独立 studio 查询端点。
- 涉及文件：backend/internal/handler/user.go、backend/internal/repository/content_repo.go

### C-08 [A] P2 — 评论折叠规则完全未实现 + 被举报评论无处置出口
- 证据：
  - `docs/reference/business-rules.md:165`：「点踩/点赞比 ≥ social.comment_fold_threshold（默认 0.30）→ 自动折叠并触发审核」。
  - `backend/config.yaml:195` `comment_fold_threshold: 0.30`、`config/config.go:299`、admin 配置页可编辑（`frontend/app/(protected)/admin/config/page.tsx:366`）、i18n 文案齐全（messages/zh.json:946）——但后端渲染/审核链路 0 处消费（grep 仅 config 定义）；前端 `CommentSection.tsx` 无折叠 UI。
  - 评论点踩在 UI 上无任何可见效果（CommentSection.tsx:208-214 仅图标无计数无折叠）。
  - 处置出口：admin 无评论管理端点（routes.go 42 条 admin 路由中 0 条评论）；`ResolveReport`（admin.go:736-775）只改 report 状态，与被举报评论无联动；`SocialService.Report` 的自动 under_review 仅对 targetType=="content"（social_service.go:296-307）。
- 影响：低质评论治理闭环缺失：既不自动折叠也不触发审核，admin 也没有删评工具。
- 修复建议方向：按规格实现折阈值折叠（前端展示 + 后端可在 ListComments 返回 fold 标记）；admin 增加 hide-comment 端点并与 ResolveReport 联动。
- 涉及文件：backend/internal/service/social_service.go、backend/internal/repository/social_repo.go、frontend/components/social/CommentSection.tsx

### C-09 [A] P2 — 内容评论列表 author 恒为零值对象，前端渲染空白用户名
- 证据：
  - `backend/internal/repository/social_repo.go:38-51` `ListComments` 无 `Preload("Author")`（对比 53-69 行 ListCommentsByTarget 讨论路径有 Preload）；POST /social/comments 响应同样返回未填充 author。
  - curl 实测：`GET /api/v1/social/comments?content_item_id=179` 每条 `"author":{"id":0,"email":"","username":"",...}`。
  - 前端 `frontend/components/social/CommentSection.tsx:186` `comment.author?.username ?? t('common.userLabel'...)`——`""` 非 nullish，渲染为空字符串（不是回退到「用户 {id}」）。
- 影响：内容详情页评论区所有发言者名字为空白（seed 场景可直接复现）。
- 修复建议方向：ListComments 补 Preload("Author")（与讨论路径对齐）；前端用 `||` 兜底。
- 涉及文件：backend/internal/repository/social_repo.go、frontend/components/social/CommentSection.tsx

### C-10 [C] P2 — 评论的「回复」前后端断裂：API 支持嵌套，UI 无入口且永远渲染不出子回复
- 证据：
  - `GET /social/comments` 默认 `parent_id IS NULL`（social_repo.go:42-46）；CommentSection.tsx:148-158 的 replies 取自同一数组 `comments.filter(c => c.parent_id === comment.id)`——该数组只含顶层评论，恒为空。
  - CommentSection 无「回复」按钮（无 api.post 带 parent_id 调用）；API `POST /social/comments` 的 parent_id 字段存在（social_service.go:70-75）。
- 影响：对评论的回复即使通过 API 创建也无法在内容页看到；嵌套深度设计（讨论贴吧式）在内容评论区退化。
- 修复建议方向：要么补回复 UI+子查询（parent_id 参数），要么明确内容评论只做一级并移除歧义。
- 涉及文件：frontend/components/social/CommentSection.tsx、backend/internal/repository/social_repo.go

### C-11 [B/C] P2 — 评论编辑/删除 API 无任何 UI 入口（后端含 A4 编辑审核语义，前端零调用）
- 证据：
  - 后端：`routes.go:136` `PATCH /social/comments/:id`（authReq+guard+CommentEditRateLimit）；`social_service.go:126-142` EditComment 重过 moderateText（action=edit_comment，#109 语义）✓；DeleteComment 软删 status=hidden（social_repo.go:71-73）仅作者（实测他人 403/ERROR）。
  - 前端：grep 全 frontend 无 `PATCH/DELETE /social/comments` 调用（仅测试文件 mock）。
- 影响：编辑/删除是「不可达出口」；A4 编辑审核投入无用户可见回报。
- 修复建议方向：CommentItem 加编辑/删除菜单（本人可见），删除走确认。
- 涉及文件：frontend/components/social/CommentSection.tsx

### C-12 [C] P2 — 私信双通道未读互不同步 + 私信全文进通知 body
- 证据：
  - 新私信产生两条信号：`message.go:124` Notify(recipient, "system", "message", "新私信", body.Text, ...)（body 为私信全文）+ 会话 unread_count。
  - 两套已读独立：`UpdateLastRead`（message_repo.go:343-350）只清会话未读；通知 is_read 只在 /messages 通知 Tab 或单条 PATCH 清。
  - 实测：322 会话 unread=1 而 bell total=0（对通知 read-all 之后）——同一封私信两处未读状态矛盾。
  - 类型 `message` 不在 business-rules.md:140 枚举（comment/like/follow/system/mention/appeal_result/content_status）内。
- 影响：用户在聊天窗读完私信，铃铛仍亮（或反之）；通知列表正文泄露私信全文（无摘要策略）。
- 修复建议方向：读会话时联动标记对应 message 通知已读（或私信不进通知频道只留会话未读+铃铛聚合）；body 改为「你有 1 条新私信」摘要。
- 涉及文件：backend/internal/handler/message.go、backend/internal/repository/message_repo.go

### C-13 [C] P2 — 私信无「发起对话」入口；SSE 实时未实现（规格明示 MVP 用 SSE）
- 证据：
  - 前端唯一发私信调用在 `ChatWindow.tsx:80`，recipient 取自已存在会话的 participants；全前端无收件人选择/用户页「私信」按钮（grep recipient_id 仅 ChatWindow）。
  - 新会话只能由协作邀请被动创建（collab_invite_service.go:278-286 SendTypedTx）。
  - `business-rules.md:143`「Websocket 或 SSE 实现实时消息（MVP 使用 SSE）」；ChatWindow 无轮询无 SSE（loadMessages 仅挂载时+手动刷新按钮）；frontend SSE 实现仅 Agent 用（lib/useSSE.ts 消费者为 UsageGuidePanel）。
- 影响：站内私信功能实际上无法被普通用户发起；已开会话内新消息不实时。
- 修复建议方向：用户页/内容页加「发私信」入口（走冷启动限制）；ChatWindow 加轮询或 SSE。
- 涉及文件：frontend/components/social/ChatWindow.tsx、frontend/app/(public)/user/[userId]/UserProfileClient.tsx

### C-14 [C] P2 — 关注 IP 的通知发给 user_id=0（无效接收者）
- 证据：`backend/internal/handler/follow.go:72` `h.notifSvc.Notify(0, "follow", "follow", "有人关注了你关注的IP", "", "ip", ipID, callerID)`；实测 stream 出现 `"user_id":0` 条目；notifications.user_id=0 无人消费（无 id=0 用户）。
- 影响：写脏数据；从文案看原意可能是「通知该 IP 的关注者们」或「通知 IP 创建者」，现均未发生——IP 被关注对任何人都无感知。
- 修复建议方向：明确接收者（IP 创建者用 type content_status/follow 变体）或删除该通知。
- 涉及文件：backend/internal/handler/follow.go

### C-15 [C] P2 — 反馈工单状态通知不完整（in_progress/resolved 静默）+ 通知文案硬编码英文
- 证据：
  - `backend/internal/service/feedback_service.go:428-435` NotifyPatchTicket 仅 status==closed/reopened 通知；实测 patch in_progress 与 resolved 后 stream 长度不变。
  - `feedback_service.go:651-662` feedbackNotificationText 返回英文（"Feedback reply received" 等），前端 MarkdownRenderer 原样展示，zh 用户收到英文通知。
  - 用户侧可见面正确：GET /feedback/:id 只回非内部回复（feedback_repo.go:40-52 过滤 is_internal_note）✓；匿名工单走邮件（本地无 mailSender 静默跳过，feedback_service.go:510-512——真实环境推演：SMTP 缺配时匿名提交者同样无回执，仅 admin 侧可见）。
  - 状态机：patchTicket 任意状态可跳（open→closed 直接合法），无流转约束（feedback_service.go:395-408）。
- 影响：「已解决」这一用户最关心的状态不通知；工单进度感知靠手动刷新 /feedback/mine。
- 修复建议方向：resolved 也通知；文案走 i18n（通知 title/body 由前端按 type 渲染或后端带 locale）。
- 涉及文件：backend/internal/service/feedback_service.go

### C-16 [A/C] P2 — /users/:id/reputation 向任意人暴露他人信誉变动日志；而 /reputation-logs/me（自查）无任何前端消费
- 证据：
  - `backend/internal/handler/user.go:210-236` GetReputation 对任意 id 调 GetLogs（reputation_service.go:57-72），无 caller==id 或 admin 校验；reason 字段（如 ai_violation、judge_error_rate）对所有人可见。实测（321 查 322）返回 200 结构（seed 无数据，代码路径明确）。
  - `GET /reputation-logs/me`（routes.go:288，handler/reputation.go:25-36）frontend 0 消费（grep reputation-logs 无结果）。
  - `frontend/components/rehab/ReputationDetail.tsx` 仅展示 rehab 奖励点数/完成徽标，无规则明细。
- 影响：双重失败：该公开的不公开（自己的加减分/处罚原因无 UI 入口，「为何被扣分/被封」信息链断裂）、不该公开的公开（他人处罚历史）。
- 修复建议方向：/users/:id/reputation 限 self/admin；前端在 rehab 或 settings 加「信誉明细」页消费 /reputation-logs/me。
- 涉及文件：backend/internal/handler/user.go、frontend/components/rehab/ReputationDetail.tsx

### C-17 [C] P3 — 通知深链错位、下拉不标记已读、broadcast 未读键缺失、轮询间隔与文档不符
- 证据：
  - `frontend/components/social/NotificationList.tsx:246-262` getNotificationHref：discussion/comment → `/content/${target_id}`（讨论 id 与内容 id 是不同序列，跳错页）；appeal/feedback_ticket/message 无 case → 不可导航（申诉结果、反馈回复、新私信通知点击无效）。实测讨论回复通知若到达将跳 /content/{讨论id}。
  - `frontend/components/social/NotificationDropdown.tsx:79-92` getNotificationUrl 定义后未使用（175-177 一律 router.push("/messages")）；下拉项点击不 markRead。
  - `frontend/contexts/AuthContext.tsx:104` UnreadCounts 类型缺 broadcast 键，Dropdown:146 `unreadCounts[ch] > 0` 对 broadcast 恒 false（后端 unread_counts 实际含 broadcast 计数并计入 total，chips 永不显示广播来源）。
  - 文档 business-rules.md:141「轮询 5 分钟间隔」vs AuthContext.tsx:181 `setInterval(pollUnread, 30000)`（30 秒）。
- 修复建议方向：统一深链映射（discussion→/ip/{x}/discussions/{id} 需后端补 ip_id 或前端从 target 查）；下拉点击标记已读；UnreadCounts 补 broadcast；同步文档。
- 涉及文件：frontend/components/social/NotificationList.tsx、frontend/components/social/NotificationDropdown.tsx、frontend/contexts/AuthContext.tsx

### C-18 [C] P3 — 交互细节杂项
- 证据与要点：
  1. admin 不能置顶讨论：PinDiscussion 仅认 IP 创建者（discussion.go:118-147），实测 admin 调用 403——运营无 override。
  2. 删除非本人评论返回 500 风格 `{"code":"ERROR","message":"an error occurred..."}`（social.go:78-80 用 SafeErrorResponse 包 400），编辑同类场景返回 403 FORBIDDEN——同一权限错误两种语义。
  3. 点踩首发通知标题错误：social_service.go:215 `Notify(..., input.Reaction, "新的赞", ...)`——dislike 也叫「新的赞」（实测 stream：type=dislike, title=新的赞, channel=like）；且点踩是否应通知作者本身值得商榷。
  4. 粉丝/关注列表 API（routes.go:222-223）无前端消费（仅 studio 用 me/followers/stats）；用户页无粉丝入口。
  5. /messages 页会话 Tab 计数硬编码 0（messages/page.tsx:28 `count: 0`）。
  6. POST /social/comments 响应 target_id=0（model 只填 content_item_id），与 list 响应（target_id=179）字段口径不一。

## 二、通知触达矩阵（J11 核心交付）

说明：「经队列」= Notify() → Redis Stream `notification.create` → 需 cmd/worker 消费落库（本环境 worker 未跑，见 C-02）；「直写」= 同步落 DB。前端频道 = NotificationList 的 channel Tab / Dropdown chips。

| # | 事件/状态转移 | 产生通知? | type / channel | 代码位置 | 前端展示频道 | 缺口判定 |
|---|---|---|---|---|---|---|
| 1 | 内容发布成功 | ✗ 无 | — | content_service.go（同步返回） | — | 可接受（同步响应即反馈）|
| 2 | 内容被 ban（AI 审核） | ✗ 无 | — | review_service.go:330-345 | — | **缺**：作者无告知，连 -3 信誉分也无感知（C-06）|
| 3 | 内容被 ban（admin） | ✗ 无 | — | admin.go:236-262 | — | **缺**（C-06）|
| 4 | 内容转 under_review（举报率/AI） | ✗ 无 | — | social_service.go:296-307; review_service.go:346-357 | — | **缺**（C-06）|
| 5 | 判官闭案（恢复/不予展示） | ✗ 无 | — | judge_service.go:150-172 | — | **缺**；且 closed_approve 不自动恢复展示（C-06）|
| 6 | admin 恢复展示 | ✗ 无 | — | admin.go:271-281 | — | **缺**（C-06）|
| 7 | 申诉提交 | ✗ 无（admin 无提醒） | — | appeal.go:28-58 | — | 可接受（admin 面板轮询），但封禁用户根本提交不了（C-03）|
| 8 | 申诉出结果 | ✓ 有 | appeal_result / system | admin.go:417 | system Tab ✓（无深链 target=appeal） | 经队列；封禁用户收到也无法登录看（C-03）|
| 9 | PR 提交（给内容所有者） | ✗ 无 | — | pr_service.go:42-77 | — | **缺**：owner 只能轮询 studio PR 列表 |
| 10 | PR 接受 | ✓ 有 | pr_accepted / pr | pr_service.go:124 | pr Tab ✓ | 经队列 |
| 11 | PR 拒绝 | ✓ 有 | pr_rejected / pr | pr_service.go:157 | pr Tab ✓ | 经队列 |
| 12 | PR 合并 | ✓ 有 | pr_merged / pr | pr_service.go:213 | pr Tab ✓ | 经队列 |
| 13 | 协作邀请（发给被邀人） | ✗ 无通知（仅 typed 私信入会话） | — | collab_invite_service.go:278-286 | 会话 Tab 未读可见 | **半缺**：无铃铛/无实时；接受/拒绝也不通知邀请者 |
| 14 | 评论（内容） | ✓ 有 | comment / reply | social_service.go:102 | reply Tab ✓ 深链 content ✓ | 经队列 |
| 15 | 回复（讨论，前端实际路径） | ✗ 无 | — | discussion.go:89-116 | — | **缺**（C-05；/social/comments 带 discussion_id 的路径有通知但前端不用）|
| 16 | 点赞/点踩（仅首次创建） | ✓ 有 | like 或 dislike / like | social_service.go:212-216 | like Tab ✓ | 经队列；dislike 标题错（C-18.3）；切换/取消不通知（合理）|
| 17 | 关注用户 | ✓ 有 | follow / follow | follow.go:41 | follow Tab ✓ 深链 user ✓ | 经队列；自关注不发 ✓ |
| 18 | 关注 IP | ✗（发给 user_id=0） | follow / follow | follow.go:72 | 无人可见 | **坏**（C-14）|
| 19 | 私信 | ✓ 有 | message / system | message.go:124 | system Tab + 会话未读双通道 | 经队列；双通道不同步+全文 body（C-12）；type 不在文档枚举 |
| 20 | 举报被处理（resolved/dismissed） | ✗ 无 | — | admin.go:736-775 | — | **缺**：举报人无任何回执 |
| 21 | rehab 完成 | ✗ 无 | — | rehab_service.go | — | **缺**：信誉恢复无感知（且 /reputation-logs/me 无 UI，C-16）|
| 22 | admin 广播 | ✓ 有 | system / broadcast（直写） | notification_service.go:66-181 | broadcast Tab ✓（chips 有 bug C-17） | 实测 26 收件人即时可见 ✓ |
| 23 | @提及 mention | ✗ 类型从未产生 | — | 全库 0 使用 | — | **规格项未实现**（枚举有 mention）|

Outbox 关系注记：四类内容事件（content.published/updated/banned/deleted）经事务性 outbox → relay → indexer worker，只更新搜索索引/嵌入（container.go:305-310），与通知系统无交集——异步链路上唯一的通知消费者是 notification.create → NotificationWorker（worker/notification_worker.go）。不要把「outbox 有事件」误读为「用户有通知」。

通知类型枚举对账（business-rules.md:140）：comment ✓（#14）、like ✓（#16）、follow ✓（#17）、system ✓（#8/#19/#22）、mention ✗ 未实现、appeal_result ✓（#8）、content_status ✗ 未实现（C-06）；实际还存在文档外类型：message（#19）、pr_accepted/pr_rejected/pr_merged（#10-12）、dislike（#16）、system 子类（反馈）。

## 三、评论/讨论 × 角色矩阵

| 动作 | 游客 | 信誉<3 用户 | 普通登录用户（内容评论区） | 讨论区 | IP 创建者 | admin |
|---|---|---|---|---|---|---|
| 浏览评论/讨论 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| 发表评论 | ✗（401 提示登录） | ✗ 403 INSUFFICIENT_REPUTATION（实测） | ✓ 本地 fail-open 直发 status=published；release 走 Green 拒绝制，无 pending 态 | 发帖：**可绕过**（C-04 实测 201）；回复：403 拦截 ✓ | 同左 | 同左 |
| 编辑评论 | — | — | 仅作者；编辑重过审核（edit_comment）✓；**无 UI**（C-11） | — | — | 无编辑他人评论路径 |
| 删除评论 | — | — | 仅作者，软删 status=hidden ✓；**无 UI**（C-11）；他人删除得到 500 风格 ERROR（C-18.2） | — | — | **无评论删除/隐藏端点**（C-08） |
| 回复嵌套 | 只读 | 403 | API 支持 parent_id；UI 无入口且子回复不可见（C-10） | flat 一层 | 同左 | 同左 |
| 点赞/点踩 | ✗ | ✗ 403 | 幂等语义：同值再点=取消（removed）、异值=切换（updated）✓ 实测；仅 created 通知 | 评论也可赞踩（无展示效果 C-08） | 同左 | 同左 |
| 折叠（≥0.30） | — | — | 未实现（C-08） | 未实现 | — | — |
| 讨论置顶 | — | — | — | 仅 IP 创建者可置顶（实测：非创建者 403、创建者 200） | ✓ | **403 无 override**（C-18.1） |
| 举报 | ✗ | ✗ 403 | ✓（重复举报 ErrAlreadyReported）；评论举报后无处置联动（C-08） | ✓ | 同左 | admin 仅标记 report 状态 |

## 四、API-UI 对照缺口

| API | 后端状态 | 前端消费 | 结论 |
|---|---|---|---|
| PATCH/DELETE /social/comments/:id | ✓（含限流+A4 编辑审核） | ✗ 零调用 | C-11 出口不可达 |
| POST /social/discussions（带门+审核版） | ✓ | ✗（前端用无守卫的 /ips/:id/discussions） | C-04 双路径分叉 |
| GET /social/comments?parent_id= | ✓ | ✗ | C-10 |
| GET /users/:id/followers / following | ✓ | ✗（仅 studio 用 me/followers/stats） | C-18.4 |
| GET /reputation-logs/me | ✓ | ✗ | C-16 |
| GET /users/:id/reputation | ✓（越权暴露） | ✗ | C-16 双向缺口 |
| GET /messages/:id（翻页） | ✓ | ChatWindow 固定首页 | 可接受（MVP） |
| GET /notifications?channel= | ✓ | NotificationList Tab ✓ | 正常 |
| POST /notifications/read-all（支持 channel） | ✓ | ✓ | 正常 |
| PATCH /discussions/:id/pin | ✓（仅 IP 创建者） | DiscussionBoard 有入口（创建者视角） | admin 无 override（C-18.1） |
| POST /feedback（匿名 + captcha） | ✓ | (public)/feedback 页 ✓ | 封禁用户被降级匿名（C-03.4） |
| GET /feedback/me、GET /feedback/:id | ✓（内部备注过滤 ✓） | (protected)/feedback/mine、[id] ✓ | 正常 |

## 五、五问快评（按旅程）
- J8 评论：入口 ✓（内容页）；状态可知 ✓（发布即见）；结果反馈 ✓；出口 ✗（编辑/删除/子回复无 UI，C-10/C-11）；治理出口 ✗（C-08）。
- J8 讨论：入口 ✓；发帖门禁 ✗（C-04）；置顶权限可解释但 admin 无 override；楼主触达 ✗（C-05）。
- J8 私信：入口 ✗（无发起对话 UI，C-13）；未读双轨 ✗（C-12）；实时 ✗（SSE 未实现）；删除/退出会话 ✓。
- J8 通知：可达性依赖 worker（C-02）；频道划分 ✓；深链部分错位（C-17）。
- J12 主页：他人视角基本正确（内容只 published ✓、私有收藏集不可见 ✓ 实测）；email 泄露 ✗（C-01）；信誉透明度 ✗（C-16）；studio 管理面 ✗（C-07）。
- J13 反馈：提交→列表→详情→admin 回复/状态流转链路 ✓（实测）；通知不完整（C-15）；封禁用户通道变形（C-03.4）。

## 附录：psql / curl 构造记录（仅动用 321/322 自有对象）

psql：
1. `UPDATE users SET is_banned=true WHERE id=321;`（封禁实测，测毕还原 false）
2. `UPDATE users SET reputation=1 WHERE id=321;`（低信誉实测，测毕还原 17；注意需 `redis-cli del user:status:321` 清 RuntimeStatusCache，否则读到旧封禁态）
3. 只读查询：notifications / content_items / ips / collections / discussions 结构与数据核对。

curl（关键复现序列，cookie+CSRF 按 API 配方）：
- 登录 321/322/admin → tokens（/auth/csrf + /auth/login）。
- `POST /social/comments`（content 179，321）→ 201 status=published，无 pending 语义；随后 PATCH 编辑（321 ✓ 200 / 322 ✗ 403）、DELETE（322 → ERROR）。
- `POST /social/reactions`×3（like→removed/created/dislike→updated）验证幂等切换。
- `POST /users/322/follow`、`POST /ips/217/follow` → stream 出现 user_id 322 与 user_id 0 两条。
- `POST /ips/217/discussions`（322、低信誉 321 两种身份）→ 均 201（后者证 C-04）。
- `PATCH /discussions/1/pin`（322 ✗ 403 / 321 ✓ 200 / admin ✗ 403）。
- `POST /discussions/1/comments`（321）→ 无通知产生（证 C-05）。
- `POST /messages`（321→322 两条）→ 第一条 201+stream「新私信」，第二条 DM_REPLY_REQUIRED（冷启动 ✓）。
- `GET /notifications/unread-count`、`PATCH /notifications/2878/read`、`POST /notifications/read-all`（322）→ 行为正常。
- 封禁态：`POST /feedback`（CONTACT_EMAIL_REQUIRED→CAPTCHA_REQUIRED）、`GET /feedback/me`、`POST /appeals`、`POST /social/comments`、`POST /messages` 均 USER_BANNED；`GET /auth/me` USER_BANNED。
- 反馈链：`POST /feedback`（web_bug）→ ticket 1；admin `PATCH /admin/feedback/1`（in_progress、resolved 均无通知）、`POST /admin/feedback/1/replies`（公开回复 → stream「Feedback reply received」）；用户 `GET /feedback/me`、`GET /feedback/1` ✓。
- 广播：`POST /admin/notifications/broadcast`（Idempotency-Key 头）→ 26 收件人直写即时可见；重放语义未测（不影响结论）。
- 每步以 `redis-cli xrange omnicraft:notification.create - +` 与 `xinfo groups ... lag` 核对（最终 lag=8，全部为本审计产生，worker 未运行）。

审计结束时状态还原：321 is_banned=false、reputation=17 已确认；创建的评论 7201（已编辑）/7202、讨论 1/2/3、消息、reaction、关注关系、工单 1、广播 1 保留（均为 321/322 自有对象，符合纪律）。
