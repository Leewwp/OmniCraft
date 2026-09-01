# OmniCraft 设计审计 Phase 5：独立复核结论与修复方案（fix-plan）

> 创建日期：2026-09-01 ｜ 预计失效日期：2026-11-01
> 输入契约：`docs/working/2026-09-01-design-audit-findings.md`（F-001~F-096，96 条）
> 复核方法：未采信任何前序审查引用，逐条重读代码（grep + Read）并按需运行时复现（curl/psql 只读为主；唯一一次写测试为 F-044 假目标申诉，已删除并确认）。复核者未参与任何审查阶段。
> 项目语境：第一阶段（简历/面试最小实现，本地 live demo），目标窗口 2026-09-03。方案以「本地可验证、不引入大型新依赖、不需要生产部署」为现实性准绳。

---

## 0. Phase 6 执行前复核与修订记录（2026-09-01，主会话）

> 方法：5 个全新域子代理（A 账号/发布/审核/申诉、B IP/PR/判官、C 社区/通知/反馈、D 收藏/搜索/Agent、E Admin/横切）在运行环境（backend :8080 / frontend :3000 / pg / redis 在线）逐条重验全部 96 条发现（代码亲读 + curl/psql/redis 复现 + F-042 真浏览器 Playwright 复验），并核查本方案每条承重声明、扫描遗漏。基线 main@f9d9ef7（本文档写就后合入 774994f 媒体签名 / 6d1d781 IP tags / fecef1b IP 发布入口 / f9d9ef7 docs）。零业务代码修改，运行时写操作均已恢复。**本节与各 FIX 条目内「Phase 6」标注为最新结论；未标注的正文维持 Phase 5 原文。**

### 0.1 裁决统计（96 条）：证实 90 ｜ 修正 5 ｜ 驳回 1

| 编号 | 变化 | 结论 |
|---|---|---|
| F-011 | 修正（轻） | 死按钮结论不变；但旧 dashboard/contents 孤儿页（零入口+字段损坏）存在一个 DELETE /contents/:id 调用，「全前端无 DELETE 调用」表述不准 |
| F-022 | 修正=部分已修复 | fecef1b 已交付创建入口+表单（/studio/publish/ip、IPPublishForm 实调 POST /ips、/ips 头部按钮、pending 成功面板、ui-spec 新节）；仍缺 GET /users/me/ips、「我的 IP」状态页、过审/驳回通知、拒绝原因透出 → FIX-23 重界定 |
| F-025 | 修正（范围收窄） | 代码结论全证实；但「compose 无 worker」「lean 只跑 server」「demo 缺步骤」三条不实：根 docker-compose.yml:290 已有 worker 服务（非 profile 门控）、interview-lean compose 与 scripts/live-demo.sh:195-198 均含 worker。真实缺口=AGENTS.md Step 2 host-run 路径+积压清理 → FIX-09 收缩 |
| F-042 | 修正（分歧收口） | 真浏览器（Playwright Chromium 1440×900）实测：封禁登录 403 后 `[role=alert]` 显示「账号已被封禁，请联系管理员」——文案链路端到端工作，Phase 5 修正最终证实；审计截图应为提交前/重访表单态（登录错误链 5 文件审计后 0 改动）。残留=无申诉/工单指引（并入 FIX-15） |
| F-083 | 修正（维持 Phase 5 修正） | 无竞态+public-config 缓存根因双域独立确认；补：AgentFeatureGate fetch 失败走 fail-closed 也呈现「unavailable」，5min TTL 不覆盖该路径（FIX-39 补错误态重试） |
| F-082 | 驳回（当前不可复现） | Playwright 375×812 匿名/登录态 scrollWidth==clientWidth==375（overflow=0）；/original 组件审计后 0 改动；审计自存截图亦无页面级横向滚动条。不排除 IAB/WebKit 特有，如需闭环在 Safari 手工复测 → FIX-40① 降级为防御性可选 |

轻微勘误（裁决不变）：F-016（judge/queue 实际返回完整案件行含票数，非「只返回 target_type/target_id」，仍无内容本体）；F-039（error.* 大写死码 15 个非 16；error.pageError/pageErrorDesc/notFound/notFoundDesc/backHome 5 个 key 在用）；F-096（i18n 已随新 commit 变为 zh=en=1882，parity 保持）；F-035（DB 10 条 banned 中 1 条已有 reason[审计期 API 测试]，UI 丢弃链不变）。

### 0.2 新增遗漏发现（F-097~F-117，21 条：P0×1、P1×3、P2×11、P3×6）

完整条目见 findings 文档「Phase 6 增补」；摘要与修复归属：

| 编号 | 级 | 摘要 | 修复归属 |
|---|---|---|---|
| F-097 | P1 | 主列表与 /users/:id/contents 不过滤 is_public，私密已发布内容公开可见（实测 id=2/1034） | FIX-12+43 合并 |
| F-098 | P2 | 封禁屏替换 children——FIX-15 白名单落地后 /appeals 仍不渲染 | FIX-15 补第 5 步 |
| F-099 | P3 | HasPendingAppeal 错误被吞 → 同 target 双 pending（表无 UNIQUE 兜底） | FIX-27 顺带 |
| F-100 | P1 | 判官考试题泄露答案：question_data 仅删 correct_key 单键，调度器题内嵌 votes_approve/reject 可机械推答案；explanation 明文下发 | FIX-36 扩展（allowlist） |
| F-101 | P2 | IP tags 免审核自由文本（6d1d781 引入） | FIX-23 加固⑥ |
| F-102 | P2 | IP tags 服务端无条数上限 | FIX-23 加固⑦ |
| F-103 | P2 | POST /ips 无 interaction guard/限流（fecef1b 使其可达后升级） | FIX-23 加固⑤ |
| F-104 | P3 | 判官撤权阈值硬编码 0.6 vs 闭案读 PassThreshold | FIX-03 补 |
| F-105 | **P0** | email 经 Author Preload 全站泄露（model/user.go Email json 未隐藏；内容列表/详情、讨论、评论、搜索、系列全出口，实测） | FIX-19 升级（model 层） |
| F-106 | P2 | 讨论列表/详情无 status 过滤（潜伏） | FIX-18 顺带 |
| F-107 | P3 | 讨论嵌套回复 API 可建、永不展示 | FIX-18/29 顺带 |
| F-108 | P3 | 内容评论固定 page_size=50 无分页 | FIX-29② 顺带 |
| F-109 | P1 | Agent SSE 前端无跨 chunk 行缓冲，事件静默丢弃（丢 delta/引用/done） | **FIX-44（新增）** |
| F-110 | P2 | 原创页热搜展示名为静态 i18n 演示标签（假榜单） | FIX-06 前提② |
| F-111 | P2 | 深色主题 primary 按钮系统性 AA 未达（4.47:1；hover 2.98:1；123 处） | FIX-05 扩展（同根裁决） |
| F-112 | P2 | hot rank ZSet 现有 member 全为 banned/under_review 内容，join 不过滤即泄漏 | FIX-06 前提① |
| F-113 | P2 | admin DLQ 卡片字段契约断裂（topic/retry 恒 undefined，后端给 original_topic/attempts） | FIX-35 必补 |
| F-114 | P2 | ResolveReport 无存在性校验（假成功 200+审计行） | FIX-34 清单补 |
| F-115 | P2 | config 页 extra_penalty min={0}：FIX-33 补 tag 后 -1 触发 :invalid 阻断全部保存 | FIX-33 必补 |
| F-116 | P3 | ResolveReport「成立」不处置被举报目标（语义缺口） | FIX-04 注记+产品定夺 |
| F-117 | P3 | admin/dashboard 同样 .catch(()=>null) 吞错 | FIX-35⑥ |

操作性提醒：notification stream 现有 9 条积压（含 user_id=0 与审计期测试通知）——FIX-09 启动 worker 前先做 FIX-31 的 follow.go Notify(0) 删除并清理积压（XTRIM/DEL+重建 group），否则全部物化为演示噪音。

### 0.3 修复方案修订总表

- **数值错误（必改）**：FIX-05——「bg-primary 双主题 6.29:1」仅 Light 成立；Dark --primary=#6366F1 白字 **4.47:1 未达 AA**（Phase 6 复算）。修订见该条；F-111（暗色按钮同根）并入裁决。
- **修法方向错误（必改）**：FIX-30①——后端冷启动规则本就允许每方向首条消息，且该 guard 是 messages 路由组唯一防骚扰机制（无独立 DM 频控），「放宽 guard」会拆掉防骚扰门；①改纯前端入口、后端不动。
- **P0 范围遗漏**：FIX-19——只修 sanitizeUser 挡不住 Author Preload 出口（F-105），升级为 model 层收敛；验收=「任意端点不含他人 email」。
- **与已合入代码过时冲突**：FIX-23（fecef1b 已交付创建侧，重界定剩余范围+⑤⑥⑦加固）；FIX-09（②③为已完成项重复施工，收缩为 AGENTS.md Step 2+积压清理）。
- **执行前提缺失（必补，否则修完仍不可用/锁死）**：FIX-06（join 过滤可见性+禁用假榜单标签）；FIX-12/43（is_public 维度）；FIX-15（layout 放行 /appeals）；FIX-29（与 FIX-19 同批；折叠计数走 reactions 聚合）；FIX-33（放宽 min={0}）；FIX-34（补 ResolveReport）；FIX-35（DLQ 字段先对齐）；FIX-36（GetExam 改 allowlist）。
- **内部矛盾（已改）**：§1.3 执行总序原把 FIX-14 排在其前置 FIX-12/16 之前，已按依赖序重写。
- **事实性小修**：FIX-13（PATCH 契约无 description，复审范围=title+cover；封面校验按「平台 OSS 域/key」表述）；FIX-26（error.* 死码 15 个+5 个在用 key 须保留）；FIX-01（e2e 无 answer_key 同构数据，风险注作废）；FIX-22（e2e 无 /dashboard 引用；SubmitPREntry 链接一并改）；FIX-04（status 合法值 resolved|dismissed 实测确认正确）。
- **新增**：FIX-44（Agent SSE 跨 chunk 行缓冲，F-109，P1）。**降级**：FIX-40①（F-082 驳回后为防御性可选）。
- **复核通过无需改动**：FIX-01/02/03(主体)/04(主体)/07/08/10/11/14/16/17/18(主体)/20/21/24/25/27(主体)/28/31/32/37/38/39(主体)/41/42 及全部不修项——承重声明经独立复核属实。

**Phase 6 后修复项总数：44（P0×7、P1×21、P2×13、P3×3）。**

---

## 1. 总览

### 1.1 复核统计

**证实 94 / 修正 2 / 驳回 0（合计 96）。**

修正项（问题在，但描述或根因需更正）：

| 编号 | 原描述 | 复核后正确版本 | 反证/证据 |
|---|---|---|---|
| F-042 | 被封禁账号登录 403 后页面无任何错误提示 | 错误文案路径**存在且完整**：`frontend/app/(public)/login/page.tsx:40` setError + `:112-116` 渲染；`frontend/lib/user-facing-error.ts:9-10` 已映射 USER_BANNED → `auth.errorBanned`；`frontend/messages/zh.json:97` 文案存在；`silentError`（`frontend/lib/error-handler.ts`）只记录不吞错。审计截图呈已登录其他账号的表单态，无法在本复核中复现「无提示」（子代理无浏览器）。**成立的部分**：文案仅「请联系管理员」，无申诉/工单指引链接——并入 FIX-15 | login/page.tsx:38-44,112-116；user-facing-error.ts:9-10 |
| F-083 | /agent gate 判定时机/条件不一致（竞态） | `AgentFeatureGate.tsx` 判定为 `loaded → allowed` 顺序执行，**无竞态**。真实根因：`frontend/lib/public-config.ts:50-52` `cachedConfig` 模块级缓存**永不失效**——运行时经 PATCH /admin/config 切换 `web_agent_enabled` 后，已加载页面前端视图与新会话不一致，造成审计观测差异 | AgentFeatureGate.tsx:28-52；public-config.ts:50-60；`GET /config/public` 实测当前 false |

运行时独立复现（本复核新增证据，均为只读或自建后清理）：

| 发现 | 复现 | 结果 |
|---|---|---|
| F-006 | 匿名 `GET /api/v1/users/334` | 200，响应含 `email` 字段 |
| F-009 | 匿名 `GET /api/v1/contents/5`（DB 状态 banned） | 200，返回全量正文 |
| F-024 | `GET /api/v1/stats/summary` vs DB `SELECT status,COUNT(*) FROM ips` | API ips:0 vs DB approved:16 |
| F-034 | admin `GET /api/v1/admin/config` | 键为 PascalCase（VideoMaxMB/PassThreshold=0.6/RepeatViolationExtraPenalty=-1），前端 defaultConfig 为 snake_case 且 extra_penalty=+1 |
| F-044 | 登录 seed-ui-002 后 `POST /appeals {target_id:999999}` | 201 创建成功（测后已 DELETE，appeals 表仅剩 1 条审计期遗留行） |
| F-048 | `GET /api/v1/contents?page_size=100` 查 1032 | banned 作者 9202 的 published 内容在公共列表 |
| F-059 | `GET /api/v1/social/comments?content_item_id=179` | `author:{id:0,username:"",...}` 零值对象 |
| F-067 | `GET /api/v1/search/trending?limit=3` | `{"trending":[{"text":"5","score":1}]}`（member 是内容 ID），前端读 `data.items` 永不匹配 |
| F-008 | admin `PATCH /api/v1/admin/reports/999999`（前端同构 payload） | 400 `{"code":"VALIDATION_ERROR","message":"status is required"}` |
| F-007/F-094 | WCAG 对比度独立重算 | 用户气泡 Light #FFFFFF/#F5F5F5=**1.09**；引用卡 Dark #6366F1/#0D1117=**4.24**（修复值 #818CF8=6.34 验证通过） |

### 1.2 严重度重定级（裁决仍为证实，仅定级调整）

| 编号 | 原级 → 终级 | 理由 |
|---|---|---|
| F-040 | P2（原 P1） | admin 运维页，影响面小于用户侧 |
| F-049 | P3（原 P2） | `archive_malware_scan_enabled=false`，本地链路不可达，纯推演 |
| F-051 | P3（原 P2） | BanIP 死代码无路由，本地 demo 无法触达 |
| F-069 | P3（原 P2） | 纯视觉冗余，无功能影响 |
| F-071 | P3（原 P2） | 增强型缺口（usage/trace 展示），非缺陷 |
| F-083 | P3（原 P2） | 见 1.1 修正表；flag 切换场景才可见 |

F-007 维持 P0：agent 是简历主线能力，demo 开启 gate 后核心交互不可读，且修复成本一行。

### 1.3 最终优先级全景与批次划分

合并共享根因后 **43 个修复项**：**P0×7、P1×20、P2×13、P3×3**。

