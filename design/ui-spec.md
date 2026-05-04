# OmniCraft UI 设计规格

> ℹ️ **生成状态**：本文件由 Gemini 批量生成，已完成初始内容填充。全局规范（Design Tokens、Interaction Patterns）和组件规范（Component 章节）可直接用于实现。以下偏差需 Agent 实现时注意：
>
> 1. **部分页面「核心组件清单」仍为模板默认值**（`EmptyState/LoadingSpinner`），Agent 应参考对应 Task 的 `ui_spec_ref` 和 `UI Design.md` 页面小节获取真实组件列表。
> 2. **表单类页面的「响应式规则」已修正**（login/register/settings/judge exam/rehab/messages/appeals 等 15 个页面），但个别页面可能仍有遗留。
>
> Agent 在实现前端时，优先以 `## Component:` 具体组件的规范和 `UI Design.md` 对应页面为准；发现不一致时以设计规范为准。

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
