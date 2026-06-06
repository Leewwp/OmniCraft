export interface SearchFilterConfig {
  category?: string;
  selectedTags?: string[];
  selected_tags?: string[];
  contentTypes?: string[];
  content_types?: string[];
  timeRange?: string;
  time_range?: string;
  sort?: string;
}

export interface NormalizedSearchFilterConfig {
  category?: string;
  selectedTags: string[];
  contentTypes: string[];
  timeRange?: string;
  sort?: string;
}

function compactList(value: string[] | undefined): string[] {
  return Array.isArray(value) ? value.map((item) => item.trim()).filter(Boolean) : [];
}

export function normalizeSearchFilters(config: SearchFilterConfig): NormalizedSearchFilterConfig {
  return {
    category: config.category?.trim() || undefined,
    selectedTags: compactList(config.selectedTags ?? config.selected_tags),
    contentTypes: compactList(config.contentTypes ?? config.content_types),
    timeRange: config.timeRange ?? config.time_range ?? undefined,
    sort: config.sort?.trim() || undefined,
  };
}

export function buildContentSearchParams(query: string, config: SearchFilterConfig): URLSearchParams {
  const filters = normalizeSearchFilters(config);
  const params = new URLSearchParams();
  params.set("q", query);
  if (filters.category) params.set("category", filters.category);
  if (filters.selectedTags.length > 0) params.set("tags", filters.selectedTags.join(","));
  if (filters.contentTypes.length > 0) params.set("content_type", filters.contentTypes.join(","));
  if (filters.timeRange) params.set("time_range", filters.timeRange);
  if (filters.sort) params.set("sort", filters.sort);
  return params;
}

export function buildContentSearchPath(query: string, config: SearchFilterConfig): string {
  return `/api/v1/contents/search?${buildContentSearchParams(query, config).toString()}`;
}
