# 审计笔记 E：Admin 后台运营者旅程 + 全站横切检查（只审计不修复）

- 创建日期：2026-09-01
- **预计失效日期**: 2026-11-01
- 审计人：设计审查子任务 E（只读审计；未改业务代码、未 commit、未勾选 checkbox）
- 方法：代码 `文件:行号` 定位 + curl 实测（admin=seed-ui-001/id313；普通用户=seed-ui-013/id325）+ psql 只读查询。所有破坏性/写操作仅对 seed-ui-013 做了 ban→unban 往返并已恢复；judge_qualifications 对 325 做了 insert→delete 往返并已清理。
- 分类：[A] 规格不符 / [B] 规格缺失或过时 / [C] 设计可优化；严重度：P0 阻断 / P1 严重 / P2 明显可优化 / P3 打磨。

## 一、发现明细

### E-01 [A][P0] /admin/reports 的「成立/驳回」操作 100% 失败且静默 —— 前后端字段契约断裂
- 证据：
  - 前端 PATCH body：`frontend/app/(protected)/admin/reports/page.tsx:173-183`（`onConfirm` 内 `api.patch(..., { resolution: "upheld"|"dismissed", explanation })`）。
  - 后端绑定结构：`backend/internal/handler/admin.go:742-749`（`Status string \`json:"status" binding:"required"\`` + `ActionTaken`），只有 `status`/`action_taken`，无 resolution/explanation。
  - 模型无这两个字段：`backend/internal/model/review.go:18-31`（Report 只有 `ActionTaken`）。
  - curl 复现（admin token，error-path）：`PATCH /api/v1/admin/reports/999999 -d '{"resolution":"upheld","explanation":"test"}'` → **HTTP 400 `{"code":"VALIDATION_ERROR","message":"status is required"}`**（binding 在查库前失败，非存在性问题）。
  - 雪上加霜：该 `onConfirm` 无 try/catch、页面无错误展示位，`ConfirmModal` 把异常吞掉仅保持弹窗打开（`frontend/components/ui/confirm-modal.tsx:54-66` 注释「caller can show the failure」但 reports 页从未展示）→ 管理员点确认后毫无反应，无任何提示。
  - 列表/详情读取的 `report.resolution`/`report.explanation`（reports/page.tsx:126-137、14-25）后端永远不返回（模型无此列）→ 即使处理成功也永远看不到处理说明。
- 影响：举报处理这一核心审核动作在 UI 上完全不可用；且失败无反馈，运营者会以为点了没反应是卡顿。
- 修复方向：前端改发 `{status:"resolved"|"dismissed", action_taken: explanation}`（或后端扩展契约），展示 `action_taken`；onConfirm 补错误展示。

### E-02 [A][P1] /admin/config 前后端字段命名断裂（PascalCase vs snake_case）：显示假数据 + 「原样保存」会静默篡改运行时配置
- 证据：
  - 配置结构体只有 mapstructure 无 json tag：`backend/config/config.go:255-300`（LimitsConfig/ReputationConfig/JudgeConfig/SocialConfig 等）→ GET /admin/config 直接以 Go 字段名序列化。
  - curl 实测（admin token）：`GET /api/v1/admin/config` → `{"limits": {"VideoMaxMB":300,...}, "judge": {"MinVotesRequired":20,"PassThreshold":0.6,...}, "reputation": {"QualityContentThreshold":10,..., "RepeatViolationExtraPenalty":-1, ...}}`；而公开配置端点 `/api/v1/config/public` 全部 snake_case（自建 map，正常）。
  - 前端读 snake_case：`frontend/app/(protected)/admin/config/page.tsx:11-46`（`video_max_mb`、`min_votes_required`…），读不到则回退到 `defaultConfig`（48-55 行）。
  - 后果 1（显示假数据）：UI 显示 `pass_threshold=0.8`（前端默认），真实运行值 0.6；所有区块实际显示的都是前端默认而非服务器值。
  - 后果 2（保存篡改）：管理员打开页面直接点保存，`buildPatch()`（161-170 行）把前端默认值当当前值全量提交；PatchConfig 对已处理字段生效（admin.go:468-530）→ 例如 `reputation.repeat_violation_extra_penalty` 会被从真实 -1 静默改为 +1（惩罚变加分）。
  - 后果 3（静默丢弃）：`judge.*` 与 `reputation.quality_content_threshold/quality_comment_threshold` 在 PatchConfig 中完全没有处理分支（grep admin.go 无 "judge"/"quality_"）→ 判官区块的 5 个可编辑字段 + 信誉 2 个阈值「可编辑但永不生效」，保存还返回 200。
