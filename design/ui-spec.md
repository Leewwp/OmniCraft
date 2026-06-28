# OmniCraft UI 设计规格

> ℹ️ **生成状态**：本文件由 Gemini 批量生成，已完成初始内容填充。全局规范（Design Tokens、Interaction Patterns）和组件规范（Component 章节）可直接用于实现。以下偏差需 Agent 实现时注意：
>
> 1. **部分页面「核心组件清单」仍为模板默认值**（`EmptyState/LoadingSpinner`），Agent 应参考对应 Task 的 `ui_spec_ref` 和 `design/design-system.md` 组件规范获取真实组件列表。
> 2. **表单类页面的「响应式规则」已修正**（login/register/settings/judge exam/rehab/messages/appeals 等 15 个页面），但个别页面可能仍有遗留。
>
> Agent 在实现前端时，优先以 `## Component:` 具体组件的规范和 `design/design-system.md` 为准；发现不一致时以设计规范为准。（注：历史 `UI Design.md` 已归档至 `docs/archive/`，不再作为视觉权威。）

---

<!-- Gemini 生成内容从此行开始 -->

## Global Design Tokens

> **来源**: 以下所有 token 值以 `design/design-system.md` 为准，是本文件的唯一设计权威。

- **颜色 token**：使用 `design/design-system.md` 定义的 CSS 自定义属性，以 `--xxx` 格式引用（`--` 前缀的 CSS 自定义属性），使用时通过 `var()` 读取值：`--background`、`--foreground`、`--primary`、`--border` 等基础色，以及 `--canvas-default`、`--canvas-subtle`、`--border-default`、`--fg-muted`、`--accent-emphasis`、`--accent-subtle` 等自定义 token。标签颜色使用预设的 6 色体系 (blue/green/purple/orange/rose/sky)。所有颜色支持 light/dark 双模式，暗色模式通过根级 `.dark` 类自动切换。
- **字体**：font-family: `--font-sans: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif`，包含中文字体回退。等宽字体 `--font-mono: var(--font-geist-mono)`。字号阶梯：text-xs 12 / text-sm 14 / text-base 16 / text-lg 18 / text-xl 20 / text-2xl 24 / text-3xl 30
- **间距阶梯**：space-1 4px / space-2 8px / space-3 12px / space-4 16px / space-6 24px / space-8 32px / space-12 48px
- **圆角**：rounded-sm (3px) 小元素 / rounded-md (4px) 按钮/输入框 / rounded-lg (8px) 卡片/容器默认 / rounded-xl (12px) 大卡片 / rounded-full (9999px) 标签/药丸按钮。核心原则：统一圆角，`rounded-lg` (8px) 为默认，标签用 `rounded-full`。
- **动效**：transition duration-150 ease-out（默认）；duration-300 ease-in-out（Modal/Sheet）
- **阴影**：**强制 box-shadow:none**（无阴影全局原则），仅 Modal/Popover/Dropdown 使用 `shadow-md`

## Global Interaction Patterns

- **Hover**: `cursor-pointer` + 颜色加深一档（border / bg）。
- **Loading**: 骨架屏（Skeleton 灰色块）+ 按钮内联 Spinner，禁止全屏遮罩。
- **Error**: Toast（右上角，自动消失 4s）+ 内联红字提示（表单字段下方）。
- **Empty**: EmptyState 组件（图标 + 标题 + 说明 + 可选 CTA），不留空白区域。
- **Disabled**: `opacity-50` + `cursor-not-allowed` + 禁用 hover/click。

## Component: Header

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 固定高度 `h-[var(--header-h)]` (52px)，sticky 顶部。
- 底边框 `border-b border-border-default`，1px 分隔。
- 背景 `bg-canvas-default`，全宽布局。

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
- 内部布局: `<div className="max-w-7xl mx-auto h-full flex items-center justify-between px-4">`
- 左侧: Logo + 主导航链接（首页 / 原创 / 二创），水平排列 `gap-6`
- 中间: 搜索栏（SearchAgentInput 或基础搜索框），最大宽度 480px
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
- 无阴影（全局原则）。

**响应式行为**
- 移动 (≤700px): 导航链接折叠为汉堡菜单，搜索框缩小或隐藏。
- 平板 (≤1100px): 导航链接文字缩小，搜索框自适应。
- PC (>1100px): 默认布局。

**暗色模式适配**
- 全局切换暗色类后 token 自动映射。

**关键交互**
- 导航链接点击: 路由跳转，选中项高亮。
- 搜索框聚焦: 展开建议下拉。
- 发布按钮: 跳转 `/studio/publish/original`（已登录）或 `/login`（未登录）。
- 用户菜单: 点击 Avatar 展开下拉，点击外部关闭。

## Component: FacetedSearchSidebar

**Key Constraints**
- 必须引入 SearchAgentInput 进行自然语言检索，失败时降级。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `Header`
- `ContentCard`
- `MasonryGrid`
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
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /search 搜索页

