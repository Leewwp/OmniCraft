# OmniCraft 设计系统

> 版本 3.0 | 2026-09-02 | SP-12 UI 精细化批次（U-01 规范定稿）：分层画布 / 控件高度三档 / 形状语义 / 统一筛选选中态 / 150ms 动效；FIX-05 dark token 裁决并入定稿（token 落地换血由 U-02 执行，差异清单见 `docs/working/2026-09-02-u02-token-diff-list.md`）

## 设计参考

- **小红书**: 纯白背景、无边框瀑布流、图文卡片
- **Bilibili**: 侧边栏分类导航、顶部极简导航
- **GitHub**: 高信息密度、扁平设计、1px 边框

## 核心原则

1. **三档细腻层级** — 静态卡片/面板、悬浮反馈、浮层分别使用 elevation 1/2/3；阴影必须配合 1px 边框，不单独承担分隔
2. **1px 边框** — 扁平基因不变，通过 `border-border` 分隔；交互态可提升为 `border-strong`
3. **分层画布** — 亮色：画布 `#F5F5F5` + 卡片/弹层纯白 `#FFFFFF` + 1px `#E8E8E8` 描边；暗色：卡片族不动（`#0D1117`），画布退深至 `#010409`。「卡片不动、画布退一档」对称分层，内容区与页面底一眼可分
4. **极简滚动条** — 3px 宽，默认透明，hover 半透明
5. **圆角与形状语义** — `rounded-lg` (8px) 为默认，操作控件与卡片同档；矩形 = 操作控件，药丸 `rounded-full` = 筛选选择与信息标签（见组件规范「形状语义」）

---

## 设计令牌

### 颜色

| Token | Light | Dark | 用途 |
|-------|-------|------|------|
| `--background` | `#F5F5F5` | `#010409` | 页面画布背景 |
| `--foreground` | `#18181B` | `#E6EDF3` | 主文字色 |
| `--primary` | `#4F46E5` | `#4F46E5` | 主色调/CTA（白字双主题 6.29:1 ≥AA；FIX-05 裁决 dark 不再用 #6366F1 底配白字） |
| `--secondary` | `#F5F5F5` | `#161B22` | 次要背景 |
| `--muted` | `#F5F5F5` | `#161B22` | 柔和背景 |
| `--muted-foreground` | `#52525B` | `#848D97` | 次要文字 |
| `--border` | `#E8E8E8` | `#30363D` | 边框色 |
| `--destructive` | `#E11D48` | `#F87171` | 错误/危险 |
| `--accent-hover` | `#4338CA` | `#4338CA` | 主色交互态（白字 7.90:1 ≥AA；「加深一档」双主题对称，dark 禁用 #818CF8 底配白字——仅 2.98:1） |
| `--ring` | `#4F46E5` | `#6366F1` | 聚焦环（非文本指示，dark 对画布 4.24:1 ≥3:1） |
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
| `--canvas-default` | `#F5F5F5` | `#010409` | 画布/通栏容器背景（Header、侧栏等；卡片面用 `--card`） |
| `--canvas-subtle` | `#F5F5F5` | `#161B22` | 微妙背景（TabsList、secondary hover） |
| `--border-default` | `#E8E8E8` | `#30363D` | 默认边框 |
| `--fg-muted` | `#52525B` | `#848D97` | 柔和前景 |
| `--accent-emphasis` | `#4F46E5` | `#818CF8` | 强调色（dark 提亮为 #818CF8，对暗卡片 #0D1117 为 6.34:1 ≥AA；FIX-05 裁决） |
| `--accent-subtle` | `#EEF2FF` | `#6366F11A` | 柔和强调色（筛选选中态底色） |

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
| 网格 gap | `16px` | 所有断点保持一致；P-01 已批准的 `/ips` 页面在 ≤700px 使用 12px gap（页面级例外，其他网格不适用） |
| 区块间距 | `24px / 32px` | 同层级保持一致 |

### 高度体系（控件三档）

| 档位 | 高度 | Tailwind | 适用 |
|------|------|----------|------|
| 紧凑 | `28px` | `h-7` / `min-h-7` | 工具栏、卡片内次级动作、密集列表行内动作 |
| 常规 | `36px` | `h-9` / `min-h-9` | 默认按钮、输入框、下拉、搜索框 |
| 表单与主 CTA | `44~48px` | `min-h-11` ~ `h-12` | 发布/提交等表单控件与页面级主 CTA；44 为标准值，48 仅限页面 hero 主 CTA |

