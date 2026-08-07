# OmniCraft 媒体体验设计规格（Media Experience Design）

> 日期：2026-08-08
> 状态：confirmed
> 设计输入：2026-08-08 /grilling + domain-modeling 会话（用户确认的 14 项决策）、本地浏览器实测、代码审查
> 适用范围：内容媒体集语义、发布上传编排、详情/浮层媒体区、媒体查看器、列表卡片纵横比、桌面双栏布局、移动端连续浏览、相关内容块、原创区筛选修复、收藏集弹窗交互

## Problem Statement

OmniCraft 当前的媒体体验在本地真实运行时暴露出三类问题：

1. **筛选不生效**：原创区点击分类 Tab 后 URL 变为 `/original?category=film_tv`，但页面以 `sort=recommended` 请求后端，而推荐排序管线无视 category 等一切筛选条件，返回混合分类内容——筛选看起来完全无效。
2. **封面被裁切**：列表卡片强制 video→16:9、其他→3:4 的 `object-cover` 裁切；详情页封面进一步强制 `aspect-[16/9] max-h-96` 裁切，竖图被剪成横向矩形。ui-spec 要求原创卡「自适应高度、按原始宽高比」，实现违背了唯一视觉权威；且上传链路从不采集图片/视频宽高，任何按比例渲染都无数据可用。
3. **媒体承载形态缺失**：image/video 内容的多文件被塞进「附件」语义，详情页渲染成下载列表而非画廊；上传器不支持多选、无预览、无排序、二次上传即覆盖；不存在全屏媒体查看器、滑动翻页、连续浏览或相关内容入口。

同时收藏集弹窗存在两处不合理交互：点击弹窗外部和 Esc 不能关闭（偏离同仓库 ConfirmModal 既有模式）；弹窗内操作成功的全局 toast 被详情浮层的变暗背景遮盖几乎不可见。

本规格把媒体体验从「附件下载列表」重新定义为「媒体集浏览体验」，覆盖语义、数据、上传、列表、详情、查看器、连续浏览与相关修复，为后续拆票、实现与发布验证建立稳定合同。

## Solution

将「媒体集」确立为独立于「附件」的内容语义：image 内容承载纯图片集（2~9 张），video 内容承载纯视频集（1~3 个），纵横比可混，不图文混排。发布端提供九宫格媒体编排（多选、拖拽排序、移除、宽高采集、视频首帧自动截帧生成 poster），「第一张即封面」。

详情与浮层媒体区采用 contain 不裁切 + 几何稳定的浏览范式，辅以低调位置指示点、滑动与按钮双翻页；点击媒体区进入全屏媒体查看器（缩放、翻页、Esc/外部点击/按钮退出）。桌面端详情浮层改为左媒体右信息双栏，移动端保持全屏单列。列表卡片按媒体集首项（或视频 poster）自然比例渲染瀑布流，极端比例限高。

移动端支持沿触发上下文列表上滑逐篇的连续浏览；桌面/web 端在正文与评论区之后展示相关内容块，不做自动加载下一篇。上传时采集宽高与稳定顺序；`063_content_media_metadata.sql` 为内容封面补宽高、为附件补 `sort_order`。老数据不回填，读取时使用确定性的兼容排序与前端 3:4 默认比例。

修复原创区筛选（前端死代码 + 后端推荐排序带筛选降级热门），收藏集弹窗补齐背板点击/Esc 关闭并在 busy 中阻止关闭，操作反馈改为弹窗内短暂通知。

## User Stories

