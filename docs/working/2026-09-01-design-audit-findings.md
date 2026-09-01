# 2026-09-01 全站设计审查 —— 交接问题清单（复核子代理唯一输入源）

> **Phase 6 提示（2026-09-01）**：本文 96 条之后又做了一轮执行前复核（证实 90/修正 5/驳回 1），并新增 F-097~F-117——**当前状态以文末「Phase 6 执行前复核增补」为准**；修复方案以 fix-plan §0 及各条目「Phase 6」标注为准。

> 创建日期：2026-09-01 ｜ **预计失效日期**: 2026-11-01
> 本文是 [设计审查主报告](2026-09-01-design-audit-report.md) 的交接契约文件（审计规程 §10.1）。
> **消费方**：未参与审查的全新子代理，按规程 §10.2 逐条独立复核（不得抽样），产出 `2026-09-01-design-audit-fix-plan.md`。
>
> **重要约定**：
> - 所有 `文件:行号` 证据均为审查阶段（5 个域子代理 + 主审者浏览器走查）引用，**标注为「未验证引用」**——复核时必须亲自重读代码/重跑复现，不得直接采信。
> - 域笔记（含完整 curl/psql 复现步骤与构造记录）：
>   - A 域（账号/发布/审核/申诉）：`docs/working/2026-09-01-audit-notes-account-publish.md`
>   - B 域（IP/PR/判官）：`docs/working/2026-09-01-audit-notes-ip-pr-judge.md`
>   - C 域（社区/通知/反馈/主页）：`docs/working/2026-09-01-audit-notes-social-notify.md`
>   - D 域（收藏/系列/搜索/Agent）：`docs/working/2026-09-01-audit-notes-collections-search-agent.md`
>   - E 域（Admin/横切）：`docs/working/2026-09-01-audit-notes-admin-crosscut.md`
> - 截图目录：`screenshots/audit/`（命名 `<页面>-<主题>-<视口>.png`）。
> - 编号映射：F-001~F-022=A-01~A-22；F-023~F-042=B-01~B-20；F-043~F-060=C-01~C-18；F-061~F-076=D-01~D-16；F-077~F-096=E-01~E-20；U-01~U-07=主审者浏览器实测。

## P0（阻断级，8 条）

### F-001（A-01）[A] 被封禁用户在产品内无任何解封申诉出路，封禁屏死链
- **描述**：账号被封禁后：登录返回 403 USER_BANNED；旧 token 调用全部 authReq 路由（含 GET /appeals/me、POST /appeals）401 USER_BANNED；前端 AuthContext.fetchMe 对任何错误清空用户踢回登录页，导致 `(protected)/layout.tsx` 中带 /appeals 链接的封禁提示屏永远不可达（且该链接自引用 protected 组）；申诉 target_type 无 account 枚举。唯一出路是匿名 POST /feedback 工单（实测 201 可用），但登录页无任何指引。
- **未验证引用**：`backend/internal/handler/auth.go:233-236`（登录 403）；`backend/internal/middleware/auth.go:100-104`（authReq 全拦）；`frontend/contexts/AuthContext.tsx:136-151`（fetchMe 清 user）；`frontend/app/(protected)/layout.tsx:47-58`（封禁屏）；`backend/internal/model/social.go:31` 或 `appeal.go:31`（target_type 枚举）。违反 `docs/reference/business-rules.md:260`（Task 105：封禁用户应见封禁 EmptyState + 申诉链接）。
- **复现**：psql 置 is_banned=true → curl 登录（403）→ 用旧 token 调 /appeals/me（401）→ 前端无法到达封禁屏。域笔记 A 附录有完整记录。
- **审查者初步定位猜测（未验证）**：需在 auth 中间件为 appeals 相关路由开白名单 + 新增 account 级申诉 + 前端封禁屏不依赖 401 API。

### F-002（A-02）[A] 作者在 Studio 看不到自己 pending/under_review/banned 的内容
- **描述**：GET /users/me/contents 不传 Status 参数，repo 默认只查 published → 作者的草稿/待审/被 ban 内容从「我的内容」消失，且列表不渲染状态徽标。
- **未验证引用**：`backend/internal/handler/user.go:266`；`backend/internal/repository/content_repo.go:156-160`；`frontend/app/(protected)/studio/contents/page.tsx`。
- **复现**：API 建一条内容 → psql 置 under_review → GET /users/me/contents 无该条。
- **猜测**：handler 需显式传 status 过滤集（作者视角含全部状态）+ 前端渲染状态列。

### F-003（A-03）[A] 众裁闭案不回写内容状态，under_review 永久滞留
- **描述**：判官闭案只更新 judge_cases 表，无 closed_approve/closed_reject → content status 的映射代码；举报 auto-hide 路径甚至不创建 judge case。两类 under_review 均无出口（仅 admin 手动）。与 F-024（B-02）同一事实的两半。
- **未验证引用**：`backend/internal/repository/judge_repo.go:98-105`；`backend/internal/service/social_service.go:296-307`；全库 grep 无 closed→content 状态转移。
- **复现**：构造 19 票 approve 闭案 → 内容仍 under_review（域笔记 B 有实证，content 177）。
- **猜测**：闭案事务中按结论回写 content status + 通知作者（联动 F-048/C-06）。