- **硬规则：同排控件同高** — 同一水平行内的按钮、输入框、下拉、筛选等可操作控件必须取同一档高度；信息标签（TagBadge）与纯文本不在此列，但同排呈现时不得与操作控件形成参差。
- 筛选药丸取 44px 触控高度（`min-h-11`），见组件规范「筛选选择控件」。
- icon-only 按钮取所在档位的同尺寸正方形；coarse pointer 下保持 44px 触控目标。

### 圆角

| Token | 值 | 用途 |
|-------|-----|------|
| `--radius-sm` | `3px` | 小元素（checkbox） |
| `--radius-md` | `8px` | 按钮/输入框/下拉（操作控件，与卡片同档） |
| `--radius-lg` | `8px` | 卡片/容器 |
| `--radius-xl` | `12px` | 大卡片 |
| `--radius-full` | `9999px` | 标签/药丸（筛选选择与信息标签） |

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

### 动效（全站统一）

- 交互过渡统一 **150ms** transition（颜色/背景/边框/阴影）；Modal/Sheet 开合维持 300ms 不变。
- `active:scale` 按压缩放**仅保留页面级主 CTA**；其余按钮 active 反馈用颜色/边框变化，不用缩放。
- hover 维持「加深一档」现行规则（token 层 `--accent-hover` / `--border-strong`）。
- reduced-motion 下禁用缩放、位移与脉冲（既有规则不变）。

---

## 组件规范

### 形状语义（全站统一）

- **矩形（8px 圆角）= 操作控件**：按钮、输入框、下拉、搜索框。
- **药丸（`rounded-full`）= 选择与信息**：筛选/类目选择控件、TagBadge、状态徽标。
- 同一概念控件不得出现两套形态；筛选类控件一律用药丸（见「筛选选择控件」），操作按钮一律用矩形。

### 按钮 (Button)

- 5 种变体: `default` (primary 色), `outline`, `secondary`, `ghost`, `destructive`
- 圆角 `rounded-lg` (8px)，与卡片同档；操作按钮不用药丸形
- 高度三档：紧凑 28 (`h-7`) / 常规 36 (`h-9`) / 表单与主 CTA 44-48（`min-h-11`~`h-12`）；**同排控件同高**（见「高度体系」）
- `active:scale` 按压缩放仅限页面级主 CTA；其余 active 用颜色/边框反馈
- Primary 按钮使用 `bg-primary text-primary-foreground`
- Hover: 加深一档亮度（`--accent-hover`，双主题白字 ≥AA）
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
- 圆角 `rounded-lg` (8px)
- 高度：常规 36px；表单内取 44-48px 并与提交按钮同高（同排同高硬规则）
- Placeholder: `text-muted-foreground/60`

### 侧边栏 (Sidebar)

- 宽度 228px（展开）/ 48px（折叠）
- 折叠动画: `transition-all duration-200`
- 折叠时仅显示图标，hover 显示 tooltip
- 热门内容区折叠时隐藏
- 折叠状态存储于 localStorage

### 标签 (TagBadge)

- 6 种颜色: blue, green, purple, orange, rose, sky
- 圆角 `rounded-full`（信息标签，形状语义见上）
- 可选 remove 按钮

### 筛选选择控件（FilterPills 形态基准）

- 形态：药丸 `rounded-full`、44px 触控高度（`min-h-11`）、容器横向滚动（溢出时不换页布局）、`aria-pressed` 表达选中。
- **选中态基准（全站唯一，零新 token）**：`bg-accent-subtle` 浅底 + `text-accent-emphasis` 文字 + 1px 主色描边（`border-accent-emphasis`）+ Check 图标（`h-3.5 w-3.5`）+ semibold。
- 未选中态：透明底/透明描边 + `text-muted-foreground`；hover `bg-muted` + `text-foreground`。
- 切换交互：**就地切换 + URL query 同步**（`router.replace`，不滚动不跳页）。
- 基准实现：`/ips` IP 库分类筛选；新筛选一律复用共享 FilterPills 组件（组件规格见 `design/ui-spec.md`）。
- 既有筛选组件（ContentTypeFilter 等）在接入批次收敛到本基准，收敛前不得新增偏离形态。

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
- [ ] 主按钮与选中态文字对比度 ≥4.5:1：dark `--primary` #4F46E5 白字 6.29:1；dark `--accent-emphasis` #818CF8 对暗卡片 6.34:1；**禁止白字配 #818CF8 底**（仅 2.98:1）