| 批次 | 定位 | 修复项 | 建议顺序理由 |
|---|---|---|---|
| **A 快赢批次** | 一行~半天级，解锁演示主线，全部可本地验证 | FIX-01~FIX-09 | 无相互依赖，先做 P0 三件（考试字段/reports 契约/对比度） |
| **B 审核链路批次** | 状态机出口、可知性、申诉出路（heavy 为主） | FIX-10~FIX-20 | FIX-15/19 独立可先行；FIX-10→16→17 有依赖序 |
| **C PR/IP 协作批次** | PR 闭环 + IP 生命周期 | FIX-21~FIX-25 | FIX-21（后端语义）先于 FIX-22（前端入口） |
| **D 触达与反馈批次** | 错误映射、申诉质量、评论/私信、通知细节 | FIX-26~FIX-32 | FIX-09（worker）是 FIX-17/28/31 的触达前提 |
| **E admin/运维与打磨批次** | admin 防护、判官体验、配置化、UI 打磨 | FIX-33~FIX-43 | 风险低，可穿插；面试窗口后仍可继续 |

执行总序建议（**Phase 6 修订**：原序把 FIX-14 排在其前置 FIX-12/16 之前，自相矛盾，已按依赖序重写）：**A（FIX-01 先于 FIX-03）→ B(FIX-19, 15 → 16 → 17 → 12 → 14 → 10 → 11 → 13 → 18 → 20) → C(FIX-21 → 22；FIX-24 → 23 → 25) → D → E（穿插）**；若 2026-09-03 窗口紧张，最小演示集 = A 全部 + FIX-19（含 F-105 邮箱全站泄露面）+ FIX-15 + FIX-16/17/12 + FIX-14 + FIX-10 + FIX-22（studio 收口）+ FIX-33（防配置污染）——FIX-14/10 依赖 FIX-12/16/17，无法再裁剪。

### 1.4 96 条裁决台账（编号 / 裁决 / 本复核自证证据）

> 完整代码引用均为本复核亲自定位；仅列关键锚点。

- F-001 证实 — middleware/auth.go:100-104（全 authReq 拦 banned）；handler/auth.go:233-236（登录 403）；AuthContext.tsx:146-151（fetchMe 失败清 user）；(protected)/layout.tsx:42-53（封禁屏依赖 user 非空，不可达）；appeal.go:31（oneof=content comment 无 account）
- F-002 证实 — handler/user.go:266-283（GetMyContents 无 Status）+ content_repo.go:156-160（默认 published）；studio/contents/page.tsx:100-118（无状态徽标）
- F-003 证实 — judge_service.go:148-170（闭案仅 CloseCase）；全库无 closed→content 映射
- F-004 证实 — judge_service.go:46（`json:"answer"`）vs judge/exam/page.tsx:95（`answer_key`）
- F-005 证实 — 同 F-003 事实；business-rules.md:160 承诺「≥60% 恢复展示」
- F-006 证实 — 运行时（见 1.1）；user.go:439-453 sanitizeUser 无条件返回 email
- F-007 证实 — AgentWorkspace.tsx:575 `ml-auto bg-accent text-white`；对比度自算 1.09
- F-008 证实 — 运行时 400；admin/reports/page.tsx:175-179 发 resolution/explanation；admin.go:742-749 要求 status；confirm-modal.tsx:61 catch 吞错
- F-009 证实 — 运行时 anon 200 banned；content.go:302-322（FindByID 无状态过滤 + 无条件 IncrViewCount）
- F-010 证实 — content_service.go:898-926（无状态检查、无重审；published 仅发 RAG updated 事件）
- F-011 证实 — studio/contents/page.tsx:110-116（死按钮）；全前端无 PATCH/DELETE /contents/:id 调用
- F-012 证实 — routes.go 无 reports/me；admin.go:736-773（ResolveReport 不通知）；无评论举报 UI 调用
- F-013 证实 — model/content.go 无 BanReason（列存在于 040 迁移）；前端 ban_reason 0 消费
- F-014 证实 — user.go:347-358（注销=is_banned true + self_deleted + 邮箱改写 + 密码随机化）
- F-015 证实 — social_service.go:296-307（auto-hide 仅 UpdateContent，无 ensureJudgeCase；对比 review_service.go:343-353）
- F-016 证实 — ReviewCard.tsx:71-75（占位文案）；judge.go:86-100（queue 只返回 id）；judge_repo.go:72-79（ListOpenCases 无已投过滤）
- F-017 证实 — pr_service.go:35-40,64-70（NewText 丢弃，ProposedVersionID 恒空）
- F-018 证实 — SubmitPREntry.tsx 0 引用；全前端无 POST /api/v1/pr 调用
- F-019 证实 — pr_service.go:110（accept 零内容效果）+ :175-177（merge 要求 open，accept 后 400）
- F-020 证实 — studio/pr-requests|contributors|tag-suggestions 均 18-19 行空壳；通知路由 NotificationDropdown case "pr"→/studio/pr-requests
- F-021 证实 — judge_service.go:185（`!= "closed"` 永假）+ VerdictDetail.tsx:129（`=== "closed"` 永假）
- F-022 证实 — POST /api/v1/ips 无 UI 调用（仅 e2e contract 测试）；无「我的 IP」页
- F-023 证实 — admin/ips/page.tsx:195-199（`_reason` 丢弃）；admin.go RejectIP 无 body 解析；ip_review_logs 无写入方
- F-024 证实 — 运行时（见 1.1）；stats_service.go:46
- F-025 证实 — container.go:274-283（ADR0005：API server 永不起消费者）+ notification_service.go:274-290（Notify 入队）+ config.yaml queue.enabled=true + AGENTS.md Step 2 仅启 server
- F-026 证实 — 与 F-001 同根三重断裂（复核实测路径同 F-001 代码链）
- F-027 证实 — discussion.go:44-68（无 ensureCanInteract、无 moderateText）；social_service.go:155-175 对照
- F-028 证实 — discussion.go:89-116（无审核、无通知；social_service.go:98-110 对照）
- F-029 证实 — grep content_status 后端/前端 0 使用；review_service.go:325-353、admin.go:236-282、judge_service.go:148-170 均无作者通知
- F-030 证实 — studio 三空壳（见 F-020）+ dashboard/contents/page.tsx:16-22（PascalCase interface vs model/content.go 全小写 json tag）
- F-031 证实 — studio/overview/page.tsx:32（setPendingTasks 声明未调用）+ tag_service.go:45-60（无通知）
- F-032 证实 — SubmitPREntry.tsx:28 链接目标不读参数（=F-018 合并）
- F-033 证实 — HomePageClient.tsx:154 / OriginalSidebar.tsx:47 均链 /studio/contents
- F-034 证实 — 运行时（见 1.1）；config.go:255+ 无 json tag；admin.go:468-530 PatchConfig 无 judge 分支
- F-035 证实 — admin/contents/page.tsx:188-194 丢 `_reason`，banContent 发 `{}`（:65）
- F-036 证实 — appeals/page.tsx:122-127（type=number 手填 target_id）；studio 无状态渲染（=F-002/F-011 合并）
- F-037 证实 — admin.go 仅 ResolveAppeal:415-418 与 feedback 发通知；BanContent/Restore/BanUser/Unban/ApproveIP/RejectIP/ResolveReport 0 处（grep Verify）
- F-038 证实 — Header.tsx 无 /admin 项；ADMIN_NAV（admin/layout.tsx:16-29）12 项无 /admin/notifications
- F-039 证实 — user-facing-error.ts:6-31 仅 12 码；`t("error.*` 0 消费；SearchAgentInput.tsx:67 裸显 e.message
- F-040 证实 — queue/page.tsx:43-44 `.catch(()=>null)`；无 replay 按钮（grep 0）
- F-041 证实 — admin.go:265-282（restore 不清 deleted_at）；前端 trash/restore 0 调用
- F-042 **修正**（见 1.1）
- F-043 证实 — interaction.go:74（PUBLISH_FROZEN）；PublishForm.tsx:421-422（catch 吞 code）；无解冻代码路径
- F-044 证实 — 运行时 201 假目标（见 1.1）；appeal.go:28-57 无目标校验；appeal_helpers.go:7-18
- F-045 证实 — invalidateContentCache 仅 content_service.go:922/955（作者路径）；admin/social/appeal 路径 0 调用
- F-046 证实 — auth.go:112-117（TTL 复用 resend_cooldown 60s）+ :119-137 门；login/page.tsx 全文无 captcha；config.yaml credential_per_minute:5
- F-047 证实 — api.ts:195 只认 TOKEN_EXPIRED；后端过期返回 UNAUTHORIZED（middleware/auth.go:44）
- F-048 证实 — 运行时（见 1.1）；content_repo.go:152-161 主分支无作者封禁过滤（对比 content_visibility.go:7-19）
- F-049 证实（降 P3）— zh.json grep "scan"=0；flag=false 本地不可达
- F-050 证实 — review_service.go:453-468（IP ban 级联批量置 banned，无通知无 ban_reason）
- F-051 证实（降 P3）— BanIP/BanIPAndContents 仅 service/repo 定义，0 调用方
- F-052 证实 — ip.go:91-109（GetIP 无过滤）+ admin.go:65 `NewIPService`（无 redis）→ ip_service.go:234-238 invalidate 空转
- F-053 证实 — judge_service.go:77-78（total=len(input.Answers)，单题可过）+ :55（提交重新抽题）
- F-054 证实 — judge.go:124-136（裸 votes）vs VerdictDetail.tsx:25-35（judge_name/upvotes）；VoteReason:138-166 无资格/自赞校验
- F-055 证实 — dashboard/contributors/page.tsx:36-56（前端拼装、blocked 恒 false）
- F-056 证实 — pr_service.go:186-202（merge 只建版本）+ handler/content.go GetContent 不读版本；routes.go:111 /versions/:id optAuth
- F-057 证实 — 同 F-002（C 域独立证实，合并）
- F-058 证实 — comment_fold_threshold 仅 config.go + admin.go:519-521，业务链路 0 消费
- F-059 证实 — 运行时（见 1.1）；social_repo.go:38-51 无 Preload（对比 :53-69 有）
- F-060 证实 — CommentSection.tsx:149,154（子回复取自顶层数组恒空）；无回复按钮
- F-061 证实 — 前端 0 处 PATCH/DELETE /social/comments
- F-062 证实 — message.go:124（全文入 body + system channel）；双未读通道独立
- F-063 证实 — ChatWindow.tsx:64-67（仅挂载加载，无轮询/SSE）；recipient_id 仅 ChatWindow 引用
- F-064 证实 — follow.go:72 `Notify(0, ...)`
- F-065 证实 — feedback_service.go:428-435（仅 closed/reopened）+ :651 英文文案
- F-066 证实 — user.go:210-236（任意 id 可查处罚日志）；/reputation-logs/me 前端 0 消费
- F-067 证实 — 运行时（见 1.1）；search.go:46-57 vs OriginalSidebar.tsx:22-34
- F-068 证实 — tag_service.go:101-124（ApproveTagSuggestion 无 reputation 调用）
- F-069 证实（降 P3）— series/page.tsx:241+256、favorites/page.tsx:190+209 同屏双 CTA
- F-070 证实 — agent-stream.ts:133（错误码丢弃）；AgentWorkspace.tsx:417-420 统一文案
- F-071 证实（降 P3）— AgentWorkspace.tsx switch（310-380）无 start/usage case
- F-072 证实 — components/content/{ComplianceCheckBadge,UploadAssistPanel} import 0 引用
- F-073 证实 — admin.go:253-262 Updates 无 RowsAffected 检查（999999→200）
- F-074 证实 — admin.go:284-308 无角色/自身/存在性防护
- F-075 证实 — admin.go:330-345（无 search/ORDER BY）；users/page.tsx 客户端过滤当前页
- F-076 证实 — admin appeal 页仅 user_id/target_id 裸列；ai_review_records 无暴露；ResolveAppeal 无 pending 守卫
- F-077 证实 — appeals/page.tsx grep admin_response=0；admin.go:417 标题硬编码中文
- F-078 证实 — agent-config/page.tsx:115-124（test 仅 ok/fail）；admin.go:679-696（activate 一律 404）
- F-079 证实 — audit-logs/page.tsx:92-108 固定 15 项，缺 llm_config_*/dlq_replay/rag_rebuild/broadcast_notification
- F-080 证实 — judge/queue/page.tsx:43（固定 page=1&page_size=20）
- F-081 证实 — appeal_helpers.go:12-16（comment approved 无映射）→ admin.go:396-399 提前返回
- F-082 证实 — 根因锁定：components/layout/Sidebar.tsx:107-109 `flex-shrink-0 w-[228px]` 无响应式隐藏（original/page.tsx:82 直接渲染）；375px 下主内容被挤压至 ~147px
- F-083 **修正**（见 1.1）
- F-084 证实 — DOM 面与 F-033 同源（合并）
- F-085 证实 — type_order 配置 0 消费；前端类型清单硬编码漂移
- F-086 证实 — PublishForm.tsx:434 硬编码 maxMB；服务端超限报通用 VALIDATION_ERROR
- F-087 证实 — 同 F-077 用户面（合并）
- F-088 证实 — 同 F-025 环境面（合并）
- F-089 证实 — CreateInitialVersion/CreateNewVersion 0 调用方；版本仅 PR merge 产生
- F-090 证实 — IPBrowseClient.tsx:60 `most_contents` vs ip_repo.go:97 `most_content`；admin ips submitter_id vs creator_id
- F-091 证实 — AwardJudgeAccuracy/AwardPRMerged/PenalizeJudgeError 0 调用方
- F-092 证实 — NotificationList.tsx:249-253（discussion→/content/{id}）；NotificationDropdown.tsx:79 定义未用（:136,:177 一律 /messages）；AuthContext.tsx 无 broadcast 键；轮询 30s vs 文档 5min
- F-093 证实 — discussion.go:133（admin 无置顶权）；social.go:79（删他人评论 400 ERROR 风格 vs 编辑 403）；social_service.go:215（dislike 也「新的赞」）；followers API 0 UI；messages/page.tsx:30 count:0
- F-094 证实 — AgentCitationCard.tsx:26 + globals.css:197（dark #6366F1）；自算 4.24
- F-095 证实 — suggestions 0 消费 + 前缀 ILIKE；users/search 仅 CollabUserPicker；SearchAgentInput.tsx:5,132 ArrowLeft；AgentWorkspace 裸文本渲染
- F-096 证实 — series/page.tsx:240 页头 h1 + :273 详情面板 h2 重复（+副标题）；Active IPs 0 视觉呈现（=F-024）；i18n/routing.ts defaultLocale zh + localePrefix never（Accept-Language 协商，英文浏览器得 en——产品决策项）

---

## 2. 修复项（FIX-01 ~ FIX-44；FIX-44 为 Phase 6 新增，排于批次 D 主题之后）

> 车道规则（AGENTS.md）：涉及安全、审核链路、封禁/申诉、数据库迁移、auth、生产配置的一律 heavy（TDD + 两阶段审查）。

### 批次 A：快赢批次

