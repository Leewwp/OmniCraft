# 2026-08-07 本地 UI/功能问题审计报告（14 项）

> 创建日期：2026-08-07 | 预计失效日期：2026-10-07
> 状态：**讨论已完成，决策已沉淀**（本报告保留为复现证据，不改变任何代码）
> 确认规格：`docs/superpowers/specs/2026-08-07-omnicraft-web-experience-corrections-design.md`
> 复核方式：见文末「附录：验证方法」，可在本地复现每个问题的实测数据
> 说明：下文保留审计发生时的“需决策”原始措辞；实施以确认规格中的最终决策为准。

## 1. 背景

2026-08-07 用户在本地完成一轮 Web Agent 相关大改动（GAP 16/17/18 浮窗与推荐页、Web Agent 1~3 已合并至 `codex/ops/ops-06`，当前 11 commits ahead of main，工作树干净），引入/暴露 14 项 UI 与功能问题。本轮仅做**问题分析与验证**，未改动任何代码。

**验证环境**：
- 后端 `go run cmd/server/main.go`（:8080），PostgreSQL/Redis 为 docker compose 容器
- 前端 `npm run dev`（:3000）
- 浏览器自动化：ego-browser（DOM 几何/样式计算，模型无法读图故用数值证据）
- 分支 `codex/ops/ops-06` @ `26dc41b`

## 2. 问题总览

| # | 问题 | 分类 | 处理建议 |
|---|------|------|----------|
| 1 | 侧边栏背景与网站背景色不同 | A. 立即修复 | light 任务 |
| 2 | 侧边栏与"万象工坊"字样不对齐 | A. 立即修复 | light 任务 |
| 3 | 点击"万象工坊"应跳"推荐"而非二创 | A. 立即修复 | 1 行改动 |
| 4 | 浮窗开合非"小红书式"同步缩放 | B. 未开发完成 | 需设计确认 + 新任务 |
| 5 | 系列目录应为浮窗内可滚动下拉框 | B. 产品决策变更 | 改 ui-spec + 新任务 |
| 6 | "下一章"按钮超出边框 | A. 立即修复 | light 任务 |
| 7 | 未登录也保留"最近访问 IP" | B. 需产品决策 | 决策后建任务 |
| 8 | IP 详情讨论区无"发起讨论"入口 | B. 实现缺口 | 小任务 |
| 9 | 详情浮窗图片与正文边框不对齐 | A. 立即修复 | light 任务 |
| 10 | 关注/已关注按钮样式逻辑反了 | A. 立即修复 | light 任务 |
| 11 | 点赞/点踩状态刷新后丢失 | A. 立即修复（契约不匹配） | 前后端联调 |
| 12 | 筛选选中态三处样式不统一 | A. 立即修复 | light 任务 |
| 13 | 排序下拉列表覆盖下拉框本体 | B. 原生 select 行为 | 自定义组件新任务 |
| 14 | 已收藏内容未显示"已收藏" | B. 未开发完成 | 新任务 |

---

## 3. A 类：真实缺陷，建议立刻修复

### 3.1 侧边栏背景与网站背景色不同

**现象**：侧边栏底色是浅灰，页面主体是白色，视觉上不统一。

**根因**：`frontend/components/layout/Sidebar.tsx:107` 的 `<aside>` 使用 `bg-canvas-subtle`；light 模式下 `--canvas-subtle: #f5f5f5`，而页面主体（`main`/body）为 `--canvas-default: #ffffff`（`frontend/app/globals.css:153-154`）。Header（`Header.tsx:100`）与正文容器均用 `bg-canvas-default`。

**实测证据**（http://localhost:3000/ 与 /original 一致）：

```
aside bg  = rgb(245, 245, 245)   // #f5f5f5 = canvas-subtle
body bg   = rgb(255, 255, 255)   // #ffffff = canvas-default
```

**涉及文件**：`Sidebar.tsx:107`

### 3.2 侧边栏与"万象工坊"字样不对齐

**现象**：侧边栏左边缘与顶部 logo"万象工坊"左边缘不在一条竖线上。

**根因**：页面主容器与 Header 内层容器宽度不一致：
- 页面主容器：`max-w-[1440px]`（`HomePageClient.tsx:167`、`original/page.tsx:90`、`IPBrowseClient.tsx:139`），视口 1710px 下容器左缘 = (1710-1440)/2 = **135px**
- Header 内层：`max-w-7xl`（1280px，`Header.tsx:101`），左缘 = (1710-1280)/2 + px-4(16) = **231px**
- 侧边栏宽度 228px，右缘 363px

