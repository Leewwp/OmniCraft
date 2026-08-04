# P-01 发现流 / Agent 工作台 / 内容详情视觉原型评审记录

**创建日期**: 2026-07-25
**预计失效日期**: 2026-09-25
**关联计划**: `docs/archive/plans/2026-07-18-omnicraft-ui-polish-hardening.md`(Gate P-01；2026-08-04 归档)
**状态**: ✅ 2026-08-01 P-01 已获批；IP 库选择 A/B/C 变体中的方案 B（Indigo 精修）。2026-07-28 取消的是历史 Hallmark Tally 双方向方案 B，不是本次 IP 库变体。U-01 已解阻；生产 `/ips` 回写仍由 U-09 执行。

---

## 1. 当前评审问题

> 如何在不重复构建现有二创、原创和 IP 详情页的前提下，重新设计 IP 库，消除固定窄卡片与弹性 Grid 轨道造成的大量空白，同时保留现有搜索、筛选、排序和进入 IP 详情的行为？

当前单方向原型以“已登录的内容探索者／轻度创作者”为第一受众：他们熟悉社区基本操作，主要发现原创与二创内容，并使用 Agent 查找、理解和延伸站内内容；活跃内容创作者作为第二受众。

单一核心任务仍是“完成一次上下文保留的内容发现闭环”：用户从推荐流或 Agent 回答发现内容，打开共享详情浮层完成理解，再关闭并无损返回原浏览或对话位置。发布、关注、私信和 Agent 写操作继续作为必要交互覆盖，但不是 P-01 的首要成功标准。

**2026-07-28 范围修订**：用户取消 Hallmark Tally 方案 B，不再生成、换肤或要求双方向对比；当前克制、实用的 OmniCraft Indigo 原型成为唯一后续视觉基线。既有 `direction=b` 镜像和截图仅是历史测试产物，不代表仍需完成 Tally 设计，也不得阻塞 P-01；A2 开始时移除误导性的 B 切换入口与 B 专属重复截图矩阵。

必须验证所有内容卡片入口（推荐卡片、分区页卡片、IP 详情页卡片、Agent 引用卡片）打开同一内容详情浮层；浮层承载与完整详情页一致的完整内容（评论区、相关推荐、关联原创/二创、所属 IP），浮层内访问关联内容形成逐层返回的导航栈（返回类手势逐层、退出类手势全退、每层滚动记忆）；完全退出后恢复最初触发入口、滚动位置和焦点。A1.5 将用户资料浮层和由其进入的私信聊天浮层纳入当前原型；真实图片上传、审核和数据迁移仍不属于 P-01。

原型现分四批：A1 修复与合并（已完成）；A1.5 共享交互修订（已完成）；A1.6 细节交互修订（已完成并验证：创作者栏/评论/资料定位/Agent 全文搜索/中性状态/居中私信）；A2 IP 库重设计。A2 仅新增 IP 库隔离表面，生产 `/`、`/original`、`/ip/[ipId]` 作为已接受视觉基线与跳转目标，不复制到 throwaway 原型。

## 2. 现有方向 A 旧基线（隔离、可丢弃）

| 项 | 值 |
|---|---|
| 路由 | `/prototype-ui-polish?view=feed\|detail&state=default\|loading\|empty\|error`(`?bare=1` 隐藏切换栏) |
| 运行 | `cd frontend && npm run dev` 后浏览器访问(无需后端,全部静态内存数据) |
| 代码 | `frontend/app/prototype-ui-polish/`(page / prototype-app / chrome / feed / detail / mock-data / prototype.css) |
| 素材 | `frontend/public/prototype/covers/*.jpg`(真实摄影,本地确定性) |
| 截图脚本 | `frontend/scripts/capture-p01-prototype.mjs`(`node scripts/capture-p01-prototype.mjs`) |
| 截图证据 | `screenshots/ui-polish/prototype/`(23 张旧基线,见 §6) |
| 生产改动 | **无**。未触碰生产页面、`globals.css`、设计 Token、共享组件 |

切换栏(仅非生产构建渲染):←/→ 或箭头按钮切换视图;状态丸切换 默认/加载/空/错误;右端切换明暗主题。

以下旧基线本身不能单独作为 P-01 批准证据；A1/A1.5/A1.6 已补齐推荐、Agent 和详情交互。当前只需沿 Indigo 方向增加 IP 库原型并验证其跳转到现有 IP 详情页。

