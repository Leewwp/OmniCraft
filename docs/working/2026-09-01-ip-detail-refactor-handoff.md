# IP 详情页「贴吧式社区枢纽」重构 — 交接文档（设计已定稿，原型已交付）

- 创建日期：2026-09-01
- 预计失效日期：2026-11-01（正式实现完成后归档）
- 性质：三轮 grilling 设计讨论的全部决策 + 现状事实索引。可交互原型已交付；正式实现由 GitHub #290 与整合总序驱动。
- 当前执行入口：先完成 U-01~U-05，再按总序第 5 段实现 #290；不得重复开发原型或把原型假数据当作生产实现。

---

## 0. 产品概念（一句话）

把 IP 详情页从"资料展示页"重构为**贴吧式社区枢纽**：同喜好用户聚集地。头部是 IP 身份区，下方主体是单页三模块浏览区（内容分享 / 讨论区 / 提案投票），全部页内切换不跳页，辅以 IP 内搜索。治理模式：**无吧主、人人共治**——IP 资料改动通过「共治提案」由关注者投票决定。

## 0.5 原型已交付（2026-09-01）

- 交互原型可运行：`/prototype/ip-detail-hub`（`frontend/app/(public)/prototype/ip-detail-hub/page.tsx` + `frontend/components/prototype/ip-detail-hub/`，生产构建下 404 兜底，不动生产代码）。覆盖本文 §4 全部 7 项；场景切换 `?scenario=member|guest|empty`。验证截图 `screenshots/proto-*.png`。注意：截图 proto-11/11b/12 中的"分组摘要"视图属已废弃的旧搜索方案，以 §1.1 修订后的 tab 内过滤为准。
- 附带 UI 修订：头部关注按钮固定最小宽度（min-w-[104px]），hover 换文案不改变宽度、不引起布局抖动；正式实现 FollowButton 时沿用此约束。

## 1. 已定稿决策（三轮讨论全部确认，不得擅改）

### 1.1 信息架构与页面结构

- **头部（IP 身份区）**：封面图、名称、类目、简介、**标签 TagBadge 照常展示（保留）**；统计三项：**关注数 / 讨论数 / 作品数**；关注按钮（沿用 FollowButton 契约）。**不做**创建者署名、不做"最近更新"小字。
- **主体（浏览区）**：三个模块 tab —— 内容分享 / 讨论区 / 提案投票。点击 tab → 主体内容切换，下方出现该模块的下级筛选。
- **单页 query 驱动，无跳页**：`/ip/[ipId]?tab=share|discussions|proposals&type=&sort=&q=&page=`，客户端换主体（先例：`frontend/components/ip/IPBrowseClient.tsx:119-128` 的 `router.replace`；collections 页 `ContentTypeFilter` 用 `replaceState` 同步 query）。浏览器后退可用。
- **两个详情用浮层不跳页**：作品详情复用 `ContentDetailOverlay` 既有模式；**讨论帖详情浮层化是新内容（含回帖功能，Esc/后退关闭）**——原型必须包含。
- **旧子路由 301**：`/ip/[id]/[category]`、`/ip/[id]/discussions` → 对应 `?tab=` 形态。
- **搜索：tab 内过滤模式（2026-09-01 修订，取代原 Q8-b「分组摘要」方案）**：搜索框常驻模块行上方；**回车或焦点离开输入框才提交**（输入过程不即时过滤）。提交后三模块 tab 保持原样、不切换独立搜索视图，各 tab 展示同一关键词在该模块的过滤结果；**tab 计数随命中数收缩**（无搜索 10/8/3 → 搜索后 5/1/0），内容分享的媒体类型 chips 计数同步收缩。当前 tab 无命中 → 页内空态「未找到与「q」相关的内容」+ 清空搜索按钮。清空输入或点 X 还原全量列表与计数；`?q=` 走 history push，后退可回到上一搜索态。废弃：分组摘要、模块 chips 深入、相关度排序、`?stab` 参数。实现口径：对三模块各自列表的查询层过滤，不建聚合端点/聚合视图。

### 1.2 各模块筛选与排序矩阵

| 模块 | 子筛选 | 排序 |
|---|---|---|
| 内容分享 | 媒体类型 chips（图片/文字/视频/音频/mod/prompt/曲谱/其他，后端白名单 9 类） | 最新 ✅ / 最热 ✅ / 最多浏览 ✅ / 最高评分 ✅（仅接线） |
| 讨论区 | **无筛选** | 最新回复 ✅ / 最新发布 ✅ / 最多回复 ✅ / **热门 ➕** |
| 提案投票 | 状态：进行中 / 已通过 / 已否决 / **改动历史** | 默认=进行中 + 即将截止优先，其次最新 |
| 搜索（激活时） | 各 tab 内过滤，媒体 chips 计数同步收缩 | 沿用当前 tab 排序（不新增相关度排序） |