**实测证据**（视口 1710×983）：

```
aside  x=135, w=228, right=363   （1440 容器左对齐）
logo   x=231, y=14                （1280 容器左对齐）
差值：96px
```

**涉及文件**：`Header.tsx:101`、`HomePageClient.tsx:167`、`original/page.tsx:90`、`IPBrowseClient.tsx:139`

### 3.3 点击"万象工坊"应跳转"推荐"而非二创

**现象**：logo 点击进入二创区（`/`）而非推荐流（`/recommend`）。

**根因**：`Header.tsx:102-108`（桌面）与 `Header.tsx:342-348`（移动菜单）的 logo `<Link href="/">`。`/` 路由渲染的是二创区（`frontend/app/(public)/page.tsx` → `HomePageClient`，实测页面无 redirect 逻辑）。

**实测证据**：

```
logoHref = "/"        （desktop header）
logoHref = "/"        （mobile menu）
```

**涉及文件**：`Header.tsx:103`、`Header.tsx:343`

### 3.4 "下一章"按钮超出边框（问题 6）

**现象**：浮窗内系列导航的"下一章"按钮右侧溢出容器边框。

**根因**：`SeriesNav.tsx:220` 导航按钮容器在 ≥1101px 时切换为 `flex max-w-[72%]` 单行；三个按钮（上一章/目录/下一章）最小总宽 542px（gap 16px 含），而容器 72% 仅约 501px，`justify-end` 使"下一章"向右溢出。另有 `nav.scrollWidth > nav.clientWidth`（790 vs 764）佐证内容超宽。

**实测证据**（浮窗内，视口 1710px）：

```
nav 容器: x=347, w=728, 右缘=1075
上一章:   x=558, w=213, 右缘=771   ✓ 正常
目录:     x=778, w=100, 右缘=878   ✓ 正常
下一章:   x=885, w=213, 右缘=1098  ✗ 超出 23px（overflowRight=true）
```

**涉及文件**：`frontend/components/content/SeriesNav.tsx:220-273`（尤以 256 行 `col-span-2 ... min-[1101px]:max-w-56` 与 220 行 `min-[1101px]:max-w-[72%]` 组合）

### 3.5 详情浮窗图片与正文边框不对齐（问题 9）

**现象**：浮窗内封面图片容器宽度窄于下方正文/卡片边框。

**根因**：`ContentDetail.tsx:112` 封面容器 `aspect-[16/9]` + `max-h-96`（384px）。正文区宽度 728px 时，16:9 高应为 409px > 384px，`max-height` 生效后 aspect-ratio 相互作用导致容器宽度被压缩到 ~649px，而正文 section 保持 728px。

**实测证据**（打开 /original 首卡浮窗，图片类型内容）：

```
封面容器:  x=347, w=649, h=365
正文 section: x=347, w=728, h=205
宽度差: 79px（封面窄于正文）
```

**涉及文件**：`frontend/components/content/ContentDetail.tsx:107-132`（CoverImage）

### 3.6 关注/已关注按钮样式逻辑反了（问题 10）

**现象**：未关注时按钮低调（白底灰框），已关注时反而高亮（indigo 实底）；hover 已关注按钮也不变"取消关注"。

**根因**：`FollowButton.tsx:54` `variant={following ? "default" : "outline"}`，逻辑与 ui-spec **完全相反**。ui-spec 明文规定（`design/ui-spec.md:3501-3533`）：

```
未关注态: bg-primary text-primary-foreground   → 显眼
已关注态: border border-border-default bg-canvas-default text-foreground hover:bg-canvas-subtle hover:text-destructive hover:border-destructive → 低调
hover 已关注: 文本显示"取消关注"
```

当前组件无 `group-hover` 文本切换逻辑（`FollowButton.tsx:59-69`）。

**实测证据**（浮窗创作者栏）：

```
"关注"    → bg=rgb(255,255,255), border=rgb(232,232,232), color=rgb(24,24,27)  （outline，低调）
"已关注"  → variant=default → bg=#4F46E5（indigo 实底白字，高亮）
```

**涉及文件**：`frontend/components/social/FollowButton.tsx:51-70`

### 3.7 点赞/点踩状态刷新后丢失（问题 11）

**现象**：点赞/点踩后刷新页面，按钮高亮状态消失，观感为"数据没保存"。

