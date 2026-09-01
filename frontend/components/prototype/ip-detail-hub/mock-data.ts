/**
 * 【原型专用，随时可删】IP 详情页社区枢纽原型 — 模拟数据。
 * 全部为静态假数据：时间用展示字符串 + 排序索引（数字越小越新），不接后端。
 * 数据形态刻意贴近未来实现的真实契约（见交接文档 §1.2 / §1.3）。
 */
import type { CSSProperties } from "react";

export type ShareType =
  | "image"
  | "article"
  | "video"
  | "audio"
  | "mod"
  | "prompt"
  | "sheet_music"
  | "other";

export type TabKey = "share" | "discussions" | "proposals";
export type ShareSort = "new" | "hot" | "views" | "rating";
export type DiscussionSort = "last_reply" | "new" | "replies" | "hot";
export type ProposalStatusFilter = "open" | "adopted" | "rejected" | "history";
export type Scenario = "member" | "guest" | "empty";

export interface DiffSeg {
  kind: "same" | "del" | "ins";
  text: string;
}

export interface Profile {
  id: number;
  name: string;
  categoryLabel: string;
  intro: string;
  coverStyle: CSSProperties;
  tags: string[];
  followers: number;
}

export interface ShareItem {
  id: string;
  type: ShareType;
  title: string;
  author: string;
  excerpt: string;
  coverStyle: CSSProperties;
  aspect: string;
  views: number;
  likes: number;
  rating: number;
  createdDisplay: string;
  createdIdx: number; // 天，越小越新
  hot: number;
}

export interface Reply {
  author: string;
  time: string;
  body: string;
}

export interface Discussion {
  id: string;
  title: string;
  author: string;
  createdDisplay: string;
  createdIdx: number;
  lastReplyDisplay: string;
  lastReplyIdx: number;
  replyCount: number;
  pinned: boolean;
  hot: number;
  body: string[];
  replies: Reply[];
}

export interface Proposal {
  id: string;
  proposer: string;
  startedDisplay: string;
  status: "open" | "adopted" | "rejected";
  introDiff?: DiffSeg[];
  coverDiff?: { oldStyle: CSSProperties; newStyle: CSSProperties };
  tagChange?: { mode: "add" | "remove"; tag: string };
  votesFor: number;
  votesAgainst: number;
  deadlineDays?: number; // open 专用
  closedDisplay?: string; // adopted/rejected 专用
  effectiveDisplay?: string; // adopted 专用（改动历史时间线）
  effectiveIdx: number; // 越小越新
  myVote?: "for" | "against"; // 场景预设的已投状态
}

// ── IP 资料 ──────────────────────────────────────────────

export const PROFILE: Profile = {
  id: 42,
  name: "星轨旅人",
  categoryLabel: "视频",
  intro:
    "《星轨旅人》是一部以「星座旅行」为主题的原创动画短篇集，讲述少年辰与神秘列车「织女号」穿越十二星座站台、收集散落星尘记忆的旅途。全篇以单元剧展开，每抵达一个站台便揭开一段与乘客有关的往事，而辰自己的身世之谜则埋藏在终点的暗线里。目前第一季全 12 话已完结，第二季制作筹备中。",
  coverStyle: {
    backgroundImage: "linear-gradient(135deg, #312e81, #6d28d9 55%, #c026d3)",
  },
  tags: ["原创", "奇幻", "治愈", "列车", "星座"],
  followers: 12847,
};

// ── 内容分享（zone=fanwork）─────────────────────────────

