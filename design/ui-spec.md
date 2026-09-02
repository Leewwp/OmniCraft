# OmniCraft UI 设计规格

> ℹ️ **生成状态**：本文件由 Gemini 批量生成，已完成初始内容填充。全局规范（Design Tokens、Interaction Patterns）和组件规范（Component 章节）可直接用于实现。以下偏差需 Agent 实现时注意：
>
> 1. **部分页面「核心组件清单」仍为模板默认值**（`EmptyState/LoadingSpinner`），Agent 应参考对应 Task 的 `ui_spec_ref` 和 `design/design-system.md` 组件规范获取真实组件列表。
> 2. **表单类页面的「响应式规则」已修正**（login/register/settings/judge exam/rehab/messages/appeals 等 15 个页面），但个别页面可能仍有遗留。
>
> Agent 在实现前端时，优先以 `## Component:` 具体组件的规范和 `design/design-system.md` 为准；发现不一致时以设计规范为准。（注：历史 `UI Design.md` 已归档至 `docs/archive/`，不再作为视觉权威。）

---

<!-- Gemini 生成内容从此行开始 -->

## Section Index（索引）

> 本索引由 2026-07-23 文档瘦身生成，后续由 doc-validator（UI 治理门 U-12）接管自动校验，请勿手改。
> 使用方式：`grep -n "## Component: TagBadge" design/ui-spec.md` 定位后按节阅读。

### Global（2）
- Global Design Tokens
- Global Interaction Patterns

### Pages（49）
- Page: / 首页
- Page: /recommend 推荐流
- Page: /search 搜索页
- Page: /ips IP 库
- Page: /login 登录页
- Page: /register 注册页
- Page: /ip/[ipId] IP 详情页（含类目就地切换，U-01 并入原独立类目页）
- Page: /ip/[ipId]/discussions 讨论区列表
- Page: /ip/[ipId]/discussions/[discussionId] 讨论详情
- Page: /ip/[ipId]/discussions/new 发帖页
- Page: /content/[contentId] 内容详情页
- Page: /original 原创区首页
- Page: /original/[contentId] 原创内容详情
- Page: /user/[userId] 用户主页
- Page: /settings 账号设置
- Page: /settings/tag-groups 标签组管理
- Page: /dashboard/contents 我的内容
- Page: /dashboard/pr-requests PR 申请管理
- Page: /dashboard/contributors 贡献者管理
- Page: /dashboard/tag-suggestions 标签建议审核
- Page: /judge/exam 赛博判官资质考核
- Page: /judge/queue 待审内容队列
- Page: /history 浏览历史
- Page: /appeals 我的申诉
- Page: /messages 消息中心
- Page: /agent Agent 工作台
- Page: /rehab 素质建设课程
- Page: /admin/ips IP 库管理
- Page: /admin/contents 内容终审
- Page: /admin/users 用户管理
- Page: /admin/appeal 申诉处理
- Page: /admin/config 系统配置
- Page: /admin/notifications 管理员系统通知广播
- Page: /admin/categories 分类与标签管理
- Page: /admin/agent-config Agent 管理
- Page: /studio 创作者工作室
- Page: /studio/publish/original 发布原创
- Page: /studio/publish/fanwork 发布二创
- Page: /studio/publish/ip 创建 IP
- Page: /studio/overview 数据概览
- Page: /studio/series 内容系列管理
- Page: /studio/favorites 收藏集管理
- Page: /studio/followers 粉丝分析
- Page: /studio/revenue 收益数据
- Page: /forgot-password 忘记密码（Task 117）
- Page: /reset-password 重置密码（Task 117）
- Page: /series/[id] 内容系列详情
- Page: /collections/[id] 收藏集详情（Task 122-124）
- Page: /user/[userId]/collections 用户收藏集列表（Task 122-123）

### Components（73）
- Component: Button 与 Badge 共享动作原语
- Component: Card 共享容器原语
- Component: Form Controls 表单原语
- Component: DropdownMenu 浮层菜单原语
- Component: Tabs 与 Separator 导航原语
- Component: Loading 与反馈原语
- Component: Header
- Component: FacetedSearchSidebar
- Component: ContentCard
- Component: MasonryGrid
- Component: TagBadge
- Component: IPCard
- Component: IPCategoryTabs
- Component: FilterPills 筛选药丸（U-01 新增，全站筛选形态基准）
- Component: ContentDetail
- Component: ContentDetailOverlay
- Component: MediaGallery 媒体集画廊
- Component: MediaViewer 全屏媒体查看器
- Component: SeriesNav 内容系列导航
- Component: SourceAttribution 灵感来源归因
- Component: RelatedFanworks 相关二创/衍生作品行
- Component: RelatedContents 相关内容块
- Component: MarkdownRenderer
- Component: SheetMusicViewer
- Component: PRCard
- Component: DiffViewer
- Component: ReactionBar
- Component: CommentSection
- Component: VersionHistory
- Component: FileUploader
- Component: ExamQuestion
- Component: ReviewCard
- Component: EmptyState
- Component: ConfirmModal
- Component: UploadAssistPanel
- Component: ComplianceCheckBadge
- Component: UsageGuidePanel
- Component: GlobalSearchInput
- Component: FollowButton
- Component: NotificationDropdown
- Component: NotificationList
- Component: ConversationList
- Component: ChatWindow
- Component: CollabInviteCard 联合创作邀请卡片
- Component: CourseCard
- Component: CourseContent
- Component: ReputationDetail
- Component: DiscussionCard
- Component: ReplyList
- Component: VerdictDetail
- Component: LLMConfigTable
- Component: LLMConfigModal
- Component: ActiveConfigCard
- Component: UserProfileCard
- Component: FollowerListModal
- Component: CreatorSupportPanel
- Component: JudgeQualBadge
- Component: StudioSidebar
- Component: ContentTypeGrid
- Component: SourceContentPicker 来源内容选择器
- Component: CollabUserPicker 联合创作者选择器
- Component: CollectionInfoCard 收藏集信息摘要
- Component: ContentTypeFilter 内容类型筛选
- Component: CollectionCard 收藏集卡片（Task 122-123）
- Component: CollectionPicker 收藏集选择器
- Component: DownloadButton 下载按钮（Task 121）
- Component: LoadingSpinner 加载旋转器
- Component: SkeletonCard 骨架屏卡片
- Component: Toast 消息提示
- Component: Footer 页脚
- Component: StatsCard 统计卡片
- Component: NotificationBell 通知铃铛
- Component: SortSelect 排序选择器

---

## Global Design Tokens

> **来源**: 以下所有 token 值以 `design/design-system.md` 为准，是本文件的唯一设计权威。

- **颜色 token**：使用 `design/design-system.md` 定义的 CSS 自定义属性，以 `--xxx` 格式引用（`--` 前缀的 CSS 自定义属性），使用时通过 `var()` 读取值：`--background`、`--foreground`、`--primary`、`--border` 等基础色，以及 `--canvas-default`、`--canvas-subtle`、`--border-default`、`--fg-muted`、`--accent-emphasis`、`--accent-subtle` 等自定义 token。标签颜色使用预设的 6 色体系 (blue/green/purple/orange/rose/sky)。所有颜色支持 light/dark 双模式，暗色模式通过根级 `.dark` 类自动切换。
- **字体**：font-family: `--font-sans: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif`，包含中文字体回退。等宽字体 `--font-mono: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace`。正文与标题只使用 12 / 14 / 16 / 20 / 24px 五档；紧凑指标数字例外须在组件章节登记。
- **间距阶梯**：使用 4px 基线；卡片 12px、区块 16px、详情卡 20/24px、页面 gutter 16/24px、网格 gap 16px、区块间距 24/32px。
- **圆角**：rounded-sm (3px) 小元素（checkbox）/ rounded-md (8px) 按钮/输入框（操作控件，与卡片同档，SP-12 U-01 起自 4px 提升）/ rounded-lg (8px) 卡片/容器默认 / rounded-xl (12px) 大卡片 / rounded-full (9999px) 筛选选择与信息标签（药丸）。核心原则：矩形=操作控件、药丸=选择/信息（形状语义见 design-system.md）。
- **控件高度**：三档体系——紧凑 28px (`h-7`) / 常规 36px (`h-9`) / 表单与主 CTA 44-48px (`min-h-11`~`h-12`)；硬规则**同排控件同高**（SP-12 U-01 起生效，权威见 design-system.md「高度体系」）。
- **动效**：transition duration-150 ease-out（默认）；duration-300 ease-in-out（Modal/Sheet）；`active:scale` 按压缩放仅限页面级主 CTA，hover 加深一档。
- **层级**：静态卡片/面板使用 elevation 1，hover 使用 elevation 2，Dropdown/Drawer/Modal 使用 elevation 3；阴影必须配合 1px border，不得造成布局位移；`prefers-reduced-motion: reduce` 下禁用缩放、位移和脉冲。

## Global Interaction Patterns

- **Hover**: `cursor-pointer` + 颜色加深一档（border / bg）。
- **Loading**: 骨架屏（Skeleton 灰色块）+ 按钮内联 Spinner，禁止全屏遮罩。
- **Error**: Toast（右上角，自动消失 4s）+ 内联红字提示（表单字段下方）。
- **Empty**: EmptyState 组件（图标 + 标题 + 说明 + 可选 CTA），不留空白区域。
- **Disabled**: `opacity-50` + `cursor-not-allowed` + 禁用 hover/click。

## Component: Button 与 Badge 共享动作原语

**视觉契约**
- Button 使用 `rounded-lg` (8px，SP-12 U-01 起与卡片同档)、14px medium 字体和 1px 透明边框以稳定状态切换；default/outline/secondary/ghost/destructive/link 只消费既有语义 token，不引入任意色。
- Primary hover 使用 `--accent-hover`（dark 为 #4338CA，白字 7.90:1 ≥AA）；outline hover 使用 `--border-strong` + `--canvas-subtle`；destructive 使用 `--destructive`、`--border-destructive` 与既有白色前景 `--primary-foreground`，不得用未登记的红色常量。
- 高度三档（SP-12 U-01 起）：紧凑 28px (`size="sm"`) / 常规 36px (`size` 缺省) / 表单与主 CTA 44px (`size="lg"`，`min-h-11`；48px 仅限页面 hero 主 CTA)；**同排控件同高**为硬规则，Button 的 size 档必须与所在行的输入框/下拉同档。icon-only 在精细指针下按同档尺寸，在 coarse pointer 下保持 44px 目标。focus-visible 统一为 2px `--ring` + 2px background offset；disabled 保持 `opacity-50`、禁止交互且不触发 hover/active 位移。
- Badge 始终 `rounded-full`、12px medium、20px 高，无 elevation；可交互 Badge 仅做 150ms 颜色/边框过渡，focus-visible 与 Button 相同。

**响应式与动效**
- 移动端不缩小文字；独立动作或 icon-only 动作保持 44px 触控目标，密集工具栏可沿用 U-02A 已批准的 coarse-pointer 媒体规则。
- active 反馈不得通过会导致布局抖动的 margin/border-width 实现；reduced-motion 下禁用位移与缩放。

## Component: Card 共享容器原语

**视觉契约**
- 默认卡片为 `bg-card` + 1px `border-border` + `rounded-lg` (8px) + elevation 1；内部 padding 使用 12px (sm) 或 16px (default)。
- 只有显式声明为可交互的卡片才在 hover/focus-within 时切换 `border-strong` 与 elevation 2；变化只使用 border-color/shadow/transform，不改变布局尺寸。
- 标题使用 14px medium，说明使用 14px muted；Footer 使用 `canvas-subtle` 与 1px 顶部分隔。
- 原创内容卡的无边框特例由对应业务组件声明，不能通过改变共享 Card 默认值实现。

## Component: Form Controls 表单原语

**覆盖文件**: `checkbox.tsx`、`field.tsx`、`input.tsx`、`label.tsx`、`select.tsx`、`switch.tsx`、`textarea.tsx`

**视觉契约**
- Input/Select/Textarea 使用 `rounded-lg` (8px，SP-12 U-01 起与卡片同档)、1px `border-input`、`bg-background`、14px 正文；移动端输入文字保持 16px 以避免浏览器自动缩放，`md` 起恢复 14px。
- 高度对齐控件三档：常规 36px；表单内取 44-48px 并与同排提交按钮同高（同排同高硬规则）。
- hover（非 disabled）提升到 `border-strong`；focus-visible 使用 2px `--ring` + 2px background offset；invalid 使用 `border-destructive` + destructive ring，不以 placeholder 或颜色单独表达错误。
- Label 为 14px medium；Field 间距使用 8px，hint/error 为 12px，error 保留 `role=alert`。
- Checkbox 使用 16px 方形、`rounded-sm` (3px)、checked=`primary`；Switch 为 44×24px 药丸轨道，checked=`primary`、unchecked=`muted`，thumb 使用 elevation 1。两者沿用同一 focus/disabled 契约。

**响应式与状态**
- 表单控件宽度默认跟随容器，禁止产生横向滚动；disabled 使用柔和背景、`opacity-50` 与禁止光标。
- placeholder 仅使用 muted foreground；不得把 placeholder 当作 label。

## Component: DropdownMenu 浮层菜单原语

**视觉契约**
- Popup/Submenu 使用 `bg-popover` + 1px `border-border` + `rounded-lg` (8px) + elevation 3，不再用 ring 模拟边框或使用未登记的 shadow 档位。
- Item 使用 `rounded-md`（--radius-md=8px，SP-12 U-01 起随操作控件档位）、14px、最小 32px 高；hover/focus 使用中性 `accent`，checked/selected 可使用 `accent-subtle` + `accent-emphasis`，destructive 只使用 destructive token。
- Label/shortcut 使用 12px muted；separator 为 1px `border`。菜单宽度不得超过可用视口，长内容省略或纵向滚动。

**动效与响应式**
- 开合使用 150ms opacity/scale；reduced-motion 下只保留即时显隐。触控设备菜单项最小 44px 高，桌面保持密集 32px。

## Component: Tabs 与 Separator 导航原语

**视觉契约**
- Default TabsList 使用 `canvas-subtle`、`rounded-lg` (8px) 和 4px 内边距；active trigger 使用 `bg-background`、1px border 与 elevation 1。
- Line Tabs 不使用卡片阴影，active 仅以 2px `primary` 下划线 + 字重/文字色共同表达；focus-visible 使用统一 2px ring。
- Trigger 为 14px medium、最小 32px 高；disabled 使用 `opacity-50`。移动端允许 TabsList 水平滚动但页面本身不得横向溢出。
- Separator 仅为 1px `border` 色，无圆角、阴影或交互状态。

## Component: Loading 与反馈原语

**覆盖文件**: `skeleton.tsx`、`empty-state.tsx`、`Toast.tsx`

**视觉契约**
- Skeleton 形状镜像最终内容，使用 `canvas-subtle`、对应最终表面的圆角与 1.6s 低对比 pulse；reduced-motion 下静止。SkeletonCard/Detail 使用 `rounded-lg`、1px border 与 elevation 1，不得出现全屏 loading overlay。
- EmptyState 为无边框、无阴影的垂直居中结构：56px `accent-subtle` 圆形图标底、16px 标题、14px muted 说明（`max-w-sm`）、可选 Button CTA；移动 `py-20`，桌面 `py-24`。
- Toast 固定右上（移动端左右各 16px），使用 `bg-card` + 1px status border + `rounded-lg` + elevation 3。success/error/warning/info 分别复用已批准的 green/rose/orange/blue 标签前景与柔和背景 token；正文仍为 foreground，颜色不是唯一状态线索，必须保留对应 Lucide 图标与 live-region 语义。
- Toast 入退场为 200ms opacity/translate；reduced-motion 下禁用位移。关闭动作沿用 U-02A 的可访问名称与 coarse-pointer 44px 目标。

## Component: Header

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 固定高度 `h-[var(--header-h)]` (52px)，sticky 顶部。
- 底边框 `border-b border-border-default`，1px 分隔。
- 背景 `bg-canvas-default`，全宽布局。
- **品牌入口（page-shell 契约）**：桌面与移动 Logo 一律跳转 `/recommend`（推荐流是唯一品牌落点，二创区 `/` 不承担品牌入口语义）。Header 内层宽度与页面主容器共享同一 page-shell 宽度/gutter 契约（见「Page Shell 宽度契约」），不得出现独立 max-width 造成品牌与内容区横向漂移。

**Page Shell 宽度契约（#64 决策 2 权威，Header/侧边栏/页面主容器共同遵守）**
- 页面主容器最大宽度 1280px（`/ips` 为 1440px 的已批准例外，P-01 方案 B），Header 内层与其对齐，桌面 gutter 24px、移动 gutter 16px。
- 公开侧边栏（桌面 228px）与页面主体使用同一背景契约：页面表面 `bg-canvas-default`，不引入与主体冲突的独立底色。
- 实现必须移除造成可见漂移的独立 max-width 值（先例：2026-08-07 审计问题 1/2 的 `max-w-[1440px]` 与 `max-w-7xl` 混用）。

**Props 接口**
```ts
interface HeaderProps {
  className?: string;
  currentUser?: {
    id: number;
    username: string;
    avatarUrl?: string;
    role: string;
  } | null;
  unreadNotificationCount?: number;
  agentEnabled?: boolean;
}
```

**视觉结构**
- 外层容器: `<header className="sticky top-0 z-40 h-[var(--header-h)] bg-canvas-default border-b border-border-default">`
- 内部布局: 与页面主容器共享 page-shell 宽度契约；`mx-auto h-full flex items-center justify-between` + 桌面 gutter 24px / 移动 gutter 16px
- 左侧: Logo（跳转 `/recommend`）+ 主导航链接（推荐 / 二创 / 原创 / AI Agent），水平排列 `gap-6`
- 中间: 全站关键词搜索栏（GlobalSearchInput），最大宽度 480px；不得提供 Agent 模式切换
- 右侧: 发布按钮（primary 色）+ 通知铃铛（NotificationBell）+ 用户菜单（Avatar 下拉）
  - 未登录: 登录/注册按钮
  - 已登录: Avatar + 用户名下拉（设置/我的内容/退出）

**尺寸规范**
- Header 高度: `var(--header-h)` = 52px
- Logo: `h-8` (32px)
- 导航链接: `text-sm` (14px)
- 搜索框: `max-w-[480px]`
- 内边距: `px-4` (16px)

**状态变体**
- default: 正常展示 Logo + 导航 + 搜索 + 用户区。
- sticky: 滚动时固定顶部，z-40 确保在其他内容上方。
- 登录态: 右侧显示用户菜单。
- 未登录态: 右侧显示登录/注册按钮。
- Header 本体保持 shadow-none；其下拉菜单按浮层规则使用 elevation 3。

**响应式行为**
- 移动 (≤700px): 导航链接折叠为汉堡菜单，搜索框缩小或隐藏。
- 平板 (≤1100px): 导航链接文字缩小，搜索框自适应。
- PC (>1100px): 默认布局。

**暗色模式适配**
- 全局切换暗色类后 token 自动映射。

**关键交互**
- Logo 点击: 桌面与移动均路由跳转 `/recommend`（不跳 `/`），导航选中态同步。
- 导航链接点击: 路由跳转，选中项高亮。
- AI Agent 跳转受保护路由 `/agent`；仓库默认功能关闭时可由 feature gate 暂时隐藏该导航项，启用后未登录用户交给受保护路由守卫跳转登录。
- 搜索框聚焦: 展开建议下拉。
- 发布按钮: 跳转 `/studio/publish/original`（已登录）或 `/login`（未登录）。
- 用户菜单: 点击 Avatar 展开下拉，点击外部关闭。

## Component: FacetedSearchSidebar

**Key Constraints**
- 仅提供普通关键词、标签和分类分面筛选；不得在侧边栏嵌入 Agent 模式或自然语言问答入口。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface FacetedSearchSidebarProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Page: / 首页

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。
- 二创区主页；品牌入口语义归 `Header`（跳 `/recommend`），本页不被视为品牌落点。
- **筛选选中态（#64 决策 4 / 审计问题 12 权威）**：分类 Tab/筛选选中态与 IP 库、原创区一致——彩色药丸 `rounded-full border border-accent-emphasis bg-accent-subtle text-accent-emphasis font-semibold` + `aria-pressed="true"`（不能只靠颜色表达）；未选中 `text-fg-muted hover:bg-canvas-subtle`。二创区固定 `sort=hot`，无同类 recommended 降级问题。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `Header`
- `ContentCard`
- `MasonryGrid`
- `ContentDetailOverlay`（共享详情浮层，source=`zone-page`）
- `Footer`

**布局规范**
- 页面最大宽度：1280px / 满宽
- 主内容区与侧边栏比例：无侧边栏（全宽）或 3:1/4:1
- 区域间距（block）：32px (`space-y-8`)
- 元素间距（inline）：16px (`gap-4`)

**状态变体**
- default: 默认数据展示或列表。
- loading: 全屏加载骨架屏（Skeleton），不使用全屏遮罩 loading。
- empty: 使用 EmptyState 组件（图标 + 标题 + 说明 + CTA）。
- error: Toast 右上角报错或内联提示。
- 特殊状态：信誉分不足、权限不足或未登录拦截。

**响应式规则**
- 移动 (≤700px): 单列瀑布流 2 列，隐藏侧边栏，折叠菜单。
- 平板 (≤1100px): 瀑布流 3 列，卡片尺寸自适应。
- PC (>1100px): 默认布局 4 列瀑布流，左右分布边距对齐。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 卡片点击：打开共享 `ContentDetailOverlay`（source=`zone-page`），不跳转详情页。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /recommend 推荐流

**Key Constraints**
- 独立顶级发现面（区别于二创 `/`、原创 `/original` 与 Agent 工作台 `/agent`）；单一"为你推荐"内容流，跨原创与二创展示，不提供分区标签、分类 Tab 或排序筛选。
- 推荐流是算法驱动的发现界面：排序综合兴趣相关性、热度质量与新内容探索价值；所有推荐参数从 `config.yaml > recommendation` 读取（见 `docs/specs/recommendation-page.md`），前端不硬编码权重。
- 全站内容卡片统一入口：卡片点击打开共享 `ContentDetailOverlay`（source=`recommendation`），不跳转详情页；直接访问内容 URL 才使用完整详情页。
- 卡片主点击区与作者身份入口分离：点击卡片打开详情浮层；悬停 200ms / 聚焦立即显示用户资料浮层，点击进入用户主页。
- 数据源：复用 `GET /api/v1/contents`（ListContents）推荐排序（`sort=recommended`，跨区参数由后端扩展，见三缺口 Ticket 03）；未登录匿名可浏览（纯热门推荐）。

**视觉层级**
- Header 下方无 Tab 栏；页面背景 `bg-canvas-subtle`，主容器居中最大宽度 1280px。
- 内容区：最短列瀑布流（`MasonryGrid`），卡片直接排列，无外层 border 容器。
- 卡片双式（P-01 §3.7 决策，按内容 zone 切换）：二创卡 = 1px border + `radius-lg` + `--elevation-1`，hover → `--border-strong` + `--elevation-2` + 封面 scale(1.03)，信息层 标题 14 → 基于 IP 12 → 标签(≤2) → 作者+互动 12，类型角标为黑 55% 半透明 chip；原创卡 = 无边框，封面自适应比例 + `radius-lg`，hover → 封面 scale(1.05) + 浅遮罩 + `--elevation-2`，信息仅 标题 14 / @作者 12 / 点赞 12。

**核心组件清单**
- `Header`
- `ContentCard`（原创卡/二创卡两式）
- `MasonryGrid`（最短列瀑布流）
- `ContentDetailOverlay`（共享详情浮层，source=`recommendation`）
- `SkeletonCard` / `EmptyState` / `Footer`

**状态变体**
- default：推荐流瀑布流卡片。
- loading：骨架屏（封面比 + 标题行 + meta 行，镜像真实布局），首次加载 12 个占位；`prefers-reduced-motion` 下静止；禁止全屏遮罩。
- empty：56px 圆形柔底图标 + 16px 标题 + 14px 说明（max-w-sm）+ CTA"浏览原创区"；垂直 py-20/24 居中。
- error：同空状态结构，图标 AlertCircle，保留当前浏览位置，"重新加载" outline 按钮；不使用 Toast 承担页面级错误。
- 已到底：显示"已经到底了"提示，不再触发请求。

**无限滚动行为**
- 初始加载：SSR 首屏 24 条。
- 客户端滚动：IntersectionObserver 监听底部 sentinel；`page=2,3...` 追加到现有列表（SWR useSWRInfinite）。
- 从浮层返回时恢复列表滚动位置与已加载数据。

**响应式规则**
- 移动 (≤700px)：2 列瀑布流，页面 gutter 16px，卡片间距 `gap-4`。
- 平板 (≤1100px)：3 列瀑布流。
- PC (>1100px)：4 列瀑布流，左对齐，页面 gutter 24px。

**暗色模式适配**
- 背景 `canvas-default` / 边框 `border-default` / 文字 `foreground` 随 `.dark` 自动映射；图片 `opacity-90`，占位 SVG 反色。

**交互细节**
- 卡片 hover：transform/shadow 反馈，不产生布局位移；`prefers-reduced-motion` 禁用缩放与脉冲。
- 卡片点击：打开共享 `ContentDetailOverlay`；关闭后恢复推荐流滚动位置与触发卡片焦点。
- 作者身份入口：悬停 200ms / 聚焦立即显示用户资料浮层，点击进入 `/user/[userId]`。
- 数据加载策略：SSR 首屏 + 客户端 SWR 无限滚动。

**i18n key namespace**
- `recommend.*`（标题、空态、错误、加载更多、已到底）。

**Playwright 截图检查点**
- `screenshots/recommend-desktop.png`
- `screenshots/recommend-mobile.png`
- `screenshots/recommend-loading.png`
- `screenshots/recommend-empty.png`
- `screenshots/recommend-error.png`

## Page: /search 搜索页

**Key Constraints**
- 必须使用 `GlobalSearchInput` 提供关键词搜索、建议、历史和热搜；自然语言问答通过顶部导航进入 `/agent`。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：搜索栏 + 分面侧边栏 + 搜索结果瀑布流

**核心组件清单**
- `Header`
- `GlobalSearchInput`
- `FacetedSearchSidebar`
- `ContentCard`
- `MasonryGrid`
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：1280px
- 左侧分面侧边栏 + 右侧搜索结果
- 区域间距（block）：24px (`space-y-6`)
- 元素间距（inline）：16px (`gap-4`)

**状态变体**
- default: 搜索结果卡片网格（默认排序）+ 分面筛选。
- loading: 搜索结果区骨架屏（Skeleton 灰色块网格）。
- empty: "未找到匹配结果" EmptyState + 搜索建议。
- error: 搜索失败 Toast + 重试按钮。
- 特殊状态：未登录与 feature flag 关闭均不改变关键词搜索能力；搜索页不显示不可用的 Agent 模式。

**响应式规则**
- 移动 (≤700px): 分面侧边栏折叠为 Sheet 抽屉（85vw 左侧滑入），搜索结果瀑布流 2 列。
- 平板 (≤1100px): 左侧侧边栏 228px + 右侧瀑布流 3 列。
- PC (>1100px): 左侧侧边栏 260px + 右侧瀑布流 4 列。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /ips IP 库

**批准来源**: P-01 方案 B（Indigo 精修），`docs/working/2026-07-25-ui-prototype-review.md` §3 / §6.3 / §7。

**页面与工具栏**
- 主容器最大宽度 1440px；移动 gutter 16px，桌面 gutter 24px。标题使用 24px semibold，说明 14px，结果总数使用 12px Indigo 柔和药丸。
- 搜索与排序保持现有 URL/API 语义；搜索框与排序选择器有显式可访问名称、44px 触控高度和 2px focus ring。
- 分类为单行横向滚动药丸；选中项同时使用 `aria-pressed=true`、Indigo 柔和背景、accent 边框和较高字重，不能只用颜色表达。

**网格与卡片**
- 320/375px 固定两列、gap 12px；701-1100px 使用 `repeat(auto-fit, minmax(168px, 1fr))`、gap 16px；1280/1440px 使用 `repeat(auto-fill, minmax(192px, 1fr))`、gap 16px。
- 卡片必须 `w-full min-w-0` 填满轨道；禁止固定 156px 卡宽与 `1fr` 轨道并存。实际相邻卡间距不得超过 24px，最后一行沿用既有轨道。
- 封面固定 16:10；标题 14px medium，类别/内容数/趋势使用 12px；卡片高度稳定，长标题与 meta 省略。暗色下信息区使用 `canvas-subtle`。
- 默认 1px border + radius-lg + elevation 1；hover 使用 `border-strong` + elevation 2，封面仅克制缩放；focus-visible 2px ring；reduced-motion 下取消缩放。

**状态与交互**
- loading 使用 12 张与最终网格同轨道、同 16:10 封面比例的骨架；empty/error 使用 EmptyState 结构并分别提供清空筛选/重试；当前查询与筛选保持。
- 加载更多必须追加而非替换既有卡片；请求中禁用重复触发；已到底显示总数。卡片整块可键盘聚焦，Enter 进入 `/ip/[ipId]`。
- 浏览器验证覆盖 320/375/768/1280/1440px、light/dark、hover/focus、loading/empty/error 和无横向溢出。

## Page: /login 登录页

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：登录表单卡片（带 1px border），居中展示

**核心组件清单**
- `Header`
- `EmptyState`（表单验证错误时行内提示）
- `LoadingSpinner`（提交登录请求时按钮内嵌）

**布局规范**
- 页面最大宽度：1280px
- 登录卡片：最大宽度 400px，垂直水平居中
- 区域间距（block）：32px (`space-y-8`)
- 元素间距（inline）：16px (`gap-4`)

**状态变体**
- default: 邮箱 + 密码输入框 + 登录按钮 + 注册链接。
- loading: 登录按钮内嵌 Spinner，输入框 disabled。
- empty: 不适用（表单始终显示）。
- error: 行内红字错误提示（邮箱格式、密码长度、账号不存在等）。
- 特殊状态：已登录用户自动跳转首页。