## 3. 当前 Indigo 方向的拟议设计决策（唯一后续基线）

### 3.1 字阶(消除任意字号)

| 用途 | 值 | 现状对比 |
|---|---|---|
| 详情页标题 | 24px / semibold / tracking-tight | 一致 |
| 区域标题(二创区) | 20px / semibold | 现行 22px arbitrary |
| 卡片/区块标题 | 14px / medium | 现行 13.5px arbitrary |
| 正文 | 16px / leading-relaxed | 一致 |
| 辅助信息(作者、meta、标签) | 12px | 现行 11.5/10.5px arbitrary |
| 侧栏小节标签 | 12px / semibold / uppercase / tracking-wider | 现行 11px arbitrary |

规则:只允许 12 / 14 / 16 / 20 / 24 五档;紧凑指标数字例外须逐个登记。

### 3.2 间距

4 的倍数:卡片内边距 12(p-3),区块内边距 16(p-4),详情卡 20/24,页面 gutter 16(移动)/24(桌面),网格 gap 16,区块间距 24/32。

### 3.3 圆角(修正现行 calc 漂移)

| Token | 值 | 用途 |
|---|---|---|
| radius-sm | 3px | 小元素、排序分段选中块 |
| radius-md | 4px | 按钮、输入框、筛选 chip |
| radius-lg | 8px | 卡片、面板、封面容器 |
| radius-xl | 12px | 大卡片(预留) |
| radius-full | 9999px | 标签、药丸 |

### 3.4 三档细腻微阴影(替代"全局无阴影"旧规则)

| 档位 | 用途 | Light | Dark |
|---|---|---|---|
| elevation-1 静态 | 静置卡片、面板 | `0 1px 2px 0 rgb(24 24 27 / 0.05)` | `0 1px 2px 0 rgb(0 0 0 / 0.30)` |
| elevation-2 悬浮 | hover 升起 | `0 4px 12px -2px rgb(24 24 27 / 0.10), 0 2px 4px -2px rgb(24 24 27 / 0.05)` | `0 4px 12px -2px rgb(0 0 0 / 0.50), 0 2px 4px -2px rgb(0 0 0 / 0.30)` |
| elevation-3 浮层 | 下拉/抽屉/Modal | `0 12px 32px -8px rgb(24 24 27 / 0.14), 0 4px 8px -4px rgb(24 24 27 / 0.06)` | `0 12px 32px -8px rgb(0 0 0 / 0.60), 0 4px 8px -4px rgb(0 0 0 / 0.40)` |

规则:阴影永远配合 1px border,不单独承担分隔;hover 只用 transform/shadow,**不产生布局位移**;`prefers-reduced-motion` 下禁用缩放与脉冲。

### 3.5 颜色(沿用现行 Indigo 极简色板)

- 基础色板不变:`#4F46E5`(light)/`#6366F1`(dark)主色,GitHub 风中性色。
- **新增** `--accent-hover`:`#4338CA` / `#818CF8`(主色深/浅一档),补齐现行 CSS 未定义而被消费的 token。
- **新增** `--border-strong`:`#D4D4D8` / `#444C56`(hover 时边框加深一档)。
- 标签 6 色体系不变。

### 3.6 克制 Indigo 使用清单(仅以下位置)

1. 主要动作:发布 / 关注 / 发布评论(primary 按钮)。
2. 选中态:导航下划线、侧栏选中项、筛选 chip 选中、排序分段。
3. 交互反馈:focus ring、点赞激活、文字链接 hover。
4. 品牌识别:Logo 标记、通知未读点。

其他一切(边框、背景、图标、次要按钮)保持中性色。

### 3.7 卡片决策

| 卡型 | 决策 |
|---|---|
| 二创卡 | 1px border + radius-lg + elevation-1;hover → border-strong + elevation-2 + 封面 scale(1.03);类型角标(黑 55% 半透明 chip);信息层:标题 14 → 基于 IP 12 → 标签(≤2)→ 作者+互动 12 |
| 原创卡 | 无边框,封面自适应比例 + radius-lg;hover → 封面 scale(1.05) + 浅遮罩 + elevation-2;信息仅 标题 14 / @作者 12 / 点赞 12 |
| 详情页卡 | 面板 = 1px border + radius-lg + elevation-1(附件、评论、侧栏卡统一) |

