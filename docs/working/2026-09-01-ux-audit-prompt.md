# OmniCraft 全站功能设计与 UI 设计审查 —— Agent 执行提示词

> **使用方式**：将本文件全文作为任务指令交给执行 Agent（可整段复制）。
> 创建日期：2026-09-01 ｜ **预计失效日期**: 2026-11-01（失效前请对照最新代码重新校准 §4「已知代码事实」）。

---

## 0. 你的角色与使命

你是一名同时具备产品思维、交互设计功底和全栈代码阅读能力的资深审查者。你的任务是对 OmniCraft（万象工坊，Next.js 前端 + Go/Gin 后端 + PostgreSQL）做一次**全量、以真实用户视角驱动**的功能设计与 UI 设计审查，并在审查结束后把全部发现交给一个全新的子代理做独立复核与修复方案。

你不是来复述文档的。项目的文档描述的是「应该是什么」，你要回答的是「实际是什么、用户真实走一遍会发生什么」。因此每一条结论都必须建立在**实际代码（`文件路径:行号`）+ 实际运行（浏览器操作 / curl 调用 / 截图）**之上。

工作分两条主线，最后合并为一份报告，并追加一个收尾阶段：

1. **功能设计审查**：以用户旅程为单位，穷举每个功能从起点到所有分支终态的完整链路，找出走不通的路、用户无法获知的关键信息、缺失的出口和可优化的交互。
2. **UI 设计审查**：对全部页面逐页走查按钮位置、元素尺寸、颜色对比度、间距、状态完备性等，找出不符合设计原则和设计规格的问题。
3. **复核与修复方案（收尾，见 §10）**：整合全部发现，交给一个**未参与审查、不带任何本会话上下文的新子代理**，逐条验证每个问题的真实原因，并产出一份完整修复方案（问题背景 / 场景 / 根因 / 优先级 / 方案 / 验证标准）。

---

## 1. 审计纪律（不可违背）

