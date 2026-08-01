# OmniCraft 设计系统

> 版本 2.1 | 2026-08-01 | 基于 P-01 批准的 Indigo 精修方向

## 设计参考

- **小红书**: 纯白背景、无边框瀑布流、图文卡片
- **Bilibili**: 侧边栏分类导航、顶部极简导航
- **GitHub**: 高信息密度、扁平设计、1px 边框

## 核心原则

1. **三档细腻层级** — 静态卡片/面板、悬浮反馈、浮层分别使用 elevation 1/2/3；阴影必须配合 1px 边框，不单独承担分隔
2. **1px 边框** — 扁平基因不变，通过 `border-border` 分隔；交互态可提升为 `border-strong`
3. **纯白背景** — `bg-background` = `#FFFFFF`，模块间用间距而非线条分隔
4. **极简滚动条** — 3px 宽，默认透明，hover 半透明
5. **统一圆角** — `rounded-lg` (8px) 为默认，标签用 `rounded-full`

---

## 设计令牌

### 颜色

| Token | Light | Dark | 用途 |
|-------|-------|------|------|
| `--background` | `#FFFFFF` | `#0D1117` | 页面背景 |
| `--foreground` | `#18181B` | `#E6EDF3` | 主文字色 |
| `--primary` | `#4F46E5` | `#6366F1` | 主色调/CTA |
| `--secondary` | `#F5F5F5` | `#161B22` | 次要背景 |
| `--muted` | `#F5F5F5` | `#161B22` | 柔和背景 |
| `--muted-foreground` | `#52525B` | `#848D97` | 次要文字 |
| `--border` | `#E8E8E8` | `#30363D` | 边框色 |
| `--destructive` | `#E11D48` | `#F87171` | 错误/危险 |
| `--accent-hover` | `#4338CA` | `#818CF8` | 主色交互态 |
| `--ring` | `#4F46E5` | `#6366F1` | 聚焦环 |
| `--border-strong` | `#D4D4D8` | `#444C56` | hover/active 强边框 |
| `--border-destructive` | `#E11D48` | `#F87171` | 错误/危险边框 |

### 标签颜色

| Tag | Light BG | Light FG | Dark BG | Dark FG |
|-----|----------|----------|---------|---------|
| blue | `#EEF2FF` | `#4F46E5` | `#6366F11A` | `#A5B4FC` |
| green | `#ECFDF5` | `#059669` | `#0596691A` | `#6EE7B7` |
| purple | `#F5F3FF` | `#7C3AED` | `#7C3AED1A` | `#C4B5FD` |
| orange | `#FFFBEB` | `#D97706` | `#D977061A` | `#FCD34D` |
| rose | `#FFF1F2` | `#E11D48` | `#E11D481A` | `#FDA4AF` |
| sky | `#F0F9FF` | `#0284C7` | `#0284C71A` | `#7DD3FC` |

### 自定义 Token

| Token | Light | Dark | 用途 |
|-------|-------|------|------|
| `--canvas-default` | `#FFFFFF` | `#0D1117` | 卡片/容器背景 |
| `--canvas-subtle` | `#F5F5F5` | `#161B22` | 微妙背景 |
| `--border-default` | `#E8E8E8` | `#30363D` | 默认边框 |
| `--fg-muted` | `#52525B` | `#848D97` | 柔和前景 |
| `--accent-emphasis` | `#4F46E5` | `#6366F1` | 强调色 |
| `--accent-subtle` | `#EEF2FF` | `#6366F11A` | 柔和强调色 |

### 布局 Token

| Token | 值 | 用途 |
|-------|-----|------|
| `--sidebar-w` | `228px` | 侧边栏展开宽度 |
| `--header-h` | `52px` | 顶部导航高度 |
| `--max-w` | `1440px` | 内容最大宽度 |

### 间距

间距只使用 4px 基线上的批准值；页面或组件不得再引入 10px、18px 等任意间距。

| 用途 | 值 | 约束 |
|------|-----|------|
| 卡片内边距 | `12px` | `p-3` |
| 区块内边距 | `16px` | `p-4` |
| 详情卡内边距 | `20px / 24px` | 按信息密度选一档 |
| 页面 gutter | `16px (mobile) / 24px (desktop)` | 不随弹性轨道分散 |
| 网格 gap | `16px` | 所有断点保持一致 |
| 区块间距 | `24px / 32px` | 同层级保持一致 |

### 圆角

| Token | 值 | 用途 |
|-------|-----|------|
| `--radius-sm` | `3px` | 小元素 |
| `--radius-md` | `4px` | 按钮/输入框 |
| `--radius-lg` | `8px` | 卡片/容器 |
| `--radius-xl` | `12px` | 大卡片 |
| `--radius-full` | `9999px` | 标签/药丸按钮 |

`rounded-2xl`、`rounded-3xl`、`rounded-4xl` 仅作为迁移兼容别名映射到 `--radius-xl`，不得扩展新的视觉圆角档位。

### 字体

| Token | 值 |
|-------|-----|
| `--font-sans` | `-apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif` |
| `--font-mono` | `'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace` |

### 字阶