### F-004（B-01）[A] 判官考试前后端字段断裂：永远 0 分
- **描述**：前端提交 `answer_key`，后端只认 `answer` → 全对也得 0 分，判官资格从 UI 永远无法获得。运行时实证：answer_key→score 0；answer→10/10 通过。
- **未验证引用**：`backend/internal/service/judge_service.go:46`；`frontend/app/(protected)/judge/exam/page.tsx:93-96`。
- **复现**：UI 考试全选正确答案提交 → 得 0；curl 直发 `answer` 字段 → 通过。
- **猜测**：统一字段名（后端兼容两者或前端改发 answer）。

### F-005（B-02）[A] 判官闭案结果不生效
- **描述**：SubmitVote 达到票数阈值闭案后，内容状态不变（仍 under_review）、作者无通知。违反 business-rules.md「≥60% 不违规 → 恢复展示」。
- **未验证引用**：`backend/internal/service/judge_service.go:148-164`；`docs/reference/business-rules.md:160`。
- **复现**：psql 构造 19 票（或读闭案代码）→ 闭案 → GET /contents/:id 仍 under_review。
- **猜测**：闭案处调用 content 状态转移 + 通知（与 F-003 合并修复）。

### F-006（C-01）[A] GET /users/:id 未登录即可拿到任意用户 email
- **描述**：用户公开资料接口返回 email 字段，匿名可取。隐私泄露。
- **未验证引用**：`backend/internal/handler/user.go:439-453`。
- **复现**：无凭证 curl GET /api/v1/users/334 → 响应含 email。
- **猜测**：DTO 剔除 email（或仅本人可见）。

### F-007（D-01）[A] Agent 用户消息气泡亮色主题对比度 1.09:1（不可读）
- **描述**：用户气泡 `text-white` 叠在 `bg-accent`（Light=#F5F5F5）上，对比度 1.09:1（WCAG 正文要求 ≥4.5:1）；Dark 17.30:1 通过。审查阶段为静态计算（token 值推导），浏览器实拍因本地 agent gate 关闭未能取得气泡实例。
- **未验证引用**：`frontend/components/agent/AgentWorkspace.tsx:575`；`frontend/app/globals.css:84`；token 表 `design/design-system.md`。
- **复核路径**：读代码确认 class 组合 → 用 design-system token 值复算对比度；若本地把 `agent.web_agent_enabled` 临时开为 true（审计不允许，复核阶段也不改业务代码——建议仅静态复核）。
- **猜测**：气泡应使用 `bg-primary text-primary-foreground` 或暗色 accent 方案。

### F-008（E-01）[A] /admin/reports「成立/驳回」100% 失败且静默
- **描述**：前端发 `{resolution, explanation}`，后端 binding 要求 `status` 必填并落库 `action_taken` → curl 复现 400 `status is required`；且 onConfirm 的错误被 ConfirmModal 吞掉，管理员无任何提示。
- **未验证引用**：`frontend/app/admin/reports/page.tsx:173-183`；`backend/internal/handler/admin.go:742-764`；`backend/internal/model/review.go:18-31`（无 resolution/explanation 列）。
- **复现**：UI 点「成立」填理由确认 → 无反馈；curl 同 payload → 400。
- **猜测**：前端改发 {status, reason}；ConfirmModal 透传错误。

## P1（严重级，34 条）

### F-009（A-04）[A] GET /contents/:id 无可见性校验
banned/under_review 内容匿名可读（handler/content.go:309 → FindByID 仅滤 deleted_at；实测 banned 182 匿名 200，SSR 详情页直接渲染）；且对隐藏内容计浏览量。猜测：FindByID 加 status 白名单 + 作者豁免。

### F-010（A-05）[A] 已发布内容改标题/封面绕过再审核
UpdateContent 无状态检查、无重审（content_service.go:898-926）；实测编辑后 ai_review_records 不变；banned 内容也可 PATCH 200。安全+公平性问题。猜测：编辑触发再审核或对已发布编辑走增量审核。

### F-011（A-06）[A] 作者编辑/删除内容在 UI 零入口
studio/contents 按钮无 onClick（studio/contents/page.tsx:104-111）；全前端无 PATCH/DELETE /contents/:id 调用；详情页无 owner 控件。后端能力是死 API。与 E-04（F-080）同根。

### F-012（A-07）[A] 举报完全黑箱
无 reports/me 端点（routes.go 穷尽）、无「我的举报」页；ResolveReport 不通知举报者（admin.go:736-773）；评论举报后端存在但前端无入口；重复举报 409 处理正常。

### F-013（A-08）[A] 内容被封原因零触达
content_items.ban_reason 列存在但 model 无字段、任何 API 不返回；ai_review_records 无用户侧端点；banned 无作者通知（outbox 只喂搜索 indexer）。用户被 ban 不知为何。与 E-03（F-079）封禁理由全链路丢失同域。

### F-014（A-14）[B] 注销账号 = is_banned=true + ban_reason='self_deleted'
非 deleted_at 软删（user.go:347-363）；已注销用户会看到「账号被封禁」语义；邮箱改写后忘记密码静默死路。违反 Task 103 软删除规范。

### F-015（B-03）[A] 举报自动隐藏不触发众裁
social_service.go:305 置 under_review 但不建 judge case，该状态无出口；business-rules.md:164 要求「触发众裁」。

### F-016（B-04）[B] 判官看不到争议内容本体，按「内容 #177」盲投
ReviewCard 不渲染内容（ReviewCard.tsx:71-75 占位文案）；无跳过/举报升级；且 under_review 内容 GET /contents/:id 匿名可读（媒体 URL 无鉴权直出）。内容安全暴露检测五项结论见域笔记 B（本条与 F-009 交叉）。

