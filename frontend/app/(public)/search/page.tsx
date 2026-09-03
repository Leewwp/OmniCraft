"use client";

import { useState, useCallback, useEffect, useRef } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { normalizeContentList } from "@/lib/content";
import {
  buildContentSearchPath,
  normalizeSearchFilters,
  type SearchFilterConfig,
} from "@/lib/search-filters";
import { SearchAgentInput } from "@/components/agent/SearchAgentInput";
import { AgentFeatureGate } from "@/components/agent/AgentFeatureGate";
import { FacetedSearchSidebar } from "@/components/layout/FacetedSearchSidebar";
import { ContentCard, type ContentCardData } from "@/components/content/ContentCard";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { TagBadge } from "@/components/ui/TagBadge";
import { Bookmark, Grid3X3, List, Search, SlidersHorizontal, User, X } from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import { silentError } from "@/lib/error-handler";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";

interface UserSearchRow {
  id: number;
  username: string;
  avatar_url?: string;
  reputation: number;
  role: string;
}

export default function SearchPage() {
  const t = useTranslations();
  const { user } = useAuth();
  const [results, setResults] = useState<ContentCardData[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [viewMode, setViewMode] = useState<"grid" | "list">("grid");
  const [filterConfig, setFilterConfig] = useState<SearchFilterConfig>({});
  const [saveOpen, setSaveOpen] = useState(false);
  const [saveName, setSaveName] = useState("");
  const [saveBusy, setSaveBusy] = useState(false);
  const [filterDrawerOpen, setFilterDrawerOpen] = useState(false);
  const [resultTab, setResultTab] = useState<"content" | "users">("content");
  const [userResults, setUserResults] = useState<UserSearchRow[]>([]);
  const [usersLoading, setUsersLoading] = useState(false);
  const [usersSearched, setUsersSearched] = useState(false);
  const openFilterButtonRef = useRef<HTMLButtonElement | null>(null);
  const closeFilterButtonRef = useRef<HTMLButtonElement | null>(null);
  const normalizedFilters = normalizeSearchFilters(filterConfig);

  /* Users tab (T21): fetch real user results from users/search only while the
   * tab is active, keyed by the current query. */
  useEffect(() => {
    if (resultTab !== "users" || !query.trim()) return;
    let cancelled = false;
    setUsersLoading(true);
    api
      .get<{ users?: UserSearchRow[] }>(`/api/v1/users/search?q=${encodeURIComponent(query)}&limit=20`)
      .then((data) => {
        if (cancelled) return;
        setUserResults(data.users ?? []);
        setUsersSearched(true);
      })
      .catch((e) => {
        if (!cancelled) {
          silentError(e, { component: "SearchPage", action: "user search" });
          setUserResults([]);
          setUsersSearched(true);
        }
      })
      .finally(() => {
        if (!cancelled) setUsersLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [resultTab, query]);

  useEffect(() => {
    if (filterDrawerOpen) {
      closeFilterButtonRef.current?.focus();
    }
  }, [filterDrawerOpen]);

  /* URL 取数（T21 搜索可达性）：全局搜索框/建议下拉跳转 /search?q= 时，
   * 首挂载读一次 q 并立即执行内容搜索（此前直接落地为空态）。 */
  useEffect(() => {
    const q = (new URLSearchParams(window.location.search).get("q") || "").trim();
    if (q) {
      setQuery(q);
      void doSearch(q, filterConfig);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function closeFilterDrawer() {
    setFilterDrawerOpen(false);
    requestAnimationFrame(() => openFilterButtonRef.current?.focus());
  }

  const doSearch = useCallback(async (q: string, filter: SearchFilterConfig) => {
    if (!q.trim()) return;
    setLoading(true);
    setError("");
    try {
      const data = await api.get<{ items?: unknown[]; contents?: unknown[]; total?: number }>(
        buildContentSearchPath(q, filter),
      );
      setResults(normalizeContentList(data.items ?? data.contents ?? []));
    } catch (e) {
      silentError(e, { component: 'SearchPage', action: 'doSearch' });
      setError(t(getUserFacingErrorKey(e, "common.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [t]);

  function handleSearch(r: Record<string, unknown>[], q: string) {
    setQuery(q);
    if (r.length > 0) {
      setResults(normalizeContentList(r));
    } else {
      // Fallback to keyword search
      void doSearch(q, filterConfig);
    }
  }

  function handleFilterChange(config: SearchFilterConfig) {
    const nextConfig = normalizeSearchFilters(config);
    setFilterConfig(nextConfig);
    if (query) void doSearch(query, nextConfig);
  }

  async function handleSaveSearch() {
    if (!saveName.trim() || !user) return;
    setSaveBusy(true);
    try {
      await api.post("/api/v1/users/me/saved-searches", {
        name: saveName.trim(),
        config: { query, ...filterConfig },
      });
      setSaveOpen(false);
      setSaveName("");
    } catch (e) {
      silentError(e, { component: 'SearchPage', action: 'handleSaveSearch' });
    } finally {
      setSaveBusy(false);
    }
  }

  return (
    <div className="mx-auto w-full max-w-[1280px] space-y-6 px-4 py-6">
      {/* Top: Search Input */}
      <div className="rounded-md border border-border bg-card p-4 ">
        <AgentFeatureGate
          capability="webAgent"
          fallback={
            <div className="flex items-center gap-2">
              <div className="relative flex-1">
                <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <input
                  type="text"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") void doSearch(query, filterConfig);
                  }}
                  placeholder={t("agent.searchKeywordPlaceholder")}
                  className="w-full rounded-md border border-border bg-background py-2.5 pl-10 pr-4 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
                />
              </div>
              <Button
                size="sm"
                onClick={() => void doSearch(query, filterConfig)}
                disabled={!query.trim()}
                aria-label={t("discussion.search")}
              >
                <Search className="h-4 w-4" />
              </Button>
            </div>
          }
        >
          <SearchAgentInput onResults={handleSearch} />
        </AgentFeatureGate>
      </div>

      <div className="flex flex-col gap-4 min-[701px]:flex-row">
        {/* Mobile filter button */}
        <div className="min-[701px]:hidden">
          <Button
            ref={openFilterButtonRef}
            variant="outline"
            size="sm"
            onClick={() => setFilterDrawerOpen(true)}
            className="w-full"
            aria-label={t("search.filter.openAdvancedFilter")}
          >
            <SlidersHorizontal className="mr-1.5 h-4 w-4" />
            {t('search.filter.advancedFilter')}
          </Button>
        </div>

        {/* Mobile filter drawer */}
        {filterDrawerOpen && (
          <div className="fixed inset-0 z-50 min-[701px]:hidden">
            <button
              type="button"
              aria-label={t("common.close")}
              className="absolute inset-0 bg-black/50"
              onClick={closeFilterDrawer}
            />
            <div
              role="dialog"
              aria-modal="true"
              aria-labelledby="mobile-filter-title"
              className="absolute inset-y-0 left-0 w-[85vw] max-w-sm overflow-y-auto border-r border-border bg-background p-4 shadow-[var(--elevation-3)]"
              onKeyDown={(e) => {
                if (e.key === "Escape") closeFilterDrawer();
                if (e.key === "Tab") {
                  const focusable = Array.from(
                    e.currentTarget.querySelectorAll<HTMLElement>(
                      'button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])',
                    ),
                  );
                  const first = focusable[0];
                  const last = focusable[focusable.length - 1];
                  if (!first || !last) return;
                  if (e.shiftKey && document.activeElement === first) {
                    e.preventDefault();
                    last.focus();
                  } else if (!e.shiftKey && document.activeElement === last) {
                    e.preventDefault();
                    first.focus();
                  }
                }
              }}
            >
              <div className="mb-3 flex items-center justify-between">
                <span id="mobile-filter-title" className="text-sm font-semibold">
                  {t('search.filter.advancedFilter')}
                </span>
                <button
                  type="button"
                  ref={closeFilterButtonRef}
                  onClick={closeFilterDrawer}
                  className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md hover:bg-muted focus:outline-none focus:ring-2 focus:ring-ring"
                  aria-label={t("common.close")}
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
              <FacetedSearchSidebar defaultAdvancedOpen onFilterChange={(c) => { handleFilterChange(c); }} />
            </div>
          </div>
        )}

        {/* Left: FacetedSearchSidebar (desktop) */}
        <aside className="hidden w-[228px] shrink-0 min-[701px]:block min-[1101px]:w-[260px]">
          <FacetedSearchSidebar onFilterChange={handleFilterChange} />
        </aside>

        {/* Right: Results */}
        <main data-testid="search-results-panel" className="min-w-0 flex-1 space-y-4">
          {/* Toolbar */}
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex items-center gap-1 rounded-md border border-border p-0.5">
              <Button
                variant={viewMode === "grid" ? "default" : "ghost"}
                size="sm"
                className="h-7 px-2 [@media(pointer:coarse)]:min-h-11 [@media(pointer:coarse)]:min-w-11"
                onClick={() => setViewMode("grid")}
                aria-label={t("search.gridView")}
                aria-pressed={viewMode === "grid"}
              >
                <Grid3X3 className="h-3.5 w-3.5" />
              </Button>
              <Button
                variant={viewMode === "list" ? "default" : "ghost"}
                size="sm"
                className="h-7 px-2 [@media(pointer:coarse)]:min-h-11 [@media(pointer:coarse)]:min-w-11"
                onClick={() => setViewMode("list")}
                aria-label={t("search.listView")}
                aria-pressed={viewMode === "list"}
              >
                <List className="h-3.5 w-3.5" />
              </Button>
            </div>

            {/* Active filter chips */}
            {normalizedFilters.category && (
              <TagBadge color="purple" onRemove={() => handleFilterChange({ ...normalizedFilters, category: "" })}>
                {normalizedFilters.category}
              </TagBadge>
            )}
            {normalizedFilters.selectedTags.map((tag) => (
              <TagBadge
                key={tag}
                color="blue"
                onRemove={() =>
                  handleFilterChange({
                    ...normalizedFilters,
                    selectedTags: normalizedFilters.selectedTags.filter((t) => t !== tag),
                  })
                }
              >
                {tag}
              </TagBadge>
            ))}

            {normalizedFilters.contentTypes.map((contentType) => (
              <TagBadge
                key={contentType}
                color="green"
                onRemove={() =>
                  handleFilterChange({
                    ...normalizedFilters,
                    contentTypes: normalizedFilters.contentTypes.filter((item) => item !== contentType),
                  })
                }
              >
                {contentType}
              </TagBadge>
            ))}
            {normalizedFilters.timeRange && (
              <TagBadge color="orange" onRemove={() => handleFilterChange({ ...normalizedFilters, timeRange: "" })}>
                {normalizedFilters.timeRange}
              </TagBadge>
            )}

            {/* Save search */}
            {user && query && (
              <div className="ml-auto">
                {saveOpen ? (
                  <div className="flex items-center gap-1">
                    <input
                      type="text"
                      value={saveName}
                      onChange={(e) => setSaveName(e.target.value)}
                      placeholder={t("search.saveNamePlaceholder")}
                      className="w-32 rounded border border-border bg-background px-2 py-1 text-xs focus:outline-none focus:ring-2 focus:ring-accent"
                    />
                    <Button size="sm" className="h-6 text-xs" onClick={handleSaveSearch} disabled={saveBusy}>
                      {t("common.save")}
                    </Button>
                    <Button size="sm" variant="ghost" className="h-6 w-6 p-0 [@media(pointer:coarse)]:min-h-11 [@media(pointer:coarse)]:min-w-11" onClick={() => setSaveOpen(false)} aria-label={t("common.close")}>
                      <X className="h-3 w-3" />
                    </Button>
                  </div>
                ) : (
                  <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => setSaveOpen(true)}>
                    <Bookmark className="mr-1 h-3 w-3" />
                    {t("search.saveSearch")}
                  </Button>
                )}
              </div>
            )}
          </div>

          <Tabs
            value={resultTab}
            onValueChange={(value) => setResultTab(value === "users" ? "users" : "content")}
          >
            <TabsList aria-label={t("search.resultTabsLabel")}>
              <TabsTrigger value="content">{t("search.tabContents")}</TabsTrigger>
              <TabsTrigger value="users">{t("search.tabUsers")}</TabsTrigger>
            </TabsList>

            <TabsContent value="content" className="mt-4 space-y-6">
              {/* Error */}
              {error && (
                <div className="rounded-md border border-border bg-card p-4 text-center ">
                  <p className="text-sm text-destructive">{error}</p>
                  <Button size="sm" variant="outline" className="mt-2" onClick={() => query && void doSearch(query, filterConfig)}>
                    {t("common.retry")}
                  </Button>
                </div>
              )}

              {/* Loading */}
              {loading && (
                <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
                  {Array.from({ length: 8 }).map((_, i) => (
                    <Skeleton key={i} className="aspect-[3/4] w-full" />
                  ))}
                </div>
              )}

              {/* Empty */}
              {!loading && !error && query && results.length === 0 && (
                <EmptyState
                  icon={Search}
                  title={t("search.noResults")}
                  description={t("search.noResultsHint")}
                  className="p-12"
                />
              )}

              {/* No query yet */}
              {!loading && !error && !query && (
                <EmptyState
                  icon={Search}
                  title={t("search.startSearch")}
                  description={t("search.startSearchHint")}
                  className="p-12"
                />
              )}

              {/* Results */}
              {!loading && results.length > 0 && (
                viewMode === "list" ? (
                  <div className="space-y-3">
                    {results.map((item) => (
                      <ContentCard key={item.id} data={item} />
                    ))}
                  </div>
                ) : (
                  <MasonryGrid items={results} />
                )
              )}
            </TabsContent>

            <TabsContent value="users" className="mt-4 space-y-6">
              {usersLoading && (
                <div className="space-y-3">
                  {Array.from({ length: 4 }).map((_, i) => (
                    <Skeleton key={i} className="h-14 w-full" />
                  ))}
                </div>
              )}
              {!usersLoading && usersSearched && userResults.length === 0 && (
                <EmptyState
                  icon={User}
                  title={t("search.usersNoResults")}
                  description={t("search.usersNoResultsHint")}
                  className="p-12"
                />
              )}
              {!usersLoading && userResults.length > 0 && (
                <ul className="space-y-2">
                  {userResults.map((row) => (
                    <li key={row.id}>
                      <a
                        href={row.id ? `/user/${row.id}` : "#"}
                        className="flex items-center gap-3 rounded-md border border-border bg-card p-3 transition-colors hover:border-ring"
                      >
                        {row.avatar_url ? (
                          // eslint-disable-next-line @next/next/no-img-element -- 远程头像域不受 next/image 管辖
                          <img src={row.avatar_url} alt="" className="h-10 w-10 rounded-full object-cover" />
                        ) : (
                          <span className="flex h-10 w-10 items-center justify-center rounded-full bg-canvas-subtle text-sm font-bold text-muted-foreground">
                            {row.username.slice(0, 1).toUpperCase()}
                          </span>
                        )}
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm font-bold text-foreground">{row.username}</span>
                          <span className="block text-xs text-muted-foreground">
                            {t("search.userReputation", { value: row.reputation })}
                          </span>
                        </span>
                      </a>
                    </li>
                  ))}
                </ul>
              )}
              {!usersLoading && !usersSearched && !query && (
                <EmptyState
                  icon={User}
                  title={t("search.startSearch")}
                  description={t("search.startSearchHint")}
                  className="p-12"
                />
              )}
            </TabsContent>
          </Tabs>
        </main>
      </div>
    </div>
  );
}