**响应式规则**
- 移动 (≤700px): 登录卡片全宽（margin 16px），padding 20px。
- 平板 (≤1100px): 登录卡片最大宽度 400px，居中。
- PC (>1100px): 登录卡片最大宽度 400px，居中，两侧留白。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /register 注册页

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：注册表单卡片（带 1px border），居中展示

**核心组件清单**
- `Header`
- `EmptyState`（表单验证错误时行内提示）
- `LoadingSpinner`（提交注册请求时按钮内嵌）

**布局规范**
- 页面最大宽度：1280px
- 注册卡片：最大宽度 400px，垂直水平居中
- 区域间距（block）：32px (`space-y-8`)
- 元素间距（inline）：16px (`gap-4`)

**状态变体**
- default: 用户名 + 邮箱 + 密码 + 确认密码输入框 + 注册按钮 + 登录链接。
- loading: 注册按钮内嵌 Spinner，输入框 disabled。
- error: 行内红字错误提示（各字段前端校验 + 服务端返回）。
- 特殊状态：已登录用户自动跳转首页。

**响应式规则**
- 移动 (≤700px): 注册卡片全宽（margin 16px），padding 20px。
- 平板 (≤1100px): 注册卡片最大宽度 400px，居中。
- PC (>1100px): 注册卡片最大宽度 400px，居中，两侧留白。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /ip/[ipId] IP 详情页（贴吧式社区枢纽，#290 重构）

