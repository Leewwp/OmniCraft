"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useTranslations } from "next-intl";
import {
  ChevronDown,
  ChevronUp,
  X,
  Bookmark,
  FolderOpen,
  SlidersHorizontal,
  Save,
  Loader2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { TagBadge } from "@/components/ui/TagBadge";
import { Skeleton } from "@/components/ui/skeleton";
import { useAuth } from "@/contexts/AuthContext";
import { useToast } from "@/components/ui/Toast";
import { api } from "@/lib/api";
import { normalizeSearchFilters, type SearchFilterConfig } from "@/lib/search-filters";
import { cn } from "@/lib/utils";

// ── Types ──────────────────────────────────────────────

interface FacetedTag {
  name: string;
  count: number;
}

interface CategoryNode {
  id: number;
  name: string;
  slug?: string;
  zone?: string;
  level?: number;
}

interface TagGroup {
  id: number;
  name: string;
  tags: string[];
}

interface SavedSearch {
  id: number;
  name: string;
  config: FilterConfig;
}

export interface FilterConfig extends SearchFilterConfig {}

export interface FacetedSearchSidebarProps {
  className?: string;
  defaultAdvancedOpen?: boolean;
  onFilterChange?: (config: FilterConfig) => void;
  data?: unknown;
  isLoading?: boolean;
  disabled?: boolean;
  onAction?: (payload: unknown) => void;
}

const CONTENT_TYPE_OPTIONS = [
  { key: "image", label: "home.image" },
  { key: "video", label: "home.video" },
  { key: "audio", label: "home.audio" },
  { key: "text", label: "home.text" },
  { key: "model", label: "content.categoryModel" },
  { key: "template", label: "home.template" },
  { key: "other", label: "home.other" },
];

const TIME_RANGE_OPTIONS = [
  { key: "", label: "search.anyTime" },
  { key: "today", label: "search.today" },
  { key: "week", label: "home.thisWeek" },
  { key: "month", label: "home.thisMonth" },
  { key: "year", label: "home.thisYear" },
];

const SORT_OPTIONS = [
  { key: "", label: "search.defaultSort" },
  { key: "hot", label: "home.hottest" },
  { key: "newest", label: "home.newest" },
  { key: "most_liked", label: "search.mostLiked" },
  { key: "most_viewed", label: "home.mostViewed" },
];

const DEFAULT_CATEGORIES_I18N: { id: number; nameKey: string }[] = [
  { id: 0, nameKey: "home.categoryRecommended" },
  { id: 1, nameKey: "home.categoryFilmTv" },
  { id: 2, nameKey: "home.categoryGaming" },
  { id: 3, nameKey: "home.categoryLiterature" },
  { id: 4, nameKey: "home.categoryPet" },
  { id: 5, nameKey: "home.categoryFood" },
  { id: 6, nameKey: "home.categoryBeautyFashion" },
  { id: 7, nameKey: "home.categoryHome" },
  { id: 8, nameKey: "home.categoryTechDigital" },
  { id: 9, nameKey: "home.categoryTravel" },
  { id: 10, nameKey: "home.categorySports" },
  { id: 11, nameKey: "home.categoryProductivity" },
];

// ── Component ──────────────────────────────────────────

export function FacetedSearchSidebar({
  className,
  defaultAdvancedOpen = false,
  onFilterChange,
  data: _data,
  isLoading: _isLoading,
  disabled: _disabled,
  onAction: _onAction,
}: FacetedSearchSidebarProps) {
  const t = useTranslations();
  const { user } = useAuth();
  const { toast } = useToast();

  const [categories] = useState<CategoryNode[]>(DEFAULT_CATEGORIES_I18N.map(c => ({ id: c.id, name: t(c.nameKey) })));
  const [selectedCategory, setSelectedCategory] = useState<string>("");
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [availableTags, setAvailableTags] = useState<FacetedTag[]>([]);
  const [tagsLoading, setTagsLoading] = useState(false);

  const [contentTypes, setContentTypes] = useState<string[]>([]);
  const [timeRange, setTimeRange] = useState<string>("");
  const [sort, setSort] = useState<string>("");

  const [advancedOpen, setAdvancedOpen] = useState(defaultAdvancedOpen);
  const [saveModalOpen, setSaveModalOpen] = useState(false);
  const [saveName, setSaveName] = useState("");
  const [saving, setSaving] = useState(false);

  const [tagGroups, setTagGroups] = useState<TagGroup[]>([]);
  const [savedSearches, setSavedSearches] = useState<SavedSearch[]>([]);
  const [groupsLoaded, setGroupsLoaded] = useState(false);

  const abortRef = useRef<AbortController | null>(null);

  // ── Emit filter changes ────────────────────────────

  const buildConfig = useCallback((): FilterConfig => ({
    category: selectedCategory || undefined,
    selectedTags: selectedTags.length > 0 ? [...selectedTags] : undefined,
    contentTypes: contentTypes.length > 0 ? [...contentTypes] : undefined,
    timeRange: timeRange || undefined,
    sort: sort || undefined,
  }), [selectedCategory, selectedTags, contentTypes, timeRange, sort]);

  const prevConfigRef = useRef<string>("");

  useEffect(() => {
    const config = buildConfig();
    const key = JSON.stringify(config);
    if (key !== prevConfigRef.current) {
      prevConfigRef.current = key;
      onFilterChange?.(config);
    }
  }, [buildConfig, onFilterChange]);

  // ── Fetch faceted tags ─────────────────────────────

  const fetchTags = useCallback(async () => {
    if (abortRef.current) {
      abortRef.current.abort();
    }
    const controller = new AbortController();
    abortRef.current = controller;

    setTagsLoading(true);
    try {
      const params = new URLSearchParams();
      if (selectedCategory) params.set("category", selectedCategory);
      selectedTags.forEach((t) => params.append("selected_tags[]", t));
      params.set("_t", Date.now().toString());

      const data = await api.get<{ tags?: FacetedTag[] }>(
        `/api/v1/tags/faceted?${params.toString()}`,
      );
      if (!controller.signal.aborted) {
        setAvailableTags(data.tags ?? []);
      }
    } catch {
      if (!controller.signal.aborted) {
        setAvailableTags([]);
      }
    } finally {
      if (!controller.signal.aborted) {
        setTagsLoading(false);
      }
    }
  }, [selectedCategory, selectedTags]);

  useEffect(() => {
    fetchTags();
    return () => {
      abortRef.current?.abort();
    };
  }, [fetchTags]);

  // ── Fetch tag groups & saved searches ──────────────

  useEffect(() => {
    if (!user) {
      setTagGroups([]);
      setSavedSearches([]);
      setGroupsLoaded(true);
      return;
    }

    let cancelled = false;
    async function load() {
      try {
        const [groupsRes, searchesRes] = await Promise.all([
          api.get<{ tag_groups?: TagGroup[] }>("/api/v1/users/me/tag-groups"),
          api.get<{ saved_searches?: SavedSearch[] }>("/api/v1/users/me/saved-searches"),
        ]);
        if (!cancelled) {
          setTagGroups(groupsRes.tag_groups ?? []);
          setSavedSearches(searchesRes.saved_searches ?? []);
        }
      } catch {
        if (!cancelled) {
          setTagGroups([]);
          setSavedSearches([]);
        }
      } finally {
        if (!cancelled) setGroupsLoaded(true);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [user]);

  // ── Handlers ───────────────────────────────────────

  function handleCategorySelect(cat: string) {
    const next = selectedCategory === cat ? "" : cat;
    setSelectedCategory(next);
    setSelectedTags([]);
    setAvailableTags([]);
  }

  function handleTagToggle(tag: string) {
    setSelectedTags((prev) =>
      prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag],
    );
  }

  function handleRemoveTag(tag: string) {
    setSelectedTags((prev) => prev.filter((t) => t !== tag));
  }

  function handleClearTags() {
    setSelectedTags([]);
  }

  function handleContentTypeToggle(ct: string) {
    setContentTypes((prev) =>
      prev.includes(ct) ? prev.filter((t) => t !== ct) : [...prev, ct],
    );
  }

  function handleApplyTagGroup(group: TagGroup) {
    setSelectedTags([...group.tags]);
    toast("success", t('search.filter.appliedTagGroup', { name: group.name }));
  }

  async function handleSaveSearch() {
    if (!saveName.trim()) return;
    setSaving(true);
    try {
      await api.post("/api/v1/users/me/saved-searches", {
        name: saveName.trim(),
        config: buildConfig(),
      });
      toast("success", t('search.filter.searchSaved'));
      setSaveModalOpen(false);
      setSaveName("");
      const searchesRes = await api.get<{ saved_searches?: SavedSearch[] }>(
        "/api/v1/users/me/saved-searches",
      );
      setSavedSearches(searchesRes.saved_searches ?? []);
    } catch {
      toast("error", t('search.filter.saveFailed'));
    } finally {
      setSaving(false);
    }
  }

  function handleApplySavedSearch(search: SavedSearch) {
    const config = normalizeSearchFilters(search.config);
    setSelectedCategory(config.category ?? "");
    setSelectedTags(config.selectedTags);
    setContentTypes(config.contentTypes);
    setTimeRange(config.timeRange ?? "");
    setSort(config.sort ?? "");
    toast("success", t('search.filter.loadedSearch', { name: search.name }));
  }

  // ── Derived ────────────────────────────────────────

  const tagColorCycle = ["blue", "green", "purple", "orange"] as const;

  // ── Render ─────────────────────────────────────────

  return (
    <aside
      className={cn(
        "flex w-full shrink-0 flex-col gap-4 rounded-md border border-border bg-card p-4 shadow-none min-[1101px]:w-[260px]",
        className,
      )}
    >
      {/* Category tabs */}
      <div className="flex flex-col gap-2">
        <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
          {t('search.filter.mainCategory')}
        </span>
        <div className="flex flex-wrap gap-1.5">
          {categories.map((cat) => {
            const active = selectedCategory === cat.name;
            return (
              <button
                key={cat.id}
                type="button"
                onClick={() => handleCategorySelect(cat.name)}
                className={cn(
                  "inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-medium transition-colors duration-150 select-none",
                  "focus:outline-none focus:ring-2 focus:ring-ring",
                  active
                    ? "border-primary bg-primary/10 text-primary"
                    : "border-transparent bg-muted text-muted-foreground hover:border-border hover:text-foreground hover:bg-muted/80 cursor-pointer",
                )}
              >
                {cat.name}
              </button>
            );
          })}
        </div>
      </div>

      {/* Selected tags chips */}
      {selectedTags.length > 0 && (
        <div className="flex flex-col gap-1.5">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
              {t('search.filter.selectedTags')}
            </span>
            <button
              type="button"
              onClick={handleClearTags}
              className="text-xs text-primary hover:underline"
            >
              {t('search.filter.clearAll')}
            </button>
          </div>
          <div className="flex flex-wrap gap-1">
            {selectedTags.map((tag, i) => (
              <TagBadge
                key={tag}
                color={tagColorCycle[i % tagColorCycle.length]}
                onRemove={() => handleRemoveTag(tag)}
              >
                {tag}
              </TagBadge>
            ))}
          </div>
        </div>
      )}

      {/* Available tags */}
      <div className="flex flex-col gap-1.5">
        <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
          {t('search.filter.tags')}
        </span>
        {tagsLoading ? (
          <div className="space-y-1.5">
            <Skeleton className="h-6 w-full" />
            <Skeleton className="h-6 w-3/4" />
            <Skeleton className="h-6 w-5/6" />
            <Skeleton className="h-6 w-2/3" />
          </div>
        ) : availableTags.length === 0 ? (
          <p className="text-xs text-muted-foreground/70 py-1">
            {selectedCategory ? t('search.filter.noTagsForCategory') : t('search.filter.selectCategoryFirst')}
          </p>
        ) : (
          <div className="flex flex-wrap gap-1 max-h-[300px] overflow-y-auto">
            {availableTags.map((tag) => {
              const isSelected = selectedTags.includes(tag.name);
              return (
                <button
                  key={tag.name}
                  type="button"
                  onClick={() => handleTagToggle(tag.name)}
                  disabled={isSelected}
                  className={cn(
                    "inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs transition-colors duration-150 select-none",
                    "focus:outline-none focus:ring-2 focus:ring-ring",
                    isSelected
                      ? "border-primary bg-primary/10 text-primary cursor-default"
                      : "border-transparent bg-muted text-muted-foreground hover:border-border hover:text-foreground hover:bg-muted/80 cursor-pointer",
                  )}
                >
                  {tag.name}
                  <span className="text-xs tabular-nums text-muted-foreground/70">
                    {tag.count}
                  </span>
                </button>
              );
            })}
          </div>
        )}
      </div>

      {/* Advanced section */}
      <div className="border-t border-border pt-2">
        <button
          type="button"
          onClick={() => setAdvancedOpen(!advancedOpen)}
          className="flex w-full items-center justify-between text-xs font-medium text-muted-foreground uppercase tracking-wide hover:text-foreground transition-colors"
        >
          <span className="inline-flex items-center gap-1.5">
            <SlidersHorizontal className="h-3.5 w-3.5" />
            {t('search.filter.advancedFilter')}
          </span>
          {advancedOpen ? (
            <ChevronUp className="h-3.5 w-3.5" />
          ) : (
            <ChevronDown className="h-3.5 w-3.5" />
          )}
        </button>

        {advancedOpen && (
          <div className="mt-3">
            <div className="flex flex-col gap-3">
              {/* Content type multi-select */}
              <div className="flex flex-col gap-1.5">
                <span className="text-xs font-medium text-muted-foreground">{t('search.contentType')}</span>
                <div className="flex flex-wrap gap-1">
                  {CONTENT_TYPE_OPTIONS.map((opt) => {
                    const active = contentTypes.includes(opt.key);
                    return (
                      <button
                        key={opt.key}
                        type="button"
                        onClick={() => handleContentTypeToggle(opt.key)}
                        className={cn(
                          "inline-flex items-center rounded-full border px-2 py-0.5 text-xs transition-colors duration-150 select-none",
                          "focus:outline-none focus:ring-2 focus:ring-ring",
                          active
                            ? "border-primary bg-primary/10 text-primary"
                            : "border-border bg-transparent text-muted-foreground hover:border-primary hover:text-foreground hover:bg-muted/30 cursor-pointer",
                        )}
                      >
                        {t(opt.label)}
                      </button>
                    );
                  })}
                </div>
              </div>

              {/* Time range */}
              <div className="flex flex-col gap-1">
                <span className="text-xs font-medium text-muted-foreground">{t('search.timeRange')}</span>
                <Select
                  aria-label={t('search.timeRange')}
                  value={timeRange}
                  onChange={(e) => setTimeRange(e.target.value)}
                  className="h-8 rounded-md bg-card px-2 py-1.5 text-xs"
                >
                  {TIME_RANGE_OPTIONS.map((opt) => (
                    <option key={opt.key} value={opt.key}>
                      {t(opt.label)}
                    </option>
                  ))}
                </Select>
              </div>

              {/* Sort */}
              <div className="flex flex-col gap-1">
                <span className="text-xs font-medium text-muted-foreground">{t('search.sortBy')}</span>
                <Select
                  aria-label={t('search.sortBy')}
                  value={sort}
                  onChange={(e) => setSort(e.target.value)}
                  className="h-8 rounded-md bg-card px-2 py-1.5 text-xs"
                >
                  {SORT_OPTIONS.map((opt) => (
                    <option key={opt.key} value={opt.key}>
                      {t(opt.label)}
                    </option>
                  ))}
                </Select>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Active advanced filters summary */}
      {(contentTypes.length > 0 || timeRange || sort) && (
        <div className="flex flex-wrap gap-1 text-xs text-muted-foreground/70">
          {contentTypes.map((ct) => (
            <span key={ct} className="inline-flex items-center gap-1 rounded border border-border px-1.5 py-0.5">
              {t(CONTENT_TYPE_OPTIONS.find((o) => o.key === ct)?.label ?? "") || ct}
              <button
                type="button"
                onClick={() => handleContentTypeToggle(ct)}
                aria-label={t('common.removeTag', { tag: t(CONTENT_TYPE_OPTIONS.find((o) => o.key === ct)?.label ?? "") || ct })}
                className="hover:text-foreground hover:bg-muted/50 rounded-sm transition-colors"
              >
                <X className="h-3 w-3" />
              </button>
            </span>
          ))}
          {timeRange && (
            <span className="inline-flex items-center gap-1 rounded border border-border px-1.5 py-0.5">
              {t(TIME_RANGE_OPTIONS.find((o) => o.key === timeRange)?.label ?? "") || timeRange}
              <button
                type="button"
                onClick={() => setTimeRange("")}
                aria-label={t('common.removeTag', { tag: t(TIME_RANGE_OPTIONS.find((o) => o.key === timeRange)?.label ?? "") || timeRange })}
                className="hover:text-foreground hover:bg-muted/50 rounded-sm transition-colors"
              >
                <X className="h-3 w-3" />
              </button>
            </span>
          )}
          {sort && (
            <span className="inline-flex items-center gap-1 rounded border border-border px-1.5 py-0.5">
              {t(SORT_OPTIONS.find((o) => o.key === sort)?.label ?? "") || sort}
              <button
                type="button"
                onClick={() => setSort("")}
                aria-label={t('common.removeTag', { tag: t(SORT_OPTIONS.find((o) => o.key === sort)?.label ?? "") || sort })}
                className="hover:text-foreground hover:bg-muted/50 rounded-sm transition-colors"
              >
                <X className="h-3 w-3" />
              </button>
            </span>
          )}
        </div>
      )}

      {/* Save search button */}
      <Button
        variant="outline"
        size="sm"
        className="w-full text-xs"
        onClick={() => setSaveModalOpen(true)}
      >
        <Save className="mr-1.5 h-3.5 w-3.5" />
        {t('search.saveSearch')}
      </Button>

      {/* Saved searches */}
      {user && savedSearches.length > 0 && (
        <div className="flex flex-col gap-1.5 border-t border-border pt-2">
          <span className="text-xs font-medium text-muted-foreground inline-flex items-center gap-1.5">
            <Bookmark className="h-3.5 w-3.5" />
            {t('search.filter.mySearches')}
          </span>
          <div className="flex flex-col gap-0.5">
            {savedSearches.map((s) => (
              <button
                key={s.id}
                type="button"
                onClick={() => handleApplySavedSearch(s)}
                className="text-xs text-left text-muted-foreground hover:text-foreground hover:bg-muted rounded-md px-2 py-1 transition-colors duration-150 truncate"
              >
                {s.name}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* My tag groups */}
      {user && tagGroups.length > 0 && (
        <div className="flex flex-col gap-1.5 border-t border-border pt-2">
          <span className="text-xs font-medium text-muted-foreground inline-flex items-center gap-1.5">
            <FolderOpen className="h-3.5 w-3.5" />
            {t('search.filter.myTagGroups')}
          </span>
          <div className="flex flex-col gap-1">
            {tagGroups.map((group) => (
              <button
                key={group.id}
                type="button"
                onClick={() => handleApplyTagGroup(group)}
                className="flex items-center justify-between text-xs text-muted-foreground hover:text-foreground hover:bg-muted rounded px-2 py-1 transition-colors"
              >
                <span className="truncate">{group.name}</span>
                <span className="ml-1 shrink-0 text-xs text-muted-foreground/70">
                  {group.tags.length}
                </span>
              </button>
            ))}
          </div>
        </div>
      )}

      {/* My tag groups empty state */}
      {user && groupsLoaded && tagGroups.length === 0 && (
        <div className="border-t border-border pt-2">
          <p className="text-xs text-muted-foreground/70">{t('search.filter.noTagGroups')}</p>
        </div>
      )}

      {/* Not logged in hint */}
      {!user && (
        <div className="border-t border-border pt-2">
          <p className="text-xs text-muted-foreground/70">
            {t('search.filter.loginToSave')}
          </p>
        </div>
      )}

      {/* Save search modal */}
      {saveModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30">
          <div className="w-full max-w-sm rounded-md border border-border bg-card p-4 shadow-md">
            <h3 className="text-sm font-semibold text-foreground mb-3">{t('search.filter.saveSearchTitle')}</h3>
            <input
              type="text"
              value={saveName}
              onChange={(e) => setSaveName(e.target.value)}
              placeholder={t('search.filter.enterSearchName')}
              className="w-full rounded-md border border-border bg-card px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground/70 focus:outline-none focus:ring-2 focus:ring-ring mb-3"
              autoFocus
              onKeyDown={(e) => {
                if (e.key === "Enter") handleSaveSearch();
                if (e.key === "Escape") {
                  setSaveModalOpen(false);
                  setSaveName("");
                }
              }}
            />
            <div className="flex justify-end gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setSaveModalOpen(false);
                  setSaveName("");
                }}
              >
                {t('common.cancel')}
              </Button>
              <Button
                size="sm"
                onClick={handleSaveSearch}
                disabled={!saveName.trim() || saving}
              >
                {saving && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
                {t('common.save')}
              </Button>
            </div>
          </div>
        </div>
      )}
    </aside>
  );
}
