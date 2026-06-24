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
- 平板 (≤1100px): 左侧侧边栏 220px + 右侧瀑布流 3 列。
- PC (>1100px): 左侧侧边栏 260px + 右侧瀑布流 4 列。

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
- 支持渲染 SWR 或 SSR，并提供加载骨架 Skeleton 动画。
- 信誉分 < 3 用户：发布/评论/点赞按钮 disabled，hover tooltip 提示「信誉分不足」。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 参考小红书网页端设计：顶部 Tab 导航 + 瀑布流内容，单层导航结构，无二级内容类型筛选。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。
- 推荐 Tab 为算法驱动的个性化推送（`sort=recommended`），非手动筛选选项。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 分类 Tab 栏：Header 下方固定（sticky），高度 `h-12`，底部 1px border 分隔，Tab 项横向滚动
- 主容器：居中最大宽度 1280px，页面背景 `bg-canvas.subtle`
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
- 选中态：`text-accent.emphasis font-semibold` + 底部 2px accent 色下划线
- 未选中态：`text-fg.muted`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
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
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
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
- 移动 (≤700px): 表单全宽（margin 16px），编辑器高度 250px，上传组件全宽。
- 平板 (≤1100px): 表单最大宽度 720px，居中。
- PC (>1100px): 表单最大宽度 960px，居中，两侧留白。

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
- 密码修改/注销等破坏性操作需 ConfirmModal 二次确认。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 数据加载策略: SSR 基础页面框架，SWR/客户端流式加载动态或个性化数据列表。

## Page: /dashboard 创作者后台概览

> ⚠️ **已废弃**：本页面已迁移至 `/studio/overview`（创作者工作室数据概览）。旧路由 `/dashboard` 保留 301 重定向到 `/studio/overview`。以下规范仅供迁移参考，**以新版 Page: /studio/overview 的最新规格为准**。

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
- 移动 (≤700px): 表格水平滚动，概览卡片单列堆叠，操作按钮展开式。
- 平板 (≤1100px): 表格自适应宽度，概览卡片 2 列。
- PC (>1100px): 表格最大宽度 1280px，概览卡片 4 列。

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
- 移动 (≤700px): 表格水平滚动，操作按钮展开式。
- 平板 (≤1100px): 表格自适应宽度。
- PC (>1100px): 表格最大宽度 1280px。

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
- 移动 (≤700px): 表格水平滚动，操作按钮展开式。
- 平板 (≤1100px): 表格自适应宽度。
- PC (>1100px): 表格最大宽度 1280px。

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
- PC (>1100px): 左侧管理导航 220px + 右侧表格自适应。

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
- 每门课程仅能完成一次，最低阅读 3 分钟（180 秒）。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 后台页面：Role 必须为 admin 且二次校验拦截，包含 ConfirmModal 二次确认。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- PC (>1100px): 左侧管理导航 220px + 右侧表格自适应。

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
- PC (>1100px): 左侧管理导航 220px + 右侧表格自适应。

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
- PC (>1100px): 左侧管理导航 220px + 右侧表格自适应。

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
- PC (>1100px): 左侧管理导航 220px + 右侧表格自适应。

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
- 热更新配置，重启后恢复 config.yaml 默认值。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- PC (>1100px): 左侧管理导航 220px + 右侧配置表单 500px。

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
- PC (>1100px): 左侧管理导航 220px + 右侧分类管理区自适应。

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
- PC (>1100px): 左侧管理导航 220px + 右侧配置区自适应。

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default overflow-hidden">`（无 padding，内容填满）
- 封面区: 3:4 比例容器，`object-cover` 图片填满
- 信息区: `p-3`，标题（`text-sm font-medium line-clamp-2`）→ 作者 + IP 名行（`text-xs text-fg.muted`）→ 互动数据行（`text-xs` ❤️ + 💬）→ 标签行（最多 3 个低饱和 TagBadge）
- 图标: `<Icon className="text-fg.muted w-4 h-4" />`