### 3.8 加载 / 空 / 错误决策

- **加载**:骨架块形状镜像真实布局(封面比 + 标题行 + meta 行),1.6s 细腻脉冲,reduced-motion 下静止;禁止全屏遮罩。
- **空状态**:56px 圆形柔底图标 + 16px 标题 + 14px 说明(max-w-sm)+ 可选 CTA;垂直 py-20/24 居中。
- **错误**:同空状态结构,图标 AlertCircle,重试用 outline 按钮;不使用 Toast 承担页面级错误。

### 3.9 导航壳层(供 U-03 参考)

- 新方向 A 必须改为推荐/二创/原创/AI Agent 四项导航；中部搜索保持 keyword-only，不提供 Agent 模式切换。
- 旧基线的首页 IP Sidebar/筛选/排序不得带入顶层推荐流；二创/原创分区页仍可保留各自分类与定向浏览结构。
- Header 52px:左 Logo+四项导航(选中 = 加粗 + 2px accent 下划线),中关键词搜索(柔底、focus 白底 + ring),右 发布(primary)+ 通知(accent 未读点)+ 头像菜单(elevation-3)。
- 移动端:汉堡 → 左侧 85vw 抽屉(elevation-3 + 黑 50% 遮罩)。

## 4. 原型共同边界

- 原型不修改生产路由、Root Layout、共享组件、API、数据库迁移或 feature flag；信息架构变化仅存在于隔离原型和设计文档。
- 不为极简删除必要信息(互动数、IP 归属、标签保留)。
- 原型内文案硬编码仅存在于 throwaway 代码;回写生产时走 next-intl。
- 当前方向继续遵守克制 Indigo 与既有扁平基因，同时保留真实信息密度、品牌可识别性、完整状态、可访问性与移动适配；不再为 Hallmark 方向保留额外设计分支。

## 5. 与现行权威文档的冲突(批准后经 U-01 回写)

| 冲突 | 现行 | 拟议 |
|---|---|---|
| 全局无阴影原则 | design-system.md「无阴影,仅 Modal/Popover shadow-md」 | 三档微阴影(§3.4) |
| 圆角 | CSS calc 派生 4.8/6.4/11.2px | 3/4/8/12px 精确值 |
| 字体栈 | CSS 缺中文回退 | 补 PingFang SC / Microsoft YaHei |
| `--accent-hover` | 未定义被消费 | 登记为权威 token |
| 创作者摘要 | `ContentSidebar`/现有原型在详情中重复展示统计 | 标题下方不重复作者信息；右侧创作者栏仅保留头像、昵称、关注，统计移入用户资料浮层 |
| 用户资料入口 | 资料浮层曾被排除在 P-01 后续轮次 | A1.5 覆盖创作者主页以外全部用户身份入口；桌面悬停/聚焦，移动点击直达主页 |
| 私信入口 | 资料浮层私信票据为 open，现有 `/messages` 才有完整会话 | A1.5 在当前上下文上方打开简化完整私信聊天浮层；真实附件能力另走 heavy |
| 私信冷启动 | 设计/代码仅“对方回复后解锁” | “接收方关注发送方或回复后解锁”；受限期仅一条纯文本，禁止图片 |
| 评论/私信图片 | 当前消息与评论模型仅文本 | P-01 mock 单图选择/预览/审核失败；生产迁移与 OSS/审核另走 heavy |
| 瀑布流 | 原型普通 Grid；生产 `react-masonry-css` 固定分列 | 推荐/原创/IP 详情按返回顺序逐条放入当前最短列 |

## 6. 旧基线截图证据索引(`screenshots/ui-polish/prototype/`)

**8 个主评审视图**:`feed-desktop-light/dark`、`feed-mobile-light/dark`、`detail-desktop-light/dark`、`detail-mobile-light/dark`

**状态证据**:`feed-desktop-light-loading/empty/error`、`detail-desktop-light-loading/empty/error`、`feed-desktop-dark-loading`、`detail-desktop-dark-empty`

**交互证据**:`feed-desktop-light-hover`(悬浮档)、`feed-desktop-light-focus`(focus ring)、`feed-desktop-light-usermenu`(浮层档)、`feed-mobile-light-drawer`(移动抽屉浮层)、`feed-desktop-light-reduced-motion`