- 影响：Task 99 目标（config 可视化 + 与 config.yaml 一致性）实质未达成；存在运行时配置被静默污染的实际风险。
- 修复方向：PublicConfig 各嵌套结构补 json tag（或 GetConfig 组装 snake_case map）；前端 defaultConfig 合并逻辑去掉（空值禁用保存）；PatchConfig 对不支持字段显式拒绝或前端隐藏 judge 区块。

### E-03 [A][P1] 内容封禁/IP 驳回的「原因」在弹窗里强制填写，但被前端丢弃——封禁理由全链路丢失
- 证据：
  - 内容 ban：`frontend/app/(protected)/admin/contents/page.tsx:188-194`（`requireReason` + `onConfirm={async (_reason) => { ... banContent(...) }}`）且 `banContent` 发空 body `{}`（66 行）；后端 `backend/internal/handler/admin.go:242-251` 明确支持 `reason` 并写 `ban_reason`。
  - IP reject：`frontend/app/(protected)/admin/ips/page.tsx`（ConfirmModal `requireReason={confirmAction?.action === "reject"}`，`onConfirm={async (_reason) => ...}`）；后端 `RejectIP`（admin.go:172-197）根本不接 body。
  - 用户侧 `ban_reason` 零消费：grep frontend 全量（除测试）无 `ban_reason` 引用；DB 实测 9 条 banned 种子内容 `has_reason` 全为 f。
- 影响：运营被要求填原因→填了白填；作者永远不知道被封原因；审计日志里 content_ban 的 reason 也是空串（admin.go:252）。
- 修复方向：`banContent(id, reason)` 传 body `{reason}`；RejectIP 接 reason 并入审计；作者侧在 studio/contents 详情暴露 ban_reason。

### E-04 [A][P1] 作者侧封禁可知性 + 工作室死按钮：申诉闭环断裂
- 证据：
  - studio/contents 加载了 `status`（`frontend/app/(protected)/studio/contents/page.tsx:16-19,40`）但 renderItem（89-121 行）从不渲染：无 pending/under_review/banned 标签、无 ban 原因、无「去申诉」入口。
  - 同页 106-117 行「编辑」「删除」两个按钮只有 title 无 onClick——纯死按钮（幽灵入口）。
  - 申诉表单要用户手填数字 `target_id`（`frontend/app/(protected)/appeals/page.tsx:121-127`），作者在 UI 上无处获知自己内容的内部数字 ID。
  - 触达链唯一残存入口：`(protected)/layout.tsx:41-49` 封禁用户进入任何受保护页显示 suspended 页 + 「提交申诉」链接（这部分是好的）。
- 影响：内容被 ban 的作者（当前 9 条 banned）在不点通知的情况下无法发现自己被 ban、为什么被 ban、申诉时要填什么 ID。
- 修复方向：studio/contents 渲染 status 徽标 + banned 时展示 ban_reason 与「申诉」按钮（自动带 target_id）；移除或实现编辑/删除按钮。

### E-05 [C][P1] admin 动作→用户通知矩阵大面积静默（详见第三节矩阵）
- 证据：
  - 后端仅有两处 admin 链路发通知：申诉处理 `admin.go:415-418`（`Notify(appeal.UserID,...,"申诉处理结果",...)`，标题硬编码中文）与 feedback 工单 `service/feedback_service.go:506`；broadcast 走全量通道。
  - `BanContent/RestoreContent/BanUser/UnbanUser/ApproveIP/RejectIP/ResolveReport` 全部无 Notify 调用（notifSvc 在 handler 上可用，admin.go:39,77-79）。
  - live 复现：ban seed-ui-013 前后 `SELECT count(*) FROM notifications WHERE user_id=325` 均为 10（无新增），unban 已恢复。
- 影响：封禁用户/驳回 IP/内容下架均无触达，用户只能自己发现；「封禁通知用户吗」的答案是「不通知」。
- 修复方向：至少 user_ban、content_ban、ip_reject、appeal 之外的 report 处理结果补通知；通知标题/正文不要硬编码中文（见 E-13）。