### F-017（B-05）[B] PR new_text 提交即丢弃
pr_service.go:64-70 不保存新文本；proposed_version_id 恒 NULL；DiffViewer 右栏永远为空。运行时实证。

### F-018（B-06）[B] 前端无任何 POST /pr 调用，PR 创建旅程断裂
SubmitPREntry 是无引用孤儿组件；链接目标页不支持创建。（与 F-020 交叉）

### F-019（B-07）[A] PR accept 与 merge 互斥且 accept 零效果
accept 后 merge 返回 400（pr_service.go:175）；accept 本身不改内容；merge 无 UI 入口。运行时实证。

### F-020（B-08）[A] /studio/pr-requests 是空壳，真实实现滞留旧 /dashboard
studio 侧边栏、待办卡、通知路由全指向空壳页；旧 /dashboard 版本无 301 且无入口。（与 D-02/F-062 合并复核）

### F-021（B-09）[A] 判官「closed」词表前后端不一致
judge_service.go:185 撤权检查与 VerdictDetail.tsx:129 结案横幅都匹配 "closed"，实际枚举是 closed_approve/closed_reject → 撤权死代码、结案横幅永不显示。

### F-022（B-10）[B] IP 创建无前端入口
POST /ips 无 UI 调用；创建者无「我的 IP」页、无状态/过审通知。

### F-023（B-11）[A] IP 拒绝原因被 UI 收集后在发送前丢弃
admin/ips/page.tsx:195 丢弃 `_reason`；后端不收 reason、ip_review_logs 恒空；无编辑重提路径（一次死刑）。运行时实证。

### F-024（B-12）[A]（=断言 F8）stats_service 用 'published' 统计 IP → 首页 Active IPs 恒 0
stats_service.go:46；DB 16 条 approved，清缓存后 /stats/summary 仍 ips=0。主审者浏览器走查视觉证实首页「Active IPs 0」。

### F-025（C-02）[B] 通知全走 Redis Stream 依赖独立 worker，本地标准启动方式下通知永不落库
notification_service.go:274-306、container.go:275-307；AGENTS.md Step 2 只启动 server → 实测 lag=8；仅 admin 广播直写可达。产品语义问题：worker 是唯一通知通道，但「本地开发模式」文档与实际部署（面试单机 lean profile）都只跑 server。

### F-026（C-03）[A] 封禁用户申诉出口三重断裂
POST /appeals 401、/auth/me 401 致前端封禁 EmptyState 不可达、申诉链接在 protected 内自循环（auth.go:90-104、AuthContext.tsx:139-148、(protected)/layout.tsx:42-53）。与 F-001 同根，合并复核。

### F-027（C-04）[A] 讨论发帖路由绕过信誉门+Green 审核
前端实际使用 /ips/:id/discussions（routes.go:274）而非 /social/discussions（routes.go:138）；discussion.go:44-68 无信誉检查无审核；rep=1 实测仍 201。

### F-028（C-05）[A] 讨论回复不走审核也不通知楼主
/discussions/:id/comments（discussion.go:89-116）与 /social/comments（social_service.go:104-108）语义分叉。

### F-029（C-06）[A] 内容状态转移零通知
banned（AI/admin）、under_review、判官闭案、恢复展示均不告知作者；content_status 通知类型枚举有定义全库 0 使用；判官 closed_approve 不自动恢复展示（review_service.go:330-345、admin.go:236-262、judge_service.go:150-172；business-rules.md:140）。与 F-003/F-005/F-013 强关联。

### F-030（D-02）[A] studio 三个协作页是空壳 + 旧 /dashboard/contents 字段大小写损坏
studio/pr-requests|contributors|tag-suggestions 为空壳页；旧 dashboard/contents/page.tsx:16-22 读大写 ID/Title 而 API 返回小写 → 标题空白、链接 /content/undefined（实测）。（断言 F9 细化）

### F-031（D-03）[A] 标签建议触达闭环断裂
提交无通知（实测 notifications 0 条）；overview 待办卡 setPendingTasks 从未被调用恒空（studio/overview/page.tsx:32、PendingTasksCard.tsx:27）；唯一可用审核页无入口。

### F-032（D-04）[B] PR 提交入口断头
SubmitPREntry 带 content_id&create=1 跳 /dashboard/pr-requests，目标页不读参数也无创建流程（components/pr/SubmitPREntry.tsx:28）；全站无 UI 调 POST /api/v1/pr。

### F-033（D-05）[A] /studio/favorites 收藏管理页近乎不可达
Header/StudioSidebar 无入口；home 与原创侧栏「我的收藏」错链到 /studio/contents（HomePageClient.tsx:154、OriginalSidebar.tsx:47）；唯一入口是公开集页「管理」按钮（collections/[id]/page.tsx:154）。主审者浏览器走查视觉证实错链。

### F-034（E-02）[A] /admin/config 字段名断裂，保存会静默改坏配置
后端结构体无 json tag → GET 返回 PascalCase（PassThreshold:0.6），前端读 snake_case 永远显示默认值（显示 0.8）；打开即保存会把 repeat_violation_extra_penalty 从 -1 静默改成 +1；judge 区块 5 字段后端完全不处理（保存 200 假成功）。config.go:255-300、model/config_public.go:5-16、admin.go:451-530。