#### FIX-01 判官考试前后端字段断裂
- **问题背景**：[A] P0；F-004
- **场景**：任意用户 → /judge/exam 全部答对提交 → score 0 / passed false（预期：≥80% 通过并授予资格）。判官体系从 UI 完全无法进入。
- **复现**：GET /api/v1/judge/exam/article（需题库≥10）→ 前端同构 payload `{answers:[{question_id, answer_key}]}` POST /judge/exam/submit → score 0；改发 `answer` 字段 → score 10。
- **根因**：前端 `frontend/app/(protected)/judge/exam/page.tsx:95` 发 `answer_key`；后端 `backend/internal/service/judge_service.go:46` 只绑定 `answer`（字段静默为零值，评分恒 0）。
- **修复方案**：前端改发 `answer`（保持后端单一契约）。i18n 无新增。可选加固：后端对未知字段 `answer_key` 出现时 400（防御性，非必须）。
- **优先级与顺序**：P0，无依赖，最先做。
- **建议车道**：heavy（判官=审核链路准入；改动极小但按规则走两阶段审查，可与 FIX-03 同 worktree 分批 commit——注意 heavy 一任务一 commit）。
- **验证标准**：新增/更新 judge_service 测试（answer_key 旧 payload 不再可能出现——前端 e2e 或 contract 测试覆盖提交路径）；`go test ./... && npm run build`；浏览器实测考试提交得满分并通过。
- **风险与回归面**：无数据迁移；注意 e2e mock（`frontend/e2e/`）里如有 answer_key 同构数据需同步（**Phase 6：grep 实测不存在，此注作废**）。

#### FIX-02 stats IP 统计状态词错误
- **问题背景**：[A] P1；F-024
- **场景**：访客 → 首页 → 「Active IPs」恒 0（DB 实际 16 approved）。
- **根因**：`backend/internal/service/stats_service.go:46` `Where("status = ?", "published")`；IP 状态机词表是 pending/approved/rejected/banned（model/ip.go:14）。
- **修复方案**：改为 `approved`；无需迁移；Redis `stats:summary` 键自然过期（可顺手 `DEL` 一次验证）。
- **优先级与顺序**：P1，无依赖。
- **建议车道**：light。
- **验证标准**：stats_service 单测（approved 计入、pending/banned 不计入）；清缓存后 `GET /api/v1/stats/summary` ips=16；首页视觉验证。
- **风险与回归面**：无（对照 ip_stats_service.go:31-37 用内容状态 published 是正确语义，勿改）。

#### FIX-03 判官 closed 词表统一 + 撤权激活 + 结案横幅 + 准确率奖励接线
- **问题背景**：[A] P1；F-021、F-091
- **场景**：①判官累计误判 >50% → 永不撤权（`judgeCase.Status != "closed"` 恒真跳过）；②判官投票后 verdict 页「结案结果」横幅永不显示；③判官 +1 准确率奖励（business-rules.md:27）无调用点，用户零感知。
- **根因**：实际终态是 `closed_approve/closed_reject`（judge_service.go:157-159）；`judge_service.go:185` 与 `frontend/components/judge/VerdictDetail.tsx:129` 都匹配裸 `"closed"`；`reputation_service.go` 的 AwardJudgeAccuracy/AwardPRMerged/PenalizeJudgeError 0 调用方。
- **修复方案**：
  1. judge_service.go:185 改 `!strings.HasPrefix(judgeCase.Status, "closed_")`；
  2. VerdictDetail.tsx:129 改 `caseData.status.startsWith("closed_")`；
  3. 闭案事务提交后，对与多数派一致的在册判官调 `AwardJudgeAccuracy(judgeID, +1, ...)`（判官侧信誉可见化）；`+3 PR 被合并`（AwardPRMerged）接线入 FIX-21 的 merge 路径。
  4.（Phase 6 补，F-104）撤权重算的 outcome 阈值 `judge_service.go:193` 硬编码 0.6，而闭案（:151-154）读 `cfg.Judge.PassThreshold`——激活撤权后两者口径不一致会误撤权，一并改为读配置。
- **优先级与顺序**：P1；依赖 FIX-01（否则无判官可测）；与 FIX-10 同文件不同函数，建议 FIX-10 先行合并后再做或同批串行。
- **建议车道**：heavy（审核链路 + 信誉分）。
- **验证标准**：TDD——先写失败测试：构造 closed_approve 案件 + 错误投票历史 → 撤权触发；多数派判官获得 +1 reputation_log；`go test ./...`；前端 verdict 页结案横幅渲染。
- **风险与回归面**：撤权激活后要防误伤（window 内票数 >10 才生效已有守卫 :178）；奖励加分会写 reputation_logs——注意与 FIX-37 的展示页联动。

#### FIX-04 admin reports 处理契约断裂 + ConfirmModal 错误透传
- **问题背景**：[A] P0；F-008
- **场景**：admin → /admin/reports → 点「成立」填理由确认 → **无任何反馈**（后端实际 400）。复现：`PATCH /api/v1/admin/reports/999999 -d '{"resolution":"upheld","explanation":"x"}'` → 400 "status is required"（本复核实测）。
- **根因**：前端 `admin/reports/page.tsx:175-179` 发 `{resolution, explanation}`；后端 `admin.go:742-749` 绑定 `{status, action_taken}`；`confirm-modal.tsx:61` catch 吞错且页面无错误位；列表读 `report.resolution/explanation`（:126-137）模型无此列（model/review.go:18-31 只有 ActionTaken）。
- **修复方案**：前端改发 `{status: "resolved"|"dismissed", action_taken: explanation}`；详情面板改读并展示 `action_taken`；onConfirm 内 try/catch → `setError(...)`（复用页面 error 位 + user-facing-error 映射）。后端不动。
- **优先级与顺序**：P0，无依赖。
- **建议车道**：light（前端契约对齐，不改安全语义；admin 动作落库逻辑不变）。
- **验证标准**：浏览器实测成立/驳回两条路径成功且列表状态变更；错误路径（断网/400）弹窗保持且有红字；`npm run build && npm run lint`。
- **风险与回归面**：`action_taken` 列已存在（模型已有），无迁移。**（Phase 6 实测确认）后端合法 status 集合 = `resolved`|`dismissed`**（admin.go:755-763 手动校验，probe "upheld"→400），本方案取值正确——「成立」映射 resolved 仅是词汇取舍，i18n 文案保持成立/驳回语义；e2e 无 answer_key/resolution 同构数据需同步（grep 0，原风险注作废）；「成立」不处置被举报目标的 enforcement 语义不在本项（F-116），PR 描述需明示。

#### FIX-05 Agent 气泡与引用卡对比度（WCAG）
- **问题背景**：[A/C] P0；F-007（P0）、F-094（P3）
- **场景**：浅色主题 /agent 工作台 → 用户发送消息 → 气泡 `#FFFFFF` on `#F5F5F5` = 1.09:1，正文不可读；深色主题引用卡标题 `#6366F1` on `#0D1117` = 4.24:1（text-sm 非大字，差 0.26 未达 AA）。
- **根因**：`AgentWorkspace.tsx:575` 用户气泡误用 `bg-accent`（shadcn 淡灰 hover 底）+ `text-white`；`globals.css:197` dark `--accent-emphasis: #6366F1` 亮度不足。
- **修复方案**（**Phase 6 修订——原「双主题 6.29:1」数值只对 Light 成立**：Dark `--primary`=#6366F1 配白字实为 **4.47:1 未达 AA**，复算见 F-111）：①用户气泡**双主题统一用 #4F46E5 系**（如新增专用 token 或固定 Light --primary 值），白字 6.29:1 双主题通过——不得直接复用 dark `bg-primary`（4.47），也**禁止白字配 #818CF8**（仅 2.98）；②dark `--accent-emphasis` 提亮为 `#818CF8`（=6.34:1，引用卡修复不变）——**同步更新 `design/design-system.md` token 表**（唯一 token 权威），并 grep 全站 `--accent-emphasis` 消费面做双主题目检；③**并入 F-111 同根裁决**：dark `--primary`（#6366F1→建议 #4F46E5）与 dark `--accent-hover`（#818CF8 配白字 2.98:1，123 处默认按钮 hover 中招）需在同一 token 决策中一并修，具体值以对比度脚本 ≥4.5 定稿。
- **优先级与顺序**：P0，无依赖（agent 开启前的必修项）。
- **建议车道**：light。
- **验证标准**：对比度脚本复算 ≥4.5:1；亮暗双主题浏览器截图（开启 web_agent_enabled 本地验证后关闭）；design-system.md 与实现一致。
- **风险与回归面**：`--accent-emphasis` 全站消费（TagBadge blue、active tab 等）——dark 提亮是全局变更，需目检所有使用面（多为 6 系色对 #0D1117，提亮只会更达标）。

#### FIX-06 /search/trending 契约错配
- **问题背景**：[A] P2；F-067
- **场景**：访客 → /original 侧栏「热门」板块永不渲染且静默（catch 吞）。复现：`GET /api/v1/search/trending` → `{"trending":[{"text":"5","score":1}]}`（本复核实测）。
- **根因**：后端 `search.go:46-57` 返回 `{trending:[{text,score}]}` 且 member 是**内容 ID**；前端 `OriginalSidebar.tsx:22-34` 读 `data.items` 的 `{name,id,participant_count}`。
- **修复方案**（推荐 A）：A）后端 `search_service.GetTrending` join content_items 把 ID 解析为标题 + 返回 `{trending:[{text,score,content_id}]}`，前端适配并深链 `/content/{content_id}`；B）改热搜词聚合（无数据源，不做）。i18n 无新词。**（Phase 6 三个必补前提）① join 必须过滤可见性**（published+未删+作者未封禁+is_public）：当前 `rank:hot:contents` 4 个 member 全是 banned/under_review 内容（id 5=banned；41/42/13=under_review，F-112），不过滤会把封禁内容标题+直链推上原创页；**② 前端展示名不得复用 `home.trendingN` 静态 i18n 演示标签**——OriginalSidebar.tsx:25-29 现状即固定 5 条假榜单文案（F-110），应渲染真实内容标题；③ ZSet 有 24h TTL 且可为空（实录先空后有），验收不得假设数据恒在。
- **优先级与顺序**：P2，无依赖。
- **建议车道**：light。
- **验证标准**：契约测试（handler 层）断言字段；浏览器验证原创页热搜渲染 + 点击跳转正确实体；`go test ./...`。
- **风险与回归面**：rank:hot:contents 的 member 语义被消费方依赖（hot_rank_service）——只读不改写。

#### FIX-07 「我的收藏」错链修复
- **问题背景**：[A] P1；F-033、F-084
- **场景**：登录用户 → 首页/原创页侧栏点「我的收藏」→ 到达「我的内容」管理页（预期 /studio/favorites 收藏集管理）。
- **根因**：`HomePageClient.tsx:154` 与 `OriginalSidebar.tsx:47` 的 href 均为 `/studio/contents`。
- **修复方案**：两处 href 改 `/studio/favorites`（未登录 redirect 同步）；顺手在 `StudioSidebar.tsx` 数据看板分组补「收藏集」入口（F-033 的可达性收口）。
- **优先级与顺序**：P1，无依赖。
- **建议车道**：light。
- **验证标准**：浏览器点击两处入口落到 /studio/favorites；`npm run build`。
- **风险与回归面**：无。

#### FIX-08 admin 后台入口与导航缺口
- **问题背景**：[C] P1；F-038
- **场景**：admin 登录后 Header 用户菜单无「管理后台」项（全站无 /admin 入链）；/admin/notifications（广播页）不在 ADMIN_NAV，只能手输 URL。
- **根因**：`Header.tsx:242-287` 菜单无 role 分支；`admin/layout.tsx:16-29` ADMIN_NAV 12 项缺 notifications。
- **修复方案**：Header 用户菜单 `user.role==='admin'` 时插入「管理后台」项（i18n zh/en）；ADMIN_NAV 加 `/admin/notifications`（navNotifications）。
- **优先级与顺序**：P1，无依赖。
- **建议车道**：light。
- **验证标准**：admin 账号浏览器可见入口并跳转；普通用户不可见；`npm run lint`（i18n key 双语补齐）。
- **风险与回归面**：Header 是全站组件，注意菜单分组样式遵循 ui-spec `Component: Header`。

#### FIX-09 本地通知 worker 闭环（部署/文档，不改架构）
- **问题背景**：[B] P1；F-025、F-088
- **场景**：按 AGENTS.md Step 2 启动（仅 `cmd/server`）→ 评论/点赞/PR/申诉结果等全部动态通知滞留 Redis Stream（`omnicraft:notification.create`），永不落库 → 铃铛/消息中心只有种子数据与广播。面试演示「触达闭环」翻车。
- **根因**：ADR0005 规定 worker 独立进程（container.go:274-283 注释明示 API server 永不起消费者、无 fallback）；`Notify()`（notification_service.go:274-290）在有 broker 时只入队；而 AGENTS.md Step 2、docker-compose、demo 文档均未包含 `cmd/worker`。
- **修复方案**（**Phase 6 收缩**——原 ②③ 为已完成项的重复施工：根 `docker-compose.yml:290` 已有 worker 服务[非 profile 门控]，`docker-compose.interview-lean.yml` 与 `scripts/live-demo.sh:195-198` 均已含 worker；照原文执行会与既有 worker 服务冲突）：① AGENTS.md Step 2 加 `cd backend && go run cmd/worker/main.go &`（含「只启一个 worker」注记，多实例虽靠 group 语义安全但无必要）；② **启动前清理积压**：notification stream 现有 9 条滞留（含 follow.go 的 user_id=0 与审计期测试通知），先做 FIX-31 的 follow.go `Notify(0)` 删除，再 `XTRIM`/`DEL` 旧流并重建 group（`XGROUP CREATE omnicraft:notification.create omnicraft-notification $`），否则积压全部物化为演示噪音；③ lag 检查命令（`redis-cli xinfo groups omnicraft:notification.create`）写入 Step 2。**不改 ADR0005 架构、不加进程内 fallback**（避免破坏 DLQ/幂等语义）。
- **优先级与顺序**：P1，先于 FIX-17/28/31（触达类修复的验证前提）。
- **建议车道**：light（文档 + compose 编排）。
- **验证标准**：按新 Step 2 启动后，触发一条评论通知 → notifications 表新增行、lag 回 0；verify-project.sh 不受影响。
- **风险与回归面**：worker 与 server 共用 Redis consumer group——重复启动多 worker 安全（group 语义），但文档要注明只启一个。

### 批次 B：审核链路批次

#### FIX-10 判官闭案回写内容状态 + 作者通知（众裁出口完备）
- **问题背景**：[A] P0；F-003、F-005、F-029（判官面）
- **场景**：作者内容被 Green 判「争议」→ under_review + 立案 → 判官投满 20 票且 ≥60% approve → 案件 closed_approve 但**内容仍 under_review、作者无通知**（列表/详情/作者 studio 均不可见，无出口）。closed_reject 同样无动作。
- **复现**：psql 构造 judge_case（min_votes=19）+ 19 票 approve → SubmitVote 触发闭案 → `SELECT status FROM content_items` 仍 under_review（域笔记 B 附录 A-8 有记录；本复核代码级确认 CloseCase 无任何 content 写）。
- **根因**：`judge_service.go:148-170` 闭案只调 `judge_repo.CloseCase`（judge_repo.go 仅 update judge_cases）；全库无 closed→content 映射；违反 business-rules.md:160「≥60% → 恢复展示；<60% → 不予展示（管理员可手动恢复）」。
- **修复方案**：闭案同一事务内（或紧随其后）：
  - `closed_approve` → 条件更新 `content_items SET status='published' WHERE id=? AND status='under_review'`（守卫防覆盖 admin/AI 终态 banned）+ 复用 `ReviewService.emitContentEvent(TopicContentPublished)`（喂 indexer/RAG）；
  - `closed_reject` → `status='banned', ban_reason='judge_verdict: 不违规比例<60%' WHERE status='under_review'`（不扣信誉分——扣分语义属 AI/admin 通道）+ `TopicContentBanned` 事件；
  - 两态均 Notify 作者（`content_status`，见 FIX-17 的 helper，channel=system）。
  - 依赖 FIX-16（model 加 BanReason 字段）先行或同 PR。