export const SHARES: ShareItem[] = [
  {
    id: "s1",
    type: "image",
    title: "同人插画：辰与织女号在第七站台",
    author: "画画的阿辰",
    excerpt: "画了整整两周的完稿！辰回头的瞬间和车窗里的星光是我最想抓住的两个元素。",
    coverStyle: { backgroundImage: "linear-gradient(160deg, #4338ca, #7c3aed 60%, #db2777)" },
    aspect: "4 / 3",
    views: 32418,
    likes: 4102,
    rating: 9.2,
    createdDisplay: "2 天前",
    createdIdx: 2,
    hot: 9.6,
  },
  {
    id: "s2",
    type: "article",
    title: "同人小说《第七站台的信》全本",
    author: "夜行笔录",
    excerpt: "如果辰在第七站台错过了那班车，故事会怎样？一篇 2.4 万字的 if 线。",
    coverStyle: { backgroundImage: "linear-gradient(160deg, #0f766e, #2563eb)" },
    aspect: "16 / 9",
    views: 12873,
    likes: 1876,
    rating: 9.0,
    createdDisplay: "5 天前",
    createdIdx: 5,
    hot: 8.1,
  },
  {
    id: "s3",
    type: "video",
    title: "「织女号」列车进站混剪 AMV",
    author: "剪轨师小柯",
    excerpt: "用第一季全部进站镜头剪的 90 秒混剪，卡点在 OP 变奏上。",
    coverStyle: { backgroundImage: "linear-gradient(160deg, #b91c1c, #f59e0b)" },
    aspect: "16 / 9",
    views: 45210,
    likes: 6320,
    rating: 9.5,
    createdDisplay: "8 天前",
    createdIdx: 8,
    hot: 9.9,
  },
  {
    id: "s4",
    type: "audio",
    title: "「星尘记忆」角色曲翻唱（女声版）",
    author: "雾中车站",
    excerpt: "原曲降半调的温柔女声翻唱，副歌加了和声。",
    coverStyle: { backgroundImage: "linear-gradient(160deg, #be185d, #7c3aed)" },
    aspect: "1 / 1",
    views: 8412,
    likes: 1204,
    rating: 8.6,
    createdDisplay: "3 天前",
    createdIdx: 3,
    hot: 7.4,
  },
  {
    id: "s5",
    type: "mod",
    title: "列车车厢室内场景 Mod（含第七站台）",
    author: "工坊老周",
    excerpt: "按设定集 1:1 复刻的车厢内构，含可互动的星尘灯。",
    coverStyle: { backgroundImage: "linear-gradient(160deg, #374151, #0f766e)" },
    aspect: "16 / 10",
    views: 6108,
    likes: 942,
    rating: 8.9,
    createdDisplay: "12 天前",
    createdIdx: 12,
    hot: 6.8,
  },
  {
    id: "s6",
    type: "prompt",
    title: "星空水彩风格 Prompt 包（12 站台配色）",
    author: "prompt诗人",
    excerpt: "按十二站台各自的代表色整理的 36 条出图 Prompt，附负面词。",
    coverStyle: { backgroundImage: "linear-gradient(160deg, #7c2d12, #b45309)" },
    aspect: "3 / 4",
    views: 5327,
    likes: 866,
    rating: 8.2,
    createdDisplay: "6 天前",
    createdIdx: 6,
    hot: 6.2,
  },
  {
    id: "s7",
    type: "sheet_music",
    title: "《夜行列车》钢琴谱（ED 完整版）",
    author: "琴键站台",
    excerpt: "听写还原的 ED 完整钢琴谱，附指法建议。",
    coverStyle: { backgroundImage: "linear-gradient(160deg, #1e3a8a, #0891b2)" },
    aspect: "4 / 3",
    views: 3189,
    likes: 521,
    rating: 8.8,
    createdDisplay: "15 天前",
    createdIdx: 15,
    hot: 5.9,
  },
  {
    id: "s8",
    type: "other",
    title: "织女号粘土人手办制作全程记录",
    author: "手办工坊·栗",
    excerpt: "从原型雕刻到上色的 47 天全记录，最后有成品展示。",
    coverStyle: { backgroundImage: "linear-gradient(160deg, #a16207, #ca8a04)" },
    aspect: "1 / 1",
    views: 9902,
    likes: 2311,
    rating: 9.3,
    createdDisplay: "9 天前",
    createdIdx: 9,
    hot: 8.7,
  },
  {
    id: "s9",
    type: "image",
    title: "十二星座站台全景设定图（考据向）",
    author: "画画的阿辰",
    excerpt: "按 OP 里一闪而过的镜头拼合推演的全景图，标注了每站代表色。",
    coverStyle: { backgroundImage: "linear-gradient(160deg, #0e7490, #4f46e5)" },
    aspect: "16 / 9",
    views: 21564,
    likes: 3478,
    rating: 9.1,
    createdDisplay: "1 天前",
    createdIdx: 1,
    hot: 9.2,
  },
  {
    id: "s10",
    type: "article",
    title: "角色解读：为什么辰一直没有全名",
    author: "夜行笔录",
    excerpt: "从剧本留白和台词节拍分析「辰」这个名字的叙事功能。",
    coverStyle: { backgroundImage: "linear-gradient(160deg, #4d7c0f, #0f766e)" },
    aspect: "16 / 10",
    views: 7431,
    likes: 1102,
    rating: 8.4,
    createdDisplay: "11 天前",
    createdIdx: 11,
    hot: 7.0,
  },
];

