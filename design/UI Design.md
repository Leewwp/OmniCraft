# OmniCraft 前端规格文档（Gemini UI 设计素材）

## 一、前端技术栈

| 层级      | 技术                                 | 说明                                                         |
| --------- | ------------------------------------ | ------------------------------------------------------------ |
| 框架      | Next.js 15（App Router + SSR）       | 公开页面 SSR，认证页面 CSR                                   |
| 语言      | TypeScript（strict mode）            | 禁止 any 类型                                                |
| 样式      | Tailwind CSS + 自定义 design token   | canvas / border / fg / accent / tag 色彩系统                 |
| 组件库    | shadcn/ui（基于 Radix UI）           | Button / Modal / Toast / Tabs / Card / DropdownMenu / Select / Popover / Switch |
| 图标      | Lucide React                         | 全站统一图标库                                               |
| 布局      | react-masonry-css                    | 内容瀑布流                                                   |
| 主题      | next-themes                          | light / dark / system 三档切换，class="dark"                 |
| 国际化    | next-intl（无路由前缀）              | zh-CN / en-US，从 cookie 读取                                |
| MD 编辑   | @uiw/react-md-editor                 | 发布页富文本编辑                                             |
| MD 渲染   | react-markdown + syntax-highlight    | 内容详情页渲染                                               |
| Diff 渲染 | diff-match-patch                     | PR 三栏对比                                                  |
| 乐谱渲染  | opensheetmusicdisplay（动态 import） | MusicXML 五线谱渲染                                          |
| MIDI 播放 | midi-player-js + soundfont-player    | MIDI 文件播放                                                |
| 状态管理  | React Context（AuthContext）         | 登录态全局管理                                               |
| HTTP 请求 | 封装 fetch（lib/api.ts）             | 自动带 JWT，统一错误格式                                     |
| SSE 流式  | 封装 EventSource（lib/useSSE.ts）    | Agent 流式响应                                               |

**设计风格基准**：GitHub 黑白底色 + 低饱和强调色，扁平无 box-shadow 卡片，系统字体栈（-apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial）。

---

## 二、页面清单

### 2.1 公开页（无需登录）

#### P01 首页 `/`（二创区默认展示）
- **定位**：二创区内容瀑布流默认入口，平台流量核心（参考 Steam 创意工坊风格）
- **核心功能**（三区域布局，从上到下）：
  - **区域 1 — 最近访问 IP**：横向滚动快捷入口栏（localStorage 存储最近 5 个 IP）
  - **区域 2 — IP 浏览区**：横向滚动 IPCard 列表 + 分类筛选 Tab（全部 / 游戏 / 影视 / 综艺 / 短剧 / 动画 / 漫画 / 小说 / 明星偶像 / 音乐 / 虚拟主播 / 其他，对应 `ips.category`）+ 排序（最热门 / 最新 / 最多内容）
  - **区域 3 — 二创内容浏览区**：
    - 内容类型筛选 Tab：全部 / 文字 / 图片 / 视频 / 音频 / Mod / AI提示词 / 乐谱 / 其他（对应 content_items.content_type 枚举 sheet_music 独立）
    - 排序 Tab：最热门 / 最新 / 最多点击 / 最高好评率
    - 时间范围筛选器：全部 / 本周 / 本月 / 本年
    - 主区域：4 列瀑布流 ContentCard（响应式：移动 2 / 平板 3 / PC 4），展示所有 IP 的二创内容
- **特殊状态**：空状态、加载骨架屏、无限滚动加载更多
- **设计说明**：首页不使用分面搜索侧边栏（与搜索页区分）。内容浏览区筛选条件均为横向 Tab / 下拉，保持简洁

#### P02 搜索页 `/search`
- **定位**：自然语言 + 分面筛选双模式搜索，类 Github 搜索
- **核心功能**：
  - 顶部 SearchAgentInput（AI 自然语言搜索，降级回关键词搜索）
  - 左侧分面搜索侧边栏（大类选择 → 标签共现列表 → Advanced 区折叠）
  - 右侧结果区：瀑布流 ContentCard（MVP 仅瀑布流，列表模式切换留待 P1）
  - 已选筛选条件 chips 展示，支持逐个取消
  - 「保存此搜索」按钮
- **特殊状态**：无结果空状态、搜索中 loading、AI 不可用时降级提示

#### P03 IP 详情页 `/ip/[ipId]`
- **定位**：IP 聚合页，类 Steam 创意工坊
- **核心功能**：
  - 顶部 IP 封面大图 + IP 名称 + 分类 + 简介 + 创作者信息
  - **关注 IP 按钮**：登录用户可关注/取消关注 IP（FollowButton 组件复用），显示关注者数量
  - 内容类目 Tab（文字 / 图片 / 音频 / 视频 / Mod / 提示词 / 乐谱 / 讨论区）——与首页 Tab 一致，乐谱（sheet_music）作为独立类目
  - 说明：表情包、动图作为图片类目下的标签筛选项，不单独设 Tab
  - 每类目下支持排序（最热门 / 最高点击 / 最新 / 最高好评率）
  - 类目内容区域：瀑布流 ContentCard
  - 讨论区入口 Tab（跳转讨论贴列表）
  - 最近访问时写入 localStorage
- **特殊状态**：IP 审核中提示、IP 被封禁提示

#### P04 IP 类目内容列表 `/ip/[ipId]/[category]`
- **定位**：IP 下单一类目的完整内容列表页
- **核心功能**：
  - 面包屑导航（IP 名称 > 类目名）
  - 排序 + 时间筛选
  - 瀑布流 ContentCard
  - 标签筛选（仅该 IP + 该类目下的标签）