**Key Constraints**
- 必须引入 SearchAgentInput 进行自然语言检索，失败时降级。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：搜索栏 + 分面侧边栏 + 搜索结果瀑布流

**核心组件清单**
- `Header`
- `SearchAgentInput`
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
- 特殊状态：未登录时 NL 搜索降级为关键词搜索。

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

## Page: /login 登录页

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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

## Page: /ip/[ipId] IP 详情页

**Key Constraints**
- 二创区页面/组件：依托于 ips.category 进行展示或跳转。
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `Header`
- `ContentCard`
- `MasonryGrid`
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
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /ip/[ipId]/[category] IP 类目内容列表

**Key Constraints**
- 二创区页面/组件：依托于 ips.category 进行展示或跳转。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `Header`
- `ContentCard`
- `MasonryGrid`
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
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /ip/[ipId]/discussions 讨论区列表

**Key Constraints**
- 二创区页面/组件：依托于 ips.category 进行展示或跳转。
- 信誉分 < 3 用户：发布/评论/点赞按钮 disabled，hover tooltip 提示「信誉分不足」。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：搜索栏 + 讨论列表 + 发帖入口

**核心组件清单**
- `Header`
- `DiscussionCard`
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：960px，居中
- 搜索框 + 发帖按钮 → 讨论卡片列表（按活跃时间倒序）
- 区域间距（block）：16px (`space-y-4`)

**状态变体**
- default: 讨论列表卡片（标题/作者/回复数/最后活跃时间）+ 搜索框。
- loading: 骨架屏（Skeleton 灰色块列表）。
- empty: "暂无讨论" EmptyState。
- error: Toast 右上角报错。
- 特殊状态：信誉分不足用户发帖按钮 disabled。

**响应式规则**
- 移动 (≤700px): 讨论列表全宽（margin 16px），卡片间距 12px。
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

## Page: /ip/[ipId]/discussions/[discussionId] 讨论详情

**Key Constraints**
- 二创区页面/组件：依托于 ips.category 进行展示或跳转。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 信誉分 < 3 用户：发布/评论/点赞按钮 disabled，hover tooltip 提示「信誉分不足」。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：讨论主帖 + 回复列表，各模块带 1px border

**核心组件清单**
- `Header`
- `DiscussionCard`
- `ReplyList`
- `CommentSection`（回复区复用楼中楼组件）
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：960px，居中
- 讨论主帖 → 回复列表（按时间排序）
- 区域间距（block）：24px (`space-y-6`)

**状态变体**
- default: 讨论标题/内容/作者 + 回复列表 + 回复输入框。
- loading: 骨架屏（Skeleton）。
- empty: 讨论不存在 404 EmptyState。
- error: Toast 右上角报错。
- 特殊状态：信誉分不足时回复按钮 disabled。

**响应式规则**
- 移动 (≤700px): 讨论详情全宽（margin 16px），回复列表卡片间距 12px。
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
- 推荐 Tab 为算法驱动的个性化推送（`sort=recommended`），非手动筛选选项。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 分类 Tab 栏：Header 下方固定（sticky），高度 `h-12`，底部 1px border 分隔，Tab 项横向滚动
- 主容器：居中最大宽度 1280px，页面背景 `bg-canvas-subtle`
- 内容模块：瀑布流卡片（无外层 border 容器，卡片直接排列）

**核心组件清单**
- `Header`
- `CategoryTabs`（原创区顶部分类 Tab 栏，横向滚动）
- `ContentCard`（原创区简化样式：封面自适应高度 + 标题 + @作者 + 点赞数）
- `MasonryGrid`（瀑布流，支持无限滚动）
- `SkeletonCard`（内容卡片骨架屏）
- `EmptyState`
- `Footer`

**分类 Tab 栏规范**
- 固定项目：`推荐`（默认选中，始终在第一位）
- 动态项目：从 `/api/v1/categories?zone=original&level=primary` 加载的 11 个一级分类
- 每个 Tab 项: `<button>` 样式，padding `px-4 py-2`，字号 `text-sm`
- 选中态：`text-accent-emphasis font-semibold` + 底部 2px accent 色下划线
- 未选中态：`text-fg-muted`
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
- 卡片点击：跳转内容详情页 `/original/[contentId]`。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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

## Page: /publish 发布内容