**根因**：数据**保存正常**，但"我的反应"状态回读契约不匹配：
- 前端 `ReactionBar.tsx:46-55`：`GET /api/v1/social/reactions?target_type=content&target_id=X`，期望响应 `{ reaction: { reaction: "like" } }`（当前用户）
- 后端 `backend/internal/handler/social.go:210-234`（ListReactions）：返回 `{ reactions: [全部用户列表], total }`，无 `user_id` 过滤、无 `reaction` 单对象字段
- 结果：`data.reaction` 恒为 `undefined` → `myReaction` 恒为 `null` → 已赞/已踩状态无法回显

**数据侧证据**（写入正常）：

```
reactions 表: 2326 行 / 35 用户（docker exec omnicraft-postgres psql）
content_items.like_count: 与 reactions 同步（如 id=47 → like_count=5, dislike_count=0）
GET /api/v1/contents/47 → like_count: 5
GET /api/v1/social/reactions?target_type=content&target_id=47 →
  {"reactions":[{"id":231,"user_id":14,...,"reaction":"like"},...],"total":N}  ← 无当前用户过滤
```

**修复方向**（二选一，需实现时确认）：后端 ListReactions 在 `optAuth` 时附带当前用户反应（如 `{ reaction: {...} }` 或 `my_reaction` 字段），或前端改为调专用接口。

**涉及文件**：`ReactionBar.tsx:46-55`、`backend/internal/handler/social.go:210-234`、`backend/internal/router/routes.go:138`

### 3.8 筛选选中态三处样式不统一（问题 12）

**现象**：IP 库选中为带色药丸，二创区选中为灰色矩形，原创区选中为白色药丸。

**根因**：三处独立实现：

| 页面 | 文件:行 | 选中样式 | 实测（DOM） |
|------|---------|----------|-------------|
| IP 库 | `IPBrowseClient.tsx:193-197` | `rounded-full border-primary bg-accent-subtle text-accent-emphasis` | bg=#EEF2FF, radius=9999px, border=#4F46E5 |
| 二创区 | `HomePageClient.tsx:250-254` | `rounded-md border-border bg-muted` | bg=#F5F5F5, radius=4px |
| 原创区 | `original/page.tsx:116-118` | `rounded-full border-border bg-card` | bg=white, radius=9999px |

**用户决策**：统一为 IP 库样式（带色药丸）。

**涉及文件**：上表三处 + 推荐页无筛选（不涉及）

---

## 4. B 类：功能未开发完成 / 需决策

### 4.1 浮窗开合非"小红书式"同步缩放（问题 4）

**现状**：`ContentDetailOverlay.tsx:296-301` 仅整层 `animate-in fade-in-0 zoom-in-95 duration-300`（0.3s 居中缩放 + 淡入），关闭 `fade-out 240ms`。**无**"触发卡主图/视频与浮窗同步放大到最终位置"的共享动画。

**实测证据**：

```
打开中: animationName="enter", duration=0.3s, transform=scale(0.95), opacity=0
```

**设计依据**：ui-spec（`design/ui-spec.md` 2365 章）要求：
> 外壳与封面共享同一开合动画进度：开 300ms / 关 240ms，同帧同缓动 `cubic-bezier(0.22,0.61,0.36,1)`；触发卡无缩略图或不在视口时，入场降级为居中轻缩放 + 淡化。

当前实现**只有降级形态**（居中缩放+淡入），主形态（封面从卡片位置放大）未实现；另关闭计时 280ms 与 spec 的 240ms 不符（`ContentDetailOverlay.tsx:206`）。

**结论**：实现未完成，需新任务；实现前建议确认动画细节（封面取色、位移起点=触发卡 cover 位置、同步进度控制）。

**涉及文件**：`frontend/components/content/ContentDetailOverlay.tsx`、`ContentDetailOverlayLayer.tsx`

### 4.2 系列目录应为浮窗内可滚动下拉框（问题 5）

**现状**：`SeriesNav.tsx:243-250` 的"系列目录"是 `Link → /series/[id]` 独立页面（ui-spec `SeriesNav` 章节亦如此规定："菜单中每个系列分别链接 /series/:id"）。实测浮窗内点击目录 href=/series/20。

**结论**：**产品决策变更**——用户希望浮窗内直接可滚动下拉选择系列章节目录。需先修改 ui-spec（`design/ui-spec.md:2416` SeriesNav 章节），再实现组件（含可滚动列表、键盘可达、与浮窗内导航栈协调——注意当前目录链接是跨浮窗的整页跳转，改为下拉后需处理"点击章节是否入栈 pushLayer"）。

**涉及文件**：`SeriesNav.tsx`、`design/ui-spec.md`（改规格）

### 4.3 未登录也显示"最近访问 IP"（问题 7）