#### P04a 讨论区列表 `/ip/[ipId]/discussions`（参考 Steam 创意工坊讨论区）
- **定位**：IP 讨论区帖子列表，参考 Steam Workshop Discussions 的设计思路
- **整体布局**（上到下）：
  - 面包屑导航：IP 名称 > 讨论区
  - 工具栏：左侧“发帖”按钮（登录后显示） + 右侧讨论区内搜索框（搜索标题+内容）
  - 筛选栏：排序 Tab（最新回复 / 最新发帖 / 最多回复）
  - 置顶帖区：高亮背景的置顶帖列表（管理员/IP 创建者可置顶），左侧图钉图标 + 「Pinned」标签
  - 普通帖列表：标准 DiscussionCard 列表，分页加载
- **DiscussionCard 布局**（参考 Steam 讨论区行项）：
  - 左侧作者头像（32px 圆形）
  - 中间：标题（1 行截断，点击进入详情）+ 作者名称 + 发帖时间
  - 右侧：回复数图标+数字 / 浏览数图标+数字 / 最后回复时间 + 最后回复者头像
  - 统一 1px border-bottom 分隔（GitHub 扁平风）
- **特殊状态**：
  - 无讨论时 EmptyState（图标 + “还没有讨论，来发第一贴吧” + “发帖” CTA）
  - 搜索无结果时显示“未找到相关讨论”
  - 列表加载中显示骨架屏
- **设计说明**：不使用分区 Tab（如 Steam 的 General/Bug 分区），保持单一列表简洁。后期 P1 可考虑增加讨论分区

#### P04b 讨论详情 `/ip/[ipId]/discussions/[discussionId]`（参考 Steam 帖子详情页）
- **定位**：单个讨论帖详情 + 回复列表
- **整体布局**（上到下）：
  - 面包屑导航：IP 名称 > 讨论区 > 帖子标题
  - **主帖区（OP）**：作者头像/名称/发帖时间 + Markdown 渲染内容 + 操作栏（举报 / 编辑（仅作者））
  - **回复列表**（ReplyList 组件，参考 Steam 楼层式回复）：
    - 每条回复：左侧头像 + 右侧气泡（用户名 + 回复时间 + 内容 + 点赞数 + 回复按钮 + 举报按钮）
    - 楼中楼缩进（最多 2 层，超过后显示“展开更多回复”）
    - 回复排序：时间正序（最旧在前，与 Steam 一致）
    - 分页加载（每次 20 条）
  - **回复输入区**：底部固定 Markdown 输入框 + “发表回复”按钮（登录后显示）
- **特殊状态**：帖子被删除/隐藏提示、未登录时回复区显示“登录后参与讨论”

#### P04c 发帖页 `/ip/[ipId]/discussions/new`（需登录）
- **核心功能**：标题输入 + Markdown 编辑器（复用 MarkdownEditor 组件），提交后跳转到该帖子详情页
- **设计说明**：简洁表单页，与 Steam 发帖框类似，无分区选择（P0 不做分区）

#### P05 内容详情页 `/content/[contentId]`
- **定位**：二创区内容详情，核心消费场景
- **核心功能**：
  - 顶部封面图（有则显示，无则显示 content_type 对应 SVG 占位）
  - 标题 + 作者信息 + IP 来源 + 发布时间 + 标签列表
  - 标签旁「+/-」微型按钮（hover 显示，点击提交标签建议）
  - 正文区：按 content_type 渲染（MD 文章 / 图片 Lightbox / 视频播放器 / 音频播放器 / 乐谱 Viewer）
  - 右侧边：「AI 使用指导」折叠卡片（UsageGuidePanel，懒加载 SSE 流式渲染）
  - 操作栏：点赞 / 点踩 / 收藏 / 下载（allow_copy=true）/ 一键部署（agent_enabled=true）/ 举报
  - 版本历史时间轴（侧抽屉，仅二创区显示）
  - PR 协同申请按钮（登录后显示，允许其他人提交修改）
  - 评论区（分页加载）

#### P06 原创区首页 `/original`
- **定位**：原创内容聚合，无 IP 概念，类小红书/B站创作区
- **核心功能**（两级导航）：
  - **一级分类 Tab**（按潜在用户量排序）：推荐（默认）/ 影视 / 游戏 / 文学 / 宠物 / 美食 / 美妆穿搭 / 家居 / 数码科技 / 旅行 / 运动 / 效率（对应 `content_items.category`）
  - **二级内容类型子筛选**（选择具体分类后显示）：全部 / 图片 / 视频 / 音频（含乐谱） / 文字 / 效率模板 / 模型与设计 / 其他
  - > **分类动态管理**：所有大类、二级分类、内容类型均从后端 API 动态加载，由管理员后台统一管理（增删改排序），降低新增品类成本
  - 排序 Tab：最热门 / 最新 / 最多点击 / 最高好评率
  - 主区域：瀑布流 ContentCard（响应式：移动 2 / 平板 3 / PC 4），展示原创内容
- **设计说明**：不使用分面搜索侧边栏（与搜索页区分），保持简洁。「推荐」分类由后端算法混合推荐

#### P07 原创内容详情页 `/original/[contentId]`
- **定位**：原创区内容详情，复用 P05 布局但隐藏 PR 相关 UI
- **差异**：无「IP 来源」标签，无「PR 协同申请」按钮，无版本历史（原创区不开放 PR）
- **原创/二创联动**：
  - 首屏加载时请求 `GET /contents/:id/related-fanworks?page=1&page_size=1`
  - 当 `total > 0` 时在主操作区展示「相关二创」按钮，点击进入 `/original/[contentId]/fanworks`
  - 展示「基于此原创发布二创」入口，跳转 `/publish?zone=fanwork&source_original_id=<id>`
  - 当无相关二创时不展示「相关二创」按钮，避免进入空列表

