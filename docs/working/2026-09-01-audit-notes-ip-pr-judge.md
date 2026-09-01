# 设计审查笔记：IP 创建管理 / PR 协同全链路 / 赛博判官（子任务 B）

- 创建日期：2026-09-01
- 预计失效日期：2026-11-01
- 性质：只审计不修复。所有结论均有代码 `文件:行号` + 运行时 curl/psql 复现（附录 A）。
- 测试账号：seed-ui-007（user 319，作者/判官）、seed-ui-008（user 320，贡献者/第二判官）；admin=313 仅调用 admin API 未改其状态。
- 环境：后端 :8080（queue.enabled=true 但 **cmd/worker 未运行**，见 B-20）、前端 :3000。

---

## 一、发现明细（按严重度排序）

### B-01 [A] P0 —— 判官考试前端字段名不匹配，UI 路径永远 0 分，判官资格无法从界面获得
- 证据：
  - 后端答案字段为 `answer`：`backend/internal/service/judge_service.go:46`（`Answer string \`json:"answer"\``）。
  - 前端提交 `answer_key`：`frontend/app/(protected)/judge/exam/page.tsx:93-96`（`{ question_id: parseInt(qid), answer_key: key }`）。
  - 运行时（附录 A-6）：先造 10 道 article 题库 → 用前端同构 payload（全对、字段名 answer_key）提交 → `score 0, passed:false`；改用 `answer` 字段 → `score 10, passed:true` 并生成 judge_qualifications。
- 影响：判官体系从 UI 完全无法进入（考试必挂 0 分）。简历/演示里「赛博判官」只能靠 curl 演示。
- 修复方向：前端改发 `answer`（或后端兼收两个字段并兼容旧 payload）。
- 涉及文件：judge_service.go、exam/page.tsx。

### B-02 [A] P1 —— 判官闭案结果不生效：内容状态不变、作者不知情，被审内容进入永久 limbo
- 证据：
  - `judge_service.go:148-164`：达到 MinVotes 后仅 `CloseCase`（judge_repo.go:98-105 只 update judge_cases），无任何 content_items 转移、无 Notify 调用。
  - 全仓库无 closed_approve/closed_reject 消费方（除出题调度器 `scheduler/judge_question_sync.go:57`）；`grep under_review` 仅 review_service 进入路径，无退出路径（admin 只能 ban，`routes.go:328-331`）。
  - 规格：`docs/reference/business-rules.md:160`「判决结果：「不违规」比例 ≥ 60% → **恢复展示**；< 60% → 有争议，不予展示（管理员可手动恢复）」。
  - 运行时（附录 A-8）：构造 19 票全 approve（min_votes 调至 19）→ 案件 `closed_approve`，**content 177 仍 under_review**，作者 320 通知数 0。
- 影响：判官投完票没有任何实际效果；作者内容被 Green 打入 review 后唯一出路是 admin 手动 ban。整个众裁闭环「出口不完备」。
- 修复方向：闭案事务里按阈值恢复 published / 保持隐藏并通知作者（含理由列表链接）。
- 涉及文件：judge_service.go、review_service.go、business-rules.md。

### B-03 [A] P1 —— 用户举报自动隐藏不触发众裁（规格明确要求），且该状态无出口
- 证据：
  - `backend/internal/service/social_service.go:305`：举报比例达标 → `UpdateContent(status=under_review)`，**未调用 ensureJudgeCase**，无 AI 复审、无通知。
  - 规格：`docs/reference/business-rules.md:164`「内容自动隐藏：… → status 改为 under_review，**触发众裁**」。
- 影响：被举报隐藏的内容永远停在 under_review（无判案、无恢复、无告知），与 Green review 路径（会建 judge case）行为不一致。
- 修复方向：Report 达阈值时同事务 ensureJudgeCase，或明确降级语义并更新规格。
- 涉及文件：social_service.go、business-rules.md。

### B-04 [B] P1 —— 判官看不到争议内容本体：按「内容 #177」盲投；内容安全暴露检测全部落空
- 证据：
  - `frontend/components/judge/ReviewCard.tsx:71-75`：正文区只有占位文案 `t('judge.reviewCard.introContent')`（zh.json=「该内容因被举报进入众裁审核，请根据社区规范判断是否违规。」），未 fetch 内容详情、无媒体渲染、无案件详情链接。
  - `GET /judge/queue` 只返回 target_type/target_id（`handler/judge.go:86-100`），队列页（queue/page.tsx）也不拉取内容。
  - ui-spec `design/ui-spec.md` §/judge/queue（L1425 附近）「default: **案例详情** + 违规/不违规投票按钮」未实现。
  - 文案与事实不符：案件来源是 Green「review」判定（review_service.go:343-353），不是「被举报」（举报根本不建案，见 B-03）。
