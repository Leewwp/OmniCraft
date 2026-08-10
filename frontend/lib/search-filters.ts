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

// #81 排序默认值语义：无分类且未显式选排序 → recommended；选中任何分类（或
// 携带其他筛选）且未显式选排序 → hot。推荐管线无视分类等筛选条件，不得把
// "recommended" 误传给带筛选的请求；显式排序原样保留（深链
// sort=recommended+分类 由后端防御性降级为 hot）。
export function resolveDefaultSort(config: SearchFilterConfig): string {
  const explicitSort = config.sort?.trim();
  if (explicitSort) return explicitSort;
  return config.category?.trim() ? "hot" : "recommended";
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