**整页证据**:`detail-desktop-light-full`、`detail-mobile-light-full`

验证方式:`npm run dev`(端口 3001)+ MCP Playwright 交互验证(视图/状态/主题切换、键盘 ←/→、0 控制台错误)+ `node scripts/capture-p01-prototype.mjs` 确定性截图(1440×900 / 375×812,localStorage 固定主题)。

## 6.1 A1.5 共享交互修订实现记录(2026-07-28)

实现范围:仅 `frontend/app/prototype-ui-polish/` 与 `frontend/scripts/capture-p01-prototype.mjs`;方向 A/B 共用同一实现(B 在批次 B 前临时复用 A 的 token)。

| A1.5 决策 | 落实位置 |
|---|---|
| 主按钮蓝底白字 / 已关注中性描边（历史状态，A1.6 改为浅灰填充）/ 自己编辑资料 / 禁用中性置灰 | `social.tsx`(FollowButton)+ `prototype.css`(修复 `.p01-root button` 颜色继承压过主按钮的优先级缺陷) |
| 外壳+封面 300/240ms 同帧动画;无缩略图/离屏降级居中轻缩放+淡化 | `content-detail-overlay.tsx`(`data-motion="fallback"`)、`prototype-app.tsx`(FLIP 视口检查)、`prototype.css` |
| 唯一内部滚动模型(dialog `overflow:hidden`) | `prototype.css`(既有 html/body 锁定保留) |
| 作者栏精简(头像+昵称+关注) | `detail.tsx`(右侧创作者区；标题下方不重复展示) |
| 返回文案 = 来源或上一条内容标题 | `content-detail-overlay.tsx`(返回推荐流/返回对话/返回 {标题}) |
| 资料浮层全入口(200ms 悬停、聚焦立即、保持打开、统计导航、私信入口) | `social.tsx`(UserIdentity);feed 卡片作者区与卡片主点击区已分离 |
| 点击头像/昵称始终进入用户主页 | `prototype-app.tsx`(`view=user` 占位视图,含移动端私信入口) |
| 私信聊天浮层(440px/≤72dvh、移动全屏、冷启动一条纯文本、≤10MB 单图) | `chat-overlay.tsx` + `mock-data.ts`(已解锁/冷启动未发/冷启动已锁三种会话) |
| 评论/回复编辑器聚焦展开、失焦保留、≤5MB 单图 | `comments.tsx` + `overlay-utils.ts`(useImageDraft) |
| 审核可见性(乐观可见、静默、失败红色感叹号、原子不投递、预览层) | `comments.tsx`、`chat-overlay.tsx`、`image-preview.tsx`;mock 开关:文件名含 `uploadfail`/`modfail` |
| Agent 侧栏(60px 窄栏持久化、搜索分组、全宽新建、移动抽屉) | `agent.tsx` + `prototype.css` |
| 最短列瀑布流(保 DOM/读屏顺序,无动画重排) | `masonry.tsx` 已用于推荐流；原创分区/IP 详情沿用生产现有 `MasonryGrid`，不进入 A2 原型 |

**A1.5 新增截图证据**(`screenshots/ui-polish/prototype/`,方向 A/B 各一套):

- 资料浮层:`profile-card-desktop`(悬停卡+三项统计+已关注/私信)、`user-stub-desktop`(用户主页占位)
- 私信:`chat-overlay-desktop`(440px 停靠+图片消息+草稿)、`chat-cold-start-desktop`(冷启动锁定)、`chat-overlay-mobile`(移动全屏+返回)
- 评论图片:`comment-composer-image-desktop`(聚焦展开+草稿就绪)、`comment-moderation-failed-desktop`(审核失败红色感叹号)、`image-preview-desktop`(独立预览层)
- Agent:`agent-sidebar-collapsed-desktop`(60px 窄栏)、`agent-drawer-mobile`(移动抽屉)
- 瀑布流:`feed-masonry-desktop`(四列最短列填充)