### E-06 [C][P1] 管理后台双重入口缺失：全站无 /admin 链接；broadcast 页是孤儿
- 证据：
  - Header 用户菜单（admin 登录后同菜单）只有 个人主页/工作台/历史/设置/申诉/退出：`frontend/components/layout/Header.tsx:242-287`；全仓 grep `href="/admin` / `push("/admin` / `'/admin'` 零命中（除 admin 目录自身与 robots.ts disallow）。
  - `/admin/notifications`（全站广播，页面实现完整含 Idempotency-Key 幂等，`frontend/app/(protected)/admin/notifications/page.tsx` + 后端 admin.go:87-128 + 审计 `broadcast_notification`）不在 `ADMIN_NAV`（`frontend/app/(protected)/admin/layout.tsx:16-29`，12 项无 notifications），也无任何其它链接——只能手输 URL 到达。
- 影响：运营者（与演示场景）无法从导航发现后台；已实现的高质量 broadcast 功能实际不可达。
- 修复方向：Header 为 `user.role==='admin'` 增加后台入口；ADMIN_NAV 加「全站通知」。

### E-07 [B][P1] DLQ replay 无 UI 入口 + queue 页吞错显示空态
- 证据：
  - 后端已实现 `POST /admin/queue/dlq/:id/replay`（含 id 校验、审计、DLQ_ENTRY_NOT_FOUND 语义，admin.go:830-869，routes.go:355）。
  - 前端 queue 页只有 stats 表 + DLQ 只读卡片列表，页脚明示 `admin.queue.readOnlyNote`（`frontend/app/(protected)/admin/queue/page.tsx:152`）；无 replay 按钮。
  - 同页 `api.get(...).catch(() => null)`（42-45 行）：Redis 不可用时显示「暂无数据/无死信」空态而非错误——误导运维判断。
- 影响：#138 建成的 DLQ 重放能力对运营者不可操作；故障态被伪装成空态。
- 修复方向：DLQ 卡片加「重放」按钮（带确认+审计提示）；stats/dlq 请求失败时展示错误与重试。

### E-08 [A][P1] RestoreContent 不清 deleted_at：回收站恢复在数据层就是坏的；且无 trash UI
- 证据：
  - `backend/internal/handler/admin.go:265-282`：restore 仅 `Updates({"status":"published","ban_reason":""})`，不动 `deleted_at`；而 trash 列表按 `deleted_at IS NOT NULL` 过滤（215-234 行）→ 恢复后内容 status=published 却仍在回收站、前台不可见。
  - 前端无任何 trash/restore UI：grep frontend `contents/trash` / `/restore` 零命中（后端 routes.go:329,331 两端点闲置）。
- 影响：「trash 页恢复流程」整体缺失；若按 API 调用 restore 也不会真正恢复。
- 修复方向：restore 事务内 `deleted_at=NULL`；contents 页加「回收站」视图。

### E-09 [A][P2] BanContent 不校验目标存在性：封禁不存在的 ID 返回假成功
- 证据：curl（admin token）`POST /api/v1/admin/contents/999999/ban -d '{"reason":"audit-test"}'` → **HTTP 200 `{"message":"content banned"}`**；admin.go:236-263（`Updates` 不检查 RowsAffected）。
- 影响：误输入/过期 ID 得到成功反馈并污染审计日志（审计已多一条 phantom content_ban，系本次验证产生，可留可清）。
- 修复方向：先 `First` 校验存在与当前状态，404 CONTENT_NOT_FOUND。

### E-10 [A][P2] BanUser 后端无角色/自身防护（仅前端禁用）
- 证据：前端 `users/page.tsx:189,198` 对 `u.role === "admin"` 禁用按钮；后端 `admin.go:284-308` 对任意 id 直接 `Updates(is_banned=true)`，无 admin 角色/自身/存在性校验。
- 影响：API 层可 ban 掉其它 admin 或自己，造成后台锁死；与前端约束不一致。
- 修复方向：后端拒绝 `role=admin` 目标与 self-ban。

