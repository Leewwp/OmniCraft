# OmniCraft UI 设计规格

> ⚠️ **存根警示（第七轮修复追加）**：本文件为 UI spec 占位存根，内容由模板批量生成，**不得作为权威设计来源**。以下两类信息存在已知偏差，请以 `UI Design.md` 为准：
>
> 1. **各页面「核心组件清单」多数为模板化默认值**（如 `/user/[userId]`、`/content/[contentId]`、`/login` 等仅列出 `EmptyState/LoadingSpinner`），未反映真实组件组合；请参考对应 Task 的 `ui_spec_ref` 引用列表和 `UI Design.md` 页面小节。
> 2. **「响应式规则」段对表单页（登录/注册/设置等）错误套用了"瀑布流 2/3/4 列"模板**，这些页面无瀑布流；应以 `UI Design.md §五补充设计约束` 的断点规则为准。
>
> Agent 在根据本文件生成代码时，**必须同时读取 `UI Design.md` 对应页面小节**验证组件清单与响应式行为；仅 `## Global Design Tokens`、`## Global Interaction Patterns`、`## Component: Header`、`## Component: FacetedSearchSidebar` 四节内容可信（Gemini 已精修）。本文件计划由 Gemini 在 P0 实现启动前整体重生成，届时本警示块将移除。

---

<!-- Gemini 生成内容从此行开始 -->

## Global Design Tokens

- **颜色 token**：直接复用 `architecture.md` §10.4 的 `colors.canvas / border / fg / accent / tag`，配置 light/dark。
- **字体**：font-family: `-apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial`，无自定义字体。字号阶梯：text-xs 12 / text-sm 14 / text-base 16 / text-lg 18 / text-xl 20 / text-2xl 24 / text-3xl 30
- **间距阶梯**：space-1 4px / space-2 8px / space-3 12px / space-4 16px / space-6 24px / space-8 32px / space-12 48px
- **圆角**：rounded-sm 4 / rounded 6（默认）/ rounded-md 8 / rounded-lg 12（标签）/ rounded-full
- **动效**：transition duration-150 ease-out（默认）；duration-300 ease-in-out（Modal/Sheet）
- **阴影**：**强制 box-shadow:none**，仅 Modal/Popover/Dropdown 用 `shadow-md`

## Global Interaction Patterns

- **Hover**: `cursor-pointer` + 颜色加深一档（border / bg）。
- **Loading**: 骨架屏（Skeleton 灰色块）+ 按钮内联 Spinner，禁止全屏遮罩。
- **Error**: Toast（右上角，自动消失 4s）+ 内联红字提示（表单字段下方）。
- **Empty**: EmptyState 组件（图标 + 标题 + 说明 + 可选 CTA），不留空白区域。
- **Disabled**: `opacity-50` + `cursor-not-allowed` + 禁用 hover/click。

## Component: Header

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface HeaderProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Page: / 首页

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /search 搜索页

**Key Constraints**
- 必须引入 SearchAgentInput 进行自然语言检索，失败时降级。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /login 登录页

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /register 注册页

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
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
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /ip/[ipId]/[category] IP 类目内容列表

**Key Constraints**
- 二创区页面/组件：依托于 ips.category 进行展示或跳转。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /ip/[ipId]/discussions 讨论区列表

**Key Constraints**
- 二创区页面/组件：依托于 ips.category 进行展示或跳转。
- 信誉分 < 3 用户：发布/评论/点赞按钮 disabled，hover tooltip 提示「信誉分不足」。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /ip/[ipId]/discussions/[discussionId] 讨论详情

**Key Constraints**
- 二创区页面/组件：依托于 ips.category 进行展示或跳转。
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 信誉分 < 3 用户：发布/评论/点赞按钮 disabled，hover tooltip 提示「信誉分不足」。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /ip/[ipId]/discussions/new 发帖页

**Key Constraints**
- 二创区页面/组件：依托于 ips.category 进行展示或跳转。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /content/[contentId] 内容详情页

**Key Constraints**
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /original 原创区首页

**Key Constraints**
- 原创区无 IP 概念，严格按照 content_items.category (推荐|影视|游戏|文学等) 分类。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /original/[contentId] 原创内容详情

**Key Constraints**
- 原创区无 IP 概念，严格按照 content_items.category (推荐|影视|游戏|文学等) 分类。
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /user/[userId] 用户主页

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /publish 发布内容

**Key Constraints**
- 必须读取 config.yaml 的文件大小限制（视频≤300MB, 图片≤20MB, 文本≤10MB等）。
- 破坏性操作（如放弃上传或退出）需弹 ConfirmModal。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 移动 (≤700px): 单列瀑布流 2 列，隐藏侧边栏，折叠菜单。
- 平板 (≤1100px): 瀑布流 3 列，卡片尺寸自适应。
- PC (>1100px): 默认布局 4 列瀑布流，左右分布边距对齐。

**暗色模式适配**
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /settings 账号设置

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /settings/tag-groups 标签组管理

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /dashboard 创作者后台概览