- **优先级与顺序**：P0；依赖 FIX-16（ban_reason 落库）与 FIX-17（通知 helper）；建议顺序 FIX-16 → FIX-17 → FIX-10。
- **建议车道**：heavy（审核链路 + 状态机 + outbox 事件；TDD 必须先写失败测试：闭案后内容状态断言）。
- **验证标准**：单测覆盖 closed_approve/closed_reject/守卫（已 banned 不覆盖）/作者通知四例；本地全链路：构造案 → 投票 → 内容恢复可见 + 通知落库（需 FIX-09 worker）；`go test ./... && go build ./... && go vet ./...`；改 routes/migrations 后跑 doc-validator --fix。
- **风险与回归面**：outbox 事件幂等（TopicContentPublished 已有消费者 indexer——重复事件需可重放）；闭案在 SubmitVote 请求路径上，注意事务边界（案件关闭失败不应丢投票——现状已是分开调用，保持先投票后闭案再回写的顺序并补偿日志）。

#### FIX-11 举报自动隐藏触发众裁
- **问题背景**：[A] P1；F-015
- **场景**：内容被举报达阈值（reports/views ≥ report_auto_hide_rate）→ status=under_review 但**无 judge case** → 永久滞留（无判案、无恢复、无告知；与 Green review 路径行为不一致）。违反 business-rules.md:164「触发众裁」。
- **根因**：`social_service.go:296-307` 仅 `UpdateContent(status=under_review)`；对照 `review_service.go:343-353` AI review 路径会 `ensureJudgeCase`。
- **修复方案**：把 `ensureJudgeCase`（review_service.go:524-547）提炼为可复用方法（如 ContentJudgeCaseEnsurer 接口或移至共享 service），Report 达阈值时同事务调用；同时补作者通知（FIX-17 helper）。附带修复：auto-hide 更新加 `WHERE status='under_review' OR status='published'` 守卫，避免把 banned 终态打回（A 域断言 F1 补充项）。
- **优先级与顺序**：P1；依赖 FIX-17；与 FIX-10 同域（都消费 ensureJudgeCase），建议随后。
- **建议车道**：heavy（审核链路；TDD：Report 达阈值 → judge_cases 新行断言）。
- **验证标准**：单测：构造高举报比 → under_review + judge case 存在 + 作者通知；banned 内容举报不再改状态；`go test ./...`。
- **风险与回归面**：ensureJudgeCase 重构波及 review_service——保持行为等价（已有测试 review_service_test.go 回归）。

#### FIX-12 内容详情可见性收口
- **问题背景**：[A] P1；F-009（+F-016 媒体暴露面前置）
- **场景**：匿名/任意用户直链 banned/under_review/pending 内容 `GET /contents/:id` → 200 全量返回（含正文、媒体 URL）；且对隐藏内容计数浏览量（污染 view_count=auto-hide 比例分母）。本复核实测 banned 内容 5 匿名 200。
- **根因**：`content.go:302-322` → `FindByID`（content_repo.go:55-65，仅滤 deleted_at）→ 无条件 `IncrViewCount`；对比 related-fanworks 用 `GetVisibleContent`（content_service.go + content_visibility.go:7-19）。
- **修复方案**：GetContent 改 viewer-aware：`published` 对所有人可见；非 published 仅 author/admin 可读（判官读取权限随 FIX-36 再开：持对应类型资格的判官可读 under_review，用于审案预览）；view_count 仅对 published 且非 author 计数。实现：handler 传入 viewerID（optAuth 已注入），service 层查状态+归属后决定 404/放行；**详情缓存仅缓存 published 视图**（缓存键不含 viewer 或对非 published 旁路缓存）。前端 SSR 详情页对 404 渲染 NotFound。**（Phase 6 两点必补）① 可见性收口必须同时覆盖列表维度**：主列表与 `GET /users/:id/contents` 均不过滤 `is_public`（F-097，实测私密内容 id=2/1034 在公共列表）——与 FIX-43 合并为「列表+详情统一 ApplyContentVisibilityScope（status+deleted+作者封禁+is_public）」；② 改造 GetContent 序列化时**保留 774994f 的 DisplayURLSigner Decorate 调用**（attachments.oss_url 瞬态签名），勿在重构中丢失。
- **优先级与顺序**：P1；是 FIX-14（作者侧要看自己 banned 内容）与 FIX-36（判官预览）的前置。
- **建议车道**：heavy（内容安全边界；TDD：匿名读 banned 404、作者读 200、admin 读 200 三例）。
- **验证标准**：`go test ./...`（含 404 断言）；curl 三角色验证；SSR 页面 404 展示；`npm run build`。
- **风险与回归面**：详情缓存逻辑变化——注意 cache TTL 300s 内的旧 published 缓存不受影响；agent 引用卡/overlay 消费同端点，published 路径行为不变。

#### FIX-13 已发布内容编辑触发再审核 + banned 禁改
- **问题背景**：[A] P1；F-010
- **场景**：作者先发合规内容过审（published）→ PATCH 改标题/封面为违规内容 → 200 且不触发任何审核（绕过 Green）；banned 内容也可 PATCH 200。
- **根因**：`content_service.go:898-926` UpdateContent 无状态检查；title/cover 变更不触发 ReviewService（published 仅发 RAG updated 事件，本复核确认 ai_review_records 不变）。
- **修复方案**：
  1. banned 内容 PATCH → 403 `CONTENT_BANNED`（终态禁改，删除仍允许）；
  2. published 内容变更走增量复审（**Phase 6 修正**：PATCH 契约实际**不含 description**——content.go:519-554 只接受 title/cover_image_url/is_public/allow_copy/agent_enabled，正文仅经 PR merge 变更——复审范围实为 **title+cover** 两字段）：复用 ReviewService 对新标题文本与封面 URL 做 ReviewText/ReviewImage（本地 fail-open 语义，release fail-closed）；结果 block → 转 banned + 扣分链路（与 AI 通道一致）；review/pass → 状态保持 published 但落 ai_review_records（记录编辑审计）；
  3. cover_image_url 校验（**Phase 6 按 774994f 后模型重述**：DB 存原始 OSS URL/key、签名发生在响应边界）：非空时必须是本站平台 OSS 域/键（防外链投毒），外链与相对路径拒绝。
- **优先级与顺序**：P1；依赖 FIX-12（可见性先收口，防编辑期间直链）。
- **建议车道**：heavy（审核链路；TDD：编辑后 ai_review_records 新增、banned PATCH 403）。
- **验证标准**：单测三例；本地（green fail-open）编辑 published 内容成功且 records+1；`go test ./...`。
- **风险与回归面**：编辑延迟增加一次审核调用（本地 mock 即时）；前端 studio 编辑入口（FIX-14）联动。

#### FIX-14 作者可知性：studio 内容管理全状态可见 + 编辑/删除/申诉入口
- **问题背景**：[A] P0；F-002、F-057、F-011、F-036
- **场景**：作者提交内容 → pending/under_review/banned 阶段从「我的内容」消失（接口默认只回 published）；列表无状态徽标、无 ban 原因；「编辑/删除」死按钮（无 onClick）；申诉表单要手填数字 target_id。
- **复现**：API 建内容 → psql 置 under_review → `GET /users/me/contents` 无该条（repo 默认 published，本复核代码级确认 user.go:266-283 + content_repo.go:156-160）。
- **根因**：GetMyContents 不传 Status；repo 默认 `status='published'`；studio/contents/page.tsx:100-118 无 status 渲染、:110-116 死按钮；appeals/page.tsx:122-127 手填 id。
- **修复方案**：
  1. 后端：GetMyContents 对**本人**放开状态过滤（返回全部非软删内容，含 draft/pending/under_review/banned）；GetUserContents（他人视角）保持仅 published+public（现状正确）；
  2. 前端 studio 列表：StatusBadge（pending=审核中 / under_review=复核中 / banned=已封禁 / draft=草稿，i18n zh/en）；banned 行展示 ban_reason（依赖 FIX-16）+「去申诉」按钮（跳 /appeals 预填 target_type=content&target_id）；
  3. 编辑按钮 → 打开编辑弹层（PATCH /contents/:id：title/description/cover/is_public/allow_copy）；删除按钮 → ConfirmModal → DELETE；
  4. appeals 页支持 query 预填 target。
- **优先级与顺序**：P0；依赖 FIX-12（作者读自己非 published 内容的 API 权限）、FIX-16（ban_reason 透出）。
- **建议车道**：heavy（含 API 契约与审核状态暴露；后端部分 TDD：本人视角状态集断言）。
- **验证标准**：单测（本人含 under_review、他人仅 published）；浏览器：建内容→置审→列表可见带徽标→编辑/删除可用→申诉预填；`go test ./... && npm run build`。
- **风险与回归面**：GetMyContents 语义变化影响 studio 数据看板统计口径（overview 若同源需同步）；StatusBadge 需在 ui-spec 登记（`Component: Badge` 变体）。

#### FIX-15 封禁用户申诉出路（封禁屏可达 + account 申诉 + 登录页指引）
- **问题背景**：[A] P0；F-001、F-026、F-042
- **场景**：用户被 ban → 登录 403 USER_BANNED（页面有文案但无指引）；持旧 token 全部 authReq 401（含 GET /appeals/me、POST /appeals）→ 前端 fetchMe 清 user → `(protected)/layout.tsx:42-53` 封禁屏永远不可达；即使可达，其 /appeals 链接在 protected 组内自循环；target_type 无 account 枚举。**产品内零出路**（唯一通道是匿名 /feedback 工单，无引导）。
- **根因**：middleware/auth.go:100-104 无豁免；AuthContext.tsx:146-151 清 user；appeal.go:31 `oneof=content comment`；违反 business-rules.md:260（Task 105：封禁用户应见 EmptyState + 申诉链接）。
- **修复方案**（推荐 A）：
  - **A（复用现有体系）**：
    1. `GET /auth/me` 对封禁用户返回 **200** + `{user, capabilities:{can_interact:false, interaction_denial_reason:"USER_BANNED"}}`（token 有效性照常校验；其余 authReq 路由仍 401/403 不变）——使 AuthContext 拿到 user、封禁屏（Task 105 设计）真正可达；
    2. auth 中间件对 appeals 相关端点（`GET /appeals/me`、`POST /appeals`）开 banned 白名单（封禁用户只能读/写自己的申诉；handler 用 callerID 天然隔离）；
    3. appeal target_type 增加 `account`（SubmitAppeal oneof 扩展 + ResolveAppeal account approved → `users.is_banned=false, ban_reason=''` + InvalidateUserStatusCache）；
    4. 封禁屏链接保持 /appeals（白名单后不再死链）；登录页 USER_BANNED 文案下方加工单入口链接（`/feedback`，公开路由）。
    5. **（Phase 6 补，F-098）** `(protected)/layout.tsx:42-54` 封禁屏目前对 banned 用户**替换** children——即使 1~4 落地，封禁屏上的 /appeals 链接点击后仍渲染封禁屏自身。需对 `/appeals` 路径放行 children 渲染（如 `pathname.startsWith('/appeals')` 时不套封禁屏），封禁屏链接改为 `/appeals?target_type=account` 预填。
  - B（独立公开申诉页）：匿名表单+邮箱——引入新滥用面，不推荐。
- **优先级与顺序**：P0；与 FIX-17（申诉结果通知已有）无强依赖，可独立先行；auth 中间件改动需与 FIX-12 的 viewer 注入协调回归。
- **建议车道**：heavy（auth + 封禁/申诉，安全敏感；TDD：banned token GET /auth/me 200、POST /appeals account 201、其他 authReq 仍 401）。
- **验证标准**：单测（中间件矩阵：banned × {auth/me, appeals, 其他} 三列）；浏览器全旅程：ban 账号 → 登录页有指引 → 旧会话封禁屏可见 → 提交 account 申诉 → admin 批准 → 解封可登录；`go test ./... && npm run build`。
- **风险与回归面**：/auth/me 语义变化是全局契约（AuthContext capabilities 已支持 denial_reason 渲染，风险低）；白名单要精确到路由前缀避免误放开其他端点；admin ResolveAppeal account 分支要写审计。

#### FIX-16 封禁理由全链路（model 字段 + 前端发送 + 作者侧透出）
- **问题背景**：[A] P1；F-013、F-035
- **场景**：admin 封内容弹窗强制填 reason → 前端丢弃 `_reason` 发 `{}`（后端支持入库但收不到）；`content_items.ban_reason` 列存在但 model 无字段 → 任何 API 不返回；作者侧 0 消费。IP 驳回同理（见 FIX-24）。
- **根因**：`admin/contents/page.tsx:65`（banContent 发 `{}`）+ :188-194（丢 `_reason`）；`model/content.go` 无 BanReason；后端 `admin.go:242-251` 已支持 reason。
- **修复方案**：
  1. model.ContentItem 加 `BanReason string \`gorm:"column:ban_reason" json:"ban_reason,omitempty"\``（**列已存在**，无迁移）——注意 omitempty + 仅作者/admin 序列化（结合 FIX-12 的 viewer DTO，或 service 层按 viewer 剥离）；
  2. 前端 banContent 传 `{reason}`（onConfirm 的 reason 参数透传）；
  3. 作者侧展示接 FIX-14（studio 列表 banned 行 + 通知 body 带 reason）。
- **优先级与顺序**：P1；FIX-10/FIX-14/FIX-17 的前置。
- **建议车道**：heavy（封禁链路 + 模型字段暴露策略；TDD：admin ban with reason → 作者 GET 内容含 ban_reason、匿名不含）。
- **验证标准**：单测；浏览器 admin 封禁填原因 → 作者 studio 可见原因；`go test ./...`；改 model 后跑 doc-validator --fix（schema 快照同步）。
- **风险与回归面**：序列化暴露策略要防匿名读取（与 FIX-12 同 PR 协调）；migration 无需但要确认 040 列类型 text 兼容。

#### FIX-17 内容状态转移通知矩阵（content_status 落地）
- **问题背景**：[A/C] P1；F-029、F-037、F-050
- **场景**：作者内容被 AI ban / admin ban / 转审 / 判官闭案 / 恢复展示 / IP 级联下架——**全程零通知**（枚举 `content_status` 定义于 business-rules.md:140 但全库 0 使用）。admin 的 user ban/unban、IP approve/reject、report resolve 同样静默。
- **根因**：review_service.go:325-353、admin.go:236-282、judge_service.go:148-170、social_service.go:296-307、review_service.go:453-468 五处状态写点均无 Notify；notifSvc 在 admin handler 已可用（admin.go:39,77-79）。
- **修复方案**：新增 `NotifyContentStatus(authorID, contentID, status, reason)` helper（channel=system, type=content_status, target=content, body 带 reason）；挂接点：① AI applyContentReviewResult banned/under_review/published；② admin BanContent/RestoreContent；③ Report auto-hide；④ 判官闭案（FIX-10 内联）；⑤ IP 级联批量（按受影响作者去重各一条）；⑥ admin BanUser/UnbanUser（type=system，标题区分）与 IP approve/reject（通知创建者）。通知文案先沿用后端中文模板（与现状一致），**Phase 2 再做 locale 模板**（在尾部不修项登记）。
- **优先级与顺序**：P1；依赖 FIX-09（worker 运行才能验证落库）；是 FIX-10/11 的组成部分。
- **建议车道**：heavy（通知语义 + 审核链路）。
- **验证标准**：单测（各状态写点断言 Notify 调用参数）；本地全链路（worker 运行）断言 notifications 表新增；`go test ./...`。
- **风险与回归面**：通知经 Redis Stream——事务外的补偿语义（发布失败仅 slog，现状如此，不扩大）；避免通知风暴（IP 级联按作者聚合）。

