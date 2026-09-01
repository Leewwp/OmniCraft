"use client";
/**
 * 【原型专用，随时可删】IP 详情页「贴吧式社区枢纽」交互原型。
 *
 * 原型问题：已定稿的设计（docs/working/2026-09-01-ip-detail-refactor-handoff.md）
 * 在真实交互下手感是否成立 —— 单页三模块 tab（内容分享/讨论区/提案投票）+
 * 搜索 tab 内过滤 + 共治提案投票流。设计已定稿，故为单一定稿原型（非多方案变体）。
 *
 * 形态：静态 mock 数据、不接后端；场景经 ?scenario=member|guest|empty 切换
 * （浮底条循环或 ←/→ 键）；URL query 同步 tab/type/sort/status/q，
 * 搜索在回车或输入框失焦时提交、主 tab 结构不变（列表与 tab 计数随结果收缩），
 * 浏览器后退可回退 tab 与搜索态；两个详情浮层支持 Esc / 后退 / 遮罩关闭。
 */
import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import {
  Check,
  ChevronLeft,
  ChevronRight,
  FlaskConical,
  Lightbulb,
  MessageSquare,
  Plus,
  Search,
  Sparkles,
  X,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SortSelect } from "@/components/ui/SortSelect";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { TagBadge } from "@/components/ui/TagBadge";
import { useToast } from "@/components/ui/Toast";
import { t } from "./copy";
import { FilterPill, StatItem, SearchEmpty } from "./bits";
import { ShareGrid, ShareOverlay, SHARE_TYPES, typeLabel } from "./share-tab";
import { DiscussionList, DiscussionOverlay } from "./discussion-tab";
import { ProposalList } from "./proposal-tab";
import { ProposalForm } from "./proposal-form";
import {
  buildScenarioData,
  searchAll,
  sortDiscussions,
  sortShares,
  withPinnedFirst,
  type Discussion,
  type Proposal,
  type ProposalStatusFilter,
  type Reply,
  type Scenario,
  type ShareItem,
  type ShareSort,
  type ShareType,
  type TabKey,
} from "./mock-data";

// ── query 解析 ──

function parseTab(v: string | null): TabKey {
  return v === "discussions" || v === "proposals" ? v : "share";
}

function parseScenario(v: string | null): Scenario {
  return v === "guest" || v === "empty" ? v : "member";
}

function parseEnum<T extends string>(v: string | null, allowed: readonly T[], fallback: T): T {
  return v && (allowed as readonly string[]).includes(v) ? (v as T) : fallback;
}

const SHARE_SORTS = ["new", "hot", "views", "rating"] as const;
const DISCUSSION_SORTS = ["last_reply", "new", "replies", "hot"] as const;
const PROPOSAL_FILTERS = ["open", "adopted", "rejected", "history"] as const;
const SCENARIOS: Scenario[] = ["member", "guest", "empty"];

const TAG_COLOR_CYCLE = ["blue", "green", "purple", "orange", "rose", "sky"] as const;

type Overlay =
  | { kind: "share"; item: ShareItem }
  | { kind: "discussion"; id: string }
  | { kind: "proposal-form" }
  | null;

export function IpHubPrototype() {
  return (
    <Suspense fallback={null}>
      <IpHubInner />
    </Suspense>
  );
}