// ── 讨论区 ──────────────────────────────────────────────

export const DISCUSSIONS: Discussion[] = [
  {
    id: "d1",
    title: "【公告】第二季设定收集帖：欢迎补充你的星台设定",
    author: "星轨运营组",
    createdDisplay: "30 天前",
    createdIdx: 30,
    lastReplyDisplay: "18 分钟前",
    lastReplyIdx: 0.2,
    replyCount: 47,
    pinned: true,
    hot: 6.9,
    body: [
      "第二季进入前期设定阶段，官方开放「隐藏站台」的共创征集。",
      "请在本帖内补充你心目中的第 13 站台：站名、代表色、站台故事。优质设定将被收录进设定集并署名。",
    ],
    replies: [
      { author: "夜行笔录", time: "2 天前", body: "提一个：「参宿四站」，代表色暗红，站台故事与「熄灭的恒星」有关。" },
      { author: "画画的阿辰", time: "1 天前", body: "附了一张草图在内容区，站台的穹顶应该是破碎的星图。" },
      { author: "雾中车站", time: "3 小时前", body: "补充音乐向设定：这一站的 BGM 应该用减和弦。" },
    ],
  },
  {
    id: "d2",
    title: "有没有人注意到 OP 里 12 个站台的顺序暗示？",
    author: "考据怪蜀黍",
    createdDisplay: "20 天前",
    createdIdx: 20,
    lastReplyDisplay: "5 小时前",
    lastReplyIdx: 0.3,
    replyCount: 31,
    pinned: false,
    hot: 9.1,
    body: [
      "逐帧看了 OP，12 个站台出现的顺序并不是剧情顺序，而是按辰失落的记忆碎片排列的。",
      "第 7 个出现的站台恰好是第七话的站台，画面里辰没有上车——这应该不是巧合。",
    ],
    replies: [
      { author: "剪轨师小柯", time: "3 天前", body: "补了一个卡点视频逐帧对照，顺序和你排的一致。" },
      { author: "琴键站台", time: "1 天前", body: "配乐也在暗示：第 7 站的变奏少了主旋律。" },
    ],
  },
  {
    id: "d3",
    title: "第一集结尾辰为什么没上车？我的解读",
    author: "夜行笔录",
    createdDisplay: "25 天前",
    createdIdx: 25,
    lastReplyDisplay: "1 天前",
    lastReplyIdx: 1,
    replyCount: 23,
    pinned: false,
    hot: 8.3,
    body: [
      "重看第一集，辰在站台犹豫的三秒里，广播念的不是站名而是他的名字。",
      "我认为「不上车」是辰第一次对织女号行使选择权，也是他后来能收集星尘的原因。",
    ],
    replies: [
      { author: "考据怪蜀黍", time: "6 天前", body: "同意，而且那三秒的背景里没有其他乘客，是编织的记忆空间。" },
      { author: "手办工坊·栗", time: "2 天前", body: "所以第八话他才说「这次换我决定停哪站」。" },
    ],
  },
  {
    id: "d4",
    title: "画了织女号的内构设定，求考据大佬指正",
    author: "画画的阿辰",
    createdDisplay: "10 天前",
    createdIdx: 10,
    lastReplyDisplay: "3 小时前",
    lastReplyIdx: 0.15,
    replyCount: 12,
    pinned: false,
    hot: 7.2,
    body: [
      "按第一话车窗倒影推了车厢剖面图，锅炉房的星尘导管走向不太确定。",
      "求设定集党指正。",
    ],
    replies: [{ author: "工坊老周", time: "3 小时前", body: "导管应该汇入车尾而不是锅炉房，设定集第 31 页有剖面。" }],
  },
  {
    id: "d5",
    title: "第八话的背景音乐叫什么？求歌名",
    author: "新人报到",
    createdDisplay: "4 天前",
    createdIdx: 4,
    lastReplyDisplay: "40 分钟前",
    lastReplyIdx: 0.1,
    replyCount: 5,
    pinned: false,
    hot: 5.1,
    body: ["第八话 14 分 20 秒进副歌的那段钢琴，翻遍 OST 歌单没找到，求歌名。"],
    replies: [{ author: "琴键站台", time: "40 分钟前", body: "是未收录的插曲《未命名的站台》，官方说第二季会正式发布。" }],
  },
  {
    id: "d6",
    title: "分享我整理的全角色时间线考据（长文预警）",
    author: "考据怪蜀黍",
    createdDisplay: "18 天前",
    createdIdx: 18,
    lastReplyDisplay: "2 天前",
    lastReplyIdx: 2,
    replyCount: 19,
    pinned: false,
    hot: 7.8,
    body: [
      "把 12 话里所有日期、广播报站、车票存根捋成了一条时间线。",
      "结论：辰的旅程在现实里只过了三夜，织女号是在梦境层叠里行驶的。",
    ],
    replies: [{ author: "夜行笔录", time: "2 天前", body: "「三夜」结构和但丁神曲呼应，这考据我服。" }],
  },
  {
    id: "d7",
    title: "新人报道！一周目通关来打卡",
    author: "雾中车站",
    createdDisplay: "2 天前",
    createdIdx: 2,
    lastReplyDisplay: "6 小时前",
    lastReplyIdx: 0.4,
    replyCount: 8,
    pinned: false,
    hot: 4.6,
    body: ["连着两个通宵看完一周目，现在满脑子都是进站的汽笛声。求二刷避雷指南（不想被刀第二次）。"],
    replies: [{ author: "新人报到", time: "6 小时前", body: "避雷指南：第八话准备纸巾，第九话别在饭点看。" }],
  },
];