#### P07a 相关二创列表 `/original/[contentId]/fanworks`
- **定位**：某个原创内容的来源二创聚合页
- **核心功能**：
  - 顶部显示源原创标题和「返回原创详情」
  - 排序：最新 / 最热门 / 最多点击 / 最高好评率
  - 类型筛选：全部 / 文字 / 图片 / 视频 / 音频/乐谱 / Mod / 其他
  - 主区域复用 MasonryGrid + ContentCard，仅展示 `source_original_id=<id>` 且已发布的二创
- **特殊状态**：无结果时显示“暂无相关二创”；源内容不存在或不是原创时进入 404

#### P08 用户主页 `/user/[userId]`
- **定位**：创作者公开主页，展示用户完整创作档案和社交关系
- **页面布局**（从上到下）：
  - **顶部用户卡片区**：
    - 用户头像（大头像）+ 用户名 + 简介文本（可换行）
    - 信誉分徽章（badge）+ 注册时间（日期）
    - 判官资质徽章区：已获得的判官资质类型 Badge 列表（JudgeQualBadge，如"文章判官"、"图片判官"），无资质时不显示
    - 关注/粉丝计数（可点击弹出 Modal）
    - 操作按钮区（非本人时显示）：
      - FollowButton（关注/取消关注，显示已关注状态）
      - 发私信按钮（跳转私信对话或打开对话 Modal）
    - 查看自己主页时显示：「编辑资料」按钮（跳转 /settings）+ 「申请判官资质」入口（跳转 /judge/exam）
  - **内容浏览 Tab 区**（四个 Tab 并列）：
    - 发布的内容：瀑布流 ContentCard 展示（响应式 2/3/4 列），排序支持 newest/ hot / most_views
    - 收藏：瀑布流展示收藏的内容
    - 参与的讨论：list 或 card 方式展示用户参与过的讨论帖（帖子标题 + 用户最后回复时间 + 回复数）
    - （预留）关注者发布的内容（P1 新增推荐 tab）
  - **FollowerListModal**（关注/粉丝列表弹窗）：
    - 弹窗标题：「关注者」或「关注中」（Tab 切换）
    - 用户列表：头像 + 用户名 + 简介 + FollowButton（支持一键关注/取关）
    - 分页加载：每页 20 条

- **特殊状态**：
  - 无发布内容时显示 EmptyState（"还没有发布任何内容"）
  - 无收藏内容时显示 EmptyState（"还没有收藏任何内容"）
  - 无讨论参与时显示 EmptyState（"还没有参与讨论"）

- **创作者支持信息展示**（P1 启用时显示）：
  - 用户卡片底部补充展示："支持创作者"区域 + 打赏码图片 + 外部链接按钮（若已配置）

#### P09 登录页 `/login`
- **核心功能**：邮箱 + 密码登录表单，错误提示，「前往注册」链接，记住登录状态开关

#### P10 注册页 `/register`
- **核心功能**：用户名 + 邮箱 + 密码 + 确认密码，注册成功自动登录跳转首页

---

### 2.2 认证页（需登录，未登录自动跳 `/login`）

#### P11 账号设置 `/settings`
- **核心功能**：
  - 修改用户名 / 头像 / 简介
  - 邮箱字段：只读展示（MVP 不开放修改，避免与 OAuth 绑定冲突；P1 再开放）
  - 修改密码（旧密码验证）
  - 语言偏好（ZH / EN）
  - 主题偏好（Light / Dark / System）
  - 账号注销入口（危险区域）

#### P12 标签组管理 `/settings/tag-groups`
- **核心功能**：
  - 已创建标签组列表（名称 + tags 数组 + 编辑 / 删除按钮）
  - 「新建标签组」按钮 → 弹出创建 Modal
  - 创建/编辑 Modal：名称输入 + 标签自动补全输入

#### P13 发布内容 `/publish`
- **定位**：多类型内容发布页，创作者核心操作页
- **核心功能**：
  - Step 1：选择发布区域（二创区 / 原创区）
    - 切换区域保护：若用户已上传文件或填写过表单字段，切换时弹出 ConfirmModal（“切换发布区域将丢失已填内容，是否继续？”），确认后清空草稿
  - Step 2：选择内容类型（图标化选择器，含乐谱类型）
  - Step 3：填写内容
    - MD 编辑器（文章类）
    - 文件上传（FileUploader，OSS 直传，进度条）
    - 封面图上传（独立上传区，可预览）
    - 乐谱文件上传（限定扩展名）
  - Step 4：元数据
    - 标题 / 简介输入
    - 标签输入（自动补全 TagInput）
    - IP 选择（仅二创区显示）
    - 来源原创选择（仅二创区显示，可由 `source_original_id` 查询参数预填；发布原创时不得提交该字段）
    - 原创区分类选择（仅原创区显示）：一级分类 Select（动态加载 `GET /categories?zone=original&level=primary`）+ 二级分类 Select（父级联动，动态加载 `GET /categories?zone=original&level=secondary&parent_id=<id>`），写入 `content_items.category`
  - Agent 面板（web_agent_enabled=true 时显示）：
    - 「AI 自动填写」按钮（上传后触发 UploadAssistPanel）
    - 合规检测徽章（ComplianceCheckBadge，提交前触发）
  - Step 5：权限配置
    - is_public 开关
    - allow_copy 开关
    - agent_enabled 开关（一键部署）
  - 预览 + 提交