- 内容安全暴露检测逐项结论（旅程 J7 第 3 组）：
  1. 争议封面/媒体**并非直出**——因为根本不渲染任何内容；无模糊/打码/预警/点击加载机制（也无从谈起）。
  2. 无「跳过此案件」选项（后端无 skip、前端无按钮；`ListOpenCases` 无 voted 过滤，已投案件仍出现在自己的队列里，只能靠本地 currentIndex 翻页）。
  3. 判官**无法举报案件内容**（无升级通道；Report API 存在但判官 UI 没有入口）。
  4. 评论类争议案件不存在（ensureJudgeCase 仅建 content 案；评论 under_review 无判案）。
  5. 媒体 URL 鉴权：`GET /contents/:id` 为 optAuth 且**不按状态过滤**——under_review 内容（含封面/附件 URL）对匿名用户完全可读（附录 A-8 验证 content 177 anon 200）。若按建议补上内容渲染，必须同时做「登录+点击确认后加载」而不是直接 img 直出。
- 影响：判官决策无依据；同时公共 API 层存在 under_review 内容裸奔。
- 修复方向：ReviewCard 内嵌受控内容预览（登录态 + 内容预警横幅 + 点击加载媒体 + 案件类型标签 + 跳过 + 举报升级按钮）；GetContent 对非 published 内容收紧到作者/admin/持资格判官。
- 涉及文件：ReviewCard.tsx、judge.go(handler)、content.go(handler)、review_service.go。

### B-05 [B] P1 —— PR 提案正文（new_text）不落库，作者无法看到贡献者改了什么
- 证据：
  - `backend/internal/service/pr_service.go:35-40,64-70`：SubmitPRInput 有 `NewText`，但构造 PullRequest 时丢弃；model 有 `ProposedVersionID`（version.go:32）从未赋值。
  - 运行时（附录 A-4）：320 对 content 176 提交带 new_text 的 PR → `proposed_version_id` 为 NULL，content_versions 未新增行。
  - 前端作者页 `dashboard/pr-requests/page.tsx:77-88`：右栏文本取 `/versions/{proposed_version_id}` → 恒为空字符串，DiffViewer（两栏）右栏永远全红、MergeEditor「PR 提案版本」栏恒空。
- 影响：PR 审阅的核心信息（改了什么）丢失；作者「接受」的依据只有 message 一行字。
- 修复方向：SubmitPR 时为提案建 ContentVersion（status 可用 proposed/非 active，不影响 RAG）并写入 proposed_version_id。
- 涉及文件：pr_service.go、version.go、dashboard/pr-requests/page.tsx。

### B-06 [B] P1 —— 贡献者无 PR 提交入口：SubmitPREntry 是孤儿组件，链接指向的页面也不支持创建
- 证据（穷尽搜索 frontend/app、components、lib）：
  - 全前端唯一 POST /api/v1/pr 的调用不存在（grep 仅 accept/reject 两处，dashboard/pr-requests/page.tsx:101,119）。
  - `components/pr/SubmitPREntry.tsx` 定义了「提交修改申请」入口（跳 `/dashboard/pr-requests?content_id=X&create=1`），但该组件**没有任何引用方**（ContentDetail 等均未 import）。
  - `dashboard/pr-requests/page.tsx` 不读取 searchParams/create 参数；其数据源是 `/contents?author_id=自己` + 各内容的 open PR（L50-58）——贡献者进来只会看到「自己作品收到的 PR」（通常为空），无法提交。
- 影响：PR 协同旅程第一步（贡献者发起）在 UI 上不存在，整条链路只能 curl 演示。
- 修复方向：内容详情页挂 SubmitPREntry + 建一个 PR 提交表单（选 base version、填 message/new_text）。
- 涉及文件：SubmitPREntry.tsx、ContentDetail.tsx、dashboard/pr-requests/page.tsx。

### B-07 [A] P1 —— accept 与 merge 状态机互斥：accept 后无法 merge，accept 本身无任何内容效果；merge 亦无 UI 入口
- 证据：
  - `pr_service.go:110`：AcceptPR 仅置 accepted + 记 contributor + 发通知，不落版本不改正文。
  - `pr_service.go:175-177`：ManualMerge 要求 `pr.Status == "open"`，**accepted 之后 merge 被拒**。
  - 运行时（附录 A-5）：accept 成功后立即 merge → 400 ERROR。
  - 前端无任何 `/pr/:id/merge` 调用（grep 全库无）；MergeEditor 只有「复制合并结果」按钮（MergeEditor.tsx:50-60）。