function IpHubInner() {
  const router = useRouter();
  const pathname = usePathname();
  const sp = useSearchParams();
  const { toast } = useToast();

  // URL 状态
  const scenario = parseScenario(sp.get("scenario"));
  const tab = parseTab(sp.get("tab"));
  const shareType = parseEnum<ShareType | "all">(sp.get("type"), [...SHARE_TYPES, "all"], "all");
  const shareSort = parseEnum<ShareSort>(sp.get("sort"), SHARE_SORTS, "new");
  const discussionSort = parseEnum(sp.get("sort"), DISCUSSION_SORTS, "last_reply");
  const status = parseEnum<ProposalStatusFilter>(sp.get("status"), PROPOSAL_FILTERS, "open");
  const q = sp.get("q") ?? "";

  // 本地状态
  const [qInput, setQInput] = useState(q);
  const [overlay, setOverlay] = useState<Overlay>(null);
  const [votes, setVotes] = useState<Record<string, "for" | "against">>({});
  const [following, setFollowing] = useState(true);
  const [gateFor, setGateFor] = useState<string | null>(null);
  const [extraProposals, setExtraProposals] = useState<Proposal[]>([]);
  const [extraReplies, setExtraReplies] = useState<Record<string, Reply[]>>({});
  const pushedRef = useRef(false);

  const base = useMemo(() => buildScenarioData(scenario), [scenario]);
  const proposals = useMemo(() => [...base.proposals, ...extraProposals], [base.proposals, extraProposals]);
  const hasOpenProposal = proposals.some((p) => p.status === "open");

  // 投票计数应用（mock：本地把票加上）
  const proposalsWithVotes = useMemo(
    () =>
      proposals.map((p) => {
        const v = votes[p.id];
        if (!v) return p;
        return {
          ...p,
          votesFor: p.votesFor + (v === "for" ? 1 : 0),
          votesAgainst: p.votesAgainst + (v === "against" ? 1 : 0),
        };
      }),
    [proposals, votes],
  );

  const searchResults = useMemo(
    () => (q ? searchAll({ shares: base.shares, discussions: base.discussions, proposals: proposalsWithVotes }, q) : null),
    [q, base.shares, base.discussions, proposalsWithVotes],
  );

  // ── URL 同步（先例：IPBrowseClient router.replace + scroll:false）──

  const setParams = useCallback(
    (mut: Record<string, string | null>, mode: "push" | "replace" = "replace") => {
      const params = new URLSearchParams(window.location.search);
      for (const [key, value] of Object.entries(mut)) {
        if (value === null || value === "") params.delete(key);
        else params.set(key, value);
      }
      const qs = params.toString();
      router[mode](qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [router, pathname],
  );

  const switchTab = useCallback(
    (next: TabKey) => {
      // tab 切换用 push（浏览器后退可回）；下级筛选随模块重置
      setParams({ tab: next === "share" ? null : next, type: null, sort: null, status: null }, "push");
    },
    [setParams],
  );

  // ── 场景切换：重置本地状态 ──

  useEffect(() => {
    setQInput(q);
  }, [q]);

  useEffect(() => {
    setVotes({});
    setFollowing(base.following);
    setGateFor(null);
    setExtraProposals([]);
    setExtraReplies({});
    closeOverlayInternal();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [scenario, base.following]);

  // ── 浮层：pushState 后退关闭 + Esc 关闭 ──

  function closeOverlayInternal() {
    setOverlay(null);
    if (pushedRef.current) {
      pushedRef.current = false;
      window.history.back();
    }
  }
  const closeOverlay = useCallback(closeOverlayInternal, []);

  useEffect(() => {
    if (!overlay || overlay.kind === "proposal-form") return;
    window.history.pushState({ protoOverlay: true }, "");
    pushedRef.current = true;
    const onPop = () => {
      pushedRef.current = false;
      setOverlay(null);
    };
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, [overlay]);

  useEffect(() => {
    if (!overlay) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") closeOverlay();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [overlay, closeOverlay]);

  // ── 场景浮条：←/→ 键循环 ──

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
      const el = document.activeElement;
      if (el && (el.tagName === "INPUT" || el.tagName === "TEXTAREA" || (el as HTMLElement).isContentEditable)) return;
      if (overlay) return;
      const idx = SCENARIOS.indexOf(scenario);
      const next = e.key === "ArrowRight" ? SCENARIOS[(idx + 1) % SCENARIOS.length] : SCENARIOS[(idx + SCENARIOS.length - 1) % SCENARIOS.length];
      setParams({ scenario: next }, "replace");
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [scenario, overlay, setParams]);

  // ── handlers ──

  // 搜索提交：回车或输入框失焦触发；主体 tab 结构不变，结果在各 tab 内过滤
  function commitSearch() {
    const next = qInput.trim();
    if (next === q) return;
    setParams({ q: next || null }, "push");
  }

  function clearSearch() {
    setParams({ q: null }, "push");
  }

  function handleSearchSubmit(e: React.FormEvent) {
    e.preventDefault();
    commitSearch();
  }

  function handleFollow(fromGate: boolean) {
    setFollowing(true);
    if (fromGate) {
      setGateFor(null);
      toast("success", t("proposal.gate.unlocked"));
    }
  }

  function handleVote(id: string, vote: "for" | "against") {
    setVotes((v) => ({ ...v, [id]: vote }));
  }

  function handleReply(threadId: string, body: string) {
    setExtraReplies((r) => ({ ...r, [threadId]: [...(r[threadId] ?? []), { author: "我（当前用户）", time: "刚刚", body }] }));
  }

  function handleProposalSubmit(proposal: Proposal) {
    setExtraProposals((list) => [...list, proposal]);
    setOverlay(null);
    setParams({ status: "open", tab: "proposals" }, "replace");
    toast("success", t("proposal.form.submitted"));
  }

  // ── 派生列表（搜索在 tab 内过滤：主结构与 tab 不变，列表与计数收缩）──

  const searching = q !== "";
  const matched = searchResults ?? {
    shares: base.shares,
    discussions: base.discussions,
    proposals: proposalsWithVotes,
  };
  const typeCounts = useMemo(() => {
    const c: Partial<Record<ShareType, number>> = {};
    for (const s of matched.shares) c[s.type] = (c[s.type] ?? 0) + 1;
    return c;
  }, [matched.shares]);
  const visibleShares = sortShares(
    shareType === "all" ? matched.shares : matched.shares.filter((s) => s.type === shareType),
    shareSort,
  );
  const visibleDiscussions = withPinnedFirst(sortDiscussions(matched.discussions, discussionSort));
  const activeThread: Discussion | undefined =
    overlay?.kind === "discussion" ? base.discussions.find((d) => d.id === overlay.id) : undefined;

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-4 px-4 py-6 md:px-6">
      {/* 原型说明条（非设计的一部分） */}
      <div className="flex items-center gap-2 rounded-lg border border-dashed border-border bg-canvas-subtle px-3 py-2 text-xs text-muted-foreground">
        <FlaskConical className="size-3.5 shrink-0 text-accent-emphasis" aria-hidden="true" />
        {t("proto.banner")}
      </div>

      {/* ── 头部：IP 身份区 ── */}
      <section className="rounded-lg border border-border bg-canvas-default p-5 shadow-[var(--elevation-1)]">
        <div className="flex flex-col gap-4 md:flex-row">
          <div className="h-36 w-full shrink-0 overflow-hidden rounded-lg border border-border md:w-52" style={base.profile.coverStyle} aria-hidden="true" />
          <div className="flex min-w-0 flex-1 flex-col gap-2">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight">{base.profile.name}</h1>
              <span className="inline-flex h-5 items-center rounded-full bg-canvas-subtle px-2 text-xs text-muted-foreground">
                {t("hub.categoryLabel")} · {base.profile.categoryLabel}
              </span>
            </div>
            <p className="text-sm leading-relaxed text-foreground/90">{base.profile.intro}</p>
            {base.profile.tags.length > 0 && (
              <div className="flex flex-wrap gap-1.5" aria-label="标签">
                {base.profile.tags.map((tag, i) => (
                  <TagBadge key={tag} color={TAG_COLOR_CYCLE[i % TAG_COLOR_CYCLE.length]}>
                    {tag}
                  </TagBadge>
                ))}
              </div>
            )}
            <div className="mt-1 flex flex-wrap items-center gap-3">
              {/* 沿用 FollowButton 契约（原型为本地 mock，不接 API）；固定宽度容纳「取消关注」，避免 hover 换文案时按钮宽度抖动 */}
              <Button
                variant={following ? "outline" : "default"}
                onClick={() => (following ? setFollowing(false) : handleFollow(false))}
                className={cn(
                  "group min-w-[104px] justify-center gap-1 px-3",
                  following && "hover:border-destructive! hover:text-destructive!",
                )}
              >
                {following ? (
                  <>
                    <Check className="size-3.5" aria-hidden="true" />
                    <span className="group-hover:hidden group-focus-visible:hidden">{t("hub.following")}</span>
                    <span className="hidden group-hover:inline group-focus-visible:inline">{t("hub.unfollow")}</span>
                  </>
                ) : (
                  <>
                    <Plus className="size-3.5" aria-hidden="true" />
                    {t("hub.follow")}
                  </>
                )}
              </Button>
              <div className="flex items-center gap-0.5">
                <StatItem value={base.profile.followers} label={t("hub.stat.followers")} />
                <StatItem value={base.discussions.length} label={t("hub.stat.discussions")} onClick={() => switchTab("discussions")} />
                <StatItem value={base.shares.length} label={t("hub.stat.works")} onClick={() => switchTab("share")} />
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ── IP 内搜索（常驻模块行上方）── */}
      <form role="search" onSubmit={handleSearchSubmit} className="relative max-w-[560px]">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
        <Input
          value={qInput}
          onChange={(e) => setQInput(e.target.value)}
          onBlur={commitSearch}
          placeholder={t("hub.search.placeholder")}
          aria-label={t("hub.search.aria")}
          className="min-h-9 w-full rounded-full border border-border bg-muted pl-9 pr-9 text-sm placeholder:text-muted-foreground/60 focus:bg-background"
        />
        {q && (
          <button
            type="button"
            onClick={clearSearch}
            aria-label={t("hub.search.clear")}
            className="absolute right-2 top-1/2 -translate-y-1/2 rounded-full p-1 text-muted-foreground transition-colors duration-150 hover:bg-canvas-subtle hover:text-foreground"
          >
            <X className="size-3.5" aria-hidden="true" />
          </button>
        )}
      </form>

      {searching && (
        <p className="text-xs text-muted-foreground">
          「{q}」的搜索结果已应用到当前 tab，各模块计数随结果收缩；清空搜索即可还原。
        </p>
      )}

      {/* ── 三模块 tab ── */}
      <Tabs value={tab} onValueChange={(value) => switchTab(value as TabKey)}>
        <TabsList aria-label="模块切换">
          <TabsTrigger value="share">
            <Sparkles className="size-3.5" aria-hidden="true" />
            {t("hub.tab.share")}
            <span className="text-xs tabular-nums text-muted-foreground">{matched.shares.length}</span>
          </TabsTrigger>
          <TabsTrigger value="discussions">
            <MessageSquare className="size-3.5" aria-hidden="true" />
            {t("hub.tab.discussions")}
            <span className="text-xs tabular-nums text-muted-foreground">{matched.discussions.length}</span>
          </TabsTrigger>
          <TabsTrigger value="proposals">
            <Lightbulb className="size-3.5" aria-hidden="true" />
            {t("hub.tab.proposals")}
            <span className="text-xs tabular-nums text-muted-foreground">{matched.proposals.length}</span>
          </TabsTrigger>
        </TabsList>
      </Tabs>

          {/* ── 模块主体 + 下级筛选 ── */}
          {tab === "share" && (
            <div className="flex flex-col gap-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div role="group" aria-label={t("hub.share.filterLabel")} className="flex min-w-0 flex-1 gap-1 overflow-x-auto pb-1">
                  <FilterPill active={shareType === "all"} onClick={() => setParams({ type: null })}>
                    {t("hub.share.type.all")}
                  </FilterPill>
                  {SHARE_TYPES.map((ty) => (
                    <FilterPill key={ty} active={shareType === ty} onClick={() => setParams({ type: ty })}>
                      {typeLabel(ty)}
                      <span className="ml-0.5 tabular-nums opacity-70">{typeCounts[ty] ?? 0}</span>
                    </FilterPill>
                  ))}
                </div>
                <SortSelect
                  ariaLabel={t("hub.share.sortLabel")}
                  value={shareSort}
                  options={[
                    { value: "new", label: "最新" },
                    { value: "hot", label: "最热" },
                    { value: "views", label: "最多浏览" },
                    { value: "rating", label: "最高评分" },
                  ]}
                  onChange={(sort) => setParams({ sort: sort === "new" ? null : sort })}
                  className="min-h-9 shrink-0"
                />
              </div>
              {searching && visibleShares.length === 0 ? (
                <SearchEmpty q={q} onClear={clearSearch} />
              ) : (
                <ShareGrid items={visibleShares} onOpen={(item) => setOverlay({ kind: "share", item })} />
              )}
            </div>
          )}

          {tab === "discussions" && (
            <div className="flex flex-col gap-3">
              {/* 讨论区无筛选，仅排序（含热门） */}
              <div className="flex items-center justify-end">
                <SortSelect
                  ariaLabel={t("hub.discussion.sortLabel")}
                  value={discussionSort}
                  options={[
                    { value: "last_reply", label: "最新回复" },
                    { value: "new", label: "最新发布" },
                    { value: "replies", label: "最多回复" },
                    { value: "hot", label: "热门" },
                  ]}
                  onChange={(sort) => setParams({ sort: sort === "last_reply" ? null : sort })}
                  className="min-h-9"
                />
              </div>
              {searching && visibleDiscussions.length === 0 ? (
                <SearchEmpty q={q} onClear={clearSearch} />
              ) : (
                <DiscussionList
                  items={visibleDiscussions}
                  showHotScore={discussionSort === "hot"}
                  onOpen={(d) => setOverlay({ kind: "discussion", id: d.id })}
                />
              )}
            </div>
          )}

          {tab === "proposals" && (
            <div className="flex flex-col gap-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div role="group" aria-label={t("proposal.filterLabel")} className="flex min-w-0 flex-1 gap-1 overflow-x-auto pb-1">
                  {PROPOSAL_FILTERS.map((f) => (
                    <FilterPill key={f} active={status === f} onClick={() => setParams({ status: f === "open" ? null : f })}>
                      {t(`proposal.filter.${f}`)}
                      {f !== "history" && (
                        <span className="ml-0.5 tabular-nums opacity-70">
                          {matched.proposals.filter((p) => p.status === f).length}
                        </span>
                      )}
                    </FilterPill>
                  ))}
                </div>
                {hasOpenProposal ? (
                  <Button
                    size="sm"
                    variant="outline"
                    className="min-h-9 shrink-0 px-3"
                    disabled
                    title={t("proposal.create.blocked")}
                  >
                    {t("proposal.create.entry")}
                  </Button>
                ) : (
                  <Button size="sm" className="min-h-9 shrink-0 gap-1 px-3" onClick={() => setOverlay({ kind: "proposal-form" })}>
                    <Plus className="size-3.5" aria-hidden="true" />
                    {t("proposal.create.entry")}
                  </Button>
                )}
              </div>
              <ProposalList
                proposals={matched.proposals}
                filter={status}
                following={following}
                votes={votes}
                gateFor={gateFor}
                searching={searching}
                searchQ={q}
                onClearSearch={clearSearch}
                onGateOpen={(id) => setGateFor(id)}
                onVote={handleVote}
                onFollow={() => handleFollow(true)}
                onCreateEntry={() => setOverlay({ kind: "proposal-form" })}
              />
            </div>
          )}

      {/* ── 浮层 ── */}
      {overlay?.kind === "share" && <ShareOverlay item={overlay.item} onClose={closeOverlay} />}
      {overlay?.kind === "discussion" && activeThread && (
        <DiscussionOverlay
          thread={activeThread}
          extraReplies={extraReplies[activeThread.id] ?? []}
          onReply={handleReply}
          onClose={closeOverlay}
        />
      )}
      {overlay?.kind === "proposal-form" && (
        <ProposalForm profile={base.profile} onSubmit={handleProposalSubmit} onClose={closeOverlay} />
      )}

      {/* ── 场景浮条（原型工具，非设计的一部分）── */}
      <ScenarioBar scenario={scenario} onSwitch={(next) => setParams({ scenario: next }, "replace")} />
    </div>
  );
}

function ScenarioBar({ scenario, onSwitch }: { scenario: Scenario; onSwitch: (s: Scenario) => void }) {
  if (process.env.NODE_ENV === "production") return null;
  const idx = SCENARIOS.indexOf(scenario);
  return (
    <div className="fixed bottom-4 left-1/2 z-[60] flex -translate-x-1/2 items-center gap-1 rounded-full bg-foreground py-1.5 pl-1.5 pr-3 text-background shadow-[var(--elevation-3)]">
      <button
        type="button"
        onClick={() => onSwitch(SCENARIOS[(idx + SCENARIOS.length - 1) % SCENARIOS.length])}
        aria-label="上一个场景"
        className="rounded-full p-1.5 transition-colors duration-150 hover:bg-background/10"
      >
        <ChevronLeft className="size-4" aria-hidden="true" />
      </button>
      <span className="text-xs font-medium">
        {t("proto.scenarioLabel")} · {t(`proto.scenario.${scenario}`)}
      </span>
      <button
        type="button"
        onClick={() => onSwitch(SCENARIOS[(idx + 1) % SCENARIOS.length])}
        aria-label="下一个场景"
        className="rounded-full p-1.5 transition-colors duration-150 hover:bg-background/10"
      >
        <ChevronRight className="size-4" aria-hidden="true" />
      </button>
      <span className="ml-1 hidden text-[10px] opacity-60 sm:inline">← / →</span>
    </div>
  );
}
