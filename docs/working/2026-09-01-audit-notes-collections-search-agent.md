# 设计审查笔记 D：收藏集/系列/浏览历史/搜索/标签 + Agent 工作台

- 创建日期：2026-09-01
- **预计失效日期**: 2026-11-01
- 范围：J9（收藏集、系列、浏览历史、搜索、标签）+ J10（Agent 工作台）；只审计不修复
- 方法：代码 `文件:行号` + 运行时 curl/psql 复现（测试账号 seed-ui-011 / seed-ui-012，见附录）
- 分类：[A] 规格不符 / [B] 规格缺失或过时 / [C] 设计可优化；严重度 P0~P3

---

## 一、发现明细（按严重度排序）

### D-01 [P0][A] Agent 用户消息气泡 Light 模式对比度 1.09:1，用户消息在浅色主题下不可读

- **证据**：
  - `frontend/components/agent/AgentWorkspace.tsx:572-577`：用户气泡 `"ml-auto bg-accent text-white"`，助手气泡 `"bg-canvas-subtle text-fg-default"`。
  - `frontend/app/globals.css:84`（:root）`--accent: #f5f5f5`；`globals.css:125`（.dark）`--accent: #161b22`。shadcn 语义中 `accent` 是「hover 淡灰底」，不是品牌主色（品牌主色是 `--primary`/`--accent-emphasis` = #4F46E5）。
  - 对照度计算（WCAG 相对亮度，见第三节 F11-① 表）：Light 下 `#FFFFFF` on `#F5F5F5` = **1.09:1**（正文需 ≥4.5:1）；Dark 下 `#FFFFFF` on `#161B22` = 17.30:1 通过。
- **影响**：浅色模式下用户自己发送的消息几乎白底白字，/agent 工作台核心交互不可用；这是 F11-① 的代码级根因。
- **修复方向**：用户气泡改 `bg-primary text-primary-foreground`（#4F46E5/#FFFFFF=6.29:1）或 `bg-accent-emphasis text-white`；同时为「气泡背景 token」补一条 ui-spec/design-system 约束，避免再把 `--accent`（淡灰）当品牌色用。
- **涉及文件**：`frontend/components/agent/AgentWorkspace.tsx`、（可选）`frontend/app/globals.css`、`design/design-system.md`。

### D-02 [P1][A] F9 双路由族并存：studio 侧三个管理页是空壳，真实实现滞留在旧 /dashboard/*，且旧页存在字段大小写漂移

- **证据**：
  - 根路由确为重定向：`app/(protected)/dashboard/page.tsx`（→ /studio/overview）、`app/(protected)/publish/page.tsx`（→ /studio/publish/original）、`app/(protected)/studio/page.tsx`（→ /studio/publish/original）。
  - 但 `dashboard/contents|contributors|pr-requests|tag-suggestions/page.tsx` 仍是**独立完整实现**，不是重定向；而 `studio/pr-requests|contributors|tag-suggestions/page.tsx` 是**空壳**（仅标题 +「XX 将在此处显示」空态，见 `messages/zh.json` studio.pr/contributors/tags.empty）。
  - 主导航把用户送进空壳：`frontend/components/studio/StudioSidebar.tsx:56-58`（prManagement→/studio/pr-requests、contributors→/studio/contributors、tagSuggestions→/studio/tag-suggestions）。
  - 旧 `/dashboard/contents` 页已损坏：`app/(protected)/dashboard/contents/page.tsx:16-22` 接口定义为 `ID/Title/ContentType/ViewCount...`（首字母大写），而后端 `internal/model/content.go:6-20` json tag 全小写（运行时实测响应仅含 `id/title/view_count`，无 `ID/Title`，见附录 #10）→ 表格标题列空白、查看链接变成 `/content/undefined`、删除请求打到 `/api/v1/contents/undefined`。
  - `app/robots.ts:10` 仍 disallow `/dashboard/`、`/publish/`，与「studio 为新家」的迁移并存。