### F-035（E-03）[A] 封禁理由全链路丢失
内容 ban / IP 驳回弹窗强制填 reason 但前端丢弃 `_reason` 未发送（admin/contents/page.tsx:188-194）；后端支持 reason 入库但 ban_reason 前端零消费；DB 9 条 banned 全无 reason。与 F-013 同域合并。

### F-036（E-04）[A] 作者侧信息不可知（studio/contents 不渲染状态、申诉要手填数字 ID）
studio/contents/page.tsx:16-19,89-121 加载 status 不渲染、编辑/删除死按钮；appeals/page.tsx:121-127 申诉表单要手填数字 target_id。与 F-002/F-011 合并。

### F-037（E-05）[C] admin 动作静默（通知矩阵缺口）
content ban/restore、user ban/unban、IP approve/reject、report resolve 全部不通知用户；仅 appeal resolve（标题硬编码中文）与 feedback 发通知。与 F-029 合并为「触达矩阵」修复项。

### F-038（E-06）[C] 全站无任何指向 /admin 的入口；/admin/notifications 孤儿页
Header 用户菜单无 admin 项（Header.tsx:242-287）；/admin/notifications 不在 ADMIN_NAV（admin/layout.tsx:16-29）。

### F-039（E-16）[C] 错误码→用户语言映射不足
user-facing-error.ts 仅 12 码；messages `error.*` 16 码是死代码；SearchAgentInput.tsx:67 裸显后端英文 e.message。高频裸出码：PUBLISH_FROZEN、APPEAL_EXISTS、SOURCE_IMMUTABLE、INSUFFICIENT_QUESTIONS、CAPTCHA_*、ARCHIVE_* 等（完整清单见域笔记 E）。

### F-040（E-07）[B] DLQ replay 后端已建但 queue 页无入口；stats/dlq 吞错伪装空态
admin/queue/page.tsx:42-47,152 `.catch(()=>null)`。

### F-041（E-08）[A] RestoreContent 不清 deleted_at；trash/restore 无 UI
恢复后内容仍留在回收站、前台不可见（admin.go:265-282）；两端点闲置。

### F-042（U-01）[A] 被封禁账号登录 403 后页面无任何错误提示
主审者浏览器实测：正确凭证登录被封账号 → 后端 403 USER_BANNED（curl 证实）→ 登录页无错误文案、无封禁告知、无申诉指引，页面呈干净待输入态（截图 `login-banned-light-1440.png`）。login/page.tsx:112 有 error 渲染位，为何未显示需复核（可能与 F-001 同根，也可能 error key 未覆盖 USER_BANNED）。

## P2（明显可优化，43 条）