**Key Constraints**
- 单页 query 驱动：`/ip/[ipId]?tab=share|discussions|proposals&type=&sort=&status=&q=&d=`；tab/筛选就地切换（`router.replace`，不滚动不跳页），搜索 `?q=` 走 history push 可后退；刷新/分享链接/后退均还原状态；词表外的参数值回落默认。
- 三模块页内切换：内容分享（媒体类型 FilterPills + 四排序 newest/hot/most_views/best_rated + OverlayMasonryGrid 作品卡）/ 讨论区（四排序 latest_reply/newest_post/most_replies/hot + 置顶标识 + 「发起讨论」入口）/ 提案投票（四状态筛选 open/adopted/rejected/history + 提案卡 + 发起表单 + 未关注引导）。
- IP 内搜索：回车或失焦提交（输入过程不即时过滤）；搜索后停留在当前三模块结构，各 tab 内过滤同关键词；tab 计数与媒体类型 chips 计数随命中数收缩，清空搜索还原全量；当前 tab 无命中 → EmptyState「未找到与「q」相关的内容」。
- 内容分享 tab 只收该 IP 的二创（zone=fanwork）；作品卡复用内容详情浮层（`Component: ContentDetailOverlay`）。
- 讨论帖详情为页内浮层（DiscussionDetailOverlay）：标题/正文/作者 + 回帖；Esc、浏览器后退、点遮罩或 X 关闭；加载失败呈现 role="alert" + 重试。
- 共治提案卡片：字段级 diff（简介文本块 / 封面 URL / 标签 +绿 −红 chips）+ 赞成/反对双色进度条 + 门槛刻度（取自后端 config，禁止前端硬编码）+ 剩余天数 + 投票按钮；已投显示所投选择并锁按钮；未关注者投票被拒（PROPOSAL_NOT_ELIGIBLE）→ 页内「关注后可参与共治投票」面板一键关注原地解锁。
- 头部身份区：封面/名称/类目/简介/TagBadge 标签 + 关注数/讨论数/作品数三统计 + FollowButton（min-w-[104px] 固定宽度防 hover 抖动）；统计随搜索命中收缩展示。
- 旧子路由 301 收敛：`/ip/[ipId]/[category]` → `?tab=share&type=<category>`；`/ip/[ipId]/discussions*` → `?tab=discussions`。
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画；SSR 只提供身份区与统计首屏，模块列表客户端拉取。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。
- 信誉分 < 3：发帖/投票等互动受既有信誉守卫约束（服务端为准，UI 以服务端错误码呈现引导）。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-background`（SP-12 分层画布：亮 #F5F5F5 画布 / 暗 #010409 画布，卡片浮于画布之上）
- 身份区卡片：封面（208px 宽 / h-36）+ 名称/类目/简介/标签/统计/关注按钮；`bg-card` + 1px border
- 粘性工具行（sticky top-[52px]）：搜索框（药丸形态输入）+ 三模块 tab（药丸 + 计数徽标）；背景 `bg-canvas-default` + 底边框

**控件规格（SP-12 精修方向延续）**
- 三模块 tab 与筛选药丸：FilterPills 形态（见 `Component: FilterPills`）；tab 触控高度三档制中取 36/44px 档；选中态 = accent-subtle 底 + accent-emphasis 字 + 描边。
- 搜索框：rounded-full 输入（信息输入类，非操作按钮），min-h-9；清空 X 按钮内嵌右侧。
- 讨论卡/提案卡：8px 圆角矩形卡片容器，hover 边框 accent 化（150ms）。
- Hero 操作行（关注按钮）：矩形操作控件，min-w-[104px] 固定宽度，同排同高。

**核心组件清单**
- `IPHubClient`（身份区 + 粘性搜索/tab 行 + 三模块编排）
- `IPShareTab` / `IPDiscussionsTab` / `IPProposalsTab`
- `DiscussionDetailOverlay`（#290 新增：讨论帖详情浮层）
- `OverlayMasonryGrid`（作品卡网格，source="ip-page"；页面禁止直接使用裸 MasonryGrid）
- `FilterPills`、`SortSelect`、`TagBadge`、`FollowButton`、`EmptyState`、`Skeleton`

**布局规范**
- 页面最大宽度：1280px（max-w-7xl）
- 无侧边栏全宽布局；区域间距（block）：24px（`gap-6`）；卡片内 16px（`p-4`）
- 分享网格：2 列（≤700px）/ 3 列（≤1100px）/ 4 列（>1100px）
- 讨论列表与提案列表：单列卡片纵排（`space-y-2` / `space-y-3`）

**状态变体**
- default: 身份区 + 三模块 tab + 当前模块列表。
- loading: 各模块自持骨架（分享=卡片网格骨架；讨论/提案=行卡骨架），身份区 SSR 直出。
- empty: 各模块 EmptyState；无 open 提案 → CTA「第一个提案由你发起」；搜索无命中 → 「未找到与「q」相关的内容」+ 清空引导。
- error: 列表加载失败静默为空态；浮层加载失败 role="alert" + 重试；投票/关注失败 Toast。
- 特殊状态：未登录关注/投票跳登录；未关注投票弹页内关注引导。

**响应式规则**
- 移动 (≤700px): 身份区封面与信息纵排；tab 行纵向堆叠（搜索框一行、tab 一行）；分享网格 2 列。
- 平板 (≤1100px): 分享网格 3 列。
- PC (>1100px): 分享网格 4 列，粘性工具行横排。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)；diff 色（emerald/red 系）沿用语义色 token。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns；动效 150ms。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 身份区 + stats；模块列表与搜索计数客户端拉取（`cache: "no-store"`）。

## Page: /ip/[ipId]/discussions 讨论区列表（已移除，#290）

- 路由已删除：301 → `/ip/[ipId]?tab=discussions`。讨论列表并入 IP 详情页 Hub 的 discussions tab（见 `Page: /ip/[ipId]`）；本节仅作历史索引，不再是实现依据。

## Page: /ip/[ipId]/discussions/[discussionId] 讨论详情（已移除，#290）

- 路由已删除：301 → `/ip/[ipId]?tab=discussions`。讨论帖详情改为 IP 详情页内的 `DiscussionDetailOverlay` 浮层（Esc/浏览器后退/遮罩/X 关闭，含回帖），规格见 `Page: /ip/[ipId]`；本节仅作历史索引，不再是实现依据。

## Page: /ip/[ipId]/discussions/new 发帖页

**Key Constraints**
- 二创区页面/组件：依托于 ips.category 进行展示或跳转。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：发帖表单卡片（标题 + 内容 + 提交按钮）

**核心组件清单**
- `Header`
- `MarkdownEditor`
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：720px，居中
- 标题输入框 → Markdown 编辑器 → 提交按钮

**状态变体**
- default: 讨论标题 + Markdown 编辑器 + 发布按钮。
- loading: 提交按钮内嵌 Spinner。
- error: 行内红字提示 + Toast 报错。
- 特殊状态：未登录拦截；信誉分不足拦截。

**响应式规则**
- 移动 (≤700px): 表单全宽（margin 16px），编辑器高度 300px。
- 平板 (≤1100px): 表单最大宽度 640px，居中。
- PC (>1100px): 表单最大宽度 720px，居中。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /content/[contentId] 内容详情页

**Key Constraints**
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：详情内容卡片 + 评论区 + 版本历史，各模块带 1px border

**核心组件清单**
- `Header`
- `ContentDetail`
- `SheetMusicViewer`（content_type=sheet_music 时渲染）
- `ReactionBar`
- `CommentSection`
- `VersionHistory`
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：960px，居中
- 详情主体 → 操作栏 → 评论区 → 版本历史，纵向排列
- 区域间距（block）：32px (`space-y-8`)
- 元素间距（inline）：16px (`gap-4`)

**状态变体**
- default: 内容详情 + 点赞/点踩/收藏 + 评论区 + 版本历史。
- loading: 骨架屏（Skeleton 灰色块填充）。
- empty: 内容不存在 404 EmptyState。
- error: 内容被 ban 或权限不足时显示对应 EmptyState。
- 特殊状态：信誉分不足用户评论/点赞按钮 disabled。

**响应式规则**
- 移动 (≤700px): 内容区全宽（margin 16px），操作栏图标按钮横向排列。
- 平板 (≤1100px): 内容区最大宽度 720px，居中。
- PC (>1100px): 内容区最大宽度 960px，居中，评论区侧栏吸附。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /original 原创区首页

**Key Constraints**
- 参考小红书网页端设计：顶部 Tab 导航 + 瀑布流内容，单层导航结构，无二级内容类型筛选。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。
- **排序语义（#81 权威）**：未选分类且未显式选择排序时请求 `sort=recommended`；选中任何分类时排序默认降级为 `hot`（推荐排序管线无视分类等筛选条件，不得把 `recommended` 误传给带筛选的请求）；深链 URL 携带 `sort=recommended` 与分类参数时同样按 hot 语义处理。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 分类 Tab 栏：Header 下方固定（sticky），高度 `h-12`，底部 1px border 分隔，Tab 项横向滚动
- 主容器：居中最大宽度 1280px，页面背景 `bg-canvas-subtle`
- 内容模块：瀑布流卡片（无外层 border 容器，卡片直接排列）

**核心组件清单**
- `Header`
- `CategoryTabs`（原创区顶部分类 Tab 栏，横向滚动）
- `ContentCard`（原创区简化样式：封面自然比例 + 标题 + @作者 + 点赞数）
- `MasonryGrid`（瀑布流，支持无限滚动）
- `ContentDetailOverlay`（共享详情浮层，source=`zone-page`）
- `SkeletonCard`（内容卡片骨架屏）
- `EmptyState`
- `Footer`

**分类 Tab 栏规范**
- 固定项目：`推荐`（默认选中，始终在第一位）
- 动态项目：从 `/api/v1/categories?zone=original&level=primary` 加载的 11 个一级分类
- 每个 Tab 项: `<button>` 样式，padding `px-4 py-2`，字号 `text-sm`
- **选中态（#64 决策 4 权威，与 IP 库彩色药丸一致）**：`rounded-full border border-accent-emphasis bg-accent-subtle text-accent-emphasis font-semibold`，同时设置 `aria-pressed="true"`（不能只靠颜色表达）；未选中态 `rounded-full text-fg-muted hover:bg-canvas-subtle`、`aria-pressed="false"`
- 横向滚动容器：`overflow-x-auto whitespace-nowrap scrollbar-hide`（隐藏滚动条）
- 无二级内容类型筛选（不区分图文/视频/音频等）

**布局规范**
- 页面最大宽度：1280px，居中
- 分类 Tab 栏：sticky top-16（Header 下方），z-10
- 内容瀑布流：`px-4 py-6`，列数响应式 2/3/4 列
- 卡片间距：`gap-4`

**状态变体**
- default: 推荐流（或选中分类的内容流），瀑布流卡片。
- loading: 骨架屏（SkeletonCard 灰色块，模拟卡片形状），首次加载 12 个占位。
- empty: EmptyState"还没有内容，快来发布第一条吧" + 发布 CTA。
- error: Toast 右上角报错 + 重试按钮。
- 特殊状态：未登录也不影响浏览（原创区公开浏览），信誉分不足不在此拦截。

**无限滚动行为**
- 初始加载：SSR 首屏 24 条
- 客户端滚动：IntersectionObserver 监听底部 sentinel 元素
- 加载更多：`page=2,3,4...` 追加到现有列表（SWR useSWRInfinite）
- 已加载全部时：显示"已经到底了"提示，不再触发请求

**响应式规则**
- 移动 (≤700px): 2 列瀑布流，Tab 栏 sticky 顶部，无 Header 时全宽。
- 平板 (≤1100px): 3 列瀑布流，分类 Tab 栏横向滚动。
- PC (>1100px): 4 列瀑布流，左对齐，分类 Tab 栏居中显示。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片使用 `opacity-90` 适配暗色，占位 SVG 使用反色。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 分类 Tab 点击：切换内容流（`router.push` 更新 URL query），保留滚动位置。
- 卡片点击：打开共享 `ContentDetailOverlay`（source=`zone-page`），不跳转详情页；直接访问 `/original/[contentId]` URL 才使用完整详情页。
- 卡片 hover：封面图 scale-105 + 浅色遮罩（小红书式）。
- 数据加载策略: SSR 首屏，客户端 SWR 无限滚动加载后续页。

## Page: /original/[contentId] 原创内容详情

**Key Constraints**
- 原创区无 IP 概念，严格按照 content_items.category (推荐|影视|游戏|文学等) 分类。
- 隐藏 PR 入口（原创区无协同机制）。
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：详情内容卡片 + 评论区，各模块带 1px border

**核心组件清单**
- `Header`
- `ContentDetail`
- `SheetMusicViewer`（content_type=sheet_music 时渲染）
- `ReactionBar`
- `CommentSection`
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：960px，居中
- 详情主体 → 操作栏 → 评论区，纵向排列
- 区域间距（block）：32px (`space-y-8`)

**状态变体**
- default: 内容详情 + 点赞/点踩/收藏 + 评论区（无 PR 版本历史）。
- loading: 骨架屏（Skeleton 灰色块填充）。
- empty: 内容不存在 404 EmptyState。
- error: 内容被 ban 或权限不足时显示对应 EmptyState。

**响应式规则**
- 移动 (≤700px): 内容区全宽（margin 16px），操作栏图标按钮横向排列。
- 平板 (≤1100px): 内容区最大宽度 720px，居中。
- PC (>1100px): 内容区最大宽度 960px，居中。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /user/[userId] 用户主页

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：用户信息卡片 + 内容 Tab 切换区（发布/收藏/讨论）

**核心组件清单**
- `Header`
- `UserProfileCard`
- `FollowButton`
- `FollowerListModal`
- `JudgeQualBadge`
- `ContentCard`
- `MasonryGrid`（内容 Tab 中使用）
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：960px，居中
- 用户信息卡片 → Tab 切换栏 → 内容列表
- 区域间距（block）：24px (`space-y-6`)

**状态变体**
- default: 用户头像/用户名/信誉分/判官资质 Badge + 关注按钮 + 内容 Tab。
- loading: 骨架屏（Skeleton 灰色块填充用户卡片和内容区）。
- empty: 用户不存在 404 EmptyState；某 Tab 无内容时显示 EmptyState。
- error: Toast 右上角报错。
- 特殊状态：未登录时关注按钮跳转 /login；当前用户自己的主页时隐藏关注按钮显示编辑入口。

**响应式规则**
- 移动 (≤700px): 用户卡片全宽（margin 16px），内容 Tab 列表单列。
- 平板 (≤1100px): 内容区最大宽度 640px，Tab 内容瀑布流 3 列。
- PC (>1100px): 内容区最大宽度 960px，用户信息左侧吸附，Tab 内容瀑布流 4 列。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /settings 账号设置

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。
- 密码修改/注销等破坏性操作需 ConfirmModal 二次确认。
- 联合创作邀请接收开关属于账号偏好设置，必须与 `users.accept_collab_invites` / `GET /api/v1/auth/me` 保持一致；默认值为开启。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：设置分组卡片（个人信息 / 安全设置 / 危险操作），每组带 1px border

**核心组件清单**
- `Header`
- `ConfirmModal`（密码修改确认、账号注销确认）
- `Switch`（接收联合创作邀请）
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：720px，居中
- 设置分组卡片纵向排列，组间距 24px (`space-y-6`)
- 元素间距（inline）：16px (`gap-4`)

**状态变体**
- default: 头像、用户名、邮箱（只读）、Bio 编辑表单。
- collaboration-default: 「联合创作邀请」设置组展示 `accept_collab_invites` 开关，说明关闭后其他用户无法向你发送联合创作邀请。
- collaboration-saving: 开关保存时当前行 disabled，显示局部保存状态；其他设置组不应被整页锁定。
- collaboration-error: 保存失败时恢复到上一次服务端值，并显示 localized Toast + 行内错误。
- loading: 保存按钮内嵌 Spinner。
- error: 行内红字错误提示（字段校验失败、旧密码错误等）。
- 特殊状态：未登录拦截跳转 /login。

**响应式规则**
- 移动 (≤700px): 设置卡片全宽（margin 16px），padding 16px。
- 平板 (≤1100px): 设置区最大宽度 640px，居中。
- PC (>1100px): 设置区最大宽度 720px，居中，两侧留白。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。
- 联合创作开关保存调用 `PATCH /api/v1/users/:id`，请求体只包含 `accept_collab_invites` 及当前表单实际变更字段；保存成功后必须刷新 `AuthContext` / `/api/v1/auth/me`。
- 开关必须可键盘操作，使用显式 label，不得只使用颜色表示开启/关闭。

**i18n key namespace**
- 建议 namespace：`settings.*`。
- 联合创作设置覆盖：`settings.collaboration.title`、`settings.collaboration.description`、`settings.collaboration.acceptInvites.label`、`settings.collaboration.acceptInvites.help`、`settings.collaboration.toast.*`、`settings.collaboration.error.*`、`settings.collaboration.a11y.*`。

**Playwright 截图检查点**
- `screenshots/community-collab-settings-desktop.png`：设置页联合创作邀请分组，开关开启。
- `screenshots/community-collab-settings-mobile.png`：移动端设置页，开关 44px 触控目标可见。

## Page: /settings/tag-groups 标签组管理

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：标签组列表卡片 + 新建/编辑表单区

**核心组件清单**
- `Header`
- `TagBadge`
- `ConfirmModal`（删除标签组确认）
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：720px，居中
- 标签组列表纵向排列，每组卡片展示名称 + 标签 Badge 列表 + 编辑/删除操作
- 区域间距（block）：24px (`space-y-6`)

**状态变体**
- default: 标签组列表 + 新建按钮。
- loading: 骨架屏（Skeleton 灰色块）。
- empty: 暂无标签组 EmptyState + 新建 CTA。
- error: Toast 右上角报错。

**响应式规则**
- 移动 (≤700px): 列表全宽（margin 16px），操作按钮展开式。
- 平板 (≤1100px): 内容区最大宽度 640px，居中。
- PC (>1100px): 内容区最大宽度 720px，居中。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /dashboard/contents 我的内容

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `AdminNav`
- `Table`
- `ConfirmModal`

**布局规范**
- 页面最大宽度：1280px / 满宽
- 主内容区与侧边栏比例：无侧边栏（全宽）或 3:1/4:1
- 区域间距（block）：32px (`space-y-8`)
- 元素间距（inline）：16px (`gap-4`)

**状态变体**
- default: 默认数据展示或列表。
- loading: 全屏加载骨架屏（Skeleton），不使用全屏遮罩 loading。
- empty: 使用 EmptyState 组件（图标 + 标题 + 说明 + CTA）。
- error: Toast 右上角报错或内联提示。
- 特殊状态：信誉分不足、权限不足或未登录拦截。

**响应式规则**
- 移动 (≤700px): 表格水平滚动，操作按钮展开式。
- 平板 (≤1100px): 表格自适应宽度。
- PC (>1100px): 表格最大宽度 1280px。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /dashboard/pr-requests PR 申请管理

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `AdminNav`
- `Table`
- `ConfirmModal`

**布局规范**
- 页面最大宽度：1280px / 满宽
- 主内容区与侧边栏比例：无侧边栏（全宽）或 3:1/4:1
- 区域间距（block）：32px (`space-y-8`)
- 元素间距（inline）：16px (`gap-4`)

**状态变体**
- default: 默认数据展示或列表。
- loading: 全屏加载骨架屏（Skeleton），不使用全屏遮罩 loading。
- empty: 使用 EmptyState 组件（图标 + 标题 + 说明 + CTA）。
- error: Toast 右上角报错或内联提示。
- 特殊状态：信誉分不足、权限不足或未登录拦截。

**响应式规则**
- 移动 (≤700px): 表格水平滚动，操作按钮展开式。
- 平板 (≤1100px): 表格自适应宽度。
- PC (>1100px): 表格最大宽度 1280px。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /dashboard/contributors 贡献者管理

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `AdminNav`
- `Table`
- `ConfirmModal`

**布局规范**
- 页面最大宽度：1280px
- 左侧管理导航 + 右侧表格
- 区域间距（block）：32px (`space-y-8`)
- 元素间距（inline）：16px (`gap-4`)

**状态变体**
- default: 数据表格 + 操作列。
- loading: 表格骨架屏（Skeleton），不使用全屏遮罩 loading。
- empty: 使用 EmptyState 组件（图标 + 标题 + 说明 + CTA）。
- error: Toast 右上角报错或内联提示。
- 特殊状态：非 admin/创作者角色 403 拦截。

**响应式规则**
- 移动 (≤700px): 表格水平滚动，操作按钮展开式。
- 平板 (≤1100px): 表格自适应宽度。
- PC (>1100px): 表格最大宽度 1280px。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /dashboard/tag-suggestions 标签建议审核

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：标签建议审核表格 + 操作列

**核心组件清单**
- `Header`
- `AdminNav`
- `Table`
- `TagBadge`
- `ConfirmModal`（通过/拒绝确认）
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：1280px
- 左侧管理导航 + 右侧标签建议审核表格
- 区域间距（block）：24px (`space-y-6`)

**状态变体**
- default: 待审标签建议表格（标签名/内容/建议人/操作）。
- loading: 表格骨架屏（Skeleton）。
- empty: "暂无待审核标签建议" EmptyState。
- error: Toast 右上角报错。
- 特殊状态：非 admin/创作者角色 403 拦截。

**响应式规则**
- 移动 (≤700px): 表格水平滚动，操作按钮展开式，管理导航折叠。
- 平板 (≤1100px): 表格自适应宽度。
- PC (>1100px): 左侧管理导航 228px + 右侧表格自适应。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /judge/exam 赛博判官资质考核

**Key Constraints**
- 赛博判官业务规则：只有具有对应类型的判官权限（judge_qualifications）或通过考核才能操作。
- 信誉分必须 >= 3 才能行使众裁权利，否则禁用功能。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：考题卡片（题目 + 选项 + 提交按钮）

**核心组件清单**
- `Header`
- `ExamQuestion`
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：640px，居中
- 考题卡片纵向排列，每次展示 1 题
- 支持进度条显示已完成/总题数

**状态变体**
- default: 考题展示 + 选项 + 提交按钮。
- loading: 骨架屏（Skeleton 灰色块）。
- empty: 该类型无可用考题时 EmptyState。
- error: Toast 右上角报错。
- 特殊状态：未登录拦截；已通过考核显示"已获得资格"；信誉分不足显示禁用提示。

**响应式规则**
- 移动 (≤700px): 考题卡片全宽（margin 16px），选项按钮纵向堆叠。
- 平板 (≤1100px): 考题区最大宽度 540px，居中。
- PC (>1100px): 考题区最大宽度 640px，居中。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /judge/queue 待审内容队列

**Key Constraints**
- 赛博判官业务规则：只有具有对应类型的判官权限（judge_qualifications）或通过考核才能操作。
- 信誉分必须 >= 3 才能行使众裁权利，否则禁用功能。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：待审案例卡片队列

**核心组件清单**
- `Header`
- `ReviewCard`
- `VerdictDetail`
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：720px，居中
- 每次展示 1 个待审案例，投票后展示下一条
- 投票分布实时显示

**状态变体**
- default: 案例详情 + 违规/不违规投票按钮 + 理由输入 + 投票分布。
- loading: 骨架屏（Skeleton 灰色块）。
- empty: 队列空时 EmptyState"暂无待审内容"。
- error: Toast 右上角报错。
- 特殊状态：未登录拦截；无对应类型判官资格显示"需先通过考核"；信誉分不足禁用投票。

**响应式规则**
- 移动 (≤700px): 案例卡片全宽（margin 16px），投票按钮全宽纵向排列。
- 平板 (≤1100px): 案例区最大宽度 640px，居中。
- PC (>1100px): 案例区最大宽度 720px，居中。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /history 浏览历史

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。
- 页面位于 `(protected)` route group；未登录访问由受保护布局重定向到 `/login?redirect=/history`。
- 保留期、分页大小、筛选条件均来自 API 响应或 URL query，组件内不得硬编码保留天数。

**目标与放置**
- 目标：让用户在配置驱动的保留窗口内快速找回看过的内容，并支持 content_type、日期范围、批量删除和失效内容识别。
- 放置：`frontend/app/(protected)/history/page.tsx`，使用全局 `Header`；页面主体不嵌入 Studio/Admin 布局。
- 失效历史项保留浏览时间，但内容区显示不可点击灰色占位，避免用户以为数据丢失。

**核心组件清单**
- `Header`
- `ContentCard`（published 内容）
- `SkeletonCard`
- `ConfirmModal`（清除全部确认）
- `EmptyState`
- `LoadingSpinner`
- `Toast`

**布局规范**
- PC (>1100px)：页面最大宽度 `960px`，居中；顶部为一行工具栏（content_type chips、日期范围、批量模式按钮），下方按日期分组纵向列表；列表项使用紧凑横向卡片，不使用瀑布流。
- 平板 (701-1100px)：最大宽度 `720px`；筛选 chips 可横向滚动，日期范围换到第二行；批量操作固定在工具栏末尾。
- 移动 (<=700px)：主体 `px-4 py-4`；工具栏拆成纵向两组，content_type chips 横向滚动；历史项单列全宽；批量操作条吸附在列表上方而非底部浮层。
- 日期分组标题使用 `text-xs text-fg-muted`，列表项间距 `gap-3`，分组间距 `space-y-6`。

**状态变体**
- default：按日期分组展示 `items`；优先读取新字段 `items`，兼容旧 `history` / `content_item`。
- loading：首屏展示工具栏骨架 + 5 条 `SkeletonCard`，保留稳定高度，避免筛选栏加载时位移。
- empty：保留工具栏，内容区显示 `EmptyState` + 回到首页 CTA；文案来自 `history.empty.*`。
- error：Toast 报错；若已有成功数据，保留旧数据并在工具栏下显示轻量内联错误提示。
- success：删除选中或清空成功后显示 Toast，并局部刷新列表；批量模式自动退出。
- disabled：批量模式无选中项时删除按钮 disabled；失效内容卡片 disabled 且不可聚焦为链接。
- expired/unavailable item：`content: null` 时渲染灰色占位卡片，标题/说明使用 `history.unavailable.*`，整卡不可点击。

**响应式规则**
- PC：工具栏一行，历史项为 `grid grid-cols-[auto_1fr_auto]`（批量复选框 / 内容摘要 / 时间与操作）。
- 平板：工具栏两行，列表项仍横向但封面缩小。
- 移动：列表项改为封面在上、信息在下；复选框保持左上角固定 44px 命中区。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- content_type chip：点击后更新 URL query `content_type` 并重新请求；再次点击当前 chip 回到全部。
- 日期范围：两个日期输入按 `start_date`、`end_date` 写入 query；无效日期使用内联错误，不发请求。
- 批量模式：点击批量按钮后列表项显示复选框；`Delete` 操作必须弹 `ConfirmModal`；清空全部也必须二次确认。
- 焦点顺序：页面标题 -> content_type chips -> start_date -> end_date -> 批量模式 -> 列表项/复选框 -> 分页或加载更多 -> 清空全部。
- 键盘：chips 和列表操作支持 `Tab`、`Enter`/`Space`；日期输入有可见 label；Esc 关闭 ConfirmModal。
- 数据加载策略：SSR 输出页面框架，客户端使用 SWR 按 query 加载；分页使用 `page` + `page_size`，兼容 `limit` 仅在 API 层处理。

**可访问性**
- 所有正文与背景对比度不低于 4.5:1；失效占位使用 `text-fg-muted` 时必须配合足够深的边框/背景 token。
- 所有可点击控件触控目标不小于 44px；横向 chips 之间至少 `gap-2`。
- 批量复选框必须有 `aria-label`，包含内容标题或失效项日期。
- 工具栏使用 `aria-label={t('history.toolbar.ariaLabel')}`；日期范围使用关联 `<label>`。

**i18n key namespace**
- 建议 namespace：`history.*`。
- 需要覆盖：`history.filters.*`、`history.dateRange.*`、`history.bulk.*`、`history.empty.*`、`history.error.*`、`history.unavailable.*`、`history.toast.*`、`history.a11y.*`。
- 组件内不得硬编码中文/英文文案；保留天数由 API `retention_days` 插值。

**与现有组件关系**
- `ContentCard` 只用于有效 published 内容；失效项不得伪装为 ContentCard 链接。
- 删除确认复用 `ConfirmModal`，Toast 复用全局提示模式。
- 不依赖 `ContentDetail`，但跳转目标必须匹配内容详情现有路由 normalize 规则。

**Playwright 截图检查点**
- `screenshots/community-browse-history-desktop.png`：PC 宽度，含筛选工具栏、日期分组、有效内容和失效占位。
- `screenshots/community-browse-history-mobile.png`：移动宽度，chips 横向滚动、批量复选框 44px 命中区可见。
- 交互检查：content_type query、日期 query、删除选中 ConfirmModal、清空后 EmptyState、网络错误保留旧数据。

## Page: /appeals 我的申诉

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：申诉列表卡片 + 新建申诉表单

**核心组件清单**
- `Header`
- `EmptyState`
- `LoadingSpinner`
- `ConfirmModal`（提交申诉确认）

**布局规范**
- 页面最大宽度：720px，居中
- 申诉列表 + 申诉表单，纵向排列
- 区域间距（block）：24px (`space-y-6`)

**状态变体**
- default: 我的申诉列表（含状态 Badge）+ 新建申诉入口。
- loading: 骨架屏（Skeleton）。
- empty: "暂无申诉记录" EmptyState。
- error: Toast 右上角报错。
- 特殊状态：未登录拦截；被 ban 内容才可申诉。

**响应式规则**
- 移动 (≤700px): 列表全宽（margin 16px）。
- 平板 (≤1100px): 内容区最大宽度 640px，居中。
- PC (>1100px): 内容区最大宽度 720px，居中。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /messages 消息中心

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border；Modal/Popover 遵守全局例外。
- 本轮 scope：通知列表、私信会话列表、`ChatWindow`、广播通知视觉标记、5 分钟未读数轮询兜底。
- Future/out-of-scope：`/api/v1/notifications/stream` 和 `/api/v1/messages/stream` SSE 实时推送；本轮不得为了 SSE 新增 provider、路由或浏览器验证。

**目标与放置**
- 目标：把通知和私信收敛到同一个双栏消息中心，保留通知时间线、私信会话、未读状态和系统广播的可扫描差异。
- 放置：`frontend/app/(protected)/messages/page.tsx`；未登录访问由 protected layout 重定向。
- Header 通知铃铛继续轮询 `GET /api/v1/notifications/unread-count`，点击跳转 `/messages?tab=notifications`。

**核心组件清单**
- `Header`
- `NotificationList`
- `ConversationList`
- `ChatWindow`
- `CollabInviteCard`（后续 collaboration-invites 计划扩展 typed message）
- `MarkdownRenderer`（广播正文）
- `EmptyState`
- `LoadingSpinner`
- `Toast`

**布局规范**
- PC (>1100px)：页面最大宽度 `1180px`，主内容为 `grid-cols-[320px_minmax(0,1fr)]`；左栏为通知/私信 segmented tabs + 对应列表，右栏为通知详情或 `ChatWindow`。
- 平板 (701-1100px)：左栏 `280px`，右栏自适应；列表项 unread badge 不得改变行高。
- 移动 (<=700px)：单栏 list/detail drill-in；顶部保留通知/私信 segmented tabs；进入聊天后提供返回列表按钮。
- 列表项高度稳定：通知项最小 `72px`，会话项最小 `64px`；头像、红点、badge 使用固定尺寸。
- 禁止页面 section 再包卡片；列表项可使用 1px border 分隔。

**状态变体**
- default：通知 Tab 显示混合时间线；私信 Tab 显示左侧会话列表和右侧聊天窗口。
- loading：列表 skeleton x6；右侧 detail skeleton 保持宽高，不跳动。
- empty：通知为空和私信为空分别使用 localized EmptyState。
- error：Toast + 局部重试按钮；保留上一份成功数据。
- unread：通知和会话列表项显示红点/数字 badge；已读项降低辅助文字对比但仍满足可读性。
- broadcast：`channel === "broadcast"` 的系统通知使用蓝色左边框、system icon 和 Markdown 摘要；不可只靠颜色区分，需有广播文本/aria-label。
- direct-message：`ChatWindow` 中本人消息靠右，对方消息靠左，输入框固定在窗口底部。
- typed-message：非 `text` 类型消息由 `ChatWindow` 分支渲染；未知 `msg_type` 显示安全 fallback 文本，不渲染原始 metadata。

**响应式规则**
- 移动 (<=700px)：列表和聊天/通知详情互斥显示；所有可点击项触控目标不小于 44px。
- 平板 (<=1100px)：左栏固定 280px，右栏 min-width 0，长标题 line-clamp。
- PC (>1100px)：左栏 320px，右栏自适应；消息气泡最大宽度 `min(70%, 620px)`。

**可访问性**
- 通知/私信切换使用 `role="tablist"` 或语义按钮组；当前项使用 `aria-current` 或 `aria-selected`。
- 会话列表项和通知列表项可键盘聚焦；Enter/Space 打开。
- `ChatWindow` 消息列表使用 `aria-live="polite"`，发送失败 Toast 不抢焦点。
- 删除/清空类动作必须使用 `ConfirmModal`，打开时焦点锁定，Esc 关闭。
- Markdown 链接可聚焦；广播图片有安全 fallback alt。

**i18n key namespace**
- 建议 namespace：`messages.*`。
- 覆盖：`messages.tabs.*`、`messages.notifications.*`、`messages.conversations.*`、`messages.chat.*`、`messages.broadcast.*`、`messages.empty.*`、`messages.error.*`、`messages.a11y.*`。
- 不在 TSX 中硬编码“通知/私信/暂无消息/系统广播/发送失败”等文案。

**Playwright 截图检查点**
- `screenshots/community-messages-notifications-desktop.png`：PC 双栏，通知 Tab 选中，含 broadcast 蓝色标记和 unread 状态。
- `screenshots/community-messages-notifications-mobile.png`：移动单栏，私信列表进入 `ChatWindow` 后可返回。
- 交互检查：tab 切换、通知标记已读、DM 冷启动错误 Toast、输入发送、未知 typed message fallback。

## Page: /rehab 素质建设课程

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。
- 每门课程仅能完成一次，最低阅读 3 分钟（180 秒）。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：课程列表卡片 + 课程内容详情区

**核心组件清单**
- `Header`
- `CourseCard`
- `CourseContent`
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：720px，居中
- 课程列表 → 课程内容（Markdown 渲染），阅读计时器

**状态变体**
- default: 课程列表 + 已匹配违规类型高亮。
- loading: 骨架屏（Skeleton）。
- empty: "暂无可选课程" EmptyState。
- error: Toast 右上角报错。
- 特殊状态：未登录拦截；信誉分 ≥ 3 时显示"信誉分正常，无需学习"。

**响应式规则**
- 移动 (≤700px): 课程区全宽（margin 16px），课程内容 Markdown 自适应。
- 平板 (≤1100px): 内容区最大宽度 640px，居中。
- PC (>1100px): 内容区最大宽度 720px，居中。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /admin/ips IP 库管理

**Key Constraints**
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：待审核 IP 表格 + 操作列

**核心组件清单**
- `Header`
- `AdminNav`
- `Table`
- `ConfirmModal`（通过/拒绝 IP 二次确认）
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：1280px
- 左侧管理导航 + 右侧 IP 审核表格
- 表格支持状态筛选、搜索

**状态变体**
- default: 待审 IP 表格（名称/分类/提交人/状态/操作）。
- loading: 表格骨架屏。
- empty: "无待审核 IP" EmptyState。
- error: Toast 右上角报错。
- 特殊状态：非 admin 角色 403 拦截。

**响应式规则**
- 移动 (≤700px): 表格水平滚动，管理导航折叠为顶部 Tab。
- 平板 (≤1100px): 表格自适应宽度。
- PC (>1100px): 左侧管理导航 228px + 右侧表格自适应。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /admin/contents 内容终审

**Key Constraints**
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：待审内容表格 + 操作列

**核心组件清单**
- `Header`
- `AdminNav`
- `Table`
- `ConfirmModal`（ban 内容二次确认）
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：1280px
- 左侧管理导航 + 右侧内容表格
- 表格支持分页、状态筛选

**状态变体**
- default: 待审内容表格（标题/作者/类型/举报数/状态/操作）。
- loading: 表格骨架屏。
- empty: "无待审内容" EmptyState。
- error: Toast 右上角报错。
- 特殊状态：非 admin 角色 403 拦截。

**响应式规则**
- 移动 (≤700px): 表格水平滚动，管理导航折叠为顶部 Tab。
- 平板 (≤1100px): 表格自适应宽度，管理导航左侧 180px。
- PC (>1100px): 左侧管理导航 228px + 右侧表格自适应。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /admin/users 用户管理

**Key Constraints**
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：用户表格 + 搜索筛选 + 操作列

**核心组件清单**
- `Header`
- `AdminNav`
- `Table`
- `ConfirmModal`（ban 用户二次确认，输入原因）
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：1280px
- 左侧管理导航 + 右侧用户表格
- 表格支持搜索、分页、信誉分筛选

**状态变体**
- default: 用户表格（头像/用户名/邮箱/信誉分/角色/状态/操作）。
- loading: 表格骨架屏。
- empty: "无匹配用户" EmptyState。
- error: Toast 右上角报错。
- 特殊状态：非 admin 角色 403 拦截。

**响应式规则**
- 移动 (≤700px): 表格水平滚动，管理导航折叠为顶部 Tab。
- 平板 (≤1100px): 表格自适应宽度。
- PC (>1100px): 左侧管理导航 228px + 右侧表格自适应。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /admin/appeal 申诉处理

**Key Constraints**
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：申诉列表 + 申诉详情侧栏 + 处理操作

**核心组件清单**
- `Header`
- `AdminNav`
- `Table`
- `ConfirmModal`（通过/驳回申诉二次确认，输入处理原因）
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：1280px
- 左侧管理导航 + 申诉列表

**状态变体**
- default: 申诉列表（申请人/类型/原内容/原因/状态/操作）。
- loading: 表格骨架屏。
- empty: "无待处理申诉" EmptyState。
- error: Toast 右上角报错。
- 特殊状态：非 admin 角色 403 拦截。

**响应式规则**
- 移动 (≤700px): 表格水平滚动，管理导航折叠为顶部 Tab。
- 平板 (≤1100px): 表格自适应宽度。
- PC (>1100px): 左侧管理导航 228px + 右侧表格自适应。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /admin/config 系统配置

**Key Constraints**
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。
- 热更新配置，重启后恢复 config.yaml 默认值。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：配置分组表单（limits/features/reputation/agent/social/labels）

**核心组件清单**
- `Header`
- `AdminNav`
- `ConfirmModal`（保存配置二次确认）
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：720px，居中
- 左侧管理导航 + 配置表单（JSON/YAML 编辑或结构化表单）

**状态变体**
- default: 当前配置键值对表单。
- loading: 骨架屏（Skeleton）。
- error: 行内红字提示 + Toast 报错。
- 特殊状态：非 admin 角色 403 拦截。

**响应式规则**
- 移动 (≤700px): 表单全宽（margin 16px），管理导航折叠。
- 平板 (≤1100px): 配置区最大宽度 640px，居中。
- PC (>1100px): 左侧管理导航 228px + 右侧配置表单 500px。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /admin/notifications 管理员系统通知广播

**Key Constraints**
- 后台页面：Role 必须为 admin；页面只负责系统广播编辑与发送，不提供撤回入口。
- 广播正文允许 Markdown 图片和链接，但预览和发送后的通知详情必须复用安全 Markdown 渲染链路，禁止原始 HTML 绕过消毒。
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none；除 ConfirmModal/Dropdown 外不得使用 shadow。

**目标与放置**
- 目标：让管理员创建面向全站活跃用户的系统通知广播，并在发送前明确预览、确认不可撤回和展示收件人数结果。
- 放置：`frontend/app/(protected)/admin/notifications/page.tsx`，使用现有 AdminLayout/AdminNav；不进入普通 `Header` 通知下拉的编辑入口。
- 视觉角色：后台表单工具页，信息密度适中，可扫描，不使用营销式 hero、装饰卡片或横幅。

**核心组件清单**
- `AdminNav`
- `MarkdownEditor`
- `MarkdownRenderer`
- `ConfirmModal`
- `Toast`
- `LoadingSpinner`

**布局规范**
- PC (>1100px)：AdminLayout 左侧 228px 导航；主区最大宽度 `1180px`，使用两列 `grid-cols-[minmax(420px,1fr)_minmax(360px,0.9fr)]`，左侧编辑表单，右侧实时预览。
- 平板 (701-1100px)：两列改为上下堆叠；预览区在编辑区下方，保持 `border border-border-default rounded-lg`。
- 移动 (<=700px)：AdminNav 折叠为顶部或抽屉；表单单列；MarkdownEditor 高度降到 `280px`；发送按钮全宽但仍在正常文档流中。
- 页面区块间距 `space-y-6`；表单字段使用显式 label，字段说明为 `text-xs text-fg-muted`。

**状态变体**
- default：标题输入、Markdown 正文编辑器、预览、发送按钮均可用。
- loading：初始化或发送中显示局部 Spinner；发送中标题/正文/按钮 disabled，预览保持最后输入内容。
- empty：预览为空时显示轻量 EmptyState，占位文案走 `adminNotifications.preview.empty.*`。
- error：字段校验错误显示在字段下方；API 错误 Toast；失败后保留用户输入。
- success：发送成功后 Toast 显示 `recipient_count` 和 `broadcast_at`；表单可选择保留内容但发送按钮恢复可用。
- disabled：非 admin 或权限刷新失败显示 403 EmptyState；发送按钮在标题/正文为空、超长、发送中时 disabled。

**核心交互与焦点顺序**
- 焦点顺序：页面标题 -> 标题输入 -> 正文编辑器 -> 预览区域跳转链接 -> 发送按钮 -> 清空/返回等次要动作。
- 标题输入实时显示字数，约束 1-120 字符；正文实时显示字数，约束 1-5000 字符。
- 点击发送：先前端校验 -> 打开 ConfirmModal -> 确认后调用 `POST /api/v1/admin/notifications/broadcast`，请求体固定 `channel: "broadcast"`。
- ConfirmModal 必须提示不可撤回、将面向活跃用户发送；键盘 Esc 关闭，Enter 不应误触发送，需聚焦确认按钮后 Space/Enter。
- 预览区使用 `MarkdownRenderer`，链接在预览中可聚焦但不自动打开新窗口；图片需有安全 fallback。

**可访问性**
- 正文、字段错误、按钮文字与背景对比度不低于 4.5:1。
- 所有按钮、Tab、预览链接触控目标不小于 44px。
- 图标按钮必须有 `aria-label`；预览区域使用 `aria-live="polite"` 但避免每次键入都朗读完整 Markdown。
- ConfirmModal 使用 `role="dialog"`、`aria-modal="true"`，打开时焦点锁定在弹窗内。

**i18n key namespace**
- 建议 namespace：`adminNotifications.*`。
- 覆盖：`adminNotifications.form.*`、`adminNotifications.preview.*`、`adminNotifications.confirm.*`、`adminNotifications.toast.*`、`adminNotifications.validation.*`、`adminNotifications.a11y.*`。
- 不在 TSX 中硬编码中文/英文按钮、字段、错误、成功文案。

**与现有组件关系**
- 复用 `MarkdownRenderer` 的安全渲染规则；若存在 `MarkdownEditor`，沿用发布页编辑器交互，不新增第二套 Markdown 工具。
- 成功发送后的用户侧展示由 `NotificationList` 负责，本页只显示发送结果，不渲染用户通知列表。
- 使用 AdminLayout/AdminNav 的现有导航密度和 1px 边框语言，不创建独立后台设计系统。

**Playwright 截图检查点**
- `screenshots/community-messages-notifications-admin-desktop.png`：PC 双列编辑 + 预览，发送确认弹窗打开。
- `screenshots/community-messages-notifications-admin-mobile.png`：移动单列表单，预览在下方且按钮 44px。
- 交互检查：空表单校验、Markdown 预览、安全链接、确认后成功 Toast 包含收件人数、API 失败保留输入。

## Page: /admin/categories 分类与标签管理

**Key Constraints**
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：分类树形列表 + 标签管理

**核心组件清单**
- `Header`
- `AdminNav`
- `Table`
- `ConfirmModal`（增删改分类二次确认）
- `TagBadge`
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：1280px
- 左侧管理导航 + 右侧分类树/标签列表
- 支持拖拽排序、增删改

**状态变体**
- default: 分类树（父/子级）+ 标签列表 + 操作按钮。
- loading: 骨架屏（Skeleton）。
- empty: 分类/标签为空时 EmptyState。
- error: Toast 右上角报错。
- 特殊状态：非 admin 角色 403 拦截。

**响应式规则**
- 移动 (≤700px): 分类树折叠，管理导航折叠为顶部 Tab。
- 平板 (≤1100px): 树形列表自适应。
- PC (>1100px): 左侧管理导航 228px + 右侧分类管理区自适应。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /admin/agent-config Agent 管理

**Key Constraints**
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 仅 admin 可见，但不依赖 `agent.web_agent_enabled=true`；管理员必须能在功能关闭时配置 Provider、执行连接/评测检查并查看“尚未开放”状态。该页面本身不得提供绕过发布门直接开启生产功能的快捷操作。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：当前生效配置卡片 + LLM 配置表格

**核心组件清单**
- `Header`
- `AdminNav`
- `ActiveConfigCard`
- `LLMConfigTable`
- `LLMConfigModal`（新建/编辑 LLM 配置弹窗）
- `ConfirmModal`（删除/切换生效确认）
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：1280px
- 左侧管理导航 + 右侧当前配置区 + 历史配置表

**状态变体**
- default: 生效配置卡片 + 配置列表表格 + 新建按钮。
- loading: 骨架屏（Skeleton）。
- empty: 无配置时 EmptyState + 新建 CTA。
- error: Toast 右上角报错。
- 特殊状态：非 admin 角色 403 拦截；`agent.web_agent_enabled=false` 时显示功能禁用提示。

**响应式规则**
- 移动 (≤700px): 表格水平滚动，管理导航折叠为顶部 Tab。
- 平板 (≤1100px): 表格自适应宽度。
- PC (>1100px): 左侧管理导航 228px + 右侧配置区自适应。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Component: ContentCard

**Key Constraints**
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 二创卡使用 1px border + radius-lg + elevation 1，hover 使用 `border-strong` + elevation 2；原创卡无默认边框，hover 使用浅遮罩、scale 1.05 与 elevation 2。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。
- 原创区卡片使用简化样式（无 IP 名、无标签 Badge、仅显示点赞数）。
- **封面自然比例（#87 权威，替代旧的 video→16:9 / 其他→3:4 固定裁切）**：
  - 封面比例由数据驱动：`cover_width`/`cover_height`（image = 媒体集首项尺寸；video = poster 尺寸）换算 `aspect-ratio`，`object-contain` 不裁切；无该字段时防御性默认 3:4。
  - 极端比例（`max(width/height, height/width) > 2`）按高度上限 contain（超高图内部按可用高度适配，超宽图不超过列宽），防止单卡主导瀑布流。
  - 二创卡与原创卡共用此规则；列表场景禁止强制 `object-cover` 裁切封面。

**Props 接口**
```ts
interface ContentCardProps {
  className?: string;
  item: ContentItem;           // 内容数据（必需，含 coverWidth/coverHeight）
  zone: 'original' | 'fanwork'; // 区域标识，决定渲染样式
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构 — 二创区卡片（zone='fanwork'）**
- 外层容器: `<div className="border border-border rounded-lg bg-card shadow-[var(--elevation-1)] overflow-hidden">`（无 padding，内容填满）
- 封面区: 自然比例容器（`aspect-ratio` 由 `cover_width/cover_height` 数据驱动，缺省 3:4），`object-contain` 图片填满
- 信息区: `p-3`，标题（`text-sm font-medium line-clamp-2`）→ 作者 + IP 名行（`text-xs text-fg-muted`）→ 互动数据行（`text-xs` ❤️ + 💬）→ 标签行（最多 2 个低饱和 TagBadge）
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**视觉结构 — 原创区卡片（zone='original'）**
- 外层容器: `<div className="rounded-md bg-canvas-default overflow-hidden cursor-pointer group">`（无 border，更干净的小红书风格）
- 封面区: 自然比例高度（`aspect-ratio` 由 `cover_width/cover_height` 数据驱动，缺省 3:4，`object-contain w-full`），`max-height: 400px`（极端比例限高），`overflow-hidden`
- 悬停遮罩: `<div className="absolute inset-0 bg-black/10 opacity-0 group-hover:opacity-100 transition-opacity" />`
- 封面缩放: `group-hover:scale-105 transition-transform duration-300`，并在 hover 时提升至 elevation 2。
- 信息区: `p-2`，标题（`text-sm font-medium line-clamp-2`）→ @作者（`text-xs text-fg-muted`）→ ❤️ 点赞数（`text-xs text-fg-muted`）
- 无标签 Badge、无 IP 名

**尺寸规范**
- 二创区卡片: padding 16px (p-4)，封面自然比例（缺省 3:4）
- 原创区卡片: padding 8px (p-2)，封面自然比例（缺省 3:4），最小高度 150px
- 字号: `text-sm` (14px) 标题，`text-xs` (12px) 辅助信息
- 间距: 元素间隙 4px (`gap-1`) 或 8px (`gap-2`)

**状态变体**
- default: 正常展示，原创区无外边框
- hover: 原创区封面微放大 scale-105 + 浅色遮罩；二创区 border 颜色加深
- active: `scale-[0.98]`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 显示 `SkeletonCard` 占位（灰色块模拟卡片形状）
- empty: 不渲染（由父级 MasonryGrid 处理 EmptyState）

**响应式行为**
- 瀑布流列内自适应宽度，不单独控制断点。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 整卡可点击，打开共享 `ContentDetailOverlay`（source 由所在页面传入：`recommendation` / `zone-page` / `ip-page`）；不跳转详情页。直接访问内容 URL 时才使用完整详情页。
- Hover 触发视觉反馈（微缩放 + 遮罩）。
- 键盘行为：支持 Tab 索引切换，Enter 选中。

## Component: MasonryGrid

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。
- 支持无限滚动加载（IntersectionObserver + useSWRInfinite）。

**Props 接口**
```ts
interface MasonryGridProps {
  className?: string;
  items: ContentItem[];
  zone: 'original' | 'fanwork';
  emptyText?: string;
  isLoading?: boolean;
  isLoadingMore?: boolean;
  hasMore?: boolean;
  onLoadMore?: () => void;
}
```

**视觉结构**
- 外层容器: 最短列瀑布流容器 — 按 items 顺序逐条放入当前最短列，DOM/键盘/读屏顺序不变；窗口与图片高度变化时无显著动画地重新平衡。不得使用 CSS `columns-*`（按列优先填充会破坏返回顺序与最短列契约）。
- 卡片高度由 `ContentCard` 封面自然比例决定（`cover_width/cover_height` 数据驱动，见 ContentCard 章节）；极端比例卡片在列内限高 contain，防止单卡主导。
- 哨兵元素: `<div ref={sentinelRef} className="h-4" />` — IntersectionObserver 触发 onLoadMore
- 底部提示: 加载中显示 Spinner，已到底显示 "已经到底了" 文本

**无限滚动行为**
- 使用 IntersectionObserver 监听底部 sentinel 元素
- sentinel 进入视口 → 调用 `onLoadMore` 回调
- 父组件负责分页逻辑（useSWRInfinite），MasonryGrid 仅发出加载信号
- `hasMore=false` 时不触发加载

**尺寸规范**
- 列间距: `gap-4` (16px)
- 列数: 2 列（移动）/ 3 列（平板）/ 4 列（PC），通过列容器/Grid 实现，不依赖 CSS `columns-*` 类名
- 内容卡片由子组件 ContentCard 自行控制高度（自然比例）

**状态变体**
- default: 正常渲染内容卡片列表。
- loading: 初始加载时显示 12 个 SkeletonCard 占位。
- loadingMore: 底部显示 Spinner。
- empty: 使用 EmptyState 组件（图标 + 标题 + 说明 + CTA）。
- error: 底部显示 "加载失败，点击重试" 按钮。

**响应式行为**
- 移动 (≤700px): 2 列瀑布流（最短列布局）
- 平板 (≤1100px): 3 列瀑布流
- PC (>1100px): 4 列瀑布流
- 极端比例封面在列内限高 contain（不撑破列布局）

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 滚动触发加载更多（无需手动点击）。
- 卡片点击由 ContentCard 内部 Link 处理。

## Component: TagBadge

**Key Constraints**
- 使用预设 6 色体系 (blue/green/purple/orange/rose/sky)，颜色值参考 design-system.md 标签颜色表。
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none。
- 标签默认无边框，使用纯色背景 + 对应文字色。
- 使用 `rounded-full`、12px medium、20px 高；hover/focus 不改变 border-width 或布局尺寸。

**Props 接口**
```ts
interface TagBadgeProps {
  className?: string;
  color?: 'blue' | 'green' | 'purple' | 'orange' | 'rose' | 'sky';
  children: React.ReactNode;
  onClick?: () => void;
  onRemove?: () => void;
}
```

**视觉结构**
- 容器: `<span className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium tag-{color}">`
- 文字: 标签文本 `{label}`
- 可选的移除按钮: Lucide `X` 图标按钮，必须有本地化可访问名称；精细指针使用扩展 hit area，coarse pointer 目标至少 44px。

**尺寸规范**
- 默认: `h-5 px-2 text-xs`，圆角始终 `rounded-full`。
- 不新增 10px 字号档；需要更紧凑的业务标签时仍使用 12px 并减少水平 padding。

**颜色映射**
- blue: `bg-[#EEF2FF] text-[#4F46E5]` / dark `bg-[#6366F11A] text-[#A5B4FC]`
- green: `bg-[#ECFDF5] text-[#059669]` / dark `bg-[#0596691A] text-[#6EE7B7]`
- purple: `bg-[#F5F3FF] text-[#7C3AED]` / dark `bg-[#7C3AED1A] text-[#C4B5FD]`
- orange: `bg-[#FFFBEB] text-[#D97706]` / dark `bg-[#D977061A] text-[#FCD34D]`
- rose: `bg-[#FFF1F2] text-[#E11D48]` / dark `bg-[#E11D481A] text-[#FDA4AF]`
- sky: `bg-[#F0F9FF] text-[#0284C7]` / dark `bg-[#0284C71A] text-[#7DD3FC]`

**状态变体**
- default: 按 color 显示对应颜色组合。
- clickable: `cursor-pointer`，hover 仅轻微降低亮度，focus-visible 使用统一 2px `ring` + offset；Enter/Space 可激活。
- removable: 文字右侧显示 Lucide `X`，hover/focus 提升图标对比度，不使用 raw SVG 或文本 `✕`。
- reduced-motion 下禁用 active scale；无 loading 状态。

## Component: IPCard

**Key Constraints**
- Browse 变体采用 P-01 方案 B：规则 Grid 中 `w-full min-w-0` 填满轨道，16:10 封面，不得恢复固定 156px 宽度。
- List 变体保留现有详情/最近浏览语义；两种变体都只跳转 `/ip/[ipId]` 并记录最近访问，不改变 API 或路由。
- 使用语义 token、1px border、radius-lg、elevation 1；hover 可提升为 elevation 2，暗色信息区使用 `canvas-subtle`。

**Props 接口**
```ts
interface IPCardProps {
  data: IPCardData;
  variant?: 'browse' | 'list';
  className?: string;
}
```

**视觉结构**
- Browse：整卡 Link；封面 16:10；信息区 12px padding，名称 14px medium，类别 + 内容数与正向趋势 12px；趋势使用 Lucide `TrendingUp`，不使用文本箭头。
- List：保留 40px 缩略图、名称/类别/描述与详情入口；字号只使用 12/14px。

**尺寸规范**
- Browse 宽度完全由父级轨道决定，`w-full min-w-0`；封面 `aspect-[16/10]`。
- 标题 `text-sm`，meta `text-xs`，间距只使用 4/8/12px。

**状态变体**
- default: border + elevation 1；hover: border-strong + elevation 2 + 封面 scale 1.015；active 不产生布局位移。
- focus-visible: 2px ring + offset；reduced-motion 下取消封面缩放。
- loading/empty/error 由 IPBrowseClient 用同轨骨架和 EmptyState 组合负责。

**响应式行为**
- Browse 卡不声明列断点或固定宽度，由 `/ips` 的两列/auto-fit/auto-fill Grid 控制。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 整卡是有完整可访问名称的 Link；Tab 聚焦、Enter 跳转。点击/键盘激活都记录最近 IP。
- **最近访问（#64 决策 16-18 / #73 权威）**：匿名用户记录在本机 localStorage（`recent_ips`，去重保留 6 条，按最近访问排序）；登录后与独立 IP 访问历史模型幂等合并（重复按最新访问时间归并，服务器确认成功后清除本地记录；合并失败时保留本地，不丢失）。可见列表保持当前 6 条上限、按最近访问时间倒序。签入态历史来自账号绑定源（跨会话/跨设备一致），不依赖内容浏览历史。

## Component: IPCategoryTabs（已移除，#290）

- 组件已删除：IP 详情页重构为单页三模块 Hub 后，类目间跳转 tab 不再存在。类目/媒体类型筛选由 `Component: FilterPills` 承担（`IPShareTab` 内），模块切换由 Hub 三 tab（`IPHubClient`）承担。本节仅作历史索引，不再是实现依据；新代码禁止复活本组件。

## Component: FilterPills 筛选药丸（SP-12 U-01 新增，全站筛选形态基准）

> 覆盖文件：`frontend/components/ui/filter-pills.tsx`（U-03 从 /ips 现有实现提炼，归入 ui 原语目录）。
> 权威：形态/选中态/动效 token 以 `design/design-system.md`「筛选选择控件」为唯一 token 权威，本节为组件规格。

**Key Constraints**
- 全站筛选/类目选择控件的唯一形态：药丸 `rounded-full`、44px 触控高度（`min-h-11`）、`aria-pressed` 表达选中。
- 选中态全站唯一基准：`bg-accent-subtle` + `text-accent-emphasis` + 1px `border-accent-emphasis` + Check 图标（`h-3.5 w-3.5`）+ `font-semibold`；未选中：透明底/透明描边 + `text-muted-foreground`，hover `bg-muted` + `text-foreground`。零新 token。
- 切换交互一律**就地切换 + URL query 同步**（`router.replace`，不滚动不跳页）；禁止整页跳转式筛选。
- 操作按钮（提交/发布等）不得使用本组件形态（形状语义：矩形=操作、药丸=选择）。

**Props 接口**
```ts
interface FilterPillsProps {
  options: { value: string; label: string; count?: number }[];
  value: string;
  onChange: (value: string) => void;
  ariaLabel: string;        // 容器 nav 的 aria-label（i18n）
  className?: string;
}
```

**布局规范**
- 容器：`nav` + 横向 `overflow-x-auto`（溢出横向滚动，`scrollbar-width: none`，底部 `pb-1` 防裁切），项间 `gap-1`。
- 每项：`inline-flex min-h-11 flex-shrink-0 items-center gap-1 rounded-full border px-3 text-xs font-medium whitespace-nowrap`。
- 移动端 (375px) 保持 44px 触控高度与横向滚动，不换行堆叠。

**状态变体**
- selected：见 Key Constraints 选中态基准（aria-pressed=true）。
- default：透明底 + muted 文字。
- hover：`bg-muted` + `text-foreground`，150ms 颜色过渡。
- focus-visible：`ring-2 ring-ring`，与 Button 统一。
- disabled/loading：整组 `opacity-50` 并保持当前选中态可见。

**暗色模式适配**
- 全部经 token 自动映射（accent-subtle/accent-emphasis dark 值）；dark `--accent-emphasis` #818CF8 对暗卡片 6.34:1 ≥AA（FIX-05 裁决）。

**可访问性**
- 容器 `nav` + `aria-label`；各项 `aria-pressed`；选中不得只靠颜色（Check 图标 + 描边 + 字重三线索）。
- 键盘：Tab 逐项、Enter/Space 选中；横向滚动区域可用方向键滚动。

**i18n key namespace**
- 由接入方传入 options label（复用各页面既有类目/类型键，如 `home.*`）；本组件自身无常驻字符串，`ariaLabel` 必传。

**收敛注记**
- 既有筛选组件（ContentTypeFilter、IP 详情类目 tab 等）在 U-03/U-04 接入时收敛到本基准；收敛完成前不得新增偏离形态的筛选控件。

## Component: ContentDetail

**Key Constraints**
- 根据 `contentType` 渲染不同的内容展示器（MarkdownRenderer / SheetMusicViewer / MediaGallery 等）。
- 标题、作者信息、分类标签在一体化卡片中展示。
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none。
- **媒体集 vs 附件（#80 语义权威）**：image/video 内容的多文件渲染为 `MediaGallery`（画廊/播放器，可顺序浏览）；mod/乐谱/音频/模板/prompt 等类型的文件保持附件下载列表语义。二者渲染与上传链路完全分离。
- **收藏状态（#74 权威）**：收藏动作显示“已收藏”（收藏成员关系）或“添加到收藏集”，由同一事实源（用户活动收藏集的成员关系）派生，与 `CollectionPicker` 保持一致。

**Props 接口**
```ts
interface ContentDetailProps {
  className?: string;
  content: {
    id: number;
    title: string;
    description: string;
    contentType: string;
    category: string;
    zone: 'original' | 'fanwork';
    status: string;
    author: { id: number; username: string; avatarUrl?: string };
    tags?: { id: number; name: string }[];
    body?: string;
    media?: Array<{ id: number; url: string; type: 'image' | 'video'; width?: number; height?: number; posterUrl?: string }>; // 媒体集（有序，按 sort_order ASC, id ASC）
    attachments?: Array<{ id: number; name: string; url: string; size?: number }>; // 附件（下载列表）
    isFavorited: boolean;      // 收藏成员关系派生
    createdAt: string;
    updatedAt?: string;
    sourceOriginalId?: number;
    sourceOriginal?: { id: number; title: string; zone: 'original' };
    sourceFanworkId?: number;
    sourceFanwork?: { id: number; title: string; zone: 'fanwork' };
    seriesMemberships?: Array<{
      seriesId: number;
      seriesTitle: string;
      currentIndex: number;
      total: number;
      previous?: { id: number; title: string };
      next?: { id: number; title: string };
    }>;
  };
  isLoading?: boolean;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-lg bg-canvas-default overflow-hidden">`
- 头部区: `p-6 pb-4 border-b border-border-default`
  - 分类标签: `<TagBadge color="blue" label={category} />`
  - 标题: `<h1 className="text-2xl font-semibold text-foreground mt-2">{title}</h1>`
  - 作者行: Avatar + 用户名 + 发布时间，`text-sm text-fg-muted`
  - 可选 `SourceAttribution`: fanwork 且存在内容级来源时显示，置于作者行之后
- 内容区: `p-6`
  - Markdown 正文渲染（MarkdownRenderer）
  - 乐谱渲染（SheetMusicViewer）— 仅 `contentType='sheet_music'`
  - **媒体集（MediaGallery）** — `contentType` 为 image/video 时渲染，见 MediaGallery 章节；附件（下载列表）仅在其他内容类型出现
- 底部操作栏: `<ReactionBar />` 组件
- 正文后扩展区: `<SeriesNav />`（如有） -> `<RelatedFanworks />`（如有） -> `<RelatedContents />`（桌面/web 详情与浮层，见 RelatedContents 章节） -> `<CommentSection />`

**尺寸规范**
- 内间距: `p-6` (24px)
- 标题: `text-2xl` (24px)
- 正文: `text-base` (16px) leading-relaxed
- 分类标签: `text-xs`

**状态变体**
- default: 完整展示内容详情。
- loading: 骨架屏（标题块 + 内容块 Skeleton）。
- empty/404: 内容不存在 EmptyState。
- error: 内容被 ban 时显示对应限制 EmptyState。
- 无 hover/active/focus/disabled 状态（内容静态展示）。

**与现有组件关系**
- 媒体区 = `MediaGallery`（image/video）；附件下载列表只保留给非媒体类型。
- 收藏入口：`ReactionBar` 打开 `CollectionPicker`；“已收藏”状态由收藏成员关系派生，与 picker 行内状态同源。
- 桌面/浮层双栏与连续浏览行为在 `ContentDetailOverlay` 章节定义；本组件是浮层内部复用表面。

## Component: ContentDetailOverlay

**Key Constraints**
- 全站唯一共享内容详情浮层：推荐流、分区页、IP 详情页、Agent 引用卡片与浮层内关联内容统一复用同一组件和内容可见性/错误状态；不得各自实现一套详情弹窗（含 Agent 专属弹窗）。
- 桌面端为覆盖式详情层，移动端占满安全区域内可用视口；直接访问内容 URL 时渲染完整详情页而非强制浮层。
- 浮层承载与完整详情页一致的完整内容（正文、评论区、相关推荐、关联原创/二创、所属 IP）。
- 打开时保存背景滚动位置与触发元素；完全退出后恢复最初触发入口的滚动位置和焦点；Agent 引用入口必须恢复原会话滚动位置并聚焦引用卡片。
- 支持显式关闭、遮罩点击、Esc 和浏览器返回关闭；浮层内部点击不关闭。

**共享元素转场（#64 决策 7-10 权威，覆盖 #67 原型与 #68 接入）**
- 转场核心为手动计算的 FLIP 几何：source 矩形 = 触发卡片封面/媒体区；开启动画把该视觉锚点放大为浮层封面几何，同时外壳按同一时间线推进；关闭时仅在 source 矩形仍可测量（在视口、未 detach）时反向回归。
- source 缺失、在视口外、detached 或无法测量时，退化为居中 scale-and-fade（直接程序化打开无卡片 source 时同样使用）。
- View Transition API 是渐进增强：`document.startViewTransition` 可用时启用，不支持的环境继续走 FLIP。
- 时长与缓动：开 300ms / 关 240ms，共享缓动 `cubic-bezier(0.22,0.61,0.36,1)`；栈内层切换水平滑动 240ms 同缓动；reduced-motion 降级为 100ms 纯透明度淡化。
- 封面加载与主体同步：浮层在最终封面几何内展示媒体加载态（骨架/稳定占位）；封面加载成功才显示主体内容；加载失败显示稳定占位符后仍展示可用详情，不无限阻塞。

**导航栈（逐层返回）**
- 浮层内访问关联内容压入内部导航栈，栈深度 ≤ 5（2026-08-04 三缺口执行规划；P-01 原型为"无硬上限"，生产以此为准）。
- 返回类手势（返回按钮、Esc、浏览器后退）逐层弹出；退出类手势（右上关闭、背板点击）退出整个栈。
- 返回文案只显示来源或上一条内容标题，不显示层数或解释文本；Agent 来源显示"返回对话"。
- 每层记忆唯一内部内容滚动容器的滚动位置：弹层恢复该层记忆位置，新层回到顶部。
- 私信聊天浮层、媒体查看器与图片预览可叠加在内容详情最上层；Esc/背板/移动返回只关闭最上层，关闭后恢复下层滚动与触发点焦点。

**Props 接口**
```ts
interface ContentDetailOverlayProps {
  contentId: number;
  zone: 'original' | 'fanwork';
  source: 'recommendation' | 'zone-page' | 'ip-page' | 'agent-citation';
  open: boolean;
  onOpenChange: (open: boolean) => void;
  returnFocusRef?: React.RefObject<HTMLElement | null>;
  // #89 连续浏览：触发上下文列表与当前索引（移动端从卡片网格进入时传入）
  contextList?: Array<{ id: number; zone: 'original' | 'fanwork' }>;
  contextIndex?: number;
}
```
（浮层内关联内容的导航栈内部状态由组件持有；对外仅暴露当前层打开/关闭契约。）

**视觉结构**
- 遮罩层：固定覆盖视口，`--elevation-3` 浮层阴影 + 1px border + 设计系统浮层背景。
- 详情容器：复用 `ContentDetail`，包含独立可访问标题、顶部返回/关闭栏（位于滚动主体外）与唯一内部内容滚动区；标题下方不重复作者信息，右侧创作者栏仅头像、昵称、关注（与完整详情页一致）。
- 外壳与封面共享同一开合动画进度：开 300ms / 关 240ms，同帧同缓动 `cubic-bezier(0.22,0.61,0.36,1)`；栈内层切换水平滑动 240ms 同缓动；reduced-motion 降级为 100ms 纯透明度淡化。
- 触发卡无缩略图或不在视口时，入场降级为居中轻缩放 + 淡化。
- Agent 来源不增加专属详情外观。
- **桌面双栏（#88 权威）**：仅 image/video 内容，PC 端为左媒体右信息——媒体区（MediaGallery）高度上限 = 视口可用高度，宽度按媒体比例自适应；信息区（标题/作者/操作/正文/评论区）独立滚动；「封面与正文共享同一水平框架」的既有约束继续成立。文本型内容（article/sheet_music 等）维持单栏。
- **移动单列（#89 权威）**：移动端全屏单列——媒体全宽 contain，信息在其下滚动；媒体集内翻页，最后一项继续上滑 = 沿触发上下文列表前进到下一篇；列表到底显示「已经到底」提示；不提供「上一篇」；查看器不参与内容级切换。

**唯一滚动模型**
- 浮层打开时锁定背景 `html/body`（含滚动条宽度 padding 补偿）；`dialog` 与浮层外壳 `overflow: hidden`，只有内部内容主体滚动；顶部返回/关闭栏位于滚动主体外；每层只记忆这一滚动容器；不得以隐藏滚动条掩盖双滚动上下文。
- 桌面双栏下信息区与媒体区各自滚动，但同一时刻只有一个键盘/滚动目标；焦点与滚动位置按层记忆。

**状态变体**
- loading：结构与真实详情一致的骨架屏（媒体加载态在封面几何内）。
- default：完整公开内容详情。
- forbidden/deleted/not-found：稳定本地化 EmptyState，不暴露原始后端错误。
- closing：仅允许 opacity/transform 退场；reduced-motion 下立即或短淡出关闭。

**响应式与可访问性**
- PC：详情层不超过可用视口，内部滚动，关闭按钮始终可达；双栏下媒体与信息区均不出界。
- 平板/移动：全屏层，顶部关闭区和底部关键操作满足安全区域与 44px 触控目标；媒体/操作控件保持在边框内（#64 决策 15 权威，桌面/平板/移动皆然）。
- 使用对话框语义、焦点陷阱和背景 inert；打开聚焦标题，逐层弹出把焦点返回上层的触发链接，完全关闭后聚焦原触发卡片/引用卡片。
- URL、浏览器历史和拦截路由的最终实现以 `docs/working/2026-07-25-wayfinder-ticket-content-modal-routing.md` 的已确认结论为准（URL 形态与错误历史语义仍 open）。

**Playwright 截图检查点（#64 Testing Decision 17 权威，本轮新增）**
- `screenshots/overlay-shared-motion-open.png` / `overlay-shared-motion-close.png`：可测量 source 下开合几何。
- `screenshots/overlay-fallback-open.png`：source 不可用时居中缩放淡化。
- `screenshots/overlay-loading.png` / `overlay-cover-error.png`：加载态与封面失败占位。
- `screenshots/overlay-two-panel-desktop.png`：桌面双栏（左媒体右信息）。
- `screenshots/overlay-single-column-mobile.png`：移动单列 + 连续浏览上滑。

## Component: MediaGallery 媒体集画廊（#80/#85 权威）

**Key Constraints**
- 只渲染 image/video 内容的有序媒体集（顺序由 `sort_order ASC, id ASC` 的存储契约决定，见 #83）；其他内容类型的文件走附件下载列表，两者语义分离（术语权威：「媒体集」≠「附件」）。
- contain 不裁切：媒体按原始纵横比完整显示，禁止强制 `object-cover` 裁切（先例缺陷：详情页 `aspect-[16/9] max-h-96` 把竖图剪成横向矩形）。
- 容器几何稳定：由首项媒体决定，浏览会话内切换不跳版。
- 超高图（`height / width > 2`）限高 + 内部滚动（如容器内 `overflow-y-auto`），不撑破详情布局。

**Props 接口**
```ts
interface MediaGalleryProps {
  className?: string;
  items: Array<{ id: number; url: string; type: 'image' | 'video'; width?: number; height?: number; posterUrl?: string }>;
  initialIndex?: number;
  onOpenViewer?: (index: number) => void;   // 点击媒体进入 MediaViewer
  onReachEnd?: () => void;                   // 移动端连续浏览：#89 媒体集内最后一项继续上滑时触发
}
```

**视觉结构**
- 外层容器: `<div className="relative border border-border-default rounded-lg bg-canvas-default overflow-hidden">`，几何由首项比例确定（桌面双栏场景由 Overlay 传入高度约束）。
- 媒体项: `object-contain w-full h-full`；当前项 `aria-current="true"`，隐藏项从焦点序移除（`inert` 或 `hidden`）。
- 指示点: 底部居中低调位置指示点——透明圆 = 未浏览，实心圆 = 当前项；`aria-label` 说明「第 X 张 / 共 N 张」；不得闪烁或随切换跳动。
- 翻页: 左右按钮（`outline`/`ghost` 变体，44px 触控目标，键盘可聚焦）+ 滑动手势（触摸滑动翻页）；按钮在首项/末项时 disabled。
- 视频项: 展示 `<video controls poster={posterUrl}>`，第一帧 poster 为兜底；点击非 controls 区域也可进入查看器。

**尺寸规范**
- 容器宽度 = 可用内容宽度（与正文共享同一水平框架，不得窄于正文区）；高度由首项纵横比 + 超高限高共同决定。
- 桌面双栏（Overlay 内）：高度上限 = 视口可用高度，宽度按媒体比例自适应。
- 指示点间距 `gap-2`，直径 8px（`w-2 h-2`）。

**状态变体**
- default: 画廊展示当前媒体 + 指示点 + 翻页控件。
- loading: 当前项骨架占位，几何稳定不跳动。
- error: 单媒体失败显示稳定占位符（i18n `media.gallery.error.*`），不阻断画廊切换。
- empty: 媒体集为空时组件不渲染（历史单图/单视频内容仍可渲染单个媒体）。

**关键交互**
- 点击媒体区（视频 controls 除外）打开 `MediaViewer`（index = 当前项）。
- 滑动/按钮翻页只切换当前项，容器几何不变。
- 移动端 `onReachEnd`：媒体集最后一项继续上滑 → 触发连续浏览切换上下文列表下一篇（#89）；桌面端不触发。

**i18n key namespace**
- 建议 namespace：`media.gallery.*`（位置、上一张/下一张、错误、a11y）。

## Component: MediaViewer 全屏媒体查看器（#80/#86 权威）

**Key Constraints**
- 规范化既有「图片预览」语义：全站媒体细节浏览统一入口，可叠加在内容详情浮层最上层（层级高于 ContentDetailOverlay）。
- 支持缩放（pinch / 滚轮 / 按钮）、媒体集内翻页、外部点击 / 关闭按钮 / Esc 三种退出。
- Esc/背板只关闭最上层，关闭后恢复下层滚动与焦点（与私信浮层叠加语义一致）。
- 只做媒体浏览，不参与内容级切换（连续浏览由 Overlay 承担）。

**Props 接口**
```ts
interface MediaViewerProps {
  className?: string;
  items: Array<{ id: number; url: string; type: 'image' | 'video'; width?: number; height?: number; posterUrl?: string }>;
  index: number;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onIndexChange?: (index: number) => void;
}
```

**视觉结构**
- 全屏固定层（`fixed inset-0 z-[60]`，高于 Overlay）：深色背景（设计系统 token，遮罩层级按 Overlay 语义）。
- 当前媒体: 居中 contain，缩放时允许平移（`touch-action: none` + 滚轮/捏合缩放）。
- 顶部: 位置指示（`第 X 张 / 共 N 张`）+ 关闭按钮（右上，44px 触控目标）。
- 底部/两侧: 上一张/下一张按钮（媒体集 >1 时显示，首末 disabled）。
- 缩放控制: 放大/缩小/重置按钮（或手势 + 滚轮等价操作）。

**状态变体**
- default: 显示当前媒体，可缩放可翻页。
- zoomed: 缩放态下平移可用；Esc/背板关闭后恢复原缩放。
- error: 媒体加载失败显示稳定占位符 + 重试。
- 视频项: 进入查看器直接展示播放器（controls），缩放不适用于视频播放器区域。

**关键交互**
- 进入：点击 MediaGallery 媒体区。
- 退出三通道：背板点击、关闭按钮、Esc；`reduced-motion` 下短淡出。
- 翻页：左右按钮 + 触摸滑动；缩放态下滑动优先平移（`gesture` 优先级缩放 > 平移 > 翻页）。
- 焦点：进入聚焦关闭按钮；关闭后焦点返回触发媒体区（或 Overlay 当前层）。
- 与 Overlay 叠加：Esc/背板只关查看器；再按 Esc 才关 Overlay 层。

**i18n key namespace**
- 建议 namespace：`media.viewer.*`（位置、关闭、上一张/下一张、缩放、错误、a11y）。

## Component: SeriesNav 内容系列导航

**Key Constraints**
- 视觉必须克制，定位在 `ContentDetail` 正文/附件与评论区之间，不得喧宾夺主。
- 默认展示前 3 个 series membership；后端返回全部紧凑 membership，超出的项目进入 `更多(N)` overflow menu，不得在 normalizer 中提前截断。
- 上一篇/下一篇不可用时必须显示 disabled 状态，不渲染无效链接。
- **响应式收口（#64 决策 13 / 审计问题 6 权威）**：系列操作行在断点间允许 reflow，但任何控制（上一项/目录/下一项）都不得溢出其容器边框；所有动作满足触控目标并保持逻辑焦点顺序。

**目标与放置**
- 目标：帮助用户在同一内容系列内跳到上一项、下一项或系列目录。
- 放置：`frontend/components/content/SeriesNav.tsx`，由 `ContentDetail` 在正文和 `CommentSection` 之间渲染。
- 多个系列时使用紧凑 tabs 或 segmented control 切换，不使用大卡片组；第 4 个及以后通过可键盘访问的 overflow menu 列出。

**目录选择器（#64 决策 14 / 审计问题 5 权威）**
- 在浮层内点击「目录」打开**有界高度、可滚动、键盘可访问的章节选择器**（listbox/menu 语义），不得离开浮层上下文跳转整页。
- 选择某个章节 = 把该内容推入浮层现有导航栈（与上一项/下一项同模型），不导航到新页面。
- 独立系列路由（`/series/[id]`）与全量系列页仍保持可用，供直接 URL 与站外入口使用；浮层内目录行为不删除或重定义这些公开页面。
- 键盘：打开后焦点进入选择器；上/下方向键移动，Enter 选择入栈，Esc 关闭并返回「目录」trigger。
- 选择器高度有界（如 `max-h-72`）内部滚动；触发项标记当前章节（`aria-selected`）。

**Props 接口**
```ts
interface SeriesNavProps {
  memberships: Array<{
    seriesId: number;
    seriesTitle: string;
    currentIndex: number;
    total: number;
    previous?: { id: number; title: string };
    next?: { id: number; title: string };
  }>;
  onNavigateInOverlay?: (contentId: number) => void; // 浮层内目录选择时压栈
}
```
（`onNavigateInOverlay` 由浮层上下文注入；独立详情页不传则目录退化为指向 `/series/[id]` 的链接。）

**布局规范**
- PC (>1100px)：外层 `border border-border-default rounded-lg bg-canvas-default p-4`，单行布局：系列信息在左，上一项/目录/下一项在右；操作区最小宽度保证三按钮整组不溢出（先例缺陷：`max-w-[72%]` + 固定按钮宽造成「下一章」溢出 23px）。
- 平板 (701-1100px)：系列信息单行，导航按钮换到下一行 `grid grid-cols-3 gap-2`。
- 移动 (<=700px)：全宽单列；tabs 横向滚动；上一项/下一项按钮分两行或两列，保证标题不溢出。
- 按钮使用 `outline`/`ghost` 变体和 lucide `ChevronLeft`、`List`、`ChevronRight` 图标；文字 `text-sm`，辅助位置 `text-xs text-fg-muted`。

**状态变体**
- default：显示当前系列标题、当前位置、上一项、目录、下一项。
- loading：通常随 `ContentDetail` 骨架一起加载；独立加载时显示一条高度稳定的 skeleton。
- empty：无 memberships 时不渲染组件，不占位。
- error：membership 数据无效时不渲染对应 tab，并可在开发日志中记录；用户界面不显示破碎导航。
- success：本组件无写操作。
- disabled：当前为第一项时上一项 disabled；当前为最后一项时下一项 disabled；disabled 按钮保留说明文本和 `aria-disabled`。

**核心交互与焦点顺序**
- 焦点顺序：series tabs（如有）-> `更多(N)` trigger（如有）-> 上一项 -> 目录 -> 下一项。
- tabs 支持键盘左右切换；Enter/Space 激活 tab。
- `更多(N)` 使用 Popover/Menu 语义，N 为 `memberships.length - 3`；菜单中每个系列分别链接 `/series/:id`。打开后用上/下方向键移动，Esc 关闭并把焦点返回 trigger。
- 上一项/下一项是 `Link`，仅在目标对象有有效 `id` 和 `title` 时渲染。
- **目录**：浮层内打开章节选择器（见「目录选择器」）；无 `onNavigateInOverlay` 的独立详情页场景目录指向 `/series/[id]`。
- 所有 trigger、tab 和链接必须使用设计系统的可见 focus ring；hover/active 不能导致布局位移。

**可访问性**
- 对比度不低于 4.5:1；disabled 文案不得低于可读阈值。
- 所有导航按钮触控目标不小于 44px。
- 图标按钮必须有 `aria-label`，包含方向和目标标题或 disabled 原因。
- tabs 使用 `role="tablist"` / `role="tab"`，当前 tab 标记 `aria-selected="true"`。

**i18n key namespace**
- 建议 namespace：`series.nav.*`。
- 覆盖：`series.nav.position`、`series.nav.previous.*`、`series.nav.next.*`、`series.nav.catalog`、`series.nav.disabled.*`、`series.nav.a11y.*`、`series.nav.directory.*`（浮层内目录选择器的标题/空态/当前位置）。
- 不硬编码“上一章/下一章/目录”等可见文案；位置数字可由 translation 插值。

**与现有组件关系**
- `ContentDetail` 渲染顺序建议：`SourceAttribution`（如有）-> 正文/附件 -> `ReactionBar` -> `SeriesNav` -> `RelatedFanworks` -> `CommentSection`。
- 数据来自 `frontend/lib/content.ts` normalize 后的 `series_memberships`。
- 不依赖 `StudioLayout`；管理入口在 `/studio/series`。

**Playwright 截图检查点**
- `screenshots/community-content-series-nav-desktop.png`：中间项显示上一项/目录/下一项。
- `screenshots/community-content-series-nav-mobile.png`：移动 tabs 和 disabled 状态不溢出。
- 交互检查：第一项 disabled previous、最后项 disabled next、多系列 tabs、目录链接到 `/series/:id`。

## Component: SourceAttribution 灵感来源归因

**Key Constraints**
- 仅展示一层来源链；不得递归展示来源的来源。
- 只在 fanwork 且存在内容级来源时渲染；仅 IP 来源时不渲染此行。
- 视觉为标题元信息下方的小字链接，不能抢占内容标题或作者信息层级。

**目标与放置**
- 目标：在二创详情页轻量说明灵感来自某个原创或二创内容，并处理来源下架情况。
- 放置：`ContentDetail` 头部区标题/作者元信息之后、正文之前；由 `frontend/components/content/SourceAttribution.tsx` 渲染。
- 与 `RelatedFanworks` 配合：归因说明“来自哪里”，RelatedFanworks 展示“基于当前内容产生了什么”。

**Props 接口**
```ts
interface SourceAttributionProps {
  zone: 'original' | 'fanwork';
  sourceOriginalId?: number;
  sourceOriginal?: { id: number; title: string; zone: 'original' };
  sourceFanworkId?: number;
  sourceFanwork?: { id: number; title: string; zone: 'fanwork' };
}
```

**布局规范**
- PC/平板：单行 `inline-flex items-center gap-1 text-xs text-fg-muted`，最大宽度跟随内容头部；来源标题超长时 `line-clamp-1`。
- 移动：允许换行到两行，但不能覆盖标题或作者行；触控链接高度至少 44px，可通过 `py-2` 扩大命中区。
- 可使用 lucide `Link` 或 `GitBranch` 小图标 `w-3.5 h-3.5`，颜色随 `text-fg-muted`。

**状态变体**
- default：来源 summary 有效时渲染低权重链接。
- loading：随 `ContentDetail` 头部骨架一起加载；不单独闪烁。
- empty：original 内容、IP-only fanwork 或无来源字段时不渲染。
- error/unavailable：存在 source ID 但 summary 缺失时渲染灰色不可点击文本，使用 `sourceAttribution.unavailable` key。
- success：无写操作。
- disabled：不可用来源不聚焦为链接，使用 `aria-disabled` 或纯文本。

**核心交互与焦点顺序**
- 来源有效时，链接位于作者信息之后、正文之前的焦点顺序。
- `source_original` 链接到原创详情路由；`source_fanwork` 链接到二创内容详情路由。
- 无效来源不响应点击，不显示 hover pointer。

**可访问性**
- 链接文本与背景对比度不低于 4.5:1；使用 underline 或 hover/focus 下划线辅助识别，不只靠颜色。
- 链接触控目标不小于 44px。
- 图标 `aria-hidden="true"`；链接 `aria-label` 包含来源类型和标题。
- 不可用来源使用普通文本，不放入 Tab 顺序。

**i18n key namespace**
- 建议 namespace：`sourceAttribution.*`。
- 覆盖：`sourceAttribution.original`、`sourceAttribution.fanwork`、`sourceAttribution.unavailable`、`sourceAttribution.a11y.*`。
- 不硬编码“灵感来源/内容已下架”等文案。

**与现有组件关系**
- 数据来自 `frontend/lib/content.ts` normalize 的 `source_original` / `source_fanwork`。
- `PublishForm`/`SourceContentPicker` 负责创建来源字段；本组件只读展示。
- 必须嵌入 `ContentDetail`，不单独创建页面区块。

**Playwright 截图检查点**
- `screenshots/community-source-attribution-desktop.png`：fanwork with original source 显示轻量链接。
- `screenshots/community-source-attribution-unavailable.png`：summary 缺失时灰色不可点击文本。
- 交互检查：IP-only fanwork 不渲染、source_fanwork 链接到内容详情、键盘可聚焦有效链接。

## Component: RelatedFanworks 相关二创/衍生作品行

**Key Constraints**
- 只展示当前内容的一层相关作品：original -> related fanworks；fanwork -> derivative works。
- total 为 0 时隐藏组件，不留空白标题。
- 横向作品行最多展示 8 张卡片；更多内容通过查看全部入口进入列表页。

**目标与放置**
- 目标：在内容详情页正文后、评论前，以低视觉权重展示相关二创或衍生作品，鼓励继续浏览或开始创作。
- 放置：`frontend/components/content/RelatedFanworks.tsx`，由原创详情页和二创详情页调用。
- 对外文案中 fanwork 的下一层统一称为 `derivatives` 对应 i18n key，不使用“三创”固定文案。

**Props 接口**
```ts
interface RelatedFanworksProps {
  sourceContentId: number;
  sourceZone: 'original' | 'fanwork';
  titleKey: string;
  createHref?: string;
  viewAllHref?: string;
}
```

**布局规范**
- PC (>1100px)：区块标题行左侧为标题和数量，右侧为查看全部/开始创作；下方横向滚动行，卡片宽 `180px`，间距 `gap-3`。
- 平板 (701-1100px)：卡片宽 `160px`，标题行保持一行，动作按钮可换行到右侧下一行。
- 移动 (<=700px)：卡片宽 `148px`，横向滚动使用 3px 极简滚动条；动作入口在标题下方以低权重按钮显示。
- 外层只使用一个 `border border-border-default rounded-lg bg-canvas-default p-4` 容器，内部不得再包卡片容器。

**状态变体**
- default：加载 page=1&page_size=8 后展示横向卡片行。
- loading：标题骨架 + 横向卡片 skeleton x4，保留高度。
- empty：total=0 时不渲染。
- error：局部内联错误 + 重试按钮；不使用全页 EmptyState。
- success：无写操作；点击开始创作只是路由跳转。
- disabled：createHref 不存在或用户不可发布时隐藏或 disabled 创作入口；查看全部仅在 `total > 8` 时显示。

**核心交互与焦点顺序**
- 焦点顺序：区块标题后的查看全部 -> 开始创作 -> 横向卡片链接。
- 横向区域支持键盘 Tab 访问每张卡片；不要求自定义左右键滚动，但不能造成焦点不可见。
- original 的开始创作链接指向 `/studio/publish/fanwork?source_original_id=<id>`；fanwork 的衍生创作可指向 `source_fanwork_id` 预填。
- 卡片使用 `ContentCard` 小号变体：封面、标题、作者、点赞数；缺少有效 `id/title/zone` 的数据不得渲染为可点击卡片。

**可访问性**
- 标题、链接、按钮对比度不低于 4.5:1；横向滚动不隐藏焦点环。
- 卡片和操作按钮触控目标不小于 44px。
- 横向列表使用 `aria-label` 区分 related fanworks 与 derivative works。
- 查看全部/开始创作图标按钮必须有可读文本或 `aria-label`。

**i18n key namespace**
- 建议 namespace：`relatedFanworks.*`。
- 覆盖：`relatedFanworks.original.*`、`relatedFanworks.derivatives.*`、`relatedFanworks.actions.*`、`relatedFanworks.error.*`、`relatedFanworks.a11y.*`。
- 不硬编码“相关二创/衍生作品/查看全部/开始创作”等文案。

**与现有组件关系**
- 嵌入 `ContentDetail` 评论区上方；与 `SeriesNav` 同级，不互相嵌套。
- 卡片复用 `ContentCard` 小号变体和 content normalize 规则。
- `PublishForm` 通过 `SourceContentPicker` 预填 source 字段；本组件只提供入口 href。

**Playwright 截图检查点**
- `screenshots/community-related-fanworks-desktop.png`：原创详情页有 8 张以内相关二创、查看全部和开始创作入口。
- `screenshots/community-derivatives-mobile.png`：二创详情页衍生作品横向滚动，不出现“三创”硬编码文案。
- 交互检查：total=0 隐藏、total>8 显示查看全部、创建链接 query 正确、无效卡片不渲染。

## Component: RelatedContents 相关内容块（#80/#90 权威）

**Key Constraints**
- 桌面/web 端内容浏览完成后的发现延续：正文与评论区之后展示，不做自动加载下一篇（「下一篇」语义明确排除）。
- 数据来源 = 关联原创/二创（source-linkage #96 交付的最终 `RelatedFanworks`/related-fanworks 合同）+ 相似内容（固定复用列表 API：同 zone、同 `content_type`、同 category，fanwork 有 IP 时再带 `ip_id`、`sort=hot&page_size=12`）。
- 不新增临时 similar endpoint；不把带筛选的 `sort=recommended` 误称为推荐管线。
- 客户端去除当前内容与关联行重复项后最多显示 8 条。
- 移动端不渲染本块（移动连续浏览由 Overlay 承担）；列表到底时显示「已经到底了」提示（到底提示是全站瀑布流统一语义）。

**Props 接口**
```ts
interface RelatedContentsProps {
  className?: string;
  contentId: number;
  zone: 'original' | 'fanwork';
  contentType: string;
  category: string;
  ipId?: number;
  relatedFanworks?: Array<{ id: number; title: string; zone: 'original' | 'fanwork' }>; // #96 合同
}
```

**视觉结构**
- 外层容器: 复用 `RelatedFanworks` 的横向卡片行契约（卡片宽 148~180px、`gap-3`、外层单容器 `border border-border-default rounded-lg bg-canvas-default p-4`）；关联区块标题行（「相关创作」）与相似区块标题行（「你可能也喜欢」）分离。
- 底部到底提示: 与 `MasonryGrid` 一致，显示「已经到底了」i18n 文案，无更多数据时不再请求。

**状态变体**
- default: 关联行（如有）+ 相似内容行（最多 8 条）。
- loading: 区块标题骨架 + 横向卡片 skeleton x4，保留高度。
- empty: 两行都无数据时渲染到底提示（不渲染空块标题）。
- error: 局部内联错误 + 重试；不使用全页 EmptyState。
- disabled: 卡片缺有效 `id/title/zone` 不渲染为可点击卡片。

**核心交互与焦点顺序**
- 焦点顺序：相关创作标题 -> 卡片链接 -> 相似内容标题 -> 卡片链接 -> 到底提示。
- 卡片点击打开共享 `ContentDetailOverlay`（source=`zone-page`，压入当前浮层导航栈）。

**可访问性**
- 对比度与触控目标遵循 RelatedFanworks 同级规则；标题与卡片链接可读文本齐全。

**i18n key namespace**
- 建议 namespace：`media.related.*`（相关创作标题、相似内容标题、到底提示、错误、a11y）。

**与现有组件关系**
- 嵌入 `ContentDetail` 评论区之后（`ContentDetail` 扩展区顺序：`SeriesNav` -> `RelatedFanworks` -> `RelatedContents` -> `CommentSection` 前的到底提示由本块承载）。
- #96 先交付最终 related-fanworks 合同后，#90 组装本块；不得在实现时临时发明数据源。

**Playwright 截图检查点**
- `screenshots/media-related-contents-desktop.png`：正文与评论区之后的相关创作 + 相似内容行。
- `screenshots/media-related-contents-end.png`：「已经到底了」提示。
- 交互检查：去重后 ≤8 条、无临时 endpoint、卡片打开共享浮层。

## Component: MarkdownRenderer

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface MarkdownRendererProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: SheetMusicViewer

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface SheetMusicViewerProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: PRCard

**Key Constraints**
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface PRCardProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。
- 破坏性或协同操作必须包含 ConfirmModal 二次确认弹窗。

## Component: DiffViewer

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface DiffViewerProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: ReactionBar

**Key Constraints**
- 信誉分 < 3 用户：点赞/点踩/收藏按钮 disabled，hover tooltip 提示「信誉分不足」。
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，使用 1px border 容器。
- 内容类型为 mod/prompt 时显示「一键部署」按钮（需 `agent_enabled=true`）。
- **查看者反应契约（#64 决策 21-24 / 审计问题 11 权威）**：
  - 读取响应分离公开聚合（`likes`/`dislikes` 计数）与当前登录用户的 `viewer_reaction`（`'like' | 'dislike' | null`）；匿名请求恒为 `null`，绝不把其他用户的反应暴露为当前查看者状态。
  - 持久化继续使用既有 reactions 模型，唯一身份 `(user_id, target_type, target_id)`；重复当前反应 = 取消（回到中性），选择相反反应 = 原子更新（切换时计数与状态不会出现不可能的组合）。
  - 点赞/点踩互斥；刷新后状态由 `viewer_reaction` 稳定回显。
- **收藏语义（#64 决策 25-26 权威）**：`isFavorited` 定义为收藏成员关系——内容属于当前用户至少一个活动收藏集即为“已收藏”；从最后一个收藏集移除后回到未收藏；成员关系由 `CollectionPicker` 与详情动作共享同一事实源。

**Props 接口**
```ts
interface ReactionBarProps {
  className?: string;
  contentId: number;
  contentType: string;
  likes: number;           // 公开聚合计数
  dislikes: number;        // 公开聚合计数
  favorites: number;
  downloads: number;
  viewerReaction?: 'like' | 'dislike' | null;  // 查看者反应（匿名/无记录为 null）
  isFavorited: boolean;    // 收藏成员关系派生
  agentEnabled?: boolean;
  isDownloadable?: boolean;
  isDisabled?: boolean;
  disableReason?: string;
  onLike: () => void;
  onDislike: () => void;
  onFavorite: () => void;
  onShare?: () => void;
}
```

**视觉结构**
- 外层容器: `<div className="flex items-center gap-1 border border-border-default rounded-md bg-canvas-default px-3 py-2">`
- 操作按钮组: 水平排列 `gap-1`，每个操作包含图标 + 计数文字
  - 点赞按钮: `<button className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm hover:bg-canvas-subtle transition-colors">` — `Icon.thumbsUp` + 计数
  - 点踩按钮: 类似点赞，可与点赞互斥
  - 收藏按钮: `<button>` — `Icon.heart` + `isFavorited ? 'text-destructive fill-current' : 'text-fg-muted'` + 计数
  - 分享按钮: `<button>` — `Icon.share` 无计数
  - 下载按钮: `<DownloadButton />` — 只有 `isDownloadable` 为 true 时渲染
  - 一键部署按钮: 只有 `agentEnabled && contentType IN ('mod','prompt')` 时渲染

**颜色规范**
- 未激活: `text-fg-muted hover:text-foreground`
- 已点赞: `text-accent-emphasis`
- 已收藏: `text-destructive`
- 已点踩: `text-fg-muted`（计数仍显示，无强调色）
- 收藏按钮文案：`isFavorited` 时显示「已收藏」，否则「添加到收藏集」（i18n `reactions.favorite.*`）；不得硬编码。

**状态变体**
- default: 显示所有操作按钮及其计数。
- liked: 点赞按钮 accent 色高亮，再次点击取消（回到中性）。
- disliked: 点踩按钮 muted 色，与点赞互斥；再次点击取消。
- favorited: 收藏按钮 destructive 色高亮 + 「已收藏」；再次点击打开 `CollectionPicker`（移除需在 picker 内确认）。
- disabled: `opacity-50 cursor-not-allowed` + tooltip 提示原因。
- loading: 对应按钮内嵌 Spinner 替换图标。
- anonymous: 显示公开聚合计数，`viewerReaction` 为 `null`，收藏入口点击跳转 `/login`。

**响应式行为**
- 移动 (≤700px): 按钮无文字仅图标，紧凑排列 `gap-0.5`。
- 平板+ (≥701px): 图标 + 文字显示，`gap-1`。

## Component: CommentSection

**Key Constraints**
- 信誉分 < 3 用户：发布/评论/点赞按钮 disabled，hover tooltip 提示「信誉分不足」。
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，使用 1px border 容器。
- 支持楼中楼回复（ReplyList 子组件）。

**Props 接口**
```ts
interface CommentSectionProps {
  className?: string;
  contentId: number;
  comments: Comment[];
  totalComments: number;
  currentUserId?: number;
  isLoading?: boolean;
  isDisabled?: boolean;
  disableReason?: string;
  onAddComment: (text: string) => Promise<void>;
  onDeleteComment: (commentId: number) => void;
  onLikeComment: (commentId: number) => void;
}

interface Comment {
  id: number;
  author: { id: number; username: string; avatarUrl?: string };
  text: string;
  createdAt: string;
  likeCount: number;
  isLiked?: boolean;
  replyCount?: number;
  replies?: Comment[];
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default">`
- 标题行: `<h3 className="text-base font-medium text-foreground px-4 pt-4 pb-2">` 评论 (N)
- 评论输入区（顶部）: `<div className="px-4 pb-4 border-b border-border-default">`
  - Avatar + `<textarea className="w-full border border-border-default rounded-md p-3 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-accent-emphasis" rows={3} placeholder="写下你的评论...">`
  - 底部行: 字数计数 + 「发布」按钮（primary 色）
- 评论列表: `<div className="divide-y divide-border-default">`
  - 每条评论: `px-4 py-3`
    - 头像 + 用户名 + 相对时间
    - 评论内容 `text-sm text-foreground`
    - 操作栏: 点赞按钮 + 回复按钮 + 删除按钮（仅作者可见）
    - 回复列表: 嵌套的 CommentSection（简化版，仅回复列表）
  - 加载更多: 底部「加载更多」按钮或 IntersectionObserver 触发

**尺寸规范**
- 内间距: `px-4 py-3`
- 头像: `w-8 h-8` (32px)
- 评论输入框: 最小高度 80px (3 行)
- 字号: `text-sm` (14px) 评论内容，`text-xs` (12px) 辅助信息

**状态变体**
- default: 评论列表 + 输入框。
- loading: 骨架屏评论块占位。
- empty: "暂无评论，快来抢沙发吧" EmptyState + 输入框。
- disabled: 输入框 + 点赞按钮 disabled + tooltip「信誉分不足」。
- error: 发布失败 Toast + 保留输入内容。
- deleted: 评论显示「该评论已被删除」占位文本。

## Component: VersionHistory

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface VersionHistoryProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: FileUploader

**Key Constraints**
- 必须读取 config.yaml 的文件大小限制（视频≤300MB, 图片≤20MB, 文本≤10MB等）。
- 破坏性操作（如放弃上传或退出）需弹 ConfirmModal。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。
- **媒体集上传编排（#80 决策 / #84 权威）**：image 内容 = 纯图片集 2~9 张；video 内容 = 纯视频集 1~3 个；数量上下限是运行时配置，由 public config 暴露安全值并前端消费（后端为权威校验方，前端只消费合同）。不允许图文混排媒体集；纵横比可混。其他内容类型（article/sheet_music/mod/audio/template/prompt）保持附件语义。

**Props 接口**
```ts
interface FileUploaderProps {
  className?: string;
  mode: 'media-gallery' | 'attachment';   // 媒体集 vs 附件语义
  contentType: 'image' | 'video' | string;
  maxCount?: number;                        // 媒体集数量上限（public config）
  minCount?: number;                        // 媒体集数量下限（public config）
  value?: UploadItem[];
  onChange?: (items: UploadItem[]) => void;
  isBusy?: boolean;
  disabled?: boolean;
  error?: string;
}

interface UploadItem {
  id: string;              // 临时 key（发布前）
  file: File;
  type: 'image' | 'video';
  width?: number;          // image: naturalWidth；video: 首帧尺寸
  height?: number;
  sortOrder: number;       // 从 0 开始，随附件提交；拖动排序后重排
  posterUrl?: string;      // video: 首帧截帧或自定义封面（不属于媒体集）
  previewUrl?: string;
  status: 'pending' | 'uploading' | 'done' | 'error';
}
```

**视觉结构（媒体集模式）**
- 外层容器: `<div className="border border-border-default rounded-lg bg-canvas-default p-4">`
- 上传入口: 点击或拖拽多选（`multiple`）；视频上传后自动从首帧（约 0.1s）经 `<video>` + canvas 截帧导出 JPEG，走既有 oss-token 流程上传为独立 poster 文件；提供「上传自定义封面」覆盖入口；发布表单注明「未上传封面将取视频第一帧作为封面」。
- 九宫格预览: 3 列网格（移动 2 列），每格缩略图 `aspect-square` + `object-contain`；第一格带「封面」标记（首图即封面，image 内容无独立「设为封面」入口）；格子右上移除按钮（可访问名称 + 44px 触控目标）。
- 拖拽排序: HTML5 DnD 在格子间交换；拖动中源格透明、目标格显示占位；排序即时重写 `sortOrder` 并刷新首格封面标记。
- 移除: 移除任意项；低于数量下限或超出上限时表单级提示（i18n `publish.media.*`）。
- 提交前宽高采集: 图片 `naturalWidth/Height`、视频首帧尺寸随附件提交写入既有 `width/height` 字段；服务端拒绝负数、重复顺序、非正宽高与媒体类型不匹配。

**状态变体**
- default: 九宫格缩略图 + 排序 + 移除可用。
- uploading: 对应格内嵌进度/Spinner，禁止移除正在上传项。
- error: 单格红色边框 + 内联错误；整体失败用局部 EmptyState。
- disabled: `opacity-50 cursor-not-allowed`（无发布权限/内容不可上传）。
- empty: 空态提示拖拽或点击选择文件。

**响应式行为**
- PC/平板: 3 列九宫格；拖拽排序可用。
- 移动: 2 列网格；拖拽排序不保证（保留移除与首格标记），键盘/按钮化排序为可选增强。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击/拖拽触发多选上传；预览生成后进入九宫格。
- 键盘行为：移除按钮可 Tab 聚焦、Enter/Space 触发；格子顺序变更通过重新排序后的焦点位置声明。
- 媒体集数量、类型、宽高与 poster 的权威校验在后端发布链路（#83），前端只做合同化提示。

## Component: ExamQuestion

**Key Constraints**
- 赛博判官业务规则：只有具有对应类型的判官权限（judge_qualifications）或通过考核才能操作。
- 信誉分必须 >= 3 才能行使众裁权利，否则禁用功能。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface ExamQuestionProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: ReviewCard

**Key Constraints**
- 赛博判官业务规则：只有具有对应类型的判官权限（judge_qualifications）或通过考核才能操作。
- 信誉分必须 >= 3 才能行使众裁权利，否则禁用功能。
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface ReviewCardProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: EmptyState

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none。
- 居中展示，图标 + 标题 + 说明 + 可选 CTA，不留空白区域。
- 无边框、无阴影容器，使用纯文本 + 图标布局。

**Props 接口**
```ts
interface EmptyStateProps {
  className?: string;
  icon?: React.ReactNode;
  title: string;
  description?: string;
  action?: {
    label: string;
    href?: string;
    onClick?: () => void;
  };
}
```

**视觉结构**
- 外层容器: `<div className="flex flex-col items-center justify-center px-4 py-20 text-center md:py-24">`
- 图标区: 56×56 `rounded-full bg-accent-subtle text-accent-emphasis` 柔底圆形，内部图标 24×24。
- 标题: `<h3 className="mt-4 text-base font-medium text-foreground">{title}</h3>`
- 说明: `<p className="mt-2 max-w-sm text-sm text-fg-muted">{description}</p>`
- CTA: 可选 `<div className="mt-4">`，内部动作必须复用 Button 规范。

**尺寸规范**
- 图标柔底: 56×56，内部图标 24×24
- 标题: `text-base` (16px) 加粗
- 说明: `text-sm` (14px)
- CTA 高度按 Button 规范；独立移动动作至少 44px 触控目标

**状态变体**
- default: 图标 + 标题 + 说明 + 可选 CTA。
- 无 hover/active/disabled/loading 状态（静态展示，CTA 按钮遵循 Button 规范）。

## Component: ConfirmModal

**Key Constraints**
- Modal 使用 elevation 3 并保留 1px border。
- 遮罩层 `bg-black/50 backdrop-blur-sm`。
- ESC 和点击遮罩关闭；busy 状态下阻止关闭，保留 U-02A 的 focus trap 与焦点恢复。

**Props 接口**
```ts
interface ConfirmModalProps {
  className?: string;
  isOpen: boolean;
  onClose: () => void;
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  variant?: 'default' | 'destructive';
  isLoading?: boolean;
  onConfirm: () => void;
}
```

**视觉结构**
- 遮罩: `<div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm" />`
- Modal 容器: `<div className="fixed inset-0 z-50 flex items-center justify-center p-4">`
- 卡片: `<div className="w-full max-w-md rounded-lg border border-border bg-card p-6 shadow-md">`，其中 `shadow-md` 映射到 elevation 3。
- 标题: 20px semibold；消息为 14px muted。
- 消息: `<p className="text-sm text-fg-muted mb-6">{message}</p>`
- 按钮行: `<div className="flex justify-end gap-3">`
  - 取消按钮: `<button className="px-4 py-2 text-sm font-medium text-foreground bg-canvas-default border border-border-default rounded-md hover:bg-canvas-subtle">`
  - 确认按钮: variant 为 destructive 时使用 `bg-destructive text-primary-foreground`，default 使用 `bg-primary text-primary-foreground`

**尺寸规范**
- Modal 最大宽度: 448px (`max-w-md`)，移动端保留 16px viewport gutter
- 内间距: `p-6` (24px)
- 按钮遵循 Button 规范；coarse pointer 目标至少 44px

**状态变体**
- default: 白色背景 + 确认/取消按钮。
- destructive: 确认按钮使用危险色 (`bg-destructive`)。
- loading: 确认按钮内嵌 Spinner，取消按钮 disabled。
- 遮罩点击: 关闭 Modal。
- ESC: 关闭 Modal。

## Page: /agent Agent 工作台

**Key Constraints**
- 受保护路由；只有 config 中 `agent.web_agent_enabled=true` 且用户已登录并满足现有邮箱验证要求时可进入。
- Agent 是顶部导航进入的独立全页工作台。不得在 Root Layout 挂载全站右下角聊天入口；现有 `AgentChatWidget.tsx` 仅是待迁移的旧实现名。
- 遵守全局 Indigo 三档层级规则：静置面板 `--elevation-1`，抽屉/搜索浮层 `--elevation-3`，阴影永远配合 1px border，不单独承担分隔；视觉方向在已批准的 P-01 决策范围内实现。
- 唯一滚动模型：工作台固定为 Header 下方剩余视口高度，外壳与会话侧栏/主列不滚动，仅对话正文区域纵向滚动。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface AgentWorkspaceProps {
  initialConversationId?: number;
  onCitationOpen?: (citation: AgentCitation) => void;
}

interface AgentCitation {
  contentId: number;
  title: string;
  zone: 'original' | 'fanwork';
  excerpt?: string;
}

interface AgentToolStatus {
  name: 'search_content' | 'get_content_detail' | 'get_usage_guide' | 'suggest_publish_metadata';
  status: 'running' | 'success' | 'failed';
  label: string;
}
```

**视觉结构**
- 页面外层：受保护的全高工作区，位于共享 Header 下方。
- 桌面布局：会话侧栏 + 主对话区；主区包含标题/会话动作、消息列表、工具状态、引用列表和固定输入区。
- 会话侧栏（A1.6 契约）：展开态自上而下为 折叠按钮 → 搜索触发框 → 全宽"开启新对话" → 会话历史（保留非空时间分组）；可收为 56–64px 窄栏并持久化（localStorage），收起态保留 展开/搜索/新对话 三图标和 Tooltip。
- 移动布局：单列全高页面，会话导航折叠即关闭抽屉，顶部标题与底部输入区保持可达。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 页面高度：`calc(100dvh - var(--header-h))`，外壳与主列不滚动，仅对话正文区域纵向滚动。
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- empty：展开后显示能力说明、隐私提示、建议问题和输入框。
- streaming：回答增量渲染，显示停止按钮；可视文本持续更新，但 `aria-live="polite"` 按完整短句/节流批次播报，不能逐 token 打断读屏。
- tool-running：显示短状态，例如“正在检索内容”，不得展示原始参数或 chain-of-thought。
- grounded-success：回答下方展示引用列表；站内事实回答至少一个有效引用。
- no-evidence：显示“未找到足够依据”和普通关键词搜索 CTA，不伪造回答。
- degraded：Provider 不可用但搜索可用时，显示降级说明和搜索结果，不渲染模型总结。
- stopped：保留已收到的内容并显示“已停止生成”；允许重新发送。
- error：显示稳定的本地化错误与重试；保留用户输入和上一份成功回答。
- disabled：请求中仅禁用会产生冲突的动作，停止和关闭仍可用。

**会话全文搜索（A1.6 契约）**
- 搜索全部未删除会话的标题与消息正文，不受侧栏加载范围限制；每条命中独立按时间倒序显示 来源/片段/日期，点击精确定位并短暂高亮。
- 桌面 `min(720px,92vw)`、高度 ≤ 80dvh、视口居中；移动全屏。
- 含快捷键、键盘导航、空态（不重复提供清空按钮）与焦点恢复；浮层内 Esc 先清空查询词，再次 Esc 关闭搜索层并恢复触发焦点。
- 生产数据来源为 owner-scoped 会话搜索 API（由 Web Agent Productization 实现）；P-01 原型仅以 mock 验证交互。

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- Enter 发送，Shift+Enter 换行；流式时发送按钮切换为停止。
- 引用必须是站内有效 `id/title/zone` 形成的可聚焦 Agent 引用卡片；点击后打开共享 `ContentDetailOverlay`，无效引用只显示不可点击 fallback 或直接丢弃；详情浮层关闭后恢复原会话滚动位置并把焦点返回引用卡片。
- 工具状态只展示名称、用户友好结果和耗时摘要；不展示 system prompt、工具 JSON 或内部推理。
- Esc 只关闭当前打开的内容详情浮层、会话搜索层或会话抽屉，不离开 Agent 工作台。
- “开始新对话”不删除旧会话且无需确认；“清空当前历史”使用 `ConfirmModal` 并调用 owner-scoped `DELETE /api/v1/agent/conversations/:id`。取消不发请求；成功聚焦新输入框；失败保留当前消息并把焦点返回 trigger。该删除不会同时删除服务器脱敏 trace、审计或聚合用量记录。
- 仅当用户停留在消息底部附近时自动跟随流式内容；用户向上阅读后停止抢滚动，并显示可聚焦的“跳到最新”按钮。
- 所有可交互元素使用设计系统可见 focus ring；动画遵守 `prefers-reduced-motion`，流式文本本身不使用逐 token 位移动画。

**可访问性与响应式**
- PC：全页双栏工作区；会话栏可收起，主对话列保持可读行宽，引用卡片不挤压消息正文。
- 移动：占满安全区域内可用宽高，顶部固定标题/会话入口，底部固定输入区。
- 所有按钮触控目标不少于 44px；引用关系不能只靠颜色表达。
- 对话列表使用语义列表；流式内容不在每个 token 到达时抢焦点。
- 在 320/375/414/768/1024/1440px 检查无横向溢出；长标题、URL 和错误码必须可换行或截断且保留可访问名称。

**i18n key namespace**
- `agent.chat.*`、`agent.tools.*`、`agent.citations.*`、`agent.noEvidence.*`、`agent.degraded.*`、`agent.errors.*`、`agent.a11y.*`。

**Playwright 截图检查点**
- `screenshots/web-agent-grounded-desktop.png`
- `screenshots/web-agent-citations-mobile.png`
- `screenshots/web-agent-citation-overlay-desktop.png`
- `screenshots/web-agent-no-evidence.png`
- `screenshots/web-agent-degraded-search.png`

## Component: UploadAssistPanel

**Key Constraints**
- 必须读取 config.yaml 的文件大小限制（视频≤300MB, 图片≤20MB, 文本≤10MB等）。
- 破坏性操作（如放弃上传或退出）需弹 ConfirmModal。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface UploadAssistResult {
  suggestedTitle?: string;
  suggestedDescription?: string;
  suggestedTags?: string[];
  suggestedCategory?: string;
}

interface AgentUploadAssistPanelProps {
  uploadedFiles: string[];
  contentId?: number;
  title?: string;
  description?: string;
  contentType?: string;
  className?: string;
  onFill?: (result: UploadAssistResult) => void;
  applyDisabled?: boolean;
  applyDisabledReason?: string;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: ComplianceCheckBadge

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface ComplianceResult {
  riskLevel: 'safe' | 'warning' | 'violation';
  reason?: string;
  suggestions?: string[];
  details?: Record<string, unknown>;
}

interface AgentComplianceCheckBadgeProps {
  contentId?: number;
  title?: string;
  description?: string;
  contentType?: string;
  className?: string;
  onResult?: (result: ComplianceResult) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: UsageGuidePanel

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface UsageGuidePanelProps {
  contentId: number;
  className?: string;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: GlobalSearchInput

**Key Constraints**
- 搜索输入本体始终提供关键词搜索、建议、历史和热搜，不显示 Agent 模式切换或 Agent 专属状态。
- 自然语言问答和受控协助通过顶部导航进入 `/agent`；不得在搜索框内复刻 Agent 工作台。
- 现有 `SearchAgentInput.tsx` 是待迁移的旧实现名；实现任务可重命名为 `GlobalSearchInput.tsx`。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface GlobalSearchInputProps {
  className?: string;
  value: string;
  onValueChange: (value: string) => void;
  onSubmit: (query: string) => void;
  onCancel?: () => void;
  isLoading?: boolean;
  disabled?: boolean;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- keyword：默认可靠模式，调用普通搜索。
- loading：输入保留，按钮显示进度并提供取消。
- error：局部错误 + 重试；不清空 query。
- disabled：保持原因说明，不能只降低透明度。

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- Enter 提交关键词搜索；Esc 关闭建议或取消当前建议请求。
- 搜索建议和热搜均进入普通搜索结果页；不得把查询静默转交 Agent Provider。
- 输入框具有可见 label 或等价可访问名称；提交、取消均有可见 focus ring，状态不能只靠颜色表达。

**i18n key namespace**
- `search.input.*`、`search.suggestions.*`、`search.history.*`、`search.a11y.*`。

## Component: FollowButton

**Key Constraints**
- FollowButton 未登录时：点击跳转 `/login`，不显示已关注状态。
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none。
- **状态契约（#64 决策 20 / 审计问题 10 权威）**：未关注是显眼主操作（primary 实底）；已关注是克制的 outline 态；hover/focus 已关注时显示「取消关注」。实现不得与本节相反（先例：2026-08-07 审计实测 `variant={following ? "default" : "outline"}` 与本节颠倒）。

**Props 接口**
```ts
interface FollowButtonProps {
  className?: string;
  userId: number;
  isFollowing: boolean;
  followerCount?: number;
  showCount?: boolean;
  isLoading?: boolean;
  size?: 'sm' | 'md';
  onToggle: (userId: number, newState: boolean) => void;
}
```

**视觉结构**
- 按钮容器: `<button className="inline-flex items-center justify-center gap-1.5 rounded-md text-sm font-medium transition-colors">`
- 未关注态: `bg-primary text-primary-foreground px-4 py-2 hover:opacity-90` — 显示 "+ 关注"
- 已关注态: `border border-border-default bg-canvas-default text-foreground px-4 py-2 hover:bg-canvas-subtle hover:text-destructive hover:border-destructive` — 显示 "已关注"（hover/focus 显示 "取消关注"）

**尺寸规范**
- md: `h-9 px-4 py-2 text-sm`（默认）
- sm: `h-7 px-3 py-1 text-xs`

**状态变体**
- default: 未关注时 primary 色按钮；已关注时 outline 按钮。
- hover 未关注: `opacity-90` 加深。
- hover/focus 已关注: 背景变 subtle + 边框/文字变 destructive 色（提示取消关注），文本切换为「取消关注」。
- loading: 按钮内嵌 Spinner，文字变为处理中。
- disabled: `opacity-50 cursor-not-allowed`（信誉分不足或未登录）。
- 未登录: 点击跳转 `/login`，无 loading 状态。

## Component: NotificationDropdown

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface NotificationDropdownProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: NotificationList

**Key Constraints**
- 用于 `/messages` 左栏通知时间线；不是 Header 下拉通知组件。
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，列表容器和列表项均使用 1px border / divider，不使用 shadow。
- `channel === "broadcast"` 的系统广播必须同时使用文本、图标或 aria-label 与普通通知区分，不得只靠蓝色边框。
- Markdown 正文摘要必须走安全 Markdown 渲染/摘要链路，不渲染原始 HTML。

**Props 接口**
```ts
type NotificationChannel = 'reply' | 'like' | 'system' | 'pr' | 'follow' | 'broadcast';

interface NotificationListItem {
  id: number;
  type: string;
  channel: NotificationChannel;
  title: string;
  body?: string;
  is_read: boolean;
  target_type?: string;
  target_id?: number;
  created_at: string;
  sender?: {
    id: number;
    username: string;
    avatar_url?: string;
  };
}

interface NotificationListProps {
  className?: string;
  notifications: NotificationListItem[];
  selectedId?: number;
  isLoading?: boolean;
  error?: string;
  onSelect?: (notification: NotificationListItem) => void;
  onMarkRead?: (notificationId: number) => void;
  onRetry?: () => void;
}
```

**视觉结构**
- 外层容器是纵向列表，不包二层卡片；列表项之间使用 `border-b border-border-default` 或 `divide-y`。
- 每项结构：左侧 channel icon / broadcast marker，中央标题、摘要、时间，右侧 unread dot 或 mark-read icon button。
- 广播项：左边框 `border-l-2 border-accent-emphasis`，标题前显示 localized `messages.broadcast.label`，摘要最多两行。
- 普通通知：无左强调边框；未读项使用更高字重和 unread dot。

**尺寸规范**
- 列表项最小高度 `72px`，`px-3 py-3`，触控目标不小于 44px。
- 标题 `text-sm font-medium`，摘要和时间 `text-xs text-fg-muted`。
- unread dot 固定 `w-2 h-2`，不得改变行高。

**状态变体**
- default: 按 `created_at` 倒序展示通知。
- unread: 标题加粗，显示 unread dot；已读项不降低到不可读对比度。
- selected: 当前详情在右栏展示时，列表项使用 tokenized accent border 或背景。
- broadcast: 见视觉结构；广播摘要可展开到右栏详情，但列表内仍保持两行。
- loading: skeleton 列表 x6，保持列表宽度不变。
- empty: `EmptyState`，文案来自 `messages.notifications.empty.*`。
- error: 局部错误 + retry button；若父页面已有旧数据，保留旧数据并在顶部显示轻量错误提示。

**响应式行为**
- PC/平板：在 `/messages` 左栏内占满宽度。
- 移动：作为单栏列表显示；点击通知进入详情视图并提供返回。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击/Enter/Space 打开详情或跳转目标；缺少有效 target 的通知只打开详情，不创建断链。
- 标记已读 icon button 必须有 `aria-label`，不能吞掉列表项点击事件。
- 所有可见文案走 `messages.notifications.*` / `messages.broadcast.*`。

## Component: ConversationList

**Key Constraints**
- 用于 `/messages` 私信 Tab 的会话列表；数据来源为 `GET /api/v1/messages`，不得再调用旧 `/api/v1/conversations`。
- 遵守扁平 1px divider 设计；头像、未读数、最后消息摘要必须有稳定尺寸，避免 unread 状态导致布局跳动。
- typed message 只显示安全摘要，不在会话列表展开 `CollabInviteCard`。

**Props 接口**
```ts
interface ConversationParticipant {
  id: number;
  username: string;
  avatar_url?: string;
}

interface ConversationSummary {
  id: number;
  participants: ConversationParticipant[];
  last_message?: {
    id: number;
    text?: string;
    body?: string;
    msg_type?: 'text' | 'collab_invite';
    created_at: string;
    sender_id: number;
  };
  unread_count: number;
  updated_at: string;
}

interface ConversationListProps {
  className?: string;
  conversations: ConversationSummary[];
  selectedConversationId?: number;
  isLoading?: boolean;
  error?: string;
  onSelect: (conversation: ConversationSummary) => void;
  onRetry?: () => void;
}
```

**视觉结构**
- 外层为纵向列表，`divide-y divide-border-default`。
- 每项：Avatar 固定 40px，中央用户名/最后消息/时间，右侧 unread badge。
- 多 participant 时显示第一个对方用户名；MVP 只支持 1:1，会话为空 participant 时显示安全 fallback。
- `msg_type='collab_invite'` 摘要使用 localized `messages.conversations.collabInviteSummary`。

**尺寸规范**
- 列表项最小高度 `64px`，`px-3 py-3`。
- Avatar `w-10 h-10`，unread badge 最小 `min-w-5 h-5`。
- 最后消息摘要 `line-clamp-1`，不得撑宽左栏。

**状态变体**
- default: 会话按 `updated_at` 倒序。
- selected: 当前会话使用 `bg-canvas-subtle` 和左侧 accent marker。
- unread: unread badge 显示数量，超过 99 显示 `99+`。
- loading: skeleton 会话 x6。
- empty: localized EmptyState，可引导从用户主页/内容页发起私信。
- error: 局部错误 + retry button。

**响应式行为**
- PC/平板：在左栏固定宽度内渲染。
- 移动：点击会话进入 `ChatWindow` 单栏详情，列表隐藏；返回按钮由父页面或 ChatWindow header 提供。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击/Enter/Space 选择会话；不得用 hover-only 控件承载核心操作。
- `onSelect` 收到完整 conversation summary，`ChatWindow` 通过 participants 推导 recipient_id。
- 所有可见文案走 `messages.conversations.*`。

## Component: ChatWindow

**Key Constraints**
- 用于 `/messages` 私信详情；读取 `GET /api/v1/messages/:id`，发送 `POST /api/v1/messages`。
- 正常 text message 渲染为消息气泡；`msg_type='collab_invite'` 渲染 `CollabInviteCard`；未知 typed message 显示安全 fallback。
- 输入框固定在窗口底部，但不得遮挡消息列表；移动端使用正常文档流或 sticky 区域。
- 不渲染原始 metadata；只把经过 normalizer 验证的邀请字段传给子组件。

**Props 接口**
```ts
interface ChatMessage {
  id: number;
  sender_id: number;
  text?: string;
  body?: string;
  msg_type?: 'text' | 'collab_invite';
  metadata?: {
    invite_id?: number;
    content_id?: number;
    content_title?: string;
    inviter_id?: number;
    inviter_username?: string;
  };
  created_at: string;
}

interface ChatWindowProps {
  className?: string;
  conversation: ConversationSummary;
  messages: ChatMessage[];
  currentUserId: number;
  isLoading?: boolean;
  isSending?: boolean;
  error?: string;
  onSend: (payload: { recipient_id: number; text: string }) => Promise<void> | void;
  onBack?: () => void;
  onRetry?: () => void;
}
```

**视觉结构**
- 外层为 `flex flex-col min-h-0 border border-border-default rounded-md bg-canvas-default`。
- Header：对方头像/用户名，移动端可显示返回 icon button。
- 消息列表：`flex-1 overflow-y-auto px-4 py-3 space-y-3`，`aria-live="polite"`。
- 本人 text 气泡靠右，使用 primary/accent subtle；对方 text 气泡靠左，使用 canvas-subtle。
- 输入区：`border-t border-border-default p-3`，textarea/input + send icon button。

**尺寸规范**
- PC 消息气泡最大宽度 `min(70%, 620px)`；移动端最大宽度 `86%`。
- 输入框最小高度 44px；发送按钮触控目标 44px。
- 邀请卡片宽度跟随消息列，不超过 text 气泡最大宽度。

**状态变体**
- default: 渲染已有 messages，初次进入滚动到底部。
- loading: header skeleton + 消息气泡 skeleton x6，输入区 disabled。
- empty: 显示 localized 空对话提示，输入区仍可用。
- sending: 发送按钮 disabled 并显示 spinner；输入内容保留直到请求成功。
- error: 局部重试按钮；发送失败用 Toast，不抢焦点。
- cold-start-error: 后端返回 `DM_REPLY_REQUIRED` 时显示 localized Toast，不清空输入。
- typed-unknown: 显示 `messages.chat.unsupportedMessage`，不显示 metadata。

**响应式行为**
- PC/平板：填满右栏高度，`min-width: 0`，长文本换行。
- 移动：单栏全宽，header 返回按钮可见；输入区不遮挡系统导航。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- Enter 发送、Shift+Enter 换行；空白内容禁用发送。
- 发送 payload 使用 `{ recipient_id, text }`，recipient 从当前会话 participant 中排除 currentUserId 后得到。
- 新消息到达时，如果用户接近底部则自动滚到底；用户正在查看旧消息时不强制跳动。
- 所有可见文案走 `messages.chat.*`、`messages.error.*`、`messages.a11y.*`。

## Component: CollabInviteCard 联合创作邀请卡片

**Key Constraints**
- 仅在 `ChatWindow` 渲染 `message.msg_type === "collab_invite"` 时使用；普通 text message 继续使用消息气泡。
- 必须清楚区分 `pending`、`accepted`、`declined`、`expired`，但不得设计成广告横幅或高饱和 CTA。
- 邀请最终状态以后端 invite DTO 为准，前端本地状态只做乐观/即时更新。

**目标与放置**
- 目标：让被邀请者在私信里理解邀请来源、关联内容，并接受或拒绝联合创作。
- 放置：`frontend/components/social/CollabInviteCard.tsx`，嵌入 `ChatWindow` 消息流；`ConversationList` 只显示简短摘要，不展开卡片。
- 与 `CollabUserPicker` 关系：Picker 负责发送邀请，InviteCard 负责接收方响应。

**Props 接口**
```ts
interface CollabInviteCardProps {
  invite: {
    id: number;
    status: 'pending' | 'accepted' | 'declined' | 'expired';
    contentId: number;
    contentTitle: string;
    inviterUsername: string;
  };
  isCurrentUserInvitee: boolean;
  onAccept: (inviteId: number) => Promise<void>;
  onDecline: (inviteId: number) => Promise<void>;
}
```

**布局规范**
- PC (>1100px)：卡片最大宽度 `420px`，位于消息列内，与对方消息左对齐；`border border-border-default rounded-lg bg-canvas-default p-4`。
- 平板 (701-1100px)：最大宽度 `min(420px, 80%)`，按钮保持一行。
- 移动 (<=700px)：宽度 `100%`，按钮可两列或纵向；每个按钮高度至少 44px。
- 内容结构：小号类型行 -> 邀请描述 -> 内容标题链接/摘要 -> 状态说明 -> 操作区。
- 状态标识使用低饱和 TagBadge 语义：pending blue、accepted green、declined rose/neutral、expired muted；必须配合文本。

**状态变体**
- pending：接收方看到接受/拒绝按钮；非接收方只看到等待状态。
- accepted：只读成功状态，按钮隐藏；可显示内容链接。
- declined：只读拒绝状态，按钮隐藏。
- expired：灰色只读状态，按钮隐藏，整体 `bg-canvas-subtle` 但文字仍可读。
- loading：点击接受/拒绝后，仅对应按钮显示 Spinner，另一按钮 disabled。
- error：响应失败时卡片内显示短错误 + Toast；卡片状态回到操作前。
- disabled：用户不是 invitee、状态非 pending、请求中或账号受限时操作 disabled。
- empty/invalid：metadata 缺少 invite_id/content_id/content_title 时不渲染操作卡，回退为安全文本消息摘要。

**核心交互与焦点顺序**
- 焦点顺序：内容链接 -> 接受按钮 -> 拒绝按钮；非 pending 状态只有内容链接可聚焦。
- 接受：调用 `/api/v1/collab-invites/:id/accept`，成功后状态变为 returned invite.status。
- 拒绝：调用 `/api/v1/collab-invites/:id/decline`，成功后状态变为 returned invite.status。
- 不在卡片内放二次确认；接受/拒绝是可明确理解的小范围操作。
- 键盘：Enter/Space 触发按钮；焦点环必须在消息流中可见。

**可访问性**
- 状态文本与背景对比度不低于 4.5:1；不能只靠 TagBadge 颜色表达状态。
- 操作按钮触控目标不小于 44px。
- 接受/拒绝按钮 `aria-label` 包含内容标题；loading 时设置 `aria-busy="true"`。
- 卡片容器可使用 `role="group"` 和 `aria-labelledby` 指向邀请标题。

**i18n key namespace**
- 建议 namespace：`collabInviteCard.*`。
- 覆盖：`collabInviteCard.status.*`、`collabInviteCard.actions.*`、`collabInviteCard.errors.*`、`collabInviteCard.summary.*`、`collabInviteCard.a11y.*`。
- 不硬编码 pending/accepted/declined/expired 的可见文案。

**与现有组件关系**
- `ChatWindow` 根据 msg_type 分支渲染本组件；消息列表布局、滚动和输入框仍归 ChatWindow 管理。
- `ConversationList` 对 typed message 只显示 i18n 摘要和状态，不嵌入完整卡片。
- 接受后 contributor 状态由后端写入；`ContentDetail` 后续可显示贡献者，但本卡不直接修改内容详情状态。

**Playwright 截图检查点**
- `screenshots/community-collab-invite-pending.png`：pending 卡片含接受/拒绝，视觉不像广告横幅。
- `screenshots/community-collab-invite-states.png`：accepted、declined、expired 三种只读状态可区分。
- `screenshots/community-collab-invite-mobile.png`：移动端按钮 44px，文本不溢出。
- 交互检查：接受成功、拒绝成功、过期不可操作、API 失败回退、普通消息不受影响。

## Component: CourseCard

**Key Constraints**
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface CourseCardProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: CourseContent

**Key Constraints**
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface CourseContentProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: ReputationDetail

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface ReputationDetailProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: DiscussionCard

**Key Constraints**
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface DiscussionCardProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: ReplyList

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface ReplyListProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: VerdictDetail

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface VerdictDetailProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: LLMConfigTable

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface LLMConfigTableProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: LLMConfigModal

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface LLMConfigModalProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: ActiveConfigCard

**Key Constraints**
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface ActiveConfigCardProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: UserProfileCard

**Key Constraints**
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface UserProfileCardProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: FollowerListModal

**Key Constraints**
- FollowButton 未登录时：点击跳转 `/login`，不显示已关注状态。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface FollowerListModalProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: CreatorSupportPanel

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface CreatorSupportPanelProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: JudgeQualBadge

**Key Constraints**
- 赛博判官业务规则：只有具有对应类型的判官权限（judge_qualifications）或通过考核才能操作。
- 信誉分必须 >= 3 才能行使众裁权利，否则禁用功能。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface JudgeQualBadgeProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: `hover:bg-canvas-subtle` 并伴随图标颜色变深
- active: `active:bg-canvas-subtle scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent-emphasis`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border-destructive` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。



## Component: StudioSidebar

**Key Constraints**
- 创作者工作室的可折叠侧边栏，必须在 `/studio/*` 布局中使用。
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 展开宽度 228px (`w-[228px]`)，收起宽度 48px (`w-12`)，过渡动画 `transition-all duration-200`。
- 收起态：仅显示图标，hover 图标时在右侧弹出 tooltip（`absolute left-full ml-2`，延迟 300ms）。
**Props 接口**
```ts
interface StudioSidebarProps {
  className?: string;
  collapsed: boolean;
  onToggle: () => void;
  currentPath: string;
  sections: SidebarSection[];
}

interface SidebarSection {
  id: string;
  title: string;
  items: SidebarItem[];
}

interface SidebarItem {
  key: string;
  icon: React.ReactNode;
  label: string;
  href: string;
  badge?: number;
}
```

**视觉结构**
- 外层容器: `<aside className="h-full flex flex-col bg-canvas-subtle border-r border-border transition-all duration-200" style={{ width: collapsed ? 48 : 228 }}>`
- 顶部分组区：Logo/标题 + 折叠切换按钮（`→` / `←` 箭头图标）
- 中间导航区：按 section 分组渲染
  - 展开态：分组标题 (`text-xs text-fg-muted uppercase tracking-wider px-3 pt-4 pb-1`) + 项列表
  - 收起态：仅图标，分组间 `border-t border-border my-2` 分隔
- 每个导航项：
  - 展开态：`h-10 px-3 flex items-center gap-3 rounded-md`（图标 20px + 文字 `text-sm` + 可选未读 Badge）
  - 收起态：`h-12 flex items-center justify-center relative group`（仅图标居中 + hover tooltip）
  - 选中态：`bg-accent-emphasis/10 text-accent-emphasis font-medium` + 左侧 3px accent 色竖线（`border-l-3 border-accent-emphasis`）
  - 未选中态：`text-fg-muted hover:bg-canvas-default hover:text-foreground`

**Tooltip 规范**（收起态 hover）
- 触发：`opacity-0 group-hover:opacity-100 transition delay-300`
- 定位：`absolute left-full ml-2 top-1/2 -translate-y-1/2`
- 样式：`bg-canvas-default border border-border rounded-md px-3 py-1.5 text-sm text-foreground whitespace-nowrap shadow-md z-50`

**状态变体**
- default: 默认展开态，当前路由对应项高亮选中。
- collapsed: 收起态，仅图标可见。
- hover: 未选中项 hover 背景变亮，收起态触发 tooltip。
- active: 选中项 accent 色左侧指示器。
- disabled: `opacity-50 cursor-not-allowed`（如收益数据 P1 占位，点击不跳转）。
**响应式行为**
- 移动 (≤700px): 侧边栏默认收起，点击展开为浮层覆盖主内容区（`absolute z-20 h-full`），点击主内容区自动收起。
- 平板+ (≥701px): 侧边栏固定，展开/收起按钮可见。
**关键交互**
- 折叠按钮点击 → `onToggle()` → 父组件更新 `collapsed` 状态，写入 `localStorage: studio_sidebar_collapsed`。
- 收起态图标 hover 300ms → tooltip 弹出，鼠标移出 → tooltip 消失。
- 键盘：`Tab` 在图标间移动，`Enter` 导航到目标路由。

## Component: ContentTypeGrid

**Key Constraints**
- 内容类型选择卡片网格，用于 `/studio/publish/*` 发布流程步骤 1。
- 卡片排列从 `config.yaml > publish.type_order_original` 或 `publish.type_order_fanwork` 读取。
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
**Props 接口**
```ts
interface ContentTypeGridProps {
  className?: string;
  zone: 'original' | 'fanwork';
  types: ContentTypeOption[];
  onSelect: (type: ContentTypeOption) => void;
}

interface ContentTypeOption {
  contentType: string;
  icon: string;
  label: string;
  description: string;
}
```

**视觉结构**
- 外层容器: `<div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 p-6">`
- 每张卡片: `<button className="border border-border rounded-lg p-6 text-center hover:border-accent-emphasis hover:bg-canvas-subtle transition-all cursor-pointer group hover:-translate-y-1">`
  - 图标: `<span className="text-4xl mb-3 block">`（emoji 图标 40px）
  - 标题: `<h3 className="text-base font-medium text-foreground mb-1">`
  - 描述: `<p className="text-xs text-fg-muted">`

**状态变体**
- default: 白色卡片 + 1px border。
- hover: border accent 色 + 轻微上浮 `-translate-y-1` + 背景变浅。
- active: `scale-95` 点击反馈。
**响应式行为**
- 移动 (≤700px): 2 列 (`grid-cols-2`)，卡片 padding `p-4`。
- 平板 (≤1100px): 3 列 (`grid-cols-3`)。
- PC (>1100px): 4 列 (`grid-cols-4`)，卡片 padding `p-6`。
**关键交互**
- 点击卡片 → `onSelect(type)` → 父组件切换到发布表单（步骤 2），zone + content_type 锁定。
- 键盘：`Tab` 在卡片间移动，`Enter` 选中。
- 发布表单顶部提供「← 返回选择类型」按钮。

## Component: SourceContentPicker 来源内容选择器

**Key Constraints**
- Canonical 组件名为 `SourceContentPicker`；旧原创来源选择器和 fanwork 专用来源选择器命名应收敛为此组件的不同 `sourceKind` 配置。
- 仅用于 fanwork 发布/编辑流程；original zone 表单不得渲染此组件。
- 同一表单中 `source_original_id` 与 `source_fanwork_id` 互斥，选择一类来源必须清除另一类来源。

**目标与放置**
- 目标：让发布二创时可选择原创来源或二创来源，并支持 URL query 预填来源摘要。
- 放置：`PublishForm` 中 IP 选择器之后、标签/权限之前；如果同时存在 IP，来源仍可选但两个来源 ID 不可共存。
- 与 `SourceAttribution` 关系：本组件写入来源字段，`SourceAttribution` 在详情页只读展示。

**Props 接口**
```ts
interface SourceContentPickerProps {
  sourceKind: 'original' | 'fanwork';
  selected?: { id: number; title: string; zone: 'original' | 'fanwork' };
  disabled?: boolean;
  onSelect: (content?: { id: number; title: string; zone: 'original' | 'fanwork' }) => void;
}
```

**布局规范**
- PC (>1100px)：表单行内区域宽度跟随 PublishForm；搜索输入 + 结果 Popover，已选来源以紧凑 summary row 展示。
- 平板 (701-1100px)：输入和已选 summary 单列；Popover 宽度等于输入。
- 移动 (<=700px)：结果使用底部 Sheet 或全宽下拉，避免窄屏 Popover 溢出；已选 summary 保持 44px 高度。
- 结果项展示封面缩略图（可选）、标题、作者、zone badge，使用 1px border 或列表分隔线，不使用嵌套卡片。

**状态变体**
- default：输入可搜索，未选择时显示 placeholder 文案 key。
- loading：搜索中在输入右侧显示 Spinner，结果区域 skeleton x3。
- empty：无搜索结果显示局部 EmptyState。
- error：搜索失败显示内联错误和重试按钮；不清除已选来源。
- success：选择成功后 summary row 展示来源标题和清除按钮；清除按钮调用 `onSelect(undefined)`。
- disabled：PublishForm 提交中、zone 非 fanwork、用户无发布权限时输入和清除按钮 disabled。
- prefilled：URL query 带 `source_original_id` 或 `source_fanwork_id` 时先加载 summary；加载失败显示 unavailable 内联错误并允许清除。

**核心交互与焦点顺序**
- 焦点顺序：source kind label -> 搜索输入 -> 结果项 -> 已选 summary 清除按钮 -> 下一个表单字段。
- 输入 debounce 搜索；original 调用 original 内容搜索，fanwork 调用 fanwork 内容搜索；结果必须只包含 published 内容。
- 选择 original 后调用父级清除 fanwork；选择 fanwork 后清除 original；清除当前选择时调用 `onSelect(undefined)`。
- 键盘：输入聚焦后 `ArrowDown` 进入结果，`Enter` 选择，Esc 关闭结果层，清除按钮可 Tab 聚焦。

**可访问性**
- 输入有 `<label>` 和 `aria-describedby`，说明 fanwork 至少需要 IP 或来源之一。
- 结果列表使用 combobox/listbox 语义或等价 aria；当前高亮项用 `aria-activedescendant`。
- 所有结果项和清除按钮触控目标不小于 44px。
- 文本对比度不低于 4.5:1；错误提示不只靠红色，需有文本说明。

**i18n key namespace**
- 建议 namespace：`sourceContentPicker.*`。
- 覆盖：`sourceContentPicker.original.*`、`sourceContentPicker.fanwork.*`、`sourceContentPicker.search.*`、`sourceContentPicker.prefill.*`、`sourceContentPicker.error.*`、`sourceContentPicker.a11y.*`。
- 不硬编码“来源原创/来源二创/搜索/清除”等文案。

**与现有组件关系**
- 嵌入 `PublishForm`；不得单独提交内容，只通过父表单写入 `source_original_id` 或 `source_fanwork_id`。
- 与 `ContentTypeGrid` 无直接交互，ContentTypeGrid 只选择内容类型。
- 与 `RelatedFanworks` 的开始创作链接配合，支持 query 预填来源。

**Playwright 截图检查点**
- `screenshots/community-source-picker-desktop.png`：PublishForm 中 original/fanwork 来源选择、互斥清除。
- `screenshots/community-source-picker-mobile.png`：移动 Sheet 结果列表不溢出。
- 交互检查：IP-only 可提交、original-source 可提交、fanwork-source 可提交、无 IP/来源禁用提交、两个来源 ID 不会同时进入 payload。

## Component: CollabUserPicker 联合创作者选择器

**Key Constraints**
- 用于发布成功后发送联合创作邀请；不能把被选用户提前写入内容作者或 contributor 列表。
- 搜索结果只显示安全用户字段：id、username、avatar_url。
- 选择器不得绕过后端防骚扰链路；前端只做重复选择和基础可用性提示。

**目标与放置**
- 目标：让创作者在 `PublishForm` 中选择一个或多个待邀请用户，并在内容创建成功后逐个发送邀请。
- 放置：`PublishForm` 的主要内容字段之后、提交操作之前；source-linkage 字段之后再显示。
- 与 `ChatWindow`/`CollabInviteCard` 关系：本组件触发邀请发送，真正响应发生在私信卡片中。

**Props 接口**
```ts
interface CollabUserPickerProps {
  selectedUsers: Array<{ id: number; username: string; avatarUrl?: string }>;
  maxSelected: number;
  disabled?: boolean;
  onChange: (users: Array<{ id: number; username: string; avatarUrl?: string }>) => void;
}
```

`maxSelected` comes from public config `collaboration.max_invitees_per_publish`; show a localized limit message when reached. The backend remains authoritative and must reject over-limit requests even if the prop is stale or bypassed.

**布局规范**
- PC (>1100px)：表单内单列；已选用户以紧凑 chips 列表展示，搜索输入下方弹出结果 Popover。
- 平板 (701-1100px)：chips 自动换行，结果 Popover 等宽。
- 移动 (<=700px)：搜索结果使用全宽下拉或底部 Sheet；chips 每行可换行，移除按钮保持 44px 命中区。
- 不使用大头像墙；头像 24-32px，信息密度与 PublishForm 其他字段一致。

**状态变体**
- default：搜索输入可用，已选用户 chips 可移除。
- loading：搜索中显示 Spinner，结果 skeleton x3。
- empty：无结果显示局部 EmptyState。
- error：搜索失败显示内联错误 + 重试，不清空已选。
- success：发布后邀请发送成功可由父级 Toast 汇总；本组件不单独显示成功 banner。
- disabled：提交中、用户信誉不足、内容创建失败、发布权限不足时禁用搜索和移除。
- duplicate：重复用户不可添加，结果项显示 disabled 状态并说明已选择。

**核心交互与焦点顺序**
- 焦点顺序：字段 label -> 已选 chips 的移除按钮 -> 搜索输入 -> 结果项 -> 下一个表单字段。
- 输入 username 搜索；点击/Enter 选择；选择后输入清空并保持焦点。
- chips 移除按钮可键盘触发；本轮不实现 Backspace 删除最后一个 chip，避免新增未测试键盘路径。
- 发布成功后父级按 selectedUsers 逐个调用邀请 API；单个邀请失败只产生 warning toast，不回滚内容发布。

**可访问性**
- 输入使用 combobox/listbox 语义或等价 aria；结果项有 `aria-selected`。
- chips 移除按钮必须有 `aria-label`，包含用户名。
- 所有结果项、chips、移除按钮触控目标不小于 44px。
- 对比度不低于 4.5:1；duplicate/disabled 状态不只靠颜色。

**i18n key namespace**
- 建议 namespace：`collabUserPicker.*`。
- 覆盖：`collabUserPicker.label`、`collabUserPicker.search.*`、`collabUserPicker.selected.*`、`collabUserPicker.error.*`、`collabUserPicker.disabled.*`、`collabUserPicker.a11y.*`。
- 不硬编码“搜索用户/已选择/移除/邀请失败”等文案。

**与现有组件关系**
- 嵌入 `PublishForm`；不参与 `ContentDetail` 展示。
- 邀请发送结果最终在 `ChatWindow` 中以 `CollabInviteCard` 展示。
- 与 `SourceContentPicker` 同属 PublishForm 扩展字段，视觉密度和表单 label 规则保持一致。

**Playwright 截图检查点**
- `screenshots/community-collab-picker-desktop.png`：PublishForm 中搜索结果、已选 chips、重复 disabled。
- `screenshots/community-collab-picker-mobile.png`：移动结果层不溢出，chips 可移除。
- 交互检查：搜索用户、选择、重复不可加、移除、发布成功后邀请失败 warning 不阻断发布成功。

## Page: /studio 创作者工作室

**Key Constraints**
- 所有 `/studio/*` 子页面共享 `StudioLayout`（侧边栏 + 主内容区）。
- 需要登录，未登录 redirect `/login`。
- 参考 B 站创作中心和 Reddit 创作者后台设计。
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none。
**视觉层级**
- 顶部：Header `h-[var(--header-h)]`，底边框 `border-b border-border`
- 主容器：`flex h-[calc(100vh-52px)]`（Header 下方全高），背景 `bg-canvas-subtle`
- 左侧：StudioSidebar（展开 `w-[228px]` / 收起 `w-12`），右侧 1px border 分隔
- 右侧：主内容区 `flex-1 overflow-y-auto`，padding `p-6`

**核心组件清单**
- `Header`
- `StudioLayout`（共享布局容器）
- `StudioSidebar`（可折叠侧边栏）
- `ContentTypeGrid`（内容类型选择）
- `FileUploader`, `MarkdownEditor`（复用）
- `FollowerTrendChart`, `FollowerSourceChart`（粉丝分析图表）

**布局规范**
- 侧边栏 + 主内容区水平排列，`flex` 布局
- 主内容区垂直滚动（`overflow-y-auto`），无外层 card 包裹

**状态变体**
- default: 默认进入 `/studio/publish/original`（ContentTypeGrid）。
- loading: 主内容区骨架屏。
- empty: 无内容/无数据时对应 EmptyState。
- error: Toast 右上角报错。
- 特殊状态：信誉分 < 3 时发布入口 disabled + tooltip「信誉分不足，暂时无法发布」。
**响应式规则**
- 移动 (≤700px): 侧边栏默认收起（48px），点击展开为浮层覆盖；主内容区 `p-4`。
- 平板 (≤1100px): 侧边栏展开，主内容区自适应。
- PC (>1100px): 侧边栏展开 228px，概览/内容管理 max-w 1280px，发布表单 max-w 960px。
**交互细节**
- 侧边栏折叠状态持久化到 `localStorage: studio_sidebar_collapsed`。
- 发布流程：类型选择 → 发布表单（同页状态切换，不改 URL）。
- 发布成功后跳转 `/studio/contents`。
- 数据加载：SWR 客户端加载概览统计、粉丝趋势等个性化数据。

## Page: /studio/publish/original 发布原创

**Key Constraints**
- 步骤 1 显示 ContentTypeGrid（原创区 7 种类型，按 config.yaml > publish.type_order_original 排序）。
- 步骤 2 显示发布表单，zone='original' + content_type 锁定，category 必填。
- 二创专属字段（ip_id、source_original_id）不渲染。
- 破坏性操作需 ConfirmModal。
- **媒体集发布约束（#80/#84 权威）**：image = 纯图片集 2~9 张；video = 纯视频集 1~3 个；数量/类型/宽高/顺序与 poster 的权威校验在后端发布链路（#83），前端只消费 public config 合同并做提示；「第一张即封面」（image），视频缺省取第一帧为封面并可在上传区上传自定义封面。
**视觉层级**
- 步骤 1：居中标题「选择原创内容类型」+ ContentTypeGrid 网格
- 步骤 2：标题「发布原创 — [类型名]」+ 发布表单（max-w 960px 居中）
**核心组件清单**
- `ContentTypeGrid`
- `FileUploader`, `MarkdownEditor`, `TagInput`
- `ComplianceCheckBadge`, `UploadAssistPanel`
- `ConfirmModal`（放弃确认）

**布局规范**
- 发布表单垂直排列：标题 → 描述/Markdown → 文件上传 → 分类选择 → 标签 → 权限开关 → 提交按钮

**状态变体**
- default: 步骤 1 类型选择网格。
- form: 步骤 2 发布表单。
- loading: 提交按钮内嵌 Spinner。
- error: 行内红字 + Toast。
- 特殊状态：信誉分不足显示拦截提示。
**响应式规则**
- 移动 (≤700px): 表单全宽 `p-4`，编辑器高度 250px。
- 平板 (≤1100px): 表单 max-w 720px 居中。
- PC (>1100px): 表单 max-w 960px 居中。
**交互细节**
- 步骤 1 → 2：`onSelect(type)` 切换状态。
- 步骤 2 → 1：顶部「← 返回选择类型」按钮（有未保存内容时弹 ConfirmModal）。
- 发布成功 → Toast + redirect `/studio/contents`。

## Page: /studio/publish/fanwork 发布二创

**Key Constraints**
- 类似发布原创，ContentTypeGrid 显示二创区 8 种类型（按 config.yaml > publish.type_order_fanwork 排序）。
- 发布表单比原创区多 fanwork 来源选择区；`ip_id`、`source_original_id`、`source_fanwork_id` 三者至少填写一个。
- `source_original_id` 与 `source_fanwork_id` 互斥；选择任一内容来源时必须清除另一个内容来源。
- 本计划阶段中 `SourceContentPicker` 负责来源字段；`CollabUserPicker` 由后续 collaboration-invites 计划追加，必须位于来源字段之后、提交按钮之前。
- **媒体集发布约束**：与发布原创一致（见 `/studio/publish/original` 章节），image/video 走媒体集编排（FileUploader media-gallery 模式），其他类型维持附件语义。

**目标与放置**
- 目标：让创作者发布二创时明确选择内容类型，并在 IP、原创来源、二创来源三类灵感来源中至少提供一类。
- 放置：`frontend/app/(protected)/studio/publish/fanwork/page.tsx`，使用 `StudioLayout`；未登录和封禁状态由 protected route group 兜底。
- Query prefill：`?source_original_id=<id>` 预填原创来源，`?source_fanwork_id=<id>` 预填二创来源；两者同时存在时前端保留 `source_original_id`、清除 `source_fanwork_id` 并显示 localized warning。

**核心组件清单**
- `ContentTypeGrid`
- `IPSelector`（IP 搜索下拉）
- `SourceContentPicker`（原创来源/二创来源统一选择器）
- `CollabUserPicker`（future/additive，由 collaboration-invites 计划实现）
- `FileUploader`
- `MarkdownEditor`
- `TagInput`
- `ComplianceCheckBadge`
- `ConfirmModal`（放弃未保存内容）
- 其余与发布原创相同

**布局规范**
- 同发布原创，额外插入来源选择区；来源选择区字段之间使用 `space-y-3`，不要卡片套卡片。
- PC (>1100px)：表单最大宽度 `960px`；来源区是普通表单 fieldset，不做独立浮动卡片。
- 平板 (701-1100px)：表单最大宽度 `720px`；来源选择器结果层等宽。
- 移动 (<=700px)：表单 `p-4`，来源选择器结果使用底部 Sheet 或全宽下拉；提交按钮保持 44px 高度。

**状态变体**
- default：步骤 1 显示 ContentTypeGrid；选择类型后显示步骤 2 fanwork 表单。
- loading：提交中按钮内嵌 Spinner，来源选择器和内容字段 disabled。
- prefill-loading：query prefill 正在加载来源 summary，来源区显示 skeleton row。
- prefill-error：query prefill id 不存在、不可见或 zone 不匹配时显示内联 warning，可清除后继续填写。
- validation：IP、原创来源、二创来源均为空时显示来源区行内错误并禁用提交。
- error：API 错误 Toast + 字段级错误；保留用户输入。
- success：发布成功 Toast + redirect `/studio/contents`。
- disabled：信誉分不足、账号封禁、无发布权限时显示 protected EmptyState 或禁用表单。

**交互细节**
- IP、原创来源、二创来源三者均为空时提交按钮 disabled，并在来源选择区显示 i18n 校验说明。
- 选择原创来源会清除二创来源；选择二创来源会清除原创来源。
- 来源内容不存在/已删除时预填失败，显示可清除的内联错误。
- 发布成功 → Toast + redirect `/studio/contents`。

**可访问性**
- 来源区使用 `fieldset` + `legend` 或等价语义，说明“IP 或灵感来源至少填写一项”。
- `SourceContentPicker` 的两个实例 label 必须不同，分别通过 i18n 表示原创来源和二创来源。
- Query prefill warning 使用 `role="status"`；提交 validation 使用 `aria-describedby` 关联到来源区。
- 所有来源选择、清除、提交、返回按钮触控目标不小于 44px。
- 未保存内容返回类型选择时打开 `ConfirmModal`，焦点锁定，Esc 关闭。

**i18n key namespace**
- 建议 namespace：`studio.publish.fanwork.*`。
- 覆盖：`studio.publish.fanwork.typeGrid.*`、`studio.publish.fanwork.source.*`、`studio.publish.fanwork.prefill.*`、`studio.publish.fanwork.validation.*`、`studio.publish.fanwork.toast.*`、`studio.publish.fanwork.a11y.*`。
- 不在页面组件中硬编码 IP/来源/二创/预填失败/至少一项等文案。

**与现有组件关系**
- `SourceContentPicker` 是来源字段 canonical 组件；本页不得引入 fanwork 专用来源 picker。
- `CollabUserPicker` 是 collaboration-invites 计划的 additive component；source-linkage 实现不得提前发送协作邀请。
- `RelatedFanworks` 的“开始创作”入口通过 query prefill 进入本页。

**Playwright 截图检查点**
- `screenshots/community-source-picker-desktop.png`：PC 表单含 IP、原创来源、二创来源，互斥清除可见。
- `screenshots/community-source-picker-mobile.png`：移动来源选择结果层不溢出，validation 文案不遮挡提交按钮。
- 交互检查：IP-only、original-source-only、fanwork-source-only 均可提交；无 IP/来源禁用提交；query prefill 成功和失败状态均覆盖。

## Page: /studio/publish/ip 创建 IP

**Key Constraints**
- 位于 `(protected)` 路由组内，未登录由既有 auth guard 拦截；使用 `StudioLayout`（含 `StudioSidebar`）。
- 表单字段与后端 `CreateIPInput` 对齐：name（必填 1-255）、description（可选）、cover_url（单图，可选）、category（单选，来源 `ipCategoryOptions` 排除 `all`）、tags（chips 输入，最多 10 个，单 tag ≤50 字符，提交由后端规范化）。
- 封面走既有 presign 链：`POST /contents/oss-token`（file_type=image）→ PUT 上传 → 以 `oss_domain + "/" + oss_key` 组装规范裸 URL 提交；后端 CreateIP 契约不变（仍接收完整 cover_url），响应侧签名由 DisplayURLSigner 出口装饰。
- 创建成功后 IP 处于 `pending`（AI 审核 + 管理员通过后才公开），页面必须给出明确的「已提交，等待审核」反馈并提供跳转 IP 详情页入口，禁止静默成功。
- 遵守全局 Indigo 三档层级规则：本页表面 shadow-none、1px border、语义 token；所有交互目标 ≥44px。
- 双入口：`/studio` 侧边栏「内容发布」组「创建 IP」项 + `/ips` 浏览页工具栏「创建 IP」按钮（登录态可见；空态 EmptyState 提供同目标 CTA）。

**视觉层级**
- 页面标题 `text-xl font-bold` + 表单卡片：`rounded-xl border border-border bg-card`，字段垂直排列。
- 分类为 rounded-full chips 单选组（选中 `border-accent-emphasis bg-accent-subtle text-accent-emphasis`）；tags 使用 `TagBadge`（removable）。
- 成功态：内联成功面板（1px border、`bg-accent-subtle` 淡色底）显示 pending 说明与跳转按钮，不自动重定向。

**核心组件清单**
- `FileUploader`（attachment 模式，fileType=image，单图）
- `TagBadge`
- `Input`、`Button`、`Toast`
- `EmptyState`（仅 /ips 页空态复用）

**布局规范**
- 表单最大宽度 672px（max-w-2xl）居中；字段顺序：name → description → cover → category → tags → 提交按钮。
- 元素间距 16px（gap-4 / space-y-6 分区）。

**状态变体**
- default: 空表单。
- uploading: FileUploader 内嵌进度条，提交按钮不禁用但提交校验上传未完成时 toast 阻止。
- loading: 提交按钮 Spinner + 禁用。
- error: API 错误 toast + 字段级红字（name 必填/超长），保留用户输入。
- success: 内联「已提交，等待审核」面板 + 「查看 IP 详情」链接 + 「继续创建」重置按钮。
- 特殊状态：未登录访问由 protected 组重定向登录页。

**响应式规则**
- 移动 (≤700px): 表单全宽 `p-4`，chips 换行。
- 平板 (≤1100px): 表单 max-w 720px 居中。
- PC (>1100px): 表单 max-w 672px 居中。

**暗色模式适配**
- 背景 token: `canvas-default`/`card` dark 变体；边框 `border` dark 变体；文字 `foreground`/`muted-foreground` dark 变体。

**交互细节**
- name 前端校验 1-255 字符，空或超长时禁用提交并红字提示；后端 400 映射为字段错误。
- tags 输入框 Enter 添加（preventDefault）、去重、trim、上限 10 个；TagBadge X 移除。
- 提交成功 → toast + success 面板；不自动 redirect。
- 破坏性操作无；「继续创建」重置表单无需 ConfirmModal。

**可访问性**
- 分类 chips 使用 `aria-pressed`；tags 容器有 i18n label。
- 上传、提交按钮触控目标 ≥44px。

**i18n key namespace**
- `studio.publishIP.*`：title/nameLabel/namePlaceholder/nameRequired/nameTooLong/descriptionLabel/descriptionPlaceholder/coverLabel/coverHint/categoryLabel/categoryPlaceholder/tagLabel/tagPlaceholder/addTag/submit/submitting/successTitle/successPending/successView/successCreateAnother/failed。
- `studio.sidebar.createIP`、`ip.createIP`（/ips 入口）。
- 不硬编码任何用户可见文案。

**Playwright 截图检查点**
- `screenshots/b003-b001-local-20260901/`：创建表单（含分类 chips 与 tags chips）、提交成功 pending 面板、IP 详情页 TagBadge 行。

## Page: /studio/overview 数据概览

**Key Constraints**
- 参考 B 站创作中心首页：统计卡片 + 趋势图 + 排行 + 待办事项。
- 数据从多个 API 聚合。
**视觉结构**
- 统计卡片行：4 卡片（总内容/总访问/总点赞/粉丝），每卡片含环比变化百分比 + 上升/下降箭头
- 访问量趋势：近 30 天 recharts LineChart，全宽卡片
- 下方双栏：左「内容排行 Top 5」，右「待处理事项」
**核心组件清单**
- `StatsCard`（含变化百分比）
- `ViewsTrendChart`（recharts LineChart）
- `ContentRankList`（Top 5 列表）
- `PendingTasksCard`（待处理 PR/标签建议/举报数）

**布局规范**
- 统计卡片：`grid grid-cols-2 lg:grid-cols-4 gap-4`
- 趋势图卡片：全宽，高度 300px
- 下方双栏：`grid grid-cols-1 lg:grid-cols-2 gap-6`

**状态变体**
- default: 数据正常，图表完整。
- loading: 卡片骨架屏 + 图表区灰色块。
- empty: 新用户「暂无数据，快去发布第一条内容吧」EmptyState + CTA。
- error: Toast 报错 + 单卡片内联错误提示。
**响应式规则**
- 移动: 统计卡片 2 列，趋势图 100%，下方单列。
- 平板+: 统计卡片 4 列。
- PC: max-w 1280px。
**交互细节**
- 内容排行项可点击跳转详情页。
- 待处理事项项可点击跳转对应管理页。

## Page: /studio/series 内容系列管理

**Key Constraints**
- 所有 `/studio/*` 页面共享 `StudioLayout`；本页必须使用 `StudioSidebar` 的既有折叠行为和宽度 token。
- 首版只管理公开系列，不提供私有/草稿系列开关。
- 系列 owner 才能创建、编辑、删除、添加、移除和排序系列项。

**目标与放置**
- 目标：让创作者创建系列、编辑元信息、添加自己发布或已确认贡献的内容，并用上/下移动或可测试排序控件维护顺序。
- 放置：`frontend/app/(protected)/studio/series/page.tsx`，StudioSidebar 内容管理分组内新增入口。
- 与 `PublishForm` 关系：发布时不强制选择系列；系列归档是发布后的管理动作。

**核心组件清单**
- `StudioLayout`
- `StudioSidebar`
- `ContentCard`（可添加内容搜索结果的紧凑形态）
- `ConfirmModal`
- `EmptyState`
- `LoadingSpinner`
- `Toast`

**布局规范**
- PC (>1100px)：主区最大宽度 `1280px`；左侧为 series 列表栏 `320px`，右侧为详情/编辑区 `minmax(0,1fr)`；两栏同级，使用 1px border 分隔，不嵌套卡片。
- 平板 (701-1100px)：列表栏宽 `280px`，详情区自适应；添加内容搜索结果单列。
- 移动 (<=700px)：StudioSidebar 默认收起；本页采用列表/详情两步视图，选中 series 后详情全屏显示，顶部提供返回列表图标按钮。
- 详情区结构：元信息表单 -> 已添加内容有序列表 -> 添加内容搜索区。

**状态变体**
- default：左侧列出我的系列，右侧显示选中系列详情和 items。
- loading：列表和详情分别显示骨架，避免整页空白。
- empty：无系列时显示 EmptyState + 创建按钮；选中系列无 items 时显示局部 EmptyState。
- error：Toast + 局部错误；列表加载失败不显示详情区假数据。
- success：创建、保存、添加、移除、重排、删除成功均使用 Toast；保存后保持当前选中系列。
- disabled：保存中禁用表单；删除默认不可用状态不适用；无权限/信誉不足时创建和管理控件 disabled 或由受保护布局拦截。

**核心交互与焦点顺序**
- 焦点顺序：StudioSidebar -> 页面标题 -> 创建按钮 -> series 列表 -> 元信息表单 -> items 上/下移动按钮 -> 移除按钮 -> 添加内容搜索 -> 保存/删除。
- 创建 series：打开内联表单或 Modal；必填 title 和 zone，zone 创建后不可变更。
- 添加内容：搜索 owner 自己发布或 owner 已确认贡献的内容；选中后追加到末尾。
- 排序：首版优先使用上移/下移按钮；若实现拖拽，必须保留键盘排序按钮。
- 删除 series：ConfirmModal 二次确认；删除成功后返回列表第一项或 EmptyState。

**可访问性**
- 所有控件触控目标不小于 44px；排序按钮使用 lucide 图标并提供 `aria-label`。
- 列表项选中状态使用 `aria-current` 或 `aria-selected`，不能只靠颜色。
- 表单字段有 `<label>`；错误提示与字段通过 `aria-describedby` 关联。
- 对比度不低于 4.5:1；禁用态说明仍需可读。

**i18n key namespace**
- 建议 namespace：`studio.series.*`。
- 覆盖：`studio.series.list.*`、`studio.series.form.*`、`studio.series.items.*`、`studio.series.search.*`、`studio.series.confirm.*`、`studio.series.toast.*`、`studio.series.a11y.*`。
- 不硬编码“创建系列/上移/下移/删除”等文案。

**与现有组件关系**
- 必须挂在 `StudioLayout`，入口由 `StudioSidebar` 提供。
- 内容搜索结果复用内容摘要/ContentCard 视觉，不新建营销卡片。
- `SeriesNav` 在 `ContentDetail` 中消费本页创建的系列 membership；本页不渲染 `SeriesNav`。

**Playwright 截图检查点**
- `screenshots/community-content-series-studio-desktop.png`：PC 两栏，列表、详情、排序按钮、添加搜索区。
- `screenshots/community-content-series-studio-mobile.png`：移动列表/详情切换，StudioSidebar 收起。
- 交互检查：创建系列、添加两条内容、上移/下移排序 payload、移除 item、删除 series、空状态。

## Page: /studio/favorites 收藏集管理

**Key Constraints**
- 所有 `/studio/*` 页面共享 `StudioLayout`；本页必须使用 `StudioSidebar` 的既有折叠行为和宽度 token。
- 页面放在 `frontend/app/(protected)/studio/favorites/page.tsx`；未登录、封禁和信誉限制由 protected route group 与后端权限兜底。
- 收藏集按 `zone` 分区管理；原创收藏集和二创收藏集必须分开展示、创建和筛选。
- 默认收藏集 `is_default=true` 不可删除；UI 删除按钮必须 disabled，并通过 tooltip 或行内说明解释。
- 收藏集可见性、默认保护、owner 权限和内容可见性最终以后端 API 为准，前端不得拿全量数据后自行隐藏私有数据。

**目标与放置**
- 目标：让创作者管理自己的原创/二创收藏集，创建、编辑、删除普通收藏集，并快速进入收藏集详情页查看内容。
- 放置：StudioSidebar 内容管理分组内新增「收藏集」入口；页面主区不使用公共 Header。
- 数据源：`GET /api/v1/collections`（需要认证，未传 `owner_id` 时列出当前用户全部收藏集）；创建、编辑、删除分别调用对应 collections API。

**核心组件清单**
- `StudioLayout`
- `StudioSidebar`
- `CollectionCard`
- `ConfirmModal`
- `EmptyState`
- `LoadingSpinner` / `SkeletonCard`
- `Toast`

**布局规范**
- PC (>1100px)：主区最大宽度 `1280px`，顶部为标题 + 新建按钮；下方使用两个同级 zone section，分别为原创收藏集和二创收藏集，每个 section 使用 `grid grid-cols-3 gap-4`。
- 平板 (701-1100px)：主区最大宽度 `960px`，两个 zone section 单列堆叠，卡片网格 `grid-cols-2`。
- 移动 (<=700px)：主区 `p-4`，StudioSidebar 默认收起；顶部操作换行，新建按钮全宽；每个 zone section 使用 2 列紧凑网格。
- 两个 zone section 是页面同级区块，禁止把整个页面包成大卡片；`CollectionCard` 只用于单个收藏集。

**状态变体**
- default：展示原创/二创两组收藏集；默认收藏集排在各自 zone 第一项。
- loading：页面标题保留，两个 zone section 各显示 `SkeletonCard` x3，网格尺寸稳定。
- empty：某个 zone 没有普通收藏集时仍显示默认收藏集；若 API 返回空列表，显示全页 EmptyState + 创建/修复默认收藏集入口。
- error：Toast + 局部重试按钮；不得展示过期缓存中的私有收藏集作为“成功”状态。
- success：创建、编辑、删除成功均 Toast，并重新验证列表数据；删除成功后焦点返回对应 zone 的新建按钮或第一张卡片。
- disabled：默认收藏集删除 disabled；保存中禁用对应表单和按钮；非 owner 场景理论上不会进入本页，若后端返回 403 显示 EmptyState。

**核心交互与焦点顺序**
- 焦点顺序：StudioSidebar -> 页面标题 -> 新建原创按钮 -> 原创收藏集卡片 -> 新建二创按钮 -> 二创收藏集卡片。
- 新建收藏集：选择 zone 后打开 Modal 或内联表单；必填 title，description 和 is_public 可选；zone 创建后不可变更。
- 编辑收藏集：从 `CollectionCard` action 打开编辑表单；仅允许修改 title、description、is_public、sort_order。
- 删除收藏集：普通收藏集必须 ConfirmModal 二次确认；默认收藏集删除按钮不可触发请求。
- 卡片主区域点击跳转 `/collections/[id]`；owner action 按钮必须阻止冒泡，不能误触跳转。

**可访问性**
- zone section 使用明确 heading，并将网格 `aria-labelledby` 指向对应 heading。
- `CollectionCard` 主链接和编辑/删除 action 是独立可聚焦元素；键盘用户可完成创建、编辑、删除和进入详情。
- 默认收藏集删除禁用说明不能只靠颜色；删除按钮有 `aria-disabled` 或 disabled 属性。
- 所有按钮和卡片 action 触控目标不小于 44px；ConfirmModal 焦点锁定，Esc 可关闭。

**i18n key namespace**
- 建议 namespace：`studio.favorites.*`。
- 覆盖：`studio.favorites.header.*`、`studio.favorites.zone.*`、`studio.favorites.actions.*`、`studio.favorites.form.*`、`studio.favorites.default.*`、`studio.favorites.empty.*`、`studio.favorites.error.*`、`studio.favorites.toast.*`、`studio.favorites.a11y.*`。
- 不硬编码“原创收藏集/二创收藏集/默认收藏集/公开/私有/删除”等文案。

**与现有组件关系**
- 收藏集卡片复用 `CollectionCard`；不要为 Studio 管理页另建第二套收藏集卡片视觉。
- 公共详情仍由 `/collections/[id]` 负责；本页只做 owner 管理入口和跳转。
- 内容详情的添加收藏弹窗使用 `CollectionPicker`；本页不承担给当前内容添加到收藏集的弹窗。

**Playwright 截图检查点**
- `screenshots/community-collections-studio-desktop.png`：PC 两个 zone section、默认收藏集、普通收藏集 owner actions。
- `screenshots/community-collections-studio-mobile.png`：移动两列网格、新建按钮全宽、默认删除 disabled。
- 交互检查：创建 original/fanwork 收藏集、编辑名称/可见性、默认收藏集删除 disabled、普通收藏集删除 ConfirmModal、卡片跳转 `/collections/[id]`。

## Page: /studio/followers 粉丝分析

**Key Constraints**
- 新增页面。数据源：`GET /api/v1/users/:id/followers/stats?days=30`。
- 参考 B 站粉丝分析页：总数 + 趋势 + 来源。
**视觉结构**
- 统计卡片：粉丝总数 + 本月新增 + 本月流失
- 粉丝增长趋势：近 30 天双线折线图（新增 / 净增）
- 粉丝来源分布：按内容来源的饼图或横向柱状图
**核心组件清单**
- `StatsCard`
- `FollowerTrendChart`（recharts LineChart，双线）
- `FollowerSourceChart`（recharts PieChart）
**布局规范**
- 统计卡片：`grid grid-cols-3 gap-4`
- 双图表：`grid grid-cols-1 lg:grid-cols-2 gap-6`

**状态变体**
- default: 数据正常。
- loading: 骨架屏。
- empty: 「你还没有粉丝，快去发布内容吸引关注吧」EmptyState。
- error: Toast + 重试按钮。
**响应式规则**
- 移动: 卡片 1 列，图表单列。
- 平板+: 卡片 3 列，图表双栏。
- PC: max-w 1280px。

## Page: /studio/revenue 收益数据

**Key Constraints**
- P1 预留页面。`features.creator_support_enabled: false`（默认）时显示占位。
**视觉结构**
- 居中 EmptyState：图标 + 「收益功能即将开放，敬请期待」说明文字

**状态变体**
- disabled: EmptyState（MVP 默认）。
- P1 enabled: 累计收益卡片 + 趋势图 + 提现按钮。

---

<!-- 以下是 Task 99-145 优化任务新增页面规格 -->

## Page: /forgot-password 忘记密码（Task 117）

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。
- 邮件发送限流：同一邮箱 60 秒内只能发送一次重置邮件。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：邮箱输入表单卡片（带 1px border），居中展示

**核心组件清单**
- `Header`
- `LoadingSpinner`（提交时按钮内嵌）
- `EmptyState`（邮件发送成功提示）

**布局规范**
- 页面最大宽度：1280px
- 表单卡片：最大宽度 400px，垂直水平居中
- 区域间距（block）：32px (`space-y-8`)

**状态变体**
- default: 邮箱输入框 + 「发送重置链接」按钮 + 返回登录链接。
- loading: 按钮内嵌 Spinner，输入框 disabled。
- success: 邮件发送成功提示卡片（图标 + "重置链接已发送到您的邮箱" + 返回登录链接）。
- error: 行内红字错误提示（邮箱格式错误、邮箱未注册、发送过于频繁等）。
- 特殊状态：已登录用户自动跳转首页。

**响应式规则**
- 移动 (<=700px): 表单卡片全宽（margin 16px），padding 20px。
- 平板 (<=1100px): 表单卡片最大宽度 400px，居中。
- PC (>1100px): 表单卡片最大宽度 400px，居中，两侧留白。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 邮箱提交后 60 秒倒计时禁用按钮，防止重复发送。

## Page: /reset-password 重置密码（Task 117）

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。
- token 有效期 1 小时，过期需重新申请。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：新密码 + 确认密码输入表单卡片

**核心组件清单**
- `Header`
- `LoadingSpinner`（提交时按钮内嵌）

**布局规范**
- 页面最大宽度：1280px
- 表单卡片：最大宽度 400px，垂直水平居中
- 区域间距（block）：32px (`space-y-8`)

**状态变体**
- default: 新密码 + 确认密码输入框 + 「重置密码」按钮。
- loading: 按钮内嵌 Spinner，输入框 disabled。
- success: 密码重置成功 -> 自动登录 -> 跳转首页（3 秒倒计时提示）。
- error: 行内红字错误提示（密码长度不足、两次密码不一致、token 无效或过期等）。
- 特殊状态：token 无效或过期时显示「链接已失效，请重新申请重置」EmptyState + 跳转 `/forgot-password`。

**响应式规则**
- 移动 (<=700px): 表单卡片全宽（margin 16px），padding 20px。
- 平板 (<=1100px): 表单卡片最大宽度 400px，居中。
- PC (>1100px): 表单卡片最大宽度 400px，居中，两侧留白。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 密码强度指示器：实时显示密码强度（弱/中/强），使用 `fg.danger` / `fg-muted` / `accent-emphasis` 颜色。
- 重置成功后自动调用登录 API，无需用户再次输入密码。

## Page: /series/[id] 内容系列详情

**Key Constraints**
- 首版只展示公开系列；页面位于 `(public)` route group，未登录用户可浏览。
- 系列内容顺序完全以后端 `sort_order` 为准，前端不得重新排序或猜测封面。
- 遵守扁平、克制的信息页风格；不得使用大 hero、装饰背景或营销型 CTA。

**目标与放置**
- 目标：展示一个创作者整理的有序内容系列，让用户按顺序进入每个内容详情。
- 放置：`frontend/app/(public)/series/[id]/page.tsx`；从 `SeriesNav` 的目录链接、作者主页或分享链接进入。
- 与 `ContentDetail` 关系：这是系列目录页，不重复渲染内容正文。

**核心组件清单**
- `Header`
- `ContentCard`（紧凑列表/网格变体）
- `EmptyState`
- `SkeletonCard`
- `Toast`

**布局规范**
- PC (>1100px)：主容器最大宽度 `1080px`，顶部为紧凑 series summary（封面 220x140、标题、作者、zone、item_count、描述），下方为有序内容列表；列表使用单列 table-like rows 或 2 列紧凑卡片，优先可扫描。
- 平板 (701-1100px)：summary 保持横向，列表单列；内容标题和序号不截断关键字。
- 移动 (<=700px)：summary 单列，封面 16:9；列表单列，每项触控区域不小于 44px。
- 序号显示为低权重 `text-xs text-fg-muted`，不作为主要视觉装饰。

**状态变体**
- default：显示 series summary + ordered items；每项可进入对应内容详情。
- loading：summary 骨架 + 列表骨架 x6。
- empty：series summary 保留，列表区域显示 EmptyState。
- error：404 使用 EmptyState；网络错误 Toast + 重试按钮。
- success：本页无写操作；从管理页跳转回来不显示额外成功状态。
- disabled：后端返回不可见或无效内容时不渲染可点击项；不要显示断链卡片。

**核心交互与焦点顺序**
- 焦点顺序：Header -> series summary 中作者链接 -> 内容列表第一项 -> 后续内容项 -> 返回顶部/分页。
- 内容项点击或 Enter 跳转详情；不得在整页上增加拖拽或管理控件。
- 如果后端后续返回分页，分页控件位于列表底部并使用 `page` + `page_size`。

**可访问性**
- 正文与背景对比度不低于 4.5:1；序号辅助信息也需可读。
- 内容项主链接触控目标不小于 44px。
- 列表容器使用 `aria-label={t('series.detail.items.ariaLabel')}`；封面图 alt 使用系列标题。
- 不只靠颜色表达 zone 或当前状态；使用文本 Badge。

**i18n key namespace**
- 建议 namespace：`series.detail.*`。
- 覆盖：`series.detail.header.*`、`series.detail.items.*`、`series.detail.empty.*`、`series.detail.error.*`、`series.detail.a11y.*`。
- 不在页面硬编码“目录/共 N 项/暂无内容”等文案。

**与现有组件关系**
- 内容列表可复用 `ContentCard` 的紧凑变体；不要引入新的卡片视觉系统。
- `SeriesNav` 从内容详情跳转到本页；本页不展示 `SeriesNav` 以免导航重复。
- 不依赖 `StudioLayout`；管理操作只出现在 `/studio/series`。

**Playwright 截图检查点**
- `screenshots/community-content-series-detail-desktop.png`：未登录打开公开系列，summary + 有序列表。
- `screenshots/community-content-series-detail-mobile.png`：移动单列，内容项触控高度达标。
- 交互检查：404 EmptyState、空系列 EmptyState、列表顺序按 sort_order、内容链接可键盘打开。

## Page: /collections/[id] 收藏集详情（Task 122-124）

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。
- 私有收藏集仅创建者可见；公开收藏集允许未登录用户浏览 published 内容。
- 页面必须放在 `(public)` route group；是否可见由后端 detail API 兜底，不在前端猜测私有权限。
- 收藏集按 `zone` 创建后不可变更；内容卡片只渲染后端返回的 published 内容。

**目标与放置**
- 目标：展示一个收藏集的公开信息、筛选后的内容列表，以及创建者可用的管理操作。
- 放置：`frontend/app/(public)/collections/[id]/page.tsx`；从用户主页收藏集列表、`CollectionPicker` 和内容详情入口跳转进入。
- 视觉角色：公共内容浏览页，信息层级弱于内容详情页，不使用 hero；收藏集信息区是页面摘要，不是营销头图。

**核心组件清单**
- `Header`
- `CollectionInfoCard`（封面、标题、描述、作者、内容数、可见性 Badge）
- `ContentTypeFilter`（按 content_type 筛选：全部/图文/视频/音频/模板...）
- `ContentCard`
- `MasonryGrid`
- `EmptyState`
- `LoadingSpinner`
- `ConfirmModal`（创建者删除收藏集/移除内容）
- `Toast`

**布局规范**
- PC (>1100px)：页面最大宽度 `1280px`，`px-6 py-6`；顶部信息区为单个横向 summary block（封面或默认缩略区域 160x106 + 文本 + owner 操作），下方为筛选栏和 4 列 MasonryGrid。
- 平板 (701-1100px)：信息区保持横向但封面缩小；内容网格 3 列；筛选标签横向滚动。
- 移动 (<=700px)：信息区单列，封面 16:9；网格 2 列；筛选栏横向滚动并贴近网格，不加额外卡片包裹。
- 区域间距 `space-y-6`；信息区、筛选栏、网格是同级区块，禁止卡片套卡片。

**状态变体**
- default：收藏集信息 + content_type 筛选 + 内容网格；公开收藏集未登录也可浏览。
- loading：信息区骨架 + 筛选栏骨架 + 网格 `SkeletonCard` x8，尺寸稳定。
- empty：保留收藏集信息区；内容区显示 EmptyState。创建者看到添加内容 CTA，非创建者仅看到说明。
- error：404/403 使用 EmptyState，不泄露私有收藏集标题；网络错误 Toast + 可重试按钮。
- success：编辑信息、移除内容、删除收藏集成功后 Toast；删除收藏集成功跳转用户收藏集列表。
- disabled：非 owner 隐藏编辑/删除/移除控件；默认收藏集删除按钮 disabled 并提供 tooltip/说明。

**响应式规则**
- 移动 (<=700px): 瀑布流 2 列，筛选标签横向滚动。
- 平板 (<=1100px): 瀑布流 3 列，筛选标签横向滚动。
- PC (>1100px): 瀑布流 4 列，筛选标签居中。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 筛选标签点击：切换 `content_type` 过滤并更新 URL query；当前筛选使用 `aria-current="true"`。
- 内容卡片点击：跳转对应内容详情；后端未返回的内容不渲染占位。
- 创建者操作：信息区提供编辑、删除；删除需 ConfirmModal。默认收藏集不可删除。
- 内容移除：owner 视角在卡片 hover/focus 时显示低权重移除按钮；移动端始终显示图标按钮，避免 hover 依赖。
- 焦点顺序：Header -> 收藏集标题区 -> owner 操作 -> 筛选标签 -> 内容卡片 -> 卡片内移除操作 -> 分页/加载更多。
- 键盘：筛选标签、编辑/删除、移除按钮均可 Tab 聚焦，Enter/Space 触发；ConfirmModal 焦点锁定。

**可访问性**
- 文字/背景对比度不低于 4.5:1；可见性 Badge 不能只靠颜色区分，需有文本或 `aria-label`。
- 所有按钮和内容卡片可点击区域触控目标不小于 44px。
- 封面图片使用收藏集标题作为 alt；无封面时占位图 `aria-hidden="true"`。
- 网格区域使用 `aria-label={t('collections.detail.grid.ariaLabel')}`，筛选栏使用 `role="tablist"` 或一组语义按钮。

**i18n key namespace**
- 建议 namespace：`collections.detail.*`。
- 覆盖：`collections.detail.header.*`、`collections.detail.filters.*`、`collections.detail.ownerActions.*`、`collections.detail.empty.*`、`collections.detail.error.*`、`collections.detail.toast.*`、`collections.detail.a11y.*`。
- 不在页面组件中硬编码公开/私有、空状态、错误、按钮文案。

**与现有组件关系**
- 内容网格复用 `MasonryGrid` + `ContentCard`；不要为收藏集另建内容卡片体系。
- 收藏入口从 `ContentDetail`/`ReactionBar` 打开 `CollectionPicker`；收藏集详情页本身不承担添加当前内容的弹窗。
- 与旧添加收藏弹窗关系：`CollectionPicker` 是后续 canonical 名称，旧 modal 只能作为其内部呈现或兼容导出。

**Playwright 截图检查点**
- `screenshots/community-collections-desktop.png`：公开收藏集未登录访问，信息区、筛选栏、4 列内容网格。
- `screenshots/community-collections-owner-mobile.png`：owner 移动端，编辑/移除控件可见且 44px。
- 交互检查：公开未登录可访问、私有非 owner 显示 EmptyState、筛选更新 query、owner 移除内容、默认收藏集删除 disabled。

## Page: /user/[userId]/collections 用户收藏集列表（Task 122-123）

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。
- 私有收藏集仅创建者可见。
- 页面位于 `(public)` route group；未登录访问他人页面可浏览公开收藏集，自己的页面由登录状态决定 owner controls。
- 可见性过滤必须由后端 `GET /api/v1/collections?owner_id=:userId` 兜底，前端不得通过拿到全量后自行隐藏私有收藏集。

**目标与放置**
- 目标：在用户主页外提供可分享的收藏集列表页，让访客浏览公开收藏集，owner 管理自己的公开/私有收藏集。
- 放置：`frontend/app/(public)/user/[userId]/collections/page.tsx`；从用户主页「收藏集」Tab、收藏集详情 owner 链接和公开分享入口进入。
- 页面是公共浏览页，不使用 Studio/Admin 布局。

**核心组件清单**
- `Header`
- `CollectionCard`（封面缩略图 + 标题 + 内容数 + 可见性 Badge）
- `EmptyState`
- `SkeletonCard`
- `ConfirmModal`（owner 删除收藏集）
- `Toast`

**布局规范**
- PC (>1100px)：页面最大宽度 `960px`，`px-6 py-6`；顶部为用户摘要行（头像、用户名、公开收藏集数），右侧 owner 新建按钮；下方 `grid grid-cols-3 gap-4`。
- 平板 (701-1100px)：最大宽度 `840px`，网格 3 列；顶部操作可换行。
- 移动 (<=700px)：主体 `px-4 py-4`，顶部摘要单列，网格 2 列；新建按钮全宽但不浮动。
- 区域间距 `space-y-6`；不要把整个页面包进大卡片。

**状态变体**
- default：收藏集网格列表；owner 看到公开与私有收藏集，非 owner/匿名只看到公开收藏集。
- owner-empty：EmptyState + 新建收藏集 CTA。
- visitor-empty：只读 EmptyState，不展示创建 CTA。
- loading：用户摘要骨架 + `CollectionCard` skeleton x6，稳定网格高度。
- error：Toast + 局部重试按钮；404 用户不存在时显示 non-leaking EmptyState。
- disabled：默认收藏集删除 disabled；非 owner 不渲染编辑/删除。

**响应式规则**
- 移动：卡片触控区域不小于 44px；owner 操作图标按钮始终可见，避免 hover-only。
- 平板/PC：卡片可在 hover/focus 时展示 owner 操作，但键盘 focus 必须可见。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`

**交互细节**
- 收藏集卡片点击：跳转 `/collections/[id]`。
- 「新建收藏集」按钮：弹出 Modal（标题输入 + 公开/私有选择 + 创建按钮）。
- 卡片 owner 操作：编辑打开表单/Modal，删除必须 ConfirmModal；默认收藏集删除按钮 disabled 并显示说明。
- `GET /api/v1/collections?owner_id=:userId` 返回空列表时按 owner/visitor 分支显示空态。
- 焦点顺序：Header -> 用户摘要链接/信息 -> 新建按钮（owner only） -> 收藏集卡片 -> 卡片 owner actions -> 分页/加载更多。

**可访问性**
- 收藏集卡片主链接和 owner action 必须是独立可聚焦元素；点击整卡时不得吞掉 action 按钮事件。
- 公开/私有 Badge 必须有文本或 aria-label，不能只靠颜色。
- 网格使用 `aria-label={t('collections.userList.grid.ariaLabel')}`。
- 所有图标按钮提供 `aria-label`；删除确认弹窗使用 `role="dialog"` 和焦点锁定。

**i18n key namespace**
- 建议 namespace：`collections.userList.*`。
- 覆盖：`collections.userList.header.*`、`collections.userList.actions.*`、`collections.userList.empty.*`、`collections.userList.error.*`、`collections.userList.toast.*`、`collections.userList.a11y.*`。

**Playwright 截图检查点**
- `screenshots/community-collections-user-list-desktop.png`：PC 访客视角，只显示公开收藏集网格。
- `screenshots/community-collections-user-list-owner-mobile.png`：移动 owner 视角，私有 Badge、新建按钮和 owner 操作可见。
- 交互检查：匿名访客访问公开列表、他人私有不泄露、owner 创建/删除默认保护、卡片跳转 `/collections/[id]`。

## Component: CollectionInfoCard 收藏集信息摘要

**Key Constraints**
- 仅用于 `/collections/[id]` 顶部摘要；不是通用页面 section wrapper。
- 保持 1px border 扁平设计，无 box-shadow；封面、文本、owner 操作在同一个 summary block 内。
- 不泄露私有收藏集信息；当后端返回 403/404 时父页面直接显示 EmptyState，不渲染本组件。

**Props 接口**
```ts
interface CollectionInfoCardProps {
  collection: {
    id: number;
    title: string;
    description?: string;
    zone: 'original' | 'fanwork';
    is_public: boolean;
    is_default: boolean;
    item_count: number;
    owner: { id: number; username: string; avatar_url?: string };
    cover_url?: string;
  };
  isOwner?: boolean;
  onEdit?: () => void;
  onDelete?: () => void;
}
```

**布局规范**
- PC (>1100px)：横向 `grid-cols-[160px_minmax(0,1fr)_auto]`；封面 160x106，文本区 line-clamp，owner 操作右侧对齐。
- 平板 (701-1100px)：封面 132x88，操作按钮仍在右侧但可换行。
- 移动 (<=700px)：单列；封面 16:9；操作按钮在标题下方一行，触控目标 44px。

**状态变体**
- default：标题、描述、作者、zone、公开/私有、内容数均显示。
- owner：显示编辑、删除；默认收藏集删除 disabled。
- non-owner：隐藏编辑、删除。
- no-cover：使用低对比默认缩略区域，`aria-hidden="true"`。

**可访问性**
- 封面图片 alt 使用收藏集标题；默认缩略图隐藏于辅助技术。
- 公开/私有状态用文本和图标双重表达，不只靠颜色。
- 删除按钮必须触发 `ConfirmModal`，焦点锁定。

**i18n key namespace**
- `collections.info.*`，覆盖 badge、actions、a11y、default-protected tooltip。

## Component: ContentTypeFilter 内容类型筛选

**Key Constraints**
- 用于收藏集详情内容列表筛选；不作为全站内容分类 tab 的替代。
- 筛选值固定为 `all`, `image`, `article`, `video`, `audio`, `template`, `sheet_music`, `mod`, `prompt`, `other`。
- 点击后更新 URL query `content_type` 并触发数据重新加载；不做客户端私有过滤。

**Props 接口**
```ts
interface ContentTypeFilterProps {
  value: string;
  counts?: Record<string, number>;
  onChange: (value: string) => void;
}
```

**布局规范**
- PC (>1100px)：横向按钮组，允许换行但不改变网格列宽。
- 平板/移动：横向滚动 chips，左右 padding 与页面内容对齐；滚动条不遮挡内容。
- 每个筛选按钮高度至少 36px；移动端触控目标 44px。

**状态变体**
- default：未选中项使用 `border-border-default`。
- selected：当前项使用 `aria-current="true"` 或 `aria-selected="true"`，并使用 tokenized accent border。
- loading：禁用所有项但保持当前选中态。
- zero-count：可点击但显示计数 0；后端返回空列表后由父页面 EmptyState 处理。

**可访问性**
- 使用 `role="tablist"`/`role="tab"` 或语义按钮组。
- 当前筛选状态不能只靠颜色；添加文本状态或 `aria-current`。

**i18n key namespace**
- `collections.filters.*`，覆盖每个 content type label、all label、a11y label。

## Component: CollectionCard 收藏集卡片（Task 122-123）

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface CollectionCardProps {
  className?: string;
  collection: {
    id: number;
    title: string;
    description?: string;
    is_public: boolean;
    item_count: number;
    cover_url?: string;
    created_at: string;
  };
  isOwner?: boolean;
  isLoading?: boolean;
  onEdit?: () => void;
  onDelete?: () => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default overflow-hidden cursor-pointer group">`
- 封面区: 3:2 比例容器，`object-cover` 图片填满；无封面时使用集合 SVG 占位图
- 信息区: `p-3`，标题（`text-sm font-medium line-clamp-1`）-> 内容数 + 可见性 Badge行（`text-xs text-fg-muted`）
- 可见性 Badge: 公开 `text-fg-muted` + unlocked图标；私有 `text-fg-muted` + locked图标
- 悬停: `group-hover:-translate-y-1 transition-transform duration-200`

**尺寸规范**
- 卡片: 自适应宽度，padding 12px (p-3)
- 封面区: aspect-ratio 3:2
- 字号: `text-sm` (14px) 标题，`text-xs` (12px) 辅助信息
- 间距: 元素间隙 4px (`gap-1`) 或 8px (`gap-2`)

**状态变体**
- default: `bg-canvas-default text-foreground`
- hover: 边框颜色加深 + 轻微上浮
- loading: SkeletonCard 占位
- empty: 不渲染（由父级处理 EmptyState）

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 整卡可点击跳转 `/collections/[id]`
- 创建者视角：hover 时右上角显示编辑/删除按钮（低视觉权重，`opacity-0 group-hover:opacity-100`）

## Component: CollectionPicker 收藏集选择器

**Key Constraints**
- Canonical 组件名为 `CollectionPicker`；现有旧添加收藏弹窗可作为兼容包装或内部实现，但新代码只能引用 `CollectionPicker`。
- 仅展示与当前内容 `zone` 匹配的收藏集；是否已添加由列表 API 的 `contains_item` / `item_id` 决定，不通过制造重复请求判断。
- Modal/Popover 可使用 `shadow-md`，其余内部列表项保持 1px border、无阴影。
- **关闭行为（#80 决策 / 审计问题 14 配套 权威，对齐 ConfirmModal 既有模式）**：背板点击与 Esc 关闭、焦点陷阱与焦点恢复；添加/创建/移除请求进行中（busy/creating）时阻止关闭，不丢失进行中的操作。只改本弹窗，不扩及其他浮层。

**目标与放置**
- 目标：从内容详情页把当前内容添加到同 zone 收藏集，支持已添加状态、移除、搜索和内联新建收藏集。
- 放置：由 `ContentDetail` 或 `ReactionBar` 的收藏/添加按钮打开；不在 `/collections/[id]` 详情页内默认打开。
- 与发布流程无关，不依赖 `PublishForm`。

**Props 接口**
```ts
interface CollectionPickerProps {
  contentId: number;
  contentTitle: string;
  zone: 'original' | 'fanwork';
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onChanged?: () => void;
}
```

**布局规范**
- PC (>1100px)：居中 Modal，宽度 `480px`，最大高度 `min(70vh, 640px)`；顶部标题/说明，正文为可滚动收藏集列表，底部为新建表单/取消。
- 平板 (701-1100px)：Modal 宽度 `min(92vw, 480px)`，列表高度限制不超过视口 60%。
- 移动 (<=700px)：底部 Sheet 形式，圆角只在顶部 `rounded-lg`；列表占可用高度，底部操作固定在 Sheet 内，不遮挡系统导航。
- 收藏集列表项：`button` 或 `div + button` 结构，`border border-border-default rounded-md p-3`，标题、item_count、公开状态、已添加状态清晰排列。

**弹窗内通知（#80 决策 权威）**
- 添加/创建/移除的结果与错误以弹窗内短暂通知呈现（footer 上方绝对定位浮动小条）：`pointer-events-none`、半透明底、约 2 秒自动淡出、出现/消失不引起布局跳动；成功与失败都走通知。
- 行内「已加入」徽标保留为持久状态标记，不因短通知消失。
- 弹窗内不再触发全局 toast；弹窗外的收藏动作不受影响。

**状态变体**
- default：显示同 zone 收藏集列表；已添加项显示只读状态和可选移除操作。
- loading：Modal 骨架列表 x5，保持标题和关闭按钮可见。
- empty：该 zone 无收藏集时显示 EmptyState + 内联新建入口。
- error：加载失败显示内联错误和重试按钮，同时 Toast 报错。
- success：添加/移除/新建成功后行内状态即时更新 + 弹窗内通知；不强制关闭 Modal。
- disabled：当前用户未登录、信誉不足、内容不可收藏、请求进行中时相关按钮 disabled。
- busy/creating：添加/创建/移除请求在途，阻止背板点击/Esc 关闭，直到请求落定。
- search：收藏集数量超过 10 时显示搜索框；无匹配结果显示局部 EmptyState。

**核心交互与焦点顺序**
- 打开后焦点落在关闭按钮或标题后的搜索框；关闭后焦点返回触发按钮。
- 焦点顺序：关闭 -> 搜索 -> 收藏集列表项 -> 已添加项移除按钮 -> 新建入口 -> 取消。
- 点击未添加项：调用添加接口；点击已添加项的移除按钮：使用 `item_id` 调用移除接口。
- 内联新建：展开标题、描述、公开开关和创建按钮；创建成功后新集合进入列表并可直接添加当前内容。
- 键盘：`ArrowDown/ArrowUp` 可在列表项内移动焦点（可选），`Enter/Space` 执行当前主操作，Esc 关闭（busy 中除外）。

**可访问性**
- Modal 使用 `role="dialog"`、`aria-modal="true"`、`aria-labelledby`；移动 Sheet 同样保持 dialog 语义。
- 所有按钮和列表项主操作触控目标不小于 44px。
- 已添加、公开/私有状态不能只靠颜色；使用文本、图标 aria-label 或 `aria-pressed`。
- 正文对比度不低于 4.5:1；disabled 状态即使降低透明度也需保留可读说明。

**i18n key namespace**
- 建议 namespace：`collections.picker.*`。
- 覆盖：`collections.picker.title`、`collections.picker.search.*`、`collections.picker.actions.*`、`collections.picker.create.*`、`collections.picker.states.*`、`collections.picker.errors.*`、`collections.picker.a11y.*`。
- 不硬编码“已添加/新建/公开/私有”等可见文案。

**与现有组件关系**
- 由 `ContentDetail`/`ReactionBar` 触发；成功后可通知父级刷新收藏状态。
- 可复用 `ConfirmModal` 做移除确认，但不要在 Modal 中再嵌套卡片容器。
- 与 `CollectionCard` 只共享收藏集摘要数据，不复用展示卡片布局。

**Playwright 截图检查点**
- `screenshots/community-collections-picker-desktop.png`：PC Modal，包含搜索、已添加、未添加和新建入口。
- `screenshots/community-collections-picker-mobile.png`：移动 Sheet，列表滚动和底部操作可见。
- 交互检查：只列同 zone 收藏集、contains_item 状态、创建后添加、移除使用 item_id、无重复请求。

## Component: DownloadButton 下载按钮（Task 121）

**Key Constraints**
- 信誉分 < 3 用户：下载按钮 disabled，hover tooltip 提示「信誉分不足」。
- 封禁用户：下载按钮 disabled，tooltip 提示「账号已被封禁」。
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**Props 接口**
```ts
interface DownloadButtonProps {
  className?: string;
  contentId: number;
  contentTitle: string;
  contentType: string;
  isDisabled?: boolean;
  disableReason?: string;
  onDownloadComplete?: () => void;
}
```

**视觉结构**
- 按钮: `<button className="inline-flex items-center gap-2 px-4 py-2 border border-border-default rounded-md bg-canvas-default text-sm font-medium hover:bg-canvas-subtle transition-colors">`
- 图标: `<Icon name="download" className="w-4 h-4" />`
- 文字: "下载" 或 content_type 对应的文字（如 "下载乐谱"）

**尺寸规范**
- 按钮高度: `h-9` (36px)
- 内间距: `px-4 py-2`
- 字号: `text-sm` (14px)
- 图标: `w-4 h-4` (16px)

**状态变体**
- default: `bg-canvas-default text-foreground border-border-default`
- hover: `hover:bg-canvas-subtle hover:border-border.emphasis`
- active: `active:bg-canvas-subtle scale-95`
- disabled: `opacity-50 cursor-not-allowed`，hover 显示 tooltip 提示原因
- loading: Spinner 替换图标，文字变为 "下载中..."

**暗色模式适配**
- 全局切换暗色类后组件自动映射 token 变量。

**关键交互**
- 点击 -> 调用 `GET /api/v1/contents/:id/download` -> 获取 OSS 签名 URL -> 触发浏览器下载
- 下载完成后 `download_count + 1`（异步，不影响用户操作）
- 信誉分不足时按钮 disabled + tooltip 显示「信誉分不足，无法下载」

## Component: LoadingSpinner 加载旋转器

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none。
- 行内使用，不占据额外块级空间。
- 颜色继承当前文字色。

**Props 接口**
```ts
interface LoadingSpinnerProps {
  className?: string;
  size?: 'sm' | 'md' | 'lg';
  color?: 'foreground' | 'muted';
}
```

**视觉结构**
- `<svg className="animate-spin" width={size} height={size}>` + `<circle>` 描边动画
- 默认 `text-foreground`，轻量场景用 `text-muted-foreground`
- 尺寸: sm 16px / md 20px / lg 28px

**状态变体**
- default: 正常旋转动画。
- 无 hover/active/disabled 状态（装饰性元素）。

## Component: SkeletonCard 骨架屏卡片

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；SkeletonCard/Detail 镜像带边框最终面板时使用 elevation 1。
- 仅用于内容加载占位，不可交互。
- pulse 周期为 1.6s；`prefers-reduced-motion: reduce` 下静止。

**Props 接口**
```ts
interface SkeletonCardProps {
  className?: string;
  zone?: 'original' | 'fanwork';
  count?: number; // 占位卡片数量，默认 1
}
```

**视觉结构**
- 外层容器: `<div className="rounded-lg border border-border bg-card p-4 shadow-sm">`，`shadow-sm` 映射到 elevation 1。
- 封面区: `<div className="rounded-lg bg-canvas-subtle animate-[pulse_1.6s_ease-in-out_infinite] motion-reduce:animate-none" />`
- 信息区: `p-3 space-y-2`
  - 标题行: `<div className="h-4 bg-canvas-subtle rounded-sm w-3/4 animate-pulse" />`
  - 描述行: `<div className="h-3 bg-canvas-subtle rounded-sm w-1/2 animate-pulse" />`

**尺寸规范**
- 二创区: 封面固定 3:4 比例
- 原创区: 封面最小高度 150px，无固定比例

## Component: Toast 消息提示

**Key Constraints**
- 位置固定右上角，z-50。
- 自动消失 4s，支持手动关闭。
- Toast 是浮层反馈，使用 elevation 3 并保留 1px status border。
- success/error/warning/info 复用 green/rose/orange/blue 标签柔和背景与前景 token，不引入任意 Tailwind 色常量。

**Props 接口**
```ts
interface ToastProps {
  id: string;
  type: 'success' | 'error' | 'info' | 'warning';
  message: string;
  duration?: number;
  onClose: (id: string) => void;
}
```

**视觉结构**
- 容器: `<div className="flex items-start gap-3 rounded-lg border bg-card p-3 text-sm shadow-md">`，`shadow-md` 映射到 elevation 3。
- 图标: type 对应图标（success=check-circle, error=x-circle, info=info, warning=alert-triangle）
- 文字: `flex-1 text-foreground`
- 关闭按钮: `<button className="text-fg-muted hover:text-foreground">`，含本地化可访问名称和 coarse-pointer 44px 目标。

**状态变体**
- success: `--tag-green-bg` / `--tag-green-fg`
- error: `--tag-rose-bg` / `--tag-rose-fg`
- info: `--tag-blue-bg` / `--tag-blue-fg`
- warning: `--tag-orange-bg` / `--tag-orange-fg`
- 入退场动效: 200ms opacity + horizontal translate；reduced-motion 下只保留即时显隐。
- 移动端容器左右各 16px 且不超过 viewport；桌面最大宽度 384px。

## Component: Footer 页脚

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none。
- 页脚内容：版权信息 + 可选链接。

**Props 接口**
```ts
interface FooterProps {
  className?: string;
}
```

**视觉结构**
- 外层容器: `<footer className="border-t border-border bg-canvas-default py-8 px-4">`
- 内部: 居中容器 `max-w-7xl mx-auto text-center`
- 版权文字: `<p className="text-sm text-fg-muted">` © OmniCraft 2026
- 可选链接行: `flex justify-center gap-6`，链接使用 `text-sm text-fg-muted hover:text-foreground`

**暗色模式适配**
- 全局切换暗色类后 token 自动映射。

## Component: StatsCard 统计卡片

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none，使用 1px border。
- 仅用于数据概览页面（/studio/overview, /studio/followers）。

**Props 接口**
```ts
interface StatsCardProps {
  className?: string;
  label: string;
  value: number | string;
  change?: number; // 环比变化百分比，正数增长负数下降
  icon?: React.ReactNode;
  isLoading?: boolean;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default p-4">`
- 图标区: 左侧/上方图标 `text-fg-muted w-8 h-8`
- 数值: `<p className="text-2xl font-semibold text-foreground">`
- 标签: `<p className="text-sm text-fg-muted">`
- 变化百分比: 正数 `<span className="text-green-600 text-xs">↑ +N%</span>`，负数 `<span className="text-destructive text-xs">↓ -N%</span>`

**状态变体**
- default: 数值 + 标签 + 可选变化率。
- loading: `<div className="h-8 w-20 bg-canvas-subtle rounded-sm animate-pulse" />` 占位数值。

## Component: NotificationBell 通知铃铛

**Key Constraints**
- 位置：Header 右侧，用户菜单前。
- 5 分钟轮询 `GET /api/v1/notifications/unread-count`。
- 可选 SSE 实时推送连接。

**Props 接口**
```ts
interface NotificationBellProps {
  className?: string;
  unreadCount: number;
  onClick: () => void;
}
```

**视觉结构**
- 按钮容器: `<button className="relative p-2 text-fg-muted hover:text-foreground transition-colors">`
- 铃铛图标: `<Icon name="bell" className="w-5 h-5" />`
- Badge 容器: `<span className="absolute -top-0.5 -right-0.5 flex items-center justify-center min-w-[18px] h-[18px] px-1 rounded-full bg-destructive text-[11px] font-medium text-white">`
- Badge 逻辑: `unreadCount > 0` 显示，`> 99` 显示 `99+`

**响应式行为**
- 移动端无变化（固定显示在 Header 右侧）。

## Component: SortSelect 排序选择器

**Key Constraints**
- 遵守全局 Indigo 三档层级规则；本节未声明 elevation，故该表面保持 shadow-none。
- **共享自定义排序控件（#64 决策 5 / 审计问题 13 权威，替换三处原生 select）**：二创区（`/`）、原创区（`/original`）、IP 库（`/ips`）统一复用本组件；由 trigger + 定位 listbox/menu 构成，不使用原生 `<select>`（原生平台弹出会覆盖触发本体且无法定位）。
- 支持键盘导航、focus 返回、外部点击与 Esc 关闭、sticky 工具栏堆叠（z-index 处理）与视口碰撞处理；跨平台不要求像素一致的弹层位置，优先可用性与视觉质量。
- 所有选项与触发控件有可访问名称；打开后焦点进入 listbox，Esc 关闭并把焦点返回 trigger。

**Props 接口**
```ts
interface SortSelectProps {
  className?: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
  ariaLabel: string;   // 可访问名称（i18n）
}
```

**视觉结构**
- 外层容器（trigger）: `<button role="combobox" aria-haspopup="listbox" aria-expanded="false" className="inline-flex items-center gap-2 border border-border-default rounded-md bg-canvas-default px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-accent-emphasis">` — 显示当前选项 label + lucide `ChevronDown`
- 弹层: `<ul role="listbox">`，定位在 trigger 下方，`--elevation-3` 浮层阴影 + 1px border + `rounded-md`；打开时不得遮挡 trigger 本体（视口碰撞时向上翻转）
- 每个选项: `<li role="option" aria-selected={value===option.value}>`，当前选项用 accent-subtle 背景 + accent-emphasis 文字
- 在 sticky 工具栏内使用时，弹层 z-index 高于工具栏其余内容

**状态变体**
- default: 显示当前选中选项。
- open: `aria-expanded=true`，弹层可见，焦点在 listbox 内；ArrowUp/ArrowDown 移动，Home/End 到首尾，Enter/Space 选择并关闭，Esc 取消并返回 trigger。
- focus: `ring-2 ring-accent-emphasis`。
- disabled: `opacity-50 cursor-not-allowed`（无选项或页面禁用）。