**视觉结构 — 原创区卡片（zone='original'）**
- 外层容器: `<div className="rounded-md bg-canvas.default overflow-hidden cursor-pointer group">`（无 border，更干净的小红书风格）
- 封面区: 自适应高度（图片按原始宽高比，`object-cover w-full`），`max-height: 400px`，`overflow-hidden`
- 悬停遮罩: `<div className="absolute inset-0 bg-black/10 opacity-0 group-hover:opacity-100 transition-opacity" />`
- 封面缩放: `group-hover:scale-105 transition-transform duration-300`
- 信息区: `p-2`，标题（`text-sm font-medium line-clamp-2`）→ @作者（`text-xs text-fg.muted`）→ ❤️ 点赞数（`text-xs text-fg.muted`）
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
- focus: `focus:outline-none focus:ring-2 focus:ring-accent.default`
- disabled: `opacity-50 cursor-not-allowed` 禁用事件
- loading: 显示 `SkeletonCard` 占位（灰色块模拟卡片形状）
- empty: 不渲染（由父级 MasonryGrid 处理 EmptyState）

**响应式行为**
- 瀑布流列内自适应宽度，不单独控制断点。

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

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
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 滚动触发加载更多（无需手动点击）。
- 卡片点击由 ContentCard 内部 Link 处理。

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



## Component: StudioSidebar

**Key Constraints**
- 鍒涗綔鑰呭伐浣滃鐨勫彲鎶樺彔渚ц竟鏍忥紝蹇呴』鍦?`/studio/*` 甯冨眬涓娇鐢ㄣ€?- 閬靛畧鍏ㄥ眬鎵佸钩鍖栨棤闃村奖璁捐瑙勮寖锛岄鑹插紩鐢ㄩ瀹氫箟 token銆?- 灞曞紑瀹藉害 228px (`w-[228px]`)锛屾敹璧峰搴?48px (`w-12`)锛岃繃娓″姩鐢?`transition-all duration-200`銆?- 鏀惰捣鎬侊細浠呮樉绀哄浘鏍囷紝hover 鍥炬爣鏃跺湪鍙充晶寮瑰嚭 tooltip锛坄absolute left-full ml-2`锛屽欢杩?300ms锛夈€?
**Props 鎺ュ彛**
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

**瑙嗚缁撴瀯**
- 澶栧眰瀹瑰櫒: `<aside className="h-full flex flex-col bg-canvas.subtle border-r border-border.muted transition-all duration-200" style={{ width: collapsed ? 64 : 224 }}>`
- 椤堕儴鍒嗙粍鍖猴細Logo/鏍囬 + 鎶樺彔鍒囨崲鎸夐挳锛坄鈫恅 / `鈫抈 绠ご鍥炬爣锛?- 涓棿瀵艰埅鍖猴細鎸?section 鍒嗙粍娓叉煋
  - 灞曞紑鎬侊細鍒嗙粍鏍囬 (`text-xs text-fg.muted uppercase tracking-wider px-3 pt-4 pb-1`) + 椤瑰垪琛?  - 鏀惰捣鎬侊細浠呭浘鏍囷紝鍒嗙粍闂?`border-t border-border.muted my-2` 鍒嗛殧
- 姣忎釜瀵艰埅椤癸細
  - 灞曞紑鎬侊細`h-10 px-3 flex items-center gap-3 rounded-md`锛堝浘鏍?20px + 鏂囧瓧 `text-sm` + 鍙€夋湭璇?Badge锛?  - 鏀惰捣鎬侊細`h-12 flex items-center justify-center relative group`锛堜粎鍥炬爣灞呬腑 + hover tooltip锛?- 閫変腑鎬侊細`bg-accent.emphasis/10 text-accent.emphasis font-medium` + 宸︿晶 3px accent 鑹茬珫绾匡紙`border-l-3 border-accent.emphasis`锛?- 鏈€変腑鎬侊細`text-fg.muted hover:bg-canvas.default hover:text-fg.default`