### F-043（A-09）发布冻结（PUBLISH_FROZEN）无剩余时间、无解冻路径、rehab 不解冻；前端吞错误码只报「发布失败」（实测 403）。
### F-044（A-10）申诉批准强制 published、不清 ban_reason、不校验目标存在（target 999999 → 201 实测）。
### F-045（A-11）admin ban/restore/appeal/auto-hide 不失效 Redis 缓存——封禁内容最长 300s 仍可读（实测预热后 ban 仍 serve）。
### F-046（A-12）登录验证码门无 UI（登录页无 CaptchaWidget）、被 429 限流掩蔽、失败计数器 TTL 仅 60s。主审者实测被限流 429 连发。
### F-047（A-13）api.ts 只认后端从不返回的 TOKEN_EXPIRED 码；轮询间隙过期请求不刷新被踢登录页且丢 query 参数。
### F-048（A-15）主列表不排除封禁作者内容（1032 仍列出），与 related-fanworks 可见性口径不一致。
### F-049（A-16）压缩包带毒（archive scan blocked）作者侧无区分文案【推演，feature flag 关闭】。
### F-050（A-17）IP 被 ban 级联下架无作者通知【代码级】。
### F-051（B-13）BanIP/BanIPAndContents 死代码（无路由）且级联下架无作者通知。
### F-052（B-14）pending/rejected/banned IP 详情匿名可直链 + admin 决策后 IP 缓存 300s 不失效。
### F-053（B-15）判官考试可单题通过（1/1=100% 实证）、提交时重新抽题、无重考限制。
### F-054（B-16）verdict API 返回裸 votes（缺 judge_name/upvotes，前端契约不符）、理由投票可自赞、409 静默、匿名可读。
### F-055（B-17）贡献者权限仅「可邀请他人」；contributors 页数据前端拼装且 blocked 恒 false、屏蔽无告知。
### F-056（B-18）merge 只建版本不同步 content_items.body（读者永远看不到合并结果）+ GET /versions/:id 无鉴权。
### F-057（C-07）[A] /users/me/contents 只返回 published（=F-002 同根，C 域独立证实）。
### F-058（C-08）[A] 评论折叠规则（点踩/点赞 ≥0.30 折叠+触发审核）完全未实现；admin 无评论处置端点；评论举报 resolve 无执行动作。
### F-059（C-09）[A] 内容评论列表 author 恒为零值（social_repo.go:38-51 无 Preload，对比 65 行）→ 前端渲染空白用户名（CommentSection.tsx:186）。
### F-060（C-10）[C] 评论回复嵌套断裂：API 支持 parent_id，UI 无回复按钮且子回复永不渲染。
### F-061（C-11）[B/C] 评论编辑/删除 API 完备（含 A4 编辑审核）但前端零调用。
### F-062（C-12）[C] 私信双通道未读不同步 + 私信全文进通知 body。
### F-063（C-13）[C] 私信无「发起对话」入口（仅协作邀请被动建会话）；规格承诺的 SSE 实时未实现。
### F-064（C-14）[C] 关注 IP 的通知发给 user_id=0，永远无人消费。
### F-065（C-15）[C] 反馈工单 in_progress/resolved 状态不通知（仅 reply/closed/reopened）；通知文案硬编码英文。
### F-066（C-16）[A/C] /users/:id/reputation 向任意人暴露他人处罚日志；/reputation-logs/me 自查端点无前端消费。
### F-067（D-06）[A] /search/trending 契约错配：后端 {trending:[{text,score}]} vs 前端 {items:[{name}]} → 原创页热搜模块永不渲染且静默（search.go:46-57、OriginalSidebar.tsx:22-36）。
### F-068（D-07）[A] 标签被认可 +1 信誉分（business-rules.md:26 承诺）未实现：ApproveTagSuggestion 零 reputation 调用（tag_service.go:101-124）。
### F-069（D-08）[C]（=断言 F11-②）series 空列表时页头按钮 + EmptyState 按钮同屏双 CTA（studio/series/page.tsx:241,256）；favorites 同型（L192+L209）。主审者浏览器实测 DOM count=2，截图 `studio-series-empty-{light,dark}-1440.png`。
### F-070（D-09）[C] Agent 429 限流无差异化 UX：错误码被丢弃（agent-stream.ts:133），统一「本次请求未完成」+重试（限流窗口内重试必败）。
### F-071（D-10）[C] SSE start/usage 事件有契约无消费：trace_id 与 token 用量均不展示（agent_stream.go:228,427）。
### F-072（D-11）[C]（=断言 F10 修正）components/content/ 下 ComplianceCheckBadge/UploadAssistPanel 为同名异构死代码（零 import），无行为漂移，但有误改风险。
### F-073（E-09）[A] ban 不存在的内容 ID 返回 200 假成功（curl /admin/contents/999999/ban 复现）。
### F-074（E-10）[A] BanUser 后端无 admin 角色/自身/存在性防护（仅前端禁用按钮）。
### F-075（E-11）[C] /admin/users 搜索只过滤当前页 20 条（后端无 search）、列表无 ORDER BY；appeals/contents/ips 无筛选。
### F-076（E-12）[C] 申诉决策信息不完整：无内容预览链接、ai_review_records 无任何 API/UI 暴露、无历史申诉；ResolveAppeal 无 pending 守卫可重复处理。
### F-077（E-13）[A] 用户侧不渲染 admin_response（后端已返回）；申诉结果只能靠通知；通知标题硬编码中文。
### F-078（E-14）[C] LLM 测试连接失败原因被吞（只显 ok/fail 图标）；Activate 任何错误统一 404。
### F-079（E-15）[C] audit-logs 筛选硬编码 15 个 action，缺 llm_config_*、dlq_replay、broadcast_notification、rag_rebuild 等。
### F-080（E-17）[C] judge/queue 固定取前 20 条无分页，超量案件不可达却显示「已完成」。
### F-081（E-20）[C] 批准「评论申诉」不会恢复评论（appealTargetUpdates 只映射 content）。
### F-082（U-02）[A] /original 移动端 375px 横向溢出 41px（主审者实测 scrollWidth-clientWidth=41；截图 `original-light-375.png`）。违反 D10「无横向滚动」。
### F-083（U-04）[B/C] /agent gate 竞态：登录态下 03:29 截到完整工作台 UI、03:45 同账号截到「Agent workspace unavailable」EmptyState——gate 判定时机/条件不一致需复核（截图 `agent-light-1440.png` vs DOM 实录）。
### F-084（U-06）[A] 首页/原创侧栏「My Favorites」错链 /studio/contents（主审者 DOM 实证：logged-out 态链接 /login?redirect=/studio/contents；=F-033 UI 面）。

## P3（打磨，12 条）

### F-085（A-18）发布类型清单与 config type_order 漂移且配置无消费者。
### F-086（A-19）上传上限前端硬编码 + 服务端超限报通用 VALIDATION_ERROR。
### F-087（A-20）申诉页不渲染 admin_response、admin 只见 pending（与 F-077 部分重叠）。
### F-088（A-21）本地无 worker 进程，notification.create 积压（demo 环境触达不闭环；=F-025 环境面）。
### F-089（A-22）版本仅 PR 合并产生，CreateInitialVersion 无调用方；用户无法理解版本历史。
### F-090（B-19）/ips 排序 4 选 3 无效（most_contents 拼写不匹配）+ admin 表 submitter_id 字段错名恒空。
### F-091（B-20）判官 +1 准确率奖励无调用点；worker 未运行通知滞留（=F-025）。
### F-092（C-17）通知深链 discussion→/content/{id} 跳错实体；appeal/feedback/message 无深链；下拉不标记已读；UnreadCounts 缺 broadcast 键；轮询 30s 与文档 5min 不符。
### F-093（C-18）杂项：admin 不能置顶讨论、删他人评论 500 风格报错与编辑 403 不一致、点踩通知标题「新的赞」、粉丝列表 API 无 UI、会话计数硬编码 0。
### F-094（D-12）[C] Agent 引用卡标题 Dark 对比度 4.24:1（#6366F1 on #0D1117，text-sm 非大字，差 0.26 未达 AA）（AgentCitationCard.tsx:26、globals.css:197）。
### F-095（D-13/14/15/16 合并）搜索/Agent 打磨集：/search/suggestions 全站无消费且前缀 ILIKE 查不到中文子串；/users/search 无搜索页入口；SearchAgentInput 用 ArrowLeft 图标语义误导；Agent 回答纯文本渲染无 markdown。
### F-096（U-03/U-05/U-07 合并）UI 打磨集：series 管理页标题三处重复 +「Delete series」危险按钮紧邻编辑区易误触（截图 studio-series-light-1440.png）；首页「Active IPs 0」错误统计的视觉呈现（=F-024）；全新浏览器默认英文 locale（产品主要面向中文用户，locale 协商按 Accept-Language 工作但默认策略值得产品确认）。