**现状**：纯 localStorage 实现：
- 写入：`IPCard.tsx:28-34`（`saveRecentIP`，key=`recent_ips`，去重保留 6 条）
- 读取：`HomePageClient.tsx:71-76` 未登录也渲染"最近访问 IP"区块

**实测证据**（干净浏览器、未登录）：

```
localStorage["recent_ips"] = [{"id":28,"name":"候鸟书店"},{"id":23,"name":"琥珀旅社"},{"id":21,"name":"风筝信号"}]
首页渲染 "最近访问 IP" 区块（hasRecent=true）
```

**设计现状**：ui-spec `IPCard` 章节仅写"记录最近访问"，未规定是否绑定用户。后端已有用户绑定的 `browse_history` 表（381 条，`GetContent` 时写入，`backend/internal/handler/content.go:254-258`），与本模块脱节。

**结论**：需产品决策——
1. 仅登录用户显示"最近访问 IP"，数据源迁移到后端 `browse_history`（与多端诉求一致，推荐）
2. 或保留匿名 localStorage，登录后再合并
决策后建新任务。

**涉及文件**：`IPCard.tsx`、`HomePageClient.tsx`（前端）；后端 browse_history 已存在

### 4.4 IP 详情讨论区无"发起讨论"入口（问题 8）

**现状**：
- 发帖功能**已开发**：`/ip/[ipId]/discussions/new` 发帖页存在；列表页 `/ip/[ipId]/discussions` 有"发帖"按钮（`page.tsx:82-84`）；后端 `routes.go:268` `POST /ips/:id/discussions`、`discussion.go:44` CreateDiscussion
- 但 `DiscussionBoard.tsx:104-111`：`{!compact && user && 发帖按钮}` —— **compact 模式（IP 详情页使用）无发帖入口**
- `DiscussionBoard.tsx:89-91`：compact 且无讨论时整个模块 `return null`（连入口都没有）
- IP 详情页实际渲染了 `DiscussionBoard compact`（`IPDetail.tsx:89`），ui-spec IP 详情页章节（`design/ui-spec.md:597-625`）核心组件清单未列讨论区——实现超前于 spec

**实测证据**（/ip/28）：

```
hasNewPostBtn = false（无"发起讨论/发帖"按钮）
页面仅有"进入讨论区"与"查看全部"链接
```

**结论**：实现缺口，小任务：IP 详情页 compact 模式补发帖入口（含空态 CTA），并同步更新 ui-spec。

**涉及文件**：`frontend/components/social/DiscussionBoard.tsx:89-122`、`IPDetail.tsx:89`

### 4.5 排序下拉列表覆盖下拉框本体（问题 13）

**现状**：三处排序均为原生 `<select>`（二创 `HomePageClient.tsx:262-270`、原创 `SortSelect.tsx:19-41`、IP 库 `IPBrowseClient.tsx:168-180`）。mac 上原生 select 的选项列表以系统覆盖式浮层弹出（非 DOM 元素、无法定位到控件下方），观感为"列表盖在下拉框本体上"。

**实测证据**（点击 /original 排序后）：

```
document.querySelector('[role="listbox"]') = null   ← 系统原生弹出，无 DOM 可定位
```

**结论**：非代码 bug，是原生组件平台行为。要满足"列表显示在下拉框下方"，需自定义下拉组件（button + 定位列表，三处统一复用；注意二创/原创的 select 位于 sticky 工具栏内，需处理 z-index 与溢出）。

**涉及文件**：三处 select + 新建共享组件

### 4.6 已收藏内容未显示"已收藏"（问题 14）

**现状**：
- 数据完整：`collections` 107 行、`collection_items` 658 行
- 后端 `GetContent`（`content.go:278-287`）返回的 `is_favorited` 查的是**旧表 favorites**（410 行），与"收藏集"系统（collections/collection_items）脱节
- 前端 `normalizeContentDetailResponse`（`content.ts:282-306`）未解析 `is_favorited`，直接丢弃
- `ContentDetail.tsx:338-355`"添加到收藏集"按钮为固定文案，无已收藏状态；仅在打开 `CollectionPicker` 后列表内显示"已添加"（`contains_item`，`CollectionPicker.tsx:325-330`）

**结论**：未开发完成。需新任务：详情页（含浮窗）按钮展示收藏状态——查询当前用户收藏集 `contains_item`（`lib/collections.ts:104` listCollections 已支持 `content_item_id` 参数，后端 `collection.go:68-70` 已支持），并显示"已收藏/已添加到 N 个收藏集"。

**涉及文件**：`ContentDetail.tsx:338-355`、`lib/content.ts:282`、`backend/internal/handler/content.go:278-287`

