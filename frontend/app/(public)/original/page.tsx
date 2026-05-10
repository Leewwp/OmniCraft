import Link from "next/link";
import { getTranslations } from 'next-intl/server';
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { SortSelect } from "@/components/original/SortSelect";
import { normalizeContentList } from "@/lib/content";

interface CategoryItem {
  id: number;
  slug: string;
  name_i18n?: Record<string, string>;
}

interface ContentResponse {
  contents?: unknown[];
}

interface SearchParams {
  category?: string;
  sort?: string;
}

const PRIMARY_CATEGORIES_FALLBACK = [
  { slug: "", label: "推荐", i18n: "home.categoryRecommended" },
  { slug: "film_tv", label: "影视", i18n: "home.categoryFilmTv" },
  { slug: "gaming", label: "游戏", i18n: "home.categoryGaming" },
  { slug: "literature", label: "文学", i18n: "home.categoryLiterature" },
  { slug: "pet", label: "宠物", i18n: "home.categoryPet" },
  { slug: "food", label: "美食", i18n: "home.categoryFood" },
  { slug: "beauty_fashion", label: "美妆穿搭", i18n: "home.categoryBeautyFashion" },
  { slug: "home", label: "家居", i18n: "home.categoryHome" },
  { slug: "tech_digital", label: "数码科技", i18n: "home.categoryTechDigital" },
  { slug: "travel", label: "旅行", i18n: "home.categoryTravel" },
  { slug: "sports", label: "运动", i18n: "home.categorySports" },
  { slug: "productivity", label: "效率", i18n: "home.categoryProductivity" },
];

function getApiBase() {
  const raw = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  return `${raw.replace(/\/$/, "")}/api/v1`;
}

function normalizeCategorySlug(slug: string) {
  return slug.endsWith("_orig") ? slug.slice(0, -5) : slug;
}

async function fetchPrimaryCategories(apiBase: string) {
  try {
    const res = await fetch(`${apiBase}/categories?zone=original&level=primary`, { cache: "no-store" });
    if (!res.ok) return PRIMARY_CATEGORIES_FALLBACK;
    const data = (await res.json()) as { categories?: CategoryItem[] };
    const categories = (data.categories || [])
      .map((item) => {
        const slug = normalizeCategorySlug(item.slug);
        const fallback = PRIMARY_CATEGORIES_FALLBACK.find((c) => c.slug === slug);
        return {
          slug,
          label: fallback?.label || item.name_i18n?.zh || item.name_i18n?.en || item.slug,
          i18n: fallback?.i18n || "",
        };
      })
      .filter((item) => PRIMARY_CATEGORIES_FALLBACK.some((c) => c.slug === item.slug));
    return [
      PRIMARY_CATEGORIES_FALLBACK[0],
      ...categories.filter((c) => c.slug !== ""),
    ];
  } catch {
    return PRIMARY_CATEGORIES_FALLBACK;
  }
}

function buildHref(next: Partial<SearchParams>, current: Required<SearchParams>) {
  const params = new URLSearchParams();
  const merged = { ...current, ...next };
  if (merged.category) params.set("category", merged.category);
  if (merged.sort && merged.sort !== "recommended") params.set("sort", merged.sort);
  const query = params.toString();
  return query ? `/original?${query}` : "/original";
}

async function fetchContents(apiBase: string, search: Required<SearchParams>) {
  const sortMode = !search.category && search.sort === "recommended" ? "recommended" : (search.sort || "hot");
  const params = new URLSearchParams({
    zone: "original",
    sort: sortMode,
    time_range: "all",
    page_size: "24",
  });
  if (search.category) params.set("category", search.category);

  try {
    const res = await fetch(`${apiBase}/contents?${params.toString()}`, { cache: "no-store" });
    if (!res.ok) return [];
    const data = (await res.json()) as ContentResponse;
    return normalizeContentList(data.contents);
  } catch {
    return [];
  }
}

export default async function OriginalPage({ searchParams }: { searchParams: Promise<SearchParams> }) {
  const t = await getTranslations();
  const rawSearch = await searchParams;
  const current = {
    category: rawSearch.category || "",
    sort: rawSearch.sort || "recommended",
  };
  const apiBase = getApiBase();
  const [categories, contents] = await Promise.all([
    fetchPrimaryCategories(apiBase),
    fetchContents(apiBase, current),
  ]);

  return (
    <div className="mx-auto w-full max-w-[1440px]">
      {/* Zone banner — clean, no border */}
      <div className="px-6 pt-6 pb-3">
        <div className="flex items-baseline gap-3">
          <h1 className="text-[22px] font-bold tracking-tight text-foreground">
            {t("content.originalZone")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("content.originalZoneDesc")}
          </p>
        </div>
        <div className="mt-3 flex gap-4">
          <span className="flex items-baseline gap-1">
            <span className="text-[15px] font-semibold text-foreground">186,247</span>
            <span className="text-xs text-muted-foreground">内容</span>
          </span>
          <span className="flex items-baseline gap-1">
            <span className="text-[15px] font-semibold text-foreground">32,814</span>
            <span className="text-xs text-muted-foreground">创作者</span>
          </span>
        </div>
      </div>

      {/* Category tabs + sort — unified row, sticky */}
      <div className="sticky top-[52px] z-40 bg-background px-6 py-2.5">
        <div className="flex items-center gap-0">
          {/* Category tabs — horizontal scroll */}
          <div className="flex flex-1 items-center gap-1 overflow-x-auto scrollbar-none" style={{ scrollbarWidth: 'none' }}>
            {categories.map((category) => {
              const active = current.category === category.slug;
              return (
                <Link
                  key={category.slug || "recommended"}
                  href={buildHref({ category: category.slug }, current)}
                  className={`flex-shrink-0 rounded-full border px-3.5 py-1.5 text-[13px] font-medium transition-colors whitespace-nowrap ${
                    active
                      ? "border-border bg-card text-foreground font-semibold"
                      : "border-transparent text-muted-foreground hover:text-foreground hover:bg-muted"
                  }`}
                >
                  {category.label}
                </Link>
              );
            })}
          </div>

          {/* Sort dropdown */}
          <div className="ml-3 flex-shrink-0">
            <SortSelect />
          </div>
        </div>
      </div>

      {/* Content masonry */}
      <div className="px-6 pt-4 pb-16">
        <MasonryGrid items={contents} emptyText={t("home.noOriginalContent")} />
      </div>
    </div>
  );
}