// ── 共治提案 ─────────────────────────────────────────────

export const INTRO_PROPOSAL_DIFF: DiffSeg[] = [
  { kind: "same", text: "《星轨旅人》是一部以「星座旅行」为主题的原创动画短篇集，讲述少年辰与神秘列车「织女号」穿越十二星座站台、收集散落星尘记忆的旅途。全篇以单元剧展开，每抵达一个站台便揭开一段与乘客有关的往事，而辰自己的身世之谜则埋藏在终点的暗线里。" },
  { kind: "del", text: "目前第一季全 12 话已完结，第二季制作筹备中。" },
  { kind: "ins", text: "第一季全 12 话已于 2026 年 7 月完结，动画 MIT 站综合评分 9.0。第二季「北落师门篇」进入正式制作阶段：新章以「第十三站台」的都市传说为引，原班声优回归，追加角色「叁」由新人声优林晚配音；预计 2027 年冬季开播，先行 PV 将在本页面首发。第二季播出期间，本页将继续作为同好聚集地汇总官方情报、讨论与二创，欢迎关注。" },
];

export const PROPOSALS: Proposal[] = [
  {
    id: "p1",
    proposer: "织女号驾驶员",
    startedDisplay: "4 天前",
    status: "open",
    introDiff: INTRO_PROPOSAL_DIFF,
    coverDiff: {
      oldStyle: { backgroundImage: "linear-gradient(135deg, #312e81, #6d28d9 55%, #c026d3)" },
      newStyle: { backgroundImage: "linear-gradient(135deg, #0f766e, #2563eb 60%, #4f46e5)" },
    },
    tagChange: { mode: "add", tag: "科幻" },
    votesFor: 6,
    votesAgainst: 2,
    deadlineDays: 3,
    effectiveIdx: 0,
  },
  {
    id: "p2",
    proposer: "考据怪蜀黍",
    startedDisplay: "20 天前",
    status: "adopted",
    tagChange: { mode: "add", tag: "东方奇幻" },
    votesFor: 14,
    votesAgainst: 3,
    closedDisplay: "3 天前",
    effectiveDisplay: "3 天前",
    effectiveIdx: 1,
  },
  {
    id: "p3",
    proposer: "星轨运营组",
    startedDisplay: "26 天前",
    status: "adopted",
    coverDiff: {
      oldStyle: { backgroundImage: "linear-gradient(135deg, #1e1b4b, #4c1d95 60%, #9d174d)" },
      newStyle: { backgroundImage: "linear-gradient(135deg, #312e81, #6d28d9 55%, #c026d3)" },
    },
    votesFor: 12,
    votesAgainst: 2,
    closedDisplay: "2026-08-24",
    effectiveDisplay: "2026-08-24",
    effectiveIdx: 5,
  },
  {
    id: "p4",
    proposer: "雾中车站",
    startedDisplay: "33 天前",
    status: "adopted",
    tagChange: { mode: "add", tag: "治愈" },
    votesFor: 11,
    votesAgainst: 2,
    closedDisplay: "2026-08-17",
    effectiveDisplay: "2026-08-17",
    effectiveIdx: 8,
  },
  {
    id: "p5",
    proposer: "夜行笔录",
    startedDisplay: "40 天前",
    status: "adopted",
    introDiff: [
      { kind: "same", text: "《星轨旅人》是一部以「星座旅行」为主题的原创动画短篇集，" },
      { kind: "del", text: "讲述少年辰与神秘列车「织女号」的旅途故事。" },
      { kind: "ins", text: "讲述少年辰与神秘列车「织女号」穿越十二星座站台、收集散落星尘记忆的旅途。" },
      { kind: "same", text: "全篇以单元剧展开。" },
    ],
    votesFor: 11,
    votesAgainst: 2,
    closedDisplay: "2026-08-10",
    effectiveDisplay: "2026-08-10",
    effectiveIdx: 12,
  },
  {
    id: "p6",
    proposer: "路人甲",
    startedDisplay: "9 天前",
    status: "rejected",
    introDiff: [
      { kind: "same", text: "《星轨旅人》是一部" },
      { kind: "del", text: "以「星座旅行」为主题" },
      { kind: "ins", text: "讲述主角团拯救世界" },
      { kind: "same", text: "的原创动画短篇集。" },
    ],
    votesFor: 4,
    votesAgainst: 5,
    closedDisplay: "2 天前",
    effectiveIdx: 2,
  },
];