## 断言验证结论（§4 的 11 条，Phase 5 须抽验）

| 断言 | 结论 | 要点 |
|---|---|---|
| F1 内容状态机 | 证实 | banned 对 AI 通道终态；仅 admin restore/申诉批准/AI pass 可回 published；补充：auto-hide 是无守卫的 under_review 入口 |
| F2 无 rejected/PATCH 绕审 | 证实 | 无 rejected 状态；PATCH 只改 5 字段从不重置 status；编辑绕审实测成立 |
| F3 重复违规冻结 | 证实 | 窗口 7 天/阈值 2/-1 分/冻结 7 天 403 PUBLISH_FROZEN；无解冻代码 |
| F4 申诉仅 content\|comment | 证实 | 无账号级；同 target 唯一 pending 409；批准强制 published；无目标存在性校验 |
| F5 封禁拦截范围 | 证实 | 全 authReq 401 + 登录 403；封禁屏不可达；/appeals 死链成立 |
| F6 判官准入 | 部分证实 | 案件仅来自 Green review 级、block 不进队列 ✅；「判官看到原始内容」证伪（UI 不渲染内容本体）❌ |
| F7 判官闭环 | 部分证实 | 单案一票 ✅、≥20 票 ≥60% 闭案 ✅；错误率撤权是死代码（词表 bug）❌、+1 奖励无调用点 ❌ |
| F8 IP 状态机+stats bug | 证实 | 状态机与 'published' 统计 bug 均实证（ips=0）；「ban 级联」是死代码 ⚠️ |
| F9 双路由族 | 证实（细化） | 三根路由确为重定向；dashboard 四子页仍独立实现且 contents 页已损坏；studio 三页是空壳——「实现 vs 空壳」漂移 |
| F10 重复组件 | 证实（修正） | 非复制品漂移，是同名异构死代码（content 版零引用），无行为影响 |
| F11-① 气泡对比度 | 证实 | 12 组表面计算：用户气泡 Light 1.09:1 不达标（P0），引用卡 Dark 4.24:1 边缘未达；其余双主题通过 |
| F11-② series 双按钮 | 证实 | 空态 L241+L256 双 CTA；主审者浏览器 DOM count=2 + 截图；favorites 同型 |

## 复核统计口径（供对账）

- 功能发现：96 条（A22 + B20 + C18 + D16 + E20）
- 主审者浏览器实测 UI 发现：U-01~U-07 已并入上文编号（F-042/F-082/F-083/F-084/F-096 及并入项）
- 合计独立编号：F-001 ~ F-096（96 条）
- 严重度分布：P0×8（F-001~F-008）、P1×34（F-009~F-042，含 U-01）、P2×42（F-043~F-084）、P3×12（F-085~F-096，含合并项）
- 预期复核产物：每条三选一裁决（证实/修正/驳回）+ 修复方案覆盖全部证实与修正项

---

## Phase 6 执行前复核增补（2026-09-01，主会话）

> 第三轮独立复核：5 个全新域子代理（代码亲读 + 运行时 curl/psql/redis + F-042 真浏览器 Playwright）。基线 main@f9d9ef7。**上文 96 条原文保留作历史记录，当前状态以本节为准**；修复方案修订见 fix-plan §0 与各 FIX 条目内「Phase 6」标注。零业务代码修改，运行时写操作全部恢复。

**裁决统计：证实 90 ｜ 修正 5（F-011、F-022、F-025、F-042、F-083）｜ 驳回 1（F-082）｜ 被并行会话顺带修复 0（F-022 部分修复计入修正）。**

### 状态变化（6 条）

| 编号 | 裁决 | 结论 |
|---|---|---|
| F-011 | 修正（轻） | 死按钮结论不变；旧 dashboard/contents 孤儿页（零入口+字段损坏）存在一个 DELETE /contents/:id 调用，「全前端无 DELETE 调用」表述不准 |
| F-022 | 修正=部分已修复 | fecef1b 已交付创建入口+表单（/studio/publish/ip、IPPublishForm.tsx:84 实调 POST /api/v1/ips、/ips 头部按钮、pending 成功面板、ui-spec 新节）；仍缺 GET /users/me/ips、「我的 IP」状态页、过审/驳回通知、拒绝原因透出 |
| F-025 | 修正（范围收窄） | 代码结论全证实；但根 docker-compose.yml:290 已有 worker 服务（非 profile 门控）、docker-compose.interview-lean.yml 与 scripts/live-demo.sh:195-198 均含 worker——缺口收窄为 AGENTS.md Step 2 host-run 路径+积压清理 |
| F-042 | 修正（分歧收口） | 真浏览器实测（Playwright Chromium 1440×900）：封禁登录 403 后 [role=alert] 显示「账号已被封禁，请联系管理员」——文案链路端到端工作；登录错误链 5 文件审计后 0 改动，审计截图应为提交前/重访态。残留=无申诉/工单指引（FIX-15） |
| F-083 | 修正（维持） | AgentFeatureGate 无竞态（:46-47 顺序判定）+ public-config.ts:50 模块缓存根因，双域独立确认；补：fetch 失败 fail-closed 也呈现 unavailable |
| F-082 | 驳回（当前不可复现） | Playwright 375×812 匿名/登录态 scrollWidth==clientWidth==375；/original 组件审计后 0 改动；审计自存截图亦无页面级横向滚动条。不排除 IAB/WebKit 特有，留 Safari 复测备注 |