1. As a 原创区浏览者, I want clicking a category tab to actually narrow the feed to that category, so that filtering behaves as advertised.
2. As a 原创区浏览者, I want selecting a category without touching sort to show popular content of that category, so that the default result is sensible rather than a cross-category mix.
3. As a deep-link visitor, I want stale URLs carrying `sort=recommended` with filters to still behave sensibly, so that old links never silently return wrong results.
4. As an image creator, I want to select multiple images in one go, so that I can publish a gallery without repeated uploads.
5. As an image creator, I want a nine-grid thumbnail preview of my uploaded media, so that I can see the gallery before publishing.
6. As an image creator, I want to drag to reorder media, so that I can control which image is seen first.
7. As an image creator, I want to remove any uploaded item, so that a mistaken upload does not force a redo.
8. As an image creator, I want the first media item to become the cover automatically, so that I do not need a separate cover step.
9. As a video creator, I want my video upload to automatically generate a first-frame poster, so that my video is always displayable without extra work.
10. As a video creator, I want to optionally upload a custom cover image, so that I can show a deliberate frame instead of the first frame.
11. As a video creator, I want the form to state that a missing custom cover falls back to the first video frame, so that I understand the default behavior.
12. As a content browser, I want the detail media area to show the full media without cropping, so that vertical and horizontal media both read completely.
13. As a content browser, I want the media container geometry to stay stable while switching images, so that the layout does not jump between items.
14. As a content browser, I want visible but subtle position dots for the media gallery, so that I always know which item I am on without distraction.
15. As a content browser, I want to page through gallery items with swipe gestures or on-screen buttons, so that navigation works with touch and mouse alike.
16. As a content browser, I want an extremely tall image to cap its height and scroll internally, so that a long image cannot break the detail layout.
17. As a content browser, I want to click into any media item to open a fullscreen media viewer, so that cropped or small media can always be inspected fully.
18. As a media viewer user, I want pinch/zoom control of the media, so that details can be examined closely.
19. As a media viewer user, I want to page through the media set inside the viewer, so that full inspection covers the whole gallery.
20. As a media viewer user, I want to exit via backdrop click, close button or Escape, so that the viewer never traps me.
21. As a desktop visitor, I want the detail overlay to use a two-panel layout with media on the left and information on the right, so that vertical media gets full viewport height and comments stay readable.
22. As a mobile visitor, I want the detail surface to remain a full-screen single column, so that media and information flow naturally on a narrow screen.
23. As a feed browser, I want cards to show the cover at its natural aspect ratio, so that the masonry reads like the original media rather than uniform crops.
24. As a feed browser, I want extremely wide or tall covers to be height-capped, so that one card cannot dominate the masonry.
25. As a mobile browser, I want to swipe up past the last media item to advance to the next content in the triggering list, so that I can keep browsing without returning to the grid.
26. As a mobile browser, I want reaching the end of the context list to show an explicit end prompt, so that I know the list is exhausted.
27. As a desktop/web browser, I want related content (similar recommendations plus linked originals/fanworks) after the body and comments, so that I can continue discovering without surprise auto-advance.
28. As a desktop/web browser, I want a clear "already at the end" prompt after the related block, so that reaching the bottom is an explained state.
29. As a collection picker user, I want clicking the backdrop or pressing Escape to close the picker, so that dismissal works the way it does everywhere else.
30. As a collection picker user, I want dismissal blocked while an add/create/remove request is in flight, so that a running operation is not lost.
31. As a collection picker user, I want add/create/remove results and errors to appear as a brief notice inside the picker, so that feedback is visible even when the page behind is dimmed.
32. As a collection picker user, I want the notice to be non-blocking and non-covering, so that I can keep interacting while it shows.
33. As a collection picker user, I want the "已加入" row badge to stay as the persistent state marker, so that short-lived notices do not replace durable state display.

## Implementation Decisions

### 语义与数据