**A1.5 自动验证（历史证据）**(`node scripts/capture-p01-prototype.mjs`,方向 A/B 均通过):既有矩阵/响应式/导航栈/滚动锁/IP 跳转/Agent 恢复/reduced-motion/FLIP 回归 + 新增 verifyMasonry(DOM 顺序=源顺序、4/2 列)、verifyProfileCard(悬停延迟/聚焦立即/保持打开/关注切换/自己/主页跳转)、verifyCommentImageFlow(初始隐藏/聚焦展开/失焦保留/禁用中性/失败重试/纯图发布/审核静默+失败/预览层只关最上层)、verifyChatOverlay(440px≤72dvh/上传前禁发/纯图发送/预览层/冷启动锁定/焦点恢复/移动全屏)、verifyAgentSidebar(标题过滤分组/Esc 两级/窄栏 56–64px/持久化/收起态搜索先展开/移动抽屉)。其中已关注描边、侧栏内标题过滤和收起态搜索先展开已被 A1.6 决策取代，不能作为新验收依据。

## 6.2 A1.6 细节交互修订实现记录（2026-07-28，已实现/验证）

| A1.6 决策 | 新验收口径 |
|---|---|
| 创作者栏 | 标题下方不重复显示昵称/关注；右侧创作者栏的头像、可省略昵称和固定约 80px×32px 关注按钮同一行，按钮不显示额外外框 |
| 创作者资料定位 | 右侧创作者栏优先下方居中、空间不足上翻并限制在详情可视区；评论发布者动态定位不变 |
| 评论/回复编辑器 | 一行起始、最多六行后内部滚动；统一 2000 字，聚焦显示计数，超长粘贴截断，失焦保留草稿 |
| 中性切换态 | 未关注“关注”仍蓝底白字；已关注/取消关注及收藏/订阅选中与取消态均浅灰背景+深色文字/图标，不使用红色 |
| 私信聊天浮层 | 桌面约 440px/70–75dvh 且默认视口居中，不跟随资料浮层定位；移动仍全屏 |
| Agent 侧栏 | 展开态折叠按钮→搜索触发框→全宽新对话→历史；收起态保留展开/搜索/新对话三图标和 Tooltip |
| Agent 会话全文搜索 | 搜索全部未删除会话标题+消息正文且不受侧栏加载限制；每条命中独立按时间倒序，显示来源/片段/日期，点击精确定位并短暂高亮；桌面 `min(720px,92vw)`/≤80dvh 居中，移动全屏，含快捷键、键盘导航、空态和焦点恢复；空态不重复提供清空按钮 |
| Agent 唯一滚动 | 工作台固定为 Header 下方剩余视口高度，外壳与主列不滚动，仅对话正文区域纵向滚动 |

P-01 仅以完整 mock 数据验证全文搜索；真实 owner-scoped API、分页和索引由 Web Agent Productization 后续任务实现。

**A1.6 实现与验证证据**：`detail.tsx` / `social.tsx` / `comments.tsx` / `agent.tsx` / `chat-overlay.tsx` 与 `prototype.css` 已落实上表契约；`node scripts/capture-p01-prototype.mjs` 在方向 A/B 均通过 320/375/414/768/1280/1920px、明暗色、移动端、reduced-motion、导航栈、资料浮层、评论图片、居中私信与 Agent 全文搜索断言。新增代表截图包括 `direction-{a,b}-agent-fulltext-search-desktop.png`，并刷新 detail/profile-card/comment-composer/chat-overlay 证据。独立浏览器复核确认 4 条“站台”正文命中按时间倒序、2000 字截断与六行内部滚动、失焦草稿保留、440px 私信窗口视口居中；`npm run lint`、`npm run build`、后端 `go test ./...` / `go vet ./...` / `go build ./...` 均通过。

## 6.3 A2 IP 库原型交接规格（2026-07-28）

**范围**

- 仅在 `frontend/app/prototype-ui-polish/` 增加当前 Indigo 的 IP 库视图并更新 `frontend/scripts/capture-p01-prototype.mjs`；不得修改生产 `IPBrowseClient.tsx`、`IPCard.tsx`、API、路由或 feature flag。
- A2 开始时删除误导性的方案 B 切换入口与 B 专属重复截图矩阵；历史截图保留，不生成新的 B 证据。
- 二创页 `/`、原创页 `/original`、IP 详情 `/ip/[ipId]` 使用现有生产 UI，不在隔离原型中复制；IP 库卡片只需能表达进入现有 IP 详情页的跳转目标。

**现状证据与问题**