### E-11 [C][P2] admin 列表检索/排序能力不足
- 证据：
  - `/admin/users` 搜索是**客户端只过滤当前页 20 条**（`users/page.tsx:89-95`），后端 `ListUsers` 无 search 参数也无 ORDER BY（admin.go:330-345）→ 搜不到不在当前页的用户；翻页顺序不稳定可能重复/漏。
  - `ListAppeals` 固定 `status=pending` 无任何筛选、无 ORDER BY（admin.go:347-362）；`ListUnderReviewContents` 固定 under_review 无搜索（199-213）。
- 影响：用户量/申诉量增长后运营无法定位对象。
- 修复方向：后端加 `search`/`status`/`role` 参数与 `ORDER BY id DESC`；前端搜索改为服务端查询。

### E-12 [C][P2] 申诉决策信息不完整（admin 侧）
- 证据：
  - appeal 页仅显示 user_id/target_type/target_id/reason/status（`appeal/page.tsx:11-20,110-150`）；无内容预览链接（对比 contents 页有 View 链接）、无举报上下文、无该用户历史申诉。
  - `ai_review_records`（模型 `backend/internal/model/review.go:5-16`，含 result/raw_response/task_id）无任何 API/前端暴露：grep 后端 routes/handler 与前端均无消费 → AI 审核依据对终审者不可见。
  - `ResolveAppeal` 无 `status=pending` 守卫（admin.go:364-420）：已 approved/rejected 的申诉可被重复处理并再次触发目标状态变更与通知。
- 修复方向：appeal 行加「查看内容」链接 + 申诉详情（ai_review_records/历史）；ResolveAppeal 加 pending 守卫（409）。

### E-13 [A][P2] 用户看不到申诉处理说明；后端通知标题硬编码中文
- 证据：
  - 后端返回 `admin_response`（`model/social.go:90` json tag），但用户申诉页 interface 与渲染均不含该字段（`appeals/page.tsx:16-23,157-172`）。
  - 通知标题硬编码：`admin.go:417` `"申诉处理结果"`（英文 locale 用户同样收到中文标题）；同类硬编码见 follow.go:41「你有新粉丝」、message.go:124「新私信」等。
- 影响：用户申诉被拒后（若无通知）无处得知理由；通知文案无多语言。
- 修复方向：appeals 列表渲染 admin_response；通知文案改为 i18n key 或双语模板。

### E-14 [C][P2] LLM 配置「测试连接」失败无原因；Activate 错误统一 404
- 证据：`agent-config/page.tsx:115-124`（test 只 set "ok"/"fail" 图标，错误被 silentError 吞）；`admin.go:679-696`（ActivateLLMConfig 任何非审计错误一律 `404 CONFIG_NOT_FOUND`，DB 错误也被误报为不存在）。
- 修复方向：test 失败展示后端 message；activate 区分 404/500。

### E-15 [C][P2] audit-logs 动作筛选硬编码且缺新动作
- 证据：`audit-logs/page.tsx:92-108` 固定 15 个 option；后端实际产生 `llm_config_create/update/delete/activate/test`、`dlq_replay`、`rag_rebuild`、`broadcast_notification`（notification_service.go:260-273）、`archive_scan_*`（admin_archive_scan.go）、`category_reorder`（PUT /categories/reorder）等均无选项，且 select 不可自定义输入 → 无法筛选。
- 修复方向：改为后端 distinct action 下拉或文本输入。

### E-16 [C][P1] 错误码→用户语言映射覆盖严重不足（Key Rule 12），存在裸英文展示
- 证据：
  - 前端唯一映射器 `frontend/lib/user-facing-error.ts:6-32` 仅覆盖 12 个码。
  - `frontend/messages/{zh,en}.json` 的 `error.*` 命名空间有 16 个码（INVALID_ID/USER_NOT_FOUND/DB_ERROR/FORBIDDEN/VALIDATION_ERROR/FEATURE_DISABLED…）但**全站无消费者**（grep `error.${` / `t("error.` 零命中）——死代码。
  - 裸英文：`frontend/components/agent/SearchAgentInput.tsx:67` `setError(e.message)` 直接显示后端英文 message（MISSING_QUERY/QUERY_TOO_LONG/VALIDATION_ERROR 等走此路径）。
  - 其余未映射码落到各页 fallback（如 `admin.reports.loadFailed`）→ 一律「加载失败/操作失败」，用户无法区分 PUBLISH_FROZEN（信誉冻结）与网络错误。