#### FIX-18 讨论发帖/回复统一走审核门
- **问题背景**：[A] P1；F-027、F-028
- **场景**：信誉分 1 的用户发评论被 403 拦截，但经前端实际使用的 `POST /ips/:id/discussions` → 201 直发（绕过信誉门+Green 审核）；讨论回复 `POST /discussions/:id/comments` 同样无审核且不通知楼主。
- **根因**：路由双轨：`routes.go:274`（ips/:id/discussions → discussion.go:44-68 直写 repo，无门）vs `routes.go:138`（/social/discussions → social_service.go:155-175 有 ensureCanInteract+moderateText）；回复侧 discussion.go:89-116 vs social_service.go:98-110 分叉；前端用的是无守卫版（ip/[ipId]/discussions/new/page.tsx:34、ReplyList.tsx:40）。
- **修复方案**：discussion handler 注入 SocialService，CreateDiscussion/ReplyToDiscussion 改走 `PostDiscussion`/`PostComment`（门+审核+通知一并复用）；路由不动（前端不用改）。注意 PostDiscussion 的 IPID 参数传递与 ReplyToDiscussion 的 reply_count 递增保留。**（Phase 6 四个实施前提）① 字段映射**：ReplyList 发 `content`，而 PostCommentInput 是 `body` binding required——handler 保留本地 binding 做映射，否则全站讨论回复 400；**②** 保留 IncrementReplyCount（含 last_active_at 递增，驱动 latest_reply 排序），并定夺 `/social/comments` 带 discussion_id 路径的 reply_count 语义（现从不递增，统一后防双计）；**③** NewDiscussionHandler 构造函数需加 SocialService 参数（routes.go），改后跑 doc-validator --fix；**④** 顺带收口：讨论列表 ListByIP/GetByID 无 status 过滤（F-106，潜伏）与讨论嵌套回复永不展示（ListCommentsByTarget 滤 parent_id IS NULL，F-107）——同批处理成本最低。
- **优先级与顺序**：P1；无硬依赖。
- **建议车道**：heavy（审核门语义；TDD：rep=1 经 /ips/:id/discussions → 403；正常用户发帖 → stream 出现楼主通知）。
- **验证标准**：单测（低信誉 403、正常 201、回复通知楼主）；`go test ./...`；本地复现 F-027 场景回归。
- **风险与回归面**：讨论创建从「无门」变「有门」是行为收紧（符合规格）；ReplyToDiscussion 现挂 commentsGuard（路由层），改造后保持双层不冲突。

#### FIX-19 用户公开资料 email 泄露
- **问题背景**：[A] P0；F-006
- **场景**：匿名 `GET /api/v1/users/334` → 200 含 `email`（本复核实测）。可批量爬取注册邮箱。
- **根因**：`user.go:439-453` sanitizeUser 无条件返回 email；GetUser 对任意 id 套用（routes.go:71 optAuth）。
- **修复方案**（**Phase 6 升级——原方案修完后 email 仍从 Author Preload 出口全站泄露，F-105[P0]**）：
  1. **model 层收敛（主修）**：`model/user.go:9` `Email` 的 `json:"email"` 改为 `json:"-"`（或统一改用 AuthorSummary DTO：id/username/avatar/display_name），凡 `Preload("Author")` 且序列化 author 的端点（内容列表/详情、讨论列表/详情、评论、搜索、系列——search_repo.go:193、series_repo.go:94、content_repo.go:57,138,146、discussion_repo.go:33,41,99、social_repo.go:65）一并不再泄露；
  2. sanitizeUser 拆两档：publicUser（去 email）与 selfUser（含 email + preferred_locale 等敏感项）；GetUser 仅 `caller==id || role==admin` 返回 self 档（/users/me、/auth/me 的本人路径改显式组装 email 字段）；
  3. **验收标准升级**：任意匿名/他人视角端点响应不含他人 email 键（含 Preload 路径）——现有 e2e/测试若断言他人 email 需同步；
  4. 联动：FIX-29①（评论 Preload）完成后每条评论携带 author 全量 JSON，必须与本项同批合并。
- **优先级与顺序**：P0；独立可先行。
- **建议车道**：heavy（隐私/安全）。
- **验证标准**：TDD：匿名/他人 GET → 无 email 键；本人/admin → 有；`go test ./...`；curl 三角色回归。
- **风险与回归面**：检查 admin/users 列表（独立端点，返回 email 是 admin 语义，保留）；测试与 e2e mock 中如有依赖他人 email 的断言需更新。

#### FIX-20 注销账号改软删除
- **问题背景**：[B] P1；F-014
- **场景**：用户注销 → `is_banned=true + ban_reason='self_deleted'`（非 deleted_at 软删）→ 已注销用户触发鉴权得到「account has been banned」误导语义；邮箱改写+密码随机化后忘记密码静默死路。违反 Task 103 软删除规范。
- **根因**：`user.go:347-363` DeleteAccount 的 updates 集合。
- **修复方案**：updates 改为：`deleted_at=NOW()` + 保留匿名化（username/email/avatar/bio 清写，防 PII 残留）+ **不设 is_banned**；auth 已支持 ErrUserStatusDeleted → 401「user not found or deleted」（middleware/auth.go:85-88，语义现成）。存量已注销用户（ban_reason='self_deleted'）提供一次性数据修正 SQL（migration 或运维脚本，置 deleted_at 并清 is_banned）——**需迁移文件**（heavy）。
- **优先级与顺序**：P1；独立。
- **建议车道**：heavy（auth 语义 + 迁移；TDD：注销后 /auth/me 401 deleted、密码重置对匿名邮箱走「不存在」路径）。
- **验证标准**：单测；迁移幂等（migration test）；`go test ./...`；doc-validator --fix。
- **风险与回归面**：content visibility scope 已排除 deleted 作者（content_visibility.go）——行为等价；审计关联（admin_audit target user）不受影响。

### 批次 C：PR/IP 协作批次

#### FIX-21 PR 后端语义修复（提案落库 + accept/merge 打通 + 正文同步 + 版本鉴权）
- **问题背景**：[B] P1；F-017、F-019、F-056
- **场景**：①贡献者提交 PR 带 new_text → 不落库（proposed_version_id 恒 NULL，DiffViewer 右栏恒空）；②作者 accept 后 merge 返回 400（互斥），accept 本身对内容零效果；③merge 只建版本不同步 content_items 正文（读者永远看不到合并结果）；④`GET /versions/:id`、`GET /pr/:id` optAuth 任意人可读（含 banned 内容版本全文）。
- **根因**：`pr_service.go:64-70`（NewText 丢弃）、`:110`（accept 仅状态+贡献者）、`:175-177`（merge 要求 open）、`:186-202`（只 SetLatest）；`handler/content.go` GetContent 不读版本；`routes.go:109-111` optAuth。
- **修复方案**：
  1. SubmitPR：为 new_text 建 `ContentVersion{status:"proposed", IsLatest:false}` + 写 proposed_version_id（proposed 版本不进 RAG/展示）；
  2. merge 允许 `open|accepted` 状态（移除互斥）；merge 事务内同步 `content_items.description=merged_text` + emit TopicContentUpdated；merge 后调用 AwardPRMerged(+3)（FIX-03 接线点）；
  3. accept 语义明确为「标记采纳」——通知文案说明仍需 merge 应用正文（i18n 更新）；
  4. GET /versions/:id 与 GET /pr/:id 加参与方校验（内容 author / PR submitter / admin）。
- **优先级与顺序**：P1；先于 FIX-22（前端入口消费这些语义）。
- **建议车道**：heavy（内容写路径 + 权限；TDD：new_text 落库、accept 后 merge 200、版本越权 403）。
- **验证标准**：单测四例；本地 curl 全链路（提交→diff 可见→accept→merge→正文更新）；`go test ./...`。
- **风险与回归面**：SetLatest 与 RAG 事件顺序（先版本后正文同事务）；proposed 版本对 `GET /contents/:id/versions` 列表的可见性（作者可见 proposed，读者只见 active）。

#### FIX-22 PR 前端入口 + studio 信息架构收口
- **问题背景**：[A/B] P1；F-018、F-032、F-020、F-030、F-031、F-055
- **场景**：①贡献者无任何 UI 提交 PR（SubmitPREntry 孤儿组件、目标页不读参数）；②studio 侧边栏/待办卡/通知路由指向三个空壳页（真实实现滞留旧 /dashboard）；③旧 /dashboard/contents 字段大小写损坏（标题空白、链接 /content/undefined）；④标签建议待办卡恒空。
- **根因**：studio/{pr-requests,contributors,tag-suggestions} 为 18-19 行空壳；dashboard 四子页为独立实现且 contents 页 interface 用 PascalCase（dashboard/contents/page.tsx:16-22 vs model 全小写）；overview/page.tsx:32 setPendingTasks 未调用。
- **修复方案**：
  1. 把 dashboard 三页实现迁入 /studio/*（路径与文案同步，`/dashboard/*` 四子页改 redirect 至对应 studio 页；删除损坏的 dashboard/contents 实现改 redirect）；robots.ts 同步（disallow /studio/ 管理区，移除 /dashboard 屏蔽）；
  2. PR 创建：/studio/pr-requests 读 `?content_id&create=1` 打开创建表单（选 base version ← GET /contents/:id/versions + message + new_text → POST /api/v1/pr）；SubmitPREntry 链接改 /studio/pr-requests；
  3. PendingTasksCard 数据源：页面加载时并行拉 `GET /dashboard/tag-suggestions?status=pending` 与各内容 open PR 计数（或后端聚合端点 `/users/me/pending-tasks`——推荐后者，一次请求）；同时 `SuggestTag` 成功后给内容作者发通知（type=system，F-031 触达面收口）；
  4. 贡献者页数据后端化（F-055）：新增 `GET /contents/:id/contributors` 聚合端点（含 pr_count/来源 merged|invite/blocked 状态），页面改为服务端数据，`blocked` 接 `/dashboard/contributors/:userId/block` 真实状态；屏蔽操作可选通知（并入本项）。
- **优先级与顺序**：P1；依赖 FIX-21（提交/merge 语义可用）；依赖 FIX-31（通知深链指向新页）弱关联。
- **建议车道**：light（纯前端迁移/收口，后端聚合端点可选 heavy 单拆）。
- **验证标准**：浏览器全链路：贡献者从内容页提交 PR → 作者 studio 收到列表+diff → accept/merge → 待办卡计数；`npm run build && npm run lint`；旧路由 301 断言。
- **风险与回归面**：i18n key 迁移（studio.pr.* 已有，dashboard 命名空间清理）；e2e mock 路由表同步（**Phase 6：实测 e2e 目录无 /dashboard 引用，此项无事可做**；另 SubmitPREntry.tsx:28 现链 /dashboard/pr-requests，迁移时一并改目标）；PendingTasksCard 新接口契约测试。

#### FIX-23 IP 创建者状态可知性（**Phase 6 重界定：创建侧已由 fecef1b 交付，勿按原文重复实现**）
- **问题背景**：[B] P1；F-022（部分已修复）+ F-101/F-102/F-103（Phase 6 新增加固）
- **对账（2026-09-01，fecef1b）**：创建入口+表单已上线——`/studio/publish/ip`、`frontend/components/studio/IPPublishForm.tsx`（实调 POST /api/v1/ips，含 tags/cover presign、pending 成功面板）、StudioSidebar 入口、/ips 页头部按钮（login-gated）、ui-spec 已有 `Page: /studio/publish/ip` 规格。原根因段「全前端无 POST /ips 调用」「ui-spec 无该页规格」**已过时**。
- **剩余范围**：
  1. 后端新增 `GET /users/me/ips`（本人全部状态 IP + 最新 review reason）；
  2. 「我的 IP」列表页（状态徽标 pending/approved/rejected + 拒绝原因[依赖 FIX-24] + 重新提交=新建）；
  3. approve/reject 通知接 FIX-17；
  4. 可选：IPPicker 加「新建 IP」深链。
- **新增加固（Phase 6，fecef1b/6d1d781 引入或暴露）**：
  5. POST /ips 挂 publishingInteractionPolicy + 限流（F-103：现仅 authReq，未验证邮箱/低信誉/发布冻结用户可建且可无限刷；对照 contents POST 有 publishGuard+UploadRateLimit）；
  6. IP tags 纳入 Green 文本审核（F-101：ipReviewInput 只送 name/description/cover，tags 免审即公开展示）；
  7. tags 服务端条数上限（F-102：normalizeIPTags 无条数 cap，前端 cap 10 仅客户端）。
- **优先级与顺序**：P1；依赖 FIX-24（原因透出）与 FIX-17（通知）；⑤为安全面建议随本批先行。
- **建议车道**：后端新端点+⑤⑥⑦ heavy（守卫/审核语义；TDD：仅本人可见、guard 拦截、tags cap）；前端 light。
- **验证标准**：单测（本人状态集、guard、tags cap）；浏览器：提交→我的 IP 列表 pending→（admin approve）→通知+状态变更；低信誉账号 POST /ips 403；`go test ./... && npm run build`。
- **风险与回归面**：复用 fecef1b 的表单与 i18n key（studio.ip.* 已存在）；改 routes.go 后 doc-validator --fix。

#### FIX-24 IP 拒绝原因落库与透出
- **问题背景**：[A] P1；F-023
- **场景**：admin 驳回 IP 弹窗强制填原因 → 前端丢弃 `_reason`；后端 RejectIP 不解析 body → ip_review_logs 恒空；创建者只见 rejected 不知为何、无重提路径。
- **根因**：`admin/ips/page.tsx:195-199` 丢弃 `_reason`；`admin.go RejectIP`（:172-197 段）无 body；IPReviewLog.Reason 无写入方。
- **修复方案**：RejectIP handler 解析 `{reason}`（binding required）→ service 层写 ip_review_logs（actor/reason/created_at）+ 通知创建者（FIX-17）；前端 `_reason` 透传；「我的 IP」列表（FIX-23）展示最新 reason。重提路径=新建（状态机无回退，产品可接受，文档标注）。
- **优先级与顺序**：P1；与 FIX-23 同批。
- **建议车道**：heavy（审核决策链路；TDD：reject 带 reason → ip_review_logs 一行 + 通知）。
- **验证标准**：单测；浏览器 admin 驳回填原因 → 创建者「我的 IP」可见；`go test ./...`。
- **风险与回归面**：ip_review_logs 表结构确认（model/ip.go:21-32 已有 Reason 字段）。

#### FIX-25 IP 详情可见性与 admin 缓存失效
- **问题背景**：[B] P2；F-052、F-051
- **场景**：pending/rejected/banned IP 匿名可直链完整可读；admin approve/reject 后 `cache:ip:{id}`（TTL 300s）不失效——决策最长 5 分钟不生效。BanIP/BanIPAndContents 是无路由死代码。
- **根因**：`ip.go:91-109` GetIP 无状态/属主过滤；`admin.go:65` AdminHandler 的 ipSvc 用 `NewIPService(repo)`（无 redis）→ `ip_service.go:234-238` invalidate 空转。
- **修复方案**：GetIP 非 approved 限 creator/admin；AdminHandler 注入带 redis 的 IPService（或 reject/approve 后直接 `DEL cache:ip:{id}`）；BanIP 补 admin 路由 `POST /admin/ips/:id/ban`（服务级联已存在，接通知）或删除死代码（推荐补路由——business-rules L58 级联语义已有消费价值）。
- **优先级与顺序**：P2；FIX-23/24 之后。
- **建议车道**：heavy（可见性边界 + admin 路由；TDD：匿名读 pending IP 404、creator 200）。
- **验证标准**：单测；curl 三角色；admin reject 后立即 GET 返回新状态；`go test ./...`；改 routes.go 后 doc-validator --fix。
- **风险与回归面**：IP 详情 SSR 页对非 approved 的 404 处理；ban 级联触发内容批量 banned——与 FIX-17 的 IP 级联通知联动。**（Phase 6 交叉引用）缓存失效半与 E2E fix-loop 台账 B-004（docs/working/2026-09-01-e2e-fix-loop-ledger.md，流程 2 批次）为同一缺陷——执行前先与该会话对账，避免重复修复。**

### 批次 D：触达与反馈批次

#### FIX-26 错误码→用户语言映射扩展
- **问题背景**：[C] P1；F-039、F-043、F-046（文案部分）
- **场景**：发布冻结用户只见「发布失败」（PUBLISH_FROZEN 被吞）；高频码（APPEAL_EXISTS/SOURCE_IMMUTABLE/INSUFFICIENT_QUESTIONS/CAPTTCHA_*/ARCHIVE_*/MISSING_QUERY 等）落到通用「操作失败」或裸英文；messages `error.*` 16 码是死代码。
- **根因**：`user-facing-error.ts:6-31` 仅 12 码；`PublishForm.tsx:421-422` catch 不读 code；`SearchAgentInput.tsx:67` setError(e.message)。
- **修复方案**：扩展 getUserFacingErrorKey 映射表（按域补 ~25 码：PUBLISH_FROZEN→publish.frozen、APPEAL_EXISTS→appeals.exists、CAPTCHA_REQUIRED→auth.captchaRequired、MISSING_QUERY→search.missingQuery 等，zh/en 双语）；PublishForm catch 按 code 分文案（冻结文案含「7 天后自动解冻/完成素质课程」指引）；SearchAgentInput 改用映射；error.* 死命名空间清理（**Phase 6 修正**：大写死码实为 **15 个**非 16；`error.pageError/pageErrorDesc/notFound/notFoundDesc/backHome` 5 个 key **在用**[not-found.tsx/error.tsx]——只删 15 个死码，保留在用 5 键）。PUBLISH_FROZEN 后端响应可选拓展 `retry_after` 字段（本次不强制）。
- **优先级与顺序**：P1；无依赖。
- **建议车道**：light。
- **验证标准**：单测/快照（code→key 全表）；浏览器触发 403 冻结（本地构造 redis key）显示专属文案；`npm run lint`（i18n parity）。
- **风险与回归面**：删除 error.* 命名空间需全 grep 确认 0 消费；zh/en key 集合必须一致（现有 parity 测试基线 1857 会变化）。