- 影响：PR 的三条终态路里，merge（唯一会改内容的路径）无 UI 入口；accept 是死胡同（对内容零效果且堵死 merge）。
- 修复方向：明确语义——「接受=采纳提案文本」应建版本，或允许 accepted→merge；前端补 merge 提交按钮。
- 涉及文件：pr_service.go、dashboard/pr-requests/page.tsx、MergeEditor.tsx。

### B-08 [A] P1 —— /studio/pr-requests 是空壳页；真实实现孤悬旧 /dashboard，与「studio 为准 + 旧路由 301」的既定迁移相悖
- 证据：
  - `app/(protected)/studio/pr-requests/page.tsx`（全文 20 行）：静态 EmptyState 占位，无任何 API 调用。
  - `business-rules.md:173` 附近：「创作者工作室…整合 /dashboard/*，旧路由保留 301 重定向，新版以 /studio/* 为准」；实际无 301（next.config/页面均无 redirect）。
  - studio 侧边栏（components/studio/StudioSidebar.tsx:56）、工作室待办卡（PendingTasksCard.tsx:27）、**PR 结果通知跳转**（NotificationDropdown.tsx:85 `case "pr": return "/studio/pr-requests"`）全部指向空壳页。
- 影响：作者从工作台/通知进入 PR 管理看到的是「暂无 PR」假象；旧页又不在 studio 信息架构内。
- 修复方向：把 dashboard 实现迁入 /studio/pr-requests（或先做 301）。
- 涉及文件：studio/pr-requests/page.tsx、StudioSidebar.tsx、NotificationDropdown.tsx。

### B-09 [A] P1 —— 「closed」状态词表前后端不一致：错误率撤权成死代码、结案横幅永不显示（F7 部分证伪）
- 证据：
  - 后端写 `closed_approve` / `closed_reject`（judge_service.go:157-159）。
  - `judge_service.go:185`：`judgeCase.Status != "closed"` → continue——**所有票都被跳过**，wrong 恒 0，撤权+扣分（L201-208）永不触发；business-rules.md:158 的撤权规则落空。
  - `frontend/components/judge/VerdictDetail.tsx:129`：`isClosed = caseData.status === "closed"` → 结案结果横幅（L169-177）永不显示。
  - 运行时（附录 A-8）：闭案后 status=closed_approve，前端判定为未结案。
- 影响：误判撤权机制完全失效（F7 该项证伪）；用户看不到「判决结果」横幅。
- 修复方向：统一为前缀匹配（closed_approve/closed_reject）或引入终态枚举共享。
- 涉及文件：judge_service.go、VerdictDetail.tsx。

### B-10 [B] P1 —— IP 创建无前端入口；创建者对自己 IP 的 pending/rejected 状态不可知、无通知
- 证据（穷尽 frontend grep：创建 IP 仅 admin/ips 页有 api.post）：
  - POST /api/v1/ips 存在（routes.go:84）但无 UI 调用；IPPicker（components/studio/IPPicker.tsx:43）仅搜索已 approved IP，无「新建 IP」分支。
  - 无「我的 IP」页面（grep creator_id/myIp 无结果）；zh.json ip.* 无 pending/rejected 文案；IPDetail.tsx 不渲染 status。
  - ApproveIP/RejectIP（handler/admin.go:145-197）无 Notify 调用；notifications 表中 319 无 ip 相关记录（附录 A-3）。
- 影响：五问全断——入口不可达、状态不可知、结果无反馈、无重提出口、无触达闭环。IP 创建旅程只有 API。
- 修复方向：/ips 或 studio 加「提交新 IP」表单 + 我的提交列表（状态+原因）+ approve/reject 通知。
- 涉及文件：IPPicker.tsx、IPBrowseClient.tsx、admin.go、ip_service.go。

### B-11 [A] P1 —— IP 拒绝原因被 UI 收集后丢弃；无修改重提路径（一次死刑）
- 证据：
  - 前端 admin/ips/page.tsx:193-199：ConfirmModal `requireReason` + `reasonLabel` 收集拒绝原因，但 `onConfirm={async (_reason) => { await handleAction(...) }}` 丢弃 `_reason`。
  - 后端 `handler/admin.go:172-197` RejectIP 不解析任何 body 字段；service 层 `RejectIP(id)`（ip_service.go:207）无 reason 参数。
  - `model.IPReviewLog`（ip.go:21-32）含 Reason 字段但**全仓库无写入方**；运行时：带 reason payload reject 后 ip_review_logs 0 行（附录 A-2）。
  - 无 PUT/PATCH /ips/:id、无重提接口（routes.go:81-86 仅 POST/GET）；CreateIP 对 rejected 无状态覆盖逻辑。
- 影响：admin 被要求填原因 → 填了也白填；创建者只见 rejected 不知为何、改了也没法重交。
- 修复方向：RejectIP 接收 reason 落 ip_review_logs + 通知创建者；提供编辑重提（rejected→pending）。
- 涉及文件：admin/ips/page.tsx、admin.go、ip_service.go、model/ip.go。

### B-12 [A] P1（断言 F8 证实）—— stats_service 用 'published' 统计 IP，首页/统计的 IP 数恒为 0
- 证据：
  - `backend/internal/service/stats_service.go:46`：`Table("ips").Where("status = ?", "published")`；全局 IP 词表是 pending/approved/rejected/banned（model/ip.go:14, ip_repo.go:69-73, admin.go:160/187）。
  - 运行时（附录 A-1）：DB 16 approved；`GET /api/v1/stats/summary` → `ips:0`（清 stats:summary 缓存后仍 0）。
  - 对照：`GET /api/v1/ips/stats/category_counts` 正常（ip_stats_service.go:31-37 按 content_items.status='published' join，用的是内容状态，语义正确）。
- 影响：任何展示平台统计的地方 IP 数恒 0。
- 修复方向：改 'approved'。
- 涉及文件：stats_service.go。

### B-13 [B] P2 —— IP ban 级联是死代码：无 API 入口；级联下架与 Green 自动 ban 均无作者通知
- 证据：
  - `IPService.BanIP`（ip_service.go:220-227）与 `BanIPAndContents`（ip_repo.go:116-123，IP+全部内容同事务置 banned）无任何 handler/route 调用（grep 全仓库仅 service/repo 两处定义）。admin 路由表（routes.go:325-366）无 /ips/:id/ban。
  - Green 自动路径存在：`review_service.go:453-468` processIPReviewResult（block → IP+内容全 banned），同样无通知。
  - 内容侧对比：content block 会扣信誉+冻结发布（review_service.go:326-342），IP 级联对创建者零告知。
- 影响：IP 封禁只能 psql；被封作者不知情、内容批量消失无解释。
- 修复方向：补 admin ban IP 路由 + 通知（含原因）；本地演示可用 admin API 构造。
- 涉及文件：routes.go、admin.go、ip_service.go、review_service.go。

### B-14 [B] P2 —— pending/rejected/banned IP 详情页对所有人可见；admin 决策后 Redis 缓存 5 分钟不失效
- 证据：
  - `handler/ip.go:91-109` GetIP（optAuth）无状态/属主过滤；运行时：anon 直接 GET 待审/被拒/被 ban 的 IP 225 均返回完整数据（附录 A-2/A-3）；列表接口默认 approved（ip_repo.go:69-73，运行时搜索 total:0），但详情页绕过。
  - IPDetail.tsx 不显示任何状态徽标（作者视角=访客视角）。
  - 缓存：AdminHandler 用 `service.NewIPService`（admin.go:59-65，**不带 redis**）→ `InvalidateIPCacheForAdmin` 内 rdb==nil 直接 return（ip_service.go:234-240）→ cache:ip:%d（TTL 300s，config.yaml:275）stale。运行时：API reject 后 anon GET 仍返回 pending；psql 改 banned 后仍返回 pending（附录 A-3）。
- 影响：未过审/被拒 IP 可被直链传播；admin 操作延迟 5 分钟生效。
- 修复方向：GetIP 对非 approved 限制为创建者/admin；AdminHandler 注入带缓存的服务或直接 DEL 键。
- 涉及文件：handler/ip.go、handler/admin.go、ip_service.go。

### B-15 [B] P2 —— 判官考试可「单题通过」且无重考限制；提交时重新随机抽题可能与答卷错位
- 证据：
  - `judge_service.go:77-78`：`total := len(input.Answers)`，分子分母都由请求方控制 → 运行时：只交 1 道对的题即 `1/1=100% ≥ 80%` 通过（附录 A-6）。
  - `judge_service.go:55`：SubmitExam 重新 `GetRandomQuestions`，与 GET /exam 展示的题目集无关——题库 >10 时按展示题作答会被按另一套题评分（正确数被低估）；当前本地题库为空（judge_questions 0 行），所有考试一律 503（附录 A-6），真实暴露少。
  - 无重考次数/冷却限制（SubmitExam 无 qualification 前置检查）；ui-spec /judge/exam 要求「已通过考核显示『已获得资格』」未实现（exam/page.tsx 无既有资格查询）。
  - 503 INSUFFICIENT_QUESTIONS 未入 user-facing-error 映射（user-facing-error.ts 无该 code），落到通用「加载失败」文案。
- 修复方向：total 用实际下发题目数；服务端暂存考试会话（或签名题目集）；加资格检查与重考冷却。
- 涉及文件：judge_service.go、judge.go(handler)、exam/page.tsx、user-facing-error.ts。

### B-16 [B] P2 —— 判决详情 API 与前端契约不匹配；理由投票可自赞、409 静默、匿名可读判官身份
- 证据：
  - 前端 VoteWithMeta 期望 `judge_name/upvotes/downvotes/user_vote_type`（VerdictDetail.tsx:25-35），后端返回裸 `[]model.JudgeVote`（judge.go:124-136）——运行时 verdict JSON 只有 id/case_id/judge_id/vote/reason/created_at（附录 A-8）；upvotes 显示为空白，点赞后 `Math.max(0, NaN)` 计数异常。
  - `POST /judge/reasons/:id/vote`（judge.go:138-166）：无判官资格校验、无「是否本人理由」校验（运行时 319 给自己的理由点赞成功）、案件未闭也可投；重复投票 409 被前端静默吞掉（VerdictDetail.tsx:87-90 不提示）。
  - GetVerdictDetail 为 optAuth：匿名可读全部判官 ID+投票+理由（运行时 anon 200）。
- 修复方向：verdict 聚合查询返回展示字段；409 给「已投过」toast；禁止自赞；要求登录。
- 涉及文件：judge.go、VerdictDetail.tsx。

### B-17 [C] P2 —— 贡献者权限边界=仅「可邀请他人」；贡献者/屏蔽管理页数据是前端拼装的假象
- 证据：
  - `IsContributor` 全仓库只有 collab_invite_service 使用（grep）——接受邀请后唯一实际权力是替该内容发邀请（collab_invite_service.go:143-151）；**不能**编辑内容、无 PR 以外的写路径，也不影响 PR 权限（PR 本就人人可交）。
  - dashboard/contributors/page.tsx:36-56：贡献者列表由前端遍历自己内容 + 各内容 **status=accepted** 的 PR 客户端拼装（merged PR 与邀请来源的贡献者不出现）；`blocked` 硬编码 false，无 GET 屏蔽列表接口，屏蔽后刷新即回「正常」。
  - 屏蔽无告知（BlockContributor，pr_service.go:218-220 无 Notify），被屏蔽者下次提交 PR 才收到 403 BLOCKED。
- 修复方向：后端出 GET contributors（含 block 状态）接口；屏蔽时可选通知；明确贡献者权益文案。
- 涉及文件：collab_invite_service.go、dashboard/contributors/page.tsx、pr_service.go。

### B-18 [C] P2 —— 版本系统与展示脱节：merge 结果读者永远看不到；版本全文 API 无鉴权
- 证据：
  - `ManualMerge`（pr_service.go:186-202）只建 ContentVersion+SetLatest，**不同步 content_items.body**；内容详情读 ContentItem（handler/content.go:302-379 GetContent 不查版本）→ 读者看到的仍是旧正文。
  - `VersionService.CreateNewVersion`（version_service.go:50）无调用方（死代码，作者自身的编辑也不产版本）。
  - `GET /versions/:id`（routes.go:109-111，optAuth）无属主/状态校验：任意内容的任意版本全文（含被 ban 内容）匿名可读（version_service.go:88-120 无任何过滤）。GET /pr/:id 同样 optAuth 可读任意 PR 与 reject_reason。
- 修复方向：merge 后同步正文（或展示层读最新 active 版本）；版本/PR 详情加属主或参与方校验。
- 涉及文件：pr_service.go、version_service.go、handler/content.go。

### B-19 [C] P3 —— /ips 排序 4 选 3 无效；admin IP 表「提交者」列恒空
- 证据：
  - 前端排序选项 `hot/most_contents/newest/name`（IPBrowseClient.tsx:59-62）；后端仅识别 `most_content`（ip_repo.go:96-102，其余全落 created_at DESC）。运行时（附录 A-9）：hot/most_contents/newest/name 四个参数返回完全相同顺序，most_content 才真正变序。
  - admin/ips/page.tsx:19 掻 `submitter_id` 字段，后端 IP json 是 `creator_id`（model/ip.go:12）→ 提交者列渲染 undefined（空白）。
- 修复方向：对齐排序词表（hot 可接 rank:hot:ips）；字段名对齐。
- 涉及文件：IPBrowseClient.tsx、ip_repo.go、admin/ips/page.tsx。

### B-20 [C] P3 —— 判官激励与通知闭环依赖未运行的 worker；判官信誉分奖励是无调用点的死代码
- 证据：
  - `AwardJudgeAccuracy`/`PenalizeJudgeError`/`AwardPRMerged`（reputation_service.go:100-185）全仓库无调用方；judge_service.go:206 的撤权扣分又在死路径里（B-09）。运行时：319/320 投票+闭案后 reputation_logs 0 行（附录 A-8）→ 「+1 准确率奖励」用户零感知（business-rules.md:27 的表项落空）。
  - `NotificationService.Notify`（notification_service.go:274-306）在 queue producer 存在时只发 Redis stream `omnicraft:notification.create`，由 cmd/worker 消费落库；本环境 worker 未运行：stream lag=8、last-delivered-id=0-0，PR accept 通知（已验证在流中）永不落库。所有动态通知（PR 结果/评论/点赞）本地 demo 不会到达消息中心。IP approve/reject/ban、判官闭案则**根本没有**通知代码（B-02/B-10/B-13）。
- 修复方向：本地演示启动 cmd/worker；闭案/裁决补 AwardJudgeAccuracy 调用。
- 涉及文件：reputation_service.go、judge_service.go、notification_service.go、部署脚本。

---

## 二、预探查断言结论

### F6 —— 部分证实，一处关键证伪
- ✅ 证实：判官案件唯一来源是 Green「review（争议）」级。`ensureJudgeCase` 仅被 `applyContentReviewResult` 的 `case "review"` 调用（review_service.go:343-353→524-547）；`block/violation` 直接 banned+扣分+冻结（L325-342），不建案。评论类无判案（B-04.4）。
- ✅ 证实：block 高危内容绝不进判官队列（同一 switch 分支互斥）。
- ❌ 证伪：「判官看到的是 under_review 原始内容」——判官 UI 不渲染任何内容本体（B-04），仅凭 target_id 盲投。
- 备注：本地无 Green（题库/案件均为 0 行），案件只能人工构造；真实阿里云下（推演）Green review 判定的文本/图片/视频争议会进入队列，同样存在「看不到内容」的问题。

### F7 —— 两条证实，两条证伪
- ✅ 单人单案一票：`HasVoted` 前置 + `(case_id,judge_id)` 唯一索引（judge.go:33-35）；运行时重复投票返回 409 ALREADY_VOTED。
- ✅ ≥MinVotes（config.yaml judge.min_votes_required=20）按 ≥60%（pass_threshold=0.60）闭案：judge_service.go:148-163；运行时 19/19 approve 闭为 closed_approve。
- ❌ 错误率>50% 撤权+扣分：`checkAndRevokeIfNeeded` 判 `Status != "closed"`（judge_service.go:185）而真实状态是 closed_approve/closed_reject → 全部 continue，永不触发（B-09）。
- ❌ 判官 +1 准确率奖励：AwardJudgeAccuracy 无调用点（B-20）。

### F8 —— 证实（含一处运行时实证，一处运行时不可达）
- ✅ 状态机 pending → approved|rejected|banned：model 默认 pending（ip.go:14）、admin approve/reject（admin.go:160/187）、banned 经 BanIPAndContents/Green block（ip_repo.go:116-123、review_service.go:461-467）。
- ⚠️ 「IP ban 级联下架全部内容」代码存在且同事务，但无 API 入口（死代码，B-13）；Green 自动 ban 路径可达（推演：真实 Green 下 IP 封面/文本 block 会触发）。
- ✅ stats_service 按 'published' 统计 IP：stats_service.go:46；运行时 16 approved vs 接口 ips:0（B-12）。

---

## 三、状态 × 角色矩阵

### IP × 角色（创建者 / 访客 / admin）
| 状态 | 创建者所见 | 访客所见 | admin 所见/操作 | 通知 | 出口 |
|---|---|---|---|---|---|
| pending | 详情页无状态提示（与访客同视图）；无「我的 IP」页 | 详情直链可看（GetIP 无过滤）；列表/搜索不可见 | admin/ips 表可 approve/reject | 无 | → approved/rejected（AI block→banned） |
| approved | 正常展示 | 列表+详情可见 | — | 无（不知道过审了） | 终态（可被 ban） |
| rejected | 详情直链可看但无原因展示（原因根本没存） | 详情直链可看 | 从列表消失 | 无 | **无重提路径**（一次死刑） |
| banned | 详情直链可看（无提示）；其内容全部消失 | 同左 | 无 API 入口（B-13） | 无 | 无解封路径 |

### PR × 角色（贡献者 / 作者 / 访客）
| 状态 | 贡献者所见 | 作者所见 | 访客 | 通知 | 出口 |
|---|---|---|---|---|---|
| open | **无任何 UI 可提交/查看自己的 PR**（B-06） | 旧 dashboard 页可见（studio 空壳，B-08）；Diff 右栏恒空（B-05） | GET /pr/:id 可读（无鉴权） | 无新 PR 提醒作者 | accept/reject/merge |
| accepted | 通知 pr_accepted（代码存在，依赖 worker） | 列表页过滤 status=open 后消失 | 同上 | pr_accepted（流中验证） | **merge 被拒（B-07）**；内容零变化 |
| rejected | 通知含原因（pr_rejected reason） | 同上 | 可读 reject_reason | pr_rejected | 无重新提交引导（可再发新 PR） |
| merged | 通知 pr_merged | — | 版本可读 | pr_merged | 版本不同步正文（B-18），无 UI 触发 merge |

### judge_case × 角色（判官 / 作者 / 访客）
| 状态 | 判官所见 | 内容作者所见 | 访客 | 出口 |
|---|---|---|---|---|
| open | 队列出（仅资格类型）；内容本体不渲染（B-04）；已投案件仍出现在队列 | 内容 under_review 仍公开可读 | 同左 | 投票达 20×60% 闭案 |
| closed_approve | verdict 页（结案横幅因词表 bug 不显示，B-09） | **内容仍 under_review + 无通知（B-02）** | verdict 可匿名读 | 无（内容 limbo） |
| closed_reject | 同上 | 同上（规格要求「不予展示」，实际内容照旧可读） | 同左 | 无下架动作 |

---

## 四、API-UI 对照缺口（后端有、前端无）
| API | 后端状态 | 前端 |
|---|---|---|
| POST /api/v1/ips | 可用（本地验证） | 无创建入口（B-10） |
| POST /api/v1/pr | 可用（本地验证） | 无提交入口（B-06） |
| POST /api/v1/pr/:id/merge | 可用（open 状态下） | 无调用（B-07） |
| POST /admin/ips/:id/reject 的 reason | 不支持 | UI 收集但丢弃（B-11） |
| GET /api/v1/notifications 列表页 | 可用 | 仅 dropdown 5 条；「查看」与每条点击一律跳 /messages（getNotificationUrl 定义了从未调用，NotificationDropdown.tsx:79-92/172-177） |
| GET /versions/:id、GET /pr/:id | 可用 | 仅 PR 详情内部用；无独立版本历史页（用户不可见版本链） |

反向缺口（前端有、后端无）：/ips 排序 hot/name（B-19）；VerdictDetail 的 judge_name/upvotes（B-16）；SubmitPREntry 的 create=1（B-06）。

---

## 五、修复优先级建议（供排期参考，不在本次执行）
1. B-01（考试字段名）——一行级修复，解锁整个判官演示。
2. B-02 + B-09（闭案生效与词表统一）——众裁闭环成立的最小集。
3. B-05 + B-07 + B-06（PR 提案落库 + merge 可达 + 提交入口）。
4. B-11 + B-10（IP 拒绝原因链 + 创建入口/我的 IP）。
5. B-12（stats 一词之改）、B-04（判官内容预览与媒体防线）。
6. 其余 P2/P3 按演示脚本需要取舍。

---

## 附录 A：运行时构造与复现记录（全部痕迹已恢复/删除）

### A-0 登录
- CSRF：`curl -s -c cookie http://localhost:8080/api/v1/auth/csrf` → csrf_token。
- 319/320/admin 登录 `/api/v1/auth/login`，密码 Audit!2026pass，得到 Bearer token（存 /tmp/B-tok*.txt）。

### A-1 F8 stats（B-12）
1. `POST /api/v1/ips`（319）→ IP 225 pending（后续清理删除）。
2. `SELECT status,COUNT(*) FROM ips GROUP BY status` → approved 16 / pending 1。
3. `GET /api/v1/stats/summary` → `{"users":26,"ips":0,"contents":169}`。
4. `redis-cli DEL stats:summary` 后重测仍 ips:0。
5. `GET /api/v1/ips/stats/category_counts` → 正常返回 6 类计数。

### A-2 IP reject 原因丢失（B-11）
1. admin `POST /api/v1/admin/ips/225/reject` body `{"reason":"审计B：测试拒绝原因是否落库"}` → 200 "ip rejected"。
2. `SELECT * FROM ip_review_logs` → 0 行；notifications(user 319) 无新记录。

### A-3 IP 可见性与缓存（B-14）
1. anon `GET /api/v1/ips/225`（pending/已 reject/banned 三阶段）均 200 返回完整 IP。
2. reject 之后 anon GET 仍返回 status=pending（缓存 stale）；`UPDATE ips SET status='banned'` 后仍 pending（证明 admin 路径失效缓存无效）。
3. `GET /ips?q=Audit B` → total 0（列表过滤正常）。

### A-4 PR 提案丢失（B-05）
1. content 176（author 319）最新版本 627。
2. 320 `POST /api/v1/pr` `{content_item_id:176, base_version_id:627, message, new_text}` → PR id=1。
3. `GET /api/v1/pr/1` → 无 proposed_version_id；pull_requests.proposed_version_id NULL；content_versions(176) 仍 1 行。

### A-5 accept→merge 互斥（B-07）
1. 319 `POST /api/v1/pr/1/accept` → 200。
2. 319 `POST /api/v1/pr/1/merge` `{merged_text}` → 400 ERROR。
3. pr_accepted 通知出现在 Redis stream（id 1788203440025-0），notifications 表无行（worker 未运行）。

### A-6 判官考试（B-01/B-15）
1. `GET /api/v1/judge/exam/article`（空题库）→ 503 INSUFFICIENT_QUESTIONS。
2. admin `POST /api/v1/admin/judge/questions`（10 题，question_data 需 base64）→ created:10。
3. `GET /judge/exam/article` → 10 题（correct_key 已删除，无泄漏）。
4. 前端同构 payload（answer_key，全对）→ `passed:false score:0`。
5. 后端字段（answer，全对）→ `passed:true score:10`，judge_qualifications 生成。
6. 单题 payload（1 对）→ `passed:true score:1/1`（作弊向量证实）。

### A-7 队列与资格
1. 319 取得 article 资格后 `GET /api/v1/judge/queue` → 返回 target_type=article 的 open 案件（psql 建案 1：article/177/min_votes 20）。
2. 319 投 approve+reason 成功；重复投票 → 409 ALREADY_VOTED。

### A-8 闭案无效果 + 契约缺失（B-02/B-09/B-16/B-20）
1. psql 填充票：users 314-318、321-332 各 1 approve（judge_id FK 需真实用户；300+/400 失败）。
2. `UPDATE judge_cases SET min_votes=19`；删除 320 旧票后 320 再投 → 案件 `closed_approve`（19-0）。
3. `SELECT status FROM content_items WHERE id=177` → 仍 under_review；notifications(user 320, %judge%) = 0；reputation_logs(319/320) = 0。
4. `GET /api/v1/judge/cases/1/verdict`（anon 200）→ votes 仅裸字段（无 judge_name/upvotes）。
5. `POST /judge/reasons/1/vote`（319 给自己的理由）→ 200；重复 → 409。

### A-9 /ips 排序（B-19）
- `GET /api/v1/ips?sort={hot|most_contents|newest|name}` → 四者 ID 序列完全相同；`sort=most_content` 才变化。

### A-10 清理（恢复原状）
- DELETE judge_reason_votes/judge_votes(case 1)/judge_cases(1)/judge_qualifications(319,320)/judge_questions(审计B%)/judge_exam_records(319,320) → 五表全 0（与初始一致）。
- DELETE content_contributors(176,320)、pull_requests(1)、ips(225)。
- `XDEL omnicraft:notification.create 1788203440025-0`（仅删本任务产生的条目；stream 回到 8，并行任务 C 的条目未动）。
- 未触碰：seed-ui-002~006/009~014、种子内容/IP（IP 215「南风食堂」为 319 既有种子，保留）、admin 账号状态。

## 附录 B：审计中未走运行时、仅代码定论的项
- B-03（举报不建案）：social_service.go:277-309 全路径无 ensureJudgeCase。
- B-13（BanIP 死代码）：grep 全仓库调用点。
- B-17（贡献者权限）：grep IsContributor 全部调用点。
- B-18（版本展示脱节）：GetContent 无版本查询；CreateNewVersion 无调用方。
- collab-invites 旅程：SendInvite/AcceptInvite/DeclineInvite 与 CollabInviteCard/ChatWindow 代码链路完整（含 Redis 配额、行锁、过期清扫调度器 wired：main.go:88-89），未做破坏性运行时测试（邀请会污染 320 的会话与邀请表，且 e2e collab-invite-flow.mock.spec.ts 已覆盖交互）。