- 完整清单见第二节表。
- 修复方向：把 `error.*` 命名空间接入 getUserFacingErrorKey（code→key 直查），补齐业务码；SearchAgentInput 改用映射。

### E-17 [C][P2] judge/queue 固定取前 20 条且无分页，超量案件不可达还报「已完成」
- 证据：`judge/queue/page.tsx:43-44` 固定 `page=1&page_size=20`；132-138 行 `currentIndex >= cases.length` 时显示 queueCompleted + `queueSummary({total: queueTotal})`——total>20 时第 21 件起永不可见。
- 修复方向：加载更多或翻页。

### E-18 [C][P3] admin 页面一致性问题集合
- 空态：`admin/contents` 用 `EmptyState`（icon+hint，contents/page.tsx:104-110）为标杆；`appeal/ips/users` 是裸 `<p>`（appeal/page.tsx:106-109、ips、users:134-137）无图标无引导。
- 枚举直出：所有 admin 表格直接渲染 `status`/`role`/`target_type`/`provider_type` 原始英文值（appeal:138,147；contents:130-137；users:172；agent-config:178-179）。
- 分页控件两种风格：`common.page` + justify-between（appeal/contents/users/ips）vs `page/totalPages` 裸数字 + justify-center（reports:258-268、audit-logs:171-181）。
- `ips` 页 `handleAction` 无 busy 状态，按钮不禁用 → 可双击重复提交（ips/page.tsx handleAction 段）。
- dashboard 四指标不含 under_review 待审内容数，quickLinks 也不含 /admin/contents（dashboard/page.tsx:69-115）——13 条待审内容在仪表盘上不可见。

### E-19 [C][P3] 幽灵入口与诚实性盘点（结论）
- `/client`：诚实 ✅。`app/(public)/client/page.tsx` 静态展示「客户端尚未发布…暂不可下载」，与 `features.desktop_deploy_enabled=false`/`client.download_enabled=false`（GET /config/public 实测均 false）一致；无下载按钮可点。注意 subtitle「桌面端应用，支持一键部署」是对未来能力的宣传，属可接受边界。
- `/deploy-grants`：前端零引用（grep 无命中）→ 无幽灵入口 ✅（后端 503 FEATURE_DISABLED 不可达）。
- 支付：`/studio/revenue` 常驻导航入口（StudioSidebar.tsx:59）指向 coming-soon 占位页——文案诚实（「收益功能即将开放」），但属于禁用功能的常驻入口，建议 payment_enabled=false 时隐藏或加 badge。
- studio/contents 编辑/删除死按钮：见 E-04。
- 其余导航链接（Header/Sidebar/StudioSidebar/admin NAV）均指向存在路由，无 404 死链（对照 `.next/types/routes.d.ts` 全量路由清单核验）。

### E-20 [C][P3] 批准「评论申诉」不会恢复评论
- 证据：`backend/internal/service/appeal_helpers.go:7-18`：approved 仅映射 `content→published`，comment 无映射 → `ResolveAppeal` 中 `len(targetUpdates)==0` 提前返回（admin.go:396-399），评论状态不变。
- 影响：评论申诉「批准」只改申诉状态，不改评论结果——语义半成品。
- 修复方向：定义 comment approved 的目标状态（如恢复可见）。

## 二、错误码 → 前端映射完整表（横切 1）

后端发出方式：`gin.H{"code":...}` / `response.Error` / `SafeErrorResponse`（safe_error.go 附加英文安全文案）+ 常量（service/runtime_status.go:17-21）。

### 已有有效映射（12）
| 错误码 | 前端 key | 来源 |
|---|---|---|
| INVALID_CREDENTIALS / USER_BANNED / EMAIL_NOT_VERIFIED / USER_EXISTS / USERNAME_TAKEN / TERMS_VERSION_MISMATCH / PRIVACY_VERSION_MISMATCH / TOKEN_EXPIRED / UNAUTHORIZED / RATE_LIMITED / CREDENTIAL_RATE_LIMIT_EXCEEDED / RESEND_COOLDOWN | auth.*/common.rateLimited | user-facing-error.ts:7-28 |

另有内置自动处理（不直达 UI）：`CSRF_TOKEN_INVALID`（api.ts:170-193 自动刷新重试）、`TOKEN_EXPIRED`（api.ts:195-215 自动 refresh）；ReactionBar 对 `ALREADY_REPORTED` 按 409 特判为已举报（social/ReactionBar.tsx:107-110）。