- 内容分享 tab **只收该 IP 的二创**（沿用现状 `zone=fanwork`，不改）。
- **热门讨论**：`log(reply_count+1)/龄期衰减`，MVP 用 SQL 表达式实时排序，不建列不建定时任务，参数进 config。热门排序下置顶帖仍置顶（`is_pinned` 优先保留）。

### 1.3 共治提案（全新域，独立于赛博判官）

- **域边界**：独立新域，新表（如 `ip_proposals` / `ip_proposal_votes`），只借鉴 judge 的 case/vote/门槛/结案代码模式，**不共表不共享队列**。判官=内容违规众裁；共治提案=IP 资料协作编辑，两者概念相反不可混用。
- **可提案字段（字段级提案，一次可含多个字段）**：
  - **简介** ✅ 第一步；
  - **封面** ✅ 第一步（注意：需接图片上传 + 图片审核链路，实现量更大）；
  - **标签** ✅（第三轮新增）：标签可在共治提案中编辑。**每次新增或移除一个标签 = 一条提案**；**已存在于 IP 上的标签不能再提案"加入"**（需做此类兼容校验；同理移除不存在的标签应被拒绝）。标签在头部照常展示。
  - **不可提案**：名称 / slug / 类目（锁死）。
- **资格**：提案权 = 登录 + 信誉分 ≥3（对齐全站 interaction guard）；投票权 = **已关注该 IP** + 信誉分 ≥3；一人一提案一票（赞成/反对二选一），提案者可投且计入；不设最短关注时长。
- **生命周期**：`open → adopted / rejected`。结案 = 达标即结案 **或** 截止时间到（未达标 = rejected）。config 新节 `ip_proposal`：`min_votes=10 / pass_threshold=0.6 / deadline_days=7`（独立于 judge 配置）。
- **生效动作（判官的反面教材，必须做对）**：通过后同事务 `UPDATE ips` + 写 `ip_profile_versions`（旧值快照，数据真源）+ 失效 Redis `cache:ip:{id}`。
- **并发约束**：**同 IP 同时只允许一个 open 提案**。
- **提交审核**：提案提交时走与 IP 创建相同的 AI 文本审核（本地 fail-open）。
- **投后不可改不可撤**（与判官一致）。
- **未关注者点投票** → 页内小面板"关注该 IP 后即可参与共治投票" + 一键关注，关注成功原地解锁；未登录 → 跳登录页。
- **提案卡片**（原型必含）：提案人昵称 + 发起时间 → 字段标签 → diff 区（**简介行内 diff**：删除线红字 + 下划线绿字，长文默认展开前 8 行；**封面旧图 → 新图并排小卡**约 160px；**标签 diff**：`+新标签` 绿 / `-移除标签` 红 chips）→ 底部操作条：赞成/反对双色进度条 + 门槛刻度（"3/10 票"）、剩余时间（"剩 4 天"）、赞成/反对按钮；已投者显示自己的选择并锁按钮。
- **改动历史**：投票区第四个状态筛选。已生效提案按**生效时间**倒排的时间线视图（顶部时间轴 + 卡片含 diff、票数、生效时间），与"已通过"（按结案时间、看投票结果）视角区分。不另立独立页面。
- **通知/outbox（实现期）**：三事件——新提案 → 全体关注者（走 outbox 新事件，修复 F-064 的正路，不在 Notify 里扇出）；提案通过/否决 → 提案人 + 投票参与者；深链 → `/ip/[id]?tab=proposals`。关注动作的 user_id=0 通知删除。本地需跑起 `cmd/worker`。

### 1.4 置顶（第三轮裁决）

- **置顶权收归系统管理员**，不归 IP 创建者（现状后端是创建者专属 `backend/internal/handler/discussion.go:129-158`，实现期需改）。
- **暂不做任何前端置顶入口**（管理员走 API）。讨论卡片可渲染置顶标识，列表排序 `is_pinned` 优先保留。
- 置顶提案化挂 Phase 2 候选。

### 1.5 头部统计数据

- 关注数 / 讨论数：后端 `GetIP` 现不含（`CountFollowers` 死代码、`ListByIP` 无 status 过滤），实现期接线；作品数已有现成统计路径（`ip_stats_service.go`）。

### 1.6 反作弊（MVP 接受现状）

- 邮箱验证 + 信誉分 ≥3 + 一人一票 DB 唯一约束 + 门槛 10 票稀释。设计文档"已知风险"记录；Phase 2 候选：投票权重 / 账号龄 / 关注时长。

## 2. 与存量工单/轨道的边界

