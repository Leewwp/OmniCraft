# U-02 token 换血差异清单（globals.css 现值 → 定稿值）

> 创建日期：2026-09-02 ｜ 预计失效日期：U-02（#278）落地后即归档（至迟 2026-11-02）
> 来源：U-01（#277）规范定稿产物。token 定稿权威 = `design/design-system.md` v3.0；本清单是其与 `frontend/app/globals.css` 现值的逐条对照，供 U-02 一次性落地，不引入清单外任何值。
> 对比度复算方法：WCAG 2.1 相对亮度公式（与审计批次一致）；涉及值均已复算，结果附注。

---

## A. 表面分层（两对 = 2 token × light/dark）

| Token | 主题 | globals.css 现值 | 定稿值 | 依据 |
|-------|------|------------------|--------|------|
| `--background` | light | `#ffffff` | `#f5f5f5` | SP-12 表面分层裁决（画布退一档，复用 canvas-subtle 值） |
| `--background` | dark | `#0d1117` | `#010409` | 同上（复用暗 canvas-inset 值） |
| `--canvas-default` | light | `#ffffff` | `#f5f5f5` | 与 `--background` 同步退档（画布/通栏容器族） |
| `--canvas-default` | dark | `#0d1117` | `#010409` | 同上 |

**不动的卡片族**：`--card`（#ffffff / #0d1117）、`--popover`（#ffffff / #161b22）、`--secondary`、`--muted`、`--accent`、`--sidebar`、`--canvas-subtle`（#f5f5f5 / #161b22）、`--canvas-inset`、border/fg/tag/elevation 全族——「卡片不动、画布退一档」。

## B. FIX-05 dark token 裁决（3 项）

| Token | 主题 | 现值 | 定稿值 | 复算 |
|-------|------|------|--------|------|
| `--primary` | dark | `#6366f1` | `#4f46e5` | 白字 4.47:1（未达 AA）→ **6.29:1** ✓ |
| `--accent-emphasis` | dark | `#6366f1` | `#818cf8` | 对暗卡片 #0d1117：4.24:1 → **6.34:1** ✓ |
| `--accent-hover` | dark | `#818cf8` | `#4338ca` | 白字 2.98:1（禁用组合）→ **7.90:1** ✓；与 light #4338ca 对称（「加深一档」） |

**禁止**：白字配 `#818cf8` 底（仅 2.98:1）；dark `bg-primary` 不得回退 #6366f1。

## C. 控件圆角（1 token）

| Token | 现值 | 定稿值 | 说明 |
|-------|------|--------|------|
| `--radius-md`（`@theme inline`） | `4px` | `8px` | 操作控件（Button/Input/Select/DropdownMenu Item 等 `rounded-md` 消费面）与卡片同档；`rounded-sm`(3px, checkbox) 与 `rounded-full`（药丸）不动 |

## D. Button 组件 size 档对齐三档高度（`frontend/components/ui/button.tsx`）

| size | 现值 | 定稿值 | 档位 |
|------|------|--------|------|
| `sm` / `icon-sm` | `h-7` / `size-7` (28px) | 不变 | 紧凑 28 |
| `xs` / `icon-xs` | `h-6` / `size-6` (24px) | `h-7` / `size-7` (28px) | 升入紧凑档下限（全站仅 1 处 prototype 消费） |
| default（缺省）/ `icon` | `h-8` / `size-8` (32px) | `h-9` / `size-9` (36px) | 常规 36（约 245 处消费，高度 +4px，属裁决内一次性换血） |
| `lg` / `icon-lg` | `h-9` / `size-9` (36px) | `min-h-11` / `size-11` (44px) | 表单与主 CTA 44~48（48 `h-12` 仅限页面 hero 主 CTA） |

coarse-pointer `[@media(pointer:coarse)]:min-h-11` / `:size-11` 既有规则保留。

## E. 显式不改动清单（防扩散）

- `--ring`（dark 保持 #6366f1：非文本聚焦指示，对画布 4.24:1 ≥ 3:1 达标，不在裁决内）
- `--sidebar-primary`（dark #6366f1：已核无消费面；若 U-02 期间发现新消费，按 `--primary` 同裁决对齐并记录）
- `--primary` light、`--accent-hover` light、`--accent-emphasis` light、`--accent-subtle` 双侧、tag 六色双侧、chart 六色、elevation 三档、字阶、间距
- `--border` / `--input` / `--border-strong` / `--border-muted` / `--border-default` 双侧

## F. U-02 落地目检注意（已知交互面，非新增改动）

1. **light 下 `--canvas-subtle` 与新画布同值**（#f5f5f5）：outline Button hover 的 `hover:bg-canvas-subtle` 在画布色底上填充不再可辨，反馈依赖 `hover:border-border-strong`——目检确认 hover 仍有可感知反馈；不足时在 U-02 内按「hover 加深一档」规则微调 outline hover 用 `--muted`（现值同 #f5f5f5，等效），不新增 token。
2. **`bg-background` 消费面**（约 73 处，含 outline Button、Input `bg-background`）：换血后= 画布色。输入框在卡片上呈「填充式」（参考图形态），outline 按钮呈中性底——属预期形态，目检确认无 readability 回退。
3. **`bg-canvas-default` 消费面**（约 56 处：Header、Sidebar、StudioSidebar、SortSelect、ContentDetailOverlay、MediaGallery、Agent 面板等）：换血后跟随画布色（通栏条与画布同色、靠 1px 边框分层）；卡片类容器必须用 `bg-card`——目检抽查是否有误用 `bg-canvas-default` 当卡片底的页面，发现即在该页改 `bg-card`（属 U-02 换血范围）。
4. **对比度复算记录**：所有涉及色对（本清单 B 列三组 + 换血后 body 文字对画布）≥4.5:1，结果写入关票评论。

## G. 验证门（U-02 执行时引用）

`cd frontend && npm run build && npm run lint` + `bash scripts/verify-project.sh --full` + 全站亮暗双主题目检清单（覆盖首页/IP 库/IP 详情/内容详情/工作台/admin/studio）+ 本清单逐条勾选。