### 死映射（messages 有 key 但无消费者，16）
error.INVALID_ID / USER_NOT_FOUND / DB_ERROR / FORBIDDEN / INVALID_BODY / NO_FIELDS / VALIDATION_ERROR / INVALID_PASSWORD / INTERNAL_ERROR / COURSE_NOT_FOUND / ALREADY_COMPLETED / FEATURE_DISABLED / UNAUTHORIZED / NOT_FOUND / CONFLICT（zh.json `error` 命名空间；grep 无 `t("error.XXX")` / `error.${` 消费）

### 无任何映射的业务码（用户侧裸英文或统一「操作失败」，Key Rule 12 违例清单）
用户旅程高频：
- `PUBLISH_FROZEN`（publish 冻结，content.go:224 / interaction.go:74）
- `APPEAL_EXISTS`（appeal.go:42）
- `SOURCE_IMMUTABLE`（content.go:535）
- `INSUFFICIENT_QUESTIONS`（judge.go:43）
- `INSUFFICIENT_REPUTATION` / `AUTH_STATUS_UNAVAILABLE` / `CONFIG_ERROR`（runtime_status.go:17-21，interaction 中间件）
- `CAPTCHA_FAILED` / `CAPTCHA_REQUIRED` / `CAPTCHA_UNAVAILABLE`
- `EMAIL_SEND_FAILED` / `PASSWORD_TOO_SHORT` / `USER_NOT_FOUND`（注册/找回）
- `FILE_TOO_LARGE` / `INVALID_MIME_TYPE` / `OSS_ERROR` / `OSS_NOT_CONFIGURED` / `UPLOAD_GRANT_INVALID` / `UPLOAD_GRANT_UNAVAILABLE` / `UPLOAD_RATE_LIMIT_EXCEEDED`（上传链）
- `READING_TIME_TOO_SHORT`（阅读时长校验）
- `ARCHIVE_*`（restore/scan/quarantine 系列 9 个码，下载/解压链）
- `MISSING_QUERY` / `QUERY_TOO_LONG`（搜索；经 SearchAgentInput **裸英文显示**）
- `AGENT_ERROR`（agent 全链；SearchAgentInput 有降级分支但其它 agent UI 不一定）
- `INVALID_IP_VISIT_MERGE`、`REVIEW_REASON_REQUIRED`、`AMBIGUOUS_ATTACHMENT`/`ATTACHMENT_*`、`CONTACT_EMAIL_REQUIRED`/`FEEDBACK_DELIVERY_FAILED`、`INVALID_CATEGORY`/`INVALID_STATUS`/`INVALID_PRIORITY`/`INVALID_OUTCOME`/`INVALID_ATTACHMENT_ID`、`ALREADY_COMPLETED`/`COURSE_NOT_FOUND`（rehab）、`NO_ATTACHMENTS`/`NO_FIELDS`/`BAD_BODY`/`INVALID_BODY`
- `RATE_LIMIT_UNAVAILABLE`、`REQUEST_BODY_TOO_LARGE`、`COMMENT_EDIT_RATE_LIMIT`、`IDEMPOTENCY_KEY_REQUIRED/REUSED`

admin 侧（页面 fallback 兜底为「操作失败」级文案）：`NO_ALLOWED_FIELDS`、`AUDIT_WRITE_FAILED`、`AUDIT_UNAVAILABLE`、`ACTIVE_CONFIG`、`CONFIG_NOT_FOUND`、`DLQ_UNAVAILABLE`、`DLQ_ENTRY_NOT_FOUND`、`REPLAY_FAILED`、`QUEUE_STATS_ERROR`、`DLQ_ERROR`、`TEST_FAILED`、`RAG_REBUILD_UNAVAILABLE`、`IP_NOT_FOUND`、`APPEAL_NOT_FOUND`、`INVALID_ID`

（i18n parity：zh.json 与 en.json key 集合完全一致 1857=1857，无双边缺失；源码内 CJK 均为注释非 UI 字符串。）

## 三、admin 动作 × 用户通知矩阵（横切补充，live+代码验证）