- **媒体集 vs 附件**：确立「媒体集」（Media Gallery，内容内可顺序浏览的媒体序列）与「附件」（面向下载的素材文件）的语义拆分。存储仍复用既有附件表（按 `file_type` 区分），不拆表；渲染与上传链路完全分离：媒体集走画廊/播放器，附件走下载列表。
- **媒体集纯净规则**：新发布的 image 内容 = 纯图片集（2~9 张）；新发布的 video 内容 = 纯视频集（1~3 个）；不允许图文混排；纵横比可混（横图/竖图/横视频/竖视频同集）。数量上下限是运行时配置，由 `config.yaml`/Go config 定义并通过 public config 暴露安全值；后端在消耗 upload grant 和写库前权威校验，前端只消费该合同。历史单图/单视频内容仍须可读取和渲染，不因新建规则变成不可见。其他内容类型（article/sheet_music/mod/audio/template/prompt）维持附件语义。
- **封面规则**：image 内容封面 = 媒体集首项（派生，无独立「设为封面」入口，排序第一位即封面）；video 内容封面 = 自定义 poster（可覆盖），缺省由前端在发布时从视频首帧（约 0.1s）经 `<video>` + canvas 截帧导出 JPEG，走既有 oss-token 流程上传为独立文件（poster 不属于媒体集）；发布表单注明「未上传封面将取视频第一帧作为封面」。
- **宽高与顺序采集**：上传时前端读取图片 `naturalWidth/Height`（视频取首帧尺寸），随附件提交写入既有 `width/height` 字段，并提交从 0 开始的 `sort_order`。服务端拒绝负数、重复顺序、非正宽高和媒体类型不匹配；repository 始终按 `sort_order ASC NULLS LAST, id ASC` 返回附件，旧行以 id 稳定兜底。首版不支持发布后重排。
- **封面反范式化**：`content_items` 新增 `cover_width` / `cover_height`（可空 int），发布时写入（image = `sort_order=0` 首项尺寸；video = poster 尺寸）。列表接口零 join 即可提供自然比例所需数据。新迁移 `063_content_media_metadata.sql` 同时新增附件 `sort_order` 及其同内容唯一约束；老数据不回填不迁移，前端对空值使用 3:4 默认比例。
- **poster 安全合同**：自定义/自动 poster 与媒体条目一起使用 OSS upload grant 提交；后端必须消费并核验 grant、确认它属于当前发布者且为图片，再派生持久 cover URL/OSS key 与尺寸。不得接受客户端任意 `cover_image_url`，也不得继续依赖当前未被后端识别的 `cover_oss_key` 漂移字段。`is_primary` 只标记派生封面条目；媒体顺序只由 `sort_order` 决定。
- **上传编排**：媒体选择器改造为多选上传 + 九宫格缩略图预览 + 拖拽排序（HTML5 DnD）+ 移除；数量上限按媒体集规则校验；视频上传后自动截帧生成 poster，提供「上传自定义封面」覆盖入口。

### 显示层

- **详情/浮层媒体区**：contain 不裁切；容器几何由首项决定并在浏览会话中保持稳定（切换不跳版）；超高图（`height / width > 2`）限高 + 内部滚动；底部低调位置指示点（透明圆 = 未浏览，实心 = 当前）；滑动 + 按钮双翻页。
- **媒体查看器**：全屏媒体浏览层（规范化既有「图片预览」语义），可叠加在内容详情浮层最上层；点媒体区进入；支持缩放（pinch/滚轮/按钮）、媒体集内翻页、外部点击/关闭按钮/Esc 退出；Esc/背板只关闭最上层，关闭后恢复下层滚动与焦点。
- **列表卡片**：按媒体集首项（或视频 poster）宽高比渲染（`aspect-ratio` 由 `cover_width/cover_height` 数据驱动），不裁切；极端比例（`max(width / height, height / width) > 2`）按高度上限 contain；无数据场景防御性默认 3:4。
- **桌面双栏**：桌面端详情浮层改为左媒体右信息——媒体区高上限 = 视口可用高、宽度按媒体比例自适应；信息区（标题/作者/操作/正文/评论区）独立滚动；「封面与正文共享同一水平框架」的既有约束继续成立。移动端保持全屏单列（媒体全宽 contain，信息在其下滚动）。双栏只用于 image/video 内容；文本型内容（article/sheet_music 等）维持单栏。
- **连续浏览（移动端）**：详情浮层入口（卡片网格）把触发上下文列表 + 当前索引传给浮层；媒体集内翻页，最后一项继续上滑 = 切换上下文列表中的下一篇；列表到底提示「已经到底」；不提供「上一篇」；查看器不参与内容级切换。
- **相关内容（桌面/web）**：正文与评论区之后展示相关内容块 = 关联原创/二创（由 source-linkage 先交付最终 `RelatedFanworks`/related-fanworks 契约）+ 相似内容。相似内容固定复用列表 API：同 zone、同 `content_type`、同 category（fanwork 有 IP 时再带 `ip_id`）、`sort=hot&page_size=12`，客户端去除当前内容及关联行重复项后最多显示 8 条；不新增临时 similar endpoint，也不把带筛选的 `sort=recommended` 误称为推荐管线。再滚动提示「已经到底了」；不做自动加载下一篇。

### 修复层

