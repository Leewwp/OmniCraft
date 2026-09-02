"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Layers, MessageSquareText, Search, Users, X } from "lucide-react";
import { FollowButton } from "@/components/social/FollowButton";
import { TagBadge } from "@/components/ui/TagBadge";
import { RecordBrowseHistory } from "@/components/tracking/RecordBrowseHistory";
import { IPShareTab } from "@/components/ip/hub/IPShareTab";
import { IPDiscussionsTab } from "@/components/ip/hub/IPDiscussionsTab";
import { IPProposalsTab } from "@/components/ip/hub/IPProposalsTab";
import { useAuth } from "@/contexts/AuthContext";

export interface IPHubData {
  id: number;
  name: string;
  description?: string;
  category?: string;
  cover_url?: string;
  tags?: string[];
  follower_count?: number;
  is_following?: boolean;
}

export interface IPHubStats {
  follower_count: number;
  discussion_count: number;
  work_count: number;
}

export type IPHubTab = "share" | "discussions" | "proposals";

const TAB_KEYS: IPHubTab[] = ["share", "discussions", "proposals"];

// 各模块筛选/排序词表：URL 传入值不在词表内时回落默认值（#290 单页 query 驱动）。
export const SHARE_TYPES = ["all", "image", "article", "video", "audio", "mod", "prompt", "sheet_music", "other"];
export const SHARE_SORTS = ["newest", "hot", "most_views", "best_rated"];
export const DISCUSSION_SORTS = ["latest_reply", "newest_post", "most_replies", "hot"];
export const PROPOSAL_STATUSES = ["open", "adopted", "rejected", "history"];

function pickValid(value: string | null, allowed: string[], fallback: string): string {
  return value && allowed.includes(value) ? value : fallback;
}

// tab 切换时携带 sort：目标 tab 词表外的值回落该 tab 默认（URL 与渲染态一致）。
function sortForTab(target: IPHubTab, current: string): string {
  const allowed = target === "discussions" ? DISCUSSION_SORTS : SHARE_SORTS;
  if (allowed.includes(current)) return current;
  return target === "discussions" ? "latest_reply" : "hot";
}

const TAG_COLOR_CYCLE = ["blue", "green", "purple", "orange", "rose", "sky"] as const;

function categoryLabelKey(category?: string): string {
  switch (category) {
    case "game": return "home.categoryGaming";
    case "film_tv": return "home.categoryFilmTv";
    case "anime": return "home.animeCategory";
    case "manga": return "home.mangaCategory";
    case "novel": return "home.novelCategory";
    case "music": return "home.audio";
    case "variety": return "home.varietyShowCategory";
    case "short_drama": return "home.shortDramaCategory";
    case "vtuber": return "home.other";
    default: return "home.other";
  }
}

interface IPHubClientProps {
  ip: IPHubData;
  stats: IPHubStats;
  apiBase: string;
}

// 搜索命中计数：tab 计数随命中数收缩（无搜索时回落 SSR 统计）。
interface HubHits {
  share: number | null;
  discussions: number | null;
  proposals: number | null;
}