| admin 动作 | 后端 | 通知用户? | 证据 |
|---|---|---|---|
| appeal resolve（approve/reject） | POST /admin/appeals/:id | ✅（system/appeal_result，标题硬编码中文） | admin.go:415-418 |
| feedback 回复/状态变更 | PATCH/replies | ✅ | feedback_service.go:506 |
| broadcast | POST /admin/notifications/broadcast | ✅（全量） | notification_service.go |
| content ban | POST /admin/contents/:id/ban | ❌ 静默 | admin.go:236-263 |
| content restore | PATCH /admin/contents/:id/restore | ❌ 静默 | admin.go:265-282 |
| user ban / unban | POST /admin/users/:id/ban|unban | ❌ 静默（live 验证 325 无新增通知） | admin.go:284-328 |
| IP approve / reject | POST /admin/ips/:id/* | ❌ 静默（提交者无感知，只能自己在列表看状态） | admin.go:145-197 |
| report resolve/dismiss | PATCH /admin/reports/:id | ❌ 静默（举报者无处理结果反馈） | admin.go:736-773 |
| （对照）PR accept/reject/merge、评论/赞/关注/私信 | — | ✅ | pr_service.go:124/157/213、social_service.go、follow.go、message.go |

## 四、角色矩阵（8 页面 × 4 角色）

后端拦截（curl 实测 2026-09-01；judge 角色以 325 插入 `judge_qualifications(image)` 构造，测后已删）：

| 页面 | 游客 | 普通用户(325) | 判官(325+qual) | admin(313) |
|---|---|---|---|---|
| / （feed） | ✅ 200 | ✅ | ✅ | ✅ |
| /original | ✅ 200（API 200） | ✅ | ✅ | ✅ |
| /content/179 | ✅ 200 | ✅ | ✅ | ✅ |
| /studio/contents（users/me/contents） | 401 UNAUTHORIZED | ✅ 200 | ✅ | ✅ |
| /messages（GET /messages） | 401 UNAUTHORIZED | ✅ 200（空列表） | ✅ | ✅ |
| /judge/queue | 401 UNAUTHORIZED | ✅ 200 但恒空（GetJudgeQueue 按 qualification 过滤，judge_service.go:106-121） | ✅ 200 按类型出案 | ✅ 200（admin 无 qualification 同样恒空） |
| /admin/users | 401 UNAUTHORIZED | **403 FORBIDDEN** | 403 FORBIDDEN | ✅ 200 |
| /appeals（appeals/me） | 401 UNAUTHORIZED | ✅ 200（仅本人） | ✅ | ✅（本人申诉） |

前端守卫形态：
- 游客 → `(protected)/layout.tsx:23-29` 重定向 `/login?redirect=<原路径>`（回跳友好 ✅）。
- 封禁用户 → `(protected)/layout.tsx:41-49` 全屏 suspended 页 + 申诉链接（✅ 闭环仅存的一环，但依赖用户主动进入 protected 页）。
- 非 admin 访问 /admin/* → `admin/layout.tsx:41-45` replace('/') + accessDenied 面板（72-82 行，有文案无提示形式突变 ✅）。
- 非判官访问 /judge/queue → 可进入，显示「暂无待审内容」+ 考试 CTA——文案对非判官有误导（真实原因是无资质，见 E-17 相邻问题）。
- 提示形式：后端 401/403 code=UNAUTHORIZED/FORBIDDEN；UNAUTHORIZED 有映射（auth.errorSessionExpired），FORBIDDEN 落死映射（error.FORBIDDEN 无人消费）→ 普通用户直敲 admin API 时前端显示通用失败文案。

## 五、空态清单（横切 4）

| 页面 | 空态形态 | 引导动作 |
|---|---|---|
| studio/contents | EmptyState（icon+title） | ✅「开始创作」CTA（page.tsx:81-86） |
| feedback/mine | EmptyState+desc | ✅ 提示去提交（mine/page.tsx:102） |
| history | EmptyState | — |
| appeals（用户） | EmptyState（icon） | ❌ 无「新建申诉」引导（顶部有按钮，可接受） |
| judge/queue | 文案 + 按钮 | ✅「参加资格考试」CTA（queue/page.tsx:118-127） |
| messages | ConversationList 内置 title/desc | ❌ 无「找人」引导 |
| admin/contents | EmptyState（icon+hint） | — （标杆） |
| admin/appeal / ips / users | 裸 `<p>` 文案 | ❌ 无图标无引导（不一致） |
| admin/reports / audit-logs / queue | icon + 文案 | — |
| admin/feedback | （有筛选，未详列，形态同 reports） | — |
| /studio/revenue | coming-soon 占位 | — |

新用户首访路径（游客注册后）：studio 空 → 有 CTA ✅；history 空 → 有浏览引导文案；messages 空 → 无引导 ⚠。

## 六、防重复提交抽查（横切 7）

- ✅ 良好：PublishForm（submitting 全按钮禁用，PublishForm.tsx:244,500-823，还排除上传中提交）、CommentSection（busy，105/115 行）、appeals 表单（submitting）、broadcast（sending + Idempotency-Key，notifications/page.tsx:76-110 —— 全站最佳实践）、ConfirmModal（busy + requireReason 非空禁用，confirm-modal.tsx:169）、admin users/contents/appeal（busy）、categories（busy）、feedback（replyBusy/patchBusy）。
- ⚠️ 缺失：admin ips handleAction 无 busy/禁用（可双击）；agent-config 的 activate/test/delete 无 busy 禁用（低风险）。
- ⚠️ reports 页 onConfirm 异常被 ConfirmModal 吞且页面无展示（E-01 关联）。

## 七、分页与长内容（横切 6）

- 参数语义一致（page/page_size，后端均默认 20、上限 100 校验）✅；控件风格两派（见 E-18）；DataList load-more（appeals/history/studio contents）与分页按钮并存，可接受。
- judge/queue 无分页且固定 20（E-17）。
- 长标题：ContentCard `line-clamp-2`/`truncate`（ContentCard.tsx:156-195,258-274）✅；admin 表格 `max-w-[200px] truncate`（appeal:142、contents:129）✅。
- 评论：两级扁平结构（top-level + 直接回复，CommentSection.tsx:149-154），无深嵌套渲染风险 ✅。
- admin users 列表无 ORDER BY（E-11）。

## 八、附录：验证记录（关键 curl/psql 摘要）

1. 登录：admin seed-ui-001 与 seed-ui-013 均 200，token 存 /tmp/E-*-token.txt。
2. `PATCH /api/v1/admin/reports/999999` body `{"resolution":"upheld","explanation":"test"}` → 400 VALIDATION_ERROR "status is required"。
3. `GET /api/v1/admin/config` → limits/judge/reputation/social 均为 PascalCase（`VideoMaxMB`、`MinVotesRequired`、`PassThreshold:0.6`、`RepeatViolationExtraPenalty:-1`）；`GET /api/v1/config/public` → snake_case 且 desktop_deploy/download=false。
4. `POST /api/v1/admin/users/325/ban {"reason":"audit-test temporary"}` → 200；user_id=325 notifications 计数 ban 前后均 10（无通知）；`POST .../unban` → 200，`is_banned=f` 恢复。
5. `POST /api/v1/admin/contents/999999/ban` → 200 "content banned"（假成功，见 E-09）。
6. `POST /api/v1/admin/appeals/999999` → 404 APPEAL_NOT_FOUND；空 body → 400 VALIDATION_ERROR。
7. 角色矩阵 curl 结果见第四节（guest/user/judge/admin × 8 端点）。
8. psql：`content_items` 状态分布 published=170/pending=21/under_review=13/banned=9/draft=6；banned 9 条 ban_reason 全空；`appeals` 0 行、`reports` 0 行、`ai_review_records` 2 行（pass）；`notifications` 总 240。
9. judge 构造：`INSERT INTO judge_qualifications(user_id,content_type) VALUES (325,'image')` → GET /judge/queue 200 空案 → `DELETE`（已清理，验证 SELECT 确认）。
10. i18n：python 扁平化 diff zh/en = 双向 0 差异（各 1857 keys）。

## 九、修复优先级建议（供排期，非本任务执行）

P0：E-01（举报处理不可用）。
P1：E-02（config 契约+污染）、E-03（ban reason 丢失）、E-04（作者可知性+死按钮）、E-05（通知矩阵）、E-06（后台入口）、E-16（错误码映射+裸英文）。
P2：E-07、E-08、E-09、E-10、E-11、E-12、E-13、E-14、E-15、E-17。
P3：E-18、E-19（除死按钮归 E-04）、E-20。