### 轻微勘误（裁决不变）

- F-016：judge/queue 实际返回完整案件行（judge.go:99，含 vote_approve/reject），非「只返回 target_type/target_id」——但确无内容数据，盲投结论不变。
- F-039：error.* 大写死码 15 个（非 16）；error.pageError/pageErrorDesc/notFound/notFoundDesc/backHome 5 个 key 在用（not-found.tsx/error.tsx）。
- F-096：i18n 词条数已随 fecef1b/6d1d781 变为 zh=en=1882（原 1857），parity 保持。
- F-035：DB 10 条 banned 中 1 条已有 reason（id 182，审计期 API 测试写入）——恰证明后端通道可用而 UI 丢弃，结论不变。

### 新增发现 F-097~F-117（21 条：P0×1、P1×3、P2×11、P3×6）

#### F-097（A）[A] P1 公共列表泄露 is_public=false 的私密内容
`content_repo.go:149-160` 主列表分支与 `GET /users/:id/contents` 均不过滤 is_public（visibility scope 的 `is_public OR author` 仅 source-linkage 分支生效）。运行时实证：匿名 `GET /api/v1/contents` 首页含 id=2（is_public=f，published）与 id=1034（「他人私有日记」）；匿名 `GET /users/313/contents` 含 id=2。与 F-009 详情洞叠加=私密内容全文可读。修复归属：FIX-12+FIX-43 统一 visibility scope（必含 is_public 维度）。

#### F-098（A）[A] P2 封禁屏替换 children，/appeals 对封禁用户永不渲染
`(protected)/layout.tsx:42-54` 对 banned 用户以封禁屏**替换** children——FIX-15 的 API 白名单落地后，封禁屏上的 /appeals 链接点击仍渲染封禁屏自身（死链第二层）。归属：FIX-15 补第 5 步（layout 对 /appeals 放行）。

#### F-099（A）[A] P3 HasPendingAppeal 错误被吞可致双 pending
`appeal.go:40` `hasPending, _ :=`；appeals 表无 UNIQUE 兜底，DB 错误静默放行可产生同 target 双 pending。归属 FIX-27 顺带（fail-closed）。

#### F-100（B）[A] P1 判官考试题目泄露正确答案（denylist 清洗不充分）
`handler/judge.go:50-57` 对 question_data 仅 `delete(data, "correct_key")` 单键，其余字段原样透传。两条通道：① 出题调度器 `scheduler/judge_question_sync.go:55-69` 从闭案生成题，question_data 内嵌 votes_approve/votes_reject——闭案规则 approve 占比≥pass_threshold → correct_key="B"，考试者可**机械计算多数派**得满分（本地 judge_cases=0 未触发，真实案件环境必然暴露）；② admin 手工题 explanation 常含答案明文（实测响应含「正确答案选A」）。归属：FIX-36 扩展——GetExam 改 allowlist（只透出 question/options/content_type）。

#### F-101（B）[A] P2 IP tags 免审核自由文本（6d1d781 引入）
`ip_service.go:175-186` ipReviewInput 只送 name/description/cover，tags 不进任何 Green 文本审核即随 IP 公开展示（IPDetail TagBadge）。归属 FIX-23 加固⑥。

#### F-102（B）[A] P2 IP tags 服务端无条数上限
`ip_service.go:63-81` normalizeIPTags 只 trim/dedupe/50 字截断，无条数 cap；前端 cap 10（IPPublishForm.tsx:22）仅客户端。API 直发可插无上限 ip_tags 行。归属 FIX-23 加固⑦。

#### F-103（B）[A] P2 POST /ips 无 interaction guard、无限流
`routes.go:86` 仅 authReq；对照 contents POST（:101）有 publishGuard+UploadRateLimit，PR/comments/collections 全带 guard。未验证邮箱/低信誉/发布冻结用户可建 IP（进入审核队列）且可无限刷。既有缺口，fecef1b 使其用户可达后升级。归属 FIX-23 加固⑤。

#### F-104（B）[A] P3 判官撤权阈值硬编码 0.6
`judge_service.go:193` 撤权重算用字面 0.6，闭案（:151-154）读 cfg.Judge.PassThreshold——FIX-03 激活撤权后 admin 调阈值会产生口径不一致误撤权。归属 FIX-03 补。

#### F-105（C）[A] P0 email 经 Author Preload 全站泄露
`model/user.go:9` `Email json:"email"` 未隐藏；凡 `Preload("Author")` 且序列化 author 的端点全部泄露真实登录邮箱（含 email_verified_at、support_info）。运行时实测：`GET /contents`（列表 author.email）、`GET /contents/5`、`GET /ips/217/discussions`、`GET /discussions/1`（discussion.author+每条 comment.author）。代码级同型出口：search_repo.go:193、series_repo.go:94、content_repo.go:57,138,146、discussion_repo.go:33,41,99、social_repo.go:65。**F-006 只是冰山一角；FIX-19 原 sanitizeUser 方案修完后这些出口仍漏**——升级为 model 层收敛。联动：FIX-29① 评论 Preload 完成后泄露面放大，必须同批。

