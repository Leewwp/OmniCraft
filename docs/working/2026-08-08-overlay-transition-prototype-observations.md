# Overlay 共享元素转场原型观察记录（#67）

- 创建日期：2026-08-08
- 预计失效日期：2026-10-08（原型验证票 #67 收口后本文件归档或删除；#68 实现期间可作为设计输入保留，此后失效）
- 归属：GitHub issue #67（Parent #64，Ticket 03，light 车道）
- 原型位置：`prototypes/overlay-transition/index.html`（自包含静态 HTML + vanilla JS，零依赖，file:// 或本地静态服务器均可运行）；自动化验证驱动：`prototypes/overlay-transition/verify-prototype.mjs`（playwright-core，17/17 断言通过）
- 截图：`screenshots/overlay-transition-*.png`（12 张，覆盖 FLIP/VT/降级/reduced-motion/中断恢复）

## 1. 权威依据（本原型实现的事实来源）

- `design/ui-spec.md` §Component: ContentDetailOverlay（#64 决策 7-10 权威正文）：
  - 转场核心为手动计算 FLIP 几何；source 矩形 = 触发卡片封面/媒体区；开启动画把该视觉锚点放大为浮层封面几何，外壳同一时间线推进；关闭仅在 source 仍可测量时反向回归。
  - source 缺失/离屏/detached/无法测量 → 居中 scale-and-fade 降级（程序化无卡片 source 同样适用）。
  - View Transition API 为渐进增强：`document.startViewTransition` 可用时启用，不支持继续 FLIP。
  - 时序：开 300ms / 关 240ms，共享缓动 `cubic-bezier(0.22,0.61,0.36,1)`；reduced-motion 100ms 纯透明度淡化。
- `docs/superpowers/specs/2026-08-07-omnicraft-web-experience-corrections-design.md` 决策 7~12 与 Testing Decision 5（overlay motion 测试面）。
- `docs/working/2026-08-08-ui-authority-ownership-matrix.md` §6：#67 不共享生产文件，原型验证；`ContentDetailOverlay.tsx` + `*Layer.tsx` 由 #68 收敛。

## 2. 原型结构

- 场景按钮（顶部控制条）：
  1. 打开卡片 #1（视口内 source）——FLIP / VT 开合正路径
  2. 程序化打开（无 source）——降级
  3. 离屏 source——降级
  4. 已卸载 source（clone 后 remove 原卡片）——降级
  5. 无法测量 source（封面 `display:none`）——降级
  6. 开→滚出→关（关闭时 source 离屏）——关闭降级，不得飞向离屏位置
  7. 连点：开→关→开——中断恢复
  8. 开关：强制禁用 VT / 强制 reduced-motion
- 执行日志面板：每条操作记录路径徽章（FLIP/VT/FALLBACK/FADE/WARN/INFO）+ source 几何 + 降级原因，供人工核对。
- 状态机：`idle | opening | closing` + run token（每次打开/关闭递增，过期回调直接丢弃）。
- 测量门 `measureSource()`：isConnected、尺寸 > 0、与视口矩形交集三者全过才返回 source 矩形，任一失败返回 null → 降级。

## 3. 各场景实测观察（Chromium 1223，headless，1440×900；verify-prototype.mjs 17/17 PASS）

| 场景 | 路径 | 观察 |
|------|------|------|
| 视口内卡片开 | FLIP | 中段 `transform: matrix(0.988, 0, 0, 0.988, -6.3, 2.75)`（截取值，随视口不同）→ 落定 `none`；300ms 同缓动；mask 与封面同一时间线同步淡入/放大，无脱节 |
| 视口内卡片关 | FLIP | 反向回归 240ms；focus 返回触发卡片（`restoreFocus` preventScroll）；日志含 source rect 便于回放 |
| VT 开 | VT | `startViewTransition` 回调内同步完成 DOM 更新；`view-transition-name: shared-cover` 在 old 态只给卡片封面、new 态只给浮层封面；落定后清理动态命名 |
| VT 关 | VT | old 态只留浮层封面、new 态只留卡片封面；`t.finished` 拒绝时恢复 overlay 再走 FLIP 兜底（脚本未触发，属防御路径） |
| VT 强制关闭 | FLIP | 复选框强制 `vtEnabled()=false` 后行为合同与原生无 VT 浏览器完全一致（同一代码路径） |
| 程序化打开（无 source） | FALLBACK | 居中 `scale(0.96)` + opacity 0→1，300ms；日志 `原因=无 source（程序化打开）` |
| 离屏 source 打开 | FALLBACK | `rect=y:-698`（已滚出视口）→ 拒绝测量；居中缩淡，无向视口外飞行 |
| detached source 打开 | FALLBACK | `isConnected=false` 被测量门拒绝 |
| `display:none` source 打开 | FALLBACK | rect 0×0 被拒绝 |
| 开→滚出→关 | FALLBACK | 关闭时 source 离屏 → 居中缩淡 240ms 关闭，不向离屏位置回归 |
| reduced-motion 开/关 | FADE | 100ms 纯 opacity；封面 transform 恒为 `none`（断言通过，无位移缩放）；强制复选框与 `emulateMedia` 双通道均验证 |
| 连点 开→关→开 | 混合 | `cancelActive()` 清过渡后第二开正常落定；日志出现 WARN 中断记录 + VT 打开完成 |

## 4. 兼容性与限制（供 #68 参考的一手经验）