- **筛选 bug**：前端修正死代码——无分类且未显式选排序 → recommended；有分类 → 默认 hot（`search.sort` 的默认值语义修正，不再把 "recommended" 误传给带筛选的请求）。后端 `handleRecommended` 防御：携带任何筛选条件（category/content_type/tags/author/ip 等）时降级为 hot 排序并打结构化日志，保证旧 URL/深链语义正确。二创首页为固定 `sort=hot`，无同类问题。
- **收藏集弹窗**：对齐 ConfirmModal 模式——背板点击 + Esc 关闭、焦点陷阱与焦点恢复、创建/添加/移除请求进行中（busy/creating）阻止关闭；只改收藏集弹窗，不扩及其他浮层。
- **弹窗内通知**：收藏集弹窗 footer 上方绝对定位浮动小条（`pointer-events-none`、半透明底、约 2 秒自动淡出、出现/消失不引起布局跳动）；成功（加/移/创建）与失败都走弹窗内通知；行内「已加入」徽标保留为持久状态标记；弹窗内不再触发全局 toast（弹窗外的收藏动作不受影响）。

### 范围

- 本轮 = Web 响应式（桌面浏览器 + 移动浏览器）；Tauri 客户端不设计不预留（桌面范围暂缓，`desktop_deploy_enabled=false`）。

## Testing Decisions

- **测试原则**：只测外部行为，不测实现细节；同一行为只有一条测试路径（handler 或浏览器），优先既有缝。
- **HTTP handler 缝（后端）**：`ListContents` 筛选+排序契约——`sort=recommended` 无筛选走推荐、带 category 降级 hot 并断言内容属于该分类（先例：`content_recommended_route_test.go` 已断言 recommended 语义）；发布链路权威校验媒体数量/类型/宽高/顺序/poster grant，断言封面派生与 `cover_width/cover_height` 写入（image = 首项、video = poster）；063 迁移结构、附件唯一顺序和确定性读取断言（先例：`series_migration_test.go`）。
- **服务集成缝（后端）**：封面派生与 recommended 降级日志行为（先例：既有 service 集成测试）。
- **浏览器缝（前端）**：上传编排（多选/九宫格/拖拽排序/移除/数量上限）、画廊翻页与指示点、媒体查看器（进入/缩放/翻页/三种退出）、双栏与移动单列响应式几何、自然比例瀑布流、移动端连续浏览（上滑前进/到底提示）、相关内容块与「已经到底」、收藏集弹窗（背板/Esc 关闭、busy 阻止、弹窗内通知）——走 `verify-project.sh` 契约 + `screenshots/` 桌面与移动截图证据（与 #64 浏览器验证模式一致）。
- **不新建测试抽象**：理想缝数量 = 2（handler + 浏览器），服务集成缝仅补充 handler 覆盖不到的事务行为。

## Out of Scope

- Tauri 客户端与桌面壳（范围暂缓）
- 服务端自动截帧（FFmpeg/OSS 媒体转码管线）
- 图文混排媒体集
- 「上一篇」内容导航
- 桌面/web 端自动加载下一篇
- 收藏集弹窗以外其他浮层/弹窗的同类修复
- 老数据回填与历史媒体迁移
- 推荐算法本身改动（相似推荐仅复用既有管线，必要时仅新增收敛查询）

## Further Notes

- **串行与共享面**：#65 先统一 #64/#80 与 `design/ui-spec.md` 权威。媒体 T03 (#83) 为 heavy 数据/发布合同，可与 #65 并行；T01/T02/T04 及显示票按原生依赖等待相应权威/共享面。#68 完成 ContentDetail 收敛和共享 overlay hook 后，媒体 T05~T09 才进入共享显示面；随后 source-linkage #96 交付最终来源/衍生作品合同，再由媒体 T10 (#90) 组装相关内容块；collaboration-invites #97 最后接入 PublishForm/i18n。ticket 必须写明精确文件预约。
- **决策记录**：媒体体验范式（contain + 双栏 + 连续浏览 + 相关内容）记录于 `docs/adr/0004-media-experience-paradigm.md`；术语「媒体集/附件」已入 GLOSSARY，「媒体查看器/连续浏览/相关内容/首图规则」随本规格一并写入。
- **ui-spec 修订**：#65 是唯一权威更新 owner，须在依赖 UI 实现前对齐 `design/ui-spec.md` 的 ContentCard/MasonryGrid、FileUploader/PublishForm、ContentDetail、ContentDetailOverlay、/original 分类 Tab 与 CollectionPicker 章节。`ui-spec` 是唯一视觉权威；后续 ticket 只实现并验证，不得各自重写相同章节。
- **i18n**：新增文案全部走 `next-intl`，建议 namespace：`media.gallery.*`、`media.viewer.*`、`media.related.*`、`collections.picker.notice.*`。