| 用途 | 字号 / 字重 / 行高 |
|------|--------------------|
| 详情页标题 | `24px / 600 / tight` |
| 区域标题 | `20px / 600 / normal` |
| 卡片/区块标题 | `14px / 500 / normal` |
| 正文 | `16px / 400 / relaxed` |
| 作者、meta、标签 | `12px / 400 / normal` |
| 侧栏小节标签 | `12px / 600 / uppercase / tracking-wider` |

只允许 12/14/16/20/24px 五档正文与标题字号；紧凑指标数字例外必须在对应组件规格中登记。

### 微阴影

| Token | Light | Dark | 用途 |
|-------|-------|------|------|
| `--elevation-1` | `0 1px 2px 0 rgb(24 24 27 / 0.05)` | `0 1px 2px 0 rgb(0 0 0 / 0.30)` | 静置卡片、面板 |
| `--elevation-2` | `0 4px 12px -2px rgb(24 24 27 / 0.10), 0 2px 4px -2px rgb(24 24 27 / 0.05)` | `0 4px 12px -2px rgb(0 0 0 / 0.50), 0 2px 4px -2px rgb(0 0 0 / 0.30)` | hover 升起 |
| `--elevation-3` | `0 12px 32px -8px rgb(24 24 27 / 0.14), 0 4px 8px -4px rgb(24 24 27 / 0.06)` | `0 12px 32px -8px rgb(0 0 0 / 0.60), 0 4px 8px -4px rgb(0 0 0 / 0.40)` | Dropdown、Drawer、Modal |

用于“升起”层级反馈的 hover 只允许 transform、border 与 shadow 过渡，不得造成布局位移；按钮、链接、选中态仍可按语义 token 进行颜色/背景过渡。`prefers-reduced-motion: reduce` 下禁用缩放、位移和脉冲。

---

## 组件规范

### 按钮 (Button)

- 5 种变体: `default` (primary 色), `outline`, `secondary`, `ghost`, `destructive`
- 默认圆角 `rounded-md`，药丸形用 `rounded-full`
- Primary 按钮使用 `bg-primary text-primary-foreground`
- Hover: 加深一档亮度
- Disabled: `opacity-50 cursor-not-allowed`
- Focus: `ring-2 ring-ring ring-offset-2`

### 卡片 (Card)

- 背景 `bg-card`，首选用 1px `border-border`
- **原创区**: 无边框 (`border-0`)，图片优先，hover 时微上移
- **二创区**: 1px 边框，信息密度高
- 静态面板可使用 elevation 1，hover 使用 elevation 2；无边框原创卡只在 hover 时使用 elevation 2
- 圆角 `rounded-lg`

### 输入框 (Input)

- 1px `border-input`，聚焦时 `ring-2 ring-ring`
- 圆角 `rounded-md`
- Placeholder: `text-muted-foreground/60`

### 侧边栏 (Sidebar)

- 宽度 228px（展开）/ 48px（折叠）
- 折叠动画: `transition-all duration-200`
- 折叠时仅显示图标，hover 显示 tooltip
- 热门内容区折叠时隐藏
- 折叠状态存储于 localStorage

### 标签 (TagBadge)

- 6 种颜色: blue, green, purple, orange, rose, sky
- 圆角 `rounded-full`
- 可选 remove 按钮

### Modal / Popover / Dropdown

- 使用 elevation 3，并始终保留 1px 边框
- 背景叠加: `bg-black/50`
- 圆角 `rounded-lg`

---

## 交互模式

| 状态 | 规则 |
|------|------|
| **Hover** | `cursor-pointer` + 颜色/背景过渡 150ms |
| **Loading** | 骨架屏 (Skeleton) + 按钮内联 Spinner |
| **Error** | Toast (右上角 4s 自动消失) + 内联红字 |
| **Empty** | EmptyState 组件 (图标 + 标题 + 说明) |
| **Disabled** | `opacity-50 cursor-not-allowed` |

---

## 布局模式

### 公共页面布局

```
Header (sticky, h-[var(--header-h)])
├── Logo + Nav
├── Search
├── Publish Button (primary)
└── Notifications + User Menu

AppLayout (flex)
├── Sidebar (228px, collapsible)
└── Main Content
    ├── Zone Banner
    ├── Toolbar (filters + sort)
    └── Masonry Grid (4-3-2 columns)
```

### 管理页面布局

```
AdminLayout
├── Sidebar (228px, collapsible)
└── Main
    ├── Page Header
    └── Content Area (table/list/cards)
```

### Studio 页面布局

```
StudioLayout
├── StudioSidebar (228px, collapsible)
└── Main
    ├── Page Header + Stats
    └── Content Area
```

---

## 暗色模式检查清单

- [ ] 所有颜色使用 CSS 变量，禁止硬编码
- [ ] 卡片/容器使用 `bg-card` 而非 `bg-white`
- [ ] 文字使用 `text-foreground` / `text-muted-foreground`
- [ ] 边框使用 `border-border`
- [ ] 标签使用 tag 颜色变量
- [ ] 图标颜色随文字色变化
- [ ] Modal 遮罩层 `bg-black/50` (dark 可用 `bg-black/70`)