- 页面：`http://localhost:3000/ips`；基线截图：`screenshots/ui-polish/prototype/ip-library-current-baseline-1280.png`。
- 生产 `IPCard` browse 变体固定 `w-[156px]`，`IPBrowseClient` 使用 `grid-cols-2 ... xl:grid-cols-5` 的弹性 Grid。在 1280px 视口，五张 156px 卡片的实际左边界约为 24、270.6、517.2、763.8、1010.4px，相邻卡片间空白约 90.6px。
- 问题不是内容数量不足，而是固定卡宽没有填满 `1fr` 轨道；A2 不得通过加入无意义装饰、扩大外边距或隐藏空白掩盖这一结构缺陷。

**必须保留的业务契约**

- 标题、浏览说明、总数；关键词搜索；分类筛选；热门/内容最多/最新/名称排序；loading/empty/error；加载更多；点击卡片进入 `/ip/[ipId]`。
- 使用代表性名称、封面、类别、内容数和趋势数据，保证评审基于真实信息密度。

**布局验收**

- 遵守 `design/ui-spec.md` 的 `Page: /ips` 与 `Component: IPCard`；规则 Grid 中卡片填满轨道，禁止固定 156px 卡宽与弹性轨道并存。
- 320/375px 两列、768px 自适应平板列、1280/1440px 自适应桌面列；小屏 gap 12px、桌面 gap 16px，任何实际卡间距不超过 24px。
- 无横向溢出；最后一行对齐既有轨道；封面维持 16:10；标题和元数据不造成卡片高度随机漂移。
- 覆盖 light/dark、键盘 focus、hover/active、loading/empty/error/reduced-motion，保存 375/768/1280/1440px 证据。

**P-01 结束条件**

- 新 Agent 完成上述 IP 库原型并通过 lint/build、自动化交互和浏览器截图后交由用户评审；用户明确批准后才勾选 P-01，并由 U-09 将原型落实到生产 `IPBrowseClient`/`IPCard`。

## 6.4 A2 IP 库原型实现记录（2026-07-29，已实现/已验证，等待用户评审）

**实现**

- 新增 `frontend/app/prototype-ui-polish/ip-library.tsx`：标题/说明/总数、关键词搜索、单行横滚分类筛选（`aria-pressed` + 边框/字重/底色共同表达当前项）、热门/内容最多/最新/名称排序、加载更多/加载中/已到底，以及 loading/empty/error 局部状态;卡片为指向现有生产 `/ip/[ipId]` 的锚点。
- `mock-data.ts` 新增 36 条真实信息密度 IP(7 个分类,12 条/页 × 3 页),含名称/封面/分类/内容数/趋势/入库日期。
- 规则 Grid:≤700px 两列 `gap 12px`;701–1100px `repeat(auto-fit, minmax(168px, 1fr))` `gap 16px`;>1100px `repeat(auto-fill, minmax(176px, 1fr))`;卡片 `w-full min-w-0`、16:10 封面、1px 边框无阴影。1280px 实测 6 轨道、卡宽 190px、相邻间距 16px,原约 90.6px 分散空白消除。
- 方案 B 评审入口(方向切换器)与 B 专属截图矩阵已移除;`?direction=b` 参数静默回落到当前 Indigo 方向,历史截图文件保留未删。

**验证(2026-07-29)**

- `npm run lint`、`npm run build`:PASS(63 个页面)。
- 后端 `go test ./...`、`go vet ./...`、`go build ./...`:PASS。
- `node scripts/capture-p01-prototype.mjs`:PASS;IP 库断言覆盖首屏密度(12 张/首行 6 列/间距 16px/右缘无残空)、16:10 封面、搜索过滤与总数同步、无结果空态(保留查询 + 单一清空入口)、分类筛选、内容最多/名称排序、加载更多 12→24→36 与已到底、骨架网格与最终轨道一致、错误重试回 default、键盘 Tab 焦点环与完整可访问名称、卡片点击进入生产 `/ip/[ipId]`;320/375/414/768/1280/1920 无横向溢出。
- 证据:`screenshots/ui-polish/prototype/direction-a-library-{375,768,1280,1440}-light.png`、`direction-a-library-1280-dark.png`、`direction-a-library-{loading,empty,error}-1280.png`、`direction-a-library-search-empty-desktop.png`、`direction-a-library-loadmore-end-desktop.png`。

## 7. 评审结论(用户填写 / Agent 记录)