#### FIX-27 申诉质量包（目标校验 + pending 守卫 + 评论申诉恢复 + admin_response 渲染）
- **问题背景**：[A/C] P2；F-044、F-076、F-081、F-077、F-087
- **场景**：①`POST /appeals {target_id:999999}` → 201（本复核实测，污染 admin 队列）；②已处理申诉可被重复 Resolve（再改目标状态+再通知）；③批准「评论申诉」不恢复评论；④用户申诉页不渲染 admin_response（后端已返回）；⑤admin 申诉列表无内容预览链接、只见 pending。
- **根因**：appeal.go:28-57 无目标存在性校验；admin.go:364-420 无 pending 守卫；appeal_helpers.go:12-16 comment 无 approved 映射；appeals/page.tsx 无 admin_response 字段；admin.go:358-361 固定 pending。
- **修复方案**：
  1. SubmitAppeal 校验目标存在（content: contentRepo.FindByID；comment: socialRepo）→ 404 `TARGET_NOT_FOUND`；
  2. ResolveAppeal 开头 `status!='pending'` → 409 `APPEAL_ALREADY_RESOLVED`；
  3. appeal_helpers comment approved → `{status:"published"}`（评论隐藏态是 status=hidden 软删，恢复语义明确）；
  4. appeals/page.tsx 渲染 admin_response（i18n label）；
  5. admin appeal 列表加 status 筛选参数 + 行内「查看内容」链接（target_type=content 时）。
  6.（Phase 6 顺带，F-099）appeal.go:40 `HasPendingAppeal` 错误被吞（DB 错误静默放行 → 同 target 双 pending，appeals 表无 UNIQUE 兜底）——目标校验同批处理该错误分支（fail-closed）。
- **优先级与顺序**：P2；无依赖。
- **建议车道**：heavy（申诉链路；TDD：假目标 404、重复 resolve 409、comment approved 恢复）。
- **验证标准**：单测四例；浏览器用户侧看 admin_response；`go test ./...`。
- **风险与回归面**：appeal 批准→content published 强制语义不变（F-044 边界中的「批准软删/封禁作者内容」问题在此仅加守卫：目标 deleted_at 非空或作者被封禁时拒绝批准 409，最小收口）。

#### FIX-28 举报反馈闭环
- **问题背景**：[A] P1；F-012
- **场景**：用户举报后永远不知道结果（无 reports/me、ResolveReport 不通知）；评论举报后端存在但前端无入口。
- **根因**：routes.go 无举报者视角端点；admin.go:736-773 只改 report 行；CommentSection 无举报 UI。
- **修复方案**：①新增 `GET /reports/me`（本人举报列表：target/status/action_taken/created_at）；②ResolveReport 后 Notify 举报者（type=system，body=action_taken；status=resolved 时按 business-rules 触发「有效举报 +1」信誉分——复用 reputation_service，恶意举报 -2 逻辑同点实现[dismissed 时若 target 维持在库]——最小实现先做通知，加分项可裁剪）；③前端「我的举报」并入 /appeals 页 tab 或独立页；④CommentSection 评论项菜单加「举报」（ConfirmModal + POST /social/comments/:id/report）。
- **优先级与顺序**：P1；依赖 FIX-17 模式（通知 helper）与 FIX-09。
- **建议车道**：heavy（举报=审核链路）。
- **验证标准**：TDD：resolve 后举报者有通知、GET /reports/me 隔离他人数据；浏览器评论举报入口；`go test ./...`。
- **风险与回归面**：举报加分/扣分自动化的误判风险（admin dismissed≠恶意举报）——最小版只做通知，评分规则登记到后续迭代（在尾部注明）。

#### FIX-29 评论体验包（author 修复 + 嵌套回复 + 编辑/删除 UI + 折叠规则）
- **问题背景**：[A/C] P2；F-059、F-060、F-061、F-058
- **场景**：①内容评论区所有发言者名字空白（author 零值，本复核实测）；②API 支持嵌套回复但 UI 无入口且子回复永不渲染；③评论编辑/删除 API 完备（含 A4 编辑审核）但前端零调用；④点踩/点赞 ≥0.30 折叠规则（business-rules.md:165）完全未实现。
- **根因**：social_repo.go:38-51 无 Preload（对比 :53-69）；CommentSection.tsx:149,154 子回复取自仅含顶层的数组；无回复/编辑/删除调用；comment_fold_threshold 仅 config 定义。
- **修复方案**：
  1. ListComments 补 `Preload("Author")`（一行）；前端 username 兜底 `||` 改 nullish 语义；
  2. CommentSection：顶层评论加「回复」按钮（POST /social/comments 带 parent_id）+ 子回复区懒加载（GET ?parent_id=）；两级扁平展示（ui-spec CommentSection 对齐）；
  3. 本人评论菜单：编辑（PATCH，重过审核语义后端已备）/删除（DELETE，确认弹窗）；
  4. 折叠：后端 ListComments 返回每条 like/dislike 计数（**Phase 6：comments.like_count 列从未被维护**——React 只更新 content 行，勿直接读，走 reactions 聚合或补冗余列维护），前端按 ratio≥config 下发阈值折叠展示（「已折叠·点击显示」）；**触发审核后端联动**（折叠时自动转 under_review 评论）列为可选子项（涉及评论状态机，可后补）。
- **Phase 6 两点必补**：① 修复①使 author 变为真实 User 序列化——**必须与 FIX-19 的 model 层邮箱收敛同批合并**，否则每条评论泄露 author.email（F-105）；② ②做回复 UI 时顺带补评论分页（CommentSection 固定 page_size=50，第 51 条起不可达，F-108）。
- **优先级与顺序**：P2；①是一行快赢可单独先行。
- **建议车道**：①light；②③light；④heavy（若含评论转审联动）——建议拆两个 commit/PR。
- **验证标准**：浏览器：评论区名字正常、回复可见、编辑/删除可用、高踩比评论默认折叠；`npm run build`；后端 `go test ./...`（Preload 断言）。
- **风险与回归面**：Preload 性能（分页内 join，可接受）；折叠阈值需 /config/public 下发或注入（避免前端硬编码——Key Rule 6）。

#### FIX-30 私信可达性（发起入口 + 未读同步 + 摘要通知）
- **问题背景**：[C] P2；F-062、F-063
- **场景**：普通用户无法发起私信（唯一入口是协作邀请被动建会话）；聊天窗读过的私信铃铛仍亮（双通道未读不同步）；通知 body 泄露私信全文。
- **根因**：recipient_id 仅 ChatWindow（已存在会话）引用；UpdateLastRead 只清会话未读；message.go:124 Notify body=body.Text。
- **修复方案**（**Phase 6 修正①的后端前提——原方案会拆掉唯一防骚扰门，禁止执行**：`SendWithColdStartGuard`[message_repo.go:239-268] 本来就允许**每个方向的首条消息**，只拦「对方零发言且最后一条是自己」的连续第二条；F-063 纯粹是前端无入口。且 messages 路由组只有信誉门、**无独立 DM 频控**，该 guard 是唯一防骚扰机制）：①用户主页「发私信」入口（纯前端；POST /messages 走既有 guard 语义，首条天然放行——**后端不动、不得放宽 guard**）；②UpdateLastRead 同事务标记该会话对应 message 通知已读（或通知点击回清会话）；③Notify body 改摘要「你有一条新私信」；④ChatWindow 加 30s 轮询（SSE 列为 Phase 2）。车道可降 light（无防骚扰边界变化；②涉通知语义仍建议按 heavy 保守）。
- **优先级与顺序**：P2；无依赖。
- **建议车道**：heavy（私信通道语义：首条放行是防骚扰边界变化）。
- **验证标准**：TDD：任意用户首条 201、限流生效、已读联动；浏览器发起对话+双端未读一致；`go test ./...`。
- **风险与回归面**：首条放行增加骚扰面——依赖既有 per-minute 限流 + 举报入口（FIX-28④）配套。

#### FIX-31 通知细节修复包
- **问题背景**：[C] P3；F-092、F-064、F-065、F-093（①②③⑤ 子项；④ 粉丝列表 UI 见 3.2 不修项）
- **场景**：discussion 通知点击跳 `/content/{讨论id}` 错实体；appeal/feedback/message 通知无深链；下拉项点击不 markRead、一律跳 /messages；UnreadCounts 无 broadcast 键（chips 永不显示广播）；关注 IP 通知发给 user_id=0；反馈 resolved/in_progress 不通知；dislike 通知标题「新的赞」；文档 5min vs 实现 30s 轮询。
- **根因**：NotificationList.tsx:249-253 深链映射错误；NotificationDropdown.tsx:79 定义未用/:136,177 一律 /messages；AuthContext.tsx:36-43 类型缺 broadcast；follow.go:72 Notify(0)；feedback_service.go:428-435；social_service.go:215。
- **修复方案**：深链映射修正（discussion→GET /discussions/:id 取 ip_id 再跳 /ip/{ipId}/discussions/{id}，或后端通知带扩展字段 target_meta；appeal→/appeals、feedback→/feedback/mine、message→/messages?tab=conversations）；dropdown 点击 markRead + 使用 getNotificationUrl；UnreadCounts 加 broadcast（后端已返回计数）；follow.go Notify(0) 删除（或改通知 IP 创建者，产品定——推荐先删除止血）；feedback resolved 也通知；dislike 通知标题区分或取消 dislike 通知（推荐取消，减少负面轰炸）；business-rules.md 轮询间隔改 30s（对齐现实）。F-093 杂项收口：admin 置顶 override（discussion.go:133 权限条件加 `role=="admin"`）；删除他人评论错误风格与编辑对齐（social.go:79 改 403 FORBIDDEN + 专用码）；/messages 会话 Tab 计数改真实未读聚合（后端 unread-count 已有会话维度或 Sum 会话 unread_count）。
- **优先级与顺序**：P3；依赖 FIX-09。
- **建议车道**：light（follow.go 一行删除可先行；深链若加后端字段则该子项 heavy）。
- **验证标准**：浏览器各类型通知点击落正确页面；下拉已读生效；`npm run build`；`go test ./...`。
- **风险与回归面**：通知深链是契约扩展（前端解讨论 id 需一次 API 查询，注意 404 容错）。

#### FIX-32 401 静默刷新 + redirect 保参
- **问题背景**：[C] P2；F-047
- **场景**：两次轮询间隙（最长 5 分钟）token 过期 → 任意 API 调用 401 UNAUTHORIZED 直接抛错；fetchMe 失败踢登录页且 redirect 只保 pathname 丢 query（筛选/预填参数丢失）。
- **根因**：api.ts:195 只认后端从不返回的 TOKEN_EXPIRED；(protected)/layout.tsx:19-24 redirect 仅 pathname。
- **修复方案**：request() 内 `res.status===401 && errBody.code==="UNAUTHORIZED" && token` 时先尝试一次 doRefreshToken 重试（与 TOKEN_EXPIRED 分支同构）；layout redirect 拼 `useSearchParams().toString()`。
- **优先级与顺序**：P2；无依赖。
- **建议车道**：light。
- **验证标准**：构造过期 token 场景（本地改短 JWT 过期或 mock）请求自动恢复；带 query 受保护 URL 踢登录后回跳保参；`npm run build`。
- **风险与回归面**：401 重试要防循环（每请求至多一次）；banned 用户 401 USER_BANNED 不得触发刷新重试（code 区分，注意与 FIX-15 的 /auth/me 200 语义协同——USER_BANNED≠UNAUTHORIZED，天然隔离）。

### 批次 E：admin/运维与打磨批次

