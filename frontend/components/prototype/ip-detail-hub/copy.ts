/**
 * 【原型专用，随时可删】IP 详情页社区枢纽原型 — 文案表。
 *
 * 原型不接 next-intl（避免改动生产 messages 文件），但所有 UI 文案都走
 * 「i18n 风格的 key 化结构」，key 命名空间按交接文档约定（proposal.* / hub.*），
 * 转正式实现时可整表迁移进 frontend/messages/zh.json + en.json。
 *
 * 原型问题（见 ip-hub-prototype.tsx 顶部说明）。
 */

const zh: Record<string, string> = {
  // ── 原型说明条 ──
  "proto.banner":
    "交互原型 · 仅验证定稿设计的手感，全部数据为模拟，不接后端。场景切换：底部浮条或 ←/→ 方向键。",
  "proto.scenario.member": "已关注成员",
  "proto.scenario.guest": "未关注访客",
  "proto.scenario.empty": "空态（全模块无内容）",
  "proto.scenarioLabel": "原型场景",

  // ── 头部 ──
  "hub.categoryLabel": "类目",
  "hub.stat.followers": "关注",
  "hub.stat.discussions": "讨论",
  "hub.stat.works": "作品",
  "hub.follow": "关注",
  "hub.following": "已关注",
  "hub.unfollow": "取消关注",

  // ── 搜索 ──
  "hub.search.placeholder": "在该 IP 内搜索作品、讨论、提案…",
  "hub.search.aria": "IP 内搜索",
  "hub.search.clear": "清空搜索",
  "hub.search.emptyTitle": "未找到与「{q}」相关的内容",
  "hub.search.emptyDesc": "换个关键词试试，或清空搜索返回浏览。",

  // ── 模块 tab ──
  "hub.tab.share": "内容分享",
  "hub.tab.discussions": "讨论区",
  "hub.tab.proposals": "提案投票",

  // ── 内容分享 ──
  "hub.share.filterLabel": "媒体类型筛选",
  "hub.share.sortLabel": "作品排序",
  "hub.share.type.all": "全部",
  "hub.share.type.image": "图片",
  "hub.share.type.article": "文字",
  "hub.share.type.video": "视频",
  "hub.share.type.audio": "音频",
  "hub.share.type.mod": "Mod",
  "hub.share.type.prompt": "Prompt",
  "hub.share.type.sheet_music": "曲谱",
  "hub.share.type.other": "其他",
  "hub.share.views": "{count} 浏览",
  "hub.share.likes": "{count} 赞",
  "hub.share.rating": "{score} 分",
  "hub.share.emptyTitle": "这个 IP 还没有二创作品",
  "hub.share.emptyDesc": "成为第一个分享作品的人，你的作品会出现在这里。",
  "hub.share.emptyAction": "去发布作品",
  "hub.share.detailZone": "二创作品 · zone=fanwork",
  "hub.share.overlayNote": "原型只读演示：真实的作品详情复用 ContentDetailOverlay 既有模式。",

  // ── 讨论区 ──
  "hub.discussion.sortLabel": "讨论排序",
  "hub.discussion.sort.last_reply": "最新回复",
  "hub.discussion.sort.new": "最新发布",
  "hub.discussion.sort.replies": "最多回复",
  "hub.discussion.sort.hot": "热门",
  "hub.discussion.pinned": "置顶",
  "hub.discussion.replyCount": "{count} 回复",
  "hub.discussion.lastReply": "最后回复 {time}",
  "hub.discussion.started": "发布于 {time}",
  "hub.discussion.hotScore": "热度 {score}",
  "hub.discussion.replies": "回复 ({count})",
  "hub.discussion.replyPlaceholder": "写下你的回复…",
  "hub.discussion.replySend": "发送",
  "hub.discussion.replyPosted": "已发布回复（模拟）",
  "hub.discussion.emptyTitle": "还没有讨论",
  "hub.discussion.emptyDesc": "发起第一个讨论，和同好聊聊这个 IP。",
  "hub.discussion.emptyAction": "发起讨论",
  "hub.discussion.overlayNote": "讨论帖详情浮层为新交互：Esc / 浏览器后退 / 点击遮罩均可关闭。",

  // ── 提案投票 ──
  "proposal.filter.open": "进行中",
  "proposal.filter.adopted": "已通过",
  "proposal.filter.rejected": "已否决",
  "proposal.filter.history": "改动历史",
  "proposal.filterLabel": "提案状态筛选",
  "proposal.status.open": "进行中",
  "proposal.status.adopted": "已通过",
  "proposal.status.rejected": "已否决",
  "proposal.field.intro": "简介",
  "proposal.field.cover": "封面",
  "proposal.field.tags": "标签",
  "proposal.startedAt": "{name} · 发起于 {time}",
  "proposal.deadline": "剩 {days} 天",
  "proposal.meta": "门槛 {minVotes} 票 · 通过线 {threshold}%",
  "proposal.deadlineConfig": "门槛 {minVotes} 票 · 通过线 {threshold}% · 截止 {days} 天",
  "proposal.passLine": "通过线 {threshold}%",
  "proposal.votesFor": "{count} 赞成",
  "proposal.votesAgainst": "{count} 反对",
  "proposal.voteFor": "赞成",
  "proposal.voteAgainst": "反对",
  "proposal.myVoteFor": "你已赞成",
  "proposal.myVoteAgainst": "你已反对",
  "proposal.lockedNote": "一人一票，投后不可修改",
  "proposal.result": "{for} 赞成 · {against} 反对 · 赞成率 {rate}%",
  "proposal.closedAt": "{status}于 {time}",
  "proposal.adoptedAt": "生效于 {time}",
  "proposal.gate.title": "关注后即可参与共治投票",
  "proposal.gate.desc": "共治提案由「已关注本 IP」的成员一人一票决定。",
  "proposal.gate.action": "关注并解锁投票",
  "proposal.gate.unlocked": "已关注，现在可以投票了",
  "proposal.openOrderNote": "进行中提案按截止时间优先排列；同 IP 同时只允许一个进行中提案。",
  "proposal.searchNoMatchInStatus": "该状态下没有与「{q}」匹配的提案",
  "proposal.adoptedNote": "已通过的提案，按结案时间排序，可查看投票结果。",
  "proposal.historyNote": "已生效的资料改动，按生效时间倒排的时间线。",
  "proposal.empty.title": "第一个提案由你发起",
  "proposal.empty.desc": "IP 的简介、封面、标签都可以由关注者发起共治提案，投票通过后自动生效。",
  "proposal.empty.action": "发起提案",
  "proposal.empty.adopted": "暂无已通过的提案",
  "proposal.empty.rejected": "暂无被否决的提案",
  "proposal.form.title": "发起共治提案",
  "proposal.form.configNote":
    "提案由关注者投票决定：{config}。提交后将进行 AI 内容审核（本地 fail-open）。",
  "proposal.form.pickFields": "选择要改动的字段（可多选）",
  "proposal.form.introLabel": "新简介",
  "proposal.form.introPlaceholder": "输入提案的新简介…",
  "proposal.form.introSame": "新简介与当前简介相同，请修改后再提交。",
  "proposal.form.introRequired": "请填写新简介内容。",
  "proposal.form.coverLabel": "新封面",
  "proposal.form.coverPick": "换一版（占位）",
  "proposal.form.coverOld": "当前封面",
  "proposal.form.coverNew": "提案封面",
  "proposal.form.tagModeAdd": "新增标签",
  "proposal.form.tagModeRemove": "移除标签",
  "proposal.form.tagLabel": "标签名",
  "proposal.form.tagPlaceholder": "输入标签名…",
  "proposal.form.tagExists": "该标签已存在于 IP 上，不能重复提案加入。",
  "proposal.form.tagMissing": "该标签不存在于 IP 上，无法提案移除。",
  "proposal.form.tagRequired": "请输入要{mode}的标签。",
  "proposal.form.currentTags": "当前标签（点击填入）",
  "proposal.form.submit": "提交提案",
  "proposal.form.submitting": "AI 审核中…",
  "proposal.form.needField": "请至少选择一个要改动的字段。",
  "proposal.form.submitted": "提案已提交，等待关注者投票",
  "proposal.form.cancel": "取消",
  "proposal.create.entry": "发起提案",
  "proposal.create.blocked": "已有进行中的提案，结案后可再发起",
  "proposal.form.diffOld": "当前简介",
  "proposal.form.diffNew": "改为",

  // ── 通用 ──
  "common.expand": "展开",
  "common.collapse": "收起",
  "common.close": "关闭",
  "common.prototypeOnly": "原型：只读演示，不接后端",
  "common.tagAdd": "新增",
  "common.tagRemove": "移除",
};

/** 极简 t()：key 直查 + {var} 插值，仅原型使用。 */
export function t(key: string, vars?: Record<string, string | number>): string {
  const template = zh[key] ?? key;
  if (!vars) return template;
  return template.replace(/\{(\w+)\}/g, (_, name: string) =>
    vars[name] != null ? String(vars[name]) : `{${name}}`,
  );
}
