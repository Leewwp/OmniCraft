import { getServerApiBase } from "@/lib/server-api";
import { getTranslations } from 'next-intl/server';
import { OverlayMasonryGrid } from "@/components/content/OverlayMasonryGrid";
import { SortSelect } from "@/components/original/SortSelect";
import { CategoryTabs } from "@/components/original/CategoryTabs";
import { SidebarWrapper } from "@/components/original/OriginalSidebar";
import { normalizeContentList } from "@/lib/content";
import { resolveDefaultSort } from "@/lib/search-filters";

interface CategoryItem {
  id: number; slug: string; name_i18n?: Record<string, string>;
}
interface CategoryDisplay {
  slug: string;
  i18n: string;
  name_i18n?: Record<string, string>;
}
interface ContentResponse { contents?: unknown[]; }
interface SearchParams { category?: string; sort?: string; }

const PRIMARY_CATEGORIES_FALLBACK: CategoryDisplay[] = [
  { slug: "", i18n: "home.categoryRecommended" },
  { slug: "film_tv", i18n: "home.categoryFilmTv" },
  { slug: "gaming", i18n: "home.categoryGaming" },
  { slug: "literature", i18n: "home.categoryLiterature" },
  { slug: "pet", i18n: "home.categoryPet" },
  { slug: "food", i18n: "home.categoryFood" },
  { slug: "beauty_fashion", i18n: "home.categoryBeautyFashion" },
  { slug: "home", i18n: "home.categoryHome" },
  { slug: "tech_digital", i18n: "home.categoryTechDigital" },
  { slug: "travel", i18n: "home.categoryTravel" },
  { slug: "sports", i18n: "home.categorySports" },
  { slug: "productivity", i18n: "home.categoryProductivity" },
];

function normalizeSlug(s: string) { return s.endsWith("_orig") ? s.slice(0, -5) : s; }

async function fetchCategories(apiBase: string): Promise<CategoryDisplay[]> {
  try {
    const res = await fetch(`${apiBase}/categories?zone=original&level=primary`, { cache: "no-store" });
    if (!res.ok) return PRIMARY_CATEGORIES_FALLBACK;
    const data = (await res.json()) as { categories?: CategoryItem[] };
    const cats = (data.categories || []).map(item => {
      const slug = normalizeSlug(item.slug);
      const fb = PRIMARY_CATEGORIES_FALLBACK.find(c => c.slug === slug);
      return { slug, i18n: fb?.i18n || "", name_i18n: item.name_i18n };
    }).filter(item => PRIMARY_CATEGORIES_FALLBACK.some(c => c.slug === item.slug));
    return [PRIMARY_CATEGORIES_FALLBACK[0], ...cats.filter(c => c.slug !== "")];
  } catch { return PRIMARY_CATEGORIES_FALLBACK; }
}

interface StatsSummary { users: number; ips: number; contents: number; }

async function fetchStats(apiBase: string): Promise<StatsSummary | null> {
  try {
    const res = await fetch(`${apiBase}/stats/summary`, { cache: "no-store" });
    if (!res.ok) return null;
    const data = await res.json() as { summary: StatsSummary };
    return data.summary || null;
  } catch { return null; }
}

async function fetchContents(apiBase: string, search: Required<SearchParams>) {
  const sort = resolveDefaultSort({ category: search.category, sort: search.sort });
  const params = new URLSearchParams({ zone: "original", sort, time_range: "all", page_size: "24" });
  if (search.category) params.set("category", search.category);
  try {
    const res = await fetch(`${apiBase}/contents?${params.toString()}`, { cache: "no-store" });
    if (!res.ok) return [];
    return normalizeContentList(((await res.json()) as ContentResponse).contents);
  } catch { return []; }
}

export default async function OriginalPage({ searchParams }: { searchParams: Promise<SearchParams> }) {
  const t = await getTranslations();
  const raw = await searchParams;
  const current = { category: raw.category || "", sort: raw.sort || "" };
  const apiBase = getServerApiBase();
  const [categories, contents, stats] = await Promise.all([fetchCategories(apiBase), fetchContents(apiBase, current), fetchStats(apiBase)]);

  return (
    <div className="mx-auto flex w-full max-w-[1280px] min-h-[calc(100vh-52px)]">
      {/* Sidebar — using client wrapper */}
      <SidebarWrapper />

      {/* Main content */}
      <div className="flex-1 min-w-0">
        {/* Zone banner */}
        <div className="px-4 pt-5 pb-3 md:px-6">
          <div className="flex items-baseline gap-3">
            <h1 className="text-[22px] font-bold tracking-tight text-foreground">{t("content.originalZone")}</h1>
            <p className="text-sm text-muted-foreground">{t("content.originalZoneDesc")}</p>
          </div>
          <div className="mt-3 flex gap-4">
            <span className="flex items-baseline gap-1"><span className="text-[15px] font-semibold text-foreground">{stats ? stats.contents.toLocaleString() : "--"}</span><span className="text-xs text-muted-foreground">{t('home.contentCountLabel')}</span></span>
            <span className="flex items-baseline gap-1"><span className="text-[15px] font-semibold text-foreground">{stats ? stats.users.toLocaleString() : "--"}</span><span className="text-xs text-muted-foreground">{t('home.creatorsLabel')}</span></span>
          </div>
        </div>

        {/* Category tabs + sort — unified sticky row */}
        <div className="sticky top-[52px] z-40 border-b border-border-default bg-canvas-default px-4 py-2.5 md:px-6">
          <div className="flex items-center gap-0">
            <CategoryTabs categories={categories} currentCategory={current.category} />
            <div className="ml-3 flex-shrink-0"><SortSelect /></div>
          </div>
        </div>

        {/* Content masonry */}
        <div className="px-4 pt-4 pb-16 md:px-6">
          <OverlayMasonryGrid items={contents} emptyText={t("home.noOriginalContent")} source="zone-page" />
        </div>
      </div>
    </div>
  );
}