#### F-106（C）[A] P2 讨论列表/详情无 status 过滤
`discussion_repo.go:18-26/39-41`（ListByIP/GetByID）无 status 条件（对比 socialRepo.ListDiscussions 有 published）。当前潜伏（无写入非 published 讨论的路径），评论/讨论治理接入状态机即成洞。归属 FIX-18 顺带。

#### F-107（C）[C] P3 讨论嵌套回复 API 可建、永不展示
`POST /discussions/:id/comments` 接受 parent_id（discussion.go:113-118），ListCommentsByTarget 滤 `parent_id IS NULL`（social_repo.go:56-58）。F-060 的讨论面变体。归属 FIX-18/29 顺带。

#### F-108（C）[C] P3 内容评论固定 50 条无分页
CommentSection.tsx:51 固定 page_size=50，第 51 条起不可达。归属 FIX-29② 顺带。

#### F-109（D）[A] P1 Agent SSE 读取无跨 chunk 行缓冲，事件静默丢失
`agent-stream.ts:144-152` 每 chunk 独立 split("\n")；跨网络分块的事件行前半 JSON.parse 失败、后半不以 `data:` 开头，双双丢弃。done（数 KB 单行含全量 answer+citations）与 citation（含 excerpt）最易中招——答案缺字/引用缺失/conversation_id 未回填。后端每事件单次 Write+Flush（agent_sse.go），分块边界取决于网络；现有测试无分块用例。归属：**FIX-44（新增）**。

#### F-110（D）[A] P2 原创页热搜展示名为静态 i18n 演示标签
OriginalSidebar.tsx:25-29,38：name 取 `t("home.trendingN")`（固定 5 条演示文案），真实 query 只进 href——契约打通后仍是假榜单。归属 FIX-06 前提②（渲染真实标题）。

#### F-111（D）[A] P2 深色主题 primary 按钮系统性 AA 未达
button.tsx:11 默认变体 `bg-primary text-primary-foreground`：Dark `--primary`=#6366F1 配白字 **4.47:1**（globals.css:119-120,127；差 0.03）；hover `bg-accent-hover` Dark #818CF8 配白字 **2.98:1**。全站 123 处默认按钮暗色中招。与 F-007/F-094 同根（#6366F1）；**同时证伪 FIX-05 原方案的 dark bg-primary（同样 4.47:1）**——Light #4F46E5 配白字 6.29 才达标；白字配 #818CF8 仅 2.98 不可用。

#### F-112（D）[A] P2 hot rank ZSet 含非公开内容
`rank:hot:contents` 当前 4 个 member：id 5=banned、41/42/13=under_review（psql 实证）；写入方（social_service.go:228 点赞、content_service.go:988/998/1039）不看内容状态。FIX-06 方案 A join 不过滤即把封禁内容标题+/content/{id} 直链推上原创页。归属 FIX-06 前提①。

#### F-113（E）[A] P2 admin DLQ 卡片字段契约断裂
前端 DLQEntry 期望 `{topic,retry_count,trace_id}`（queue/page.tsx:21-29）；后端实际 `{original_topic,original_id,consumer_group,attempts,error,failed_at}`（dlq_worker.go:102-111，无 trace_id）。运行时 18 条死信卡片 topic 全空白。归属 FIX-35 必补（先对齐字段再做 replay UI；Replay 语义=重投不删原条目）。

#### F-114（E）[A] P2 ResolveReport 无存在性校验
`admin.go:765-777` 对不存在 ID+合法 body 同样 Updates 0 行 → 200 假成功+审计行（F-073 同类未被覆盖；ResolveAppeal 有 First+404 对照 :385-388）。归属 FIX-34 清单补。

#### F-115（E）[A] P2 config 页 extra_penalty min={0} 与惩罚语义冲突
`admin/config/page.tsx:330` 数字输入 min={0}——FIX-33 补 json tag 后页面显示真实值 -1，原生 `:invalid` 校验（prepareSave）会阻断**一切**保存。归属 FIX-33 必补（放宽 min）。

#### F-116（E）[C] P3 ResolveReport「成立」不处置被举报目标
status=resolved 只改 report 行，对 content/comment 零动作；FIX-04/FIX-28 均未覆盖 enforcement。最小修复可接受，但需明示语义并留产品定夺。

#### F-117（E）[C] P3 admin/dashboard 吞错
dashboard/page.tsx:40 `.catch(()=>null)`（F-040 同族较轻变体；顺带修正 E 笔记：GET /admin/reports/stats 有消费方）。归属 FIX-35⑥。

### 操作性提醒（非编号发现）

notification stream（`omnicraft:notification.create`）现有 9 条积压（lag=9=XLEN，group last-delivered-id=0-0，残留 consumer 条目为 2026-08-28 worker 验证遗留、无隐藏消费者），含 follow.go 的 user_id=0 条目与审计期测试通知——FIX-09 启动 worker 前先做 FIX-31 的 follow.go Notify(0) 删除并清理积压（XTRIM/DEL+重建 group），否则全部物化为演示噪音。