1. **VT 快照内命名唯一性**：`view-transition-name` 在**每个快照状态内**必须唯一。打开时 old 态卡片封面带名、new 态浮层封面带名（回调内先给浮层封面命名再清掉卡片封面名）；若不清，Chrome 会中止转场（AbortError）。命名只在转场期间动态添加，结束后清除，避免污染后续 DOM 快照。
2. **VT 进行期间 rAF 被挂起**（Chromium 实测）：`startViewTransition` 回调内如果 `await requestAnimationFrame`，`updateCallbackDone` 永不 resolve。**回调必须同步完成 DOM 更新**；动画辅助的 `nextFrame()` 用双 rAF + 120ms setTimeout 双保险，避免在 VT 期间调用时永久挂起。
3. **getBoundingClientRect 时序**：FLIP 的 First 测量必须在任何布局变更（滚动锁、overlay 插入、display 切换）**之前**完成；Invert 与 Play 之间必须强制至少一帧（或双 rAF），确保 Invert 态已绘制再放开过渡，否则首帧闪跳。开启动画里 Play 起点即 Invert 位姿，关闭动画里 Invert 位姿是终点（先 none 起点 → 播放到 invert 变换），方向不可反，反了会从 source 处"掉出"。
4. **关闭可逆性必须重新测量**：打开时可测量的 source 不等于关闭时可测量（用户可能滚走、卡片可能被虚拟化卸载）。关闭路径必须重新走 `measureSource()`，失败即降级居中缩淡——这是 AC3 在关闭方向的直接延伸。
5. **中断处理**：任何路径都带 run token + `cancelActive()`（transition:none 强制清过渡）。VT 中途被 `skipTransition`/Abort 时 `t.finished` 会 reject，需恢复 overlay 可见再走 FLIP 兜底；`startViewTransition` 同步抛错（已有进行中转场）也要走同一兜底。中断后必须保证状态机回 idle，不做残留清理时死锁。
6. **reduced-motion 判定通道**：`matchMedia('(prefers-reduced-motion: reduce)')` 监听 change 事件实时刷新状态（实测可用）；原型还提供强制复选框，两通道共用同一 `reducedMotion()` 判定，保证测试可复现。
7. **浏览器兼容性**：View Transition API 现为 Chromium/Safari 26+ 支持、Firefox 不支持——Firefox 即"VT 不支持 → FLIP"的真实环境，原型强制禁用开关可精确模拟该环境；FLIP 路径为所有现代浏览器共享代码。原型在 file:// 下可用（封面用 SVG data-URI，无网络依赖）。
8. **scroll-lock 与滚动条宽度**：原型同时锁定 `html/body` overflow，并把滚动条宽度补偿到 `body.paddingRight`，解锁时恢复原始内联样式与 `document.scrollingElement.scrollTop`；overlay 自身 `overflow:hidden`，内部滚动只发生在 sheet 的 panel 容器。
9. **降级参数**：`scale(0.96)` + 300ms（开）/ 240ms（关）+ 同缓动；reduced-motion 100ms 纯 opacity。这些数值是原型实测值，正式实现的最终数值以 ui-spec/design-system 为准（本原型不替代 token 权威）。

## 5. 给 #68 的正式实现契约（可复现输入）

- **入口 API**：`openOverlay(trigger: Element | null, sourceRect: Rect | null)` / `closeOverlay()`；测量门为 `measureSource(trigger): Rect | null`，契约同 §3 三条件。`#68` 建议抽出 `shared overlay hook`（矩阵 §6 已预约 owner 顺序 #67 → #68）。
- **路径选择顺序**（每次操作独立判定，不缓存）：`reducedMotion() ? fade : (!sourceRect ? fallback : (vtEnabled() ? vt : flip))`。
- **FLIP 打开**：First = source rect → Last = 浮层封面自然位姿 → Invert = `translate(dx,dy) scale(sx,sy)`（transition:none）→ 双 rAF 确保绘制 → Play 300ms `cubic-bezier(0.22,0.61,0.36,1)` → `transform:none`；mask opacity 同帧 0→1。封面加载态在最终封面几何内展示，主体内容仅在转场落定后 reveal（决策 11）。
- **FLIP 关闭**：Last 重测 = `measureSource`（或降级）→ 起点 identity → Play 到 invert 位姿 240ms + mask 1→0；结束后移除 overlay、解锁滚动、焦点回触发卡。
- **VT 开/关**：回调内同步 DOM 更新 + 换名（唯一性规则见 §4.1）；`t.finished` reject 或同步 Abort → 恢复状态后走 FLIP 同函数。
- **降级**：居中 `scale(0.96)` + opacity，开 300ms / 关 240ms 同缓动；日志/断言按 `原因=无 source/离屏/已卸载/无法测量` 区分，便于测试复现。
- **reduced-motion**：100ms 纯 opacity，cover 不设 transform；关闭同。
- **测试面**（承接规格 Testing Decision 5）：浏览器级断言覆盖 视口内 source 开合、source 滚出后关闭、detached、程序化打开、VT 支持/不支持（强制开关模拟）、reduced-motion（emulateMedia + 强制开关）、快速中断恢复；几何断言用 computed transform/opacity，不断言实现细节字符串。
- **不继承项**：原型使用 `display:none` 切换 overlay（生产建议 `dialog`/portal + inert，见 ui-spec 响应式与可访问性节）；SVG data-URI 封面仅为原型自包含，生产走真实媒体加载态。

## 6. 复现步骤（可追溯）

```bash
# 1) 本地打开原型（或任意静态服务器 / file:// 直接打开）
open prototypes/overlay-transition/index.html
# 2) 自动化全场景验证（playwright-core，按脚本位置解析当前仓库 frontend/node_modules）
node prototypes/overlay-transition/verify-prototype.mjs
# 预期：== 17/17 checks passed ==；截图落 screenshots/overlay-transition-*.png
# 3) 人工抽查：控制条逐个场景按钮 + 右侧日志路径徽章
```