#### FIX-33 admin config 契约修复（防配置静默污染）
- **问题背景**：[A] P1；F-034
- **场景**：admin 打开 /admin/config → 全部区块显示**前端默认值**（GET 返回 PascalCase 读不到）；直接点保存 → `repeat_violation_extra_penalty` 从真实 -1 被静默改为 +1（惩罚变加分）；judge 区块 5 字段与 reputation 2 个质量阈值「保存 200 假成功」（PatchConfig 无处理分支）。本复核实测 GET 返回 PascalCase + 默认值 +1。
- **根因**：`config/config.go:255+` 各嵌套结构体只有 mapstructure 无 json tag；`admin.go:468-530` PatchConfig 缺 judge/quality_* 分支；`admin/config/page.tsx:48-55` defaultConfig 兜底 + buildPatch 全量提交。
- **修复方案**：
  1. config.go 各嵌套结构体补 `json:"snake_case"` tag（与 mapstructure 同名）——一次修复 GET 序列化；**doc-validator --fix** 同步；
  2. PatchConfig 补 judge 五字段与 reputation quality_* 两字段分支；
  3. 前端：加载失败或字段缺失时**禁用保存**（去除默认值兜底语义——显示「加载失败重试」而非假数据）。
  4. **（Phase 6 必补，F-115）** 放宽 `admin/config/page.tsx:330` repeat_violation_extra_penalty 输入框的 `min={0}`（惩罚语义允许负值，如 min={-10}）——否则补 json tag 后页面显示真实值 -1，原生 `:invalid` 校验（prepareSave）会判定非法并**阻断一切保存**（报 admin.config.invalidValue），本修复会把「静默污染」变成「完全锁死」。Phase 6 实测补充：`data/config_override.yaml` 当前不存在（污染尚未发生），链条四环（PascalCase→默认值兜底→全量提交→SaveOverride 落盘持久化）全部就绪。
- **优先级与顺序**：P1；独立（面试前至少做 1+3 防污染，2 可随后）。
- **建议车道**：heavy（生产配置语义；TDD：GET json 断言 snake_case；PATCH judge 字段生效断言）。
- **验证标准**：`go test ./...`（config 序列化测试）；浏览器打开页面显示真实值（pass_threshold 0.6）、保存后不漂移；`npm run build`。
- **风险与回归面**：config 结构体 json tag 影响所有以 json 序列化 config 的路径（grep 确认 PublicConfig 之外无直接序列化点）；config_override.yaml 回放不受影响（mapstructure 路径）。

#### FIX-34 admin 防护包（存在性校验 + BanUser 守卫 + restore 修死 + trash UI）
- **问题背景**：[A/C] P2；F-073、F-074、F-041
- **场景**：①`POST /admin/contents/999999/ban` → 200 假成功（污染审计日志）；②API 可 ban admin 或自己（后台锁死，前端仅按钮禁用）；③restore 不清 deleted_at——恢复的内容 status=published 却仍在回收站且前台不可见；④trash/restore 两端点无任何 UI。
- **根因**：admin.go:253-262（Updates 无 RowsAffected/前置 First）；:284-308（无 role/self/存在性守卫）；:265-282（updates 无 deleted_at）。
- **修复方案**：①BanContent/RestoreContent/BanUser/UnbanUser 前置 First 校验 → 404 `*_NOT_FOUND`（BanContent 顺带 RowsAffected 断言；**Phase 6 补，F-114：ResolveReport 同样无存在性校验**[admin.go:765-777，不存在 ID+合法 body → 200 假成功+审计行]——纳入本清单；ResolveAppeal 已有 First+404 可照抄）；②BanUser 拒绝 `role=admin` 目标与 self → 400 `ADMIN_TARGET_FORBIDDEN`/`SELF_BAN_FORBIDDEN`；③RestoreContent updates 加 `deleted_at: nil`（同事务）；④admin/contents 加「回收站」视图（GET /admin/contents/trash 已有 + restore 按钮 ConfirmModal）。
- **优先级与顺序**：P2；无依赖。
- **建议车道**：heavy（admin 安全防护；TDD：999999→404、ban admin→400、restore 后 deleted_at IS NULL）。
- **验证标准**：单测；curl 错误路径；浏览器回收站恢复全链路（恢复内容前台可见）；`go test ./...`。
- **风险与回归面**：restore 清 deleted_at 后内容重新进入 published 缓存/索引——配合 FIX-38 缓存失效与 TopicContentPublished 事件（restore 应同样发事件喂 indexer，顺带补）。

#### FIX-35 admin 检索与运维增强包
- **问题背景**：[C] P2/P3；F-075、F-079、F-078、F-040
- **场景**：①/admin/users 搜索只过滤当前页 20 条、无 ORDER BY（翻页可能重复/漏）；②audit-logs 动作下拉硬编码 15 项，缺 llm_config_*/dlq_replay/rag_rebuild/broadcast_notification 等；③LLM「测试连接」失败只显 fail 图标无原因、Activate 任何错误统一 404；④queue 页 stats/dlq `.catch(()=>null)` 吞错伪装空态、DLQ replay 无按钮。
- **根因**：admin.go:330-345（无 search/order）；audit-logs/page.tsx:92-108；agent-config/page.tsx:115-124 + admin.go:679-696；queue/page.tsx:42-47,152。
- **修复方案**：①ListUsers 加 `search`（username/email ILIKE）+ `ORDER BY id DESC`，前端改服务端查询（与 ListAppeals 的 status 筛选一并，接 FIX-27⑤）；②audit-logs 动作改后端 `GET /admin/audit-logs/actions`（distinct）下拉；③test 失败展示后端 message（SafeErrorResponse 已返回安全文案）；Activate 区分 404/500；④queue 页错误态展示+重试，DLQ 卡片加「重放」按钮（POST /admin/queue/dlq/:id/replay，ConfirmModal + 结果刷新）。**（Phase 6 必补）⑤ DLQ 字段契约先对齐（F-113）**：前端 DLQEntry 期望 `{topic,retry_count,trace_id}`（queue/page.tsx:21-29），后端实际返回 `{original_topic,original_id,consumer_group,attempts,error,failed_at}`（dlq_worker.go:102-111，无 trace_id）——当前 18 条死信卡片 topic 全空白，不对齐则 replay UI 建成仍不可用；另注意 Replay 语义是「重投**不删**原条目」（dlq_worker.go XAdd 后原条目留 DLQ），UI 需刷新/标记。⑥（次要，F-117）admin/dashboard 的 `.catch(()=>null)`（dashboard/page.tsx:40）同族吞错一并处理。
- **优先级与顺序**：P2；无依赖。
- **建议车道**：light（纯 admin UI + 小 handler 扩展；audit actions 新端点轻量）。
- **验证标准**：浏览器搜索跨页命中、replay 后条目消失/重试；`go test ./...`（新端点契约）；`npm run build`。
- **风险与回归面**：ListUsers 加参数向后兼容（无参=现状+order）。

#### FIX-36 判官体验包（内容预览 + 考试防作弊 + verdict 契约 + queue 分页）
- **问题背景**：[B/A] P1/P2；F-016、F-053、F-054、F-080
- **场景**：①判官按「内容 #177」盲投（ReviewCard 只有占位文案，看不到内容本体——违反 ui-spec /judge/queue「案例详情+投票」）；无跳过/举报升级；已投案件仍出现在队列；②考试单题通过（1/1=100%）+提交时重新抽题（题库>10 时按错题集评分）；③verdict API 返回裸 votes（前端 judge_name/upvotes 契约不符，点赞 NaN）；理由可自赞、匿名可读；④queue 固定前 20 条，超量显示「已完成」。
- **根因**：ReviewCard.tsx:71-75；judge_service.go:77-78（total=len(answers)）+:55（重抽）；judge.go:124-136（裸 votes）+:138-166（VoteReason 无守卫）；judge_repo.go:72-79（无 voted 过滤）；queue/page.tsx:43。
- **修复方案**：
  1. ReviewCard 内嵌**受控内容预览**：登录态 + 内容预警横幅 +「点击加载」（GET /contents/:id——依赖 FIX-12 为持资格判官放行 under_review 读取）+ 案件类型标签 + 「跳过本案」（前端 currentIndex，顺带后端 ListOpenCases 排除本人已投）+「举报此内容」入口；
  2. 考试：SubmitExam `total=服务端下发题数`（10）；抽题会话绑定（最小实现：GET /exam 时把题 ID 集写入 Redis `judge:exam:{uid}:{category}` TTL 15min，SubmitExam 按会话题集评分）；已有资格再考返回已持资格提示；
  3. verdict 聚合：GetVerdictDetail 返回 `votes:[{...judge_name, upvotes, downvotes}]`（join users + reason votes 聚合）；VoteReason 加判官资格校验 + 禁自赞（vote owner==caller 409）；verdict 路由 authReq；
  4. queue 分页（page 参数透传 + 「加载更多」；后端 judge.go:92-93 已支持 page，纯前端接线）。
  5. **（Phase 6 必补，F-100[P1]）GetExam 序列化改 allowlist**：现仅 `delete(data, "correct_key")` 单键清洗（handler/judge.go:50-57），question_data 其余字段原样下发——出题调度器（scheduler/judge_question_sync.go:55-69）生成的题内嵌 votes_approve/votes_reject，多数派即可**机械推算正确答案**；admin 手工题 explanation 常含答案明文。仅靠 ② 的会话绑定挡不住机械算答案，必须只透出 question/options/content_type。
  6. 考试 Redis 会话 TTL（15min）过期的兜底行为：返回「会话过期，请重新抽题」，不得按新题集评分。
  7.（决策点）③ 中「verdict 路由 authReq」是匿名可见性收紧——business-rules 未明示结案详情是否公开，执行前需产品确认（保守替代：维持 optAuth、仅给 VoteReason 加 authReq+资格校验）。
- **优先级与顺序**：P1（①是众裁可信度核心）；依赖 FIX-12。
- **建议车道**：heavy（审核链路 + 内容暴露边界；TDD：单题不通过、verdict 契约字段、自赞 409）。
- **验证标准**：浏览器判官全旅程：考试→队列→看到内容预览→投票→verdict 页理由/点赞正常；`go test ./...`；预览需截图（screenshots/）。
- **风险与回归面**：内容预览的媒体加载必须「点击后」加载（F-016 安全五项结论）；考试 Redis 会话在本地多实例下用共享 Redis 无问题。

#### FIX-37 reputation 收敛与激励接线
- **问题背景**：[A/C] P2；F-066、F-068
- **场景**：①任意人可查他人处罚日志（/users/:id/reputation 无 self/admin 校验，reason 如 ai_violation 对外暴露）；②自己的加减分明细无 UI 入口（/reputation-logs/me 0 消费）；③标签建议被认可 +1（business-rules.md:26）未实现。
- **根因**：user.go:210-236；grep reputation-logs 前端 0；tag_service.go:101-124 无 reputation 调用。
- **修复方案**：①GetReputation 限 self/admin（403 FORBIDDEN）；②settings 或 rehab 页加「信誉明细」区（分页列表 + 加减分原因 i18n 映射 ai_violation/judge_error_rate/rehab 等）；③ApproveTagSuggestion（action=add 且建议者≠作者）→ +1 reputation_log。
- **优先级与顺序**：P2；③依赖 FIX-22（审核页可达）弱关联。
- **建议车道**：heavy（信誉分语义）；①单独可 light 先行。
- **验证标准**：TDD：他人查 403、本人 200；标签 approve 后建议者 reputation_log +1；浏览器明细页；`go test ./...`。
- **风险与回归面**：用户主页若已有信誉入口（ReputationDetail 组件在 rehab）不破坏；reason i18n 映射表进 messages。

#### FIX-38 admin/审核路径缓存失效补全
- **问题背景**：[A] P2；F-045
- **场景**：admin ban/restore、申诉批准、举报 auto-hide 均不失效内容详情缓存（TTL 300s）——封禁内容最长 5 分钟仍可读（预热缓存后实测复现于域笔记 A-11；本复核代码级确认四处写点无 invalidate）。
- **根因**：invalidateContentCache 仅 content_service.go:922/955（作者路径）；admin.go/social_service.go/appeal 路径直接 Updates 不经过 ContentService。
- **修复方案**：提取缓存失效为共享 helper（如 repository 层 `InvalidateContentCache(rdb, id)` 或 admin handler 注入 ContentService），四个写点统一调用：BanContent/RestoreContent/ResolveAppeal(target=content)/Report auto-hide；FIX-10/12 涉及的状态写点一并覆盖。
- **优先级与顺序**：P2；与 FIX-10/12/34 同域可合并 PR。
- **建议车道**：heavy（审核生效及时性）。
- **验证标准**：TDD：预热→ban→立即 GET 返回 404/banned；`go test ./...`。
- **风险与回归面**：helper 需处理 rdb==nil（本地无 redis 不 panic）。

#### FIX-39 Agent/搜索打磨包
- **问题背景**：[C] P2/P3；F-070、F-071、F-095、F-083
- **场景**：①Agent 429 限流错误码被丢弃，统一「本次请求未完成」+重试（窗口内必败）；②SSE start/usage 事件无消费（trace_id/token 用量不可见）；③Agent 回答纯文本渲染（** ** 与代码围栏原样展示）；④搜索建议端点闲置（前缀 ILIKE 查不到中文子串）、users/search 无通用入口、SearchAgentInput 用 ArrowLeft 语义误导；⑤public config 前端缓存永不失效（运行时切 flag 视图不一致，F-083 真实根因）。
- **根因**：agent-stream.ts:133；AgentWorkspace.tsx switch 无 start/usage case + 消息体 whitespace-pre-wrap；search_repo.go:89-130 LIKE prefix%；public-config.ts:50-52。
- **修复方案**：①onError 透传 code；AgentWorkspace 对 AGENT_RATE_LIMIT_EXCEEDED 显示专属文案（含窗口提示）+隐藏重试按钮；②done 折叠区展示本轮 token 用量 + 错误框附 trace_id；③助手消息接入受控 MarkdownRenderer（禁原始 HTML）；④GlobalSearchInput 接 suggestions 防抖下拉（后端 ILIKE 改 contains `%q%`——中文子串可用）；搜索页加 Users tab（复用 users/search）；图标换 Search/Sparkles；⑤fetchPublicConfig 缓存加 5min TTL。
- **优先级与顺序**：P2；agent 部分（①②③⑤）在开启 agent gate 演示前完成。
- **建议车道**：light。
- **验证标准**：浏览器 429 文案（本地把 per_minute 调 1 复现后还原）；markdown 列表/代码块渲染；搜索建议下拉含中文子串命中；`npm run build && npm run lint`。
- **风险与回归面**：MarkdownRenderer 须禁 script/html（XSS 边界；接入前先核验其现有实现确无 raw HTML 通道）；contains 查询性能（有 GIN 索引的 tsvector 路径不动，仅 suggestions 轻量）；⑤ 5min TTL **不覆盖 fetch 失败的 fail-closed 路径**（AgentFeatureGate.tsx:36-38 网络错误也呈现 unavailable）——应顺带加错误态/重试而非仅禁用文案。