**Key Constraints**
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 移动 (≤700px): 单列瀑布流 2 列，隐藏侧边栏，折叠菜单。
- 平板 (≤1100px): 瀑布流 3 列，卡片尺寸自适应。
- PC (>1100px): 默认布局 4 列瀑布流，左右分布边距对齐。

**暗色模式适配**
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /dashboard/contents 我的内容

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 移动 (≤700px): 单列瀑布流 2 列，隐藏侧边栏，折叠菜单。
- 平板 (≤1100px): 瀑布流 3 列，卡片尺寸自适应。
- PC (>1100px): 默认布局 4 列瀑布流，左右分布边距对齐。

**暗色模式适配**
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /dashboard/pr-requests PR 申请管理

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 移动 (≤700px): 单列瀑布流 2 列，隐藏侧边栏，折叠菜单。
- 平板 (≤1100px): 瀑布流 3 列，卡片尺寸自适应。
- PC (>1100px): 默认布局 4 列瀑布流，左右分布边距对齐。

**暗色模式适配**
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /dashboard/contributors 贡献者管理

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 移动 (≤700px): 单列瀑布流 2 列，隐藏侧边栏，折叠菜单。
- 平板 (≤1100px): 瀑布流 3 列，卡片尺寸自适应。
- PC (>1100px): 默认布局 4 列瀑布流，左右分布边距对齐。

**暗色模式适配**
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /dashboard/tag-suggestions 标签建议审核

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 移动 (≤700px): 单列瀑布流 2 列，隐藏侧边栏，折叠菜单。
- 平板 (≤1100px): 瀑布流 3 列，卡片尺寸自适应。
- PC (>1100px): 默认布局 4 列瀑布流，左右分布边距对齐。

**暗色模式适配**
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /judge/exam 赛博判官资质考核

**Key Constraints**
- 赛博判官业务规则：只有具有对应类型的判官权限（judge_qualifications）或通过考核才能操作。
- 信誉分必须 >= 3 才能行使众裁权利，否则禁用功能。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /judge/queue 待审内容队列

**Key Constraints**
- 赛博判官业务规则：只有具有对应类型的判官权限（judge_qualifications）或通过考核才能操作。
- 信誉分必须 >= 3 才能行使众裁权利，否则禁用功能。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /history 浏览历史

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /appeals 我的申诉

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /messages 消息中心

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /rehab 素质建设课程

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /admin/ips IP 库管理

**Key Constraints**
- 二创区页面/组件：依托于 ips.category 进行展示或跳转。
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /admin/contents 内容终审

**Key Constraints**
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /admin/users 用户管理

**Key Constraints**
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /admin/appeal 申诉处理

**Key Constraints**
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /admin/config 系统配置

**Key Constraints**
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /admin/categories 分类与标签管理

**Key Constraints**
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /admin/agent-config Agent 管理

**Key Constraints**
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 只有 config 中 `agent.web_agent_enabled=true` 且用户已登录才可见该功能。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
- 内容模块：带 1px 边框的卡片容器

**核心组件清单**
- `EmptyState`
- `LoadingSpinner`

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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
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

**Props 接口**
```ts
interface ContentCardProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: MasonryGrid

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface MasonryGridProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: TagBadge

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface TagBadgeProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: ContentDetail

**Key Constraints**
- ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示。
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface ContentDetailProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: ReactionBar

**Key Constraints**
- 信誉分 < 3 用户：发布/评论/点赞按钮 disabled，hover tooltip 提示「信誉分不足」。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface ReactionBarProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: CommentSection

**Key Constraints**
- 信誉分 < 3 用户：发布/评论/点赞按钮 disabled，hover tooltip 提示「信誉分不足」。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface CommentSectionProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: EmptyState

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface EmptyStateProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: ConfirmModal

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface ConfirmModalProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。
- 破坏性或协同操作必须包含 ConfirmModal 二次确认弹窗。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

## Component: FollowButton

**Key Constraints**
- FollowButton 未登录时：点击跳转 `/login`，不显示已关注状态。
- 组件必须保持 1px border 扁平设计，无阴影 `shadow-none`。
- 所有间距（gap/padding/margin）使用 Tailwind 类名。

**Props 接口**
```ts
interface FollowButtonProps {
  className?: string;
  data?: any;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: any) => void;
}
```

**视觉结构**
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default p-4">`
- 内部布局: 依据业务包含 Flex 纵向/横向排列，以及 `gap-3` 分隔。
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**尺寸规范**
- 默认尺寸: height 自适应，padding 16px (p-4)
- 字号: `text-sm` (14px) 主要信息，`text-xs` 辅助说明
- 间距: 元素间隙 8px (`gap-2`) 或 12px (`gap-3`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: `hover:bg-canvas.subtle` 并伴随图标颜色变深
- active: `active:bg-canvas.muted scale-95`
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 内部嵌 `Spinner` 并替换默认图标文本
- empty/error: 显示红色边框 `border-border.danger` 或局部 EmptyState

**响应式行为**
- 内部采用 Flex/Grid wrap，小屏下 `flex-col`，大屏下排成一行。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 点击行为触发传入的回调 `onAction` 或 Link 路由跳转。
- 键盘行为：支持 Tab 索引切换，Enter 选中，Esc 取消浮层。