**Tooltip 瑙勮寖**锛堟敹璧锋€?hover锛?- 瑙﹀彂锛歚opacity-0 group-hover:opacity-100 transition delay-300`
- 瀹氫綅锛歚absolute left-full ml-2 top-1/2 -translate-y-1/2`
- 鏍峰紡锛歚bg-canvas.default border border-border rounded-md px-3 py-1.5 text-sm text-fg.default whitespace-nowrap shadow-md z-50`

**鐘舵€佸彉浣?*
- default: 榛樿灞曞紑鎬侊紝褰撳墠璺敱瀵瑰簲椤归珮浜€変腑銆?- collapsed: 鏀惰捣鎬侊紝浠呭浘鏍囧彲瑙併€?- hover: 鏈€変腑椤?hover 鑳屾櫙鍙樹寒锛屾敹璧锋€佽Е鍙?tooltip銆?- active: 閫変腑椤?accent 鑹插乏渚ф寚绀哄櫒銆?- disabled: `opacity-50 cursor-not-allowed`锛堝鏀剁泭鏁版嵁 P1 鍗犱綅锛岀偣鍑讳笉璺宠浆锛夈€?
**鍝嶅簲寮忚涓?*
- 绉诲姩 (鈮?00px): 渚ц竟鏍忛粯璁ゆ敹璧凤紝鐐瑰嚮灞曞紑涓烘诞灞傝鐩栦富鍐呭鍖猴紙`absolute z-20 h-full`锛夛紝鐐瑰嚮涓诲唴瀹瑰尯鑷姩鏀惰捣銆?- 骞虫澘+ (鈮?01px): 渚ц竟鏍忓浐瀹氾紝灞曞紑/鏀惰捣鎸夐挳鍙銆?
**鍏抽敭浜や簰**
- 鎶樺彔鎸夐挳鐐瑰嚮 鈫?`onToggle()` 鈫?鐖剁粍浠舵洿鏂?`collapsed` 鐘舵€侊紝鍐欏叆 `localStorage: studio_sidebar_collapsed`銆?- 鏀惰捣鎬佸浘鏍?hover 300ms 鈫?tooltip 寮瑰嚭锛岄紶鏍囩Щ鍑?鈫?tooltip 娑堝け銆?- 閿洏锛歚Tab` 鍦ㄥ浘鏍囬棿绉诲姩锛宍Enter` 瀵艰埅鍒扮洰鏍囪矾鐢便€?
## Component: ContentTypeGrid

**Key Constraints**
- 鍐呭绫诲瀷閫夋嫨鍗＄墖缃戞牸锛岀敤浜?`/studio/publish/*` 鍙戝竷娴佺▼姝ラ 1銆?- 鍗＄墖鎺掑垪浠?`config.yaml > publish.type_order_original` 鎴?`publish.type_order_fanwork` 璇诲彇銆?- 閬靛畧鍏ㄥ眬鎵佸钩鍖栨棤闃村奖璁捐瑙勮寖锛岄鑹插紩鐢ㄩ瀹氫箟 token銆?
**Props 鎺ュ彛**
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

**瑙嗚缁撴瀯**
- 澶栧眰瀹瑰櫒: `<div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 p-6">`
- 姣忓紶鍗＄墖: `<button className="border border-border.rounded-lg p-6 text-center hover:border-accent.emphasis hover:bg-canvas.subtle transition-all cursor-pointer group hover:-translate-y-1">`
  - 鍥炬爣: `<span className="text-4xl mb-3 block">`锛坋moji 鍥炬爣 40px锛?  - 鏍囬: `<h3 className="text-base font-medium text-fg.default mb-1">`
  - 鎻忚堪: `<p className="text-xs text-fg.muted">`

**鐘舵€佸彉浣?*
- default: 鐧借壊鍗＄墖 + 1px border銆?- hover: border accent 鑹?+ 杞诲井涓婃诞 `-translate-y-1` + 鑳屾櫙鍙樻祬銆?- active: `scale-95` 鐐瑰嚮鍙嶉銆?
**鍝嶅簲寮忚涓?*
- 绉诲姩 (鈮?00px): 2 鍒?(`grid-cols-2`)锛屽崱鐗?padding `p-4`銆?- 骞虫澘 (鈮?100px): 3 鍒?(`grid-cols-3`)銆?- PC (>1100px): 4 鍒?(`grid-cols-4`)锛屽崱鐗?padding `p-6`銆?
**鍏抽敭浜や簰**
- 鐐瑰嚮鍗＄墖 鈫?`onSelect(type)` 鈫?鐖剁粍浠跺垏鎹㈠埌鍙戝竷琛ㄥ崟锛堟楠?2锛夛紝zone + content_type 閿佸畾銆?- 閿洏锛歚Tab` 鍦ㄥ崱鐗囬棿绉诲姩锛宍Enter` 閫変腑銆?- 鍙戝竷琛ㄥ崟椤堕儴鎻愪緵銆屸啇 杩斿洖閫夋嫨绫诲瀷銆嶆寜閽€?
## Page: /studio 鍒涗綔鑰呭伐浣滃

**Key Constraints**
- 鎵€鏈?`/studio/*` 瀛愰〉闈㈠叡浜?`StudioLayout`锛堜晶杈规爮 + 涓诲唴瀹瑰尯锛夈€?- 闇€瑕佺櫥褰曪紝鏈櫥褰?redirect `/login`銆?- 鍙傝€?B 绔欏垱浣滀腑蹇冨拰灏忕孩涔﹀垱浣滆€呭悗鍙拌璁°€?- 閬靛畧鍏ㄥ眬鎵佸钩鍖栨棤闃村奖璁捐瑙勮寖銆?
**瑙嗚灞傜骇**
- 椤堕儴锛欻eader `h-13`锛屽簳杈规 `border-b border-border.muted`
- 涓诲鍣細`flex h-[calc(100vh-52px)]`锛圚eader 涓嬫柟鍏ㄩ珮锛夛紝鑳屾櫙 `bg-canvas.subtle`
- 宸︿晶锛歋tudioSidebar锛堝睍寮€ `w-[228px]` / 鏀惰捣 `w-12`锛夛紝鍙充晶 1px border 鍒嗛殧
- 鍙充晶锛氫富鍐呭鍖?`flex-1 overflow-y-auto`锛宲adding `p-6`

**鏍稿績缁勪欢娓呭崟**
- `Header`
- `StudioLayout`锛堝叡浜竷灞€瀹瑰櫒锛?- `StudioSidebar`锛堝彲鎶樺彔渚ц竟鏍忥級
- `ContentTypeGrid`锛堝唴瀹圭被鍨嬮€夋嫨锛?- `FileUploader`, `MarkdownEditor`锛堝鐢級
- `FollowerAnalytics`锛堢矇涓濆垎鏋愬浘琛級

**甯冨眬瑙勮寖**
- 渚ц竟鏍?+ 涓诲唴瀹瑰尯姘村钩鎺掑垪锛宍flex` 甯冨眬
- 涓诲唴瀹瑰尯鍨傜洿婊氬姩锛坄overflow-y-auto`锛夛紝鏃犲灞?card 鍖呰９

**鐘舵€佸彉浣?*
- default: 榛樿杩涘叆 `/studio/publish/original`锛圕ontentTypeGrid锛夈€?- loading: 涓诲唴瀹瑰尯楠ㄦ灦灞忋€?- empty: 鏃犲唴瀹?鏃犳暟鎹椂瀵瑰簲 EmptyState銆?- error: Toast 鍙充笂瑙掓姤閿欍€?- 鐗规畩鐘舵€侊細淇¤獕鍒?< 3 鏃跺彂甯冨叆鍙?disabled + tooltip銆屼俊瑾夊垎涓嶈冻锛屾殏鏃犳硶鍙戝竷銆嶃€?
**鍝嶅簲寮忚鍒?*
- 绉诲姩 (鈮?00px): 渚ц竟鏍忛粯璁ゆ敹璧凤紙48px锛夛紝鐐瑰嚮灞曞紑涓烘诞灞傝鐩栵紱涓诲唴瀹瑰尯 `p-4`銆?- 骞虫澘 (鈮?100px): 渚ц竟鏍忓睍寮€锛屼富鍐呭鍖鸿嚜閫傚簲銆?- PC (>1100px): 渚ц竟鏍忓睍寮€ 228px锛屾瑙?鍐呭绠＄悊 max-w 1280px锛屽彂甯冭〃鍗?max-w 960px銆?
**浜や簰缁嗚妭**
- 渚ц竟鏍忔姌鍙犵姸鎬佹寔涔呭寲鍒?`localStorage: studio_sidebar_collapsed`銆?- 鍙戝竷娴佺▼锛氱被鍨嬮€夋嫨 鈫?鍙戝竷琛ㄥ崟锛堝悓椤电姸鎬佸垏鎹紝涓嶆敼鍙?URL锛夈€?- 鍙戝竷鎴愬姛鍚庤烦杞?`/studio/contents`銆?- 鏁版嵁鍔犺浇锛歋WR 瀹㈡埛绔姞杞芥瑙堢粺璁°€佺矇涓濊秼鍔跨瓑涓€у寲鏁版嵁銆?
## Page: /studio/publish/original 鍙戝竷鍘熷垱

**Key Constraints**
- 姝ラ 1 鏄剧ず ContentTypeGrid锛堝師鍒涘尯 7 绉嶇被鍨嬶紝鎸?config.yaml > publish.type_order_original 鎺掑簭锛夈€?- 姝ラ 2 鏄剧ず鍙戝竷琛ㄥ崟锛寊one='original' + content_type 閿佸畾锛宑ategory 蹇呭～銆?- 浜屽垱涓撳睘瀛楁锛坕p_id銆乻ource_original_id锛変笉娓叉煋銆?- 鐮村潖鎬ф搷浣滈渶 ConfirmModal銆?
**瑙嗚灞傜骇**
- 姝ラ 1锛氬眳涓爣棰樸€岄€夋嫨鍘熷垱鍐呭绫诲瀷銆? ContentTypeGrid 缃戞牸
- 姝ラ 2锛氭爣棰樸€屽彂甯冨師鍒?鈥?[绫诲瀷鍚峕銆? 鍙戝竷琛ㄥ崟锛坢ax-w 960px 灞呬腑锛?
**鏍稿績缁勪欢娓呭崟**
- `ContentTypeGrid`
- `FileUploader`, `MarkdownEditor`, `TagInput`
- `ComplianceCheckBadge`, `UploadAssistPanel`
- `ConfirmModal`锛堟斁寮冪‘璁わ級

**甯冨眬瑙勮寖**
- 鍙戝竷琛ㄥ崟鍨傜洿鎺掑垪锛氭爣棰?鈫?鎻忚堪/Markdown 鈫?鏂囦欢涓婁紶 鈫?鍒嗙被閫夋嫨 鈫?鏍囩 鈫?鏉冮檺寮€鍏?鈫?鎻愪氦鎸夐挳

**鐘舵€佸彉浣?*
- default: 姝ラ 1 绫诲瀷閫夋嫨缃戞牸銆?- form: 姝ラ 2 鍙戝竷琛ㄥ崟銆?- loading: 鎻愪氦鎸夐挳鍐呭祵 Spinner銆?- error: 琛屽唴绾㈠瓧 + Toast銆?- 鐗规畩鐘舵€侊細淇¤獕鍒嗕笉瓒虫樉绀烘嫤鎴彁绀恒€?
**鍝嶅簲寮忚鍒?*
- 绉诲姩 (鈮?00px): 琛ㄥ崟鍏ㄥ `p-4`锛岀紪杈戝櫒楂樺害 250px銆?- 骞虫澘 (鈮?100px): 琛ㄥ崟 max-w 720px 灞呬腑銆?- PC (>1100px): 琛ㄥ崟 max-w 960px 灞呬腑銆?
**浜や簰缁嗚妭**
- 姝ラ 1 鈫?2锛歚onSelect(type)` 鍒囨崲鐘舵€併€?- 姝ラ 2 鈫?1锛氶《閮ㄣ€屸啇 杩斿洖閫夋嫨绫诲瀷銆嶆寜閽紙鏈夋湭淇濆瓨鍐呭鏃跺脊 ConfirmModal锛夈€?- 鍙戝竷鎴愬姛 鈫?Toast + redirect `/studio/contents`銆?
## Page: /studio/publish/fanwork 鍙戝竷浜屽垱

**Key Constraints**
- 绫讳技鍙戝竷鍘熷垱锛孋ontentTypeGrid 鏄剧ず浜屽垱鍖?8 绉嶇被鍨嬶紙鎸?config.yaml > publish.type_order_fanwork 鎺掑簭锛夈€?- 鍙戝竷琛ㄥ崟姣斿師鍒涘尯澶?`ip_id`锛堝繀濉級鍜?`source_original_id`锛堝彲閫夛級銆?
**瑙嗚缁撴瀯**
- 姝ラ 1锛氭爣棰樸€岄€夋嫨浜屽垱鍐呭绫诲瀷銆? ContentTypeGrid
- 姝ラ 2锛氭爣棰樸€屽彂甯冧簩鍒?鈥?[绫诲瀷鍚峕銆? 琛ㄥ崟锛?  - 鏍囬 鈫?鎻忚堪 鈫?鏂囦欢涓婁紶 鈫?**IP 鎼滅储閫夋嫨鍣?*锛堝繀濉級 鈫?**鏉ユ簮鍘熷垱鎼滅储鍣?*锛堝彲閫夛級 鈫?鏍囩 鈫?鏉冮檺寮€鍏?鈫?鎻愪氦

**鏍稿績缁勪欢娓呭崟**
- `ContentTypeGrid`
- `IPSelector`锛圛P 鎼滅储涓嬫媺锛屽繀濉」锛?- `OriginalSourceSelector`锛堟潵婧愬師鍒涙悳绱紝鍙€夛級
- 鍏朵綑涓庡彂甯冨師鍒涚浉鍚?
**甯冨眬瑙勮寖**
- 鍚屽彂甯冨師鍒涳紝棰濆鎻掑叆 IP 閫夋嫨鍣ㄨ鍜屾潵婧愬師鍒涙悳绱㈠櫒琛屻€?
**浜や簰缁嗚妭**
- IP 鏈€夋嫨鏃舵彁浜ゆ寜閽?disabled + 琛屽唴鎻愮ず銆岃閫夋嫨涓€涓?IP銆嶃€?- 鏉ユ簮鍘熷垱涓嶅瓨鍦?宸插垹闄ゆ椂鎼滅储鏃犵粨鏋滐紝鍙暀绌恒€?- 鍙戝竷鎴愬姛 鈫?Toast + redirect `/studio/contents`銆?
## Page: /studio/overview 鏁版嵁姒傝

**Key Constraints**
- 鍙傝€?B 绔欏垱浣滀腑蹇冮椤碉細缁熻鍗＄墖 + 瓒嬪娍鍥?+ 鎺掕 + 寰呭姙浜嬮」銆?- 鏁版嵁浠庡涓?API 鑱氬悎銆?
**瑙嗚缁撴瀯**
- 缁熻鍗＄墖琛岋細4 鍗＄墖锛堟€诲唴瀹?鎬昏闂?鎬荤偣璧?绮変笣锛夛紝姣忓崱鐗囧惈鐜瘮鍙樺寲鐧惧垎姣?+ 涓婂崌/涓嬮檷绠ご
- 璁块棶閲忚秼鍔匡細杩?30 澶?recharts LineChart锛屽叏瀹藉崱鐗?- 涓嬫柟鍙屾爮锛氬乏銆屽唴瀹规帓琛?Top 5銆嶏紝鍙炽€屽緟澶勭悊浜嬮」銆?
**鏍稿績缁勪欢娓呭崟**
- `StatsCard`锛堝惈鍙樺寲鐧惧垎姣旓級
- `ViewsTrendChart`锛坮echarts LineChart锛?- `ContentRankList`锛圱op 5 鍒楄〃锛?- `PendingTasksCard`锛堝緟澶勭悊 PR/鏍囩寤鸿/涓炬姤鏁帮級

**甯冨眬瑙勮寖**
- 缁熻鍗＄墖锛歚grid grid-cols-2 lg:grid-cols-4 gap-4`
- 瓒嬪娍鍥惧崱鐗囷細鍏ㄥ锛岄珮搴?300px
- 涓嬫柟鍙屾爮锛歚grid grid-cols-1 lg:grid-cols-2 gap-6`

**鐘舵€佸彉浣?*
- default: 鏁版嵁姝ｅ父锛屽浘琛ㄥ畬鏁淬€?- loading: 鍗＄墖楠ㄦ灦灞?+ 鍥捐〃鍖虹伆鑹插潡銆?- empty: 鏂扮敤鎴枫€屾殏鏃犳暟鎹紝蹇幓鍙戝竷绗竴鏉″唴瀹瑰惂銆岴mptyState + CTA銆?- error: Toast 鎶ラ敊 + 鍗曞崱鐗囧唴鑱旈敊璇彁绀恒€?
**鍝嶅簲寮忚鍒?*
- 绉诲姩: 缁熻鍗＄墖 2 鍒楋紝瓒嬪娍鍥?100%锛屼笅鏂瑰崟鍒椼€?- 骞虫澘+: 缁熻鍗＄墖 4 鍒椼€?- PC: max-w 1280px銆?
**浜や簰缁嗚妭**
- 鍐呭鎺掕椤瑰彲鐐瑰嚮璺宠浆璇︽儏椤点€?- 寰呭鐞嗕簨椤归」鍙偣鍑昏烦杞搴旂鐞嗛〉銆?
## Page: /studio/followers 绮変笣鍒嗘瀽

**Key Constraints**
- 鏂板椤甸潰銆傛暟鎹簮锛歚GET /api/v1/users/:id/followers/stats?days=30`銆?- 鍙傝€?B 绔欑矇涓濆垎鏋愰〉锛氭€绘暟 + 瓒嬪娍 + 鏉ユ簮銆?
**瑙嗚缁撴瀯**
- 缁熻鍗＄墖锛氱矇涓濇€绘暟 + 鏈湀鏂板 + 鏈湀娴佸け
- 绮変笣澧為暱瓒嬪娍锛氳繎 30 澶╁弻绾挎姌绾垮浘锛堟柊澧?/ 鍑€澧烇級
- 绮変笣鏉ユ簮鍒嗗竷锛氭寜鍐呭鏉ユ簮鐨勯ゼ鍥炬垨妯悜鏌辩姸鍥?
**鏍稿績缁勪欢娓呭崟**
- `StatsCard`
- `FollowerTrendChart`锛坮echarts LineChart锛屽弻绾匡級
- `FollowerSourceChart`锛坮echarts PieChart锛?
**甯冨眬瑙勮寖**
- 缁熻鍗＄墖锛歚grid grid-cols-3 gap-4`
- 鍙屽浘琛細`grid grid-cols-1 lg:grid-cols-2 gap-6`

**鐘舵€佸彉浣?*
- default: 鏁版嵁姝ｅ父銆?- loading: 楠ㄦ灦灞忋€?- empty: 銆屼綘杩樻病鏈夌矇涓濓紝蹇幓鍙戝竷鍐呭鍚稿紩鍏虫敞鍚с€岴mptyState銆?- error: Toast + 閲嶈瘯鎸夐挳銆?
**鍝嶅簲寮忚鍒?*
- 绉诲姩: 鍗＄墖 1 鍒楋紝鍥捐〃鍗曞垪銆?- 骞虫澘+: 鍗＄墖 3 鍒楋紝鍥捐〃鍙屾爮銆?- PC: max-w 1280px銆?
## Page: /studio/revenue 鏀剁泭鏁版嵁

**Key Constraints**
- P1 棰勭暀椤甸潰銆俙features.creator_support_enabled: false`锛堥粯璁わ級鏃舵樉绀哄崰浣嶃€?
**瑙嗚缁撴瀯**
- 灞呬腑 EmptyState锛氬浘鏍?+ 銆屾敹鐩婂姛鑳藉嵆灏嗗紑鏀撅紝鏁鏈熷緟銆? 璇存槑鏂囧瓧

**鐘舵€佸彉浣?*
- disabled: EmptyState锛圡VP 榛樿锛夈€?- P1 enabled: 绱鏀剁泭鍗＄墖 + 瓒嬪娍鍥?+ 鎻愮幇鎸夐挳銆?


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
- 对话窗口：右侧展示消息历史（按时间排列，自己消息靠右 bg-accent.emphasis，对方消息靠左 bg-canvas.subtle）
- 消息输入：底部固定输入框 + 发送按钮，Enter 发送，Shift+Enter 换行
- SSE 实时：使用 EventSource 订阅 `/api/v1/messages/stream` 实时接收新消息
- 响应式：移动端单栏切换（列表或对话窗口全宽），平板和 PC 左右分栏

## Page: /forgot-password 忘记密码（Task 117）

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。
- 邮件发送限流：同一邮箱 60 秒内只能发送一次重置邮件。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 破坏性操作必须 ConfirmModal 二次确认。
- 邮箱提交后 60 秒倒计时禁用按钮，防止重复发送。

## Page: /reset-password 重置密码（Task 117）

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。
- token 有效期 1 小时，过期需重新申请。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`

**交互细节**
- 按钮 hover/active/disabled: 依据 Global Interaction Patterns。
- 密码强度指示器：实时显示密码强度（弱/中/强），使用 `fg.danger` / `fg.muted` / `accent.emphasis` 颜色。
- 重置成功后自动调用登录 API，无需用户再次输入密码。

## Page: /collections/[collectionId] 收藏集详情（Task 122-124）

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。
- 私有收藏集仅创建者可见；公开收藏集所有登录用户可浏览。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`
- 图片/图标特殊处理: 图片和占位图 SVG 使用反色或透明度调整 (`opacity-90`)。

**交互细节**
- 筛选标签点击：切换 content_type 过滤（`router.push` 更新 URL query）。
- 创建者操作：收藏集信息区域显示「编辑」和「删除」按钮（删除需 ConfirmModal）。
- 内容卡片右下角：创建者视角显示「移除」按钮（小号，低视觉权重）。
- 添加内容：仅在创建者视角，内容详情页 ReactionBar 区显示「添加到收藏集」按钮。

## Page: /user/[userId]/collections 用户收藏集列表（Task 122-123）

**Key Constraints**
- 遵守全局扁平化无阴影设计规范，颜色引用预定义 token。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。
- 私有收藏集仅创建者可见。

**视觉层级**
- 顶部区域：导航栏 `h-16`，背景 `bg-canvas.default`，底边框 `border-b border-border.muted`
- 主容器：居中最大宽度，页面背景 `bg-canvas.subtle`
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
- 背景色 token: `canvas.default` -> `canvas.default.dark`
- 边框色 token: `border.default` -> `border.default.dark`
- 文字色 token: `fg.default` -> `fg.default.dark`

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
- 外层容器: `<div className="border border-border.default rounded-md bg-canvas.default overflow-hidden cursor-pointer group">`
- 封面区: 3:2 比例容器，`object-cover` 图片填满；无封面时使用集合 SVG 占位图
- 信息区: `p-3`，标题（`text-sm font-medium line-clamp-1`）-> 内容数 + 可见性 Badge行（`text-xs text-fg.muted`）
- 可见性 Badge: 公开 `text-fg.muted` + unlocked图标；私有 `text-fg.muted` + locked图标
- 悬停: `group-hover:-translate-y-1 transition-transform duration-200`

**尺寸规范**
- 卡片: 自适应宽度，padding 12px (p-3)
- 封面区: aspect-ratio 3:2
- 字号: `text-sm` (14px) 标题，`text-xs` (12px) 辅助信息
- 间距: 元素间隙 4px (`gap-1`) 或 8px (`gap-2`)

**状态变体**
- default: `bg-canvas.default text-fg.default`
- hover: 边框颜色加深 + 轻微上浮
- loading: SkeletonCard 占位
- empty: 不渲染（由父级处理 EmptyState）

**暗色模式适配**
- 全局切换暗色类后组件自动映射 `canvas.default.dark` 等 token 变量。

**关键交互**
- 整卡可点击跳转 `/collections/[id]`
- 创建者视角：hover 时右上角显示编辑/删除按钮（低视觉权重，`opacity-0 group-hover:opacity-100`）

## Component: DownloadButton 下载按钮（Task 121）

**Key Constraints**
- 信誉分 < 3 用户：下载按钮 disabled，hover tooltip 提示「信誉分不足」。
- 封禁用户：下载按钮 disabled，tooltip 提示「账号已被封禁」。
- 遵守全局扁平化无阴影设计规范。
- 绝无 box-shadow（GitHub 扁平风），使用 1px border。

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
- 按钮: `<button className="inline-flex items-center gap-2 px-4 py-2 border border-border.default rounded-md bg-canvas.default text-sm font-medium hover:bg-canvas.subtle transition-colors">`
- 图标: `<Icon name="download" className="w-4 h-4" />`
- 文字: "下载" 或 content_type 对应的文字（如 "下载乐谱"）

**尺寸规范**
- 按钮高度: `h-9` (36px)
- 内间距: `px-4 py-2`
- 字号: `text-sm` (14px)
- 图标: `w-4 h-4` (16px)

**状态变体**
- default: `bg-canvas.default text-fg.default border-border.default`
- hover: `hover:bg-canvas.subtle hover:border-border.emphasis`
- active: `active:bg-canvas.muted scale-95`
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
- Modal 容器: `<div className="bg-canvas.default rounded-lg shadow-md max-w-md w-full mx-auto p-6">`
- 标题: `<h2 className="text-lg font-medium text-fg.default">` 添加到收藏集
- 收藏集列表: 每项 `button border border-border.default rounded-md p-3 hover:bg-canvas.subtle flex items-center gap-3`
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
- Modal 容器: `bg-canvas.default.dark` (token 自动映射)。

**关键交互**
- 点击收藏集 -> `POST /api/v1/collections/:id/items` -> 成功后该项显示 check
- 点击「新建收藏集」-> 弹出新建收藏集子 Modal（标题 + 公开/私有选择）
- 重复添加：前端灰显 + 禁止点击，后端 UNIQUE 约束兜底返回 409
- ESC 或点击遮罩 -> 关闭 Modal