- **吸收进本计划**：类目页死排序（`[category]/page.tsx:112-121` 无 onChange）、`mod` 类目 label 错映射（`frontend/components/ip/ipCategory.ts:12`）、FIX-25（IP 详情可见性与 admin 缓存失效）、F-064（关注通知 user_id=0）。
- **保持独立不随重构走**：FIX-18（讨论发帖信誉门/审核横切修复）、FIX-23（标签上限/审核加固，标签保留后继续有效）。
- UI 遵守 2026-09-01 精修方向（灰色画布 + 白卡片、全控件 8px 圆角矩形、等高行、页内筛选优先），**design tokens 以 `design/design-system.md` 为准**。

## 3. 现状关键事实索引（实现时直接查）

- 前端路由与组件：`frontend/app/(public)/ip/[ipId]/page.tsx`、`[category]/page.tsx`、`discussions/` 三页；`frontend/components/ip/IPDetail.tsx`（头部）、`IPCategoryTabs.tsx`（Link 跳转，需重做为页内 tab）、`components/social/DiscussionBoard.tsx`（compact 模式 + 贴内搜索）、`components/content/OverlayMasonryGrid.tsx`（点卡开浮层）。
- 可复用组件：`components/content/ContentTypeFilter.tsx`（pill chips + 计数 + replaceState 先例）、`components/ui/tabs.tsx`（零引用的 tab 原语）、`components/ui/data-list.tsx`（无限滚动）、`components/ui/SortSelect.tsx`。
- 后端：IP model `backend/internal/model/ip.go`（无 update 端点、无版本表）；follows 表支持 `target_type='ip'`（migration 011/033）；讨论 `discussions` 表三种排序已有（`repository/discussion_repo.go:18-37`）；`GET /ips/:id/contents`（`handler/ip.go:118`）未透传 type/sort（底层 `ListContentsFilter` 全支持）；`/contents/search` 无 `ip_id` 参数；`/ips/:id/discussions/search` 存在但 `plainto_tsquery` 中文整句匹配基本搜不到（需升级分词 + ILIKE 兜底）。
- 判官参考件（只借模式）：`service/judge_service.go:123-170`（投票/门槛/结案）、UNIQUE(case,judge) 防重复；反面教材：裁决不回写对象（F-003）、无截止时间、SubmitVote 不校验资格。
- 词表权威：`docs/GLOSSARY.md`——禁止叫"IP 主页/IP 空间"；「IP 详情页」词条需改写；新增「共治提案 | IP Proposal | 关注者投票决定 IP 资料（简介/封面/标签）改动的协作治理机制；禁止与"众裁/判决/判官"混用」。
- ui-spec 需改写章节：`design/ui-spec.md` 的 `## Page: /ip/[ipId]`（612 行）、`## Page: /ip/[ipId]/[category]`（662 行）、`## Page: /ip/[ipId]/discussions*`（708/754/803 行）、`## Component: IPCategoryTabs`（2273 行）。
- 实现期命名约定：config 新节 `ip_proposal.*`；迁移从 073 起；i18n 新命名空间 `proposal.*`（消息文件 `frontend/messages/zh.json`/`en.json`，全量 i18n 硬禁令）。

## 4. 原型范围（2026-09-01 已交付；正式实现边界）

1. 单页 `/ip/[ipId]`：头部（图/名/类目/简介/标签/三统计/关注按钮）+ 三模块 tab + 下级筛选 + 排序，全部页内切换。
2. 内容分享 tab：媒体类型 chips + 四排序 + OverlayMasonryGrid 作品卡 + 作品详情浮层。
3. 讨论区 tab：四排序（含热门）+ 讨论卡片 + **讨论帖详情浮层（含回帖）**。
4. 提案投票 tab：四状态筛选（含改动历史时间线）+ 提案卡片（简介行内 diff / 封面并排 / 标签 ±chips + 双色进度条 + 门槛刻度 + 剩余时间 + 投票按钮与已投锁定态）+ 发起提案表单 + 未关注引导面板。
5. IP 内搜索：搜索框 + 三模块 tab 内过滤 + 当前 tab 排序；原型中的「分组摘要 / 模块 chips 深入」为废弃方案，不得在正式实现中恢复。
6. 空态：无 open 提案 CTA（"第一个提案由你发起"）、各模块空态。
7. 上述静态/模拟数据已用于原型验收；正式实现必须接入 #290 规定的真实 API、迁移、权限与 i18n 契约。

## 5. 正式实现提示

- 正式实现遵循 #290 的 heavy 车道、迁移与 API 验收；不再调用 `prototype` 生成第二份原型。
- 开工前先读：`design/design-system.md`、`design/ui-spec.md` 对应章节、本文档第 1~4 节及 `docs/working/2026-09-02-integrated-execution-order.md`。

## 6. 正式实现边界

- 原型阶段的「不动生产代码 / 不建迁移 / 不改 routes.go」已结束；正式实现按 #290 允许修改生产代码、迁移、routes、配置与 i18n，并执行 heavy 验证门。
- 不做置顶前端入口；不做名称/类目提案；不做投票权重/账号龄反作弊；不做 Phase 2 项（置顶提案化、热门分落列、投票权重）。
