"use client";

import { useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { SearchAgentInput } from "@/components/agent/SearchAgentInput";
import { AgentFeatureGate } from "@/components/agent/AgentFeatureGate";
import { FacetedSearchSidebar } from "@/components/layout/FacetedSearchSidebar";
import { ContentCard, type ContentCardData } from "@/components/content/ContentCard";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { Button } from "@/components/ui/button";
import { TagBadge } from "@/components/ui/TagBadge";
import { Bookmark, Grid3X3, List, Search, SlidersHorizontal, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { silentError } from "@/lib/error-handler";

interface FilterConfig {
  category?: string;
  selectedTags?: string[];
  contentTypes?: string[];
  timeRange?: string;
  sort?: string;
}

export default function SearchPage() {
  const t = useTranslations();
  const { user } = useAuth();
  const [results, setResults] = useState<ContentCardData[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [viewMode, setViewMode] = useState<"grid" | "list">("grid");
  const [filterConfig, setFilterConfig] = useState<FilterConfig>({});
  const [saveOpen, setSaveOpen] = useState(false);
  const [saveName, setSaveName] = useState("");
  const [saveBusy, setSaveBusy] = useState(false);
  const [filterDrawerOpen, setFilterDrawerOpen] = useState(false);

  const doSearch = useCallback(async (q: string, filter: FilterConfig) => {
    if (!q.trim()) return;
    setLoading(true);
    setError("");
    try {
      const params = new URLSearchParams();
      params.set("q", q);
      if (filter.category) params.set("category", filter.category);
      if (filter.selectedTags?.length) params.set("tags", filter.selectedTags.join(","));
      if (filter.contentTypes?.length) params.set("content_type", filter.contentTypes.join(","));
      if (filter.timeRange) params.set("time_range", filter.timeRange);
      if (filter.sort) params.set("sort", filter.sort);

      const data = await api.get<{ contents?: ContentCardData[]; total?: number }>(
        `/api/v1/contents?${params.toString()}`,
      );
      setResults(data.contents ?? []);
    } catch (e) {
      silentError(e, { component: 'SearchPage', action: 'doSearch' });
      setError(e instanceof ApiRequestError ? e.message : t("common.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  function handleSearch(r: ContentCardData[], q: string) {
    setQuery(q);
    if (r.length > 0) {
      setResults(r);
    } else {
      // Fallback to keyword search
      doSearch(q, filterConfig);
    }
  }

  function handleFilterChange(config: FilterConfig) {
    setFilterConfig(config);
    if (query) doSearch(query, config);
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
                    if (e.key === "Enter") doSearch(query, filterConfig);
                  }}
                  placeholder={t("agent.searchKeywordPlaceholder")}
                  className="w-full rounded-md border border-border bg-background py-2.5 pl-10 pr-4 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
                />
              </div>
              <Button size="sm" onClick={() => doSearch(query, filterConfig)} disabled={!query.trim()}>
                <Search className="h-4 w-4" />
              </Button>
            </div>
          }
        >
          <SearchAgentInput onResults={(r, q) => handleSearch(r as unknown as ContentCardData[], q)} />
        </AgentFeatureGate>
      </div>

      <div className="flex gap-4">
        {/* Mobile filter button */}
        <div className="md:hidden mb-2">
          <Button variant="outline" size="sm" onClick={() => setFilterDrawerOpen(true)} className="w-full">
            <SlidersHorizontal className="mr-1.5 h-4 w-4" />
            {t('search.filter.advancedFilter')}
          </Button>
        </div>

        {/* Mobile filter drawer */}
        {filterDrawerOpen && (
          <div className="fixed inset-0 z-50 md:hidden">
            <div className="absolute inset-0 bg-black/40" onClick={() => setFilterDrawerOpen(false)} />
            <div className="absolute bottom-0 left-0 right-0 max-h-[80vh] overflow-y-auto rounded-t-xl bg-background p-4 border-t border-border">
              <div className="flex items-center justify-between mb-3">
                <span className="text-sm font-semibold">{t('search.filter.advancedFilter')}</span>
                <button onClick={() => setFilterDrawerOpen(false)} className="p-1 rounded hover:bg-muted">
                  <X className="h-4 w-4" />
                </button>
              </div>
              <FacetedSearchSidebar onFilterChange={(c) => { handleFilterChange(c); }} />
            </div>
          </div>
        )}

        {/* Left: FacetedSearchSidebar (desktop) */}
        <aside className="hidden w-[260px] shrink-0 md:block">
          <FacetedSearchSidebar onFilterChange={handleFilterChange} />
        </aside>

        {/* Right: Results */}
        <main className="min-w-0 flex-1 space-y-4">
          {/* Toolbar */}
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex items-center gap-1 rounded-md border border-border p-0.5">
              <Button
                variant={viewMode === "grid" ? "default" : "ghost"}
                size="sm"
                className="h-7 px-2"
                onClick={() => setViewMode("grid")}
              >
                <Grid3X3 className="h-3.5 w-3.5" />
              </Button>
              <Button
                variant={viewMode === "list" ? "default" : "ghost"}
                size="sm"
                className="h-7 px-2"
                onClick={() => setViewMode("list")}
              >
                <List className="h-3.5 w-3.5" />
              </Button>
            </div>

            {/* Active filter chips */}
            {filterConfig.category && (
              <TagBadge color="purple" onRemove={() => handleFilterChange({ ...filterConfig, category: undefined })}>
                {filterConfig.category}
              </TagBadge>
            )}
            {filterConfig.selectedTags?.map((tag) => (
              <TagBadge
                key={tag}
                color="blue"
                onRemove={() =>
                  handleFilterChange({
                    ...filterConfig,
                    selectedTags: filterConfig.selectedTags?.filter((t) => t !== tag),
                  })
                }
              >
                {tag}
              </TagBadge>
            ))}

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
                    <Button size="sm" variant="ghost" className="h-6 w-6 p-0" onClick={() => setSaveOpen(false)}>
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

          {/* Error */}
          {error && (
            <div className="rounded-md border border-border bg-card p-4 text-center ">
              <p className="text-sm text-destructive">{error}</p>
              <Button size="sm" variant="outline" className="mt-2" onClick={() => query && doSearch(query, filterConfig)}>
                {t("common.retry")}
              </Button>
            </div>
          )}

          {/* Loading */}
          {loading && (
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
              {Array.from({ length: 8 }).map((_, i) => (
                <div key={i} className="aspect-[3/4] animate-pulse rounded-md bg-muted/30" />
              ))}
            </div>
          )}

          {/* Empty */}
          {!loading && !error && query && results.length === 0 && (
            <div className="rounded-md border border-border bg-card p-12 text-center ">
              <Search className="mx-auto h-10 w-10 text-muted-foreground" />
              <p className="mt-3 text-sm font-medium">{t("search.noResults")}</p>
              <p className="mt-1 text-xs text-muted-foreground">{t("search.noResultsHint")}</p>
            </div>
          )}

          {/* No query yet */}
          {!loading && !error && !query && (
            <div className="rounded-md border border-border bg-card p-12 text-center ">
              <Search className="mx-auto h-10 w-10 text-muted-foreground" />
              <p className="mt-3 text-sm text-muted-foreground">{t("search.startSearch")}</p>
            </div>
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
        </main>
      </div>
    </div>
  );
}