#### P14 创作者后台概览 `/dashboard`
- **核心功能**：
  - 数据卡片：总发布数 / 总浏览数 / 总点赞数 / 本周新增
  - 待处理事项快捷入口：待审 PR N 条 / 待审标签建议 N 条
  - 最近发布的内容列表（5 条）
  - 创作者支持设置区域（`features.creator_support_enabled=true` 时显示，复用 `CreatorSupportPanel` 编辑模式：打赏码图片上传 + 最多 3 个外部链接，详见 Task 77 / P1 模块）
  - 左侧后台导航栏（Overview / 我的内容 / PR 申请 / 贡献者 / 标签建议）

#### P15 我的内容 `/dashboard/contents`
- **核心功能**：
  - 内容列表（表格或卡片模式）：标题 / 类型 / 状态 / 浏览数 / 发布时间
  - 状态筛选（全部 / 已发布 / 审核中 / 已隐藏）
  - 每行操作：编辑 / 下架 / 查看详情
  - 批量操作：批量下架

#### P16 协同 PR 申请管理 `/dashboard/pr-requests`
- **核心功能**：
  - PR 列表：提交者头像 + 用户名 / 对应内容标题 / 提交时间 / 状态
  - 状态 Tab：待处理 / 已接受 / 已拒绝
  - 点击进入 PR 详情（展开或跳转）：DiffViewer 三栏对比 + 接受 / 拒绝 / 手动合并按钮

#### P17 贡献者管理 `/dashboard/contributors`
- **核心功能**：贡献者列表（头像 + 用户名 + 贡献 PR 数 + 最近贡献时间），可解除贡献者身份

#### P18 标签建议审核 `/dashboard/tag-suggestions`
- **核心功能**：
  - 建议列表：内容标题 / 建议标签 / 操作（添加/删除）/ 提交用户 / 时间
  - 每条操作：通过（写入 content_tags）/ 拒绝 / 举报（恶意建议）
  - 批量通过 / 批量拒绝

#### P19 赛博判官资质考核 `/judge/exam`
- **核心功能**：
  - 选择要报考的内容类型
  - 考题区域（单选/多选题，10 题）
  - 计时器
  - 提交后立即显示成绩 + 通过/未通过状态
  - 通过：颁发判官资质徽章 + 跳转判官队列

#### P20 待审内容队列 `/judge/queue`
- **核心功能**：
  - 当前有资质的内容类型 Tab
  - 待审内容卡片：预览图 + 标题 + 举报原因 + 当前投票进度（已投 / 最低阈值）
  - 投票操作：不违规 / 违规（ConfirmModal 二次确认）+ 可选输入判定理由
  - **撤回机制**：投票提交后 10 秒内在卡片顶部显示“撤回投票”按钮（倒计时 Badge），点击调用 `DELETE /judge/votes/:voteId` 撤销；10 秒后按钮消失，投票最终生效不可修改
  - **判决详情页**（投票后展示）：当前投票分布（不违规/违规比例） + 其他判官提交的理由列表（可点赞/点踩，按赞数排序）
  - 已完成的判定历史（折叠）

#### P21 浏览历史 `/history`
- **核心功能**：按日期分组（今天 / 昨天 / 更早）的浏览记录列表，ContentCard 缩小版，底部「清除所有记录」按钮（ConfirmModal 二次确认）
- **分页**：每页 30 条，按 `viewed_at` 倒序，滚动到底加载更多
- **特殊状态**：无浏览记录时 EmptyState（“还没有浏览记录，去逛逛吧”）

#### P27 我的申诉 `/appeals`
- **定位**：用户查看和管理自己提交的申诉
- **核心功能**：
  - 申诉列表：被封内容标题 / 申诉理由 / 提交时间 / 当前状态（pending/approved/rejected）
  - 提交新申诉按钮（选择被封内容 + 输入申诉理由）
  - 已处理申诉显示处理结果和管理员反馈
- **特殊状态**：无申诉时 EmptyState

#### P28 消息中心 `/messages`
- **定位**：通知 + 私信统一入口
- **核心功能**：
  - 左侧 Tab 切换：回复我的 / 收到的赞 / 系统消息 / 我的消息（私信）
  - **回复我的**（channel='reply'）：他人回复我的评论/讨论列表，点击跳转对应内容
  - **收到的赞**（channel='like'）：他人点赞我的内容/评论列表
  - **系统消息**（channel='system'）：审核结果、判官结果、信誉分变动等
  - **我的消息**：私信对话列表（ConversationList），点击进入对话窗口（ChatWindow）
  - 每个通知 Tab 顶部「全部已读」按钮
  - 单条通知右侧 hover 显示「标记已读」/「删除」图标按钮（调用 `PATCH /notifications/:id/read` / `DELETE /notifications/:id`）
  - Header 铃铛图标显示总未读数 Badge
- **特殊状态**：各频道空状态、消息加载中

#### P29 素质建设课程 `/rehab`
- **定位**：信誉分低于阈值的用户通过学习恢复信誉分
- **核心功能**：
  - 可用课程列表（基于用户扣分记录自动匹配）：CourseCard 显示课程标题 / 违规类型 / 可恢复分值 / 是否已完成
  - 课程详情页：AI 生成的 Markdown 教学内容（CourseContent 渲染）
  - 底部计时器（倒计时 3 分钟最低阅读时间）
  - 阅读完成后「完成学习」按钮，点击后加信誉分
  - 我的学习进度概览
- **特殊状态**：信誉分正常时提示「无需学习」、所有课程已完成提示
- **设计说明**：课程内容支持 i18n（zh-CN / en-US），教学风格友善非惩罚性

---

### 2.3 管理员后台（role=admin 才可访问）