1. **只审计，不修复**。本次任务不修改任何业务代码、不勾选任何计划文件的 checkbox、不提交功能 commit。你唯一允许写入的是：审查报告、审计截图、审计用的临时笔记、progress.txt 的任务记录，以及交给子代理的交接问题清单；子代理按 §10 额外产出复核结论与修复方案文档，同样不触碰任何业务代码。
2. **代码优先于文档**。architecture.md / docs/reference/* / design/* 用来理解设计意图；但「文档说有、代码没有」「代码有、文档没说」「文档与代码矛盾」三种情况都必须如实记录为发现，不允许只看文档下结论，也不允许把文档当挡箭牌掩盖实现缺陷。
3. **每个发现必须标注三类之一**：
   - **[A] 规格不符**：ui-spec / design-system 已有定义，实现与之不一致；
   - **[B] 规格缺失**：规格文档没覆盖此处，或已过时（例如 ui-spec 仍以 `/dashboard/*` 为准而实际已迁移 `/studio/*`）；
   - **[C] 设计可优化**：规格与实现一致，但按用户视角和现代设计原则仍有更好做法。
4. **不臆造**。「找不到入口」不等于「功能不存在」。下「不存在」结论前，必须穷尽：前端路由文件、组件目录、i18n 词条（frontend/messages/zh.json 与 en.json）、后端 routes.go 与 handler/service。报告中写明你搜索过的位置。
5. **本地开发模式语境**。真实外部凭证（阿里云 Green、OSS、SMTP、LLM）缺失不阻塞，系统走 fail-open / mock 路径。你遇到的审核结果都来自本地语义；对「真实阿里云会如何判定」的分支，按 PRD 三级审核语义（architecture.md §3.4）推演并标注「推演」，不冒充实测。
6. **范围边界**：以 Web 为主。Tauri 桌面端整体暂缓（`features.desktop_deploy_enabled=false`），只审 Web 端对桌面能力的宣传与入口是否如实（如 /client 下载页），不审桌面功能本身。支付（`/payments/*`）与一键部署（`/deploy-grants`）已知返回 503 `FEATURE_DISABLED`，这本身不算缺陷，但这些功能的前端入口是否存在、点击后的用户预期是否被管理（明确提示「暂未开放」还是裸报错），要审。
7. 遵守仓库 AGENTS.md 的一般纪律：i18n、错误不裸暴露、软删除等项目规则既是审查基准，也约束你自己的产出（报告用中文写）。

---

## 2. 基线阅读清单（按序读完再动手）

1. `AGENTS.md` —— 工作流、车道、文档权威源与冲突优先级。
2. `architecture.md` §1（产品定位）、§3（页面路由清单 + API 路由清单 + 三级审核链路图）。
3. `docs/reference/business-rules.md` **全文** —— 信誉分体系、赛博判官、原创/二创来源联动、上传限制、studio 规则、自动风控阈值。
4. `design/design-system.md` **全文** —— 唯一设计 token 权威；注意末尾的「暗色模式检查清单」。
5. `design/ui-spec.md` 的 Section Index（约 L14 起）—— 建立规格地图（49 个 Page + 72 个 Component 条目）；后续逐页审查时用 `grep -A 60 "## Page: /xxx"` 取对应规格，它是该页面的唯一视觉权威。
6. `docs/specs/studio.md`、`docs/specs/web-agent-v0.4-mvp.md`、`docs/specs/recommendation-page.md` —— 对应功能域的设计输入。
7. `backend/internal/router/routes.go` —— 完整 API 面。
8. 后端状态机与审核链路源码（功能审查的核心证据区）：
   - `backend/internal/service/review_service.go`（AI 审核结果应用、judge case 创建、重复违规惩罚）
   - `backend/internal/service/appeal_service.go` 与 `appeal_helpers.go`
   - `backend/internal/service/judge_service.go`
   - `backend/internal/service/content_service.go`（创建/更新的守卫与状态不变量）
   - `backend/internal/model/{content,ip,user,social,review,judge,version,collab_invite,feedback}.go`（状态枚举）
   - `backend/internal/middleware/auth.go`（封禁用户的 401 `USER_BANNED` 拦截范围）
   - `frontend/app/(protected)/layout.tsx`（客户端登录守卫与封禁屏）

---

## 3. 环境准备（必须实际运行，不接受纯静态审查）

```bash
cd backend && go mod tidy && cd ../frontend && npm install   # 首次
docker compose up -d postgres redis
cd backend && go run cmd/server/main.go &                     # :8080
cd frontend && npm run dev &                                  # :3000
```

- 用你可用的浏览器自动化工具（Playwright MCP / browser-use / computer-use 均可）逐页操作。所有截图存 `screenshots/audit/`，命名 `<页面>-<主题>-<视口>.png`（例：`agent-dark-1440.png`）。
- UI 走查必须覆盖 **亮色 + 暗色两种主题**（项目用 next-themes）与 **两个视口：桌面 1440 与移动 375**。
- 测试账号：优先走真实注册流程（注册流程本身是被审对象）；若被验证码拦截，检查 `backend/config.yaml` 中 captcha 开关（本地 dev 通常可关闭），或使用库里已有测试账号。**允许用 psql 直接改本地库构造罕见状态**（banned 内容、publish freeze、under_review、封禁账号等）来覆盖平时走不到的分支——把你做的每一步数据构造记录进报告附录。
- 后端接口验证用 curl，正常路径与错误路径都要打。

---

## 4. 已知代码事实（初步探查断言 —— 引用前必须逐条亲自验证）

以下是预探查得到的结论，**用于把你的注意力指向正确位置**。你必须亲自读代码/实测验证后才可写进报告；若与你的验证结果矛盾，以你的验证为准并明确指出。

| # | 断言 | 验证入口 |
|---|------|----------|
| F1 | 内容状态机：`pending → published \| under_review \| banned`；`banned` 对 AI 通道是**终态**（迟到的异步结果不能复活 banned 内容）；仅 admin restore 与申诉批准两条路能回到 `published` | model/content.go、review_service.go `applyContentReviewResult` |
| F2 | **不存在 `rejected` 状态**；「修改后重新提交」= 全新 `POST /contents` 新建一行重走全审核；`PATCH /contents/:id` 只改 title/cover/is_public/allow_copy/agent_enabled 且**从不重置 status** —— 意味着已发布内容编辑标题/封面**不再过审核** | content_service.go、handler/content.go |
| F3 | 重复违规惩罚：窗口期（默认 7 天）内违规记录达阈值 → 额外扣信誉分 + Redis 发布冻结（默认 7 天），期间 `POST /contents` 返回 403 `PUBLISH_FROZEN` | review_service.go `applyRepeatViolationPenalty` |
| F4 | 申诉仅支持 `target_type = content \| comment`（**没有账号级申诉**）；同一 target 同时只允许一个 pending（409 `APPEAL_EXISTS`）；admin 批准 content 申诉会把内容**强制置为 published** | model/social.go Appeal、appeal_service.go |
| F5 | 封禁用户：auth 中间件对所有 authReq 路由返回 401 `USER_BANNED`；前端 `(protected)/layout.tsx` 有封禁提示屏并链接到 `/appeals` —— **被封禁用户是否真能到达并使用 /appeals（页面数据 GET /appeals/me、提交 POST /appeals 会不会也被 401 拦截）是本次审查的最高优先级死链检测点** | middleware/auth.go、(protected)/layout.tsx、appeals 页面与 API |
| F6 | 判官案件**只**来自阿里云「争议（review）」判定（`ensureJudgeCase`）；高危 block 内容绝不进判官队列。判官看到的是 under_review 原始内容 | review_service.go、judge_service.go、components/judge/ReviewCard.tsx |
| F7 | 判官闭环：单人单案件一票；总票数 ≥ MinVotes（默认 20）时按 ≥60% 比例闭案；判官错误率超限会撤销资格并扣分 | judge_service.go、config.yaml judge 段 |
| F8 | IP 状态机：`pending → approved \| rejected \| banned`；IP 被 ban 级联下架其下全部内容；stats_service 中疑似把 IP 状态按 `'published'` 统计（与全局 `'approved'` 不一致的疑似 bug） | model/ip.go、stats_service.go |
| F9 | 前端存在**双路由族并存**：`/dashboard/*`（旧，仍可用）与 `/studio/*`（新）；`/publish`、`/dashboard`、`/studio` 为纯重定向 | app/(protected)/ 下目录 |
| F10 | 前端存在**重复组件**：`ComplianceCheckBadge` 与 `UploadAssistPanel` 在 `components/agent/` 与 `components/content/` 各有一份 | 两目录 diff |
| F11 | 已知用户报告的 UI 问题（必须复核并定位根因）：① /agent 对话中气泡背景与文本颜色对比度过低难以看清；② /studio/series 点击「创建系列」后，页面右上角与底部同时出现两个「创建系列」按钮，明显冗余 | components/agent/AgentWorkspace.tsx、app/(protected)/studio/series/ |

---

## 5. 功能审查方法论

对每个实体状态 × 每种角色，回答**五个问题**，任何一问答不上来就是一个发现：

1. **入口可达**：用户从哪里进入这个流程？入口在导航/页面上可发现吗？
2. **状态可知**：流程中用户能否清楚知道「我现在的内容/账号处于什么状态、接下来会发生什么、大概多久」？
3. **结果反馈**：成功与失败都有反馈吗？失败原因是否用人类语言说清（对照后端 ERROR_CODE 与前端 i18n 映射）？
4. **出口完备**：每个终态/中间态之后，用户有没有合理的下一步（重试 / 修改重提 / 申诉 / 放弃）？**没有任何出口的状态 = 死胡同**，按 P0/P1 上报。
5. **触达闭环**：关键状态变化有没有通知（站内通知 / 未读计数 / 邮件语义推演）？用户不登录对应页面就永远不知道结果吗？

配套手段：

- **状态穷举表**：为 content / IP / PR / report / appeal / judge_case / feedback / collab_invite 各画一张「状态 × 角色 × 用户能否看到/能否行动」矩阵，直接暴露不可见或不可行动的格子。
- **API-UI 对照**：routes.go 里每个面向普通用户的端点，前端是否有调用与入口；反向，前端每个按钮是否有对应端点支撑。
- **错误路径实测**：每个表单/提交动作至少打一次错误路径（缺字段、越权、重复操作、限流），看前端呈现。

---

## 6. 必须走查的 14 条用户旅程

### J1 访客与账号生命周期
注册（含验证码、邮箱验证、重发冷却）→ 登录（失败次数与验证码门）→ 忘记/重置密码 → 未验证邮箱用户的使用限制（VerificationReminderCard 的限制范围与提醒频率）→ 修改密码 / 注销账号（确认弹窗、软删除语义、注销后数据呈现）。
重点：`?redirect=` 登录后回跳是否处处有效；token 过期（2h access / 7d refresh）时的前端表现——是静默刷新还是把用户踢到登录页丢上下文。

### J2 内容发布全链路（核心样例，逐分支穷举 —— 用户原始诉求，最细走查）
- **入口**：登录用户从首页/Header/studio 进入 `/studio/publish/original|fanwork`；fanwork 的来源选择（source_original_id / source_fanwork_id 预填与冲突警告）。首页对「发布」的引流是否明显？
- **表单与上传**：ContentTypeGrid 类型选择 → PublishForm；FileUploader 的 OSS 直传；限制（格式/大小/数量，来自 config.yaml）是否在上传前告知用户？超限报错文案是否可理解？
- **提交后 pending 期**：`/studio/contents` 里状态徽标显示什么？有无「审核中」说明与预期时长？有无提交成功的通知？
- **分支 A（AI pass + 附件干净 → published）**：
  - 作者是否收到发布成功通知？内容是否立即出现在 /、/original、搜索、IP 页？
  - **发布后管理**：编辑入口能改哪些字段？结合 F2 验证：**编辑已发布内容的标题/封面是否绕过再审核直接生效**——若是，这既是安全漏洞也是公平性问题（改完违规内容无人再审），按 P1 上报并给出「编辑触发再审核」的方案建议。
  - 删除是软删吗？作者有「回收站/恢复」入口吗（还是只有 admin 有 /admin/contents/trash）？
  - 版本历史（VersionHistory）展示什么？版本只在 PR merge 时产生，还是编辑也产生？用户能否理解？
  - is_public / allow_copy / agent_enabled 开关的即时生效与说明。
- **分支 B（AI 争议 → under_review + 判官审理）**：作者看到什么状态文案？判官闭案（恢复展示 / 不予展示）后作者是否收到**结果与理由**？closed_reject 后的出口（申诉）是否可达？
- **分支 C（AI 违规 → banned 终态）**：
  - **作者从哪里、以何种形式得知「不通过 + 具体原因」**？检查 ai_review_records 的原因字段是否透出到任何前端界面（studio/contents、通知、内容详情）。若只有 admin 可见 → 用户被 banned 却不知为何，P0。
  - banned 内容对作者本人是否可见（可见性 scope 只放行 published——作者看自己的 banned 内容是否 404）？
  - **「改动后再次提交」路径是否存在**？结合 F2：无 rejected 状态、重提=新建。UI 上有没有「基于此内容修改重投」的引导，还是用户只能从零再来？两者体验差异必须写清。
- **分支 D（多次上传违规内容）**：结合 F3 验证：第 N 次违规后触发发布冻结——前端如何呈现 403 `PUBLISH_FROZEN`？是否告知冻结原因与剩余时间？rehab 素质课程是否作为解冻/恢复路径被引导（还是仅作为加分项躺在 /rehab）？**「多次上传淫秽内容→封号」这条链路是否存在**：账号级 ban 只能由 admin 手动执行（POST /admin/users/:id/ban），自动封号阈值是否存在？若不存在，评估：恶意用户可无限重复「上传→被 ban→再上传」的成本是否足够高（冻结 7 天是否可被多账号绕过），如实记录设计现状。
- **分支 E（压缩包带毒 → archive scan blocked）**：内容保持不可发布。作者看到什么？文案是否与「内容违规」（分支 C）区分开（这是文件问题，不是处罚）？
- **分支 F（IP 被 ban 级联）**：其下全部内容被下架时，作者从哪里得知原因？

### J3 IP 创建与管理
`POST /ips` → pending → admin approve/reject。被 reject 的用户能否看到**拒绝原因**？能否修改后重新提交（还是一次死刑）？IP 详情页对 pending/rejected 作者视角的呈现；IP banned 的级联告知；`/ips` 浏览页的筛选排序。顺带验证 F8 的统计 bug。

### J4 PR 协同全链路（GitHub 式护城河，双视角）
- 贡献者视角：从内容详情找到「提交 PR」入口 → DiffViewer 对照 → 提交 → 等待。
- 作者视角：`/studio/pr-requests`（与旧 `/dashboard/pr-requests` 对比）审阅 PR：accept / reject / **manual merge（MergeEditor 手动合并编辑器）**；三个终态后贡献者是否收到结果通知与理由？
- 协作邀请（collab-invites）：发送/接受/拒绝/过期；被邀请人的通知与入口。
- 贡献者管理：屏蔽/解除屏蔽贡献者的后果与告知。
- 检查：PR 链路与版本系统（ContentVersion）的关系是否让用户困惑；diff 三栏对比在移动端的表现。

### J5 投诉（举报）
内容举报与评论举报的入口位置（ReactionBar？详情页？）；举报理由选项；提交后举报者自己**能否在任何地方看到处理结果**（「我的举报」列表存在吗？结果通知存在吗？——若举报完全黑箱，如实记录）；举报阈值自动转 under_review（auto-hide）后当事作者的知情链路；重复举报 409 的 UX。

### J6 申诉（含最高优先级死链检测）
- 常规链路：对 banned 内容/评论提交申诉（/appeals）→ admin 处理（/admin/appeal）→ 结果通知。admin 处理界面给出的信息（违规原因、审核记录、举报上下文）是否足以做出决策？
- **死链检测（P0 候选）**：结合 F4/F5 完整走一遍「账号被封禁的用户想申诉解封」：
  1. 被封账号登录时前端显示什么？（protected 布局的封禁屏是本地渲染还是依赖被 401 拦截的 API？）
  2. 封禁屏上链接的 /appeals，点击后页面能加载吗？GET /appeals/me 会不会被 auth 中间件 401 拦下？
  3. 即便页面能开，申诉表单支持 target_type=account 吗？
  4. 结论模板：「被封禁用户【能/不能】通过任何产品内路径发起账号解封申诉」。若不能 → P0，并注明用户唯一出路是什么（邮件？反馈工单 feedback？）。
- **信息充分性**：提交 content 申诉时，用户**能否在同一页面看到「该内容为何被封」的事实与原因**（用户原话：「能否在申诉功能页找到他『被封禁』这个事实，并用这个事实进行申诉」）；普通用户如何知道 target_id（手填？从内容页一键申诉？）。
- admin 批准申诉即强制 published 的边界：若内容确属违规而被误批，有无复核机制？记录为设计风险。
- APPEAL_EXISTS 的提示质量。

### J7 赛博判官（含内容安全暴露检测）
- 考试：/judge/exam 按内容类型抽题、80% 通过、资格生效；考试中途退出/反复重考有无限制；题库不足（503 INSUFFICIENT_QUESTIONS）的 UX。
- 队列：/judge/queue 只展示有资格类型的 open 案件。
- **内容安全暴露检测（P1 候选，用户原始诉求）**：结合 F6，判官审理的是 Green 判为「争议」的内容——**可能包含软色情、暴力等敏感内容**。逐项检查 ReviewCard 与案件详情：
  1. 封面图/媒体是否原样直出？有无模糊/打码、点击确认后才加载、内容预警横幅、案件类型标签？
  2. 有无「跳过此案件」选项让判官自主回避不适内容？
  3. 判官能否举报案件内容本身（发现更严重违规时的升级通道）？
  4. 评论类争议案件是否同样直出？
  若以上皆无 → 判官被迫直接观看敏感内容且无法回避，按 P1 上报，给出分级/遮罩/跳过机制的方案建议。
- 投票与闭环：投票+理由提交；20 票 60% 闭案；verdict detail 结果页；对他人理由的赞同/反对投票。
- 判官误判率撤销资格：被撤销者本人如何得知（通知？静默？）；判官激励与信誉分联动的用户感知。
- 准入审计（用户原始诉求：「系统是否能判断哪些内容适合交给判官」）：验证 ensureJudgeCase 的准入条件确实只有 Green review 级；评估该分界是否合理（例如低龄向内容争议是否应直接交 admin）。

### J8 社区互动
评论（发布/编辑——编辑后是否再过审核、删除）→ 回复列表；讨论区（IP 维度贴吧式：发帖/回复/置顶——置顶权限）；点赞点踩；关注用户/IP；私信（发送走 Green gate——本地 fail-open 下有无提示语义；会话列表/删除消息/离开会话）；通知（频道 Tab、未读计数、全部已读、下拉与全页一致性）。

### J9 收藏 / 系列 / 历史 / 搜索 / 标签
- 收藏集：创建/增删条目/公开性（公开收藏集可被浏览）/与「收藏」按钮的状态联动（is_favorited 以成员关系为源）。
- 系列：创建/候选内容/增删/拖拽排序；**复核 F11-② 双按钮问题并检查全站是否还有同类「同屏重复 CTA」**；公开系列页 /series/[id] 的导航（SeriesNav 上一件/下一件）。
- 浏览历史：分组、多选删除、保留期说明；IP 访问历史（最近 IP）。
- 搜索：faceted 侧栏筛选、保存搜索、建议/热搜、用户搜索；搜索结果空态。
- 标签：标签组管理（/settings/tag-groups）、内容上的标签建议（suggest → admin/作者审核）。

### J10 Agent 工作台
/agent 全页工作台：SSE 流式对话（start/tool_status/delta/citation/usage/done/error 事件的前端呈现）、服务端校验后的站内引用（可点击跳转？）、工具状态摘要、会话侧栏（列表/加载/删除）；upload-assist 发布元数据建议；compliance-check 合规预检（与发布表单的衔接）；usage-guide（?stream）；配额（429 AGENT_RATE_LIMIT_EXCEEDED / 503）时的 UX；feature gate 关闭时的呈现。
**复核 F11-① 对比度问题**：分别在亮/暗主题截图，量化气泡背景与文本的对比度比值（文本要求 ≥4.5:1），定位是哪个 class/token 的问题，检查代码块、引用卡、工具状态行等所有文本表面。

### J11 消息与触达
通知频道划分（系统/互动/审核结果…）与未读数在 Header 下拉和 /messages 的同步；审核类关键事件（内容发布/下架/申诉结果/PR 结果/判官闭案）逐一确认是否产生通知（grep notification_service 的调用点做成矩阵）。

### J12 用户主页与他人视角
/user/[userId]：作品（只显示 published？）、收藏集、讨论、信誉分展示与 ReputationDetail 规则透明度；关注/粉丝列表；自己主页与他人主页的差异是否合理。

### J13 反馈工单
提交（含附件预签上传）→ /feedback/mine 列表 → 详情（回复/附件）→ 状态流转 open→in_progress→resolved→closed 与 reopened 的用户侧展示；admin 回复后的通知。

### J14 管理后台运营者旅程（admin 也是用户）
逐页走查 /admin/*（dashboard、reports、ips、contents、users、appeal、feedback、categories、queue、audit-logs、config、agent-config、notifications）：
- 每个列表页：筛选是否够用、批量操作有无、分页、空态。
- **申诉处理页**：可获得的决策信息是否完整（内容本体、违规原因、历史记录）？
- **内容终审**：ban/restore 的确认流程与误操作防护；trash 恢复。
- admin config 页与 config.yaml 的一致性；LLM 配置的激活/测试闭环。
- 审计日志的查询与展开。
- admin 操作后的用户侧通知联动（与 J11 矩阵合并）。

---

## 7. 横切检查（跨页面、跨旅程）

1. **错误码 → 用户语言映射完整性**：grep 后端全部错误码（response 包 / handler 里的 code 常量），对照前端 i18n 与错误处理代码，列出生僻码会裸出的场景（违反 Key Rule 12「禁止裸错误暴露」）。
2. **i18n 完整性**：抽查每个页面的 zh/en 词条；grep 组件内硬编码中文/英文文案。
3. **角色矩阵**：取 8 个代表页面 × 4 角色（游客 / 普通登录用户 / 判官 / admin），验证渲染内容与拦截行为（未登录访问 protected、普通用户访问 admin、判官功能入口的显隐逻辑）。
4. **通知触达矩阵**：全部状态转移 × 是否通知（见 J11），缺格即为发现。
5. **空态审计**：每个列表/仪表盘页的空态是否有文案 + 引导动作（新用户首次进 studio/messages/history/appeals/judge 的体验）。
6. **幽灵入口**：grep 指向已禁用功能的链接/按钮（支付、桌面客户端下载、一键部署），检查点击后的呈现是否管理了预期。
7. **长内容与边界数据**：超长标题/无封面/大量标签/深嵌套评论的排版；分页控件一致性。
8. **并发与竞态的 UX 面**：重复点击提交按钮、SSE 断线重连提示、乐观更新与失败回滚。

---

## 8. UI 设计审查（第二主线）

### 8.1 范围（页面全集，逐页过）

- **认证/静态（public）**：/login、/register、/forgot-password、/reset-password、/verify-email（含 pending）、/privacy、/terms、/help、/client、/feedback、/home（登出营销页）
- **浏览（public）**：/（首页）、/ips、/ip/[ipId]、/ip/[ipId]/[category]、/ip/[ipId]/discussions（+[discussionId]，并核对 ui-spec L802 的 `/discussions/new` 发帖页规格是否与实现一致）、/original、/original/[contentId]、/original/[contentId]/fanworks、/recommend、/search、/content/[contentId]、/series/[id]、/collections/[id]、/user/[userId]（+/collections）
- **个人（protected）**：/settings（+/tag-groups）、/history、/appeals、/messages、/feedback/mine（+/[id]）、/rehab、/agent
- **Studio**：/studio/publish/original、/studio/publish/fanwork、/studio/overview、/studio/contents、/studio/series、/studio/favorites、/studio/followers、/studio/contributors、/studio/pr-requests、/studio/tag-suggestions、/studio/revenue
- **旧路由族**：/dashboard/*（contents、contributors、pr-requests、tag-suggestions）——与 studio 版逐项对比，找残留不一致
- **判官**：/judge/exam、/judge/queue
- **Admin**：/admin/* 全部 12 页
- 共享面：Header/Footer/Sidebar、ContentDetailOverlay 浮层体系（首页/搜索/agent 多处复用，一次审查覆盖多页）、所有 ui/ 原语

### 8.2 审查维度（每页过 14 个问题）

- **D1 对比度与可读性**：正文文本 ≥4.5:1、大字号 ≥3:1（WCAG）。重点复查：agent 对话气泡（F11-①）、暗色下次级文本与 placeholder、disabled 文案、卡片上的 ghost/secondary 按钮、图表配色。用取色工具量化，不凭感觉。
- **D2 冗余与重复**：同屏重复 CTA（F11-② series 双按钮，复核并全站扫描同类）；同一功能双入口（dashboard vs studio、Header 下拉 vs /messages）；双份组件（F10）导致的视觉漂移。
- **D3 按钮纪律**：一屏一个 primary；主/次/危险按钮层级；危险操作（删号/删内容/封禁）二次确认；按钮尺寸与触控热区（移动端 ≥44px）；提交按钮 loading 态防重复点击。
- **D4 交互状态完备性**（参照 hallmark 的 8 态纪律：default / hover / focus-visible / active / disabled / loading / error / success）：抽查至少 10 个核心交互组件（Button、Tab、下拉、点赞、关注、上传、投票、合并编辑器），缺态即记录。
- **D5 Token 纪律**：grep 前端组件中绕过 design token 的硬编码色值（hex/rgb/任意 Tailwind 色名而 design-system 未定义），列出清单；对照 design-system.md 令牌表核对使用是否一致。
- **D6 间距与节奏**：4/8pt 间距；同类卡片 padding 一致；页面左右留白统一；卡片间距与内部密度的层级感。
- **D7 排版**：字阶使用是否符合 design-system 字阶表；行高与行长（正文 measure）；标题截断（ellipsis/换行策略）；中英混排。
- **D8 三态覆盖**：每页的 loading（skeleton）/ empty / error 三态是否齐备且非阻塞。
- **D9 弹层与焦点**：Modal 焦点陷阱、Esc 关闭、遮罩点击行为、focus ring 可见性（键盘走查一遍核心流程）、ConfirmModal 的文案与按钮语义。
- **D10 移动端 375**：无横向滚动；表格溢出处理；侧栏变抽屉；筛选侧栏（FacetedSearchSidebar 的移动端抽屉）；双列塌陷；触控目标尺寸。
- **D11 导航与信息架构**：Header 信息密度与「最近 IP」；studio 可折叠侧栏的收起态可用性；admin 侧栏；面包屑/返回路径；用户能否始终知道自己在哪。
- **D12 数据可视化**：overview/followers 的图表在空数据、单点数据时的呈现；tooltip 可读性；色盲安全。
- **D13 一致性矩阵**：同类列表页的「筛选栏位置 + 操作列 + 分页 + 空态」是否统一（admin 各页之间、studio 各页之间、admin 与 studio 之间）。
- **D14 现代简约参照（hallmark modern-minimal 流派）**：可加载 `~/.agents/skills/hallmark/SKILL.md`（或 `~/.zcode/skills/hallmark/`）及其 `references/genres/modern-minimal.md`、`references/anti-patterns.md` 作为评审问题来源——检查：装饰是否克制（无滥用渐变/emoji 当图标/无意义阴影卡片）、留白是否支撑层级、信息层级是否一眼可分、文案是否直白。**注意：hallmark 只提供评审视角，不得用它推翻项目自身 token；与 design-system.md 冲突时以项目 token 为准。**

### 8.3 规格符合性核对流程（每页固定动作）

1. `grep -n "## Page: /xxx" design/ui-spec.md` 取该页规格（含 `ui_spec_ref` 指向的组件规格）；
2. 对照实现逐条核（布局结构、组件、尺寸、颜色、状态、交互）；
3. 差异标注 [A]（实现不符）或 [B]（规格过时，如仍写 /dashboard）；
4. 规格未覆盖处用 §8.2 维度审，标注 [C]。

---

## 9. 严重度分级与交付物

**严重度**：
- **P0 阻断**：用户无法完成核心操作，或关键信息（被封原因、申诉路径）完全不可达，或账号级申诉死链坐实；
- **P1 严重**：明显伤害体验或安全（编辑绕过再审核、判官被迫直视敏感内容、对比度严重不足影响可读）；
- **P2 明显可优化**：冗余按钮、状态缺失、不一致、反馈缺失；
- **P3 打磨**：间距、文案、细节美化。

**交付物**：
1. 主报告：`docs/working/<执行日期>-design-audit-report.md`（遵循 AGENTS.md 命名与头部失效日期规范），结构：
   - 执行摘要（Top 10 必修问题 + 一段总评）
   - 功能覆盖矩阵（14 条旅程 × 走通情况 × 最高严重度）
   - 功能发现明细（编号 F-01…，含类别 [A/B/C]、严重度、证据 `文件:行号` / curl 步骤 / 截图、影响描述、修复建议、涉及文件）
   - UI 发现明细（编号 U-01…，同上格式，逐页组织，附截图路径）
   - 横切检查结果（错误码映射表、i18n 缺口、角色矩阵、通知矩阵、空态清单）
   - 附录：构造测试数据的 psql 记录、验证过的 §4 断言逐条结论（证实/证伪）
2. 截图：`screenshots/audit/`（页面-主题-视口命名）
3. progress.txt 追加本任务条目（What was done / Testing / Notes）
4. 交接问题清单：`docs/working/<执行日期>-design-audit-findings.md`（或 .json），按 §10.1 的交接契约组织，作为子代理的唯一问题输入
5. 修复方案（由 §10 的新子代理产出）：`docs/working/<执行日期>-design-audit-fix-plan.md`
6. 报告**不修改任何业务代码**；如建议登记 GitHub issue，在报告尾部列出建议的 issue 标题清单，由人工决定是否立票。

**执行顺序建议**：Phase 1 基线阅读 + 环境启动 → Phase 2 功能走查（可按域拆给并行子 agent，注意共享发现去重）→ Phase 3 UI 逐页走查 → Phase 4 汇总成报告与交接清单 → Phase 5 按 §10 交给全新子代理完成复核与修复方案。每个 Phase 结束输出一段中间小结，最终以「主报告 + 复核结论 + 修复方案」三件套收口。

**自查门槛（报告提交前）**：14 条旅程每条至少有 3 个证据点；§4 的 11 条断言全部给出证实/证伪结论；每个 P0/P1 都有复现步骤；截图覆盖全部一级导航页面 × 亮暗双主题。**最终收口前**：主报告中每个问题都有子代理复核结论（证实 / 修正 / 驳回三选一），修复方案覆盖全部「证实 + 修正」项，被驳回项附反证。

---

## 10. Phase 5：交给全新子代理 —— 独立复核与修复方案生成

审查全部结束、主报告与交接清单成形后，你必须把全部发现移交给一个**全新的子代理**执行复核与修复方案。要求：该子代理**未参与任何审查阶段**、不共享你的会话记忆——独立性是它存在的意义，它必须以怀疑态度对待你的每条结论。你只负责组织交接、验收完整性，不替它下结论。

### 10.1 交接契约

- 交接物 = `docs/working/<执行日期>-design-audit-findings.md`（主报告可一并给出作为背景，但**问题清单才是唯一输入源**）。
- 清单中每个问题必须包含：编号（F-xx / U-xx）、类别 [A/B/C]、审查阶段定的严重度、问题描述、全部证据（`文件:行号` / curl 步骤 / 复现操作 / 截图路径）、审查者的初步定位猜测（明确标注为「未验证猜测」）。
- 派发子代理的提示词必须**自包含**：仓库根路径、环境启动命令（§3）、浓缩版基线阅读清单（§2 的 1–5 项 + 与该问题相关的状态机源文件）、本交接契约、§10.2–10.4 的完整协议。不得假设它知道本会话的任何上下文。

### 10.2 子代理复核协议（每个问题逐条走，不得抽样）

1. **独立重验**：不信审查者的结论与证据描述，自己重读代码 / 重跑复现，重新引用 `文件:行号`。审查者标注「不存在」的功能，必须按 §1.4 的穷尽要求重新搜索一遍。
2. **追溯真实根因**：到原因层而非症状层——不是「按钮对比度不足」，而是「哪个 class/硬编码色值/token 缺失导致」；不是「用户看不到封禁原因」，而是「哪个 API 没返回该字段 / 前端没渲染 / 状态机根本没记录」。跨层根因（前端呈现 ← API 契约 ← 状态机设计）必须逐层写明，因为它决定修复发生在哪一层。
3. **三选一裁决**：**证实**（问题与描述相符）/ **修正**（问题存在但描述、定位或严重度不准——给出正确版本）/ **驳回**（无法复现或不成立——必须附反证 `文件:行号` 或复现记录）。
4. **重新定级**：按验证后的真实影响面给最终严重度，可与审查阶段不同并说明理由。
5. **合并与依赖**：识别多个症状共享同一根因的项，合并为一个修复项；标注问题间的修复依赖（如「申诉死链」依赖「封禁原因透出」先落地）。

### 10.3 修复方案文档格式

子代理输出 `docs/working/<执行日期>-design-audit-fix-plan.md`（头部失效日期），结构：

1. **总览**：复核统计（证实 / 修正 / 驳回各多少，驳回项及反证索引）、最终优先级全景、建议批次划分（按依赖关系与风险聚合，如「快赢批次 / 审核链路批次 / 信息架构批次」）。
2. **每个修复项一节**，固定字段：
   - **问题背景**：这是什么问题、为什么值得修（类别 [A/B/C] + 最终严重度 + 关联的原始问题编号）；
   - **场景**：触发问题的用户场景（角色 + 操作序列 + 实际发生 vs 预期发生；P0/P1 必须含完整复现步骤）；
   - **根因**：验证过的代码级原因，`文件:行号` 逐层列出；
   - **修复方案**：具体技术方案——要改的文件、新增/修改的 API 与数据结构、是否需要数据库迁移、前端组件与 i18n 词条（zh/en 双语）变更；存在多个可选方案时列出取舍并给出推荐；
   - **优先级与顺序**：最终优先级 + 依赖哪些其他修复项先落地 + 建议执行顺序；
   - **建议车道**：按 AGENTS.md 标注 light / heavy——凡涉及安全、审核链路、封禁/申诉、数据库迁移、auth、生产配置的一律 heavy（TDD + 两阶段审查），并说明理由；
   - **验证标准**：完成的定义——需要新增/修改的测试清单、浏览器验证点、要跑过的命令（`go test ./...`、`npm run build`、`verify-project.sh` 等）；
   - **风险与回归面**：修复可能影响的现有行为、需要小心的既有契约（如 outbox 事件、幂等键、i18n key、ui-spec 条目需同步更新）。
3. **尾部**：建议的 issue 拆分清单（每个修复项一个建议 issue 标题 + 验收标准，供人工决定立票）、本轮明确不修的项及理由（如 Phase 2 范围、依赖真实凭证）。

### 10.4 子代理约束（与主审计一致）

- 只产出文档，不修改任何业务代码、不勾选 checkbox。
- 方案必须遵循项目既有架构与规范：AGENTS.md 工作流、design-system.md 唯一 token 权威、i18n 强制、软删除优先、错误信封、本地开发模式语义；确实需要突破既有约束的方案必须显式标注并论证。
- 涉及真实外部服务（阿里云 Green、OSS、SMTP、LLM）的修复项，方案中写明「本地语义」与「生产语义」的边界，不要求真实凭证即可验证的部分单独列出验证路径。

### 10.5 你的收口动作

子代理返回后：核对问题数量对账（清单条数 = 证实 + 修正 + 驳回）、确认修复方案覆盖全部证实与修正项、把复核结论与修复方案作为附件结论并入主报告（或互加链接），在 progress.txt 补记 Phase 5 条目，最后以三件套（主报告 / 复核+修复方案 / 截图目录）作最终交付说明。