**2026-08-01 P-01 批准结论（最新，取代此前“待评审”状态）**:

1. 用户认可当前原型整体效果；IP 库在 2026-07-29 的 A/B/C 三变体对比中选择**方案 B（Indigo 精修）**作为最终视觉方向。方案 B 定义于 `frontend/app/prototype-ui-polish/ip-library-b.css` 与 `ip-library-variants.tsx`：沿用 Indigo 设计 token 精修信息层级、筛选激活态、卡片元数据、暗色表面与克制 hover；生产 `/ips` 仍在固定 156px 卡宽 + 弹性轨道缺陷上，交由 U-09 回写修正。
2. P-01 视觉门正式通过：勾选计划 Gate P-01，解除 U-01 阻塞。U-09 将方案 B 的页面/网格/卡片契约回写 `design/ui-spec.md` 并落实到生产 `IPBrowseClient`/`IPCard`。
3. 生产 `/`、`/original`、`/ip/[ipId]` 继续作为已接受视觉基线；prototype 保持 throwaway，不做生产回写。
4. 前端剩余任务由 opencode 车道执行（U-01→U-02B→U-03→U-06~U-10→U-12 及 U-11 前端部分）；后端任务由 codex 执行（U-11 后端能力 + Production Readiness Ops-01~08）。

**2026-07-28 A2 范围收窄结论（历史；已被 2026-08-01 批准结论取代）**:

1. 生产二创页 `/`、原创页 `/original` 和 IP 详情 `/ip/[ipId]` 已有可接受 UI，A2 不再为它们构建隔离原型。
2. IP 库 `/ips` 的现有设计存在明显分散空白，A2 只构建新的 IP 库原型并保留现有业务行为。
3. IP 库原型获批即满足剩余 P-01 视觉门；生产实现由后续 U-09 承接，不能在 throwaway 原型任务中直接修改生产页面。

**2026-07-28 当前范围结论（历史；A2 表面清单已被上方最新结论收窄）**:

1. 用户对当前 Indigo 原型较满意，当前方向成为唯一后续视觉基线。
2. 取消 Hallmark Tally 方案 B；不再实现 B 换肤、不再生成 B 截图矩阵，也不再要求双方向对比后才能批准 P-01。
3. 当前满意度不等于 P-01 已完成；按最新结论，唯一缺失表面是新的 IP 库原型。
4. 完成 IP 库响应式、暗色和浏览器截图验证后，再请求用户最终批准。

**2026-07-27 用户评审原型2 结论（历史；其中方案 B 路线已被 2026-07-28 范围结论取代）**:

1. 以原型1(07-25 基线)为视觉与详情完整性基准,保留原型2 的四项新功能代码与共享契约,方向 A 向原型1 对齐(恢复完整评论区、相关推荐、附件、IP 区块)。
2. 内容详情浮层扩展为所有内容卡片入口(推荐/分区页/IP 详情页/Agent 引用)统一使用;仅内容详情使用浮层。
3. 浮层承载与完整详情页一致的完整内容(评论区、相关推荐、关联原创/二创、所属 IP),内设逐层返回导航栈:返回类手势(返回按钮/Esc/浏览器后退)逐层弹出,退出类手势(关闭按钮/背板点击)退出整个浮层。
4. 浮层动效:封面级 FLIP(开 300ms/关 240ms),栈内方向感水平滑动 240ms,统一 `cubic-bezier(0.22,0.61,0.36,1)`,reduced-motion 降级为 100ms 纯透明度淡化。
5. 修复双重滚动条:浮层打开时锁定 html+body,每表面单一滚动上下文;Agent 工作台桌面端固定视口列内滚动。
6. 方向 B(Newsprint)否决并删除;新方向 B = Hallmark Tally(SaaS · modern-minimal)全语音 + hue-preserving 暗色适配,功能与方向 A 完全一致。
7. 首轮范围扩展为八表面:推荐流、二创分区页、原创分区页、IP 库(重设计)、IP 详情页、Agent 工作台、内容详情浮层、完整详情页(仅直达);导航四项不变,IP 库由二创分区页进入。
8. 实施批次原定 A1 修复合并 → A2 新表面 → B Tally 换肤；2026-07-28 先插入 A1.5 共享交互修订，再由本轮确认插入 A1.6 细节交互修订，逐批浏览器验证+截图+用户评审。