- **影响**：PR 管理、贡献者管理、标签建议审核三个功能在主导航下全部呈现「空数据」假象（文案是数据空态而非功能缺失）；真正可用的旧页无任何站内入口（唯一入链见 D-04）。五问中「入口可达」「状态可知」双失败。
- **修复方向**：一次性收口——把旧 dashboard 三页实现迁移到 /studio/*（或先做成 redirect 到旧页），删除空壳与已损坏的旧 contents 页；同步 robots.ts。
- **涉及文件**：`frontend/app/(protected)/studio/{pr-requests,contributors,tag-suggestions}/page.tsx`、`frontend/app/(protected)/dashboard/*`、`frontend/app/robots.ts`。

### D-03 [P1][A] 标签建议审核触达闭环断裂：无通知 + 待办卡片死数据 + 唯一可用审核页无入口

- **证据**：
  - 提交建议不发通知：`backend/internal/service/tag_service.go:45-60`（SuggestTag 只写 tag_suggestions 表）；运行时验证：user 324 对 user 323 的内容提交建议后，`SELECT count(*) FROM notifications WHERE user_id=323 AND type LIKE '%tag%'` = 0，且 notifications 表现有 type 仅 system/like/comment（附录 #9）。
  - 待办卡片永远为空：`frontend/app/(protected)/studio/overview/page.tsx:32` 声明 `setPendingTasks` 后**从未调用**（全文件仅 L32/L200 两处引用），`components/studio/PendingTasksCard.tsx` 恒显示「暂无待办」。
  - 待办卡片的链接也指向空壳：`PendingTasksCard.tsx:27` → `/studio/tag-suggestions`（见 D-02）。
  - 真正能用的审核流在 `/dashboard/tag-suggestions`（`app/(protected)/dashboard/tag-suggestions/page.tsx`，调 GET/PATCH `/api/v1/dashboard/tag-suggestions`，routes.go:196-197），但无任何导航指向它。
  - 运行时链路本身健康：324 提交建议→201；323 列表→返回 pending；323 approve→200 且标签生效（附录 #5-#7）。
- **影响**：作者几乎不可能发现待审建议；建议者提交后石沉大海。配合 D-07（无 +1 分激励），标签共建机制整体名存实亡。
- **修复方向**：SuggestTag 成功后给作者发 notification；overview 接一个「待处理 PR + 待审标签建议」聚合接口填充 PendingTasksCard；审核页随 D-02 迁入 /studio。
- **涉及文件**：`backend/internal/service/tag_service.go`、`backend/internal/handler/tag.go`、`frontend/app/(protected)/studio/overview/page.tsx`、`frontend/components/studio/PendingTasksCard.tsx`。

### D-04 [P1][B] PR 提交入口断头：入口按钮指向无创建流程的页面，全站无任何创建 PR 的 UI

- **证据**：
  - `frontend/components/pr/SubmitPREntry.tsx:28`：fanwork 内容详情的「提交 PR」按钮链接 `/dashboard/pr-requests?content_id=${contentId}&create=1`。
  - 目标页 `app/(protected)/dashboard/pr-requests/page.tsx` **不读取任何 query 参数**（无 useSearchParams），也没有创建 PR 的表单；页面只有列表/accept/reject。
  - 全前端 grep 无任何对 `POST /api/v1/pr`（SubmitPR，`backend/internal/router/routes.go:116`）的调用；后端创建能力闲置。
- **影响**：同人区二次创作协同的核心闭环（提交 PR → 作者审核 → 合并）前半段在 Web UI 缺失；按钮带参数跳转后「什么都不发生」，属于典型的入口可达但出口不完备。
- **修复方向**：要么在 PR 管理页接收 `content_id&create=1` 打开创建表单（调 POST /api/v1/pr），要么摘掉 SubmitPREntry 入口并在 spec 中把 PR 创建标为暂缓。
- **涉及文件**：`frontend/components/pr/SubmitPREntry.tsx`、`frontend/app/(protected)/dashboard/pr-requests/page.tsx`、（规范）`docs/reference/api.md`。

### D-05 [P1][A] 收藏集管理页 /studio/favorites 近乎不可达：Header/StudioSidebar 无入口，两处「我的收藏」侧栏链接错指 /studio/contents

- **证据**：
  - `frontend/components/layout/Header.tsx` grep `favorites|收藏` 零结果——顶栏无入口。
  - `frontend/components/studio/StudioSidebar.tsx:38-62` 三个分组共 9 项，无 favorites。
  - `frontend/components/home/HomePageClient.tsx:154`：`label: t('home.myFavorites')`（zh「我的收藏」）`href: "/studio/contents"`；`frontend/components/original/OriginalSidebar.tsx:47`：`label: t('nav.favorites')`（zh「我的喜欢」）同样 `href: "/studio/contents"`。
  - 唯一在链入口：公开收藏集页 `app/(public)/collections/[id]/page.tsx:154` 的「管理」按钮 `router.push("/studio/favorites")`——前提是用户先访问自己的某个公开收藏集。
- **影响**：收藏 cutover 后功能本体（管理页 `app/(protected)/studio/favorites/page.tsx`，305 行，含分区/新建/公开开关/删除保护）质量良好但入口断裂；用户点「我的收藏」到达的是「我的内容」管理页，目标错位。
- **修复方向**：两处侧栏链接改指 `/studio/favorites`；StudioSidebar 增加收藏集入口；如「我的收藏」确需直达内容列表，则应换 label。
- **涉及文件**：`frontend/components/home/HomePageClient.tsx`、`frontend/components/original/OriginalSidebar.tsx`、`frontend/components/studio/StudioSidebar.tsx`。

### D-06 [P2][A] GET /search/trending 前后端契约错配：原创区热搜模块永远不渲染（静默失败）

- **证据**：
  - 后端 `backend/internal/handler/search.go:46-57`：返回 `{"trending": [{"text": ..., "score": ...}]}`；数据源 `internal/service/search_service.go:29-45` 读 Redis ZSet `rank:hot:contents`，member 是**内容 ID**（运行时实测 `{"text":"182","score":3}`，附录 #12）。
  - 前端 `frontend/components/original/OriginalSidebar.tsx:22-36`：请求 `/api/v1/search/trending` 却解构 `data.items`，且按 `{name, id, participant_count}` 取值 → `Array.isArray(data.items)` 恒 false → `trendingSearches` 恒空 → 热搜板块永不渲染，catch 静默。
  - 语义也错位：即使字段对上，「热搜」显示的是内容 ID 而非搜索词/话题名。
- **影响**：API-UI 双侧各说各话，模块整体死亡且无报错；home 首页的热搜（`HomePageClient.tsx:162` 用热门 IP 自建）不受影响，形成「首页有热搜、原创页没有」的不一致。
- **修复方向**：统一契约（推荐后端改回搜索词聚合 + `{items:[...]}` 包装，或前端改读 `trending[].text` 并由后端把 member 解析为标题）；补一条契约测试。
- **涉及文件**：`frontend/components/original/OriginalSidebar.tsx`、`backend/internal/handler/search.go`、`backend/internal/service/search_service.go`。

### D-07 [P2][A] 标签建议被认可的 +1 信誉分规则未实现（business-rules 承诺 vs 代码零实现）

- **证据**：
  - `docs/reference/business-rules.md:26`：「标签相关 | 成功举报 or 添加标签被认可 | +1」。
  - `backend/internal/service/tag_service.go:101-124`（ApproveTagSuggestion）：仅改状态 + AddTag/RemoveTag + IncrementUsage，无任何 reputation 调用；`internal/handler/tag.go` 同样没有（grep reputation 零结果）。
  - 运行时实测：建议被 approve 后 `SELECT count(*) FROM reputation_logs WHERE user_id=324` = 0（附录 #8）。
- **影响**：激励规则与文档不符，标签共建无正向反馈；结合 D-03 形成完整断链。
- **修复方向**：ApproveTagSuggestion（action=add 分支）写入 +1 reputation_log（复用既有信誉分服务通道），或修订 business-rules.md 明确该规则暂缓。
- **涉及文件**：`backend/internal/service/tag_service.go`、`docs/reference/business-rules.md`。

### D-08 [P2][C] F11-② 同屏重复 CTA：系列页与收藏页在「空列表」状态下头尾各出一个同文案创建按钮

- **证据**：
  - 系列：`app/(protected)/studio/series/page.tsx:241` 页头常驻 `<Button>…{t("studio.series.create")}</Button>`（toggle 表单）；L256 空态 `EmptyState action={<Button onClick={()=>setShowCreate(true)}>{t("studio.series.create")}</Button>}`。`series.length===0` 时两处**同屏**、同文案（zh.json `studio.series.create`="创建系列"）；且此时 L244 创建表单若展开，屏幕上还有第三个提交按钮（`studio.series.form.createSubmit`）。
  - 收藏页同型：`app/(protected)/studio/favorites/page.tsx:192` 分区头创建按钮 + L209 该分区空态创建按钮，同用 `studio.favorites.actions.create` 文案。
  - 全站扫描：`studio/contents/page.tsx` 头部 `publishNew`（L70 区域）与空态 `startCreating`（L84）同屏但**文案不同**、目标相同（/studio/publish/original）——同模式的弱化变体。
- **影响**：F11-② 所指「右上角 + 底部同时出现」的根因即此：页面头 CTA 常驻 + EmptyState 再放一个同目标 CTA，无条件互斥。视觉冗余且让「主操作在哪」产生歧义。
- **修复方向**：空态出现时隐藏页头 CTA（`series.length===0 && !showCreate` 时不再渲染页头按钮），或 EmptyState 不带 action 只做引导文案；favorites/contents 同步处理。
- **涉及文件**：`frontend/app/(protected)/studio/series/page.tsx`、`frontend/app/(protected)/studio/favorites/page.tsx`、`frontend/app/(protected)/studio/contents/page.tsx`。

### D-09 [P2][C] Agent 工作台对 429 限流无差异化 UX：配额耗尽与普通错误同文案，重试必然再失败

- **证据**：
  - `frontend/lib/agent-stream.ts:116-134`：非 2xx 响应只读取 `code` 用于 CSRF/401 重试，最终 `onError(new Error("agent stream failed: ${res.status}"))`——错误码（AGENT_RATE_LIMIT_EXCEEDED）被丢弃。
  - `frontend/components/agent/AgentWorkspace.tsx:417-420`：`onError: () => { setTurnError(true); setStreaming(false); }` → 统一显示 `agent.workspace.errorTitle`（zh「本次请求未完成」）+「重新发送」（L634-645）。
  - `lib/user-facing-error.ts` 无 AGENT_RATE_LIMIT 相关映射；config.yaml agent.rate_limit_per_day=50 / per_minute=5 的配额余量对用户完全不可见。
  - 对照组：搜索页 `components/agent/SearchAgentInput.tsx:62-65` 对 429/5xx/FEATURE_DISABLED 有明确降级文案 + 自动回退关键词搜索——工作台未达到同等水准。
  - 运行时 gate 关闭实测：chat/stream 返回 503 FEATURE_DISABLED（附录 #14），该路径同样落入通用错误分支（但页面级 AgentFeatureGate 已先行挡住，见 F11-① 节末尾）。
- **影响**：用户在限流窗口内点「重新发送」只会反复失败，无「今日额度已用完/稍后再试」的告知；触达闭环缺反馈。
- **修复方向**：startAgentStream 把响应 body 的 `code` 透传给 onError；工作台对 AGENT_RATE_LIMIT_EXCEEDED 显示专属文案（含窗口信息）并隐藏重试按钮；可选展示当日剩余配额。
- **涉及文件**：`frontend/lib/agent-stream.ts`、`frontend/components/agent/AgentWorkspace.tsx`、`frontend/lib/user-facing-error.ts`、`frontend/messages/{zh,en}.json`。

### D-10 [P2][C] SSE `start` 与 `usage` 事件有契约无消费：trace_id 不可见、token 用量不展示

- **证据**：
  - 后端契约：`backend/internal/service/agent_stream.go:22-29` 定义七类事件；L228 发 start（trace_id/conversation_id）、L427 发 usage。
  - 前端类型完整（`frontend/lib/agent-stream.ts:27-48` 含 start/usage），但 `AgentWorkspace.tsx:310-380` 的 `handleStreamEvent` switch 只处理 delta/tool_status/citation/done/error，`start`/`usage` 落入 `default: break`。
- **影响**：观测性设计（trace_id 贯穿）在用户侧不可见，排障需查后端日志；usage（token 消耗）完全不展示。属「结果反馈」维度的缺失而非错误。
- **修复方向**：done/折叠区展示本轮 token 用量（usage 事件或 done 聚合）；错误框附 trace_id 便于报障。
- **涉及文件**：`frontend/components/agent/AgentWorkspace.tsx`。

### D-11 [P2][C] F10 重复组件：content/ 目录下 ComplianceCheckBadge 与 UploadAssistPanel 是无人引用的同名异构死代码

- **证据**：
  - `frontend/components/agent/ComplianceCheckBadge.tsx`（135 行，导出 `AgentComplianceCheckBadge`：真实调 POST /agent/compliance-check，三档风险徽章 + 展开详情）vs `frontend/components/content/ComplianceCheckBadge.tsx`（32 行，导出 `ComplianceCheckBadge`：纯 props 就绪指示器 hasTitle/hasCover/hasAttachment）——**同名文件、不同组件、不同导出名**。
  - `UploadAssistPanel` 同型：agent 版 135 行（分析 + 一键填充 + onFill 回调）vs content 版 38 行（按 content_type 的静态上传提示文案）。
  - 引用关系：全仓仅 `frontend/components/studio/PublishForm.tsx:17-18` import agent 版；content 版两文件 **grep import 零结果 = 死代码**（附录 #3 的 diff 与引用扫描）。
- **影响**：当前无行为/视觉漂移（死代码不渲染），但同名文件 + 相似职责是高概率陷阱——后续维护者极易改错文件或重复造轮子。
- **修复方向**：删除 `components/content/ComplianceCheckBadge.tsx` 与 `components/content/UploadAssistPanel.tsx`；或将 content 版改名为静态提示组件（如 UploadTypeHints）消除同名。
- **涉及文件**：`frontend/components/content/ComplianceCheckBadge.tsx`、`frontend/components/content/UploadAssistPanel.tsx`。

### D-12 [P3][C] 引用卡标题 Dark 模式对比度 4.24:1，小字号未达 AA（4.5:1）

- **证据**：`frontend/components/agent/AgentCitationCard.tsx:26`：`text-sm font-medium text-accent-emphasis` on `bg-canvas-default`；dark 值 `--accent-emphasis: #6366F1`（globals.css:197）on `#0D1117` = 4.24:1。`text-sm font-medium`（14px/500）不属 WCAG 大字（≥18.66px bold 或 ≥24px），需 4.5:1。Light 下 6.29:1 通过。
- **修复方向**：dark 的 `--accent-emphasis` 提亮至 ≥#737FF2 量级（如 #818CF8 与 `--accent-hover` 一致，对比约 6.3:1），或引用卡标题升为 font-semibold+text-base 走大字标准。
- **涉及文件**：`frontend/app/globals.css`、`design/design-system.md`（token 表同步）。

### D-13 [P3][C] GET /search/suggestions 能力闲置：全站无消费方，且前缀匹配对中文场景偏弱

- **证据**：
  - `backend/internal/router/routes.go:216` 提供 `/search/suggestions`；`internal/repository/search_repo.go:89-130` 实现 tags+已发布标题的 `LIKE prefix%`（ILIKE）。
  - 前端 grep `search/suggestions` 零结果：`components/search/GlobalSearchInput.tsx`（Header 三处共用）是纯 keyword 提交跳 `/search?q=`，无建议下拉。
  - 运行时：`q=镜头` 无法匹配《原创实验｜镜头下的普通朋友 11》（前缀从「原创实验」开始），返回空（附录 #11）。
- **修复方向**：GlobalSearchInput 接入 suggestions 做搜索联想下拉（含防抖），或后端改 contains 匹配；若产品决定不做联想，应在 api.md 标注「预留未接线」。
- **涉及文件**：`frontend/components/search/GlobalSearchInput.tsx`、`backend/internal/repository/search_repo.go`。

### D-14 [P3][B] GET /users/search 无通用入口：仅协作邀请选人组件使用

- **证据**：`backend/internal/router/routes.go:224` 提供用户搜索；前端唯一消费是 `frontend/components/content/CollabUserPicker.tsx:71`（发布表单协作者选择器）。`app/(public)/search/page.tsx` 无用户维度/tab；Header 搜索固定跳内容搜索。
- **影响**：「找用户/找创作者」这一常见诉求在全站搜索中缺席（只能通过关注链/评论昵称跳转）。
- **修复方向**：搜索页增加 Users tab（复用现有 API），或明确记录为 Phase 2。
- **涉及文件**：`frontend/app/(public)/search/page.tsx`。

### D-15 [P3][C] SearchAgentInput 模式切换按钮用 ArrowLeft 图标表达「切回关键词搜索」，语义误导

- **证据**：`frontend/components/agent/SearchAgentInput.tsx:5,123-134`：keyword 模式按钮渲染 `<ArrowLeft className="h-3 w-3" />` + `t("agent.keywordSearch")`。左箭头通常意为「返回」，此处实为模式切换。
- **修复方向**：换成 Search/Type 等中性图标，或做成同一组 segmented control。
- **涉及文件**：`frontend/components/agent/SearchAgentInput.tsx`。

### D-16 [P3][B] Agent 回答纯文本渲染：无 markdown/代码块渲染，「代码块」表面不存在

- **证据**：`frontend/components/agent/AgentWorkspace.tsx:570-585` 消息体是 `whitespace-pre-wrap` 裸文本；无 MarkdownRenderer/代码高亮引用（对比 `components/content/MarkdownRenderer.tsx` 在内容侧存在）。LLM（deepseek-chat）回答常含 `**`、列表、代码围栏，会原样展示。
- **影响**：呈现质量受限；也因此「代码块对比度」检查项无对应表面（不适用），但引用卡/工具行/气泡的对比度已全部覆盖（见第三节）。
- **修复方向**：助手消息接入受控 MarkdownRenderer（禁原始 HTML），代码块走等宽 + 独立底色并按 token 表校验对比度。
- **涉及文件**：`frontend/components/agent/AgentWorkspace.tsx`。

---

## 二、预探查断言结论

### F9「/dashboard/*（旧）与 /studio/*（新）双路由族并存」——证实（带细化）

- `/dashboard`、`/publish`、`/studio` 三个根路径确为纯重定向（分别 → /studio/overview、/studio/publish/original、/studio/publish/original）。
- 但 dashboard 四个子页仍是独立实现：contents（**已损坏**：读大写字段 `ID/Title`，API 实际返回小写 `id/title`，标题列空白、链接 `/content/undefined`，见附录 #10）、contributors（功能完整，调 /dashboard/contributors/:id/block，路由仍在 routes.go:125-126）、pr-requests（功能完整）、tag-suggestions（功能完整）。
- 与 studio 版**不是重复实现而是「实现 vs 空壳」的漂移**：studio 的 pr-requests/contributors/tag-suggestions 三页只有标题 + 空态文案；studio/contents 则是全新实现（走 `/users/me/contents` 分页版）。主导航全部指向 studio 侧 → 三个功能在导航层面不可用（D-02）。
- 额外发现：`components/pr/SubmitPREntry.tsx:28` 仍硬链接旧族 `/dashboard/pr-requests` 并携带被目标页忽略的 `content_id/create` 参数（D-04）；robots.ts 仍屏蔽 /dashboard/、/publish/。

### F10「ComplianceCheckBadge 与 UploadAssistPanel 在 components/agent/ 与 components/content/ 各有一份」——证实（性质修正）

- 两对文件**不是彼此的复制品，而是同名异构**：agent 版是真实 API 面板（`AgentComplianceCheckBadge`/`AgentUploadAssistPanel`），content 版是静态提示/就绪指示（`ComplianceCheckBadge`/`UploadAssistPanel`），diff 全量不一致（附录 #3）。
- 引用扫描：仅 `PublishForm.tsx:17-18` 引用 agent 版；content 版两文件**零引用，是死代码**。因此当前**无行为或视觉漂移**，风险是混淆与误改（D-11）。

### F11-①「Agent 工作台对比度问题」——证实，代码级定位 + 数值计算

token 来源：`design/design-system.md`「自定义 Token」表 + `frontend/app/globals.css`（:root L69-113、.dark L115-150、自定义 token L151-199）。对比度按 WCAG 2.x 相对亮度公式计算（脚本见附录 #15）。

| # | 表面（代码位置） | 前景/背景 class | Light 色对 | Light 比 | Dark 色对 | Dark 比 | 判定（正文≥4.5 / 大字≥3） |
|---|---|---|---|---|---|---|---|
| 1 | 用户气泡（AgentWorkspace.tsx:575） | `text-white` on `bg-accent` | #FFFFFF/#F5F5F5 | **1.09** | #FFFFFF/#161B22 | 17.30 | **Light FAIL（严重）** / Dark PASS |
| 2 | 助手气泡（:576） | `text-fg-default` on `bg-canvas-subtle` | #18181B/#F5F5F5 | 16.25 | #E6EDF3/#161B22 | 14.64 | PASS/PASS |
| 3 | 工具状态行（AgentToolStatus.tsx:69-71） | `text-fg-muted` on `bg-canvas-default` | #52525B/#FFFFFF | 7.73 | #848D97/#0D1117 | 5.62 | PASS/PASS |
| 4 | 工具状态行标签（:71） | `text-fg-default` on `bg-canvas-default` | #18181B/#FFFFFF | 17.72 | #E6EDF3/#0D1117 | 16.02 | PASS/PASS |
| 5 | 引用卡标题（AgentCitationCard.tsx:26） | `text-accent-emphasis` on `bg-canvas-default` | #4F46E5/#FFFFFF | 6.29 | #6366F1/#0D1117 | **4.24** | PASS / **Dark FAIL（小字差 0.26；作大字 PASS）** |
| 6 | 引用卡序号/分区/摘录（:27,29,36） | `text-fg-muted` on `bg-canvas-default` | #52525B/#FFFFFF | 7.73 | #848D97/#0D1117 | 5.62 | PASS/PASS |
| 7 | 输入框 placeholder（AgentWorkspace.tsx:686） | `placeholder:text-fg-muted` on `bg-canvas-default` | #52525B/#FFFFFF | 7.73 | #848D97/#0D1117 | 5.62 | PASS/PASS |
| 8 | composer 提示行（:710） | `text-fg-muted` on `bg-canvas-default` | 同上 | 7.73 | 同上 | 5.62 | PASS/PASS |
| 9 | 空态建议按钮（:559） | `text-fg-muted` on `bg-canvas-default` | 同上 | 7.73 | 同上 | 5.62 | PASS/PASS |
| 10 | 降级/无证据/错误框正文（:590,608,635） | `text-fg-default` on `bg-canvas-default` | #18181B/#FFFFFF | 17.72 | #E6EDF3/#0D1117 | 16.02 | PASS/PASS |
| 11 | 降级/无证据框描述（:594,612） | `text-fg-muted` on `bg-canvas-default` | #52525B/#FFFFFF | 7.73 | #848D97/#0D1117 | 5.62 | PASS/PASS |
| 12 | 错误框图标（:636） | `text-destructive` on `bg-canvas-default` | #E11D48/#FFFFFF | 4.70 | #F87171/#0D1117 | 6.84 | PASS/PASS（图标按 3:1） |

- 代码块：工作台无 markdown/代码块渲染（D-16），该表面不存在，不适用。
- 占位提示（加载骨架 `bg-canvas-subtle`、空态图标 `bg-accent-subtle text-accent-emphasis` L546-547）：装饰性，非文本，未列。
- **不达标清单**：① 用户气泡 Light（1.09:1，P0，D-01）；② 引用卡标题 Dark（4.24:1，P3，D-12）。其余 10 组表面双主题达标。

### F11-②「series 页创建按钮右上+底部同屏双现」——证实，根因定位

- 根因：`app/(protected)/studio/series/page.tsx:239-242` 页头按钮（`studio.series.create`，toggle 语义）无条件渲染；L255-256 空态分支再渲染一个同 key、同文案的 `EmptyState action` 按钮（仅 `setShowCreate(true)`）。`series.length===0` 时两者同屏。展开表单后还有 L251 的第三个提交按钮（不同文案 createSubmit）。
- 同型扩散：`studio/favorites/page.tsx:192 + L209`（同文案 `actions.create`，分区头 + 分区空态）；`studio/contents/page.tsx`（头 `publishNew` + 空态 `startCreating`，文案不同、目标相同）。
- 头部按钮 toggle（可收起）与空态按钮只开不收，行为也不一致。

---

## 三、经核验的正常面（无发现，供主审对照）

- **收藏集后端语义完整**：GET /collections 自动确保 original+fanwork 双默认集（collection_service.go:26-33，运行时 user 323 返回 id 823/824 is_default=true）；重复添加 409 DUPLICATE_COLLECTION_ITEM（collection.go:299-300，实测）；默认集删除保护 DEFAULT_COLLECTION_PROTECTED；私有集他人/匿名访问 404 COLLECTION_NOT_FOUND 不泄漏存在性（collection_repo.go GetCollectionForViewer，实测 404）；owner_id 视角只返回 public 集（实测 user 324 与匿名均只见公开集 875）；zone 不可变（ZONE_IMMUTABLE）。
- **is_favorited 唯一事实源**：`internal/handler/content.go:357-363` 以 CountActiveMembershipsForContent（收藏集成员关系）判定；前端 ContentDetail 收藏按钮 + CollectionPicker onMembershipChange 共用该状态（ContentDetail.tsx:451-466）；旧 favorites 前端路径零残留（grep lib/components 无 `/favorites` API 调用；routes.go 无 favorites 路由）。
- **浏览历史**：按天分组（今天/昨天/日期，history/page.tsx:371-386）；retention_days 由后端下发（browse_history.go GetHistory 返回，实测=7）并在 UI 展示（L212/227）；多选删除（ids≤100）与 clear_all 互斥校验（ClearHistory DELETE_MODE_CONFLICT/CLEAR_CONFIRMATION_REQUIRED）；记录时机=浏览即记（ContentDetail mount 即 POST，user 323 的 useEffect L230-233；因 ContentDetail 同时服务独立页与共享浮层，**浮层打开也会记录**——设计事实，非缺陷）；IP 访问历史 merge 闭环完整（AuthContext.tsx:194-199 登录即 mergeLocalIpsIntoAccount，home 显示 recent IPs，IPCard 记录）。
- **系列**：管理页创建/候选搜索（250ms 防抖 + zone 过滤 + 去重 + 截断 8 条）/增删/上下移排序（PUT reorder 全量 item_ids）/封面选择/删除确认齐全；公开页 SeriesNav（上一件/下一件/目录 listbox/多系列 tab/溢出菜单）a11y 与焦点管理质量高（SeriesNav.tsx 全文）；候选 API 实测返回带封面/状态的已发布内容。
- **Agent gate 与发布助手**：config.yaml `agent.web_agent_enabled: false`（config.yaml:238）→ 页面 EmptyState（agent/page.tsx:16-29，gate 还要求 email_verified）；API 全线 503 FEATURE_DISABLED（chat/conversations/upload-assist/compliance 实测）；PublishForm 的 AgentUploadAssistPanel（自动填充 + undo 快照 + violation 阻断填充）与 AgentComplianceCheckBadge（risk_level 三档 + violation 阻断提交）均包在 AgentFeatureGate 内，位置在提交区之前，语义清晰。
- **SSE 主链路**：delta 增量拼接、tool_status 汇聚、citation 聚合、done（conversation_id 回填 + no_evidence/degraded 摘除半截回答 + 关键词降级 fallback /contents/search）、error（provider_error → 降级 + abort）分支齐全；CSRF/401 双重试（agent-stream.ts:118-128）；停止生成/手动重试/回到底部/移动端会话抽屉齐备；会话删除 ConfirmModal + owner-scoped DELETE；引用卡点击打开共享 ContentDetailOverlay（source=agent-citation）并归还焦点。
- **搜索页**：移动端筛选抽屉 + 桌面 sidebar 双渲染（search/page.tsx:206/213）、grid/list 视图切换、保存搜索（FacetedSearchSidebar 内嵌 + 页面快捷保存）。

## 四、API-UI 对照缺口汇总

| API | 后端状态 | 前端消费 | 缺口 |
|---|---|---|---|
| GET /search/suggestions | 正常（前缀匹配） | **无** | 死能力（D-13） |
| GET /search/trending | 正常（返回内容 ID） | OriginalSidebar 读错契约 | 模块永不渲染（D-06） |
| GET /users/search | 正常 | 仅 CollabUserPicker | 无通用用户搜索入口（D-14） |
| POST /api/v1/pr | 正常 | **无** | PR 创建闭环缺失（D-04） |
| GET/PATCH /dashboard/tag-suggestions | 正常 | 仅旧 dashboard 页 | 页面孤儿 + 无通知 + 待办死数据（D-02/D-03） |
| POST/DELETE /dashboard/contributors/:id/block | 正常 | 仅旧 dashboard 页 | 页面孤儿（D-02） |
| SSE start / usage | 正常发送 | 类型有、handler 无 | trace_id/用量不可见（D-10） |
| GET /collections 系列 | 正常 | CollectionPicker/favorites/公开页 | 无（正面） |
| GET/DELETE /users/me/history | 正常 | /history 页 | 无（正面） |
| GET /users/me/ip-visits + merge | 正常 | AuthContext/HomePageClient/IPCard | 无（正面） |

## 五、附录：运行时构造与验证记录（全部仅动 seed-ui-011/012 及其对象）

1. 登录：CSRF `GET /api/v1/auth/csrf` → `POST /api/v1/auth/login`（seed-ui-011 → TOK，seed-ui-012 → TOK2），cookie 分别存 /tmp/D-cookies{,2}.txt。
2. `GET /api/v1/collections`（TOK）：返回 items 含 id 823「默认原创收藏」/824「默认二创收藏」（is_default=true）+ 自建集 825（私有）→ 默认集自愈实证。
3. `diff components/agent/{ComplianceCheckBadge,UploadAssistPanel}.tsx components/content/{...}`：全文不一致；`grep -rn "from \"@/components/content/(ComplianceCheckBadge|UploadAssistPanel)\""` 零结果 → F10 定性。
4. 私有集可见性：`GET /api/v1/collections/825` with TOK2 → 404 COLLECTION_NOT_FOUND；匿名 → 404。
5. 标签建议提交：user 324 `POST /api/v1/contents/12/tags/suggest {"tag":"审计标签D","action":"add"}` → 201。
6. 作者列表：user 323 `GET /api/v1/dashboard/tag-suggestions?content_id=12` → 返回 pending 建议（id=1）。
7. 审核：user 323 `PATCH /api/v1/dashboard/tag-suggestions/1 {"status":"approved"}` → 200，标签写入 content 12（后已清理）。
8. 激励验证：`SELECT count(*) FROM reputation_logs WHERE user_id=324` = 0 → +1 未实现。
9. 通知验证：`SELECT count(*) FROM notifications WHERE user_id=323 AND type LIKE '%tag%'` = 0；`SELECT DISTINCT type FROM notifications` = system/like/comment。
10. 字段大小写：`GET /api/v1/contents?author_id=323` 响应首项仅含 `id/title/view_count`（小写），无 `ID/Title/ViewCount` → 旧 dashboard/contents 页解析断裂实证。
11. `GET /api/v1/search/suggestions?q=镜头` → `{"suggestions":[]}`（前缀不匹配《原创实验｜镜头下…》）。
12. `GET /api/v1/search/trending?limit=5` → `{"trending":[{"text":"182","score":3},{"text":"179","score":3}]}`。
13. `GET /api/v1/users/me/history`（TOK）→ `{"items":[],...,"retention_days":7}`；`GET /api/v1/users/me/ip-visits` → `{"items":[],"limit":6}`。
14. gate：`GET /api/v1/config/public` → features.web_agent_enabled=false；`POST /api/v1/agent/chat/stream`、`GET /agent/conversations`、`POST /agent/upload-assist`、`POST /agent/compliance-check` 均 503 FEATURE_DISABLED。
15. 对比度计算脚本：Python，WCAG 相对亮度（sRGB 分段线性化），输入为上文 token 表色值；结果见 F11-① 表。
16. 清理：`DELETE /api/v1/collections/875`（审计公开集，200）；`DELETE FROM content_tags WHERE content_item_id=12 AND tag='审计标签D'`（影响 1 行）；`tags` 表中「审计标签D」残留 0 行。tag_suggestions id=1（approved）保留于自己内容上，作为流程证据，无运行时影响。