> ⚠️ **已废弃**：本页面已迁移至 `/studio/publish/original` 和 `/studio/publish/fanwork`（创作者工作室发布流程）。旧路由 `/publish` 保留 301 重定向到 `/studio/publish/original`。以下规范仅供迁移参考，**以新版 §13 创作者工作室章节和 Page: /studio/* 系列的最新规格为准**。

**Key Constraints**
- 必须读取 config.yaml 的文件大小限制（视频≤300MB, 图片≤20MB, 文本≤10MB等）。
- 破坏性操作（如放弃上传或退出）需弹 ConfirmModal。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `FileUploader`
- `MarkdownEditor`
- `UploadAssistPanel`

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
- 移动 (≤700px): 表单全宽（margin 16px），编辑器高度 250px，上传组件全宽。
- 平板 (≤1100px): 表单最大宽度 720px，居中。
- PC (>1100px): 表单最大宽度 960px，居中，两侧留白。

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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。
- 密码修改/注销等破坏性操作需 ConfirmModal 二次确认。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：设置分组卡片（个人信息 / 安全设置 / 危险操作），每组带 1px border

**核心组件清单**
- `Header`
- `ConfirmModal`（密码修改确认、账号注销确认）
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：720px，居中
- 设置分组卡片纵向排列，组间距 24px (`space-y-6`)
- 元素间距（inline）：16px (`gap-4`)

**状态变体**
- default: 头像、用户名、邮箱（只读）、Bio 编辑表单。
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

## Page: /settings/tag-groups 标签组管理

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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

## Page: /dashboard 创作者后台概览

> ⚠️ **已废弃**：本页面已迁移至 `/studio/overview`（创作者工作室数据概览）。旧路由 `/dashboard` 保留 301 重定向到 `/studio/overview`。以下规范仅供迁移参考，**以新版 Page: /studio/overview 的最新规格为准**。

**Key Constraints**
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
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
- 移动 (≤700px): 表格水平滚动，概览卡片单列堆叠，操作按钮展开式。
- 平板 (≤1100px): 表格自适应宽度，概览卡片 2 列。
- PC (>1100px): 表格最大宽度 1280px，概览卡片 4 列。

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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：按日期分组的浏览历史列表

**核心组件清单**
- `Header`
- `ContentCard`
- `ConfirmModal`（清除全部确认）
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：720px，居中
- 日期分组标题 + 卡片列表，每页 30 条滚动加载
- 区域间距（block）：24px (`space-y-6`)

**状态变体**
- default: 按日期分组（今天/昨天/日期）展示浏览过的内容卡片。
- loading: 骨架屏（Skeleton 灰色块）。
- empty: "暂无浏览记录" EmptyState + 探索首页 CTA。
- error: Toast 右上角报错。
- 特殊状态：未登录拦截跳转 /login。

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

## Page: /appeals 我的申诉

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：通知列表 + 私信对话列表，Tab 切换

**核心组件清单**
- `Header`
- `NotificationList`
- `ConversationList`
- `ChatWindow`
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：960px，居中
- 左侧对话列表 + 右侧聊天窗口（桌面端），移动端单栏切换
- 区域间距（block）：16px (`space-y-4`)

**状态变体**
- default: 通知/私信 Tab + 列表 + 聊天窗口。
- loading: 骨架屏（Skeleton 灰色块）。
- empty: "暂无消息" EmptyState。
- error: Toast 右上角报错。
- 特殊状态：未登录拦截；hover 显示单条标记已读/删除操作。

**响应式规则**
- 移动 (≤700px): 单栏切换（列表或聊天窗口全宽），底部 Tab 切换通知/私信。
- 平板 (≤1100px): 左侧列表 240px + 右侧聊天区自适应。
- PC (>1100px): 左侧列表 300px + 右侧聊天区自适应。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /rehab 素质建设课程

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 只有 config 中 `agent.web_agent_enabled=true` 且用户已登录才可见该功能。
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
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。
- 原创区卡片使用简化样式（无 IP 名、无标签 Badge、仅显示点赞数）。

**Props 接口**
```ts
interface ContentCardProps {
  className?: string;
  item: ContentItem;           // 内容数据（必需）
  zone: 'original' | 'fanwork'; // 区域标识，决定渲染样式
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构 — 二创区卡片（zone='fanwork'）**
- 外层容器: `<div className="border border-border-default rounded-md bg-canvas-default overflow-hidden">`（无 padding，内容填满）
- 封面区: 3:4 比例容器，`object-cover` 图片填满
- 信息区: `p-3`，标题（`text-sm font-medium line-clamp-2`）→ 作者 + IP 名行（`text-xs text-fg-muted`）→ 互动数据行（`text-xs` ❤️ + 💬）→ 标签行（最多 3 个低饱和 TagBadge）
- 图标: `<Icon className="text-fg-muted w-4 h-4" />`

**视觉结构 — 原创区卡片（zone='original'）**
- 外层容器: `<div className="rounded-md bg-canvas-default overflow-hidden cursor-pointer group">`（无 border，更干净的小红书风格）
- 封面区: 自适应高度（图片按原始宽高比，`object-cover w-full`），`max-height: 400px`，`overflow-hidden`
- 悬停遮罩: `<div className="absolute inset-0 bg-black/10 opacity-0 group-hover:opacity-100 transition-opacity" />`
- 封面缩放: `group-hover:scale-105 transition-transform duration-300`
- 信息区: `p-2`，标题（`text-sm font-medium line-clamp-2`）→ @作者（`text-xs text-fg-muted`）→ ❤️ 点赞数（`text-xs text-fg-muted`）
- 无标签 Badge、无 IP 名

**尺寸规范**
- 二创区卡片: padding 16px (p-4)，封面固定 3:4
- 原创区卡片: padding 8px (p-2)，封面高度自适应，最小高度 150px
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
- 整卡可点击（`<Link href={...}>` 包裹），跳转详情页。
- Hover 触发视觉反馈（微缩放 + 遮罩）。
- 键盘行为：支持 Tab 索引切换，Enter 选中。

## Component: MasonryGrid

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 外层容器: `<div className="columns-2 md:columns-3 lg:columns-4 gap-4 px-4">`
- 哨兵元素: `<div ref={sentinelRef} className="h-4" />` — IntersectionObserver 触发 onLoadMore
- 底部提示: 加载中显示 Spinner，已到底显示 "已经到底了" 文本

**无限滚动行为**
- 使用 IntersectionObserver 监听底部 sentinel 元素
- sentinel 进入视口 → 调用 `onLoadMore` 回调
- 父组件负责分页逻辑（useSWRInfinite），MasonryGrid 仅发出加载信号
- `hasMore=false` 时不触发加载

**尺寸规范**
- 列间距: `gap-4` (16px)
- 列数: 2 列（移动）/ 3 列（平板）/ 4 列（PC）
- 内容卡片由子组件 ContentCard 自行控制高度

**状态变体**
- default: 正常渲染内容卡片列表。
- loading: 初始加载时显示 12 个 SkeletonCard 占位。
- loadingMore: 底部显示 Spinner。
- empty: 使用 EmptyState 组件（图标 + 标题 + 说明 + CTA）。
- error: 底部显示 "加载失败，点击重试" 按钮。

**响应式行为**
- 移动 (≤700px): 2 列瀑布流 (`columns-2`)
- 平板 (≤1100px): 3 列瀑布流 (`columns-3`)
- PC (>1100px): 4 列瀑布流 (`columns-4`)

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas-default.dark` 等 token 变量。

**关键交互**
- 滚动触发加载更多（无需手动点击）。
- 卡片点击由 ContentCard 内部 Link 处理。

## Component: TagBadge

**Key Constraints**
- 使用预设 6 色体系 (blue/green/purple/orange/rose/sky)，颜色值参考 design-system.md 标签颜色表。
- 遵守全局扁平化无阴影设计规范。
- 标签默认无边框，使用纯色背景 + 对应文字色。

**Props 接口**
```ts
interface TagBadgeProps {
  className?: string;
  label: string;
  color?: 'blue' | 'green' | 'purple' | 'orange' | 'rose' | 'sky';
  size?: 'sm' | 'md';
  removable?: boolean;
  onRemove?: (label: string) => void;
}
```

**视觉结构**
- 容器: `<span className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium tag-{color}">`
- 文字: 标签文本 `{label}`
- 可选的移除按钮: `<button className="ml-0.5 hover:opacity-70" onClick={onRemove}>` ✕

**尺寸规范**
- sm: `px-1.5 py-0.5 text-[10px]`
- md: `px-2 py-0.5 text-xs`（默认）
- 圆角: `rounded-full`（始终为药丸形）

**颜色映射**
- blue: `bg-[#EEF2FF] text-[#4F46E5]` / dark `bg-[#6366F11A] text-[#A5B4FC]`
- green: `bg-[#ECFDF5] text-[#059669]` / dark `bg-[#0596691A] text-[#6EE7B7]`
- purple: `bg-[#F5F3FF] text-[#7C3AED]` / dark `bg-[#7C3AED1A] text-[#C4B5FD]`
- orange: `bg-[#FFFBEB] text-[#D97706]` / dark `bg-[#D977061A] text-[#FCD34D]`
- rose: `bg-[#FFF1F2] text-[#E11D48]` / dark `bg-[#E11D481A] text-[#FDA4AF]`
- sky: `bg-[#F0F9FF] text-[#0284C7]` / dark `bg-[#0284C71A] text-[#7DD3FC]`

**状态变体**
- default: 按 color 显示对应颜色组合。
- removable: 文字右侧显示 ✕ 按钮，hover 时颜色加深。
- 无 active/disabled/focus/loading 状态（标签静态展示）。

## Component: IPCard

**Key Constraints**
- 二创区页面/组件：依托于 ips.category 进行展示或跳转。
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface IPCardProps {
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

## Component: IPCategoryTabs

**Key Constraints**
- 二创区页面/组件：依托于 ips.category 进行展示或跳转。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface IPCategoryTabsProps {
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

## Component: ContentDetail

**Key Constraints**
- 根据 `contentType` 渲染不同的内容展示器（MarkdownRenderer / SheetMusicViewer 等）。
- 标题、作者信息、分类标签在一体化卡片中展示。
- 遵守全局扁平化无阴影设计规范。

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
    mediaUrls?: string[];
    createdAt: string;
    updatedAt?: string;
    sourceOriginalId?: number;
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
- 内容区: `p-6`
  - Markdown 正文渲染（MarkdownRenderer）
  - 乐谱渲染（SheetMusicViewer）— 仅 `contentType='sheet_music'`
  - 媒体展示（图片/视频/音频播放器）— 根据 contentType 选择
- 底部操作栏: `<ReactionBar />` 组件

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

## Component: MarkdownRenderer

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，使用 1px border 容器。
- 内容类型为 mod/prompt 时显示「一键部署」按钮（需 `agent_enabled=true`）。

**Props 接口**
```ts
interface ReactionBarProps {
  className?: string;
  contentId: number;
  contentType: string;
  likes: number;
  dislikes: number;
  favorites: number;
  downloads: number;
  userReaction?: 'like' | 'dislike' | null;
  isFavorited: boolean;
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

**状态变体**
- default: 显示所有操作按钮及其计数。
- liked: 点赞按钮 accent 色高亮，再次点击取消。
- disliked: 点踩按钮 muted 色，与点赞互斥。
- favorited: 收藏按钮 destructive 色高亮，再次点击取消。
- disabled: `opacity-50 cursor-not-allowed` + tooltip 提示原因。
- loading: 对应按钮内嵌 Spinner 替换图标。

**响应式行为**
- 移动 (≤700px): 按钮无文字仅图标，紧凑排列 `gap-0.5`。
- 平板+ (≥701px): 图标 + 文字显示，`gap-1`。

## Component: CommentSection

**Key Constraints**
- 信誉分 < 3 用户：发布/评论/点赞按钮 disabled，hover tooltip 提示「信誉分不足」。
- 遵守全局扁平化无阴影设计规范，使用 1px border 容器。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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

**Props 接口**
```ts
interface FileUploaderProps {
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
- 支持文件拖拽（Drag & Drop）与点击上传。异步上传有进度条反馈。

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
- 遵守全局扁平化无阴影设计规范。
- 居中展示，图标 + 标题 + 说明 + 可选 CTA，不留空白区域。
- 无边框容器，使用纯文本 + 图标布局。

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
- 外层容器: `<div className="flex flex-col items-center justify-center py-16 px-4 text-center">`
- 图标区: `<div className="text-fg-muted mb-4">{icon}</div>` — 64x64 居中图标（默认使用 Inbox 或对应主题图标）
- 标题: `<h3 className="text-base font-medium text-foreground mb-2">{title}</h3>`
- 说明: `<p className="text-sm text-fg-muted max-w-sm">{description}</p>`
- CTA: 可选 `<Link href={action.href} className="mt-4 inline-flex items-center gap-2 px-4 py-2 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:opacity-90 transition-opacity">`

**尺寸规范**
- 图标: 64x64 (w-16 h-16)
- 标题: `text-base` (16px) 加粗
- 说明: `text-sm` (14px)
- CTA 高度: `h-9` (36px)

**状态变体**
- default: 图标 + 标题 + 说明 + 可选 CTA。
- 无 hover/active/disabled/loading 状态（静态展示，CTA 按钮遵循 Button 规范）。

## Component: ConfirmModal

**Key Constraints**
- Modal 是唯一允许 `shadow-md` 的组件（全局无阴影例外）。
- 遮罩层 `bg-black/50 backdrop-blur-sm`。
- ESC 和点击遮罩关闭。

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
- 遮罩: `<div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm" onClick={onClose} />`
- Modal 容器: `<div className="fixed inset-0 z-50 flex items-center justify-center p-4">`
- 卡片: `<div className="bg-canvas-default rounded-lg shadow-md max-w-sm w-full mx-auto p-6">`
- 标题: `<h2 className="text-lg font-medium text-foreground mb-2">{title}</h2>`
- 消息: `<p className="text-sm text-fg-muted mb-6">{message}</p>`
- 按钮行: `<div className="flex justify-end gap-3">`
  - 取消按钮: `<button className="px-4 py-2 text-sm font-medium text-foreground bg-canvas-default border border-border-default rounded-md hover:bg-canvas-subtle">`
  - 确认按钮: variant 为 destructive 时使用 `bg-destructive text-destructive-foreground`，default 使用 `bg-primary text-primary-foreground`

**尺寸规范**
- Modal 最大宽度: 384px (`max-w-sm`)
- 内间距: `p-6` (24px)
- 按钮高度: `h-9` (36px)

**状态变体**
- default: 白色背景 + 确认/取消按钮。
- destructive: 确认按钮使用危险色 (`bg-destructive`)。
- loading: 确认按钮内嵌 Spinner，取消按钮 disabled。
- 遮罩点击: 关闭 Modal。
- ESC: 关闭 Modal。

## Component: AgentChatWidget

**Key Constraints**
- 只有 config 中 `agent.web_agent_enabled=true` 且用户已登录才可见该功能。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface AgentChatWidgetProps {
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

## Component: UploadAssistPanel

**Key Constraints**
- 必须读取 config.yaml 的文件大小限制（视频≤300MB, 图片≤20MB, 文本≤10MB等）。
- 破坏性操作（如放弃上传或退出）需弹 ConfirmModal。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface UploadAssistPanelProps {
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

## Component: ComplianceCheckBadge

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface ComplianceCheckBadgeProps {
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

## Component: UsageGuidePanel

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface UsageGuidePanelProps {
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

## Component: SearchAgentInput

**Key Constraints**
- 只有 config 中 `agent.web_agent_enabled=true` 且用户已登录才可见该功能。
- 必须引入 SearchAgentInput 进行自然语言检索，失败时降级。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface SearchAgentInputProps {
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

## Component: FollowButton

**Key Constraints**
- FollowButton 未登录时：点击跳转 `/login`，不显示已关注状态。
- 遵守全局扁平化无阴影设计规范。

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
- 已关注态: `border border-border-default bg-canvas-default text-foreground px-4 py-2 hover:bg-canvas-subtle hover:text-destructive hover:border-destructive` — 显示 "已关注"（hover 显示 "取消关注"）

**尺寸规范**
- md: `h-9 px-4 py-2 text-sm`（默认）
- sm: `h-7 px-3 py-1 text-xs`

**状态变体**
- default: 未关注时 primary 色按钮；已关注时 outline 按钮。
- hover 未关注: `opacity-90` 加深。
- hover 已关注: 背景变 subtle + 边框/文字变 destructive 色（提示取消关注）。
- loading: 按钮内嵌 Spinner，文字变为处理中。
- disabled: `opacity-50 cursor-not-allowed`（信誉分不足或未登录）。
- 未登录: 点击跳转 `/login`，无 loading 状态。

## Component: NotificationDropdown

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface NotificationListProps {
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

## Component: ConversationList

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface ConversationListProps {
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

## Component: ChatWindow

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface ChatWindowProps {
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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

## Page: /studio 创作者工作室

**Key Constraints**
- 所有 `/studio/*` 子页面共享 `StudioLayout`（侧边栏 + 主内容区）。
- 需要登录，未登录 redirect `/login`。
- 参考 B 站创作中心和 Reddit 创作者后台设计。
- 遵守全局扁平化无阴影设计规范。
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
- 发布表单比原创区多 `ip_id`（必填）和 `source_original_id`（可选）。
**视觉结构**
- 步骤 1：标题「选择二创内容类型」+ ContentTypeGrid
- 步骤 2：标题「发布二创 — [类型名]」+ 表单
  - 标题 → 描述 → 文件上传 → **IP 搜索选择器**（必填）→ **来源原创搜索器**（可选）→ 标签 → 权限开关 → 提交

**核心组件清单**
- `ContentTypeGrid`
- `IPSelector`（IP 搜索下拉，必填项）
- `OriginalSourceSelector`（来源原创搜索，可选）
- 其余与发布原创相同
**布局规范**
- 同发布原创，额外插入 IP 选择器行和来源原创搜索器行。
**交互细节**
- IP 未选择时提交按钮 disabled + 行内提示「请选择一个 IP」。
- 来源原创不存在/已删除时搜索无结果，可留空。
- 发布成功 → Toast + redirect `/studio/contents`。

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

## Page: /messages 通知中心（增强 -- Task 114-116）

> 更新已有 `/messages` 页面规格，增加通知铃铛、未读计数、私信 SSE 实时消息功能。

**新增/更新的核心组件**
- `NotificationBell`（通知铃铛，置入 Header，Task 115）
- `NotificationList`（通知列表，已有但需增加类型：comment、like、follow、system、mention、appeal_result、content_status）
- `SSENotificationProvider`（SSE 实时推送 Provider，5 分钟间隔轮询兜底）

**通知铃铛 -- NotificationBell 规格**
- 位置：Header 右侧用户菜单前，图标 Bell + 未读计数 Badge（红色圆点或数字）
- Badge 逻辑：`unread_count > 0` 时显示，`unread_count > 99` 时显示 `99+`
- 点击：跳转 `/messages?tab=notifications`
- 轮询：`GET /api/v1/notifications/unread-count`，5 分钟间隔
- SSE：连接 `/api/v1/notifications/stream`（EventSource），实时更新未读数

**通知列表增强规格**
- 通知类型图标：comment / like / follow / system / mention / appeal_result / content_status
- 通知内容：行动者头像 + 行动者用户名 + 通知文本 + 相对时间
- 单条操作：hover 显示「标记已读」和「删除」按钮
- 批量操作：顶部「全部标记已读」按钮
- 分页：游标分页，滚动加载

**私信增强规格（Task 116）**
- 对话列表：左侧展示最近对话（对方头像 + 用户名 + 最新消息预览 + 未读数 Badge）
- 对话窗口：右侧展示消息历史（按时间排列，自己消息靠右 bg-accent-emphasis，对方消息靠左 bg-canvas-subtle）
- 消息输入：底部固定输入框 + 发送按钮，Enter 发送，Shift+Enter 换行
- SSE 实时：使用 EventSource 订阅 `/api/v1/messages/stream` 实时接收新消息
- 响应式：移动端单栏切换（列表或对话窗口全宽），平板和 PC 左右分栏

## Page: /forgot-password 忘记密码（Task 117）

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
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

## Page: /collections/[collectionId] 收藏集详情（Task 122-124）

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。
- 私有收藏集仅创建者可见；公开收藏集所有登录用户可浏览。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：收藏集信息卡片 + 内容筛选 + 瀑布流内容列表

**核心组件清单**
- `Header`
- `CollectionInfoCard`（封面、标题、描述、作者、内容数、可见性 Badge）
- `ContentTypeFilter`（按 content_type 筛选：全部/图文/视频/音频/模板...）
- `ContentCard`
- `MasonryGrid`
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：1280px，居中
- 收藏集信息卡片 -> 筛选栏 -> 瀑布流内容
- 区域间距（block）：24px (`space-y-6`)
- 元素间距（inline）：16px (`gap-4`)

**状态变体**
- default: 收藏集信息 + 筛选标签 + 内容瀑布流。
- loading: 骨架屏（Skeleton 灰色块）。
- empty: 收藏集信息 + "收藏集为空" EmptyState（创建者显示「去添加内容」CTA）。
- error: 收藏集不存在 404 EmptyState。
- 特殊状态：未登录访问私有收藏集 -> 403 EmptyState。

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
- 筛选标签点击：切换 content_type 过滤（`router.push` 更新 URL query）。
- 创建者操作：收藏集信息区域显示「编辑」和「删除」按钮（删除需 ConfirmModal）。
- 内容卡片右下角：创建者视角显示「移除」按钮（小号，低视觉权重）。
- 添加内容：仅在创建者视角，内容详情页 ReactionBar 区显示「添加到收藏集」按钮。

## Page: /user/[userId]/collections 用户收藏集列表（Task 122-123）

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。
- 私有收藏集仅创建者可见。

**视觉层级**
- 顶部区域：导航栏 `h-[var(--header-h)]`，背景 `bg-canvas-default`，底边框 `border-b border-border`
- 主容器：居中最大宽度，页面背景 `bg-canvas-subtle`
- 内容模块：收藏集网格列表

**核心组件清单**
- `Header`
- `CollectionCard`（封面缩略图 + 标题 + 内容数 + 可见性 Badge）
- `EmptyState`
- `LoadingSpinner`

**布局规范**
- 页面最大宽度：960px，居中
- 收藏集卡片网格：`grid grid-cols-2 md:grid-cols-3 gap-4`
- 区域间距（block）：24px (`space-y-6`)

**状态变体**
- default: 收藏集网格列表 + 「新建收藏集」按钮（仅自己的主页显示）。
- loading: 骨架屏（Skeleton 灰色块网格）。
- empty: "还没有收藏集" EmptyState + 新建 CTA（自己的主页）或 空状态文本（他人主页）。
- error: Toast 右上角报错。
- 特殊状态：他人主页只显示公开收藏集；自己主页显示所有收藏集。

**响应式规则**
- 移动 (<=700px): 网格 2 列。
- 平板 (<=1100px): 网格 3 列。
- PC (>1100px): 网格 3 列，最大宽度 960px。

**暗色模式适配**
- 背景色 token: `canvas-default` -> `canvas-default.dark`
- 边框色 token: `border-default` -> `border-default.dark`
- 文字色 token: `foreground` -> `foreground.dark`

**交互细节**
- 收藏集卡片点击：跳转 `/collections/[id]`。
- 「新建收藏集」按钮：弹出 Modal（标题输入 + 公开/私有选择 + 创建按钮）。
- 卡片 hover：轻微上浮 `hover:-translate-y-1` + border 颜色加深。

## Component: CollectionCard 收藏集卡片（Task 122-123）

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
``ts
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
``

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

## Component: DownloadButton 下载按钮（Task 121）

**Key Constraints**
- 信誉分 < 3 用户：下载按钮 disabled，hover tooltip 提示「信誉分不足」。
- 封禁用户：下载按钮 disabled，tooltip 提示「账号已被封禁」。
- 遵守全局扁平化无阴影设计规范。
- 绝无 box-shadow（Indigo 扁平风），使用 1px border。

**Props 接口**
``ts
interface DownloadButtonProps {
  className?: string;
  contentId: number;
  contentTitle: string;
  contentType: string;
  isDisabled?: boolean;
  disableReason?: string;
  onDownloadComplete?: () => void;
}
``

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

## Component: AddToCollectionModal 添加到收藏集（Task 122-124）

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，Modal 使用 `shadow-md`（唯一允许阴影的组件）。
- 同一收藏集内不允许重复添加同一内容。

**Props 接口**
``ts
interface AddToCollectionModalProps {
  className?: string;
  contentId: number;
  contentTitle: string;
  isOpen: boolean;
  onClose: () => void;
  onAdded?: (collectionId: number) => void;
}
``

**视觉结构**
- 遮罩: `<div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm">`
- Modal 容器: `<div className="bg-canvas-default rounded-lg shadow-md max-w-md w-full mx-auto p-6">`
- 标题: `<h2 className="text-lg font-medium text-foreground">` 添加到收藏集
- 收藏集列表: 每项 `button border border-border-default rounded-md p-3 hover:bg-canvas-subtle flex items-center gap-3`
  - 左侧: 收藏集标题 + 内容数
  - 右侧: 已添加 check 标记 或 添加 plus 按钮
- 底部: 「新建收藏集」按钮 + 「取消」按钮

**尺寸规范**
- Modal 最大宽度: 448px (`max-w-md`)
- 内间距: `p-6` (24px)
- 列表项间距: `gap-2` (8px)
- 字号: `text-lg` (18px) 标题，`text-sm` (14px) 列表项，`text-xs` (12px) 辅助

**状态变体**
- default: 收藏集列表展示，可点击添加。
- loading: 添加中 Spinner。
- error: 数据加载失败 Toast。
- added: 列表项显示 check 已添加标记，不可重复点击。
- full: 收藏集列表为空时展示 EmptyState + 创建 CTA。

**响应式行为**
- 移动: Modal 全宽（m-4），`max-w-full`。
- 平板/PC: `max-w-md` 居中。

**暗色模式适配**
- 遮罩: `bg-black/50` 不变。
- Modal 容器: `bg-canvas-default.dark` (token 自动映射)。

**关键交互**
- 点击收藏集 -> `POST /api/v1/collections/:id/items` -> 成功后该项显示 check
- 点击「新建收藏集」-> 弹出新建收藏集子 Modal（标题 + 公开/私有选择）
- 重复添加：前端灰显 + 禁止点击，后端 UNIQUE 约束兜底返回 409
- ESC 或点击遮罩 -> 关闭 Modal

## Component: LoadingSpinner 加载旋转器

**Key Constraints**
- 遵守全局扁平化无阴影设计规范。
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
- 遵守全局扁平化无阴影设计规范。
- 仅用于内容加载占位，不可交互。

**Props 接口**
```ts
interface SkeletonCardProps {
  className?: string;
  zone?: 'original' | 'fanwork';
  count?: number; // 占位卡片数量，默认 1
}
```

**视觉结构**
- 外层容器: `<div className="rounded-md bg-canvas-default overflow-hidden">`
- 封面区: `<div className="bg-canvas-subtle animate-pulse" style={{ aspectRatio: zone === 'fanwork' ? '3/4' : 'auto', minHeight: 150 }} />`
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
- 遵守全局扁平化无阴影设计规范（Toast 容器无阴影只使用 1px border）。

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
- 容器: `<div className="flex items-center gap-3 px-4 py-3 border border-border-default rounded-md bg-canvas-default text-sm">`
- 图标: type 对应图标（success=check-circle, error=x-circle, info=info, warning=alert-triangle）
- 文字: `flex-1 text-foreground`
- 关闭按钮: `<button className="text-fg-muted hover:text-foreground">`

**状态变体**
- success: `border-border` + 绿色图标 (`text-green-600`)
- error: `border-border-destructive` + 红色图标 (`text-destructive`)
- info: `border-border` + 蓝色图标 (`text-accent-emphasis`)
- warning: `border-border` + 橙色图标 (`text-orange-500`)
- 入场动效: `animate-in slide-in-from-right duration-200`

## Component: Footer 页脚

**Key Constraints**
- 遵守全局扁平化无阴影设计规范。
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
- 遵守全局扁平化无阴影设计规范，使用 1px border。
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
- 遵守全局扁平化无阴影设计规范。
- 下拉选择器，用于内容列表的排序切换。

**Props 接口**
```ts
interface SortSelectProps {
  className?: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
}
```

**视觉结构**
- 外层容器: `<select className="border border-border-default rounded-md bg-canvas-default px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-accent-emphasis">`
- 每个选项: `<option value={value} className="text-foreground bg-canvas-default">`

**状态变体**
- default: 选中当前排序值。
- focus: `ring-2 ring-accent-emphasis`。