**2026-07-28 A1.5 共享交互修订结论（31 个访谈决策归并）**:

1. **主动作与关注状态**：发布、发送、未关注时的“关注”等 primary 按钮统一蓝底白字；已关注改为中性描边“已关注”，悬停/聚焦显示“取消关注”。查看自己时以“编辑资料”次要按钮替代关注/私信。
2. **内容详情与浮层**：直达详情页与浮层共用精简创作者身份栏，只显示头像、昵称、关注；弹窗外壳与封面共享同一动画进度（开 300ms、关 240ms、同帧同缓动），Agent 引用入口复用，缺缩略图时降级居中轻缩放+淡化。返回文案只显示来源或上一条内容标题，不显示层数/解释文本。
3. **唯一滚动模型**：锁定背景 `html/body`；`dialog` 与浮层外壳 `overflow: hidden`，只有内部内容主体滚动；顶部返回/关闭栏位于滚动主体外。每层只记忆这一滚动容器；不得以隐藏滚动条掩盖双滚动上下文。
4. **用户资料浮层**：创作者主页以外的所有头像/昵称入口统一接入；桌面悬停显示与关闭均延迟 200ms，键盘聚焦立即显示，移动端点击直接进入主页。资料含头像、昵称、可选简介、内容发布、累计获赞、关注者和关注/私信；简介未填即省略。三项统计使用当前公开未删除内容口径并导航到主页相应视图，不展示点赞用户名单。
5. **私信聊天浮层**：点击资料浮层“私信”后在当前页面/内容浮层上方打开简化完整聊天，资料浮层关闭；桌面约 440px、最高 70–75dvh，移动全屏，昵称头与编辑器固定，中间历史单独滚动。关闭只退当前最上层并恢复下层位置/焦点。文本或单图任一非空即可发送；单图限 JPEG/PNG/GIF/WebP、≤10MB；冷启动期仅允许一条纯文本，接收方关注或回复后解锁图片与后续消息。
6. **评论编辑与图片**：评论框聚焦后才显示“友善交流，尊重创作”、图片按钮和发布按钮；离开整个编辑区即隐藏，但文字与已选图片不丢失，不使用边框加粗/缩放表达聚焦。顶层评论和回复均支持文字、单图或组合；单图限 JPEG/PNG/GIF/WebP、≤5MB。
7. **图片上传与审核**：选图后立即校验并临时上传，成功前不可发送；预览可移除/重试，失败保留草稿且不暴露原始 OSS 错误。发送者提交后立即本地看到，接收方/公众审核通过前不可见；审核中对发送者完全静默，失败仅在图片侧显示红色感叹号。文字与图片是原子消息/评论，图片失败则整条不投递。图片点击进入独立预览层，最上层优先关闭。
8. **Agent 工作台**：桌面会话侧栏可收为 56–64px 窄栏并持久化，保留展开与新建图标；移动端折叠即关闭抽屉。顶部第一行搜索+折叠，第二行全宽“开启新对话”，删除主区域重复入口。搜索按最近对话标题即时包含匹配，保留非空时间分组；收起时搜索先展开，Esc 先清词再退出搜索。
9. **最短列瀑布流**：推荐、原创分区和 IP 详情的可变高卡片按后端顺序逐条放入当前最短列；DOM/键盘/读屏顺序不变，窗口和图片高度变化时无显著动画地重新平衡。固定比例二创卡与 IP 库卡继续规则网格。
10. **实施边界**：A1.5 只用静态 mock 覆盖图片选择、上传、审核与失败状态；生产图片私信/评论需要 heavy 车道 TDD、迁移、用途隔离上传授权、OSS 清理和真实内容安全验证。P-01 批准不代表生产能力已实现。

**2026-07-28 A1.6 细节交互修订结论**：以 §6.2 为最新权威，取代上述 A1.5 中“已关注中性描边”“侧栏内按已加载标题过滤”“收起态搜索先展开”等旧口径；真实 Agent 全历史搜索同样不由 P-01 原型实现。

> 待 A2 当前 Indigo IP 库原型完成并经浏览器验证后，由用户给出：批准 / 需迭代 / 否决。
> 批准后:勾选计划 P-01,按 §3 回写 U-01(design-system.md + globals.css),再启动 U-02B、U-03。