---

## 5. 处理建议汇总

### 建议分两批：

**批次 1（A 类 8 项，light 车道小任务）**：1、2、3、6、9、10、12 为纯前端 CSS/文案级改动；11 为前后端契约修复。可合并为 1-2 个 light 任务，改完跑验证门（`npm run lint && npm run build`、`go vet/test` 相关包、`bash scripts/verify-project.sh`）。

**批次 2（B 类 6 项，逐项新任务）**：
- 4（浮窗共享动画）：先确认动画设计细节，再实现
- 5（系列目录下拉）：先改 ui-spec 再实现（产品决策）
- 7（最近访问绑定用户）：先做产品决策（建议迁移 browse_history）
- 8（讨论区发帖入口）：小任务，含 ui-spec 同步
- 13（自定义排序下拉）：共享组件开发
- 14（已收藏状态）：小任务

### 注意点

- 本报告 14 项均不在当前 4 个活计划（AGENTS.md 注册表）内，建议新建计划/issue 追踪
- 问题 11、14 涉及前后端契约，需后端联调验证
- 问题 4/5/7 涉及设计规格变更，须先更新 ui-spec 再实现（AGENTS.md 冲突解决：ui-spec 为视觉唯一权威）

---

## 附录：验证方法（复核用）

### 环境

```bash
docker compose up -d postgres redis
cd backend && go run cmd/server/main.go &
cd frontend && npm run dev &
```

### 浏览器数值验证（ego-browser）

```js
// 问题 1/2：侧边栏背景与 logo 对齐
(() => {
  const aside = document.querySelector('aside');
  const ar = aside.getBoundingClientRect();
  const logo = document.querySelector('header a');
  const lr = logo.getBoundingClientRect();
  return {
    asideBg: getComputedStyle(aside).backgroundColor,
    bodyBg: getComputedStyle(document.body).backgroundColor,
    asideX: ar.x, asideRight: ar.right,
    logoX: lr.x,
  };
})()
// 期望：asideBg=rgb(245,245,245)≠bodyBg=rgb(255,255,255)；asideX(135)≠logoX(231)
```

```js
// 问题 6：下一章溢出
(() => {
  const nav = document.querySelector('dialog nav[aria-label]');
  const nr = nav.getBoundingClientRect();
  return [...nav.querySelectorAll('a,button')].map(el => {
    const r = el.getBoundingClientRect();
    return { text: el.textContent.trim().slice(0,12), right: Math.round(r.right), navRight: Math.round(nr.right), overflow: r.right > nr.right + 0.5 };
  });
})()
```

```js
// 问题 9：封面与正文宽度
(() => {
  const d = document.querySelector('dialog.content-detail-overlay');
  const scroller = d.querySelector('[data-slot="overlay-scroller"]');
  const els = [...scroller.querySelectorAll('div,section')].filter(el => {
    const r = el.getBoundingClientRect();
    return r.width > 100 && (el.className||'').includes('aspect') || (el.className||'').includes('rounded-md border');
  });
  return els.slice(0,4).map(el => { const r = el.getBoundingClientRect(); return { cls: el.className.slice(0,40), w: Math.round(r.width) }; });
})()
// 期望：封面容器 ~649 vs 正文 ~728
```

```js
// 问题 12：三种选中样式
// /ips：active button aria-pressed=true → bg #EEF2FF, radius 9999px
// /   ：active button → bg #F5F5F5, radius 4px
// /original：active Link → bg white, radius 9999px
```

```js
// 问题 13：原生 select 弹出无 DOM
// 点击排序 select 后：document.querySelector('[role="listbox"]') === null
```

### API/DB 验证（问题 7/8/11/14）

```bash
# 问题 11：读取契约不匹配
curl -s "http://localhost:8080/api/v1/social/reactions?target_type=content&target_id=47"
# → {"reactions":[...],"total":N}，无当前用户单对象字段；前端期望 {reaction:{reaction}}

# 问题 7/14：数据侧
docker exec omnicraft-postgres psql -U omnicraft -d omnicraft -c "SELECT count(*) FROM reactions"
docker exec omnicraft-postgres psql -U omnicraft -d omnicraft -c "SELECT count(*) FROM collection_items"
docker exec omnicraft-postgres psql -U omnicraft -d omnicraft -c "SELECT count(*) FROM browse_history"
```

### 截图证据

- `screenshots/` 无本轮新增截图（验证以数值为准）；用户手头截图：下一章溢出（问题 6）、下拉覆盖本体（问题 13）