/** 投票配置镜像：正式实现时从 backend config `ip_proposal.*` 读取。 */
export const PROPOSAL_CONFIG = { minVotes: 10, passThreshold: 0.6, deadlineDays: 7 };

// ── 场景构建与派生工具 ────────────────────────────────────

export function buildScenarioData(scenario: Scenario): {
  profile: Profile;
  shares: ShareItem[];
  discussions: Discussion[];
  proposals: Proposal[];
  following: boolean;
} {
  if (scenario === "empty") {
    return {
      profile: { ...PROFILE, followers: 0, tags: [] },
      shares: [],
      discussions: [],
      proposals: [],
      following: true,
    };
  }
  return { profile: PROFILE, shares: SHARES, discussions: DISCUSSIONS, proposals: PROPOSALS, following: scenario !== "guest" };
}

/** 轻量搜索：标题 / 正文 / 标签 / 作者 / 提案人 contains 匹配。 */
export function searchAll(
  data: { shares: ShareItem[]; discussions: Discussion[]; proposals: Proposal[] },
  q: string,
): { shares: ShareItem[]; discussions: Discussion[]; proposals: Proposal[]; total: number } {
  const needle = q.trim().toLowerCase();
  if (!needle) return { shares: [], discussions: [], proposals: [], total: 0 };
  const hit = (text: string) => text.toLowerCase().includes(needle);
  const shares = data.shares.filter((s) => hit(s.title) || hit(s.excerpt) || hit(s.author));
  const discussions = data.discussions.filter(
    (d) => hit(d.title) || hit(d.body.join("")) || hit(d.author) || d.replies.some((r) => hit(r.body)),
  );
  const proposals = data.proposals.filter(
    (p) =>
      hit(p.proposer) ||
      (p.tagChange && hit(p.tagChange.tag)) ||
      (p.introDiff && hit(p.introDiff.map((seg) => seg.text).join(""))),
  );
  return { shares, discussions, proposals, total: shares.length + discussions.length + proposals.length };
}

export function sortShares(shares: ShareItem[], sort: ShareSort): ShareItem[] {
  const list = [...shares];
  switch (sort) {
    case "new":
      return list.sort((a, b) => a.createdIdx - b.createdIdx);
    case "hot":
      return list.sort((a, b) => b.hot - a.hot);
    case "views":
      return list.sort((a, b) => b.views - a.views);
    case "rating":
      return list.sort((a, b) => b.rating - a.rating);
  }
}

export function sortDiscussions(discussions: Discussion[], sort: DiscussionSort): Discussion[] {
  const list = [...discussions];
  if (sort === "replies") return list.sort((a, b) => b.replyCount - a.replyCount);
  if (sort === "new") return list.sort((a, b) => a.createdIdx - b.createdIdx);
  if (sort === "last_reply") return list.sort((a, b) => a.lastReplyIdx - b.lastReplyIdx);
  // 热门：log(reply_count+1) / 龄期衰减 的 mock 值；置顶帖仍置顶优先
  return list.sort((a, b) => b.hot - a.hot);
}

/** 置顶恒在最前（is_pinned 优先保留）。 */
export function withPinnedFirst(list: Discussion[]): Discussion[] {
  return [...list].sort((a, b) => Number(b.pinned) - Number(a.pinned));
}