#### P22 IP 库管理 `/admin/ips`
- 待审 IP 列表（状态：pending），详情弹窗，通过 / 拒绝 + 拒绝原因输入

#### P23 内容终审 `/admin/contents`
- 已被举报 / AI 标记的内容列表，预览，封禁 / 恢复 + 操作原因，支持搜索用户名/内容标题

#### P24 用户管理 `/admin/users`
- 用户列表（用户名 / 邮箱 / 信誉分 / 注册时间 / 是否封禁），封禁 / 解封，查看信誉分日志

#### P25 申诉处理 `/admin/appeal`
- 申诉列表（用户 / 被封内容或评论 / 申诉理由 / 时间），恢复 / 维持下架

#### P26 系统配置 `/admin/config`
- 配置项表单（对应 config.yaml）：上传限制 / Feature Flag 开关 / 信誉分阈值，保存即热更新

#### P26a 分类与标签管理 `/admin/categories`
- **定位**：管理员统一管理全站分类体系
- **核心功能**：
  - 分区 Tab：二创区 IP 分类 / 二创区内容类型 / 原创区大类 / 原创区二级分类
  - 分类列表（拖拽排序）：名称（i18n）/ slug / 是否启用 / 操作（编辑 / 删除）
  - 新建分类按钮：弹出 Modal（名称 zh + en / slug / 父分类选择）
  - 删除校验：存在子分类或关联内容时禁止删除，显示提示

#### P26b Agent 管理 `/admin/agent-config`
- **定位**：管理员可视化管理 LLM 提供商配置，支持多配置切换和连接测试
- **核心功能**：
  - **当前激活配置摘要卡片**：顶部高亮卡片，显示当前激活的 LLM 配置名称 / 提供商类型 / 模型名 / 状态（✅ 已激活）；无激活配置时显示“未配置，将使用 config.yaml 默认值”警告
  - **LLM 配置列表表格**：
    - 列：配置名称 / 提供商类型（`qwen` | `openai_compat`）/ API 地址（截断显示）/ 模型 / 状态（✅ 激活 / ⬜ 未激活）/ 操作
    - 激活行操作：编辑 / 测试连接
    - 未激活行操作：激活 / 编辑 / 删除 / 测试连接
  - **新增配置按钮**：“+ 新增配置”按钮，打开新建/编辑 Modal
  - **新建/编辑 Modal**：
    - 配置名称：TextInput
    - 提供商类型：Select（`qwen` / `openai_compat`）
    - API 地址：TextInput（占位符提示如 `https://dashscope.aliyuncs.com`）
    - 模型名称：TextInput（占位符提示如 `qwen-turbo`）
    - API Key：密码输入框 + 显示/隐藏切换按钮（Eye/EyeOff 图标），编辑时显示“•••••”脱敏化，可重新输入替换
    - 扩展参数（可选折叠）：temperature / max_tokens / JSON 编辑器
    - 底部按钮：取消 / 测试连接 / 保存
  - **测试连接反馈**：点击“测试连接”后按钮显示 loading spinner，成功时显示绿色 Toast（“连接成功，模型响应正常”）+ 返回模型名称，失败时显示红色 Toast + 错误信息
  - **切换激活确认**：点击“激活”后 ConfirmModal（“确认切换当前 LLM 提供商到 [xxx] ？”），确认后刷新列表
- **特殊状态**：
  - 无配置时 EmptyState（“尚未配置 LLM 提供商，当前使用 config.yaml 默认配置” + “新增配置” CTA）
  - 删除确认（ConfirmModal，仅非 active 配置可删）
- **设计说明**：表格布局与 /admin/users 类似，1px border 扁平风；Modal 使用 shadcn/ui Dialog 组件；API Key 不返回明文（后端仅返回脱敏化前 4 位 + 后 4 位）

---

### 2.4 独立弹窗 / 侧抽屉（非独立页面）

| 弹窗                  | 触发场景                     | 说明                            |
| --------------------- | ---------------------------- | ------------------------------- |
| 举报 Modal            | 内容详情页 / 评论区举报按钮  | 举报类型选择 + 说明输入         |
| 私信对话 Modal        | 用户主页「发私信」按钮（移动端跳转 /messages，桌面端可弹窗） | 复用 ChatWindow 组件，首次发送自动创建 conversation |
| PR 提交 Modal         | 内容详情页「申请协同」按钮   | 修改内容输入（MD Editor）+ 说明 |
| 标签建议 Modal        | 内容详情页标签旁 +/- 按钮    | 确认建议添加 / 删除该标签       |
| 保存搜索 Modal        | 分面搜索侧边栏「保存此搜索」 | 命名输入 + 保存                 |
| 标签组创建/编辑 Modal | 标签组管理页                 | 名称 + 标签输入                 |
| 图片 Lightbox         | 内容详情页图片点击           | 全屏预览 + 左右切换             |
| 版本历史侧抽屉        | 内容详情页「版本历史」按钮   | 时间轴 + 版本对比入口           |
| 部署确认 Modal        | 内容详情页「一键部署」按钮   | 确认目标目录 + 唤醒 Tauri       |

---

## 三、组件拆分

### 3.1 布局组件（`components/layout/`）