#### FIX-40 UI 打磨包（移动端溢出 + 双 CTA + 死代码清理）
- **问题背景**：[A/C] P2/P3；F-082、F-069、F-096（series 部分）、F-072、F-090
- **场景**：①/original 375px 横向溢出 41px（根因锁定：Sidebar 228px 固定无响应式隐藏，主内容被压至 ~147px）；②series/favorites 空态双 CTA 同屏；③series 管理页标题三处重复 + Delete 危险按钮紧邻编辑区；④components/content/ 两死组件；⑤/ips 排序 most_contents 拼写不匹配 + admin ips submitter_id 字段错名恒空。
- **根因**：Sidebar.tsx:107-109；series/page.tsx:241+256、favorites/page.tsx:190+209；IPBrowseClient.tsx:60 vs ip_repo.go:97；admin/ips submitter_id vs creator_id。
- **修复方案**：①（**Phase 6 降级**：F-082 经 Playwright 375×812 匿名/登录态复测 overflow=0，无法复现，不排除 IAB/WebKit 特有——留 Safari 手工复测备注；`Sidebar` 加 `hidden lg:flex` 作为**防御性可选**保留，非本项重点）Sidebar 加 `hidden lg:flex`（original/home 移动端无侧栏，内容区全宽；检查 home 页是否同型一并修）；②空态出现时隐藏页头 CTA（`length===0 && !showCreate` 条件渲染）favorites 同步；③series 页信息结构简化（详情面板标题去重）+ Delete 移入溢出菜单或加间距；④删除 components/content/{ComplianceCheckBadge,UploadAssistPanel}.tsx；⑤后端 sort 词表加 `most_contents` 别名（或前端改 most_content——推荐后端双收防再漂移）+ admin ips 改 creator_id。
- **优先级与顺序**：P2（①为 D10 规格违例）；无依赖。
- **建议车道**：light。
- **验证标准**：375px 视口 scrollWidth==clientWidth（截图存 screenshots/）；空态单 CTA；`npm run build && npm run lint`；`go test ./...`。
- **风险与回归面**：移动端隐藏侧栏后 trending/导航入口移动端不可达（original 页移动端本就不可用，可接受；ui-spec 如有 original 移动端规格需同步）。

#### FIX-41 发布配置消费化（type_order + upload limits）
- **问题背景**：[B/C] P3；F-085、F-086
- **场景**：发布类型清单/顺序前端硬编码（与 config `publish.type_order_*` 漂移且配置 0 消费者=死配置）；上传上限前端硬编码 500/50/300/20（config limits 是真源且 admin 可运行时改），服务端超限报通用 VALIDATION_ERROR。
- **根因**：studio/publish/*/page.tsx 硬编码；PublishForm.tsx:434；content.go presign 校验错误码。
- **修复方案**：/config/public 扩展返回 `publish.type_order_*` 与 `limits`（脱敏数值无安全风险）→ 前端 ContentTypeGrid 与 FileUploader maxMB 动态读取；服务端超限返回专用码 `FILE_TOO_LARGE`（接 FIX-26 映射）。
- **优先级与顺序**：P3；无依赖。
- **建议车道**：light（public config 字段扩展 + 前端消费）。
- **验证标准**：admin 改 limit → 前端校验即时变化（无需发版）；`go test ./...`（public config 契约测试更新）+ `npm run build`。
- **风险与回归面**：public_config_test.go 契约快照更新；type_order 含 prompt 与否的产品口径同步进 config。

#### FIX-42 版本历史 UX
- **问题背景**：[C] P3；F-089
- **场景**：版本仅 PR 合并产生（CreateInitialVersion/CreateNewVersion 0 调用方）；普通作者内容版本历史恒空，用户无法理解「版本」概念。
- **根因**：version_service.go:32,50 无调用方。
- **修复方案**：CreateContent 成功后建初始版本（v1 full=description）；FIX-13 的编辑路径可选建新版本（最小版先只做初始版本）；VersionHistory 组件空态文案说明「编辑/PR 合并会产生版本」。
- **优先级与顺序**：P3；依赖 FIX-21（版本链语义稳定）。
- **建议车道**：light。
- **验证标准**：发布后 GET /contents/:id/versions 至少 1 条；`go test ./...`。
- **风险与回归面**：存量内容无 v1（懒补：首次编辑时补建或不回填——文档标注）。

#### FIX-43 主列表封禁作者口径统一
- **问题背景**：[A] P2；F-048
- **场景**：banned 作者的 published 内容仍在主列表（本复核实测 1032 在列）；与 related-fanworks 的可见性 scope（排除封禁作者）口径不一致。
- **根因**：content_repo.go:152-161 主分支仅 status+deleted_at；content_visibility.go:7-19 集中 scope 只用于 source-linkage。
- **修复方案**（推荐 A）：A）主列表分支复用 ApplyContentVisibilityScope（JOIN users 排除 is_banned/deleted；IP banned 是否连坐按 business-rules L58——IP banned 时内容已被级联置 banned，无需额外 JOIN ips）；B）明确「封号不连坐」并反向改 scope（规格变更，不推荐）。产品口径需在 business-rules.md 补一句明示。**（Phase 6）与 FIX-12 合并为统一 visibility scope 时必须包含 is_public 维度（F-097，见 FIX-12 修订）——只加「封禁作者排除」会漏掉私密内容泄露。**
- **优先级与顺序**：P2；与 FIX-12 同域（可见性体系）可同批。
- **建议车道**：heavy（主 feed 查询语义）。
- **验证标准**：TDD：封禁作者内容不出现在 /contents 与 /original 列表；性能（主列表是热路径——JOIN users 走主键索引，回归 list 延迟）；`go test ./...`。
- **风险与回归面**：主列表 SQL 变化影响推荐流/搜索的数据源（recommendation/search 若独立查询需同步口径——grep 确认搜索 repo 的列表过滤分支）。

#### FIX-44 Agent SSE 流式解析跨 chunk 行缓冲（Phase 6 新增）
- **问题背景**：[A] P1；F-109（Phase 6 新增）
- **场景**：Agent 对话流式回答期间，事件行跨网络分块边界时被静默丢弃——表现为答案偶发缺字、引用卡缺失、conversation_id 未回填；done（含全量 answer+citations，数 KB 单行）与 citation（含 excerpt）最易中招。
- **根因**：`frontend/lib/agent-stream.ts:144-152` 每个 `reader.read()` chunk 独立 `split("\n")`，无跨 chunk 缓冲——前半段 JSON.parse 失败、后半段不以 `data:` 开头，两段都被丢弃。后端每事件单次 Write+Flush（agent_sse.go），分块边界完全取决于网络；现有测试只覆盖空流/整行流，无分块用例。
- **修复方案**：读取循环改行缓冲——chunk 追加到 buffer，按 `\n` 切分后仅处理完整行，尾部残行留待下一 chunk；流结束时处理残留 buffer（设 buffer 上限防恶意超长行，超出断流报错）。补分块单测（同一大事件拆 2~3 个 chunk 注入断言完整解析）。
- **优先级与顺序**：P1；agent 是简历主线，与 FIX-39 同批（开启 agent 演示前完成）。
- **建议车道**：light（纯前端解析层 + 单测）。
- **验证标准**：分块单测通过；本地 agent 流式对话抽查无丢字/丢引用；`npm run build && npm run lint`。
- **风险与回归面**：与 FIX-39② 的 usage 展示共用事件管线，注意回归。

---

## 3. 尾部

### 3.1 建议 issue 拆分清单（每个修复项一票）

> **2026-09-01 已立票（模块化版）**：本清单已按代码所有权重组为 11 个模块 issue 发布于 GitHub——总览 #209（模块表/依赖矩阵/共享文件协调表/执行纪律），SP-01~SP-11 = #210~#220（label `ready-for-agent`，每个 issue 完整携带问题背景/根因/方案/验证标准）。执行以模块 issue 为准；下表保留作单修复项验收速查。

| Issue 标题 | 验收标准（摘要） |
|---|---|
| 判官考试 answer_key→answer 字段对齐（FIX-01） | UI 全对提交 passed=true；contract 测试覆盖 |
| stats IP 统计改 approved（FIX-02） | 清缓存后 /stats/summary ips=DB approved 数 |
| 判官 closed 词表统一+撤权+奖励接线（FIX-03） | 误判撤权单测过；结案横幅渲染；多数派 +1 log |
| admin reports 契约修复+错误透传（FIX-04） | 成立/驳回 UI 成功；400 场景有红字 |
| Agent 对比度 AA 修复（FIX-05，Phase 6 修值） | 气泡/引用卡/暗色 primary 按钮双主题 ≥4.5:1；design-system 同步 |
| trending 契约统一（FIX-06） | 原创页热搜渲染且跳正确实体 |
| 我的收藏错链修复（FIX-07） | 两处侧栏入口落 /studio/favorites |
| admin 入口与 notifications 导航（FIX-08） | admin 可见入口；broadcast 页在 NAV |
| 本地 worker 启动闭环（FIX-09，Phase 6 收缩） | Step2 补 worker 行；启动前积压已清理；lag=0 |
| 判官闭案回写内容状态+通知（FIX-10） | 19 票闭案→内容 published/banned+作者通知 |
| 举报 auto-hide 触发众裁（FIX-11） | 达阈值 → judge_cases 新行 |
| 内容详情可见性收口（FIX-12） | 匿名读 banned 404；作者/admin 200 |
| 编辑再审核+banned 禁改（FIX-13） | 编辑后 ai_review_records+1；banned PATCH 403 |
| studio 全状态可见+编辑删除申诉入口（FIX-14） | 作者可见 pending→banned 全态+徽标+操作可用 |
| 封禁用户申诉出路（FIX-15） | banned 会话封禁屏可达+account 申诉提交/批准解封 |
| 封禁理由全链路（FIX-16） | admin 填 reason 落库+作者可见 |
| content_status 通知矩阵（FIX-17） | 五类状态转移各有作者/相关人通知 |
| 讨论发帖回复统一审核门（FIX-18） | 低信誉经 /ips/:id/discussions 403；回复通知楼主 |
| 用户资料 email 收敛（FIX-19，Phase 6 扩展） | 任意端点（含 Author Preload 出口）不含他人 email；本人/admin 有 |
| 注销改软删（FIX-20） | deleted_at 语义；存量修正迁移幂等 |
| PR 后端语义修复（FIX-21） | new_text 落库；accept 后 merge 200；正文同步 |
| PR 入口+studio 收口（FIX-22） | 贡献者 UI 全链路提交→合并；空壳页消除 |
| IP 创建者状态可知性+加固（FIX-23，Phase 6 重界定） | 我的 IP 状态可见+过审通知；POST /ips 有 guard；tags 审核与上限 |
| IP 拒绝原因落库（FIX-24） | ip_review_logs 有行+创建者可见 |
| IP 详情可见性+缓存（FIX-25） | 匿名读非 approved 404；决策即时生效 |
| 错误码映射扩展（FIX-26） | 高频码专属文案；冻结含指引 |
| 申诉质量包（FIX-27） | 假目标 404；重复 resolve 409；admin_response 渲染 |
| 举报反馈闭环（FIX-28） | reports/me+处理通知+评论举报入口 |
| 评论体验包（FIX-29） | 作者名正常；回复/编辑/删除可用；折叠生效 |
| 私信可达性（FIX-30，①改纯前端） | 任意用户发起对话（后端不动）；未读同步；摘要通知 |
| 通知细节包（FIX-31） | 深链正确；dropdown 已读；broadcast 键 |
| 401 静默刷新+保参（FIX-32） | 过期自动恢复；redirect 保 query |
| admin config 契约（FIX-33） | 真实值显示；保存不污染；judge 字段生效 |
| admin 防护包（FIX-34） | 999999→404；admin/self 禁 ban；restore 清 deleted_at |
| admin 检索运维包（FIX-35） | 服务端搜索；DLQ replay 可用；错误不吞 |
| 判官体验包（FIX-36） | 预览可见可跳过；考试防作弊；verdict 契约 |
| reputation 收敛+激励（FIX-37） | 他人 403；明细页；标签 +1 |
| 缓存失效补全（FIX-38） | ban 后立即生效 |
| Agent/搜索打磨（FIX-39） | 429 文案；markdown；建议下拉 |
| UI 打磨包（FIX-40） | 375px 无横向滚动；单 CTA；死代码删除 |
| 发布配置消费化（FIX-41） | 前端从 config 读类型顺序与上限 |
| 版本历史 UX（FIX-42） | 发布产生 v1 |
| 主列表封禁作者口径（FIX-43） | banned 作者内容不在公共列表；口径入文档 |
| Agent SSE 跨 chunk 行缓冲（FIX-44，Phase 6 新增） | 分块单测通过；大事件跨 chunk 不丢 |

### 3.2 明确不修项及理由

| 项 | 理由 |
|---|---|
| @提及 mention 通知（触达矩阵 #23） | 无正文 @ 解析基础设施；Phase 2（#208 范围），需要先立 spec |
| 私信 SSE 实时推送（business-rules.md:143 承诺） | FIX-30 以 30s 轮询最小实现；SSE 属 #208 Agent 技术深化/实时性范围 |
| 反馈/申诉通知文案 locale 模板化（F-065/F-077 硬编码中文部分） | 现有通知 title 均为后端中文模板，统一 locale 模板是跨子系统改造，Phase 2；FIX-31 仅修语义错误项 |
| 举报信誉分自动加减（有效举报 +1 / 恶意举报 -2 全自动判定） | admin dismissed≠恶意举报，全自动有误判风险；FIX-28 先做通知闭环，评分规则待产品确认后立项 |
| archive scan 作者侧完整区分文案（F-049） | `archive_malware_scan_enabled=false` 且依赖 ClamAV 真实签名库（AGENTS #148/151 边界）；随恢复生产/开启 flag 时再做 |
| 登录验证码门 UI 与独立 TTL（F-046 的 CaptchaWidget/计数器 15min 部分） | 本地 captcha provider=bypass、且被 credential 限流（5/min）掩蔽，门实际不可达；文案映射已入 FIX-26；生产语义（真实阿里云验证码 + 独立 TTL 配置）随真实凭证恢复时一并做（本地开发模式例外条款） |
| 粉丝/关注列表独立 UI（F-093.4） | API 已备、无产品入口设计；粉丝分析页（/studio/followers）已覆盖主要诉求；Phase 2 社区扩展 |
| 全新浏览器默认 locale 策略（F-096 部分） | 按 Accept-Language 协商是 next-intl 标准行为；默认改 zh 优先是产品决策，需用户确认后一行可改（i18n/routing.ts） |
| 判官评论类争议案件（评论 under_review 无判案） | 依赖评论审核状态机扩展（FIX-29 仅做折叠展示）；Phase 2 |
| `/studio/revenue` 常驻入口隐藏（E-19 建议） | 文案诚实且 feature flag 语义正确；属产品取舍，不属缺陷 |
| PRD「已注销用户内容标记 author_deleted」完整链路（Task 103 后半） | FIX-20 的软删+visibility scope 已达成不可见效果；author_deleted 显式标记随 Phase 2 内容状态机重构 |

### 3.3 执行纪律提醒（对后续执行 Agent）

1. 所有涉及 config.go / migrations / routes.go 的修复项（FIX-10/15/16/20/25/33/35/36）提交前必须 `cd tools/doc-validator && go run . --fix`。
2. heavy 车道一任务一 worktree 一分支一 commit，先写失败测试。
3. 触达类修复（FIX-17/28/31）验证前先按 FIX-09 启动 worker（并先清理积压，见 §0.2 操作性提醒）。
4. 执行任何 FIX 前先读 §0「Phase 6 修订记录」与该条目内「Phase 6」标注——FIX-05/09/19/23/30 的原正文已被修订取代，FIX-30① 原文**禁止执行**。
5. 本文档预计 2026-11-01 失效；批次 E 未完成项在 2026-09-03 面试窗口后按活计划注册表排期，不自动成为任务来源（须先登记）。