// /ip/[ipId] 贴吧式社区枢纽（#290）：单页 query 驱动（tab/type/sort/status/q），
// 三模块页内切换 + IP 内搜索（回车/失焦提交），全流程不跳页；浏览器后退可用。
export function IPHubClient({ ip, stats, apiBase }: IPHubClientProps) {
  const t = useTranslations();
  const router = useRouter();
  const searchParams = useSearchParams();
  const { user } = useAuth();

  const initialTab = (searchParams.get("tab") as IPHubTab) || "share";
  const tab: IPHubTab = TAB_KEYS.includes(initialTab) ? initialTab : "share";
  const query = searchParams.get("q")?.trim() || "";
  // 301 旧类目路由落点 ?tab=share&type=<category>：type 在词表内即被消费
  const type = pickValid(searchParams.get("type"), SHARE_TYPES, "all");
  const sort = pickValid(
    searchParams.get("sort"),
    tab === "discussions" ? DISCUSSION_SORTS : SHARE_SORTS,
    tab === "discussions" ? "latest_reply" : "hot",
  );
  const status = pickValid(searchParams.get("status"), PROPOSAL_STATUSES, "open");

  const [searchInput, setSearchInput] = useState(query);
  const [liveStats, setLiveStats] = useState(stats);
  const [proposalTotal, setProposalTotal] = useState<number | null>(null);
  const [hits, setHits] = useState<HubHits>({ share: null, discussions: null, proposals: null });
  const searchInputRef = useRef<HTMLInputElement>(null);

  // URL 写回：筛选/tab 就地 replace（不入历史、不滚动）；q 走 push 可后退。
  const buildUrl = useCallback((overrides: Record<string, string | null>) => {
    const params = new URLSearchParams();
    const merged: Record<string, string | null> = { tab, type, sort, status, q: query };
    for (const [key, value] of Object.entries(overrides)) merged[key] = value;
    params.set("tab", merged.tab ?? "share");
    if (merged.type && merged.type !== "all") params.set("type", merged.type);
    if (merged.sort) params.set("sort", merged.sort);
    if (merged.status && merged.status !== "open") params.set("status", merged.status);
    if (merged.q) params.set("q", merged.q);
    return `/ip/${ip.id}?${params.toString()}`;
  }, [ip.id, tab, type, sort, status, query]);

  const switchFilter = useCallback((overrides: Record<string, string | null>) => {
    router.replace(buildUrl(overrides), { scroll: false });
  }, [buildUrl, router]);

  // 搜索：回车或失焦提交；?q= 走 history push 可后退（handoff §1.1）
  const submitSearch = useCallback(() => {
    const next = searchInput.trim();
    if (next === query) return;
    router.push(buildUrl({ q: next || null }), { scroll: false });
  }, [searchInput, query, buildUrl, router]);

  const clearSearch = useCallback(() => {
    setSearchInput("");
    router.push(buildUrl({ q: null }), { scroll: false });
  }, [buildUrl, router]);

  // 浏览器后退/前进时同步输入框内容
  useEffect(() => {
    setSearchInput(searchParams.get("q")?.trim() || "");
  }, [searchParams]);

  // 提案 tab 基线计数（GetIP stats 无提案数，单独拉一次跨状态 total）
  useEffect(() => {
    let cancelled = false;
    fetch(`${apiBase}/ips/${ip.id}/proposals?status=all`, { cache: "no-store" })
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => { if (!cancelled) setProposalTotal(data?.total ?? null); })
      .catch(() => {});
    return () => { cancelled = true; };
  }, [apiBase, ip.id]);

  // 搜索命中计数：q 非空时并行取三模块命中 total；q 清空回落基线
  useEffect(() => {
    if (!query) {
      setHits({ share: null, discussions: null, proposals: null });
      return;
    }
    let cancelled = false;
    const enc = encodeURIComponent(query);
    (async () => {
      const get = (path: string) =>
        fetch(`${apiBase}${path}`, { cache: "no-store" }).then((res) => (res.ok ? res.json() : null)).catch(() => null);
      const [contents, discussions, proposals] = await Promise.all([
        get(`/ips/${ip.id}/contents?page_size=1&q=${enc}`),
        get(`/ips/${ip.id}/discussions?page_size=1&q=${enc}`),
        get(`/ips/${ip.id}/proposals?status=all&q=${enc}`),
      ]);
      if (!cancelled) {
        setHits({
          share: contents?.total ?? 0,
          discussions: discussions?.total ?? 0,
          proposals: proposals?.total ?? 0,
        });
      }
    })();
    return () => { cancelled = true; };
  }, [query, apiBase, ip.id]);

  const tabCount = (key: IPHubTab): number | null => {
    if (query) return hits[key];
    switch (key) {
      case "share": return liveStats.work_count;
      case "discussions": return liveStats.discussion_count;
      case "proposals": return proposalTotal;
    }
  };

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6">
      <RecordBrowseHistory contentType="ip" targetId={ip.id} />

      {/* 头部：IP 身份区 */}
      <section className="space-y-4 rounded-md border border-border bg-card p-4">
        <div className="flex flex-col gap-4 md:flex-row">
          <div className="relative flex h-36 w-full items-center justify-center overflow-hidden rounded-md border border-border bg-muted/40 md:w-52">
            {ip.cover_url ? (
              <Image src={ip.cover_url} alt={ip.name} fill className="rounded-md object-cover" sizes="208px" />
            ) : (
              <span className="text-sm text-muted-foreground">{t('ip.cover')}</span>
            )}
          </div>
          <div className="flex flex-1 flex-col gap-2">
            <h1 className="text-2xl font-bold tracking-tight">{ip.name}</h1>
            {ip.category && (
              <p className="text-sm text-muted-foreground">{t('ip.category', { category: t(categoryLabelKey(ip.category)) })}</p>
            )}
            <p className="text-sm leading-relaxed text-foreground/90">{ip.description || t('ip.noDescription')}</p>
            {ip.tags && ip.tags.length > 0 && (
              <div className="flex flex-wrap gap-1.5" aria-label={t('ip.tagsLabel')}>
                {ip.tags.map((tag, index) => (
                  <TagBadge key={tag} color={TAG_COLOR_CYCLE[index % TAG_COLOR_CYCLE.length]}>{tag}</TagBadge>
                ))}
              </div>
            )}
            <div className="flex flex-wrap items-center gap-3">
              <FollowButton
                targetType="ip"
                targetId={ip.id}
                initialFollowing={ip.is_following ?? false}
                className="min-w-[104px]"
                key={ip.is_following ? "following" : "not-following"}
              />
              <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                <Users className="h-3.5 w-3.5" aria-hidden="true" />
                {t('ip.followerCount', { count: liveStats.follower_count })}
              </span>
              <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                <MessageSquareText className="h-3.5 w-3.5" aria-hidden="true" />
                {t('ip.hubDiscussionCount', { count: query ? (hits.discussions ?? liveStats.discussion_count) : liveStats.discussion_count })}
              </span>
              <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                <Layers className="h-3.5 w-3.5" aria-hidden="true" />
                {t('ip.hubWorkCount', { count: query ? (hits.share ?? liveStats.work_count) : liveStats.work_count })}
              </span>
              {!user && (
                <Link href={`/login?redirect=/ip/${ip.id}`} className="text-xs text-accent-emphasis hover:underline">
                  {t('ip.hubLoginHint')}
                </Link>
              )}
            </div>
          </div>
        </div>
      </section>

      {/* 搜索框 + 三模块 tab */}
      <div className="sticky top-[52px] z-40 -mx-4 border-b border-border-default bg-canvas-default px-4 py-2.5 md:-mx-6 md:px-6">
        <div className="flex flex-col gap-2 md:flex-row md:items-center">
          <form
            role="search"
            className="relative min-w-0 flex-1"
            onSubmit={(e) => { e.preventDefault(); submitSearch(); }}
            onBlur={() => submitSearch()}
          >
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
            <input
              ref={searchInputRef}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder={t('ip.hubSearchPlaceholder')}
              aria-label={t('ip.hubSearchPlaceholder')}
              className="min-h-9 w-full rounded-full border border-border bg-muted pl-9 pr-9 text-sm placeholder:text-muted-foreground/60 focus:bg-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
            {searchInput && (
              <button
                type="button"
                aria-label={t('ip.hubClearSearch')}
                onClick={clearSearch}
                className="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
              >
                <X className="h-3.5 w-3.5" aria-hidden="true" />
              </button>
            )}
          </form>
          <nav aria-label={t('ip.hubModules')} className="flex items-center gap-1 overflow-x-auto pb-1" style={{ scrollbarWidth: "none" }}>
            {TAB_KEYS.map((key) => {
              const active = tab === key;
              const count = tabCount(key);
              const label = t(`ip.hubTab_${key}`);
              return (
                <button
                  key={key}
                  type="button"
                  onClick={() => switchFilter({ tab: key, sort: sortForTab(key, sort) })}
                  aria-pressed={active}
                  className={`inline-flex min-h-9 flex-shrink-0 items-center gap-1 rounded-full border px-3 text-xs font-medium transition-colors duration-150 whitespace-nowrap focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                    active
                      ? "border-accent-emphasis bg-accent-subtle text-accent-emphasis font-semibold"
                      : "border-transparent text-muted-foreground hover:bg-muted hover:text-foreground"
                  }`}
                >
                  {label}
                  {count != null && <span className="text-[11px] tabular-nums opacity-70">{count}</span>}
                </button>
              );
            })}
          </nav>
        </div>
      </div>

      {/* 三模块主体 */}
      {tab === "share" && (
        <IPShareTab
          ipId={ip.id}
          apiBase={apiBase}
          query={query}
          type={type}
          sort={sort}
          onTypeChange={(next) => switchFilter({ type: next })}
          onSortChange={(next) => switchFilter({ sort: next })}
        />
      )}
      {tab === "discussions" && (
        <IPDiscussionsTab
          ipId={ip.id}
          apiBase={apiBase}
          query={query}
          sort={sort}
          onSortChange={(next) => switchFilter({ sort: next })}
          initialDiscussionId={(() => {
            const d = parseInt(searchParams.get("d") ?? "", 10);
            return Number.isFinite(d) && d > 0 ? d : null;
          })()}
        />
      )}
      {tab === "proposals" && (
        <IPProposalsTab
          ipId={ip.id}
          apiBase={apiBase}
          query={query}
          status={status}
          canCreate={!!user}
          currentDescription={ip.description}
          ipCoverUrl={ip.cover_url}
          onStatusChange={(next) => switchFilter({ status: next })}
          onFollowed={() => setLiveStats((s) => ({ ...s, follower_count: s.follower_count + 1 }))}
        />
      )}
    </div>
  );
}
