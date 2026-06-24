# 任务：为 OmniCraft 项目生成完整的 UI 设计规格文档

> ⚠️ **归档说明**：本文档中所有对 `UI Design.md` 的引用均指向已归档文件（现位于 `docs/archive/UI Design.md`）。该文件仅作历史参考，**不再作为视觉权威**。当前视觉权威为 `design/design-system.md` 与 `design/ui-spec.md`。下方涉及 `UI Design.md` 的章节编号、页面描述等内容保留以供历史追溯，但实现时应以 `design/design-system.md` 为准。

你是一名资深 UI/UX 设计师，需要为 OmniCraft（万象工坊）的前端实现填充完整的视觉设计规格。该规格文档将作为 AI 编程 Agent 实现前端代码的**唯一视觉权威**，必须可执行、可检索、无歧义。

## 一、必读上下文文件（按顺序通读）

1. `OmniCraft（万象工坊）V0.3 正式版产品需求文档.md` —— 业务功能与用户角色
2. [architecture.md](cci:7://file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md:0:0-0:0) §3.1（Web 前端路由清单与组件树）+ §10（V0.3.1 增强：颜色 token、瀑布流、内容浏览布局）+ §11.9（Agent 组件定位）
3. `UI Design.md`（**已归档至 `docs/archive/UI Design.md`**，仅作历史参考）—— **本文档是规格的输入清单**，列出了所有 P01-P29 页面、所有组件类别（§3.1-3.11）、设计风格基准、响应式断点
4. [CLAUDE.md](cci:7://file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/CLAUDE.md:0:0-0:0)「关键业务规则」+「Tauri 客户端文件操作白名单」—— 用于推断按钮/状态/权限相关 UI 的展示逻辑
5. [task.json](cci:7://file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/task.json:0:0-0:0) —— 检查每个前端 task 的 `ui_spec_ref` 字段，**你生成的章节标题必须与这些引用完全一致**
6. [design/ui-spec.md](cci:7://file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/design/ui-spec.md:0:0-0:0) —— **当前文件**，已有 79 个空白 heading 占位符，你只需填充每个 heading 下方的内容（不要修改 heading 本身、不要新增/删除 heading）

## 二、设计风格基准（不可偏离）

- **风格定位**：GitHub 黑白底色 + 低饱和强调色 + Steam 创意工坊聚合风 + 小红书瀑布流
- **核心特征**：
  - 1px border 卡片，**绝无 box-shadow**（GitHub 扁平风）
  - 系统字体栈 `-apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial`，无自定义字体
  - 标签使用低饱和色（4 色：blue/green/purple/orange，已在 architecture.md §10.4 定义）
  - 圆角统一 6px（按钮/卡片/输入框）；标签徽章圆角 12px
  - 暗色模式：`<html class="dark">` 切换，所有颜色 token 必须有 light/dark 双值
  - 图标全用 Lucide React 线条风格，**禁止 Emoji 装饰**
  - 加载用骨架屏（Skeleton），按钮内联 Spinner，**禁止全屏遮罩 loading**
  - 空状态用 EmptyState 组件（图标 + 标题 + 说明 + 可选 CTA）
- **响应式断点**：移动 ≤ 700px / 平板 ≤ 1100px / PC > 1100px
  - 瀑布流列数：移动 2 / 平板 3 / PC 4
  - 移动端 FacetedSearchSidebar 折叠为 shadcn/ui Sheet 抽屉（85vw 左侧滑入）

## 三、输出格式（严格遵守）

每个 heading 下方按下列模板填充内容：

### 页面（## Page: ...）模板
```
**Key Constraints**（3-5 条最关键约束，供 Agent 快速 grep）
- [约束 1，如布局结构]
- [约束 2，如响应式行为]
- [约束 3，如交互核心规则]

**视觉层级**（从上到下/从外到内描述区域划分，标注间距 px / 颜色 token）

**核心组件清单**（页面引用了哪些 ## Component: 子章节，列表形式）

**布局规范**
- 页面最大宽度：[如 1280px / 全宽]
- 主内容区与侧边栏比例：[如 4:1 / 3:1]
- 区域间距（block）：[如 32px / 48px]
- 元素间距（inline）：[如 12px / 16px]

**状态变体**（每种都要描述）
- default: 
- loading: [骨架屏样式 / Skeleton 形状]
- empty: [使用 EmptyState 组件，文案、图标、CTA]
- error: [Toast / 内联错误提示]
- 特殊状态：[如未登录、信誉分 < 3、内容被封禁等]

**响应式规则**
- 移动 (≤700px): [布局变化、隐藏元素、抽屉化等]
- 平板 (≤1100px): 
- PC (>1100px): 

**暗色模式适配**
- 背景色 token: canvas.default → canvas.default.dark
- 边框色 token: ...
- 文字色 token: ...
- 图片/图标特殊处理: [如反色、加滤镜等]

**交互细节**
- [按钮 hover / active / disabled 行为]
- [破坏性操作必须 ConfirmModal，参考 CLAUDE.md「业务规则」]
- [数据加载策略：SSR / 客户端 / SWR 等]
```

### 组件（## Component: ...）模板
```
**Key Constraints**（3-5 条）

**Props 接口**（TypeScript 风格，名称要与 architecture.md §3.1 组件树或 UI Design.md §三 描述对齐）
```ts
interface XxxProps {
  // ...
}
```

**视觉结构**（DOM 树伪代码，标注 className 关键 Tailwind 类）

**尺寸规范**
- 默认尺寸: [如 height 40px, padding 12px 16px]
- 字号: [如 text-sm (14px) / text-base (16px)]
- 间距: 

**状态变体**
- default / hover / active / focus / disabled / loading / empty / error
- 每种状态描述视觉变化（颜色 / 边框 / 透明度等），引用 token 名

**响应式行为**（如组件内布局在不同断点的变化）

**暗色模式适配**

**关键交互**
- 点击行为、键盘行为（accessibility，至少 Tab/Enter/Esc）
- 加载/异步反馈
- 错误反馈
```

### Global Design Tokens 模板（已存在的 heading）
完整列出：
- **颜色 token**：直接复用 [architecture.md](cci:7://file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md:0:0-0:0) §10.4 的 `colors.canvas / border / fg / accent / tag`，按 light/dark 列表（Tailwind 配置可直接复制粘贴）
- **字体**：font-family + 字号阶梯（text-xs 12 / text-sm 14 / text-base 16 / text-lg 18 / text-xl 20 / text-2xl 24 / text-3xl 30）
- **间距阶梯**：space-1 4px / space-2 8px / space-3 12px / space-4 16px / space-6 24px / space-8 32px / space-12 48px
- **圆角**：rounded-sm 4 / rounded 6（默认）/ rounded-md 8 / rounded-lg 12（标签）/ rounded-full
- **动效**：transition duration-150 ease-out（默认）；duration-300 ease-in-out（Modal/Sheet）
- **阴影**：**强制 box-shadow:none**，仅 Modal/Popover/Dropdown 用 `shadow-md`

### Global Interaction Patterns 模板（已存在的 heading）
覆盖全站通用的 5 种交互态：
- **Hover**: cursor-pointer + 颜色加深一档（border / bg）
- **Loading**: 骨架屏（Skeleton 灰色块）+ 按钮内联 Spinner，禁止全屏遮罩
- **Error**: Toast（右上角，自动消失 4s）+ 内联红字提示（表单字段下方）
- **Empty**: EmptyState 组件（图标 + 标题 + 说明 + 可选 CTA），不留空白区域
- **Disabled**: opacity-50 + cursor-not-allowed + 禁用 hover/click

## 四、关键约束（违反即返工）

1. **章节标题不可改动**：必须严格保留 [design/ui-spec.md](cci:7://file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/design/ui-spec.md:0:0-0:0) 中现有的 79 个 `## ` heading 文本（包括标点、空格、中英文混排）。Agent 通过 `grep "## Component: ContentCard"` 精确检索，任何字符不一致都会失败。
2. **所有颜色必须用 token 名引用**（如 `canvas.subtle`、`fg.muted`、`accent.emphasis`），禁止硬编码 hex 值（除 Global Design Tokens 章节本身）。
3. **所有间距用 Tailwind 类名引用**（如 `p-4`、`gap-3`、`mt-6`），禁止硬编码 px。
4. **业务规则约束必须显式标注**，例如：
   - ContentCard 上的「一键部署」按钮：`agent_enabled=true && content_type IN ('mod','prompt')` 才显示
   - FollowButton 未登录时：点击跳转 `/login`，不显示已关注状态
   - 信誉分 < 3 用户：发布/评论/点赞按钮 disabled，hover tooltip 提示「信誉分不足」
   - AgentChatWidget：仅 `agent.web_agent_enabled=true` 且已登录才渲染
5. **每个组件必须给出 Props 接口**，方便 Agent 直接复制使用。Props 命名风格：camelCase，事件用 `onXxx`，布尔类用 `isXxx` / `hasXxx`。
6. **破坏性操作（删除/封禁/拒绝/注销/下架）必须备注「需 ConfirmModal 二次确认」**，与 [CLAUDE.md](cci:7://file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/CLAUDE.md:0:0-0:0) 业务规则对齐。
7. **乐谱 Viewer / Diff Viewer / Markdown Renderer 等专业组件**：明确指出对应库的版本、SSR 限制（如 OSMD 必须 `dynamic import + ssr:false`）。

## 五、参考清单（互查）

填写每个 heading 时按下列优先级查阅参考：

> 📌 **注意**：下表中所有 `UI Design.md` 引用均指向已归档文件（`docs/archive/UI Design.md`），仅作历史参考。实现时如与 `design/design-system.md` 冲突，以 `design/design-system.md` 为准。

| heading 类型 | 主参考 | 次参考 |
|------|------|------|
| `## Page: /xxx` | UI Design.md §二（P01-P29 描述） | architecture.md §3.1 路由清单 |
| `## Component: Header / Footer / Sidebar / *Nav` | UI Design.md §3.1 布局组件 | task.json Task 20/29/32 |
| `## Component: ContentCard / IPCard / *Detail / *Renderer / *Viewer / *Uploader / *History` | UI Design.md §3.3-3.4 | architecture.md §10.5 ContentCard 规范 |
| `## Component: PR* / Diff* / Merge*` | UI Design.md §3.5 | task.json Task 13/14/27 |
| `## Component: Follow* / Reaction* / Notification* / Conversation* / Chat* / Comment* / Discussion* / Reply* / UserProfileCard / FollowerListModal / CreatorSupportPanel` | UI Design.md §3.6 + P08/P28/P04a-c | task.json Task 70/76/77/84 |
| `## Component: Exam* / Review* / JudgeQual* / VerdictDetail` | UI Design.md §3.7 + P19/P20 | CLAUDE.md「赛博判官」业务规则 |
| `## Component: Tag*` | UI Design.md §3.8 | architecture.md §10.1 |
| `## Component: AgentChatWidget / UploadAssist* / ComplianceCheck* / UsageGuide* / SearchAgent*` | UI Design.md §3.9 | architecture.md §11.9 |
| `## Component: Course* / ReputationDetail` | UI Design.md §3.10 + P29 | CLAUDE.md「信誉分体系」 |
| `## Component: LLMConfig* / ActiveConfig*` | UI Design.md §3.11 + P26b | task.json Task 83 |
| `## Component: TagBadge / MasonryGrid / ReputationBadge / SortTabs / TimeRangePicker / EmptyState / LoadingSpinner / ConfirmModal / InfiniteScroll` | UI Design.md §3.2 通用 UI | shadcn/ui 文档 |

## 六、交付要求

1. 在 [design/ui-spec.md](cci:7://file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/design/ui-spec.md:0:0-0:0) **顶部第 14 行**（`<!-- Gemini 生成内容从此行开始 -->` 下方）开始填充
2. **删除文件最顶部 1-12 行的「待生成」提示块**（包含「⏳ 待生成」标识与 Agent 使用说明）
3. 保留所有 79 个 heading 不动，只在每个 heading 下方追加内容
4. 文件总长度预计 4000-6000 行（每个 heading 约 50-80 行规格）
5. 完成后运行如下校验确认所有 heading 完整：
   ```bash
   grep -c "^## " design/ui-spec.md   # 应输出 81（79 个 ## + 2 个 Global）
   ```
6. 输出请使用简体中文，专有名词（组件名、token 名、Tailwind 类名）保留英文

完成后请告诉我已经填充完毕，我会进行抽样校验。