| 组件                       | 说明                                                         |
| -------------------------- | ------------------------------------------------------------ |
| `Header.tsx`               | 顶部导航：Logo / 最近访问 IP 快捷栏 / 搜索框 / 发布按钮 / NotificationDropdown（铃铛+未读数）/ 登录状态（头像 + 下拉菜单）/ 语言切换（ZH/EN）/ 主题切换（Sun/Moon 图标） |
| `Footer.tsx`               | 底部：版权信息 / 链接 / 语言切换备选                         |
| `Sidebar.tsx`              | 侧边栏容器：包裹 FacetedSearchSidebar + 我的标签组快捷按钮区域 |
| `FacetedSearchSidebar.tsx` | 分面搜索核心组件：大类 Tab → 动态标签列表（含内容数量 Badge）→ 已选 chips → Advanced 折叠区（内容类型多选 + 时间范围 + 排序）→ 保存搜索按钮 → 我的标签组列表 |
| `DashboardNav.tsx`         | 创作者后台左侧导航（仅 /dashboard/* 使用）                   |
| `AdminNav.tsx`             | 管理员后台左侧导航（仅 /admin/* 使用）：IP 审核 / 内容终审 / 用户管理 / 申诉处理 / 系统配置 / 分类管理 / Agent 配置 |

### 3.2 通用 UI 组件（`components/ui/`）

| 组件                  | 说明                                                         |
| --------------------- | ------------------------------------------------------------ |
| `TagBadge.tsx`        | 低饱和色标签徽章，color prop（blue/green/purple/orange），自动适配暗色 |
| `MasonryGrid.tsx`     | 瀑布流容器（react-masonry-css），breakpoints: 默认 4 / ≤1100 3 / ≤700 2 |
| `ReputationBadge.tsx` | 信誉分数字 + 颜色状态（绿色≥7 / 黄色3-6 / 红色<3）           |
| `SortTabs.tsx`        | 排序 Tab 组件（最热门 / 最新 / 最多点击），可复用于多个列表页 |
| `TimeRangePicker.tsx` | 时间范围下拉（全部 / 本周 / 本月 / 本年）                    |
| `EmptyState.tsx`      | 空状态占位（图标 + 标题 + 说明 + 可选 CTA 按钮）             |
| `LoadingSpinner.tsx`  | 全屏/局部 loading 动画                                       |
| `ConfirmModal.tsx`    | 通用确认弹窗（标题 + 说明 + 确认/取消按钮）                  |
| `InfiniteScroll.tsx`  | 无限滚动触发器（Intersection Observer）                      |

### 3.3 IP 组件（`components/ip/`）

| 组件                 | 说明                                                  |
| -------------------- | ----------------------------------------------------- |
| `IPCard.tsx`         | IP 列表卡片：封面图 + IP 名 + 分类标签 + 内容数量统计 |
| `IPDetail.tsx`       | IP 详情页顶部布局：大封面图（宽屏横幅） + IP 信息区   |
| `IPCategoryTabs.tsx` | 内容类目 Tab 切换，点击切换显示对应类型内容           |

### 3.4 内容组件（`components/content/`）

| 组件                     | 说明                                                         |
| ------------------------ | ------------------------------------------------------------ |
| `ContentCard.tsx`        | 瀑布流卡片：封面图（3:4，object-cover）/ 无封面时 SVG 占位 + 标题（2行截断）+ 作者 + 点赞/评论数 + 最多3个 TagBadge |
| `ContentDetail.tsx`      | 内容详情布局分发器：根据 content_type 渲染对应内容组件       |
| `MarkdownRenderer.tsx`   | MD 内容渲染（react-markdown + 代码高亮 + 表格支持）          |
| `MarkdownEditor.tsx`     | MD 编辑器（@uiw/react-md-editor，支持实时预览）              |
| `FileUploader.tsx`       | OSS 直传上传组件：拖放 + 点击选择 + 进度条 + 多文件支持      |
| `CoverImageUploader.tsx` | 封面图专用上传：正方形预览 + 裁剪提示                        |
| `VersionHistory.tsx`     | 版本历史时间轴（侧抽屉内）：版本号 + 时间 + 修改摘要 + 查看 Diff 入口 |
| `SheetMusicViewer.tsx`   | 乐谱内容分发器，根据 MIME 类型分发到子组件                   |
| `OSMDRenderer.tsx`       | MusicXML 五线谱渲染（opensheetmusicdisplay，动态 import，ssr:false） |
| `MIDIPlayer.tsx`         | MIDI 播放器：播放/暂停 + 进度条 + 时长显示                   |
| `PDFViewer.tsx`          | PDF 嵌入（embed 标签）+ 新窗口打开链接                       |

### 3.5 PR 协同组件（`components/pr/`）

| 组件              | 说明                                                         |
| ----------------- | ------------------------------------------------------------ |
| `PRCard.tsx`      | PR 申请卡片：提交者头像 + 修改摘要 + 状态标签 + 时间         |
| `DiffViewer.tsx`  | 三栏 Diff 渲染：原始版本 / PR 版本 / 对比高亮（diff-match-patch） |
| `MergeEditor.tsx` | 手动合并编辑器：可编辑的最终合并内容输入区                   |

### 3.6 社交组件（`components/social/`）

| 组件                  | 说明                                                         |
| --------------------- | ------------------------------------------------------------ |
| `FollowButton.tsx`    | 关注/取消关注按钮（用户主页 + IP 详情页复用），显示关注状态 + 关注数/粉丝数 |
| `ReactionBar.tsx`     | 操作栏：点赞按钮（含数量）/ 点踩（含数量）/ 收藏 / 分享 / 举报 |
| `NotificationDropdown.tsx` | Header 消息下拉菜单：铃铛图标 + 未读 Badge + 下拉显示最近通知预览 + 「查看全部」链接 |
| `NotificationList.tsx` | 通知列表（按 channel 分 Tab），单条通知含发送者头像 + 内容摘要 + 时间 + 跳转链接 |
| `ConversationList.tsx` | 私信对话列表：对方头像 + 用户名 + 最后消息预览 + 未读标记 |
| `ChatWindow.tsx`      | 私信对话窗口：消息气泡列表 + 输入框 + 发送按钮 |
| `CommentSection.tsx`  | 评论区容器：评论列表 + 评论输入框 + 分页                     |
| `CommentCard.tsx`     | 单条评论：头像 + 用户名 + 内容 + 时间 + 点赞 + 回复 + 举报   |
| `CommentInput.tsx`    | 评论输入框（含 @ 提及，Markdown 支持）                       |
| `DiscussionCard.tsx`  | 讨论帖卡片：帖子标题 / 作者头像+名称 / 回复数 / 最后回复时间（贴吧风格） |
| `ReplyList.tsx`       | 回复列表组件：含楼中楼缩进、分页加载、回复/举报按钮             |
| `UserProfileCard.tsx` | 用户主页信息卡片：头像 / 用户名 / 简介 / 信誉分徽章 / 注册时间 / 判官资质 Badge 列表 |
| `FollowerListModal.tsx` | 关注/粉丝列表弹窗：分页加载，每项含头像 + 用户名 + FollowButton |
| `CreatorSupportPanel.tsx` | 创作者支持面板：打赏码图片上传 + 外部链接输入（最多 3 个），同时支持只读模式（内容详情页 + 用户主页嵌入展示） |

### 3.7 赛博判官组件（`components/judge/`）

| 组件                 | 说明                                                         |
| -------------------- | ------------------------------------------------------------ |
| `ExamQuestion.tsx`   | 考题卡片：题目文本 + 单/多选选项 + 选中状态                  |
| `ReviewCard.tsx`     | 待审内容卡片：内容预览 + 举报原因 + 当前投票比例进度条 + 投票按钮 |
| `JudgeQualBadge.tsx` | 判官资质徽章（按内容类型，可多个）                           |
| `VerdictDetail.tsx`  | 判决详情：投票分布进度条（不违规/违规比例） + 其他判官提交的理由列表（可点赞/点踩，按赞数排序） |

### 3.8 标签系统组件（`components/tags/`）

| 组件               | 说明                                                         |
| ------------------ | ------------------------------------------------------------ |
| `TagInput.tsx`     | 标签自动补全输入（调用 GET /tags/search，下拉候选列表，已选 Badge 显示） |
| `TagGroupCard.tsx` | 标签组卡片（名称 + tags 列表 + 编辑/删除 + 一键应用按钮）    |

### 3.9 Agent 组件（`components/agent/`）

| 组件                       | 说明                                                         |
| -------------------------- | ------------------------------------------------------------ |
| `AgentChatWidget.tsx`      | 全站右下角悬浮聊天球，点击展开对话窗口，SSE 流式渲染，仅 web_agent_enabled=true 且已登录时显示 |
| `UploadAssistPanel.tsx`    | 发布页「AI 自动填写」入口，分析已上传文件，一键填充表单      |
| `ComplianceCheckBadge.tsx` | 发布页合规检测状态徽章（safe 绿色 / warning 黄色 / violation 红色）+ 详情 Popover |
| `UsageGuidePanel.tsx`      | 内容详情页右侧「AI 使用指导」折叠卡片，SSE 流式渲染 Markdown 指导内容 |
| `SearchAgentInput.tsx`     | 搜索页自然语言搜索框，底部提供「切换普通搜索」降级链接       |

### 3.10 素质建设组件（`components/rehab/`）

| 组件                 | 说明                                                         |
| -------------------- | ------------------------------------------------------------ |
| `CourseCard.tsx`      | 课程卡片：违规类型图标 + 课程标题 + 可恢复分值 + 完成状态标签 |
| `CourseContent.tsx`   | 课程教学内容渲染（react-markdown），底部倒计时进度条         |
| `ReputationDetail.tsx`| 信誉分详情面板：当前分值 + 加减分历史列表 + 规则说明         |

### 3.11 管理员组件（`components/admin/`）

| 组件                       | 说明                                                         |
| -------------------------- | ------------------------------------------------------------ |
| `LLMConfigTable.tsx`       | LLM 配置列表表格：配置名 / 提供商 / API 地址 / 模型 / 状态 / 操作按钮组 |
| `LLMConfigModal.tsx`       | LLM 配置新建/编辑 Modal：表单字段 + 测试连接按钮 + 保存       |
| `ActiveConfigCard.tsx`     | 当前激活 LLM 配置摘要卡片（顶部高亮显示）                 |

---

## 四、前端路由结构

```
app/
├── layout.tsx
│     # Root layout：ThemeProvider + NextIntlClientProvider
│     # + AuthContext + AgentChatWidget（全站悬浮）
│
├── (public)/
│   ├── layout.tsx           # Header + Footer，无 auth guard
│   │
│   ├── page.tsx             # / 首页：三区域布局（最近IP + IP浏览区 + 二创内容瀑布流）
│   ├── search/
│   │   └── page.tsx         # /search 搜索页
│   ├── ip/
│   │   └── [ipId]/
│   │       ├── page.tsx     # /ip/:ipId IP 详情页
│   │       ├── [category]/
│   │       │   └── page.tsx # /ip/:ipId/:category 类目内容列表
│   │       └── discussions/
│   │           ├── page.tsx # /ip/:ipId/discussions 讨论区列表
│   │           └── [discussionId]/
│   │               └── page.tsx # /ip/:ipId/discussions/:discussionId 讨论详情
│   ├── content/
│   │   └── [contentId]/
│   │       └── page.tsx     # /content/:contentId 内容详情（二创区）
│   ├── original/
│   │   ├── page.tsx         # /original 原创区首页
│   │   └── [contentId]/
│   │       ├── page.tsx     # /original/:contentId 原创内容详情
│   │       └── fanworks/
│   │           └── page.tsx # /original/:contentId/fanworks 相关二创列表
│   ├── user/
│   │   └── [userId]/
│   │       └── page.tsx     # /user/:userId 用户主页
│   ├── login/
│   │   └── page.tsx         # /login 登录页
│   └── register/
│       └── page.tsx         # /register 注册页
│
├── (protected)/
│   ├── layout.tsx           # auth guard：无 JWT → redirect /login
│   │
│   ├── settings/
│   │   ├── page.tsx         # /settings 账号设置
│   │   └── tag-groups/
│   │       └── page.tsx     # /settings/tag-groups 标签组管理
│   ├── publish/
│   │   └── page.tsx         # /publish 发布内容
│   ├── dashboard/
│   │   ├── layout.tsx       # DashboardNav 左侧导航
│   │   ├── page.tsx         # /dashboard 概览
│   │   ├── contents/
│   │   │   └── page.tsx     # /dashboard/contents 我的内容
│   │   ├── pr-requests/
│   │   │   └── page.tsx     # /dashboard/pr-requests PR 申请管理
│   │   ├── contributors/
│   │   │   └── page.tsx     # /dashboard/contributors 贡献者管理
│   │   └── tag-suggestions/
│   │       └── page.tsx     # /dashboard/tag-suggestions 标签建议审核
│   ├── ip/
│   │   └── [ipId]/
│   │       └── discussions/
│   │           └── new/
│   │               └── page.tsx # /ip/:ipId/discussions/new 发帖（需登录）
│   ├── judge/
│   │   ├── exam/
│   │   │   └── page.tsx     # /judge/exam 赛博判官考核
│   │   └── queue/
│   │       └── page.tsx     # /judge/queue 待审内容队列
│   ├── history/
│   │   └── page.tsx         # /history 浏览历史
│   ├── appeals/
│   │   └── page.tsx         # /appeals 我的申诉
│   ├── messages/
│   │   └── page.tsx         # /messages 消息中心（通知 + 私信）
│   └── rehab/
│       └── page.tsx         # /rehab 素质建设课程
│
└── admin/
    ├── layout.tsx           # admin guard：role≠admin → 403 页面
    ├── ips/
    │   └── page.tsx         # /admin/ips IP 库审核
    ├── contents/
    │   └── page.tsx         # /admin/contents 内容终审
    ├── users/
    │   └── page.tsx         # /admin/users 用户管理
    ├── appeal/
    │   └── page.tsx         # /admin/appeal 申诉处理
    ├── config/
    │   └── page.tsx         # /admin/config 系统配置热更新
    ├── categories/
    │   └── page.tsx         # /admin/categories 分类与标签管理
    └── agent-config/
        └── page.tsx         # /admin/agent-config Agent / LLM 配置管理
```

---

## 五、补充设计约束（供 Gemini 参考）

**颜色体系**（Tailwind token）：

- `canvas.default` / `canvas.subtle`：页面背景（白 / 浅灰），暗色时深灰 / 更深灰
- `border.default`：组件描边（1px）
- `fg.default` / `fg.muted`：正文 / 次要文字
- `accent.emphasis`：主操作按钮（低饱和蓝/紫）
- `tag.blue` / `tag.green` / `tag.purple` / `tag.orange`：标签色（低饱和）

**卡片规范**：1px border，hover border 加深，无 box-shadow（GitHub 扁平风）

**响应式断点**：移动 ≤ 700px（2 列）/ 平板 ≤ 1100px（3 列）/ PC > 1100px（4 列）

**移动端 FacetedSearchSidebar**：屏宽 ≤ 700px 时，分面筛选侧边栏折叠为顶部「筛选」按钮，点击后以 shadcn/ui Sheet 抽屉形式从左侧滑入，宽度 85vw；操作完毕后点击「应用」关闭抽屉并刷新结果列表

**字体**：`-apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial`，无自定义字体

**图标**：全部使用 Lucide React，线条风格，不使用 Emoji 装饰

**加载态**：列表页使用骨架屏（Skeleton），操作按钮使用内联 Spinner，不使用全屏遮罩

**空状态**：使用 EmptyState 组件（图标 + 标题 + 说明），不留空白区域

```
输出格式要求（严格遵守，供 AI 编程 Agent 直接引用）：

1. 文档顶部必须有 "## Global Design Tokens" 章节（颜色/字体/间距/圆角/动效）
2. 每个页面用 "## Page: [路由]" 作为 heading，例如 "## Page: / 首页"
3. 每个独立组件用 "## Component: [组件名]" 作为 heading，名称必须与以下列表完全一致：
   Header / FacetedSearchSidebar / MasonryGrid / ContentCard / TagBadge / 
   IPCard / ContentDetail / MarkdownRenderer / SheetMusicViewer / 
   PRCard / DiffViewer / ReactionBar / CommentSection / DiscussionCard / ReplyList /
   ExamQuestion / ReviewCard / AgentChatWidget / UploadAssistPanel / 
   ComplianceCheckBadge / UsageGuidePanel / SearchAgentInput / 
   VersionHistory / FileUploader / EmptyState / ConfirmModal
4. 每个章节开头必须有 "**Key Constraints**" 子标题，列出 3-5 条最关键的视觉约束（供 Agent 快速检索）
5. 包含以下信息：视觉层级 / 间距规范 / 状态变体（default/hover/active/disabled/loading/empty）/ 响应式规则 / 暗色模式适配
6. 结果后粘贴至 C:\Users\16278\Desktop\file\code\project\OmniCraft\design\ui-spec.md，删除文件顶部的占位提示块即可。
```
